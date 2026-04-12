package dashboard

import (
	"time"

	"github.com/gofiber/fiber/v2"
	fn "github.com/patrick/cocobase/internal/services/functions"
)

// ListFunctionFiles GET /_/api/projects/:id/functions
// Returns all .js function files for a project.
func ListFunctionFiles(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	names := fn.ListFunctionFiles(projectID)
	type fileInfo struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	files := make([]fileInfo, len(names))
	for i, name := range names {
		files[i] = fileInfo{
			Name: name,
			Path: fn.FunctionFilePath(projectID, name),
		}
	}
	return c.JSON(fiber.Map{"data": files, "total": len(files)})
}

// GetFunctionFile GET /_/api/projects/:id/functions/:name
// Returns the code of a single function file.
func GetFunctionFile(c *fiber.Ctx) error {
	projectID := c.Params("id")
	name := c.Params("name")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	code, mtime := fn.ReadFunctionFile(projectID, name)
	if code == "" && mtime.IsZero() {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Function file not found"})
	}
	return c.JSON(fiber.Map{
		"name":     name,
		"code":     code,
		"path":     fn.FunctionFilePath(projectID, name),
		"modified": mtime,
	})
}

// SaveFunctionFile PUT /_/api/projects/:id/functions/:name
// Writes code to a function file and invalidates the registry.
func SaveFunctionFile(c *fiber.Ctx) error {
	projectID := c.Params("id")
	name := c.Params("name")
	project, err := getProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}

	if err := fn.WriteFunctionCode(projectID, name, req.Code); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to write file"})
	}

	fn.InvalidateRegistry(projectID)
	fn.ReloadProjectCrons(projectID, project.Name)

	return c.JSON(fiber.Map{
		"name":     name,
		"path":     fn.FunctionFilePath(projectID, name),
		"modified": time.Now(),
	})
}

// CreateFunctionFile POST /_/api/projects/:id/functions
// Creates a new named function file with a starter stub.
func CreateFunctionFile(c *fiber.Ctx) error {
	projectID := c.Params("id")
	project, err := getProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Name string `json:"name"`
		Code string `json:"code"` // optional — uses starter stub if empty
	}
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "name is required"})
	}

	// Don't overwrite an existing file
	existing, _ := fn.ReadFunctionFile(projectID, req.Name)
	if existing != "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": true, "message": "A function with that name already exists"})
	}

	code := req.Code
	if code == "" {
		code = starterStub(req.Name, project.ID)
	}

	if err := fn.WriteFunctionCode(projectID, req.Name, code); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create file"})
	}

	fn.InvalidateRegistry(projectID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"name": req.Name,
		"path": fn.FunctionFilePath(projectID, req.Name),
		"code": code,
	})
}

// DeleteFunctionFile DELETE /_/api/projects/:id/functions/:name
func DeleteFunctionFileHandler(c *fiber.Ctx) error {
	projectID := c.Params("id")
	name := c.Params("name")
	project, err := getProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	fn.DeleteFunctionFile(projectID, name)
	fn.InvalidateRegistry(projectID)
	fn.ReloadProjectCrons(projectID, project.Name)

	return c.JSON(fiber.Map{"message": "deleted"})
}

// RunFunctionFile POST /_/api/projects/:id/functions/:name/run
// Test-runs an HTTP route from a specific file against a given method+path.
func RunFunctionFile(c *fiber.Ctx) error {
	projectID := c.Params("id")
	project, err := getProjectByID(projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	var req struct {
		Method string            `json:"method"`
		Path   string            `json:"path"`
		Body   string            `json:"body"`
		Query  map[string]string `json:"query"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Path == "" {
		req.Path = "/"
	}

	rctx := &fn.RunContext{
		ProjectID:   projectID,
		ProjectName: project.Name,
		ReqMethod:   req.Method,
		ReqPath:     req.Path,
		ReqBody:     req.Body,
		ReqQuery:    req.Query,
		ReqHeaders:  map[string]string{},
	}

	start := time.Now()
	responded, runErr := fn.DispatchHTTP(projectID, project.Name, rctx)
	duration := time.Since(start).Milliseconds()

	result := fiber.Map{
		"duration_ms": duration,
		"output":      rctx.LogOutput.String(),
		"responded":   responded,
		"success":     runErr == nil,
	}
	if responded {
		result["status"] = rctx.ResponseStatus
		result["body"] = rctx.ResponseBody
	}
	if runErr != nil {
		result["error"] = runErr.Error()
	}
	return c.JSON(result)
}

// GetCronSchedule GET /_/api/projects/:id/functions/crons
func GetCronSchedule(c *fiber.Ctx) error {
	projectID := c.Params("id")
	if _, err := getProjectByID(projectID); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}
	entries := fn.GetCronEntries(projectID)
	return c.JSON(fiber.Map{"data": entries, "total": len(entries)})
}

// starterStub returns a sensible default for a new function file.
func starterStub(name, projectID string) string {
	return "// " + name + ".js\n" +
		"// URL: /functions/" + projectID + "/func/" + name + "\n\n" +
		"app.get(\"/" + name + "\", (ctx) => {\n" +
		"  ctx.respond(200, JSON.stringify({ message: \"" + name + " ok\" }), {\n" +
		"    \"Content-Type\": \"application/json\",\n" +
		"  });\n" +
		"});\n"
}
