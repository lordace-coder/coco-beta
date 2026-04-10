package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/patrick/cocobase/internal/api/handlers"
	dashhandlers "github.com/patrick/cocobase/internal/api/handlers/dashboard"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/api/routes"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"github.com/patrick/cocobase/pkg/config"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	_ "github.com/patrick/cocobase/docs"
	fiberSwagger "github.com/swaggo/fiber-swagger"
)

// @title Cocobase API
// @version 1.0
// @description Backend as a Service with flexible collections and document management
// @termsOfService http://swagger.io/terms/

// @contact.name Cocobase Support
// @contact.email support@cocobase.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:3000
// @BasePath /
// @schemes http https

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and API key.

const usage = `Cocobase v0.1.0 — self-hosted Backend as a Service

Usage:
  cocobase [command]

Commands:
  serve              Start the server (default when no command is given)
  reset-password     Reset the admin dashboard password
  wipe-project       Delete all data (users, collections, documents) for a project
  wipe-all           Delete ALL projects and their data (keeps admin account)
  list-projects      List all projects and their API keys

Flags (for serve):
  -port string       Override the PORT env variable
  -env  string       Path to a custom .env file (default: .env)

Examples:
  cocobase                        # start the server
  cocobase serve -port 8080
  cocobase reset-password
  cocobase list-projects
  cocobase wipe-project
`

func main() {
	if len(os.Args) < 2 || os.Args[1] == "serve" {
		runServe()
		return
	}

	switch os.Args[1] {
	case "reset-password":
		runWithDB(cmdResetPassword)
	case "wipe-project":
		runWithDB(cmdWipeProject)
	case "wipe-all":
		runWithDB(cmdWipeAll)
	case "list-projects":
		runWithDB(cmdListProjects)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

// runWithDB loads config, connects to DB, runs fn, then exits.
func runWithDB(fn func()) {
	cfg := config.LoadConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set. Make sure your .env is present.")
	}
	if err := database.Connect(cfg.DatabaseURL, false); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	fn()
}

// ── Commands ──────────────────────────────────────────────────────────────────

func cmdResetPassword() {
	fmt.Println("=== Reset admin password ===")
	email := prompt("Admin email: ")

	var admin models.AdminUser
	if err := database.DB.Where("email = ?", strings.ToLower(email)).First(&admin).Error; err != nil {
		log.Fatalf("No admin found with email %q", email)
	}

	newPass := promptPassword("New password (min 8 chars): ")
	if len(newPass) < 8 {
		log.Fatal("Password must be at least 8 characters")
	}
	confirm := promptPassword("Confirm password: ")
	if newPass != confirm {
		log.Fatal("Passwords do not match")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	if err := database.DB.Model(&admin).Update("password", string(hash)).Error; err != nil {
		log.Fatalf("Failed to update password: %v", err)
	}
	fmt.Printf("✅ Password updated for %s\n", admin.Email)
}

func cmdWipeProject() {
	fmt.Println("=== Wipe project data ===")
	fmt.Println("This will delete ALL users, collections, and documents for a project.")

	var projects []models.Project
	database.DB.Order("name").Find(&projects)
	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return
	}

	fmt.Println("\nAvailable projects:")
	for i, p := range projects {
		fmt.Printf("  [%d] %s (id: %s)\n", i+1, p.Name, p.ID[:8])
	}

	input := prompt("\nEnter project number or ID: ")
	var target *models.Project
	for i, p := range projects {
		n := fmt.Sprintf("%d", i+1)
		if input == n || strings.HasPrefix(p.ID, input) || p.ID == input {
			p := p
			target = &p
			break
		}
	}
	if target == nil {
		log.Fatalf("No project matched %q", input)
	}

	fmt.Printf("\nProject: %s\n", target.Name)
	confirm := prompt("Type the project name to confirm deletion: ")
	if confirm != target.Name {
		log.Fatal("Name did not match — aborting")
	}

	// Delete documents, then collections, then users
	var cols []models.Collection
	database.DB.Where("project_id = ?", target.ID).Find(&cols)
	for _, c := range cols {
		database.DB.Where("collection_id = ?", c.ID).Delete(&models.Document{})
	}
	database.DB.Where("project_id = ?", target.ID).Delete(&models.Collection{})
	database.DB.Where("project_id = ?", target.ID).Delete(&models.AppUser{})
	database.DB.Where("project_id = ?", target.ID).Delete(&models.ActivityLog{})

	fmt.Printf("✅ All data wiped for project %q. The project itself was kept.\n", target.Name)
}

func cmdWipeAll() {
	fmt.Println("=== Wipe ALL project data ===")
	fmt.Println("⚠️  This will delete EVERY project, user, collection, and document.")
	fmt.Println("    The admin account will be preserved.")

	confirm := prompt("\nType WIPE to confirm: ")
	if confirm != "WIPE" {
		log.Fatal("Aborted — nothing was deleted")
	}

	database.DB.Exec("DELETE FROM documents")
	database.DB.Exec("DELETE FROM collections")
	database.DB.Exec("DELETE FROM app_users")
	database.DB.Exec("DELETE FROM activity_logs")
	database.DB.Exec("DELETE FROM projects")

	fmt.Println("✅ All project data has been wiped.")
}

func cmdListProjects() {
	var projects []models.Project
	database.DB.Order("created_at desc").Find(&projects)

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return
	}

	fmt.Printf("\n%-30s  %-38s  %s\n", "Name", "ID", "API Key")
	fmt.Println(strings.Repeat("-", 100))
	for _, p := range projects {
		active := ""
		if !p.Active {
			active = " [inactive]"
		}
		fmt.Printf("%-30s  %-38s  %s%s\n", p.Name, p.ID, p.APIKey, active)
	}
	fmt.Printf("\n%d project(s)\n", len(projects))
}

// ── Server ────────────────────────────────────────────────────────────────────

func runServe() {
	serveFlags := flag.NewFlagSet("serve", flag.ExitOnError)
	portOverride := serveFlags.String("port", "", "Override PORT")
	_ = serveFlags.String("env", ".env", "Path to .env file") // godotenv is loaded in config

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	serveFlags.Parse(args)

	cfg := config.LoadConfig()
	if *portOverride != "" {
		cfg.Port = *portOverride
	}

	if cfg.DatabaseURL != "" {
		if err := database.Connect(cfg.DatabaseURL, cfg.Environment == "development"); err != nil {
			log.Fatalf("❌ Failed to connect to database: %v", err)
		}
		defer database.Close()

		services.InitRedis()

		if err := services.InitializeS3(); err != nil {
			log.Fatalf("❌ Failed to initialize S3: %v", err)
		}

		handlers.InitHandlerServices()

		if err := database.Migrate(); err != nil {
			log.Printf("⚠️  Database migration warning: %v", err)
		}

		dashhandlers.LoadDashboardConfigIntoAppConfig()
	} else {
		log.Println("⚠️  No DATABASE_URL provided, running without database connection")
	}

	app := fiber.New(fiber.Config{
		AppName:      "Cocobase v0.1.0",
		ServerHeader: "Cocobase",
		ErrorHandler: customErrorHandler,

		Prefork:              false,
		CaseSensitive:        true,
		StrictRouting:        false,
		Concurrency:          256 * 1024,
		ReadBufferSize:       4096,
		WriteBufferSize:      4096,
		CompressedFileSuffix: ".gz",

		DisablePreParseMultipartForm: true,
		ReduceMemoryUsage:            false,
	})
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
		Next: func(c *fiber.Ctx) bool {
			return len(c.Path()) >= 2 && c.Path()[:2] == "/_"
		},
	}))

	middleware.SetupMiddleware(app)
	routes.SetupRoutes(app)
	routes.SetupDashboardRoutes(app)
	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	port := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("🚀 Cocobase v0.1.0 starting on port %s in %s mode", cfg.Port, cfg.Environment)
	log.Printf("📊 Dashboard: http://localhost%s/_/", port)
	log.Fatal(app.Listen(port))
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func prompt(label string) string {
	fmt.Print(label)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

func promptPassword(label string) string {
	fmt.Print(label)
	// Use terminal raw mode to hide input if stdin is a terminal
	if term.IsTerminal(int(syscall.Stdin)) {
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err == nil {
			return string(b)
		}
	}
	// Fallback: read normally (e.g. piped input)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}

// customErrorHandler handles errors globally
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{
		"error":   true,
		"message": err.Error(),
	})
}

