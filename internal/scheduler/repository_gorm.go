//go:build gorm

package scheduler

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Tenant model for GORM queries (public schema)
type tenantModel struct {
	ID         string `gorm:"column:id;primaryKey"`
	SchemaName string `gorm:"column:schema_name"`
	IsActive   bool   `gorm:"column:is_active"`
}

func (tenantModel) TableName() string {
	return "tenants"
}

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// ListActiveTenants returns all active tenants for scheduled job processing
func (r *GORMRepository) ListActiveTenants(ctx context.Context) ([]TenantInfo, error) {
	var tenants []tenantModel
	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Find(&tenants).Error
	if err != nil {
		return nil, fmt.Errorf("list active tenants: %w", err)
	}

	result := make([]TenantInfo, len(tenants))
	for i, t := range tenants {
		result[i] = TenantInfo{
			ID:         t.ID,
			SchemaName: t.SchemaName,
		}
	}

	return result, nil
}
