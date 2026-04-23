package routes

import (
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers/dashboard"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/dashboardfs"
)

// SetupDashboardRoutes registers all /_/api/* routes and serves the embedded SPA.
func SetupDashboardRoutes(app *fiber.App) {
	api := app.Group("/_/api")

	// ── Public (no auth) ──────────────────────────────────────────────────────
	auth := api.Group("/auth")
	auth.Get("/setup-status", dashboard.SetupStatus)
	auth.Post("/setup", dashboard.CreateAdmin)
	auth.Post("/login", dashboard.AdminLogin)

	// Health (public so monitoring tools can hit it without auth)
	api.Get("/health", dashboard.DashboardHealth)

	// ── Protected (admin JWT required) ───────────────────────────────────────
	protected := api.Group("", middleware.RequireAdmin)

	// Current admin
	protected.Get("/auth/me", dashboard.AdminMe)

	// Global config (SMTP, etc.)
	protected.Get("/config", dashboard.GetConfig)
	protected.Patch("/config", dashboard.UpdateConfig)
	protected.Post("/config/smtp/test", dashboard.TestSMTP)

	// Instance config
	protected.Get("/instance", dashboard.GetInstance)
	protected.Patch("/instance", dashboard.UpdateInstance)
	protected.Post("/instance/regen-key", dashboard.RegenAPIKey)

	// Users
	protected.Get("/users", dashboard.ListUsers)
	protected.Post("/users", dashboard.CreateUser)
	protected.Get("/users/:userId", dashboard.GetUser)
	protected.Patch("/users/:userId", dashboard.UpdateUser)
	protected.Delete("/users/:userId", dashboard.DeleteUser)
	protected.Delete("/users", dashboard.DeleteAllUsers)

	// Collections
	protected.Get("/collections", dashboard.ListCollections)
	protected.Post("/collections", dashboard.CreateCollection)
	protected.Get("/collections/:colId", dashboard.GetCollection)
	protected.Patch("/collections/:colId", dashboard.UpdateCollection)
	protected.Delete("/collections/:colId", dashboard.DeleteCollection)

	// Collection documents
	protected.Get("/collections/:colId/documents", dashboard.ListDocuments)
	protected.Post("/collections/:colId/documents", dashboard.CreateDocumentDashboard)
	protected.Get("/collections/:colId/documents/:docId", dashboard.GetDocument)
	protected.Patch("/collections/:colId/documents/:docId", dashboard.UpdateDocument)
	protected.Delete("/collections/:colId/documents/:docId", dashboard.DeleteDocument)

	// Activity logs
	protected.Get("/logs", dashboard.ListLogs)

	// Files
	protected.Get("/files", dashboard.ListFiles)
	protected.Delete("/files", dashboard.DeleteFile)

	// Integrations
	protected.Get("/integrations/catalogue", dashboard.ListIntegrations)
	protected.Get("/integrations", dashboard.ListProjectIntegrations)
	protected.Get("/integrations/:piId", dashboard.GetProjectIntegration)
	protected.Put("/integrations/:integrationName", dashboard.UpsertProjectIntegration)
	protected.Delete("/integrations/:piId", dashboard.DeleteProjectIntegration)

	// Cloud functions
	protected.Get("/functions", dashboard.ListFunctionFiles)
	protected.Post("/functions", dashboard.CreateFunctionFile)
	protected.Get("/functions/crons", dashboard.GetCronSchedule)
	protected.Get("/functions/:name", dashboard.GetFunctionFile)
	protected.Put("/functions/:name", dashboard.SaveFunctionFile)
	protected.Delete("/functions/:name", dashboard.DeleteFunctionFileHandler)
	protected.Post("/functions/:name/run", dashboard.RunFunctionFile)

	// ── Serve embedded React SPA at /_/* ──────────────────────────────────────
	app.Get("/_", serveSPA)
	app.Get("/_/*", serveSPA)
}

var mimeTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".js":   "application/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".svg":  "image/svg+xml",
	".png":  "image/png",
	".ico":  "image/x-icon",
	".json": "application/json",
}

func serveSPA(c *fiber.Ctx) error {
	reqPath := c.Path()
	filePath := strings.TrimPrefix(reqPath, "/_")
	if filePath == "" || filePath == "/" {
		filePath = "/index.html"
	}

	root := dashboardfs.HTTPRoot()

	f, err := root.Open(filePath)
	if err != nil {
		// SPA fallback — serve index.html for client-side routes
		f, err = root.Open("/index.html")
		if err != nil {
			return fiber.ErrNotFound
		}
		filePath = "/index.html"
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	// Determine MIME type
	dot := strings.LastIndex(filePath, ".")
	mimeType := "application/octet-stream"
	if dot >= 0 {
		if mt, ok := mimeTypes[filePath[dot:]]; ok {
			mimeType = mt
		}
	}

	c.Set("Content-Type", mimeType)
	return c.Send(data)
}
