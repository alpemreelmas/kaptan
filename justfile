set shell := ["bash", "-c"]

GOBIN := `go env GOPATH`
export PATH := env("PATH") + ":" + GOBIN + "/bin"

# Regenerate gRPC code from proto/agent.proto
proto:
  cd proto && buf generate

# Build the kaptan-agent binary
agent:
  go build -o bin/kaptan-agent ./agent

# Build the kaptan CLI (m)
cli:
  go build -o bin/m ./cli

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
  cp bin/m {{GOBIN}}/m
  @echo "Installed m to {{GOBIN}}/m"

# Run agent locally (no TLS)
dev-agent:
  cd agent && go run . --config /dev/null

# Show all available tasks
help:
  @just --list
