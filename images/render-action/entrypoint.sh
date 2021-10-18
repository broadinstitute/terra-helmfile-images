#!/bin/sh

set -eo pipefail

# This variable is expected by the render tool
if [[ -z "${TERRA_HELMFILE_PATH}" ]]; then
  echo "${TERRA_HELMFILE_PATH} is required" >&2
  exit 1
fi

if [[ -z "${OUTPUT_DIR}" ]]; then
  echo "${OUTPUT_DIR} is required" >&2
  exit 1
fi

argocd=
if [[ "${ARGOCD_MODE}" == "true" ]]; then
  argocd="--argocd"
fi

env=
if [[ -n "${TERRA_ENV}" ]]; then
  env="-e ${TERRA_ENV}"
fi

numWorkers=
if [[ -n "${NUM_WORKERS}" ]]; then
  numWorkers="--workers ${NUM_WORKERS}"
fi

set -x
echo "Running render in $( pwd )"
/tools/bin/render $numWorkers --output-dir="${OUTPUT_DIR}" $env $argocd
