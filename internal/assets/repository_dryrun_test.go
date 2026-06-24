package assets

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type assetDryRunConnPool struct{}

func (assetDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tests should not prepare statements")
}

func (assetDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tests should not execute statements")
}

func (assetDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tests should not query rows")
}

func (assetDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (assetDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &assetDryRunTx{}, nil
}

type assetDryRunTx struct {
	assetDryRunConnPool
}

func (*assetDryRunTx) Commit() error {
	return nil
}

func (*assetDryRunTx) Rollback() error {
	return nil
}

type assetDryRunDBOption func(t *testing.T, db *gorm.DB)

type assetDryRunFixtures struct {
	category   *models.AssetCategory
	categories []models.AssetCategory
	asset      *models.FixedAsset
	assets     []models.FixedAsset
	entries    []models.DepreciationEntry
}

func newAssetDryRunDB(t *testing.T, opts ...assetDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: assetDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withAssetDryRunFixtures(fixtures assetDryRunFixtures) assetDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Query().After("gorm:query").Register(assetDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *models.AssetCategory:
				if fixtures.category != nil {
					*dest = *fixtures.category
					tx.RowsAffected = 1
				}
			case *[]models.AssetCategory:
				if fixtures.categories != nil {
					*dest = append([]models.AssetCategory(nil), fixtures.categories...)
					tx.RowsAffected = int64(len(fixtures.categories))
				}
			case *models.FixedAsset:
				if fixtures.asset != nil {
					*dest = *fixtures.asset
					tx.RowsAffected = 1
				}
			case *[]models.FixedAsset:
				if fixtures.assets != nil {
					*dest = append([]models.FixedAsset(nil), fixtures.assets...)
					tx.RowsAffected = int64(len(fixtures.assets))
				}
			case *[]models.DepreciationEntry:
				if fixtures.entries != nil {
					*dest = append([]models.DepreciationEntry(nil), fixtures.entries...)
					tx.RowsAffected = int64(len(fixtures.entries))
				}
			}
		})
		require.NoError(t, err)
	}
}

func withAssetDryRunCreateError(expectedErr error) assetDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Create().Before("gorm:create").Register(assetDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAssetDryRunQueryError(expectedErr error) assetDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Query().Before("gorm:query").Register(assetDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAssetDryRunUpdateRows(rows int64) assetDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Update().After("gorm:update").Register(assetDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		require.NoError(t, err)
	}
}

func withAssetDryRunUpdateError(expectedErr error) assetDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Update().Before("gorm:update").Register(assetDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAssetDryRunDeleteRows(rows int64) assetDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Delete().After("gorm:delete").Register(assetDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		require.NoError(t, err)
	}
}

func withAssetDryRunDeleteError(expectedErr error) assetDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Delete().Before("gorm:delete").Register(assetDryRunCallbackName(t, "delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func assetDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "assets_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_assets"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	category := assetDryRunCategory(tenantID, now)
	asset := assetDryRunAsset(tenantID, category.ID, now)
	entry := assetDryRunDepreciationEntry(tenantID, asset.ID, now)
	repo := NewGORMRepository(newAssetDryRunDB(t,
		withAssetDryRunFixtures(assetDryRunFixtures{
			category:   assetCategoryToModel(category),
			categories: []models.AssetCategory{*assetCategoryToModel(category)},
			asset:      fixedAssetToModel(asset),
			assets:     []models.FixedAsset{*fixedAssetToModel(asset)},
			entries:    []models.DepreciationEntry{*depreciationEntryToModel(entry)},
		}),
		withAssetDryRunUpdateRows(1),
		withAssetDryRunDeleteRows(1),
	))

	require.NoError(t, repo.CreateCategory(ctx, schemaName, category))

	gotCategory, err := repo.GetCategoryByID(ctx, schemaName, tenantID, category.ID)
	require.NoError(t, err)
	assert.Equal(t, category.ID, gotCategory.ID)

	categories, err := repo.ListCategories(ctx, schemaName, tenantID)
	require.NoError(t, err)
	require.Len(t, categories, 1)
	assert.Equal(t, category.Name, categories[0].Name)

	category.Name = "Updated machinery"
	require.NoError(t, repo.UpdateCategory(ctx, schemaName, category))
	require.NoError(t, repo.DeleteCategory(ctx, schemaName, tenantID, category.ID))

	require.NoError(t, repo.Create(ctx, schemaName, asset))

	gotAsset, err := repo.GetByID(ctx, schemaName, tenantID, asset.ID)
	require.NoError(t, err)
	assert.Equal(t, asset.ID, gotAsset.ID)

	assets, err := repo.List(ctx, schemaName, tenantID, &AssetFilter{
		Status:     AssetStatusActive,
		CategoryID: category.ID,
		Search:     " forklift ",
	})
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, asset.AssetNumber, assets[0].AssetNumber)

	asset.Name = "Updated forklift"
	require.NoError(t, repo.Update(ctx, schemaName, asset))
	require.NoError(t, repo.UpdateStatus(ctx, schemaName, tenantID, asset.ID, AssetStatusActive))

	disposalDate := now.AddDate(0, 1, 0)
	disposalMethod := DisposalSold
	disposalJournalEntryID := "journal-1"
	asset.DisposalDate = &disposalDate
	asset.DisposalMethod = &disposalMethod
	asset.DisposalProceeds = decimal.NewFromInt(1500)
	asset.DisposalNotes = "Sold to upgrade equipment"
	asset.DisposalJournalEntryID = &disposalJournalEntryID
	require.NoError(t, repo.UpdateDisposal(ctx, schemaName, asset, AssetStatusSold))

	require.NoError(t, repo.Delete(ctx, schemaName, tenantID, asset.ID))

	require.NoError(t, repo.CreateDepreciationEntry(ctx, schemaName, entry))

	entries, err := repo.ListDepreciationEntries(ctx, schemaName, tenantID, asset.ID)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entry.DepreciationAmount.Equal(entries[0].DepreciationAmount))

	asset.AccumulatedDepreciation = decimal.NewFromInt(1000)
	asset.BookValue = decimal.NewFromInt(11000)
	asset.LastDepreciationDate = &now
	require.NoError(t, repo.UpdateAssetDepreciation(ctx, schemaName, asset))
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_assets"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	category := assetDryRunCategory(tenantID, now)
	asset := assetDryRunAsset(tenantID, category.ID, now)
	entry := assetDryRunDepreciationEntry(tenantID, asset.ID, now)
	dbErr := errors.New("database failed")

	t.Run("create errors", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t, withAssetDryRunCreateError(dbErr)))

		err := repo.CreateCategory(ctx, schemaName, category)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insert category")
		assert.ErrorIs(t, err, dbErr)

		err = repo.Create(ctx, schemaName, asset)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insert asset")
		assert.ErrorIs(t, err, dbErr)

		err = repo.CreateDepreciationEntry(ctx, schemaName, entry)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insert depreciation entry")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("not found query errors", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t, withAssetDryRunQueryError(gorm.ErrRecordNotFound)))

		gotCategory, err := repo.GetCategoryByID(ctx, schemaName, tenantID, category.ID)
		assert.Nil(t, gotCategory)
		assert.ErrorIs(t, err, ErrCategoryNotFound)

		gotAsset, err := repo.GetByID(ctx, schemaName, tenantID, asset.ID)
		assert.Nil(t, gotAsset)
		assert.ErrorIs(t, err, ErrAssetNotFound)
	})

	t.Run("query errors", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t, withAssetDryRunQueryError(dbErr)))

		gotCategory, err := repo.GetCategoryByID(ctx, schemaName, tenantID, category.ID)
		assert.Nil(t, gotCategory)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get category")
		assert.ErrorIs(t, err, dbErr)

		categories, err := repo.ListCategories(ctx, schemaName, tenantID)
		assert.Nil(t, categories)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list categories")
		assert.ErrorIs(t, err, dbErr)

		gotAsset, err := repo.GetByID(ctx, schemaName, tenantID, asset.ID)
		assert.Nil(t, gotAsset)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "get asset")
		assert.ErrorIs(t, err, dbErr)

		assets, err := repo.List(ctx, schemaName, tenantID, &AssetFilter{Search: "forklift"})
		assert.Nil(t, assets)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list assets")
		assert.ErrorIs(t, err, dbErr)

		entries, err := repo.ListDepreciationEntries(ctx, schemaName, tenantID, asset.ID)
		assert.Nil(t, entries)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "list depreciation entries")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("scan errors", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t))

		number, err := repo.GenerateNumber(ctx, schemaName, tenantID)
		assert.Empty(t, number)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "generate asset number")
	})

	t.Run("update errors", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t, withAssetDryRunUpdateError(dbErr)))

		assertWrappedUpdateError(t, repo.UpdateCategory(ctx, schemaName, category), "update category", dbErr)
		assertWrappedUpdateError(t, repo.Update(ctx, schemaName, asset), "update asset", dbErr)
		assertWrappedUpdateError(t, repo.UpdateStatus(ctx, schemaName, tenantID, asset.ID, AssetStatusActive), "update status", dbErr)
		assertWrappedUpdateError(t, repo.UpdateDisposal(ctx, schemaName, asset, AssetStatusSold), "update disposal", dbErr)
		assertWrappedUpdateError(t, repo.UpdateAssetDepreciation(ctx, schemaName, asset), "update asset depreciation", dbErr)
	})

	t.Run("update missing rows", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t, withAssetDryRunUpdateRows(0)))

		assert.ErrorIs(t, repo.UpdateCategory(ctx, schemaName, category), ErrCategoryNotFound)
		assert.ErrorIs(t, repo.Update(ctx, schemaName, asset), ErrAssetNotFound)
		assert.ErrorIs(t, repo.UpdateStatus(ctx, schemaName, tenantID, asset.ID, AssetStatusActive), ErrAssetNotFound)
		assert.ErrorIs(t, repo.UpdateDisposal(ctx, schemaName, asset, AssetStatusSold), ErrAssetNotFound)
		assert.ErrorIs(t, repo.UpdateAssetDepreciation(ctx, schemaName, asset), ErrAssetNotFound)
	})

	t.Run("delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t, withAssetDryRunDeleteError(dbErr)))

		err := repo.DeleteCategory(ctx, schemaName, tenantID, category.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete category")
		assert.ErrorIs(t, err, dbErr)

		err = repo.Delete(ctx, schemaName, tenantID, asset.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete asset")
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("delete missing rows", func(t *testing.T) {
		repo := NewGORMRepository(newAssetDryRunDB(t, withAssetDryRunDeleteRows(0)))

		assert.ErrorIs(t, repo.DeleteCategory(ctx, schemaName, tenantID, category.ID), ErrCategoryNotFound)
		assert.ErrorIs(t, repo.Delete(ctx, schemaName, tenantID, asset.ID), ErrAssetNotFound)
	})
}

func assertWrappedUpdateError(t *testing.T, err error, message string, target error) {
	t.Helper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), message)
	assert.ErrorIs(t, err, target)
}

func assetDryRunCategory(tenantID string, now time.Time) *AssetCategory {
	assetAccountID := "asset-account"
	depreciationExpenseAccountID := "depreciation-expense"
	accumulatedDepreciationAccountID := "accumulated-depreciation"
	return &AssetCategory{
		ID:                            "category-1",
		TenantID:                      tenantID,
		Name:                          "Machinery",
		Description:                   "Production equipment",
		DepreciationMethod:            DepreciationStraightLine,
		DefaultUsefulLifeMonths:       60,
		DefaultResidualValuePercent:   decimal.NewFromInt(10),
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
		CreatedAt:                     now,
		UpdatedAt:                     now,
	}
}

func assetDryRunAsset(tenantID, categoryID string, now time.Time) *FixedAsset {
	assetAccountID := "asset-account"
	depreciationExpenseAccountID := "depreciation-expense"
	accumulatedDepreciationAccountID := "accumulated-depreciation"
	depreciationStartDate := now.AddDate(0, 0, 1)
	return &FixedAsset{
		ID:                            "asset-1",
		TenantID:                      tenantID,
		AssetNumber:                   "FA-00007",
		Name:                          "Forklift",
		Description:                   "Warehouse forklift",
		CategoryID:                    &categoryID,
		Status:                        AssetStatusActive,
		PurchaseDate:                  now,
		PurchaseCost:                  decimal.NewFromInt(12000),
		SerialNumber:                  "SN-DRY-RUN",
		Location:                      "Warehouse",
		DepreciationMethod:            DepreciationStraightLine,
		UsefulLifeMonths:              60,
		ResidualValue:                 decimal.NewFromInt(1200),
		DepreciationStartDate:         &depreciationStartDate,
		AccumulatedDepreciation:       decimal.Zero,
		BookValue:                     decimal.NewFromInt(12000),
		AssetAccountID:                &assetAccountID,
		DepreciationExpenseAccountID:  &depreciationExpenseAccountID,
		AccumulatedDepreciationAcctID: &accumulatedDepreciationAccountID,
		CreatedAt:                     now,
		CreatedBy:                     "user-1",
		UpdatedAt:                     now,
	}
}

func assetDryRunDepreciationEntry(tenantID, assetID string, now time.Time) *DepreciationEntry {
	journalEntryID := "journal-1"
	return &DepreciationEntry{
		ID:                 "depreciation-1",
		TenantID:           tenantID,
		AssetID:            assetID,
		DepreciationDate:   now,
		PeriodStart:        now.AddDate(0, 0, -30),
		PeriodEnd:          now,
		DepreciationAmount: decimal.NewFromInt(180),
		AccumulatedTotal:   decimal.NewFromInt(180),
		BookValueAfter:     decimal.NewFromInt(11820),
		JournalEntryID:     &journalEntryID,
		Notes:              "Monthly depreciation",
		CreatedAt:          now,
		CreatedBy:          "user-1",
	}
}
