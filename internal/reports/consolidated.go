package reports

import (
	"sort"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/shopspring/decimal"
)

// ConsolidatedFinancialReport combines core statements across multiple companies.
type ConsolidatedFinancialReport struct {
	TenantID        string                      `json:"tenant_id"`
	TenantIDs       []string                    `json:"tenant_ids"`
	TenantCount     int                         `json:"tenant_count"`
	AsOfDate        time.Time                   `json:"as_of_date"`
	StartDate       time.Time                   `json:"start_date"`
	EndDate         time.Time                   `json:"end_date"`
	GeneratedAt     time.Time                   `json:"generated_at"`
	TrialBalance    *accounting.TrialBalance    `json:"trial_balance"`
	BalanceSheet    *accounting.BalanceSheet    `json:"balance_sheet"`
	IncomeStatement *accounting.IncomeStatement `json:"income_statement"`
	Entities        []ConsolidatedTenantReport  `json:"entities"`
}

// ConsolidatedTenantReport contains one company's source reports inside a consolidation.
type ConsolidatedTenantReport struct {
	TenantID        string                      `json:"tenant_id"`
	TenantName      string                      `json:"tenant_name"`
	TenantSlug      string                      `json:"tenant_slug"`
	TrialBalance    *accounting.TrialBalance    `json:"trial_balance"`
	BalanceSheet    *accounting.BalanceSheet    `json:"balance_sheet"`
	IncomeStatement *accounting.IncomeStatement `json:"income_statement"`
}

// BuildConsolidatedFinancialReport aggregates already-generated tenant statements.
func BuildConsolidatedFinancialReport(anchorTenantID string, asOfDate, startDate, endDate time.Time, entities []ConsolidatedTenantReport) *ConsolidatedFinancialReport {
	report := &ConsolidatedFinancialReport{
		TenantID:    anchorTenantID,
		AsOfDate:    asOfDate,
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: time.Now(),
		Entities:    entities,
		TenantCount: len(entities),
	}
	for _, entity := range entities {
		report.TenantIDs = append(report.TenantIDs, entity.TenantID)
	}

	report.TrialBalance = aggregateTrialBalances(asOfDate, entities)
	report.BalanceSheet = aggregateBalanceSheets(asOfDate, entities)
	report.IncomeStatement = aggregateIncomeStatements(startDate, endDate, entities)
	return report
}

func aggregateTrialBalances(asOfDate time.Time, entities []ConsolidatedTenantReport) *accounting.TrialBalance {
	tb := &accounting.TrialBalance{
		TenantID:    "consolidated",
		AsOfDate:    asOfDate,
		GeneratedAt: time.Now(),
		Accounts:    aggregateAccountBalances(collectTrialBalanceAccounts(entities)),
	}
	for _, account := range tb.Accounts {
		tb.TotalDebits = tb.TotalDebits.Add(account.DebitBalance)
		tb.TotalCredits = tb.TotalCredits.Add(account.CreditBalance)
	}
	tb.IsBalanced = tb.TotalDebits.Equal(tb.TotalCredits)
	return tb
}

func aggregateBalanceSheets(asOfDate time.Time, entities []ConsolidatedTenantReport) *accounting.BalanceSheet {
	bs := &accounting.BalanceSheet{
		TenantID:    "consolidated",
		AsOfDate:    asOfDate,
		GeneratedAt: time.Now(),
	}
	for _, entity := range entities {
		if entity.BalanceSheet == nil {
			continue
		}
		bs.Assets = append(bs.Assets, entity.BalanceSheet.Assets...)
		bs.Liabilities = append(bs.Liabilities, entity.BalanceSheet.Liabilities...)
		bs.Equity = append(bs.Equity, entity.BalanceSheet.Equity...)
		bs.TotalAssets = bs.TotalAssets.Add(entity.BalanceSheet.TotalAssets)
		bs.TotalLiabilities = bs.TotalLiabilities.Add(entity.BalanceSheet.TotalLiabilities)
		bs.TotalEquity = bs.TotalEquity.Add(entity.BalanceSheet.TotalEquity)
		bs.RetainedEarnings = bs.RetainedEarnings.Add(entity.BalanceSheet.RetainedEarnings)
	}
	bs.Assets = aggregateAccountBalances(bs.Assets)
	bs.Liabilities = aggregateAccountBalances(bs.Liabilities)
	bs.Equity = aggregateAccountBalances(bs.Equity)
	bs.IsBalanced = bs.TotalAssets.Equal(bs.TotalLiabilities.Add(bs.TotalEquity))
	return bs
}

func aggregateIncomeStatements(startDate, endDate time.Time, entities []ConsolidatedTenantReport) *accounting.IncomeStatement {
	is := &accounting.IncomeStatement{
		TenantID:    "consolidated",
		StartDate:   startDate,
		EndDate:     endDate,
		GeneratedAt: time.Now(),
	}
	for _, entity := range entities {
		if entity.IncomeStatement == nil {
			continue
		}
		is.Revenue = append(is.Revenue, entity.IncomeStatement.Revenue...)
		is.Expenses = append(is.Expenses, entity.IncomeStatement.Expenses...)
		is.TotalRevenue = is.TotalRevenue.Add(entity.IncomeStatement.TotalRevenue)
		is.TotalExpenses = is.TotalExpenses.Add(entity.IncomeStatement.TotalExpenses)
	}
	is.Revenue = aggregateAccountBalances(is.Revenue)
	is.Expenses = aggregateAccountBalances(is.Expenses)
	is.NetIncome = is.TotalRevenue.Sub(is.TotalExpenses)
	return is
}

func collectTrialBalanceAccounts(entities []ConsolidatedTenantReport) []accounting.AccountBalance {
	accounts := make([]accounting.AccountBalance, 0)
	for _, entity := range entities {
		if entity.TrialBalance == nil {
			continue
		}
		accounts = append(accounts, entity.TrialBalance.Accounts...)
	}
	return accounts
}

func aggregateAccountBalances(accounts []accounting.AccountBalance) []accounting.AccountBalance {
	type accountKey struct {
		code        string
		accountType accounting.AccountType
	}
	byAccount := make(map[accountKey]*accounting.AccountBalance)
	order := make([]accountKey, 0)
	for _, account := range accounts {
		key := accountKey{code: account.AccountCode, accountType: account.AccountType}
		agg, ok := byAccount[key]
		if !ok {
			copy := account
			copy.AccountID = account.AccountCode
			copy.DebitBalance = decimal.Zero
			copy.CreditBalance = decimal.Zero
			copy.NetBalance = decimal.Zero
			byAccount[key] = &copy
			order = append(order, key)
			agg = &copy
		}
		agg.DebitBalance = agg.DebitBalance.Add(account.DebitBalance)
		agg.CreditBalance = agg.CreditBalance.Add(account.CreditBalance)
		agg.NetBalance = agg.NetBalance.Add(account.NetBalance)
		byAccount[key] = agg
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].code == order[j].code {
			return order[i].accountType < order[j].accountType
		}
		return order[i].code < order[j].code
	})

	result := make([]accounting.AccountBalance, 0, len(order))
	for _, key := range order {
		result = append(result, *byAccount[key])
	}
	return result
}
