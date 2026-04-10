package handlers

import (
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/services"
)

// Shared service instances used across all handlers
// These are initialized lazily to avoid nil database.DB
var (
	permChecker *services.PermissionChecker
)

func init() {
	// Initialize services that don't depend on database
	permChecker = services.NewPermissionChecker()
}

// InitHandlerServices initializes handler services after database connection
// This should be called from main() after database is connected
func InitHandlerServices() {
	queryBuilder = services.NewQueryBuilder(database.DB)
	relationshipResolver = services.NewRelationshipResolver()
	// Register the collection cache clearer so the system cron can evict it
	database.RegisterCollectionCacheClearer(func() {
		collectionCache.Range(func(k, _ interface{}) bool {
			collectionCache.Delete(k)
			return true
		})
	})
}
