# Build variables
BINARY_NAME=iperf3_exporter
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE} -X main.builtBy=makefile
REPO_NAME=$(shell grep -m 1 "^module" go.mod | awk '{print $$2}')

.PHONY: all default build clean test lint vet mod generate docker help

default: all

all: mod generate lint vet test build

help:
	@echo "Available commands:"
	@echo "  all               - Run mod, generate, lint, vet, tests and build the binary (default)"
	@echo "  build             - Build the binary"
	@echo "  clean             - Remove build artifacts"
	@echo "  test              - Run tests"
	@echo "  lint              - Run golangci-lint"
	@echo "  vet               - Run go vet"
	@echo "  mod               - Run go mod tidy and download"
	@echo "  generate          - Run go generate"
	@echo "  docker            - Build Docker image for local development"

build:
	CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o ${BINARY_NAME} ./cmd/iperf3_exporter

clean:
	rm -f ${BINARY_NAME}
	rm -rf dist/

test:
	go test ./...

lint:
	golangci-lint run

vet:
	go vet ./...

mod:
	go mod tidy
	go mod download

generate:
	go generate ./...

docker:
	@echo "Building Docker image for local development..."
	docker build -t ${REPO_NAME}:${VERSION} .
