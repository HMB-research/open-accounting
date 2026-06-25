package invoicing

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
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

type invoicingDryRunConnPool struct{}

func (invoicingDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run invoicing tests should not prepare statements")
}

func (invoicingDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run invoicing tests should not execute statements")
}

func (invoicingDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run invoicing tests should not query rows")
}

func (invoicingDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (invoicingDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &invoicingDryRunTx{}, nil
}

type invoicingDryRunTx struct {
	invoicingDryRunConnPool
}

func (*invoicingDryRunTx) Commit() error {
	return nil
}

func (*invoicingDryRunTx) Rollback() error {
	return nil
}

type invoicingDryRunDBOption func(t *testing.T, db *gorm.DB)

type invoicingDryRunFixtures struct {
	invoice          *models.Invoice
	invoices         []models.Invoice
	invoiceLines     []models.InvoiceLine
	sequence         *int
	reminders        []models.PaymentReminder
	rule             *models.ReminderRule
	rules            []models.ReminderRule
	interest         *models.InvoiceInterest
	interests        []models.InvoiceInterest
	interestInvoice  *interestInvoice
	interestInvoices []interestInvoice
	counts           []int64
	lastSentAt       *time.Time
	scanRows         []map[string]interface{}
}

type invoicingDryRunSQLCapture struct {
	statements []string
}

func newInvoicingDryRunDB(t *testing.T, opts ...invoicingDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: invoicingDryRunConnPool{}}), &gorm.Config{
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

func withInvoicingDryRunFixtures(fixtures invoicingDryRunFixtures) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var countIndex int
		err := db.Callback().Query().After("gorm:query").Register(invoicingDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *models.Invoice:
				if fixtures.invoice != nil {
					*dest = *fixtures.invoice
					tx.RowsAffected = 1
				}
			case *[]models.Invoice:
				if fixtures.invoices != nil {
					*dest = append([]models.Invoice(nil), fixtures.invoices...)
					tx.RowsAffected = int64(len(fixtures.invoices))
				}
			case *[]models.InvoiceLine:
				if fixtures.invoiceLines != nil {
					*dest = append([]models.InvoiceLine(nil), fixtures.invoiceLines...)
					tx.RowsAffected = int64(len(fixtures.invoiceLines))
				}
			case *int:
				if fixtures.sequence != nil {
					*dest = *fixtures.sequence
					tx.RowsAffected = 1
				}
			case *[]models.PaymentReminder:
				if fixtures.reminders != nil {
					*dest = append([]models.PaymentReminder(nil), fixtures.reminders...)
					tx.RowsAffected = int64(len(fixtures.reminders))
				}
			case *models.ReminderRule:
				if fixtures.rule != nil {
					*dest = *fixtures.rule
					tx.RowsAffected = 1
				}
			case *[]models.ReminderRule:
				if fixtures.rules != nil {
					*dest = append([]models.ReminderRule(nil), fixtures.rules...)
					tx.RowsAffected = int64(len(fixtures.rules))
				}
			case *models.InvoiceInterest:
				if fixtures.interest != nil {
					*dest = *fixtures.interest
					tx.RowsAffected = 1
				}
			case *[]models.InvoiceInterest:
				if fixtures.interests != nil {
					*dest = append([]models.InvoiceInterest(nil), fixtures.interests...)
					tx.RowsAffected = int64(len(fixtures.interests))
				}
			case *interestInvoice:
				if fixtures.interestInvoice != nil {
					*dest = *fixtures.interestInvoice
					tx.RowsAffected = 1
				}
			case *[]interestInvoice:
				if fixtures.interestInvoices != nil {
					*dest = append([]interestInvoice(nil), fixtures.interestInvoices...)
					tx.RowsAffected = int64(len(fixtures.interestInvoices))
				}
			case *int64:
				*dest = invoicingDryRunNextCount(fixtures.counts, &countIndex)
				tx.RowsAffected = 1
			default:
				if invoicingDryRunFillAnonymousDest(dest, fixtures, &countIndex) {
					tx.RowsAffected = int64(max(1, len(fixtures.scanRows)))
				}
			}
		})
		require.NoError(t, err)
	}
}

func withInvoicingDryRunSQLCapture(capture *invoicingDryRunSQLCapture) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(invoicingDryRunCallbackName(t, "capture_create"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Query().After("gorm:query").Register(invoicingDryRunCallbackName(t, "capture_query"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Update().After("gorm:update").Register(invoicingDryRunCallbackName(t, "capture_update"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Delete().After("gorm:delete").Register(invoicingDryRunCallbackName(t, "capture_delete"), capture.capture)
		require.NoError(t, err)
	}
}

func withInvoicingDryRunQueryError(expectedErr error) invoicingDryRunDBOption {
	return withInvoicingDryRunQueryErrorOnCall(1, expectedErr)
}

func withInvoicingDryRunQueryErrorOnCall(call int, expectedErr error) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var calls int
		err := db.Callback().Query().Before("gorm:query").Register(invoicingDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			calls++
			if calls == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func withInvoicingDryRunCreateErrorOnCall(call int, expectedErr error) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var calls int
		err := db.Callback().Create().Before("gorm:create").Register(invoicingDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			calls++
			if calls == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func withInvoicingDryRunUpdateRows(rows ...int64) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(invoicingDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
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

func withInvoicingDryRunUpdateError(expectedErr error) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(invoicingDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withInvoicingDryRunDeleteRows(rows ...int64) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(invoicingDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
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

func withInvoicingDryRunDeleteError(expectedErr error) invoicingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Delete().Before("gorm:delete").Register(invoicingDryRunCallbackName(t, "delete_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func invoicingDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "invoicing_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func invoicingDryRunNextCount(counts []int64, index *int) int64 {
	if len(counts) == 0 {
		return 0
	}
	count := counts[len(counts)-1]
	if *index < len(counts) {
		count = counts[*index]
	}
	*index++
	return count
}

func invoicingDryRunFillAnonymousDest(dest interface{}, fixtures invoicingDryRunFixtures, countIndex *int) bool {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}

	elem := value.Elem()
	switch elem.Kind() {
	case reflect.Struct:
		countField := elem.FieldByName("Count")
		if countField.IsValid() && countField.CanSet() && countField.Kind() == reflect.Int {
			countField.SetInt(invoicingDryRunNextCount(fixtures.counts, countIndex))
			lastSentAtField := elem.FieldByName("LastSentAt")
			if lastSentAtField.IsValid() && lastSentAtField.CanSet() && fixtures.lastSentAt != nil {
				lastSentAtField.Set(reflect.ValueOf(fixtures.lastSentAt))
			}
			return true
		}
	case reflect.Slice:
		if len(fixtures.scanRows) == 0 {
			return false
		}
		rowType := elem.Type().Elem()
		if rowType.Kind() != reflect.Struct {
			return false
		}
		rows := reflect.MakeSlice(elem.Type(), 0, len(fixtures.scanRows))
		for _, scanRow := range fixtures.scanRows {
			row := reflect.New(rowType).Elem()
			for name, fieldValue := range scanRow {
				invoicingDryRunSetField(row, name, fieldValue)
			}
			rows = reflect.Append(rows, row)
		}
		elem.Set(rows)
		return true
	}
	return false
}

func invoicingDryRunSetField(row reflect.Value, name string, value interface{}) {
	field := row.FieldByName(name)
	if !field.IsValid() || !field.CanSet() {
		return
	}
	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return
	}
	fieldValue := reflect.ValueOf(value)
	if fieldValue.Type().AssignableTo(field.Type()) {
		field.Set(fieldValue)
		return
	}
	if fieldValue.Type().ConvertibleTo(field.Type()) {
		field.Set(fieldValue.Convert(field.Type()))
	}
}

func (c *invoicingDryRunSQLCapture) capture(tx *gorm.DB) {
	if sql := strings.TrimSpace(tx.Statement.SQL.String()); sql != "" {
		c.statements = append(c.statements, sql)
	}
}

func (c *invoicingDryRunSQLCapture) assertContains(t *testing.T, expected string) {
	t.Helper()
	for _, statement := range c.statements {
		if strings.Contains(statement, expected) {
			return
		}
	}
	t.Fatalf("expected dry-run SQL to contain %q in %#v", expected, c.statements)
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "11111111-1111-1111-1111-111111111111"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	invoice := invoicingDryRunInvoice(tenantID, now)
	invoiceModel := invoiceToModel(invoice)
	lineModel := invoiceLineToModel(&invoice.Lines[0])
	capture := &invoicingDryRunSQLCapture{}
	repo := NewGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{
			invoice:      invoiceModel,
			invoices:     []models.Invoice{*invoiceModel},
			invoiceLines: []models.InvoiceLine{*lineModel},
		}),
		withInvoicingDryRunUpdateRows(1, 1, 2),
		withInvoicingDryRunSQLCapture(capture),
	))

	require.NoError(t, repo.Create(ctx, schemaName, invoice))
	assert.Equal(t, invoice.ID, invoice.Lines[0].InvoiceID)

	gotInvoice, err := repo.GetByID(ctx, schemaName, tenantID, invoice.ID)
	require.NoError(t, err)
	require.Len(t, gotInvoice.Lines, 1)
	assert.Equal(t, invoice.ID, gotInvoice.ID)
	assert.Equal(t, invoice.Lines[0].ID, gotInvoice.Lines[0].ID)

	fromDate := now.AddDate(0, 0, -7)
	toDate := now.AddDate(0, 0, 7)
	listedInvoices, err := repo.List(ctx, schemaName, tenantID, &InvoiceFilter{
		InvoiceType: InvoiceTypeSales,
		Status:      StatusSent,
		ContactID:   invoice.ContactID,
		FromDate:    &fromDate,
		ToDate:      &toDate,
		Search:      "INV-00042",
	})
	require.NoError(t, err)
	require.Len(t, listedInvoices, 1)
	assert.Equal(t, invoice.InvoiceNumber, listedInvoices[0].InvoiceNumber)

	require.NoError(t, repo.UpdateStatus(ctx, schemaName, tenantID, invoice.ID, StatusPaid))
	require.NoError(t, repo.UpdatePayment(ctx, schemaName, tenantID, invoice.ID, decimal.RequireFromString("122.00"), StatusPaid))
	updated, err := repo.UpdateOverdueStatus(ctx, schemaName, tenantID)
	require.NoError(t, err)
	assert.Equal(t, 2, updated)

	capture.assertContains(t, `"tenant_invoicing"."invoices"`)
	capture.assertContains(t, `"tenant_invoicing"."invoice_lines"`)
	capture.assertContains(t, `invoice_number ILIKE`)
	capture.assertContains(t, `ORDER BY issue_date DESC`)
	capture.assertContains(t, `line_number`)
}

func TestGORMRepositoryDryRunInvalidSchema(t *testing.T) {
	ctx := context.Background()
	invalidSchema := "tenant-invoicing"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	invoice := invoicingDryRunInvoice(tenantID, now)
	repo := NewGORMRepository(newInvoicingDryRunDB(t))

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, invalidSchema, invoice)
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				got, err := repo.GetByID(ctx, invalidSchema, tenantID, invoice.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				got, err := repo.List(ctx, invalidSchema, tenantID, &InvoiceFilter{Status: StatusDraft})
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				return repo.UpdateStatus(ctx, invalidSchema, tenantID, invoice.ID, StatusSent)
			},
		},
		{
			name: "UpdatePayment",
			run: func(t *testing.T) error {
				return repo.UpdatePayment(ctx, invalidSchema, tenantID, invoice.ID, decimal.NewFromInt(1), StatusPartiallyPaid)
			},
		},
		{
			name: "GenerateNumber",
			run: func(t *testing.T) error {
				number, err := repo.GenerateNumber(ctx, invalidSchema, tenantID, InvoiceTypeSales)
				assert.Empty(t, number)
				return err
			},
		},
		{
			name: "UpdateOverdueStatus",
			run: func(t *testing.T) error {
				count, err := repo.UpdateOverdueStatus(ctx, invalidSchema, tenantID)
				assert.Zero(t, count)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid SQL identifier")
		})
	}
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	invoice := invoicingDryRunInvoice(tenantID, now)
	invoiceModel := invoiceToModel(invoice)

	t.Run("Create wraps invoice insert errors", func(t *testing.T) {
		expectedErr := errors.New("invoice insert failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunCreateErrorOnCall(1, expectedErr)))

		err := repo.Create(ctx, schemaName, invoice)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "insert invoice")
	})

	t.Run("Create wraps invoice line insert errors", func(t *testing.T) {
		expectedErr := errors.New("invoice line insert failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunCreateErrorOnCall(2, expectedErr)))

		err := repo.Create(ctx, schemaName, invoice)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "insert invoice line")
	})

	t.Run("GetByID returns domain not found", func(t *testing.T) {
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(gorm.ErrRecordNotFound)))

		got, err := repo.GetByID(ctx, schemaName, tenantID, invoice.ID)

		assert.Nil(t, got)
		require.ErrorIs(t, err, ErrInvoiceNotFound)
	})

	t.Run("GetByID wraps invoice query errors", func(t *testing.T) {
		expectedErr := errors.New("invoice query failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.GetByID(ctx, schemaName, tenantID, invoice.ID)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "get invoice")
	})

	t.Run("GetByID wraps line query errors", func(t *testing.T) {
		expectedErr := errors.New("line query failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t,
			withInvoicingDryRunFixtures(invoicingDryRunFixtures{invoice: invoiceModel}),
			withInvoicingDryRunQueryErrorOnCall(2, expectedErr),
		))

		got, err := repo.GetByID(ctx, schemaName, tenantID, invoice.ID)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "get invoice lines")
	})

	t.Run("List wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("list query failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.List(ctx, schemaName, tenantID, nil)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "list invoices")
	})

	t.Run("UpdateStatus wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("status update failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateError(expectedErr)))

		err := repo.UpdateStatus(ctx, schemaName, tenantID, invoice.ID, StatusSent)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "update status")
	})

	t.Run("UpdateStatus returns not found when no rows change", func(t *testing.T) {
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateRows(0)))

		err := repo.UpdateStatus(ctx, schemaName, tenantID, invoice.ID, StatusSent)

		require.ErrorIs(t, err, ErrInvoiceNotFound)
	})

	t.Run("UpdatePayment wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("payment update failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateError(expectedErr)))

		err := repo.UpdatePayment(ctx, schemaName, tenantID, invoice.ID, decimal.NewFromInt(100), StatusPaid)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "update payment")
	})

	t.Run("UpdatePayment returns not found when no rows change", func(t *testing.T) {
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateRows(0)))

		err := repo.UpdatePayment(ctx, schemaName, tenantID, invoice.ID, decimal.NewFromInt(100), StatusPaid)

		require.ErrorIs(t, err, ErrInvoiceNotFound)
	})

	t.Run("GenerateNumber wraps dry-run scan errors", func(t *testing.T) {
		tests := []InvoiceType{InvoiceTypeSales, InvoiceTypePurchase, InvoiceTypeCreditNote}
		for _, invoiceType := range tests {
			t.Run(string(invoiceType), func(t *testing.T) {
				repo := NewGORMRepository(newInvoicingDryRunDB(t))

				number, err := repo.GenerateNumber(ctx, schemaName, tenantID, invoiceType)

				assert.Empty(t, number)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "generate invoice number")
				assert.Contains(t, err.Error(), "dry run mode unsupported")
			})
		}
	})

	t.Run("UpdateOverdueStatus wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("overdue update failed")
		repo := NewGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateError(expectedErr)))

		count, err := repo.UpdateOverdueStatus(ctx, schemaName, tenantID)

		assert.Zero(t, count)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "update overdue status")
	})
}

func TestReminderGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "11111111-1111-1111-1111-111111111111"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	reminder := invoicingDryRunPaymentReminder(tenantID, now)
	reminderModel := paymentReminderToModel(reminder)
	capture := &invoicingDryRunSQLCapture{}
	repo := NewReminderGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{
			reminders: []models.PaymentReminder{*reminderModel},
		}),
		withInvoicingDryRunSQLCapture(capture),
	))

	require.NoError(t, repo.CreateReminder(ctx, schemaName, reminder))
	require.NoError(t, repo.UpdateReminderStatus(ctx, schemaName, reminder.ID, ReminderStatusSent, &now, ""))

	reminders, err := repo.GetRemindersByInvoice(ctx, schemaName, tenantID, "invoice-1")
	require.NoError(t, err)
	require.Len(t, reminders, 1)
	assert.Equal(t, reminder.ID, reminders[0].ID)
	assert.Equal(t, reminder.ContactEmail, reminders[0].ContactEmail)

	capture.assertContains(t, `"tenant_invoicing"."payment_reminders"`)
	capture.assertContains(t, `ORDER BY created_at DESC`)
}

func TestReminderGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	reminder := invoicingDryRunPaymentReminder(tenantID, now)

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewReminderGORMRepository(newInvoicingDryRunDB(t))

		got, err := repo.GetRemindersByInvoice(ctx, "tenant-invoicing", tenantID, "invoice-1")

		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SQL identifier")
	})

	t.Run("GetOverdueInvoices wraps scan errors", func(t *testing.T) {
		repo := NewReminderGORMRepository(newInvoicingDryRunDB(t))

		got, err := repo.GetOverdueInvoices(ctx, schemaName, tenantID, now)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query overdue invoices")
		assert.Contains(t, err.Error(), "dry run mode unsupported")
	})

	t.Run("GetReminderCount wraps scan errors", func(t *testing.T) {
		repo := NewReminderGORMRepository(newInvoicingDryRunDB(t))

		count, lastSentAt, err := repo.GetReminderCount(ctx, schemaName, tenantID, "invoice-1")

		assert.Zero(t, count)
		assert.Nil(t, lastSentAt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query reminder count")
	})

	t.Run("CreateReminder wraps create errors", func(t *testing.T) {
		expectedErr := errors.New("create reminder failed")
		repo := NewReminderGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunCreateErrorOnCall(1, expectedErr)))

		err := repo.CreateReminder(ctx, schemaName, reminder)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "insert reminder")
	})

	t.Run("UpdateReminderStatus wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("update reminder failed")
		repo := NewReminderGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateError(expectedErr)))

		err := repo.UpdateReminderStatus(ctx, schemaName, reminder.ID, ReminderStatusFailed, nil, "smtp error")

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "update reminder status")
	})

	t.Run("GetRemindersByInvoice wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("query reminders failed")
		repo := NewReminderGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.GetRemindersByInvoice(ctx, schemaName, tenantID, "invoice-1")

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "query reminders")
	})
}

func TestReminderRuleGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "11111111-1111-1111-1111-111111111111"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	rule := invoicingDryRunReminderRule(tenantID, now)
	ruleModel := reminderRuleToModel(rule)
	reminder := invoicingDryRunPaymentReminder(tenantID, now)
	capture := &invoicingDryRunSQLCapture{}
	repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{
			rule:   ruleModel,
			rules:  []models.ReminderRule{*ruleModel},
			counts: []int64{1},
		}),
		withInvoicingDryRunUpdateRows(1),
		withInvoicingDryRunDeleteRows(1),
		withInvoicingDryRunSQLCapture(capture),
	))

	rules, err := repo.ListRules(ctx, schemaName, tenantID)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, rule.ID, rules[0].ID)

	activeRules, err := repo.ListActiveRules(ctx, schemaName, tenantID)
	require.NoError(t, err)
	require.Len(t, activeRules, 1)

	gotRule, err := repo.GetRule(ctx, schemaName, tenantID, rule.ID)
	require.NoError(t, err)
	assert.Equal(t, rule.ID, gotRule.ID)

	require.NoError(t, repo.CreateRule(ctx, schemaName, rule))
	require.NoError(t, repo.UpdateRule(ctx, schemaName, rule))
	require.NoError(t, repo.DeleteRule(ctx, schemaName, tenantID, rule.ID))

	sent, err := repo.HasReminderBeenSent(ctx, schemaName, tenantID, "invoice-1", rule.ID)
	require.NoError(t, err)
	assert.True(t, sent)
	require.NoError(t, repo.RecordReminderSent(ctx, schemaName, reminder))

	capture.assertContains(t, `"tenant_invoicing"."reminder_rules"`)
	capture.assertContains(t, `"tenant_invoicing"."payment_reminders"`)
}

func TestReminderRuleGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	rule := invoicingDryRunReminderRule(tenantID, now)
	reminder := invoicingDryRunPaymentReminder(tenantID, now)

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t))

		got, err := repo.ListRules(ctx, "tenant-invoicing", tenantID)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SQL identifier")
	})

	t.Run("GetRule returns domain not found", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(gorm.ErrRecordNotFound)))

		got, err := repo.GetRule(ctx, schemaName, tenantID, rule.ID)

		assert.Nil(t, got)
		require.ErrorIs(t, err, ErrRuleNotFound)
	})

	t.Run("ListActiveRules wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("active rule query failed")
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.ListActiveRules(ctx, schemaName, tenantID)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "query active rules")
	})

	t.Run("CreateRule wraps create errors", func(t *testing.T) {
		expectedErr := errors.New("create rule failed")
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunCreateErrorOnCall(1, expectedErr)))

		err := repo.CreateRule(ctx, schemaName, rule)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "insert rule")
	})

	t.Run("UpdateRule wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("update rule failed")
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateError(expectedErr)))

		err := repo.UpdateRule(ctx, schemaName, rule)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "update rule")
	})

	t.Run("UpdateRule returns not found when no rows change", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunUpdateRows(0)))

		err := repo.UpdateRule(ctx, schemaName, rule)

		require.ErrorIs(t, err, ErrRuleNotFound)
	})

	t.Run("DeleteRule wraps delete errors", func(t *testing.T) {
		expectedErr := errors.New("delete rule failed")
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunDeleteError(expectedErr)))

		err := repo.DeleteRule(ctx, schemaName, tenantID, rule.ID)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "delete rule")
	})

	t.Run("DeleteRule returns not found when no rows change", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunDeleteRows(0)))

		err := repo.DeleteRule(ctx, schemaName, tenantID, rule.ID)

		require.ErrorIs(t, err, ErrRuleNotFound)
	})

	t.Run("GetInvoicesForRule wraps scan errors", func(t *testing.T) {
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t))

		got, err := repo.GetInvoicesForRule(ctx, schemaName, tenantID, rule, now)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "query invoices for rule")
		assert.Contains(t, err.Error(), "dry run mode unsupported")
	})

	t.Run("GetInvoicesForRule supports before and on due trigger branches", func(t *testing.T) {
		for _, triggerType := range []TriggerType{TriggerBeforeDue, TriggerOnDue} {
			triggerType := triggerType
			t.Run(string(triggerType), func(t *testing.T) {
				repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t))
				branchRule := *rule
				branchRule.TriggerType = triggerType

				got, err := repo.GetInvoicesForRule(ctx, schemaName, tenantID, &branchRule, now)

				assert.Nil(t, got)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "query invoices for rule")
				assert.Contains(t, err.Error(), "dry run mode unsupported")
			})
		}
	})

	t.Run("HasReminderBeenSent wraps count errors", func(t *testing.T) {
		expectedErr := errors.New("count failed")
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		sent, err := repo.HasReminderBeenSent(ctx, schemaName, tenantID, "invoice-1", rule.ID)

		assert.False(t, sent)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "check reminder sent")
	})

	t.Run("RecordReminderSent wraps create errors", func(t *testing.T) {
		expectedErr := errors.New("record reminder failed")
		repo := NewReminderRuleGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunCreateErrorOnCall(1, expectedErr)))

		err := repo.RecordReminderSent(ctx, schemaName, reminder)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "insert reminder")
	})
}

func TestInterestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "11111111-1111-1111-1111-111111111111"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	invoice := interestInvoice{
		ID:            "invoice-1",
		InvoiceNumber: "INV-00042",
		DueDate:       now.AddDate(0, 0, -5),
		Total:         decimal.RequireFromString("122.00"),
		AmountPaid:    decimal.RequireFromString("22.00"),
		Currency:      "EUR",
		Status:        string(StatusOverdue),
	}
	interest := invoicingDryRunInterest(now)
	interestModel := invoiceInterestToModel(interest)
	capture := &invoicingDryRunSQLCapture{}
	repo := NewInterestGORMRepository(newInvoicingDryRunDB(t,
		withInvoicingDryRunFixtures(invoicingDryRunFixtures{
			interestInvoice:  &invoice,
			interestInvoices: []interestInvoice{invoice},
			interest:         interestModel,
			interests:        []models.InvoiceInterest{*interestModel},
		}),
		withInvoicingDryRunSQLCapture(capture),
	))

	gotInvoice, err := repo.GetInvoiceForInterest(ctx, schemaName, tenantID, invoice.ID)
	require.NoError(t, err)
	assert.Equal(t, invoice.ID, gotInvoice.ID)
	assert.True(t, gotInvoice.Total.Equal(invoice.Total))

	require.NoError(t, repo.CreateInterest(ctx, schemaName, interest))

	latest, err := repo.GetLatestInterest(ctx, schemaName, invoice.ID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, interest.ID, latest.ID)

	history, err := repo.ListInterestHistory(ctx, schemaName, invoice.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)

	overdue, err := repo.ListOverdueInvoices(ctx, schemaName, tenantID, now)
	require.NoError(t, err)
	require.Len(t, overdue, 1)
	assert.Equal(t, invoice.InvoiceNumber, overdue[0].InvoiceNumber)

	capture.assertContains(t, `"tenant_invoicing"."invoices"`)
	capture.assertContains(t, `"tenant_invoicing"."invoice_interest"`)
	capture.assertContains(t, `ORDER BY calculated_at DESC`)
	capture.assertContains(t, `ORDER BY due_date ASC`)
}

func TestInterestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	interest := invoicingDryRunInterest(now)

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t))

		got, err := repo.ListInterestHistory(ctx, "tenant-invoicing", "invoice-1")

		assert.Nil(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SQL identifier")
	})

	t.Run("GetInvoiceForInterest returns domain not found", func(t *testing.T) {
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(gorm.ErrRecordNotFound)))

		got, err := repo.GetInvoiceForInterest(ctx, schemaName, tenantID, "invoice-1")

		assert.Nil(t, got)
		var notFound *NotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Equal(t, "invoice", notFound.Entity)
	})

	t.Run("GetInvoiceForInterest wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("invoice query failed")
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.GetInvoiceForInterest(ctx, schemaName, tenantID, "invoice-1")

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "get invoice")
	})

	t.Run("CreateInterest wraps create errors", func(t *testing.T) {
		expectedErr := errors.New("create interest failed")
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunCreateErrorOnCall(1, expectedErr)))

		err := repo.CreateInterest(ctx, schemaName, interest)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "save interest calculation")
	})

	t.Run("GetLatestInterest returns nil when not found", func(t *testing.T) {
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(gorm.ErrRecordNotFound)))

		got, err := repo.GetLatestInterest(ctx, schemaName, "invoice-1")

		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("GetLatestInterest wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("latest interest query failed")
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.GetLatestInterest(ctx, schemaName, "invoice-1")

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "get latest interest")
	})

	t.Run("ListInterestHistory wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("history query failed")
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.ListInterestHistory(ctx, schemaName, "invoice-1")

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "list interest history")
	})

	t.Run("ListOverdueInvoices wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("overdue interest query failed")
		repo := NewInterestGORMRepository(newInvoicingDryRunDB(t, withInvoicingDryRunQueryError(expectedErr)))

		got, err := repo.ListOverdueInvoices(ctx, schemaName, tenantID, now)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "list overdue invoices")
	})
}

func invoicingDryRunInvoice(tenantID string, now time.Time) *Invoice {
	productID := "product-1"
	return &Invoice{
		ID:            "invoice-1",
		TenantID:      tenantID,
		InvoiceNumber: "INV-00042",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     "contact-1",
		IssueDate:     now,
		DueDate:       now.AddDate(0, 0, 14),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		Subtotal:      decimal.RequireFromString("100.00"),
		VATAmount:     decimal.RequireFromString("22.00"),
		Total:         decimal.RequireFromString("122.00"),
		BaseSubtotal:  decimal.RequireFromString("100.00"),
		BaseVATAmount: decimal.RequireFromString("22.00"),
		BaseTotal:     decimal.RequireFromString("122.00"),
		AmountPaid:    decimal.Zero,
		Status:        StatusSent,
		Reference:     "PO-42",
		Notes:         "dry-run invoice",
		CreatedAt:     now,
		CreatedBy:     "user-1",
		UpdatedAt:     now,
		Lines: []InvoiceLine{
			{
				ID:              "invoice-line-1",
				TenantID:        tenantID,
				InvoiceID:       "invoice-1",
				LineNumber:      1,
				Description:     "Consulting",
				Quantity:        decimal.NewFromInt(1),
				Unit:            "hour",
				UnitPrice:       decimal.RequireFromString("100.00"),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.RequireFromString("22.00"),
				VATTreatment:    VATTreatmentStandard,
				LineSubtotal:    decimal.RequireFromString("100.00"),
				LineVAT:         decimal.RequireFromString("22.00"),
				LineTotal:       decimal.RequireFromString("122.00"),
				ProductID:       &productID,
			},
		},
	}
}

func invoicingDryRunPaymentReminder(tenantID string, now time.Time) *PaymentReminder {
	ruleID := "rule-1"
	return &PaymentReminder{
		ID:             "reminder-1",
		TenantID:       tenantID,
		InvoiceID:      "invoice-1",
		InvoiceNumber:  "INV-00042",
		ContactID:      "contact-1",
		ContactName:    "Customer OU",
		ContactEmail:   "customer@example.com",
		RuleID:         &ruleID,
		TriggerType:    string(TriggerAfterDue),
		DaysOffset:     5,
		ReminderNumber: 2,
		Status:         ReminderStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func invoicingDryRunReminderRule(tenantID string, now time.Time) *ReminderRule {
	return &ReminderRule{
		ID:                "rule-1",
		TenantID:          tenantID,
		Name:              "Five days overdue",
		TriggerType:       TriggerAfterDue,
		DaysOffset:        5,
		EmailTemplateType: "OVERDUE_REMINDER",
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func invoicingDryRunInterest(now time.Time) *InvoiceInterest {
	return &InvoiceInterest{
		ID:                "interest-1",
		InvoiceID:         "invoice-1",
		CalculatedAt:      now,
		DaysOverdue:       5,
		PrincipalAmount:   decimal.RequireFromString("100.00"),
		InterestRate:      decimal.RequireFromString("0.001"),
		InterestAmount:    decimal.RequireFromString("0.50"),
		TotalWithInterest: decimal.RequireFromString("100.50"),
		CreatedAt:         now,
	}
}
