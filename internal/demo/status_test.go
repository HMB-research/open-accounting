//go:build integration

package demo

import (
	"context"
	"fmt"
	"testing"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStatusReaderRejectsNilPool(t *testing.T) {
	reader, err := NewStatusReader(nil)
	require.Error(t, err)
	assert.Nil(t, reader)
	assert.Contains(t, err.Error(), "database pool is not configured")
}

func TestStatusReaderReadsDemoEntityCountsAndKeys(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	schema := testutil.SetupTestSchema(t, pool)
	ctx := context.Background()

	createDemoStatusTable(t, ctx, pool, schema, "accounts", "name TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "contacts", "name TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "invoices", "invoice_number TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "employees", "first_name TEXT NOT NULL, last_name TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "payments", "payment_number TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "journal_entries", "entry_number TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "bank_accounts", "name TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "recurring_invoices", "name TEXT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "payroll_runs", "period_year INT NOT NULL, period_month INT NOT NULL")
	createDemoStatusTable(t, ctx, pool, schema, "tsd_declarations", "period_year INT NOT NULL, period_month INT NOT NULL")

	insertDemoStatusRows(t, ctx, pool, schema, "accounts", "name", []string{
		"Sales revenue", "Bank", "Accounts receivable", "VAT payable", "Payroll liabilities",
		"Office expenses", "Inventory", "Retained earnings", "Accounts payable", "Cash", "Loan",
	})
	insertDemoStatusRows(t, ctx, pool, schema, "contacts", "name", []string{"Customer B", "Customer A"})
	insertDemoStatusRows(t, ctx, pool, schema, "invoices", "invoice_number", []string{"INV-002", "INV-001"})
	insertDemoStatusRows(t, ctx, pool, schema, "payments", "payment_number", []string{"PAY-002", "PAY-001"})
	insertDemoStatusRows(t, ctx, pool, schema, "journal_entries", "entry_number", []string{"JE-002", "JE-001"})
	insertDemoStatusRows(t, ctx, pool, schema, "bank_accounts", "name", []string{"Operating account"})
	insertDemoStatusRows(t, ctx, pool, schema, "recurring_invoices", "name", []string{"Monthly subscription"})
	execDemoStatusSQL(t, ctx, pool, schema, "employees", "INSERT INTO %s (first_name, last_name) VALUES ('Mari', 'Maasikas'), ('Jaan', 'Tamm')")
	execDemoStatusSQL(t, ctx, pool, schema, "payroll_runs", "INSERT INTO %s (period_year, period_month) VALUES (2026, 3), (2025, 12)")
	execDemoStatusSQL(t, ctx, pool, schema, "tsd_declarations", "INSERT INTO %s (period_year, period_month) VALUES (2026, 1)")

	reader, err := NewStatusReader(pool)
	require.NoError(t, err)

	status, err := reader.ReadDemoStatus(ctx, schema, 3)
	require.NoError(t, err)

	assert.Equal(t, 3, status.User)
	assert.Equal(t, 11, status.Accounts.Count)
	assert.Equal(t, []string{
		"Accounts payable", "Accounts receivable", "Bank", "Cash", "Inventory",
		"Loan", "Office expenses", "Payroll liabilities", "Retained earnings", "Sales revenue",
	}, status.Accounts.Keys)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"Customer A", "Customer B"}}, status.Contacts)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"INV-001", "INV-002"}}, status.Invoices)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"Jaan Tamm", "Mari Maasikas"}}, status.Employees)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"PAY-001", "PAY-002"}}, status.Payments)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"JE-001", "JE-002"}}, status.JournalEntries)
	assert.Equal(t, EntityStatus{Count: 1, Keys: []string{"Operating account"}}, status.BankAccounts)
	assert.Equal(t, EntityStatus{Count: 1, Keys: []string{"Monthly subscription"}}, status.RecurringInvoices)
	assert.Equal(t, EntityStatus{Count: 2, Keys: []string{"2025-12", "2026-03"}}, status.PayrollRuns)
	assert.Equal(t, EntityStatus{Count: 1, Keys: []string{"2026-01"}}, status.TsdDeclarations)
}

func TestStatusReaderRejectsUnsafeSchemaName(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	reader, err := NewStatusReader(pool)
	require.NoError(t, err)

	_, err = reader.ReadDemoStatus(context.Background(), "tenant-demo-invalid", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid SQL identifier "tenant-demo-invalid"`)
}

func createDemoStatusTable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, table, columns string) {
	t.Helper()
	execDemoStatusSQL(t, ctx, pool, schema, table, "CREATE TABLE %s ("+columns+")")
}

func insertDemoStatusRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, table, column string, values []string) {
	t.Helper()
	qualifiedTable := demoStatusQualifiedTable(t, schema, table)
	quotedColumn, err := database.QuoteIdentifier(column)
	require.NoError(t, err)

	for _, value := range values {
		_, err := pool.Exec(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES ($1)", qualifiedTable, quotedColumn), value)
		require.NoError(t, err)
	}
}

func execDemoStatusSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, schema, table, statement string) {
	t.Helper()
	_, err := pool.Exec(ctx, fmt.Sprintf(statement, demoStatusQualifiedTable(t, schema, table)))
	require.NoError(t, err)
}

func demoStatusQualifiedTable(t *testing.T, schema, table string) string {
	t.Helper()
	qualifiedTable, err := database.QualifiedTable(schema, table)
	require.NoError(t, err)
	return qualifiedTable
}
