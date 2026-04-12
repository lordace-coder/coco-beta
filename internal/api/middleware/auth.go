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
		c.Locals("project", cached.project)
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

	// Store in cache
	projectCacheMutex.Lock()
	projectCache[apiKey] = &projectCacheEntry{
		project:   &project,
		expiresAt: time.Now().Add(cacheTTL),
	}
	projectCacheMutex.Unlock()

	c.Locals("project", &project)

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

// LoadProjectByID loads a project from the :projectId URL param (no API key needed).
// Used by the /fn/:projectId/* route so cloud functions don't require x-api-key.
func LoadProjectByID(c *fiber.Ctx) error {
	projectID := c.Params("projectId")
	if projectID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "project ID required in URL",
		})
	}

	// Check cache first (keyed by project ID)
	cacheKey := "id:" + projectID
	projectCacheMutex.RLock()
	cached, found := projectCache[cacheKey]
	projectCacheMutex.RUnlock()

	if found && time.Now().Before(cached.expiresAt) {
		c.Locals("project", cached.project)
		return c.Next()
	}

	var project models.Project
	if err := database.DB.Where("id = ? AND active = ?", projectID, true).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   true,
				"message": "Project not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Database error",
		})
	}

	projectCacheMutex.Lock()
	projectCache[cacheKey] = &projectCacheEntry{
		project:   &project,
		expiresAt: time.Now().Add(cacheTTL),
	}
	projectCacheMutex.Unlock()

	c.Locals("project", &project)
	return c.Next()
}

// LoadDefaultProject loads the single default project (first project in DB).
// Used by /functions/func/* routes in single-instance mode — no project ID in URL.
func LoadDefaultProject(c *fiber.Ctx) error {
	const cacheKey = "default_project"

	projectCacheMutex.RLock()
	cached, found := projectCache[cacheKey]
	projectCacheMutex.RUnlock()

	if found && time.Now().Before(cached.expiresAt) {
		c.Locals("project", cached.project)
		return c.Next()
	}

	var project models.Project
	if err := database.DB.Where("active = ?", true).Order("created_at asc").First(&project).Error; err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   true,
			"message": "No active project found",
		})
	}

	projectCacheMutex.Lock()
	projectCache[cacheKey] = &projectCacheEntry{
		project:   &project,
		expiresAt: time.Now().Add(cacheTTL),
	}
	projectCacheMutex.Unlock()

	c.Locals("project", &project)
	return c.Next()
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
