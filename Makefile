.PHONY: build install test test-e2e vet lint tidy clean

BIN     := bin/basego
PKG     := ./cmd/basego
VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%d)

LDFLAGS := -X github.com/guilherme-grimm/basego/internal/cli.version=$(VERSION) \
           -X github.com/guilherme-grimm/basego/internal/cli.commit=$(COMMIT) \
           -X github.com/guilherme-grimm/basego/internal/cli.date=$(DATE)

build:
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

# E2E builds basego + scaffolds a project; isolated so `make test-e2e` can
# be run without the rest, e.g. for slow-machine local loops.
test-e2e:
	go test ./test/e2e/...

vet:
	go vet ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed; see https://golangci-lint.run/"; exit 1; }
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -rf bin
