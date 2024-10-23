package rest

import (
	"fmt"
	"kontroler-server/pkg/auth"
	"kontroler-server/pkg/db"
	kclient "kontroler-server/pkg/kClient"
	"kontroler-server/pkg/logs"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/dynamic"
)

func addV1(app *fiber.App, dbManager db.DbManager, kubClient dynamic.Interface, authManager auth.AuthManager, logFetcher logs.LogFetcher) {

	router := app.Group("/api/v1")

	addDags(router, dbManager, kubClient)
	addStats(router, dbManager)
	addAccountAuth(router, authManager)

	// check if a bucket has been selected/log fetching enabled
	if logFetcher != nil {
		addLogs(router, logFetcher)
	}
}

func addDags(router fiber.Router, dbManager db.DbManager, kubClient dynamic.Interface) {
	dagRouter := router.Group("/dag")

	dagRouter.Get("/names", func(c *fiber.Ctx) error {
		term := c.Query("term")
		if term == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		names, err := dbManager.GetDagNames(c.Context(), term, 10)
		if err != nil {
			log.Error().Err(err).Msg("Error getting dags")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"names": names,
		})
	})

	dagRouter.Get("/parameters", func(c *fiber.Ctx) error {
		name := c.Query("name")
		if name == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		parameters, err := dbManager.GetDagParameters(c.Context(), name)
		if err != nil {
			log.Error().Err(err).Msg("Error getting parameters")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"parameters": parameters,
		})
	})

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

	dagRouter.Get("/pages/count", func(c *fiber.Ctx) error {
		pageCount, err := dbManager.GetDagPageCount(c.Context(), 10)
		if err != nil {
			log.Error().Err(err).Msg("Error getting dag page count")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"count": pageCount,
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

	dagRouter.Post("/create", func(c *fiber.Ctx) error {
		var dagForm kclient.DagFormObj
		if err := c.BodyParser(&dagForm); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "cannot parse JSON",
			})
		}

		if err := kclient.CreateDAG(c.Context(), dagForm, kubClient); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to create DAG: %v", err),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "DAG created successfully",
		})

	})

	dagRouter.Get("/run/pages/count", func(c *fiber.Ctx) error {
		pageCount, err := dbManager.GetDagRunPageCount(c.Context(), 10)
		if err != nil {
			log.Error().Err(err).Msg("Error getting dag run page count")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"count": pageCount,
		})
	})

	dagRouter.Post("/run/create", func(c *fiber.Ctx) error {
		var dagrunForm kclient.DagRunForm
		if err := c.BodyParser(&dagrunForm); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "cannot parse JSON",
			})
		}

		keys := make([]string, 0, len(dagrunForm.Parameters))
		for k := range dagrunForm.Parameters {
			keys = append(keys, k)
		}

		isSecretMap, err := dbManager.GetIsSecrets(c.Context(), dagrunForm.Name, keys)
		if err != nil {
			// TODO: Improve better error handling
			log.Error().Err(err).Msg("Error getting dag run parameters")
			return c.SendStatus(fiber.StatusBadRequest)
		}

		for k := range isSecretMap {
			fmt.Println(k, isSecretMap[k])
		}

		if err := kclient.CreateDagRun(c.Context(), dagrunForm, isSecretMap, dagrunForm.Namespace, kubClient); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("Failed to create DagRun: %v", err),
			})
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"message": "DagRun created successfully",
		})

	})

}

func addStats(router fiber.Router, dbManager db.DbManager) {
	statsRouter := router.Group("/stats")

	statsRouter.Get("/dashboard", func(c *fiber.Ctx) error {
		stats, err := dbManager.GetDashboardStats(c.Context())
		if err != nil {
			log.Error().Err(err).Msg("Error getting main stats")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(stats)
	})
}

func addAccountAuth(router fiber.Router, authManager auth.AuthManager) {
	authRouter := router.Group("/auth")

	authRouter.Post("/login", func(c *fiber.Ctx) error {
		var req auth.Credentials
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		token, err := authManager.Login(c.Context(), &req)
		if err != nil {
			log.Error().Err(err).Msg("Error checking credentials")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		if token == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		// Set the JWT as an HTTP-only cookie
		c.Cookie(&fiber.Cookie{
			Name:     "jwt-kontroler",
			Value:    token,
			Expires:  time.Now().Add(24 * time.Hour), // Set cookie expiration time as needed
			HTTPOnly: true,                           // Ensure the cookie is HTTP-only
			Secure:   false,                          // Set to true in production (requires HTTPS)
			SameSite: "Strict",                       // or "Lax", depending on your requirements
		})

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"message": "Login successful",
		})
	})
	authRouter.Post("/create", func(c *fiber.Ctx) error {
		var req auth.Credentials
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// Validate username and password
		if err := ValidateCredentials(req); err != nil {
			log.Error().Err(err).Msg("Error checking ValidateCredentials")
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		if err := authManager.CreateAccount(c.Context(), &req); err != nil {
			log.Error().Err(err).Msg("Error creating account")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.SendStatus(fiber.StatusCreated)
	})

	authRouter.Post("/password/change", func(c *fiber.Ctx) error {
		var req auth.ChangeCredentials
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		if err := authManager.ChangePassword(c.Context(), c.Locals("username").(string), req); err != nil {
			log.Error().Err(err).Msg("Error updating password")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.SendStatus(fiber.StatusCreated)
	})

	authRouter.Post("/logout", func(c *fiber.Ctx) error {
		token := c.Locals("token").(string)

		if err := authManager.RevokeToken(c.Context(), token); err != nil {
			log.Error().Err(err).Msg("Error creating account")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.SendStatus(fiber.StatusAccepted)
	})

	authRouter.Get("/check", func(c *fiber.Ctx) error {
		jwtToken := c.Cookies("jwt-kontroler")
		if jwtToken == "" {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		username, err := authManager.IsValidLogin(c.Context(), jwtToken)
		if err != nil {
			return c.SendStatus(fiber.StatusUnauthorized)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"username": username,
		})
	})

	authRouter.Get("/users/page/:page", func(c *fiber.Ctx) error {
		page, err := strconv.Atoi(c.Params("page"))
		if err != nil {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		if page < 0 {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		users, err := authManager.GetUsers(c.Context(), 10, page*10)
		if err != nil {
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"users": users,
		})
	})

	authRouter.Get("/users/pages/count", func(c *fiber.Ctx) error {
		pageCount, err := authManager.GetUserPageCount(c.Context(), 10)
		if err != nil {
			log.Error().Err(err).Msg("Error getting user page count")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"count": pageCount,
		})
	})

	authRouter.Delete("/users/:user", func(c *fiber.Ctx) error {
		user := c.Params("user")
		if user == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		if user == "admin" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "cannot delete admin account",
			})
		}

		if err := authManager.DeleteUser(c.Context(), user); err != nil {
			log.Error().Err(err).Msg("Failed to delete user")
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		return c.SendStatus(fiber.StatusOK)
	})
}

func addLogs(router fiber.Router, logFetcher logs.LogFetcher) {
	logRouter := router.Group("/logs")

	logRouter.Get("/pod/:pod", func(c *fiber.Ctx) error {
		podName := c.Params("pod")
		if podName == "" {
			return c.SendStatus(fiber.StatusBadRequest)
		}

		return logs.ServeLogWithRange(c, podName, logFetcher)
	})
}
