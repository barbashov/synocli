#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  testplans/filestation.sh --endpoint <url> --credentials-file <path> [options]

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
EOF
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
  run "$BIN" fs delete "$BASE" -r --json "${COMMON_ARGS[@]}" || true
  run "$BIN" fs mkdir "$BASE_PARENT" "$BASE_NAME" --parents "${COMMON_ARGS[@]}"
}

cleanup_remote_artifacts() {
  run "$BIN" fs delete "$BASE/a.txt" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/a-renamed.txt" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/archive.zip" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/copy" -r "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/moved" -r "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$BASE/extracted" -r "${COMMON_ARGS[@]}" || true
}

main() {
  prepare_fixtures
  reset_remote
  cleanup_remote_artifacts

  run "$BIN" auth ping "${COMMON_ARGS[@]}"
  run "$BIN" auth api-info --prefix SYNO.FileStation "${COMMON_ARGS[@]}"

  run "$BIN" fs mkdir "$BASE_PARENT" "$BASE_NAME" --parents "${COMMON_ARGS[@]}"
  run "$BIN" fs ls "$BASE_PARENT" "${COMMON_ARGS[@]}"

  run "$BIN" fs upload ./tmp/fs-fixtures/a.txt "$BASE" "${COMMON_ARGS[@]}"
  run "$BIN" fs rename "$BASE/a.txt" a-renamed.txt "${COMMON_ARGS[@]}"
  run "$BIN" fs get "$BASE/a-renamed.txt" "${COMMON_ARGS[@]}"

  run "$BIN" fs cp "$BASE/a-renamed.txt" --to "$BASE/copy" "${COMMON_ARGS[@]}"
  run "$BIN" fs mv "$BASE/copy/a-renamed.txt" --to "$BASE/moved" "${COMMON_ARGS[@]}"

  run "$BIN" fs search "$BASE" --pattern renamed "${COMMON_ARGS[@]}"
  run "$BIN" fs dir-size "$BASE" "${COMMON_ARGS[@]}"
  run "$BIN" fs md5 "$BASE/moved/a-renamed.txt" "${COMMON_ARGS[@]}"

  run "$BIN" fs compress "$BASE" --to "$BASE/archive.zip" "${COMMON_ARGS[@]}"
  run "$BIN" fs extract "$BASE/archive.zip" --to "$BASE/extracted" "${COMMON_ARGS[@]}"

  run "$BIN" fs tasks "${COMMON_ARGS[@]}"

  run "$BIN" fs delete "$BASE/moved/a-renamed.txt" "${COMMON_ARGS[@]}"

  echo
  echo "Filestation manual script finished."
}

main "$@"
