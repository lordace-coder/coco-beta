package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────
// Custom Types
// ─────────────────────────────────────────

// StringArray handles PostgreSQL JSONB string arrays
type StringArray []string

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = []string{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*s = []string{}
		return nil
	}
	if err := json.Unmarshal(b, s); err != nil {
		*s = []string{}
		return nil
	}
	return nil
}

func (s StringArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(s)
}

// JSONMap handles PostgreSQL JSONB map fields
type JSONMap map[string]interface{}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(map[string]interface{})
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*m = make(map[string]interface{})
		return nil
	}
	if err := json.Unmarshal(b, m); err != nil {
		*m = make(map[string]interface{})
		return nil
	}
	return nil
}

func (m JSONMap) Value() (driver.Value, error) {
	if len(m) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(m)
}

// ─────────────────────────────────────────
// User & Project Models
// ─────────────────────────────────────────

// User represents the main platform user (project owner)
type User struct {
	ID             string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	Username       *string   `gorm:"type:varchar(255);index" json:"username,omitempty"`
	Email          string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Password       string    `gorm:"type:varchar(255);not null" json:"-"`
	FullName       *string   `gorm:"type:varchar(255)" json:"full_name,omitempty"`
	CreatedAt      time.Time `gorm:"type:timestamp;default:now()" json:"created_at"`
	GoogleID       *string   `gorm:"type:varchar(255);uniqueIndex" json:"google_id,omitempty"`
	ConfirmedEmail bool      `gorm:"type:boolean;default:false" json:"confirmed_email"`
	IsStaff        bool      `gorm:"type:boolean;default:false" json:"is_staff"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// Project represents a project/application created by a user
type Project struct {
	ID                 string      `gorm:"type:varchar(255);primaryKey" json:"id"`
	Name               string      `gorm:"type:varchar(255);not null" json:"name"`
	UserID             string      `gorm:"type:varchar(255);not null;index" json:"user_id"`
	APIKey             string      `gorm:"type:varchar(255);uniqueIndex;not null" json:"api_key"`
	CreatedAt          time.Time   `gorm:"type:timestamp;default:now()" json:"created_at"`
	AllowedOrigins     StringArray `gorm:"type:jsonb" json:"allowed_origins,omitempty"`
	CallbackURL        *string     `gorm:"type:varchar(255)" json:"callback_url,omitempty"`
	Configs            JSONMap     `gorm:"type:jsonb;default:'{}'" json:"configs"`
	Active             bool        `gorm:"type:boolean;default:true" json:"active"`
	StorageUsedBytes   *int64      `gorm:"column:storage_used_bytes;default:0" json:"storage_used_bytes,omitempty"`
	StorageLastUpdated *time.Time  `gorm:"column:storage_last_updated;type:timestamp" json:"storage_last_updated,omitempty"`
}

func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.APIKey == "" {
		p.APIKey = "coco_" + uuid.New().String()
	}
	return nil
}

// ProjectShare represents the many-to-many relationship for shared projects
type ProjectShare struct {
	ProjectID string    `gorm:"type:varchar(255);primaryKey" json:"project_id"`
	UserID    string    `gorm:"type:varchar(255);primaryKey" json:"user_id"`
	SharedAt  time.Time `gorm:"type:timestamp;default:now()" json:"shared_at"`
}

func (ProjectShare) TableName() string {
	return "project_shares"
}

// ─────────────────────────────────────────
// App User Models
// ─────────────────────────────────────────

// AppUser represents end-users of a client application
type AppUser struct {
	ID              string      `gorm:"type:varchar(255);primaryKey" json:"id"`
	ClientID        string      `gorm:"column:client_id;type:varchar(255);not null;index" json:"client_id"`
	Email           string      `gorm:"type:varchar(255);not null" json:"email"`
	Password        string      `gorm:"type:varchar(255);not null" json:"-"`
	Data            JSONMap     `gorm:"type:jsonb;default:'{}'" json:"data"`
	Roles           StringArray `gorm:"type:jsonb;default:'[]'" json:"roles"`
	CreatedAt       time.Time   `gorm:"column:created_at;type:timestamp;default:now()" json:"created_at"`
	OAuthID         *string     `gorm:"column:oauth_id;type:varchar(255)" json:"oauth_id,omitempty"`
	OAuthProvider   *string     `gorm:"column:oauth_provider;type:varchar(255)" json:"oauth_provider,omitempty"`
	EmailVerified   bool        `gorm:"column:email_verified;type:boolean;not null;default:false" json:"email_verified"`
	EmailVerifiedAt *time.Time  `gorm:"column:email_verified_at;type:timestamp" json:"email_verified_at,omitempty"`
	PhoneNumber     *string     `gorm:"column:phone_number;type:varchar(20)" json:"phone_number,omitempty"`
	PhoneVerified   bool        `gorm:"column:phone_verified;type:boolean;not null;default:false" json:"phone_verified"`
	PhoneVerifiedAt *time.Time  `gorm:"column:phone_verified_at;type:timestamp" json:"phone_verified_at,omitempty"`
}

func (a *AppUser) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

func (AppUser) TableName() string {
	return "app_users"
}

// PasswordResetToken for app user password resets
type PasswordResetToken struct {
	ID        string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id;type:varchar(255);index" json:"user_id"`
	Token     string    `gorm:"column:token;type:varchar(255);uniqueIndex" json:"token"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamp" json:"expires_at"`
	IsUsed    bool      `gorm:"column:is_used;type:boolean;default:false" json:"is_used"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:now()" json:"created_at"`
}

func (PasswordResetToken) TableName() string {
	return "app_users_password_reset_tokens"
}

// EmailVerificationToken for app user email verification
type EmailVerificationToken struct {
	ID        string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id;type:varchar(255);not null;index" json:"user_id"`
	ClientID  string    `gorm:"column:client_id;type:varchar(255);not null;index" json:"client_id"`
	Token     string    `gorm:"column:token;type:varchar(255);uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamp;not null" json:"expires_at"`
	IsUsed    bool      `gorm:"column:is_used;type:boolean;not null;default:false" json:"is_used"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:now()" json:"created_at"`
}

func (EmailVerificationToken) TableName() string {
	return "app_users_email_verification_tokens"
}

// ─────────────────────────────────────────
// 2FA Models
// ─────────────────────────────────────────

// TwoFactorCode stores OTP codes for 2FA verification
type TwoFactorCode struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"column:user_id;type:varchar(255);not null;index" json:"user_id"`
	ProjectID string    `gorm:"column:project_id;type:varchar(255);not null;index" json:"project_id"`
	Code      string    `gorm:"column:code;type:varchar(10);not null" json:"code"`
	IsUsed    bool      `gorm:"column:is_used;type:boolean;default:false" json:"is_used"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamp;not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;not null;default:now()" json:"created_at"`
}

func (TwoFactorCode) TableName() string {
	return "two_factor_codes"
}

func (t *TwoFactorCode) IsExpired() bool {
	return time.Now().UTC().After(t.ExpiresAt)
}

func (t *TwoFactorCode) IsValid() bool {
	return !t.IsUsed && !t.IsExpired()
}

// TwoFactorSettings stores 2FA configuration per user per project
type TwoFactorSettings struct {
	ID             int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID         string     `gorm:"column:user_id;type:varchar(255);not null;uniqueIndex" json:"user_id"`
	ProjectID      string     `gorm:"column:project_id;type:varchar(255);not null;index" json:"project_id"`
	IsEnabled      bool       `gorm:"column:is_enabled;type:boolean;default:false" json:"is_enabled"`
	BackupEmail    *string    `gorm:"column:backup_email;type:varchar(255)" json:"backup_email,omitempty"`
	LastVerifiedAt *time.Time `gorm:"column:last_verified_at;type:timestamp" json:"last_verified_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:timestamp;not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;type:timestamp;not null;default:now()" json:"updated_at"`
}

func (TwoFactorSettings) TableName() string {
	return "two_factor_settings"
}
