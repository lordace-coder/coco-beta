package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/patrick/cocobase/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Connect establishes a connection to the PostgreSQL database
func Connect(databaseURL string, debug bool) error {
	var err error

	// Configure GORM logger
	logLevel := logger.Silent
	if debug {
		logLevel = logger.Info
	}

	gormCfg := &gorm.Config{
		Logger:               logger.Default.LogMode(logLevel),
		DisableAutomaticPing: false,
		SkipDefaultTransaction: true,
		PrepareStmt:          false, // caching prepared statements causes unbounded memory growth
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	isSQLite := strings.HasPrefix(databaseURL, "sqlite://") ||
		strings.HasSuffix(databaseURL, ".db") ||
		strings.HasSuffix(databaseURL, ".sqlite") ||
		strings.HasSuffix(databaseURL, ".sqlite3")

	if isSQLite {
		// Strip sqlite:// prefix if present
		filePath := strings.TrimPrefix(databaseURL, "sqlite://")
		DB, err = gorm.Open(sqlite.Open(filePath), gormCfg)
	} else {
		DB, err = gorm.Open(postgres.Open(databaseURL), gormCfg)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database to configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if isSQLite {
		// SQLite doesn't support concurrent writers — use a single connection
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
		log.Println("✅ SQLite database connected")
	} else {
		sqlDB.SetMaxIdleConns(25)
		sqlDB.SetMaxOpenConns(200)
		sqlDB.SetConnMaxLifetime(time.Hour)
		sqlDB.SetConnMaxIdleTime(10 * time.Minute)
		log.Println("✅ PostgreSQL database connected")
	}
	return nil
}

// schemaVersion is the current schema version. Bump this whenever you add new models or columns.
const schemaVersion = 6

type schemaVersionRow struct {
	Version int `gorm:"primaryKey"`
}

func (schemaVersionRow) TableName() string { return "schema_version" }

// Migrate runs auto-migration only when the schema version has changed.
// This avoids slow table-inspection on every cold start.
func Migrate() error {
	// Ensure the version tracker table exists
	if err := DB.AutoMigrate(&schemaVersionRow{}); err != nil {
		return fmt.Errorf("failed to create schema_version table: %w", err)
	}

	var current schemaVersionRow
	result := DB.First(&current)

	// Already at current version — skip
	if result.Error == nil && current.Version >= schemaVersion {
		log.Printf("Schema already at version %d, skipping migration", schemaVersion)
		return nil
	}

	log.Printf("Running schema migration to version %d…", schemaVersion)

	if err := DB.AutoMigrate(
		// Platform models
		&models.User{},
		&models.Project{},
		&models.ProjectShare{},
		// App user models
		&models.AppUser{},
		&models.PasswordResetToken{},
		&models.EmailVerificationToken{},
		// 2FA
		&models.TwoFactorCode{},
		&models.TwoFactorSettings{},
		// Collections & documents
		&models.Collection{},
		&models.Document{},
		// Integrations
		&models.Integration{},
		&models.ProjectIntegration{},
		// Dashboard
		&models.AdminUser{},
		&models.DashboardConfig{},
		&models.ActivityLog{},
		// Cloud functions
		&models.Function{},
	); err != nil {
		return fmt.Errorf("auto-migrate failed: %w", err)
	}

	// Upsert version record
	DB.Delete(&schemaVersionRow{}, "version > 0")
	DB.Create(&schemaVersionRow{Version: schemaVersion})

	log.Printf("✅ Schema migration to version %d complete", schemaVersion)
	return nil
}

// ClearCollectionCache is called by the system cron to evict stale collection name→ID cache.
// The actual cache lives in the handlers package; this is a no-op hook that packages
// can override by registering a callback.
var collectionCacheClearer func()

func RegisterCollectionCacheClearer(fn func()) {
	collectionCacheClearer = fn
}

func ClearCollectionCache() {
	if collectionCacheClearer != nil {
		collectionCacheClearer()
	}
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}
