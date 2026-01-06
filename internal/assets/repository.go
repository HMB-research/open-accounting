package assets

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository defines the contract for asset data access
type Repository interface {
	// Categories
	CreateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error
	GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*AssetCategory, error)
	ListCategories(ctx context.Context, schemaName, tenantID string) ([]AssetCategory, error)
	UpdateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error
	DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error

	// Assets
	Create(ctx context.Context, schemaName string, asset *FixedAsset) error
	GetByID(ctx context.Context, schemaName, tenantID, assetID string) (*FixedAsset, error)
	List(ctx context.Context, schemaName, tenantID string, filter *AssetFilter) ([]FixedAsset, error)
	Update(ctx context.Context, schemaName string, asset *FixedAsset) error
	UpdateStatus(ctx context.Context, schemaName, tenantID, assetID string, status AssetStatus) error
	Delete(ctx context.Context, schemaName, tenantID, assetID string) error
	GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error)

	// Depreciation
	CreateDepreciationEntry(ctx context.Context, schemaName string, entry *DepreciationEntry) error
	ListDepreciationEntries(ctx context.Context, schemaName, tenantID, assetID string) ([]DepreciationEntry, error)
	UpdateAssetDepreciation(ctx context.Context, schemaName string, asset *FixedAsset) error
}

// ErrAssetNotFound is returned when an asset is not found
var ErrAssetNotFound = fmt.Errorf("asset not found")

// ErrCategoryNotFound is returned when a category is not found
var ErrCategoryNotFound = fmt.Errorf("category not found")

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// CreateCategory inserts a new asset category
func (r *PostgresRepository) CreateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.asset_categories (
			id, tenant_id, name, description, depreciation_method,
			default_useful_life_months, default_residual_value_percent,
			asset_account_id, depreciation_expense_account_id, accumulated_depreciation_account_id,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, schemaName),
		cat.ID, cat.TenantID, cat.Name, cat.Description, cat.DepreciationMethod,
		cat.DefaultUsefulLifeMonths, cat.DefaultResidualValuePercent,
		cat.AssetAccountID, cat.DepreciationExpenseAccountID, cat.AccumulatedDepreciationAcctID,
		cat.CreatedAt, cat.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert category: %w", err)
	}
	return nil
}

// GetCategoryByID retrieves a category by ID
func (r *PostgresRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*AssetCategory, error) {
	var cat AssetCategory
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, COALESCE(description, ''), depreciation_method,
		       default_useful_life_months, default_residual_value_percent,
		       asset_account_id, depreciation_expense_account_id, accumulated_depreciation_account_id,
		       created_at, updated_at
		FROM %s.asset_categories
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), categoryID, tenantID).Scan(
		&cat.ID, &cat.TenantID, &cat.Name, &cat.Description, &cat.DepreciationMethod,
		&cat.DefaultUsefulLifeMonths, &cat.DefaultResidualValuePercent,
		&cat.AssetAccountID, &cat.DepreciationExpenseAccountID, &cat.AccumulatedDepreciationAcctID,
		&cat.CreatedAt, &cat.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}
	return &cat, nil
}

// ListCategories retrieves all categories for a tenant
func (r *PostgresRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]AssetCategory, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, COALESCE(description, ''), depreciation_method,
		       default_useful_life_months, default_residual_value_percent,
		       asset_account_id, depreciation_expense_account_id, accumulated_depreciation_account_id,
		       created_at, updated_at
		FROM %s.asset_categories
		WHERE tenant_id = $1
		ORDER BY name
	`, schemaName), tenantID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var categories []AssetCategory
	for rows.Next() {
		var cat AssetCategory
		if err := rows.Scan(
			&cat.ID, &cat.TenantID, &cat.Name, &cat.Description, &cat.DepreciationMethod,
			&cat.DefaultUsefulLifeMonths, &cat.DefaultResidualValuePercent,
			&cat.AssetAccountID, &cat.DepreciationExpenseAccountID, &cat.AccumulatedDepreciationAcctID,
			&cat.CreatedAt, &cat.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

// UpdateCategory updates a category
func (r *PostgresRepository) UpdateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.asset_categories
		SET name = $1, description = $2, depreciation_method = $3,
		    default_useful_life_months = $4, default_residual_value_percent = $5,
		    asset_account_id = $6, depreciation_expense_account_id = $7,
		    accumulated_depreciation_account_id = $8, updated_at = $9
		WHERE id = $10 AND tenant_id = $11
	`, schemaName),
		cat.Name, cat.Description, cat.DepreciationMethod,
		cat.DefaultUsefulLifeMonths, cat.DefaultResidualValuePercent,
		cat.AssetAccountID, cat.DepreciationExpenseAccountID, cat.AccumulatedDepreciationAcctID,
		time.Now(), cat.ID, cat.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// DeleteCategory deletes a category
func (r *PostgresRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.asset_categories
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), categoryID, tenantID)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// Create inserts a new fixed asset
func (r *PostgresRepository) Create(ctx context.Context, schemaName string, asset *FixedAsset) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.fixed_assets (
			id, tenant_id, asset_number, name, description, category_id, status,
			purchase_date, purchase_cost, supplier_id, invoice_id, serial_number, location,
			depreciation_method, useful_life_months, residual_value, depreciation_start_date,
			accumulated_depreciation, book_value, last_depreciation_date,
			asset_account_id, depreciation_expense_account_id, accumulated_depreciation_account_id,
			created_at, created_by, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26)
	`, schemaName),
		asset.ID, asset.TenantID, asset.AssetNumber, asset.Name, asset.Description, asset.CategoryID, asset.Status,
		asset.PurchaseDate, asset.PurchaseCost, asset.SupplierID, asset.InvoiceID, asset.SerialNumber, asset.Location,
		asset.DepreciationMethod, asset.UsefulLifeMonths, asset.ResidualValue, asset.DepreciationStartDate,
		asset.AccumulatedDepreciation, asset.BookValue, asset.LastDepreciationDate,
		asset.AssetAccountID, asset.DepreciationExpenseAccountID, asset.AccumulatedDepreciationAcctID,
		asset.CreatedAt, asset.CreatedBy, asset.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert asset: %w", err)
	}
	return nil
}

// GetByID retrieves an asset by ID
func (r *PostgresRepository) GetByID(ctx context.Context, schemaName, tenantID, assetID string) (*FixedAsset, error) {
	var a FixedAsset
	var depStartDate, lastDepDate, dispDate *time.Time
	var dispMethod *string
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, asset_number, name, COALESCE(description, ''), category_id, status,
		       purchase_date, purchase_cost, supplier_id, invoice_id, COALESCE(serial_number, ''), COALESCE(location, ''),
		       depreciation_method, useful_life_months, residual_value, depreciation_start_date,
		       accumulated_depreciation, book_value, last_depreciation_date,
		       disposal_date, disposal_method, disposal_proceeds, COALESCE(disposal_notes, ''),
		       asset_account_id, depreciation_expense_account_id, accumulated_depreciation_account_id,
		       created_at, created_by, updated_at
		FROM %s.fixed_assets
		WHERE id = $1 AND tenant_id = $2
	`, schemaName), assetID, tenantID).Scan(
		&a.ID, &a.TenantID, &a.AssetNumber, &a.Name, &a.Description, &a.CategoryID, &a.Status,
		&a.PurchaseDate, &a.PurchaseCost, &a.SupplierID, &a.InvoiceID, &a.SerialNumber, &a.Location,
		&a.DepreciationMethod, &a.UsefulLifeMonths, &a.ResidualValue, &depStartDate,
		&a.AccumulatedDepreciation, &a.BookValue, &lastDepDate,
		&dispDate, &dispMethod, &a.DisposalProceeds, &a.DisposalNotes,
		&a.AssetAccountID, &a.DepreciationExpenseAccountID, &a.AccumulatedDepreciationAcctID,
		&a.CreatedAt, &a.CreatedBy, &a.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrAssetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}
	a.DepreciationStartDate = depStartDate
	a.LastDepreciationDate = lastDepDate
	a.DisposalDate = dispDate
	if dispMethod != nil {
		dm := DisposalMethod(*dispMethod)
		a.DisposalMethod = &dm
	}
	return &a, nil
}

// List retrieves assets with optional filtering
func (r *PostgresRepository) List(ctx context.Context, schemaName, tenantID string, filter *AssetFilter) ([]FixedAsset, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, asset_number, name, COALESCE(description, ''), category_id, status,
		       purchase_date, purchase_cost, supplier_id, invoice_id, COALESCE(serial_number, ''), COALESCE(location, ''),
		       depreciation_method, useful_life_months, residual_value, depreciation_start_date,
		       accumulated_depreciation, book_value, last_depreciation_date,
		       disposal_date, disposal_method, disposal_proceeds, COALESCE(disposal_notes, ''),
		       asset_account_id, depreciation_expense_account_id, accumulated_depreciation_account_id,
		       created_at, created_by, updated_at
		FROM %s.fixed_assets
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	argNum := 2

	if filter != nil {
		if filter.Status != "" {
			query += fmt.Sprintf(" AND status = $%d", argNum)
			args = append(args, filter.Status)
			argNum++
		}
		if filter.CategoryID != "" {
			query += fmt.Sprintf(" AND category_id = $%d", argNum)
			args = append(args, filter.CategoryID)
			argNum++
		}
		if filter.Search != "" {
			query += fmt.Sprintf(" AND (name ILIKE $%d OR asset_number ILIKE $%d)", argNum, argNum)
			args = append(args, "%"+filter.Search+"%")
		}
	}

	query += " ORDER BY purchase_date DESC, asset_number DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	var assets []FixedAsset
	for rows.Next() {
		var a FixedAsset
		var depStartDate, lastDepDate, dispDate *time.Time
		var dispMethod *string
		if err := rows.Scan(
			&a.ID, &a.TenantID, &a.AssetNumber, &a.Name, &a.Description, &a.CategoryID, &a.Status,
			&a.PurchaseDate, &a.PurchaseCost, &a.SupplierID, &a.InvoiceID, &a.SerialNumber, &a.Location,
			&a.DepreciationMethod, &a.UsefulLifeMonths, &a.ResidualValue, &depStartDate,
			&a.AccumulatedDepreciation, &a.BookValue, &lastDepDate,
			&dispDate, &dispMethod, &a.DisposalProceeds, &a.DisposalNotes,
			&a.AssetAccountID, &a.DepreciationExpenseAccountID, &a.AccumulatedDepreciationAcctID,
			&a.CreatedAt, &a.CreatedBy, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		a.DepreciationStartDate = depStartDate
		a.LastDepreciationDate = lastDepDate
		a.DisposalDate = dispDate
		if dispMethod != nil {
			dm := DisposalMethod(*dispMethod)
			a.DisposalMethod = &dm
		}
		assets = append(assets, a)
	}

	return assets, nil
}

// Update updates an asset
func (r *PostgresRepository) Update(ctx context.Context, schemaName string, asset *FixedAsset) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.fixed_assets
		SET name = $1, description = $2, category_id = $3, serial_number = $4, location = $5,
		    depreciation_method = $6, useful_life_months = $7, residual_value = $8,
		    asset_account_id = $9, depreciation_expense_account_id = $10, accumulated_depreciation_account_id = $11,
		    updated_at = $12
		WHERE id = $13 AND tenant_id = $14 AND status IN ('DRAFT', 'ACTIVE')
	`, schemaName),
		asset.Name, asset.Description, asset.CategoryID, asset.SerialNumber, asset.Location,
		asset.DepreciationMethod, asset.UsefulLifeMonths, asset.ResidualValue,
		asset.AssetAccountID, asset.DepreciationExpenseAccountID, asset.AccumulatedDepreciationAcctID,
		time.Now(), asset.ID, asset.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update asset: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// UpdateStatus updates the status of an asset
func (r *PostgresRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, assetID string, status AssetStatus) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.fixed_assets
		SET status = $1, updated_at = $2
		WHERE id = $3 AND tenant_id = $4
	`, schemaName), status, time.Now(), assetID, tenantID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// Delete removes a draft asset
func (r *PostgresRepository) Delete(ctx context.Context, schemaName, tenantID, assetID string) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s.fixed_assets
		WHERE id = $1 AND tenant_id = $2 AND status = 'DRAFT'
	`, schemaName), assetID, tenantID)
	if err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// GenerateNumber generates a new asset number
func (r *PostgresRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	var seq int
	err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(asset_number FROM 'FA-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.fixed_assets WHERE tenant_id = $1
	`, schemaName), tenantID).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("generate asset number: %w", err)
	}
	return fmt.Sprintf("FA-%05d", seq), nil
}

// CreateDepreciationEntry inserts a new depreciation entry
func (r *PostgresRepository) CreateDepreciationEntry(ctx context.Context, schemaName string, entry *DepreciationEntry) error {
	_, err := r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.depreciation_entries (
			id, tenant_id, asset_id, depreciation_date, period_start, period_end,
			depreciation_amount, accumulated_total, book_value_after, journal_entry_id, notes,
			created_at, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, schemaName),
		entry.ID, entry.TenantID, entry.AssetID, entry.DepreciationDate, entry.PeriodStart, entry.PeriodEnd,
		entry.DepreciationAmount, entry.AccumulatedTotal, entry.BookValueAfter, entry.JournalEntryID, entry.Notes,
		entry.CreatedAt, entry.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("insert depreciation entry: %w", err)
	}
	return nil
}

// ListDepreciationEntries retrieves depreciation entries for an asset
func (r *PostgresRepository) ListDepreciationEntries(ctx context.Context, schemaName, tenantID, assetID string) ([]DepreciationEntry, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, asset_id, depreciation_date, period_start, period_end,
		       depreciation_amount, accumulated_total, book_value_after, journal_entry_id, COALESCE(notes, ''),
		       created_at, created_by
		FROM %s.depreciation_entries
		WHERE asset_id = $1 AND tenant_id = $2
		ORDER BY depreciation_date DESC
	`, schemaName), assetID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list depreciation entries: %w", err)
	}
	defer rows.Close()

	var entries []DepreciationEntry
	for rows.Next() {
		var e DepreciationEntry
		if err := rows.Scan(
			&e.ID, &e.TenantID, &e.AssetID, &e.DepreciationDate, &e.PeriodStart, &e.PeriodEnd,
			&e.DepreciationAmount, &e.AccumulatedTotal, &e.BookValueAfter, &e.JournalEntryID, &e.Notes,
			&e.CreatedAt, &e.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("scan depreciation entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// UpdateAssetDepreciation updates the depreciation values on an asset
func (r *PostgresRepository) UpdateAssetDepreciation(ctx context.Context, schemaName string, asset *FixedAsset) error {
	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.fixed_assets
		SET accumulated_depreciation = $1, book_value = $2, last_depreciation_date = $3, updated_at = $4
		WHERE id = $5 AND tenant_id = $6
	`, schemaName),
		asset.AccumulatedDepreciation, asset.BookValue, asset.LastDepreciationDate, time.Now(), asset.ID, asset.TenantID,
	)
	if err != nil {
		return fmt.Errorf("update asset depreciation: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrAssetNotFound
	}
	return nil
}
