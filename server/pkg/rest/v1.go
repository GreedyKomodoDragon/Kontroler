package rest

import (
	"kubeconductor-server/pkg/db"
	"kubeconductor-server/pkg/kube"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/types"
)

func addV1(app *fiber.App, kubeClient kube.KubeClient, dbManager db.DbManager) {

	router := app.Group("/api/v1")

	addCronJob(router, dbManager)
	addCrds(router, kubeClient)
	addDags(router, dbManager)
}

func addDags(router fiber.Router, dbManager db.DbManager) {
	dagRouter := router.Group("/dag")

	dagRouter.Get("/meta/:page", func(c *fiber.Ctx) error {
		page, err := strconv.Atoi(c.Params("page"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		if page < 1 {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		dags, err := dbManager.GetAllDagMetaData(c.Context(), 10, (page-1)*10)
		if err != nil {
			log.Error().Err(err).Msg("Error getting dags")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"dags": dags,
		})
	})

	dagRouter.Get("/runs/:page", func(c *fiber.Ctx) error {
		page, err := strconv.Atoi(c.Params("page"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		if page < 1 {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		dags, err := dbManager.GetDagRuns(c.Context(), 10, (page-1)*10)
		if err != nil {
			log.Error().Err(err).Msg("Error getting dag runs")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(dags)
	})

	dagRouter.Get("/run/:id", func(c *fiber.Ctx) error {
		id, err := strconv.Atoi(c.Params("id"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		dagRun, err := dbManager.GetDagRun(c.Context(), id)
		if err != nil {
			log.Error().Err(err).Msg("Error getting dag run")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(dagRun)
	})

	dagRouter.Get("/run/all/:id", func(c *fiber.Ctx) error {
		id, err := strconv.Atoi(c.Params("id"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		dagRun, err := dbManager.GetDagRunAll(c.Context(), id)
		if err != nil {
			log.Error().Err(err).Msg("Error getting dag run all")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(dagRun)
	})

	dagRouter.Get("/run/task/:runId/:taskId", func(c *fiber.Ctx) error {
		runId, err := strconv.Atoi(c.Params("runId"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		taskId, err := strconv.Atoi(c.Params("taskId"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		taskDetails, err := dbManager.GetTaskRunDetails(c.Context(), runId, taskId)
		if err != nil {
			log.Error().Err(err).Msg("Error getting GetTaskDetails")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(taskDetails)
	})

	dagRouter.Get("/task/:taskId", func(c *fiber.Ctx) error {
		taskId, err := strconv.Atoi(c.Params("taskId"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		taskDetails, err := dbManager.GetTaskDetails(c.Context(), taskId)
		if err != nil {
			log.Error().Err(err).Msg("Error getting GetTaskDetails")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(taskDetails)
	})

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

	cronJobRouter.Get("/run/:page", func(c *fiber.Ctx) error {
		page, err := strconv.Atoi(c.Params("page"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		if page < 1 {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		runs, err := dbManager.GetAllRuns(c.Context(), 10, (page-1)*10)
		if err != nil {
			log.Error().Err(err).Msg("Error getting runs")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"runs": runs,
		})
	})

	cronJobRouter.Get("/run/:id/pods", func(c *fiber.Ctx) error {
		id := c.Params("id")
		if id == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		pods, err := dbManager.GetRunsPods(c.Context(), types.UID(id))
		if err != nil {
			log.Error().Err(err).Msg("Error getting runs")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"pods": pods,
		})
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
