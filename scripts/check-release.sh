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

Always validates:
  - '## [Unreleased]' is present and has a well-formed Agent Notes block
  - release sections are unique and listed newest-first
  - the latest release section has a well-formed Agent Notes block

With --tag vX.Y.Z additionally validates that specific release section.
Historical release sections are NOT re-validated, so adding required Agent
Notes keys in the future does not retroactively break old releases.
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

if [[ -n "$release_tag" && ! "$release_tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "tag must match vX.Y.Z: $release_tag" >&2
  exit 1
fi

grep -q '^## \[Unreleased\]$' "$CHANGELOG_FILE" || {
  echo "CHANGELOG.md must include '## [Unreleased]'" >&2
  exit 1
}

# extract_section <heading> — prints the lines of a release section
# (between the heading and the next '## [' heading), with the heading itself
# stripped. <heading> is either "Unreleased" or "X.Y.Z". The version match is
# literal (no regex metacharacters), so e.g. "0.1.0" does not accidentally
# match "0X1Y0".
extract_section() {
  local heading="$1"
  awk -v h="$heading" '
    BEGIN {
      version_prefix = "## [" h "] - "
      version_prefix_len = length(version_prefix)
    }
    !in_section {
      if (h == "Unreleased") {
        if ($0 == "## [Unreleased]") in_section = 1
      } else if (substr($0, 1, version_prefix_len) == version_prefix) {
        tail = substr($0, version_prefix_len + 1)
        if (tail ~ /^[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]$/) in_section = 1
      }
      next
    }
    /^## \[/ { exit }
    { print }
  ' "$CHANGELOG_FILE"
}

# extract_agent_notes_yaml — reads a release section on stdin and prints the
# YAML body of its '### Agent Notes' block (between the ```yaml and ``` fences).
# Exit codes:
#   0 — success, YAML body printed
#   1 — '### Agent Notes' heading not found
#   2 — heading found but no opening ```yaml fence before the next subsection
#   3 — opening fence found but never closed
extract_agent_notes_yaml() {
  awk '
    BEGIN { state = 0 }
    state == 0 && /^### Agent Notes$/ { state = 1; next }
    state == 1 && /^```yaml$/ { state = 2; next }
    state == 1 && /^## \[/ { state = 91; exit }
    state == 1 && /^### / { state = 92; exit }
    state == 2 && /^```$/ { state = 3; exit }
    state == 2 { print }
    END {
      if (state == 3) exit 0
      if (state == 0) exit 1
      if (state == 1 || state == 91 || state == 92) exit 2
      if (state == 2) exit 3
      exit 9
    }
  '
}

validate_section() {
  local heading="$1" section yaml rc=0
  section="$(extract_section "$heading")"
  [[ -n "$section" ]] || {
    echo "missing section: ## [$heading]" >&2
    exit 1
  }

  grep -q '^### Agent Notes$' <<<"$section" || {
    echo "section [$heading] is missing '### Agent Notes'" >&2
    exit 1
  }

  yaml="$(extract_agent_notes_yaml <<<"$section")" || rc=$?
  case "$rc" in
    0) ;;
    2)
      echo "section [$heading] Agent Notes is missing the opening '\`\`\`yaml' fence" >&2
      exit 1
      ;;
    3)
      echo "section [$heading] Agent Notes YAML block is not closed by '\`\`\`'" >&2
      exit 1
      ;;
    *)
      echo "section [$heading] Agent Notes YAML extraction failed (state=$rc)" >&2
      exit 1
      ;;
  esac

  local key
  for key in "${required_agent_keys[@]}"; do
    grep -Eq "^${key}:" <<<"$yaml" || {
      echo "section [$heading] Agent Notes is missing key: $key" >&2
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

duplicates="$(printf '%s\n' "$release_versions" | sort | uniq -d)"
if [[ -n "$duplicates" ]]; then
  echo "CHANGELOG.md has duplicate release version sections:" >&2
  sed 's/^/  /' <<<"$duplicates" >&2
  exit 1
fi

sorted_desc="$(printf '%s\n' "$release_versions" | sort -Vr)"
if [[ "$release_versions" != "$sorted_desc" ]]; then
  echo "CHANGELOG.md release sections must appear in descending version order (newest first)" >&2
  echo "expected order:" >&2
  sed 's/^/  /' <<<"$sorted_desc" >&2
  echo "actual order:" >&2
  sed 's/^/  /' <<<"$release_versions" >&2
  exit 1
fi

validate_section "Unreleased"

if [[ -n "$release_tag" ]]; then
  validate_section "${release_tag#v}"
else
  latest="$(printf '%s\n' "$release_versions" | head -n1)"
  validate_section "$latest"
fi

echo "release metadata validation passed"
