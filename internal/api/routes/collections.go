package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers"
	"github.com/patrick/cocobase/internal/api/middleware"
)

// SetupCollectionRoutes sets up all collection-related routes
func SetupCollectionRoutes(app *fiber.App) {
	// All collection routes require API key authentication
	// Python API uses /collections not /api/collections
	collections := app.Group("/collections", middleware.RequireAPIKey)

	// Specific routes MUST come before /:id to avoid route conflicts
	{
		// Legacy document creation with query param (Python compatibility)
		collections.Post("/documents", handlers.CreateDocumentLegacy)

		// File management
		collections.Post("/file", handlers.UploadFile)
		collections.Get("/files", handlers.ListFiles)
		collections.Delete("/file", handlers.DeleteFile)

		// Collection CRUD
		collections.Post("/", handlers.CreateCollection)
	}

	// Collection-level advanced routes (MUST come before /:id)
	collectionAdvanced := app.Group("/collections", middleware.RequireAPIKey)
	{
		collectionAdvanced.Get("/:id/schema", handlers.GetCollectionSchema)
		collectionAdvanced.Get("/:id/export", handlers.ExportCollection)
		collectionAdvanced.Get("/:id/realtime/stats", handlers.GetRealtimeStats)
	}

	// NOTE: WebSocket /collections/:id/realtime is registered in realtime.go

	// Document routes (MUST come before /:id)
	documents := app.Group("/collections/:id/documents", middleware.RequireAPIKey)
	{
		// CRUD operations
		documents.Post("/upload", handlers.CreateDocumentWithFile)
		documents.Get("/", handlers.ListDocuments)
		documents.Get("/:docId", handlers.GetDocument)
		documents.Patch("/:docId", handlers.UpdateDocument)
		documents.Delete("/:docId", handlers.DeleteDocument)
	}

	// Dynamic /:id routes MUST come last
	collectionsById := app.Group("/collections", middleware.RequireAPIKey)
	{
		collectionsById.Get("/:id", handlers.GetCollection)
		collectionsById.Patch("/:id", handlers.UpdateCollection)
		collectionsById.Delete("/:id", handlers.DeleteCollection)
	}

	advancedDocumentRoutes := app.Group("/collections/:id/query/documents", middleware.RequireAPIKey)
	{
		// Advanced query routes (MUST come before /:docId to avoid route conflicts)
		advancedDocumentRoutes.Get("/count", handlers.CountDocuments)
		advancedDocumentRoutes.Get("/aggregate", handlers.AggregateDocuments)
		advancedDocumentRoutes.Get("/group-by", handlers.GroupByField)
		advancedDocumentRoutes.Get("/schema", handlers.GetCollectionSchema)
		advancedDocumentRoutes.Get("/export", handlers.ExportCollection)

		// Batch operations
		advancedDocumentRoutes.Post("/batch-create", handlers.BatchCreateDocuments)
		advancedDocumentRoutes.Post("/batch-update", handlers.BatchUpdateDocuments)
		advancedDocumentRoutes.Post("/batch-delete", handlers.BatchDeleteDocuments)

	}
}
