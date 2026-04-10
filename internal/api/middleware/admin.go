package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
)

// RequireAdmin validates the admin JWT and loads the admin user into context.
func RequireAdmin(c *fiber.Ctx) error {
	auth := c.Get("Authorization")
	if auth == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Authorization header required"})
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Invalid authorization header"})
	}

	claims, err := services.DecodeAdminToken(parts[1])
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Invalid or expired admin token"})
	}

	adminID, _ := claims["adminId"].(string)
	var admin models.AdminUser
	if err := database.DB.Where("id = ?", adminID).First(&admin).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Admin not found"})
	}

	c.Locals("admin", &admin)
	return c.Next()
}

// GetAdmin retrieves the admin user from context.
func GetAdmin(c *fiber.Ctx) *models.AdminUser {
	admin, _ := c.Locals("admin").(*models.AdminUser)
	return admin
}
