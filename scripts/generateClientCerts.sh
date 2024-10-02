#!/bin/bash

CLIENT_CERT=client.crt
CA_CERT=ca.crt
CLIENT_KEY=client.key

# Generate client private key
openssl genrsa -out $CLIENT_KEY 2048

# Generate client CSR with CN=postgres
openssl req -new -key $CLIENT_KEY -out client.csr -subj "/CN=postgres"

# Sign the client CSR with the CA certificate
openssl x509 -req -in client.csr -CA $CA_CERT -CAkey $CLIENT_KEY -CAcreateserial \
  -out $CLIENT_CERT -days 365 -sha256

# Create Kubernetes secret containing the CA and PostgreSQL certs
kubectl delete secret postgresql-client-tls
kubectl create secret generic postgresql-client-tls \
  --from-file=ca.crt=ca.crt \
  --from-file=$CLIENT_CERT=$CLIENT_CERT \
  --from-file=$CLIENT_KEY=$CLIENT_KEY
