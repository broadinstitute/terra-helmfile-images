#!/bin/bash

#
# Render manifests for a Terra chart release in a target environment or cluster.
#
# This script is executed by the ArgoCD applications for Terra apps. (eg. cromwell-dev)
#
# TODO: We should make render use viper at some point, it would make these scripts
# muuuch simpler.

set -eo pipefail
set -x

PATH="/thelma/bin:${PATH}"
export PATH

target=
if [[ -n "${TERRA_ENV}" ]]; then
  target="--env ${TERRA_ENV}"
elif [[ -n "${TERRA_CLUSTER}" ]]; then
  target="--cluster ${TERRA_CLUSTER}"
else
  echo "Usage: Please specify TERRA_ENV or TERRA_CLUSTER as an environment variable" >&2
  exit 1
fi

if [[ -z "${TERRA_RELEASE}" ]]; then
  if [[ -z "${TERRA_APP}" ]]; then
    echo "Usage: Please specify TERRA_RELEASE as an environment variable" >&2
    exit 1
  fi
  # Make it possible to use TERRA_APP to set release name,
  # for backwards compatibility
  TERRA_RELEASE="${TERRA_APP}"
fi

if [[ -z "${THELMA_RENDER_MODE}" ]]; then
  echo "Usage: Please specify THELMA_RENDER_MODE as an environment variable" >&2
  exit 1
fi

if [[ "$1" == 'init' ]]; then
  : # Nothing to do
elif [[ "$1" == 'generate' ]]; then
  # Delegate to `thelma render`
  args=()
  if [[ -n "${TERRA_APP_VERSION}" ]]; then
    args+=( --app-version "${TERRA_APP_VERSION}" )
  fi

  if [[ -n "${TERRA_CHART_VERSION}" ]]; then
    args+=( --chart-version "${TERRA_CHART_VERSION}" )
  fi

  THELMA_HOME=$( pwd )
  export THELMA_HOME

  thelma render --stdout $target -r "${TERRA_RELEASE}" --mode "${THELMA_RENDER_MODE}" "${args[@]}"
else
  echo "Usage: ${0} (init|generate)" >&2
  exit 1
fi
