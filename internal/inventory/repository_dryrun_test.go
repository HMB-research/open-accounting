package inventory

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type inventoryDryRunConnPool struct{}

func (inventoryDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run inventory tests should not prepare statements")
}

func (inventoryDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run inventory tests should not execute statements")
}

func (inventoryDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run inventory tests should not query rows")
}

func (inventoryDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (inventoryDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &inventoryDryRunTx{}, nil
}

type inventoryDryRunTx struct {
	inventoryDryRunConnPool
}

func (*inventoryDryRunTx) Commit() error {
	return nil
}

func (*inventoryDryRunTx) Rollback() error {
	return nil
}

type inventoryDryRunDBOption func(t *testing.T, db *gorm.DB)

type inventoryDryRunFixtures struct {
	product      *productRow
	products     []productRow
	category     *productCategoryRow
	categories   []productCategoryRow
	warehouse    *Warehouse
	warehouses   []Warehouse
	stockLevel   *StockLevel
	stockLevels  []StockLevel
	reservations []InventoryLotReservation
	movements    []inventoryMovementRow
	nextCodeNum  *int
}

type inventoryDryRunSQLCapture struct {
	statements []string
}

func newInventoryDryRunDB(t *testing.T, opts ...inventoryDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: inventoryDryRunConnPool{}}), &gorm.Config{
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

func withInventoryDryRunFixtures(fixtures inventoryDryRunFixtures) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(inventoryDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *productRow:
				if fixtures.product != nil {
					*dest = *fixtures.product
					tx.RowsAffected = 1
				}
			case *[]productRow:
				if fixtures.products != nil {
					*dest = append([]productRow(nil), fixtures.products...)
					tx.RowsAffected = int64(len(fixtures.products))
				}
			case *productCategoryRow:
				if fixtures.category != nil {
					*dest = *fixtures.category
					tx.RowsAffected = 1
				}
			case *[]productCategoryRow:
				if fixtures.categories != nil {
					*dest = append([]productCategoryRow(nil), fixtures.categories...)
					tx.RowsAffected = int64(len(fixtures.categories))
				}
			case *Warehouse:
				if fixtures.warehouse != nil {
					*dest = *fixtures.warehouse
					tx.RowsAffected = 1
				}
			case *[]Warehouse:
				if fixtures.warehouses != nil {
					*dest = append([]Warehouse(nil), fixtures.warehouses...)
					tx.RowsAffected = int64(len(fixtures.warehouses))
				}
			case *StockLevel:
				if fixtures.stockLevel != nil {
					*dest = *fixtures.stockLevel
					tx.RowsAffected = 1
				}
			case *[]StockLevel:
				if fixtures.stockLevels != nil {
					*dest = append([]StockLevel(nil), fixtures.stockLevels...)
					tx.RowsAffected = int64(len(fixtures.stockLevels))
				}
			case *[]InventoryLotReservation:
				if fixtures.reservations != nil {
					*dest = append([]InventoryLotReservation(nil), fixtures.reservations...)
					tx.RowsAffected = int64(len(fixtures.reservations))
				}
			case *[]inventoryMovementRow:
				if fixtures.movements != nil {
					*dest = append([]inventoryMovementRow(nil), fixtures.movements...)
					tx.RowsAffected = int64(len(fixtures.movements))
				}
			default:
				if fixtures.nextCodeNum != nil && setInventoryDryRunNextNum(dest, *fixtures.nextCodeNum) {
					tx.RowsAffected = 1
				}
			}
		})
		require.NoError(t, err)
	}
}

func setInventoryDryRunNextNum(dest interface{}, nextNum int) bool {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}
	elem := value.Elem()
	if elem.Kind() != reflect.Struct {
		return false
	}
	field := elem.FieldByName("NextNum")
	if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.Int {
		return false
	}
	field.SetInt(int64(nextNum))
	return true
}

func withInventoryDryRunSQLCapture(capture *inventoryDryRunSQLCapture) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(inventoryDryRunCallbackName(t, "capture_create"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Query().After("gorm:query").Register(inventoryDryRunCallbackName(t, "capture_query"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Update().After("gorm:update").Register(inventoryDryRunCallbackName(t, "capture_update"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Delete().After("gorm:delete").Register(inventoryDryRunCallbackName(t, "capture_delete"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Row().After("gorm:row").Register(inventoryDryRunCallbackName(t, "capture_row"), capture.capture)
		require.NoError(t, err)
	}
}

func withInventoryDryRunCreateError(expectedErr error) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(inventoryDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withInventoryDryRunQueryError(expectedErr error) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(inventoryDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withInventoryDryRunUpdateRows(rows ...int64) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(inventoryDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
			rowCount := int64(0)
			if len(rows) > 0 {
				rowCount = rows[len(rows)-1]
				if index < len(rows) {
					rowCount = rows[index]
				}
				index++
			}
			tx.RowsAffected = rowCount
		})
		require.NoError(t, err)
	}
}

func withInventoryDryRunUpdateError(expectedErr error) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(inventoryDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withInventoryDryRunDeleteRows(rows ...int64) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(inventoryDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
			rowCount := int64(0)
			if len(rows) > 0 {
				rowCount = rows[len(rows)-1]
				if index < len(rows) {
					rowCount = rows[index]
				}
				index++
			}
			tx.RowsAffected = rowCount
		})
		require.NoError(t, err)
	}
}

func withInventoryDryRunDeleteError(expectedErr error) inventoryDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(inventoryDryRunCallbackName(t, "delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func inventoryDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "inventory_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func (c *inventoryDryRunSQLCapture) capture(tx *gorm.DB) {
	if sql := strings.TrimSpace(tx.Statement.SQL.String()); sql != "" {
		c.statements = append(c.statements, sql)
	}
}

func (c *inventoryDryRunSQLCapture) assertContains(t *testing.T, expected string) {
	t.Helper()
	for _, statement := range c.statements {
		if strings.Contains(statement, expected) {
			return
		}
	}
	t.Fatalf("expected dry-run SQL to contain %q in %#v", expected, c.statements)
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_inventory"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	product := inventoryDryRunProduct(tenantID, now)
	category := inventoryDryRunCategory(tenantID, now)
	warehouse := inventoryDryRunWarehouse(tenantID, now)
	level := inventoryDryRunStockLevel(tenantID, product.ID, warehouse.ID, now)
	reservation := inventoryDryRunLotReservation(tenantID, product.ID, warehouse.ID, now)
	movement := inventoryDryRunMovement(tenantID, product.ID, warehouse.ID, now)
	capture := &inventoryDryRunSQLCapture{}
	repo := &GORMRepository{db: newInventoryDryRunDB(t,
		withInventoryDryRunFixtures(inventoryDryRunFixtures{
			product:      inventoryDryRunProductRow(product),
			products:     []productRow{*inventoryDryRunProductRow(product)},
			category:     inventoryDryRunProductCategoryRow(category),
			categories:   []productCategoryRow{*inventoryDryRunProductCategoryRow(category)},
			warehouse:    warehouse,
			warehouses:   []Warehouse{*warehouse},
			stockLevel:   level,
			stockLevels:  []StockLevel{*level},
			reservations: []InventoryLotReservation{*reservation},
			movements:    []inventoryMovementRow{*inventoryDryRunMovementRow(movement)},
		}),
		withInventoryDryRunUpdateRows(1, 1, 1),
		withInventoryDryRunDeleteRows(1, 1, 1),
		withInventoryDryRunSQLCapture(capture),
	)}

	require.NoError(t, repo.CreateProduct(ctx, schemaName, product))

	gotProduct, err := repo.GetProductByID(ctx, schemaName, tenantID, product.ID)
	require.NoError(t, err)
	assert.Equal(t, product.ID, gotProduct.ID)
	assert.Equal(t, product.SalesPrice.String(), gotProduct.SalesPrice.String())

	products, err := repo.ListProducts(ctx, schemaName, tenantID, &ProductFilter{
		ProductType: ProductTypeGoods,
		Status:      ProductStatusActive,
		CategoryID:  category.ID,
		Search:      " widget ",
		LowStock:    true,
	})
	assert.Nil(t, products)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")

	unfilteredProducts, err := repo.ListProducts(ctx, schemaName, tenantID, nil)
	assert.Nil(t, unfilteredProducts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")

	product.Name = "Updated widget"
	require.NoError(t, repo.UpdateProduct(ctx, schemaName, product))
	require.NoError(t, repo.DeleteProduct(ctx, schemaName, tenantID, product.ID))

	code, err := repo.GenerateCode(ctx, schemaName, tenantID)
	assert.Empty(t, code)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")

	require.NoError(t, repo.CreateCategory(ctx, schemaName, category))
	gotCategory, err := repo.GetCategoryByID(ctx, schemaName, tenantID, category.ID)
	require.NoError(t, err)
	assert.Equal(t, category.Name, gotCategory.Name)

	categories, err := repo.ListCategories(ctx, schemaName, tenantID)
	assert.Nil(t, categories)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")
	require.NoError(t, repo.DeleteCategory(ctx, schemaName, tenantID, category.ID))

	require.NoError(t, repo.CreateWarehouse(ctx, schemaName, warehouse))
	gotWarehouse, err := repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouse.ID)
	require.NoError(t, err)
	assert.Equal(t, warehouse.Code, gotWarehouse.Code)

	activeWarehouses, err := repo.ListWarehouses(ctx, schemaName, tenantID, true)
	assert.Nil(t, activeWarehouses)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")

	allWarehouses, err := repo.ListWarehouses(ctx, schemaName, tenantID, false)
	assert.Nil(t, allWarehouses)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")

	warehouse.Name = "Updated warehouse"
	require.NoError(t, repo.UpdateWarehouse(ctx, schemaName, warehouse))
	require.NoError(t, repo.DeleteWarehouse(ctx, schemaName, tenantID, warehouse.ID))

	gotLevel, err := repo.GetStockLevel(ctx, schemaName, tenantID, product.ID, warehouse.ID)
	require.NoError(t, err)
	assert.True(t, level.AvailableQty.Equal(gotLevel.AvailableQty))

	levels, err := repo.GetStockLevelsByProduct(ctx, schemaName, tenantID, product.ID)
	assert.Nil(t, levels)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")
	require.NoError(t, repo.UpsertStockLevel(ctx, schemaName, level))

	reservations, err := repo.ListLotReservations(ctx, schemaName, tenantID, product.ID, warehouse.ID)
	assert.Nil(t, reservations)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")
	require.NoError(t, repo.UpsertLotReservation(ctx, schemaName, reservation))

	releasedReservation, err := repo.ReleaseLotReservation(ctx, schemaName, tenantID, product.ID, warehouse.ID, reservation.LotNumber, reservation.SerialNumber, reservation.ExpiryDate, decimal.NewFromInt(1), "release", "user-2")
	assert.Nil(t, releasedReservation)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")

	require.NoError(t, repo.CreateMovement(ctx, schemaName, movement))
	movements, err := repo.ListMovements(ctx, schemaName, tenantID, product.ID)
	assert.Nil(t, movements)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dry run mode unsupported")

	require.NoError(t, repo.UpdateProductStock(ctx, schemaName, tenantID, product.ID, decimal.NewFromInt(12)))

	require.NoError(t, repo.WithInventoryLedgerTransaction(ctx, nil, func(txRepo Repository, ledger accountingPoster) error {
		require.NotNil(t, txRepo)
		require.NotNil(t, ledger)
		return nil
	}))

	capture.assertContains(t, `"tenant_inventory"."products"`)
	capture.assertContains(t, `"tenant_inventory"."product_categories"`)
	capture.assertContains(t, `"tenant_inventory"."warehouses"`)
	capture.assertContains(t, `"tenant_inventory"."stock_levels"`)
	capture.assertContains(t, `"tenant_inventory"."inventory_lot_reservations"`)
	capture.assertContains(t, `"tenant_inventory"."inventory_movements"`)
	capture.assertContains(t, `name ILIKE`)
	capture.assertContains(t, `current_stock <= reorder_point`)
	capture.assertContains(t, `ORDER BY name ASC`)
	capture.assertContains(t, `movement_date DESC`)
	capture.assertContains(t, `quantity -`)
}

func TestGORMRepositoryDryRunInvalidSchema(t *testing.T) {
	ctx := context.Background()
	invalidSchema := "tenant-inventory"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	product := inventoryDryRunProduct(tenantID, now)
	category := inventoryDryRunCategory(tenantID, now)
	warehouse := inventoryDryRunWarehouse(tenantID, now)
	level := inventoryDryRunStockLevel(tenantID, product.ID, warehouse.ID, now)
	reservation := inventoryDryRunLotReservation(tenantID, product.ID, warehouse.ID, now)
	movement := inventoryDryRunMovement(tenantID, product.ID, warehouse.ID, now)
	repo := &GORMRepository{db: newInventoryDryRunDB(t)}

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "CreateProduct",
			run: func(t *testing.T) error {
				return repo.CreateProduct(ctx, invalidSchema, product)
			},
		},
		{
			name: "GetProductByID",
			run: func(t *testing.T) error {
				got, err := repo.GetProductByID(ctx, invalidSchema, tenantID, product.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "ListProducts",
			run: func(t *testing.T) error {
				got, err := repo.ListProducts(ctx, invalidSchema, tenantID, &ProductFilter{ProductType: ProductTypeGoods})
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpdateProduct",
			run: func(t *testing.T) error {
				return repo.UpdateProduct(ctx, invalidSchema, product)
			},
		},
		{
			name: "DeleteProduct",
			run: func(t *testing.T) error {
				return repo.DeleteProduct(ctx, invalidSchema, tenantID, product.ID)
			},
		},
		{
			name: "GenerateCode",
			run: func(t *testing.T) error {
				got, err := repo.GenerateCode(ctx, invalidSchema, tenantID)
				assert.Empty(t, got)
				return err
			},
		},
		{
			name: "CreateCategory",
			run: func(t *testing.T) error {
				return repo.CreateCategory(ctx, invalidSchema, category)
			},
		},
		{
			name: "GetCategoryByID",
			run: func(t *testing.T) error {
				got, err := repo.GetCategoryByID(ctx, invalidSchema, tenantID, category.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "ListCategories",
			run: func(t *testing.T) error {
				got, err := repo.ListCategories(ctx, invalidSchema, tenantID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "DeleteCategory",
			run: func(t *testing.T) error {
				return repo.DeleteCategory(ctx, invalidSchema, tenantID, category.ID)
			},
		},
		{
			name: "CreateWarehouse",
			run: func(t *testing.T) error {
				return repo.CreateWarehouse(ctx, invalidSchema, warehouse)
			},
		},
		{
			name: "GetWarehouseByID",
			run: func(t *testing.T) error {
				got, err := repo.GetWarehouseByID(ctx, invalidSchema, tenantID, warehouse.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "ListWarehouses",
			run: func(t *testing.T) error {
				got, err := repo.ListWarehouses(ctx, invalidSchema, tenantID, true)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpdateWarehouse",
			run: func(t *testing.T) error {
				return repo.UpdateWarehouse(ctx, invalidSchema, warehouse)
			},
		},
		{
			name: "DeleteWarehouse",
			run: func(t *testing.T) error {
				return repo.DeleteWarehouse(ctx, invalidSchema, tenantID, warehouse.ID)
			},
		},
		{
			name: "GetStockLevel",
			run: func(t *testing.T) error {
				got, err := repo.GetStockLevel(ctx, invalidSchema, tenantID, product.ID, warehouse.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "GetStockLevelsByProduct",
			run: func(t *testing.T) error {
				got, err := repo.GetStockLevelsByProduct(ctx, invalidSchema, tenantID, product.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpsertStockLevel",
			run: func(t *testing.T) error {
				return repo.UpsertStockLevel(ctx, invalidSchema, level)
			},
		},
		{
			name: "ListLotReservations",
			run: func(t *testing.T) error {
				got, err := repo.ListLotReservations(ctx, invalidSchema, tenantID, product.ID, warehouse.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpsertLotReservation",
			run: func(t *testing.T) error {
				return repo.UpsertLotReservation(ctx, invalidSchema, reservation)
			},
		},
		{
			name: "ReleaseLotReservation",
			run: func(t *testing.T) error {
				got, err := repo.ReleaseLotReservation(ctx, invalidSchema, tenantID, product.ID, warehouse.ID, reservation.LotNumber, reservation.SerialNumber, reservation.ExpiryDate, decimal.NewFromInt(1), "release", "user-1")
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "CreateMovement",
			run: func(t *testing.T) error {
				return repo.CreateMovement(ctx, invalidSchema, movement)
			},
		},
		{
			name: "ListMovements",
			run: func(t *testing.T) error {
				got, err := repo.ListMovements(ctx, invalidSchema, tenantID, product.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpdateProductStock",
			run: func(t *testing.T) error {
				return repo.UpdateProductStock(ctx, invalidSchema, tenantID, product.ID, decimal.NewFromInt(12))
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid SQL identifier")
		})
	}
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_inventory"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 11, 0, 0, 0, time.UTC)
	product := inventoryDryRunProduct(tenantID, now)
	category := inventoryDryRunCategory(tenantID, now)
	warehouse := inventoryDryRunWarehouse(tenantID, now)
	level := inventoryDryRunStockLevel(tenantID, product.ID, warehouse.ID, now)
	reservation := inventoryDryRunLotReservation(tenantID, product.ID, warehouse.ID, now)
	movement := inventoryDryRunMovement(tenantID, product.ID, warehouse.ID, now)
	dbErr := errors.New("inventory database failed")

	t.Run("not found query errors", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunQueryError(gorm.ErrRecordNotFound))}

		gotProduct, err := repo.GetProductByID(ctx, schemaName, tenantID, product.ID)
		assert.Nil(t, gotProduct)
		require.EqualError(t, err, "product not found")

		gotCategory, err := repo.GetCategoryByID(ctx, schemaName, tenantID, category.ID)
		assert.Nil(t, gotCategory)
		require.EqualError(t, err, "category not found")

		gotWarehouse, err := repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouse.ID)
		assert.Nil(t, gotWarehouse)
		require.EqualError(t, err, "warehouse not found")

		gotLevel, err := repo.GetStockLevel(ctx, schemaName, tenantID, product.ID, warehouse.ID)
		assert.Nil(t, gotLevel)
		require.EqualError(t, err, "stock level not found")
	})

	t.Run("query errors", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunQueryError(dbErr))}

		gotProduct, err := repo.GetProductByID(ctx, schemaName, tenantID, product.ID)
		assert.Nil(t, gotProduct)
		assert.ErrorIs(t, err, dbErr)

		gotCategory, err := repo.GetCategoryByID(ctx, schemaName, tenantID, category.ID)
		assert.Nil(t, gotCategory)
		assert.ErrorIs(t, err, dbErr)

		gotWarehouse, err := repo.GetWarehouseByID(ctx, schemaName, tenantID, warehouse.ID)
		assert.Nil(t, gotWarehouse)
		assert.ErrorIs(t, err, dbErr)

		gotLevel, err := repo.GetStockLevel(ctx, schemaName, tenantID, product.ID, warehouse.ID)
		assert.Nil(t, gotLevel)
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("scan methods return dry-run scan errors", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t)}

		products, err := repo.ListProducts(ctx, schemaName, tenantID, &ProductFilter{Search: "widget"})
		assert.Nil(t, products)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")

		categories, err := repo.ListCategories(ctx, schemaName, tenantID)
		assert.Nil(t, categories)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")

		warehouses, err := repo.ListWarehouses(ctx, schemaName, tenantID, true)
		assert.Nil(t, warehouses)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")

		levels, err := repo.GetStockLevelsByProduct(ctx, schemaName, tenantID, product.ID)
		assert.Nil(t, levels)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")

		reservations, err := repo.ListLotReservations(ctx, schemaName, tenantID, product.ID, warehouse.ID)
		assert.Nil(t, reservations)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")

		movements, err := repo.ListMovements(ctx, schemaName, tenantID, product.ID)
		assert.Nil(t, movements)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")
	})

	t.Run("create errors", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunCreateError(dbErr))}

		assert.ErrorIs(t, repo.CreateProduct(ctx, schemaName, product), dbErr)
		assert.ErrorIs(t, repo.CreateCategory(ctx, schemaName, category), dbErr)
		assert.ErrorIs(t, repo.CreateWarehouse(ctx, schemaName, warehouse), dbErr)
		assert.ErrorIs(t, repo.UpsertStockLevel(ctx, schemaName, level), dbErr)
		assert.ErrorIs(t, repo.UpsertLotReservation(ctx, schemaName, reservation), dbErr)
		assert.ErrorIs(t, repo.CreateMovement(ctx, schemaName, movement), dbErr)
	})

	t.Run("update errors", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunUpdateError(dbErr))}

		assert.ErrorIs(t, repo.UpdateProduct(ctx, schemaName, product), dbErr)
		assert.ErrorIs(t, repo.UpdateWarehouse(ctx, schemaName, warehouse), dbErr)
		released, err := repo.ReleaseLotReservation(ctx, schemaName, tenantID, product.ID, warehouse.ID, reservation.LotNumber, reservation.SerialNumber, reservation.ExpiryDate, decimal.NewFromInt(1), "release", "user-1")
		assert.Nil(t, released)
		assert.ErrorIs(t, err, dbErr)
		assert.ErrorIs(t, repo.UpdateProductStock(ctx, schemaName, tenantID, product.ID, decimal.NewFromInt(12)), dbErr)
	})

	t.Run("delete errors", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunDeleteError(dbErr))}

		assert.ErrorIs(t, repo.DeleteProduct(ctx, schemaName, tenantID, product.ID), dbErr)
		assert.ErrorIs(t, repo.DeleteCategory(ctx, schemaName, tenantID, category.ID), dbErr)
		assert.ErrorIs(t, repo.DeleteWarehouse(ctx, schemaName, tenantID, warehouse.ID), dbErr)
	})

	t.Run("generate code scan errors", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t)}

		code, err := repo.GenerateCode(ctx, schemaName, tenantID)

		assert.Empty(t, code)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")
	})
}

func TestGORMRepositoryDryRunReleaseLotReservationEdges(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_inventory"
	tenantID := "tenant-1"
	productID := "product-1"
	warehouseID := "warehouse-1"

	t.Run("no update rows returns not found", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunUpdateRows(0))}

		got, err := repo.ReleaseLotReservation(ctx, schemaName, tenantID, productID, warehouseID, " lot-1 ", " serial-1 ", " 2027-01-31 ", decimal.NewFromInt(1), "release", "user-1")

		assert.Nil(t, got)
		require.EqualError(t, err, "tracked lot reservation not found")
	})

	t.Run("post-update scan error is returned", func(t *testing.T) {
		repo := &GORMRepository{db: newInventoryDryRunDB(t, withInventoryDryRunUpdateRows(1))}

		got, err := repo.ReleaseLotReservation(ctx, schemaName, tenantID, productID, warehouseID, "lot-1", "serial-1", "2027-01-31", decimal.NewFromInt(1), "release", "user-1")

		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dry run mode unsupported")
	})
}

func TestProductSelectColumnsIncludesStableAliases(t *testing.T) {
	columns := productSelectColumns()

	assert.Contains(t, columns, "COALESCE(description, '') AS description")
	assert.Contains(t, columns, "COALESCE(sale_price, 0) AS sales_price")
	assert.Contains(t, columns, "sale_account_id")
	assert.Contains(t, columns, "purchase_account_id")
	assert.Contains(t, columns, "inventory_account_id")
	assert.Contains(t, columns, "COALESCE(track_inventory, false) AS track_inventory")
	assert.Contains(t, columns, "COALESCE(is_active, true) AS is_active")
}

func TestInventoryServiceHelperBranches(t *testing.T) {
	quantity, err := parsePositiveStockQuantity("2.50")
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("2.50").Equal(quantity))

	_, err = parsePositiveStockQuantity("not-a-number")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid quantity")

	_, err = parsePositiveStockQuantity("0")
	require.EqualError(t, err, "quantity must be positive")

	_, err = parsePositiveStockQuantity("-1")
	require.EqualError(t, err, "quantity must be positive")

	_, err = normalizeRequiredInventoryUUIDString(" ", "product_id")
	require.EqualError(t, err, "product_id is required")

	normalizedID, err := normalizeRequiredInventoryUUIDString(" 11111111-1111-4111-8111-111111111111 ", "product_id")
	require.NoError(t, err)
	assert.Equal(t, "11111111-1111-4111-8111-111111111111", normalizedID)

	product := Product{ID: "product-1", PurchasePrice: decimal.RequireFromString("9.99")}
	assert.True(t, product.PurchasePrice.Equal(inventoryPositionUnitCost(product, nil)))
	assert.True(t, product.PurchasePrice.Equal(inventoryPositionUnitCost(product, &inventoryLotAccumulator{})))

	position := &inventoryLotAccumulator{
		costQuantity: decimal.NewFromInt(4),
		costTotal:    decimal.NewFromInt(40),
	}
	assert.True(t, decimal.NewFromInt(10).Equal(inventoryPositionUnitCost(product, position)))

	allocation := newInventoryIssueAllocation(product, inventoryLotKey{productID: product.ID}, decimal.NewFromInt(3), decimal.NewFromInt(-5))
	assert.True(t, allocation.unitCost.IsZero())
	assert.True(t, allocation.totalCost.IsZero())

	allocation = newInventoryIssueAllocation(product, inventoryLotKey{productID: product.ID}, decimal.NewFromInt(3), decimal.NewFromInt(7))
	assert.True(t, decimal.NewFromInt(21).Equal(allocation.totalCost))
}

func inventoryDryRunProduct(tenantID string, now time.Time) *Product {
	return &Product{
		ID:                 "product-1",
		TenantID:           tenantID,
		Code:               "PRD-00042",
		Name:               "Widget",
		Description:        "Tracked widget",
		ProductType:        ProductTypeGoods,
		CategoryID:         "category-1",
		Unit:               "pcs",
		PurchasePrice:      decimal.RequireFromString("10.50"),
		SalesPrice:         decimal.RequireFromString("15.75"),
		VATRate:            decimal.RequireFromString("22"),
		MinStockLevel:      decimal.NewFromInt(2),
		CurrentStock:       decimal.NewFromInt(4),
		ReorderPoint:       decimal.NewFromInt(5),
		SaleAccountID:      "sale-account-1",
		PurchaseAccountID:  "purchase-account-1",
		InventoryAccountID: "inventory-account-1",
		TrackInventory:     true,
		IsActive:           true,
		Barcode:            "123456789",
		SupplierID:         "supplier-1",
		LeadTimeDays:       7,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func inventoryDryRunProductRow(product *Product) *productRow {
	return &productRow{
		ID:                 product.ID,
		TenantID:           product.TenantID,
		Code:               product.Code,
		Name:               product.Name,
		Description:        product.Description,
		ProductType:        product.ProductType,
		CategoryID:         &product.CategoryID,
		Unit:               product.Unit,
		PurchasePrice:      product.PurchasePrice,
		SalesPrice:         product.SalesPrice,
		VATRate:            product.VATRate,
		MinStockLevel:      product.MinStockLevel,
		CurrentStock:       product.CurrentStock,
		ReorderPoint:       product.ReorderPoint,
		SaleAccountID:      &product.SaleAccountID,
		PurchaseAccountID:  &product.PurchaseAccountID,
		InventoryAccountID: &product.InventoryAccountID,
		TrackInventory:     product.TrackInventory,
		IsActive:           product.IsActive,
		Barcode:            product.Barcode,
		SupplierID:         &product.SupplierID,
		LeadTimeDays:       product.LeadTimeDays,
		CreatedAt:          product.CreatedAt,
		UpdatedAt:          product.UpdatedAt,
	}
}

func inventoryDryRunCategory(tenantID string, now time.Time) *ProductCategory {
	return &ProductCategory{
		ID:          "category-1",
		TenantID:    tenantID,
		Name:        "Parts",
		Description: "Spare parts",
		ParentID:    "parent-1",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func inventoryDryRunProductCategoryRow(category *ProductCategory) *productCategoryRow {
	return &productCategoryRow{
		ID:          category.ID,
		TenantID:    category.TenantID,
		Name:        category.Name,
		Description: category.Description,
		ParentID:    &category.ParentID,
		CreatedAt:   category.CreatedAt,
		UpdatedAt:   category.UpdatedAt,
	}
}

func inventoryDryRunWarehouse(tenantID string, now time.Time) *Warehouse {
	return &Warehouse{
		ID:        "warehouse-1",
		TenantID:  tenantID,
		Code:      "MAIN",
		Name:      "Main warehouse",
		Address:   "1 Storage Way",
		IsDefault: true,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func inventoryDryRunStockLevel(tenantID, productID, warehouseID string, now time.Time) *StockLevel {
	return &StockLevel{
		ID:           "stock-level-1",
		TenantID:     tenantID,
		ProductID:    productID,
		WarehouseID:  warehouseID,
		Quantity:     decimal.NewFromInt(10),
		ReservedQty:  decimal.NewFromInt(3),
		AvailableQty: decimal.NewFromInt(7),
		LastUpdated:  now,
	}
}

func inventoryDryRunLotReservation(tenantID, productID, warehouseID string, now time.Time) *InventoryLotReservation {
	return &InventoryLotReservation{
		ID:           "lot-reservation-1",
		TenantID:     tenantID,
		ProductID:    productID,
		WarehouseID:  warehouseID,
		LotNumber:    "lot-1",
		SerialNumber: "serial-1",
		ExpiryDate:   "2027-01-31",
		Quantity:     decimal.NewFromInt(2),
		Reason:       "order reserve",
		CreatedAt:    now,
		UpdatedAt:    now,
		CreatedBy:    "user-1",
	}
}

func inventoryDryRunMovement(tenantID, productID, warehouseID string, now time.Time) *InventoryMovement {
	return &InventoryMovement{
		ID:            "movement-1",
		TenantID:      tenantID,
		ProductID:     productID,
		WarehouseID:   warehouseID,
		MovementType:  MovementTypeTransfer,
		Quantity:      decimal.NewFromInt(4),
		UnitCost:      decimal.RequireFromString("10.50"),
		TotalCost:     decimal.RequireFromString("42.00"),
		LotNumber:     "lot-1",
		SerialNumber:  "serial-1",
		ExpiryDate:    "2027-01-31",
		Reference:     "MOVE-1",
		SourceType:    "ORDER",
		SourceID:      "order-1",
		ToWarehouseID: "warehouse-2",
		Notes:         "dry-run transfer",
		MovementDate:  now,
		CreatedAt:     now,
		CreatedBy:     "user-1",
	}
}

func inventoryDryRunMovementRow(movement *InventoryMovement) *inventoryMovementRow {
	return &inventoryMovementRow{
		ID:            movement.ID,
		TenantID:      movement.TenantID,
		ProductID:     movement.ProductID,
		WarehouseID:   movement.WarehouseID,
		MovementType:  movement.MovementType,
		Quantity:      movement.Quantity,
		UnitCost:      movement.UnitCost,
		TotalCost:     movement.TotalCost,
		LotNumber:     movement.LotNumber,
		SerialNumber:  movement.SerialNumber,
		ExpiryDate:    movement.ExpiryDate,
		Reference:     movement.Reference,
		SourceType:    movement.SourceType,
		SourceID:      movement.SourceID,
		ToWarehouseID: &movement.ToWarehouseID,
		Notes:         movement.Notes,
		MovementDate:  movement.MovementDate,
		CreatedAt:     movement.CreatedAt,
		CreatedBy:     movement.CreatedBy,
	}
}
