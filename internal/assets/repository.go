package assets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
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
	UpdateDisposal(ctx context.Context, schemaName string, asset *FixedAsset, status AssetStatus) error
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

// GORMRepository implements Repository with the shared ORM layer.
type GORMRepository struct {
	db *gorm.DB
}

func NewRepository(db *pgxpool.Pool) *GORMRepository {
	if db == nil {
		return &GORMRepository{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create assets GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

// CreateCategory inserts a new asset category
func (r *GORMRepository) CreateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error {
	db, err := r.tenantTable(ctx, schemaName, "asset_categories")
	if err != nil {
		return fmt.Errorf("qualify asset categories table: %w", err)
	}
	if err := db.Create(assetCategoryToModel(cat)).Error; err != nil {
		return fmt.Errorf("insert category: %w", err)
	}
	return nil
}

// GetCategoryByID retrieves a category by ID
func (r *GORMRepository) GetCategoryByID(ctx context.Context, schemaName, tenantID, categoryID string) (*AssetCategory, error) {
	db, err := r.tenantTable(ctx, schemaName, "asset_categories")
	if err != nil {
		return nil, fmt.Errorf("qualify asset categories table: %w", err)
	}

	var categoryModel models.AssetCategory
	err = db.Where("id = ? AND tenant_id = ?", categoryID, tenantID).First(&categoryModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCategoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}
	return assetCategoryFromModel(&categoryModel), nil
}

// ListCategories retrieves all categories for a tenant
func (r *GORMRepository) ListCategories(ctx context.Context, schemaName, tenantID string) ([]AssetCategory, error) {
	db, err := r.tenantTable(ctx, schemaName, "asset_categories")
	if err != nil {
		return nil, fmt.Errorf("qualify asset categories table: %w", err)
	}

	var categoryModels []models.AssetCategory
	if err := db.Where("tenant_id = ?", tenantID).Order("name ASC").Find(&categoryModels).Error; err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	categories := make([]AssetCategory, len(categoryModels))
	for i := range categoryModels {
		categories[i] = *assetCategoryFromModel(&categoryModels[i])
	}
	return categories, nil
}

// UpdateCategory updates a category
func (r *GORMRepository) UpdateCategory(ctx context.Context, schemaName string, cat *AssetCategory) error {
	db, err := r.tenantTable(ctx, schemaName, "asset_categories")
	if err != nil {
		return fmt.Errorf("qualify asset categories table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", cat.ID, cat.TenantID).
		Updates(map[string]interface{}{
			"name":                                cat.Name,
			"description":                         cat.Description,
			"depreciation_method":                 string(cat.DepreciationMethod),
			"default_useful_life_months":          cat.DefaultUsefulLifeMonths,
			"default_residual_value_percent":      cat.DefaultResidualValuePercent.String(),
			"asset_account_id":                    cat.AssetAccountID,
			"depreciation_expense_account_id":     cat.DepreciationExpenseAccountID,
			"accumulated_depreciation_account_id": cat.AccumulatedDepreciationAcctID,
			"updated_at":                          time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update category: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// DeleteCategory deletes a category
func (r *GORMRepository) DeleteCategory(ctx context.Context, schemaName, tenantID, categoryID string) error {
	db, err := r.tenantTable(ctx, schemaName, "asset_categories")
	if err != nil {
		return fmt.Errorf("qualify asset categories table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", categoryID, tenantID).Delete(&models.AssetCategory{})
	if result.Error != nil {
		return fmt.Errorf("delete category: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

// Create inserts a new fixed asset
func (r *GORMRepository) Create(ctx context.Context, schemaName string, asset *FixedAsset) error {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return fmt.Errorf("qualify fixed assets table: %w", err)
	}
	if err := db.Create(fixedAssetToModel(asset)).Error; err != nil {
		return fmt.Errorf("insert asset: %w", err)
	}
	return nil
}

// GetByID retrieves an asset by ID
func (r *GORMRepository) GetByID(ctx context.Context, schemaName, tenantID, assetID string) (*FixedAsset, error) {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return nil, fmt.Errorf("qualify fixed assets table: %w", err)
	}

	var assetModel models.FixedAsset
	err = db.Where("id = ? AND tenant_id = ?", assetID, tenantID).First(&assetModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrAssetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}
	return fixedAssetFromModel(&assetModel), nil
}

// List retrieves assets with optional filtering
func (r *GORMRepository) List(ctx context.Context, schemaName, tenantID string, filter *AssetFilter) ([]FixedAsset, error) {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return nil, fmt.Errorf("qualify fixed assets table: %w", err)
	}

	query := db.Where("tenant_id = ?", tenantID)
	if filter != nil {
		if filter.Status != "" {
			query = query.Where("status = ?", string(filter.Status))
		}
		if filter.CategoryID != "" {
			query = query.Where("category_id = ?", filter.CategoryID)
		}
		if strings.TrimSpace(filter.Search) != "" {
			search := "%" + strings.TrimSpace(filter.Search) + "%"
			query = query.Where("name ILIKE ? OR asset_number ILIKE ?", search, search)
		}
	}

	var assetModels []models.FixedAsset
	if err := query.Order("purchase_date DESC").Order("asset_number DESC").Find(&assetModels).Error; err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}

	assets := make([]FixedAsset, len(assetModels))
	for i := range assetModels {
		assets[i] = *fixedAssetFromModel(&assetModels[i])
	}
	return assets, nil
}

// Update updates an asset
func (r *GORMRepository) Update(ctx context.Context, schemaName string, asset *FixedAsset) error {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return fmt.Errorf("qualify fixed assets table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ? AND status IN ?", asset.ID, asset.TenantID, []string{string(AssetStatusDraft), string(AssetStatusActive)}).
		Updates(map[string]interface{}{
			"name":                                asset.Name,
			"description":                         asset.Description,
			"category_id":                         asset.CategoryID,
			"serial_number":                       asset.SerialNumber,
			"location":                            asset.Location,
			"depreciation_method":                 string(asset.DepreciationMethod),
			"useful_life_months":                  asset.UsefulLifeMonths,
			"residual_value":                      asset.ResidualValue.String(),
			"asset_account_id":                    asset.AssetAccountID,
			"depreciation_expense_account_id":     asset.DepreciationExpenseAccountID,
			"accumulated_depreciation_account_id": asset.AccumulatedDepreciationAcctID,
			"updated_at":                          time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update asset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// UpdateStatus updates the status of an asset
func (r *GORMRepository) UpdateStatus(ctx context.Context, schemaName, tenantID, assetID string, status AssetStatus) error {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return fmt.Errorf("qualify fixed assets table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", assetID, tenantID).
		Updates(map[string]interface{}{"status": string(status), "updated_at": time.Now()})
	if result.Error != nil {
		return fmt.Errorf("update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// UpdateDisposal persists disposal details and marks the asset as sold or disposed.
func (r *GORMRepository) UpdateDisposal(ctx context.Context, schemaName string, asset *FixedAsset, status AssetStatus) error {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return fmt.Errorf("qualify fixed assets table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ? AND status = ?", asset.ID, asset.TenantID, string(AssetStatusActive)).
		Updates(map[string]interface{}{
			"status":                    string(status),
			"disposal_date":             asset.DisposalDate,
			"disposal_method":           disposalMethodString(asset.DisposalMethod),
			"disposal_proceeds":         asset.DisposalProceeds.String(),
			"disposal_notes":            asset.DisposalNotes,
			"disposal_journal_entry_id": asset.DisposalJournalEntryID,
			"updated_at":                time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update disposal: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func disposalMethodString(method *DisposalMethod) *string {
	if method == nil {
		return nil
	}
	value := string(*method)
	return &value
}

// Delete removes a draft asset
func (r *GORMRepository) Delete(ctx context.Context, schemaName, tenantID, assetID string) error {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return fmt.Errorf("qualify fixed assets table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ? AND status = ?", assetID, tenantID, string(AssetStatusDraft)).
		Delete(&models.FixedAsset{})
	if result.Error != nil {
		return fmt.Errorf("delete asset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

// GenerateNumber generates a new asset number
func (r *GORMRepository) GenerateNumber(ctx context.Context, schemaName, tenantID string) (string, error) {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return "", fmt.Errorf("qualify fixed assets table: %w", err)
	}

	var seq int
	if err := db.
		Select(`
			COALESCE(MAX(
				CASE
					WHEN asset_number ~ ? THEN CAST(SUBSTRING(asset_number FROM ?) AS INTEGER)
					ELSE 0
				END
			), 0) + 1
		`, "FA-[0-9]+$", "FA-([0-9]+)$").
		Where("tenant_id = ?", tenantID).
		Scan(&seq).Error; err != nil {
		return "", fmt.Errorf("generate asset number: %w", err)
	}
	return fmt.Sprintf("FA-%05d", seq), nil
}

// CreateDepreciationEntry inserts a new depreciation entry
func (r *GORMRepository) CreateDepreciationEntry(ctx context.Context, schemaName string, entry *DepreciationEntry) error {
	db, err := r.tenantTable(ctx, schemaName, "depreciation_entries")
	if err != nil {
		return fmt.Errorf("qualify depreciation entries table: %w", err)
	}
	if err := db.Create(depreciationEntryToModel(entry)).Error; err != nil {
		return fmt.Errorf("insert depreciation entry: %w", err)
	}
	return nil
}

// ListDepreciationEntries retrieves depreciation entries for an asset
func (r *GORMRepository) ListDepreciationEntries(ctx context.Context, schemaName, tenantID, assetID string) ([]DepreciationEntry, error) {
	db, err := r.tenantTable(ctx, schemaName, "depreciation_entries")
	if err != nil {
		return nil, fmt.Errorf("qualify depreciation entries table: %w", err)
	}

	var entryModels []models.DepreciationEntry
	if err := db.
		Where("asset_id = ? AND tenant_id = ?", assetID, tenantID).
		Order("depreciation_date DESC").
		Find(&entryModels).Error; err != nil {
		return nil, fmt.Errorf("list depreciation entries: %w", err)
	}

	entries := make([]DepreciationEntry, len(entryModels))
	for i := range entryModels {
		entries[i] = *depreciationEntryFromModel(&entryModels[i])
	}
	return entries, nil
}

// UpdateAssetDepreciation updates the depreciation values on an asset
func (r *GORMRepository) UpdateAssetDepreciation(ctx context.Context, schemaName string, asset *FixedAsset) error {
	db, err := r.tenantTable(ctx, schemaName, "fixed_assets")
	if err != nil {
		return fmt.Errorf("qualify fixed assets table: %w", err)
	}

	result := db.Where("id = ? AND tenant_id = ?", asset.ID, asset.TenantID).
		Updates(map[string]interface{}{
			"accumulated_depreciation": asset.AccumulatedDepreciation.String(),
			"book_value":               asset.BookValue.String(),
			"last_depreciation_date":   asset.LastDepreciationDate,
			"updated_at":               time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("update asset depreciation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func assetCategoryToModel(category *AssetCategory) *models.AssetCategory {
	return &models.AssetCategory{
		ID:                            category.ID,
		TenantID:                      category.TenantID,
		Name:                          category.Name,
		Description:                   category.Description,
		DepreciationMethod:            string(category.DepreciationMethod),
		DefaultUsefulLifeMonths:       category.DefaultUsefulLifeMonths,
		DefaultResidualValuePercent:   models.Decimal{Decimal: category.DefaultResidualValuePercent},
		AssetAccountID:                category.AssetAccountID,
		DepreciationExpenseAccountID:  category.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: category.AccumulatedDepreciationAcctID,
		CreatedAt:                     category.CreatedAt,
		UpdatedAt:                     category.UpdatedAt,
	}
}

func assetCategoryFromModel(category *models.AssetCategory) *AssetCategory {
	return &AssetCategory{
		ID:                            category.ID,
		TenantID:                      category.TenantID,
		Name:                          category.Name,
		Description:                   category.Description,
		DepreciationMethod:            DepreciationMethod(category.DepreciationMethod),
		DefaultUsefulLifeMonths:       category.DefaultUsefulLifeMonths,
		DefaultResidualValuePercent:   category.DefaultResidualValuePercent.Decimal,
		AssetAccountID:                category.AssetAccountID,
		DepreciationExpenseAccountID:  category.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: category.AccumulatedDepreciationAcctID,
		CreatedAt:                     category.CreatedAt,
		UpdatedAt:                     category.UpdatedAt,
	}
}

func fixedAssetToModel(asset *FixedAsset) *models.FixedAsset {
	return &models.FixedAsset{
		ID:                            asset.ID,
		TenantID:                      asset.TenantID,
		AssetNumber:                   asset.AssetNumber,
		Name:                          asset.Name,
		Description:                   asset.Description,
		CategoryID:                    asset.CategoryID,
		Status:                        string(asset.Status),
		PurchaseDate:                  asset.PurchaseDate,
		PurchaseCost:                  models.Decimal{Decimal: asset.PurchaseCost},
		SupplierID:                    asset.SupplierID,
		InvoiceID:                     asset.InvoiceID,
		SerialNumber:                  asset.SerialNumber,
		Location:                      asset.Location,
		DepreciationMethod:            string(asset.DepreciationMethod),
		UsefulLifeMonths:              asset.UsefulLifeMonths,
		ResidualValue:                 models.Decimal{Decimal: asset.ResidualValue},
		DepreciationStartDate:         asset.DepreciationStartDate,
		AccumulatedDepreciation:       models.Decimal{Decimal: asset.AccumulatedDepreciation},
		BookValue:                     models.Decimal{Decimal: asset.BookValue},
		LastDepreciationDate:          asset.LastDepreciationDate,
		DisposalDate:                  asset.DisposalDate,
		DisposalMethod:                disposalMethodString(asset.DisposalMethod),
		DisposalProceeds:              models.Decimal{Decimal: asset.DisposalProceeds},
		DisposalNotes:                 asset.DisposalNotes,
		DisposalJournalEntryID:        asset.DisposalJournalEntryID,
		AssetAccountID:                asset.AssetAccountID,
		DepreciationExpenseAccountID:  asset.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: asset.AccumulatedDepreciationAcctID,
		CreatedAt:                     asset.CreatedAt,
		CreatedBy:                     asset.CreatedBy,
		UpdatedAt:                     asset.UpdatedAt,
	}
}

func fixedAssetFromModel(asset *models.FixedAsset) *FixedAsset {
	var disposalMethod *DisposalMethod
	if asset.DisposalMethod != nil {
		method := DisposalMethod(*asset.DisposalMethod)
		disposalMethod = &method
	}

	return &FixedAsset{
		ID:                            asset.ID,
		TenantID:                      asset.TenantID,
		AssetNumber:                   asset.AssetNumber,
		Name:                          asset.Name,
		Description:                   asset.Description,
		CategoryID:                    asset.CategoryID,
		Status:                        AssetStatus(asset.Status),
		PurchaseDate:                  asset.PurchaseDate,
		PurchaseCost:                  asset.PurchaseCost.Decimal,
		SupplierID:                    asset.SupplierID,
		InvoiceID:                     asset.InvoiceID,
		SerialNumber:                  asset.SerialNumber,
		Location:                      asset.Location,
		DepreciationMethod:            DepreciationMethod(asset.DepreciationMethod),
		UsefulLifeMonths:              asset.UsefulLifeMonths,
		ResidualValue:                 asset.ResidualValue.Decimal,
		DepreciationStartDate:         asset.DepreciationStartDate,
		AccumulatedDepreciation:       asset.AccumulatedDepreciation.Decimal,
		BookValue:                     asset.BookValue.Decimal,
		LastDepreciationDate:          asset.LastDepreciationDate,
		DisposalDate:                  asset.DisposalDate,
		DisposalMethod:                disposalMethod,
		DisposalProceeds:              asset.DisposalProceeds.Decimal,
		DisposalNotes:                 asset.DisposalNotes,
		DisposalJournalEntryID:        asset.DisposalJournalEntryID,
		AssetAccountID:                asset.AssetAccountID,
		DepreciationExpenseAccountID:  asset.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: asset.AccumulatedDepreciationAcctID,
		CreatedAt:                     asset.CreatedAt,
		CreatedBy:                     asset.CreatedBy,
		UpdatedAt:                     asset.UpdatedAt,
	}
}

func depreciationEntryToModel(entry *DepreciationEntry) *models.DepreciationEntry {
	return &models.DepreciationEntry{
		ID:                 entry.ID,
		TenantID:           entry.TenantID,
		AssetID:            entry.AssetID,
		DepreciationDate:   entry.DepreciationDate,
		PeriodStart:        entry.PeriodStart,
		PeriodEnd:          entry.PeriodEnd,
		DepreciationAmount: models.Decimal{Decimal: entry.DepreciationAmount},
		AccumulatedTotal:   models.Decimal{Decimal: entry.AccumulatedTotal},
		BookValueAfter:     models.Decimal{Decimal: entry.BookValueAfter},
		JournalEntryID:     entry.JournalEntryID,
		Notes:              entry.Notes,
		CreatedAt:          entry.CreatedAt,
		CreatedBy:          entry.CreatedBy,
	}
}

func depreciationEntryFromModel(entry *models.DepreciationEntry) *DepreciationEntry {
	return &DepreciationEntry{
		ID:                 entry.ID,
		TenantID:           entry.TenantID,
		AssetID:            entry.AssetID,
		DepreciationDate:   entry.DepreciationDate,
		PeriodStart:        entry.PeriodStart,
		PeriodEnd:          entry.PeriodEnd,
		DepreciationAmount: entry.DepreciationAmount.Decimal,
		AccumulatedTotal:   entry.AccumulatedTotal.Decimal,
		BookValueAfter:     entry.BookValueAfter.Decimal,
		JournalEntryID:     entry.JournalEntryID,
		Notes:              entry.Notes,
		CreatedAt:          entry.CreatedAt,
		CreatedBy:          entry.CreatedBy,
	}
}
