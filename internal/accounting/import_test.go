package accounting

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ImportAccountsCSV(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"

	t.Run("imports accounts with parent codes and aliases", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepo(nil, repo)

		req := &ImportAccountsRequest{
			FileName: "accounts.csv",
			CSVContent: "account_code;account_name;type;description;parent_account\n" +
				"1100;Cash in Office;asset;Petty cash account;\n" +
				"1110;Cash Drawer;ASSET;Drawer cash;1100\n" +
				"4000;Sales Revenue;tulu;Main revenue account;\n",
		}

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, req)
		require.NoError(t, err)
		assert.Equal(t, "accounts.csv", result.FileName)
		assert.Equal(t, 3, result.RowsProcessed)
		assert.Equal(t, 3, result.AccountsCreated)
		assert.Equal(t, 0, result.RowsSkipped)
		assert.Empty(t, result.Errors)

		var cashDrawer *Account
		for _, account := range repo.accounts {
			if account.Code == "1110" {
				cashDrawer = account
			}
		}
		require.NotNil(t, cashDrawer)
		require.NotNil(t, cashDrawer.ParentID)
		parent, ok := repo.accounts[*cashDrawer.ParentID]
		require.True(t, ok)
		assert.Equal(t, "1100", parent.Code)
	})

	t.Run("skips duplicates and unresolved parents", func(t *testing.T) {
		repo := NewMockRepository()
		repo.accounts["existing"] = &Account{
			ID:          "existing",
			TenantID:    tenantID,
			Code:        "1000",
			Name:        "Existing Cash",
			AccountType: AccountTypeAsset,
			IsActive:    true,
		}
		svc := NewServiceWithRepo(nil, repo)

		req := &ImportAccountsRequest{
			CSVContent: "code,name,account_type,parent_code\n" +
				"1000,Duplicate Cash,ASSET,\n" +
				"2000,Accounts Payable,LIABILITY,2999\n" +
				",Missing Code,EXPENSE,\n" +
				"3000,Owner Equity,EQUITY,\n",
		}

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, req)
		require.NoError(t, err)
		assert.Equal(t, 4, result.RowsProcessed)
		assert.Equal(t, 1, result.AccountsCreated)
		assert.Equal(t, 3, result.RowsSkipped)
		require.Len(t, result.Errors, 3)
		assert.Contains(t, result.Errors[0].Message, "duplicate code")
		assert.Contains(t, result.Errors[1].Message, "code is required")
		assert.Contains(t, result.Errors[2].Message, "parent_code")
	})

	t.Run("rejects csv without required columns", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepo(nil, repo)

		_, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, &ImportAccountsRequest{
			CSVContent: "name,description\nCash,Missing columns\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required columns")
	})
}
