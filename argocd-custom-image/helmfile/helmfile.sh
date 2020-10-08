#!/bin/bash

set -eo pipefail
set -x

HELMFILE_ENV="${HELMFILE_ENV:-}"
HELMFILE_NAMESPACE="${HELMFILE_NAMESPACE:-}"
HELMFILE_SELECTOR="${HELMFILE_SELECTOR:-}"

if [[ "$1" == 'init' ]]; then
  # Set --allow-no-matching-release for repos initialization
  # (in case the default environment has no releases)
  helmfile --allow-no-matching-release repos
elif [[ "$1" == 'generate' ]]; then
  env=
  if [[ ! -z "${HELMFILE_ENV}" ]]; then
    env="--environment ${HELMFILE_ENV}"
  fi

  namespace=
  if [[ ! -z "${HELMFILE_NAMESPACE}" ]]; then
    namespace="--namespace ${HELMFILE_NAMESPACE}"
  fi

  selector=
  if [[ ! -z "${HELMFILE_SELECTOR}" ]]; then
    selector="--selector ${HELMFILE_SELECTOR}"
  fi

  helmfile $env $selector template --skip-deps
else
  echo "Usage: ${0} (init|generate)" >&2
  exit 1
fi
