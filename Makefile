VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test install
build:
	go build -ldflags "-X main.version=$(VERSION)" -o cataas ./cmd/cataas

test:
	go vet ./...
	go test ./...

install:
	go install -ldflags "-X main.version=$(VERSION)" ./cmd/cataas
