package rest

import (
	"kubeconductor-server/pkg/db"
	"kubeconductor-server/pkg/kube"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func NewFiberHttpServer(kubeClient kube.KubeClient, dbManager db.DbManager) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowCredentials: false,
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH",
	}))

	addV1(app, kubeClient, dbManager)

	return app
}
