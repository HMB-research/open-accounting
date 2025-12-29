package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides analytics and reporting functionality
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates a new analytics service
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
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
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN a.account_type = 'REVENUE' THEN jel.base_credit - jel.base_debit ELSE 0 END), 0) as revenue,
			COALESCE(SUM(CASE WHEN a.account_type = 'EXPENSE' THEN jel.base_debit - jel.base_credit ELSE 0 END), 0) as expenses
		FROM %s.journal_entry_lines jel
		JOIN %s.journal_entries je ON jel.journal_entry_id = je.id
		JOIN %s.accounts a ON jel.account_id = a.id
		WHERE je.status = 'POSTED'
		  AND je.entry_date >= $1 AND je.entry_date <= $2
	`, schemaName, schemaName, schemaName)

	var revenue, expenses decimal.Decimal
	err := s.pool.QueryRow(ctx, query, periodStart, periodEnd).Scan(&revenue, &expenses)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue/expenses: %w", err)
	}
	summary.TotalRevenue = revenue
	summary.TotalExpenses = expenses
	summary.NetIncome = revenue.Sub(expenses)

	// Get previous period for comparison
	var prevRevenue, prevExpenses decimal.Decimal
	err = s.pool.QueryRow(ctx, query, prevPeriodStart, prevPeriodEnd).Scan(&prevRevenue, &prevExpenses)
	if err == nil && !prevRevenue.IsZero() {
		summary.RevenueChange = revenue.Sub(prevRevenue).Div(prevRevenue).Mul(decimal.NewFromInt(100)).Round(2)
	}
	if err == nil && !prevExpenses.IsZero() {
		summary.ExpensesChange = expenses.Sub(prevExpenses).Div(prevExpenses).Mul(decimal.NewFromInt(100)).Round(2)
	}

	// Get receivables and payables
	receivablesQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(total - amount_paid), 0) as total,
			COALESCE(SUM(CASE WHEN due_date < CURRENT_DATE THEN total - amount_paid ELSE 0 END), 0) as overdue
		FROM %s.invoices
		WHERE invoice_type = 'SALES' AND status NOT IN ('PAID', 'VOIDED')
	`, schemaName)

	err = s.pool.QueryRow(ctx, receivablesQuery).Scan(&summary.TotalReceivables, &summary.OverdueReceivables)
	if err != nil {
		return nil, fmt.Errorf("failed to get receivables: %w", err)
	}

	payablesQuery := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(total - amount_paid), 0) as total,
			COALESCE(SUM(CASE WHEN due_date < CURRENT_DATE THEN total - amount_paid ELSE 0 END), 0) as overdue
		FROM %s.invoices
		WHERE invoice_type = 'PURCHASE' AND status NOT IN ('PAID', 'VOIDED')
	`, schemaName)

	err = s.pool.QueryRow(ctx, payablesQuery).Scan(&summary.TotalPayables, &summary.OverduePayables)
	if err != nil {
		return nil, fmt.Errorf("failed to get payables: %w", err)
	}

	// Get invoice counts
	countQuery := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'DRAFT') as draft,
			COUNT(*) FILTER (WHERE status IN ('SENT', 'PARTIALLY_PAID')) as pending,
			COUNT(*) FILTER (WHERE status NOT IN ('PAID', 'VOIDED') AND due_date < CURRENT_DATE) as overdue
		FROM %s.invoices
		WHERE invoice_type = 'SALES'
	`, schemaName)

	err = s.pool.QueryRow(ctx, countQuery).Scan(&summary.DraftInvoices, &summary.PendingInvoices, &summary.OverdueInvoices)
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice counts: %w", err)
	}

	return summary, nil
}

// GetRevenueExpenseChart returns monthly revenue and expense data
func (s *Service) GetRevenueExpenseChart(ctx context.Context, tenantID, schemaName string, months int) (*RevenueExpenseChart, error) {
	if months <= 0 {
		months = 12
	}

	query := fmt.Sprintf(`
		WITH months AS (
			SELECT generate_series(
				date_trunc('month', CURRENT_DATE - interval '%d months'),
				date_trunc('month', CURRENT_DATE),
				interval '1 month'
			) as month
		)
		SELECT
			to_char(m.month, 'Mon YYYY') as label,
			COALESCE(SUM(CASE WHEN a.account_type = 'REVENUE' THEN jel.base_credit - jel.base_debit ELSE 0 END), 0) as revenue,
			COALESCE(SUM(CASE WHEN a.account_type = 'EXPENSE' THEN jel.base_debit - jel.base_credit ELSE 0 END), 0) as expenses
		FROM months m
		LEFT JOIN %s.journal_entries je ON date_trunc('month', je.entry_date) = m.month AND je.status = 'POSTED'
		LEFT JOIN %s.journal_entry_lines jel ON jel.journal_entry_id = je.id
		LEFT JOIN %s.accounts a ON jel.account_id = a.id
		GROUP BY m.month
		ORDER BY m.month
	`, months-1, schemaName, schemaName, schemaName)

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue/expense chart: %w", err)
	}
	defer rows.Close()

	chart := &RevenueExpenseChart{
		Labels:   make([]string, 0),
		Revenue:  make([]decimal.Decimal, 0),
		Expenses: make([]decimal.Decimal, 0),
	}

	for rows.Next() {
		var label string
		var revenue, expenses decimal.Decimal
		if err := rows.Scan(&label, &revenue, &expenses); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		chart.Labels = append(chart.Labels, label)
		chart.Revenue = append(chart.Revenue, revenue)
		chart.Expenses = append(chart.Expenses, expenses)
	}

	return chart, nil
}

// GetCashFlowChart returns monthly cash flow data
func (s *Service) GetCashFlowChart(ctx context.Context, tenantID, schemaName string, months int) (*CashFlowChart, error) {
	if months <= 0 {
		months = 12
	}

	query := fmt.Sprintf(`
		WITH months AS (
			SELECT generate_series(
				date_trunc('month', CURRENT_DATE - interval '%d months'),
				date_trunc('month', CURRENT_DATE),
				interval '1 month'
			) as month
		)
		SELECT
			to_char(m.month, 'Mon YYYY') as label,
			COALESCE(SUM(CASE WHEN p.payment_type = 'RECEIVED' THEN p.base_amount ELSE 0 END), 0) as inflows,
			COALESCE(SUM(CASE WHEN p.payment_type = 'MADE' THEN p.base_amount ELSE 0 END), 0) as outflows
		FROM months m
		LEFT JOIN %s.payments p ON date_trunc('month', p.payment_date) = m.month
		GROUP BY m.month
		ORDER BY m.month
	`, months-1, schemaName)

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get cash flow chart: %w", err)
	}
	defer rows.Close()

	chart := &CashFlowChart{
		Labels:   make([]string, 0),
		Inflows:  make([]decimal.Decimal, 0),
		Outflows: make([]decimal.Decimal, 0),
		Net:      make([]decimal.Decimal, 0),
	}

	for rows.Next() {
		var label string
		var inflows, outflows decimal.Decimal
		if err := rows.Scan(&label, &inflows, &outflows); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		chart.Labels = append(chart.Labels, label)
		chart.Inflows = append(chart.Inflows, inflows)
		chart.Outflows = append(chart.Outflows, outflows)
		chart.Net = append(chart.Net, inflows.Sub(outflows))
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
	query := fmt.Sprintf(`
		SELECT
			c.id as contact_id,
			c.name as contact_name,
			COALESCE(SUM(CASE WHEN i.due_date >= CURRENT_DATE THEN i.total - i.amount_paid ELSE 0 END), 0) as current,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE AND i.due_date >= CURRENT_DATE - 30 THEN i.total - i.amount_paid ELSE 0 END), 0) as days_1_30,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE - 30 AND i.due_date >= CURRENT_DATE - 60 THEN i.total - i.amount_paid ELSE 0 END), 0) as days_31_60,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE - 60 AND i.due_date >= CURRENT_DATE - 90 THEN i.total - i.amount_paid ELSE 0 END), 0) as days_61_90,
			COALESCE(SUM(CASE WHEN i.due_date < CURRENT_DATE - 90 THEN i.total - i.amount_paid ELSE 0 END), 0) as days_90_plus
		FROM %s.invoices i
		JOIN %s.contacts c ON i.contact_id = c.id
		WHERE i.invoice_type = $1 AND i.status NOT IN ('PAID', 'VOIDED')
		GROUP BY c.id, c.name
		HAVING SUM(i.total - i.amount_paid) > 0
		ORDER BY SUM(i.total - i.amount_paid) DESC
	`, schemaName, schemaName)

	rows, err := s.pool.Query(ctx, query, invoiceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get aging report: %w", err)
	}
	defer rows.Close()

	report := &AgingReport{
		ReportType: reportType,
		AsOfDate:   time.Now(),
		ByContact:  make([]ContactAging, 0),
		Buckets: []AgingBucket{
			{Label: "Current", Amount: decimal.Zero, Count: 0},
			{Label: "1-30 Days", Amount: decimal.Zero, Count: 0},
			{Label: "31-60 Days", Amount: decimal.Zero, Count: 0},
			{Label: "61-90 Days", Amount: decimal.Zero, Count: 0},
			{Label: "90+ Days", Amount: decimal.Zero, Count: 0},
		},
	}

	for rows.Next() {
		var ca ContactAging
		if err := rows.Scan(&ca.ContactID, &ca.ContactName, &ca.Current, &ca.Days1to30, &ca.Days31to60, &ca.Days61to90, &ca.Days90Plus); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		ca.Total = ca.Current.Add(ca.Days1to30).Add(ca.Days31to60).Add(ca.Days61to90).Add(ca.Days90Plus)
		report.ByContact = append(report.ByContact, ca)

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

	query := fmt.Sprintf(`
		SELECT
			c.id,
			c.name,
			COALESCE(SUM(i.total), 0) as amount,
			COUNT(i.id) as count
		FROM %s.contacts c
		LEFT JOIN %s.invoices i ON i.contact_id = c.id AND i.invoice_type = 'SALES' AND i.status != 'VOIDED'
		WHERE c.contact_type IN ('CUSTOMER', 'BOTH')
		GROUP BY c.id, c.name
		ORDER BY amount DESC
		LIMIT $1
	`, schemaName, schemaName)

	rows, err := s.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top customers: %w", err)
	}
	defer rows.Close()

	items := make([]TopItem, 0)
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Amount, &item.Count); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		items = append(items, item)
	}

	return items, nil
}
