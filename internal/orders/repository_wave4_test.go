package orders

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave4MissingRows(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_orders"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	order := orderDryRunOrder(tenantID, now)
	reservation := orderDryRunStockReservation(tenantID, order.ID, now)

	tests := []struct {
		name string
		run  func(t *testing.T) error
		want error
	}{
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunQueryError(gorm.ErrRecordNotFound)))
				got, err := repo.GetByID(ctx, schemaName, tenantID, order.ID)
				if got != nil {
					t.Fatalf("GetByID() order = %#v, want nil", got)
				}
				return err
			},
			want: ErrOrderNotFound,
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateRows(0)))
				return repo.UpdateStatus(ctx, schemaName, tenantID, order.ID, OrderStatusShipped)
			},
			want: ErrOrderNotFound,
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunDeleteRows(0)))
				return repo.Delete(ctx, schemaName, tenantID, order.ID)
			},
			want: ErrOrderNotFound,
		},
		{
			name: "SetConvertedToInvoice",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateRows(0)))
				return repo.SetConvertedToInvoice(ctx, schemaName, tenantID, order.ID, "invoice-1")
			},
			want: ErrOrderNotFound,
		},
		{
			name: "GetStockReservation",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunQueryError(gorm.ErrRecordNotFound)))
				got, err := repo.GetStockReservation(ctx, schemaName, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID)
				if got != nil {
					t.Fatalf("GetStockReservation() reservation = %#v, want nil", got)
				}
				return err
			},
			want: ErrOrderStockReservationNotFound,
		},
		{
			name: "ReleaseStockReservation",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateRows(0)))
				got, err := repo.ReleaseStockReservation(ctx, schemaName, tenantID, order.ID, reservation.ProductID, reservation.WarehouseID, decimal.NewFromInt(1), "", "")
				if got != nil {
					t.Fatalf("ReleaseStockReservation() reservation = %#v, want nil", got)
				}
				return err
			},
			want: ErrOrderStockReservationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if !errors.Is(err, tt.want) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, tt.want)
			}
		})
	}
}

func TestGORMRepositoryWave4WrapsUpdateAndDeleteErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_orders"
	tenantID := "tenant-1"
	orderID := "order-1"
	expectedErr := errors.New("repository write failed")

	tests := []struct {
		name string
		run  func(t *testing.T) error
		want string
	}{
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateError(expectedErr)))
				return repo.UpdateStatus(ctx, schemaName, tenantID, orderID, OrderStatusConfirmed)
			},
			want: "update status",
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunDeleteErrorWave4(expectedErr)))
				return repo.Delete(ctx, schemaName, tenantID, orderID)
			},
			want: "delete order",
		},
		{
			name: "SetConvertedToInvoice",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newOrderDryRunDB(t, withOrderDryRunUpdateError(expectedErr)))
				return repo.SetConvertedToInvoice(ctx, schemaName, tenantID, orderID, "invoice-1")
			},
			want: "set converted to invoice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if !errors.Is(err, expectedErr) {
				t.Fatalf("%s error = %v, want %v", tt.name, err, expectedErr)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("%s error = %q, want containing %q", tt.name, err, tt.want)
			}
		})
	}
}

func withOrderDryRunDeleteErrorWave4(expectedErr error) orderDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(orderDryRunCallbackName(t, "wave4_delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register delete error callback: %v", err)
		}
	}
}
