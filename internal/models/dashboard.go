package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DashboardConfig stores instance-level configuration set from the admin dashboard.
type DashboardConfig struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	IsSecret  bool      `gorm:"column:is_secret;not null;default:false" json:"is_secret"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (DashboardConfig) TableName() string { return "dashboard_configs" }

func (d *DashboardConfig) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}

// ActivityLog records admin actions on projects for audit purposes.
type ActivityLog struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	ProjectID  string    `gorm:"index" json:"project_id"`
	Action     string    `gorm:"not null" json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Detail     string    `gorm:"type:text" json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ActivityLog) TableName() string { return "activity_logs" }

func (a *ActivityLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// AdminUser represents the dashboard administrator account.
type AdminUser struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

func (AdminUser) TableName() string { return "admin_users" }

func (a *AdminUser) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}
