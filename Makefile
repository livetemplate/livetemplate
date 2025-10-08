.PHONY: build install test clean version

# Build variables
BINARY_NAME=lvt
BUILD_DIR=cmd/lvt
VERSION?=dev
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-X main.version=$(VERSION) -X main.date=$(BUILD_TIME) -X main.commit=$(GIT_COMMIT)

# Build the lvt binary with timestamp
build:
	@echo "Building $(BINARY_NAME) with build timestamp..."
	@cd $(BUILD_DIR) && go build -ldflags "$(LDFLAGS)" -o ../../$(BINARY_NAME)
	@echo "✅ Built $(BINARY_NAME) ($(BUILD_TIME))"

# Install the binary to $GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME) to $$GOPATH/bin..."
	@cd $(BUILD_DIR) && go install -ldflags "$(LDFLAGS)"
	@echo "✅ Installed $(BINARY_NAME)"

# Run all tests
test:
	@echo "Running tests..."
	@go test ./... -v

# Run tests without cache
test-nocache:
	@echo "Running tests without cache..."
	@go test ./... -v -count=1

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -f cmd/lvt/$(BINARY_NAME)
	@echo "✅ Cleaned"

# Show version info
version:
	@./$(BINARY_NAME) version 2>/dev/null || echo "Binary not built yet. Run 'make build' first."

# Quick build and test
quick: build
	@./$(BINARY_NAME) version

# Development build (same as build, but with explicit dev version)
dev:
	@$(MAKE) build VERSION=dev-$(GIT_COMMIT)

# Help target
help:
	@echo "LiveTemplate lvt Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make build         Build lvt binary with timestamp"
	@echo "  make install       Install lvt to \$$GOPATH/bin"
	@echo "  make test          Run all tests"
	@echo "  make test-nocache  Run tests without cache"
	@echo "  make clean         Remove build artifacts"
	@echo "  make version       Show version of built binary"
	@echo "  make quick         Build and show version"
	@echo "  make dev           Build with dev version tag"
	@echo "  make help          Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build                    # Build with timestamp"
	@echo "  make build VERSION=v1.0.0     # Build with specific version"
	@echo "  make install                  # Install to \$$GOPATH/bin"
