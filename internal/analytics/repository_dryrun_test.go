package analytics

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type analyticsDryRunConnPool struct{}

func (analyticsDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tests should not prepare statements")
}

func (analyticsDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tests should not execute statements")
}

func (analyticsDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tests should not query rows")
}

func (analyticsDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func newAnalyticsDryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: analyticsDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)
	return db
}

func TestGORMRepositoryDryRunQueries(t *testing.T) {
	repo := NewGORMRepository(newAnalyticsDryRunDB(t))
	ctx := context.Background()
	schemaName := "tenant_schema"
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	table, err := repo.tenantTable(ctx, schemaName, "invoices", "i")
	require.NoError(t, err)
	require.NotNil(t, table)

	revenue, expenses, err := repo.GetRevenueExpenses(ctx, schemaName, start, end)
	requireDryRunScanError(t, err, "get revenue expenses")
	assert.True(t, revenue.IsZero())
	assert.True(t, expenses.IsZero())

	receivables, overdueReceivables, err := repo.GetReceivablesSummary(ctx, schemaName)
	requireDryRunScanError(t, err, "get invoice balance summary")
	assert.True(t, receivables.IsZero())
	assert.True(t, overdueReceivables.IsZero())

	payables, overduePayables, err := repo.GetPayablesSummary(ctx, schemaName)
	requireDryRunScanError(t, err, "get invoice balance summary")
	assert.True(t, payables.IsZero())
	assert.True(t, overduePayables.IsZero())

	draft, pending, overdue, err := repo.GetInvoiceCounts(ctx, schemaName)
	requireDryRunScanError(t, err, "get invoice counts")
	assert.Zero(t, draft)
	assert.Zero(t, pending)
	assert.Zero(t, overdue)

	revenueExpenses, err := repo.GetMonthlyRevenueExpenses(ctx, schemaName, 3)
	requireDryRunScanError(t, err, "get monthly revenue expenses")
	assert.Nil(t, revenueExpenses)

	cashFlow, err := repo.GetMonthlyCashFlow(ctx, schemaName, 2)
	requireDryRunScanError(t, err, "get monthly bank cash flow")
	assert.Nil(t, cashFlow)

	aging, err := repo.GetAgingByContact(ctx, schemaName, string(models.InvoiceTypeSales))
	requireDryRunScanError(t, err, "get aging by contact")
	assert.Nil(t, aging)

	topCustomers, err := repo.GetTopCustomers(ctx, schemaName, 5)
	requireDryRunScanError(t, err, "get top customers")
	assert.Nil(t, topCustomers)

	activity, err := repo.GetRecentActivity(ctx, schemaName, 5)
	requireDryRunScanError(t, err, "get recent invoice activity")
	assert.Nil(t, activity)

	invoiceActivity, err := repo.recentInvoiceActivity(ctx, schemaName, 5)
	requireDryRunScanError(t, err, "get recent invoice activity")
	assert.Nil(t, invoiceActivity)

	paymentActivity, err := repo.recentPaymentActivity(ctx, schemaName, 5)
	requireDryRunScanError(t, err, "get recent payment activity")
	assert.Nil(t, paymentActivity)

	journalActivity, err := repo.recentJournalEntryActivity(ctx, schemaName, 5)
	requireDryRunScanError(t, err, "get recent journal entry activity")
	assert.Nil(t, journalActivity)

	contactActivity, err := repo.recentContactActivity(ctx, schemaName, 5)
	requireDryRunScanError(t, err, "get recent contact activity")
	assert.Nil(t, contactActivity)
}

func requireDryRunScanError(t *testing.T, err error, operation string) {
	t.Helper()

	require.Error(t, err)
	assert.Contains(t, err.Error(), operation)
	assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)
}

func TestGORMRepositoryInvalidSchemaErrors(t *testing.T) {
	repo := NewGORMRepository(newAnalyticsDryRunDB(t))
	ctx := context.Background()
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)
	schemaName := "tenant-schema"

	_, _, err := repo.GetRevenueExpenses(ctx, schemaName, start, end)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "qualify journal entry lines table")

	_, _, err = repo.GetReceivablesSummary(ctx, schemaName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "qualify invoices table")

	_, err = repo.GetMonthlyRevenueExpenses(ctx, schemaName, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "qualify journal entry lines table")

	_, err = repo.GetRecentActivity(ctx, schemaName, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "qualify invoices table")
}
