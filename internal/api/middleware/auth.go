package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"gorm.io/gorm"
)

// Cache entry for project lookups
type projectCacheEntry struct {
	project   *models.Project
	user      *models.User
	expiresAt time.Time
}

// Simple in-memory cache for API key lookups
var (
	projectCache      = make(map[string]*projectCacheEntry)
	projectCacheMutex sync.RWMutex
	cacheTTL          = 5 * time.Minute // Cache for 5 minutes
)

// Background goroutine to clean up expired cache entries
func init() {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			projectCacheMutex.Lock()
			now := time.Now()
			for key, entry := range projectCache {
				if now.After(entry.expiresAt) {
					delete(projectCache, key)
				}
			}
			projectCacheMutex.Unlock()
		}
	}()
}

// RequireAPIKey validates API key and loads project
func RequireAPIKey(c *fiber.Ctx) error {
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		apiKey = c.Get("Authorization")
		if apiKey != "" {
			// Remove "Bearer " prefix if present
			apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		}
	}

	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "API key required",
		})
	}

	// Check cache first
	projectCacheMutex.RLock()
	cached, found := projectCache[apiKey]
	projectCacheMutex.RUnlock()

	if found && time.Now().Before(cached.expiresAt) {
		// Cache hit - use cached data
		c.Locals("project", cached.project)
		c.Locals("user", cached.user)
		return c.Next()
	}

	// Cache miss - query database
	var project models.Project
	if err := database.DB.Where("api_key = ? AND active = ?", apiKey, true).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   true,
				"message": "Invalid API key",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Database error",
		})
	}

	// Load project owner
	var user models.User
	if err := database.DB.Where("id = ?", project.UserID).First(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to load project owner",
		})
	}

	// Store in cache
	projectCacheMutex.Lock()
	projectCache[apiKey] = &projectCacheEntry{
		project:   &project,
		user:      &user,
		expiresAt: time.Now().Add(cacheTTL),
	}
	projectCacheMutex.Unlock()

	// Store project and user in context
	c.Locals("project", &project)
	c.Locals("user", &user)

	return c.Next()
}

// GetAppUser retrieves the authenticated app user from context (optional)
func GetAppUser(c *fiber.Ctx) error {
	// Check for app user token (if implementing app user auth)
	token := c.Get("X-App-User-Token")
	if token != "" {
		// TODO: Implement app user token validation
		// For now, we'll skip this and make it optional
	}

	// App user is optional, so always continue
	return c.Next()
}

// GetProject retrieves the project from context
func GetProject(c *fiber.Ctx) *models.Project {
	project, ok := c.Locals("project").(*models.Project)
	if !ok {
		return nil
	}
	return project
}

// GetUser retrieves the user from context
func GetUser(c *fiber.Ctx) *models.User {
	user, ok := c.Locals("user").(*models.User)
	if !ok {
		return nil
	}
	return user
}

// GetAppUserFromContext retrieves the app user from context (if authenticated)
func GetAppUserFromContext(c *fiber.Ctx) *models.AppUser {
	appUser, ok := c.Locals("app_user").(*models.AppUser)
	if !ok {
		return nil
	}
	return appUser
}

// RequireAppUser validates JWT token and loads app user
func RequireAppUser(c *fiber.Ctx) error {
	// Extract token from Authorization header
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Authorization header required",
		})
	}

	// Extract bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid authorization header format. Expected: Bearer <token>",
		})
	}

	token := parts[1]

	// Decode and validate token
	claims, err := services.DecodeAppUserToken(token, GetProject(c).ID)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid or expired token",
		})
	}

	// Load app user from database
	var appUser models.AppUser
	if err := database.DB.Where("id = ?", claims["userId"]).First(&appUser).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   true,
				"message": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Database error",
		})
	}

	// Store app user in context
	c.Locals("app_user", &appUser)

	return c.Next()
}
