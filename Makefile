VERSION ?= $(shell git describe --tags --dirty --always | sed 's/-[0-9]*-g/-g/')
LDFLAGS := -X main.version=$(VERSION)
GOLANGCI_LINT_VERSION := v2.11.4

.PHONY: all build install test lint clean

all: lint test

build:
	go build -ldflags "$(LDFLAGS)" -o revise .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

clean:
	rm -f revise
