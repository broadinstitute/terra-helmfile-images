#!/bin/bash

set -ex

HELM_REPO_NAME="${HELM_REPO_NAME:-terra-helm}"
HELM_REPO_URL="${HELM_REPO_URL:-https://broadinstitute.github.io/terra-helm}"
HELM_CHART_NAME="${HELM_CHART_NAME:-}"
HELM_CHART_VERSION="${HELM_CHART_VERSION:-}"
HELM_CHART_NAMESPACE="${HELM_CHART_NAMESPACE:-${ARGOCD_APP_NAMESPACE:-default}}"
HELM_CHART_VALUES_FILES="${HELM_CHART_VALUES_FILES:-}"
HELM_CHART_RELEASE="${HELM_CHART_RELEASE:-${ARGOCD_APP_NAME:-${HELM_CHART_NAME}}}"

if [[ "$1" == 'init' ]]; then
  helm repo add "${HELM_REPO_NAME}" "${HELM_REPO_URL}" 
elif [[ "$1" == 'generate' ]]; then
  if [[ -z "${HELM_CHART_NAME}" ]]; then
    echo "Please specify HELM_CHART_NAME" >&2
    exit 1
  fi

  if [[ -z "${HELM_CHART_RELEASE}" ]]; then
    echo "Please specify HELM_CHART_RELEASE" >&2
    exit 1
  fi

  version=
  if [[ ! -z "${HELM_CHART_VERSION}" ]]; then
     version="--version ${HELM_CHART_VERSION}"
  fi

  values=
  if [[ ! -z "${HELM_CHART_VALUES_FILES}" ]]; then
     values="--values ${HELM_CHART_VALUES_FILES}"
  fi

  namespace=
  if [[ ! -z "${HELM_CHART_NAMESPACE}" ]]; then
     namespace="--namespace ${HELM_CHART_NAMESPACE}"
  fi

  helm template \
    "${HELM_CHART_RELEASE}" \
    "${HELM_REPO_NAME}/${HELM_CHART_NAME}" \
    $namespace \
    $values $version
else
  echo "Usage: ${0} (init|generate)" >&2
  exit 1
fi

