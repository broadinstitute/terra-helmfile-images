#!/bin/bash

#
# Render manifests for a Terra app in an environment
#
# This is run by the ArgoCD applications for Terra apps.
#

set -eo pipefail
set -x

if [[ -z "${TERRA_ENV}" ]]; then
  echo "Usage: Please specify TERRA_ENV as an environment variable" >&2
  exit 1
fi

if [[ -z "${TERRA_APP}" ]]; then
  echo "Usage: Please specify TERRA_APP as an environment variable" >&2
  exit 1
fi

if [[ "$1" == 'init' ]]; then
  : # Nothing to do
elif [[ "$1" == 'generate' ]]; then
  # Delegate to render script
  args=()
  if [[ -n "${TERRA_APP_VERSION}" ]]; then
    args+=( --app-version "${TERRA_APP_VERSION}" )
  fi

  if [[ -n "${TERRA_CHART_VERSION}" ]]; then
    args+=( --chart-version "${TERRA_CHART_VERSION}" )
  fi

  ./bin/render -e "${TERRA_ENV}" -a "${TERRA_APP}" "${args[@]}"
else
  echo "Usage: ${0} (init|generate)" >&2
  exit 1
fi
