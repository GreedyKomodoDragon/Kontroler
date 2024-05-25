package rest

import (
	"kubeconductor-server/pkg/db"
	"kubeconductor-server/pkg/kube"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

func addV1(app *fiber.App, kubeClient kube.KubeClient, dbManager db.DbManager) {

	router := app.Group("/api/v1")

	addCronJob(router, dbManager)
	addCrds(router, kubeClient)
}

func addCronJob(router fiber.Router, dbManager db.DbManager) {
	cronJobRouter := router.Group("/single")

	cronJobRouter.Get("/cronjob", func(c *fiber.Ctx) error {

		cronJobs, err := dbManager.GetAllCronJobs(c.Context())
		if err != nil {
			log.Error().Err(err).Msg("Error getting cronjobs")
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
		runs, err := dbManager.GetAllRuns(c.Context())
		if err != nil {
			log.Error().Err(err).Msg("Error getting runs")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"runs": runs,
		})
	})

	cronJobRouter.Get("/run/:id", func(c *fiber.Ctx) error {
		return nil
	})
}

func addCrds(router fiber.Router, kubeClient kube.KubeClient) {
	crdsRouter := router.Group("/crd")

	crdsRouter.Get("/cronjob", func(c *fiber.Ctx) error {
		crds, err := kubeClient.GetAllCronJobCrds()
		if err != nil {
			log.Error().Err(err).Msg("Error getting crds")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"crds": crds,
		})
	})
}
