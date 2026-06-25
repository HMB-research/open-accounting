package inventory

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/contactrefs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type inventoryWave9StockLevelsRepo struct {
	*MockRepository
	levels []StockLevel
	err    error
	calls  int
}

func (r *inventoryWave9StockLevelsRepo) GetStockLevelsByProduct(context.Context, string, string, string) ([]StockLevel, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return append([]StockLevel(nil), r.levels...), nil
}

type inventoryWave9ListProductsRepo struct {
	*MockRepository
	failOnCall int
	calls      int
}

func (r *inventoryWave9ListProductsRepo) ListProducts(ctx context.Context, schemaName, tenantID string, filter *ProductFilter) ([]Product, error) {
	r.calls++
	if r.calls == r.failOnCall {
		return nil, errors.New("list products failed")
	}
	return r.MockRepository.ListProducts(ctx, schemaName, tenantID, filter)
}

type inventoryWave9MovementRepo struct {
	*MockRepository
	movements []InventoryMovement
}

func (r *inventoryWave9MovementRepo) ListMovements(context.Context, string, string, string) ([]InventoryMovement, error) {
	return append([]InventoryMovement(nil), r.movements...), nil
}

type inventoryWave9Poster struct {
	accounts []accounting.Account
}

func (p inventoryWave9Poster) ListAccounts(context.Context, string, string, bool) ([]accounting.Account, error) {
	return append([]accounting.Account(nil), p.accounts...), nil
}

func (p inventoryWave9Poster) CreateJournalEntry(context.Context, string, string, *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	return &accounting.JournalEntry{ID: "journal-1"}, nil
}

func (p inventoryWave9Poster) PostJournalEntry(context.Context, string, string, string, string) error {
	return nil
}

func TestInventoryWave9ConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := inventoryWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewGORMRepository(pool)
	})
}

func TestInventoryWave9ValuationAndSubledgerEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("fifo returns purchase price when all layers have zero value", func(t *testing.T) {
		cost := fifoInventoryUnitCost(Product{PurchasePrice: decimal.Zero}, nil, "tenant-1", decimal.NewFromInt(3))
		assert.True(t, cost.IsZero())
	})

	t.Run("valuation skips cross tenant levels and adds unassigned fallback", func(t *testing.T) {
		base := NewMockRepository()
		base.Products[inventoryStockProductID] = &Product{
			ID:             inventoryStockProductID,
			TenantID:       "tenant-1",
			Code:           "PRD",
			Name:           "Widget",
			ProductType:    ProductTypeGoods,
			TrackInventory: true,
			PurchasePrice:  decimal.NewFromInt(7),
			CurrentStock:   decimal.NewFromInt(3),
		}
		repo := &inventoryWave9StockLevelsRepo{
			MockRepository: base,
			levels: []StockLevel{
				{TenantID: "other-tenant", ProductID: inventoryStockProductID, WarehouseID: "other", Quantity: decimal.NewFromInt(99)},
				{TenantID: "tenant-1", ProductID: inventoryStockProductID, Quantity: decimal.NewFromInt(3), AvailableQty: decimal.NewFromInt(3)},
			},
		}

		report, err := NewServiceWithRepository(repo).GetInventoryValuation(ctx, "tenant-1", "tenant_demo", "", InventoryValuationMethodStandardCost)

		require.NoError(t, err)
		require.Len(t, report.Lines, 1)
		assert.Equal(t, "UNASSIGNED", report.Lines[0].WarehouseCode)
		assert.True(t, report.TotalQuantity.Equal(decimal.NewFromInt(3)))
	})

	t.Run("valuation uses product stock when no stock levels exist", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Products[inventoryStockProductID] = &Product{
			ID:             inventoryStockProductID,
			TenantID:       "tenant-1",
			Code:           "PRD",
			Name:           "Widget",
			ProductType:    ProductTypeGoods,
			TrackInventory: true,
			PurchasePrice:  decimal.NewFromInt(5),
			CurrentStock:   decimal.NewFromInt(4),
		}

		report, err := NewServiceWithRepository(repo).GetInventoryValuation(ctx, "tenant-1", "tenant_demo", "", InventoryValuationMethodStandardCost)

		require.NoError(t, err)
		require.Len(t, report.Lines, 1)
		assert.Empty(t, report.Lines[0].WarehouseID)
		assert.True(t, report.Lines[0].Quantity.Equal(decimal.NewFromInt(4)))
	})

	t.Run("subledger requires balance service", func(t *testing.T) {
		_, err := NewServiceWithRepository(NewMockRepository()).GetInventorySubledgerReconciliation(ctx, "tenant-1", "tenant_demo", "", InventoryValuationMethodStandardCost, time.Time{})
		require.ErrorContains(t, err, "accounting balance service is not configured")
	})

	t.Run("subledger propagates post valuation list errors", func(t *testing.T) {
		base := NewMockRepository()
		base.Products[inventoryStockProductID] = &Product{
			ID:             inventoryStockProductID,
			TenantID:       "tenant-1",
			ProductType:    ProductTypeGoods,
			TrackInventory: true,
			PurchasePrice:  decimal.NewFromInt(1),
		}
		repo := &inventoryWave9ListProductsRepo{MockRepository: base, failOnCall: 2}
		service := NewServiceWithRepositoryAndAccounting(repo, &inventoryWave5Accounting{})

		_, err := service.GetInventorySubledgerReconciliation(ctx, "tenant-1", "tenant_demo", "", InventoryValuationMethodStandardCost, time.Now())

		require.ErrorContains(t, err, "list products failed")
	})

	t.Run("subledger propagates valuation errors", func(t *testing.T) {
		repo := &inventoryWave9ListProductsRepo{MockRepository: NewMockRepository(), failOnCall: 1}
		service := NewServiceWithRepositoryAndAccounting(repo, &inventoryWave5Accounting{})

		_, err := service.GetInventorySubledgerReconciliation(ctx, "tenant-1", "tenant_demo", "", InventoryValuationMethodStandardCost, time.Now())

		require.ErrorContains(t, err, "list products failed")
	})

	t.Run("subledger skips untracked product rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Products["untracked-1"] = &Product{
			ID:             "untracked-1",
			TenantID:       "tenant-1",
			ProductType:    ProductTypeGoods,
			TrackInventory: false,
		}
		service := NewServiceWithRepositoryAndAccounting(repo, &inventoryWave5Accounting{})

		report, err := service.GetInventorySubledgerReconciliation(ctx, "tenant-1", "tenant_demo", "", InventoryValuationMethodStandardCost, time.Now())

		require.NoError(t, err)
		assert.Empty(t, report.Lines)
	})

	t.Run("lot report skips movements from other tenants", func(t *testing.T) {
		base := NewMockRepository()
		base.Products[inventoryStockProductID] = &Product{
			ID:             inventoryStockProductID,
			TenantID:       "tenant-1",
			ProductType:    ProductTypeGoods,
			TrackInventory: true,
			PurchasePrice:  decimal.NewFromInt(2),
		}
		repo := &inventoryWave9MovementRepo{
			MockRepository: base,
			movements: []InventoryMovement{{
				TenantID:     "other-tenant",
				ProductID:    inventoryStockProductID,
				MovementType: MovementTypeIn,
				Quantity:     decimal.NewFromInt(1),
				UnitCost:     decimal.NewFromInt(2),
			}},
		}

		report, err := NewServiceWithRepository(repo).GetInventoryLotReport(ctx, "tenant-1", "tenant_demo", "", "", false)

		require.NoError(t, err)
		assert.Empty(t, report.Lines)
	})
}

func TestInventoryWave9IssueAndReservationEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("transfer wraps destination stock level errors", func(t *testing.T) {
		base := inventoryWave9StockFixture()
		repo := &inventoryWave9StockLevelsRepo{
			MockRepository: base,
			levels: []StockLevel{{
				TenantID:     "tenant-1",
				ProductID:    inventoryStockProductID,
				WarehouseID:  inventoryStockWarehouseID,
				Quantity:     decimal.NewFromInt(5),
				AvailableQty: decimal.NewFromInt(5),
			}},
		}
		repo.err = nil
		service := NewServiceWithRepository(&inventoryWave9SecondStockLevelErrorRepo{inventoryWave9StockLevelsRepo: repo})

		err := service.TransferStock(ctx, "tenant-1", "tenant_demo", &TransferStockRequest{
			ProductID:       inventoryStockProductID,
			FromWarehouseID: inventoryStockWarehouseID,
			ToWarehouseID:   inventoryStockWarehouseID2,
			Quantity:        "1",
		})

		require.ErrorContains(t, err, "get destination stock level")
	})

	t.Run("post to ledger requires configured ledger", func(t *testing.T) {
		service := NewServiceWithRepository(inventoryWave9StockFixture())
		_, err := service.IssueStock(ctx, "tenant-1", "tenant_demo", inventoryWave9IssueRequest(true))
		require.ErrorContains(t, err, "accounting transaction is unavailable")
	})

	t.Run("post to ledger requires positive issue cost", func(t *testing.T) {
		repo := inventoryWave9StockFixture()
		repo.Products[inventoryStockProductID].PurchasePrice = decimal.Zero
		repo.Movements[inventoryStockProductID] = nil
		service := NewServiceWithRepositoryAndAccounting(repo, inventoryWave9Poster{accounts: []accounting.Account{
			{ID: "11111111-1111-4111-8111-111111111111", AccountType: accounting.AccountTypeExpense},
			{ID: "22222222-2222-4222-8222-222222222222", AccountType: accounting.AccountTypeAsset},
		}})
		_, err := service.IssueStock(ctx, "tenant-1", "tenant_demo", inventoryWave9IssueRequest(true))
		require.ErrorContains(t, err, "positive issue cost")
	})

	t.Run("invalid inventory account id is rejected before accounting", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())
		_, err := service.inventoryIssueAccounting(ctx, "tenant_demo", "tenant-1", Product{}, &IssueStockRequest{
			CostOfGoodsSoldAccountID: "11111111-1111-4111-8111-111111111111",
			InventoryAccountID:       "not-a-uuid",
		}, "source-1", decimal.NewFromInt(1))
		require.ErrorContains(t, err, "inventory_account_id must be a valid UUID")
	})

	t.Run("issue accounting defaults reference and source type", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository())
		accountingLines, err := service.inventoryIssueAccounting(ctx, "tenant_demo", "tenant-1",
			Product{Name: "Widget", InventoryAccountID: "22222222-2222-4222-8222-222222222222"},
			&IssueStockRequest{CostOfGoodsSoldAccountID: "11111111-1111-4111-8111-111111111111"},
			"source-1",
			decimal.NewFromInt(9),
		)

		require.NoError(t, err)
		require.NotNil(t, accountingLines)
		assert.Equal(t, "Inventory Issue", accountingLines.Reference)
		assert.Equal(t, inventoryIssueSourceTypeDefault, accountingLines.SourceType)
	})

	t.Run("reserve wraps lot reservation list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ErrOnListLotReservations = true
		service := NewServiceWithRepository(repo)
		err := service.reserveLotAllocations(ctx, "tenant-1", "tenant_demo",
			Product{ID: inventoryStockProductID, TenantID: "tenant-1"},
			&StockLevel{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID},
			&StockReservationRequest{},
			decimal.NewFromInt(1),
		)
		require.NoError(t, err)

		repo.Movements[inventoryStockProductID] = []InventoryMovement{{
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(1),
			UnitCost:     decimal.NewFromInt(1),
			LotNumber:    "LOT-A",
		}}
		err = service.reserveLotAllocations(ctx, "tenant-1", "tenant_demo",
			Product{ID: inventoryStockProductID, TenantID: "tenant-1", PurchasePrice: decimal.NewFromInt(1)},
			&StockLevel{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID, ReservedQty: decimal.NewFromInt(1)},
			&StockReservationRequest{},
			decimal.NewFromInt(1),
		)
		require.ErrorContains(t, err, "list lot reservations")
	})

	t.Run("reserve skips fully reserved lots and propagates create errors", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo)
		product := Product{ID: inventoryStockProductID, TenantID: "tenant-1", PurchasePrice: decimal.NewFromInt(2)}
		level := &StockLevel{ProductID: inventoryStockProductID, WarehouseID: inventoryStockWarehouseID}
		repo.Movements[inventoryStockProductID] = []InventoryMovement{{
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			MovementType: MovementTypeIn,
			Quantity:     decimal.NewFromInt(1),
			UnitCost:     decimal.NewFromInt(2),
			LotNumber:    "LOT-A",
		}}
		repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "")] = &InventoryLotReservation{
			TenantID:    "tenant-1",
			ProductID:   inventoryStockProductID,
			WarehouseID: inventoryStockWarehouseID,
			LotNumber:   "LOT-A",
			Quantity:    decimal.NewFromInt(1),
		}

		require.NoError(t, service.reserveLotAllocations(ctx, "tenant-1", "tenant_demo", product, level, &StockReservationRequest{}, decimal.NewFromInt(1)))

		repo.LotReservations = map[string]*InventoryLotReservation{}
		repo.ErrOnUpsertLotReservation = true
		err := service.reserveLotAllocations(ctx, "tenant-1", "tenant_demo", product, level, &StockReservationRequest{}, decimal.NewFromInt(1))
		require.ErrorContains(t, err, "reserve tracked lot stock")
	})

	t.Run("release untracked lot allocation wraps release errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.LotReservations[inventoryLotReservationKey(inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "")] = &InventoryLotReservation{
			TenantID:    "tenant-1",
			ProductID:   inventoryStockProductID,
			WarehouseID: inventoryStockWarehouseID,
			LotNumber:   "LOT-A",
			Quantity:    decimal.NewFromInt(2),
		}
		repo.ErrOnReleaseLotReservation = true
		service := NewServiceWithRepository(repo)

		err := service.releaseLotAllocations(ctx, "tenant-1", "tenant_demo", &StockReservationRequest{
			ProductID:   inventoryStockProductID,
			WarehouseID: inventoryStockWarehouseID,
		}, decimal.NewFromInt(1))

		require.ErrorContains(t, err, "release tracked lot stock")
	})
}

func TestInventoryWave9FIFOZeroCostFallback(t *testing.T) {
	product := Product{PurchasePrice: decimal.NewFromInt(11)}
	got := fifoInventoryUnitCost(product, []InventoryMovement{{
		TenantID:     "tenant-1",
		MovementType: MovementTypeIn,
		Quantity:     decimal.NewFromInt(1),
		UnitCost:     decimal.Zero,
		TotalCost:    decimal.Zero,
	}}, "tenant-1", decimal.NewFromInt(1))

	assert.True(t, got.Equal(decimal.NewFromInt(11)))
}

func TestInventoryWave9ImportBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("skips blank existing product codes", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Products["existing"] = &Product{ID: "existing", TenantID: "tenant-1", Code: " ", Name: "Blank code"}
		result, err := NewServiceWithRepository(repo).ImportProductsCSV(ctx, "tenant-1", "tenant_demo", &ImportProductsRequest{
			CSVContent: "code,name,sales_price\nSKU-1,Widget,10\n",
		})
		require.NoError(t, err)
		assert.Equal(t, 1, result.ProductsCreated)
	})

	t.Run("records blank generated code as row error", func(t *testing.T) {
		repo := &inventoryWave9BlankCodeRepo{MockRepository: NewMockRepository()}
		result, err := NewServiceWithRepository(repo).ImportProductsCSV(ctx, "tenant-1", "tenant_demo", &ImportProductsRequest{
			CSVContent: "name,sales_price\nWidget,10\n",
		})
		require.NoError(t, err)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "code is required")
	})

	t.Run("parses supported uppercase product types through validation", func(t *testing.T) {
		product, err := buildProductFromImportRow(productImportRow{values: map[string]string{
			"name":         "Consulting",
			"sales_price":  "100",
			"product_type": "SERVICE",
		}}, "tenant-1", nil, nil, nil, contactrefs.NewSupplierLookup(nil))
		require.NoError(t, err)
		assert.Equal(t, ProductTypeService, product.ProductType)
	})

	t.Run("returns product validation errors after parsing", func(t *testing.T) {
		_, err := buildProductFromImportRow(productImportRow{values: map[string]string{
			"name":        "Broken",
			"sales_price": "-1",
		}}, "tenant-1", nil, nil, nil, contactrefs.NewSupplierLookup(nil))
		require.ErrorContains(t, err, "sales_price cannot be negative")
	})

	t.Run("invalid CSV header is reported", func(t *testing.T) {
		_, err := parseProductImportRows(`"unterminated`)
		require.ErrorContains(t, err, "parse csv header")
	})
}

type inventoryWave9SecondStockLevelErrorRepo struct {
	*inventoryWave9StockLevelsRepo
}

func (r *inventoryWave9SecondStockLevelErrorRepo) GetStockLevelsByProduct(ctx context.Context, schemaName, tenantID, productID string) ([]StockLevel, error) {
	r.calls++
	if r.calls == 2 {
		return nil, errors.New("stock levels unavailable")
	}
	return append([]StockLevel(nil), r.levels...), nil
}

type inventoryWave9BlankCodeRepo struct {
	*MockRepository
}

func (r *inventoryWave9BlankCodeRepo) GenerateCode(context.Context, string, string) (string, error) {
	return " ", nil
}

func inventoryWave9StockFixture() *MockRepository {
	repo := NewMockRepository()
	repo.Products[inventoryStockProductID] = &Product{
		ID:             inventoryStockProductID,
		TenantID:       "tenant-1",
		Code:           "PRD",
		Name:           "Widget",
		ProductType:    ProductTypeGoods,
		TrackInventory: true,
		PurchasePrice:  decimal.NewFromInt(3),
		CurrentStock:   decimal.NewFromInt(5),
	}
	repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Name: "Main"}
	repo.Warehouses[inventoryStockWarehouseID2] = &Warehouse{ID: inventoryStockWarehouseID2, TenantID: "tenant-1", Name: "Overflow"}
	repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(5),
		AvailableQty: decimal.NewFromInt(5),
	}
	repo.Movements[inventoryStockProductID] = []InventoryMovement{{
		TenantID:     "tenant-1",
		ProductID:    inventoryStockProductID,
		WarehouseID:  inventoryStockWarehouseID,
		MovementType: MovementTypeIn,
		Quantity:     decimal.NewFromInt(5),
		UnitCost:     decimal.NewFromInt(3),
		TotalCost:    decimal.NewFromInt(15),
	}}
	return repo
}

func inventoryWave9IssueRequest(postToLedger bool) *IssueStockRequest {
	return &IssueStockRequest{
		ProductID:                inventoryStockProductID,
		WarehouseID:              inventoryStockWarehouseID,
		Quantity:                 "1",
		CostingMethod:            InventoryIssueCostingMethodStandardCost,
		PostToLedger:             postToLedger,
		UserID:                   "user-1",
		CostOfGoodsSoldAccountID: "11111111-1111-4111-8111-111111111111",
		InventoryAccountID:       "22222222-2222-4222-8222-222222222222",
		Reference:                strings.TrimSpace(""),
		SourceType:               strings.TrimSpace(""),
	}
}

func inventoryWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
