package models

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// WebhookEndpoint represents a tenant-scoped outbound webhook endpoint.
type WebhookEndpoint struct {
	ID             string         `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID       string         `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	Name           string         `gorm:"size:255;not null" json:"name"`
	URL            string         `gorm:"column:url;type:text;not null" json:"url"`
	Events         pq.StringArray `gorm:"type:text[];not null" json:"events"`
	Secret         string         `gorm:"type:text;not null;default:''" json:"-"`
	IsActive       bool           `gorm:"column:is_active;not null;default:true" json:"is_active"`
	LastDeliveryAt *time.Time     `gorm:"column:last_delivery_at" json:"last_delivery_at,omitempty"`
	CreatedAt      time.Time      `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for GORM.
func (WebhookEndpoint) TableName() string {
	return "webhook_endpoints"
}

// WebhookDelivery represents one outbound webhook delivery attempt.
type WebhookDelivery struct {
	ID            string          `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()" json:"id"`
	TenantID      string          `gorm:"column:tenant_id;type:uuid;not null;index" json:"tenant_id"`
	EndpointID    string          `gorm:"column:endpoint_id;type:uuid;not null;index" json:"endpoint_id"`
	EventID       string          `gorm:"column:event_id;type:text;not null" json:"event_id"`
	EventType     string          `gorm:"column:event_type;size:128;not null" json:"event_type"`
	Status        string          `gorm:"size:16;not null" json:"status"`
	StatusCode    *int            `gorm:"column:status_code" json:"status_code,omitempty"`
	AttemptNumber int             `gorm:"column:attempt_number;not null;default:1" json:"attempt_number"`
	RequestBody   json.RawMessage `gorm:"column:request_body;type:jsonb;not null;default:'{}'" json:"request_body,omitempty"`
	ResponseBody  string          `gorm:"column:response_body;type:text;not null;default:''" json:"-"`
	Error         string          `gorm:"column:error;type:text;not null;default:''" json:"error,omitempty"`
	DeliveredAt   time.Time       `gorm:"column:delivered_at;not null;default:now()" json:"delivered_at"`
	CreatedAt     time.Time       `gorm:"not null;default:now()" json:"created_at"`
}

// TableName returns the table name for GORM.
func (WebhookDelivery) TableName() string {
	return "webhook_deliveries"
}
