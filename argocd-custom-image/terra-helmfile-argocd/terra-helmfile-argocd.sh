#!/bin/bash

# Wrapper for generating ArgoCD apps for a Terra environments

set -eo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 (init|generate)" >&2
  exit 1
fi

# Only render argocd resources
export HELMFILE_SELECTOR="group=argocd"

# Run the helmfile plugin multiple times, once per env
for envfile in environments/*/*.yaml; do
  env=$( basename $envfile .yaml )
  export HELMFILE_ENV="${env}"
  helmfile.sh "$@"
done
