#!/bin/sh
# Entry point for the synocli-e2e image.
#
# - Synocli refuses to read a credentials file unless it is mode 0600. A bind-
#   mounted file inherits its host perms, which are commonly 0644. Copying the
#   mount into a writable tmpfs path lets us enforce 0600 without touching the
#   user's file on the host.
# - Any positional args are forwarded to pytest. The standard invocation is:
#       docker run --rm -v /host/creds:/creds:ro synocli-e2e \
#         --endpoint https://nas:5001 --insecure-tls
#   In that case the entrypoint also injects --credentials-file pointing at
#   the normalized copy. If the caller passes their own --credentials-file
#   the override is honored as-is.
set -eu

CREDS_SRC="${SYNOCLI_E2E_CREDS:-/creds}"
CREDS_NORMALIZED="/tmp/synocli-creds"
INJECT_CREDS=1

# If the caller already passes --credentials-file we honor their choice rather
# than overriding it with the normalized copy.
for arg in "$@"; do
    case "$arg" in
        --credentials-file|--credentials-file=*)
            INJECT_CREDS=0
            break
            ;;
    esac
done

if [ -f "$CREDS_SRC" ]; then
    cp "$CREDS_SRC" "$CREDS_NORMALIZED"
    chmod 600 "$CREDS_NORMALIZED"
fi

EXTRA=""
if [ "$INJECT_CREDS" = "1" ] && [ -f "$CREDS_NORMALIZED" ]; then
    EXTRA="--credentials-file $CREDS_NORMALIZED"
fi

exec pytest tests_e2e/ $EXTRA "$@"
