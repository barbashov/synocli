#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  tests_e2e/downloadstation.sh --endpoint <url> --credentials-file <path> [options]

Required:
  --endpoint <url>             DSM endpoint, e.g. https://nas:5001
  --credentials-file <path>    Credentials file (user=..., password=...)

Options:
  --bin <path>                 synocli binary path (default: ./bin/synocli)
  --destination <path>         DS destination folder (default: DSM default destination)
  --insecure-tls               Pass --insecure-tls to all commands (default)
  --no-insecure-tls            Do not pass --insecure-tls
  -h, --help                   Show this help
USAGE
}

BIN="./bin/synocli"
ENDPOINT=""
CREDS=""
DESTINATION=""
INSECURE_TLS=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin)
      BIN="$2"
      shift 2
      ;;
    --endpoint)
      ENDPOINT="$2"
      shift 2
      ;;
    --credentials-file)
      CREDS="$2"
      shift 2
      ;;
    --destination)
      DESTINATION="$2"
      shift 2
      ;;
    --insecure-tls)
      INSECURE_TLS=1
      shift
      ;;
    --no-insecure-tls)
      INSECURE_TLS=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "$ENDPOINT" || -z "$CREDS" ]]; then
  echo "--endpoint and --credentials-file are required." >&2
  usage >&2
  exit 1
fi

if [[ ! -x "$BIN" ]]; then
  echo "synocli binary is not executable: $BIN" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for JSON assertions and fixture parsing." >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for built-in fixture fetching." >&2
  exit 1
fi

COMMON_ARGS=(--endpoint "$ENDPOINT" --credentials-file "$CREDS")
if [[ "$INSECURE_TLS" == "1" ]]; then
  COMMON_ARGS+=(--insecure-tls)
fi

CREATED_IDS=()
STATE_FILE="./tmp/ds-fixtures/.downloadstation-created-task-ids"

run() {
  local args=("$@")
  local has_debug=0
  local arg
  for arg in "${args[@]}"; do
    if [[ "$arg" == "--debug" ]]; then
      has_debug=1
      break
    fi
  done

  echo
  echo ">>> ${args[*]}"
  if "${args[@]}"; then
    return 0
  fi

  if [[ "$has_debug" == "1" ]]; then
    echo "Command failed (already in debug mode)." >&2
    return 1
  fi

  echo "Command failed. Retrying with --debug:" >&2
  echo ">>> ${args[*]} --debug" >&2
  "${args[@]}" --debug
}

run_json_capture() {
  local args=("$@")
  local out
  local rc
  local has_debug=0
  local arg

  echo >&2
  echo ">>> ${args[*]}" >&2
  for arg in "${args[@]}"; do
    if [[ "$arg" == "--debug" ]]; then
      has_debug=1
      break
    fi
  done

  set +e
  out="$("${args[@]}")"
  rc=$?
  set -e
  if [[ $rc -eq 0 ]]; then
    printf '%s\n' "$out"
    return 0
  fi

  if [[ "$has_debug" == "1" ]]; then
    printf '%s\n' "$out"
    return "$rc"
  fi

  echo "Command failed. Retrying with --debug:" >&2
  echo ">>> ${args[*]} --debug" >&2
  set +e
  out="$("${args[@]}" --debug)"
  rc=$?
  set -e
  printf '%s\n' "$out"
  return "$rc"
}

run_json_capture_expect_fail() {
  local args=("$@")
  local out
  local rc

  echo >&2
  echo ">>> ${args[*]}" >&2

  set +e
  out="$("${args[@]}")"
  rc=$?
  set -e
  printf '%s\n' "$out"
  return "$rc"
}

json_assert_envelope() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
if not isinstance(obj, dict):
    raise SystemExit("expected JSON object envelope")
for key in ("ok", "command", "meta"):
    if key not in obj:
        raise SystemExit(f"missing envelope field: {key}")
if obj.get("ok") is not True:
    raise SystemExit(f"expected ok=true, got {obj.get('\"'\"'ok'\"'\"')}")'
}

json_extract_first_task_id() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
data=obj.get("data") or {}
ids=data.get("task_ids") or []
if not ids or not ids[0]:
    raise SystemExit("data.task_ids[0] not found")
print(ids[0])'
}

json_assert_tasks_contain_id() {
  local task_id="$1"
  python3 -c 'import json,sys
wanted=sys.argv[1]
obj=json.load(sys.stdin)
data=obj.get("data") or {}
tasks=data.get("tasks") or []
ids={t.get("task_id") for t in tasks if isinstance(t, dict)}
if wanted not in ids:
    raise SystemExit(f"task {wanted} not found in ds list output")' "$task_id"
}

json_assert_wait_failure() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
if obj.get("ok") is not False:
    raise SystemExit("expected ok=false for wait failure")
err=obj.get("error") or {}
code=err.get("code")
if code not in ("timeout", "task_failed"):
    raise SystemExit(f"unexpected wait error code: {code}")'
}

json_assert_synology_code_405() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
if obj.get("ok") is not False:
    raise SystemExit("expected ok=false")
err=obj.get("error") or {}
if err.get("code") != "synology_error":
    raise SystemExit("unexpected error code: %r" % (err.get("code"),))
details=err.get("details") or {}
if details.get("synology_code") != 405:
    raise SystemExit("expected synology_code=405, got %r" % (details.get("synology_code"),))'
}

json_assert_deleted_error() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
if obj.get("ok") is not False:
    raise SystemExit("expected ok=false for deleted task get")
err=obj.get("error") or {}
if err.get("code") != "synology_error":
    raise SystemExit(f"unexpected error.code: {err.get('"'"'code'"'"')}")
details=err.get("details") or {}
code=details.get("synology_code")
if code not in (401, 402, 404):
    raise SystemExit(f"unexpected synology_code: {code}")'
}

json_assert_watch_snapshot() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
if obj.get("ok") is not True:
    raise SystemExit("expected ok=true")
data=obj.get("data") or {}
if data.get("event") != "snapshot":
    raise SystemExit("expected data.event=snapshot")
if not isinstance(data.get("tasks"), list):
    raise SystemExit("expected data.tasks list")'
}

register_task_id() {
  local task_id="$1"
  CREATED_IDS+=("$task_id")
  mkdir -p "$(dirname "$STATE_FILE")"
  printf '%s\n' "$task_id" >> "$STATE_FILE"
}

json_extract_inferred_download_path() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
data=obj.get("data") or {}
dest=(data.get("destination") or "").strip()
title=(data.get("title") or "").strip()
if not title:
    raise SystemExit("missing task title for inferred cleanup path")
if "/" in title or "\\" in title:
    raise SystemExit(f"unsafe task title for inferred cleanup path: {title!r}")
dest=dest.strip("/")
if dest:
    print(f"/{dest}/{title}")
else:
    print(f"/{title}")'
}

resolve_task_data_path_best_effort() {
  local tid="$1"
  local out
  out="$(run_json_capture "$BIN" ds get "$tid" --json "${COMMON_ARGS[@]}")" || return 1
  printf '%s\n' "$out" | json_extract_inferred_download_path
}

cleanup_data_path_best_effort() {
  local path="$1"
  [[ -n "$path" ]] || return 0
  "$BIN" fs delete "$path" --recursive --json "${COMMON_ARGS[@]}" >/dev/null 2>&1 || true
}

cleanup_task_id_best_effort() {
  local tid="$1"
  local inferred_path=""
  [[ -n "$tid" ]] || return
  inferred_path="$(resolve_task_data_path_best_effort "$tid" 2>/dev/null || true)"
  "$BIN" ds delete "$tid" --json "${COMMON_ARGS[@]}" >/dev/null 2>&1 || true
  cleanup_data_path_best_effort "$inferred_path"
}

cleanup_stale_state_tasks() {
  if [[ ! -s "$STATE_FILE" ]]; then
    return
  fi
  echo "Cleaning stale task IDs from previous interrupted run..."
  while IFS= read -r tid; do
    cleanup_task_id_best_effort "$tid"
  done < "$STATE_FILE"
  : > "$STATE_FILE"
}

cleanup_tasks() {
  set +e
  if [[ -s "$STATE_FILE" ]]; then
    while IFS= read -r tid; do
      cleanup_task_id_best_effort "$tid"
    done < "$STATE_FILE"
  fi
  local tid
  for tid in "${CREATED_IDS[@]}"; do
    cleanup_task_id_best_effort "$tid"
  done
  rm -f "$STATE_FILE"
  set -e
}

on_signal() {
  echo "Interrupted, running cleanup..." >&2
  exit 130
}

trap on_signal INT TERM
trap cleanup_tasks EXIT

prepare_fixtures() {
  mkdir -p ./tmp/ds-fixtures
  local page=./tmp/ds-fixtures/webtorrent-free-torrents.html
  local fixture=./tmp/ds-fixtures/free.torrent

  echo "Fetching WebTorrent fixture index..."
  curl -fsSL https://webtorrent.io/free-torrents > "$page"

  fixture_info="$(python3 - "$page" <<'PY'
import html
import re
import sys

page_path = sys.argv[1]
text = open(page_path, "r", encoding="utf-8").read()

name = "Big Buck Bunny"
pattern = re.compile(
    r"<li><p>" + re.escape(name) +
    r" <a href=\"([^\"]+\.torrent)\">\(torrent file\)</a> " +
    r"<a href=\"(magnet:[^\"]+)\">\(magnet link\)</a>",
    re.IGNORECASE,
)
m = pattern.search(text)
if not m:
    raise SystemExit("failed to parse Big Buck Bunny fixture links from webtorrent free-torrents page")

torrent_url = html.unescape(m.group(1))
magnet_uri = html.unescape(m.group(2))
print(torrent_url)
print(magnet_uri)
PY
)"

  TORRENT_URL="$(printf '%s\n' "$fixture_info" | sed -n '1p')"
  MAGNET_FIXTURE="$(printf '%s\n' "$fixture_info" | sed -n '2p')"
  if [[ -z "$TORRENT_URL" || -z "$MAGNET_FIXTURE" ]]; then
    echo "Failed to parse torrent/magnet fixture details." >&2
    exit 1
  fi
  HTTPS_FIXTURE="https://raw.githubusercontent.com/barbashov/synocli/main/README.md"
  TORRENT_FILE="$fixture"

  echo "Downloading torrent fixture: $TORRENT_URL"
  curl -fsSL "$TORRENT_URL" -o "$TORRENT_FILE"

  export TORRENT_FILE
  export MAGNET_FIXTURE
  export HTTPS_FIXTURE
}

capture_watch_snapshot_line() {
  local args=("$@")
  python3 - "${args[@]}" <<'PY'
import subprocess
import sys
import time

cmd = sys.argv[1:]
p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
line = ""
try:
    deadline = time.time() + 20
    while time.time() < deadline:
        line = p.stdout.readline()
        if line:
            print(line.rstrip("\n"))
            break
    if not line:
        raise SystemExit("ds watch did not produce a JSON snapshot line in time")
finally:
    if p.poll() is None:
        p.terminate()
        try:
            p.wait(timeout=3)
        except subprocess.TimeoutExpired:
            p.kill()
            p.wait(timeout=3)

if not line:
    err = p.stderr.read().strip()
    if err:
        sys.stderr.write(err + "\n")
    sys.exit(1)
PY
}

run_ds_add_json() {
  local input="$1"
  if [[ -n "$DESTINATION" ]]; then
    run_json_capture "$BIN" ds add "$input" --destination "$DESTINATION" --json "${COMMON_ARGS[@]}"
    return
  fi
  run_json_capture "$BIN" ds add "$input" --json "${COMMON_ARGS[@]}"
}

assert_task_deleted() {
  local task_id="$1"
  local out
  local i
  for i in 1 2 3 4 5 6 7 8 9 10; do
    if out="$(run_json_capture_expect_fail "$BIN" ds get "$task_id" --json "${COMMON_ARGS[@]}")"; then
      sleep 1
      continue
    fi
    if printf '%s\n' "$out" | json_assert_deleted_error; then
      return 0
    fi
    echo "Deleted-task check got unexpected error payload for $task_id:" >&2
    printf '%s\n' "$out" >&2
    return 1
  done
  echo "Task $task_id still exists after delete polling window." >&2
  return 1
}

delete_task_and_cleanup_strict() {
  local task_id="$1"
  local inferred_path=""
  inferred_path="$(resolve_task_data_path_best_effort "$task_id" 2>/dev/null || true)"
  run_json_capture "$BIN" ds delete "$task_id" --json "${COMMON_ARGS[@]}" | json_assert_envelope
  cleanup_data_path_best_effort "$inferred_path"
}

json_extract_normalized_status() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
data=obj.get("data") or {}
st=data.get("normalized_status")
if not st:
    raise SystemExit("normalized_status not found")
print(st)'
}

is_pause_candidate_status() {
  local st="$1"
  case "$st" in
    waiting|downloading|seeding|finishing)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

main() {
  mkdir -p "$(dirname "$STATE_FILE")"
  cleanup_stale_state_tasks
  prepare_fixtures

  run "$BIN" auth ping "${COMMON_ARGS[@]}"
  run "$BIN" auth api-info --prefix SYNO.DownloadStation "${COMMON_ARGS[@]}"

  run "$BIN" downloadstation list "${COMMON_ARGS[@]}"
  run_json_capture "$BIN" ds list --json "${COMMON_ARGS[@]}" | json_assert_envelope

  local url_json
  local magnet_json
  local torrent_json
  local url_task_id
  local magnet_task_id
  local torrent_task_id

  url_json="$(run_ds_add_json "$HTTPS_FIXTURE")"
  printf '%s\n' "$url_json" | json_assert_envelope
  url_task_id="$(printf '%s\n' "$url_json" | json_extract_first_task_id)"
  register_task_id "$url_task_id"

  magnet_json="$(run_ds_add_json "$MAGNET_FIXTURE")"
  printf '%s\n' "$magnet_json" | json_assert_envelope
  magnet_task_id="$(printf '%s\n' "$magnet_json" | json_extract_first_task_id)"
  register_task_id "$magnet_task_id"

  torrent_json="$(run_ds_add_json "$TORRENT_FILE")"
  printf '%s\n' "$torrent_json" | json_assert_envelope
  torrent_task_id="$(printf '%s\n' "$torrent_json" | json_extract_first_task_id)"
  register_task_id "$torrent_task_id"

  run_json_capture "$BIN" ds get "$url_task_id" --json "${COMMON_ARGS[@]}" | json_assert_envelope
  run_json_capture "$BIN" ds get "$magnet_task_id" --json "${COMMON_ARGS[@]}" | json_assert_envelope
  run_json_capture "$BIN" ds get "$torrent_task_id" --json "${COMMON_ARGS[@]}" | json_assert_envelope

  local list_json
  list_json="$(run_json_capture "$BIN" ds list --json "${COMMON_ARGS[@]}")"
  printf '%s\n' "$list_json" | json_assert_envelope
  printf '%s\n' "$list_json" | json_assert_tasks_contain_id "$url_task_id"
  printf '%s\n' "$list_json" | json_assert_tasks_contain_id "$magnet_task_id"
  printf '%s\n' "$list_json" | json_assert_tasks_contain_id "$torrent_task_id"

  local pause_json
  local paused_task_id=""
  local candidates=("$magnet_task_id" "$torrent_task_id" "$url_task_id")
  local candidate_id
  local get_json
  local candidate_status
  local attempt
  for attempt in 1 2 3; do
    for candidate_id in "${candidates[@]}"; do
      if ! get_json="$(run_json_capture "$BIN" ds get "$candidate_id" --json "${COMMON_ARGS[@]}")"; then
        continue
      fi
      candidate_status="$(printf '%s\n' "$get_json" | json_extract_normalized_status || true)"
      if ! is_pause_candidate_status "$candidate_status"; then
        continue
      fi
      if pause_json="$(run_json_capture "$BIN" ds pause "$candidate_id" --json "${COMMON_ARGS[@]}")"; then
        printf '%s\n' "$pause_json" | json_assert_envelope
        paused_task_id="$candidate_id"
        break 2
      fi
      if ! printf '%s\n' "$pause_json" | json_assert_synology_code_405; then
        echo "Pause failed with unexpected error for task $candidate_id." >&2
        printf '%s\n' "$pause_json" >&2
        exit 1
      fi
    done
    sleep 2
  done
  if [[ -z "$paused_task_id" ]]; then
    echo "Could not pause any created task after retries. Failing hard by policy." >&2
    exit 1
  fi

  local wait_json
  if wait_json="$(run_json_capture_expect_fail "$BIN" ds wait "$paused_task_id" --interval 1s --max-wait 3s --json "${COMMON_ARGS[@]}")"; then
    echo "Expected ds wait to fail with timeout/task_failed, but it succeeded." >&2
    exit 1
  fi
  printf '%s\n' "$wait_json" | json_assert_wait_failure

  local resume_json
  if resume_json="$(run_json_capture "$BIN" ds resume "$paused_task_id" --json "${COMMON_ARGS[@]}")"; then
    printf '%s\n' "$resume_json" | json_assert_envelope
  else
    echo "Expected resume to succeed for paused task $paused_task_id, but it failed." >&2
    printf '%s\n' "$resume_json" >&2
    exit 1
  fi

  local watch_json_line
  watch_json_line="$(capture_watch_snapshot_line "$BIN" ds list --watch --json --interval 1s --id "$paused_task_id" "${COMMON_ARGS[@]}")"
  printf '%s\n' "$watch_json_line" | json_assert_watch_snapshot

  delete_task_and_cleanup_strict "$paused_task_id"
  if [[ "$url_task_id" != "$paused_task_id" ]]; then
    delete_task_and_cleanup_strict "$url_task_id"
  fi
  if [[ "$torrent_task_id" != "$paused_task_id" ]]; then
    delete_task_and_cleanup_strict "$torrent_task_id"
  fi

  assert_task_deleted "$magnet_task_id"
  assert_task_deleted "$url_task_id"
  assert_task_deleted "$torrent_task_id"

  echo
  echo "Downloadstation manual script finished."
}

main "$@"
