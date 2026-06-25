package payments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

type paymentsDryRunConnPool struct{}

func (paymentsDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run payments tests should not prepare statements")
}

func (paymentsDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run payments tests should not execute statements")
}

func (paymentsDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run payments tests should not query rows")
}

func (paymentsDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (paymentsDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &paymentsDryRunTx{}, nil
}

type paymentsDryRunTx struct {
	paymentsDryRunConnPool
}

func (*paymentsDryRunTx) Commit() error {
	return nil
}

func (*paymentsDryRunTx) Rollback() error {
	return nil
}

type paymentsDryRunDBOption func(t *testing.T, db *gorm.DB)

type paymentsDryRunFixtures struct {
	payment        *models.Payment
	payments       []models.Payment
	allocations    []models.PaymentAllocation
	paymentNumbers []string
}

type paymentsDryRunRecorder struct {
	queries            []string
	rows               []string
	creates            []string
	updates            []string
	createdPayments    []models.Payment
	createdAllocations []models.PaymentAllocation
}

var paymentsDryRunCallbackID uint64

func newPaymentsDryRunDB(t *testing.T, opts ...paymentsDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: paymentsDryRunConnPool{}}), &gorm.Config{
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

func withPaymentsDryRunFixtures(fixtures paymentsDryRunFixtures, recorder *paymentsDryRunRecorder) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(paymentsDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.queries = append(recorder.queries, tx.Statement.SQL.String())
			}
			populatePaymentsDryRunQueryDest(tx, tx.Statement.Dest, &fixtures)
		})
		require.NoError(t, err)
	}
}

func withPaymentsDryRunQueryErrors(queryErrors ...error) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Query().Before("gorm:query").Register(paymentsDryRunCallbackName("query_error"), func(tx *gorm.DB) {
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

func withPaymentsDryRunRowRecorder(recorder *paymentsDryRunRecorder) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().After("gorm:row").Register(paymentsDryRunCallbackName("row_recorder"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.rows = append(recorder.rows, tx.Statement.SQL.String())
			}
		})
		require.NoError(t, err)
	}
}

func withPaymentsDryRunRowErrors(rowErrors ...error) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(paymentsDryRunCallbackName("row_error"), func(tx *gorm.DB) {
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

func withPaymentsDryRunCreateCapture(recorder *paymentsDryRunRecorder) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(paymentsDryRunCallbackName("create_capture"), func(tx *gorm.DB) {
			if recorder != nil {
				recorder.creates = append(recorder.creates, tx.Statement.SQL.String())
				switch dest := tx.Statement.Dest.(type) {
				case *models.Payment:
					recorder.createdPayments = append(recorder.createdPayments, *dest)
				case *models.PaymentAllocation:
					recorder.createdAllocations = append(recorder.createdAllocations, *dest)
				}
			}
			if tx.RowsAffected == 0 {
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func withPaymentsDryRunCreateErrors(createErrors ...error) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Create().Before("gorm:create").Register(paymentsDryRunCallbackName("create_error"), func(tx *gorm.DB) {
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

func withPaymentsDryRunUpdateRows(recorder *paymentsDryRunRecorder, rows ...int64) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(paymentsDryRunCallbackName("update_rows"), func(tx *gorm.DB) {
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

func withPaymentsDryRunUpdateError(expectedErr error) paymentsDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(paymentsDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func paymentsDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&paymentsDryRunCallbackID, 1)
	return fmt.Sprintf("payments_dryrun:%d:%s", id, suffix)
}

func populatePaymentsDryRunQueryDest(tx *gorm.DB, dest any, fixtures *paymentsDryRunFixtures) {
	switch typed := dest.(type) {
	case *models.Payment:
		if fixtures.payment != nil {
			*typed = *fixtures.payment
			tx.RowsAffected = 1
		}
	case *[]models.Payment:
		*typed = append([]models.Payment(nil), fixtures.payments...)
		tx.RowsAffected = int64(len(fixtures.payments))
	case *[]models.PaymentAllocation:
		*typed = append([]models.PaymentAllocation(nil), fixtures.allocations...)
		tx.RowsAffected = int64(len(fixtures.allocations))
	case *[]string:
		*typed = append([]string(nil), fixtures.paymentNumbers...)
		tx.RowsAffected = int64(len(fixtures.paymentNumbers))
	}
}

func TestGORMRepositoryDryRunCreateOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payments"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	payment := paymentsDryRunPayment("payment-1", "tenant-1", now)
	allocation := &PaymentAllocation{
		ID:        "allocation-1",
		TenantID:  payment.TenantID,
		PaymentID: payment.ID,
		InvoiceID: "invoice-1",
		Amount:    decimal.NewFromInt(25),
		CreatedAt: now,
	}

	t.Run("captures create models", func(t *testing.T) {
		recorder := &paymentsDryRunRecorder{}
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunCreateCapture(recorder)))

		require.NoError(t, repo.Create(ctx, schemaName, payment))
		require.NoError(t, repo.CreateAllocation(ctx, schemaName, allocation))

		require.Len(t, recorder.createdPayments, 1)
		assert.Equal(t, payment.ID, recorder.createdPayments[0].ID)
		assert.True(t, recorder.createdPayments[0].Amount.Decimal.Equal(payment.Amount))
		require.Len(t, recorder.createdAllocations, 1)
		assert.Equal(t, allocation.ID, recorder.createdAllocations[0].ID)
		assert.True(t, recorder.createdAllocations[0].Amount.Decimal.Equal(allocation.Amount))
		assertPaymentsRecordedSQLContains(t, recorder.creates,
			`INSERT INTO "tenant_payments"."payments"`,
			`INSERT INTO "tenant_payments"."payment_allocations"`,
		)
	})

	t.Run("wraps create payment errors", func(t *testing.T) {
		expectedErr := errors.New("create payment failed")
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunCreateErrors(expectedErr)))

		err := repo.Create(ctx, schemaName, payment)

		require.ErrorContains(t, err, "create payment")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps create allocation errors", func(t *testing.T) {
		expectedErr := errors.New("create allocation failed")
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunCreateErrors(expectedErr)))

		err := repo.CreateAllocation(ctx, schemaName, allocation)

		require.ErrorContains(t, err, "create allocation")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunReadQueriesMapRowsAndBuildSQL(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payments"
	tenantID := "tenant-1"
	contactID := "contact-1"
	now := time.Date(2026, time.June, 25, 11, 0, 0, 0, time.UTC)
	paymentModel := paymentsDryRunPaymentModel("payment-1", tenantID, now)
	paymentModel.ContactID = &contactID
	allocationModel := models.PaymentAllocation{
		ID:        "allocation-1",
		TenantID:  tenantID,
		PaymentID: paymentModel.ID,
		InvoiceID: "invoice-1",
		Amount:    models.Decimal{Decimal: decimal.NewFromInt(30)},
		CreatedAt: now,
	}
	recorder := &paymentsDryRunRecorder{}
	repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunFixtures(paymentsDryRunFixtures{
		payment:        &paymentModel,
		payments:       []models.Payment{paymentModel},
		allocations:    []models.PaymentAllocation{allocationModel},
		paymentNumbers: []string{"PMT-00002", "PMT-00005"},
	}, recorder)))

	gotPayment, err := repo.GetByID(ctx, schemaName, tenantID, paymentModel.ID)
	require.NoError(t, err)
	require.NotNil(t, gotPayment)
	assert.Equal(t, paymentModel.ID, gotPayment.ID)
	assert.True(t, gotPayment.Amount.Equal(paymentModel.Amount.Decimal))

	fromDate := now.AddDate(0, 0, -7)
	toDate := now
	payments, err := repo.List(ctx, schemaName, tenantID, &PaymentFilter{
		PaymentType:   PaymentTypeReceived,
		PaymentMethod: "BANK",
		ContactID:     contactID,
		FromDate:      &fromDate,
		ToDate:        &toDate,
	})
	require.NoError(t, err)
	require.Len(t, payments, 1)
	assert.Equal(t, paymentModel.ID, payments[0].ID)

	allocations, err := repo.GetAllocations(ctx, schemaName, tenantID, paymentModel.ID)
	require.NoError(t, err)
	require.Len(t, allocations, 1)
	assert.Equal(t, allocationModel.ID, allocations[0].ID)

	next, err := repo.GetNextPaymentNumber(ctx, schemaName, tenantID, PaymentTypeReceived)
	require.NoError(t, err)
	assert.Equal(t, 6, next)

	assertPaymentsRecordedSQLContains(t, recorder.queries,
		`FROM "tenant_payments"."payments"`,
		`id = $1 AND tenant_id = $2`,
		`payment_type = $2`,
		`payment_method = $3`,
		`contact_id = $4`,
		`payment_date >= $5`,
		`payment_date <= $6`,
		`ORDER BY payment_date DESC, payment_number DESC`,
		`FROM "tenant_payments"."payment_allocations"`,
		`payment_id = $1 AND tenant_id = $2`,
		`payment_number LIKE $3`,
	)
}

func TestGORMRepositoryDryRunCreateReversalTransaction(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payments"
	tenantID := "tenant-1"
	originalPaymentID := "payment-original"
	reversedAt := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	reversalID := "payment-reversal"
	reversal := paymentsDryRunPayment(reversalID, tenantID, reversedAt)
	reversal.ReversalOfPaymentID = &originalPaymentID
	allocation := PaymentAllocation{
		ID:        "allocation-reversal",
		TenantID:  tenantID,
		PaymentID: reversalID,
		InvoiceID: "invoice-1",
		Amount:    decimal.NewFromInt(40),
		CreatedAt: reversedAt,
	}

	t.Run("creates reversal allocation and marks original", func(t *testing.T) {
		recorder := &paymentsDryRunRecorder{}
		repo := NewGORMRepository(newPaymentsDryRunDB(t,
			withPaymentsDryRunCreateCapture(recorder),
			withPaymentsDryRunUpdateRows(recorder, 1),
		))

		err := repo.CreateReversal(ctx, schemaName, originalPaymentID, reversal, []PaymentAllocation{allocation}, reversedAt, "user-1", "Duplicate import")

		require.NoError(t, err)
		require.Len(t, recorder.createdPayments, 1)
		assert.Equal(t, reversalID, recorder.createdPayments[0].ID)
		require.Len(t, recorder.createdAllocations, 1)
		assert.Equal(t, allocation.ID, recorder.createdAllocations[0].ID)
		assertPaymentsRecordedSQLContains(t, recorder.updates,
			`UPDATE "tenant_payments"."payments"`,
			`reversed_by_payment_id`,
			`reversal_reason`,
			`id = $`,
			`reversed_by_payment_id IS NULL`,
		)
	})

	t.Run("returns already reversed when original update affects no rows", func(t *testing.T) {
		repo := NewGORMRepository(newPaymentsDryRunDB(t,
			withPaymentsDryRunCreateCapture(nil),
			withPaymentsDryRunUpdateRows(nil, 0),
		))

		err := repo.CreateReversal(ctx, schemaName, originalPaymentID, reversal, []PaymentAllocation{allocation}, reversedAt, "user-1", "Duplicate import")

		assert.ErrorIs(t, err, ErrPaymentAlreadyReversed)
	})

	t.Run("wraps allocation create errors", func(t *testing.T) {
		expectedErr := errors.New("allocation insert failed")
		repo := NewGORMRepository(newPaymentsDryRunDB(t,
			withPaymentsDryRunCreateCapture(nil),
			withPaymentsDryRunCreateErrors(nil, expectedErr),
		))

		err := repo.CreateReversal(ctx, schemaName, originalPaymentID, reversal, []PaymentAllocation{allocation}, reversedAt, "user-1", "Duplicate import")

		require.ErrorContains(t, err, "create reversal allocation")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("wraps original update errors", func(t *testing.T) {
		expectedErr := errors.New("update failed")
		repo := NewGORMRepository(newPaymentsDryRunDB(t,
			withPaymentsDryRunCreateCapture(nil),
			withPaymentsDryRunUpdateError(expectedErr),
		))

		err := repo.CreateReversal(ctx, schemaName, originalPaymentID, reversal, []PaymentAllocation{allocation}, reversedAt, "user-1", "Duplicate import")

		require.ErrorContains(t, err, "mark original payment reversed")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunErrorPaths(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_payments"
	tenantID := "tenant-1"
	paymentID := "payment-1"
	expectedErr := errors.New("dry-run query failed")

	t.Run("get by id not found", func(t *testing.T) {
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunQueryErrors(gorm.ErrRecordNotFound)))

		payment, err := repo.GetByID(ctx, schemaName, tenantID, paymentID)

		require.Nil(t, payment)
		assert.ErrorIs(t, err, ErrPaymentNotFound)
	})

	t.Run("get by id query error", func(t *testing.T) {
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunQueryErrors(expectedErr)))

		payment, err := repo.GetByID(ctx, schemaName, tenantID, paymentID)

		require.Nil(t, payment)
		require.ErrorContains(t, err, "get payment")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("list query error", func(t *testing.T) {
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunQueryErrors(expectedErr)))

		payments, err := repo.List(ctx, schemaName, tenantID, nil)

		require.Nil(t, payments)
		require.ErrorContains(t, err, "list payments")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("get allocations query error", func(t *testing.T) {
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunQueryErrors(expectedErr)))

		allocations, err := repo.GetAllocations(ctx, schemaName, tenantID, paymentID)

		require.Nil(t, allocations)
		require.ErrorContains(t, err, "get allocations")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("get next payment number query error", func(t *testing.T) {
		repo := NewGORMRepository(newPaymentsDryRunDB(t, withPaymentsDryRunQueryErrors(expectedErr)))

		next, err := repo.GetNextPaymentNumber(ctx, schemaName, tenantID, PaymentTypeReceived)

		assert.Zero(t, next)
		require.ErrorContains(t, err, "get next payment number")
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("get unallocated payments scan error records SQL", func(t *testing.T) {
		rowErr := errors.New("dry-run scan failed")
		recorder := &paymentsDryRunRecorder{}
		repo := NewGORMRepository(newPaymentsDryRunDB(t,
			withPaymentsDryRunRowRecorder(recorder),
			withPaymentsDryRunRowErrors(rowErr),
		))

		payments, err := repo.GetUnallocatedPayments(ctx, schemaName, tenantID, PaymentTypeReceived)

		require.Nil(t, payments)
		require.ErrorContains(t, err, "get unallocated payments")
		assert.ErrorIs(t, err, rowErr)
		assertPaymentsRecordedSQLContains(t, recorder.rows,
			`FROM "tenant_payments"."payments" AS p`,
			`FROM "tenant_payments"."payment_allocations" AS pa`,
			`p.amount > COALESCE`,
			`ORDER BY p.payment_date`,
		)
	})
}

func TestGORMRepositoryDryRunRejectsInvalidTenantSchema(t *testing.T) {
	ctx := context.Background()
	repo := NewGORMRepository(newPaymentsDryRunDB(t))
	now := time.Date(2026, time.June, 25, 13, 0, 0, 0, time.UTC)
	payment := paymentsDryRunPayment("payment-1", "tenant-1", now)
	allocation := &PaymentAllocation{
		ID:        "allocation-1",
		TenantID:  payment.TenantID,
		PaymentID: payment.ID,
		InvoiceID: "invoice-1",
		Amount:    decimal.NewFromInt(10),
		CreatedAt: now,
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "Create",
			run: func() error {
				return repo.Create(ctx, "tenant-payments", payment)
			},
		},
		{
			name: "CreateReversal",
			run: func() error {
				return repo.CreateReversal(ctx, "tenant-payments", "payment-original", payment, []PaymentAllocation{*allocation}, now, "user-1", "Duplicate import")
			},
		},
		{
			name: "GetByID",
			run: func() error {
				_, err := repo.GetByID(ctx, "tenant-payments", payment.TenantID, payment.ID)
				return err
			},
		},
		{
			name: "List",
			run: func() error {
				_, err := repo.List(ctx, "tenant-payments", payment.TenantID, nil)
				return err
			},
		},
		{
			name: "CreateAllocation",
			run: func() error {
				return repo.CreateAllocation(ctx, "tenant-payments", allocation)
			},
		},
		{
			name: "GetAllocations",
			run: func() error {
				_, err := repo.GetAllocations(ctx, "tenant-payments", payment.TenantID, payment.ID)
				return err
			},
		},
		{
			name: "GetNextPaymentNumber",
			run: func() error {
				_, err := repo.GetNextPaymentNumber(ctx, "tenant-payments", payment.TenantID, PaymentTypeReceived)
				return err
			},
		},
		{
			name: "GetUnallocatedPayments",
			run: func() error {
				_, err := repo.GetUnallocatedPayments(ctx, "tenant-payments", payment.TenantID, PaymentTypeReceived)
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

func paymentsDryRunPayment(id, tenantID string, now time.Time) *Payment {
	return &Payment{
		ID:            id,
		TenantID:      tenantID,
		PaymentNumber: "PMT-00001",
		PaymentType:   PaymentTypeReceived,
		PaymentDate:   now,
		Amount:        decimal.NewFromInt(100),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.NewFromInt(100),
		PaymentMethod: "BANK",
		Reference:     "INV-1",
		CreatedAt:     now,
		CreatedBy:     "user-1",
	}
}

func paymentsDryRunPaymentModel(id, tenantID string, now time.Time) models.Payment {
	return models.Payment{
		ID:            id,
		TenantID:      tenantID,
		PaymentNumber: "PMT-00001",
		PaymentType:   models.PaymentTypeReceived,
		PaymentDate:   now,
		Amount:        models.Decimal{Decimal: decimal.NewFromInt(100)},
		Currency:      "EUR",
		ExchangeRate:  models.Decimal{Decimal: decimal.NewFromInt(1)},
		BaseAmount:    models.Decimal{Decimal: decimal.NewFromInt(100)},
		PaymentMethod: "BANK",
		Reference:     "INV-1",
		CreatedAt:     now,
		CreatedBy:     "user-1",
	}
}

func assertPaymentsRecordedSQLContains(t *testing.T, sqlStatements []string, fragments ...string) {
	t.Helper()

	allSQL := strings.Join(sqlStatements, "\n")
	require.NotEmpty(t, allSQL)
	for _, fragment := range fragments {
		assert.Contains(t, allSQL, fragment)
	}
}
