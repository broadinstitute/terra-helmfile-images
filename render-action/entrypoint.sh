#!/bin/sh

set -eo pipefail

# Render app manifests, and then ArgoCD manifests, and merge into
# a single directory with rsync so they can be easily compared with diff -r

if [[ $# -lt 1 ]]; then
  echo "Error: render_all expects 1 argument, got $#" >&2
  return 1
fi
local srcdir="$1"
local outdir="$2"

local render="${srcdir}/bin/render"

$render --output-dir="${outdir}"
