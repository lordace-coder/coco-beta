package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	applogger "github.com/patrick/cocobase/pkg/logger"
)

// SetupMiddleware configures all application middleware
func SetupMiddleware(app *fiber.App) {
	// Recover from panics — log them before re-raising
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			applogger.Error("PANIC recovered: %v | %s %s | ip=%s", e, c.Method(), c.Path(), c.IP())
		},
	}))

	// Detailed request logger
	app.Use(requestLogger())

	// CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, x-api-key",
		AllowMethods: "GET, POST, PUT, DELETE, PATCH, OPTIONS",
	}))
}

// requestLogger is a custom Fiber middleware that writes detailed structured
// logs for every request — including headers, body size, latency, and errors.
func requestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Run the handler chain
		chainErr := c.Next()

		status := c.Response().StatusCode()
		errMsg := ""
		if chainErr != nil {
			errMsg = chainErr.Error()
			if status == 200 {
				status = fiber.StatusInternalServerError
			}
		}

		applogger.LogRequest(applogger.RequestLog{
			Method:      c.Method(),
			Path:        c.Path(),
			Query:       string(c.Request().URI().QueryString()),
			IP:          c.IP(),
			UserAgent:   c.Get("User-Agent"),
			APIKey:      c.Get("x-api-key"),
			AuthHeader:  c.Get("Authorization"),
			ContentType: c.Get("Content-Type"),
			BodySize:    len(c.Body()),
			Status:      status,
			Latency:     time.Since(start),
			Err:         errMsg,
		})

		return chainErr
	}
}
