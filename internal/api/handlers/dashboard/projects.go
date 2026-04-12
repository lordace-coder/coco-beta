package dashboard

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	fnservice "github.com/patrick/cocobase/internal/services/functions"
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

	// Scaffold the functions directory, type declarations, and sample function.
	// Must run synchronously — the project row must be committed before RegisterSampleInDB inserts.
	fnservice.EnsureProjectFunctionsDir(project.ID, project.Name)

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
		Name           *string                `json:"name"`
		AllowedOrigins models.StringArray     `json:"allowed_origins"`
		Active         *bool                  `json:"active"`
		Configs        map[string]interface{} `json:"configs"`
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
	if req.Configs != nil {
		updates["configs"] = models.JSONMap(req.Configs)
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

// GetInstance handles GET /_/api/instance — returns the single default project.
// Cocobase is a single-instance BaaS; there is always exactly one project.
func GetInstance(c *fiber.Ctx) error {
	project, err := getOrCreateDefaultProject()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to load instance"})
	}
	return c.JSON(project)
}

// getOrCreateDefaultProject returns the first project, creating it if none exists.
func getOrCreateDefaultProject() (*models.Project, error) {
	var project models.Project
	if err := database.DB.Order("created_at asc").First(&project).Error; err == nil {
		return &project, nil
	}
	// None exists — create the default instance project
	project = models.Project{
		Name:   "default",
		UserID: "admin",
		Active: true,
	}
	if err := database.DB.Create(&project).Error; err != nil {
		return nil, err
	}
	fnservice.EnsureProjectFunctionsDir(project.ID, project.Name)
	return &project, nil
}

// EnsureDefaultProject is called at startup to guarantee the project row exists.
func EnsureDefaultProject() {
	if _, err := getOrCreateDefaultProject(); err != nil {
		return
	}
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
