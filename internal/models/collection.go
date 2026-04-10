package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Permissions represents the permission structure for collections
type Permissions struct {
	Create []string `json:"create"`
	Read   []string `json:"read"`
	Update []string `json:"update"`
	Delete []string `json:"delete"`
}

func (p *Permissions) Scan(value interface{}) error {
	if value == nil {
		*p = Permissions{Create: []string{}, Read: []string{}, Update: []string{}, Delete: []string{}}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*p = Permissions{Create: []string{}, Read: []string{}, Update: []string{}, Delete: []string{}}
		return nil
	}
	if err := json.Unmarshal(b, p); err != nil {
		*p = Permissions{Create: []string{}, Read: []string{}, Update: []string{}, Delete: []string{}}
	}
	return nil
}

func (p Permissions) Value() (driver.Value, error) {
	b, err := json.Marshal(p)
	return string(b), err
}

// Webhooks holds event-based webhook URLs for a collection.
type Webhooks struct {
	PreSave    string `json:"pre_save,omitempty"`
	PostSave   string `json:"post_save,omitempty"`
	PreDelete  string `json:"pre_delete,omitempty"`
	PostDelete string `json:"post_delete,omitempty"`
}

func (w *Webhooks) Scan(value interface{}) error {
	if value == nil {
		*w = Webhooks{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*w = Webhooks{}
		return nil
	}
	if err := json.Unmarshal(b, w); err != nil {
		*w = Webhooks{}
	}
	return nil
}

func (w Webhooks) Value() (driver.Value, error) {
	b, err := json.Marshal(w)
	return string(b), err
}

// Sentinels holds per-operation security expressions for a collection.
// Each field is a Sentinel expression string evaluated at request time.
// An empty string means no restriction for that operation.
type Sentinels struct {
	List   string `json:"list,omitempty"`
	View   string `json:"view,omitempty"`
	Create string `json:"create,omitempty"`
	Update string `json:"update,omitempty"`
	Delete string `json:"delete,omitempty"`
}

func (s *Sentinels) Scan(value interface{}) error {
	if value == nil {
		*s = Sentinels{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		*s = Sentinels{}
		return nil
	}
	if err := json.Unmarshal(b, s); err != nil {
		*s = Sentinels{}
	}
	return nil
}

func (s Sentinels) Value() (driver.Value, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

// Collection represents a collection of documents within a project
type Collection struct {
	ID          string      `gorm:"primaryKey" json:"id"`
	Name        string      `gorm:"not null" json:"name"`
	CreatedAt   time.Time   `json:"created_at"`
	ProjectID   string      `gorm:"not null;index" json:"project_id"`
	WebhookURL  *string     `json:"webhook_url,omitempty"` // kept for backwards compat
	Webhooks    Webhooks    `gorm:"type:text" json:"webhooks"`
	Permissions Permissions `gorm:"type:text" json:"permissions"`
	Sentinels   Sentinels   `gorm:"type:text" json:"sentinels"`
}

func (c *Collection) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if len(c.Permissions.Create) == 0 && len(c.Permissions.Read) == 0 &&
		len(c.Permissions.Update) == 0 && len(c.Permissions.Delete) == 0 {
		c.Permissions = Permissions{
			Create: []string{},
			Read:   []string{},
			Update: []string{},
			Delete: []string{},
		}
	}
	return nil
}

// HasWebhooks returns true if any webhook URL is configured.
func (c *Collection) HasWebhooks() bool {
	return c.Webhooks.PreSave != "" || c.Webhooks.PostSave != "" ||
		c.Webhooks.PreDelete != "" || c.Webhooks.PostDelete != ""
}

func (Collection) TableName() string { return "collections" }

// Document represents a document stored in a collection
type Document struct {
	ID           string    `gorm:"primaryKey" json:"id"`
	CollectionID string    `gorm:"not null;index" json:"collection_id"`
	Data         JSONMap   `gorm:"type:text;not null" json:"data"`
	CreatedAt    time.Time `json:"created_at"`
}

func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}

func (Document) TableName() string { return "documents" }
