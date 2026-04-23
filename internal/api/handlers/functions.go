package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/api/middleware"
	fn "github.com/patrick/cocobase/internal/services/functions"
	"github.com/patrick/cocobase/internal/services"
)

// HandleHTTPFunction is mounted at /functions/:projectId/func/:path[/*]
// It dispatches to the matching route registered in the project's functions.js.
func HandleHTTPFunction(c *fiber.Ctx) error {

	// Build the request path: everything after /func  → /:functionName[/*]
	funcName := c.Params("functionName")
	extra := c.Params("*")
	if extra != "" {
		extra = "/" + extra
	}
	reqPath := "/" + funcName + extra

	method := strings.ToUpper(c.Method())

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
		ReqMethod:   method,
		ReqPath:     reqPath,
		ReqHeaders:  headers,
		ReqBody:     string(c.Body()),
		ReqQuery:    query,
		User:        appUser,
		ProjectID:   instanceID(),
		ProjectName: "default",
		Broadcast:   BroadcastToProject,
	}

	responded, err := fn.DispatchHTTP(instanceID(), "default", rctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   true,
			"message": err.Error(),
		})
	}

	if !responded {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   true,
			"message": "No route matched " + method + " " + reqPath,
		})
	}

	status := rctx.ResponseStatus
	if status == 0 {
		status = fiber.StatusOK
	}
	for k, v := range rctx.ResponseHeaders {
		c.Set(k, v)
	}
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

// BroadcastToProject is the pub/sub broadcaster injected into RunContext.
var BroadcastToProject func(channel string, data interface{}) = func(channel string, data interface{}) {
	doc, _ := data.(map[string]interface{})
	services.PublishEvent(services.RealtimeEvent{
		CollectionID: channel,
		Action:       "publish",
		Document:     doc,
		Timestamp:    time.Now(),
	})
}
