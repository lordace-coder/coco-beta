package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	Environment string
	DatabaseURL string
	JWTSecret   string
	APIVersion  string

	// File storage (Backblaze B2 / S3-compatible)
	BackblazeKeyID          string
	BackblazeKeyName        string
	BackblazeApplicationKey string
	BucketName              string
	BucketEndpoint          string

	// App URLs
	FrontendURL string

	// Redis (optional - for realtime features)
	RedisURL string

	// Mailer (optional - for email features)
	MailerAPIKey string
	MailerURL    string

	// SMTP (optional - configure to send emails directly)
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string
	SMTPSecure   bool // true = TLS, false = STARTTLS/plain

	// OAuth (optional)
	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string
	AppleClientID      string
	AppleTeamID        string
	AppleKeyID         string
	ApplePrivateKey    string

	// Admin dashboard
	AdminEmail    string
	AdminPassword string

	// Rate limiting (optional, 0 = unlimited)
	RateLimitRequests int
	RateLimitWindow   int // seconds
}

var AppConfig *Config

// LoadConfig loads environment variables and initializes the config
func LoadConfig() *Config {
	// Load .env file — try current dir, then up two levels (for air running from bin/)
	loaded := false
	for _, path := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(path); err == nil {
			loaded = true
			break
		}
	}
	if !loaded {
		log.Println("No .env file found, using system environment variables")
	}

	config := &Config{
		Port:        getEnv("PORT", "3000"),
		Environment: getEnv("ENVIRONMENT", "development"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		JWTSecret:   getEnv("SECRET", "change-me-in-production"),
		APIVersion:  getEnv("API_VERSION", "v1"),

		BackblazeKeyID:          getEnv("BACKBLAZE_KEY_ID", ""),
		BackblazeKeyName:        getEnv("BACKBLAZE_KEY_NAME", ""),
		BackblazeApplicationKey: getEnv("BACKBLAZE_APPLICATION_KEY", ""),
		BucketName:              getEnv("BUCKET_NAME", ""),
		BucketEndpoint:          getEnv("BUCKET_ENDPOINT", ""),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		RedisURL:    getEnv("REDIS_URL", ""),

		MailerAPIKey: getEnv("MAILER_API_KEY", ""),
		MailerURL:    getEnv("MAILER_URL", ""),

		SMTPHost:     getEnv("SMTP_HOST", ""),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUsername: getEnv("SMTP_USERNAME", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),
		SMTPFromName: getEnv("SMTP_FROM_NAME", "Cocobase"),
		SMTPSecure:   getEnv("SMTP_SECURE", "false") == "true",

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GithubClientID:     getEnv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
		AppleClientID:      getEnv("APPLE_CLIENT_ID", ""),
		AppleTeamID:        getEnv("APPLE_TEAM_ID", ""),
		AppleKeyID:         getEnv("APPLE_KEY_ID", ""),
		ApplePrivateKey:    getEnv("APPLE_PRIVATE_KEY", ""),

		AdminEmail:    getEnv("ADMIN_EMAIL", ""),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),

		RateLimitRequests: getEnvInt("RATE_LIMIT_REQUESTS", 0),
		RateLimitWindow:   getEnvInt("RATE_LIMIT_WINDOW", 60),
	}

	AppConfig = config
	return config
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as int or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		if _, err := fmt.Sscanf(value, "%d", &i); err == nil {
			return i
		}
	}
	return defaultValue
}
