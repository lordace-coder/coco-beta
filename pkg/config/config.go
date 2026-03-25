package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                    string
	Environment             string
	DatabaseURL             string
	JWTSecret               string
	APIVersion              string
	BackblazeKeyID          string
	BackblazeKeyName        string
	BackblazeApplicationKey string
	BucketName              string
	BucketEndpoint          string
	PaystackPublicKey       string
	PaystackSecretKey       string
	FrontendURL             string
	RedisURL                string
	MailerAPIKey            string
}

var AppConfig *Config

// LoadConfig loads environment variables and initializes the config
func LoadConfig() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	config := &Config{
		Port:                    getEnv("PORT", "3000"),
		Environment:             getEnv("ENVIRONMENT", "development"),
		DatabaseURL:             getEnv("DATABASE_URL", ""),
		JWTSecret:               getEnv("SECRET", "your-secret-key-change-in-production"),
		APIVersion:              getEnv("API_VERSION", "v1"),
		BackblazeKeyID:          getEnv("BACKBLAZE_KEY_ID", ""),
		BackblazeKeyName:        getEnv("BACKBLAZE_KEY_NAME", ""),
		BackblazeApplicationKey: getEnv("BACKBLAZE_APPLICATION_KEY", ""),
		BucketName:              getEnv("BUCKET_NAME", ""),
		BucketEndpoint:          getEnv("BUCKET_ENDPOINT", ""),
		PaystackPublicKey:       getEnv("PAYSTACK_PUBLIC_KEY", ""),
		PaystackSecretKey:       getEnv("PAYSTACK_SECRET_KEY", ""),
		FrontendURL:             getEnv("FRONTEND_URL", "http://localhost:3000"),
		RedisURL:                getEnv("REDIS_URL", ""),
		MailerAPIKey:            getEnv("MAILER_API_KEY", ""),
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
