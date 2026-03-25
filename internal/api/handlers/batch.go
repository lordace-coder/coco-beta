package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
)

// BatchCreateDocuments creates multiple documents at once
// @Summary Batch create documents
// @Description Create multiple documents in a single transaction. Optimized for bulk inserts.
// @Tags Batch Operations
// @Accept json
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param documents body models.BatchCreateRequest true "Documents to create"
// @Success 200 {array} models.DocumentResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/documents/batch-create [post]
func BatchCreateDocuments(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	collectionID := c.Params("id")

	// Get collection
	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Collection not found",
		})
	}

	// Check permissions
	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionCreate, appUser); err != nil {
		return err
	}

	// Parse request body
	var req models.BatchCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if len(req.Documents) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "No documents provided",
		})
	}

	if len(req.Documents) > 1000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Maximum 1000 documents per batch",
		})
	}

	// Create documents in transaction using bulk insert for better performance
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Pre-allocate slice with exact capacity
	createdDocuments := make([]models.Document, 0, len(req.Documents))

	// Prepare all documents first
	for _, docData := range req.Documents {
		if len(docData) == 0 {
			continue
		}

		document := models.Document{
			CollectionID: collection.ID,
			Data:         docData,
		}
		createdDocuments = append(createdDocuments, document)
	}

	// Bulk insert for performance (single INSERT with multiple VALUES)
	if len(createdDocuments) > 0 {
		// Use CreateInBatches for very large batches (splits into chunks)
		batchSize := 100
		if err := tx.CreateInBatches(&createdDocuments, batchSize).Error; err != nil {
			tx.Rollback()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to create documents",
			})
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to commit transaction",
		})
	}

	// Convert to response
	responses := make([]models.DocumentResponse, len(createdDocuments))
	for i, doc := range createdDocuments {
		responses[i] = toDocumentResponse(&doc)
	}

	return c.Status(fiber.StatusCreated).JSON(responses)
}

// BatchUpdateDocuments updates multiple documents at once
// @Summary Batch update documents
// @Description Update multiple documents at once. Batch operation reduces queries.
// @Tags Batch Operations
// @Accept json
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param updates body models.BatchUpdateRequest true "Document updates"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/documents/batch-update [post]
func BatchUpdateDocuments(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	collectionID := c.Params("id")

	// Get collection
	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Collection not found",
		})
	}

	// Check permissions
	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionUpdate, appUser); err != nil {
		return err
	}

	// Parse request body
	var req models.BatchUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if len(req.Updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "No updates provided",
		})
	}

	// Get all document IDs
	docIDs := make([]string, 0, len(req.Updates))
	for _, update := range req.Updates {
		docIDs = append(docIDs, update.ID)
	}

	// Fetch all documents in one query
	var documents []models.Document
	if err := database.DB.Where("id IN ? AND collection_id = ?", docIDs, collection.ID).
		Find(&documents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to fetch documents",
		})
	}

	// Update documents in transaction
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	updatedCount := 0
	for i := range documents {
		doc := &documents[i]
		// Find matching update
		for _, update := range req.Updates {
			if update.ID == doc.ID {
				// Merge data
				for key, value := range update.Data {
					doc.Data[key] = value
				}
				if err := tx.Save(doc).Error; err != nil {
					tx.Rollback()
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error":   true,
						"message": "Failed to update documents",
					})
				}
				updatedCount++
				break
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to commit transaction",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Documents updated successfully",
		"count":   updatedCount,
	})
}

// BatchDeleteDocuments deletes multiple documents at once
// @Summary Batch delete documents
// @Description Delete multiple documents at once. Single query for optimal performance.
// @Tags Batch Operations
// @Accept json
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param deletes body models.BatchDeleteRequest true "Document IDs to delete"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/documents/batch-delete [post]
func BatchDeleteDocuments(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	collectionID := c.Params("id")

	// Get collection
	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Collection not found",
		})
	}

	// Check permissions
	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionDelete, appUser); err != nil {
		return err
	}

	// Parse request body
	var req models.BatchDeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "No document IDs provided",
		})
	}

	// Delete documents
	result := database.DB.Where("id IN ? AND collection_id = ?", req.IDs, collection.ID).
		Delete(&models.Document{})

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to delete documents",
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Documents deleted successfully",
		"count":   result.RowsAffected,
	})
}
