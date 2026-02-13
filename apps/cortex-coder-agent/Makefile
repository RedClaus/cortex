all: deps build test lint

# Variables
BINARY_NAME=coder
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-X github.com/RedClaus/cortex-coder-agent/internal/version.Version=$(VERSION) \
        -X github.com/RedClaus/cortex-coder-agent/internal/version.BuildTime=$(BUILD_TIME) \
        -X github.com/RedClaus/cortex-coder-agent/internal/version.GitCommit=$(COMMIT)

.PHONY: deps build test lint install clean run docker uninstall benchmark

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME) ./cmd/coder

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter
lint:
	golangci-lint run ./...

# Install the binary
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/coder

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Run the application
run: build
	./bin/$(BINARY_NAME)

# Run in development mode with TUI
dev: build
	./bin/$(BINARY_NAME) tui

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Create a release build
release: clean deps
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/coder
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/coder
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/coder
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/coder

# Show help
help:
	@echo "Available targets:"
	@echo "  all           - Run deps, build, test, and lint"
	@echo "  deps          - Download dependencies"
	@echo "  build         - Build the binary"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  lint          - Run linter"
	@echo "  install       - Install the binary"
	@echo "  clean         - Clean build artifacts"
	@echo "  run           - Build and run"
	@echo "  dev           - Build and run in dev mode"
	@echo "  fmt           - Format code"
	@echo "  vet           - Vet code"
	@echo "  release       - Create release builds"
	@echo "  help          - Show this help"
