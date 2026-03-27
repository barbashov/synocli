BINARY := synocli
BUILD_VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X 'synocli/internal/cli.buildVersion=$(BUILD_VERSION)' -X 'synocli/internal/cli.buildCommit=$(COMMIT)' -X 'synocli/internal/cli.buildDate=$(BUILD_DATE)'

.PHONY: build test lint release-check release

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/synocli

test:
	go test ./...

lint:
	docker run --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v2.11-alpine golangci-lint run ./...

release-check:
	./scripts/check-release.sh $(if $(VERSION),--tag $(VERSION),)

release:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (example: make release VERSION=v0.1.0)"; \
		exit 1; \
	fi
	./scripts/release.sh $(VERSION)
