.PHONY: help swag swag-init swag-install build run run-loc run-testnet test clean docker-build docker-up docker-down docker-logs dev deps fmt lint

# Default target
help:
	@echo "Available targets:"
	@echo "  swag-install - Install swag tool"
	@echo "  swag-init    - Initialize Swagger documentation"
	@echo "  swag         - Alias for swag-init"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application (default: mainnet)"
	@echo "  run-loc      - Run the application with local environment"
	@echo "  run-testnet  - Run the application with testnet environment"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-up    - Start Docker containers"
	@echo "  docker-down  - Stop Docker containers"
	@echo "  docker-logs  - Show Docker logs"
	@echo "  dev          - Development workflow (swag-init + build + run-loc)"
	@echo "  deps         - Download and tidy dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code (requires golangci-lint)"

# Get GOPATH
GOPATH := $(shell go env GOPATH)
SWAG_PATH := $(GOPATH)/bin/swag

# Install swag tool
swag-install:
	@echo "Installing swag tool..."
	@go install github.com/swaggo/swag/cmd/swag@latest
	@echo "swag tool installed successfully!"
	@echo "Location: $(SWAG_PATH)"

# Swagger documentation
swag-init:
	@echo "Generating Swagger documentation..."
	@if [ ! -f "$(SWAG_PATH)" ] && ! command -v swag > /dev/null 2>&1; then \
		echo "swag not found, installing..."; \
		$(MAKE) swag-install; \
	fi
	@if command -v swag > /dev/null 2>&1; then \
		swag init -g cmd/indexer/main.go -o ./docs --parseDependency --parseInternal; \
	elif [ -f "$(SWAG_PATH)" ]; then \
		$(SWAG_PATH) init -g cmd/indexer/main.go -o ./docs --parseDependency --parseInternal; \
	else \
		echo "Using go run to execute swag..."; \
		go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/indexer/main.go -o ./docs --parseDependency --parseInternal; \
	fi
	@echo "Swagger documentation generated successfully!"

swag: swag-init

# Build
build:
	@echo "Building application..."
	@go build -o bin/indexer ./cmd/main.go
	@echo "Build completed: bin/indexer"

# Run
run:
	@echo "Running application (mainnet)..."
	@go run ./cmd/main.go --env=mainnet

run-loc:
	@echo "Running application (local)..."
	@go run ./cmd/main.go --env=loc

run-testnet:
	@echo "Running application (testnet)..."
	@go run ./cmd/main.go --env=testnet

# Test
test:
	@echo "Running tests..."
	@go test -v ./...

# Clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf docs/
	@echo "Clean completed"

# Docker
docker-build:
	@echo "Building Docker image..."
	@cd deploy && docker-compose -f docker-compose.indexer.yml build

docker-up:
	@echo "Starting Docker containers..."
	@cd deploy && docker-compose -f docker-compose.indexer.yml up -d

docker-down:
	@echo "Stopping Docker containers..."
	@cd deploy && docker-compose -f docker-compose.indexer.yml down

docker-logs:
	@echo "Showing Docker logs..."
	@cd deploy && docker-compose -f docker-compose.indexer.yml logs -f

# Development
dev: swag-init build run-loc

# Install dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	@golangci-lint run ./... || echo "golangci-lint not installed, skipping..."

