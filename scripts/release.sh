#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

tag="${1:-}"
if [[ -z "$tag" ]]; then
  echo "usage: scripts/release.sh vX.Y.Z" >&2
  exit 1
fi
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "tag must match vX.Y.Z: $tag" >&2
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "working tree is not clean; commit or stash changes before tagging" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
  echo "tag already exists: $tag" >&2
  exit 1
fi

"$ROOT_DIR/scripts/check-release.sh" --tag "$tag"
go test ./...

git tag -a "$tag" -m "synocli $tag"
echo "created annotated tag: $tag"
echo "next: git push origin main $tag"
