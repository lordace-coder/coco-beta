package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
)

// Log writes an activity log entry. Call after any mutating dashboard action.
func Log(projectID, action, resource, resourceID, detail string) {
	entry := models.ActivityLog{
		ProjectID:  projectID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
	}
	database.DB.Create(&entry)
}

// ListLogs handles GET /_/api/projects/:id/logs
func ListLogs(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	limit := c.QueryInt("limit", 50)
	offset := c.QueryInt("offset", 0)
	if limit > 200 {
		limit = 200
	}

	var total int64
	database.DB.Model(&models.ActivityLog{}).Where("project_id = ?", projectID).Count(&total)

	var logs []models.ActivityLog
	if err := database.DB.Where("project_id = ?", projectID).
		Order("created_at desc").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch logs"})
	}

	return c.JSON(fiber.Map{
		"data":     logs,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"has_more": int64(offset+limit) < total,
	})
}
