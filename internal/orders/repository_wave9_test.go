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

func TestGORMRepositoryWave9UpdateLineInsertAndReservationQueryErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_orders"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	order := orderDryRunOrder(tenantID, now)
	reservation := orderDryRunStockReservation(tenantID, order.ID, now)

	t.Run("Update wraps replacement line insert errors", func(t *testing.T) {
		expectedErr := errors.New("replacement line insert failed")
		repo := NewGORMRepository(newOrderDryRunDB(t,
			withOrderDryRunUpdateRows(1),
			withOrderDryRunDeleteRows(1),
			withOrderDryRunCreateErrorOnCall(1, expectedErr),
		))

		err := repo.Update(ctx, schemaName, order)

		require.ErrorContains(t, err, "insert order line")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("ListStockReservations wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("reservation list failed")
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunQueryError(expectedErr)))

		reservations, err := repo.ListStockReservations(ctx, schemaName, tenantID, order.ID)

		assert.Nil(t, reservations)
		require.ErrorContains(t, err, "list order stock reservations")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("GetStockReservation wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("reservation lookup failed")
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunQueryError(expectedErr)))

		found, err := repo.GetStockReservation(ctx, schemaName, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID)

		assert.Nil(t, found)
		require.ErrorContains(t, err, "get order stock reservation")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("GetByID propagates order line query errors", func(t *testing.T) {
		expectedErr := errors.New("order line lookup failed")
		repo := NewGORMRepository(newOrderDryRunDB(t,
			withOrderDryRunFixtures(orderDryRunFixtures{order: orderToModel(order)}),
			withOrderWave9DryRunQueryErrorOnCall(2, expectedErr),
		))

		found, err := repo.GetByID(ctx, schemaName, tenantID, order.ID)

		assert.Nil(t, found)
		require.ErrorContains(t, err, "get order lines")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("ReleaseStockReservation returns not found when update touches no rows", func(t *testing.T) {
		repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateRows(0)))

		found, err := repo.ReleaseStockReservation(ctx, schemaName, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID, decimal.NewFromInt(1), "", "")

		assert.Nil(t, found)
		require.ErrorIs(t, err, ErrOrderStockReservationNotFound)
	})
}

func withOrderWave9DryRunQueryErrorOnCall(call int, expectedErr error) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var seen int
		err := db.Callback().Query().Before("gorm:query").Register(orderDryRunCallbackName(t, "query_error_wave9"), func(tx *gorm.DB) {
			seen++
			if seen == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}
