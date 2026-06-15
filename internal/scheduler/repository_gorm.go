package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// Tenant model for GORM queries (public schema)
type tenantModel struct {
	ID         string `gorm:"column:id;primaryKey"`
	SchemaName string `gorm:"column:schema_name"`
	Name       string `gorm:"column:name"`
	Settings   []byte `gorm:"column:settings;type:jsonb"`
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
			ID:             t.ID,
			SchemaName:     t.SchemaName,
			CompanyName:    t.Name,
			Email:          stringValueFromSettings(t.Settings, "email"),
			PeriodLockDate: stringValueFromSettings(t.Settings, "period_lock_date"),
		}
	}

	return result, nil
}

func stringValueFromSettings(settings []byte, key string) string {
	if len(settings) == 0 {
		return ""
	}

	var values map[string]any
	if err := json.Unmarshal(settings, &values); err != nil {
		return ""
	}

	value, _ := values[key].(string)
	return value
}
