package assets

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepository_NilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "CreateCategory",
			run: func(t *testing.T) error {
				return repo.CreateCategory(ctx, schemaName, &AssetCategory{TenantID: tenantID})
			},
		},
		{
			name: "GetCategoryByID",
			run: func(t *testing.T) error {
				category, err := repo.GetCategoryByID(ctx, schemaName, tenantID, "category-1")
				assert.Nil(t, category)
				return err
			},
		},
		{
			name: "ListCategories",
			run: func(t *testing.T) error {
				categories, err := repo.ListCategories(ctx, schemaName, tenantID)
				assert.Nil(t, categories)
				return err
			},
		},
		{
			name: "UpdateCategory",
			run: func(t *testing.T) error {
				return repo.UpdateCategory(ctx, schemaName, &AssetCategory{ID: "category-1", TenantID: tenantID})
			},
		},
		{
			name: "DeleteCategory",
			run: func(t *testing.T) error {
				return repo.DeleteCategory(ctx, schemaName, tenantID, "category-1")
			},
		},
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, schemaName, &FixedAsset{TenantID: tenantID})
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				asset, err := repo.GetByID(ctx, schemaName, tenantID, "asset-1")
				assert.Nil(t, asset)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				assets, err := repo.List(ctx, schemaName, tenantID, &AssetFilter{
					Status:     AssetStatusActive,
					CategoryID: "category-1",
					Search:     " asset ",
				})
				assert.Nil(t, assets)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T) error {
				return repo.Update(ctx, schemaName, &FixedAsset{ID: "asset-1", TenantID: tenantID})
			},
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				return repo.UpdateStatus(ctx, schemaName, tenantID, "asset-1", AssetStatusActive)
			},
		},
		{
			name: "UpdateDisposal",
			run: func(t *testing.T) error {
				return repo.UpdateDisposal(ctx, schemaName, &FixedAsset{ID: "asset-1", TenantID: tenantID}, AssetStatusSold)
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				return repo.Delete(ctx, schemaName, tenantID, "asset-1")
			},
		},
		{
			name: "GenerateNumber",
			run: func(t *testing.T) error {
				number, err := repo.GenerateNumber(ctx, schemaName, tenantID)
				assert.Empty(t, number)
				return err
			},
		},
		{
			name: "CreateDepreciationEntry",
			run: func(t *testing.T) error {
				return repo.CreateDepreciationEntry(ctx, schemaName, &DepreciationEntry{TenantID: tenantID})
			},
		},
		{
			name: "ListDepreciationEntries",
			run: func(t *testing.T) error {
				entries, err := repo.ListDepreciationEntries(ctx, schemaName, tenantID, "asset-1")
				assert.Nil(t, entries)
				return err
			},
		},
		{
			name: "UpdateAssetDepreciation",
			run: func(t *testing.T) error {
				return repo.UpdateAssetDepreciation(ctx, schemaName, &FixedAsset{ID: "asset-1", TenantID: tenantID})
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "fixed_assets")
				assert.Nil(t, table)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "assets repository database is not configured")
		})
	}
}

func TestAssetCategoryModelMappingRoundTrip(t *testing.T) {
	assetAccountID := "asset-account-id"
	depreciationExpenseAccountID := "depreciation-expense-account-id"
	accumulatedDepreciationAccountID := "accumulated-depreciation-account-id"
	createdAt := time.Date(2026, time.January, 5, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.February, 6, 10, 0, 0, 0, time.UTC)
	category := &AssetCategory{
		ID:                            "category-id",
		TenantID:                      "tenant-id",
		Name:                          "Machinery",
		Description:                   "Production machinery",
		DepreciationMethod:            DepreciationDecliningBalance,
		DefaultUsefulLifeMonths:       84,
		DefaultResidualValuePercent:   decimal.RequireFromString("7.50"),
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
		CreatedAt:                     createdAt,
		UpdatedAt:                     updatedAt,
	}

	model := assetCategoryToModel(category)
	assert.Equal(t, string(DepreciationDecliningBalance), model.DepreciationMethod)
	assert.True(t, model.DefaultResidualValuePercent.Decimal.Equal(category.DefaultResidualValuePercent))

	roundTrip := assetCategoryFromModel(model)
	assert.Equal(t, category, roundTrip)
}

func TestFixedAssetModelMappingRoundTrip(t *testing.T) {
	categoryID := "category-id"
	supplierID := "supplier-id"
	invoiceID := "invoice-id"
	assetAccountID := "asset-account-id"
	depreciationExpenseAccountID := "depreciation-expense-account-id"
	accumulatedDepreciationAccountID := "accumulated-depreciation-account-id"
	disposalJournalEntryID := "disposal-journal-entry-id"
	purchaseDate := time.Date(2026, time.March, 10, 0, 0, 0, 0, time.UTC)
	depreciationStartDate := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	lastDepreciationDate := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	disposalDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.March, 11, 8, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.June, 30, 16, 45, 0, 0, time.UTC)
	disposalMethod := DisposalSold
	asset := &FixedAsset{
		ID:                            "asset-id",
		TenantID:                      "tenant-id",
		AssetNumber:                   "FA-00042",
		Name:                          "CNC mill",
		Description:                   "Shop floor production asset",
		CategoryID:                    &categoryID,
		Status:                        AssetStatusSold,
		PurchaseDate:                  purchaseDate,
		PurchaseCost:                  decimal.RequireFromString("125000.00"),
		SupplierID:                    &supplierID,
		InvoiceID:                     &invoiceID,
		SerialNumber:                  "SN-12345",
		Location:                      "Tallinn workshop",
		DepreciationMethod:            DepreciationStraightLine,
		UsefulLifeMonths:              60,
		ResidualValue:                 decimal.RequireFromString("5000.00"),
		DepreciationStartDate:         &depreciationStartDate,
		AccumulatedDepreciation:       decimal.RequireFromString("20000.00"),
		BookValue:                     decimal.RequireFromString("105000.00"),
		LastDepreciationDate:          &lastDepreciationDate,
		DisposalDate:                  &disposalDate,
		DisposalMethod:                &disposalMethod,
		DisposalProceeds:              decimal.RequireFromString("99000.00"),
		DisposalNotes:                 "Sold during replacement cycle",
		DisposalJournalEntryID:        &disposalJournalEntryID,
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
		CreatedAt:                     createdAt,
		CreatedBy:                     "creator-id",
		UpdatedAt:                     updatedAt,
	}

	model := fixedAssetToModel(asset)
	assert.True(t, model.PurchaseCost.Decimal.Equal(asset.PurchaseCost))
	assert.True(t, model.ResidualValue.Decimal.Equal(asset.ResidualValue))
	require.NotNil(t, model.DisposalMethod)
	assert.Equal(t, string(DisposalSold), *model.DisposalMethod)

	roundTrip := fixedAssetFromModel(model)
	assert.Equal(t, asset, roundTrip)
}

func TestDisposalMethodStringNil(t *testing.T) {
	assert.Nil(t, disposalMethodString(nil))
}

func TestDepreciationEntryModelMappingRoundTrip(t *testing.T) {
	journalEntryID := "journal-entry-id"
	depreciationDate := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	periodStart := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.June, 1, 7, 15, 0, 0, time.UTC)
	entry := &DepreciationEntry{
		ID:                 "depreciation-entry-id",
		TenantID:           "tenant-id",
		AssetID:            "asset-id",
		DepreciationDate:   depreciationDate,
		PeriodStart:        periodStart,
		PeriodEnd:          periodEnd,
		DepreciationAmount: decimal.RequireFromString("1666.67"),
		AccumulatedTotal:   decimal.RequireFromString("21666.67"),
		BookValueAfter:     decimal.RequireFromString("103333.33"),
		JournalEntryID:     &journalEntryID,
		Notes:              "Monthly depreciation",
		CreatedAt:          createdAt,
		CreatedBy:          "creator-id",
	}

	model := depreciationEntryToModel(entry)
	assert.True(t, model.DepreciationAmount.Decimal.Equal(entry.DepreciationAmount))
	assert.True(t, model.AccumulatedTotal.Decimal.Equal(entry.AccumulatedTotal))
	assert.True(t, model.BookValueAfter.Decimal.Equal(entry.BookValueAfter))

	roundTrip := depreciationEntryFromModel(model)
	assert.Equal(t, entry, roundTrip)
}
