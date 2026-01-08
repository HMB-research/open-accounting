//go:build gorm

package tax

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM tax repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// EnsureSchema creates tax tables if they don't exist
// Note: Uses raw SQL as GORM AutoMigrate is not suitable for dynamic schema names
func (r *GORMRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.kmd_declarations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
			total_output_vat NUMERIC(28,8) NOT NULL DEFAULT 0,
			total_input_vat NUMERIC(28,8) NOT NULL DEFAULT 0,
			submitted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (tenant_id, year, month)
		);

		CREATE TABLE IF NOT EXISTS %s.kmd_rows (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			declaration_id UUID NOT NULL REFERENCES %s.kmd_declarations(id) ON DELETE CASCADE,
			code VARCHAR(10) NOT NULL,
			description TEXT NOT NULL,
			tax_base NUMERIC(28,8) NOT NULL DEFAULT 0,
			tax_amount NUMERIC(28,8) NOT NULL DEFAULT 0
		);

		CREATE INDEX IF NOT EXISTS idx_kmd_declarations_tenant ON %s.kmd_declarations(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_kmd_rows_declaration ON %s.kmd_rows(declaration_id);
	`, schemaName, schemaName, schemaName, schemaName, schemaName)

	return r.db.WithContext(ctx).Exec(query).Error
}

// QueryVATData queries VAT data from journal entries for a period
// Note: Uses raw SQL for complex aggregation query across multiple tables
func (r *GORMRepository) QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]VATAggregateRow, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var results []struct {
		VATRate   models.Decimal
		IsOutput  bool
		TaxBase   models.Decimal
		TaxAmount models.Decimal
	}

	err := db.Raw(`
		SELECT
			COALESCE(jl.vat_rate, 0) as vat_rate,
			CASE
				WHEN a.account_type IN ('REVENUE', 'INCOME') THEN true
				ELSE false
			END as is_output,
			SUM(jl.credit_amount - jl.debit_amount) as tax_base,
			SUM((jl.credit_amount - jl.debit_amount) * COALESCE(jl.vat_rate, 0) / 100) as tax_amount
		FROM journal_entries je
		JOIN journal_entry_lines jl ON je.id = jl.journal_entry_id
		JOIN accounts a ON jl.account_id = a.id
		WHERE je.tenant_id = ?
			AND je.status = 'POSTED'
			AND je.entry_date >= ?
			AND je.entry_date <= ?
			AND COALESCE(jl.vat_rate, 0) > 0
		GROUP BY jl.vat_rate, a.account_type
	`, tenantID, startDate, endDate).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query VAT data: %w", err)
	}

	rows := make([]VATAggregateRow, len(results))
	for i, r := range results {
		rows[i] = VATAggregateRow{
			VATRate:   r.VATRate.Decimal,
			IsOutput:  r.IsOutput,
			TaxBase:   r.TaxBase.Decimal,
			TaxAmount: r.TaxAmount.Decimal,
		}
	}

	return rows, nil
}

// SaveDeclaration saves a KMD declaration (upsert)
func (r *GORMRepository) SaveDeclaration(ctx context.Context, schemaName string, decl *KMDDeclaration) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// Upsert declaration using raw SQL for ON CONFLICT
		err := tx.Exec(`
			INSERT INTO kmd_declarations (id, tenant_id, year, month, status, total_output_vat, total_input_vat, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (tenant_id, year, month) DO UPDATE SET
				status = EXCLUDED.status,
				total_output_vat = EXCLUDED.total_output_vat,
				total_input_vat = EXCLUDED.total_input_vat,
				updated_at = EXCLUDED.updated_at
		`, decl.ID, decl.TenantID, decl.Year, decl.Month, decl.Status,
			decl.TotalOutputVAT.String(), decl.TotalInputVAT.String(),
			decl.CreatedAt, decl.UpdatedAt).Error
		if err != nil {
			return fmt.Errorf("insert declaration: %w", err)
		}

		// Delete old rows
		if err := tx.Where("declaration_id = ?", decl.ID).Delete(&models.KMDRow{}).Error; err != nil {
			return fmt.Errorf("delete old rows: %w", err)
		}

		// Insert new rows
		for _, row := range decl.Rows {
			rowModel := &models.KMDRow{
				DeclarationID: decl.ID,
				Code:          row.Code,
				Description:   row.Description,
				TaxBase:       models.Decimal{Decimal: row.TaxBase},
				TaxAmount:     models.Decimal{Decimal: row.TaxAmount},
			}
			if err := tx.Create(rowModel).Error; err != nil {
				return fmt.Errorf("insert row: %w", err)
			}
		}

		return nil
	})
}

// GetDeclaration retrieves a KMD declaration for a given period
func (r *GORMRepository) GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*KMDDeclaration, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var declModel models.KMDDeclaration
	err := db.Where("tenant_id = ? AND year = ? AND month = ?", tenantID, year, month).First(&declModel).Error
	if err != nil {
		return nil, fmt.Errorf("get declaration: %w", err)
	}

	// Get rows
	var rowModels []models.KMDRow
	if err := db.Where("declaration_id = ?", declModel.ID).Order("code").Find(&rowModels).Error; err != nil {
		return nil, fmt.Errorf("get rows: %w", err)
	}

	decl := modelToKMDDeclaration(&declModel)
	decl.Rows = make([]KMDRow, len(rowModels))
	for i, rm := range rowModels {
		decl.Rows[i] = *modelToKMDRow(&rm)
	}

	return decl, nil
}

// ListDeclarations lists all KMD declarations for a tenant
func (r *GORMRepository) ListDeclarations(ctx context.Context, schemaName, tenantID string) ([]KMDDeclaration, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var declModels []models.KMDDeclaration
	if err := db.Where("tenant_id = ?", tenantID).
		Order("year DESC, month DESC").
		Find(&declModels).Error; err != nil {
		return nil, fmt.Errorf("list declarations: %w", err)
	}

	declarations := make([]KMDDeclaration, len(declModels))
	for i, dm := range declModels {
		declarations[i] = *modelToKMDDeclaration(&dm)
	}

	return declarations, nil
}

// Conversion helpers

func modelToKMDDeclaration(m *models.KMDDeclaration) *KMDDeclaration {
	return &KMDDeclaration{
		ID:             m.ID,
		TenantID:       m.TenantID,
		Year:           m.Year,
		Month:          m.Month,
		Status:         m.Status,
		TotalOutputVAT: m.TotalOutputVAT.Decimal,
		TotalInputVAT:  m.TotalInputVAT.Decimal,
		SubmittedAt:    m.SubmittedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func modelToKMDRow(m *models.KMDRow) *KMDRow {
	return &KMDRow{
		Code:        m.Code,
		Description: m.Description,
		TaxBase:     m.TaxBase.Decimal,
		TaxAmount:   m.TaxAmount.Decimal,
	}
}
