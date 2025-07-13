#!/bin/bash

# Generate a proper development certificate
# This script creates a self-signed certificate that can be trusted for development

set -e

CERT_DIR="./certs"
CERT_FILE="$CERT_DIR/cert.pem"
KEY_FILE="$CERT_DIR/key.pem"
CA_FILE="$CERT_DIR/ca.pem"
CA_KEY_FILE="$CERT_DIR/ca-key.pem"

# Create certs directory if it doesn't exist
mkdir -p "$CERT_DIR"

echo "Generating CA certificate..."
openssl req -x509 -newkey rsa:4096 -keyout "$CA_KEY_FILE" -out "$CA_FILE" -days 365 -nodes \
    -subj "/C=US/ST=Development/L=Development/O=Sentinel Development/OU=Development/CN=Sentinel Development CA"

echo "Generating server certificate..."
openssl req -newkey rsa:2048 -keyout "$KEY_FILE" -out "$CERT_DIR/server.csr" -nodes \
    -subj "/C=US/ST=Development/L=Development/O=Sentinel Development/OU=Development/CN=localhost"

echo "Creating certificate configuration..."
cat > "$CERT_DIR/server.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = 127.0.0.1
IP.1 = 127.0.0.1
EOF

echo "Signing server certificate with CA..."
openssl x509 -req -in "$CERT_DIR/server.csr" -CA "$CA_FILE" -CAkey "$CA_KEY_FILE" \
    -CAcreateserial -out "$CERT_FILE" -days 365 -extfile "$CERT_DIR/server.ext"

echo "Cleaning up temporary files..."
rm -f "$CERT_DIR/server.csr" "$CERT_DIR/server.ext" "$CERT_DIR/ca.srl"

echo "Certificate generation complete!"
echo "Certificate: $CERT_FILE"
echo "Private Key: $KEY_FILE"
echo "CA Certificate: $CA_FILE"
echo ""
echo "To trust this certificate for development:"
echo "sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain $CA_FILE"
echo ""
echo "Or use curl with the CA certificate:"
echo "curl --cacert $CA_FILE https://localhost:8443/api/v1" 