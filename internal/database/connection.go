package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
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

	// Connect to PostgreSQL with performance optimizations
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),

		// Performance optimizations
		DisableAutomaticPing:   false,
		SkipDefaultTransaction: true, // Don't wrap every operation in a transaction
		PrepareStmt:            true, // Cache prepared statements

		// Connection pooling
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database to configure connection pool
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings for optimal performance
	sqlDB.SetMaxIdleConns(25)                  // Idle connections ready for reuse
	sqlDB.SetMaxOpenConns(200)                 // Maximum open connections
	sqlDB.SetConnMaxLifetime(time.Hour)        // Connection max lifetime
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Idle connection timeout

	log.Println("✅ Database connection established")
	return nil
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
