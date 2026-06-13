#!/bin/sh
set -eu

version="${1:?version is required}"
image="${2:?image is required}"
out=".tmp/goreleaser/kured-${version}-combined.yaml"

mkdir -p .tmp/goreleaser
cat kured-rbac.yaml > "${out}"
sed "s#image: ghcr.io/.*kured.*#image: ${image}:${version}#g" kured-ds.yaml >> "${out}"
