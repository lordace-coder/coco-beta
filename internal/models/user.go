package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────
// Custom Types (work with both SQLite and PostgreSQL)
// ─────────────────────────────────────────

// StringArray stores string slices as JSON text (compatible with SQLite and PostgreSQL)
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
		return "[]", nil
	}
	b, err := json.Marshal(s)
	return string(b), err
}

// JSONMap stores map[string]interface{} as JSON text
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
		return "{}", nil
	}
	b, err := json.Marshal(m)
	return string(b), err
}

// ─────────────────────────────────────────
// User & Project Models
// ─────────────────────────────────────────

type User struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	Username       *string   `gorm:"index" json:"username,omitempty"`
	Email          string    `gorm:"uniqueIndex;not null" json:"email"`
	Password       string    `gorm:"not null" json:"-"`
	FullName       *string   `json:"full_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	GoogleID       *string   `gorm:"uniqueIndex" json:"google_id,omitempty"`
	ConfirmedEmail bool      `gorm:"default:false" json:"confirmed_email"`
	IsStaff        bool      `gorm:"default:false" json:"is_staff"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

type Project struct {
	ID                 string      `gorm:"primaryKey" json:"id"`
	Name               string      `gorm:"not null" json:"name"`
	UserID             string      `gorm:"not null;index" json:"user_id"`
	APIKey             string      `gorm:"uniqueIndex;not null" json:"api_key"`
	CreatedAt          time.Time   `json:"created_at"`
	AllowedOrigins     StringArray `gorm:"type:text" json:"allowed_origins,omitempty"`
	CallbackURL        *string     `json:"callback_url,omitempty"`
	Configs            JSONMap     `gorm:"type:text" json:"configs"`
	Active             bool        `gorm:"default:true" json:"active"`
	StorageUsedBytes   *int64      `gorm:"column:storage_used_bytes;default:0" json:"storage_used_bytes,omitempty"`
	StorageLastUpdated *time.Time  `gorm:"column:storage_last_updated" json:"storage_last_updated,omitempty"`
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

type ProjectShare struct {
	ProjectID string    `gorm:"primaryKey" json:"project_id"`
	UserID    string    `gorm:"primaryKey" json:"user_id"`
	SharedAt  time.Time `json:"shared_at"`
}

func (ProjectShare) TableName() string { return "project_shares" }

// ─────────────────────────────────────────
// App User Models
// ─────────────────────────────────────────

type AppUser struct {
	ID              string      `gorm:"primaryKey" json:"id"`
	ClientID        string      `gorm:"column:client_id;not null;index" json:"client_id"`
	Email           string      `gorm:"not null" json:"email"`
	Password        string      `gorm:"not null" json:"-"`
	Data            JSONMap     `gorm:"type:text" json:"data"`
	Roles           StringArray `gorm:"type:text" json:"roles"`
	CreatedAt       time.Time   `gorm:"column:created_at" json:"created_at"`
	OAuthID         *string     `gorm:"column:oauth_id" json:"oauth_id,omitempty"`
	OAuthProvider   *string     `gorm:"column:oauth_provider" json:"oauth_provider,omitempty"`
	EmailVerified   bool        `gorm:"column:email_verified;not null;default:false" json:"email_verified"`
	EmailVerifiedAt *time.Time  `gorm:"column:email_verified_at" json:"email_verified_at,omitempty"`
	PhoneNumber     *string     `gorm:"column:phone_number" json:"phone_number,omitempty"`
	PhoneVerified   bool        `gorm:"column:phone_verified;not null;default:false" json:"phone_verified"`
	PhoneVerifiedAt *time.Time  `gorm:"column:phone_verified_at" json:"phone_verified_at,omitempty"`
}

func (a *AppUser) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

func (AppUser) TableName() string { return "app_users" }

type PasswordResetToken struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id;index" json:"user_id"`
	Token     string    `gorm:"column:token;uniqueIndex" json:"token"`
	ExpiresAt time.Time `gorm:"column:expires_at" json:"expires_at"`
	IsUsed    bool      `gorm:"column:is_used;default:false" json:"is_used"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PasswordResetToken) TableName() string { return "app_users_password_reset_tokens" }

type EmailVerificationToken struct {
	ID        string    `gorm:"primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id;not null;index" json:"user_id"`
	ClientID  string    `gorm:"column:client_id;not null;index" json:"client_id"`
	Token     string    `gorm:"column:token;uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"expires_at"`
	IsUsed    bool      `gorm:"column:is_used;not null;default:false" json:"is_used"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (EmailVerificationToken) TableName() string {
	return "app_users_email_verification_tokens"
}

// ─────────────────────────────────────────
// 2FA Models
// ─────────────────────────────────────────

type TwoFactorCode struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    string    `gorm:"column:user_id;not null;index" json:"user_id"`
	ProjectID string    `gorm:"column:project_id;not null;index" json:"project_id"`
	Code      string    `gorm:"column:code;not null" json:"code"`
	IsUsed    bool      `gorm:"column:is_used;default:false" json:"is_used"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (TwoFactorCode) TableName() string { return "two_factor_codes" }

func (t *TwoFactorCode) IsExpired() bool { return time.Now().UTC().After(t.ExpiresAt) }
func (t *TwoFactorCode) IsValid() bool   { return !t.IsUsed && !t.IsExpired() }

type TwoFactorSettings struct {
	ID             int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID         string     `gorm:"column:user_id;not null;uniqueIndex" json:"user_id"`
	ProjectID      string     `gorm:"column:project_id;not null;index" json:"project_id"`
	IsEnabled      bool       `gorm:"column:is_enabled;default:false" json:"is_enabled"`
	BackupEmail    *string    `gorm:"column:backup_email" json:"backup_email,omitempty"`
	LastVerifiedAt *time.Time `gorm:"column:last_verified_at" json:"last_verified_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null" json:"updated_at"`
}

func (TwoFactorSettings) TableName() string { return "two_factor_settings" }
