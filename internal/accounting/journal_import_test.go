package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ImportJournalEntriesCSV(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"

	t.Run("imports balanced grouped journal as posted entry", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{
			ID:          "acc-1000",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}
		repo.accounts["acc-4000"] = &Account{
			ID:          "acc-4000",
			TenantID:    tenantID,
			Code:        "4000",
			Name:        "Revenue",
			AccountType: AccountTypeRevenue,
			IsActive:    true,
		}
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportJournalEntriesCSV(ctx, schemaName, tenantID, &ImportJournalEntriesRequest{
			FileName:    "journals.csv",
			PostEntries: true,
			UserID:      "user-1",
			CSVContent: "entry_reference,entry_date,entry_description,account_code,line_description,debit,credit\n" +
				"LEG-001,2026-03-31,Imported sale,1000,Cash received,100.00,0\n" +
				"LEG-001,2026-03-31,Imported sale,4000,Revenue,0,100.00\n",
		})

		require.NoError(t, err)
		assert.Equal(t, "journals.csv", result.FileName)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.EntriesCreated)
		assert.Equal(t, 2, result.LinesImported)
		assert.Equal(t, 0, result.RowsSkipped)
		assert.True(t, result.TotalDebit.Equal(decimal.NewFromInt(100)))
		assert.True(t, result.TotalCredit.Equal(decimal.NewFromInt(100)))
		require.Len(t, result.JournalEntries, 1)
		assert.Equal(t, StatusPosted, result.JournalEntries[0].Status)
		assert.Equal(t, "LEG-001", result.JournalEntries[0].Reference)
		assert.Equal(t, defaultJournalImportSourceType, result.JournalEntries[0].SourceType)
	})

	t.Run("skips invalid groups and imports valid groups", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{ID: "acc-1000", TenantID: tenantID, Code: "1000", Name: "Cash", AccountType: AccountTypeAsset, IsActive: true}
		repo.accounts["acc-4000"] = &Account{ID: "acc-4000", TenantID: tenantID, Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, IsActive: true}
		svc := NewServiceWithRepository(repo)

		lockDate := time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)
		result, err := svc.ImportJournalEntriesCSV(ctx, schemaName, tenantID, &ImportJournalEntriesRequest{
			UserID:         "user-1",
			PeriodLockDate: &lockDate,
			CSVContent: "entry_reference,entry_date,account_code,debit,credit\n" +
				"LOCKED,2026-02-15,1000,100.00,0\n" +
				"LOCKED,2026-02-15,4000,0,100.00\n" +
				"UNBAL,2026-03-31,1000,100.00,0\n" +
				"UNBAL,2026-03-31,4000,0,90.00\n" +
				"OK,2026-03-31,1000,75.00,0\n" +
				"OK,2026-03-31,4000,0,75.00\n",
		})

		require.NoError(t, err)
		assert.Equal(t, 6, result.RowsProcessed)
		assert.Equal(t, 1, result.EntriesCreated)
		assert.Equal(t, 2, result.LinesImported)
		assert.Equal(t, 4, result.RowsSkipped)
		require.Len(t, result.Errors, 2)
		assert.Contains(t, result.Errors[0].Message, "period locked through 2026-02-28")
		assert.Contains(t, result.Errors[1].Message, "does not balance")
		require.Len(t, result.JournalEntries, 1)
		assert.Equal(t, "OK", result.JournalEntries[0].Reference)
	})

	t.Run("rejects missing required headers", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		_, err := svc.ImportJournalEntriesCSV(ctx, schemaName, tenantID, &ImportJournalEntriesRequest{
			UserID:     "user-1",
			CSVContent: "entry_date,account_code,debit\n2026-03-31,1000,10.00\n",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required column")
	})

	t.Run("skips journals with invalid source id", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{ID: "acc-1000", TenantID: tenantID, Code: "1000", Name: "Cash", AccountType: AccountTypeAsset, IsActive: true}
		repo.accounts["acc-4000"] = &Account{ID: "acc-4000", TenantID: tenantID, Code: "4000", Name: "Revenue", AccountType: AccountTypeRevenue, IsActive: true}
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportJournalEntriesCSV(ctx, schemaName, tenantID, &ImportJournalEntriesRequest{
			UserID: "user-1",
			CSVContent: "entry_reference,entry_date,account_code,debit,credit,source_id\n" +
				"BAD-SOURCE,2026-03-31,1000,75.00,0,legacy-source\n" +
				"BAD-SOURCE,2026-03-31,4000,0,75.00,\n",
		})

		require.NoError(t, err)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Zero(t, result.EntriesCreated)
		assert.Zero(t, result.LinesImported)
		assert.Equal(t, 2, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "source_id must be a valid UUID")
	})
}

func TestParseJournalImportAmounts(t *testing.T) {
	t.Run("accepts debit-only amount", func(t *testing.T) {
		debit, credit, err := parseJournalImportAmounts(journalImportRow{
			values: map[string]string{"debit": "1,250.50", "credit": ""},
		})

		require.NoError(t, err)
		assert.True(t, decimal.RequireFromString("1250.50").Equal(debit))
		assert.True(t, credit.IsZero())
	})

	t.Run("accepts credit-only amount", func(t *testing.T) {
		debit, credit, err := parseJournalImportAmounts(journalImportRow{
			values: map[string]string{"debit": "", "credit": "99.95"},
		})

		require.NoError(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, decimal.RequireFromString("99.95").Equal(credit))
	})

	t.Run("rejects invalid debit", func(t *testing.T) {
		debit, credit, err := parseJournalImportAmounts(journalImportRow{
			values: map[string]string{"debit": "not-a-number", "credit": "0"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "invalid debit")
	})

	t.Run("rejects invalid credit", func(t *testing.T) {
		debit, credit, err := parseJournalImportAmounts(journalImportRow{
			values: map[string]string{"debit": "0", "credit": "not-a-number"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "invalid credit")
	})

	t.Run("rejects negative amounts", func(t *testing.T) {
		debit, credit, err := parseJournalImportAmounts(journalImportRow{
			values: map[string]string{"debit": "-10.00", "credit": "0"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "amounts cannot be negative")
	})

	t.Run("requires one-sided amount", func(t *testing.T) {
		debit, credit, err := parseJournalImportAmounts(journalImportRow{
			values: map[string]string{"debit": "0", "credit": "0"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "either debit or credit is required")
	})

	t.Run("rejects two-sided amount", func(t *testing.T) {
		debit, credit, err := parseJournalImportAmounts(journalImportRow{
			values: map[string]string{"debit": "10.00", "credit": "10.00"},
		})

		require.Error(t, err)
		assert.True(t, debit.IsZero())
		assert.True(t, credit.IsZero())
		assert.Contains(t, err.Error(), "row cannot contain both debit and credit amounts")
	})
}

func TestParseJournalImportExchangeRate(t *testing.T) {
	t.Run("defaults blank exchange rate to one", func(t *testing.T) {
		exchangeRate, err := parseJournalImportExchangeRate(" ")

		require.NoError(t, err)
		assert.True(t, decimal.NewFromInt(1).Equal(exchangeRate))
	})

	t.Run("accepts positive exchange rate", func(t *testing.T) {
		exchangeRate, err := parseJournalImportExchangeRate("0.92")

		require.NoError(t, err)
		assert.True(t, decimal.RequireFromString("0.92").Equal(exchangeRate))
	})

	t.Run("rejects invalid exchange rate", func(t *testing.T) {
		exchangeRate, err := parseJournalImportExchangeRate("bad-rate")

		require.Error(t, err)
		assert.True(t, exchangeRate.IsZero())
		assert.Contains(t, err.Error(), "invalid exchange_rate")
	})

	t.Run("rejects negative exchange rate", func(t *testing.T) {
		exchangeRate, err := parseJournalImportExchangeRate("-0.92")

		require.Error(t, err)
		assert.True(t, exchangeRate.IsZero())
		assert.Contains(t, err.Error(), "exchange_rate cannot be negative")
	})
}

func TestParseJournalImportDate(t *testing.T) {
	t.Run("accepts trimmed ISO date", func(t *testing.T) {
		parsed, err := parseJournalImportDate(" 2026-03-31 ")

		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), parsed)
	})

	t.Run("requires entry date", func(t *testing.T) {
		parsed, err := parseJournalImportDate(" \t ")

		require.Error(t, err)
		assert.True(t, parsed.IsZero())
		assert.Contains(t, err.Error(), "entry_date is required")
	})

	t.Run("rejects non ISO date", func(t *testing.T) {
		parsed, err := parseJournalImportDate("31/03/2026")

		require.Error(t, err)
		assert.True(t, parsed.IsZero())
		assert.Contains(t, err.Error(), "entry_date must be in YYYY-MM-DD format")
	})
}

func TestJournalImportHeaderAndOptionalUUID(t *testing.T) {
	t.Run("canonicalizes journal import header aliases", func(t *testing.T) {
		assert.Equal(t, "entry_reference", canonicalJournalImportHeader(" Voucher Number "))
		assert.Equal(t, "entry_date", canonicalJournalImportHeader("posting-date"))
		assert.Equal(t, "entry_description", canonicalJournalImportHeader("entry memo"))
		assert.Equal(t, "account_code", canonicalJournalImportHeader("Account"))
		assert.Equal(t, "line_description", canonicalJournalImportHeader("Memo"))
		assert.Equal(t, "debit", canonicalJournalImportHeader("Debit Amount"))
		assert.Equal(t, "credit", canonicalJournalImportHeader("credit_amount"))
		assert.Equal(t, "custom_field", canonicalJournalImportHeader("Custom Field"))
	})

	t.Run("parses optional UUID fields", func(t *testing.T) {
		id, err := optionalJournalImportUUID("source_id", "550e8400-e29b-41d4-a716-446655440000")

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", *id)
	})

	t.Run("allows blank optional UUID fields", func(t *testing.T) {
		id, err := optionalJournalImportUUID("source_id", " \t ")

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("rejects invalid optional UUID fields", func(t *testing.T) {
		id, err := optionalJournalImportUUID("source_id", "legacy-source")

		require.Error(t, err)
		assert.Nil(t, id)
		assert.Contains(t, err.Error(), "source_id must be a valid UUID")
	})
}
