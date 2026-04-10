package services

import (
	"fmt"
	"log"

	"gopkg.in/gomail.v2"

	"github.com/patrick/cocobase/pkg/config"
)

// EmailMessage holds the data for an outgoing email
type EmailMessage struct {
	To      string
	Subject string
	HTML    string
	Text    string // optional plain-text fallback
}

// SendEmail sends an email using the configured SMTP settings.
// Falls back gracefully and logs if SMTP is not configured.
func SendEmail(msg EmailMessage) error {
	cfg := config.AppConfig
	if cfg == nil || cfg.SMTPHost == "" {
		log.Printf("📧 SMTP not configured — skipping email to %s (subject: %s)", msg.To, msg.Subject)
		return nil // soft fail: don't break flows when mailer isn't set up
	}

	m := gomail.NewMessage()
	from := cfg.SMTPFrom
	if from == "" {
		from = cfg.SMTPUsername
	}
	fromName := cfg.SMTPFromName
	if fromName == "" {
		fromName = "Cocobase"
	}

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

	d := gomail.NewDialer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword)
	d.SSL = cfg.SMTPSecure

	if err := d.DialAndSend(m); err != nil {
		log.Printf("❌ Failed to send email to %s: %v", msg.To, err)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("✅ Email sent to %s (subject: %s)", msg.To, msg.Subject)
	return nil
}

// SendPasswordResetEmail sends a password reset email
func SendPasswordResetEmail(toEmail, resetToken, frontendURL string) error {
	resetLink := fmt.Sprintf("%s/auth-collections/reset-password-page?token=%s", frontendURL, resetToken)
	return SendEmail(EmailMessage{
		To:      toEmail,
		Subject: "Reset your password",
		HTML: fmt.Sprintf(`<p>Click the link below to reset your password. It expires in 1 hour.</p>
<p><a href="%s">Reset Password</a></p>
<p>If you didn't request this, ignore this email.</p>`, resetLink),
		Text: fmt.Sprintf("Reset your password: %s\n\nExpires in 1 hour.", resetLink),
	})
}

// SendVerificationEmail sends an email verification email
func SendVerificationEmail(toEmail, verifyToken, frontendURL string) error {
	verifyLink := fmt.Sprintf("%s/verify-email?token=%s", frontendURL, verifyToken)
	return SendEmail(EmailMessage{
		To:      toEmail,
		Subject: "Verify your email address",
		HTML: fmt.Sprintf(`<p>Click the link below to verify your email address.</p>
<p><a href="%s">Verify Email</a></p>
<p>This link expires in 24 hours.</p>`, verifyLink),
		Text: fmt.Sprintf("Verify your email: %s\n\nExpires in 24 hours.", verifyLink),
	})
}
