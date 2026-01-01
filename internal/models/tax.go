package models

import (
	"time"
)

// KMDDeclaration represents an Estonian VAT declaration (GORM model)
type KMDDeclaration struct {
	ID             string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TenantID       string     `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Year           int        `gorm:"not null" json:"year"`
	Month          int        `gorm:"not null" json:"month"`
	Status         string     `gorm:"size:20;not null;default:'DRAFT'" json:"status"`
	TotalOutputVAT Decimal    `gorm:"column:total_output_vat;type:numeric(28,8);not null;default:0" json:"total_output_vat"`
	TotalInputVAT  Decimal    `gorm:"column:total_input_vat;type:numeric(28,8);not null;default:0" json:"total_input_vat"`
	SubmittedAt    *time.Time `gorm:"column:submitted_at" json:"submitted_at,omitempty"`
	CreatedAt      time.Time  `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"not null;default:now()" json:"updated_at"`

	// Relations
	Rows []KMDRow `gorm:"foreignKey:DeclarationID" json:"rows,omitempty"`
}

// TableName returns the table name for GORM
func (KMDDeclaration) TableName() string {
	return "kmd_declarations"
}

// KMDRow represents a single row in the KMD declaration (GORM model)
type KMDRow struct {
	ID            string  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DeclarationID string  `gorm:"column:declaration_id;type:uuid;not null;index" json:"declaration_id"`
	Code          string  `gorm:"size:10;not null" json:"code"`
	Description   string  `gorm:"type:text;not null" json:"description"`
	TaxBase       Decimal `gorm:"column:tax_base;type:numeric(28,8);not null;default:0" json:"tax_base"`
	TaxAmount     Decimal `gorm:"column:tax_amount;type:numeric(28,8);not null;default:0" json:"tax_amount"`

	// Relations
	Declaration *KMDDeclaration `gorm:"foreignKey:DeclarationID" json:"declaration,omitempty"`
}

// TableName returns the table name for GORM
func (KMDRow) TableName() string {
	return "kmd_rows"
}
