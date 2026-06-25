package recurring

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

type recurringDryRunConnPool struct{}

func (recurringDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run recurring tests should not prepare statements")
}

func (recurringDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run recurring tests should not execute statements")
}

func (recurringDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run recurring tests should not query rows")
}

func (recurringDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (recurringDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &recurringDryRunTx{}, nil
}

type recurringDryRunTx struct {
	recurringDryRunConnPool
}

func (*recurringDryRunTx) Commit() error {
	return nil
}

func (*recurringDryRunTx) Rollback() error {
	return nil
}

type recurringDryRunDBOption func(t *testing.T, db *gorm.DB)

type recurringDryRunFixtures struct {
	invoice     *models.RecurringInvoice
	invoices    []models.RecurringInvoice
	lines       []models.RecurringInvoiceLine
	dueIDs      []string
	contactName string
}

type recurringDryRunRecorder struct {
	queries         []string
	creates         []string
	updates         []string
	deletes         []string
	createdInvoices []models.RecurringInvoice
	createdLines    []models.RecurringInvoiceLine
}

func newRecurringDryRunDB(t *testing.T, opts ...recurringDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: recurringDryRunConnPool{}}), &gorm.Config{
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

func withRecurringDryRunFixtures(fixtures recurringDryRunFixtures, recorder *recurringDryRunRecorder) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(recurringDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.queries = append(recorder.queries, tx.Statement.SQL.String())
			}
			populateRecurringDryRunQueryDest(tx, tx.Statement.Dest, &fixtures)
		})
		require.NoError(t, err)
	}
}

func withRecurringDryRunQueryErrors(queryErrors ...error) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(recurringDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
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

func withRecurringDryRunCreateCapture(recorder *recurringDryRunRecorder) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(recurringDryRunCallbackName(t, "create_capture"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.creates = append(recorder.creates, tx.Statement.SQL.String())
				switch dest := tx.Statement.Dest.(type) {
				case *models.RecurringInvoice:
					recorder.createdInvoices = append(recorder.createdInvoices, *dest)
				case *models.RecurringInvoiceLine:
					recorder.createdLines = append(recorder.createdLines, *dest)
				}
			}
			if tx.RowsAffected == 0 {
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func withRecurringDryRunCreateErrors(createErrors ...error) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Create().Before("gorm:create").Register(recurringDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			if len(createErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(createErrors) {
				errIndex = len(createErrors) - 1
			}
			index++
			if createErrors[errIndex] != nil {
				tx.AddError(createErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withRecurringDryRunUpdateRows(recorder *recurringDryRunRecorder, rows ...int64) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(recurringDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
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

func withRecurringDryRunUpdateErrors(updateErrors ...error) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().Before("gorm:update").Register(recurringDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			if len(updateErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(updateErrors) {
				errIndex = len(updateErrors) - 1
			}
			index++
			if updateErrors[errIndex] != nil {
				tx.AddError(updateErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func withRecurringDryRunDeleteRows(recorder *recurringDryRunRecorder, rows ...int64) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(recurringDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.deletes = append(recorder.deletes, tx.Statement.SQL.String())
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

func withRecurringDryRunDeleteErrors(deleteErrors ...error) recurringDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().Before("gorm:delete").Register(recurringDryRunCallbackName(t, "delete_error"), func(tx *gorm.DB) {
			if len(deleteErrors) == 0 {
				return
			}
			errIndex := index
			if errIndex >= len(deleteErrors) {
				errIndex = len(deleteErrors) - 1
			}
			index++
			if deleteErrors[errIndex] != nil {
				tx.AddError(deleteErrors[errIndex])
			}
		})
		require.NoError(t, err)
	}
}

func recurringDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "recurring_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func populateRecurringDryRunQueryDest(tx *gorm.DB, dest any, fixtures *recurringDryRunFixtures) {
	switch typed := dest.(type) {
	case *models.RecurringInvoice:
		if fixtures.invoice != nil {
			*typed = *fixtures.invoice
			tx.RowsAffected = 1
		}
	case *[]models.RecurringInvoice:
		if fixtures.invoices != nil {
			*typed = append([]models.RecurringInvoice(nil), fixtures.invoices...)
			tx.RowsAffected = int64(len(fixtures.invoices))
		}
	case *[]models.RecurringInvoiceLine:
		if fixtures.lines != nil {
			*typed = append([]models.RecurringInvoiceLine(nil), fixtures.lines...)
			tx.RowsAffected = int64(len(fixtures.lines))
		}
	case *[]string:
		if fixtures.dueIDs != nil {
			*typed = append([]string(nil), fixtures.dueIDs...)
			tx.RowsAffected = int64(len(fixtures.dueIDs))
		}
	default:
		if populateRecurringDryRunJoinDest(dest, fixtures) {
			tx.RowsAffected = recurringDryRunRowsAffected(dest)
		}
	}
}

func populateRecurringDryRunJoinDest(dest any, fixtures *recurringDryRunFixtures) bool {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return false
	}

	elem := value.Elem()
	switch elem.Kind() {
	case reflect.Struct:
		if fixtures.invoice == nil {
			return false
		}
		return fillRecurringDryRunJoinStruct(elem, *fixtures.invoice, fixtures.contactName)
	case reflect.Slice:
		if fixtures.invoices == nil || elem.Type().Elem().Kind() != reflect.Struct {
			return false
		}
		result := reflect.MakeSlice(elem.Type(), len(fixtures.invoices), len(fixtures.invoices))
		for i, invoice := range fixtures.invoices {
			fillRecurringDryRunJoinStruct(result.Index(i), invoice, fixtures.contactName)
		}
		elem.Set(result)
		return true
	default:
		return false
	}
}

func recurringDryRunRowsAffected(dest any) int64 {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return 0
	}
	elem := value.Elem()
	if elem.Kind() == reflect.Slice {
		return int64(elem.Len())
	}
	return 1
}

func fillRecurringDryRunJoinStruct(target reflect.Value, invoice models.RecurringInvoice, contactName string) bool {
	filled := false
	invoiceField := target.FieldByName("RecurringInvoice")
	if invoiceField.IsValid() && invoiceField.CanSet() && invoiceField.Type() == reflect.TypeOf(models.RecurringInvoice{}) {
		invoiceField.Set(reflect.ValueOf(invoice))
		filled = true
	}
	contactField := target.FieldByName("ContactName")
	if contactField.IsValid() && contactField.CanSet() && contactField.Kind() == reflect.String {
		contactField.SetString(contactName)
		filled = true
	}
	return filled
}

func assertRecurringRecordedSQLContains(t *testing.T, statements []string, fragments ...string) {
	t.Helper()

	joined := strings.Join(statements, "\n")
	for _, fragment := range fragments {
		assert.Contains(t, joined, fragment)
	}
}

func TestNewServiceWithNilDatabaseLeavesRepositoryUnconfigured(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil, nil)

	require.NotNil(t, service)
	assert.Nil(t, service.repo)
	assert.Nil(t, service.invoicing)
	assert.Nil(t, service.email)
	assert.Nil(t, service.pdfSvc)
	assert.Nil(t, service.tenant)
	assert.Nil(t, service.contacts)
}

func TestGORMRepositoryDryRunRecurringOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_recurring"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	invoice := recurringDryRunInvoice(tenantID, now)
	line := recurringDryRunLine(invoice.ID)
	invoiceModel := recurringInvoiceToModel(invoice)
	lineModel := recurringInvoiceLineToModel(line)
	sentAt := now.Add(30 * time.Minute)
	nextDate := now.AddDate(0, 1, 0)
	recorder := &recurringDryRunRecorder{}
	repo := NewGORMRepository(newRecurringDryRunDB(t,
		withRecurringDryRunFixtures(recurringDryRunFixtures{
			invoice:     invoiceModel,
			invoices:    []models.RecurringInvoice{*invoiceModel},
			lines:       []models.RecurringInvoiceLine{*lineModel},
			dueIDs:      []string{invoice.ID},
			contactName: "Acme OU",
		}, recorder),
		withRecurringDryRunCreateCapture(recorder),
		withRecurringDryRunUpdateRows(recorder, 1, 1, 1, 1, 1),
		withRecurringDryRunDeleteRows(recorder, 1, 1),
	))

	require.NoError(t, repo.Create(ctx, schemaName, invoice))
	require.NoError(t, repo.CreateLine(ctx, schemaName, line))
	require.Len(t, recorder.createdInvoices, 1)
	assert.Equal(t, invoice.ID, recorder.createdInvoices[0].ID)
	require.Len(t, recorder.createdLines, 1)
	assert.Equal(t, line.ID, recorder.createdLines[0].ID)

	got, err := repo.GetByID(ctx, schemaName, tenantID, invoice.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, invoice.ID, got.ID)
	assert.Equal(t, "Acme OU", got.ContactName)

	lines, err := repo.GetLines(ctx, schemaName, invoice.ID)
	require.NoError(t, err)
	require.Len(t, lines, 1)
	assert.Equal(t, line.ID, lines[0].ID)

	active, err := repo.List(ctx, schemaName, tenantID, true)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "Acme OU", active[0].ContactName)

	all, err := repo.List(ctx, schemaName, tenantID, false)
	require.NoError(t, err)
	require.Len(t, all, 1)

	require.NoError(t, repo.Update(ctx, schemaName, invoice))
	require.NoError(t, repo.DeleteLines(ctx, schemaName, invoice.ID))
	require.NoError(t, repo.Delete(ctx, schemaName, tenantID, invoice.ID))
	require.NoError(t, repo.SetActive(ctx, schemaName, tenantID, invoice.ID, false))

	dueIDs, err := repo.GetDueRecurringInvoiceIDs(ctx, schemaName, tenantID, now)
	require.NoError(t, err)
	assert.Equal(t, []string{invoice.ID}, dueIDs)

	require.NoError(t, repo.UpdateAfterGeneration(ctx, schemaName, tenantID, invoice.ID, nextDate, now))
	require.NoError(t, repo.UpdateInvoiceEmailStatus(ctx, schemaName, "invoice-1", &sentAt, "SENT", "log-1"))

	assertRecurringRecordedSQLContains(t, recorder.creates,
		`INSERT INTO "tenant_recurring"."recurring_invoices"`,
		`INSERT INTO "tenant_recurring"."recurring_invoice_lines"`,
	)
	assertRecurringRecordedSQLContains(t, recorder.queries,
		`FROM "tenant_recurring"."recurring_invoices" AS r`,
		`LEFT JOIN "tenant_recurring"."contacts" AS c`,
		`r.is_active = $2`,
		`FROM "tenant_recurring"."recurring_invoice_lines"`,
		`ORDER BY line_number`,
		`next_generation_date <= $3`,
	)
	assertRecurringRecordedSQLContains(t, recorder.updates,
		`UPDATE "tenant_recurring"."recurring_invoices"`,
		`"send_email_on_generation"`,
		`"is_active"`,
		`UPDATE "tenant_recurring"."invoices"`,
	)
	assertRecurringRecordedSQLContains(t, recorder.deletes,
		`DELETE FROM "tenant_recurring"."recurring_invoice_lines"`,
		`DELETE FROM "tenant_recurring"."recurring_invoices"`,
	)
}

func TestGORMRepositoryDryRunRecurringErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_recurring"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 13, 0, 0, 0, time.UTC)
	invoice := recurringDryRunInvoice(tenantID, now)
	line := recurringDryRunLine(invoice.ID)
	expectedErr := errors.New("dry-run failed")

	t.Run("rejects invalid tenant schema", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t))

		createErr := repo.Create(ctx, "tenant-recurring", invoice)
		require.Error(t, createErr)
		assert.Contains(t, createErr.Error(), "invalid SQL identifier")

		got, getErr := repo.GetByID(ctx, "tenant-recurring", tenantID, invoice.ID)
		assert.Nil(t, got)
		require.Error(t, getErr)
		assert.Contains(t, getErr.Error(), "invalid SQL identifier")

		listed, listErr := repo.List(ctx, "tenant-recurring", tenantID, true)
		assert.Nil(t, listed)
		require.Error(t, listErr)
		assert.Contains(t, listErr.Error(), "invalid SQL identifier")
	})

	t.Run("wraps create invoice errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunCreateErrors(expectedErr)))

		err := repo.Create(ctx, schemaName, invoice)

		require.ErrorContains(t, err, "create recurring invoice")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps create line errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunCreateErrors(expectedErr)))

		err := repo.CreateLine(ctx, schemaName, line)

		require.ErrorContains(t, err, "create recurring invoice line")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("maps get not found", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunQueryErrors(gorm.ErrRecordNotFound)))

		got, err := repo.GetByID(ctx, schemaName, tenantID, invoice.ID)

		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrRecurringInvoiceNotFound)
	})

	t.Run("wraps get query errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunQueryErrors(expectedErr)))

		got, err := repo.GetByID(ctx, schemaName, tenantID, invoice.ID)

		assert.Nil(t, got)
		require.ErrorContains(t, err, "get recurring invoice")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps get lines errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunQueryErrors(expectedErr)))

		lines, err := repo.GetLines(ctx, schemaName, invoice.ID)

		assert.Nil(t, lines)
		require.ErrorContains(t, err, "get recurring invoice lines")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps list errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunQueryErrors(expectedErr)))

		invoices, err := repo.List(ctx, schemaName, tenantID, true)

		assert.Nil(t, invoices)
		require.ErrorContains(t, err, "list recurring invoices")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunUpdateErrors(expectedErr)))

		err := repo.Update(ctx, schemaName, invoice)

		require.ErrorContains(t, err, "update recurring invoice")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps delete lines errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunDeleteErrors(expectedErr)))

		err := repo.DeleteLines(ctx, schemaName, invoice.ID)

		require.ErrorContains(t, err, "delete recurring invoice lines")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps delete errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunDeleteErrors(expectedErr)))

		err := repo.Delete(ctx, schemaName, tenantID, invoice.ID)

		require.ErrorContains(t, err, "delete recurring invoice")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("maps delete not found", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunDeleteRows(nil, 0)))

		err := repo.Delete(ctx, schemaName, tenantID, invoice.ID)

		assert.ErrorIs(t, err, ErrRecurringInvoiceNotFound)
	})

	t.Run("wraps set active errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunUpdateErrors(expectedErr)))

		err := repo.SetActive(ctx, schemaName, tenantID, invoice.ID, true)

		require.ErrorContains(t, err, "set active")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("maps set active not found", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunUpdateRows(nil, 0)))

		err := repo.SetActive(ctx, schemaName, tenantID, invoice.ID, true)

		assert.ErrorIs(t, err, ErrRecurringInvoiceNotFound)
	})

	t.Run("wraps due invoice query errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunQueryErrors(expectedErr)))

		ids, err := repo.GetDueRecurringInvoiceIDs(ctx, schemaName, tenantID, now)

		assert.Nil(t, ids)
		require.ErrorContains(t, err, "get due recurring invoice IDs")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps update after generation errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunUpdateErrors(expectedErr)))

		err := repo.UpdateAfterGeneration(ctx, schemaName, tenantID, invoice.ID, now.AddDate(0, 1, 0), now)

		require.ErrorContains(t, err, "update after generation")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps invoice email status errors", func(t *testing.T) {
		repo := NewGORMRepository(newRecurringDryRunDB(t, withRecurringDryRunUpdateErrors(expectedErr)))

		err := repo.UpdateInvoiceEmailStatus(ctx, schemaName, "invoice-1", nil, "FAILED", "log-1")

		require.ErrorContains(t, err, "update invoice email status")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func recurringDryRunInvoice(tenantID string, now time.Time) *RecurringInvoice {
	endDate := now.AddDate(1, 0, 0)
	return &RecurringInvoice{
		ID:                     "recurring-1",
		TenantID:               tenantID,
		Name:                   "Monthly service",
		ContactID:              "contact-1",
		InvoiceType:            "SALES",
		Currency:               "EUR",
		Frequency:              FrequencyMonthly,
		StartDate:              now.AddDate(0, -1, 0),
		EndDate:                &endDate,
		NextGenerationDate:     now,
		PaymentTermsDays:       14,
		Reference:              "PO-42",
		Notes:                  "Recurring managed service",
		IsActive:               true,
		GeneratedCount:         2,
		CreatedAt:              now.AddDate(0, -2, 0),
		CreatedBy:              "creator-1",
		UpdatedAt:              now,
		SendEmailOnGeneration:  true,
		EmailTemplateType:      "INVOICE_SEND",
		RecipientEmailOverride: "billing@example.com",
		AttachPDFToEmail:       true,
		EmailSubjectOverride:   "Monthly invoice",
		EmailMessage:           "Thank you.",
	}
}

func recurringDryRunLine(recurringInvoiceID string) *RecurringInvoiceLine {
	accountID := "account-1"
	productID := "product-1"
	return &RecurringInvoiceLine{
		ID:                 "line-1",
		RecurringInvoiceID: recurringInvoiceID,
		LineNumber:         1,
		Description:        "Managed service",
		Quantity:           decimal.NewFromInt(2),
		Unit:               "month",
		UnitPrice:          decimal.RequireFromString("150.00"),
		DiscountPercent:    decimal.Zero,
		VATRate:            decimal.RequireFromString("22.00"),
		AccountID:          &accountID,
		ProductID:          &productID,
	}
}
