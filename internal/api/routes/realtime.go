package routes

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers"
	"github.com/patrick/cocobase/internal/api/middleware"
)

func wsUpgradeCheck(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		return c.Next()
	}
	return c.Status(fiber.StatusUpgradeRequired).JSON(fiber.Map{
		"error":   true,
		"message": "WebSocket upgrade required. Use ws:// protocol to connect.",
	})
}

// SetupRealtimeRoutes sets up WebSocket realtime routes
// These MUST be registered before other collection routes to avoid conflicts
func SetupRealtimeRoutes(app *fiber.App) {
	// Collection document change stream
	app.Get("/collections/:id/realtime", wsUpgradeCheck, websocket.New(handlers.SubscribeToCollection))

	// Broadcast rooms — matches Python's /realtime/* routes
	// GET /realtime/rooms — list active rooms (HTTP)
	app.Get("/realtime/rooms", middleware.RequireAPIKey, handlers.ListBroadcastRooms)

	// WS /realtime/broadcast — general broadcast (maps to global notifications)
	app.Get("/realtime/broadcast", wsUpgradeCheck, websocket.New(handlers.NotificationWebSocket))

	// WS /realtime/rooms/:room_id — join a named broadcast room (maps to channel notifications)
	app.Get("/realtime/rooms/:room_id", wsUpgradeCheck, websocket.New(handlers.ChannelNotificationWebSocket))
}
