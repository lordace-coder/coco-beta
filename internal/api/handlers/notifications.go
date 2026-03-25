package handlers

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
)

// NotificationManager manages notification WebSocket connections
type NotificationManager struct {
	// Global connections (receive all notifications)
	globalConns map[*websocket.Conn]string // conn -> projectID

	// Channel-specific connections (receive only channel notifications)
	channelConns map[string]map[*websocket.Conn]string // channelID -> conn -> projectID

	mu sync.RWMutex
}

type NotificationMessage struct {
	Type      string                 `json:"type"` // "notification", "broadcast", "channel"
	Channel   string                 `json:"channel,omitempty"`
	From      string                 `json:"from,omitempty"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	ProjectID string                 `json:"project_id,omitempty"`
}

var (
	notificationManager *NotificationManager
	notifOnce           sync.Once
)

// GetNotificationManager returns singleton instance
func GetNotificationManager() *NotificationManager {
	notifOnce.Do(func() {
		notificationManager = &NotificationManager{
			globalConns:  make(map[*websocket.Conn]string),
			channelConns: make(map[string]map[*websocket.Conn]string),
		}
	})
	return notificationManager
}

// AddGlobalConnection adds a connection to global notifications
func (nm *NotificationManager) AddGlobalConnection(conn *websocket.Conn, projectID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	nm.globalConns[conn] = projectID
	log.Printf("📢 Global notification connection added (project: %s, total: %d)", projectID, len(nm.globalConns))
}

// AddChannelConnection adds a connection to a specific channel
func (nm *NotificationManager) AddChannelConnection(channel string, conn *websocket.Conn, projectID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if nm.channelConns[channel] == nil {
		nm.channelConns[channel] = make(map[*websocket.Conn]string)
	}
	nm.channelConns[channel][conn] = projectID

	log.Printf("📢 Channel connection added: %s (project: %s, total: %d)",
		channel, projectID, len(nm.channelConns[channel]))
}

// RemoveConnection removes a connection from all subscriptions
func (nm *NotificationManager) RemoveConnection(conn *websocket.Conn) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Remove from global
	delete(nm.globalConns, conn)

	// Remove from all channels
	for channel, conns := range nm.channelConns {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(nm.channelConns, channel)
		}
	}

	log.Printf("📢 Connection removed")
}

// BroadcastGlobal sends a message to all global connections in the same project (excluding sender)
func (nm *NotificationManager) BroadcastGlobal(msg NotificationMessage, projectID string, excludeConn *websocket.Conn) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	msg.Type = "global"
	msg.Timestamp = time.Now()
	msg.ProjectID = projectID

	data, _ := json.Marshal(msg)
	sentCount := 0

	for conn, connProjectID := range nm.globalConns {
		// Only send to connections in the same project, excluding the sender
		if connProjectID == projectID && conn != excludeConn {
			sentCount++
			go func(c *websocket.Conn) {
				if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("Error broadcasting global: %v", err)
				}
			}(conn)
		}
	}

	log.Printf("📢 Global broadcast sent to project %s (%d connections)", projectID, sentCount)
}

// BroadcastChannel sends a message to all connections subscribed to a channel (excluding sender)
func (nm *NotificationManager) BroadcastChannel(channel string, msg NotificationMessage, projectID string, excludeConn *websocket.Conn) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	conns, exists := nm.channelConns[channel]
	if !exists {
		log.Printf("No subscribers for channel: %s", channel)
		return
	}

	msg.Type = "channel"
	msg.Channel = channel
	msg.Timestamp = time.Now()
	msg.ProjectID = projectID

	data, _ := json.Marshal(msg)

	sentCount := 0
	for conn, connProjectID := range conns {
		// Only send to connections in the same project, excluding the sender
		if connProjectID == projectID && conn != excludeConn {
			go func(c *websocket.Conn) {
				if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("Error broadcasting to channel %s: %v", channel, err)
				}
			}(conn)
			sentCount++
		}
	}

	log.Printf("📢 Channel broadcast: %s to project %s (%d/%d connections)",
		channel, projectID, sentCount, len(conns))
}

// GetStats returns notification statistics
func (nm *NotificationManager) GetStats() map[string]interface{} {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	channelStats := make(map[string]int)
	for channel, conns := range nm.channelConns {
		channelStats[channel] = len(conns)
	}

	return map[string]interface{}{
		"global_connections":  len(nm.globalConns),
		"active_channels":     len(nm.channelConns),
		"channel_connections": channelStats,
	}
}

// NotificationWebSocket handles global notification WebSocket connections
// @Summary Subscribe to global notifications
// @Description WebSocket for global peer-to-peer notifications. Send auth message: {"api_key": "YOUR_API_KEY"}
// @Tags Notifications
// @Router /notifications/global [get]
func NotificationWebSocket(c *websocket.Conn) {
	// Wait for authentication
	c.SetReadDeadline(time.Now().Add(10 * time.Second))

	var authMsg struct {
		APIKey string `json:"api_key"` // Support both for compatibility
		Auth   string `json:"auth"`    // Legacy field
	}

	if err := c.ReadJSON(&authMsg); err != nil {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "Authentication required. Send: {\"api_key\": \"YOUR_API_KEY\"}",
		})
		c.Close()
		return
	}

	// Use api_key if provided, otherwise fall back to auth
	apiKey := authMsg.APIKey
	if apiKey == "" {
		apiKey = authMsg.Auth
	}

	if apiKey == "" {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "API key required in first message",
		})
		c.Close()
		return
	}

	// Authenticate
	var project models.Project
	if err := database.DB.Where("api_key = ? AND active = true", apiKey).First(&project).Error; err != nil {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "Invalid or inactive API key",
		})
		c.Close()
		return
	}

	c.SetReadDeadline(time.Time{})

	// Register connection
	manager := GetNotificationManager()
	manager.AddGlobalConnection(c, project.ID)
	defer manager.RemoveConnection(c)

	// Send welcome
	c.WriteJSON(fiber.Map{
		"type":       "connected",
		"scope":      "global",
		"project":    project.Name,
		"project_id": project.ID,
		"timestamp":  time.Now(),
	})

	// Handle messages
	for {
		var msg NotificationMessage
		if err := c.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping
		if msg.Type == "ping" {
			c.WriteJSON(fiber.Map{"type": "pong", "timestamp": time.Now()})
			continue
		}

		// Broadcast message to all global peers in the same project (excluding sender)
		msg.From = c.RemoteAddr().String()
		manager.BroadcastGlobal(msg, project.ID, c)
	}
}

// ChannelNotificationWebSocket handles channel-specific notification WebSocket connections
// @Summary Subscribe to channel notifications
// @Description WebSocket for channel-specific peer-to-peer notifications. Send auth: {"api_key": "YOUR_API_KEY", "channel": "chat-room-1"}
// @Tags Notifications
// @Param channel path string true "Channel name"
// @Router /notifications/channel/{channel} [get]
func ChannelNotificationWebSocket(c *websocket.Conn) {
	// Wait for authentication
	c.SetReadDeadline(time.Now().Add(10 * time.Second))

	var authMsg struct {
		APIKey  string `json:"api_key"` // Support both for compatibility
		Auth    string `json:"auth"`    // Legacy field
		Channel string `json:"channel"`
	}

	if err := c.ReadJSON(&authMsg); err != nil {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "Authentication required. Send: {\"api_key\": \"YOUR_API_KEY\", \"channel\": \"your-channel\"}",
		})
		c.Close()
		return
	}

	// Get channel from path if not in auth message
	channelParam := c.Params("channel")
	if authMsg.Channel == "" && channelParam != "" {
		authMsg.Channel = channelParam
	}

	if authMsg.Channel == "" {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "Channel name is required",
		})
		c.Close()
		return
	}

	// Use api_key if provided, otherwise fall back to auth
	apiKey := authMsg.APIKey
	if apiKey == "" {
		apiKey = authMsg.Auth
	}

	if apiKey == "" {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "API key required in first message",
		})
		c.Close()
		return
	}

	// Authenticate
	var project models.Project
	if err := database.DB.Where("api_key = ? AND active = true", apiKey).First(&project).Error; err != nil {
		c.WriteJSON(fiber.Map{
			"error":   true,
			"message": "Invalid or inactive API key",
		})
		c.Close()
		return
	}

	c.SetReadDeadline(time.Time{})

	// Register connection to channel
	manager := GetNotificationManager()
	manager.AddChannelConnection(authMsg.Channel, c, project.ID)
	defer manager.RemoveConnection(c)

	// Send welcome
	c.WriteJSON(fiber.Map{
		"type":       "connected",
		"scope":      "channel",
		"channel":    authMsg.Channel,
		"project":    project.Name,
		"project_id": project.ID,
		"timestamp":  time.Now(),
	})

	// Handle messages
	for {
		var msg NotificationMessage
		if err := c.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping
		if msg.Type == "ping" {
			c.WriteJSON(fiber.Map{"type": "pong", "timestamp": time.Now()})
			continue
		}

		// Broadcast message to all channel peers in the same project (excluding sender)
		msg.From = c.RemoteAddr().String()
		manager.BroadcastChannel(authMsg.Channel, msg, project.ID, c)
	}
}

// GetNotificationStats returns notification statistics
// @Summary Get notification statistics
// @Description Get statistics about active notification connections
// @Tags Notifications
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /notifications/stats [get]
func GetNotificationStats(c *fiber.Ctx) error {
	// Authenticate
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	var project models.Project
	if err := database.DB.Where("api_key = ? AND active = true", apiKey).First(&project).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid API key",
		})
	}

	manager := GetNotificationManager()
	stats := manager.GetStats()

	return c.JSON(fiber.Map{
		"project":   project.Name,
		"stats":     stats,
		"timestamp": time.Now(),
	})
}

// SendNotification sends a notification via HTTP POST (alternative to WebSocket)
// @Summary Send a notification
// @Description Send a notification to global or channel subscribers
// @Tags Notifications
// @Accept json
// @Produce json
// @Param notification body NotificationMessage true "Notification data"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /notifications/send [post]
func SendNotification(c *fiber.Ctx) error {
	// Authenticate
	apiKey := c.Get("X-API-Key")
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	var project models.Project
	if err := database.DB.Where("api_key = ? AND active = true", apiKey).First(&project).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid API key",
		})
	}

	// Parse notification
	var msg NotificationMessage
	if err := c.BodyParser(&msg); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if msg.Data == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Data is required",
		})
	}

	manager := GetNotificationManager()

	// Send to global or channel
	if msg.Channel != "" {
		// HTTP POST - broadcast to all (no sender to exclude)
		manager.BroadcastChannel(msg.Channel, msg, project.ID, nil)
	} else {
		// HTTP POST - broadcast to all (no sender to exclude)
		manager.BroadcastGlobal(msg, project.ID, nil)
	}

	return c.JSON(fiber.Map{
		"status":    "sent",
		"type":      msg.Type,
		"channel":   msg.Channel,
		"timestamp": time.Now(),
	})
}
