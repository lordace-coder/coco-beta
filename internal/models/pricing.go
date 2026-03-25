package models

import "time"

// Plan represents a pricing plan
type Plan struct {
	ID                  int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name                string    `gorm:"column:name;type:varchar(50);not null;unique" json:"name"`
	Description         string    `gorm:"column:description;type:varchar(255)" json:"description"`
	Price               float64   `gorm:"column:price;not null;default:0.0" json:"price"`
	Currency            string    `gorm:"column:currency;type:varchar(10);not null;default:'USD'" json:"currency"`
	DurationDays        *int      `gorm:"column:duration_days;default:30" json:"duration_days"`
	MaxRequestsPerMonth *int      `gorm:"column:max_requests_per_month" json:"max_requests_per_month"`
	MaxStorageMB        *int      `gorm:"column:max_storage_mb" json:"max_storage_mb"`
	MaxUsers            *int      `gorm:"column:max_users" json:"max_users"`
	MaxFuncExecuteTime  *int      `gorm:"column:max_func_execute_time" json:"max_func_execute_time"`
	MaxCronJobRuns      *int      `gorm:"column:max_cron_job_runs" json:"max_cron_job_runs"`
	MaxActiveLobbies    *int      `gorm:"column:max_active_lobbies" json:"max_active_lobbies"`
	OrmRateLimit        *int      `gorm:"column:orm_rate_limit" json:"orm_rate_limit"`
	Features            JSONMap   `gorm:"column:features;type:jsonb" json:"features"`
	IsActive            bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	IsFree              bool      `gorm:"column:is_free;not null;default:false" json:"is_free"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at;type:timestamp with time zone" json:"updated_at"`
}

// TableName specifies the table name
func (Plan) TableName() string {
	return "pricing_plans"
}

// ProjectPlan links projects to plans (ProjectSubscription in Python)
type ProjectPlan struct {
	ID              int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ProjectID       string     `gorm:"column:project_id;not null;index" json:"project_id"`
	PlanID          int        `gorm:"column:plan_id;not null" json:"plan_id"`
	StartDate       time.Time  `gorm:"column:start_date;type:timestamp with time zone;default:now()" json:"start_date"`
	EndDate         *time.Time `gorm:"column:end_date;type:timestamp with time zone" json:"end_date"`
	IsActive        bool       `gorm:"column:is_active;not null;default:true" json:"is_active"`
	AutoRenew       bool       `gorm:"column:auto_renew;not null;default:true" json:"auto_renew"`
	GracePeriodDays int        `gorm:"column:grace_period_days;not null;default:3" json:"grace_period_days"`

	// Relationships
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	Plan    *Plan    `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
}

// TableName specifies the table name
func (ProjectPlan) TableName() string {
	return "project_subscriptions"
}

// IsExpired checks if subscription is expired
func (pp *ProjectPlan) IsExpired() bool {
	if pp.EndDate == nil {
		return false
	}
	return time.Now().After(*pp.EndDate)
}

// DaysRemaining returns days until expiration
func (pp *ProjectPlan) DaysRemaining() *int {
	if pp.EndDate == nil {
		return nil
	}
	days := int(time.Until(*pp.EndDate).Hours() / 24)
	return &days
}

// Payment represents a payment transaction
type Payment struct {
	ID                int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Reference         string     `gorm:"column:reference;type:varchar(255);unique;not null;index" json:"reference"`
	Provider          string     `gorm:"column:provider;type:varchar(20);not null;default:'paystack'" json:"provider"` // paystack, stripe, flutterwave
	Amount            float64    `gorm:"column:amount;not null" json:"amount"`
	Currency          string     `gorm:"column:currency;type:varchar(10);not null;default:'NGN'" json:"currency"`
	Status            string     `gorm:"column:status;type:varchar(20);not null;default:'pending';index" json:"status"` // pending, success, failed, cancelled
	ProjectID         string     `gorm:"column:project_id;not null" json:"project_id"`
	UserID            string     `gorm:"column:user_id;type:varchar(100);not null" json:"user_id"`
	PlanID            int        `gorm:"column:plan_id;not null" json:"plan_id"`
	SubscriptionID    *int       `gorm:"column:subscription_id" json:"subscription_id"`
	PaymentMetadata   JSONMap    `gorm:"column:payment_metadata;type:jsonb" json:"payment_metadata"`
	ProviderResponse  JSONMap    `gorm:"column:provider_response;type:jsonb" json:"provider_response"`
	WebhookReceived   bool       `gorm:"column:webhook_received;default:false" json:"webhook_received"`
	WebhookReceivedAt *time.Time `gorm:"column:webhook_received_at;type:timestamp with time zone" json:"webhook_received_at"`
	PaidAt            *time.Time `gorm:"column:paid_at;type:timestamp with time zone" json:"paid_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;type:timestamp with time zone;default:now()" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;type:timestamp with time zone" json:"updated_at"`

	// Relationships
	Project      *Project     `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
	User         *User        `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Plan         *Plan        `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	Subscription *ProjectPlan `gorm:"foreignKey:SubscriptionID" json:"subscription,omitempty"`
}

// TableName specifies the table name
func (Payment) TableName() string {
	return "payments"
}

// ApiUsageCounter tracks API usage per project per month
type ApiUsageCounter struct {
	ID           int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ProjectID    string    `gorm:"column:project_id;not null;index" json:"project_id"`
	Month        string    `gorm:"column:month;type:varchar(7);not null" json:"month"` // Format: "2025-10"
	RequestCount int       `gorm:"column:request_count;not null;default:0" json:"request_count"`
	LastSyncedAt time.Time `gorm:"column:last_synced_at;type:timestamp with time zone" json:"last_synced_at"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamp with time zone;default:now()" json:"created_at"`
}

// TableName specifies the table name
func (ApiUsageCounter) TableName() string {
	return "api_usage_counters"
}
