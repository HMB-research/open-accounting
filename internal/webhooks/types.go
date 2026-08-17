package webhooks

import (
	"encoding/json"
	"time"
)

const (
	DeliveryStatusSucceeded = "SUCCEEDED"
	DeliveryStatusFailed    = "FAILED"
)

// Endpoint is a tenant-scoped outbound webhook target.
type Endpoint struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	Events         []string   `json:"events"`
	Secret         string     `json:"-"`
	SecretSet      bool       `json:"secret_set"`
	IsActive       bool       `json:"is_active"`
	LastDeliveryAt *time.Time `json:"last_delivery_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CreateEndpointRequest creates a tenant webhook endpoint.
type CreateEndpointRequest struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Events   []string `json:"events"`
	Secret   string   `json:"secret,omitempty"`
	IsActive *bool    `json:"is_active,omitempty"`
}

// UpdateEndpointRequest updates a tenant webhook endpoint.
type UpdateEndpointRequest struct {
	Name     *string  `json:"name,omitempty"`
	URL      *string  `json:"url,omitempty"`
	Events   []string `json:"events,omitempty"`
	Secret   *string  `json:"secret,omitempty"`
	IsActive *bool    `json:"is_active,omitempty"`
}

// Event is the normalized payload delivered to webhook endpoints.
type Event struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	TenantID  string          `json:"tenant_id"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

// Delivery records one webhook delivery attempt.
type Delivery struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	EndpointID    string          `json:"endpoint_id"`
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	Status        string          `json:"status"`
	StatusCode    int             `json:"status_code,omitempty"`
	AttemptNumber int             `json:"attempt_number"`
	RequestBody   json.RawMessage `json:"request_body,omitempty"`
	ResponseBody  string          `json:"-"`
	Error         string          `json:"error,omitempty"`
	DeliveredAt   time.Time       `json:"delivered_at"`
	CreatedAt     time.Time       `json:"created_at"`
}

// TestDeliveryRequest triggers a deterministic test delivery to one endpoint.
type TestDeliveryRequest struct {
	EventType string          `json:"event_type,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// DeliveryResult is returned after dispatching a webhook event.
type DeliveryResult struct {
	Event      Event      `json:"event"`
	Deliveries []Delivery `json:"deliveries"`
}
