package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TriggerType defines what fires a function.
type TriggerType string

const (
	TriggerHTTP TriggerType = "http"
	TriggerHook TriggerType = "hook"
	TriggerCron TriggerType = "cron"
)

// HookEvent is which collection lifecycle event fires a hook function.
type HookEvent string

const (
	HookBeforeCreate HookEvent = "beforeCreate"
	HookAfterCreate  HookEvent = "afterCreate"
	HookBeforeUpdate HookEvent = "beforeUpdate"
	HookAfterUpdate  HookEvent = "afterUpdate"
	HookBeforeDelete HookEvent = "beforeDelete"
	HookAfterDelete  HookEvent = "afterDelete"
)

// TriggerConfig holds trigger-specific settings stored as JSON.
type TriggerConfig struct {
	// HTTP
	Method string `json:"method,omitempty"` // GET, POST, ANY, etc.
	Path   string `json:"path,omitempty"`   // e.g. /hello

	// Hook
	Event      string `json:"event,omitempty"`      // HookEvent value
	Collection string `json:"collection,omitempty"` // collection name/id, "" = all

	// Cron
	Schedule string `json:"schedule,omitempty"` // cron expression e.g. "*/5 * * * *"
}

func (t TriggerConfig) Value() (driver.Value, error) {
	b, err := json.Marshal(t)
	return string(b), err
}
func (t *TriggerConfig) Scan(src interface{}) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("unsupported type: %T", src)
	}
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), t)
}

// FunctionLog is one execution record.
type FunctionLog struct {
	RunAt    time.Time `json:"run_at"`
	Duration int64     `json:"duration_ms"`
	Success  bool      `json:"success"`
	Output   string    `json:"output"`
	Error    string    `json:"error,omitempty"`
}

type FunctionLogs []FunctionLog

func (l FunctionLogs) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal(l)
	return string(b), err
}
func (l *FunctionLogs) Scan(src interface{}) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("unsupported type: %T", src)
	}
	if s == "" {
		return nil
	}
	return json.Unmarshal([]byte(s), l)
}

// Function is a user-defined JS function scoped to a project.
type Function struct {
	ID            string        `gorm:"primaryKey;type:text" json:"id"`
	ProjectID     string        `gorm:"type:text;not null;index" json:"project_id"`
	Name          string        `gorm:"type:text;not null" json:"name"`
	Code          string        `gorm:"type:text" json:"code"`
	TriggerType   TriggerType   `gorm:"type:text;not null" json:"trigger_type"`
	TriggerConfig TriggerConfig `gorm:"type:text" json:"trigger_config"`
	Enabled       bool          `gorm:"default:true" json:"enabled"`
	Timeout       int           `gorm:"default:10" json:"timeout"` // seconds
	// Last 20 execution logs kept inline for quick display
	Logs      FunctionLogs `gorm:"type:text" json:"logs,omitempty"`
	LastRunAt *time.Time   `json:"last_run_at,omitempty"`
	LastError string       `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

func (f *Function) BeforeCreate(tx *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
	return nil
}

func (Function) TableName() string { return "functions" }
