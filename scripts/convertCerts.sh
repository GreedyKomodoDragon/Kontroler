openssl pkcs12 -export \
    -in certs/server.crt \
    -inkey certs/server.key \
    -certfile certs/ca.crt \
    -out client-cert.p12
