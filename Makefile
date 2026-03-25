BINARY := synocli

.PHONY: build test lint

build:
	CGO_ENABLED=0 go build -o bin/$(BINARY) ./cmd/synocli

test:
	go test ./...

lint:
	docker run --rm -v $(CURDIR):/app -w /app golangci/golangci-lint:v2.11-alpine golangci-lint run ./...
