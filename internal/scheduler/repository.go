package scheduler

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantInfo contains minimal tenant information for scheduling
type TenantInfo struct {
	ID          string
	SchemaName  string
	CompanyName string // Needed for email templates
}

// Repository defines the interface for scheduler data access
type Repository interface {
	ListActiveTenants(ctx context.Context) ([]TenantInfo, error)
}

// PostgresRepository implements Repository for PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// ListActiveTenants returns all active tenants for scheduled job processing
func (r *PostgresRepository) ListActiveTenants(ctx context.Context) ([]TenantInfo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, schema_name, COALESCE(name, '') FROM tenants WHERE is_active = true
	`)
	if err != nil {
		return nil, fmt.Errorf("list active tenants: %w", err)
	}
	defer rows.Close()

	var tenants []TenantInfo
	for rows.Next() {
		var t TenantInfo
		if err := rows.Scan(&t.ID, &t.SchemaName, &t.CompanyName); err != nil {
			return nil, fmt.Errorf("scan tenant: %w", err)
		}
		tenants = append(tenants, t)
	}

	return tenants, nil
}
