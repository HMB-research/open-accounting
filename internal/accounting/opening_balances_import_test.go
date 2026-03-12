package accounting

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ImportOpeningBalancesCSV(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"

	t.Run("imports balanced opening balances into a posted journal entry", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{
			ID:          "acc-1000",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}
		repo.accounts["acc-3000"] = &Account{
			ID:          "acc-3000",
			TenantID:    tenantID,
			Code:        "3000",
			Name:        "Owner Equity",
			AccountType: AccountTypeEquity,
			IsActive:    true,
		}

		svc := NewServiceWithRepo(nil, repo)

		result, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			FileName:    "opening-balances.csv",
			EntryDate:   "2026-01-01",
			Description: "Opening balances",
			Reference:   "OB-2026",
			UserID:      "user-1",
			CSVContent: "account_code,debit,credit,description\n" +
				"1000,1500.00,0,Cash opening balance\n" +
				"3000,0,1500.00,Equity opening balance\n",
		})
		require.NoError(t, err)
		assert.Equal(t, "opening-balances.csv", result.FileName)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 2, result.LinesImported)
		assert.True(t, result.TotalDebit.Equal(decimal.NewFromInt(1500)))
		assert.True(t, result.TotalCredit.Equal(decimal.NewFromInt(1500)))
		require.NotNil(t, result.JournalEntry)
		assert.Equal(t, StatusPosted, result.JournalEntry.Status)
		assert.Equal(t, "OPENING_BALANCE", result.JournalEntry.SourceType)
		assert.Len(t, result.JournalEntry.Lines, 2)
	})

	t.Run("rejects unbalanced opening balances", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{
			ID:          "acc-1000",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}
		repo.accounts["acc-3000"] = &Account{
			ID:          "acc-3000",
			TenantID:    tenantID,
			Code:        "3000",
			Name:        "Owner Equity",
			AccountType: AccountTypeEquity,
			IsActive:    true,
		}

		svc := NewServiceWithRepo(nil, repo)

		_, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n1000,100.00,0\n3000,0,90.00\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "opening balances do not balance")
	})

	t.Run("rejects unknown account codes", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["acc-1000"] = &Account{
			ID:          "acc-1000",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}

		svc := NewServiceWithRepo(nil, repo)

		_, err := svc.ImportOpeningBalancesCSV(ctx, schemaName, tenantID, &ImportOpeningBalancesRequest{
			EntryDate:  "2026-01-01",
			UserID:     "user-1",
			CSVContent: "account_code,debit,credit\n9999,100.00,0\n3000,0,100.00\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account_code")
		assert.Contains(t, err.Error(), "was not found")
	})
}
