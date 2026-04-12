package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/pkg/config"
)

// SetupRoutes configures all application routes
func SetupRoutes(app *fiber.App) {
	// Root route
	app.Get("/", handlers.Welcome)

	// Health check
	app.Get("/health", handlers.HealthCheck)

	// Setup WebSocket realtime route FIRST (before other collection routes)
	SetupRealtimeRoutes(app)

	// Setup collection routes
	SetupCollectionRoutes(app)

	// Cloud function HTTP routes — single-instance: /functions/func/:functionName[/*]
	// No project ID in URL; the default (only) project is loaded automatically.
	app.All("/functions/func/:functionName", middleware.LoadDefaultProject, handlers.HandleHTTPFunction)
	app.All("/functions/func/:functionName/*", middleware.LoadDefaultProject, handlers.HandleHTTPFunction)

	// Legacy: /functions/:projectId/func/:functionName[/*] — kept for backwards compat.
	app.All("/functions/:projectId/func/:functionName", middleware.LoadProjectByID, handlers.HandleHTTPFunction)
	app.All("/functions/:projectId/func/:functionName/*", middleware.LoadProjectByID, handlers.HandleHTTPFunction)

	// Setup auth routes
	SetupAuthRoutes(app)

	// Setup notification routes (peer-to-peer messaging)
	SetupNotificationRoutes(app)

	// API version group
	api := app.Group("/api/" + config.AppConfig.APIVersion)

	// Health endpoint under API
	api.Get("/health", handlers.HealthCheck)
}
