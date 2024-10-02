package db

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

func UpdateDBSSLConfig(tlsConfig *tls.Config) error {
	// Load CA cert
	if caCertPath, _ := os.LookupEnv("DB_SSL_CA_CERT"); caCertPath != "" {
		rootCertPool := x509.NewCertPool()
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return err
		}
		rootCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = rootCertPool
	}

	// Load client certificate and key (for mTLS)
	clientCertPath, _ := os.LookupEnv("DB_SSL_CERT")
	clientKeyPath, _ := os.LookupEnv("DB_SSL_KEY")

	if clientCertPath != "" && clientKeyPath != "" {
		clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			return err
		}

		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return nil
}
