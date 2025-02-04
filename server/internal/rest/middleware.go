package rest

import (
	"fmt"
	"kontroler-server/internal/auth"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/gofiber/fiber/v2"
)

const (
	AdminRole  = "admin"
	EditorRole = "editor"
	ViewerRole = "viewer"
)

var roleHierarchy = map[string]int{
	AdminRole:  3,
	EditorRole: 2,
	ViewerRole: 1,
}

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

	username, role, err := authManager.IsValidLogin(c.Context(), jwtToken)
	if err != nil {
		return c.SendStatus(fiber.StatusUnauthorized)
	}

	c.Locals("token", jwtToken)
	c.Locals("username", username)
	c.Locals("role", role)

	return c.Next()
}

// roleMiddleware checks if the user has the required role
func roleMiddleware(requiredRole string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole := c.Locals("role")

		if userRole == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}

		// Convert to strings for role comparison
		role, ok := userRole.(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid role",
			})
		}

		// Check if the user has the required role or higher
		if !isRoleGreaterOrEqual(role, requiredRole) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": fmt.Sprintf("Forbidden: You must have '%s' or higher role", requiredRole),
			})
		}

		return c.Next()
	}
}

// isRoleGreaterOrEqual compares user role rank with required role rank
func isRoleGreaterOrEqual(userRole, requiredRole string) bool {
	userRank, userExists := roleHierarchy[userRole]
	requiredRank, requiredExists := roleHierarchy[requiredRole]

	// If the role doesn't exist, deny access
	if !userExists || !requiredExists {
		return false
	}

	return userRank >= requiredRank
}
