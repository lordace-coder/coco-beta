package dashboard

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/patrick/cocobase/internal/database"
	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/internal/services"
	"github.com/patrick/cocobase/pkg/config"
)

// GetConfig handles GET /_/api/config
func GetConfig(c *fiber.Ctx) error {
	var configs []models.DashboardConfig
	database.DB.Find(&configs)

	result := make([]fiber.Map, len(configs))
	for i, cfg := range configs {
		val := cfg.Value
		if cfg.IsSecret && val != "" {
			val = "••••••••"
		}
		result[i] = fiber.Map{
			"key":       cfg.Key,
			"value":     val,
			"is_secret": cfg.IsSecret,
		}
	}
	return c.JSON(fiber.Map{"data": result})
}

// UpdateConfig handles PATCH /_/api/config
// Body: [{key, value}] or {key, value}
func UpdateConfig(c *fiber.Ctx) error {
	var items []struct {
		Key      string `json:"key"`
		Value    string `json:"value"`
		IsSecret *bool  `json:"is_secret"`
	}
	if err := c.BodyParser(&items); err != nil {
		// Try single object
		var single struct {
			Key      string `json:"key"`
			Value    string `json:"value"`
			IsSecret *bool  `json:"is_secret"`
		}
		if err2 := c.BodyParser(&single); err2 != nil || single.Key == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "Invalid request body"})
		}
		items = append(items, single)
	}

	secretKeys := map[string]bool{
		"smtp.password": true,
		"admin.password": true,
	}

	for _, item := range items {
		if item.Key == "" {
			continue
		}
		isSecret := secretKeys[item.Key]
		if item.IsSecret != nil {
			isSecret = *item.IsSecret
		}

		var existing models.DashboardConfig
		err := database.DB.Where("key = ?", item.Key).First(&existing).Error
		if err != nil {
			// Create
			cfg := models.DashboardConfig{Key: item.Key, Value: item.Value, IsSecret: isSecret}
			database.DB.Create(&cfg)
		} else {
			database.DB.Model(&existing).Updates(map[string]interface{}{"value": item.Value, "is_secret": isSecret})
		}
	}

	// Reload config into memory
	reloadSMTPFromDB()

	return c.JSON(fiber.Map{"message": "Config updated"})
}

// TestSMTP handles POST /_/api/config/smtp/test
func TestSMTP(c *fiber.Ctx) error {
	var req struct {
		To string `json:"to"`
	}
	if err := c.BodyParser(&req); err != nil || req.To == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": "to email is required"})
	}

	err := services.SendEmail(services.EmailMessage{
		To:      req.To,
		Subject: "Cocobase SMTP Test",
		HTML:    "<p>Your SMTP configuration is working correctly.</p>",
		Text:    "Your SMTP configuration is working correctly.",
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": true, "message": fmt.Sprintf("SMTP test failed: %v", err)})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Test email sent successfully"})
}

// reloadSMTPFromDB reads SMTP settings from dashboard_configs and updates AppConfig.
// Dashboard config takes priority over .env.
func reloadSMTPFromDB() {
	if config.AppConfig == nil {
		return
	}
	keys := []string{"smtp.host", "smtp.port", "smtp.username", "smtp.password", "smtp.from", "smtp.from_name", "smtp.secure"}
	vals := map[string]string{}

	var configs []models.DashboardConfig
	database.DB.Where("key IN ?", keys).Find(&configs)
	for _, c := range configs {
		vals[c.Key] = c.Value
	}

	if v, ok := vals["smtp.host"]; ok && v != "" {
		config.AppConfig.SMTPHost = v
	}
	if v, ok := vals["smtp.port"]; ok && v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			config.AppConfig.SMTPPort = p
		}
	}
	if v, ok := vals["smtp.username"]; ok && v != "" {
		config.AppConfig.SMTPUsername = v
	}
	if v, ok := vals["smtp.password"]; ok && v != "" {
		config.AppConfig.SMTPPassword = v
	}
	if v, ok := vals["smtp.from"]; ok && v != "" {
		config.AppConfig.SMTPFrom = v
	}
	if v, ok := vals["smtp.from_name"]; ok && v != "" {
		config.AppConfig.SMTPFromName = v
	}
	if v, ok := vals["smtp.secure"]; ok {
		config.AppConfig.SMTPSecure = strings.ToLower(v) == "true"
	}
}

// LoadDashboardConfigIntoAppConfig should be called at startup to apply DB config over .env.
func LoadDashboardConfigIntoAppConfig() {
	reloadSMTPFromDB()
}

// testSMTPDirect is used internally for validation with custom params (not relying on AppConfig).
func testSMTPDirect(host string, port int, username, password string, secure bool) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.PlainAuth("", username, password, host)

	if secure {
		tlsConfig := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer client.Close()
		return client.Auth(auth)
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return err
	}
	return client.Auth(auth)
}
