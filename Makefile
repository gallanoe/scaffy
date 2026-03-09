.PHONY: build test lint run clean vet

build:
	go build -o scaffy ./cmd/scaffy

test:
	go test -v -race ./...

lint:
	golangci-lint run

vet:
	go vet ./...

run: build
	./scaffy

clean:
	rm -f scaffy coverage.out
