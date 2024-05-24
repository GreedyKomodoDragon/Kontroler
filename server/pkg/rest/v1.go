package rest

import (
	"kubeconductor-server/pkg/db"

	"github.com/gofiber/fiber/v2"
)

func addV1(app *fiber.App, dbManager db.DbManager) {

	router := app.Group("/api/v1")

	addCronJob(router, dbManager)
	addCrds(router)
}

func addCronJob(router fiber.Router, dbManager db.DbManager) {
	cronJobRouter := router.Group("/single")

	cronJobRouter.Get("/cronjob", func(c *fiber.Ctx) error {

		cronJobs, err := dbManager.GetAllCronJobs(c.Context())
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"cronJobs": cronJobs,
		})
	})

	cronJobRouter.Get("/cronjob/:id", func(c *fiber.Ctx) error {
		return nil
	})

	cronJobRouter.Get("/cronjob/:id/runs", func(c *fiber.Ctx) error {
		return nil
	})

	cronJobRouter.Get("/run", func(c *fiber.Ctx) error {
		return nil
	})

	cronJobRouter.Get("/run/:id", func(c *fiber.Ctx) error {
		return nil
	})
}

func addCrds(router fiber.Router) {
	crdsRouter := router.Group("/crd")

	crdsRouter.Get("/cronjob", func(c *fiber.Ctx) error {
		return nil
	})
}
