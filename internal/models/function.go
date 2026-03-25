package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RuntimeEnum represents supported runtimes for cloud functions
type RuntimeEnum string

const (
	RuntimePython RuntimeEnum = "python3.10"
	RuntimeNode   RuntimeEnum = "node18"
	RuntimeGo     RuntimeEnum = "go1.20"
)

// CloudFunction represents a serverless function in a project
type CloudFunction struct {
	ID          string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	ProjectID   string    `gorm:"type:varchar(255);not null;index" json:"project_id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	Runtime     string    `gorm:"type:varchar(50);not null" json:"runtime"`
	Code        string    `gorm:"type:text;not null" json:"code"`
	CreatedAt   time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamp with time zone;default:now()" json:"updated_at"`
}

// BeforeCreate hook to generate UUID
func (cf *CloudFunction) BeforeCreate(tx *gorm.DB) error {
	if cf.ID == "" {
		cf.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name for CloudFunction
func (CloudFunction) TableName() string {
	return "cloud_functions"
}

// FunctionExecution represents an execution log of a cloud function
type FunctionExecution struct {
	ID          string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	FunctionID  string    `gorm:"type:varchar(255);not null;index" json:"function_id"`
	Status      string    `gorm:"type:varchar(50);not null" json:"status"` // success, failed, timeout
	Logs        *string   `gorm:"type:text" json:"logs,omitempty"`
	DurationMs  *int      `gorm:"type:integer" json:"duration_ms,omitempty"`
	TriggeredBy *string   `gorm:"type:varchar(255)" json:"triggered_by,omitempty"` // manual, http, schedule, db-event
	CreatedAt   time.Time `gorm:"type:timestamp with time zone;default:now()" json:"created_at"`
}

// BeforeCreate hook to generate UUID
func (fe *FunctionExecution) BeforeCreate(tx *gorm.DB) error {
	if fe.ID == "" {
		fe.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name for FunctionExecution
func (FunctionExecution) TableName() string {
	return "function_executions"
}
