#!/bin/sh

#
# A script to facilitate triggering ArgoCD deploys from Jenkins.
#

set -e
set -o pipefail

OUTPUT_DIR="${OUTPUT_DIR:-$( pwd )/output}"
TMP_DIR="/tmp/deploy.sh-$$-tmp"

ARGOCD_ADDR="${ARGOCD_ADDR:-ap-argocd.dsp-devops.broadinstitute.org:443}"

ARGOCD_SYNC_TIMEOUT="${ARGOCD_SYNC_TIMEOUT:-180}"

COLORIZE=${COLORIZE:-true}

export VAULT_ADDR=${VAULT_ADDR:-https://clotho.broadinstitute.org:8200}

fetch_argocd_token_from_vault(){
  local env="$1"

  if [[ -z "${VAULT_TOKEN}" ]]; then
    echo "Error: \$VAULT_TOKEN not set" >&2
    exit 1
  fi

  path=secret/devops/ci/argocd/jenkins-terra-sync-token
  if [[ "$env" == "prod" ]]; then
    path=secret/suitable/argocd/jenkins-terra-sync-token
  fi

  token=$( vault read -format=json "${path}" | jq -r '.data.token' )
  export ARGOCD_TOKEN="${token}"
}

# Echo a message formatted in ANSI color, with vertical spacing
colorize_message() {
  fmt="$1"
  shift

  [[ "${COLORIZE}" == "true" ]] && echo -ne "\e[${fmt}m" >&2
  echo -n "$@" >&2
  [[ "${COLORIZE}" == "true" ]] && echo -ne "\e[0m" >&2
  echo >&2
}

debug(){
  colorize_message "0" "[debug] " $@
}

info(){
  colorize_message "1" "[ info] " $@
}

warn(){
  colorize_message "1;31" "[ warn] " $@
}

error(){
  colorize_message "1;31" "[error] " $@
}


print_help(){
  cat <<HELP
Usage: $0 (properties|sync) ENV [PROJECT]

This script supports two actions:

  * properties - generate properties for a Jenkins trigger. Eg.

           \$ $0 properties dev

           will generate a list of properties files in the current working
           directory like:

             cromwell.properties
             workspacemanager.properties

           Where the contents of each file looks like:

             PROJECT=cromwell
             ENV=dev

           This is used to trigger downstream jobs in Jenkins.

  * sync PROJECT - sync a project in an environment. Eg.

           \$ $0 sync dev cromwell

  Note: The VAULT_TOKEN environment variable is required.

HELP
}

# Run an ArgoCD CLI command
argo_cli() {
  if [[ -z "${ARGOCD_TOKEN}" ]]; then
    error "Please supply an ArgoCD token via ARGOCD_TOKEN variable"
    return 1
  fi
  debug "\$ argocd $@"
  argocd \
  --grpc-web \
  --server "${ARGOCD_ADDR}" \
  --auth-token "${ARGOCD_TOKEN}" \
  "$@"
}

# List all ArgoCD Apps
list_all(){
  argo_cli app list -o name
}

# List ArgoCD Apps by label
list_selector(){
  local selector="$1"
  argo_cli app list -o name -l "${selector}"
}

# Check whether an app exists
app_exists(){
  local app="$1"
  list_all | grep -Fx "${app}"
}

# Diff an app, returning 0 if no differences and 1 if there are differences
diff() {
  local app="$1"

  # For some annoying reason, ArgoCD diff calls are sometimes flaky and fail
  # with error messages like this:
  #
  # msg="rpc error: code = Unknown desc = POST https://ap-argocd.dsp-devops.broadinstitute.org:443/application.ApplicationService/Get failed with status code 502"
  #
  # `argocd` diff returns 1 if an error occurs, OR if there are differences, so we
  # have implemented silly error checking / retry by sending errors to a file and checking
  # if it includes the text "rpc error"
  #
  local tries=3
  while [[ $tries -gt 0 ]]; do
    tries=$(( $tries - 1 ));

    local errfile="${TMP_DIR}/diff-${tries}.err"

    if argo_cli app diff "${app}" --hard-refresh 2>"${errfile}"; then
      cat "${errfile}" >&2

      debug "No differences found for ${app}"

      return 0
    else
      cat "${errfile}" >&2

      if grep "rpc error" "${errfile}" >/dev/null; then
        warn "Failed to pull a diff for ${app} from argocd, will retry ${tries} more times"

      else
        debug "${app} is out of sync"
        return 1
      fi
    fi
    sleep 5
  done

  error "Could not successfully pull a diff for ${app} from ArgoCD after multiple tries"
  return 2
}

# Sync an app
sync() {
  local app="$1"

  info "Preparing to sync ArgoCD app: ${app}"

  argo_cli app sync "${app}" --prune --timeout "${ARGOCD_SYNC_TIMEOUT}"
}

# Restart all deployments in an ArgoCD Application
# (actions don't support label selection so this applies to a single app)
restart(){
  local app="$1"
  info "Restarting all Deployments in ArgoCD Application: ${app}"

  if ! argo_cli app actions list --kind Deployment "${app}"; then
    warn "No Deployments found in ${app}"
    return
  fi

  argo_cli app actions run --kind Deployment "${app}" restart --all
}

# Check if a project requires a sync
sync_required(){
  local env="$1"
  local project="$2"
  local app="${project}-${env}"
  local legacy_configs_app="${project}-configs-${env}"

  if app_exists "${legacy_configs_app}"; then
    if ! diff "${legacy_configs_app}"; then
      info "${legacy_configs_app} has differences, ${project} sync is required!"
      return 0
    fi
  fi

  if ! diff "${app}"; then
    info "${app} has differences, ${project} sync is required!"
    return 0
  fi

  return 1
}

sync_project(){
  local env="$1"
  local project="$2"
  local app="${project}-${env}"
  local legacy_configs_app="${project}-configs-${env}"

  # While transitioning to Kubernetes, some apps have a separate upstream
  # app that pulls configuration from firecloud-develop. This helps guarantee
  # that K8s configuration and GCE vm configuration are identical.
  #
  # If this app has a corresponding legacy-configs app that pulls
  # configuration out of firecloud-develop, then:
  #   1. sync the firecloud-develop configs
  #   2. sync the app
  #   3. restart all deployments in the app to ensure firecloud-develop config
  #      changes are picked up
  #
  # Note: This means that apps with a legacy configs app might be restarted
  # twice during deployments.
  #
  local has_legacy_configs=false
  if app_exists "${legacy_configs_app}"; then
    has_legacy_configs=true
  fi

  if [[ "${has_legacy_configs}" == "true" ]]; then
    diff "${legacy_configs_app}" || true # Print diff
    sync "${legacy_configs_app}"
  fi

  diff "${app}" || true # Print diff
  sync "${app}"

  if [[ "${has_legacy_configs}" == "true" ]]; then
    restart "${app}"
  fi
}

generate_properties(){
  local env="$1"

  info "Will write properties to ${OUTPUT_DIR}"

  mkdir -p $OUTPUT_DIR

  info "Looking for projects in ${env}..."
  local selector="env=${env},type=app,jenkins-sync-enabled=true"

  list_selector "${selector}" | while read app; do
    local project=$( echo "${app}" | cut -d- -f1 )

    info "--------------------------------------------------"
    info ">>> ${project}"
    info "Checking if ${project} requires a sync in ${env}"

    if sync_required "${env}" "${project}"; then
      local file="${OUTPUT_DIR}/${project}.properties"

      info "Sync required, generating deploy properties ${file} for ${project} in ${env}"

      cat <<EOF | tee "${file}"
PROJECT=${project}
ENV=${env}
EOF

    else
      info "No sync required, skipping deploy for ${project} in ${env}!"
    fi

  done
}

mkdir -p "${TMP_DIR}"
trap "rm -rf ${TMP_DIR}" EXIT

if [[ $# -lt 2 ]]; then
  print_help
  exit 1
fi

action="$1"
environment="$2"

if [[ -z "${environment}" ]]; then
  echo "Invalid environment: \"${environment}\""
fi

fetch_argocd_token_from_vault "${environment}"

if [[ "$action" == "properties" ]] && [[ $# -eq 2 ]]; then
  generate_properties "${environment}"

elif [[ "$action" == "sync" ]] && [[ $# -eq 3 ]]; then
  project="$3"
  if [[ -z "${project}" ]]; then
    echo "Invalid project: \"${project}\""
  fi
  sync_project "${environment}" "${project}"

else
  print_help
  exit 1
fi
