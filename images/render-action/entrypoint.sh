#!/bin/sh

set -eo pipefail

# This variable is expected by the render tool
if [[ -z "${THELMA_HOME}" ]]; then
  echo "env var ${THELMA_HOME} is required" >&2
  exit 1
fi

if [[ -z "${OUTPUT_DIR}" ]]; then
  echo "env var OUTPUT_DIR is required" >&2
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

parallelWorkers=
if [[ -n "${PARALLEL_WORKERS}" ]]; then
  parallelWorkers="--parallel-workers ${PARALLEL_WORKERS}"
fi

set -x
echo "Running render in $( pwd )"
/tools/bin/thelma render $parallelWorkers --output-dir="${OUTPUT_DIR}" $env $argocd
