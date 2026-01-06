package analytics

import (
	"time"

	"github.com/shopspring/decimal"
)

// DashboardSummary contains key metrics for the dashboard
type DashboardSummary struct {
	// Revenue and expenses
	TotalRevenue   decimal.Decimal `json:"total_revenue"`
	TotalExpenses  decimal.Decimal `json:"total_expenses"`
	NetIncome      decimal.Decimal `json:"net_income"`
	RevenueChange  decimal.Decimal `json:"revenue_change"`  // % change from previous period
	ExpensesChange decimal.Decimal `json:"expenses_change"` // % change from previous period

	// Receivables and payables
	TotalReceivables   decimal.Decimal `json:"total_receivables"`
	TotalPayables      decimal.Decimal `json:"total_payables"`
	OverdueReceivables decimal.Decimal `json:"overdue_receivables"`
	OverduePayables    decimal.Decimal `json:"overdue_payables"`

	// Invoice counts
	DraftInvoices   int `json:"draft_invoices"`
	PendingInvoices int `json:"pending_invoices"`
	OverdueInvoices int `json:"overdue_invoices"`

	// Period
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
}

// ChartDataPoint represents a single data point in a chart
type ChartDataPoint struct {
	Label string          `json:"label"`
	Value decimal.Decimal `json:"value"`
}

// RevenueExpenseChart contains data for revenue vs expense chart
type RevenueExpenseChart struct {
	Labels   []string          `json:"labels"`
	Revenue  []decimal.Decimal `json:"revenue"`
	Expenses []decimal.Decimal `json:"expenses"`
	Profit   []decimal.Decimal `json:"profit"` // Revenue - Expenses
}

// CashFlowChart contains data for cash flow chart
type CashFlowChart struct {
	Labels   []string          `json:"labels"`
	Inflows  []decimal.Decimal `json:"inflows"`
	Outflows []decimal.Decimal `json:"outflows"`
	Net      []decimal.Decimal `json:"net"`
}

// AgingBucket represents an aging period bucket
type AgingBucket struct {
	Label  string          `json:"label"`
	Amount decimal.Decimal `json:"amount"`
	Count  int             `json:"count"`
}

// AgingReport contains receivables or payables aging data
type AgingReport struct {
	ReportType string          `json:"report_type"` // "receivables" or "payables"
	AsOfDate   time.Time       `json:"as_of_date"`
	Total      decimal.Decimal `json:"total"`
	Buckets    []AgingBucket   `json:"buckets"`
	ByContact  []ContactAging  `json:"by_contact,omitempty"`
}

// ContactAging shows aging for a specific contact
type ContactAging struct {
	ContactID   string          `json:"contact_id"`
	ContactName string          `json:"contact_name"`
	Current     decimal.Decimal `json:"current"`
	Days1to30   decimal.Decimal `json:"days_1_30"`
	Days31to60  decimal.Decimal `json:"days_31_60"`
	Days61to90  decimal.Decimal `json:"days_61_90"`
	Days90Plus  decimal.Decimal `json:"days_90_plus"`
	Total       decimal.Decimal `json:"total"`
}

// TopItem represents a top customer/supplier/product
type TopItem struct {
	ID     string          `json:"id"`
	Name   string          `json:"name"`
	Amount decimal.Decimal `json:"amount"`
	Count  int             `json:"count"`
}

// AccountBalanceWidget shows account balance summary
type AccountBalanceWidget struct {
	AccountID   string          `json:"account_id"`
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Balance     decimal.Decimal `json:"balance"`
	Currency    string          `json:"currency"`
}
