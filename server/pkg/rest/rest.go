package rest

import (
	"kubeconductor-server/pkg/db"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"k8s.io/client-go/dynamic"
)

func NewFiberHttpServer(dbManager db.DbManager, kClient dynamic.Interface) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowCredentials: false,
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH",
	}))

	addV1(app, dbManager, kClient)

	return app
}
