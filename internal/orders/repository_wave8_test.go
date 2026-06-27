package orders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave8NoLineAndSequenceBranches(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_orders"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	order := orderDryRunOrder(tenantID, now)
	order.Lines = nil
	repo := NewGORMRepository(newOrderDryRunDB(t,
		withOrderDryRunUpdateRows(1),
		withOrderDryRunDeleteRows(1),
	))

	require.NoError(t, repo.Create(ctx, schemaName, order))
	require.NoError(t, repo.Update(ctx, schemaName, order))
}

func TestGORMRepositoryWave8DryRunErrorBranches(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_orders"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	order := orderDryRunOrder(tenantID, now)
	reservation := orderDryRunStockReservation(tenantID, order.ID, now)
	expectedErr := errors.New("wave8 repository failure")

	t.Run("create wraps order insert errors", func(t *testing.T) {
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunCreateErrorOnCall(1, expectedErr)))

		err := repo.Create(ctx, schemaName, order)

		require.ErrorContains(t, err, "insert order")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("update wraps order line delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newOrderDryRunDB(t,
			withOrderDryRunUpdateRows(1),
			withOrderWave8DryRunDeleteError(expectedErr),
		))

		err := repo.Update(ctx, schemaName, order)

		require.ErrorContains(t, err, "delete order lines")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("base gorm errors are wrapped by query and mutation methods", func(t *testing.T) {
		repo := newOrderWave8ErrorRepository(t, expectedErr)

		got, err := repo.GetByID(ctx, schemaName, tenantID, order.ID)
		assert.Nil(t, got)
		require.ErrorContains(t, err, "get order")
		assert.ErrorIs(t, err, expectedErr)

		list, err := repo.List(ctx, schemaName, tenantID, nil)
		assert.Nil(t, list)
		require.ErrorContains(t, err, "list orders")
		assert.ErrorIs(t, err, expectedErr)

		number, err := repo.GenerateNumber(ctx, schemaName, tenantID)
		assert.Empty(t, number)
		require.ErrorContains(t, err, "generate order number")
		assert.ErrorIs(t, err, expectedErr)

		err = repo.UpsertStockReservation(ctx, schemaName, reservation)
		require.ErrorContains(t, err, "upsert order stock reservation")
		assert.ErrorIs(t, err, expectedErr)

		released, err := repo.ReleaseStockReservation(ctx, schemaName, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID, decimal.NewFromInt(1), "release", "user-1")
		assert.Nil(t, released)
		require.ErrorContains(t, err, "release order stock reservation")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func newOrderWave8ErrorRepository(t *testing.T, expectedErr error) *GORMRepository {
	t.Helper()

	db := newOrderDryRunDB(t)
	db.AddError(expectedErr)
	return NewGORMRepository(db)
}

func withOrderWave8DryRunDeleteError(expectedErr error) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(orderDryRunCallbackName(t, "delete_error_wave8"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}
