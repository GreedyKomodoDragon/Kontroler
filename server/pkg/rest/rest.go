package rest

import (
	"kontroler-server/pkg/auth"
	"kontroler-server/pkg/db"

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
