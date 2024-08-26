package rest

import (
	"kubeconductor-server/pkg/auth"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func Authentication(c *fiber.Ctx, authManager auth.AuthManager) error {
	if strings.HasSuffix(c.Path(), "/login") {
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
