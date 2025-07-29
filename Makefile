# Makefile for ch
# A professional CLI chat tool for multiple AI platforms

.PHONY: build install clean test lint fmt vet help dev run

# Variables
BINARY_NAME=ch
BINARY_PATH=./bin/$(BINARY_NAME)
BUILD_DIR=./bin
CMD_DIR=./cmd/ch
MAIN_FILE=$(CMD_DIR)/main.go

# Build information
VERSION?=v1.0.0
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Default target
all: build

## Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) $(MAIN_FILE)
	@echo "Build complete: $(BINARY_PATH)"

## Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(GOPATH)/bin/$(BINARY_NAME) $(MAIN_FILE)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

## Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Clean complete"

## Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## Run the application in development mode
dev: build
	@echo "Running in development mode..."
	$(BINARY_PATH)

## Run the application with arguments
run: build
	$(BINARY_PATH) $(ARGS)

## Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)
	@echo "Multi-platform build complete"

## Create a release tarball
release: build-all
	@echo "Creating release tarballs..."
	@mkdir -p $(BUILD_DIR)/release
	@for binary in $(BUILD_DIR)/$(BINARY_NAME)-*; do \
		if [ -f "$$binary" ]; then \
			basename=$$(basename "$$binary"); \
			tar -czf "$(BUILD_DIR)/release/$$basename.tar.gz" -C $(BUILD_DIR) "$$basename" README.md LICENSE; \
		fi; \
	done
	@echo "Release tarballs created in $(BUILD_DIR)/release/"

## Display help
help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  install     - Install the binary to \$$GOPATH/bin"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  lint        - Run linter (requires golangci-lint)"
	@echo "  fmt         - Format code"
	@echo "  vet         - Run go vet"
	@echo "  deps        - Download and tidy dependencies"
	@echo "  dev         - Build and run in development mode"
	@echo "  run         - Build and run with ARGS"
	@echo "  build-all   - Build for multiple platforms"
	@echo "  release     - Create release tarballs"
	@echo "  help        - Show this help"
	@echo ""
	@echo "Example usage:"
	@echo "  make build"
	@echo "  make run ARGS='--help'"
	@echo "  make run ARGS='-p groq what is AI?'"
	@echo "  make run ARGS='-w https://example.com'"
	@echo ""
	@echo "Dependencies:"
	@echo "  - fzf (brew install fzf)"
