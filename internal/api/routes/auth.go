package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/handlers"
	"github.com/patrick/cocobase/internal/api/middleware"
)

// SetupAuthRoutes sets up all authentication routes for app users
func SetupAuthRoutes(app *fiber.App) {
	auth := app.Group("/auth-collections", middleware.RequireAPIKey)

	// Basic auth
	auth.Post("/login", handlers.UserLogin)
	auth.Post("/signup", handlers.UserSignup)

	// OAuth - social login (verify tokens obtained by client)
	auth.Post("/google-verify", handlers.VerifyGoogleToken)
	auth.Post("/github-verify", handlers.VerifyGitHubToken)
	auth.Post("/apple-verify", handlers.VerifyAppleToken)

	// Google redirect-based flow (optional)
	auth.Get("/login-google", handlers.LoginWithGoogle)

	// User management
	auth.Get("/users", handlers.ListAllUsers)
	auth.Get("/users/:id", handlers.GetUserByID)

	// Password reset (no JWT needed, uses reset token)
	auth.Get("/reset-password-page", handlers.ResetPasswordPage)
	auth.Post("/forgot-password", handlers.ForgotPassword)
	auth.Post("/reset-password", handlers.ResetPassword)

	// Protected endpoints (require valid JWT)
	authJWT := app.Group("/auth-collections", middleware.RequireAPIKey, middleware.RequireAppUser)
	authJWT.Get("/user", handlers.GetCurrentUser)
	authJWT.Patch("/user", handlers.UpdateCurrentUser)

	// Email verification (send/resend require JWT; verify uses token only)
	authJWT.Post("/verify-email/send", handlers.SendVerificationEmail)
	authJWT.Post("/verify-email/resend", handlers.ResendVerificationEmail)
	auth.Post("/verify-email/verify", handlers.VerifyEmail)
}
