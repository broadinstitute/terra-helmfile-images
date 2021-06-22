#!/bin/sh

set -eo pipefail

# Render manifests

if [[ $# -lt 1 ]]; then
  echo "Error: render_all expects 1 argument, got $#" >&2
  return 1
fi

argomode=
env=

srcdir="$1"
outdir="$2"

# Render ArgoCD manifests 
if [[ "$3" == "true" ]]; then
  argomode="--argocd"
fi
if [[ -n "$4" ]]; then
  env="-e ${4}"
fi
mkdir -p "$outdir"
render="${srcdir}/bin/render"

$render $env --output-dir="${outdir}" $argomode
