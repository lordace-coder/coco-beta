package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"gopkg.in/gomail.v2"

	"github.com/patrick/cocobase/internal/models"
	"github.com/patrick/cocobase/pkg/config"
)

// EmailMessage holds the data for an outgoing email.
type EmailMessage struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// MailerConfig holds resolved mailer settings for a project.
// Priority: project Configs > global .env
type MailerConfig struct {
	// SMTP
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string
	SMTPSecure   bool
	// Resend
	ResendAPIKey string
	ResendFrom   string
}

// resolveMailerConfig builds a MailerConfig for a project.
// Project-level configs (stored in project.Configs) override global .env.
func resolveMailerConfig(project *models.Project) MailerConfig {
	cfg := config.AppConfig
	mc := MailerConfig{}

	// Start with global .env defaults
	if cfg != nil {
		mc.SMTPHost = cfg.SMTPHost
		mc.SMTPPort = cfg.SMTPPort
		mc.SMTPUsername = cfg.SMTPUsername
		mc.SMTPPassword = cfg.SMTPPassword
		mc.SMTPFrom = cfg.SMTPFrom
		mc.SMTPFromName = cfg.SMTPFromName
		mc.SMTPSecure = cfg.SMTPSecure
	}
	if mc.SMTPPort == 0 {
		mc.SMTPPort = 587
	}

	if project == nil {
		return mc
	}

	// Override with project-level configs
	str := func(key string) string {
		if v, ok := project.Configs[key]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	if v := str("smtp_host"); v != "" {
		mc.SMTPHost = v
	}
	if v := str("smtp_port"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			mc.SMTPPort = p
		}
	}
	if v := str("smtp_username"); v != "" {
		mc.SMTPUsername = v
	}
	if v := str("smtp_password"); v != "" {
		mc.SMTPPassword = v
	}
	if v := str("smtp_from"); v != "" {
		mc.SMTPFrom = v
	}
	if v := str("smtp_from_name"); v != "" {
		mc.SMTPFromName = v
	}
	if v := str("smtp_secure"); v != "" {
		mc.SMTPSecure = strings.ToLower(v) == "true"
	}
	if v := str("resend_api_key"); v != "" {
		mc.ResendAPIKey = v
	}
	if v := str("resend_from"); v != "" {
		mc.ResendFrom = v
	}

	return mc
}

// SendEmail sends an email using the project's mailer config.
// Falls back to global config, then soft-fails if nothing is configured.
func SendEmail(msg EmailMessage) error {
	return SendEmailForProject(nil, msg)
}

// SendEmailForProject sends using the project's mailer settings.
// Tries Resend first if configured, then SMTP, then soft-fails.
func SendEmailForProject(project *models.Project, msg EmailMessage) error {
	mc := resolveMailerConfig(project)

	// Try Resend first
	if mc.ResendAPIKey != "" {
		return sendViaResend(mc, msg)
	}

	// Try SMTP
	if mc.SMTPHost != "" {
		return sendViaSMTP(mc, msg)
	}

	// Nothing configured — soft fail
	log.Printf("📧 No mailer configured — skipping email to %s (subject: %s)", msg.To, msg.Subject)
	return nil
}

func sendViaSMTP(mc MailerConfig, msg EmailMessage) error {
	from := mc.SMTPFrom
	if from == "" {
		from = mc.SMTPUsername
	}
	fromName := mc.SMTPFromName
	if fromName == "" {
		fromName = "Cocobase"
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("%s <%s>", fromName, from))
	m.SetHeader("To", msg.To)
	m.SetHeader("Subject", msg.Subject)

	if msg.HTML != "" {
		m.SetBody("text/html", msg.HTML)
		if msg.Text != "" {
			m.AddAlternative("text/plain", msg.Text)
		}
	} else {
		m.SetBody("text/plain", msg.Text)
	}

	d := gomail.NewDialer(mc.SMTPHost, mc.SMTPPort, mc.SMTPUsername, mc.SMTPPassword)
	d.SSL = mc.SMTPSecure

	if err := d.DialAndSend(m); err != nil {
		log.Printf("❌ SMTP failed to %s: %v", msg.To, err)
		return fmt.Errorf("SMTP send failed: %w", err)
	}

	log.Printf("✅ Email sent via SMTP to %s (subject: %s)", msg.To, msg.Subject)
	return nil
}

func sendViaResend(mc MailerConfig, msg EmailMessage) error {
	from := mc.ResendFrom
	if from == "" {
		from = "onboarding@resend.dev"
	}

	body := map[string]interface{}{
		"from":    from,
		"to":      []string{msg.To},
		"subject": msg.Subject,
	}
	if msg.HTML != "" {
		body["html"] = msg.HTML
	}
	if msg.Text != "" {
		body["text"] = msg.Text
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("resend: failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("resend: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+mc.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var resErr map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&resErr)
		return fmt.Errorf("resend: API error %d: %v", resp.StatusCode, resErr)
	}

	log.Printf("✅ Email sent via Resend to %s (subject: %s)", msg.To, msg.Subject)
	return nil
}

// SendPasswordResetEmail sends a password reset email for a project.
func SendPasswordResetEmail(project *models.Project, toEmail, resetToken, frontendURL string) error {
	resetLink := fmt.Sprintf("%s/auth-collections/reset-password-page?token=%s", frontendURL, resetToken)
	return SendEmailForProject(project, EmailMessage{
		To:      toEmail,
		Subject: "Reset your password",
		HTML: fmt.Sprintf(`<p>Click the link below to reset your password. It expires in 1 hour.</p>
<p><a href="%s">Reset Password</a></p>
<p>If you didn't request this, ignore this email.</p>`, resetLink),
		Text: fmt.Sprintf("Reset your password: %s\n\nExpires in 1 hour.", resetLink),
	})
}

// SendVerificationEmail sends a verification email for a project.
func SendVerificationEmail(project *models.Project, toEmail, verifyToken, frontendURL string) error {
	verifyLink := fmt.Sprintf("%s/verify-email?token=%s", frontendURL, verifyToken)
	return SendEmailForProject(project, EmailMessage{
		To:      toEmail,
		Subject: "Verify your email address",
		HTML: fmt.Sprintf(`<p>Click the link below to verify your email address.</p>
<p><a href="%s">Verify Email</a></p>
<p>This link expires in 24 hours.</p>`, verifyLink),
		Text: fmt.Sprintf("Verify your email: %s\n\nExpires in 24 hours.", verifyLink),
	})
}
