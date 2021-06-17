#!/bin/sh

set -eo pipefail

# Render manifests

if [[ $# -lt 1 ]]; then
  echo "Error: render_all expects 1 argument, got $#" >&2
  return 1
fi

argomode=

srcdir="$1"
outdir="$2"

# Render ArgoCD manifests 
if [[ "$3" == "true" ]]; then
  argomode="--argocd"
fi
mkdir -p "$outdir"
render="${srcdir}/bin/render.sh"

$render --output-dir="${outdir}" $argomode
