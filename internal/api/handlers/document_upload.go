package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
)

// uploadFileHeader uploads a single multipart file header and returns the public URL.
func uploadFileHeader(fh *multipart.FileHeader, projectID, collectionID string) (string, error) {
	file, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content := make([]byte, fh.Size)
	if _, err = file.Read(content); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	ext := filepath.Ext(fh.Filename)
	base := strings.TrimSuffix(fh.Filename, ext)
	newName := fmt.Sprintf("%s_%d_%s%s", base, time.Now().Unix(), uuid.New().String()[:8], ext)
	subdir := fmt.Sprintf("collections/%s", collectionID)

	result, err := services.UploadFile(context.Background(), content, projectID, newName, subdir)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

// processMultipartFiles reads all form fields from a multipart request.
// Non-file fields (except "data", "collection", "override") are ignored.
// File fields are uploaded and their URLs collected by field name.
// Returns a map of fieldName -> URL (string) or []URL ([]interface{}) when multiple files share a field name.
func processMultipartFiles(c *fiber.Ctx, projectID, collectionID string, skipFields ...string) (map[string]interface{}, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, fmt.Errorf("failed to parse multipart form: %w", err)
	}

	skip := map[string]bool{"data": true, "collection": true, "override": true}
	for _, f := range skipFields {
		skip[f] = true
	}

	result := map[string]interface{}{}

	for fieldName, headers := range form.File {
		if skip[fieldName] {
			continue
		}

		var urls []string
		for _, fh := range headers {
			url, err := uploadFileHeader(fh, projectID, collectionID)
			if err != nil {
				return nil, fmt.Errorf("upload failed for field %q: %w", fieldName, err)
			}
			urls = append(urls, url)
		}

		if len(urls) == 1 {
			result[fieldName] = urls[0]
		} else {
			arr := make([]interface{}, len(urls))
			for i, u := range urls {
				arr[i] = u
			}
			result[fieldName] = arr
		}
	}

	return result, nil
}

// CreateDocumentWithFile is called by CreateDocumentLegacy and CreateDocument when
// the request is multipart/form-data. It should NOT be registered as its own route —
// the regular create endpoints detect content-type and delegate here.
//
// Form fields:
//   - data        (optional) — document fields as JSON string
//   - collection  — (for legacy endpoint) collection name/ID
//   - <any>       — any other field whose value is a file gets uploaded; URL stored under that field name
func CreateDocumentWithFile(c *fiber.Ctx) error {

	collectionID := c.Params("id")
	if collectionID == "" {
		collectionID = c.Query("collection")
	}
	if collectionID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "collection is required"})
	}

	collection, err := getCollectionByIDOrName(collectionID, instanceID())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionCreate, appUser); err != nil {
		return err
	}

	// Parse base document data from the "data" form field
	documentData := map[string]interface{}{}
	if dataStr := c.FormValue("data", ""); dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), &documentData); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid JSON in 'data' field"})
		}
	}

	// Upload all file fields and merge into documentData
	fileData, err := processMultipartFiles(c, instanceID(), collection.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": err.Error()})
	}
	for k, v := range fileData {
		documentData[k] = v
	}

	if len(documentData) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Document data or files are required"})
	}

	document := models.Document{CollectionID: collection.ID, Data: documentData}
	if err := database.DB.Create(&document).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create document"})
	}

	go BroadcastDocumentChange(collection.ID, "created", &document, instanceID())
	return c.Status(fiber.StatusCreated).JSON(toDocumentResponse(&document))
}

// UpdateDocumentWithFile is called by UpdateDocument when the request is multipart/form-data.
// It should NOT be registered as its own route.
//
// Form fields:
//   - data     (optional) — fields to merge/replace as JSON string; supports $append/$remove ops
//   - override (optional) — "true" to fully replace data instead of merging
//   - <any>    — any other field whose value is a file gets uploaded; URL stored under that field name
func UpdateDocumentWithFile(c *fiber.Ctx) error {

	collectionID := c.Params("id")
	documentID := c.Params("docId")

	collection, err := getCollectionByIDOrName(collectionID, instanceID())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionUpdate, appUser); err != nil {
		return err
	}

	var document models.Document
	if err := database.DB.Where("id = ? AND collection_id = ?", documentID, collection.ID).First(&document).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Document not found"})
	}

	// Parse update data from the "data" form field
	updateData := map[string]interface{}{}
	if dataStr := c.FormValue("data", ""); dataStr != "" {
		if err := json.Unmarshal([]byte(dataStr), &updateData); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid JSON in 'data' field"})
		}
	}

	// Upload all file fields and merge into updateData
	fileData, err := processMultipartFiles(c, instanceID(), collection.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": err.Error()})
	}
	for k, v := range fileData {
		updateData[k] = v
	}

	if len(updateData) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "No data or files provided for update"})
	}

	override := c.FormValue("override", "false") == "true"
	existing := map[string]interface{}{}
	if document.Data != nil {
		for k, v := range document.Data {
			existing[k] = v
		}
	}

	if override {
		existing = updateData
	} else {
		// Extract special array operations
		appendOps, _ := updateData["$append"].(map[string]interface{})
		removeOps, _ := updateData["$remove"].(map[string]interface{})
		delete(updateData, "$append")
		delete(updateData, "$remove")

		// 1. Apply $remove
		for field, items := range removeOps {
			arr, ok := existing[field].([]interface{})
			if !ok {
				continue
			}
			toRemove, ok := items.([]interface{})
			if !ok {
				toRemove = []interface{}{items}
			}
			removeSet := map[interface{}]bool{}
			for _, v := range toRemove {
				removeSet[v] = true
			}
			filtered := []interface{}{}
			for _, v := range arr {
				if !removeSet[v] {
					filtered = append(filtered, v)
				}
			}
			existing[field] = filtered
		}

		// 2. Apply $append
		for field, items := range appendOps {
			toAdd, ok := items.([]interface{})
			if !ok {
				toAdd = []interface{}{items}
			}
			arr, ok := existing[field].([]interface{})
			if !ok {
				arr = []interface{}{}
			}
			existing[field] = arr
			existSet := map[interface{}]bool{}
			for _, v := range arr {
				existSet[v] = true
			}
			for _, v := range toAdd {
				if !existSet[v] {
					existing[field] = append(existing[field].([]interface{}), v)
				}
			}
		}

		// 3. Merge regular fields
		for k, v := range updateData {
			existing[k] = v
		}
	}

	document.Data = existing
	if err := database.DB.Save(&document).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update document"})
	}

	go BroadcastDocumentChange(collection.ID, "updated", &document, instanceID())
	return c.JSON(toDocumentResponse(&document))
}
