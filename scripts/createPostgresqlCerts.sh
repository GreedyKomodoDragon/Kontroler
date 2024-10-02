# Generate CA private key
openssl genrsa -out ca.key 2048

# Generate CA certificate (for 1 year)
openssl req -x509 -new -nodes -key ca.key -sha256 -days 365 -out ca.crt -subj "/CN=postgres-ca"

# --------------------- PostgreSQL Server Certificate ---------------------

# Create a private key for PostgreSQL
openssl genrsa -out postgresql.key 2048

# Generate a Certificate Signing Request (CSR) for PostgreSQL (hostname must match server's FQDN)
openssl req -new -key postgresql.key -out postgresql.csr -subj "/CN=my-release-postgresql.default.svc.cluster.local"

# Sign the PostgreSQL CSR with the CA certificate
openssl x509 -req -in postgresql.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out postgresql.crt -days 365 -sha256

# Clean up the CSR
rm postgresql.csr

# Create Kubernetes secret containing the CA and PostgreSQL server certs
kubectl delete secret postgresql-tls || true
kubectl create secret generic postgresql-tls \
  --from-file=ca.crt=ca.crt \
  --from-file=postgresql.crt=postgresql.crt \
  --from-file=postgresql.key=postgresql.key

# --------------------- Client Certificate ---------------------

# Generate client private key
openssl genrsa -out client.key 2048

# Generate a Certificate Signing Request (CSR) for the client (CN must match the PostgreSQL username)
openssl req -new -key client.key -out client.csr -subj "/CN=postgres"

# Sign the client CSR with the CA certificate
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt -days 365 -sha256

# Clean up the CSR
rm client.csr

# Create Kubernetes secret containing the CA and client certs
kubectl delete secret postgresql-client-tls || true
kubectl create secret generic postgresql-client-tls \
  --from-file=ca.crt=ca.crt \
  --from-file=client.crt=client.crt \
  --from-file=client.key=client.key

echo "Certificates generated and secrets created successfully."
