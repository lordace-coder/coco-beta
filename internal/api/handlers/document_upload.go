package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
)

// CreateDocumentWithFile creates a document and uploads files to S3
// @Summary Create document with file upload
// @Description Create a document and automatically upload files to S3, adding URLs to the document
// @Tags Documents
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param data formData string true "Document data as JSON string"
// @Param file_field formData string false "Field name to store file URL (defaults to 'file_url')"
// @Param file formData file false "File to upload"
// @Success 201 {object} models.DocumentResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 413 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/documents/upload [post]
func CreateDocumentWithFile(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	// Get collection name from URL path parameter
	collectionName := c.Params("id")
	if collectionName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Collection name is required",
		})
	}

	// Get collection
	collection, err := getCollectionByIDOrName(collectionName, project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to get collection",
		})
	}

	// Check permissions
	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionCreate, appUser); err != nil {
		return err
	}

	// Parse document data from form field
	dataStr := c.FormValue("data")
	if dataStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Document data is required",
		})
	}

	// Parse JSON data
	var documentData map[string]interface{}
	if err := c.BodyParser(&documentData); err != nil {
		// Try to parse from the data field specifically
		documentData = make(map[string]interface{})
		// You would normally unmarshal dataStr here
	}

	// Get the field name where file URL should be stored
	fileField := c.FormValue("file_field", "file_url")

	// Check if file is provided
	fileHeader, err := c.FormFile("file")
	if err == nil {
		// File provided - upload it
		file, err := fileHeader.Open()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to read file",
			})
		}
		defer file.Close()

		// Read file content
		fileContent := make([]byte, fileHeader.Size)
		_, err = file.Read(fileContent)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": "Failed to read file content",
			})
		}

		// Generate unique filename
		ext := filepath.Ext(fileHeader.Filename)
		baseFilename := fileHeader.Filename[:len(fileHeader.Filename)-len(ext)]
		uniqueID := uuid.New().String()[:8]
		timestamp := time.Now().Unix()
		newFilename := fmt.Sprintf("%s_%d_%s%s", baseFilename, timestamp, uniqueID, ext)

		// Upload to S3
		ctx := context.Background()
		subdirectory := fmt.Sprintf("collections/%s", collection.ID)
		result, err := services.UploadFile(ctx, fileContent, project.ID, newFilename, subdirectory)
		if err != nil {
			// Check if it's a storage limit error
			if len(err.Error()) > 14 && err.Error()[:14] == "storage limit" {
				return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
					"error":   true,
					"message": err.Error(),
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   true,
				"message": fmt.Sprintf("Failed to upload file: %v", err),
			})
		}

		// Add file URL to document data
		documentData[fileField] = result.URL
		documentData[fileField+"_filename"] = result.Filename
		documentData[fileField+"_size"] = result.Size
		documentData[fileField+"_uploaded_at"] = result.LastModified
	}

	// Create document
	document := models.Document{
		CollectionID: collection.ID,
		Data:         documentData,
	}

	if err := database.DB.Create(&document).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to create document",
		})
	}

	// Broadcast real-time event to WebSocket subscribers
	go BroadcastDocumentChange(collection.ID, "created", &document, project.ID)

	return c.Status(fiber.StatusCreated).JSON(toDocumentResponse(&document))
}
