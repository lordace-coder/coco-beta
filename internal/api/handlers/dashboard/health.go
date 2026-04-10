package dashboard

import (
	"context"
	"runtime"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/services"
)

var startTime = time.Now()

// DashboardHealth handles GET /_/api/health
func DashboardHealth(c *fiber.Ctx) error {
	svcStatus := fiber.Map{}

	// Check database
	sqlDB, err := database.DB.DB()
	if err == nil {
		if pingErr := sqlDB.PingContext(context.Background()); pingErr != nil {
			svcStatus["database"] = fiber.Map{"status": "error", "message": pingErr.Error()}
		} else {
			svcStatus["database"] = fiber.Map{"status": "ok"}
		}
	} else {
		svcStatus["database"] = fiber.Map{"status": "error", "message": err.Error()}
	}

	// Check Redis if configured
	if rc := services.GetRedisClient(); rc != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if pingErr := rc.Ping(ctx).Err(); pingErr != nil {
			svcStatus["redis"] = fiber.Map{"status": "error", "message": pingErr.Error()}
		} else {
			svcStatus["redis"] = fiber.Map{"status": "ok"}
		}
	} else {
		svcStatus["redis"] = fiber.Map{"status": "not_configured"}
	}

	overallStatus := "ok"
	for _, v := range svcStatus {
		if m, ok := v.(fiber.Map); ok {
			if m["status"] == "error" {
				overallStatus = "degraded"
				break
			}
		}
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return c.JSON(fiber.Map{
		"status":   overallStatus,
		"uptime":   time.Since(startTime).String(),
		"services": svcStatus,
		"memory": fiber.Map{
			"alloc_mb":       mem.Alloc / 1024 / 1024,
			"sys_mb":         mem.Sys / 1024 / 1024,
			"num_goroutines": runtime.NumGoroutine(),
		},
	})
}
