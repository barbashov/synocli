#!/usr/bin/env bash
set -euo pipefail

REPO="barbashov/synocli"
API_URL="https://api.github.com/repos/${REPO}"
INSTALL_DIR="${SYNOCLI_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${SYNOCLI_VERSION:-}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd tar
need_cmd mktemp

OS_RAW="$(uname -s)"
ARCH_RAW="$(uname -m)"

case "${OS_RAW}" in
  Linux) GOOS="linux" ;;
  Darwin) GOOS="darwin" ;;
  *)
    echo "error: unsupported OS ${OS_RAW}; installer supports Linux/macOS (WSL2 uses Linux path)" >&2
    exit 1
    ;;
esac

case "${ARCH_RAW}" in
  x86_64|amd64) GOARCH="amd64" ;;
  arm64|aarch64) GOARCH="arm64" ;;
  *)
    echo "error: unsupported architecture ${ARCH_RAW}; supported: amd64, arm64" >&2
    exit 1
    ;;
esac

if [[ -z "${VERSION}" ]]; then
  VERSION="$(curl -fsSL "${API_URL}/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
fi

if [[ ! "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: could not resolve a valid release tag (got: ${VERSION:-<empty>})" >&2
  exit 1
fi

ARCHIVE="synocli_${VERSION}_${GOOS}_${GOARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
ARCHIVE_URL="${BASE_URL}/${ARCHIVE}"
SUMS_URL="${BASE_URL}/SHA256SUMS"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

curl -fsSL "${ARCHIVE_URL}" -o "${TMP_DIR}/${ARCHIVE}"
curl -fsSL "${SUMS_URL}" -o "${TMP_DIR}/SHA256SUMS"

EXPECTED="$(awk -v f="${ARCHIVE}" '$2==f || $2=="*"f {print $1; exit}' "${TMP_DIR}/SHA256SUMS")"
if [[ -z "${EXPECTED}" ]]; then
  echo "error: checksum for ${ARCHIVE} not found in SHA256SUMS" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMP_DIR}/${ARCHIVE}" | awk '{print $1}')"
else
  need_cmd shasum
  ACTUAL="$(shasum -a 256 "${TMP_DIR}/${ARCHIVE}" | awk '{print $1}')"
fi

if [[ "${ACTUAL}" != "${EXPECTED}" ]]; then
  echo "error: checksum mismatch for ${ARCHIVE}" >&2
  exit 1
fi

tar -xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"
BIN_SRC="$(find "${TMP_DIR}" -type f -name synocli | head -n1)"
if [[ -z "${BIN_SRC}" ]]; then
  echo "error: synocli binary not found in archive" >&2
  exit 1
fi

mkdir -p "${INSTALL_DIR}"
TARGET="${INSTALL_DIR}/synocli"
if command -v install >/dev/null 2>&1; then
  install -m 0755 "${BIN_SRC}" "${TARGET}"
else
  cp "${BIN_SRC}" "${TARGET}"
  chmod 0755 "${TARGET}"
fi

echo "Installed synocli ${VERSION} to ${TARGET}"
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo "Add ${INSTALL_DIR} to PATH, for example:" >&2
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\"" >&2
    ;;
esac
