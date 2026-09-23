.PHONY: build test test-integration vet lint fmt

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