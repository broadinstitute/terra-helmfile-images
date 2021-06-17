#!/bin/bash

#
# Render ArgoCD manifests for all Terra apps/environments
# This is run by the terra-app-generator ArgoCD app
#

set -eo pipefail
set -x

if [[ "$1" == 'init' ]]; then
  : # Nothing to do
elif [[ "$1" == 'generate' ]]; then
  # Delegate to render script
  ./bin/render.sh --argocd
else
  echo "Usage: ${0} (init|generate)" >&2
  exit 1
fi
