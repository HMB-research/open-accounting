package accounting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave6JournalEntryLookupErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"

	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(gorm.ErrRecordNotFound)))
	entry, err := repo.GetJournalEntryByID(ctx, "tenant_accounting", tenantID, "entry-1")
	assert.Nil(t, entry)
	require.ErrorContains(t, err, "journal entry not found")

	expectedErr := errors.New("entry query failed")
	repo = NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(expectedErr)))
	entry, err = repo.GetJournalEntryByID(ctx, "tenant_accounting", tenantID, "entry-1")
	assert.Nil(t, entry)
	require.ErrorContains(t, err, "get journal entry")
	assert.ErrorIs(t, err, expectedErr)

	entry, err = repo.GetJournalEntryBySource(ctx, "tenant_accounting", tenantID, "IMPORT", "source-1")
	assert.Nil(t, entry)
	require.ErrorContains(t, err, "get journal entry by source")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave6JournalEntryLineQueryErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	entryDate := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)
	entryModel := models.JournalEntry{
		ID:          "entry-1",
		TenantID:    tenantID,
		EntryNumber: "JE-00001",
		EntryDate:   entryDate,
		Description: "Accrual",
		Status:      models.JournalStatusDraft,
		CreatedAt:   entryDate,
		CreatedBy:   "user-1",
	}
	lineModel := models.JournalEntryLine{
		ID:             "line-1",
		TenantID:       tenantID,
		JournalEntryID: entryModel.ID,
		AccountID:      "account-1",
		DebitAmount:    models.NewDecimal(decimal.NewFromInt(10)),
		Currency:       "EUR",
		ExchangeRate:   models.NewDecimal(decimal.NewFromInt(1)),
		BaseDebit:      models.NewDecimal(decimal.NewFromInt(10)),
	}

	expectedErr := errors.New("line query failed")
	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{
			journalEntries:    []models.JournalEntry{entryModel},
			journalEntryLines: []models.JournalEntryLine{lineModel},
		}),
		withAccountingDryRunQueryErrorOnCallWave6(2, expectedErr),
	))
	entry, err := repo.GetJournalEntryByID(ctx, "tenant_accounting", tenantID, entryModel.ID)
	assert.Nil(t, entry)
	require.ErrorContains(t, err, "get journal entry lines")
	assert.ErrorIs(t, err, expectedErr)

	repo = NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{
			journalEntries:    []models.JournalEntry{entryModel},
			journalEntryLines: []models.JournalEntryLine{lineModel},
		}),
		withAccountingDryRunQueryErrorOnCallWave6(2, expectedErr),
	))
	entries, err := repo.ListJournalEntries(ctx, "tenant_accounting", tenantID, 5)
	assert.Nil(t, entries)
	require.ErrorContains(t, err, "list journal entry lines")
	assert.ErrorIs(t, err, expectedErr)
}

func TestGORMRepositoryWave6ListJournalEntriesEmpty(t *testing.T) {
	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
		journalEntries: []models.JournalEntry{},
	})))

	entries, err := repo.ListJournalEntries(context.Background(), "tenant_accounting", "tenant-1", 0)

	require.NoError(t, err)
	assert.Empty(t, entries)
}

func withAccountingDryRunQueryErrorOnCallWave6(call int, expectedErr error) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var queryCall int
		err := db.Callback().Query().Before("gorm:query").Register(accountingDryRunCallbackName("query_error_wave6"), func(tx *gorm.DB) {
			queryCall++
			if queryCall == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}
