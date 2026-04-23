package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	fnservice "github.com/patrick/cocobase/internal/services/functions"
)

// checkSentinel evaluates a sentinel expression for the current request/doc.
// Returns a 403 Fiber error if denied, or nil if access is granted.
func checkSentinel(c *fiber.Ctx, expr string, user *models.AppUser, doc map[string]interface{}) error {
	if expr == "" {
		return nil
	}
	ctx := services.SentinelContext{
		User:   user,
		Doc:    doc,
		IP:     c.IP(),
		Method: c.Method(),
	}
	granted, err := services.EvalSentinel(expr, ctx)
	if err != nil || !granted {
		return fiber.NewError(fiber.StatusForbidden, "Access denied by sentinel rule")
	}
	return nil
}

// TODO: Invalidate collectionCache after collection renames/deletes from dashboard

var (
	queryBuilder         = services.NewQueryBuilder(database.DB)
	relationshipResolver = services.NewRelationshipResolver()
	collectionCache      sync.Map
)

// reserved params — never treated as filters
var reservedParams = []string{
	"limit", "offset", "sort", "order",
	"populate", "select", "id", "count",
}

// ─────────────────────────────────────────
// CreateDocumentLegacy
// ─────────────────────────────────────────

// @Summary Create a document (legacy)
// @Tags Documents
// @Accept json
// @Produce json
// @Param collection query string true "Collection ID or Name"
// @Param document body models.DocumentCreateRequest true "Document data"
// @Success 200 {object} models.DocumentResponse
// @Security ApiKeyAuth
// @Router /collections/documents [post]
// CreateDocument handles POST /collections/:id/documents (JSON or multipart)
func CreateDocument(c *fiber.Ctx) error {
	if strings.Contains(c.Get("Content-Type"), "multipart/form-data") {
		return CreateDocumentWithFile(c)
	}


	collectionID := c.Params("id")
	collection, err := getCollectionByIDOrName(collectionID, instanceID())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionCreate, appUser); err != nil {
		return err
	}

	var req models.DocumentCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}
	if len(req.Data) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Document data is required"})
	}

	// Sentinel check for create
	if err := checkSentinel(c, collection.Sentinels.Create, appUser, req.Data); err != nil {
		return err
	}

	// beforeCreate hook — can cancel
	if cancelled, msg := fnservice.DispatchHook(models.HookBeforeCreate, instanceID(), collection, req.Data, appUser, BroadcastToProject); cancelled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": msg})
	}

	// pre_save webhook (fire-and-forget — does not block create)
	services.FireWebhook(collection.Webhooks.PreSave, "pre_save", collection.ID, "", req.Data)

	document := models.Document{CollectionID: collection.ID, Data: req.Data}
	if err := database.DB.Create(&document).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create document"})
	}

	// post_save webhook
	services.FireWebhook(collection.Webhooks.PostSave, "post_save", collection.ID, document.ID, map[string]interface{}(document.Data))

	// afterCreate hook — fire-and-forget
	go fnservice.DispatchHook(models.HookAfterCreate, instanceID(), collection, map[string]interface{}(document.Data), appUser, BroadcastToProject)

	go BroadcastDocumentChange(collection.ID, "created", &document, instanceID())
	return c.Status(fiber.StatusCreated).JSON(toDocumentResponse(&document))
}

// CreateDocumentLegacy handles POST /collections/documents?collection=name (legacy query-param style)
func CreateDocumentLegacy(c *fiber.Ctx) error {
	// Detect multipart — delegate to file handler (same as Python)
	if strings.Contains(c.Get("Content-Type"), "multipart/form-data") {
		return CreateDocumentWithFile(c)
	}


	collectionName := c.Query("collection")
	if collectionName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Collection name is required"})
	}

	var req models.DocumentCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}
	if len(req.Data) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Document data is required"})
	}

	collection, err := getCollectionByIDOrName(collectionName, instanceID())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to get collection"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionCreate, appUser); err != nil {
		return err
	}

	document := models.Document{CollectionID: collection.ID, Data: req.Data}
	if err := database.DB.Create(&document).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create document"})
	}

	return c.Status(fiber.StatusOK).JSON(toDocumentResponse(&document))
}

// ─────────────────────────────────────────
// ListDocuments
// ─────────────────────────────────────────

// @Summary List documents
// @Tags Documents
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param limit query int false "Page size (max 1000)" default(100)
// @Param offset query int false "Skip N" default(0)
// @Param sort query string false "Sort field" default(created_at)
// @Param order query string false "asc|desc" default(desc)
// @Param populate query string false "Fields to populate e.g. product,user"
// @Param select query string false "Dot-notation fields to return"
// @Success 200 {array} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/documents [get]
func ListDocuments(c *fiber.Ctx) error {

	collectionID := c.Params("id")

	// ── 1. Collection lookup (cached) ────────────────────────────────────
	cacheKey := fmt.Sprintf("col:%s:%s", instanceID(), collectionID)
	collection, err := getCollectionCached(cacheKey, collectionID, instanceID())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	// ── 2. Permission check ──────────────────────────────────────────────
	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	// ── 3. Parse & sanitize params ───────────────────────────────────────
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	populate := c.Query("populate")
	selectFields := c.Query("select")

	// Sanitize sort: must be a plain string field name, not an object/JSON
	sort := c.Query("sort", "created_at")
	if strings.ContainsAny(sort, "{}[]") || strings.Contains(sort, "object") {
		sort = "created_at"
	}
	order := c.Query("order", "desc")
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	if limit > 1000 {
		limit = 1000
	}
	if limit <= 0 {
		limit = 100
	}

	// ── 4. Collect raw query params — skip all reserved keys ─────────────
	rawParams := make(map[string][]string)
	c.Request().URI().QueryArgs().VisitAll(func(k, v []byte) {
		key := string(k)
		// Skip reserved params so they never bleed into filters
		for _, r := range reservedParams {
			if key == r {
				return
			}
		}
		rawParams[key] = []string{string(v)}
	})

	// ── 5. Separate regular vs relationship filters ──────────────────────
	regular, relationship := services.ParseRelationshipFilters(rawParams, reservedParams)

	// ── 6. Build query ───────────────────────────────────────────────────
	query := database.DB.Model(&models.Document{}).
		Where("collection_id = ?", collection.ID)

	if len(regular) > 0 {
		regularParams := make(map[string][]string, len(regular))
		for k, v := range regular {
			regularParams[k] = []string{v}
		}
		query = queryBuilder.BuildQuery(query, regularParams, reservedParams)
	}

	if len(relationship) > 0 {
		query = queryBuilder.ApplyRelationshipFilters(query, relationship, instanceID())
	}

	query = queryBuilder.ApplySorting(query, sort, order)
	query = queryBuilder.ApplyPagination(query, limit, offset)

	// ── 7. Execute ───────────────────────────────────────────────────────
	var documents []models.Document
	if err := query.Find(&documents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch documents"})
	}

	if len(documents) == 0 {
		return c.JSON([]interface{}{})
	}

	// ── 8. Convert to maps ───────────────────────────────────────────────
	results := make([]map[string]interface{}, 0, len(documents))
	for i := range documents {
		m := documentToMap(&documents[i])
		// ── Sentinel filter for list ─────────────────────────────────────
		if collection.Sentinels.List != "" {
			ok, _ := services.EvalSentinel(collection.Sentinels.List, services.SentinelContext{User: appUser, Doc: m, IP: c.IP(), Method: c.Method()})
			if !ok {
				continue // exclude this document from results
			}
		}
		results = append(results, m)
	}

	// ── 9. Populate / select ─────────────────────────────────────────────
	populateRequests := services.ParsePopulateParams(populate, selectFields)
	if len(populateRequests) > 0 {
		_ = relationshipResolver.PopulateDocuments(results, instanceID(), populateRequests)
	} else if selectFields != "" {
		selectList := strings.Split(selectFields, ",")
		for i := range results {
			results[i] = services.SelectFields(results[i], selectList)
		}
	}

	return c.JSON(results)
}

// ─────────────────────────────────────────
// GetDocument
// ─────────────────────────────────────────

// @Summary Get document
// @Tags Documents
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param docId path string true "Document ID"
// @Param populate query string false "Fields to populate"
// @Param select query string false "Fields to select"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/documents/{docId} [get]
func GetDocument(c *fiber.Ctx) error {

	collectionID := c.Params("id")
	documentID := c.Params("doc_id")

	collection, err := getCollectionByIDOrName(collectionID, instanceID())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	var document models.Document
	if err := database.DB.Where("id = ? AND collection_id = ?", documentID, collection.ID).
		First(&document).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Document not found"})
	}

	// Sentinel check for view
	if err := checkSentinel(c, collection.Sentinels.View, appUser, map[string]interface{}(document.Data)); err != nil {
		return err
	}

	populate := c.Query("populate")
	selectFields := c.Query("select")
	populateRequests := services.ParsePopulateParams(populate, selectFields)

	result := documentToMap(&document)

	if len(populateRequests) > 0 {
		results := []map[string]interface{}{result}
		_ = relationshipResolver.PopulateDocuments(results, instanceID(), populateRequests)
		result = results[0]
	} else if selectFields != "" {
		result = services.SelectFields(result, strings.Split(selectFields, ","))
	}

	return c.JSON(result)
}

// ─────────────────────────────────────────
// UpdateDocument
// ─────────────────────────────────────────

// @Summary Update document
// @Tags Documents
// @Accept json
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param docId path string true "Document ID"
// @Param document body models.DocumentUpdateRequest true "Updates"
// @Success 200 {object} models.DocumentResponse
// @Security ApiKeyAuth
// @Router /collections/{id}/documents/{docId} [patch]
func UpdateDocument(c *fiber.Ctx) error {
	// Detect multipart — delegate to file handler (same as Python)
	if strings.Contains(c.Get("Content-Type"), "multipart/form-data") {
		return UpdateDocumentWithFile(c)
	}


	collectionID := c.Params("id")
	documentID := c.Params("doc_id")

	collection, err := getCollectionByIDOrName(collectionID, instanceID())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionUpdate, appUser); err != nil {
		return err
	}

	var document models.Document
	if err := database.DB.Where("id = ? AND collection_id = ?", documentID, collection.ID).
		First(&document).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Document not found"})
	}

	// Sentinel check for update (evaluated on existing data)
	if err := checkSentinel(c, collection.Sentinels.Update, appUser, map[string]interface{}(document.Data)); err != nil {
		return err
	}

	var req struct {
		models.DocumentUpdateRequest
		Override bool `json:"override"` // if true, Data fully replaces existing data
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	if req.Data != nil {
		if req.Override {
			document.Data = req.Data
		} else {
			for key, value := range req.Data {
				document.Data[key] = value
			}
		}
	}

	// beforeUpdate hook — can cancel
	if cancelled, msg := fnservice.DispatchHook(models.HookBeforeUpdate, instanceID(), collection, map[string]interface{}(document.Data), appUser, BroadcastToProject); cancelled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": msg})
	}

	// pre_save webhook
	services.FireWebhook(collection.Webhooks.PreSave, "pre_save", collection.ID, document.ID, map[string]interface{}(document.Data))

	if err := database.DB.Save(&document).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update document"})
	}

	// post_save webhook
	services.FireWebhook(collection.Webhooks.PostSave, "post_save", collection.ID, document.ID, map[string]interface{}(document.Data))

	// afterUpdate hook — fire-and-forget
	go fnservice.DispatchHook(models.HookAfterUpdate, instanceID(), collection, map[string]interface{}(document.Data), appUser, BroadcastToProject)

	go BroadcastDocumentChange(collection.ID, "updated", &document, instanceID())

	return c.JSON(toDocumentResponse(&document))
}

// ─────────────────────────────────────────
// DeleteDocument
// ─────────────────────────────────────────

// @Summary Delete document
// @Tags Documents
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param docId path string true "Document ID"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/documents/{docId} [delete]
func DeleteDocument(c *fiber.Ctx) error {

	collectionID := c.Params("id")
	documentID := c.Params("doc_id")

	collection, err := getCollectionByIDOrName(collectionID, instanceID())
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionDelete, appUser); err != nil {
		return err
	}

	var document models.Document
	if err := database.DB.Where("id = ? AND collection_id = ?", documentID, collection.ID).
		First(&document).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Document not found"})
	}

	// Sentinel check for delete
	if err := checkSentinel(c, collection.Sentinels.Delete, appUser, map[string]interface{}(document.Data)); err != nil {
		return err
	}

	// beforeDelete hook — can cancel
	if cancelled, msg := fnservice.DispatchHook(models.HookBeforeDelete, instanceID(), collection, map[string]interface{}(document.Data), appUser, BroadcastToProject); cancelled {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": msg})
	}

	// pre_delete webhook
	services.FireWebhook(collection.Webhooks.PreDelete, "pre_delete", collection.ID, document.ID, map[string]interface{}(document.Data))

	if err := database.DB.Delete(&document).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to delete document"})
	}

	// Remove any locally-stored files referenced by this document.
	go services.DeleteDocumentFiles(map[string]interface{}(document.Data))

	// Evict cache on delete
	cacheKey := fmt.Sprintf("col:%s:%s", instanceID(), collectionID)
	collectionCache.Delete(cacheKey)

	// post_delete webhook
	services.FireWebhook(collection.Webhooks.PostDelete, "post_delete", collection.ID, document.ID, map[string]interface{}(document.Data))

	// afterDelete hook — fire-and-forget
	go fnservice.DispatchHook(models.HookAfterDelete, instanceID(), collection, map[string]interface{}(document.Data), appUser, BroadcastToProject)

	go BroadcastDocumentChange(collection.ID, "deleted", &document, instanceID())

	return c.JSON(fiber.Map{"status": "success", "message": "Document deleted successfully"})
}

// ─────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────

func getCollectionCached(cacheKey, identifier, projectID string) (*models.Collection, error) {
	if cached, ok := collectionCache.Load(cacheKey); ok {
		return cached.(*models.Collection), nil
	}
	collection, err := getCollectionByIDOrName(identifier, projectID)
	if err != nil {
		return nil, err
	}
	collectionCache.Store(cacheKey, collection)
	time.AfterFunc(5*time.Minute, func() {
		collectionCache.Delete(cacheKey)
	})
	return collection, nil
}

func calculateWorkers(docCount int) int {
	switch {
	case docCount <= 50:
		return 2
	case docCount <= 200:
		return 4
	case docCount <= 500:
		return 8
	default:
		return 16
	}
}

func toDocumentResponse(document *models.Document) models.DocumentResponse {
	return models.DocumentResponse{
		ID:           document.ID,
		CollectionID: document.CollectionID,
		Data:         document.Data,
		CreatedAt:    document.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func documentToMap(document *models.Document) map[string]interface{} {
	return map[string]interface{}{
		"id":            document.ID,
		"collection_id": document.CollectionID,
		"created_at":    document.CreatedAt.Format("2006-01-02T15:04:05.999999"),
		"data":          document.Data,
	}
}