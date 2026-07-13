# Makefile for ch
# A professional CLI chat tool for multiple AI platforms

# Declare phony (non-files)
.PHONY: build install clean test lint fmt fmt-check vet security-static security-vuln security-secrets security-secrets-staged security-secrets-working security verify install-hooks help dev run

# Variables
BINARY_NAME=ch
BINARY_PATH=./bin/$(BINARY_NAME)
BUILD_DIR=./bin
CMD_DIR=./cmd/ch
MAIN_FILE=$(CMD_DIR)/main.go

# Build information
VERSION?=v5.0.3
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
GOSEC?=$(shell if command -v gosec >/dev/null 2>&1; then command -v gosec; else printf "%s/bin/gosec" "$$(go env GOPATH 2>/dev/null)"; fi)
GITLEAKS?=$(shell command -v gitleaks 2>/dev/null)
GOVULNCHECK?=$(shell if command -v govulncheck >/dev/null 2>&1; then command -v govulncheck; else printf "%s/bin/govulncheck" "$$(go env GOPATH 2>/dev/null)"; fi)

# Default target
all: build

## Build the binary
build: security-static
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

## Check formatting without modifying files
fmt-check:
	@echo "Checking formatting..."
	@files=$$($(GOFMT) -l .); \
	if [ -n "$$files" ]; then \
		echo "Go files need formatting:"; \
		echo "$$files"; \
		exit 1; \
	fi

## Run go vet
vet:
	@echo "Running go vet..."
	@$(GOVET) ./...

## Run gosec static security scan (requires gosec)
security-static:
	@echo "Running gosec..."
	@if [ ! -x "$(GOSEC)" ]; then \
		echo "gosec not found. Install with: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
		exit 1; \
	fi
	@$(GOSEC) ./...

## Verify modules and run Go vulnerability checks
security-vuln:
	@echo "Verifying modules..."
	@$(GOMOD) verify
	@echo "Running govulncheck..."
	@if [ -x "$(GOVULNCHECK)" ]; then \
		$(GOVULNCHECK) ./...; \
	else \
		$(GOCMD) run golang.org/x/vuln/cmd/govulncheck@latest ./...; \
	fi

## Scan the repository for committed secrets (requires gitleaks)
security-secrets:
	@echo "Running gitleaks..."
	@if [ -z "$(GITLEAKS)" ] || [ ! -x "$(GITLEAKS)" ]; then \
		echo "gitleaks not found. Install with: brew install gitleaks"; \
		exit 1; \
	fi
	@$(GITLEAKS) git --no-banner --redact .

## Scan staged changes for secrets (requires gitleaks)
security-secrets-staged:
	@echo "Running gitleaks on staged changes..."
	@if [ -z "$(GITLEAKS)" ] || [ ! -x "$(GITLEAKS)" ]; then \
		echo "gitleaks not found. Install with: brew install gitleaks"; \
		exit 1; \
	fi
	@$(GITLEAKS) git --no-banner --redact --staged .

## Scan the current working tree for secrets (requires gitleaks)
security-secrets-working:
	@echo "Running gitleaks on working tree..."
	@if [ -z "$(GITLEAKS)" ] || [ ! -x "$(GITLEAKS)" ]; then \
		echo "gitleaks not found. Install with: brew install gitleaks"; \
		exit 1; \
	fi
	@$(GITLEAKS) dir --no-banner --redact .

## Run all local security checks
security: security-static security-secrets security-secrets-working security-vuln

## Run the full verification gate (formatting, vet, tests, security).
## Portable and provider-agnostic: any CI, self-hosted runner, server-side
## hook, or manual pre-merge check can run this single command.
verify:
	@$(MAKE) fmt-check
	@$(MAKE) vet
	@echo "Running tests..."
	@$(GOTEST) -count=1 ./...
	@$(MAKE) security
	@echo "All verification checks passed"

## Configure this checkout to use versioned local git hooks
install-hooks:
	@test -f .githooks/pre-commit || { echo ".githooks/pre-commit not found"; exit 1; }
	@chmod +x .githooks/pre-commit .githooks/pre-push
	@git config core.hooksPath .githooks
	@echo "Git hooks installed from .githooks/ for this checkout"
	@echo "Verify with: git config --get core.hooksPath"

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
	@echo "  fmt-check   - Check formatting without modifying files"
	@echo "  vet         - Run go vet"
	@echo "  security    - Run gosec, gitleaks, and govulncheck"
	@echo "  security-secrets-working - Scan current checkout for secrets"
	@echo "  verify      - Run the full gate: fmt, vet, tests, security (portable/CI)"
	@echo "  install-hooks - Enable versioned local git hooks"
	@echo "  deps        - Download and tidy dependencies"
	@echo "  dev         - Build and run in development mode"
	@echo "  run         - Build and run with ARGS"
	@echo "  build-all   - Build for multiple platforms"
	@echo "  release     - Create release tarballs"
	@echo "  help        - Show this help"
	@echo "Example usage:"
	@echo "  make build"
	@echo "  make run ARGS='--help'"
	@echo "  make run ARGS='-p groq what is AI?'"
	@echo "  make run ARGS='-w https://example.com'"
	@echo "Dependencies:"
	@echo "  - fzf (brew install fzf)"
	@echo "  - gosec (go install github.com/securego/gosec/v2/cmd/gosec@latest)"
	@echo "  - gitleaks (brew install gitleaks)"
