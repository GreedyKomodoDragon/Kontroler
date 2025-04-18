package rest

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"kontroler-server/internal/auth"
	"kontroler-server/internal/db"
	"kontroler-server/internal/logs"
	"os"
	"unicode"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"k8s.io/client-go/dynamic"
)

func NewFiberHttpServer(dbManager db.DbManager, kClient dynamic.Interface, authManager auth.AuthManager, corsUiAddress string, auditLogs bool, logFetcher logs.LogFetcher) *fiber.App {
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

	addV1(app, dbManager, kClient, authManager, logFetcher)

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

func ValidateCredentials(req auth.CreateAccountReq) error {
	if len(req.Username) < 3 || len(req.Username) > 100 {
		return fmt.Errorf("username must be between 3 and 100 characters long")
	}

	if !unicode.IsLetter(rune(req.Username[0])) {
		return fmt.Errorf("username must start with a letter")
	}

	for _, r := range req.Username {
		if !(unicode.IsLetter(r) || unicode.IsNumber(r)) {
			return fmt.Errorf("username must use only letter or number characters")
		}
	}

	if len(req.Password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}

	hasLetter := false
	for _, r := range req.Password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if !(unicode.IsLetter(r) || unicode.IsNumber(r)) {
			return fmt.Errorf("password must use only letter or number characters")
		}
	}

	if !hasLetter {
		return fmt.Errorf("password must contain at least one letter")
	}

	if req.Role != "admin" && req.Role != "editor" && req.Role != "viewer" {
		return fmt.Errorf("role must be either admin, editor, or viewer")
	}

	return nil
}
