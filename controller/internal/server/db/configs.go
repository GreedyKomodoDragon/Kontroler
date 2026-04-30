package db

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
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

func ConfigurePostgres() (*pgxpool.Config, error) {
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		return nil, fmt.Errorf("missing DB_NAME")
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		return nil, fmt.Errorf("missing DB_USER")
	}

	pgEndpoint := os.Getenv("DB_ENDPOINT")
	if dbUser == "" {
		return nil, fmt.Errorf("missing DB_ENDPOINT")
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		return nil, fmt.Errorf("missing DB_PASSWORD")
	}

	sslMode, exists := os.LookupEnv("DB_SSL_MODE")
	if !exists {
		sslMode = "disable"
	}

	pgConfig, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s", dbUser, dbPassword, pgEndpoint, dbName, sslMode))
	if err != nil {
		return nil, err
	}

	pgConfig.ConnConfig.TLSConfig = &tls.Config{}
	if sslMode != "disable" {
		if err := UpdateDBSSLConfig(pgConfig.ConnConfig.TLSConfig); err != nil {
			panic(err)
		}

		if sslMode == "require" {
			pgConfig.ConnConfig.TLSConfig.InsecureSkipVerify = true
		} else if sslMode == "verify-ca" || sslMode == "verify-full" {
			pgConfig.ConnConfig.TLSConfig.InsecureSkipVerify = false
		}
	}

	return pgConfig, nil
}

func ConfigureSqlite() (*SQLiteReadOnlyConfig, error) {
	config := &SQLiteReadOnlyConfig{}

	config.DBPath = os.Getenv("SQLITE_PATH")
	if config.DBPath == "" {
		return nil, fmt.Errorf("missing SQLITE_PATH")
	}

	config.Synchronous = os.Getenv("SQLITE_SYNCHRONOUS")
	if config.Synchronous == "" {
		config.Synchronous = "NORMAL"
	}

	cacheSize := os.Getenv("SQLITE_CACHE_SIZE")
	if cacheSize == "" {
		config.CacheSize = -2000
	}

	return config, nil
}
