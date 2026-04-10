package dashboard

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// AdminLogin handles POST /_/api/auth/login
func AdminLogin(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "email and password are required"})
	}

	var admin models.AdminUser
	if err := database.DB.Where("email = ?", strings.ToLower(req.Email)).First(&admin).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Invalid credentials"})
	}

	token, err := services.CreateAdminToken(admin.ID, admin.Email)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to generate token"})
	}

	return c.JSON(fiber.Map{
		"access_token": token,
		"admin": fiber.Map{
			"id":    admin.ID,
			"email": admin.Email,
		},
	})
}

// AdminMe handles GET /_/api/auth/me
func AdminMe(c *fiber.Ctx) error {
	admin := middleware.GetAdmin(c)
	return c.JSON(fiber.Map{
		"id":         admin.ID,
		"email":      admin.Email,
		"created_at": admin.CreatedAt,
	})
}

// SetupStatus handles GET /_/api/auth/setup-status
// Returns whether an admin account exists (used by frontend to decide login vs setup)
func SetupStatus(c *fiber.Ctx) error {
	var count int64
	database.DB.Model(&models.AdminUser{}).Count(&count)
	return c.JSON(fiber.Map{"setup_complete": count > 0})
}

// CreateAdmin handles POST /_/api/auth/setup
// Only works when no admin exists yet (first-run)
func CreateAdmin(c *fiber.Ctx) error {
	var count int64
	database.DB.Model(&models.AdminUser{}).Count(&count)
	if count > 0 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": true, "message": "Admin already exists"})
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "email and password are required"})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "password must be at least 8 characters"})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to hash password"})
	}

	admin := models.AdminUser{
		ID:       uuid.New().String(),
		Email:    strings.ToLower(req.Email),
		Password: string(hashed),
	}
	if err := database.DB.Create(&admin).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": true, "message": "Failed to create admin"})
	}

	token, _ := services.CreateAdminToken(admin.ID, admin.Email)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"access_token": token,
		"admin": fiber.Map{
			"id":    admin.ID,
			"email": admin.Email,
		},
	})
}
