package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Repository defines the contract for analytics data access
type Repository interface {
	// Summary queries
	GetRevenueExpenses(ctx context.Context, schemaName string, start, end time.Time) (revenue, expenses decimal.Decimal, err error)
	GetReceivablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error)
	GetPayablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error)
	GetInvoiceCounts(ctx context.Context, schemaName string) (draft, pending, overdue int, err error)

	// Chart queries
	GetMonthlyRevenueExpenses(ctx context.Context, schemaName string, months int) ([]MonthlyData, error)
	GetMonthlyCashFlow(ctx context.Context, schemaName string, months int) ([]MonthlyCashFlowData, error)

	// Aging queries
	GetAgingByContact(ctx context.Context, schemaName, invoiceType string) ([]ContactAging, error)

	// Top items
	GetTopCustomers(ctx context.Context, schemaName string, limit int) ([]TopItem, error)

	// Activity feed
	GetRecentActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error)
}

// MonthlyData represents monthly financial data
type MonthlyData struct {
	Label    string
	Revenue  decimal.Decimal
	Expenses decimal.Decimal
}

// MonthlyCashFlowData represents monthly cash flow data
type MonthlyCashFlowData struct {
	Label    string
	Inflows  decimal.Decimal
	Outflows decimal.Decimal
}

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// GetRevenueExpenses retrieves revenue and expenses for a period
func (r *PostgresRepository) GetRevenueExpenses(ctx context.Context, schemaName string, start, end time.Time) (revenue, expenses decimal.Decimal, err error) {
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

	err = r.pool.QueryRow(ctx, query, start, end).Scan(&revenue, &expenses)
	return
}

// GetReceivablesSummary retrieves receivables totals
func (r *PostgresRepository) GetReceivablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error) {
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(total - amount_paid), 0) as total,
			COALESCE(SUM(CASE WHEN due_date < CURRENT_DATE THEN total - amount_paid ELSE 0 END), 0) as overdue
		FROM %s.invoices
		WHERE invoice_type = 'SALES' AND status NOT IN ('PAID', 'VOIDED')
	`, schemaName)

	err = r.pool.QueryRow(ctx, query).Scan(&total, &overdue)
	return
}

// GetPayablesSummary retrieves payables totals
func (r *PostgresRepository) GetPayablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error) {
	query := fmt.Sprintf(`
		SELECT
			COALESCE(SUM(total - amount_paid), 0) as total,
			COALESCE(SUM(CASE WHEN due_date < CURRENT_DATE THEN total - amount_paid ELSE 0 END), 0) as overdue
		FROM %s.invoices
		WHERE invoice_type = 'PURCHASE' AND status NOT IN ('PAID', 'VOIDED')
	`, schemaName)

	err = r.pool.QueryRow(ctx, query).Scan(&total, &overdue)
	return
}

// GetInvoiceCounts retrieves invoice counts by status
func (r *PostgresRepository) GetInvoiceCounts(ctx context.Context, schemaName string) (draft, pending, overdue int, err error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE status = 'DRAFT') as draft,
			COUNT(*) FILTER (WHERE status IN ('SENT', 'PARTIALLY_PAID')) as pending,
			COUNT(*) FILTER (WHERE status NOT IN ('PAID', 'VOIDED') AND due_date < CURRENT_DATE) as overdue
		FROM %s.invoices
		WHERE invoice_type = 'SALES'
	`, schemaName)

	err = r.pool.QueryRow(ctx, query).Scan(&draft, &pending, &overdue)
	return
}

// GetMonthlyRevenueExpenses retrieves monthly revenue and expense data
func (r *PostgresRepository) GetMonthlyRevenueExpenses(ctx context.Context, schemaName string, months int) ([]MonthlyData, error) {
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

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MonthlyData
	for rows.Next() {
		var data MonthlyData
		if err := rows.Scan(&data.Label, &data.Revenue, &data.Expenses); err != nil {
			return nil, err
		}
		results = append(results, data)
	}
	return results, nil
}

// GetMonthlyCashFlow retrieves monthly cash flow data
func (r *PostgresRepository) GetMonthlyCashFlow(ctx context.Context, schemaName string, months int) ([]MonthlyCashFlowData, error) {
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

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MonthlyCashFlowData
	for rows.Next() {
		var data MonthlyCashFlowData
		if err := rows.Scan(&data.Label, &data.Inflows, &data.Outflows); err != nil {
			return nil, err
		}
		results = append(results, data)
	}
	return results, nil
}

// GetAgingByContact retrieves aging data grouped by contact
func (r *PostgresRepository) GetAgingByContact(ctx context.Context, schemaName, invoiceType string) ([]ContactAging, error) {
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

	rows, err := r.pool.Query(ctx, query, invoiceType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ContactAging
	for rows.Next() {
		var ca ContactAging
		if err := rows.Scan(&ca.ContactID, &ca.ContactName, &ca.Current, &ca.Days1to30, &ca.Days31to60, &ca.Days61to90, &ca.Days90Plus); err != nil {
			return nil, err
		}
		ca.Total = ca.Current.Add(ca.Days1to30).Add(ca.Days31to60).Add(ca.Days61to90).Add(ca.Days90Plus)
		results = append(results, ca)
	}
	return results, nil
}

// GetTopCustomers retrieves top customers by revenue
func (r *PostgresRepository) GetTopCustomers(ctx context.Context, schemaName string, limit int) ([]TopItem, error) {
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

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []TopItem
	for rows.Next() {
		var item TopItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Amount, &item.Count); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// GetRecentActivity retrieves recent activity from invoices, payments, journal entries, and contacts
func (r *PostgresRepository) GetRecentActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error) {
	query := fmt.Sprintf(`
		WITH activity AS (
			-- Recent invoices
			SELECT
				i.id::text as id,
				'INVOICE' as type,
				CASE
					WHEN i.status = 'DRAFT' THEN 'created'
					WHEN i.status = 'SENT' THEN 'sent'
					WHEN i.status = 'PAID' THEN 'paid'
					WHEN i.status = 'VOIDED' THEN 'voided'
					ELSE 'updated'
				END as action,
				CASE
					WHEN i.invoice_type = 'SALES' THEN 'Invoice ' || i.invoice_number || ' to ' || c.name
					ELSE 'Bill ' || i.invoice_number || ' from ' || c.name
				END as description,
				i.created_at as created_at,
				i.total as amount
			FROM %[1]s.invoices i
			LEFT JOIN %[1]s.contacts c ON i.contact_id = c.id

			UNION ALL

			-- Recent payments
			SELECT
				p.id::text as id,
				'PAYMENT' as type,
				CASE
					WHEN p.payment_type = 'RECEIVED' THEN 'received'
					ELSE 'made'
				END as action,
				CASE
					WHEN p.payment_type = 'RECEIVED' THEN 'Payment received from ' || COALESCE(c.name, 'Unknown')
					ELSE 'Payment made to ' || COALESCE(c.name, 'Unknown')
				END as description,
				p.payment_date::timestamptz as created_at,
				p.amount as amount
			FROM %[1]s.payments p
			LEFT JOIN %[1]s.invoices i ON p.invoice_id = i.id
			LEFT JOIN %[1]s.contacts c ON i.contact_id = c.id

			UNION ALL

			-- Recent journal entries
			SELECT
				je.id::text as id,
				'ENTRY' as type,
				CASE
					WHEN je.status = 'POSTED' THEN 'posted'
					ELSE 'created'
				END as action,
				'Journal entry: ' || COALESCE(je.description, je.reference) as description,
				je.created_at as created_at,
				(SELECT COALESCE(SUM(base_debit), 0) FROM %[1]s.journal_entry_lines WHERE journal_entry_id = je.id) as amount
			FROM %[1]s.journal_entries je
			WHERE je.description IS NOT NULL OR je.reference IS NOT NULL

			UNION ALL

			-- Recent contacts
			SELECT
				c.id::text as id,
				'CONTACT' as type,
				'created' as action,
				'New contact: ' || c.name as description,
				c.created_at as created_at,
				NULL::numeric as amount
			FROM %[1]s.contacts c
		)
		SELECT id, type, action, description, created_at, amount
		FROM activity
		WHERE created_at IS NOT NULL
		ORDER BY created_at DESC
		LIMIT $1
	`, schemaName)

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ActivityItem
	for rows.Next() {
		var item ActivityItem
		var amount *decimal.Decimal
		if err := rows.Scan(&item.ID, &item.Type, &item.Action, &item.Description, &item.CreatedAt, &amount); err != nil {
			return nil, err
		}
		item.Amount = amount
		items = append(items, item)
	}
	return items, nil
}
