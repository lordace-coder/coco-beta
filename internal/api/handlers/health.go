package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// HealthCheck handles the health check endpoint
func HealthCheck(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status":  "ok",
		"message": "Cocobase is running",
		"service": "cocobase",
	})
}

// Welcome handles the root endpoint
func Welcome(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Welcome to Cocobase - Backend as a Service",
		"version": "1.0.0",
		"docs":    "/api/docs",
	})
}
