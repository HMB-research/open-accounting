package analytics

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides analytics and reporting functionality
type Service struct {
	pool *pgxpool.Pool
	repo Repository
}

// NewService creates a new analytics service
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		pool: pool,
		repo: NewPostgresRepository(pool),
	}
}

// NewServiceWithRepository creates a new analytics service with a custom repository (for testing)
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// GetDashboardSummary returns key metrics for the dashboard
func (s *Service) GetDashboardSummary(ctx context.Context, tenantID, schemaName string) (*DashboardSummary, error) {
	now := time.Now()
	periodEnd := now
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	prevPeriodStart := periodStart.AddDate(0, -1, 0)
	prevPeriodEnd := periodStart.Add(-time.Second)

	summary := &DashboardSummary{
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}

	// Get current period revenue and expenses
	revenue, expenses, err := s.repo.GetRevenueExpenses(ctx, schemaName, periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	summary.TotalRevenue = revenue
	summary.TotalExpenses = expenses
	summary.NetIncome = revenue.Sub(expenses)

	// Get previous period for comparison
	prevRevenue, prevExpenses, err := s.repo.GetRevenueExpenses(ctx, schemaName, prevPeriodStart, prevPeriodEnd)
	if err == nil && !prevRevenue.IsZero() {
		summary.RevenueChange = revenue.Sub(prevRevenue).Div(prevRevenue).Mul(decimal.NewFromInt(100)).Round(2)
	}
	if err == nil && !prevExpenses.IsZero() {
		summary.ExpensesChange = expenses.Sub(prevExpenses).Div(prevExpenses).Mul(decimal.NewFromInt(100)).Round(2)
	}

	// Get receivables and payables
	summary.TotalReceivables, summary.OverdueReceivables, err = s.repo.GetReceivablesSummary(ctx, schemaName)
	if err != nil {
		return nil, err
	}

	summary.TotalPayables, summary.OverduePayables, err = s.repo.GetPayablesSummary(ctx, schemaName)
	if err != nil {
		return nil, err
	}

	// Get invoice counts
	summary.DraftInvoices, summary.PendingInvoices, summary.OverdueInvoices, err = s.repo.GetInvoiceCounts(ctx, schemaName)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

// GetRevenueExpenseChart returns monthly revenue and expense data
func (s *Service) GetRevenueExpenseChart(ctx context.Context, tenantID, schemaName string, months int) (*RevenueExpenseChart, error) {
	if months <= 0 {
		months = 12
	}

	data, err := s.repo.GetMonthlyRevenueExpenses(ctx, schemaName, months)
	if err != nil {
		return nil, err
	}

	chart := &RevenueExpenseChart{
		Labels:   make([]string, 0, len(data)),
		Revenue:  make([]decimal.Decimal, 0, len(data)),
		Expenses: make([]decimal.Decimal, 0, len(data)),
		Profit:   make([]decimal.Decimal, 0, len(data)),
	}

	for _, d := range data {
		chart.Labels = append(chart.Labels, d.Label)
		chart.Revenue = append(chart.Revenue, d.Revenue)
		chart.Expenses = append(chart.Expenses, d.Expenses)
		chart.Profit = append(chart.Profit, d.Revenue.Sub(d.Expenses))
	}

	return chart, nil
}

// GetCashFlowChart returns monthly cash flow data
func (s *Service) GetCashFlowChart(ctx context.Context, tenantID, schemaName string, months int) (*CashFlowChart, error) {
	if months <= 0 {
		months = 12
	}

	data, err := s.repo.GetMonthlyCashFlow(ctx, schemaName, months)
	if err != nil {
		return nil, err
	}

	chart := &CashFlowChart{
		Labels:   make([]string, 0, len(data)),
		Inflows:  make([]decimal.Decimal, 0, len(data)),
		Outflows: make([]decimal.Decimal, 0, len(data)),
		Net:      make([]decimal.Decimal, 0, len(data)),
	}

	for _, d := range data {
		chart.Labels = append(chart.Labels, d.Label)
		chart.Inflows = append(chart.Inflows, d.Inflows)
		chart.Outflows = append(chart.Outflows, d.Outflows)
		chart.Net = append(chart.Net, d.Inflows.Sub(d.Outflows))
	}

	return chart, nil
}

// GetReceivablesAging returns aging report for receivables
func (s *Service) GetReceivablesAging(ctx context.Context, tenantID, schemaName string) (*AgingReport, error) {
	return s.getAgingReport(ctx, schemaName, "SALES", "receivables")
}

// GetPayablesAging returns aging report for payables
func (s *Service) GetPayablesAging(ctx context.Context, tenantID, schemaName string) (*AgingReport, error) {
	return s.getAgingReport(ctx, schemaName, "PURCHASE", "payables")
}

func (s *Service) getAgingReport(ctx context.Context, schemaName, invoiceType, reportType string) (*AgingReport, error) {
	contacts, err := s.repo.GetAgingByContact(ctx, schemaName, invoiceType)
	if err != nil {
		return nil, err
	}

	report := &AgingReport{
		ReportType: reportType,
		AsOfDate:   time.Now(),
		ByContact:  contacts,
		Buckets: []AgingBucket{
			{Label: "Current", Amount: decimal.Zero, Count: 0},
			{Label: "1-30 Days", Amount: decimal.Zero, Count: 0},
			{Label: "31-60 Days", Amount: decimal.Zero, Count: 0},
			{Label: "61-90 Days", Amount: decimal.Zero, Count: 0},
			{Label: "90+ Days", Amount: decimal.Zero, Count: 0},
		},
	}

	for _, ca := range contacts {
		// Update buckets
		if !ca.Current.IsZero() {
			report.Buckets[0].Amount = report.Buckets[0].Amount.Add(ca.Current)
			report.Buckets[0].Count++
		}
		if !ca.Days1to30.IsZero() {
			report.Buckets[1].Amount = report.Buckets[1].Amount.Add(ca.Days1to30)
			report.Buckets[1].Count++
		}
		if !ca.Days31to60.IsZero() {
			report.Buckets[2].Amount = report.Buckets[2].Amount.Add(ca.Days31to60)
			report.Buckets[2].Count++
		}
		if !ca.Days61to90.IsZero() {
			report.Buckets[3].Amount = report.Buckets[3].Amount.Add(ca.Days61to90)
			report.Buckets[3].Count++
		}
		if !ca.Days90Plus.IsZero() {
			report.Buckets[4].Amount = report.Buckets[4].Amount.Add(ca.Days90Plus)
			report.Buckets[4].Count++
		}

		report.Total = report.Total.Add(ca.Total)
	}

	return report, nil
}

// GetTopCustomers returns top customers by revenue
func (s *Service) GetTopCustomers(ctx context.Context, tenantID, schemaName string, limit int) ([]TopItem, error) {
	if limit <= 0 {
		limit = 10
	}

	return s.repo.GetTopCustomers(ctx, schemaName, limit)
}

// GetRecentActivity returns recent activity from invoices, payments, journal entries, and contacts
func (s *Service) GetRecentActivity(ctx context.Context, tenantID, schemaName string, limit int) ([]ActivityItem, error) {
	if limit <= 0 {
		limit = 10
	}

	return s.repo.GetRecentActivity(ctx, schemaName, limit)
}
