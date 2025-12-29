package tax

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// VATEntry represents a VAT entry for aggregation
type VATEntry struct {
	VATCode   string
	TaxBase   float64
	TaxAmount float64
	IsOutput  bool
}

// Service provides tax declaration operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new tax service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// EnsureSchema creates tax tables if they don't exist
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
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

	_, err := s.db.Exec(ctx, query)
	return err
}

// GenerateKMD generates a KMD declaration for a given period
func (s *Service) GenerateKMD(ctx context.Context, tenantID, schemaName string, req *CreateKMDRequest) (*KMDDeclaration, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	// Calculate period boundaries
	startDate := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	// Query VAT data from journal entries
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(jl.vat_rate, 0) as vat_rate,
			CASE
				WHEN a.account_type = 'REVENUE' THEN true
				ELSE false
			END as is_output,
			SUM(CASE WHEN jl.is_debit THEN -jl.amount ELSE jl.amount END) as tax_base,
			SUM(CASE WHEN jl.is_debit THEN -jl.amount ELSE jl.amount END) * COALESCE(jl.vat_rate, 0) / 100 as tax_amount
		FROM %s.journal_entries je
		JOIN %s.journal_lines jl ON je.id = jl.journal_entry_id
		JOIN %s.accounts a ON jl.account_id = a.id
		WHERE je.tenant_id = $1
			AND je.status = 'POSTED'
			AND je.entry_date >= $2
			AND je.entry_date <= $3
			AND COALESCE(jl.vat_rate, 0) > 0
		GROUP BY jl.vat_rate, a.account_type
	`, schemaName, schemaName, schemaName), tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query VAT data: %w", err)
	}
	defer rows.Close()

	// Aggregate into KMD rows
	kmdRows := make([]KMDRow, 0)
	var totalOutput, totalInput decimal.Decimal

	for rows.Next() {
		var vatRate decimal.Decimal
		var isOutput bool
		var taxBase, taxAmount decimal.Decimal

		if err := rows.Scan(&vatRate, &isOutput, &taxBase, &taxAmount); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		code := mapVATRateToKMDCode(vatRate, isOutput)
		desc := getKMDRowDescription(code)

		kmdRows = append(kmdRows, KMDRow{
			Code:        code,
			Description: desc,
			TaxBase:     taxBase.Abs(),
			TaxAmount:   taxAmount.Abs(),
		})

		if isOutput {
			totalOutput = totalOutput.Add(taxAmount.Abs())
		} else {
			totalInput = totalInput.Add(taxAmount.Abs())
		}
	}

	// Create declaration
	decl := &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		Year:           req.Year,
		Month:          req.Month,
		Status:         "DRAFT",
		TotalOutputVAT: totalOutput,
		TotalInputVAT:  totalInput,
		Rows:           kmdRows,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Save to database
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.kmd_declarations (id, tenant_id, year, month, status, total_output_vat, total_input_vat, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, year, month) DO UPDATE SET
			status = EXCLUDED.status,
			total_output_vat = EXCLUDED.total_output_vat,
			total_input_vat = EXCLUDED.total_input_vat,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`, schemaName), decl.ID, tenantID, req.Year, req.Month, decl.Status, decl.TotalOutputVAT, decl.TotalInputVAT, decl.CreatedAt, decl.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert declaration: %w", err)
	}

	// Delete old rows and insert new ones
	_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.kmd_rows WHERE declaration_id = $1`, schemaName), decl.ID)
	if err != nil {
		return nil, fmt.Errorf("delete old rows: %w", err)
	}

	for _, row := range decl.Rows {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.kmd_rows (declaration_id, code, description, tax_base, tax_amount)
			VALUES ($1, $2, $3, $4, $5)
		`, schemaName), decl.ID, row.Code, row.Description, row.TaxBase, row.TaxAmount)
		if err != nil {
			return nil, fmt.Errorf("insert row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return decl, nil
}

// GetKMD retrieves a KMD declaration for a given period
func (s *Service) GetKMD(ctx context.Context, tenantID, schemaName, yearStr, monthStr string) (*KMDDeclaration, error) {
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return nil, fmt.Errorf("invalid year: %w", err)
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid month: %w", err)
	}

	var decl KMDDeclaration
	err = s.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, year, month, status, total_output_vat, total_input_vat, submitted_at, created_at, updated_at
		FROM %s.kmd_declarations
		WHERE tenant_id = $1 AND year = $2 AND month = $3
	`, schemaName), tenantID, year, month).Scan(
		&decl.ID, &decl.TenantID, &decl.Year, &decl.Month, &decl.Status,
		&decl.TotalOutputVAT, &decl.TotalInputVAT, &decl.SubmittedAt, &decl.CreatedAt, &decl.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get declaration: %w", err)
	}

	// Get rows
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT code, description, tax_base, tax_amount
		FROM %s.kmd_rows
		WHERE declaration_id = $1
		ORDER BY code
	`, schemaName), decl.ID)
	if err != nil {
		return nil, fmt.Errorf("get rows: %w", err)
	}
	defer rows.Close()

	decl.Rows = make([]KMDRow, 0)
	for rows.Next() {
		var row KMDRow
		if err := rows.Scan(&row.Code, &row.Description, &row.TaxBase, &row.TaxAmount); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		decl.Rows = append(decl.Rows, row)
	}

	return &decl, nil
}

// ListKMD lists all KMD declarations for a tenant
func (s *Service) ListKMD(ctx context.Context, tenantID, schemaName string) ([]KMDDeclaration, error) {
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, year, month, status, total_output_vat, total_input_vat, submitted_at, created_at, updated_at
		FROM %s.kmd_declarations
		WHERE tenant_id = $1
		ORDER BY year DESC, month DESC
	`, schemaName), tenantID)
	if err != nil {
		return nil, fmt.Errorf("list declarations: %w", err)
	}
	defer rows.Close()

	declarations := make([]KMDDeclaration, 0)
	for rows.Next() {
		var decl KMDDeclaration
		if err := rows.Scan(
			&decl.ID, &decl.TenantID, &decl.Year, &decl.Month, &decl.Status,
			&decl.TotalOutputVAT, &decl.TotalInputVAT, &decl.SubmittedAt, &decl.CreatedAt, &decl.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan declaration: %w", err)
		}
		declarations = append(declarations, decl)
	}

	return declarations, nil
}

// mapVATRateToKMDCode maps a VAT rate to the appropriate KMD row code
func mapVATRateToKMDCode(rate decimal.Decimal, isOutput bool) string {
	rateFloat, _ := rate.Float64()

	if isOutput {
		switch {
		case rateFloat >= 20:
			return KMDRow1 // Standard rate (20/22/24%)
		case rateFloat == 13:
			return KMDRow21 // Accommodation (13%)
		case rateFloat == 9:
			return KMDRow2 // Reduced rate (9%)
		case rateFloat == 0:
			return KMDRow3 // Zero-rated
		default:
			return KMDRow1
		}
	}
	return KMDRow4 // Input VAT
}

// getKMDRowDescription returns the description for a KMD row code
func getKMDRowDescription(code string) string {
	descriptions := map[string]string{
		KMDRow1:  "Maksustatav käive standardmääraga / Taxable sales at standard rate",
		KMDRow2:  "Maksustatav käive vähendatud määraga 9% / Taxable sales at 9%",
		KMDRow21: "Maksustatav käive vähendatud määraga 13% / Taxable sales at 13%",
		KMDRow3:  "Nullmääraga käive (eksport) / Zero-rated exports",
		KMDRow31: "Nullmääraga käive (EL-i sisene) / Zero-rated intra-EU",
		KMDRow4:  "Sisendkäibemaks / Input VAT on domestic purchases",
		KMDRow5:  "Sisendkäibemaks impordilt / Input VAT on imports",
		KMDRow6:  "Sisendkäibemaks põhivaralt / Input VAT on fixed assets",
	}
	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Unknown"
}

// aggregateVATByCode aggregates VAT entries by code (used for testing)
func aggregateVATByCode(entries []VATEntry) []KMDRow {
	aggregated := make(map[string]*KMDRow)

	for _, e := range entries {
		code := e.VATCode
		if _, exists := aggregated[code]; !exists {
			aggregated[code] = &KMDRow{
				Code:        code,
				Description: getKMDRowDescription(code),
				TaxBase:     decimal.Zero,
				TaxAmount:   decimal.Zero,
			}
		}
		aggregated[code].TaxBase = aggregated[code].TaxBase.Add(decimal.NewFromFloat(e.TaxBase))
		aggregated[code].TaxAmount = aggregated[code].TaxAmount.Add(decimal.NewFromFloat(e.TaxAmount))
	}

	rows := make([]KMDRow, 0, len(aggregated))
	for _, row := range aggregated {
		rows = append(rows, *row)
	}
	return rows
}
