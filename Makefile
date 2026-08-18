.PHONY: build test lint

build:
	go build -o bin/media-inventory ./cmd/media-inventory

test:
	go test -race ./...

lint:
	gofmt -l .
	go vet ./...
