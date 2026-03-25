package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers"
	"github.com/patrick/cocobase/internal/api/middleware"
)

// SetupAuthRoutes sets up all authentication routes for app users
func SetupAuthRoutes(app *fiber.App) {
	// Auth routes with API key authentication (matching Python API structure)
	auth := app.Group("/auth-collections", middleware.RequireAPIKey)

	// Public auth endpoints (require API key only)
	auth.Post("/login", handlers.UserLogin)
	auth.Post("/signup", handlers.UserSignup)

	// Google OAuth - Method 1: Redirect flow
	auth.Get("/login-google", handlers.LoginWithGoogle)

	// Google OAuth - Method 2: Frontend token verification
	auth.Post("/verify-google-token", handlers.VerifyGoogleToken)

	// User management endpoints (require API key)
	auth.Get("/users", handlers.ListAllUsers)
	auth.Get("/users/:id", handlers.GetUserByID)

	// Protected endpoints (require valid JWT token in Authorization header)
	auth.Get("/user", middleware.RequireAppUser, handlers.GetCurrentUser)
	auth.Patch("/user", middleware.RequireAppUser, handlers.UpdateCurrentUser)

}
