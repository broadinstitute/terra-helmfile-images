#!/bin/bash

#
# Render ArgoCD manifests for all Terra apps/environments
# This is run by the terra-app-generator ArgoCD app
#

set -eo pipefail
set -x

THELMA_RENDER_PARALLEL_WORKERS=${THELMA_RENDER_PARALLEL_WORKERS:-32}

if [[ "$1" == 'init' ]]; then
  : # Nothing to do
elif [[ "$1" == 'generate' ]]; then
  # Delegate to `thelma render`
  THELMA_HOME=$( pwd )
  export THELMA_HOME
  thelma render --stdout --argocd --parallel-workers="${THELMA_RENDER_PARALLEL_WORKERS}"
else
  echo "Usage: ${0} (init|generate)" >&2
  exit 1
fi
