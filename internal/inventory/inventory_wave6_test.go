package inventory

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave6NilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	repo := NewGORMRepository(nil)

	err := repo.CreateProduct(ctx, "tenant_inventory", &Product{ID: inventoryStockProductID})
	require.ErrorContains(t, err, "inventory repository database is not configured")

	transactioner, ok := repo.(inventoryLedgerTransactioner)
	require.True(t, ok)
	err = transactioner.WithInventoryLedgerTransaction(ctx, nil, func(repo Repository, ledger accountingPoster) error {
		t.Fatal("transaction callback should not run without a database")
		return nil
	})
	require.ErrorContains(t, err, "inventory repository database is not configured")
}

func TestGORMRepositoryWave6DryRunListConversions(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	categoryID := "33333333-3333-4333-8333-333333333333"
	nextCode := 0
	toWarehouseID := inventoryStockWarehouseID2
	repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunFixtures(inventoryDryRunFixtures{nextCodeNum: &nextCode}),
		withInventoryWave6ScanRows(
			inventoryWave6RowSet{
				columns: []string{"next_num"},
				values:  [][]driver.Value{{int64(0)}},
			},
			inventoryWave6RowSet{
				columns: []string{"id", "tenant_id", "code", "name", "description", "product_type", "category_id", "unit", "purchase_price", "sales_price", "vat_rate", "min_stock_level", "current_stock", "reorder_point", "sale_account_id", "purchase_account_id", "inventory_account_id", "track_inventory", "is_active", "barcode", "supplier_id", "lead_time_days", "created_at", "updated_at"},
				values: [][]driver.Value{{
					inventoryStockProductID, tenantID, "PRD-1", "Tracked widget", "", string(ProductTypeGoods), categoryID, "pcs",
					"7", "10", "0", "0", "3", "4", nil, nil, nil, true, true, "", nil, int64(0), now, now,
				}},
			},
			inventoryWave6RowSet{
				columns: []string{"id", "tenant_id", "name", "description", "parent_id", "created_at", "updated_at"},
				values:  [][]driver.Value{{categoryID, tenantID, "Hardware", "", nil, now, now}},
			},
			inventoryWave6RowSet{
				columns: []string{"id", "tenant_id", "code", "name", "address", "is_default", "is_active", "created_at", "updated_at"},
				values:  [][]driver.Value{{inventoryStockWarehouseID, tenantID, "MAIN", "Main", "", false, true, now, now}},
			},
			inventoryWave6RowSet{
				columns: []string{"id", "tenant_id", "product_id", "warehouse_id", "quantity", "reserved_qty", "available_qty", "last_updated"},
				values:  [][]driver.Value{{"stock-1", tenantID, inventoryStockProductID, inventoryStockWarehouseID, "3", "0", "3", now}},
			},
			inventoryWave6RowSet{
				columns: []string{"id", "tenant_id", "product_id", "warehouse_id", "lot_number", "serial_number", "expiry_date", "quantity", "reason", "created_at", "updated_at", "created_by"},
				values:  [][]driver.Value{{"reservation-1", tenantID, inventoryStockProductID, inventoryStockWarehouseID, "LOT-A", "", "", "1", "", now, now, ""}},
			},
			inventoryWave6RowSet{
				columns: []string{"id", "tenant_id", "product_id", "warehouse_id", "movement_type", "quantity", "unit_cost", "total_cost", "lot_number", "serial_number", "expiry_date", "reference", "source_type", "source_id", "to_warehouse_id", "notes", "movement_date", "created_at", "created_by"},
				values: [][]driver.Value{{
					"movement-1", tenantID, inventoryStockProductID, inventoryStockWarehouseID, string(MovementTypeTransfer), "2", "7", "14", "", "", "", "", "", "", toWarehouseID, "", now, now, "",
				}},
			},
		))}

	code, err := repo.GenerateCode(ctx, "tenant_inventory", tenantID)
	require.NoError(t, err)
	assert.Equal(t, "PRD-00001", code)

	products, err := repo.ListProducts(ctx, "tenant_inventory", tenantID, &ProductFilter{
		ProductType: ProductTypeGoods,
		Status:      ProductStatusActive,
		CategoryID:  categoryID,
		Search:      "widget",
		LowStock:    true,
	})
	require.NoError(t, err)
	require.Len(t, products, 1)
	assert.Equal(t, categoryID, products[0].CategoryID)

	categories, err := repo.ListCategories(ctx, "tenant_inventory", tenantID)
	require.NoError(t, err)
	require.Len(t, categories, 1)
	assert.Equal(t, "Hardware", categories[0].Name)

	warehouses, err := repo.ListWarehouses(ctx, "tenant_inventory", tenantID, true)
	require.NoError(t, err)
	require.Len(t, warehouses, 1)
	assert.Equal(t, "MAIN", warehouses[0].Code)

	levels, err := repo.GetStockLevelsByProduct(ctx, "tenant_inventory", tenantID, inventoryStockProductID)
	require.NoError(t, err)
	require.Len(t, levels, 1)

	reservations, err := repo.ListLotReservations(ctx, "tenant_inventory", tenantID, inventoryStockProductID, inventoryStockWarehouseID)
	require.NoError(t, err)
	require.Len(t, reservations, 1)

	movements, err := repo.ListMovements(ctx, "tenant_inventory", tenantID, inventoryStockProductID)
	require.NoError(t, err)
	require.Len(t, movements, 1)
	assert.Equal(t, inventoryStockWarehouseID2, movements[0].ToWarehouseID)
}

func TestGORMRepositoryWave6ReleaseLotReservationFallback(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-4111-8111-111111111111"
	repo := &GORMRepository{db: newInventoryDryRunDB(t,
		withInventoryDryRunUpdateRows(1),
		withInventoryWave6ScanRows(inventoryWave6RowSet{
			columns: []string{"id", "tenant_id", "product_id", "warehouse_id", "lot_number", "serial_number", "expiry_date", "quantity", "reason", "created_at", "updated_at", "created_by"},
			values:  [][]driver.Value{},
		}),
	)}

	reservation, err := repo.ReleaseLotReservation(ctx, "tenant_inventory", tenantID, inventoryStockProductID, inventoryStockWarehouseID, " LOT-A ", " SN-1 ", " 2026-12-31 ", decimal.NewFromInt(1), "released", "user-1")

	require.NoError(t, err)
	require.NotNil(t, reservation)
	assert.Equal(t, "LOT-A", reservation.LotNumber)
	assert.Equal(t, "SN-1", reservation.SerialNumber)
	assert.Equal(t, "2026-12-31", reservation.ExpiryDate)
	assert.True(t, reservation.Quantity.IsZero())
}

func TestInventoryWave6UpdateProductValidationBranch(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.Products[inventoryStockProductID] = &Product{
		ID:          inventoryStockProductID,
		TenantID:    "tenant-1",
		Code:        "PRD-1",
		Name:        "Widget",
		ProductType: ProductTypeGoods,
		SalesPrice:  decimal.NewFromInt(10),
		IsActive:    true,
	}
	service := NewServiceWithRepository(repo)

	updated, err := service.UpdateProduct(ctx, "tenant-1", "tenant_inventory", inventoryStockProductID, &UpdateProductRequest{Name: ""})

	assert.Nil(t, updated)
	require.ErrorContains(t, err, "validation failed")
	assert.ErrorContains(t, err, "product name is required")
}

func TestInventoryWave6ValuationAndLotHelpers(t *testing.T) {
	product := Product{ID: inventoryStockProductID, Code: "PRD", Name: "Widget", PurchasePrice: decimal.NewFromInt(9)}
	otherTenantMovement := InventoryMovement{
		TenantID:     "tenant-2",
		ProductID:    product.ID,
		WarehouseID:  inventoryStockWarehouseID,
		MovementType: MovementTypeIn,
		Quantity:     decimal.NewFromInt(5),
		UnitCost:     decimal.NewFromInt(3),
	}
	zeroCostMovement := InventoryMovement{
		TenantID:     "tenant-1",
		ProductID:    product.ID,
		WarehouseID:  inventoryStockWarehouseID,
		MovementType: MovementTypeIn,
		Quantity:     decimal.NewFromInt(5),
		UnitCost:     decimal.Zero,
		TotalCost:    decimal.Zero,
	}

	assert.True(t, weightedAverageInventoryUnitCost(product, []InventoryMovement{otherTenantMovement, zeroCostMovement}, "tenant-1").Equal(decimal.NewFromInt(9)))
	assert.True(t, fifoInventoryUnitCost(product, []InventoryMovement{zeroCostMovement}, "tenant-1", decimal.NewFromInt(2)).Equal(decimal.NewFromInt(9)))

	positions := map[inventoryLotKey]*inventoryLotAccumulator{}
	addInventoryLotReportQuantity(positions, product, nil, InventoryMovement{TenantID: "tenant-1"}, "", decimal.Zero)
	assert.Empty(t, positions)

	addInventoryLotReportQuantity(positions, product, nil, InventoryMovement{
		TenantID:    "tenant-1",
		ProductID:   product.ID,
		WarehouseID: inventoryStockWarehouseID2,
		LotNumber:   "LOT-B",
	}, inventoryStockWarehouseID, decimal.NewFromInt(1))
	assert.Empty(t, positions)

	addInventoryLotReportQuantity(positions, product, nil, InventoryMovement{
		TenantID:     "tenant-1",
		ProductID:    product.ID,
		MovementType: MovementTypeIn,
		LotNumber:    "LOT-A",
	}, "", decimal.NewFromInt(2))
	require.Len(t, positions, 1)
	for _, position := range positions {
		assert.Equal(t, "UNASSIGNED", position.line.WarehouseCode)
	}

	tenantPositions := inventoryLotPositionsFromMovements(product, []InventoryMovement{otherTenantMovement}, "tenant-1", inventoryStockWarehouseID)
	assert.Empty(t, tenantPositions)

	keys := sortedInventoryLotKeys(map[inventoryLotKey]*inventoryLotAccumulator{
		{productID: product.ID, warehouseID: inventoryStockWarehouseID}: nil,
		{productID: product.ID, warehouseID: inventoryStockWarehouseID, lotNumber: "LOT-Z"}: {
			line: InventoryLotLine{Quantity: decimal.Zero},
		},
		{productID: product.ID, warehouseID: inventoryStockWarehouseID, lotNumber: "LOT-A"}: {
			line: InventoryLotLine{Quantity: decimal.NewFromInt(1)},
		},
	})
	require.Len(t, keys, 1)
	assert.Equal(t, "LOT-A", keys[0].lotNumber)
}

func TestInventoryWave6TransferStockLateErrors(t *testing.T) {
	ctx := context.Background()
	newRepo := func() *inventoryWave6Repository {
		repo := &inventoryWave6Repository{MockRepository: NewMockRepository()}
		repo.Products[inventoryStockProductID] = &Product{
			ID:            inventoryStockProductID,
			TenantID:      "tenant-1",
			Code:          "PRD-1",
			Name:          "Widget",
			ProductType:   ProductTypeGoods,
			PurchasePrice: decimal.NewFromInt(5),
			IsActive:      true,
		}
		repo.Warehouses[inventoryStockWarehouseID] = &Warehouse{ID: inventoryStockWarehouseID, TenantID: "tenant-1", Code: "FROM", Name: "From"}
		repo.Warehouses[inventoryStockWarehouseID2] = &Warehouse{ID: inventoryStockWarehouseID2, TenantID: "tenant-1", Code: "TO", Name: "To"}
		repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID)] = &StockLevel{
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID,
			Quantity:     decimal.NewFromInt(5),
			AvailableQty: decimal.NewFromInt(5),
		}
		repo.StockLevels[inventoryStockLevelKey(inventoryStockProductID, inventoryStockWarehouseID2)] = &StockLevel{
			TenantID:     "tenant-1",
			ProductID:    inventoryStockProductID,
			WarehouseID:  inventoryStockWarehouseID2,
			Quantity:     decimal.Zero,
			AvailableQty: decimal.Zero,
		}
		return repo
	}
	req := func() *TransferStockRequest {
		return &TransferStockRequest{
			ProductID:       inventoryStockProductID,
			FromWarehouseID: inventoryStockWarehouseID,
			ToWarehouseID:   inventoryStockWarehouseID2,
			Quantity:        "1",
			UserID:          "user-1",
		}
	}

	repo := newRepo()
	repo.failWarehouseID = inventoryStockWarehouseID2
	err := NewServiceWithRepository(repo).TransferStock(ctx, "tenant-1", "tenant_inventory", req())
	require.ErrorContains(t, err, "get destination warehouse")

	repo = newRepo()
	repo.createMovementErrOnCall = 2
	err = NewServiceWithRepository(repo).TransferStock(ctx, "tenant-1", "tenant_inventory", req())
	require.ErrorContains(t, err, "create in movement")

	repo = newRepo()
	repo.upsertErrOnCall = 2
	err = NewServiceWithRepository(repo).TransferStock(ctx, "tenant-1", "tenant_inventory", req())
	require.ErrorContains(t, err, "update destination stock level")
}

type inventoryWave6Repository struct {
	*MockRepository
	failWarehouseID         string
	createMovementErrOnCall int
	createMovementCalls     int
	upsertErrOnCall         int
	upsertCalls             int
}

func (r *inventoryWave6Repository) GetWarehouseByID(ctx context.Context, schemaName, tenantID, warehouseID string) (*Warehouse, error) {
	if warehouseID == r.failWarehouseID {
		return nil, fmt.Errorf("mock destination warehouse error")
	}
	return r.MockRepository.GetWarehouseByID(ctx, schemaName, tenantID, warehouseID)
}

func (r *inventoryWave6Repository) CreateMovement(ctx context.Context, schemaName string, movement *InventoryMovement) error {
	r.createMovementCalls++
	if r.createMovementErrOnCall == r.createMovementCalls {
		return fmt.Errorf("mock create movement error")
	}
	return r.MockRepository.CreateMovement(ctx, schemaName, movement)
}

func (r *inventoryWave6Repository) UpsertStockLevel(ctx context.Context, schemaName string, level *StockLevel) error {
	r.upsertCalls++
	if r.upsertErrOnCall == r.upsertCalls {
		return fmt.Errorf("mock upsert stock level error")
	}
	return r.MockRepository.UpsertStockLevel(ctx, schemaName, level)
}

type inventoryWave6RowSet struct {
	columns []string
	values  [][]driver.Value
}

var inventoryWave6RowsDSNID uint64
var inventoryWave6RowsDriverOnce sync.Once
var inventoryWave6RowsMu sync.Mutex
var inventoryWave6RowsByDSN = map[string]inventoryWave6RowSet{}

func withInventoryWave6ScanRows(rowSets ...inventoryWave6RowSet) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(inventoryDryRunCallbackName(t, "scan_rows_wave6"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing inventory dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newInventoryWave6SQLRows(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func newInventoryWave6SQLRows(t *testing.T, rowSet inventoryWave6RowSet) *sql.Rows {
	t.Helper()

	inventoryWave6RowsDriverOnce.Do(func() {
		sql.Register("inventory_wave6_rows", inventoryWave6RowsDriver{})
	})

	dsn := fmt.Sprintf("inventory-wave6-rows-%d", atomic.AddUint64(&inventoryWave6RowsDSNID, 1))
	inventoryWave6RowsMu.Lock()
	inventoryWave6RowsByDSN[dsn] = rowSet
	inventoryWave6RowsMu.Unlock()

	db, err := sql.Open("inventory_wave6_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		inventoryWave6RowsMu.Lock()
		delete(inventoryWave6RowsByDSN, dsn)
		inventoryWave6RowsMu.Unlock()
	})

	return rows
}

type inventoryWave6RowsDriver struct{}

func (inventoryWave6RowsDriver) Open(name string) (driver.Conn, error) {
	return inventoryWave6RowsConn{dsn: name}, nil
}

type inventoryWave6RowsConn struct {
	dsn string
}

func (inventoryWave6RowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("inventory wave6 rows do not prepare statements")
}

func (inventoryWave6RowsConn) Close() error {
	return nil
}

func (inventoryWave6RowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("inventory wave6 rows do not begin transactions")
}

func (c inventoryWave6RowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	inventoryWave6RowsMu.Lock()
	rowSet, ok := inventoryWave6RowsByDSN[c.dsn]
	inventoryWave6RowsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("inventory wave6 row set %q not found", c.dsn)
	}
	return &inventoryWave6SQLRows{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type inventoryWave6SQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *inventoryWave6SQLRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*inventoryWave6SQLRows) Close() error {
	return nil
}

func (r *inventoryWave6SQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
