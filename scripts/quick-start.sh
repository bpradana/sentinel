#!/bin/bash

# Sentinel Quick Start Script
# This script helps you get Sentinel up and running quickly

set -e

echo "🚀 Sentinel Quick Start"
echo "======================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.23.8 or later."
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    print_success "Go version $GO_VERSION found"
}

# Check if Docker is installed (optional)
check_docker() {
    if command -v docker &> /dev/null; then
        print_success "Docker found"
        DOCKER_AVAILABLE=true
    else
        print_warning "Docker not found. Docker commands will be skipped."
        DOCKER_AVAILABLE=false
    fi
}

# Build the project
build_project() {
    print_status "Building Sentinel binaries..."
    
    # Create bin directory
    mkdir -p bin
    
    # Build all binaries
    go build -o bin/sentinel cmd/proxy/main.go
    go build -o bin/validator cmd/validator/main.go
    go build -o bin/certgen cmd/certgen/main.go
    
    print_success "Build complete!"
}

# Generate certificates
generate_certificates() {
    print_status "Generating self-signed certificates..."
    
    # Create certs directory
    mkdir -p certs
    
    # Generate certificates
    ./bin/certgen -hosts "localhost,127.0.0.1" -output ./certs
    
    print_success "Certificates generated!"
}

# Validate configuration
validate_config() {
    print_status "Validating configuration..."
    
    if ./bin/validator -config ./config; then
        print_success "Configuration validation passed!"
    else
        print_error "Configuration validation failed!"
        exit 1
    fi
}

# Start the proxy
start_proxy() {
    print_status "Starting Sentinel proxy..."
    print_status "Press Ctrl+C to stop"
    echo ""
    
    ./bin/sentinel -config ./config
}

# Start with Docker
start_docker() {
    if [ "$DOCKER_AVAILABLE" = true ]; then
        print_status "Starting with Docker Compose..."
        
        # Build and start services
        docker compose up --build -d
        
        print_success "Docker services started!"
        echo ""
        echo "Services available at:"
        echo "  - Sentinel Proxy: http://localhost:8080"
        echo "  - Sentinel HTTPS: https://localhost:8443"
        echo "  - Health Check: http://localhost:8081/health"
        echo "  - Metrics: http://localhost:8082/metrics"
        echo "  - Prometheus: http://localhost:9090"
        echo "  - Grafana: http://localhost:3000 (admin/admin)"
        echo ""
        echo "To view logs: docker compose logs -f"
        echo "To stop: docker compose down"
    else
        print_error "Docker not available. Please install Docker to use this option."
        exit 1
    fi
}

# Show usage
show_usage() {
    echo "Usage: $0 [OPTION]"
    echo ""
    echo "Options:"
    echo "  build       - Build all binaries"
    echo "  certgen     - Generate self-signed certificates"
    echo "  validate    - Validate configuration"
    echo "  start       - Start the proxy server"
    echo "  docker      - Start with Docker Compose"
    echo "  setup       - Complete setup (build, certgen, validate)"
    echo "  help        - Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 setup    # Complete setup"
    echo "  $0 start    # Start the proxy"
    echo "  $0 docker   # Start with Docker"
}

# Main script logic
main() {
    case "${1:-help}" in
        "build")
            check_go
            build_project
            ;;
        "certgen")
            check_go
            build_project
            generate_certificates
            ;;
        "validate")
            check_go
            build_project
            validate_config
            ;;
        "start")
            check_go
            build_project
            validate_config
            start_proxy
            ;;
        "docker")
            check_docker
            start_docker
            ;;
        "setup")
            check_go
            check_docker
            build_project
            generate_certificates
            validate_config
            print_success "Setup complete! You can now run:"
            echo "  $0 start    # Start the proxy"
            echo "  $0 docker   # Start with Docker"
            ;;
        "help"|*)
            show_usage
            ;;
    esac
}

# Run main function
main "$@" 