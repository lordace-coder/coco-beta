package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/services"
)

// UploadFile handles file uploads to Backblaze B2 storage
// @Summary Upload file to project
// @Description Upload a file to Backblaze B2 storage with storage limit checking
// @Tags Collections
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "File to upload"
// @Param subdirectory query string false "Subdirectory path within project folder"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 413 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/file [post]
func UploadFile(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	// Get file from request
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "File is required",
		})
	}

	// Open file
	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to read file",
		})
	}
	defer file.Close()

	// Read file content
	fileContent := make([]byte, fileHeader.Size)
	_, err = file.Read(fileContent)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": "Failed to read file content",
		})
	}

	// Get optional subdirectory parameter
	subdirectory := c.Query("subdirectory", "")

	// Generate unique filename
	ext := filepath.Ext(fileHeader.Filename)
	baseFilename := fileHeader.Filename[:len(fileHeader.Filename)-len(ext)]
	uniqueID := uuid.New().String()[:8]
	timestamp := time.Now().Unix()
	newFilename := fmt.Sprintf("%s_%d_%s%s", baseFilename, timestamp, uniqueID, ext)

	// Upload to S3 in background (non-blocking for response)
	ctx := context.Background()
	result, err := services.UploadFile(ctx, fileContent, project.ID, newFilename, subdirectory)
	if err != nil {
		// Check if it's a storage limit error
		if err.Error()[:14] == "storage limit" {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"error":   true,
				"message": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to upload file: %v", err),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":       true,
		"filename":      result.Filename,
		"original_name": fileHeader.Filename,
		"size":          result.Size,
		"url":           result.URL,
		"content_type":  fileHeader.Header.Get("Content-Type"),
		"uploaded_at":   result.LastModified,
	})
}

// ListFiles lists all files for a project
// @Summary List project files
// @Description List all files uploaded to project storage
// @Tags Collections
// @Produce json
// @Param subdirectory query string false "Subdirectory to list files from"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/files [get]
func ListFiles(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	subdirectory := c.Query("subdirectory", "")

	files, err := services.GetFiles(project.ID, subdirectory)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to list files: %v", err),
		})
	}

	// Get storage usage
	usage, _ := services.GetProjectStorageUsage(project.ID)
	plan := services.GetCurrentPlan(project)

	var storageLimitBytes int64
	var limitMB *int
	if plan.MaxStorageMB != nil {
		storageLimitBytes = int64(*plan.MaxStorageMB) * 1024 * 1024
		limitMB = plan.MaxStorageMB
	}

	return c.JSON(fiber.Map{
		"files":         files,
		"total_files":   len(files),
		"storage_usage": usage,
		"storage_limit": storageLimitBytes, // 0 if unlimited
		"usage_mb":      float64(usage) / 1024 / 1024,
		"limit_mb":      limitMB, // nil if unlimited
	})
}

// DeleteFile deletes a file from storage
// @Summary Delete project file
// @Description Delete a file from project storage
// @Tags Collections
// @Produce json
// @Param filename query string true "Filename to delete"
// @Param subdirectory query string false "Subdirectory path"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /collections/file [delete]
func DeleteFile(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error":   true,
			"message": "Unauthorized",
		})
	}

	filename := c.Query("filename")
	if filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   true,
			"message": "Filename is required",
		})
	}

	subdirectory := c.Query("subdirectory", "")

	err := services.DeleteFile(project.ID, filename, subdirectory)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": fmt.Sprintf("Failed to delete file: %v", err),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "File deleted successfully",
	})
}
