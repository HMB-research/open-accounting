package reports

import (
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsolidatedWave8SkipsNilEntityReportsAndSortsTypeTies(t *testing.T) {
	asOf := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	report := BuildConsolidatedFinancialReport("tenant-a", asOf, asOf.AddDate(0, -1, 0), asOf, []ConsolidatedTenantReport{
		{TenantID: "tenant-empty"},
		{
			TenantID: "tenant-a",
			TrialBalance: &accounting.TrialBalance{Accounts: []accounting.AccountBalance{
				accountBalance("2000", "Liability", accounting.AccountTypeLiability, 0, 10, -10),
				accountBalance("2000", "Asset", accounting.AccountTypeAsset, 10, 0, 10),
			}},
			BalanceSheet: &accounting.BalanceSheet{
				Assets:           []accounting.AccountBalance{accountBalance("1000", "Cash", accounting.AccountTypeAsset, 10, 0, 10)},
				TotalAssets:      decimal.NewFromInt(10),
				TotalLiabilities: decimal.NewFromInt(3),
				TotalEquity:      decimal.NewFromInt(7),
			},
			IncomeStatement: &accounting.IncomeStatement{
				Revenue:       []accounting.AccountBalance{accountBalance("4000", "Sales", accounting.AccountTypeRevenue, 0, 12, -12)},
				Expenses:      []accounting.AccountBalance{accountBalance("5000", "Costs", accounting.AccountTypeExpense, 5, 0, 5)},
				TotalRevenue:  decimal.NewFromInt(12),
				TotalExpenses: decimal.NewFromInt(5),
			},
		},
	})

	require.NotNil(t, report)
	assert.Equal(t, []string{"tenant-empty", "tenant-a"}, report.TenantIDs)
	require.Len(t, report.TrialBalance.Accounts, 2)
	assert.Equal(t, accounting.AccountTypeAsset, report.TrialBalance.Accounts[0].AccountType)
	assert.Equal(t, accounting.AccountTypeLiability, report.TrialBalance.Accounts[1].AccountType)
	assert.True(t, report.BalanceSheet.IsBalanced)
	assert.True(t, report.IncomeStatement.NetIncome.Equal(decimal.NewFromInt(7)))
}
