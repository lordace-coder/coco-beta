package middleware

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/singleflight"
)

var requestGroup singleflight.Group

// ================= CACHE =================

type CacheEntry struct {
	Data      []byte
	Status    int
	ExpiresAt time.Time
}

type Cache struct {
	mu    sync.RWMutex
	store map[string]*CacheEntry
}

var cache = &Cache{
	store: make(map[string]*CacheEntry),
}

// run cleanup ONLY ONCE
var cleanupOnce sync.Once

func startCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()

			cache.mu.Lock()
			for key, entry := range cache.store {
				if now.After(entry.ExpiresAt) {
					delete(cache.store, key)
				}
			}
			cache.mu.Unlock()
		}
	}()
}

// ================= MIDDLEWARE =================

func CacheMiddleware(duration time.Duration) fiber.Handler {

	// ensure cleanup runs once
	cleanupOnce.Do(startCleanup)

	return func(c *fiber.Ctx) error {

		// only cache GET
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}

		cacheKey := string(c.Request().URI().FullURI())

		// ================= 1. CACHE CHECK =================

		cache.mu.RLock()
		entry, found := cache.store[cacheKey]
		cache.mu.RUnlock()

		if found && time.Now().Before(entry.ExpiresAt) {
			c.Response().Header.Set("X-Cache", "HIT")
			return c.Status(entry.Status).Send(entry.Data)
		}

		// ================= 2. SINGLEFLIGHT =================

		result, err, shared := requestGroup.Do(cacheKey, func() (interface{}, error) {

			// ONLY first request enters here
			if err := c.Next(); err != nil {
				return nil, err
			}

			// copy response safely
			body := append([]byte(nil), c.Response().Body()...)
			status := c.Response().StatusCode()

			// store in cache
			if status == fiber.StatusOK {
				cache.mu.Lock()
				cache.store[cacheKey] = &CacheEntry{
					Data:      body,
					Status:    status,
					ExpiresAt: time.Now().Add(duration),
				}
				cache.mu.Unlock()
			}

			return &CacheEntry{
				Data:   body,
				Status: status,
			}, nil
		})

		if err != nil {
			return err
		}

		// ================= 3. DEDUPED RESPONSE =================

		if shared {
			entry := result.(*CacheEntry)

			c.Response().Header.Set("X-Cache", "DEDUPED")
			return c.Status(entry.Status).Send(entry.Data)
		}

		// original request already wrote response
		c.Response().Header.Set("X-Cache", "MISS")
		return nil
	}
}
