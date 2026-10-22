BINARY  := walltui
PKG     := ./cmd/$(BINARY)
VERSION := $(shell v=$$(git describe --tags --always --dirty 2>/dev/null || echo dev); echo $${v#v})
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test race vet lint run clean

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

lint: vet
	@command -v staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipped"

run:
	go run $(PKG)

clean:
	rm -rf bin
