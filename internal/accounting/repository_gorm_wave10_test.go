package accounting

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave10CreateJournalEntryWriteErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_accounting"

	newEntry := func() *JournalEntry {
		return &JournalEntry{
			TenantID:    "tenant-1",
			EntryDate:   time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC),
			Description: "Wave 10 accrual",
			Status:      StatusDraft,
			CreatedBy:   "user-1",
			Lines: []JournalEntryLine{{
				AccountID:    "expense-1",
				Description:  "Debit",
				DebitAmount:  decimal.NewFromInt(50),
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.NewFromInt(50),
				BaseCredit:   decimal.Zero,
			}},
		}
	}
	sequenceRows := func() accountingDryRunDBOption {
		return withAccountingDryRunScanRowsWave4(accountingDryRunRowSetWave4{
			columns: []string{"sequence"},
			values:  [][]driver.Value{{int64(42)}},
		})
	}

	t.Run("entry insert error", func(t *testing.T) {
		expectedErr := errors.New("entry insert failed")
		repo := NewGORMRepository(newAccountingDryRunDB(t,
			sequenceRows(),
			withAccountingWave8CreateErrorOnCall(1, expectedErr),
		))

		err := repo.CreateJournalEntry(ctx, schemaName, newEntry())

		require.ErrorContains(t, err, "insert journal entry")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("line insert error", func(t *testing.T) {
		expectedErr := errors.New("line insert failed")
		repo := NewGORMRepository(newAccountingDryRunDB(t,
			sequenceRows(),
			withAccountingWave8CreateErrorOnCall(2, expectedErr),
		))

		err := repo.CreateJournalEntry(ctx, schemaName, newEntry())

		require.ErrorContains(t, err, "insert journal entry line")
		assert.ErrorIs(t, err, expectedErr)
	})
}
