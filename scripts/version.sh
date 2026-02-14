#!/bin/sh
set -eu

tag=$(git tag -l 'v[0-9]*' --sort=-v:refname | head -n 1)
if [ -z "$tag" ]; then
  echo "0.0.0"
else
  echo "${tag#v}"
fi
