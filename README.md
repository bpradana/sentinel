# Sentinel - High-Performance Reverse Proxy

Sentinel is a modern, high-performance reverse proxy written in Go. It provides advanced load balancing, health checking, TLS termination, middleware support, and comprehensive monitoring capabilities.

## 📚 Table of Contents

- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Configuration](#-configuration)
- [TLS & Certificates](#-tls--certificates)
- [Command Line Tools](#-command-line-tools)
- [Load Balancing Strategies](#-load-balancing-strategies)
- [Middleware](#-middleware)
- [Monitoring](#-monitoring)
- [Hot Reload](#-hot-reload)
- [Production Deployment](#-production-deployment)
- [Troubleshooting](#-troubleshooting)
- [Contributing](#-contributing)
- [License](#-license)
- [Support](#-support)
- [Related Projects](#-related-projects)

## 🚀 Features

- **Load Balancing**: Round-robin, least connections, and IP hash strategies
- **Health Checking**: Configurable health checks for upstream services
- **TLS Termination**: Support for manual certificates and Let's Encrypt auto-cert
- **Middleware**: Logging, rate limiting, authentication, and compression
- **Monitoring**: Prometheus metrics and health endpoints
- **Hot Reload**: Configuration reloading without downtime
- **HTTP/2 Support**: Native HTTP/2 support for improved performance

## 📦 Installation

### Prerequisites

- Go 1.23.8 or later
- Git

### Build from Source

```bash
# Clone the repository
git clone https://github.com/bpradana/sentinel.git
cd sentinel

# Build all binaries
go build -o bin/sentinel cmd/proxy/main.go
go build -o bin/validator cmd/validator/main.go
go build -o bin/certgen cmd/certgen/main.go
```

## 🛠️ Quick Start

### 1. Generate Self-Signed Certificates (Development)

You can either manually generate self-signed certificates or let Sentinel auto-generate them on startup (see [TLS & Certificates](#-tls--certificates)).

**Manual generation:**

```bash
# Generate certificates for localhost
./bin/certgen -hosts "localhost,127.0.0.1" -output ./certs

# This will create:
# - ./certs/cert.pem
# - ./certs/key.pem
# - ./certs/tls-example.yaml
```

**Auto-generation:**

If you configure `auto_generate: true` and `self_signed: true` in your `tls.yaml`, Sentinel will automatically generate self-signed certificates for the specified hosts if the certificate files do not exist.

### 2. Validate Configuration

```bash
# Validate your configuration
./bin/validator -config ./config -verbose

# This will check all configuration files and provide a summary
```

### 3. Start the Proxy

```bash
# Start with default configuration
./bin/sentinel -config ./config

# Start with custom log level
./bin/sentinel -config ./config -log-level debug
```

## 📁 Configuration

Sentinel uses YAML configuration files organized in a directory structure:

```
config/
├── global.yaml      # Server and logging settings
├── upstreams.yaml   # Upstream service definitions
├── routes.yaml      # Routing rules
├── middleware.yaml  # Middleware configuration
├── tls.yaml        # TLS settings
├── health.yaml     # Health check settings
└── metrics.yaml    # Metrics configuration
```

### Configuration Examples

#### Global Settings (`global.yaml`)

```yaml
server:
  http_port: 8080
  https_port: 8443
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s
  max_header_size: 1048576  # 1MB
  http2_enabled: true

log:
  level: "info"
  format: "json"
```

#### Upstream Services (`upstreams.yaml`)

```yaml
services:
  api-service:
    load_balancer: "round_robin"
    health_check:
      enabled: true
      path: "/health"
      interval: 30s
      timeout: 5s
      failure_threshold: 3
      success_threshold: 2
    targets:
      - url: "http://localhost:3001"
        weight: 1
      - url: "http://localhost:3002"
        weight: 1
```

#### Routes (`routes.yaml`)

```yaml
rules:
  - host: "localhost"
    path: "/api/v1"
    methods: ["GET", "POST", "PUT", "DELETE"]
    upstream: "api-service"
    middleware: ["logging", "rate_limit"]
    headers:
      X-API-Version: "v1"
    timeout: 30s
    retry_policy:
      attempts: 3
      backoff: 1s
```

## 🔐 TLS & Certificates

Sentinel supports flexible TLS configuration, including manual certificates, Let's Encrypt (autocert), and automatic self-signed certificate generation for development and CI environments.

### TLS Configuration Example (`tls.yaml`)

```yaml
enabled: true

autocert:
  enabled: false
  email: "admin@example.com"
  hosts:
    - "localhost"
    - "api.example.com"
  cache_dir: "./certs"
  staging: true

certificates:
  - hosts:
      - "localhost"
      - "127.0.0.1"
    auto_generate: true      # <--- Enable auto-generation
    self_signed: true        # <--- Use self-signed certs
    valid_for: "8760h"      # <--- (Optional) Validity duration
    rsa_bits: 2048          # <--- (Optional) Key size
    common_name: "localhost" # <--- (Optional) Common Name
    organization: "Sentinel Inc." # <--- (Optional)
    cert_file: "./certs/localhost-cert.pem"
    key_file: "./certs/localhost-key.pem"
```

#### Auto-Generate Self-Signed Certificates

- If `auto_generate: true` and `self_signed: true` are set, Sentinel will create a new self-signed certificate for the specified hosts if the files do not exist.
- This is ideal for local development, CI pipelines, and ephemeral environments.
- You can customize validity, key size, and subject fields.
- If the certificate files already exist, Sentinel will use them as-is.

#### Manual Certificate Management

- Omit `auto_generate` or set it to `false` to use existing certificates only.
- You can use the provided `certgen` tool to generate certificates manually.

#### Let's Encrypt (Autocert)

- Enable the `autocert` section for automatic Let's Encrypt certificate management in production.

## 🔧 Command Line Tools

### Configuration Validator

Validate your configuration without running the proxy:

```bash
./bin/validator -config ./config -verbose
```

Options:
- `-config`: Configuration directory (default: `./config`)
- `-log-level`: Log level (default: `info`)
- `-verbose`: Enable detailed configuration summary

### Certificate Generator

Generate self-signed certificates for development:

```bash
./bin/certgen -hosts "localhost,127.0.0.1" -output ./certs
```

Options:
- `-hosts`: Comma-separated list of hosts
- `-output`: Output directory (default: `./certs`)
- `-days`: Certificate validity in days (default: `365`)
- `-key-size`: RSA key size in bits (default: `2048`)
- `-cn`: Common name for certificate
- `-org`: Organization name
- `-country`: Country code
- `-state`: State or province
- `-city`: City

## 🔄 Load Balancing Strategies

Sentinel supports three load balancing strategies:

1. **Round Robin** (`round_robin`): Distributes requests evenly across targets
2. **Least Connections** (`least_connections`): Routes to the target with the fewest active connections
3. **IP Hash** (`ip_hash`): Routes based on client IP address for session affinity

## 🔒 Middleware

### Available Middleware

1. **Logging**: Request/response logging with configurable detail level
2. **Rate Limiting**: Per-client rate limiting with burst support
3. **Authentication**: JWT-based authentication with public path exclusions
4. **Compression**: Gzip compression for supported content types

### Middleware Configuration

```yaml
chain:
  - name: "logging"
    type: "logging"
    enabled: true
    order: 1
    config:
      log_requests: true
      log_responses: true
      log_headers: false
      log_body: false

  - name: "rate_limit"
    type: "rate_limit"
    enabled: true
    order: 2
    config:
      requests_per_second: 100
      burst: 50
      key_func: "ip"
```

## 📊 Monitoring

### Health Checks

Health check endpoint available at `http://localhost:8081/health` (configurable port).

### Metrics

Prometheus metrics available at `http://localhost:8082/metrics` (configurable port).

Key metrics:
- `sentinel_requests_total`: Total number of requests
- `sentinel_request_duration_seconds`: Request duration
- `sentinel_upstream_health_status`: Upstream health status
- `sentinel_active_connections`: Active connections

## 🔄 Hot Reload

Sentinel supports configuration hot reloading. When configuration files are modified, the proxy will automatically reload the configuration without downtime.

## 🚀 Production Deployment

### Docker

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o sentinel cmd/proxy/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/sentinel .
COPY config/ ./config/
EXPOSE 8080 8443
CMD ["./sentinel"]
```

### Systemd Service

```ini
[Unit]
Description=Sentinel Reverse Proxy
After=network.target

[Service]
Type=simple
User=sentinel
WorkingDirectory=/opt/sentinel
ExecStart=/opt/sentinel/sentinel -config /opt/sentinel/config
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## 🔧 Troubleshooting

### Common Issues

1. **Port already in use**: Check if ports 8080/8443 are available
2. **Certificate errors**:
   - If using auto-generation, ensure Sentinel has write permissions to the cert/key file paths.
   - If using manual certificates, ensure certificate files exist and are readable.
3. **Upstream connection failures**: Verify upstream services are running
4. **Configuration validation errors**: Use the validator tool to check configuration

### Debug Mode

Run with debug logging for detailed information:

```bash
./bin/sentinel -config ./config -log-level debug
```

### Health Check

Check if the proxy is running:

```bash
curl http://localhost:8081/health
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Issues**: Use GitHub issues for bug reports and feature requests
- **Documentation**: Check this README and inline code comments
- **Configuration**: Use the validator tool to check your configuration

## 🔗 Related Projects

- [Traefik](https://traefik.io/) - Modern HTTP reverse proxy
- [Nginx](https://nginx.org/) - High-performance web server
- [HAProxy](https://www.haproxy.org/) - Reliable load balancer 