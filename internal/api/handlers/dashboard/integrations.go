package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/instance"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
)

// ListProjectIntegrations handles GET /_/api/projects/:id/integrations
func ListProjectIntegrations(c *fiber.Ctx) error {
	projectID := instance.ID()
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var pis []models.ProjectIntegration
	if err := database.DB.Where("project_id = ?", projectID).
		Preload("Integration").Find(&pis).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch integrations"})
	}

	result := make([]fiber.Map, len(pis))
	for i, pi := range pis {
		entry := fiber.Map{
			"id":             pi.ID,
			"project_id":     pi.ProjectID,
			"integration_id": pi.IntegrationID,
			"is_enabled":     pi.IsEnabled,
			"created_at":     pi.CreatedAt,
		}
		// Mask secret config values
		safeConfig := make(map[string]interface{})
		for k, v := range pi.Config {
			if isSecretKey(k) {
				if s, ok := v.(string); ok && s != "" {
					safeConfig[k] = "••••••••"
				} else {
					safeConfig[k] = v
				}
			} else {
				safeConfig[k] = v
			}
		}
		entry["config"] = safeConfig
		if pi.Integration != nil {
			entry["integration"] = fiber.Map{
				"id":           pi.Integration.ID,
				"name":         pi.Integration.Name,
				"display_name": pi.Integration.DisplayName,
				"description":  pi.Integration.Description,
				"icon_url":     pi.Integration.IconURL,
				"is_active":    pi.Integration.IsActive,
			}
		}
		result[i] = entry
	}
	return c.JSON(fiber.Map{"data": result, "total": len(result)})
}

// GetProjectIntegration handles GET /_/api/projects/:id/integrations/:piId
func GetProjectIntegration(c *fiber.Ctx) error {
	pi, err := getProjectIntegration(instance.ID(), c.Params("piId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Integration not found"})
	}
	return c.JSON(pi)
}

// UpsertProjectIntegration handles PUT /_/api/projects/:id/integrations/:integrationName
// Creates or updates a project integration by integration name.
func UpsertProjectIntegration(c *fiber.Ctx) error {
	projectID := instance.ID()
	integrationName := c.Params("integrationName")

	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var integration models.Integration
	if err := database.DB.Where("name = ?", integrationName).First(&integration).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Integration not found"})
	}

	var req struct {
		Config    map[string]interface{} `json:"config"`
		IsEnabled *bool                  `json:"is_enabled"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	var pi models.ProjectIntegration
	err := database.DB.Where("project_id = ? AND integration_id = ?", projectID, integration.ID).First(&pi).Error
	if err != nil {
		// Create
		pi = models.ProjectIntegration{
			ProjectID:     projectID,
			IntegrationID: integration.ID,
			Config:        req.Config,
			IsEnabled:     true,
		}
		if req.IsEnabled != nil {
			pi.IsEnabled = *req.IsEnabled
		}
		if err := database.DB.Create(&pi).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create integration"})
		}
	} else {
		updates := map[string]interface{}{}
		if req.Config != nil {
			// Merge config
			for k, v := range req.Config {
				pi.Config[k] = v
			}
			updates["config"] = pi.Config
		}
		if req.IsEnabled != nil {
			updates["is_enabled"] = *req.IsEnabled
		}
		if len(updates) > 0 {
			if err := database.DB.Model(&pi).Updates(updates).Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update integration"})
			}
		}
	}

	return c.JSON(fiber.Map{"message": "Integration saved", "id": pi.ID})
}

// DeleteProjectIntegration handles DELETE /_/api/projects/:id/integrations/:piId
func DeleteProjectIntegration(c *fiber.Ctx) error {
	pi, err := getProjectIntegration(instance.ID(), c.Params("piId"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Integration not found"})
	}
	if err := database.DB.Delete(pi).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to delete integration"})
	}
	return c.JSON(fiber.Map{"message": "Integration removed"})
}

// ListIntegrations handles GET /_/api/integrations — all available integrations
func ListIntegrations(c *fiber.Ctx) error {
	var integrations []models.Integration
	if err := database.DB.Where("is_active = ?", true).Order("name asc").Find(&integrations).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch integrations"})
	}
	return c.JSON(fiber.Map{"data": integrations, "total": len(integrations)})
}

func getProjectIntegration(projectID, piID string) (*models.ProjectIntegration, error) {
	var pi models.ProjectIntegration
	err := database.DB.Where("id = ? AND project_id = ?", piID, projectID).
		Preload("Integration").First(&pi).Error
	if err != nil {
		return nil, err
	}
	return &pi, nil
}

func isSecretKey(key string) bool {
	secrets := map[string]bool{
		"api_key": true, "secret": true, "password": true,
		"token": true, "client_secret": true,
	}
	return secrets[key]
}
