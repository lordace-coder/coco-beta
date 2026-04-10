package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

const envExample = `# ── Required ──────────────────────────────────────────────────────────────────
DATABASE_URL=./cocobase.db        # SQLite file, or a PostgreSQL URL
SECRET=change-me-to-a-long-random-string

# ── Server ────────────────────────────────────────────────────────────────────
PORT=3000
ENVIRONMENT=production            # development | production

# ── SMTP (optional — needed for email verification / password reset) ───────────
# SMTP_HOST=smtp.gmail.com
# SMTP_PORT=587
# SMTP_USERNAME=you@example.com
# SMTP_PASSWORD=your-app-password
# SMTP_FROM=no-reply@yourapp.com
# SMTP_FROM_NAME=My App
# SMTP_SECURE=false               # true for SSL/port 465

# ── File storage — Backblaze B2 / S3-compatible (optional) ───────────────────
# BACKBLAZE_KEY_ID=
# BACKBLAZE_APPLICATION_KEY=
# BACKBLAZE_KEY_NAME=
# BUCKET_NAME=
# BUCKET_ENDPOINT=https://s3.us-west-004.backblazeb2.com

# ── Redis (optional — for real-time features) ─────────────────────────────────
# REDIS_URL=redis://localhost:6379

# ── OAuth — social login (optional) ──────────────────────────────────────────
# GOOGLE_CLIENT_ID=
# GOOGLE_CLIENT_SECRET=
# GITHUB_CLIENT_ID=
# GITHUB_CLIENT_SECRET=
# APPLE_CLIENT_ID=
# APPLE_TEAM_ID=
# APPLE_KEY_ID=
# APPLE_PRIVATE_KEY=

# ── Rate limiting (optional, 0 = unlimited) ───────────────────────────────────
# RATE_LIMIT_REQUESTS=100
# RATE_LIMIT_WINDOW=60
`

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
		// Write a .env.example if one doesn't exist yet, so the user knows what to configure
		if _, err := os.Stat(".env.example"); os.IsNotExist(err) {
			if err := os.WriteFile(".env.example", []byte(envExample), 0644); err == nil {
				log.Println("📄 Created .env.example — copy it to .env and fill in your values")
			}
		}
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
