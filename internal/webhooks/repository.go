package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Repository defines outbound webhook persistence.
type Repository interface {
	ListEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]Endpoint, error)
	GetEndpoint(ctx context.Context, tenantID, endpointID string) (*Endpoint, error)
	CreateEndpoint(ctx context.Context, endpoint *Endpoint) error
	UpdateEndpoint(ctx context.Context, endpoint *Endpoint) error
	DeleteEndpoint(ctx context.Context, tenantID, endpointID string) (int64, error)
	CreateDelivery(ctx context.Context, delivery *Delivery) error
	ListDeliveries(ctx context.Context, tenantID, endpointID string, limit int) ([]Delivery, error)
	UpdateEndpointLastDelivery(ctx context.Context, tenantID, endpointID string, deliveredAt time.Time) error
}

// GORMRepository implements Repository with the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates an ORM-backed webhook repository.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// ListEndpoints returns tenant webhook endpoints.
func (r *GORMRepository) ListEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]Endpoint, error) {
	query := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}

	var endpointModels []models.WebhookEndpoint
	if err := query.Order("name ASC, created_at ASC").Find(&endpointModels).Error; err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}

	endpoints := make([]Endpoint, len(endpointModels))
	for i := range endpointModels {
		endpoints[i] = *modelToEndpoint(&endpointModels[i])
	}
	return endpoints, nil
}

// GetEndpoint returns one tenant webhook endpoint.
func (r *GORMRepository) GetEndpoint(ctx context.Context, tenantID, endpointID string) (*Endpoint, error) {
	var endpointModel models.WebhookEndpoint
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, endpointID).
		First(&endpointModel).Error
	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("webhook endpoint not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get webhook endpoint: %w", err)
	}
	return modelToEndpoint(&endpointModel), nil
}

// CreateEndpoint stores a tenant webhook endpoint.
func (r *GORMRepository) CreateEndpoint(ctx context.Context, endpoint *Endpoint) error {
	if err := r.db.WithContext(ctx).Create(endpointToModel(endpoint)).Error; err != nil {
		return fmt.Errorf("create webhook endpoint: %w", err)
	}
	return nil
}

// UpdateEndpoint updates a tenant webhook endpoint.
func (r *GORMRepository) UpdateEndpoint(ctx context.Context, endpoint *Endpoint) error {
	if err := r.db.WithContext(ctx).Model(&models.WebhookEndpoint{}).
		Where("tenant_id = ? AND id = ?", endpoint.TenantID, endpoint.ID).
		Updates(map[string]interface{}{
			"name":       endpoint.Name,
			"url":        endpoint.URL,
			"events":     pq.StringArray(endpoint.Events),
			"secret":     endpoint.Secret,
			"is_active":  endpoint.IsActive,
			"updated_at": endpoint.UpdatedAt,
		}).Error; err != nil {
		return fmt.Errorf("update webhook endpoint: %w", err)
	}
	return nil
}

// DeleteEndpoint deletes a tenant webhook endpoint.
func (r *GORMRepository) DeleteEndpoint(ctx context.Context, tenantID, endpointID string) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, endpointID).
		Delete(&models.WebhookEndpoint{})
	if result.Error != nil {
		return 0, fmt.Errorf("delete webhook endpoint: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// CreateDelivery records a webhook delivery attempt.
func (r *GORMRepository) CreateDelivery(ctx context.Context, delivery *Delivery) error {
	if err := r.db.WithContext(ctx).Create(deliveryToModel(delivery)).Error; err != nil {
		return fmt.Errorf("create webhook delivery: %w", err)
	}
	return nil
}

// ListDeliveries returns recent delivery attempts for one endpoint.
func (r *GORMRepository) ListDeliveries(ctx context.Context, tenantID, endpointID string, limit int) ([]Delivery, error) {
	var deliveryModels []models.WebhookDelivery
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND endpoint_id = ?", tenantID, endpointID).
		Order("delivered_at DESC, created_at DESC").
		Limit(limit).
		Find(&deliveryModels).Error; err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}

	deliveries := make([]Delivery, len(deliveryModels))
	for i := range deliveryModels {
		deliveries[i] = *modelToDelivery(&deliveryModels[i])
	}
	return deliveries, nil
}

// UpdateEndpointLastDelivery stores the most recent attempted delivery time.
func (r *GORMRepository) UpdateEndpointLastDelivery(ctx context.Context, tenantID, endpointID string, deliveredAt time.Time) error {
	if err := r.db.WithContext(ctx).Model(&models.WebhookEndpoint{}).
		Where("tenant_id = ? AND id = ?", tenantID, endpointID).
		Updates(map[string]interface{}{
			"last_delivery_at": deliveredAt,
			"updated_at":       time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("update webhook endpoint last delivery: %w", err)
	}
	return nil
}

func endpointToModel(endpoint *Endpoint) *models.WebhookEndpoint {
	return &models.WebhookEndpoint{
		ID:             endpoint.ID,
		TenantID:       endpoint.TenantID,
		Name:           endpoint.Name,
		URL:            endpoint.URL,
		Events:         pq.StringArray(endpoint.Events),
		Secret:         endpoint.Secret,
		IsActive:       endpoint.IsActive,
		LastDeliveryAt: endpoint.LastDeliveryAt,
		CreatedAt:      endpoint.CreatedAt,
		UpdatedAt:      endpoint.UpdatedAt,
	}
}

func modelToEndpoint(endpoint *models.WebhookEndpoint) *Endpoint {
	return &Endpoint{
		ID:             endpoint.ID,
		TenantID:       endpoint.TenantID,
		Name:           endpoint.Name,
		URL:            endpoint.URL,
		Events:         []string(endpoint.Events),
		Secret:         endpoint.Secret,
		SecretSet:      endpoint.Secret != "",
		IsActive:       endpoint.IsActive,
		LastDeliveryAt: endpoint.LastDeliveryAt,
		CreatedAt:      endpoint.CreatedAt,
		UpdatedAt:      endpoint.UpdatedAt,
	}
}

func deliveryToModel(delivery *Delivery) *models.WebhookDelivery {
	var statusCode *int
	if delivery.StatusCode != 0 {
		statusCode = &delivery.StatusCode
	}
	return &models.WebhookDelivery{
		ID:            delivery.ID,
		TenantID:      delivery.TenantID,
		EndpointID:    delivery.EndpointID,
		EventID:       delivery.EventID,
		EventType:     delivery.EventType,
		Status:        delivery.Status,
		StatusCode:    statusCode,
		AttemptNumber: delivery.AttemptNumber,
		RequestBody:   delivery.RequestBody,
		ResponseBody:  delivery.ResponseBody,
		Error:         delivery.Error,
		DeliveredAt:   delivery.DeliveredAt,
		CreatedAt:     delivery.CreatedAt,
	}
}

func modelToDelivery(delivery *models.WebhookDelivery) *Delivery {
	statusCode := 0
	if delivery.StatusCode != nil {
		statusCode = *delivery.StatusCode
	}
	return &Delivery{
		ID:            delivery.ID,
		TenantID:      delivery.TenantID,
		EndpointID:    delivery.EndpointID,
		EventID:       delivery.EventID,
		EventType:     delivery.EventType,
		Status:        delivery.Status,
		StatusCode:    statusCode,
		AttemptNumber: delivery.AttemptNumber,
		RequestBody:   delivery.RequestBody,
		ResponseBody:  delivery.ResponseBody,
		Error:         delivery.Error,
		DeliveredAt:   delivery.DeliveredAt,
		CreatedAt:     delivery.CreatedAt,
	}
}
