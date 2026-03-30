VERSION ?= $(shell git describe --tags --dirty --always | sed 's/-[0-9]*-g/-g/')
LDFLAGS := -X main.version=$(VERSION)
GOLANGCI_LINT_VERSION := v2.11.4
GOBIN := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)

.PHONY: all build install test lint lint-gomod clean

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

lint: lint-gomod
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run ./...

lint-gomod:
	@cp go.mod go.mod.bak && cp go.sum go.sum.bak && \
	go mod tidy && \
	if ! diff -q go.mod go.mod.bak > /dev/null 2>&1; then \
		echo "error: go.mod is not tidy (go version may have drifted). Run 'go mod tidy' and commit the result." >&2; \
		mv go.mod.bak go.mod; mv go.sum.bak go.sum; \
		exit 1; \
	fi; \
	mv go.sum.bak go.sum; rm -f go.mod.bak

clean:
	rm -f revise revise@*
