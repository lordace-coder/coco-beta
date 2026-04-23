package dashboard

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/instance"
	"github.com/patrick/cocobase/internal/models"
	fnservice "github.com/patrick/cocobase/internal/services/functions"
)

// EnsureDefaultProject is called at startup. It guarantees exactly one Project
// row exists, stores its ID in the instance package, and prints the API key.
func EnsureDefaultProject() {
	project, err := getOrCreateDefaultProject()
	if err != nil {
		log.Fatalf("❌ Failed to initialise instance: %v", err)
	}
	instance.Set(project.ID)
	log.Printf("🔑 API Key: %s", project.APIKey)
}

// GetInstance handles GET /_/api/instance — returns instance info.
func GetInstance(c *fiber.Ctx) error {
	var project models.Project
	if err := database.DB.First(&project, "id = ?", instance.ID()).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Instance not found"})
	}
	return c.JSON(fiber.Map{
		"id":              project.ID,
		"api_key":         project.APIKey,
		"allowed_origins": project.AllowedOrigins,
		"configs":         project.Configs,
	})
}

// UpdateInstance handles PATCH /_/api/instance.
func UpdateInstance(c *fiber.Ctx) error {
	var req struct {
		AllowedOrigins models.StringArray     `json:"allowed_origins"`
		Configs        map[string]interface{} `json:"configs"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	updates := map[string]interface{}{}
	if req.AllowedOrigins != nil {
		updates["allowed_origins"] = req.AllowedOrigins
	}
	if req.Configs != nil {
		updates["configs"] = models.JSONMap(req.Configs)
	}
	if len(updates) > 0 {
		database.DB.Model(&models.Project{}).Where("id = ?", instance.ID()).Updates(updates)
	}
	return GetInstance(c)
}

// RegenAPIKey handles POST /_/api/instance/regen-key.
func RegenAPIKey(c *fiber.Ctx) error {
	newKey := "coco_" + uuid.New().String()
	if err := database.DB.Model(&models.Project{}).Where("id = ?", instance.ID()).Update("api_key", newKey).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to regenerate key"})
	}
	log.Printf("🔑 API Key regenerated: %s", newKey)
	return c.JSON(fiber.Map{"api_key": newKey})
}

// getOrCreateDefaultProject returns the first project row, creating it if none exists.
func getOrCreateDefaultProject() (*models.Project, error) {
	var project models.Project
	if err := database.DB.Order("created_at asc").First(&project).Error; err == nil {
		return &project, nil
	}
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

// getProjectByID is kept for internal use by other dashboard handlers.
func getProjectByID(id string) (*models.Project, error) {
	var project models.Project
	if err := database.DB.Where("id = ?", id).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}
