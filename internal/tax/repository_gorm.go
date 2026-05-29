//go:build gorm

package tax

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
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

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

// EnsureSchema creates tax tables if they don't exist
// Note: Uses raw SQL as GORM AutoMigrate is not suitable for dynamic schema names
func (r *GORMRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	quotedSchema, err := database.QuoteIdentifier(schemaName)
	if err != nil {
		return err
	}

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
	`, quotedSchema, quotedSchema, quotedSchema, quotedSchema, quotedSchema)

	return r.db.WithContext(ctx).Exec(query).Error
}

// QueryVATData queries VAT data from journal entries for a period
// Note: Uses raw SQL for complex aggregation query across multiple tables
func (r *GORMRepository) QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]VATAggregateRow, error) {
	entriesTable, err := database.QualifiedTable(schemaName, "journal_entries")
	if err != nil {
		return nil, err
	}
	linesTable, err := database.QualifiedTable(schemaName, "journal_entry_lines")
	if err != nil {
		return nil, err
	}
	accountsTable, err := database.QualifiedTable(schemaName, "accounts")
	if err != nil {
		return nil, err
	}

	var results []struct {
		VATRate   models.Decimal
		IsOutput  bool
		TaxBase   models.Decimal
		TaxAmount models.Decimal
	}

	err = r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT
			COALESCE(jl.vat_rate, 0) as vat_rate,
			CASE
				WHEN a.account_type IN ('REVENUE', 'INCOME') THEN true
				ELSE false
			END as is_output,
			SUM(jl.credit_amount - jl.debit_amount) as tax_base,
			SUM((jl.credit_amount - jl.debit_amount) * COALESCE(jl.vat_rate, 0) / 100) as tax_amount
		FROM %s je
		JOIN %s jl ON je.id = jl.journal_entry_id
		JOIN %s a ON jl.account_id = a.id
		WHERE je.tenant_id = ?
			AND je.status = 'POSTED'
			AND je.entry_date >= ?
			AND je.entry_date <= ?
			AND COALESCE(jl.vat_rate, 0) > 0
		GROUP BY jl.vat_rate, a.account_type
	`, entriesTable, linesTable, accountsTable), tenantID, startDate, endDate).Scan(&results).Error
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

// QueryKMDINFData queries invoice rows eligible for the KMD INF appendix.
func (r *GORMRepository) QueryKMDINFData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time, threshold decimal.Decimal) ([]KMDINFReportRow, error) {
	invoicesTable, err := database.QualifiedTable(schemaName, "invoices")
	if err != nil {
		return nil, err
	}
	contactsTable, err := database.QualifiedTable(schemaName, "contacts")
	if err != nil {
		return nil, err
	}

	var results []struct {
		Part                       KMDINFPart
		ContactID                  string
		ContactName                string
		ContactRegCode             string
		ContactVATNumber           string
		InvoiceID                  string
		InvoiceNumber              string
		InvoiceDate                time.Time
		InvoiceType                string
		TaxableAmount              models.Decimal
		VATAmount                  models.Decimal
		TotalAmount                models.Decimal
		PartnerPeriodTaxableAmount models.Decimal
	}

	err = r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		WITH invoice_rows AS (
			SELECT
				CASE
					WHEN i.invoice_type = 'SALES' THEN 'A'
					WHEN i.invoice_type = 'PURCHASE' THEN 'B'
				END AS part,
				i.contact_id,
				COALESCE(c.name, '') AS contact_name,
				COALESCE(c.reg_code, '') AS contact_reg_code,
				COALESCE(c.vat_number, '') AS contact_vat_number,
				i.id AS invoice_id,
				i.invoice_number,
				i.issue_date AS invoice_date,
				i.invoice_type,
				i.base_subtotal AS taxable_amount,
				i.base_vat_amount AS vat_amount,
				i.base_total AS total_amount
			FROM %s i
			JOIN %s c ON c.id = i.contact_id AND c.tenant_id = i.tenant_id
			WHERE i.tenant_id = ?
				AND i.issue_date >= ?
				AND i.issue_date < ?
				AND i.status NOT IN ('DRAFT', 'VOIDED')
				AND i.invoice_type IN ('SALES', 'PURCHASE')
				AND COALESCE(i.base_vat_amount, 0) <> 0
				AND COALESCE(NULLIF(c.country_code, ''), 'EE') = 'EE'
		),
		qualified_rows AS (
			SELECT
				invoice_rows.*,
				SUM(taxable_amount) OVER (PARTITION BY part, contact_id) AS partner_period_taxable_amount
			FROM invoice_rows
		)
		SELECT
			part, contact_id, contact_name, contact_reg_code, contact_vat_number,
			invoice_id, invoice_number, invoice_date, invoice_type,
			taxable_amount, vat_amount, total_amount, partner_period_taxable_amount
		FROM qualified_rows
		WHERE partner_period_taxable_amount >= ?
		ORDER BY part, contact_name, invoice_date, invoice_number
	`, invoicesTable, contactsTable), tenantID, startDate, endDate, threshold).Scan(&results).Error
	if err != nil {
		return nil, fmt.Errorf("query KMD INF data: %w", err)
	}

	rows := make([]KMDINFReportRow, len(results))
	for i, result := range results {
		rows[i] = KMDINFReportRow{
			Part:                       result.Part,
			ContactID:                  result.ContactID,
			ContactName:                result.ContactName,
			ContactRegCode:             result.ContactRegCode,
			ContactVATNumber:           result.ContactVATNumber,
			InvoiceID:                  result.InvoiceID,
			InvoiceNumber:              result.InvoiceNumber,
			InvoiceDate:                result.InvoiceDate,
			InvoiceType:                result.InvoiceType,
			TaxableAmount:              result.TaxableAmount.Decimal,
			VATAmount:                  result.VATAmount.Decimal,
			TotalAmount:                result.TotalAmount.Decimal,
			PartnerPeriodTaxableAmount: result.PartnerPeriodTaxableAmount.Decimal,
		}
	}

	return rows, nil
}

// SaveDeclaration saves a KMD declaration (upsert)
func (r *GORMRepository) SaveDeclaration(ctx context.Context, schemaName string, decl *KMDDeclaration) error {
	declarationsDB, err := r.tenantTable(ctx, schemaName, "kmd_declarations")
	if err != nil {
		return err
	}
	declarationsTable, err := database.QualifiedTable(schemaName, "kmd_declarations")
	if err != nil {
		return err
	}
	rowsTable, err := database.QualifiedTable(schemaName, "kmd_rows")
	if err != nil {
		return err
	}

	return declarationsDB.Transaction(func(tx *gorm.DB) error {
		rowsDB, err := database.TenantTable(tx, schemaName, "kmd_rows")
		if err != nil {
			return err
		}

		// Upsert declaration using raw SQL for ON CONFLICT
		err = tx.Raw(fmt.Sprintf(`
			INSERT INTO %s (id, tenant_id, year, month, status, total_output_vat, total_input_vat, submitted_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (tenant_id, year, month) DO UPDATE SET
				status = EXCLUDED.status,
				total_output_vat = EXCLUDED.total_output_vat,
				total_input_vat = EXCLUDED.total_input_vat,
				submitted_at = EXCLUDED.submitted_at,
				updated_at = EXCLUDED.updated_at
			RETURNING id
		`, declarationsTable), decl.ID, decl.TenantID, decl.Year, decl.Month, decl.Status,
			decl.TotalOutputVAT.String(), decl.TotalInputVAT.String(),
			decl.SubmittedAt, decl.CreatedAt, decl.UpdatedAt).Scan(&decl.ID).Error
		if err != nil {
			return fmt.Errorf("insert declaration: %w", err)
		}

		// Delete old rows
		if err := tx.Exec(fmt.Sprintf(`DELETE FROM %s WHERE declaration_id = ?`, rowsTable), decl.ID).Error; err != nil {
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
			if err := rowsDB.Create(rowModel).Error; err != nil {
				return fmt.Errorf("insert row: %w", err)
			}
		}

		return nil
	})
}

// GetDeclaration retrieves a KMD declaration for a given period
func (r *GORMRepository) GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*KMDDeclaration, error) {
	db, err := r.tenantTable(ctx, schemaName, "kmd_declarations")
	if err != nil {
		return nil, err
	}

	var declModel models.KMDDeclaration
	err = db.Where("tenant_id = ? AND year = ? AND month = ?", tenantID, year, month).First(&declModel).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get declaration: %w", err)
	}

	// Get rows
	rowsDB, err := r.tenantTable(ctx, schemaName, "kmd_rows")
	if err != nil {
		return nil, err
	}
	var rowModels []models.KMDRow
	if err := rowsDB.Where("declaration_id = ?", declModel.ID).Order("code").Find(&rowModels).Error; err != nil {
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
	db, err := r.tenantTable(ctx, schemaName, "kmd_declarations")
	if err != nil {
		return nil, err
	}

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
