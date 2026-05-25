.PHONY: build run test lint generate verify-generated tidy clean

BIN := bin/api
PKG := ./cmd/api

build:
	@mkdir -p bin
	go build -o $(BIN) $(PKG)

run:
	go run $(PKG)

test:
	go test ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed; see https://golangci-lint.run/"; exit 1; }
	golangci-lint run

generate:
	go generate ./...

# verify-generated re-runs codegen and fails if any tracked file changed.
# Used in CI to catch spec/code drift between PRs.
verify-generated: generate
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "generated files drifted; run 'make generate' and commit"; \
		git status --porcelain; \
		exit 1; \
	fi

tidy:
	go mod tidy

clean:
	rm -rf bin
