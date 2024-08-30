package rest

import (
	"kubeconductor-server/pkg/auth"
	"kubeconductor-server/pkg/db"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"k8s.io/client-go/dynamic"
)

func NewFiberHttpServer(dbManager db.DbManager, kClient dynamic.Interface, authManager auth.AuthManager) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000",
		AllowCredentials: true,
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH",
	}))

	// Middleware for authentication
	// TODO: Make Authentication toggl-able
	app.Use(func(c *fiber.Ctx) error {
		return Authentication(c, authManager)
	})

	addV1(app, dbManager, kClient, authManager)

	return app
}
