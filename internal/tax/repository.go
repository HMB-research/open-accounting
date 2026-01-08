package tax

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// VATAggregateRow represents a VAT aggregate from journal entries
type VATAggregateRow struct {
	VATRate   decimal.Decimal
	IsOutput  bool
	TaxBase   decimal.Decimal
	TaxAmount decimal.Decimal
}

// Repository defines the contract for tax data access
type Repository interface {
	// EnsureSchema creates tax tables if they don't exist
	EnsureSchema(ctx context.Context, schemaName string) error

	// QueryVATData queries VAT data from journal entries for a period
	QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]VATAggregateRow, error)

	// SaveDeclaration saves a KMD declaration (upsert)
	SaveDeclaration(ctx context.Context, schemaName string, decl *KMDDeclaration) error

	// GetDeclaration retrieves a KMD declaration for a given period
	GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*KMDDeclaration, error)

	// ListDeclarations lists all KMD declarations for a tenant
	ListDeclarations(ctx context.Context, schemaName, tenantID string) ([]KMDDeclaration, error)
}

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// EnsureSchema creates tax tables if they don't exist
func (r *PostgresRepository) EnsureSchema(ctx context.Context, schemaName string) error {
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

	_, err := r.db.Exec(ctx, query)
	return err
}

// QueryVATData queries VAT data from journal entries for a period
func (r *PostgresRepository) QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]VATAggregateRow, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(jl.vat_rate, 0) as vat_rate,
			CASE
				WHEN a.account_type IN ('REVENUE', 'INCOME') THEN true
				ELSE false
			END as is_output,
			SUM(jl.credit_amount - jl.debit_amount) as tax_base,
			SUM((jl.credit_amount - jl.debit_amount) * COALESCE(jl.vat_rate, 0) / 100) as tax_amount
		FROM %s.journal_entries je
		JOIN %s.journal_entry_lines jl ON je.id = jl.journal_entry_id
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

	result := []VATAggregateRow{}
	for rows.Next() {
		var row VATAggregateRow
		if err := rows.Scan(&row.VATRate, &row.IsOutput, &row.TaxBase, &row.TaxAmount); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		result = append(result, row)
	}

	return result, nil
}

// SaveDeclaration saves a KMD declaration (upsert)
func (r *PostgresRepository) SaveDeclaration(ctx context.Context, schemaName string, decl *KMDDeclaration) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
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
	`, schemaName), decl.ID, decl.TenantID, decl.Year, decl.Month, decl.Status, decl.TotalOutputVAT, decl.TotalInputVAT, decl.CreatedAt, decl.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert declaration: %w", err)
	}

	// Delete old rows and insert new ones
	_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.kmd_rows WHERE declaration_id = $1`, schemaName), decl.ID)
	if err != nil {
		return fmt.Errorf("delete old rows: %w", err)
	}

	for _, row := range decl.Rows {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.kmd_rows (declaration_id, code, description, tax_base, tax_amount)
			VALUES ($1, $2, $3, $4, $5)
		`, schemaName), decl.ID, row.Code, row.Description, row.TaxBase, row.TaxAmount)
		if err != nil {
			return fmt.Errorf("insert row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// GetDeclaration retrieves a KMD declaration for a given period
func (r *PostgresRepository) GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*KMDDeclaration, error) {
	var decl KMDDeclaration
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, year, month, status, total_output_vat, total_input_vat, submitted_at, created_at, updated_at
		FROM %s.kmd_declarations
		WHERE tenant_id = $1 AND year = $2 AND month = $3
	`, schemaName), tenantID, year, month).Scan(
		&decl.ID, &decl.TenantID, &decl.Year, &decl.Month, &decl.Status,
		&decl.TotalOutputVAT, &decl.TotalInputVAT, &decl.SubmittedAt, &decl.CreatedAt, &decl.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get declaration: %w", err)
	}

	// Get rows
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
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

// ListDeclarations lists all KMD declarations for a tenant
func (r *PostgresRepository) ListDeclarations(ctx context.Context, schemaName, tenantID string) ([]KMDDeclaration, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
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
