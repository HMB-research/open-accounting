package expenses

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"sync"
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

type expensesDryRunConnPool struct{}

func (expensesDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run expenses tests should not prepare statements")
}

func (expensesDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run expenses tests should not execute statements")
}

func (expensesDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run expenses tests should not query rows")
}

func (expensesDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (expensesDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &expensesDryRunTx{}, nil
}

type expensesDryRunTx struct {
	expensesDryRunConnPool
}

func (*expensesDryRunTx) Commit() error {
	return nil
}

func (*expensesDryRunTx) Rollback() error {
	return nil
}

type expensesDryRunDBOption func(t *testing.T, db *gorm.DB)

type expensesDryRunFixtures struct {
	expense  *models.Expense
	expenses []models.Expense
	seq      *int
}

type expensesDryRunRecorder struct {
	queries         []string
	creates         []string
	updates         []string
	createdExpenses []models.Expense
}

func newExpensesDryRunDB(t *testing.T, opts ...expensesDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: expensesDryRunConnPool{}}), &gorm.Config{
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

func withExpensesDryRunFixtures(fixtures expensesDryRunFixtures, recorder *expensesDryRunRecorder) expensesDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(expensesDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.queries = append(recorder.queries, tx.Statement.SQL.String())
			}
			switch dest := tx.Statement.Dest.(type) {
			case *models.Expense:
				if fixtures.expense != nil {
					*dest = *fixtures.expense
					tx.RowsAffected = 1
				}
			case *[]models.Expense:
				if fixtures.expenses != nil {
					*dest = append([]models.Expense(nil), fixtures.expenses...)
					tx.RowsAffected = int64(len(fixtures.expenses))
				}
			case *int:
				if fixtures.seq != nil {
					*dest = *fixtures.seq
					tx.RowsAffected = 1
				}
			}
		})
		require.NoError(t, err)
	}
}

func withExpensesDryRunQueryErrors(queryErrors ...error) expensesDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(expensesDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
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

func withExpensesDryRunCreateCapture(recorder *expensesDryRunRecorder) expensesDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(expensesDryRunCallbackName(t, "create_capture"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.creates = append(recorder.creates, tx.Statement.SQL.String())
				if dest, ok := tx.Statement.Dest.(*models.Expense); ok {
					recorder.createdExpenses = append(recorder.createdExpenses, *dest)
				}
			}
			if tx.RowsAffected == 0 {
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func withExpensesDryRunCreateErrors(createErrors ...error) expensesDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Create().Before("gorm:create").Register(expensesDryRunCallbackName(t, "create_error"), func(tx *gorm.DB) {
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

func withExpensesDryRunUpdateRows(recorder *expensesDryRunRecorder, rows ...int64) expensesDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(expensesDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
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

func withExpensesDryRunUpdateErrors(updateErrors ...error) expensesDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().Before("gorm:update").Register(expensesDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
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

func expensesDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "expenses_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func assertExpensesRecordedSQLContains(t *testing.T, statements []string, fragments ...string) {
	t.Helper()

	joined := strings.Join(statements, "\n")
	for _, fragment := range fragments {
		assert.Contains(t, joined, fragment)
	}
}

type expensesScanDriver struct{}

type expensesScanConn struct{}

type expensesScanRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

type expensesScanTx struct{}

var registerExpensesScanDriverOnce sync.Once

func (expensesScanDriver) Open(string) (driver.Conn, error) {
	return expensesScanConn{}, nil
}

func (expensesScanConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("dry-run expenses scan tests should not prepare statements")
}

func (expensesScanConn) Close() error {
	return nil
}

func (expensesScanConn) Begin() (driver.Tx, error) {
	return expensesScanTx{}, nil
}

func (expensesScanConn) Ping(context.Context) error {
	return nil
}

func (expensesScanConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &expensesScanRows{
		columns: []string{"seq"},
		values:  [][]driver.Value{{int64(42)}},
	}, nil
}

func (expensesScanConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

func (expensesScanRows) Columns() []string {
	return []string{"seq"}
}

func (r *expensesScanRows) Close() error {
	return nil
}

func (r *expensesScanRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func (expensesScanTx) Commit() error {
	return nil
}

func (expensesScanTx) Rollback() error {
	return nil
}

func newExpensesScanDB(t *testing.T) *gorm.DB {
	t.Helper()

	registerExpensesScanDriverOnce.Do(func() {
		sql.Register("expenses-scan-test", expensesScanDriver{})
	})
	sqlDB, err := sql.Open("expenses-scan-test", "")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, sqlDB.Close()) })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		DisableAutomaticPing:   true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)
	return db
}

func TestGORMRepositoryDryRunExpenseOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_expenses"
	tenantID := "tenant-1"
	expense := expensesDryRunExpense(tenantID)
	expenseModel := expenseToModel(expense)
	seq := 42
	recorder := &expensesDryRunRecorder{}
	repo := NewGORMRepository(newExpensesDryRunDB(t,
		withExpensesDryRunFixtures(expensesDryRunFixtures{
			expense:  expenseModel,
			expenses: []models.Expense{*expenseModel},
			seq:      &seq,
		}, recorder),
		withExpensesDryRunCreateCapture(recorder),
		withExpensesDryRunUpdateRows(recorder, 1),
	))

	require.NoError(t, repo.Create(ctx, schemaName, expense))
	require.Len(t, recorder.createdExpenses, 1)
	assert.Equal(t, expense.ID, recorder.createdExpenses[0].ID)
	assert.True(t, recorder.createdExpenses[0].Amount.Decimal.Equal(expense.Amount))

	got, err := repo.GetByID(ctx, schemaName, tenantID, expense.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, expense.ID, got.ID)
	assert.True(t, got.Amount.Equal(expense.Amount))

	capped, err := repo.List(ctx, schemaName, tenantID, ListExpensesFilter{
		Status: StatusSubmitted,
		Limit:  600,
	})
	require.NoError(t, err)
	require.Len(t, capped, 1)
	assert.Equal(t, expense.ExpenseNumber, capped[0].ExpenseNumber)

	defaultLimit, err := repo.List(ctx, schemaName, tenantID, ListExpensesFilter{})
	require.NoError(t, err)
	require.Len(t, defaultLimit, 1)

	require.NoError(t, repo.Update(ctx, schemaName, expense))

	numberRepo := NewGORMRepository(newExpensesScanDB(t))
	number, err := numberRepo.GenerateNumber(ctx, schemaName, tenantID)
	require.NoError(t, err)
	assert.Equal(t, "EXP-00042", number)

	assertExpensesRecordedSQLContains(t, recorder.creates, `INSERT INTO "tenant_expenses"."expenses"`)
	assertExpensesRecordedSQLContains(t, recorder.queries,
		`FROM "tenant_expenses"."expenses"`,
		`id = $1 AND tenant_id = $2`,
		`status = $2`,
		`ORDER BY expense_date DESC, created_at DESC`,
		`LIMIT`,
	)
	assertExpensesRecordedSQLContains(t, recorder.updates,
		`UPDATE "tenant_expenses"."expenses"`,
		`"journal_entry_id"`,
		`"updated_at"`,
	)
}

func TestGORMRepositoryDryRunExpenseErrors(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_expenses"
	tenantID := "tenant-1"
	expense := expensesDryRunExpense(tenantID)
	expectedErr := errors.New("dry-run failed")

	t.Run("rejects invalid tenant schema", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t))

		err := repo.Create(ctx, "tenant-expenses", expense)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SQL identifier")
	})

	t.Run("wraps create errors", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t, withExpensesDryRunCreateErrors(expectedErr)))

		err := repo.Create(ctx, schemaName, expense)

		require.ErrorContains(t, err, "create expense")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("maps get not found", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t, withExpensesDryRunQueryErrors(gorm.ErrRecordNotFound)))

		got, err := repo.GetByID(ctx, schemaName, tenantID, expense.ID)

		assert.Nil(t, got)
		assert.ErrorIs(t, err, ErrExpenseNotFound)
	})

	t.Run("wraps get query errors", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t, withExpensesDryRunQueryErrors(expectedErr)))

		got, err := repo.GetByID(ctx, schemaName, tenantID, expense.ID)

		assert.Nil(t, got)
		require.ErrorContains(t, err, "get expense")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps list query errors", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t, withExpensesDryRunQueryErrors(expectedErr)))

		got, err := repo.List(ctx, schemaName, tenantID, ListExpensesFilter{Status: StatusApproved})

		assert.Nil(t, got)
		require.ErrorContains(t, err, "list expenses")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t, withExpensesDryRunUpdateErrors(expectedErr)))

		err := repo.Update(ctx, schemaName, expense)

		require.ErrorContains(t, err, "update expense")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("maps update not found", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t, withExpensesDryRunUpdateRows(nil, 0)))

		err := repo.Update(ctx, schemaName, expense)

		assert.ErrorIs(t, err, ErrExpenseNotFound)
	})

	t.Run("wraps generate number errors", func(t *testing.T) {
		repo := NewGORMRepository(newExpensesDryRunDB(t, withExpensesDryRunQueryErrors(expectedErr)))

		number, err := repo.GenerateNumber(ctx, schemaName, tenantID)

		assert.Empty(t, number)
		require.ErrorContains(t, err, "generate expense number")
	})
}

func expensesDryRunExpense(tenantID string) *Expense {
	employeeID := "employee-1"
	contactID := "contact-1"
	journalEntryID := "journal-entry-1"
	submittedAt := time.Date(2026, time.June, 24, 14, 0, 0, 0, time.UTC)
	submittedBy := "submitter-1"
	approvedAt := time.Date(2026, time.June, 24, 15, 0, 0, 0, time.UTC)
	approvedBy := "approver-1"
	createdAt := time.Date(2026, time.June, 24, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, time.June, 24, 15, 30, 0, 0, time.UTC)
	return &Expense{
		ID:               "expense-1",
		TenantID:         tenantID,
		ExpenseNumber:    "EXP-00041",
		ExpenseDate:      time.Date(2026, time.June, 23, 0, 0, 0, 0, time.UTC),
		Merchant:         "Office Supplies OU",
		Description:      "Printer paper",
		EmployeeID:       &employeeID,
		ContactID:        &contactID,
		ExpenseAccountID: "account-expense-1",
		PaymentAccountID: "account-payment-1",
		Amount:           decimal.RequireFromString("42.50"),
		Currency:         "EUR",
		ExchangeRate:     decimal.NewFromInt(1),
		BaseAmount:       decimal.RequireFromString("42.50"),
		RequiresReceipt:  true,
		Status:           StatusSubmitted,
		JournalEntryID:   &journalEntryID,
		SubmittedAt:      &submittedAt,
		SubmittedBy:      &submittedBy,
		ApprovedAt:       &approvedAt,
		ApprovedBy:       &approvedBy,
		CreatedAt:        createdAt,
		CreatedBy:        "creator-1",
		UpdatedAt:        updatedAt,
	}
}
