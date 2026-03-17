set shell := ["bash", "-c"]

GOBIN := `go env GOPATH`
export PATH := env("PATH") + ":" + GOBIN + "/bin"

# Regenerate gRPC code from proto/agent.proto
proto:
  cd proto && buf generate

# Build the reis binary
agent:
  go build -o bin/reis ./agent

# Build the kaptan CLI
cli:
  go build -o bin/kaptan ./cli

# Build everything
build: agent cli

# Run all tests
test:
  go test ./agent/... ./cli/...

# Run golangci-lint
lint:
  golangci-lint run ./agent/... ./cli/...

# Go mod tidy in all modules
tidy:
  cd proto/gen && go mod tidy
  cd agent    && go mod tidy
  cd cli      && go mod tidy

# Remove build artifacts
clean:
  rm -rf bin/

# Install CLI to GOBIN
install: cli
  cp bin/kaptan {{GOBIN}}/kaptan
  @echo "Installed kaptan to {{GOBIN}}/kaptan"

# Run agent locally (no TLS)
dev-agent:
  cd agent && go run . --config /dev/null

# Show all available tasks
help:
  @just --list
