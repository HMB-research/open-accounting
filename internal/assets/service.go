package assets

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides fixed asset operations
type Service struct {
	db   *pgxpool.Pool
	repo Repository
}

// NewService creates a new assets service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewPostgresRepository(db),
	}
}

// NewServiceWithRepository creates a new assets service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// CreateCategory creates a new asset category
func (s *Service) CreateCategory(ctx context.Context, tenantID, schemaName string, req *CreateCategoryRequest) (*AssetCategory, error) {
	cat := &AssetCategory{
		ID:                          uuid.New().String(),
		TenantID:                    tenantID,
		Name:                        req.Name,
		Description:                 req.Description,
		DepreciationMethod:          req.DepreciationMethod,
		DefaultUsefulLifeMonths:     req.DefaultUsefulLifeMonths,
		DefaultResidualValuePercent: req.DefaultResidualValuePercent,
		AssetAccountID:              req.AssetAccountID,
		DepreciationExpenseAccountID: req.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: req.AccumulatedDepreciationAcctID,
		CreatedAt:                   time.Now(),
		UpdatedAt:                   time.Now(),
	}

	if cat.DepreciationMethod == "" {
		cat.DepreciationMethod = DepreciationStraightLine
	}
	if cat.DefaultUsefulLifeMonths <= 0 {
		cat.DefaultUsefulLifeMonths = 60
	}

	if err := s.repo.CreateCategory(ctx, schemaName, cat); err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}

	return cat, nil
}

// GetCategoryByID retrieves a category by ID
func (s *Service) GetCategoryByID(ctx context.Context, tenantID, schemaName, categoryID string) (*AssetCategory, error) {
	cat, err := s.repo.GetCategoryByID(ctx, schemaName, tenantID, categoryID)
	if err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}
	return cat, nil
}

// ListCategories retrieves all categories for a tenant
func (s *Service) ListCategories(ctx context.Context, tenantID, schemaName string) ([]AssetCategory, error) {
	categories, err := s.repo.ListCategories(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	return categories, nil
}

// DeleteCategory deletes a category
func (s *Service) DeleteCategory(ctx context.Context, tenantID, schemaName, categoryID string) error {
	if err := s.repo.DeleteCategory(ctx, schemaName, tenantID, categoryID); err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}

// Create creates a new fixed asset
func (s *Service) Create(ctx context.Context, tenantID, schemaName string, req *CreateAssetRequest) (*FixedAsset, error) {
	asset := &FixedAsset{
		ID:                            uuid.New().String(),
		TenantID:                      tenantID,
		Name:                          req.Name,
		Description:                   req.Description,
		CategoryID:                    req.CategoryID,
		Status:                        AssetStatusDraft,
		PurchaseDate:                  req.PurchaseDate,
		PurchaseCost:                  req.PurchaseCost,
		SupplierID:                    req.SupplierID,
		SerialNumber:                  req.SerialNumber,
		Location:                      req.Location,
		DepreciationMethod:            req.DepreciationMethod,
		UsefulLifeMonths:              req.UsefulLifeMonths,
		ResidualValue:                 req.ResidualValue,
		DepreciationStartDate:         req.DepreciationStartDate,
		AccumulatedDepreciation:       decimal.Zero,
		AssetAccountID:                req.AssetAccountID,
		DepreciationExpenseAccountID:  req.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: req.AccumulatedDepreciationAcctID,
		CreatedAt:                     time.Now(),
		CreatedBy:                     req.UserID,
		UpdatedAt:                     time.Now(),
	}

	// Set defaults
	if asset.DepreciationMethod == "" {
		asset.DepreciationMethod = DepreciationStraightLine
	}
	if asset.UsefulLifeMonths <= 0 {
		asset.UsefulLifeMonths = 60
	}

	// Calculate initial book value
	asset.BookValue = asset.PurchaseCost

	// Validate
	if err := asset.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate asset number
	assetNumber, err := s.repo.GenerateNumber(ctx, schemaName, tenantID)
	if err != nil {
		return nil, fmt.Errorf("generate asset number: %w", err)
	}
	asset.AssetNumber = assetNumber

	// Create asset via repository
	if err := s.repo.Create(ctx, schemaName, asset); err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	return asset, nil
}

// GetByID retrieves an asset by ID
func (s *Service) GetByID(ctx context.Context, tenantID, schemaName, assetID string) (*FixedAsset, error) {
	asset, err := s.repo.GetByID(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}
	return asset, nil
}

// List retrieves assets with optional filtering
func (s *Service) List(ctx context.Context, tenantID, schemaName string, filter *AssetFilter) ([]FixedAsset, error) {
	assets, err := s.repo.List(ctx, schemaName, tenantID, filter)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	return assets, nil
}

// Update updates an asset (only draft/active)
func (s *Service) Update(ctx context.Context, tenantID, schemaName, assetID string, req *UpdateAssetRequest) (*FixedAsset, error) {
	// Get existing asset
	existing, err := s.repo.GetByID(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}

	if existing.Status != AssetStatusDraft && existing.Status != AssetStatusActive {
		return nil, fmt.Errorf("only draft or active assets can be updated")
	}

	// Update fields
	existing.Name = req.Name
	existing.Description = req.Description
	existing.CategoryID = req.CategoryID
	existing.SerialNumber = req.SerialNumber
	existing.Location = req.Location
	existing.DepreciationMethod = req.DepreciationMethod
	existing.UsefulLifeMonths = req.UsefulLifeMonths
	existing.ResidualValue = req.ResidualValue
	existing.AssetAccountID = req.AssetAccountID
	existing.DepreciationExpenseAccountID = req.DepreciationExpenseAccountID
	existing.AccumulatedDepreciationAcctID = req.AccumulatedDepreciationAcctID
	existing.UpdatedAt = time.Now()

	if existing.DepreciationMethod == "" {
		existing.DepreciationMethod = DepreciationStraightLine
	}

	// Validate
	if err := existing.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Update via repository
	if err := s.repo.Update(ctx, schemaName, existing); err != nil {
		return nil, fmt.Errorf("update asset: %w", err)
	}

	return existing, nil
}

// Activate marks an asset as active
func (s *Service) Activate(ctx context.Context, tenantID, schemaName, assetID string) error {
	asset, err := s.repo.GetByID(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if asset.Status != AssetStatusDraft {
		return fmt.Errorf("asset is not in draft status")
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, assetID, AssetStatusActive); err != nil {
		return fmt.Errorf("activate asset: %w", err)
	}
	return nil
}

// Dispose marks an asset as disposed
func (s *Service) Dispose(ctx context.Context, tenantID, schemaName, assetID string, req *DisposeAssetRequest) error {
	asset, err := s.repo.GetByID(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return fmt.Errorf("get asset: %w", err)
	}
	if asset.Status != AssetStatusActive {
		return fmt.Errorf("only active assets can be disposed")
	}

	// Update disposal information
	asset.DisposalDate = &req.DisposalDate
	asset.DisposalMethod = &req.DisposalMethod
	asset.DisposalProceeds = req.DisposalProceeds
	asset.DisposalNotes = req.DisposalNotes

	var newStatus AssetStatus
	if req.DisposalMethod == DisposalSold {
		newStatus = AssetStatusSold
	} else {
		newStatus = AssetStatusDisposed
	}

	if err := s.repo.UpdateStatus(ctx, schemaName, tenantID, assetID, newStatus); err != nil {
		return fmt.Errorf("dispose asset: %w", err)
	}
	return nil
}

// Delete deletes a draft asset
func (s *Service) Delete(ctx context.Context, tenantID, schemaName, assetID string) error {
	if err := s.repo.Delete(ctx, schemaName, tenantID, assetID); err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	return nil
}

// RecordDepreciation records depreciation for an asset
func (s *Service) RecordDepreciation(ctx context.Context, tenantID, schemaName, assetID, userID string, periodStart, periodEnd time.Time) (*DepreciationEntry, error) {
	asset, err := s.repo.GetByID(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return nil, fmt.Errorf("get asset: %w", err)
	}

	if asset.Status != AssetStatusActive {
		return nil, fmt.Errorf("only active assets can be depreciated")
	}

	// Calculate depreciation amount
	depAmount := asset.CalculateMonthlyDepreciation()
	if depAmount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("no depreciation to record")
	}

	// Don't exceed the depreciable amount
	maxDepreciation := asset.PurchaseCost.Sub(asset.ResidualValue).Sub(asset.AccumulatedDepreciation)
	if depAmount.GreaterThan(maxDepreciation) {
		depAmount = maxDepreciation
	}

	if depAmount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("asset is fully depreciated")
	}

	// Create depreciation entry
	newAccumulated := asset.AccumulatedDepreciation.Add(depAmount)
	newBookValue := asset.PurchaseCost.Sub(newAccumulated)

	entry := &DepreciationEntry{
		ID:                 uuid.New().String(),
		TenantID:           tenantID,
		AssetID:            assetID,
		DepreciationDate:   time.Now(),
		PeriodStart:        periodStart,
		PeriodEnd:          periodEnd,
		DepreciationAmount: depAmount,
		AccumulatedTotal:   newAccumulated,
		BookValueAfter:     newBookValue,
		CreatedAt:          time.Now(),
		CreatedBy:          userID,
	}

	if err := s.repo.CreateDepreciationEntry(ctx, schemaName, entry); err != nil {
		return nil, fmt.Errorf("create depreciation entry: %w", err)
	}

	// Update asset values
	asset.AccumulatedDepreciation = newAccumulated
	asset.BookValue = newBookValue
	now := time.Now()
	asset.LastDepreciationDate = &now

	if err := s.repo.UpdateAssetDepreciation(ctx, schemaName, asset); err != nil {
		return nil, fmt.Errorf("update asset depreciation: %w", err)
	}

	return entry, nil
}

// GetDepreciationHistory retrieves depreciation entries for an asset
func (s *Service) GetDepreciationHistory(ctx context.Context, tenantID, schemaName, assetID string) ([]DepreciationEntry, error) {
	entries, err := s.repo.ListDepreciationEntries(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return nil, fmt.Errorf("list depreciation entries: %w", err)
	}
	return entries, nil
}
