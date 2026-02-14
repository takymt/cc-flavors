#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: scripts/bump.sh <major|minor|patch>" >&2
  exit 1
fi

level="$1"
case "$level" in
major | minor | patch) ;;
*)
  echo "invalid level: $level (use major|minor|patch)" >&2
  exit 1
  ;;
esac

version=$(scripts/version.sh)
IFS='.' read -r major minor patch <<EOF
$version
EOF

major=${major:-0}
minor=${minor:-0}
patch=${patch:-0}

case "$level" in
major)
  major=$((major + 1))
  minor=0
  patch=0
  ;;
minor)
  minor=$((minor + 1))
  patch=0
  ;;
patch)
  patch=$((patch + 1))
  ;;
esac

next_tag="v${major}.${minor}.${patch}"

if git rev-parse -q --verify "refs/tags/$next_tag" >/dev/null; then
  echo "tag already exists: $next_tag" >&2
  exit 1
fi

git tag -a "$next_tag" -m "$next_tag"
git push origin "$next_tag"

echo "pushed $next_tag"
