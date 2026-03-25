package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
)

// advancedReserved — keys that must never become JSONB field filters.
// Shared across all advanced handlers via buildFilterParams.
var advancedReserved = map[string]bool{
	"limit": true, "offset": true, "sort": true, "order": true,
	"populate": true, "select": true, "id": true, "count": true,
	"field": true, "operation": true, "count_field": true, "format": true,
}

// buildFilterParams collects query params, skipping all reserved keys.
func buildFilterParams(c *fiber.Ctx) map[string][]string {
	params := make(map[string][]string)
	c.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		k := string(key)
		if !advancedReserved[k] {
			params[k] = []string{string(value)}
		}
	})
	return params
}

// ─────────────────────────────────────────
// CountDocuments
// ─────────────────────────────────────────

// @Summary Count documents
// @Tags Advanced Queries
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Success 200 {object} models.CountResponse
// @Security ApiKeyAuth
// @Router /collections/{id}/query/documents/count [get]
func CountDocuments(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	collectionID := c.Params("id")

	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	query := database.DB.Model(&models.Document{}).Where("collection_id = ?", collection.ID)

	// buildFilterParams skips populate, select, and all other reserved keys
	params := buildFilterParams(c)
	if len(params) > 0 {
		query = queryBuilder.BuildQuery(query, params, reservedParams)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to count documents"})
	}

	return c.JSON(models.CountResponse{Count: count})
}

// ─────────────────────────────────────────
// AggregateDocuments
// ─────────────────────────────────────────

// @Summary Aggregate documents
// @Tags Advanced Queries
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param field query string true "Field to aggregate"
// @Param operation query string false "count|sum|avg|min|max" default(count)
// @Success 200 {object} models.AggregateResponse
// @Security ApiKeyAuth
// @Router /collections/{id}/query/documents/aggregate [get]
func AggregateDocuments(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	collectionID := c.Params("id")
	field := c.Query("field")
	operation := c.Query("operation", "count")

	if field == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Field parameter is required"})
	}

	validOps := map[string]bool{"count": true, "sum": true, "avg": true, "min": true, "max": true}
	if !validOps[operation] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid operation. Must be: count, sum, avg, min, or max"})
	}

	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	query := database.DB.Model(&models.Document{}).Where("collection_id = ?", collection.ID)

	params := buildFilterParams(c)
	if len(params) > 0 {
		query = queryBuilder.BuildQuery(query, params, reservedParams)
	}

	var result interface{}
	var err2 error

	switch operation {
	case "count":
		var count int64
		err2 = query.Where("data->>? IS NOT NULL", field).Count(&count).Error
		result = count
	case "sum":
		var sum float64
		err2 = query.Select("SUM((data->>?)::numeric) as sum", field).Scan(&sum).Error
		result = sum
	case "avg":
		var avg float64
		err2 = query.Select("AVG((data->>?)::numeric) as avg", field).Scan(&avg).Error
		result = avg
	case "min":
		var min float64
		err2 = query.Select("MIN((data->>?)::numeric) as min", field).Scan(&min).Error
		result = min
	case "max":
		var max float64
		err2 = query.Select("MAX((data->>?)::numeric) as max", field).Scan(&max).Error
		result = max
	}

	if err2 != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Aggregation failed. Ensure the field contains numeric values"})
	}

	return c.JSON(models.AggregateResponse{
		Field: field, Operation: operation, Result: result, Collection: collection.Name,
	})
}

// ─────────────────────────────────────────
// GroupByField
// ─────────────────────────────────────────

// @Summary Group by field
// @Tags Advanced Queries
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Param field query string true "Field to group by"
// @Success 200 {array} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/{id}/query/documents/group-by [get]
func GroupByField(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	collectionID := c.Params("id")
	field := c.Query("field")

	if field == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Field parameter is required"})
	}

	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	query := database.DB.Model(&models.Document{}).Where("collection_id = ?", collection.ID)

	params := buildFilterParams(c)
	if len(params) > 0 {
		query = queryBuilder.BuildQuery(query, params, reservedParams)
	}

	type GroupResult struct {
		Value string
		Count int64
	}

	var results []GroupResult
	err = query.Select("data->>? as value, COUNT(*) as count", field).
		Group("value").Order("count DESC").Scan(&results).Error

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Group by failed"})
	}

	response := make([]map[string]interface{}, 0, len(results))
	for _, r := range results {
		if r.Value != "" {
			response = append(response, map[string]interface{}{field: r.Value, "count": r.Count})
		}
	}

	return c.JSON(response)
}

// ─────────────────────────────────────────
// GetCollectionSchema
// ─────────────────────────────────────────

// @Summary Get collection schema
// @Tags Advanced Queries
// @Produce json
// @Param id path string true "Collection ID or Name"
// @Success 200 {object} models.SchemaResponse
// @Security ApiKeyAuth
// @Router /collections/{id}/query/schema [get]
func GetCollectionSchema(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	collectionID := c.Params("id")

	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	var sampleDocs []models.Document
	if err := database.DB.Where("collection_id = ?", collection.ID).
		Limit(100).Find(&sampleDocs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch sample documents"})
	}

	var totalDocs int64
	database.DB.Model(&models.Document{}).Where("collection_id = ?", collection.ID).Count(&totalDocs)

	if len(sampleDocs) == 0 {
		return c.JSON(models.SchemaResponse{
			Collection:            collection.Name,
			DocumentCount:         0,
			AnalyzedDocuments:     0,
			Fields:                make(map[string]models.FieldInfo),
			DetectedRelationships: make(map[string]models.RelationshipInfo),
			UsageHint:             "Use ?populate=<relationship_name> to auto-fetch related data",
		})
	}

	fieldAnalysis := make(map[string]*models.FieldInfo)
	detectedRelationships := make(map[string]models.RelationshipInfo)

	for _, doc := range sampleDocs {
		for key, value := range doc.Data {
			if _, exists := fieldAnalysis[key]; !exists {
				fieldAnalysis[key] = &models.FieldInfo{Types: []string{}, Samples: []interface{}{}}
			}
			info := fieldAnalysis[key]
			var valueType string
			if value == nil {
				valueType = "null"
			} else {
				valueType = fmt.Sprintf("%T", value)
			}
			typeExists := false
			for _, t := range info.Types {
				if t == valueType {
					typeExists = true
					break
				}
			}
			if !typeExists {
				info.Types = append(info.Types, valueType)
			}
			if len(info.Samples) < 3 && value != nil {
				info.Samples = append(info.Samples, value)
			}
			if strings.HasSuffix(key, "_id") {
				if strValue, ok := value.(string); ok && len(strValue) > 20 {
					baseName := strings.TrimSuffix(key, "_id")
					detectedRelationships[key] = models.RelationshipInfo{
						Type: "belongs_to", Field: key,
						TargetCollection: baseName + "s", Confidence: "high",
					}
				}
			}
		}
	}

	fields := make(map[string]models.FieldInfo)
	for key, info := range fieldAnalysis {
		primaryType := "unknown"
		if len(info.Types) > 0 {
			primaryType = info.Types[0]
		}
		fields[key] = models.FieldInfo{
			Types: info.Types, PrimaryType: primaryType,
			Nullable: contains(info.Types, "null"), Samples: info.Samples,
		}
	}

	return c.JSON(models.SchemaResponse{
		Collection: collection.Name, DocumentCount: totalDocs,
		AnalyzedDocuments:     len(sampleDocs),
		Fields:                fields,
		DetectedRelationships: detectedRelationships,
		UsageHint:             "Use ?populate=<relationship_name> to auto-fetch related data",
	})
}

// ─────────────────────────────────────────
// ExportCollection
// ─────────────────────────────────────────

func ExportCollection(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	collectionID := c.Params("id")
	format := c.Query("format", "json")

	if format != "json" && format != "csv" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid format. Must be 'json' or 'csv'"})
	}

	collection, err := getCollectionByIDOrName(collectionID, project.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Collection not found"})
	}

	appUser := middleware.GetAppUserFromContext(c)
	if err := permChecker.CanAccessCollection(collection, services.PermissionRead, appUser); err != nil {
		return err
	}

	var documents []models.Document
	if err := database.DB.Where("collection_id = ?", collection.ID).
		Limit(10000).Find(&documents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch documents"})
	}

	documentsData := make([]map[string]interface{}, len(documents))
	for i, doc := range documents {
		documentsData[i] = documentToMap(&doc)
	}

	if format == "json" {
		c.Set("Content-Type", "application/json")
		c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_export.json"`, collection.Name))
		jsonData, err := json.MarshalIndent(documentsData, "", "  ")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to generate JSON"})
		}
		return c.Send(jsonData)
	}

	if len(documentsData) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "No documents to export"})
	}

	fieldSet := make(map[string]bool)
	for _, doc := range documentsData {
		for key := range doc {
			fieldSet[key] = true
		}
	}

	fields := []string{}
	if fieldSet["id"] {
		fields = append(fields, "id")
		delete(fieldSet, "id")
	}
	if fieldSet["created_at"] {
		fields = append(fields, "created_at")
		delete(fieldSet, "created_at")
	}
	for field := range fieldSet {
		fields = append(fields, field)
	}

	var csvBuilder strings.Builder
	writer := csv.NewWriter(&csvBuilder)
	writer.Write(fields)
	for _, doc := range documentsData {
		row := make([]string, len(fields))
		for i, field := range fields {
			if value, ok := doc[field]; ok {
				row[i] = fmt.Sprintf("%v", value)
			}
		}
		writer.Write(row)
	}
	writer.Flush()

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s_export.csv"`, collection.Name))
	return c.SendString(csvBuilder.String())
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
