VERSION ?= $(shell git describe --tags --dirty --always | sed 's/-[0-9]*-g/-g/')
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install test clean

build:
	go build -ldflags "$(LDFLAGS)" -o revise .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test ./...

clean:
	rm -f revise
