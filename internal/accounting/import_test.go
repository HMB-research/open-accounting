package accounting

import (
	"context"
	"errors"
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
		svc := NewServiceWithRepository(repo)

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

	t.Run("preserves supplied account ids", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		rootID := "11111111-1111-1111-1111-111111111111"
		childID := "22222222-2222-2222-2222-222222222222"
		req := &ImportAccountsRequest{
			CSVContent: "account_id,account_code,account_name,type,parent_account\n" +
				rootID + ",1100,Cash in Office,ASSET,\n" +
				childID + ",1110,Cash Drawer,ASSET,1100\n",
		}

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, req)
		require.NoError(t, err)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 2, result.AccountsCreated)
		assert.Equal(t, 0, result.RowsSkipped)
		assert.Empty(t, result.Errors)

		root := repo.accounts[rootID]
		require.NotNil(t, root)
		assert.Equal(t, "1100", root.Code)
		child := repo.accounts[childID]
		require.NotNil(t, child)
		require.NotNil(t, child.ParentID)
		assert.Equal(t, rootID, *child.ParentID)
	})

	t.Run("rejects invalid and duplicate preserved ids", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		accountID := "33333333-3333-3333-3333-333333333333"
		req := &ImportAccountsRequest{
			CSVContent: "id,code,name,account_type\n" +
				accountID + ",1100,Cash,ASSET\n" +
				accountID + ",1110,Duplicate Cash,ASSET\n" +
				"not-a-uuid,1120,Bad ID,ASSET\n",
		}

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, req)
		require.NoError(t, err)
		assert.Equal(t, 3, result.RowsProcessed)
		assert.Equal(t, 1, result.AccountsCreated)
		assert.Equal(t, 2, result.RowsSkipped)
		require.Len(t, result.Errors, 2)
		assert.Contains(t, result.Errors[0].Message, "duplicate id")
		assert.Contains(t, result.Errors[1].Message, "id must be a valid UUID")
		assert.NotNil(t, repo.accounts[accountID])
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
		svc := NewServiceWithRepository(repo)

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
		svc := NewServiceWithRepository(repo)

		_, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, &ImportAccountsRequest{
			CSVContent: "name,description\nCash,Missing columns\n",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing required columns")
	})

	t.Run("requires csv content", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		for name, req := range map[string]*ImportAccountsRequest{
			"nil request": nil,
			"blank csv":   {CSVContent: " \n\t "},
		} {
			t.Run(name, func(t *testing.T) {
				result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, req)

				require.Error(t, err)
				assert.Nil(t, result)
				assert.Contains(t, err.Error(), "csv_content is required")
			})
		}
	})

	t.Run("returns parser errors", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, &ImportAccountsRequest{
			CSVContent: "\"code,name,account_type\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parse csv header")
	})

	t.Run("rejects header-only csv", func(t *testing.T) {
		repo := NewMockRepository()
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, &ImportAccountsRequest{
			CSVContent: "code,name,account_type\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no accounts found in CSV")
	})

	t.Run("wraps list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.listAccountsErr = errors.New("list unavailable")
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, &ImportAccountsRequest{
			CSVContent: "code,name,account_type\n1100,Cash,ASSET\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "list existing accounts: list unavailable")
	})

	t.Run("records create errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.createAccountErr = errors.New("create unavailable")
		svc := NewServiceWithRepository(repo)

		result, err := svc.ImportAccountsCSV(ctx, schemaName, tenantID, &ImportAccountsRequest{
			CSVContent: "code,name,account_type\n1100,Cash,ASSET\n",
		})

		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 0, result.AccountsCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 2, result.Errors[0].Row)
		assert.Equal(t, "1100", result.Errors[0].Code)
		assert.Contains(t, result.Errors[0].Message, "create unavailable")
	})
}

func TestParseAccountImportRows(t *testing.T) {
	t.Run("parses aliases blank rows blank headers and short records", func(t *testing.T) {
		rows, err := parseAccountImportRows("\ufeffAccount Code,Account Name,Account.Type,Description,Parent/Account,\n" +
			"1100, Cash , Vara , Cash account ,, ignored blank-header value\n" +
			",,,,,\n" +
			"1110,Cash Drawer,asset,Drawer cash,1100\n")

		require.NoError(t, err)
		require.Len(t, rows, 2)
		assert.Equal(t, 2, rows[0].rowNumber)
		assert.Equal(t, "1100", rows[0].values["code"])
		assert.Equal(t, "Cash", rows[0].values["name"])
		assert.Equal(t, "Vara", rows[0].values["account_type"])
		assert.Equal(t, "Cash account", rows[0].values["description"])
		assert.Empty(t, rows[0].values["parent_code"])
		assert.NotContains(t, rows[0].values, "")

		assert.Equal(t, 4, rows[1].rowNumber)
		assert.Equal(t, "1110", rows[1].values["code"])
		assert.Equal(t, "Cash Drawer", rows[1].values["name"])
		assert.Equal(t, "asset", rows[1].values["account_type"])
		assert.Equal(t, "Drawer cash", rows[1].values["description"])
		assert.Equal(t, "1100", rows[1].values["parent_code"])
	})

	t.Run("requires content", func(t *testing.T) {
		rows, err := parseAccountImportRows(" \ufeff \n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "csv_content is required")
	})

	t.Run("returns header parse errors", func(t *testing.T) {
		rows, err := parseAccountImportRows("\"code,name,account_type\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv header")
	})

	t.Run("requires columns", func(t *testing.T) {
		rows, err := parseAccountImportRows("code,name\n1100,Cash\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "missing required columns")
	})

	t.Run("returns row parse errors", func(t *testing.T) {
		rows, err := parseAccountImportRows("code,name,account_type\n\"1100,Cash,ASSET\n")

		require.Error(t, err)
		assert.Nil(t, rows)
		assert.Contains(t, err.Error(), "parse csv row 2")
	})
}

func TestBuildCreateAccountRequestFromImportRow(t *testing.T) {
	t.Run("builds request", func(t *testing.T) {
		req, parentCode, err := buildCreateAccountRequestFromImportRow(accountImportRow{
			rowNumber: 2,
			values: map[string]string{
				"id":           " 11111111-1111-4111-8111-111111111111 ",
				"code":         " 1100 ",
				"name":         " Cash ",
				"account_type": " asset ",
				"description":  " Petty cash ",
				"parent_code":  " 1000 ",
			},
		})

		require.NoError(t, err)
		require.NotNil(t, req)
		assert.Equal(t, "11111111-1111-4111-8111-111111111111", req.ID)
		assert.Equal(t, "1100", req.Code)
		assert.Equal(t, "Cash", req.Name)
		assert.Equal(t, AccountTypeAsset, req.AccountType)
		assert.Equal(t, "Petty cash", req.Description)
		assert.Equal(t, "1000", parentCode)
	})

	tests := []struct {
		name    string
		values  map[string]string
		message string
	}{
		{
			name: "rejects invalid ids",
			values: map[string]string{
				"id":           "not-a-uuid",
				"code":         "1100",
				"name":         "Cash",
				"account_type": "ASSET",
			},
			message: "id must be a valid UUID",
		},
		{
			name: "requires code",
			values: map[string]string{
				"name":         "Cash",
				"account_type": "ASSET",
			},
			message: "code is required",
		},
		{
			name: "requires name",
			values: map[string]string{
				"code":         "1100",
				"account_type": "ASSET",
			},
			message: "name is required",
		},
		{
			name: "requires account type",
			values: map[string]string{
				"code": "1100",
				"name": "Cash",
			},
			message: "account_type is required",
		},
		{
			name: "rejects self parent",
			values: map[string]string{
				"code":         " 1100 ",
				"name":         "Cash",
				"account_type": "ASSET",
				"parent_code":  "1100",
			},
			message: "parent_code cannot match code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, parentCode, err := buildCreateAccountRequestFromImportRow(accountImportRow{
				rowNumber: 2,
				values:    tt.values,
			})

			require.Error(t, err)
			assert.Nil(t, req)
			assert.Empty(t, parentCode)
			assert.Contains(t, err.Error(), tt.message)
		})
	}
}

func TestAccountImportHelpers(t *testing.T) {
	assert.Equal(t, ',', detectAccountImportDelimiter("code,name,account_type\n"))
	assert.Equal(t, ';', detectAccountImportDelimiter("code;name;account_type\n"))
	assert.Equal(t, '\t', detectAccountImportDelimiter("code\tname\taccount_type\n"))

	assert.Equal(t, "code", canonicalAccountImportHeader("Account-Code"))
	assert.Equal(t, "parent_code", canonicalAccountImportHeader("Parent/Account"))
	assert.Equal(t, "custom_field", canonicalAccountImportHeader(" Custom.Field "))
}

func TestParseAccountImportType(t *testing.T) {
	t.Run("accepts canonical account types", func(t *testing.T) {
		accountType, err := parseAccountImportType(" expense ")
		require.NoError(t, err)
		assert.Equal(t, AccountTypeExpense, accountType)
	})

	t.Run("accepts aliases", func(t *testing.T) {
		accountType, err := parseAccountImportType("omakapital")
		require.NoError(t, err)
		assert.Equal(t, AccountTypeEquity, accountType)
	})

	t.Run("requires account type", func(t *testing.T) {
		accountType, err := parseAccountImportType(" \t ")
		require.Error(t, err)
		assert.Empty(t, accountType)
		assert.Contains(t, err.Error(), "account_type is required")
	})

	t.Run("rejects unknown account types", func(t *testing.T) {
		accountType, err := parseAccountImportType("contra")
		require.Error(t, err)
		assert.Empty(t, accountType)
		assert.Contains(t, err.Error(), `invalid account_type "contra"`)
	})
}
