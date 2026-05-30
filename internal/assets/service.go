package assets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type accountingPoster interface {
	GetAccount(ctx context.Context, schemaName, tenantID, accountID string) (*accounting.Account, error)
	CreateJournalEntry(ctx context.Context, schemaName, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error)
	PostJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID string) error
}

// Service provides fixed asset operations
type Service struct {
	db     *pgxpool.Pool
	repo   Repository
	ledger accountingPoster
}

// NewService creates a new assets service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:     db,
		repo:   NewPostgresRepository(db),
		ledger: accounting.NewService(db),
	}
}

// NewServiceWithRepository creates a new assets service with a custom repository
func NewServiceWithRepository(repo Repository) *Service {
	return NewServiceWithRepositoryAndAccounting(repo, nil)
}

// NewServiceWithRepositoryAndAccounting creates a new assets service with custom repository and ledger poster.
func NewServiceWithRepositoryAndAccounting(repo Repository, ledger accountingPoster) *Service {
	return &Service{
		repo:   repo,
		ledger: ledger,
	}
}

// CreateCategory creates a new asset category
func (s *Service) CreateCategory(ctx context.Context, tenantID, schemaName string, req *CreateCategoryRequest) (*AssetCategory, error) {
	cat := &AssetCategory{
		ID:                            uuid.New().String(),
		TenantID:                      tenantID,
		Name:                          req.Name,
		Description:                   req.Description,
		DepreciationMethod:            req.DepreciationMethod,
		DefaultUsefulLifeMonths:       req.DefaultUsefulLifeMonths,
		DefaultResidualValuePercent:   req.DefaultResidualValuePercent,
		AssetAccountID:                req.AssetAccountID,
		DepreciationExpenseAccountID:  req.DepreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: req.AccumulatedDepreciationAcctID,
		CreatedAt:                     time.Now(),
		UpdatedAt:                     time.Now(),
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
	categoryID := trimmedStringPtr(req.CategoryID)
	var category *AssetCategory
	var normalizedCategoryID *string
	if categoryID != "" {
		cat, err := s.repo.GetCategoryByID(ctx, schemaName, tenantID, categoryID)
		if err != nil {
			return nil, fmt.Errorf("get category: %w", err)
		}
		category = cat
		normalizedCategoryID = &categoryID
	}

	asset := &FixedAsset{
		ID:                            uuid.New().String(),
		TenantID:                      tenantID,
		Name:                          req.Name,
		Description:                   req.Description,
		CategoryID:                    normalizedCategoryID,
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
		if category != nil && category.DepreciationMethod != "" {
			asset.DepreciationMethod = category.DepreciationMethod
		} else {
			asset.DepreciationMethod = DepreciationStraightLine
		}
	}
	if asset.UsefulLifeMonths <= 0 {
		if category != nil && category.DefaultUsefulLifeMonths > 0 {
			asset.UsefulLifeMonths = category.DefaultUsefulLifeMonths
		} else {
			asset.UsefulLifeMonths = 60
		}
	}
	if asset.ResidualValue.IsZero() && category != nil && category.DefaultResidualValuePercent.GreaterThan(decimal.Zero) {
		asset.ResidualValue = asset.PurchaseCost.Mul(category.DefaultResidualValuePercent).Div(decimal.NewFromInt(100)).Round(2)
	}
	if category != nil {
		if asset.AssetAccountID == nil {
			asset.AssetAccountID = category.AssetAccountID
		}
		if asset.DepreciationExpenseAccountID == nil {
			asset.DepreciationExpenseAccountID = category.DepreciationExpenseAccountID
		}
		if asset.AccumulatedDepreciationAcctID == nil {
			asset.AccumulatedDepreciationAcctID = category.AccumulatedDepreciationAcctID
		}
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

	if err := s.repo.UpdateDisposal(ctx, schemaName, asset, newStatus); err != nil {
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

	journalEntryID, err := s.recordDepreciationJournal(ctx, schemaName, tenantID, asset, entry, userID)
	if err != nil {
		return nil, err
	}
	entry.JournalEntryID = journalEntryID

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

func (s *Service) recordDepreciationJournal(ctx context.Context, schemaName, tenantID string, asset *FixedAsset, entry *DepreciationEntry, userID string) (*string, error) {
	expenseAccountID := trimmedStringPtr(asset.DepreciationExpenseAccountID)
	accumulatedAccountID := trimmedStringPtr(asset.AccumulatedDepreciationAcctID)
	if expenseAccountID == "" && accumulatedAccountID == "" {
		return nil, nil
	}
	if expenseAccountID == "" || accumulatedAccountID == "" {
		return nil, fmt.Errorf("%w: depreciation expense and accumulated depreciation accounts are required together", ErrAssetAccountingInvalid)
	}
	if s.ledger == nil {
		return nil, fmt.Errorf("%w: accounting service is unavailable", ErrAssetAccountingInvalid)
	}
	expenseAccount, err := s.ledger.GetAccount(ctx, schemaName, tenantID, expenseAccountID)
	if err != nil {
		return nil, fmt.Errorf("%w: load depreciation expense account: %v", ErrAssetAccountingInvalid, err)
	}
	if expenseAccount.AccountType != accounting.AccountTypeExpense {
		return nil, fmt.Errorf("%w: depreciation expense account must be EXPENSE", ErrAssetAccountingInvalid)
	}
	accumulatedAccount, err := s.ledger.GetAccount(ctx, schemaName, tenantID, accumulatedAccountID)
	if err != nil {
		return nil, fmt.Errorf("%w: load accumulated depreciation account: %v", ErrAssetAccountingInvalid, err)
	}
	if accumulatedAccount.AccountType != accounting.AccountTypeAsset {
		return nil, fmt.Errorf("%w: accumulated depreciation account must be ASSET", ErrAssetAccountingInvalid)
	}

	sourceID := entry.ID
	description := depreciationJournalDescription(asset)
	journalEntry, err := s.ledger.CreateJournalEntry(ctx, schemaName, tenantID, &accounting.CreateJournalEntryRequest{
		EntryDate:   entry.PeriodEnd,
		Description: description,
		Reference:   depreciationJournalReference(asset, entry),
		SourceType:  SourceTypeAssetDepreciation,
		SourceID:    &sourceID,
		UserID:      userID,
		Lines: []accounting.CreateJournalEntryLineReq{
			{
				AccountID:    expenseAccountID,
				Description:  description,
				DebitAmount:  entry.DepreciationAmount,
				CreditAmount: decimal.Zero,
			},
			{
				AccountID:    accumulatedAccountID,
				Description:  description,
				DebitAmount:  decimal.Zero,
				CreditAmount: entry.DepreciationAmount,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create depreciation journal: %w", err)
	}
	if err := s.ledger.PostJournalEntry(ctx, schemaName, tenantID, journalEntry.ID, userID); err != nil {
		return nil, fmt.Errorf("post depreciation journal: %w", err)
	}

	return &journalEntry.ID, nil
}

func trimmedStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func depreciationJournalDescription(asset *FixedAsset) string {
	assetNumber := strings.TrimSpace(asset.AssetNumber)
	name := strings.TrimSpace(asset.Name)
	switch {
	case assetNumber != "" && name != "":
		return fmt.Sprintf("Depreciation %s - %s", assetNumber, name)
	case assetNumber != "":
		return fmt.Sprintf("Depreciation %s", assetNumber)
	case name != "":
		return fmt.Sprintf("Depreciation %s", name)
	default:
		return fmt.Sprintf("Depreciation asset %s", asset.ID)
	}
}

func depreciationJournalReference(asset *FixedAsset, entry *DepreciationEntry) string {
	assetNumber := strings.TrimSpace(asset.AssetNumber)
	if assetNumber == "" {
		assetNumber = strings.TrimSpace(asset.ID)
	}
	period := entry.PeriodEnd.Format("2006-01")
	if period == "0001-01" {
		return assetNumber
	}
	return fmt.Sprintf("%s-%s", assetNumber, period)
}

// GetDepreciationHistory retrieves depreciation entries for an asset
func (s *Service) GetDepreciationHistory(ctx context.Context, tenantID, schemaName, assetID string) ([]DepreciationEntry, error) {
	entries, err := s.repo.ListDepreciationEntries(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return nil, fmt.Errorf("list depreciation entries: %w", err)
	}
	return entries, nil
}
