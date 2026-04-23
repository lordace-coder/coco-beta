package handlers

import (
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/instance"
	"github.com/patrick/cocobase/internal/services"
)

// Shared service instances used across all handlers
var (
	permChecker *services.PermissionChecker
)

func init() {
	permChecker = services.NewPermissionChecker()
}

// instanceID returns the single instance project ID.
func instanceID() string {
	return instance.ID()
}

// InvalidateCollectionCache clears cached collection lookups for an identifier.
func InvalidateCollectionCache(_, colID string) {
	collectionCache.Delete("col:" + instanceID() + ":" + colID)
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
