package handlers

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"gorm.io/gorm"
)

// SubscribeToCollection WebSocket endpoint with filtering support
// Authentication: Send API key as first message: {"api_key": "your-api-key"}
// @Summary Subscribe to real-time collection updates
// @Description WebSocket endpoint for real-time document changes. Send API key as first message.
// @Tags Realtime
// @Param id path string true "Collection ID or name"
// @Router /collections/{id}/realtime [get]
func SubscribeToCollection(c *websocket.Conn) {
	collectionID := c.Params("id")

	// Wait for authentication message (first message must be API key)
	var authMsg struct {
		APIKey  string                 `json:"api_key"`
		Actions []string               `json:"actions,omitempty"`
		Filter  map[string]interface{} `json:"filter,omitempty"`
	}

	// Set read deadline for authentication (10 seconds)
	c.SetReadDeadline(time.Now().Add(10 * time.Second))

	if err := c.ReadJSON(&authMsg); err != nil {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "Authentication required. Send: {\"api_key\": \"your-key\"}",
		})
		c.Close()
		return
	}

	// Remove read deadline after authentication
	c.SetReadDeadline(time.Time{})

	if authMsg.APIKey == "" {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "API key required in first message",
		})
		c.Close()
		return
	}

	// Authenticate API key
	var project models.Project
	if err := database.DB.Where("api_key = ? AND active = ?", authMsg.APIKey, true).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.WriteJSON(fiber.Map{
				"error":   true,
				"message": "Invalid API key",
			})
		} else {
			c.WriteJSON(fiber.Map{
				"error":   true,
				"message": "Database error",
			})
		}
		c.Close()
		return
	}

	// Verify collection exists and user has access
	var collection models.Collection
	if err := database.DB.Where("(id = ? OR name = ?) AND project_id = ?", collectionID, collectionID, instanceID()).
		First(&collection).Error; err != nil {
		log.Printf("Collection not found: %v", err)
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "Collection not found or access denied",
		})
		c.Close()
		return
	}

	// Create subscription filter
	filter := services.SubscriptionFilter{
		Actions:      authMsg.Actions,
		FieldFilters: authMsg.Filter,
	}

	// Register connection with filter
	manager := services.GetConnectionManager()
	manager.AddConnection(collection.ID, c, filter, instanceID())
	defer manager.RemoveConnection(collection.ID, c)

	// Send welcome message
	welcomeMsg := fiber.Map{
		"action":          "authenticated",
		"collection_id":   collection.ID,
		"collection_name": collection.Name,
		"project_id":      instanceID(),
		"filter":          filter,
		"timestamp":       time.Now(),
		"redis_enabled":   services.IsRedisEnabled(),
	}
	c.WriteJSON(welcomeMsg)

	log.Printf("📡 WebSocket authenticated: collection=%s, project=%s, filters=%v",
		collection.ID, instanceID(), filter)

	// Keep connection alive and handle incoming messages
	for {
		messageType, message, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping/pong
		if messageType == websocket.PingMessage {
			c.WriteMessage(websocket.PongMessage, nil)
			continue
		}

		// Handle client messages (e.g., update filters)
		log.Printf("Received message from client: %s", message)

		// Allow clients to update filters dynamically
		var updateMsg struct {
			Actions []string               `json:"actions,omitempty"`
			Filter  map[string]interface{} `json:"filter,omitempty"`
		}
		if err := json.Unmarshal(message, &updateMsg); err == nil {
			if len(updateMsg.Actions) > 0 {
				filter.Actions = updateMsg.Actions
			}
			if updateMsg.Filter != nil {
				filter.FieldFilters = updateMsg.Filter
			}
			c.WriteJSON(fiber.Map{
				"message": "Filter updated",
				"filter":  filter,
			})
		}
	}
}

// WebSocketUpgrade middleware to upgrade HTTP connection to WebSocket (DEPRECATED)
func WebSocketUpgrade(c *fiber.Ctx) error {
	// Authenticate

	collectionID := c.Params("id")

	// Check if WebSocket upgrade is requested
	if websocket.IsWebSocketUpgrade(c) {
		// Parse query parameters for filters
		actions := []string{}
		if actionsParam := c.Query("actions"); actionsParam != "" {
			json.Unmarshal([]byte("["+actionsParam+"]"), &actions)
		}

		filterJSON := c.Query("filter", "{}")

		c.Locals("collectionID", collectionID)
		c.Locals("projectID", instanceID())
		c.Locals("filter", filterJSON)
		c.Locals("actions", actions)
		return c.Next()
	}

	return c.Status(fiber.StatusUpgradeRequired).JSON(fiber.Map{
		"error":   true,
		"message": "WebSocket upgrade required",
	})
}

// BroadcastDocumentChange broadcasts document changes via Redis Pub/Sub
func BroadcastDocumentChange(collectionID string, action string, document *models.Document, projectID string) {
	services.BroadcastDocumentChange(collectionID, action, document, projectID)
}

// GetRealtimeStats returns real-time statistics
// @Summary Get real-time connection statistics
// @Description Get statistics about active WebSocket connections
// @Tags Realtime
// @Produce json
// @Param id path string true "Collection ID or name"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/realtime/stats [get]
func GetRealtimeStats(c *fiber.Ctx) error {

	collectionID := c.Params("id")

	// Get collection
	var collection models.Collection
	if err := database.DB.Where("(id = ? OR name = ?) AND project_id = ?", collectionID, collectionID, instanceID()).
		First(&collection).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   true,
				"message": "Collection not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Database error",
		})
	}

	manager := services.GetConnectionManager()
	connectionCount := manager.GetConnectionCount(collection.ID)

	return c.JSON(fiber.Map{
		"collection_id":      collection.ID,
		"collection_name":    collection.Name,
		"active_connections": connectionCount,
		"redis_enabled":      services.IsRedisEnabled(),
		"timestamp":          time.Now(),
	})
}

// GetAllRealtimeStats returns stats for all collections
// @Summary Get all real-time statistics
// @Description Get WebSocket connection statistics for all collections
// @Tags Realtime
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /realtime/stats [get]
func GetAllRealtimeStats(c *fiber.Ctx) error {

	manager := services.GetConnectionManager()
	stats := manager.GetAllStats()

	return c.JSON(fiber.Map{
		"stats":         stats,
		"redis_enabled": services.IsRedisEnabled(),
		"timestamp":     time.Now(),
	})
}
