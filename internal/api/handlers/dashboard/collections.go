package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"gorm.io/gorm"
)

// CreateCollection handles POST /_/api/projects/:id/collections
func CreateCollection(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "name is required"})
	}

	col := models.Collection{ProjectID: projectID, Name: req.Name}
	if err := database.DB.Create(&col).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create collection"})
	}

	Log(projectID, "create_collection", "collection", col.ID, col.Name)
	return c.Status(fiber.StatusCreated).JSON(col)
}

// CreateDocument handles POST /_/api/projects/:id/collections/:colId/documents (dashboard)
func CreateDocumentDashboard(c *fiber.Ctx) error {
	col, err := getCollection(c.Params("id"), c.Params("colId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	var req struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := c.BodyParser(&req); err != nil || req.Data == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "data is required"})
	}

	doc := models.Document{CollectionID: col.ID, Data: req.Data}
	if err := database.DB.Create(&doc).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create document"})
	}

	Log(c.Params("id"), "create_document", "document", doc.ID, col.Name)
	return c.Status(fiber.StatusCreated).JSON(doc)
}

// ListCollections handles GET /_/api/projects/:id/collections
func ListCollections(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var collections []models.Collection
	if err := database.DB.Where("project_id = ?", projectID).Order("created_at desc").Find(&collections).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch collections"})
	}
	return c.JSON(fiber.Map{"data": collections, "total": len(collections)})
}

// GetCollection handles GET /_/api/projects/:id/collections/:colId
func GetCollection(c *fiber.Ctx) error {
	col, err := getCollection(c.Params("id"), c.Params("colId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	// Count documents
	var docCount int64
	database.DB.Model(&models.Document{}).Where("collection_id = ?", col.ID).Count(&docCount)

	return c.JSON(fiber.Map{
		"id":             col.ID,
		"name":           col.Name,
		"project_id":     col.ProjectID,
		"created_at":     col.CreatedAt,
		"document_count": docCount,
	})
}

// DeleteCollection handles DELETE /_/api/projects/:id/collections/:colId
func DeleteCollection(c *fiber.Ctx) error {
	col, err := getCollection(c.Params("id"), c.Params("colId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	// Delete all documents first
	database.DB.Where("collection_id = ?", col.ID).Delete(&models.Document{})

	if err := database.DB.Delete(col).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to delete collection"})
	}
	Log(c.Params("id"), "delete_collection", "collection", col.ID, col.Name)
	return c.JSON(fiber.Map{"message": "Collection deleted"})
}

// ListDocuments handles GET /_/api/projects/:id/collections/:colId/documents
func ListDocuments(c *fiber.Ctx) error {
	col, err := getCollection(c.Params("id"), c.Params("colId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	sort := c.Query("sort", "created_at")
	order := c.Query("order", "desc")
	if limit > 500 {
		limit = 500
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	var total int64
	database.DB.Model(&models.Document{}).Where("collection_id = ?", col.ID).Count(&total)

	var docs []models.Document
	orderClause := "created_at " + order
	if sort == "created_at" || sort == "updated_at" {
		orderClause = sort + " " + order
	}

	if err := database.DB.Where("collection_id = ?", col.ID).
		Order(orderClause).Limit(limit).Offset(offset).Find(&docs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch documents"})
	}

	return c.JSON(fiber.Map{
		"data":     docs,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"has_more": int64(offset+limit) < total,
	})
}

// GetDocument handles GET /_/api/projects/:id/collections/:colId/documents/:docId
func GetDocument(c *fiber.Ctx) error {
	col, err := getCollection(c.Params("id"), c.Params("colId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	var doc models.Document
	if err := database.DB.Where("id = ? AND collection_id = ?", c.Params("docId"), col.ID).First(&doc).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Document not found"})
	}
	return c.JSON(doc)
}

// UpdateDocument handles PATCH /_/api/projects/:id/collections/:colId/documents/:docId
func UpdateDocument(c *fiber.Ctx) error {
	col, err := getCollection(c.Params("id"), c.Params("colId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	var doc models.Document
	if err := database.DB.Where("id = ? AND collection_id = ?", c.Params("docId"), col.ID).First(&doc).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Document not found"})
	}

	var req struct {
		Data     map[string]interface{} `json:"data"`
		Override bool                   `json:"override"`
	}
	if err := c.BodyParser(&req); err != nil || req.Data == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "data is required"})
	}

	if req.Override {
		doc.Data = req.Data
	} else {
		for k, v := range req.Data {
			doc.Data[k] = v
		}
	}

	if err := database.DB.Save(&doc).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update document"})
	}
	return c.JSON(doc)
}

// DeleteDocument handles DELETE /_/api/projects/:id/collections/:colId/documents/:docId
func DeleteDocument(c *fiber.Ctx) error {
	col, err := getCollection(c.Params("id"), c.Params("colId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	if err := database.DB.Where("id = ? AND collection_id = ?", c.Params("docId"), col.ID).
		Delete(&models.Document{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to delete document"})
	}
	Log(c.Params("id"), "delete_document", "document", c.Params("docId"), col.Name)
	return c.JSON(fiber.Map{"message": "Document deleted"})
}

func getCollection(projectID, colID string) (*models.Collection, error) {
	var col models.Collection
	err := database.DB.Where("(id = ? OR name = ?) AND project_id = ?", colID, colID, projectID).First(&col).Error
	if err != nil {
		return nil, err
	}
	return &col, nil
}

// needed by gorm delete
func init() {
	_ = gorm.ErrRecordNotFound
}
