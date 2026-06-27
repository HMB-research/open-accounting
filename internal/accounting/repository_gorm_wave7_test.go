package accounting

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryWave7ListJournalEntriesSkipsUnmatchedLines(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	entryDate := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)
	entry := models.JournalEntry{
		ID:          "entry-1",
		TenantID:    tenantID,
		EntryNumber: "JE-00001",
		EntryDate:   entryDate,
		Description: "Posted entry",
		Status:      models.JournalStatusPosted,
		CreatedAt:   entryDate,
		CreatedBy:   "user-1",
	}
	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
		journalEntries: []models.JournalEntry{entry},
		journalEntryLines: []models.JournalEntryLine{
			{
				ID:             "line-1",
				TenantID:       tenantID,
				JournalEntryID: entry.ID,
				AccountID:      "account-1",
				DebitAmount:    models.NewDecimal(decimal.NewFromInt(50)),
				Currency:       "EUR",
				ExchangeRate:   models.NewDecimal(decimal.NewFromInt(1)),
				BaseDebit:      models.NewDecimal(decimal.NewFromInt(50)),
			},
			{
				ID:             "line-stray",
				TenantID:       tenantID,
				JournalEntryID: "entry-not-in-page",
				AccountID:      "account-2",
				CreditAmount:   models.NewDecimal(decimal.NewFromInt(50)),
				Currency:       "EUR",
				ExchangeRate:   models.NewDecimal(decimal.NewFromInt(1)),
				BaseCredit:     models.NewDecimal(decimal.NewFromInt(50)),
			},
		},
	})))

	entries, err := repo.ListJournalEntries(ctx, "tenant_accounting", tenantID, 10)

	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Len(t, entries[0].Lines, 1)
	assert.Equal(t, "line-1", entries[0].Lines[0].ID)
}

func TestGORMRepositoryWave7GetAccountBalanceDebitNormalBranch(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	asOfDate := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)
	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
		accounts: []models.Account{
			{
				ID:          "account-1",
				TenantID:    tenantID,
				Code:        "1000",
				Name:        "Cash",
				AccountType: models.AccountTypeAsset,
				IsActive:    true,
				CreatedAt:   asOfDate,
			},
		},
	}), withAccountingDryRunScanRowsWave4(accountingDryRunRowSetWave4{
		columns: []string{"debit_sum", "credit_sum"},
		values:  [][]driver.Value{{"125.00", "25.00"}},
	})))

	balance, err := repo.GetAccountBalance(ctx, "tenant_accounting", tenantID, "account-1", asOfDate)

	require.NoError(t, err)
	assert.True(t, balance.Equal(decimal.NewFromInt(100)))
}
