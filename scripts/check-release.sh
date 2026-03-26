#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CHANGELOG_FILE="$ROOT_DIR/CHANGELOG.md"

required_agent_keys=(
  breaking_changes
  commands_added
  commands_changed
  flags_added
  flags_changed
  behavior_changes
  skill_update_action
)

usage() {
  cat <<'EOF'
Usage: scripts/check-release.sh [--tag vX.Y.Z]

Validates CHANGELOG.md structure and required Agent Notes schema.
When --tag is provided, validates that specific release section exists.
EOF
}

release_tag=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --tag)
      [[ $# -ge 2 ]] || { echo "missing value for --tag" >&2; exit 1; }
      release_tag="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

[[ -f "$CHANGELOG_FILE" ]] || { echo "missing $CHANGELOG_FILE" >&2; exit 1; }
grep -q '^## \[Unreleased\]$' "$CHANGELOG_FILE" || {
  echo "CHANGELOG.md must include '## [Unreleased]'" >&2
  exit 1
}

if [[ -n "$release_tag" && ! "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "tag must match vX.Y.Z: $release_tag" >&2
  exit 1
fi

extract_section() {
  local version="$1"
  awk -v ver="$version" '
    $0 ~ "^## \\[" ver "\\] - [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$" { in_section=1; next }
    /^## \[/ && in_section { exit }
    in_section { print }
  ' "$CHANGELOG_FILE"
}

validate_release_section() {
  local version="$1"
  local section
  section="$(extract_section "$version")"
  [[ -n "$section" ]] || {
    echo "missing release section for [$version]" >&2
    exit 1
  }

  grep -q '^### Agent Notes$' <<<"$section" || {
    echo "release [$version] is missing '### Agent Notes'" >&2
    exit 1
  }
  grep -q '^```yaml$' <<<"$section" || {
    echo "release [$version] is missing Agent Notes YAML block start" >&2
    exit 1
  }
  grep -q '^```$' <<<"$section" || {
    echo "release [$version] is missing Agent Notes YAML block end" >&2
    exit 1
  }

  local key
  for key in "${required_agent_keys[@]}"; do
    grep -Eq "^${key}:" <<<"$section" || {
      echo "release [$version] is missing Agent Notes key: $key" >&2
      exit 1
    }
  done
}

release_versions="$(
  grep -E '^## \[[0-9]+\.[0-9]+\.[0-9]+\] - [0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$' "$CHANGELOG_FILE" \
    | sed -E 's/^## \[([0-9]+\.[0-9]+\.[0-9]+)\] - .*/\1/'
)"

if [[ -z "$release_versions" ]]; then
  echo "CHANGELOG.md must include at least one release section (## [X.Y.Z] - YYYY-MM-DD)" >&2
  exit 1
fi

if [[ -n "$release_tag" ]]; then
  validate_release_section "${release_tag#v}"
else
  while IFS= read -r version; do
    [[ -n "$version" ]] || continue
    validate_release_section "$version"
  done <<<"$release_versions"
fi

echo "release metadata validation passed"
