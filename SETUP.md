# Sentinel Setup Guide

This guide explains the tools and features that have been added to the Sentinel reverse proxy project.

## 🛠️ New Tools Added

### 1. Configuration Validator (`cmd/validator/main.go`)

A standalone tool to validate your configuration without running the proxy.

**Features:**
- Validates all configuration files
- Checks for syntax errors and logical inconsistencies
- Provides detailed configuration summary
- Supports verbose output for debugging

**Usage:**
```bash
# Basic validation
./bin/validator -config ./config

# Verbose validation with configuration summary
./bin/validator -config ./config -verbose
```

### 2. Self-Signed Certificate Generator (`cmd/certgen/main.go`)

A tool to generate self-signed certificates for development and testing.

**Features:**
- Generates RSA private keys and X.509 certificates
- Supports multiple hosts (DNS names and IP addresses)
- Configurable certificate validity and key size
- Automatic certificate validation
- Generates example TLS configuration

**Usage:**
```bash
# Generate certificates for localhost
./bin/certgen -hosts "localhost,127.0.0.1" -output ./certs

# Custom certificate settings
./bin/certgen -hosts "example.com,api.example.com" -days 730 -key-size 4096 -output ./certs
```

### 3. Makefile

A comprehensive Makefile for building and managing the project.

**Key targets:**
- `make build` - Build all binaries
- `make validate` - Validate configuration
- `make certgen` - Generate certificates
- `make run` - Start the proxy
- `make setup` - Complete setup (build, certgen, validate)
- `make docker-build` - Build Docker image
- `make docker-run` - Run Docker container

**Usage:**
```bash
# Complete setup
make setup

# Start the proxy
make run

# Build for production
make prod-build
```

### 4. Quick Start Script (`scripts/quick-start.sh`)

A bash script for easy setup and deployment.

**Features:**
- Automated setup process
- Docker support
- Colored output
- Error handling
- Multiple deployment options

**Usage:**
```bash
# Complete setup
./scripts/quick-start.sh setup

# Start with Docker
./scripts/quick-start.sh docker

# Start the proxy
./scripts/quick-start.sh start
```

## 🐳 Docker Support

### Dockerfile

Multi-stage Dockerfile for containerizing Sentinel.

**Features:**
- Multi-stage build for smaller images
- Non-root user for security
- Health checks
- Alpine Linux base for minimal size

### Docker Compose

Complete development environment with monitoring stack.

**Services:**
- Sentinel proxy
- Example upstream services (API, Web, Static)
- Prometheus for metrics
- Grafana for visualization

**Usage:**
```bash
# Start all services
docker compose up --build -d

# View logs
docker compose logs -f

# Stop services
docker compose down
```

## 📚 Documentation

### Comprehensive README.md

Complete documentation covering:
- Features and capabilities
- Installation instructions
- Configuration examples
- Command-line tools
- Load balancing strategies
- Middleware configuration
- Monitoring and metrics
- Production deployment
- Troubleshooting guide

### Configuration Examples

Detailed examples for all configuration files:
- Global settings
- Upstream services
- Routing rules
- TLS configuration
- Middleware chains
- Health checks
- Metrics

## 🔧 Development Tools

### Test Services

Example nginx configurations for testing:
- `test/nginx-api-1.conf` - First API service
- `test/nginx-api-2.conf` - Second API service
- `test/nginx-web-1.conf` - First web service
- `test/nginx-web-2.conf` - Second web service
- `test/nginx-static.conf` - Static content service

### Monitoring Stack

Complete monitoring setup:
- `monitoring/prometheus.yml` - Prometheus configuration
- Docker Compose services for Prometheus and Grafana

## 🚀 Getting Started

### Option 1: Quick Start Script

```bash
# Clone the repository
git clone <repository-url>
cd sentinel

# Complete setup
./scripts/quick-start.sh setup

# Start the proxy
./scripts/quick-start.sh start
```

### Option 2: Makefile

```bash
# Build and setup
make setup

# Start the proxy
make run
```

### Option 3: Docker

```bash
# Start with Docker Compose
docker compose up --build -d

# Access services
# - Proxy: http://localhost:8080
# - HTTPS: https://localhost:8443
# - Health: http://localhost:8081/health
# - Metrics: http://localhost:8082/metrics
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000
```

## 🔍 Validation Examples

### Configuration Validation

```bash
$ ./bin/validator -config ./config -verbose
🔍 Sentinel Configuration Validator
====================================
📁 Validating configuration in: ./config

✅ Configuration files loaded successfully
✅ Configuration validation passed

📊 Configuration Summary:
------------------------
🌐 Global Settings:
  HTTP Port: 8080
  HTTPS Port: 8443
  Read Timeout: 30s
  Write Timeout: 30s
  Idle Timeout: 1m0s
  HTTP/2 Enabled: true
  Log Level: info
  Log Format: json

🔄 Upstream Services (4):
  api-service:
    Load Balancer: round_robin
    Targets: 2
    Health Check: true
  web-service:
    Load Balancer: least_connections
    Targets: 2
    Health Check: true
  static-service:
    Load Balancer: ip_hash
    Targets: 1
    Health Check: false
  serviceA:
    Load Balancer: round_robin
    Targets: 1
    Health Check: true

🛣️  Routes (4):
  1. localhost/api/v1 -> api-service
  2. localhost/ -> web-service
  3. localhost/static -> static-service
  4. localhost/service/A/transactions -> serviceA

🔧 Middleware Chains (4):
  logging (logging) - Order: 1
  rate_limit (rate_limit) - Order: 2
  compression (compression) - Order: 4

🔒 TLS Configuration:
  Enabled: true
  Auto-cert: false
  Manual Certificates: 1

💚 Health Check:
  Enabled: true
  Port: 8081
  Interval: 30s
  Timeout: 5s

📈 Metrics:
  Enabled: true
  Port: 8082
  Path: /metrics

🎉 All validations passed! Your configuration is ready to use.
```

### Certificate Generation

```bash
$ ./bin/certgen -hosts "localhost,127.0.0.1" -output ./certs
🔐 Sentinel Self-Signed Certificate Generator
=============================================
📋 Generating certificate for hosts: localhost, 127.0.0.1
📁 Output directory: ./certs
⏰ Validity: 365 days

🔑 Generating RSA private key...
📜 Creating certificate...
🔍 Validating generated certificate...

✅ Certificate generated successfully!
📄 Certificate: certs/cert.pem
🔑 Private Key: certs/key.pem
⏰ Valid until: 2026-07-14 00:28:09

📝 Next steps:
1. Update your TLS configuration to use these certificates
2. Add the certificate files to your .gitignore
3. For production, use proper CA-signed certificates
📄 Example TLS config: certs/tls-example.yaml
```

## 🎯 Key Benefits

1. **Easy Setup**: One-command setup with validation
2. **Development Ready**: Self-signed certificates and test services
3. **Production Ready**: Docker support and monitoring
4. **Comprehensive Validation**: Catch configuration errors early
5. **Documentation**: Complete guides and examples
6. **Monitoring**: Built-in metrics and health checks

## 🔧 Next Steps

1. **Customize Configuration**: Modify the YAML files in `config/` for your needs
2. **Add Upstream Services**: Update `upstreams.yaml` with your services
3. **Configure Routes**: Update `routes.yaml` with your routing rules
4. **Set Up Monitoring**: Configure Prometheus and Grafana for production
5. **Security**: Replace self-signed certificates with proper CA certificates
6. **Scaling**: Configure load balancing and health checks for your services

## 🆘 Support

- **Configuration Issues**: Use the validator tool to check your config
- **Certificate Issues**: Use the certgen tool to generate new certificates
- **Docker Issues**: Check the docker compose logs
- **Monitoring**: Access Grafana at http://localhost:3000 (admin/admin)

The Sentinel project now provides a complete, production-ready reverse proxy solution with comprehensive tooling for development, testing, and deployment. 