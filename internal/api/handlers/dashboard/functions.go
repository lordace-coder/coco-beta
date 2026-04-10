package dashboard

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	fn "github.com/patrick/cocobase/internal/services/functions"
)

// ListFunctions GET /_/api/projects/:id/functions
func ListFunctions(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var fns []models.Function
	database.DB.Where("project_id = ?", projectID).Order("created_at desc").Find(&fns)

	// Include cron timing info
	cronEntries := fn.GetCronEntries(projectID)
	cronMap := map[string]fn.CronEntry{}
	for _, e := range cronEntries {
		cronMap[e.FunctionID] = e
	}

	type fnResponse struct {
		models.Function
		NextRun *time.Time `json:"next_run,omitempty"`
		PrevRun *time.Time `json:"prev_run,omitempty"`
	}

	result := make([]fnResponse, len(fns))
	for i, f := range fns {
		r := fnResponse{Function: f}
		if e, ok := cronMap[f.ID]; ok {
			if !e.Next.IsZero() {
				r.NextRun = &e.Next
			}
			if !e.Prev.IsZero() {
				r.PrevRun = &e.Prev
			}
		}
		result[i] = r
	}

	return c.JSON(fiber.Map{"data": result, "total": len(result)})
}

// GetFunction GET /_/api/projects/:id/functions/:fnId
func GetFunction(c *fiber.Ctx) error {
	projectID := c.Params("id")
	fnID := c.Params("fnId")

	var f models.Function
	if err := database.DB.Where("id = ? AND project_id = ?", fnID, projectID).First(&f).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Function not found"})
	}
	return c.JSON(f)
}

// CreateFunction POST /_/api/projects/:id/functions
func CreateFunction(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Name          string              `json:"name"`
		Code          string              `json:"code"`
		TriggerType   models.TriggerType  `json:"trigger_type"`
		TriggerConfig models.TriggerConfig `json:"trigger_config"`
		Enabled       *bool               `json:"enabled"`
		Timeout       int                 `json:"timeout"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "name is required"})
	}
	if req.TriggerType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "trigger_type is required"})
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10
	}

	f := models.Function{
		ID:            uuid.New().String(),
		ProjectID:     projectID,
		Name:          req.Name,
		Code:          req.Code,
		TriggerType:   req.TriggerType,
		TriggerConfig: req.TriggerConfig,
		Enabled:       enabled,
		Timeout:       timeout,
	}

	if err := database.DB.Create(&f).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create function"})
	}

	fn.InvalidatePool(projectID)
	if f.TriggerType == models.TriggerCron {
		fn.ReloadCronFunction(&f)
	}

	return c.Status(fiber.StatusCreated).JSON(f)
}

// UpdateFunction PATCH /_/api/projects/:id/functions/:fnId
func UpdateFunction(c *fiber.Ctx) error {
	projectID := c.Params("id")
	fnID := c.Params("fnId")

	var f models.Function
	if err := database.DB.Where("id = ? AND project_id = ?", fnID, projectID).First(&f).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Function not found"})
	}

	var req struct {
		Name          *string              `json:"name"`
		Code          *string              `json:"code"`
		TriggerConfig *models.TriggerConfig `json:"trigger_config"`
		Enabled       *bool                `json:"enabled"`
		Timeout       *int                 `json:"timeout"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	if req.Name != nil {
		f.Name = *req.Name
	}
	if req.Code != nil {
		f.Code = *req.Code
	}
	if req.TriggerConfig != nil {
		f.TriggerConfig = *req.TriggerConfig
	}
	if req.Enabled != nil {
		f.Enabled = *req.Enabled
	}
	if req.Timeout != nil && *req.Timeout > 0 {
		f.Timeout = *req.Timeout
	}

	if err := database.DB.Save(&f).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to update function"})
	}

	fn.InvalidatePool(projectID)
	fn.ReloadCronFunction(&f)

	return c.JSON(f)
}

// DeleteFunction DELETE /_/api/projects/:id/functions/:fnId
func DeleteFunction(c *fiber.Ctx) error {
	projectID := c.Params("id")
	fnID := c.Params("fnId")

	var f models.Function
	if err := database.DB.Where("id = ? AND project_id = ?", fnID, projectID).First(&f).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Function not found"})
	}

	fn.UnscheduleCronFunction(fnID)
	fn.InvalidatePool(projectID)
	database.DB.Delete(&f)

	return c.JSON(fiber.Map{"message": "Function deleted"})
}

// RunFunction POST /_/api/projects/:id/functions/:fnId/run — manual trigger from dashboard
func RunFunction(c *fiber.Ctx) error {
	projectID := c.Params("id")
	fnID := c.Params("fnId")

	var f models.Function
	if err := database.DB.Where("id = ? AND project_id = ?", fnID, projectID).First(&f).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Function not found"})
	}

	rctx := &fn.RunContext{
		ProjectID: projectID,
		ReqMethod: "MANUAL",
	}

	start := time.Now()
	runErr := fn.Execute(&f, rctx)
	duration := time.Since(start).Milliseconds()

	result := fiber.Map{
		"duration_ms": duration,
		"output":      rctx.LogOutput.String(),
		"success":     runErr == nil,
	}
	if runErr != nil {
		result["error"] = runErr.Error()
	}
	return c.JSON(result)
}

// ListHTTPRoutes GET /_/api/projects/:id/functions/routes — read-only route list
func ListHTTPRoutes(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var fns []models.Function
	database.DB.Where("project_id = ? AND trigger_type = ?", projectID, models.TriggerHTTP).
		Order("name").Find(&fns)

	type route struct {
		FunctionID string `json:"function_id"`
		Name       string `json:"name"`
		Method     string `json:"method"`
		Path       string `json:"path"`
		Enabled    bool   `json:"enabled"`
	}

	routes := make([]route, 0, len(fns))
	for _, f := range fns {
		method := f.TriggerConfig.Method
		if method == "" {
			method = "ANY"
		}
		routes = append(routes, route{
			FunctionID: f.ID,
			Name:       f.Name,
			Method:     method,
			Path:       "/fn" + f.TriggerConfig.Path,
			Enabled:    f.Enabled,
		})
	}

	return c.JSON(fiber.Map{"data": routes, "total": len(routes)})
}
