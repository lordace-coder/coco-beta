package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/patrick/cocobase/internal/models"
	"github.com/redis/go-redis/v9"
)

var (
	redisClient    *redis.Client
	redisOnce      sync.Once
	ctx            = context.Background()
	isRedisEnabled bool
	redisPubSub    *redis.PubSub
	redisInitMutex sync.Mutex
)

// RealtimeEvent represents a document change event
type RealtimeEvent struct {
	Action       string                 `json:"action"` // created, updated, deleted
	CollectionID string                 `json:"collection_id"`
	DocumentID   string                 `json:"document_id"`
	Document     map[string]interface{} `json:"document,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	ProjectID    string                 `json:"project_id"` // For multi-tenant filtering
}

// SubscriptionFilter allows users to filter which events they receive
type SubscriptionFilter struct {
	Actions      []string               // e.g., ["created", "updated"]
	FieldFilters map[string]interface{} // e.g., {"status": "active", "user_id": "123"}
}

// WebSocketSubscription represents an active subscription
type WebSocketSubscription struct {
	Conn       *websocket.Conn
	Filter     SubscriptionFilter
	ProjectID  string
	Collection string
}

// ConnectionManager manages WebSocket connections with Redis Pub/Sub
type ConnectionManager struct {
	subscriptions map[string]map[*websocket.Conn]*WebSocketSubscription // collectionID -> connections
	mu            sync.RWMutex
	redisListener chan *redis.Message
}

var (
	connManager *ConnectionManager
	managerOnce sync.Once
)

// InitRedis initializes Redis connection
func InitRedis() {
	redisOnce.Do(func() {
		redisURL := os.Getenv("REDIS_URL")
		if redisURL == "" {
			redisURL = "localhost:6379" // default
		}

		var opt *redis.Options
		var err error

		// Try to parse as URL first (handles redis:// and rediss:// schemes)
		opt, err = redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("⚠️  Failed to parse REDIS_URL as URL: %v", err)
			log.Printf("⚠️  Attempting to use as plain address...")

			// Fallback: treat as plain address (host:port)
			redisPassword := os.Getenv("REDIS_PASSWORD")
			opt = &redis.Options{
				Addr:     redisURL,
				Password: redisPassword,
				DB:       0,
			}
		}

		// Set additional options
		opt.DialTimeout = 5 * time.Second
		opt.ReadTimeout = 3 * time.Second
		opt.WriteTimeout = 3 * time.Second
		opt.PoolSize = 10

		redisClient = redis.NewClient(opt)

		// Test connection
		ctx := context.Background()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("⚠️  Redis connection failed: %v (real-time features disabled)", err)
			isRedisEnabled = false
			redisClient = nil
		} else {
			log.Println("✅ Redis connected - Real-time enabled")
			isRedisEnabled = true
		}
	})
}

// GetRedisClient returns the Redis client instance (can be nil if Redis is not available)
func GetRedisClient() *redis.Client {
	return redisClient
}

// GetConnectionManager returns singleton instance
func GetConnectionManager() *ConnectionManager {
	managerOnce.Do(func() {
		connManager = &ConnectionManager{
			subscriptions: make(map[string]map[*websocket.Conn]*WebSocketSubscription),
			redisListener: make(chan *redis.Message, 256),
		}

		// Initialize Redis if not already
		InitRedis()

		if isRedisEnabled {
			go connManager.listenToRedis()
		}
	})
	return connManager
}

// listenToRedis subscribes to Redis Pub/Sub and forwards messages to WebSocket clients
func (cm *ConnectionManager) listenToRedis() {
	redisInitMutex.Lock()
	if redisPubSub == nil {
		// Subscribe to all collection channels with pattern
		redisPubSub = redisClient.PSubscribe(ctx, "cocobase:collection:*")
		log.Println("🔔 Subscribed to Redis Pub/Sub pattern: cocobase:collection:*")
	}
	redisInitMutex.Unlock()

	// Listen for messages
	for {
		msg, err := redisPubSub.ReceiveMessage(ctx)
		if err != nil {
			log.Printf("Redis PubSub error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		// Parse event
		var event RealtimeEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("Failed to parse Redis message: %v", err)
			continue
		}

		// Forward to WebSocket clients with filtering
		cm.broadcastEvent(event.CollectionID, event)
	}
}

// broadcastEvent sends event to filtered subscribers
func (cm *ConnectionManager) broadcastEvent(collectionID string, event RealtimeEvent) {
	cm.mu.RLock()
	subscriptions := cm.subscriptions[collectionID]
	cm.mu.RUnlock()

	if len(subscriptions) == 0 {
		return
	}

	eventJSON, _ := json.Marshal(event)

	for conn, sub := range subscriptions {
		// Check if event passes filters
		if !cm.eventMatchesFilter(event, sub.Filter) {
			continue
		}

		// Check project isolation
		if sub.ProjectID != "" && sub.ProjectID != event.ProjectID {
			continue
		}

		go func(c *websocket.Conn) {
			if err := c.WriteMessage(websocket.TextMessage, eventJSON); err != nil {
				log.Printf("WebSocket write error: %v", err)
				cm.RemoveConnection(collectionID, c)
			}
		}(conn)
	}
}

// eventMatchesFilter checks if event matches subscription filter
func (cm *ConnectionManager) eventMatchesFilter(event RealtimeEvent, filter SubscriptionFilter) bool {
	// Filter by action
	if len(filter.Actions) > 0 {
		actionMatch := false
		for _, action := range filter.Actions {
			if action == event.Action {
				actionMatch = true
				break
			}
		}
		if !actionMatch {
			return false
		}
	}

	// Filter by document fields
	if len(filter.FieldFilters) > 0 {
		for key, expectedValue := range filter.FieldFilters {
			if actualValue, exists := event.Document[key]; !exists || actualValue != expectedValue {
				return false
			}
		}
	}

	return true
}

// AddConnection registers a new WebSocket connection with filter
func (cm *ConnectionManager) AddConnection(collectionID string, conn *websocket.Conn, filter SubscriptionFilter, projectID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.subscriptions[collectionID] == nil {
		cm.subscriptions[collectionID] = make(map[*websocket.Conn]*WebSocketSubscription)
	}

	cm.subscriptions[collectionID][conn] = &WebSocketSubscription{
		Conn:       conn,
		Filter:     filter,
		ProjectID:  projectID,
		Collection: collectionID,
	}

	log.Printf("WebSocket connected to collection: %s (filters: actions=%v, fields=%v, total: %d)",
		collectionID, filter.Actions, filter.FieldFilters, len(cm.subscriptions[collectionID]))
}

// RemoveConnection unregisters a WebSocket connection
func (cm *ConnectionManager) RemoveConnection(collectionID string, conn *websocket.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conns, exists := cm.subscriptions[collectionID]; exists {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(cm.subscriptions, collectionID)
		}
	}

	log.Printf("WebSocket disconnected from collection: %s", collectionID)
}

// PublishEvent publishes event to Redis (both Go and Python can consume)
func PublishEvent(event RealtimeEvent) {
	if !isRedisEnabled || redisClient == nil {
		return
	}

	event.Timestamp = time.Now()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal event: %v", err)
		return
	}

	// Publish to Redis with collection-specific channel
	channel := fmt.Sprintf("cocobase:collection:%s", event.CollectionID)
	if err := redisClient.Publish(ctx, channel, eventJSON).Err(); err != nil {
		log.Printf("Failed to publish to Redis: %v", err)
	}
}

// BroadcastDocumentChange is a helper to publish document changes
func BroadcastDocumentChange(collectionID string, action string, document *models.Document, projectID string) {
	event := RealtimeEvent{
		Action:       action,
		CollectionID: collectionID,
		DocumentID:   document.ID,
		Document:     document.Data,
		ProjectID:    projectID,
	}

	PublishEvent(event)
}

// GetConnectionCount returns the number of active connections for a collection
func (cm *ConnectionManager) GetConnectionCount(collectionID string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if conns, exists := cm.subscriptions[collectionID]; exists {
		return len(conns)
	}
	return 0
}

// GetAllStats returns stats for all collections
func (cm *ConnectionManager) GetAllStats() map[string]int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	stats := make(map[string]int)
	for collectionID, conns := range cm.subscriptions {
		stats[collectionID] = len(conns)
	}
	return stats
}

// IsRedisEnabled returns whether Redis is available
func IsRedisEnabled() bool {
	return isRedisEnabled
}
