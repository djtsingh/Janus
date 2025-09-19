.PHONY: build run fmt vet

BINARY=janus
CMD=./cmd/janus

build:
	go build -o $(BINARY) ./cmd/janus

run: build
	./$(BINARY)

fmt:
	gofmt -s -w .

vet:
	go vet ./...
