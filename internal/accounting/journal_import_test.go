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
		svc := NewServiceWithRepo(nil, repo)

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
		svc := NewServiceWithRepo(nil, repo)

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
		svc := NewServiceWithRepo(nil, repo)

		_, err := svc.ImportJournalEntriesCSV(ctx, schemaName, tenantID, &ImportJournalEntriesRequest{
			UserID:     "user-1",
			CSVContent: "entry_date,account_code,debit\n2026-03-31,1000,10.00\n",
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required column")
	})
}
