package routes

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers"
	"github.com/patrick/cocobase/internal/api/middleware"
)

// SetupNotificationRoutes sets up notification-related routes
func SetupNotificationRoutes(app *fiber.App) {
	// WebSocket routes (no auth middleware - auth via first message)
	// Global notifications (all peers in project)
	app.Get("/notifications/global", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusUpgradeRequired).JSON(fiber.Map{
			"error":   true,
			"message": "WebSocket upgrade required. Use ws:// protocol to connect.",
		})
	}, websocket.New(handlers.NotificationWebSocket))

	// Channel-specific notifications
	app.Get("/notifications/channel/:channel", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusUpgradeRequired).JSON(fiber.Map{
			"error":   true,
			"message": "WebSocket upgrade required. Use ws:// protocol to connect.",
		})
	}, websocket.New(handlers.ChannelNotificationWebSocket))

	// HTTP routes (require API key middleware)
	notifications := app.Group("/notifications", middleware.RequireAPIKey)
	{
		notifications.Get("/stats", handlers.GetNotificationStats)
		notifications.Post("/send", handlers.SendNotification)
	}
}
