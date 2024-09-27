package rest

import (
	"kontroler-server/pkg/auth"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/gofiber/fiber/v2"
)

// AuditLoggerMiddleware creates an audit log for each request
func AuditLoggerMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Record start time
		startTime := time.Now()

		// Process the request
		err := c.Next()

		// Log the details of the request and response
		log.Info().
			Time("timestamp", startTime).
			Str("ip", c.IP()).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("user_agent", c.Get("User-Agent")).
			Int("status_code", c.Response().StatusCode()).
			Dur("latency", time.Since(startTime)).
			Msg("Request processed")

		return err
	}
}

func Authentication(c *fiber.Ctx, authManager auth.AuthManager) error {
	if strings.HasSuffix(c.Path(), "/login") || strings.HasSuffix(c.Path(), "/check") {
		return c.Next()
	}

	// Get JWT from cookie
	jwtToken := c.Cookies("jwt-kontroler")
	if jwtToken == "" {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	id, err := authManager.IsValidLogin(c.Context(), jwtToken)
	if err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	c.Locals("token", jwtToken)
	c.Locals("id", id)

	return c.Next()
}
