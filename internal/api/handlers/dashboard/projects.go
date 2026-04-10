package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"gorm.io/gorm"
)

// ListProjects handles GET /_/api/projects
func ListProjects(c *fiber.Ctx) error {
	var projects []models.Project
	if err := database.DB.Order("created_at desc").Find(&projects).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to fetch projects"})
	}
	return c.JSON(fiber.Map{"data": projects, "total": len(projects)})
}

// CreateProject handles POST /_/api/projects
func CreateProject(c *fiber.Ctx) error {
	var req struct {
		Name   string `json:"name"`
		UserID string `json:"user_id"` // optional — admin can assign to a user
	}
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "name is required"})
	}

	// Use a synthetic user ID if none provided (self-hosted, single admin)
	userID := req.UserID
	if userID == "" {
		userID = "admin"
	}

	project := models.Project{
		Name:   req.Name,
		UserID: userID,
	}
	if err := database.DB.Create(&project).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create project"})
	}
	return c.Status(fiber.StatusCreated).JSON(project)
}

// GetProject handles GET /_/api/projects/:id
func GetProject(c *fiber.Ctx) error {
	project, err := getProjectByID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}
	return c.JSON(project)
}

// UpdateProject handles PATCH /_/api/projects/:id
func UpdateProject(c *fiber.Ctx) error {
	project, err := getProjectByID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Name           *string            `json:"name"`
		AllowedOrigins models.StringArray `json:"allowed_origins"`
		Active         *bool              `json:"active"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.AllowedOrigins != nil {
		updates["allowed_origins"] = req.AllowedOrigins
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}

	if len(updates) > 0 {
		if err := database.DB.Model(project).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update project"})
		}
	}
	return c.JSON(project)
}

// DeleteProject handles DELETE /_/api/projects/:id
func DeleteProject(c *fiber.Ctx) error {
	project, err := getProjectByID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}
	if err := database.DB.Delete(project).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to delete project"})
	}
	return c.JSON(fiber.Map{"message": "Project deleted"})
}

// RegenAPIKey handles POST /_/api/projects/:id/regen-key
func RegenAPIKey(c *fiber.Ctx) error {
	project, err := getProjectByID(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	newKey := "coco_" + uuid.New().String()
	if err := database.DB.Model(project).Update("api_key", newKey).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to regenerate key"})
	}
	return c.JSON(fiber.Map{"api_key": newKey})
}

func getProjectByID(id string) (*models.Project, error) {
	var project models.Project
	if err := database.DB.Where("id = ?", id).First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, err
	}
	return &project, nil
}
