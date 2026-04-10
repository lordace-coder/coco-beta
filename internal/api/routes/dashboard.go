package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
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

	// Available integrations catalogue
	protected.Get("/integrations", dashboard.ListIntegrations)

	// Projects
	projects := protected.Group("/projects")
	projects.Get("/", dashboard.ListProjects)
	projects.Post("/", dashboard.CreateProject)
	projects.Get("/:id", dashboard.GetProject)
	projects.Patch("/:id", dashboard.UpdateProject)
	projects.Delete("/:id", dashboard.DeleteProject)
	projects.Post("/:id/regen-key", dashboard.RegenAPIKey)

	// Project → users
	projects.Get("/:id/users", dashboard.ListUsers)
	projects.Get("/:id/users/:userId", dashboard.GetUser)
	projects.Patch("/:id/users/:userId", dashboard.UpdateUser)
	projects.Delete("/:id/users/:userId", dashboard.DeleteUser)
	projects.Delete("/:id/users", dashboard.DeleteAllUsers)

	// Project → collections
	projects.Get("/:id/collections", dashboard.ListCollections)
	projects.Get("/:id/collections/:colId", dashboard.GetCollection)
	projects.Delete("/:id/collections/:colId", dashboard.DeleteCollection)

	// Project → collection documents
	projects.Get("/:id/collections/:colId/documents", dashboard.ListDocuments)
	projects.Get("/:id/collections/:colId/documents/:docId", dashboard.GetDocument)
	projects.Patch("/:id/collections/:colId/documents/:docId", dashboard.UpdateDocument)
	projects.Delete("/:id/collections/:colId/documents/:docId", dashboard.DeleteDocument)

	// Project → files
	projects.Get("/:id/files", dashboard.ListFiles)
	projects.Delete("/:id/files", dashboard.DeleteFile)

	// Project → integrations
	projects.Get("/:id/integrations", dashboard.ListProjectIntegrations)
	projects.Get("/:id/integrations/:piId", dashboard.GetProjectIntegration)
	projects.Put("/:id/integrations/:integrationName", dashboard.UpsertProjectIntegration)
	projects.Delete("/:id/integrations/:piId", dashboard.DeleteProjectIntegration)

	// ── Serve embedded React SPA at /_/* ──────────────────────────────────────
	spaFS := filesystem.New(filesystem.Config{
		Root:         dashboardfs.HTTPRoot(),
		Browse:       false,
		Index:        "index.html",
		NotFoundFile: "index.html",
	})
	app.Get("/_", spaFS)
	app.Get("/_/", spaFS)
	app.Get("/_/*", spaFS)
}
