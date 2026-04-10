package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/middleware"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	fn "github.com/patrick/cocobase/internal/services/functions"
)

// HandleHTTPFunction is mounted at /fn/* and dispatches to matching HTTP functions.
func HandleHTTPFunction(c *fiber.Ctx) error {
	project := middleware.GetProject(c)
	if project == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": true, "message": "Unauthorized"})
	}

	// The path after /fn/
	reqPath := "/" + c.Params("*")
	method := strings.ToUpper(c.Method())

	// Find a matching HTTP function for this project
	var fns []models.Function
	database.DB.Where(
		"project_id = ? AND trigger_type = ? AND enabled = true",
		project.ID, models.TriggerHTTP,
	).Find(&fns)

	var matched *models.Function
	for i := range fns {
		cfg := fns[i].TriggerConfig
		fnMethod := strings.ToUpper(cfg.Method)
		if fnMethod != "ANY" && fnMethod != method && fnMethod != "" {
			continue
		}
		if matchPath(cfg.Path, reqPath) {
			matched = &fns[i]
			break
		}
	}

	if matched == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "No function found for " + method + " " + reqPath,
		})
	}

	// Build headers map
	headers := map[string]string{}
	c.Request().Header.VisitAll(func(k, v []byte) {
		headers[string(k)] = string(v)
	})

	// Build query params map
	query := map[string]string{}
	c.Request().URI().QueryArgs().VisitAll(func(k, v []byte) {
		query[string(k)] = string(v)
	})

	appUser := middleware.GetAppUserFromContext(c)

	rctx := &fn.RunContext{
		ReqMethod:  method,
		ReqPath:    reqPath,
		ReqHeaders: headers,
		ReqBody:    string(c.Body()),
		ReqQuery:   query,
		User:       appUser,
		ProjectID:  project.ID,
		Broadcast:  BroadcastToProject,
	}

	if err := fn.Execute(matched, rctx); err != nil && !rctx.Responded {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": err.Error(),
		})
	}

	if rctx.Responded {
		status := rctx.ResponseStatus
		if status == 0 {
			status = fiber.StatusOK
		}
		// Apply custom headers
		for k, v := range rctx.ResponseHeaders {
			c.Set(k, v)
		}
		// Auto-detect content type if not set
		if _, ok := rctx.ResponseHeaders["Content-Type"]; !ok {
			body := rctx.ResponseBody
			if len(body) > 0 && (body[0] == '{' || body[0] == '[') {
				c.Set("Content-Type", "application/json")
			} else if strings.HasPrefix(strings.TrimSpace(body), "<") {
				c.Set("Content-Type", "text/html; charset=utf-8")
			} else {
				c.Set("Content-Type", "text/plain; charset=utf-8")
			}
		}
		return c.Status(status).SendString(rctx.ResponseBody)
	}

	return c.JSON(fiber.Map{"status": "ok"})
}

// matchPath does simple path matching — exact or with * wildcard at end.
func matchPath(pattern, path string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(path, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == path
}

// BroadcastToProject is the pub/sub broadcaster injected into RunContext.
// Functions call ctx.publish(channel, data) which routes here.
var BroadcastToProject func(channel string, data interface{}) = func(channel string, data interface{}) {
	doc, _ := data.(map[string]interface{})
	services.PublishEvent(services.RealtimeEvent{
		CollectionID: channel,
		Action:       "publish",
		Document:     doc,
		Timestamp:    time.Now(),
	})
}
