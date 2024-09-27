#!/bin/bash

# Define variables
OUTPUT_DIR="./certs"
SECRET_NAME="my-tls-secret" # Name for the Kubernetes secret
NAMESPACE="default"           # Namespace where the secret will be created

# Check if the necessary files exist
if [[ ! -f "$OUTPUT_DIR/ca.crt" || ! -f "$OUTPUT_DIR/ca.key" || ! -f "$OUTPUT_DIR/server.crt" || ! -f "$OUTPUT_DIR/server.key" ]]; then
    echo "One or more certificate files are missing. Please run the certificate generation script first."
    exit 1
fi

# Create the Kubernetes secret
echo "Creating Kubernetes secret..."

kubectl create secret tls "$SECRET_NAME" \
    --cert="$OUTPUT_DIR/server.crt" \
    --key="$OUTPUT_DIR/server.key" \
    --dry-run=client -o yaml > secret.yaml

echo "---" >> secret.yaml

# # Create a secret for the CA
kubectl create secret generic ca-secret \
    --from-file=ca.crt="$OUTPUT_DIR/ca.crt" \
    --from-file=ca.key="$OUTPUT_DIR/ca.key" \
    --dry-run=client -o yaml >> secret.yaml

echo "Done!"
