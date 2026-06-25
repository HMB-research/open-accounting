package orders

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestOrdersWave9ConstructorPanicsForUnreachablePool(t *testing.T) {
	pool := ordersWave9UnreachablePool(t)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewService(pool)
	})
}

func TestOrdersWave9CreateDefaultsOrderDate(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	order, err := service.Create(context.Background(), "tenant-1", "tenant_demo", &CreateOrderRequest{
		ContactID: "contact-1",
		UserID:    "user-1",
		Lines: []CreateOrderLineRequest{{
			Description: "Consulting",
			Quantity:    decimal.NewFromInt(2),
			UnitPrice:   decimal.NewFromInt(100),
			VATRate:     decimal.NewFromInt(22),
		}},
	})

	require.NoError(t, err)
	require.False(t, order.OrderDate.IsZero())
	require.Equal(t, "EUR", order.Currency)
	require.True(t, decimal.NewFromInt(1).Equal(order.ExchangeRate))
}

func TestOrdersWave9UpdateErrorBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("get error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetErr = errors.New("lookup failed")

		_, err := NewServiceWithRepository(repo).Update(ctx, "tenant-1", "tenant_demo", "order-1", &UpdateOrderRequest{})

		require.ErrorContains(t, err, "lookup failed")
	})

	t.Run("validation error after replacing lines", func(t *testing.T) {
		repo := NewMockRepository()
		repo.Orders["order-1"] = &Order{
			ID:           "order-1",
			TenantID:     "tenant-1",
			ContactID:    "contact-1",
			OrderNumber:  "ORD-1",
			OrderDate:    time.Now(),
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
			Status:       OrderStatusPending,
		}

		_, err := NewServiceWithRepository(repo).Update(ctx, "tenant-1", "tenant_demo", "order-1", &UpdateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			Lines: []CreateOrderLineRequest{{
				Description: "",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromInt(10),
			}},
		})

		require.ErrorContains(t, err, "validation failed")
	})

	t.Run("repository update error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.UpdateErr = errors.New("update failed")
		repo.Orders["order-1"] = &Order{
			ID:           "order-1",
			TenantID:     "tenant-1",
			ContactID:    "contact-1",
			OrderNumber:  "ORD-1",
			OrderDate:    time.Now(),
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
			Status:       OrderStatusConfirmed,
		}

		_, err := NewServiceWithRepository(repo).Update(ctx, "tenant-1", "tenant_demo", "order-1", &UpdateOrderRequest{
			ContactID: "contact-1",
			OrderDate: time.Now(),
			Lines: []CreateOrderLineRequest{{
				Description: "Consulting",
				Quantity:    decimal.NewFromInt(1),
				UnitPrice:   decimal.NewFromInt(10),
			}},
		})

		require.ErrorContains(t, err, "update failed")
	})
}

func TestOrdersWave9ImportParserEdges(t *testing.T) {
	t.Run("header parse error", func(t *testing.T) {
		_, err := parseOrderImportRows(`"unterminated`)
		require.ErrorContains(t, err, "parse csv header")
	})

	t.Run("uppercase status candidate", func(t *testing.T) {
		status, err := parseOrderImportStatus("SHIPPED")
		require.NoError(t, err)
		require.Equal(t, OrderStatusShipped, status)
	})
}

func ordersWave9UnreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
