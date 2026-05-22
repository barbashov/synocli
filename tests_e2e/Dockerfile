# Multi-stage image for running the synocli e2e suite against a live NAS.
#
# Stage 1 builds the synocli binary so the host doesn't need Go installed.
# Stage 2 is a slim Python runtime with pytest and the test sources baked in.
#
# Build context is the repo root:
#   docker build -f tests_e2e/Dockerfile -t synocli-e2e .

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/synocli ./cmd/synocli


FROM python:3.12-slim AS runtime

# ca-certificates is needed for HTTPS to webtorrent.io and to the NAS endpoint;
# tini gives us clean signal handling so Ctrl-C from `docker run` cleans up the
# pytest run instead of orphaning it.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tini \
    && rm -rf /var/lib/apt/lists/*

# Install pytest + plugins as a standalone layer so the rebuild only fires
# when this line changes. Keep these versions in sync with tests_e2e/pyproject.toml.
RUN pip install --no-cache-dir \
    "pytest>=8.0" \
    "pytest-timeout>=2.3" \
    "pytest-subtests>=0.13"

WORKDIR /work

# Place the binary where conftest.py's DEFAULT_BIN resolves it
# (parents[1]/bin/synocli relative to tests_e2e/conftest.py).
COPY --from=builder /out/synocli /work/bin/synocli

# Copy the test suite last so source edits don't bust earlier layers.
COPY tests_e2e/ /work/tests_e2e/

COPY tests_e2e/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENTRYPOINT ["/usr/bin/tini", "--", "/usr/local/bin/docker-entrypoint.sh"]
CMD []
