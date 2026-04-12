package dashboard

import (
	"bufio"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	applogger "github.com/patrick/cocobase/pkg/logger"
)

// Log writes an activity log entry. Call after any mutating dashboard action.
func Log(projectID, action, resource, resourceID, detail string) {
	entry := models.ActivityLog{
		ProjectID:  projectID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
	}
	database.DB.Create(&entry)
}

// ListLogs handles GET /_/api/projects/:id/logs
// It reads the server log file and returns the most recent lines.
// Each line is returned as-is so the dashboard can render the raw log output.
func ListLogs(c *fiber.Ctx) error {
	if _, err := getProjectByID(c.Params("id")); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": true, "message": "Project not found"})
	}

	limit := c.QueryInt("limit", 100)
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	logPath := applogger.LogFile()
	f, err := os.Open(logPath)
	if err != nil {
		// Log file may not exist yet — return empty list
		return c.JSON(fiber.Map{"data": []string{}, "total": 0})
	}
	defer f.Close()

	// Read all lines, keep last `limit` lines (tail behaviour)
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}

	total := len(lines)
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}

	// Reverse so newest is first
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return c.JSON(fiber.Map{
		"data":  lines,
		"total": total,
	})
}
