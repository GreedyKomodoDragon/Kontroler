#!/bin/bash

DOMAIN="localhost"
DAYS_CA=3650      
DAYS_CERT=365     
KEY_SIZE=2048
OUTPUT_DIR="./certs"

# Create directories for CA and certs
mkdir -p "$OUTPUT_DIR"

# Generate a private key for the CA
echo "Generating CA private key..."
openssl genrsa -out "$OUTPUT_DIR/ca.key" $KEY_SIZE

# Generate a self-signed certificate for the CA
echo "Generating CA certificate..."
openssl req -x509 -new -nodes -key "$OUTPUT_DIR/ca.key" -sha256 -days $DAYS_CA -out "$OUTPUT_DIR/ca.crt" -subj "/C=US/ST=State/L=City/O=Organization/OU=Unit/CN=My Root CA"

# Generate a private key for the server
echo "Generating server private key..."
openssl genrsa -out "$OUTPUT_DIR/server.key" $KEY_SIZE

# Generate a Certificate Signing Request (CSR) for the server
echo "Generating server CSR..."
openssl req -new -key "$OUTPUT_DIR/server.key" -out "$OUTPUT_DIR/server.csr" -subj "/C=UK/ST=State/L=City/O=Organization/OU=Unit/CN=$DOMAIN"

# Sign the server CSR with the CA certificate
echo "Signing server certificate with CA..."
openssl x509 -req -in "$OUTPUT_DIR/server.csr" -CA "$OUTPUT_DIR/ca.crt" -CAkey "$OUTPUT_DIR/ca.key" -CAcreateserial -out "$OUTPUT_DIR/server.crt" -days $DAYS_CERT -sha256

# Clean up CSR
rm "$OUTPUT_DIR/server.csr"

echo "Self-signed certificate generated successfully!"
