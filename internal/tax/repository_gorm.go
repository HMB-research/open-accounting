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
	"gorm.io/gorm/clause"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM tax repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

var euVATOSSCountryCodes = []string{
	"AT", "BE", "BG", "CY", "CZ", "DE", "DK", "EE", "EL", "ES", "FI", "FR", "HR",
	"HU", "IE", "IT", "LT", "LU", "LV", "MT", "NL", "PL", "PT", "RO", "SE", "SI", "SK",
}

type vatAggregateScanRow struct {
	VATRate   models.Decimal
	IsOutput  bool
	TaxBase   models.Decimal
	TaxAmount models.Decimal
}

type vatAggregateKey struct {
	vatRate  string
	isOutput bool
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return database.TenantTable(db, schemaName, tableName)
}

func (r *GORMRepository) dbWithContext(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("tax repository database is not configured")
	}
	return r.db.WithContext(ctx), nil
}

// QueryVATData queries VAT data from journal entries for a period
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
	invoicesTable, err := database.QualifiedTable(schemaName, "invoices")
	if err != nil {
		return nil, err
	}
	invoiceLinesTable, err := database.QualifiedTable(schemaName, "invoice_lines")
	if err != nil {
		return nil, err
	}
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var rows []vatAggregateScanRow
	if err := db.
		Table(entriesTable+" AS je").
		Select(`
			COALESCE(jl.vat_rate, 0) AS vat_rate,
			CASE WHEN a.account_type IN ('REVENUE', 'INCOME') THEN true ELSE false END AS is_output,
			SUM(jl.credit_amount - jl.debit_amount) AS tax_base,
			SUM((jl.credit_amount - jl.debit_amount) * COALESCE(jl.vat_rate, 0) / 100) AS tax_amount
		`).
		Joins("JOIN "+linesTable+" AS jl ON je.id = jl.journal_entry_id").
		Joins("JOIN "+accountsTable+" AS a ON jl.account_id = a.id").
		Where("je.tenant_id = ?", tenantID).
		Where("je.status = ?", "POSTED").
		Where("je.entry_date >= ?", startDate).
		Where("je.entry_date <= ?", endDate).
		Where("COALESCE(jl.vat_rate, 0) > 0").
		Group("jl.vat_rate, a.account_type").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query VAT data: %w", err)
	}

	var reverseChargeRows []struct {
		VATRate   models.Decimal
		TaxBase   models.Decimal
		TaxAmount models.Decimal
	}
	if err := db.
		Table(invoicesTable+" AS i").
		Select(`
			il.vat_rate,
			SUM(il.line_subtotal * i.exchange_rate) AS tax_base,
			SUM(il.line_subtotal * i.exchange_rate * il.vat_rate / 100) AS tax_amount
		`).
		Joins("JOIN "+invoiceLinesTable+" AS il ON il.invoice_id = i.id AND il.tenant_id = i.tenant_id").
		Where("i.tenant_id = ?", tenantID).
		Where("i.invoice_type = ?", "PURCHASE").
		Where("i.status NOT IN ?", []string{"DRAFT", "VOIDED"}).
		Where("i.issue_date >= ?", startDate).
		Where("i.issue_date <= ?", endDate).
		Where("il.vat_treatment = ?", "REVERSE_CHARGE").
		Where("il.vat_rate > 0").
		Group("il.vat_rate").
		Scan(&reverseChargeRows).Error; err != nil {
		return nil, fmt.Errorf("query reverse charge VAT data: %w", err)
	}

	for _, row := range reverseChargeRows {
		rows = append(rows,
			vatAggregateScanRow{
				VATRate:   row.VATRate,
				IsOutput:  true,
				TaxBase:   row.TaxBase,
				TaxAmount: row.TaxAmount,
			},
			vatAggregateScanRow{
				VATRate:   row.VATRate,
				IsOutput:  false,
				TaxBase:   row.TaxBase,
				TaxAmount: row.TaxAmount,
			},
		)
	}

	return mergeVATAggregateRows(rows), nil
}

func mergeVATAggregateRows(rows []vatAggregateScanRow) []VATAggregateRow {
	aggregates := make(map[vatAggregateKey]VATAggregateRow)
	order := make([]vatAggregateKey, 0, len(rows))

	for _, row := range rows {
		if row.TaxAmount.IsZero() {
			continue
		}

		key := vatAggregateKey{
			vatRate:  row.VATRate.String(),
			isOutput: row.IsOutput,
		}
		aggregate, exists := aggregates[key]
		if !exists {
			order = append(order, key)
			aggregate = VATAggregateRow{
				VATRate:  row.VATRate.Decimal,
				IsOutput: row.IsOutput,
			}
		}
		aggregate.TaxBase = aggregate.TaxBase.Add(row.TaxBase.Decimal)
		aggregate.TaxAmount = aggregate.TaxAmount.Add(row.TaxAmount.Decimal)
		aggregates[key] = aggregate
	}

	result := make([]VATAggregateRow, 0, len(order))
	for _, key := range order {
		result = append(result, aggregates[key])
	}
	return result
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
	db, err := r.dbWithContext(ctx)
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

	invoiceRows := db.
		Table(invoicesTable+" AS i").
		Select(`
			CASE WHEN i.invoice_type = 'SALES' THEN 'A' WHEN i.invoice_type = 'PURCHASE' THEN 'B' END AS part,
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
		`).
		Joins("JOIN "+contactsTable+" AS c ON c.id = i.contact_id AND c.tenant_id = i.tenant_id").
		Where("i.tenant_id = ?", tenantID).
		Where("i.issue_date >= ?", startDate).
		Where("i.issue_date < ?", endDate).
		Where("i.status NOT IN ?", []string{"DRAFT", "VOIDED"}).
		Where("i.invoice_type IN ?", []string{"SALES", "PURCHASE"}).
		Where("COALESCE(i.base_vat_amount, 0) <> 0").
		Where("COALESCE(NULLIF(c.country_code, ''), 'EE') = ?", "EE")
	qualifiedRows := db.
		Table("(?) AS invoice_rows", invoiceRows).
		Select(`
			invoice_rows.*,
			SUM(taxable_amount) OVER (PARTITION BY part, contact_id) AS partner_period_taxable_amount
		`)

	if err := db.
		Table("(?) AS qualified_rows", qualifiedRows).
		Select(`
			part,
			contact_id,
			contact_name,
			contact_reg_code,
			contact_vat_number,
			invoice_id,
			invoice_number,
			invoice_date,
			invoice_type,
			taxable_amount,
			vat_amount,
			total_amount,
			partner_period_taxable_amount
		`).
		Where("partner_period_taxable_amount >= ?", threshold).
		Order("part, contact_name, invoice_date, invoice_number").
		Scan(&results).Error; err != nil {
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

// QueryEUVATOSSData queries EU VAT OSS destination-country aggregates.
func (r *GORMRepository) QueryEUVATOSSData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time, includeB2B bool) ([]EUVATOSSReportRow, error) {
	invoicesTable, err := database.QualifiedTable(schemaName, "invoices")
	if err != nil {
		return nil, err
	}
	contactsTable, err := database.QualifiedTable(schemaName, "contacts")
	if err != nil {
		return nil, err
	}
	invoiceLinesTable, err := database.QualifiedTable(schemaName, "invoice_lines")
	if err != nil {
		return nil, err
	}
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return nil, err
	}

	var results []struct {
		CountryCode   string
		VATRate       models.Decimal
		InvoiceCount  int
		LineCount     int
		TaxableAmount models.Decimal
		VATAmount     models.Decimal
		TotalAmount   models.Decimal
	}

	countryCodeExpr := "COALESCE(NULLIF(UPPER(c.country_code), ''), 'EE')"
	if err := db.
		Table(invoicesTable+" AS i").
		Select(fmt.Sprintf(`
			%s AS country_code,
			il.vat_rate,
			COUNT(DISTINCT i.id) AS invoice_count,
			COUNT(*) AS line_count,
			SUM(il.line_subtotal * i.exchange_rate) AS taxable_amount,
			SUM(il.line_vat * i.exchange_rate) AS vat_amount,
			SUM(il.line_total * i.exchange_rate) AS total_amount
		`, countryCodeExpr)).
		Joins("JOIN "+contactsTable+" AS c ON c.id = i.contact_id AND c.tenant_id = i.tenant_id").
		Joins("JOIN "+invoiceLinesTable+" AS il ON il.invoice_id = i.id AND il.tenant_id = i.tenant_id").
		Where("i.tenant_id = ?", tenantID).
		Where("i.invoice_type = ?", "SALES").
		Where("i.status NOT IN ?", []string{"DRAFT", "VOIDED"}).
		Where("i.issue_date >= ?", startDate).
		Where("i.issue_date < ?", endDate).
		Where(countryCodeExpr+" IN ?", euVATOSSCountryCodes).
		Where(countryCodeExpr+" <> ?", "EE").
		Where("COALESCE(NULLIF(il.vat_treatment, ''), 'STANDARD') = ?", "STANDARD").
		Where("il.vat_rate > 0").
		Where("il.line_vat <> 0").
		Where("? OR COALESCE(NULLIF(TRIM(c.vat_number), ''), '') = ''", includeB2B).
		Group("country_code, il.vat_rate").
		Order("country_code, il.vat_rate").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("query EU VAT OSS data: %w", err)
	}

	rows := make([]EUVATOSSReportRow, len(results))
	for i, result := range results {
		rows[i] = EUVATOSSReportRow{
			CountryCode:   result.CountryCode,
			VATRate:       result.VATRate.Decimal,
			InvoiceCount:  result.InvoiceCount,
			LineCount:     result.LineCount,
			TaxableAmount: result.TaxableAmount.Decimal,
			VATAmount:     result.VATAmount.Decimal,
			TotalAmount:   result.TotalAmount.Decimal,
		}
	}

	return rows, nil
}

// SaveDeclaration saves a KMD declaration (upsert)
func (r *GORMRepository) SaveDeclaration(ctx context.Context, schemaName string, decl *KMDDeclaration) error {
	db, err := r.dbWithContext(ctx)
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		declarationsDB, err := database.TenantTable(tx, schemaName, "kmd_declarations")
		if err != nil {
			return err
		}
		rowsDB, err := database.TenantTable(tx, schemaName, "kmd_rows")
		if err != nil {
			return err
		}

		declModel := kmdDeclarationToModel(decl)
		if err := declarationsDB.
			Clauses(
				clause.OnConflict{
					Columns: []clause.Column{{Name: "tenant_id"}, {Name: "year"}, {Name: "month"}},
					DoUpdates: clause.Assignments(map[string]interface{}{
						"status":           decl.Status,
						"total_output_vat": models.Decimal{Decimal: decl.TotalOutputVAT},
						"total_input_vat":  models.Decimal{Decimal: decl.TotalInputVAT},
						"submitted_at":     decl.SubmittedAt,
						"updated_at":       decl.UpdatedAt,
					}),
				},
				clause.Returning{Columns: []clause.Column{{Name: "id"}}},
			).
			Create(declModel).Error; err != nil {
			return fmt.Errorf("insert declaration: %w", err)
		}
		decl.ID = declModel.ID

		if err := rowsDB.Where("declaration_id = ?", decl.ID).Delete(&models.KMDRow{}).Error; err != nil {
			return fmt.Errorf("delete old rows: %w", err)
		}

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

// MarkKMDSubmitted records the e-MTA submission timestamp for a KMD declaration.
func (r *GORMRepository) MarkKMDSubmitted(ctx context.Context, schemaName, tenantID, declarationID string, submittedAt time.Time) error {
	db, err := r.tenantTable(ctx, schemaName, "kmd_declarations")
	if err != nil {
		return err
	}

	result := db.
		Where("tenant_id = ? AND id = ?", tenantID, declarationID).
		Updates(map[string]any{
			"status":       KMDStatusSubmitted,
			"submitted_at": submittedAt,
			"updated_at":   submittedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("mark KMD submitted: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrKMDDeclarationNotFound
	}
	return nil
}

// UpdateKMDStatus updates a KMD declaration status without replacing its rows.
func (r *GORMRepository) UpdateKMDStatus(ctx context.Context, schemaName, tenantID, declarationID, status string, updatedAt time.Time) error {
	db, err := r.tenantTable(ctx, schemaName, "kmd_declarations")
	if err != nil {
		return err
	}

	result := db.
		Where("tenant_id = ? AND id = ?", tenantID, declarationID).
		Updates(map[string]any{
			"status":     status,
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update KMD status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrKMDDeclarationNotFound
	}
	return nil
}

// Conversion helpers

func kmdDeclarationToModel(decl *KMDDeclaration) *models.KMDDeclaration {
	return &models.KMDDeclaration{
		ID:             decl.ID,
		TenantID:       decl.TenantID,
		Year:           decl.Year,
		Month:          decl.Month,
		Status:         decl.Status,
		TotalOutputVAT: models.Decimal{Decimal: decl.TotalOutputVAT},
		TotalInputVAT:  models.Decimal{Decimal: decl.TotalInputVAT},
		SubmittedAt:    decl.SubmittedAt,
		CreatedAt:      decl.CreatedAt,
		UpdatedAt:      decl.UpdatedAt,
	}
}

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
