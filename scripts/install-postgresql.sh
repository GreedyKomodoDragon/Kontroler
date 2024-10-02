helm upgrade -i my-release oci://registry-1.docker.io/bitnamicharts/postgresql \
    --set tls.enabled=true \
    --set tls.certificatesSecret=postgresql-tls \
    --set tls.certFilename=postgresql.crt \
    --set tls.certKeyFilename=postgresql.key \
    -f values.yaml

# --set tls.certCAFilename=ca.crt \
