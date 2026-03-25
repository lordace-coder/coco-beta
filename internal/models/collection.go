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

// Scan implements the sql.Scanner interface for Permissions
func (p *Permissions) Scan(value interface{}) error {
	if value == nil {
		*p = Permissions{
			Create: []string{},
			Read:   []string{},
			Update: []string{},
			Delete: []string{},
		}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		*p = Permissions{
			Create: []string{},
			Read:   []string{},
			Update: []string{},
			Delete: []string{},
		}
		return nil
	}

	// Try to unmarshal as JSON
	err := json.Unmarshal(bytes, p)
	if err != nil {
		// If it fails (e.g., pickled data from Python), set to default empty permissions
		*p = Permissions{
			Create: []string{},
			Read:   []string{},
			Update: []string{},
			Delete: []string{},
		}
		return nil
	}

	return nil
} // Value implements the driver.Valuer interface for Permissions
func (p Permissions) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// Collection represents a collection of documents within a project
type Collection struct {
	ID          string      `gorm:"type:varchar(255);primaryKey" json:"id"`
	Name        string      `gorm:"type:varchar(255);not null" json:"name"`
	CreatedAt   time.Time   `gorm:"type:timestamp;default:now()" json:"created_at"`
	ProjectID   string      `gorm:"type:varchar(255);not null;index" json:"project_id"`
	WebhookURL  *string     `gorm:"type:varchar(255)" json:"webhook_url,omitempty"`
	Permissions Permissions `gorm:"type:jsonb;default:'{\"create\":[],\"read\":[],\"update\":[],\"delete\":[]}'::jsonb" json:"permissions"`
}

// BeforeCreate hook to generate UUID
func (c *Collection) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	// Set default permissions if not provided
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

// TableName specifies the table name for Collection
func (Collection) TableName() string {
	return "collections"
}

// Document represents a document stored in a collection
type Document struct {
	ID           string    `gorm:"type:varchar(255);primaryKey" json:"id"`
	CollectionID string    `gorm:"type:varchar(255);not null;index" json:"collection_id"`
	Data         JSONMap   `gorm:"type:jsonb;not null" json:"data"`
	CreatedAt    time.Time `gorm:"type:timestamp;default:now()" json:"created_at"`
}

// BeforeCreate hook to generate UUID
func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}

// TableName specifies the table name for Document
func (Document) TableName() string {
	return "documents"
}
