package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration represents available integrations in the system
// These are predefined integrations that users can enable
type Integration struct {
	ID           string                 `gorm:"type:varchar(255);primaryKey" json:"id"`
	Name         string                 `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	DisplayName  string                 `gorm:"type:varchar(255);not null" json:"display_name"`
	Description  *string                `gorm:"type:text" json:"description,omitempty"`
	IconURL      *string                `gorm:"type:varchar(255)" json:"icon_url,omitempty"`
	ModuleName   *string                `gorm:"type:varchar(255)" json:"module_name,omitempty"` // e.g., "openai_integration"
	ModuleCode   *string                `gorm:"type:text" json:"module_code,omitempty"`         // Actual Python code
	ConfigSchema map[string]interface{} `gorm:"type:jsonb" json:"config_schema,omitempty"`      // e.g., {"api_key": "string", "region": "string"}
	IsActive     bool                   `gorm:"type:boolean;default:true" json:"is_active"`
	CreatedAt    time.Time              `gorm:"type:timestamp;default:now()" json:"created_at"`
}

// BeforeCreate hook to generate UUID
func (i *Integration) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name for Integration
func (Integration) TableName() string {
	return "integrations"
}

// ProjectIntegration represents integrations enabled for a specific project
// User turns on integration for project and optionally sets config
type ProjectIntegration struct {
	ID            string                 `gorm:"type:varchar(255);primaryKey" json:"id"`
	ProjectID     string                 `gorm:"type:varchar(255);not null;index;constraint:OnDelete:CASCADE" json:"project_id"`
	IntegrationID string                 `gorm:"type:varchar(255);not null;index;constraint:OnDelete:CASCADE" json:"integration_id"`
	Config        map[string]interface{} `gorm:"type:jsonb;default:'{}'" json:"config"`
	IsEnabled     bool                   `gorm:"type:boolean;default:true" json:"is_enabled"`
	CreatedAt     time.Time              `gorm:"type:timestamp;default:now()" json:"created_at"`
	UpdatedAt     time.Time              `gorm:"type:timestamp;default:now()" json:"updated_at"`

	// Relationships
	Project     *Project     `gorm:"foreignKey:ProjectID;references:ID" json:"project,omitempty"`
	Integration *Integration `gorm:"foreignKey:IntegrationID;references:ID" json:"integration,omitempty"`
}

// BeforeCreate hook to generate UUID
func (pi *ProjectIntegration) BeforeCreate(tx *gorm.DB) error {
	if pi.ID == "" {
		pi.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name for ProjectIntegration
func (ProjectIntegration) TableName() string {
	return "project_integrations"
}
