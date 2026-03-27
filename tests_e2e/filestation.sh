#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  tests_e2e/filestation.sh --endpoint <url> --credentials-file <path> [options]

Required:
  --endpoint <url>             DSM endpoint, e.g. https://nas:5001
  --credentials-file <path>    Credentials file (user=..., password=...)

Options:
  --bin <path>                 synocli binary path (default: ./bin/synocli)
  --base <path>                Remote test root (default: /volume1/synocli-e2e)
  --insecure-tls               Pass --insecure-tls to all commands (default)
  --no-insecure-tls            Do not pass --insecure-tls
  --reset-remote               Delete and recreate base directory before tests
  -h, --help                   Show this help
USAGE
}

BIN="./bin/synocli"
ENDPOINT=""
CREDS=""
BASE="/volume1/synocli-e2e"
INSECURE_TLS=1
RESET_REMOTE=0

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
    --base)
      BASE="$2"
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
    --reset-remote)
      RESET_REMOTE=1
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

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for JSON assertions in this script." >&2
  exit 1
fi

BASE_PARENT="$(dirname "$BASE")"
BASE_NAME="$(basename "$BASE")"

COMMON_ARGS=(--endpoint "$ENDPOINT" --credentials-file "$CREDS")
if [[ "$INSECURE_TLS" == "1" ]]; then
  COMMON_ARGS+=(--insecure-tls)
fi

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

json_assert_envelope() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
if not isinstance(obj, dict):
    raise SystemExit("expected JSON object envelope")
for key in ("ok", "command", "meta"):
    if key not in obj:
        raise SystemExit(f"missing envelope field: {key}")'
}

json_extract_task_id() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
data=obj.get("data", {})
task_id=data.get("task_id") or data.get("taskid")
if not task_id:
    raise SystemExit("task_id not found in JSON output")
print(task_id)'
}

json_assert_watch_snapshot() {
  local expected_mode="$1"
  python3 -c 'import json,sys
expected=sys.argv[1]
obj=json.load(sys.stdin)
if obj.get("ok") is not True:
    raise SystemExit("expected ok=true")
data=obj.get("data") or {}
if data.get("event") != "snapshot":
    raise SystemExit("expected data.event=snapshot")
if data.get("mode") != expected:
    raise SystemExit(f"expected data.mode={expected}")' "$expected_mode"
}

json_assert_finished_true() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
data=obj.get("data") or {}
if data.get("finished") is not True:
    raise SystemExit("expected finished=true")'
}

json_get_synology_code() {
  python3 -c 'import json,sys
obj=json.load(sys.stdin)
err=obj.get("error") or {}
details=err.get("details") or {}
code=details.get("synology_code")
if code is None:
    raise SystemExit("missing error.details.synology_code")
print(code)'
}

run_stop_with_race_fallback() {
  local parent_cmd="$1"
  local task_id="$2"
  local stop_json
  local status_json
  local synology_code

  if stop_json="$(run_json_capture "$BIN" fs "$parent_cmd" stop "$task_id" --json "${COMMON_ARGS[@]}")"; then
    printf '%s\n' "$stop_json" | json_assert_envelope
    return 0
  fi

  echo "Stop command failed, checking status..." >&2
  if status_json="$(run_json_capture "$BIN" fs "$parent_cmd" status "$task_id" --json "${COMMON_ARGS[@]}")"; then
    printf '%s\n' "$status_json" | json_assert_envelope
    if printf '%s\n' "$status_json" | json_assert_finished_true; then
      return 0
    fi
  fi

  if [[ -z "${status_json:-}" ]]; then
    echo "Status command returned no JSON output after stop failure." >&2
    return 1
  fi

  printf '%s\n' "$status_json" | json_assert_envelope
  if synology_code="$(printf '%s\n' "$status_json" | json_get_synology_code 2>/dev/null)"; then
    case "$synology_code" in
      401|599)
        echo "Status unavailable after stop due to terminal task state (synology_code=$synology_code)." >&2
        return 0
        ;;
    esac
  fi

  echo "Stop fallback failed with non-race status error." >&2
  printf '%s\n' "$status_json" >&2
  return 1
}

prepare_fixtures() {
  echo "Preparing local fixtures..."
  mkdir -p ./tmp/fs-fixtures/tree/sub
  printf 'hello synocli\n' > ./tmp/fs-fixtures/a.txt
  printf 'nested file\n' > ./tmp/fs-fixtures/tree/sub/file.txt
  dd if=/dev/zero of=./tmp/fs-fixtures/large.bin bs=1m count=50 status=none
  (cd ./tmp/fs-fixtures && zip -r ./archive.zip ./tree ./a.txt >/dev/null)
}

reset_remote() {
  if [[ "$RESET_REMOTE" != "1" ]]; then
    return
  fi
  run "$BIN" fs delete "$BASE" --recursive --json "${COMMON_ARGS[@]}" || true
  run "$BIN" fs mkdir "$BASE_PARENT" "$BASE_NAME" --parents "${COMMON_ARGS[@]}"
}

cleanup_remote_artifacts() {
  run "$BIN" fs delete "$BASE/a.txt" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/a-renamed.txt" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/archive.zip" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/archive-async.zip" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/copy" --recursive "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/moved" --recursive "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/extracted" --recursive "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/extracted-async" --recursive "${COMMON_ARGS[@]}" || true
}

main() {
  prepare_fixtures
  reset_remote
  cleanup_remote_artifacts

  run "$BIN" auth ping "${COMMON_ARGS[@]}"
  run "$BIN" auth api-info --prefix SYNO.FileStation "${COMMON_ARGS[@]}"

  run "$BIN" fs shares "${COMMON_ARGS[@]}"

  run "$BIN" fs mkdir "$BASE_PARENT" "$BASE_NAME" --parents "${COMMON_ARGS[@]}"
  run "$BIN" fs ls "$BASE_PARENT" "${COMMON_ARGS[@]}"

  run "$BIN" fs upload ./tmp/fs-fixtures/a.txt "$BASE" "${COMMON_ARGS[@]}"
  run "$BIN" fs rename "$BASE/a.txt" a-renamed.txt "${COMMON_ARGS[@]}"
  run "$BIN" fs get "$BASE/a-renamed.txt" "${COMMON_ARGS[@]}"

  run "$BIN" fs download "$BASE/a-renamed.txt" --output ./tmp/fs-fixtures/a-downloaded.txt "${COMMON_ARGS[@]}"
  cmp ./tmp/fs-fixtures/a.txt ./tmp/fs-fixtures/a-downloaded.txt

  run "$BIN" fs cp "$BASE/a-renamed.txt" --to "$BASE/copy" "${COMMON_ARGS[@]}"
  run "$BIN" fs mv "$BASE/copy/a-renamed.txt" --to "$BASE/moved" "${COMMON_ARGS[@]}"

  search_task_id="$(run_json_capture "$BIN" fs search "$BASE" --pattern renamed --async --json "${COMMON_ARGS[@]}" | json_extract_task_id)"
  run "$BIN" fs search results "$search_task_id" "${COMMON_ARGS[@]}"
  run "$BIN" fs search stop "$search_task_id" "${COMMON_ARGS[@]}" || run "$BIN" fs search results "$search_task_id" "${COMMON_ARGS[@]}"
  run "$BIN" fs search clear "$search_task_id" "${COMMON_ARGS[@]}"

  dir_size_task_id="$(run_json_capture "$BIN" fs dir-size "$BASE" --async --json "${COMMON_ARGS[@]}" | json_extract_task_id)"
  run "$BIN" fs dir-size status "$dir_size_task_id" "${COMMON_ARGS[@]}"
  run_stop_with_race_fallback "dir-size" "$dir_size_task_id"

  md5_task_id="$(run_json_capture "$BIN" fs md5 "$BASE/moved/a-renamed.txt" --async --json "${COMMON_ARGS[@]}" | json_extract_task_id)"
  run "$BIN" fs md5 status "$md5_task_id" "${COMMON_ARGS[@]}"
  run_stop_with_race_fallback "md5" "$md5_task_id"
  run "$BIN" fs md5 "$BASE/moved/a-renamed.txt" "${COMMON_ARGS[@]}"

  compress_task_id="$(run_json_capture "$BIN" fs compress "$BASE" --to "$BASE/archive-async.zip" --async --json "${COMMON_ARGS[@]}" | json_extract_task_id)"
  run "$BIN" fs compress status "$compress_task_id" "${COMMON_ARGS[@]}"
  run_stop_with_race_fallback "compress" "$compress_task_id"

  run "$BIN" fs compress "$BASE" --to "$BASE/archive.zip" "${COMMON_ARGS[@]}"

  extract_task_id="$(run_json_capture "$BIN" fs extract "$BASE/archive.zip" --to "$BASE/extracted-async" --async --json "${COMMON_ARGS[@]}" | json_extract_task_id)"
  run "$BIN" fs extract status "$extract_task_id" "${COMMON_ARGS[@]}"
  run_stop_with_race_fallback "extract" "$extract_task_id"

  run "$BIN" fs extract "$BASE/archive.zip" --to "$BASE/extracted" "${COMMON_ARGS[@]}"

  run "$BIN" fs tasks "${COMMON_ARGS[@]}"
  run "$BIN" fs tasks-clear "${COMMON_ARGS[@]}"

  run_json_capture "$BIN" fs tasks --json "${COMMON_ARGS[@]}" | json_assert_envelope
  run_json_capture "$BIN" fs list "$BASE" --json "${COMMON_ARGS[@]}" | json_assert_envelope

  run "$BIN" fs delete "$BASE/moved/a-renamed.txt" "${COMMON_ARGS[@]}"

  echo
  echo "Filestation manual script finished."
}

main "$@"
