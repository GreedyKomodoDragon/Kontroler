package rest

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"kontroler-server/pkg/auth"
	"kontroler-server/pkg/db"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"k8s.io/client-go/dynamic"
)

func NewFiberHttpServer(dbManager db.DbManager, kClient dynamic.Interface, authManager auth.AuthManager, corsUiAddress string, auditLogs bool) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsUiAddress,
		AllowCredentials: true,
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH",
	}))

	if auditLogs {
		app.Use(AuditLoggerMiddleware())
	}

	// Middleware for authentication
	// TODO: Make Authentication toggl-able
	app.Use(func(c *fiber.Ctx) error {
		return Authentication(c, authManager)
	})

	addV1(app, dbManager, kClient, authManager)

	return app
}

func CreateTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair("/tls.crt", "/tls.key")
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %v", err)
	}

	caCert, err := os.ReadFile("/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Create base TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
	}

	if os.Getenv("INSECURE") == "true" {
		// In insecure mode
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
		tlsConfig.InsecureSkipVerify = true
		return tlsConfig, nil
	}

	// In secure mode (mTLS), client certificates are required
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	return tlsConfig, nil
}
