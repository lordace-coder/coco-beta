package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"gorm.io/gorm"
)

// CreateCollection creates a new collection
// @Summary Create a new collection
// @Description Create a new collection with optional permissions
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection body models.CollectionCreateRequest true "Collection data"
// @Success 201 {object} models.CollectionResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections [post]
func CreateCollection(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	// Parse request body
	var req models.CollectionCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	// Check if collection with same name exists
	var existing models.Collection
	err := database.DB.Where("name = ? AND project_id = ?", req.Name, project.ID).First(&existing).Error
	if err == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "A collection with this name already exists in this project",
		})
	}

	// Create new collection
	collection := models.Collection{
		Name:       req.Name,
		ProjectID:  project.ID,
		WebhookURL: req.WebhookURL,
	}

	// Set webhooks
	if req.Webhooks != nil {
		collection.Webhooks = models.Webhooks{
			PreSave:    req.Webhooks.PreSave,
			PostSave:   req.Webhooks.PostSave,
			PreDelete:  req.Webhooks.PreDelete,
			PostDelete: req.Webhooks.PostDelete,
		}
	}

	// Set permissions
	if req.Permissions != nil {
		collection.Permissions = models.Permissions{
			Create: req.Permissions.Create,
			Read:   req.Permissions.Read,
			Update: req.Permissions.Update,
			Delete: req.Permissions.Delete,
		}
	}

	// Set sentinels
	if req.Sentinels != nil {
		collection.Sentinels = models.Sentinels{
			List:   req.Sentinels.List,
			View:   req.Sentinels.View,
			Create: req.Sentinels.Create,
			Update: req.Sentinels.Update,
			Delete: req.Sentinels.Delete,
		}
	}

	if err := database.DB.Create(&collection).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create collection",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(toCollectionResponse(&collection))
}

// GetCollection retrieves a collection by ID or name
// @Summary Get a collection
// @Description Retrieve a collection by its ID or name
// @Tags Collections
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Success 200 {object} models.CollectionResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id} [get]
func GetCollection(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	collectionID := c.Params("id")

	// Try to get collection
	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Collection not found",
		})
	}

	// Check permissions
	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	return c.JSON(toCollectionResponse(collection))
}

// UpdateCollection updates a collection
// @Summary Update collection
// @Description Update a collection. Use helper function, invalidate cache after update.
// @Tags Collections
// @Accept json
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param collection body models.CollectionUpdateRequest true "Update data"
// @Success 200 {object} models.CollectionResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id} [patch]
func UpdateCollection(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	collectionID := c.Params("id")

	// Find collection by ID or name
	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Collection not found",
		})
	}

	// Parse request body
	var req models.CollectionUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Invalid request body",
		})
	}

	// Update name if provided
	if req.Name != nil && *req.Name != collection.Name {
		// Check for name conflicts
		var existing models.Collection
		err := database.DB.Where("name = ? AND project_id = ? AND id != ?", *req.Name, project.ID, collection.ID).
			First(&existing).Error
		if err == nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "A collection with this name already exists in this project",
			})
		}
		collection.Name = *req.Name
	}

	// Update webhooks if provided
	if req.Webhooks != nil {
		collection.Webhooks = models.Webhooks{
			PreSave:    req.Webhooks.PreSave,
			PostSave:   req.Webhooks.PostSave,
			PreDelete:  req.Webhooks.PreDelete,
			PostDelete: req.Webhooks.PostDelete,
		}
	}

	// Update permissions if provided
	if req.Permissions != nil {
		collection.Permissions = models.Permissions{
			Create: req.Permissions.Create,
			Read:   req.Permissions.Read,
			Update: req.Permissions.Update,
			Delete: req.Permissions.Delete,
		}
	}

	// Update sentinels if provided
	if req.Sentinels != nil {
		collection.Sentinels = models.Sentinels{
			List:   req.Sentinels.List,
			View:   req.Sentinels.View,
			Create: req.Sentinels.Create,
			Update: req.Sentinels.Update,
			Delete: req.Sentinels.Delete,
		}
	}

	if err := database.DB.Save(collection).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to update collection",
		})
	}

	return c.JSON(toCollectionResponse(collection))
}

// DeleteCollection deletes a collection
// @Summary Delete a collection
// @Description Delete a collection and all its documents
// @Tags Collections
// @Param id path string true "Collection ID or Name"
// @Success 204
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id} [delete]
func DeleteCollection(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	collectionID := c.Params("id")

	// Find collection
	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "Collection not found",
		})
	}

	// Delete collection (cascades to documents)
	if err := database.DB.Delete(&collection).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to delete collection",
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Helper functions

// getCollectionByIDOrName retrieves a collection by ID or name
func getCollectionByIDOrName(identifier, projectID string) (*models.Collection, error) {
	var collection models.Collection
	err := database.DB.Where("(id = ? OR name = ?) AND project_id = ?", identifier, identifier, projectID).
		First(&collection).Error

	// If not found, create it (auto-create on first use)
	if err == gorm.ErrRecordNotFound {
		collection = models.Collection{
			Name:      identifier,
			ProjectID: projectID,
			Permissions: models.Permissions{
				Create: []string{},
				Read:   []string{},
				Update: []string{},
				Delete: []string{},
			},
		}
		if createErr := database.DB.Create(&collection).Error; createErr != nil {
			return nil, createErr
		}
		return &collection, nil
	}

	if err != nil {
		return nil, err
	}

	return &collection, nil
}

// toCollectionResponse converts a Collection model to response format
func toCollectionResponse(collection *models.Collection) models.CollectionResponse {
	return models.CollectionResponse{
		ID:          collection.ID,
		Name:        collection.Name,
		ProjectID:   collection.ProjectID,
		WebhookURL:  collection.WebhookURL,
		Webhooks:    collection.Webhooks,
		Permissions: collection.Permissions,
		Sentinels:   collection.Sentinels,
		CreatedAt:   collection.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
