#!/bin/sh

set -eo pipefail

GITHUB_COMMENT_MAX_CHARS=65535

if [[ $# -ne 2 ]]; then
  echo "Usage: $0 path/to/base/checkout path/to/head/checkout" >&2
  exit 1
fi

if [[ -z "$GITHUB_EVENT_PATH" ]]; then
  echo "Please supply a path to GitHub event.json file" >&2
  exit 2
fi

if [[ -z "$GITHUB_TOKEN" ]]; then
  echo "Please supply a GitHub token via GITHUB_TOKEN environment variable" >&2
  exit 2
fi

if [[ -z "$GITHUB_EVENT_NAME" ]]; then
  echo "Please supply a GitHub event name via GITHUB_EVENT_NAME environment variable" >&2
  exit 2
fi

if [[ -z "$GITHUB_REPOSITORY" ]]; then
  echo "Please supply a GitHub repository name via GITHUB_REPOSITORY environment variable" >&2
  exit 2
fi

if [[ -z "$GITHUB_RUN_ID" ]]; then
  echo "Please supply a GitHub run id via GITHUB_RUN_ID environment variable" >&2
  exit 2
fi

set -ux

# Used to provide a click-through URL on approval status
WORKFLOW_URL="https://github.com/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}"

# Directory containing checkout of this PR's base revision
basedir=$1

# Directory containing checkout of this PR's head revision
headdir=$2

mkdir -p output

# Generate diffs
env-differ --debug "${basedir}" "${headdir}" --output-dir=output

# Post Markdown diff summary as comment on pull request
# (only on pull request events, not pull_request_review events)
if [[ "${GITHUB_EVENT_NAME}" == "pull_request" ]]; then
  # If the markdown is too big, log a warning and move on.
  chars=$( wc -c output/diff.md | awk '{ print $1 }' )
  if [[ "$chars" -gt "${GITHUB_COMMENT_MAX_CHARS}" ]]; then
    echo "Warning: diff output too large to post (${chars} chars > ${GITHUB_COMMENT_MAX_CHARS} character limit)" >&2
  else
    pull-request post-comment output/diff.md
  fi
fi

# If the prod environment was updated, ensure the PR has at least 1 approval
approvals=0
if cat output/diff.json | jq -e 'has("prod")' >/dev/null; then
  echo "This PR includes changes to prod! At least one approval is required before merging." >&2
  approvals=1
fi

# Create a status on the head commit for this PR --
# pending if approvals missing, success if approvals are present
pull-request check-approved --at-least="${approvals}" --target-url="${WORKFLOW_URL}"
