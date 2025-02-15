#!/bin/bash

export SECRET_NAME="minio"

# Set your MinIO credentials and configuration
export AWS_ACCESS_KEY_ID=$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.data.root-user}' | base64 --decode)
export AWS_SECRET_ACCESS_KEY=$(kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" -o jsonpath='{.data.root-password}' | base64 --decode)

export AWS_DEFAULT_REGION="eu-west-2"  # MinIO typically doesn't use region, but this is required by AWS CLI

MINIO_ENDPOINT="http://localhost:9000"  # Replace with your MinIO server endpoint
BUCKET_NAME="kontroler"

# Configure the AWS CLI to use MinIO as an S3-compatible endpoint
aws configure set default.s3.signature_version s3v4
aws configure set default.region "$AWS_DEFAULT_REGION"

# List and delete all objects in the bucket
aws --endpoint-url "$MINIO_ENDPOINT" s3 rm s3://"$BUCKET_NAME" --recursive

# Optionally, you can also remove the bucket itself if needed
# aws --endpoint-url "$MINIO_ENDPOINT" s3 rb s3://"$BUCKET_NAME" --force
