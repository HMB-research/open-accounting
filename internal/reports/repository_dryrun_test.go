package reports

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
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

var reportsDryRunCallbackID uint64

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
		`FROM "tenant_reports"."invoices" AS i`,
		`JOIN "tenant_reports"."contacts" AS c ON i.contact_id = c.id AND i.tenant_id = c.tenant_id`,
		`FROM "tenant_reports"."payments" AS p`,
		`JOIN "tenant_reports"."invoice_lines" AS il ON il.invoice_id = i.id AND il.tenant_id = i.tenant_id`,
		`LEFT JOIN "tenant_reports"."products" AS p ON p.id = il.product_id AND p.tenant_id = i.tenant_id`,
	)
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
