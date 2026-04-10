package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Integration struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"not null;uniqueIndex" json:"name"`
	DisplayName  string    `gorm:"not null" json:"display_name"`
	Description  *string   `gorm:"type:text" json:"description,omitempty"`
	IconURL      *string   `json:"icon_url,omitempty"`
	ModuleName   *string   `json:"module_name,omitempty"`
	ModuleCode   *string   `gorm:"type:text" json:"module_code,omitempty"`
	ConfigSchema JSONMap   `gorm:"type:text" json:"config_schema,omitempty"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

func (i *Integration) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

func (Integration) TableName() string { return "integrations" }

type ProjectIntegration struct {
	ID            string                 `gorm:"primaryKey" json:"id"`
	ProjectID     string                 `gorm:"not null;index" json:"project_id"`
	IntegrationID string                 `gorm:"not null;index" json:"integration_id"`
	Config        JSONMap                `gorm:"type:text" json:"config"`
	IsEnabled     bool                   `gorm:"default:true" json:"is_enabled"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`

	Project     *Project     `gorm:"foreignKey:ProjectID;references:ID" json:"project,omitempty"`
	Integration *Integration `gorm:"foreignKey:IntegrationID;references:ID" json:"integration,omitempty"`
}

func (pi *ProjectIntegration) BeforeCreate(tx *gorm.DB) error {
	if pi.ID == "" {
		pi.ID = uuid.New().String()
	}
	return nil
}

func (ProjectIntegration) TableName() string { return "project_integrations" }
