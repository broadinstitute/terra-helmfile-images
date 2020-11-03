#!/bin/bash

# An ArgoCD application for rendering application configs from firecloud-develop.
set -eo pipefail

## Required arguments
: "${ENV?Env variable ENV is missing but required. Eg. \"dev\"}"
: "${APP_NAME?Env variable APP_NAME is missing but required. Eg. \"cromwell\"}"
: "${RUN_CONTEXT?Env variable RUN_CONTEXT is missing but required. Eg. \"live\"}"

## Optional arguments
# Comma-separated list of instance types
INSTANCE_TYPES=${INSTANCE_TYPES:-}
# Path to where firecloud-develop is checked out
CHECKOUT_DIR="${CHECKOUT_DIR:-$( pwd )}"

## Arg processing
CONFIGURE_RB="${CHECKOUT_DIR}/configure.rb"
WORKING_DIR="${CHECKOUT_DIR}/output.$$"

if [[ ! -f ${CONFIGURE_RB} ]]; then
  echo -n "Could not find configure.rb in checkout dir ${CHECKOUT_DIR}, " >&2
  echo "is it a valid clone of firecloud-develop?" >&2
  exit 1
fi

# Remove whitespace in INSTANCE_TYPES then replace comma with space
# so it can be iterated on
INSTANCE_TYPES=$(
  echo $INSTANCE_TYPES |\
  tr -d '[:space:]' |\
  tr ',' ' '
)
if [[ -z "${INSTANCE_TYPES}" ]]; then
  INSTANCE_TYPES=${INSTANCE_TYPES:-__none__}
fi

## Populate VAULT_TOKEN (required for configure.rb)
export VAULT_ADDR=${VAULT_ADDR:-https://clotho.broadinstitute.org:8200}

if [[ -z "${VAULT_TOKEN}" ]]; then
  if [[ -z "${VAULT_ROLE_ID}" ]] || [[ -z "${VAULT_SECRET_ID}" ]]; then
    echo "You must supply either VAULT_TOKEN or VAULT_ROLE_ID and VAULT_SECRET_ID" >&2
    exit 1
  fi

  export VAULT_TOKEN=$(
    vault write -field=token auth/approle/login \
      role_id="${VAULT_ROLE_ID}" \
      secret_id="${VAULT_SECRET_ID}"
  )
fi

# Set required environment variables for configure.rb
export ENV
export APP_NAME
export RUN_CONTEXT
export INPUT_DIR="${CHECKOUT_DIR}"
export GKE_DEPLOY=true
export USE_DOCKER_CONSUL_TEMPLATE=false
export HOST_TAG="${APP_NAME}-${ENV}" # required for tcell

for type in $INSTANCE_TYPES; do
  # Default - no instance type
  INSTANCE_TYPE=
  INSTANCE_NAME="gke-${ENV}-${APP_NAME}"
  OUTPUT_DIR="${WORKING_DIR}/${APP_NAME}"

  if [[ $type != "__none__" ]]; then
    # There is an instance type.
    # Set INSTANCE_TYPE, append to output dir and name
    INSTANCE_TYPE="${type}"
    INSTANCE_NAME="${INSTANCE_NAME}-${type}"
    OUTPUT_DIR="${OUTPUT_DIR}/${type}"
  fi

  # Now export these values so configure.rb can read them
  export INSTANCE_TYPE
  export INSTANCE_NAME
  export OUTPUT_DIR

  # Redirect output to stderr, k8s yamls must be written to stdout
  $CONFIGURE_RB >&2

  if [[ ! -f "${OUTPUT_DIR}/kustomization.yaml" ]]; then
    echo "Error: No kustomization.yaml found in output dir ${OUTPUT_DDIR}" >&2
  fi

  echo "Running kustomize build on ${OUTPUT_DIR}" >&2
  echo "---" # Have to add separator between different kustomize invocations
  kustomize build ${OUTPUT_DIR}
done
