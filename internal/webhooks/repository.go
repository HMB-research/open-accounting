package webhooks

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

// PostgresRepository implements Repository with PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a webhook PostgreSQL repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ListEndpoints returns tenant webhook endpoints.
func (r *PostgresRepository) ListEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]Endpoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, name, url, events, COALESCE(secret, ''), is_active, last_delivery_at, created_at, updated_at
		FROM webhook_endpoints
		WHERE tenant_id = $1 AND ($2 = false OR is_active = true)
		ORDER BY name ASC, created_at ASC
	`, tenantID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []Endpoint
	for rows.Next() {
		endpoint, err := scanEndpoint(rows)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, *endpoint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook endpoints: %w", err)
	}
	return endpoints, nil
}

// GetEndpoint returns one tenant webhook endpoint.
func (r *PostgresRepository) GetEndpoint(ctx context.Context, tenantID, endpointID string) (*Endpoint, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, url, events, COALESCE(secret, ''), is_active, last_delivery_at, created_at, updated_at
		FROM webhook_endpoints
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, endpointID)
	endpoint, err := scanEndpoint(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("webhook endpoint not found")
	}
	if err != nil {
		return nil, err
	}
	return endpoint, nil
}

// CreateEndpoint stores a tenant webhook endpoint.
func (r *PostgresRepository) CreateEndpoint(ctx context.Context, endpoint *Endpoint) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_endpoints (id, tenant_id, name, url, events, secret, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, endpoint.ID, endpoint.TenantID, endpoint.Name, endpoint.URL, endpoint.Events, endpoint.Secret, endpoint.IsActive, endpoint.CreatedAt, endpoint.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create webhook endpoint: %w", err)
	}
	return nil
}

// UpdateEndpoint updates a tenant webhook endpoint.
func (r *PostgresRepository) UpdateEndpoint(ctx context.Context, endpoint *Endpoint) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE webhook_endpoints
		SET name = $3,
		    url = $4,
		    events = $5,
		    secret = $6,
		    is_active = $7,
		    updated_at = $8
		WHERE tenant_id = $1 AND id = $2
	`, endpoint.TenantID, endpoint.ID, endpoint.Name, endpoint.URL, endpoint.Events, endpoint.Secret, endpoint.IsActive, endpoint.UpdatedAt)
	if err != nil {
		return fmt.Errorf("update webhook endpoint: %w", err)
	}
	return nil
}

// DeleteEndpoint deletes a tenant webhook endpoint.
func (r *PostgresRepository) DeleteEndpoint(ctx context.Context, tenantID, endpointID string) (int64, error) {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM webhook_endpoints
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, endpointID)
	if err != nil {
		return 0, fmt.Errorf("delete webhook endpoint: %w", err)
	}
	return result.RowsAffected(), nil
}

// CreateDelivery records a webhook delivery attempt.
func (r *PostgresRepository) CreateDelivery(ctx context.Context, delivery *Delivery) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO webhook_deliveries (
			id, tenant_id, endpoint_id, event_id, event_type, status, status_code,
			attempt_number, request_body, response_body, error, delivered_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, 0), $8, $9, $10, $11, $12, $13)
	`, delivery.ID, delivery.TenantID, delivery.EndpointID, delivery.EventID, delivery.EventType, delivery.Status,
		delivery.StatusCode, delivery.AttemptNumber, delivery.RequestBody, delivery.ResponseBody, delivery.Error,
		delivery.DeliveredAt, delivery.CreatedAt)
	if err != nil {
		return fmt.Errorf("create webhook delivery: %w", err)
	}
	return nil
}

// ListDeliveries returns recent delivery attempts for one endpoint.
func (r *PostgresRepository) ListDeliveries(ctx context.Context, tenantID, endpointID string, limit int) ([]Delivery, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, endpoint_id, event_id, event_type, status, COALESCE(status_code, 0),
		       attempt_number, request_body, COALESCE(response_body, ''), COALESCE(error, ''), delivered_at, created_at
		FROM webhook_deliveries
		WHERE tenant_id = $1 AND endpoint_id = $2
		ORDER BY delivered_at DESC, created_at DESC
		LIMIT $3
	`, tenantID, endpointID, limit)
	if err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	defer rows.Close()

	var deliveries []Delivery
	for rows.Next() {
		var delivery Delivery
		if err := rows.Scan(
			&delivery.ID,
			&delivery.TenantID,
			&delivery.EndpointID,
			&delivery.EventID,
			&delivery.EventType,
			&delivery.Status,
			&delivery.StatusCode,
			&delivery.AttemptNumber,
			&delivery.RequestBody,
			&delivery.ResponseBody,
			&delivery.Error,
			&delivery.DeliveredAt,
			&delivery.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan webhook delivery: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook deliveries: %w", err)
	}
	return deliveries, nil
}

// UpdateEndpointLastDelivery stores the most recent attempted delivery time.
func (r *PostgresRepository) UpdateEndpointLastDelivery(ctx context.Context, tenantID, endpointID string, deliveredAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE webhook_endpoints
		SET last_delivery_at = $3, updated_at = now()
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, endpointID, deliveredAt)
	if err != nil {
		return fmt.Errorf("update webhook endpoint last delivery: %w", err)
	}
	return nil
}

type endpointScanner interface {
	Scan(dest ...any) error
}

func scanEndpoint(row endpointScanner) (*Endpoint, error) {
	var endpoint Endpoint
	if err := row.Scan(
		&endpoint.ID,
		&endpoint.TenantID,
		&endpoint.Name,
		&endpoint.URL,
		&endpoint.Events,
		&endpoint.Secret,
		&endpoint.IsActive,
		&endpoint.LastDeliveryAt,
		&endpoint.CreatedAt,
		&endpoint.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan webhook endpoint: %w", err)
	}
	endpoint.SecretSet = endpoint.Secret != ""
	return &endpoint, nil
}
