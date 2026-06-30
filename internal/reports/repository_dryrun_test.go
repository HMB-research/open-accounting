package reports

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type reportsDryRunConnPool struct{}

func (reportsDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run reports tests should not prepare statements")
}

func (reportsDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run reports tests should not execute statements")
}

func (reportsDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run reports tests should not query rows")
}

func (reportsDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (reportsDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &reportsDryRunTx{}, nil
}

type reportsDryRunTx struct {
	reportsDryRunConnPool
}

func (*reportsDryRunTx) Commit() error {
	return nil
}

func (*reportsDryRunTx) Rollback() error {
	return nil
}

type reportsDryRunDBOption func(t *testing.T, db *gorm.DB)

type reportsDryRunFixtures struct {
	contact *ContactInfo
	tenants []models.Tenant
}

type reportsDryRunRecorder struct {
	rows    []string
	queries []string
	updates []string
}

type reportsDryRunRowSet struct {
	columns []string
	values  [][]driver.Value
}

var reportsDryRunCallbackID uint64
var reportsDryRunRowsDSNID uint64
var reportsDryRunRowsDriverOnce sync.Once
var reportsDryRunRowsMu sync.Mutex
var reportsDryRunRowsByDSN = map[string]reportsDryRunRowSet{}

func newReportsDryRunDB(t *testing.T, opts ...reportsDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: reportsDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withReportsDryRunRowRecorder(recorder *reportsDryRunRecorder) reportsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().After("gorm:row").Register(reportsDryRunCallbackName("row_recorder"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.rows = append(recorder.rows, tx.Statement.SQL.String())
			}
		})
		require.NoError(t, err)
	}
}

func withReportsDryRunRowErrors(rowErrors ...error) reportsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(reportsDryRunCallbackName("row_error"), func(tx *gorm.DB) {
			if len(rowErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(rowErrors) {
				errIndex = len(rowErrors) - 1
			}
			index++
			if rowErrors[errIndex] != nil {
				tx.AddError(rowErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withReportsDryRunScanRows(rowSets ...reportsDryRunRowSet) reportsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(reportsDryRunCallbackName("scan_rows"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing reports dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newReportsDryRunSQLRows(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func withReportsDryRunFixtures(fixtures reportsDryRunFixtures, recorder *reportsDryRunRecorder) reportsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(reportsDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.queries = append(recorder.queries, tx.Statement.SQL.String())
			}
			populateReportsDryRunQueryDest(tx, tx.Statement.Dest, &fixtures)
		})
		require.NoError(t, err)
	}
}

func withReportsDryRunQueryErrors(queryErrors ...error) reportsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(reportsDryRunCallbackName("query_error"), func(tx *gorm.DB) {
			if len(queryErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(queryErrors) {
				errIndex = len(queryErrors) - 1
			}
			index++
			if queryErrors[errIndex] != nil {
				tx.AddError(queryErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withReportsDryRunUpdateRows(recorder *reportsDryRunRecorder, rows ...int64) reportsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(reportsDryRunCallbackName("update_rows"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.updates = append(recorder.updates, tx.Statement.SQL.String())
			}
			rowCount := int64(0)
			if len(rows) > 0 {
				rowCount = rows[len(rows)-1]
				if index < len(rows) {
					rowCount = rows[index]
				}
				index++
			}
			tx.RowsAffected = rowCount
		})
		require.NoError(t, err)
	}
}

func withReportsDryRunUpdateError(expectedErr error) reportsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(reportsDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func reportsDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&reportsDryRunCallbackID, 1)
	return fmt.Sprintf("reports_dryrun:%d:%s", id, suffix)
}

func newReportsDryRunSQLRows(t *testing.T, rowSet reportsDryRunRowSet) *sql.Rows {
	t.Helper()

	reportsDryRunRowsDriverOnce.Do(func() {
		sql.Register("reports_dryrun_rows", reportsDryRunRowsDriver{})
	})

	dsn := fmt.Sprintf("reports-dry-run-rows-%d", atomic.AddUint64(&reportsDryRunRowsDSNID, 1))
	reportsDryRunRowsMu.Lock()
	reportsDryRunRowsByDSN[dsn] = rowSet
	reportsDryRunRowsMu.Unlock()

	db, err := sql.Open("reports_dryrun_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		reportsDryRunRowsMu.Lock()
		delete(reportsDryRunRowsByDSN, dsn)
		reportsDryRunRowsMu.Unlock()
	})

	return rows
}

type reportsDryRunRowsDriver struct{}

func (reportsDryRunRowsDriver) Open(name string) (driver.Conn, error) {
	return reportsDryRunRowsConn{dsn: name}, nil
}

type reportsDryRunRowsConn struct {
	dsn string
}

func (reportsDryRunRowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("reports dry-run rows do not prepare statements")
}

func (reportsDryRunRowsConn) Close() error {
	return nil
}

func (reportsDryRunRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("reports dry-run rows do not begin transactions")
}

func (c reportsDryRunRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	reportsDryRunRowsMu.Lock()
	rowSet, ok := reportsDryRunRowsByDSN[c.dsn]
	reportsDryRunRowsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("reports dry-run row set %q not found", c.dsn)
	}
	return &reportsDryRunSQLRows{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type reportsDryRunSQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *reportsDryRunSQLRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*reportsDryRunSQLRows) Close() error {
	return nil
}

func (r *reportsDryRunSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func populateReportsDryRunQueryDest(tx *gorm.DB, dest any, fixtures *reportsDryRunFixtures) {
	switch typed := dest.(type) {
	case *ContactInfo:
		if fixtures.contact != nil {
			*typed = *fixtures.contact
			tx.RowsAffected = 1
		}
	case *models.Tenant:
		if tenant, ok := nextReportsDryRunRow(&fixtures.tenants); ok {
			*typed = tenant
			tx.RowsAffected = 1
		}
	}
}

func nextReportsDryRunRow[T any](rows *[]T) (T, bool) {
	var zero T
	if len(*rows) == 0 {
		return zero, false
	}
	row := (*rows)[0]
	if len(*rows) > 1 {
		*rows = (*rows)[1:]
	}
	return row, true
}

func TestGORMRepositoryDryRunScanQueriesBuildTenantSQLAndWrapRowErrors(t *testing.T) {
	ctx := context.Background()
	rowErr := errors.New("dry-run row failure")
	recorder := &reportsDryRunRecorder{}
	repo := &GORMRepository{db: newReportsDryRunDB(t,
		withReportsDryRunRowRecorder(recorder),
		withReportsDryRunRowErrors(rowErr),
	)}
	schemaName := "tenant_reports"
	tenantID := "tenant-1"
	contactID := "contact-1"
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "journal entries",
			run: func() error {
				_, err := repo.GetJournalEntriesForPeriod(ctx, schemaName, tenantID, startDate, endDate)
				return err
			},
			want: "query journal entries",
		},
		{
			name: "cash balance",
			run: func() error {
				_, err := repo.GetCashAccountBalance(ctx, schemaName, tenantID, endDate)
				return err
			},
			want: "query cash balance",
		},
		{
			name: "outstanding invoices",
			run: func() error {
				_, err := repo.GetOutstandingInvoicesByContact(ctx, schemaName, tenantID, string(models.InvoiceTypeSales), endDate)
				return err
			},
			want: "query outstanding invoices",
		},
		{
			name: "contact invoices",
			run: func() error {
				_, err := repo.GetContactInvoices(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), endDate)
				return err
			},
			want: "query contact invoices",
		},
		{
			name: "statement opening invoice total",
			run: func() error {
				_, err := repo.sumInvoiceStatementAmountBefore(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), startDate)
				return err
			},
			want: "query contact statement invoice opening balance",
		},
		{
			name: "statement opening payment total",
			run: func() error {
				_, err := repo.sumPaymentStatementAmountBefore(ctx, schemaName, tenantID, contactID, string(models.PaymentTypeReceived), startDate)
				return err
			},
			want: "query contact statement payment opening balance",
		},
		{
			name: "statement invoice entries",
			run: func() error {
				_, err := repo.getContactStatementInvoiceEntries(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), startDate, endDate)
				return err
			},
			want: "query contact statement invoices",
		},
		{
			name: "statement payment entries",
			run: func() error {
				_, err := repo.getContactStatementPaymentEntries(ctx, schemaName, tenantID, contactID, string(models.PaymentTypeReceived), startDate, endDate)
				return err
			},
			want: "query contact statement payments",
		},
		{
			name: "statement opening top-level",
			run: func() error {
				_, err := repo.GetContactStatementOpeningBalance(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), string(models.PaymentTypeReceived), startDate)
				return err
			},
			want: "query contact statement invoice opening balance",
		},
		{
			name: "statement entries top-level",
			run: func() error {
				_, err := repo.GetContactStatementEntries(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), string(models.PaymentTypeReceived), startDate, endDate)
				return err
			},
			want: "query contact statement invoices",
		},
		{
			name: "sales margin lines",
			run: func() error {
				_, err := repo.GetSalesMarginLines(ctx, schemaName, tenantID, startDate, endDate)
				return err
			},
			want: "query sales margin lines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			require.ErrorContains(t, err, tt.want)
			assert.ErrorIs(t, err, rowErr)
		})
	}

	assertRecordedSQLContains(t, recorder.rows,
		`FROM "tenant_reports"."journal_entries" AS je`,
		`JOIN "tenant_reports"."journal_entry_lines" AS jl ON je.id = jl.journal_entry_id`,
		`JOIN "tenant_reports"."accounts" AS a ON jl.account_id = a.id`,
		`(a.code LIKE $4 OR a.code LIKE $5)`,
		`FROM "tenant_reports"."invoices" AS i`,
		`JOIN "tenant_reports"."contacts" AS c ON i.contact_id = c.id AND i.tenant_id = c.tenant_id`,
		`FROM "tenant_reports"."payments" AS p`,
		`JOIN "tenant_reports"."invoice_lines" AS il ON il.invoice_id = i.id AND il.tenant_id = i.tenant_id`,
		`LEFT JOIN "tenant_reports"."products" AS p ON p.id = il.product_id AND p.tenant_id = i.tenant_id`,
	)
}

func TestCashAccountCodeConditionMatchesCashFlowClassifierPrefixes(t *testing.T) {
	condition, args := cashAccountCodeCondition("a")

	assert.Equal(t, "(a.code LIKE ? OR a.code LIKE ?)", condition)
	assert.Equal(t, []interface{}{"10%", "11%"}, args)
}

func TestGORMRepositoryDryRunScanQueriesReturnGormDryRunError(t *testing.T) {
	ctx := context.Background()
	repo := &GORMRepository{db: newReportsDryRunDB(t)}
	schemaName := "tenant_reports"
	tenantID := "tenant-1"
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	_, err := repo.GetSalesMarginLines(ctx, schemaName, tenantID, startDate, endDate)
	require.ErrorContains(t, err, "query sales margin lines")
	assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)
}

func TestGORMRepositoryDryRunScanSuccessMappings(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_reports"
	tenantID := "tenant-1"
	contactID := "contact-1"
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	t.Run("journal entries group lines in query order", func(t *testing.T) {
		entryDate := time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC)
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunScanRows(reportsDryRunRowSet{
			columns: []string{"id", "entry_date", "description", "account_code", "account_name", "account_type", "debit", "credit"},
			values: [][]driver.Value{
				{"journal-1", entryDate, "Customer invoice", "1200", "Accounts receivable", "asset", "250.00", "0"},
				{"journal-1", entryDate, "Customer invoice", "4000", "Sales revenue", "revenue", "0", "250.00"},
				{"journal-2", entryDate.AddDate(0, 0, 1), "Bank fee", "6900", "Bank fees", "expense", "5.00", "0"},
			},
		}))}

		entries, err := repo.GetJournalEntriesForPeriod(ctx, schemaName, tenantID, startDate, endDate)

		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "journal-1", entries[0].ID)
		assert.Equal(t, "Customer invoice", entries[0].Description)
		require.Len(t, entries[0].Lines, 2)
		assert.Equal(t, "1200", entries[0].Lines[0].AccountCode)
		assert.True(t, entries[0].Lines[0].Debit.Equal(decimal.RequireFromString("250.00")))
		assert.True(t, entries[0].Lines[1].Credit.Equal(decimal.RequireFromString("250.00")))
		assert.Equal(t, "journal-2", entries[1].ID)
	})

	t.Run("cash balance scans decimal total", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunScanRows(reportsDryRunRowSet{
			columns: []string{"total"},
			values:  [][]driver.Value{{"1234.56"}},
		}))}

		balance, err := repo.GetCashAccountBalance(ctx, schemaName, tenantID, endDate)

		require.NoError(t, err)
		assert.True(t, balance.Equal(decimal.RequireFromString("1234.56")))
	})

	t.Run("outstanding invoices map contact balances and invoice rows", func(t *testing.T) {
		oldestInvoice := time.Date(2026, time.January, 3, 0, 0, 0, 0, time.UTC)
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunScanRows(reportsDryRunRowSet{
			columns: []string{"contact_id", "contact_name", "contact_code", "contact_email", "balance", "invoice_count", "oldest_invoice"},
			values: [][]driver.Value{
				{"contact-1", "Acme OU", "ACME", "billing@example.com", "375.00", int64(2), oldestInvoice},
				{"contact-2", "No Date OU", "", "", "42.00", int64(1), nil},
			},
		}))}

		balances, err := repo.GetOutstandingInvoicesByContact(ctx, schemaName, tenantID, string(models.InvoiceTypeSales), endDate)

		require.NoError(t, err)
		require.Len(t, balances, 2)
		assert.Equal(t, "contact-1", balances[0].ContactID)
		assert.True(t, balances[0].Balance.Equal(decimal.RequireFromString("375.00")))
		assert.Equal(t, 2, balances[0].InvoiceCount)
		assert.Equal(t, "2026-01-03", balances[0].OldestInvoice)
		assert.Equal(t, "", balances[1].OldestInvoice)

		repo = &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunScanRows(reportsDryRunRowSet{
			columns: []string{"invoice_id", "invoice_number", "invoice_date", "due_date", "total_amount", "amount_paid", "currency", "days_overdue"},
			values: [][]driver.Value{
				{"invoice-1", "INV-0001", oldestInvoice, oldestInvoice.AddDate(0, 0, 14), "500.00", "125.00", "EUR", int64(10)},
			},
		}))}

		invoices, err := repo.GetContactInvoices(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), endDate)

		require.NoError(t, err)
		require.Len(t, invoices, 1)
		assert.Equal(t, "INV-0001", invoices[0].InvoiceNumber)
		assert.Equal(t, "2026-01-03", invoices[0].InvoiceDate)
		assert.True(t, invoices[0].OutstandingAmount.Equal(decimal.RequireFromString("375.00")))
		assert.Equal(t, 10, invoices[0].DaysOverdue)
	})

	t.Run("contact statement maps opening balance and sorted entries", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunScanRows(
			reportsDryRunRowSet{
				columns: []string{"total"},
				values:  [][]driver.Value{{"500.00"}},
			},
			reportsDryRunRowSet{
				columns: []string{"total"},
				values:  [][]driver.Value{{"125.00"}},
			},
		))}

		opening, err := repo.GetContactStatementOpeningBalance(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), string(models.PaymentTypeReceived), startDate)

		require.NoError(t, err)
		assert.True(t, opening.Equal(decimal.RequireFromString("375.00")))

		invoiceDate := time.Date(2026, time.January, 12, 0, 0, 0, 0, time.UTC)
		paymentDate := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
		repo = &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunScanRows(
			reportsDryRunRowSet{
				columns: []string{"document_id", "document_number", "document_date", "due_date", "reference", "notes", "currency", "document_amount", "statement_amount"},
				values: [][]driver.Value{
					{"invoice-1", "INV-0001", invoiceDate, invoiceDate.AddDate(0, 0, 14), "", "Consulting", "EUR", "500.00", "500.00"},
				},
			},
			reportsDryRunRowSet{
				columns: []string{"document_id", "document_number", "document_date", "reference", "notes", "currency", "document_amount", "statement_amount"},
				values: [][]driver.Value{
					{"payment-1", "PMT-0001", paymentDate, "PAY-REF", "", "EUR", "125.00", "-125.00"},
				},
			},
		))}

		entries, err := repo.GetContactStatementEntries(ctx, schemaName, tenantID, contactID, string(models.InvoiceTypeSales), string(models.PaymentTypeReceived), startDate, endDate)

		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "PAYMENT", entries[0].DocumentType)
		assert.Equal(t, "PAY-REF", entries[0].Description)
		assert.True(t, entries[0].StatementAmount.Equal(decimal.RequireFromString("-125.00")))
		assert.Equal(t, "INVOICE", entries[1].DocumentType)
		assert.Equal(t, "Consulting", entries[1].Description)
		assert.Equal(t, "2026-01-26", entries[1].DueDate)
	})

	t.Run("sales margin lines map scanned rows", func(t *testing.T) {
		invoiceDate := time.Date(2026, time.January, 22, 0, 0, 0, 0, time.UTC)
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunScanRows(reportsDryRunRowSet{
			columns: []string{"invoice_id", "invoice_number", "invoice_date", "contact_id", "contact_name", "product_id", "product_code", "product_name", "description", "quantity", "revenue", "unit_cost", "cost"},
			values: [][]driver.Value{
				{"invoice-1", "INV-0022", invoiceDate, "contact-1", "Acme OU", "product-1", "CONS", "Consulting", "Implementation", "3", "900.00", "100.00", "300.00"},
			},
		}))}

		lines, err := repo.GetSalesMarginLines(ctx, schemaName, tenantID, startDate, endDate)

		require.NoError(t, err)
		require.Len(t, lines, 1)
		assert.Equal(t, "INV-0022", lines[0].InvoiceNumber)
		assert.Equal(t, "2026-01-22", lines[0].InvoiceDate)
		assert.True(t, lines[0].Quantity.Equal(decimal.NewFromInt(3)))
		assert.True(t, lines[0].Revenue.Equal(decimal.RequireFromString("900.00")))
		assert.True(t, lines[0].Cost.Equal(decimal.RequireFromString("300.00")))
	})
}

func TestGORMRepositoryDryRunRejectsInvalidTenantSchema(t *testing.T) {
	ctx := context.Background()
	repo := &GORMRepository{db: newReportsDryRunDB(t)}
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "GetJournalEntriesForPeriod",
			run: func() error {
				_, err := repo.GetJournalEntriesForPeriod(ctx, "tenant-bad", "tenant-1", startDate, endDate)
				return err
			},
		},
		{
			name: "GetCashAccountBalance",
			run: func() error {
				_, err := repo.GetCashAccountBalance(ctx, "tenant-bad", "tenant-1", endDate)
				return err
			},
		},
		{
			name: "GetOutstandingInvoicesByContact",
			run: func() error {
				_, err := repo.GetOutstandingInvoicesByContact(ctx, "tenant-bad", "tenant-1", string(models.InvoiceTypeSales), endDate)
				return err
			},
		},
		{
			name: "GetContactInvoices",
			run: func() error {
				_, err := repo.GetContactInvoices(ctx, "tenant-bad", "tenant-1", "contact-1", string(models.InvoiceTypeSales), endDate)
				return err
			},
		},
		{
			name: "GetContact",
			run: func() error {
				_, err := repo.GetContact(ctx, "tenant-bad", "tenant-1", "contact-1")
				return err
			},
		},
		{
			name: "GetContactStatementOpeningBalance",
			run: func() error {
				_, err := repo.GetContactStatementOpeningBalance(ctx, "tenant-bad", "tenant-1", "contact-1", string(models.InvoiceTypeSales), string(models.PaymentTypeReceived), startDate)
				return err
			},
		},
		{
			name: "GetContactStatementEntries",
			run: func() error {
				_, err := repo.GetContactStatementEntries(ctx, "tenant-bad", "tenant-1", "contact-1", string(models.InvoiceTypeSales), string(models.PaymentTypeReceived), startDate, endDate)
				return err
			},
		},
		{
			name: "GetSalesMarginLines",
			run: func() error {
				_, err := repo.GetSalesMarginLines(ctx, "tenant-bad", "tenant-1", startDate, endDate)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid SQL identifier")
		})
	}
}

func TestGORMRepositoryDryRunContactAndCashFlowMappingSuccessPaths(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	contactID := "contact-1"
	recorder := &reportsDryRunRecorder{}
	repo := &GORMRepository{db: newReportsDryRunDB(t,
		withReportsDryRunFixtures(reportsDryRunFixtures{
			contact: &ContactInfo{
				ID:    contactID,
				Name:  "Acme OU",
				Code:  "ACME",
				Email: "billing@example.com",
			},
			tenants: []models.Tenant{
				{
					ID:       tenantID,
					Settings: json.RawMessage(`{"cash_flow_mapping":{"operating_account_codes":["4000"]}}`),
				},
				{
					ID:       tenantID,
					Settings: json.RawMessage(`{"company_name":"Demo"}`),
				},
			},
		}, recorder),
		withReportsDryRunUpdateRows(recorder, 1),
	)}

	contact, err := repo.GetContact(ctx, "tenant_reports", tenantID, contactID)
	require.NoError(t, err)
	assert.Equal(t, ContactInfo{ID: contactID, Name: "Acme OU", Code: "ACME", Email: "billing@example.com"}, contact)

	mapping, err := repo.GetCashFlowMappingOverrides(ctx, tenantID)
	require.NoError(t, err)
	assert.Equal(t, []string{"4000"}, mapping.OperatingAccountCodes)

	updatedMapping, err := repo.UpdateCashFlowMappingOverrides(ctx, tenantID, CashFlowMappingOverrides{
		OperatingAccountCodes: []string{"4010"},
		InvestingAccountCodes: []string{"1200"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"4010"}, updatedMapping.OperatingAccountCodes)
	assert.Equal(t, []string{"1200"}, updatedMapping.InvestingAccountCodes)

	assertRecordedSQLContains(t, recorder.queries,
		`FROM "tenant_reports"."contacts" AS c`,
		`FROM "tenants"`,
	)
	assertRecordedSQLContains(t, recorder.updates, `UPDATE "tenants" SET`)
}

func TestGORMRepositoryDryRunContactAndCashFlowMappingErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	contactID := "contact-1"
	expectedErr := errors.New("query failed")
	updateErr := errors.New("update failed")

	t.Run("contact not found", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunQueryErrors(gorm.ErrRecordNotFound))}

		_, err := repo.GetContact(ctx, "tenant_reports", tenantID, contactID)
		require.ErrorContains(t, err, "contact not found")
	})

	t.Run("contact query error", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunQueryErrors(expectedErr))}

		_, err := repo.GetContact(ctx, "tenant_reports", tenantID, contactID)
		require.ErrorContains(t, err, "query contact")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("read tenant not found", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunQueryErrors(gorm.ErrRecordNotFound))}

		_, err := repo.GetCashFlowMappingOverrides(ctx, tenantID)
		require.ErrorContains(t, err, "tenant not found")
	})

	t.Run("read tenant query error", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunQueryErrors(expectedErr))}

		_, err := repo.GetCashFlowMappingOverrides(ctx, tenantID)
		require.ErrorContains(t, err, "query cash flow mapping")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("update tenant not found before write", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunQueryErrors(gorm.ErrRecordNotFound))}

		_, err := repo.UpdateCashFlowMappingOverrides(ctx, tenantID, CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"4000"},
		})
		require.ErrorContains(t, err, "tenant not found")
	})

	t.Run("update tenant query error", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t, withReportsDryRunQueryErrors(expectedErr))}

		_, err := repo.UpdateCashFlowMappingOverrides(ctx, tenantID, CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"4000"},
		})
		require.ErrorContains(t, err, "query cash flow mapping")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("update rejects invalid existing settings", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t,
			withReportsDryRunFixtures(reportsDryRunFixtures{
				tenants: []models.Tenant{{ID: tenantID, Settings: json.RawMessage(`{`)}},
			}, nil),
		)}

		_, err := repo.UpdateCashFlowMappingOverrides(ctx, tenantID, CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"4000"},
		})
		require.ErrorContains(t, err, "parse tenant settings")
	})

	t.Run("update error", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t,
			withReportsDryRunFixtures(reportsDryRunFixtures{
				tenants: []models.Tenant{{ID: tenantID, Settings: json.RawMessage(`{}`)}},
			}, nil),
			withReportsDryRunUpdateError(updateErr),
		)}

		_, err := repo.UpdateCashFlowMappingOverrides(ctx, tenantID, CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"4000"},
		})
		require.ErrorContains(t, err, "update cash flow mapping")
		assert.ErrorIs(t, err, updateErr)
	})

	t.Run("update affects no rows", func(t *testing.T) {
		repo := &GORMRepository{db: newReportsDryRunDB(t,
			withReportsDryRunFixtures(reportsDryRunFixtures{
				tenants: []models.Tenant{{ID: tenantID, Settings: json.RawMessage(`{}`)}},
			}, nil),
			withReportsDryRunUpdateRows(nil, 0),
		)}

		_, err := repo.UpdateCashFlowMappingOverrides(ctx, tenantID, CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"4000"},
		})
		require.ErrorContains(t, err, "tenant not found")
	})
}

func assertRecordedSQLContains(t *testing.T, sqlStatements []string, fragments ...string) {
	t.Helper()

	allSQL := strings.Join(sqlStatements, "\n")
	require.NotEmpty(t, allSQL)
	for _, fragment := range fragments {
		assert.Contains(t, allSQL, fragment)
	}
}
