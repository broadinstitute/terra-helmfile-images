#!/bin/sh

set -e
set -o pipefail

#
# A script for retrieving the version of an app from terra-helmfile's versions.yaml file, via
# the GitHub API.
#
REPO="broadinstitute/terra-helmfile"
VERSIONS_FILE=${VERSIONS_FILE:-"versions/app/dev.yaml"}

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <project>" >&2
  exit 1
fi

APP="$1"

if [[ -z "${GITHUB_TOKEN}" ]]; then
  echo "Error: Please supply a valid GitHub token via the GITHUB_TOKEN environment variable" >&2
  exit 1
fi

set -x
curl --fail -L -sS \
  -H "Accept: application/vnd.github.v3+json" \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  "https://api.github.com/repos/${REPO}/contents/${VERSIONS_FILE}" |\
  jq -r '.content' |\
  base64 -d  |\
  yq --exit-status=1 e ".releases.${APP}.appVersion" -
