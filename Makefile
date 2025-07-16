# MicroLib Go Framework Makefile

# Variables
BINARY_NAME=microlib
GO_VERSION=1.22
GOLANGCI_LINT_VERSION=v1.55.2

# Build info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME = $(shell date -u '+%Y-%m-%d_%H:%M:%S')
BUILD_HASH = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go build flags
LDFLAGS = -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.buildHash=$(BUILD_HASH)"

.PHONY: help build test lint format clean deps install-tools check coverage benchmark

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/$(BINARY_NAME)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/$(BINARY_NAME)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/$(BINARY_NAME)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/$(BINARY_NAME)

# Test targets
test: ## Run tests
	@echo "Running tests..."
	go test -v -race ./...

test-short: ## Run short tests
	@echo "Running short tests..."
	go test -v -short ./...

coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -v -bench=. -benchmem ./...

# Code quality targets
lint: ## Run linter
	@echo "Running linter..."
	golangci-lint run

lint-fix: ## Run linter with auto-fix
	@echo "Running linter with auto-fix..."
	golangci-lint run --fix

format: ## Format code
	@echo "Formatting code..."
	gofmt -s -w .
	goimports -w .

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Development tools
install-tools: ## Install development tools
	@echo "Installing development tools..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION))
	@which goimports > /dev/null || (echo "Installing goimports..." && go install golang.org/x/tools/cmd/goimports@latest)
	@which pre-commit > /dev/null || (echo "Installing pre-commit..." && pip install pre-commit)

# Git hooks
install-hooks: ## Install git hooks
	@echo "Installing git hooks..."
	pre-commit install

# Quality checks
check: format lint test ## Run all quality checks

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):$(VERSION) .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 $(BINARY_NAME):$(VERSION)

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache
	go clean -testcache

clean-all: clean ## Clean everything including dependencies
	go clean -modcache

# Development server
dev: ## Run in development mode with hot reload
	@echo "Starting development server..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Generate code
generate: ## Run go generate
	@echo "Running go generate..."
	go generate ./...

# Security scan
security: ## Run security scan
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec ./...

# Mod verification
verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	go mod verify

# Full CI pipeline
ci: deps verify generate format lint test coverage ## Run full CI pipeline

.DEFAULT_GOAL := help