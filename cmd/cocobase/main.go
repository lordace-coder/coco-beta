package main

import (
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/patrick/cocobase/internal/api/handlers"
	dashhandlers "github.com/patrick/cocobase/internal/api/handlers/dashboard"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/api/routes"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/services"
	"github.com/patrick/cocobase/pkg/config"

	_ "github.com/patrick/cocobase/docs" // Import generated docs
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

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Connect to database
	if cfg.DatabaseURL != "" {
		if err := database.Connect(cfg.DatabaseURL, cfg.Environment == "development"); err != nil {
			log.Fatalf("❌ Failed to connect to database: %v", err)
		}
		defer database.Close()

		// Initialize Redis for real-time features
		services.InitRedis()

		// Initialize S3/Backblaze B2 for file storage
		if err := services.InitializeS3(); err != nil {
			log.Fatalf("❌ Failed to initialize S3: %v", err)
		}

		// Initialize handler services after database connection
		handlers.InitHandlerServices()

		// Auto-migrate dashboard models
		if err := database.Migrate(); err != nil {
			log.Printf("⚠️  Database migration warning: %v", err)
		}

		// Apply dashboard config overrides (SMTP, etc.) from DB over .env
		dashhandlers.LoadDashboardConfigIntoAppConfig()
	} else {
		log.Println("⚠️  No DATABASE_URL provided, running without database connection")
	}

	// Create Fiber app with performance optimizations
	app := fiber.New(fiber.Config{
		AppName:      "Cocobase v1.0.0",
		ServerHeader: "Cocobase",
		ErrorHandler: customErrorHandler,

		// Performance optimizations
		Prefork:              false, // Set to true in production for multi-core performance
		CaseSensitive:        true,  // Faster routing
		StrictRouting:        false,
		Concurrency:          256 * 1024, // Max concurrent connections
		ReadBufferSize:       4096,       // Buffer sizes
		WriteBufferSize:      4096,
		CompressedFileSuffix: ".gz",

		// Reduce allocations
		DisablePreParseMultipartForm: true,
		ReduceMemoryUsage:            false, // Trade memory for speed
	})
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))
	// Setup middleware
	middleware.SetupMiddleware(app)

	// Setup routes
	routes.SetupRoutes(app)

	// Setup admin dashboard routes
	routes.SetupDashboardRoutes(app)

	// Swagger documentation
	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	// Start server
	port := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("🚀 Cocobase server starting on port %s in %s mode", cfg.Port, cfg.Environment)
	log.Fatal(app.Listen(port))
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
