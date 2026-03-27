BINARY := synocli
DIST_DIR ?= dist
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64
BUILD_VERSION ?= $(if $(VERSION),$(VERSION),dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X 'synocli/internal/cli.buildVersion=$(BUILD_VERSION)' -X 'synocli/internal/cli.buildCommit=$(COMMIT)' -X 'synocli/internal/cli.buildDate=$(BUILD_DATE)'
SHA256_TOOL ?= $(shell command -v sha256sum 2>/dev/null || command -v shasum 2>/dev/null)

.PHONY: build build-platform build-release checksums clean-dist test lint release-check release

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/synocli

build-platform:
	@if [ -z "$(GOOS)" ] || [ -z "$(GOARCH)" ]; then \
		echo "GOOS and GOARCH are required (example: make build-platform VERSION=v0.1.0 GOOS=linux GOARCH=amd64)"; \
		exit 1; \
	fi
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (example: make build-platform VERSION=v0.1.0 GOOS=linux GOARCH=amd64)"; \
		exit 1; \
	fi
	@mkdir -p "$(DIST_DIR)"
	@tmpdir="$$(mktemp -d)"; \
		archive_base="$(BINARY)_$(VERSION)_$(GOOS)_$(GOARCH)"; \
		stage_dir="$$tmpdir/$$archive_base"; \
		bin_name="$(BINARY)"; \
		if [ "$(GOOS)" = "windows" ]; then bin_name="$(BINARY).exe"; fi; \
		mkdir -p "$$stage_dir"; \
		CGO_ENABLED=0 GOOS="$(GOOS)" GOARCH="$(GOARCH)" go build -ldflags "$(LDFLAGS)" -o "$$stage_dir/$$bin_name" ./cmd/synocli; \
		if [ "$(GOOS)" = "windows" ]; then \
			( cd "$$tmpdir" && zip -rq "$(CURDIR)/$(DIST_DIR)/$$archive_base.zip" "$$archive_base" ); \
		else \
			tar -C "$$tmpdir" -czf "$(DIST_DIR)/$$archive_base.tar.gz" "$$archive_base"; \
		fi; \
		rm -rf "$$tmpdir"

build-release: clean-dist
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (example: make build-release VERSION=v0.1.0)"; \
		exit 1; \
	fi
	@for platform in $(PLATFORMS); do \
		os="$${platform%/*}"; \
		arch="$${platform#*/}"; \
		$(MAKE) --no-print-directory build-platform VERSION="$(VERSION)" GOOS="$$os" GOARCH="$$arch" DIST_DIR="$(DIST_DIR)"; \
	done
	@$(MAKE) --no-print-directory checksums DIST_DIR="$(DIST_DIR)"

checksums:
	@if [ -z "$(SHA256_TOOL)" ]; then \
		echo "sha256sum or shasum is required"; \
		exit 1; \
	fi
	@mkdir -p "$(DIST_DIR)"
	@cd "$(DIST_DIR)" && rm -f SHA256SUMS
	@if command -v sha256sum >/dev/null 2>&1; then \
		cd "$(DIST_DIR)" && sha256sum synocli_* > SHA256SUMS; \
	else \
		cd "$(DIST_DIR)" && shasum -a 256 synocli_* > SHA256SUMS; \
	fi

clean-dist:
	rm -rf "$(DIST_DIR)"

test:
	go test ./...

lint:
	docker run --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v2.11.4-alpine golangci-lint run ./...

release-check:
	./scripts/check-release.sh $(if $(VERSION),--tag $(VERSION),)

release:
	@if [ -z "$(VERSION)" ]; then \
		echo "VERSION is required (example: make release VERSION=v0.1.0)"; \
		exit 1; \
	fi
	./scripts/release.sh $(VERSION)
