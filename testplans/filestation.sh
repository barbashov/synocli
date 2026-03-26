#!/usr/bin/env bash
set -euo pipefail

BIN="${BIN:-./bin/synocli}"
ENDPOINT="${ENDPOINT:-}"
CREDS="${CREDS:-}"
BASE="${BASE:-/volume1/synocli-e2e}"
INSECURE_TLS="${INSECURE_TLS:-1}"
RESET_REMOTE="${RESET_REMOTE:-0}"

if [[ -z "$ENDPOINT" || -z "$CREDS" ]]; then
  echo "ENDPOINT and CREDS are required"
  echo "Example: ENDPOINT=https://nas:5001 CREDS=/path/creds.env BASE=/volume1/synocli-e2e $0"
  exit 1
fi

BASE_PARENT="$(dirname "$BASE")"
BASE_NAME="$(basename "$BASE")"

COMMON_ARGS=(--credentials-file "$CREDS")
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
    echo "Command failed (already in debug mode)."
    return 1
  fi

  echo "Command failed. Retrying with --debug:"
  echo ">>> ${args[*]} --debug"
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
  run "$BIN" fs delete "$ENDPOINT" "$BASE" -r --json "${COMMON_ARGS[@]}" || true
  run "$BIN" fs mkdir "$ENDPOINT" "$BASE_PARENT" "$BASE_NAME" --parents "${COMMON_ARGS[@]}"
}

cleanup_remote_artifacts() {
  run "$BIN" fs delete "$ENDPOINT" "$BASE/a.txt" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$ENDPOINT" "$BASE/a-renamed.txt" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$ENDPOINT" "$BASE/archive.zip" "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$ENDPOINT" "$BASE/copy" -r "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$ENDPOINT" "$BASE/moved" -r "${COMMON_ARGS[@]}" || true
  run "$BIN" fs delete "$ENDPOINT" "$BASE/extracted" -r "${COMMON_ARGS[@]}" || true
}

main() {
  prepare_fixtures
  reset_remote
  cleanup_remote_artifacts

  run "$BIN" auth ping "$ENDPOINT" "${COMMON_ARGS[@]}"
  run "$BIN" auth api-info "$ENDPOINT" --prefix SYNO.FileStation "${COMMON_ARGS[@]}"

  run "$BIN" fs mkdir "$ENDPOINT" "$BASE_PARENT" "$BASE_NAME" --parents "${COMMON_ARGS[@]}"
  run "$BIN" fs ls "$ENDPOINT" "$BASE_PARENT" "${COMMON_ARGS[@]}"

  run "$BIN" fs upload "$ENDPOINT" ./tmp/fs-fixtures/a.txt "$BASE" "${COMMON_ARGS[@]}"
  run "$BIN" fs rename "$ENDPOINT" "$BASE/a.txt" a-renamed.txt "${COMMON_ARGS[@]}"
  run "$BIN" fs get "$ENDPOINT" "$BASE/a-renamed.txt" "${COMMON_ARGS[@]}"

  run "$BIN" fs cp "$ENDPOINT" "$BASE/a-renamed.txt" --to "$BASE/copy" "${COMMON_ARGS[@]}"
  run "$BIN" fs mv "$ENDPOINT" "$BASE/copy/a-renamed.txt" --to "$BASE/moved" "${COMMON_ARGS[@]}"

  run "$BIN" fs search "$ENDPOINT" "$BASE" --pattern renamed "${COMMON_ARGS[@]}"
  run "$BIN" fs dir-size "$ENDPOINT" "$BASE" "${COMMON_ARGS[@]}"
  run "$BIN" fs md5 "$ENDPOINT" "$BASE/moved/a-renamed.txt" "${COMMON_ARGS[@]}"

  run "$BIN" fs compress "$ENDPOINT" "$BASE" --to "$BASE/archive.zip" "${COMMON_ARGS[@]}"
  run "$BIN" fs extract "$ENDPOINT" "$BASE/archive.zip" --to "$BASE/extracted" "${COMMON_ARGS[@]}"

  run "$BIN" fs tasks "$ENDPOINT" "${COMMON_ARGS[@]}"

  run "$BIN" fs delete "$ENDPOINT" "$BASE/moved/a-renamed.txt" "${COMMON_ARGS[@]}"

  echo
  echo "Filestation manual script finished."
}

main "$@"
