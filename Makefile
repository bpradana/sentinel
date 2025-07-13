# Sentinel Makefile
# Build and manage Sentinel reverse proxy tools

.PHONY: help build clean test validate certgen run docker-build docker-run

# Variables
BINARY_DIR = bin
MAIN_PROXY = cmd/proxy/main.go
MAIN_VALIDATOR = cmd/validator/main.go
MAIN_CERTGEN = cmd/certgen/main.go
CONFIG_DIR = config
CERT_DIR = certs

# Default target
help:
	@echo "Sentinel - High-Performance Reverse Proxy"
	@echo "========================================"
	@echo ""
	@echo "Available targets:"
	@echo "  build      - Build all binaries (proxy, validator, certgen)"
	@echo "  clean      - Remove build artifacts"
	@echo "  test       - Run tests"
	@echo "  validate   - Validate configuration"
	@echo "  certgen    - Generate self-signed certificates"
	@echo "  run        - Run the proxy server"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo ""

# Build all binaries
build: clean
	@echo "🔨 Building Sentinel binaries..."
	@mkdir -p $(BINARY_DIR)
	@go build -o $(BINARY_DIR)/sentinel $(MAIN_PROXY)
	@go build -o $(BINARY_DIR)/validator $(MAIN_VALIDATOR)
	@go build -o $(BINARY_DIR)/certgen $(MAIN_CERTGEN)
	@echo "✅ Build complete!"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BINARY_DIR)
	@echo "✅ Clean complete!"

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test ./...
	@echo "✅ Tests complete!"

# Validate configuration
validate: build
	@echo "🔍 Validating configuration..."
	@./$(BINARY_DIR)/validator -config $(CONFIG_DIR) -verbose

# Generate self-signed certificates
certgen: build
	@echo "🔐 Generating self-signed certificates..."
	@./$(BINARY_DIR)/certgen -hosts "localhost,127.0.0.1" -output $(CERT_DIR)

# Run the proxy server
run: build
	@echo "🚀 Starting Sentinel proxy server..."
	@./$(BINARY_DIR)/sentinel -config $(CONFIG_DIR)

# Run with debug logging
run-debug: build
	@echo "🚀 Starting Sentinel proxy server (debug mode)..."
	@./$(BINARY_DIR)/sentinel -config $(CONFIG_DIR) -log-level debug

# Build Docker image
docker-build:
	@echo "🐳 Building Docker image..."
	@docker build -t sentinel:latest .
	@echo "✅ Docker build complete!"

# Run Docker container
docker-run: docker-build
	@echo "🐳 Running Docker container..."
	@docker run -d \
		--name sentinel \
		-p 8080:8080 \
		-p 8443:8443 \
		-p 8081:8081 \
		-p 8082:8082 \
		-v $(PWD)/$(CONFIG_DIR):/app/config \
		-v $(PWD)/$(CERT_DIR):/app/certs \
		sentinel:latest
	@echo "✅ Docker container started!"

# Stop Docker container
docker-stop:
	@echo "🛑 Stopping Docker container..."
	@docker stop sentinel || true
	@docker rm sentinel || true
	@echo "✅ Docker container stopped!"

# Show logs
logs:
	@docker logs -f sentinel

# Quick setup (build, generate certs, validate)
setup: build certgen validate
	@echo "🎉 Setup complete! You can now run: make run"

# Development mode (with hot reload)
dev: build
	@echo "🔧 Starting in development mode..."
	@./$(BINARY_DIR)/sentinel -config $(CONFIG_DIR) -log-level debug

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod tidy
	@echo "✅ Dependencies installed!"

# Format code
fmt:
	@echo "🎨 Formatting code..."
	@go fmt ./...
	@echo "✅ Code formatted!"

# Lint code
lint:
	@echo "🔍 Linting code..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Security check
security:
	@echo "🔒 Running security checks..."
	@go list -json -deps . | nancy sleuth || echo "⚠️  nancy not found. Install with: go install github.com/sonatype-nexus-community/nancy@latest"

# Performance benchmark
bench:
	@echo "⚡ Running benchmarks..."
	@go test -bench=. ./...

# Generate documentation
docs:
	@echo "📚 Generating documentation..."
	@if command -v godoc > /dev/null; then \
		godoc -http=:6060 & echo "📖 Documentation available at http://localhost:6060"; \
	else \
		echo "⚠️  godoc not found. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Show help for all targets
list:
	@$(MAKE) -pRrn : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'

# Check system requirements
check:
	@echo "🔍 Checking system requirements..."
	@command -v go > /dev/null || (echo "❌ Go not found. Please install Go 1.23.8 or later." && exit 1)
	@go version
	@echo "✅ System requirements met!"

# Full development setup
dev-setup: check deps fmt lint build certgen validate
	@echo "🎉 Development environment ready!"

# Production build
prod-build: clean
	@echo "🏭 Building production binaries..."
	@mkdir -p $(BINARY_DIR)
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BINARY_DIR)/sentinel $(MAIN_PROXY)
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BINARY_DIR)/validator $(MAIN_VALIDATOR)
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BINARY_DIR)/certgen $(MAIN_CERTGEN)
	@echo "✅ Production build complete!" 