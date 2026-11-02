VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: build test install dist
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o cataas ./cmd/cataas

test:
	go vet ./...
	go test ./...

install:
	go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/cataas

# Cross-compile the committed binaries in bin/ (used by install.sh).
dist:
	rm -rf bin && mkdir -p bin
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "bin/cataas-$$os-$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o bin/cataas-$$os-$$arch ./cmd/cataas || exit 1; \
	done
	cd bin && sha256sum cataas-* > SHA256SUMS
