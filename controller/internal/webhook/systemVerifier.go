package webhook

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
)

type systemURLVerifier struct {
	roots *x509.CertPool
}

func NewSystemURLValidator() SSLVerifier {
	// Load the system root pool
	roots, err := x509.SystemCertPool()
	if err != nil {
		// if failed to load system root pool, create a new one
		roots = x509.NewCertPool()
	}

	return &systemURLVerifier{
		roots: roots,
	}
}

func (r *systemURLVerifier) VerifySSL(inputURL string) error {
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}

	if parsedURL.Scheme != "https" {
		return fmt.Errorf("invalid URL scheme: %s, must be https", parsedURL.Scheme)
	}

	// If no port is provided, assume the default HTTPS port (443)
	if parsedURL.Port() == "" {
		parsedURL.Host = fmt.Sprintf("%s:443", parsedURL.Hostname())
	}

	// Dial the server using the correct host and port
	conn, err := tls.Dial("tcp", parsedURL.Host, &tls.Config{
		RootCAs: r.roots,
	})
	if err != nil {
		return fmt.Errorf("failed to establish TLS connection: %v", err)
	}
	defer conn.Close()

	if err := conn.VerifyHostname(parsedURL.Hostname()); err != nil {
		return fmt.Errorf("hostname verification failed: %v", err)
	}

	// If everything is fine, check that there are certificates
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("no peer certificates found")
	}

	return nil
}
