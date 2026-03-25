package routes

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers"
)

// SetupRealtimeRoutes sets up WebSocket realtime routes
// These MUST be registered before other collection routes to avoid conflicts
func SetupRealtimeRoutes(app *fiber.App) {
	// Real-time WebSocket endpoint (NO auth middleware - auth via first message)
	// Must use middleware that allows WebSocket upgrade
	app.Get("/collections/:id/realtime", func(c *fiber.Ctx) error {
		// Check if this is a WebSocket upgrade request
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusUpgradeRequired).JSON(fiber.Map{
			"error":   true,
			"message": "WebSocket upgrade required. Use ws:// protocol to connect.",
		})
	}, websocket.New(handlers.SubscribeToCollection))
}
