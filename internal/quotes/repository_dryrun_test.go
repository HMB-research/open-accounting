package quotes

import (
	"context"
	"database/sql"
	"database/sql/driver"
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

type quoteDryRunConnPool struct{}

func (quoteDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run quotes tests should not prepare statements")
}

func (quoteDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run quotes tests should not execute statements")
}

func (quoteDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run quotes tests should not query rows")
}

func (quoteDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (quoteDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &quoteDryRunTx{}, nil
}

type quoteDryRunTx struct {
	quoteDryRunConnPool
}

func (*quoteDryRunTx) Commit() error {
	return nil
}

func (*quoteDryRunTx) Rollback() error {
	return nil
}

type quoteDryRunDBOption func(t *testing.T, db *gorm.DB)

type quoteDryRunFixtures struct {
	quote      *models.Quote
	quotes     []models.Quote
	quoteLines []models.QuoteLine
	sequence   *int
}

type quoteDryRunSQLCapture struct {
	statements []string
}

func newQuoteDryRunDB(t *testing.T, opts ...quoteDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: quoteDryRunConnPool{}}), &gorm.Config{
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

func withQuoteDryRunFixtures(fixtures quoteDryRunFixtures) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(quoteDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *models.Quote:
				if fixtures.quote != nil {
					*dest = *fixtures.quote
					tx.RowsAffected = 1
				}
			case *[]models.Quote:
				if fixtures.quotes != nil {
					*dest = append([]models.Quote(nil), fixtures.quotes...)
					tx.RowsAffected = int64(len(fixtures.quotes))
				}
			case *[]models.QuoteLine:
				if fixtures.quoteLines != nil {
					*dest = append([]models.QuoteLine(nil), fixtures.quoteLines...)
					tx.RowsAffected = int64(len(fixtures.quoteLines))
				}
			case *int:
				if fixtures.sequence != nil {
					*dest = *fixtures.sequence
					tx.RowsAffected = 1
				}
			}
		})
		require.NoError(t, err)
	}
}

func withQuoteDryRunSQLCapture(capture *quoteDryRunSQLCapture) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(quoteDryRunCallbackName(t, "capture_create"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Query().After("gorm:query").Register(quoteDryRunCallbackName(t, "capture_query"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Update().After("gorm:update").Register(quoteDryRunCallbackName(t, "capture_update"), capture.capture)
		require.NoError(t, err)
		err = db.Callback().Delete().After("gorm:delete").Register(quoteDryRunCallbackName(t, "capture_delete"), capture.capture)
		require.NoError(t, err)
	}
}

func withQuoteDryRunQueryError(expectedErr error) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(quoteDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withQuoteDryRunCreateErrorOnCall(call int, expectedErr error) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var calls int
		err := db.Callback().Create().Before("gorm:create").Register(quoteDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
			calls++
			if calls == call {
				tx.AddError(expectedErr)
			}
		})
		require.NoError(t, err)
	}
}

func withQuoteDryRunUpdateRows(rows ...int64) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(quoteDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
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

func withQuoteDryRunUpdateError(expectedErr error) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(quoteDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withQuoteDryRunDeleteRows(rows ...int64) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(quoteDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
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

func quoteDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "quotes_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func (c *quoteDryRunSQLCapture) capture(tx *gorm.DB) {
	if sql := strings.TrimSpace(tx.Statement.SQL.String()); sql != "" {
		c.statements = append(c.statements, sql)
	}
}

func (c *quoteDryRunSQLCapture) assertContains(t *testing.T, expected string) {
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
	schemaName := "tenant_quotes"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	quote := quoteDryRunQuote(tenantID, now)
	quoteModel := quoteToModel(quote)
	lineModel := quoteLineToModel(&quote.Lines[0])
	sequence := 42
	capture := &quoteDryRunSQLCapture{}
	repo := NewGORMRepository(newQuoteDryRunDB(t,
		withQuoteDryRunFixtures(quoteDryRunFixtures{
			quote:      quoteModel,
			quotes:     []models.Quote{*quoteModel},
			quoteLines: []models.QuoteLine{*lineModel},
			sequence:   &sequence,
		}),
		withQuoteWave11ScanRows(quoteWave11RowSet{
			columns: []string{"seq"},
			values:  [][]driver.Value{{sequence}},
		}),
		withQuoteDryRunUpdateRows(1, 1, 1, 1),
		withQuoteDryRunDeleteRows(1),
		withQuoteDryRunSQLCapture(capture),
	))

	require.NoError(t, repo.Create(ctx, schemaName, quote))
	assert.Equal(t, quote.ID, quote.Lines[0].QuoteID)

	gotQuote, err := repo.GetByID(ctx, schemaName, tenantID, quote.ID)
	require.NoError(t, err)
	require.Len(t, gotQuote.Lines, 1)
	assert.Equal(t, quote.ID, gotQuote.ID)
	assert.Equal(t, quote.Lines[0].ID, gotQuote.Lines[0].ID)

	fromDate := now.AddDate(0, 0, -7)
	toDate := now.AddDate(0, 0, 7)
	listedQuotes, err := repo.List(ctx, schemaName, tenantID, &QuoteFilter{
		Status:    QuoteStatusSent,
		ContactID: quote.ContactID,
		FromDate:  &fromDate,
		ToDate:    &toDate,
		Search:    " Q-00042 ",
	})
	require.NoError(t, err)
	require.Len(t, listedQuotes, 1)
	assert.Equal(t, quote.QuoteNumber, listedQuotes[0].QuoteNumber)

	nextNumber, err := repo.GenerateNumber(ctx, schemaName, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "Q-00042", nextNumber)

	quote.Notes = "updated dry-run quote"
	require.NoError(t, repo.Update(ctx, schemaName, quote))
	require.NoError(t, repo.UpdateStatus(ctx, schemaName, tenantID, quote.ID, QuoteStatusAccepted))
	require.NoError(t, repo.Delete(ctx, schemaName, tenantID, quote.ID))

	require.NoError(t, repo.SetConvertedToOrder(ctx, schemaName, tenantID, quote.ID, "order-1"))
	require.NoError(t, repo.SetConvertedToInvoice(ctx, schemaName, tenantID, quote.ID, "invoice-1"))

	capture.assertContains(t, `"tenant_quotes"."quotes"`)
	capture.assertContains(t, `"tenant_quotes"."quote_lines"`)
	capture.assertContains(t, `quote_number ILIKE`)
	capture.assertContains(t, `ORDER BY quote_date DESC`)
	capture.assertContains(t, `line_number ASC`)
}

func TestGORMRepositoryDryRunInvalidSchema(t *testing.T) {
	ctx := context.Background()
	invalidSchema := "tenant-quotes"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	quote := quoteDryRunQuote(tenantID, now)
	repo := NewGORMRepository(newQuoteDryRunDB(t))

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "Create",
			run: func(t *testing.T) error {
				return repo.Create(ctx, invalidSchema, quote)
			},
		},
		{
			name: "GetByID",
			run: func(t *testing.T) error {
				got, err := repo.GetByID(ctx, invalidSchema, tenantID, quote.ID)
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "List",
			run: func(t *testing.T) error {
				got, err := repo.List(ctx, invalidSchema, tenantID, &QuoteFilter{Status: QuoteStatusDraft})
				assert.Nil(t, got)
				return err
			},
		},
		{
			name: "Update",
			run: func(t *testing.T) error {
				return repo.Update(ctx, invalidSchema, quote)
			},
		},
		{
			name: "UpdateStatus",
			run: func(t *testing.T) error {
				return repo.UpdateStatus(ctx, invalidSchema, tenantID, quote.ID, QuoteStatusSent)
			},
		},
		{
			name: "Delete",
			run: func(t *testing.T) error {
				return repo.Delete(ctx, invalidSchema, tenantID, quote.ID)
			},
		},
		{
			name: "GenerateNumber",
			run: func(t *testing.T) error {
				got, err := repo.GenerateNumber(ctx, invalidSchema, tenantID)
				assert.Empty(t, got)
				return err
			},
		},
		{
			name: "SetConvertedToOrder",
			run: func(t *testing.T) error {
				return repo.SetConvertedToOrder(ctx, invalidSchema, tenantID, quote.ID, "order-1")
			},
		},
		{
			name: "SetConvertedToInvoice",
			run: func(t *testing.T) error {
				return repo.SetConvertedToInvoice(ctx, invalidSchema, tenantID, quote.ID, "invoice-1")
			},
		},
		{
			name: "listQuoteLines",
			run: func(t *testing.T) error {
				got, err := repo.listQuoteLines(ctx, invalidSchema, tenantID, quote.ID)
				assert.Nil(t, got)
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid SQL identifier")
		})
	}
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_quotes"
	tenantID := "tenant-1"
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	quote := quoteDryRunQuote(tenantID, now)

	t.Run("GenerateNumber wraps dry-run scan errors", func(t *testing.T) {
		repo := NewGORMRepository(newQuoteDryRunDB(t))

		got, err := repo.GenerateNumber(ctx, schemaName, tenantID)

		assert.Empty(t, got)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "generate quote number")
		assert.Contains(t, err.Error(), "dry run mode unsupported")
	})

	t.Run("listQuoteLines wraps query errors", func(t *testing.T) {
		expectedErr := errors.New("line query failed")
		repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunQueryError(expectedErr)))

		got, err := repo.listQuoteLines(ctx, schemaName, tenantID, quote.ID)

		assert.Nil(t, got)
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "get quote lines")
	})

	t.Run("Update wraps update errors", func(t *testing.T) {
		expectedErr := errors.New("update failed")
		repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateError(expectedErr)))

		err := repo.Update(ctx, schemaName, quote)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "update quote")
	})

	t.Run("Update returns not found when no rows change", func(t *testing.T) {
		repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunUpdateRows(0)))

		err := repo.Update(ctx, schemaName, quote)

		require.ErrorIs(t, err, ErrQuoteNotFound)
	})

	t.Run("Create wraps quote line insert errors", func(t *testing.T) {
		expectedErr := errors.New("line insert failed")
		repo := NewGORMRepository(newQuoteDryRunDB(t, withQuoteDryRunCreateErrorOnCall(2, expectedErr)))

		err := repo.Create(ctx, schemaName, quote)

		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "insert quote line")
	})
}

type quoteWave11RowSet struct {
	columns []string
	values  [][]driver.Value
}

var quoteWave11RowsDSNID uint64
var quoteWave11RowsDriverOnce sync.Once
var quoteWave11RowsMu sync.Mutex
var quoteWave11RowsByDSN = map[string]quoteWave11RowSet{}

func withQuoteWave11ScanRows(rowSets ...quoteWave11RowSet) quoteDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(quoteDryRunCallbackName(t, "scan_rows_wave11"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing quotes dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newQuoteWave11SQLRows(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func newQuoteWave11SQLRows(t *testing.T, rowSet quoteWave11RowSet) *sql.Rows {
	t.Helper()

	quoteWave11RowsDriverOnce.Do(func() {
		sql.Register("quotes_wave11_rows", quoteWave11RowsDriver{})
	})

	dsn := fmt.Sprintf("quotes-wave11-rows-%d", atomic.AddUint64(&quoteWave11RowsDSNID, 1))
	quoteWave11RowsMu.Lock()
	quoteWave11RowsByDSN[dsn] = rowSet
	quoteWave11RowsMu.Unlock()

	db, err := sql.Open("quotes_wave11_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		quoteWave11RowsMu.Lock()
		delete(quoteWave11RowsByDSN, dsn)
		quoteWave11RowsMu.Unlock()
	})

	return rows
}

type quoteWave11RowsDriver struct{}

func (quoteWave11RowsDriver) Open(name string) (driver.Conn, error) {
	return quoteWave11RowsConn{dsn: name}, nil
}

type quoteWave11RowsConn struct {
	dsn string
}

func (quoteWave11RowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("quotes wave11 rows do not support Prepare")
}

func (quoteWave11RowsConn) Close() error {
	return nil
}

func (quoteWave11RowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("quotes wave11 rows do not support Begin")
}

func (c quoteWave11RowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	quoteWave11RowsMu.Lock()
	rowSet, ok := quoteWave11RowsByDSN[c.dsn]
	quoteWave11RowsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("quotes wave11 row set %q not found", c.dsn)
	}
	return &quoteWave11Rows{columns: rowSet.columns, values: rowSet.values}, nil
}

type quoteWave11Rows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *quoteWave11Rows) Columns() []string {
	return r.columns
}

func (*quoteWave11Rows) Close() error {
	return nil
}

func (r *quoteWave11Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func quoteDryRunQuote(tenantID string, now time.Time) *Quote {
	validUntil := now.AddDate(0, 0, 30)
	productID := "product-1"
	return &Quote{
		ID:           "quote-1",
		TenantID:     tenantID,
		QuoteNumber:  "Q-00042",
		ContactID:    "contact-1",
		QuoteDate:    now,
		ValidUntil:   &validUntil,
		Status:       QuoteStatusSent,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.RequireFromString("100.00"),
		VATAmount:    decimal.RequireFromString("22.00"),
		Total:        decimal.RequireFromString("122.00"),
		Notes:        "dry-run quote",
		CreatedAt:    now,
		CreatedBy:    "user-1",
		UpdatedAt:    now,
		Lines: []QuoteLine{
			{
				ID:              "quote-line-1",
				TenantID:        tenantID,
				QuoteID:         "quote-1",
				LineNumber:      1,
				Description:     "Consulting",
				Quantity:        decimal.NewFromInt(1),
				Unit:            "hour",
				UnitPrice:       decimal.RequireFromString("100.00"),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.RequireFromString("22.00"),
				LineSubtotal:    decimal.RequireFromString("100.00"),
				LineVAT:         decimal.RequireFromString("22.00"),
				LineTotal:       decimal.RequireFromString("122.00"),
				ProductID:       &productID,
			},
		},
	}
}
