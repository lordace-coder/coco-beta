package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/instance"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"gorm.io/gorm"
)

// ── API key cache (single instance key, cached to avoid hitting DB on every request) ──

var (
	cachedAPIKey   string
	cachedKeyMu    sync.RWMutex
	cachedKeyUntil time.Time
)

func getInstanceAPIKey() string {
	cachedKeyMu.RLock()
	if time.Now().Before(cachedKeyUntil) {
		k := cachedAPIKey
		cachedKeyMu.RUnlock()
		return k
	}
	cachedKeyMu.RUnlock()

	var project models.Project
	if err := database.DB.Select("api_key").Order("created_at asc").First(&project).Error; err != nil {
		return ""
	}

	cachedKeyMu.Lock()
	cachedAPIKey = project.APIKey
	cachedKeyUntil = time.Now().Add(5 * time.Minute)
	cachedKeyMu.Unlock()
	return project.APIKey
}

// InvalidateAPIKeyCache forces the next request to re-read the key from DB.
// Call this after RegenAPIKey.
func InvalidateAPIKeyCache() {
	cachedKeyMu.Lock()
	cachedKeyUntil = time.Time{}
	cachedKeyMu.Unlock()
}

// ValidateAPIKey returns true if the given key matches the instance API key.
func ValidateAPIKey(key string) bool {
	expected := getInstanceAPIKey()
	return expected != "" && key == expected
}

// RequireAPIKey validates that the request carries the instance API key.
func RequireAPIKey(c *fiber.Ctx) error {
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		apiKey = strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
	}
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "API key required",
		})
	}

	expected := getInstanceAPIKey()
	if expected == "" || apiKey != expected {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid API key",
		})
	}
	return c.Next()
}

// GetUser retrieves the dashboard admin from context.
func GetUser(c *fiber.Ctx) *models.User {
	user, ok := c.Locals("user").(*models.User)
	if !ok {
		return nil
	}
	return user
}

// GetAppUserFromContext retrieves the app user from context (if authenticated).
func GetAppUserFromContext(c *fiber.Ctx) *models.AppUser {
	appUser, ok := c.Locals("app_user").(*models.AppUser)
	if !ok {
		return nil
	}
	return appUser
}

// RequireAppUser validates the Bearer JWT and loads the AppUser into context.
func RequireAppUser(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Authorization header required",
		})
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid authorization header format. Expected: Bearer <token>",
		})
	}

	token := parts[1]

	claims, err := services.DecodeAppUserToken(token, instance.ID())
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid or expired token",
		})
	}

	var appUser models.AppUser
	if err := database.DB.Where("id = ?", claims["userId"]).First(&appUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   true,
				"message": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Database error"})
	}

	c.Locals("app_user", &appUser)
	return c.Next()
}
