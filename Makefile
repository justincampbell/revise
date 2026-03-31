VERSION ?= $(shell git describe --tags --dirty --always | sed 's/-[0-9]*-g/-g/')
LDFLAGS := -X main.version=$(VERSION)
GOLANGCI_LINT_VERSION := v2.11.4
GOBIN := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)

.PHONY: all build install test lint clean

all: lint test

build:
	go build -ldflags "$(LDFLAGS)" -o revise .
	cp revise "revise@$(VERSION)"

install:
	go build -ldflags "$(LDFLAGS)" -o "$(GOBIN)/revise" .
	cp "$(GOBIN)/revise" "$(GOBIN)/revise@$(VERSION)"
	@echo "Installed $(GOBIN)/revise and $(GOBIN)/revise@$(VERSION)"

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

clean:
	rm -f revise revise@*
