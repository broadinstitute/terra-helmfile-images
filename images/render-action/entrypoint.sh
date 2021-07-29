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

set -x
/tools/bin/render --output-dir="${OUTPUT_DIR}" $env $argocd
