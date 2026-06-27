package quotes

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestGORMRepositoryWave4MissingRows(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_quotes"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	quote := quoteDryRunQuote(tenantID, now)

	tests := []struct {
		name string
		run  func(t *testing.T) error
		want error
	}{
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunQueryError(gorm.ErrRecordNotFound)))
				got, err := repo.GetByID(ctx, schemaName, tenantID, quote.ID)
				if got != nil {
					t.Fatalf("GetByID() quote = %#v, want nil", got)
				}
				return err
			},
			want: ErrQuoteNotFound,
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateRows(0)))
				return repo.UpdateStatus(ctx, schemaName, tenantID, quote.ID, QuoteStatusAccepted)
			},
			want: ErrQuoteNotFound,
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunDeleteRows(0)))
				return repo.Delete(ctx, schemaName, tenantID, quote.ID)
			},
			want: ErrQuoteNotFound,
		},
		{
			name: "SetConvertedToOrder",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateRows(0)))
				return repo.SetConvertedToOrder(ctx, schemaName, tenantID, quote.ID, "order-1")
			},
			want: ErrQuoteNotFound,
		},
		{
			name: "SetConvertedToInvoice",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateRows(0)))
				return repo.SetConvertedToInvoice(ctx, schemaName, tenantID, quote.ID, "invoice-1")
			},
			want: ErrQuoteNotFound,
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
	schemaName := "tenant_quotes"
	tenantID := "tenant-1"
	quoteID := "quote-1"
	expectedErr := errors.New("repository write failed")

	tests := []struct {
		name string
		run  func(t *testing.T) error
		want string
	}{
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateError(expectedErr)))
				return repo.UpdateStatus(ctx, schemaName, tenantID, quoteID, QuoteStatusSent)
			},
			want: "update status",
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunDeleteErrorWave4(expectedErr)))
				return repo.Delete(ctx, schemaName, tenantID, quoteID)
			},
			want: "delete quote",
		},
		{
			name: "SetConvertedToOrder",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateError(expectedErr)))
				return repo.SetConvertedToOrder(ctx, schemaName, tenantID, quoteID, "order-1")
			},
			want: "set converted to order",
		},
		{
			name: "SetConvertedToInvoice",
			run: func(t *testing.T) error {
				repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateError(expectedErr)))
				return repo.SetConvertedToInvoice(ctx, schemaName, tenantID, quoteID, "invoice-1")
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

func withQuoteDryRunDeleteErrorWave4(expectedErr error) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(quoteDryRunCallbackName(t, "wave4_delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		if err != nil {
			t.Fatalf("register delete error callback: %v", err)
		}
	}
}
