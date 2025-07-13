# Multi-stage build for Sentinel reverse proxy
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build all binaries
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sentinel cmd/proxy/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o validator cmd/validator/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o certgen cmd/certgen/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S sentinel && \
    adduser -u 1001 -S sentinel -G sentinel

# Set working directory
WORKDIR /app

# Copy binaries from builder stage
COPY --from=builder /app/sentinel /app/validator /app/certgen ./

# Create necessary directories
RUN mkdir -p /app/config /app/certs && \
    chown -R sentinel:sentinel /app

# Switch to non-root user
USER sentinel

# Expose ports
EXPOSE 8080 8443 8081 8082

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8081/health || exit 1

# Default command
CMD ["./sentinel", "-config", "/app/config"] 