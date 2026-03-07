.PHONY: build install test clean

build:
	go build -o revise .

install:
	go install .

test:
	go test ./...

clean:
	rm -f revise
