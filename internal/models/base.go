package models

import (
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
)

// Re-export database types for convenience
type Decimal = database.Decimal
type JSONB = database.JSONB
type JSONBRaw = database.JSONBRaw

// Convenience functions
var (
	NewDecimal           = database.NewDecimal
	NewDecimalFromFloat  = database.NewDecimalFromFloat
	NewDecimalFromString = database.NewDecimalFromString
	DecimalZero          = database.DecimalZero
)

// TenantModel is the base model for all tenant-scoped entities
// Embed this in models that belong to a specific tenant
type TenantModel struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID  string    `gorm:"type:uuid;not null;index" json:"tenant_id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// PublicModel is the base model for entities in the public schema
// Embed this in models that are shared across tenants
type PublicModel struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}
