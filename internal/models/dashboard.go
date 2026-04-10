package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DashboardConfig stores instance-level configuration set from the admin dashboard.
// These values override .env settings at runtime.
type DashboardConfig struct {
	ID        string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	Key       string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	IsSecret  bool      `gorm:"column:is_secret;not null;default:false" json:"is_secret"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp with time zone;default:now()" json:"updated_at"`
}

func (DashboardConfig) TableName() string {
	return "dashboard_configs"
}

func (d *DashboardConfig) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}

// AdminUser represents the dashboard administrator account.
// Separate from the platform User model — admin can only access /_/api/*.
type AdminUser struct {
	ID        string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	Email     string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp with time zone;default:now()" json:"created_at"`
}

func (AdminUser) TableName() string {
	return "admin_users"
}

func (a *AdminUser) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
