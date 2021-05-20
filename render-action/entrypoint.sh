#!/bin/sh

set -eo pipefail

# Render app manifests, and then ArgoCD manifests, and merge into
# a single directory with rsync so they can be easily compared with diff -r

if [[ $# -lt 1 ]]; then
  echo "Error: render_all expects 1 argument, got $#" >&2
  return 1
fi


srcdir="$1"
outdir="$2"

mkdir -p "$outdir"
render="${srcdir}/bin/render"


$render --output-dir="${outdir}"
