.PHONY: all build test test-integration vet lint fmt

all: fmt vet lint test build

build:
	go build -o bin/dbtote ./cmd/dbtote

test:
	go test ./... -race -cover

test-integration:
	go test ./... -tags=integration

vet:
	go vet ./...

lint:
	golangci-lint run

fmt:
	gofmt -l -w .
	goimports -l -w .