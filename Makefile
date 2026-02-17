.PHONY: build clean test lint

BINARY=sentinel
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/sentinel

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

lint:
	golangci-lint run ./...

# Quick commands — usage: make discover PROVIDER=openai
PROVIDER ?= openai
discover:
	go run ./cmd/sentinel discover --provider=$(PROVIDER)

diff:
	go run ./cmd/sentinel diff

sync:
	go run ./cmd/sentinel sync --dry-run

validate:
	go run ./cmd/sentinel validate --catalog-path=../model-catalog
