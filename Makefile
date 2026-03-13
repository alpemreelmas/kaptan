.PHONY: proto agent cli build test lint clean

GOBIN := $(shell go env GOPATH)/bin
PATH  := $(PATH):$(GOBIN)

## proto: regenerate gRPC code from proto/agent.proto
proto:
	cd proto && $(GOBIN)/buf generate

## agent: build the kaptan-agent binary
agent:
	go build -o bin/kaptan-agent ./agent

## cli: build the kaptan CLI (m)
cli:
	go build -o bin/m ./cli

## build: build everything
build: agent cli

## test: run all tests
test:
	go test ./agent/... ./cli/...

## lint: run golangci-lint
lint:
	golangci-lint run ./agent/... ./cli/...

## tidy: go mod tidy in all modules
tidy:
	cd proto/gen && go mod tidy
	cd agent    && go mod tidy
	cd cli      && go mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin/

## install: install CLI to GOBIN
install: cli
	cp bin/m $(GOBIN)/m
	@echo "Installed m to $(GOBIN)/m"

## dev: run agent locally (no TLS)
dev-agent:
	cd agent && go run . --config /dev/null
