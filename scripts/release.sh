#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

tag="${1:-}"
if [[ -z "$tag" ]]; then
  echo "usage: scripts/release.sh vX.Y.Z" >&2
  echo "env: RELEASE_BRANCH=<name> override the required branch (default: main)" >&2
  echo "     RELEASE_SKIP_REMOTE=1   skip the upstream-sync check" >&2
  exit 1
fi
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "tag must match vX.Y.Z: $tag" >&2
  exit 1
fi

allowed_branch="${RELEASE_BRANCH:-main}"
current_branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "$current_branch" != "$allowed_branch" ]]; then
  echo "must be on branch '$allowed_branch' (currently on '$current_branch'); override with RELEASE_BRANCH=<name>" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree is not clean; commit or stash all changes (including untracked files) before tagging:" >&2
  git status --short >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
  echo "tag already exists: $tag" >&2
  exit 1
fi

remote_name="origin"
if [[ "${RELEASE_SKIP_REMOTE:-0}" != "1" ]]; then
  upstream="$(git rev-parse --abbrev-ref '@{u}' 2>/dev/null)" || {
    echo "branch '$current_branch' has no upstream; set one with" >&2
    echo "  git branch --set-upstream-to=origin/$current_branch" >&2
    echo "or override the check with RELEASE_SKIP_REMOTE=1" >&2
    exit 1
  }
  remote_name="${upstream%%/*}"
  git fetch --quiet "$remote_name" "$current_branch" || {
    echo "git fetch '$remote_name' '$current_branch' failed; check network or override with RELEASE_SKIP_REMOTE=1" >&2
    exit 1
  }
  local_sha="$(git rev-parse HEAD)"
  upstream_sha="$(git rev-parse "$upstream")"
  if [[ "$local_sha" != "$upstream_sha" ]]; then
    if git merge-base --is-ancestor "$upstream_sha" "$local_sha"; then
      echo "local '$current_branch' is ahead of '$upstream'; push first (git push $remote_name $current_branch)" >&2
    elif git merge-base --is-ancestor "$local_sha" "$upstream_sha"; then
      echo "local '$current_branch' is behind '$upstream'; pull before tagging (git pull --ff-only)" >&2
    else
      echo "local '$current_branch' diverges from '$upstream'; rebase or reset before tagging" >&2
    fi
    exit 1
  fi
fi

"$ROOT_DIR/scripts/check-release.sh" --tag "$tag"
go test ./...

git tag -a "$tag" -m "synocli $tag"
echo "created annotated tag: $tag"
echo "next: git push $remote_name $current_branch $tag"
