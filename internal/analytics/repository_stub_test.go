package analytics

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestNewRepositoryPoolConversion(t *testing.T) {
	original := newAnalyticsGormDBFromPool
	t.Cleanup(func() { newAnalyticsGormDBFromPool = original })

	gormDB, _ := newAnalyticsStubGormDB(t)
	pool := &pgxpool.Pool{}
	newAnalyticsGormDBFromPool = func(ctx context.Context, receivedPool *pgxpool.Pool) (*gorm.DB, error) {
		assert.Equal(t, context.Background(), ctx)
		assert.Same(t, pool, receivedPool)
		return gormDB, nil
	}

	repository := NewRepository(pool)

	require.NotNil(t, repository)
	assert.Same(t, gormDB, repository.db)
}

func TestNewRepositoryPanicsWhenPoolConversionFails(t *testing.T) {
	original := newAnalyticsGormDBFromPool
	t.Cleanup(func() { newAnalyticsGormDBFromPool = original })

	newAnalyticsGormDBFromPool = func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("convert failed")
	}

	require.PanicsWithError(t, "create analytics GORM repository: convert failed", func() {
		NewRepository(&pgxpool.Pool{})
	})
}

func TestGORMRepositoryScansSummaryQueries(t *testing.T) {
	ctx := context.Background()
	schema := "tenant_schema"

	t.Run("revenue expenses", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t, analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."journal_entry_lines" AS jel`, `JOIN "tenant_schema"."journal_entries" AS je`, `JOIN "tenant_schema"."accounts" AS a`},
			columns:  []string{"revenue", "expenses"},
			rows:     [][]driver.Value{{"1200.50", "700.25"}},
		})

		revenue, expenses, err := repo.GetRevenueExpenses(ctx, schema, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))

		require.NoError(t, err)
		assert.True(t, revenue.Equal(decimal.RequireFromString("1200.50")))
		assert.True(t, expenses.Equal(decimal.RequireFromString("700.25")))
		stub.requireExhausted(t)
	})

	t.Run("receivables summary", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t, analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."invoices"`, "SUM(CASE WHEN due_date < CURRENT_DATE"},
			columns:  []string{"total", "overdue"},
			rows:     [][]driver.Value{{"500.00", "125.50"}},
		})

		total, overdue, err := repo.GetReceivablesSummary(ctx, schema)

		require.NoError(t, err)
		assert.True(t, total.Equal(decimal.RequireFromString("500.00")))
		assert.True(t, overdue.Equal(decimal.RequireFromString("125.50")))
		stub.requireExhausted(t)
	})

	t.Run("invoice counts", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t, analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."invoices"`, "COUNT(*) FILTER"},
			columns:  []string{"draft", "pending", "overdue"},
			rows:     [][]driver.Value{{int64(2), int64(5), int64(1)}},
		})

		draft, pending, overdue, err := repo.GetInvoiceCounts(ctx, schema)

		require.NoError(t, err)
		assert.Equal(t, 2, draft)
		assert.Equal(t, 5, pending)
		assert.Equal(t, 1, overdue)
		stub.requireExhausted(t)
	})
}

func TestGORMRepositoryScansMonthlyQueries(t *testing.T) {
	ctx := context.Background()
	schema := "tenant_schema"
	monthStarts := recentMonthStarts(3)
	require.Len(t, monthStarts, 3)

	t.Run("revenue expenses fills missing months", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t, analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."journal_entry_lines" AS jel`, "date_trunc('month', je.entry_date)"},
			columns:  []string{"month", "revenue", "expenses"},
			rows:     [][]driver.Value{{monthStarts[1], "900.00", "300.00"}},
		})

		data, err := repo.GetMonthlyRevenueExpenses(ctx, schema, 3)

		require.NoError(t, err)
		require.Len(t, data, 3)
		assert.Equal(t, monthLabel(monthStarts[0]), data[0].Label)
		assert.True(t, data[0].Revenue.IsZero())
		assert.True(t, data[0].Expenses.IsZero())
		assert.Equal(t, monthLabel(monthStarts[1]), data[1].Label)
		assert.True(t, data[1].Revenue.Equal(decimal.RequireFromString("900.00")))
		assert.True(t, data[1].Expenses.Equal(decimal.RequireFromString("300.00")))
		assert.Equal(t, monthLabel(monthStarts[2]), data[2].Label)
		assert.True(t, data[2].Revenue.IsZero())
		assert.True(t, data[2].Expenses.IsZero())
		stub.requireExhausted(t)
	})

	t.Run("cash flow fills missing months", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t,
			analyticsStubQuery{
				contains: []string{`FROM "tenant_schema"."bank_transactions" AS bt`, "date_trunc('month', bt.transaction_date)", "bt.amount > 0", "bt.amount < 0"},
				columns:  []string{"month", "inflows", "outflows"},
				rows:     [][]driver.Value{{monthStarts[2], "450.00", "120.00"}},
			},
			analyticsStubQuery{
				contains: []string{`FROM "tenant_schema"."payments" AS p`, "date_trunc('month', p.payment_date)", "p.payment_method", "NOT IN"},
				columns:  []string{"month", "inflows", "outflows"},
				rows:     [][]driver.Value{{monthStarts[1], "50.00", "10.00"}, {monthStarts[2], "999.00", "999.00"}},
			},
			analyticsStubQuery{
				contains: []string{"WITH cash_entry_movements", `FROM "tenant_schema"."journal_entries" AS je`, "a.code ~ '^10[0-9]+$'", "movement > 0", "movement < 0"},
				columns:  []string{"month", "inflows", "outflows"},
				rows:     [][]driver.Value{{monthStarts[0], "75.00", "25.00"}, {monthStarts[1], "999.00", "999.00"}},
			},
		)

		data, err := repo.GetMonthlyCashFlow(ctx, schema, 3)

		require.NoError(t, err)
		require.Len(t, data, 3)
		assert.Equal(t, monthLabel(monthStarts[0]), data[0].Label)
		assert.True(t, data[0].Inflows.Equal(decimal.RequireFromString("75.00")))
		assert.True(t, data[0].Outflows.Equal(decimal.RequireFromString("25.00")))
		assert.Equal(t, monthLabel(monthStarts[1]), data[1].Label)
		assert.True(t, data[1].Inflows.Equal(decimal.RequireFromString("50.00")))
		assert.True(t, data[1].Outflows.Equal(decimal.RequireFromString("10.00")))
		assert.Equal(t, monthLabel(monthStarts[2]), data[2].Label)
		assert.True(t, data[2].Inflows.Equal(decimal.RequireFromString("450.00")))
		assert.True(t, data[2].Outflows.Equal(decimal.RequireFromString("120.00")))
		stub.requireExhausted(t)
	})

	t.Run("cash flow propagates payment query errors", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t,
			analyticsStubQuery{
				contains: []string{`FROM "tenant_schema"."bank_transactions" AS bt`, "date_trunc('month', bt.transaction_date)"},
				columns:  []string{"month", "inflows", "outflows"},
			},
			analyticsStubQuery{
				contains: []string{`FROM "tenant_schema"."payments" AS p`, "date_trunc('month', p.payment_date)"},
				err:      errors.New("payment query failed"),
			},
		)

		data, err := repo.GetMonthlyCashFlow(ctx, schema, 3)

		require.Error(t, err)
		assert.Nil(t, data)
		assert.Contains(t, err.Error(), "get monthly cash flow")
		stub.requireExhausted(t)
	})
}

func TestGORMRepositoryExactCashFlowPeriodAndLedgerErrors(t *testing.T) {
	ctx := context.Background()
	schema := "tenant_schema"
	start := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC)

	t.Run("fills exact period with zero months", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t,
			analyticsStubQuery{contains: []string{`FROM "tenant_schema"."bank_transactions" AS bt`}, columns: []string{"month", "inflows", "outflows"}},
			analyticsStubQuery{contains: []string{`FROM "tenant_schema"."payments" AS p`}, columns: []string{"month", "inflows", "outflows"}},
			analyticsStubQuery{contains: []string{"WITH cash_entry_movements"}, columns: []string{"month", "inflows", "outflows"}},
		)

		data, err := repo.GetMonthlyCashFlowForPeriod(ctx, schema, start, end)

		require.NoError(t, err)
		require.Len(t, data, 2)
		assert.Equal(t, "Jan 2026", data[0].Label)
		assert.True(t, data[0].Inflows.IsZero())
		assert.True(t, data[0].Outflows.IsZero())
		assert.Equal(t, "Feb 2026", data[1].Label)
		stub.requireExhausted(t)
	})

	t.Run("rejects reversed period without querying", func(t *testing.T) {
		repo := NewGORMRepository(nil)
		data, err := repo.GetMonthlyCashFlowForPeriod(ctx, schema, end, start)
		require.NoError(t, err)
		assert.Empty(t, data)
	})

	t.Run("empty internal month selection does not query", func(t *testing.T) {
		repo := NewGORMRepository(nil)
		data, err := repo.getMonthlyCashFlow(ctx, schema, nil, start, end)
		require.NoError(t, err)
		assert.Empty(t, data)
	})

	t.Run("propagates ledger query errors", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t,
			analyticsStubQuery{contains: []string{`FROM "tenant_schema"."bank_transactions" AS bt`}, columns: []string{"month", "inflows", "outflows"}},
			analyticsStubQuery{contains: []string{`FROM "tenant_schema"."payments" AS p`}, columns: []string{"month", "inflows", "outflows"}},
			analyticsStubQuery{contains: []string{"WITH cash_entry_movements"}, err: errors.New("ledger query failed")},
		)

		data, err := repo.GetMonthlyCashFlowForPeriod(ctx, schema, start, end)

		require.ErrorContains(t, err, "get monthly ledger cash flow")
		assert.Nil(t, data)
		stub.requireExhausted(t)
	})
}

func TestIsMigrationSettlementPaymentMethod(t *testing.T) {
	assert.True(t, IsMigrationSettlementPaymentMethod("CUTOVER_SETTLEMENT"))
	assert.True(t, IsMigrationSettlementPaymentMethod(" migration_settlement "))
	assert.False(t, IsMigrationSettlementPaymentMethod("BANK_TRANSFER"))
	assert.False(t, IsMigrationSettlementPaymentMethod(""))
}

func TestGORMRepositoryScansAgingAndTopCustomers(t *testing.T) {
	ctx := context.Background()
	schema := "tenant_schema"

	t.Run("aging totals by contact", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t, analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."invoices" AS i`, `JOIN "tenant_schema"."contacts" AS c`},
			columns:  []string{"contact_id", "contact_name", "current", "days_1_30", "days_31_60", "days_61_90", "days_90_plus"},
			rows: [][]driver.Value{
				{"contact-1", "Acme OU", "10.00", "20.00", "30.00", "40.00", "50.00"},
			},
		})

		items, err := repo.GetAgingByContact(ctx, schema, string(models.InvoiceTypeSales))

		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "contact-1", items[0].ContactID)
		assert.Equal(t, "Acme OU", items[0].ContactName)
		assert.True(t, items[0].Current.Equal(decimal.RequireFromString("10.00")))
		assert.True(t, items[0].Total.Equal(decimal.RequireFromString("150.00")))
		stub.requireExhausted(t)
	})

	t.Run("top customers", func(t *testing.T) {
		repo, stub := newAnalyticsStubRepository(t, analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."contacts" AS c`, `LEFT JOIN "tenant_schema"."invoices" AS i`},
			columns:  []string{"id", "name", "amount", "count"},
			rows: [][]driver.Value{
				{"customer-1", "Best Customer", "1234.56", int64(7)},
			},
		})

		items, err := repo.GetTopCustomers(ctx, schema, 5)

		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, TopItem{
			ID:     "customer-1",
			Name:   "Best Customer",
			Amount: decimal.RequireFromString("1234.56"),
			Count:  7,
		}, items[0])
		stub.requireExhausted(t)
	})
}

func TestGORMRepositoryRecentActivityCombinesAndSortsSources(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 24, 12, 0, 0, 0, time.UTC)
	repo, stub := newAnalyticsStubRepository(t,
		analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."invoices" AS i`, `LEFT JOIN "tenant_schema"."contacts" AS c`},
			columns:  []string{"id", "invoice_type", "invoice_number", "status", "contact_name", "created_at", "amount"},
			rows: [][]driver.Value{
				{"inv-sales", string(models.InvoiceTypeSales), "INV-001", string(models.InvoiceStatusSent), "Acme OU", now.Add(-1 * time.Hour), "125.00"},
				{"inv-purchase", string(models.InvoiceTypePurchase), "BILL-001", string(models.InvoiceStatusPaid), "Supply OU", now.Add(-6 * time.Hour), "80.00"},
			},
		},
		analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."payments" AS p`, `LEFT JOIN "tenant_schema"."contacts" AS c`},
			columns:  []string{"id", "payment_type", "contact_name", "created_at", "amount"},
			rows: [][]driver.Value{
				{"pay-received", string(models.PaymentTypeReceived), "Acme OU", now, "125.00"},
				{"pay-made", string(models.PaymentTypeMade), "Supply OU", now.Add(-3 * time.Hour), "80.00"},
			},
		},
		analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."journal_entries" AS je`, `LEFT JOIN "tenant_schema"."journal_entry_lines" AS jel`},
			columns:  []string{"id", "status", "label", "created_at", "amount"},
			rows: [][]driver.Value{
				{"je-posted", string(models.JournalStatusPosted), "Month close", now.Add(-2 * time.Hour), "250.00"},
				{"je-draft", string(models.JournalStatusDraft), "Accrual", now.Add(-4 * time.Hour), "90.00"},
			},
		},
		analyticsStubQuery{
			contains: []string{`FROM "tenant_schema"."contacts" AS c`},
			columns:  []string{"id", "name", "created_at"},
			rows: [][]driver.Value{
				{"contact-1", "New Customer", now.Add(-30 * time.Minute)},
			},
		},
	)

	items, err := repo.GetRecentActivity(ctx, "tenant_schema", 3)

	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, "pay-received", items[0].ID)
	assert.Equal(t, "PAYMENT", items[0].Type)
	assert.Equal(t, "received", items[0].Action)
	assert.Equal(t, "Payment received from Acme OU", items[0].Description)
	require.NotNil(t, items[0].Amount)
	assert.True(t, items[0].Amount.Equal(decimal.RequireFromString("125.00")))

	assert.Equal(t, "contact-1", items[1].ID)
	assert.Equal(t, "CONTACT", items[1].Type)
	assert.Equal(t, "created", items[1].Action)
	assert.Equal(t, "New contact: New Customer", items[1].Description)
	assert.Nil(t, items[1].Amount)

	assert.Equal(t, "inv-sales", items[2].ID)
	assert.Equal(t, "INVOICE", items[2].Type)
	assert.Equal(t, "sent", items[2].Action)
	assert.Equal(t, "Invoice INV-001 to Acme OU", items[2].Description)
	stub.requireExhausted(t)
}

func TestGORMRepositoryRecentActivityPropagatesLaterSourceErrors(t *testing.T) {
	tests := []struct {
		name    string
		queries []analyticsStubQuery
		wantErr string
	}{
		{
			name: "payment",
			queries: []analyticsStubQuery{
				emptyActivityQuery(`FROM "tenant_schema"."invoices" AS i`),
				errorActivityQuery(`FROM "tenant_schema"."payments" AS p`, "payment query failed"),
			},
			wantErr: "get recent payment activity",
		},
		{
			name: "journal entry",
			queries: []analyticsStubQuery{
				emptyActivityQuery(`FROM "tenant_schema"."invoices" AS i`),
				emptyActivityQuery(`FROM "tenant_schema"."payments" AS p`),
				errorActivityQuery(`FROM "tenant_schema"."journal_entries" AS je`, "journal query failed"),
			},
			wantErr: "get recent journal entry activity",
		},
		{
			name: "contact",
			queries: []analyticsStubQuery{
				emptyActivityQuery(`FROM "tenant_schema"."invoices" AS i`),
				emptyActivityQuery(`FROM "tenant_schema"."payments" AS p`),
				emptyActivityQuery(`FROM "tenant_schema"."journal_entries" AS je`),
				errorActivityQuery(`FROM "tenant_schema"."contacts" AS c`, "contact query failed"),
			},
			wantErr: "get recent contact activity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, _ := newAnalyticsStubRepository(t, tt.queries...)

			items, err := repo.GetRecentActivity(context.Background(), "tenant_schema", 5)

			require.Error(t, err)
			assert.Nil(t, items)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func emptyActivityQuery(tableToken string) analyticsStubQuery {
	switch {
	case strings.Contains(tableToken, "invoices"):
		return analyticsStubQuery{
			contains: []string{tableToken},
			columns:  []string{"id", "invoice_type", "invoice_number", "status", "contact_name", "created_at", "amount"},
		}
	case strings.Contains(tableToken, "payments"):
		return analyticsStubQuery{
			contains: []string{tableToken},
			columns:  []string{"id", "payment_type", "contact_name", "created_at", "amount"},
		}
	case strings.Contains(tableToken, "journal_entries"):
		return analyticsStubQuery{
			contains: []string{tableToken},
			columns:  []string{"id", "status", "label", "created_at", "amount"},
		}
	default:
		return analyticsStubQuery{
			contains: []string{tableToken},
			columns:  []string{"id", "name", "created_at"},
		}
	}
}

func errorActivityQuery(tableToken, message string) analyticsStubQuery {
	query := emptyActivityQuery(tableToken)
	query.err = errors.New(message)
	return query
}

func newAnalyticsStubRepository(t *testing.T, queries ...analyticsStubQuery) (*GORMRepository, *analyticsStubDB) {
	t.Helper()

	gormDB, stub := newAnalyticsStubGormDB(t, queries...)
	return NewGORMRepository(gormDB), stub
}

func newAnalyticsStubGormDB(t *testing.T, queries ...analyticsStubQuery) (*gorm.DB, *analyticsStubDB) {
	t.Helper()

	stub := &analyticsStubDB{queries: append([]analyticsStubQuery(nil), queries...)}
	sqlDB := sql.OpenDB(analyticsStubConnector{stub: stub})
	t.Cleanup(func() { _ = sqlDB.Close() })

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 sqlDB,
		PreferSimpleProtocol: true,
	}), &gorm.Config{
		DisableAutomaticPing:   true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)
	return gormDB, stub
}

type analyticsStubQuery struct {
	contains []string
	columns  []string
	rows     [][]driver.Value
	err      error
}

type analyticsStubDB struct {
	mu      sync.Mutex
	queries []analyticsStubQuery
	calls   []string
}

func (s *analyticsStubDB) query(query string) (driver.Rows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = append(s.calls, query)
	if len(s.queries) == 0 {
		return nil, fmt.Errorf("unexpected query: %s", query)
	}

	expected := s.queries[0]
	s.queries = s.queries[1:]
	for _, token := range expected.contains {
		if !strings.Contains(query, token) {
			return nil, fmt.Errorf("query missing %q in %s", token, query)
		}
	}
	if expected.err != nil {
		return nil, expected.err
	}
	return &analyticsStubRows{
		columns: expected.columns,
		rows:    expected.rows,
	}, nil
}

func (s *analyticsStubDB) requireExhausted(t *testing.T) {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()
	require.Empty(t, s.queries)
}

type analyticsStubConnector struct {
	stub *analyticsStubDB
}

func (c analyticsStubConnector) Connect(context.Context) (driver.Conn, error) {
	return analyticsStubConn{stub: c.stub}, nil
}

func (c analyticsStubConnector) Driver() driver.Driver {
	return analyticsStubDriver{}
}

type analyticsStubDriver struct{}

func (analyticsStubDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("analytics stub driver requires a connector")
}

type analyticsStubConn struct {
	stub *analyticsStubDB
}

func (c analyticsStubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("analytics stub does not prepare statements")
}

func (c analyticsStubConn) Close() error {
	return nil
}

func (c analyticsStubConn) Begin() (driver.Tx, error) {
	return analyticsStubTx{}, nil
}

func (c analyticsStubConn) Ping(context.Context) error {
	return nil
}

func (c analyticsStubConn) CheckNamedValue(*driver.NamedValue) error {
	return nil
}

func (c analyticsStubConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	return c.stub.query(query)
}

func (c analyticsStubConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

type analyticsStubTx struct{}

func (analyticsStubTx) Commit() error {
	return nil
}

func (analyticsStubTx) Rollback() error {
	return nil
}

type analyticsStubRows struct {
	columns []string
	rows    [][]driver.Value
	index   int
}

func (r *analyticsStubRows) Columns() []string {
	return r.columns
}

func (r *analyticsStubRows) Close() error {
	return nil
}

func (r *analyticsStubRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.index]
	r.index++
	if len(row) != len(dest) {
		return fmt.Errorf("row has %d values, destination has %d", len(row), len(dest))
	}
	copy(dest, row)
	return nil
}
