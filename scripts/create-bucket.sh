#!/bin/bash

# Script to create a bucket in MinIO using AWS CLI
# Ensure you have the AWS CLI and kubectl installed and configured before running this script

# Variables
ENDPOINT_URL="http://localhost:9000"  # Change this to your MinIO server URL
SECRET_NAME="minio"           # Name of the Kubernetes Secret containing MinIO credentials
NAMESPACE="default"                   # Namespace where the secret is located
BUCKET_NAME="kontroler"               # Bucket name to be created

# Check if bucket name is passed as an argument
if [ -z "$1" ]; then
  echo "Usage: $0 <bucket-name>"
  exit 1
else
  BUCKET_NAME="$1"
fi

# Check if AWS CLI and kubectl are installed
if ! command -v aws &> /dev/null; then
  echo "Error: AWS CLI is not installed. Please install it first."
  exit 1
fi

if ! command -v kubectl &> /dev/null; then
  echo "Error: kubectl is not installed. Please install it first."
  exit 1
fi

# Retrieve MinIO credentials from Kubernetes Secret
ACCESS_KEY=$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.data.root-user}' | base64 --decode)
SECRET_KEY=$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.data.root-password}' | base64 --decode)

# Check if credentials were retrieved successfully
if [ -z "$ACCESS_KEY" ] || [ -z "$SECRET_KEY" ]; then
  echo "Error: Failed to retrieve MinIO credentials from Kubernetes Secret."
  exit 1
fi

# Export MinIO credentials
export AWS_ACCESS_KEY_ID="$ACCESS_KEY"
export AWS_SECRET_ACCESS_KEY="$SECRET_KEY"

# Create the bucket
aws --endpoint-url="$ENDPOINT_URL" s3 mb "s3://$BUCKET_NAME"

# Verify bucket creation
if [ $? -eq 0 ]; then
  echo "Bucket '$BUCKET_NAME' created successfully at endpoint '$ENDPOINT_URL'."
else
  echo "Failed to create bucket '$BUCKET_NAME'."
  exit 1
fi
