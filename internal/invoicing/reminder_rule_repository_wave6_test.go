package invoicing

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestReminderRuleGORMRepositoryWave6RuleQueryErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "tenant-1"
	expectedErr := errors.New("rule query failed")
	repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

	rules, err := repo.ListRules(ctx, schemaName, tenantID)
	assert.Nil(t, rules)
	require.ErrorContains(t, err, "query rules")
	assert.ErrorIs(t, err, expectedErr)
}

func TestReminderRuleGORMRepositoryWave6GetInvoicesForRuleBranches(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "tenant-1"
	asOfDate := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)
	rule := invoicingDryRunReminderRule(tenantID, asOfDate)

	t.Run("invalid trigger", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t))
		invalidRule := *rule
		invalidRule.TriggerType = TriggerType("INVALID")

		invoices, err := repo.GetInvoicesForRule(ctx, schemaName, tenantID, &invalidRule, asOfDate)

		assert.Nil(t, invoices)
		assert.ErrorIs(t, err, ErrInvalidTriggerType)
	})

	t.Run("nil database", func(t *testing.T) {
		repo := NewReminderRuleRepository(nil)

		invoices, err := repo.GetInvoicesForRule(ctx, schemaName, tenantID, rule, asOfDate)

		assert.Nil(t, invoices)
		require.ErrorContains(t, err, "query invoices for rule")
		assert.ErrorContains(t, err, "database is not configured")
	})

	t.Run("scan rows convert overdue invoices", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingWave6ScanRows(invoicingWave6RowSet{
			columns: []string{"id", "invoice_number", "contact_id", "contact_name", "contact_email", "issue_date", "due_date", "total", "amount_paid", "outstanding_amount", "currency"},
			values: [][]driver.Value{{
				"invoice-1",
				"INV-00042",
				"contact-1",
				"Customer OU",
				"billing@example.com",
				"2026-06-01",
				"2026-06-20",
				"122.00",
				"22.00",
				"100.00",
				"EUR",
			}},
		})))

		invoices, err := repo.GetInvoicesForRule(ctx, schemaName, tenantID, rule, asOfDate)

		require.NoError(t, err)
		require.Len(t, invoices, 1)
		assert.Equal(t, "INV-00042", invoices[0].InvoiceNumber)
		assert.Equal(t, -5, invoices[0].DaysUntilDue)
		assert.Equal(t, 5, invoices[0].DaysOverdue)
	})

	t.Run("invalid due date", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingWave6ScanRows(invoicingWave6RowSet{
			columns: []string{"id", "invoice_number", "contact_id", "contact_name", "contact_email", "issue_date", "due_date", "total", "amount_paid", "outstanding_amount", "currency"},
			values: [][]driver.Value{{
				"invoice-1",
				"INV-00042",
				"contact-1",
				"Customer OU",
				"billing@example.com",
				"2026-06-01",
				"not-a-date",
				"122.00",
				"22.00",
				"100.00",
				"EUR",
			}},
		})))

		invoices, err := repo.GetInvoicesForRule(ctx, schemaName, tenantID, rule, asOfDate)

		assert.Nil(t, invoices)
		require.ErrorContains(t, err, "parse due date")
	})
}

type invoicingWave6RowSet struct {
	columns []string
	values  [][]driver.Value
}

var invoicingWave6RowsDSNID uint64
var invoicingWave6RowsDriverOnce sync.Once
var invoicingWave6RowsMu sync.Mutex
var invoicingWave6RowsByDSN = map[string]invoicingWave6RowSet{}

func withInvoicingWave6ScanRows(rowSets ...invoicingWave6RowSet) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(invoicingDryRunCallbackName(t, "scan_rows_wave6"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing invoicing dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newInvoicingWave6SQLRows(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func newInvoicingWave6SQLRows(t *testing.T, rowSet invoicingWave6RowSet) *sql.Rows {
	t.Helper()

	invoicingWave6RowsDriverOnce.Do(func() {
		sql.Register("invoicing_wave6_rows", invoicingWave6RowsDriver{})
	})

	dsn := fmt.Sprintf("invoicing-wave6-rows-%d", atomic.AddUint64(&invoicingWave6RowsDSNID, 1))
	invoicingWave6RowsMu.Lock()
	invoicingWave6RowsByDSN[dsn] = rowSet
	invoicingWave6RowsMu.Unlock()

	db, err := sql.Open("invoicing_wave6_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		invoicingWave6RowsMu.Lock()
		delete(invoicingWave6RowsByDSN, dsn)
		invoicingWave6RowsMu.Unlock()
	})

	return rows
}

type invoicingWave6RowsDriver struct{}

func (invoicingWave6RowsDriver) Open(name string) (driver.Conn, error) {
	return invoicingWave6RowsConn{dsn: name}, nil
}

type invoicingWave6RowsConn struct {
	dsn string
}

func (invoicingWave6RowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("invoicing wave6 rows do not prepare statements")
}

func (invoicingWave6RowsConn) Close() error {
	return nil
}

func (invoicingWave6RowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("invoicing wave6 rows do not begin transactions")
}

func (c invoicingWave6RowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	invoicingWave6RowsMu.Lock()
	rowSet, ok := invoicingWave6RowsByDSN[c.dsn]
	invoicingWave6RowsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("invoicing wave6 row set %q not found", c.dsn)
	}
	return &invoicingWave6SQLRows{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type invoicingWave6SQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *invoicingWave6SQLRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*invoicingWave6SQLRows) Close() error {
	return nil
}

func (r *invoicingWave6SQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
