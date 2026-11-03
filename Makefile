.PHONY: fmt vet test race build verify demo binaries

demo:
	go run ./cmd/demo

VERSION ?= dev
binaries:
	go build -trimpath -ldflags="-X main.version=$(VERSION)" -o bin/tunnelforge ./cmd/tunnelforge
	go build -trimpath -o bin/relay ./cmd/relay

fmt:
	gofmt -w .

vet:
	go vet ./...

test:
	go test -count=1 ./...

race:
	go test -race -count=1 ./...

build:
	go build ./...

verify: fmt vet test race build
