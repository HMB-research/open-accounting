package assets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

type accountingPoster interface {
	ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.Account, error)
	GetAccount(ctx context.Context, schemaName, tenantID, accountID string) (*accounting.Account, error)
	CreateJournalEntry(ctx context.Context, schemaName, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error)
	PostJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string) error
}

type contactLister interface {
	List(ctx context.Context, tenantID, schemaName string, filter *contacts.ContactFilter) ([]contacts.Contact, error)
}

type assetInvoiceResolver interface {
	ResolveInvoiceIDByNumber(ctx context.Context, tenantID, schemaName, invoiceNumber string) (string, error)
}

// Service provides fixed asset operations
type Service struct {
	repo      Repository
	ledger    accountingPoster
	contacts  contactLister
	invoicing assetInvoiceResolver
}

var (
	newAssetsAccountingService = accounting.NewService
	newAssetsContactsService   = func(db *pgxpool.Pool) contactLister {
		return contacts.NewService(db)
	}
	newAssetsInvoicingService = func(db *pgxpool.Pool, ledger *accounting.Service) assetInvoiceResolver {
		return invoicing.NewService(db, ledger)
	}
)

// NewService creates a new assets service with an ORM-backed repository.
func NewService(db *pgxpool.Pool) *Service {
	ledger := newAssetsAccountingService(db)
	service := &Service{
		repo:     NewRepository(db),
		ledger:   ledger,
		contacts: newAssetsContactsService(db),
	}
	if db != nil {
		service.invoicing = newAssetsInvoicingService(db, ledger)
	}
	return service
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

	var category *AssetCategory
	if categoryID := trimmedStringPtr(req.CategoryID); categoryID != "" {
		cat, err := s.repo.GetCategoryByID(ctx, schemaName, tenantID, categoryID)
		if err != nil {
			return nil, fmt.Errorf("get category: %w", err)
		}
		category = cat
		existing.CategoryID = &categoryID
	}

	// Update fields
	if strings.TrimSpace(req.Name) != "" {
		existing.Name = req.Name
	}
	if strings.TrimSpace(req.Description) != "" {
		existing.Description = req.Description
	}
	if strings.TrimSpace(req.SerialNumber) != "" {
		existing.SerialNumber = req.SerialNumber
	}
	if strings.TrimSpace(req.Location) != "" {
		existing.Location = req.Location
	}
	if req.DepreciationMethod != "" {
		existing.DepreciationMethod = req.DepreciationMethod
	} else if category != nil && category.DepreciationMethod != "" {
		existing.DepreciationMethod = category.DepreciationMethod
	}
	if req.UsefulLifeMonths > 0 {
		existing.UsefulLifeMonths = req.UsefulLifeMonths
	} else if category != nil && category.DefaultUsefulLifeMonths > 0 {
		existing.UsefulLifeMonths = category.DefaultUsefulLifeMonths
	}
	if !req.ResidualValue.IsZero() {
		existing.ResidualValue = req.ResidualValue
	} else if category != nil && category.DefaultResidualValuePercent.GreaterThan(decimal.Zero) {
		existing.ResidualValue = existing.PurchaseCost.Mul(category.DefaultResidualValuePercent).Div(decimal.NewFromInt(100)).Round(2)
	}
	if req.AssetAccountID != nil {
		existing.AssetAccountID = nonEmptyStringPtr(*req.AssetAccountID)
	} else if category != nil && category.AssetAccountID != nil {
		existing.AssetAccountID = category.AssetAccountID
	}
	if req.DepreciationExpenseAccountID != nil {
		existing.DepreciationExpenseAccountID = nonEmptyStringPtr(*req.DepreciationExpenseAccountID)
	} else if category != nil && category.DepreciationExpenseAccountID != nil {
		existing.DepreciationExpenseAccountID = category.DepreciationExpenseAccountID
	}
	if req.AccumulatedDepreciationAcctID != nil {
		existing.AccumulatedDepreciationAcctID = nonEmptyStringPtr(*req.AccumulatedDepreciationAcctID)
	} else if category != nil && category.AccumulatedDepreciationAcctID != nil {
		existing.AccumulatedDepreciationAcctID = category.AccumulatedDepreciationAcctID
	}
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
	if req.DisposalDate.IsZero() {
		return fmt.Errorf("disposal date is required")
	}
	if !isValidDisposalMethod(req.DisposalMethod) {
		return fmt.Errorf("invalid disposal method %q", req.DisposalMethod)
	}
	if req.DisposalProceeds.LessThan(decimal.Zero) {
		return fmt.Errorf("disposal proceeds cannot be negative")
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

	journalEntryID, err := s.recordDisposalJournal(ctx, schemaName, tenantID, asset, req, req.UserID)
	if err != nil {
		return err
	}
	asset.DisposalJournalEntryID = journalEntryID

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
	if expenseAccountID == "" || accumulatedAccountID == "" {
		return nil, fmt.Errorf("%w: depreciation expense and accumulated depreciation accounts are required", ErrAssetAccountingInvalid)
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
	if err := s.ledger.PostJournalEntry(ctx, schemaName, tenantID, journalEntry.ID, userID, "Fixed asset depreciation posting"); err != nil {
		return nil, fmt.Errorf("post depreciation journal: %w", err)
	}

	return &journalEntry.ID, nil
}

func (s *Service) recordDisposalJournal(ctx context.Context, schemaName, tenantID string, asset *FixedAsset, req *DisposeAssetRequest, userID string) (*string, error) {
	assetAccountID := trimmedStringPtr(asset.AssetAccountID)
	accumulatedAccountID := trimmedStringPtr(asset.AccumulatedDepreciationAcctID)
	proceedsAccountID := trimmedStringPtr(req.DisposalProceedsAccountID)
	gainLossAccountID := trimmedStringPtr(req.DisposalGainLossAccountID)
	if assetAccountID == "" || accumulatedAccountID == "" {
		return nil, fmt.Errorf("%w: asset and accumulated depreciation accounts are required for disposal posting", ErrAssetAccountingInvalid)
	}
	if s.ledger == nil {
		return nil, fmt.Errorf("%w: accounting service is unavailable", ErrAssetAccountingInvalid)
	}
	if asset.PurchaseCost.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("%w: asset purchase cost must be positive for disposal posting", ErrAssetAccountingInvalid)
	}
	if asset.AccumulatedDepreciation.LessThan(decimal.Zero) {
		return nil, fmt.Errorf("%w: accumulated depreciation cannot be negative for disposal posting", ErrAssetAccountingInvalid)
	}

	if err := s.requireAccountType(ctx, schemaName, tenantID, assetAccountID, "asset account", accounting.AccountTypeAsset); err != nil {
		return nil, err
	}
	if err := s.requireAccountType(ctx, schemaName, tenantID, accumulatedAccountID, "accumulated depreciation account", accounting.AccountTypeAsset); err != nil {
		return nil, err
	}
	if req.DisposalProceeds.GreaterThan(decimal.Zero) {
		if proceedsAccountID == "" {
			return nil, fmt.Errorf("%w: disposal proceeds account is required when proceeds are recorded", ErrAssetAccountingInvalid)
		}
		if err := s.requireAccountType(ctx, schemaName, tenantID, proceedsAccountID, "disposal proceeds account", accounting.AccountTypeAsset); err != nil {
			return nil, err
		}
	}

	bookValue := asset.PurchaseCost.Sub(asset.AccumulatedDepreciation)
	gainLoss := req.DisposalProceeds.Sub(bookValue)
	if !gainLoss.IsZero() && gainLossAccountID == "" {
		return nil, fmt.Errorf("%w: disposal gain or loss account is required when disposal differs from book value", ErrAssetAccountingInvalid)
	}
	if gainLoss.GreaterThan(decimal.Zero) {
		if err := s.requireAccountType(ctx, schemaName, tenantID, gainLossAccountID, "disposal gain account", accounting.AccountTypeRevenue); err != nil {
			return nil, err
		}
	} else if gainLoss.LessThan(decimal.Zero) {
		if err := s.requireAccountType(ctx, schemaName, tenantID, gainLossAccountID, "disposal loss account", accounting.AccountTypeExpense); err != nil {
			return nil, err
		}
	}

	description := disposalJournalDescription(asset)
	lines := make([]accounting.CreateJournalEntryLineReq, 0, 4)
	if asset.AccumulatedDepreciation.GreaterThan(decimal.Zero) {
		lines = append(lines, accounting.CreateJournalEntryLineReq{
			AccountID:    accumulatedAccountID,
			Description:  description,
			DebitAmount:  asset.AccumulatedDepreciation,
			CreditAmount: decimal.Zero,
		})
	}
	if req.DisposalProceeds.GreaterThan(decimal.Zero) {
		lines = append(lines, accounting.CreateJournalEntryLineReq{
			AccountID:    proceedsAccountID,
			Description:  description,
			DebitAmount:  req.DisposalProceeds,
			CreditAmount: decimal.Zero,
		})
	}
	if gainLoss.LessThan(decimal.Zero) {
		lines = append(lines, accounting.CreateJournalEntryLineReq{
			AccountID:    gainLossAccountID,
			Description:  description,
			DebitAmount:  gainLoss.Abs(),
			CreditAmount: decimal.Zero,
		})
	}
	lines = append(lines, accounting.CreateJournalEntryLineReq{
		AccountID:    assetAccountID,
		Description:  description,
		DebitAmount:  decimal.Zero,
		CreditAmount: asset.PurchaseCost,
	})
	if gainLoss.GreaterThan(decimal.Zero) {
		lines = append(lines, accounting.CreateJournalEntryLineReq{
			AccountID:    gainLossAccountID,
			Description:  description,
			DebitAmount:  decimal.Zero,
			CreditAmount: gainLoss,
		})
	}

	sourceID := asset.ID
	journalEntry, err := s.ledger.CreateJournalEntry(ctx, schemaName, tenantID, &accounting.CreateJournalEntryRequest{
		EntryDate:   req.DisposalDate,
		Description: description,
		Reference:   disposalJournalReference(asset, req.DisposalDate),
		SourceType:  SourceTypeAssetDisposal,
		SourceID:    &sourceID,
		UserID:      userID,
		Lines:       lines,
	})
	if err != nil {
		return nil, fmt.Errorf("create disposal journal: %w", err)
	}
	if err := s.ledger.PostJournalEntry(ctx, schemaName, tenantID, journalEntry.ID, userID, "Fixed asset disposal posting"); err != nil {
		return nil, fmt.Errorf("post disposal journal: %w", err)
	}

	return &journalEntry.ID, nil
}

func (s *Service) requireAccountType(ctx context.Context, schemaName, tenantID, accountID, label string, expected accounting.AccountType) error {
	account, err := s.ledger.GetAccount(ctx, schemaName, tenantID, accountID)
	if err != nil {
		return fmt.Errorf("%w: load %s: %v", ErrAssetAccountingInvalid, label, err)
	}
	if account.AccountType != expected {
		return fmt.Errorf("%w: %s must be %s", ErrAssetAccountingInvalid, label, expected)
	}
	return nil
}

func trimmedStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func nonEmptyStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isValidDisposalMethod(method DisposalMethod) bool {
	switch method {
	case DisposalSold, DisposalScrapped, DisposalDonated, DisposalLost:
		return true
	default:
		return false
	}
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

func disposalJournalDescription(asset *FixedAsset) string {
	assetNumber := strings.TrimSpace(asset.AssetNumber)
	name := strings.TrimSpace(asset.Name)
	switch {
	case assetNumber != "" && name != "":
		return fmt.Sprintf("Disposal %s - %s", assetNumber, name)
	case assetNumber != "":
		return fmt.Sprintf("Disposal %s", assetNumber)
	case name != "":
		return fmt.Sprintf("Disposal %s", name)
	default:
		return fmt.Sprintf("Disposal asset %s", asset.ID)
	}
}

func disposalJournalReference(asset *FixedAsset, disposalDate time.Time) string {
	assetNumber := strings.TrimSpace(asset.AssetNumber)
	if assetNumber == "" {
		assetNumber = strings.TrimSpace(asset.ID)
	}
	if disposalDate.IsZero() {
		return assetNumber
	}
	return fmt.Sprintf("%s-%s", assetNumber, disposalDate.Format("2006-01-02"))
}

// GetDepreciationHistory retrieves depreciation entries for an asset
func (s *Service) GetDepreciationHistory(ctx context.Context, tenantID, schemaName, assetID string) ([]DepreciationEntry, error) {
	entries, err := s.repo.ListDepreciationEntries(ctx, schemaName, tenantID, assetID)
	if err != nil {
		return nil, fmt.Errorf("list depreciation entries: %w", err)
	}
	return entries, nil
}
