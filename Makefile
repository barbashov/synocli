BINARY := synocli

.PHONY: build test lint

build:
	CGO_ENABLED=0 go build -o bin/$(BINARY) ./cmd/synocli

test:
	go test ./...

lint:
	go test ./...
