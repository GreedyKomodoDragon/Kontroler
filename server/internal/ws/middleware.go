package ws

import (
	"kontroler-server/internal/auth"

	"github.com/gofiber/fiber/v2"
)

func Auth(authManager auth.AuthManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get JWT token from HTTP-only cookie
		jwtToken := c.Cookies("jwt-kontroler")
		if jwtToken == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "Missing authentication token")
		}

		// Validate the JWT token
		username, _, err := authManager.IsValidLogin(c.Context(), jwtToken)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Invalid token")
		}

		// Store username in Fiber context (accessible in WebSocket handler)
		c.Locals("username", username)

		// ✅ If authentication succeeds, allow WebSocket upgrade
		return c.Next()
	}
}
