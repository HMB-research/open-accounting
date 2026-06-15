package reports

import (
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConsolidatedFinancialReport(t *testing.T) {
	asOf := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := asOf

	report := BuildConsolidatedFinancialReport("tenant-a", asOf, start, end, []ConsolidatedTenantReport{
		{
			TenantID:   "tenant-a",
			TenantName: "Alpha",
			TenantSlug: "alpha",
			TrialBalance: &accounting.TrialBalance{
				Accounts: []accounting.AccountBalance{
					accountBalance("1000", "Cash", accounting.AccountTypeAsset, 100, 0, 100),
					accountBalance("4000", "Revenue", accounting.AccountTypeRevenue, 0, 80, -80),
				},
				TotalDebits:  decimal.NewFromInt(100),
				TotalCredits: decimal.NewFromInt(80),
			},
			BalanceSheet: &accounting.BalanceSheet{
				Assets:           []accounting.AccountBalance{accountBalance("1000", "Cash", accounting.AccountTypeAsset, 100, 0, 100)},
				TotalAssets:      decimal.NewFromInt(100),
				TotalLiabilities: decimal.Zero,
				TotalEquity:      decimal.NewFromInt(100),
				RetainedEarnings: decimal.NewFromInt(80),
			},
			IncomeStatement: &accounting.IncomeStatement{
				Revenue:      []accounting.AccountBalance{accountBalance("4000", "Revenue", accounting.AccountTypeRevenue, 0, 80, -80)},
				TotalRevenue: decimal.NewFromInt(80),
				NetIncome:    decimal.NewFromInt(80),
			},
		},
		{
			TenantID:   "tenant-b",
			TenantName: "Beta",
			TenantSlug: "beta",
			TrialBalance: &accounting.TrialBalance{
				Accounts: []accounting.AccountBalance{
					accountBalance("1000", "Cash", accounting.AccountTypeAsset, 50, 0, 50),
					accountBalance("5000", "Expenses", accounting.AccountTypeExpense, 20, 0, 20),
				},
				TotalDebits: decimal.NewFromInt(70),
			},
			BalanceSheet: &accounting.BalanceSheet{
				Assets:           []accounting.AccountBalance{accountBalance("1000", "Cash", accounting.AccountTypeAsset, 50, 0, 50)},
				TotalAssets:      decimal.NewFromInt(50),
				TotalLiabilities: decimal.Zero,
				TotalEquity:      decimal.NewFromInt(50),
				RetainedEarnings: decimal.NewFromInt(-20),
			},
			IncomeStatement: &accounting.IncomeStatement{
				Expenses:      []accounting.AccountBalance{accountBalance("5000", "Expenses", accounting.AccountTypeExpense, 20, 0, 20)},
				TotalExpenses: decimal.NewFromInt(20),
				NetIncome:     decimal.NewFromInt(-20),
			},
		},
	})

	require.NotNil(t, report)
	assert.Equal(t, "tenant-a", report.TenantID)
	assert.Equal(t, []string{"tenant-a", "tenant-b"}, report.TenantIDs)
	assert.Equal(t, 2, report.TenantCount)
	require.NotNil(t, report.TrialBalance)
	require.Len(t, report.TrialBalance.Accounts, 3)
	assert.True(t, report.TrialBalance.Accounts[0].NetBalance.Equal(decimal.NewFromInt(150)))
	require.NotNil(t, report.BalanceSheet)
	assert.True(t, report.BalanceSheet.TotalAssets.Equal(decimal.NewFromInt(150)))
	assert.True(t, report.BalanceSheet.TotalEquity.Equal(decimal.NewFromInt(150)))
	require.NotNil(t, report.IncomeStatement)
	assert.True(t, report.IncomeStatement.TotalRevenue.Equal(decimal.NewFromInt(80)))
	assert.True(t, report.IncomeStatement.TotalExpenses.Equal(decimal.NewFromInt(20)))
	assert.True(t, report.IncomeStatement.NetIncome.Equal(decimal.NewFromInt(60)))
}

func accountBalance(code, name string, accountType accounting.AccountType, debit, credit, net int64) accounting.AccountBalance {
	return accounting.AccountBalance{
		AccountID:     code,
		AccountCode:   code,
		AccountName:   name,
		AccountType:   accountType,
		DebitBalance:  decimal.NewFromInt(debit),
		CreditBalance: decimal.NewFromInt(credit),
		NetBalance:    decimal.NewFromInt(net),
	}
}
