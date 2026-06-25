package banking

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
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type bankingDryRunConnPool struct{}

func (bankingDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run banking tests should not prepare statements")
}

func (bankingDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run banking tests should not execute statements")
}

func (bankingDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run banking tests should not query rows")
}

func (bankingDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (bankingDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &bankingDryRunTx{}, nil
}

type bankingDryRunTx struct {
	bankingDryRunConnPool
}

func (*bankingDryRunTx) Commit() error {
	return nil
}

func (*bankingDryRunTx) Rollback() error {
	return nil
}

type bankingDryRunDBOption func(t *testing.T, db *gorm.DB)

type bankingDryRunFixtures struct {
	bankAccount     *models.BankAccount
	bankAccounts    []models.BankAccount
	bankMatchRule   *BankMatchRule
	bankMatchRules  []BankMatchRule
	transactions    []models.BankTransaction
	transaction     *models.BankTransaction
	reconciliation  *models.BankReconciliation
	reconciliations []models.BankReconciliation
	importRecord    *models.BankStatementImport
	imports         []models.BankStatementImport
	counts          []int64
	paymentNumbers  []string
}

type bankingDryRunRowSet struct {
	columns []string
	values  [][]driver.Value
}

var bankingDryRunRowsDSNID uint64
var bankingDryRunRowsDriverOnce sync.Once
var bankingDryRunRowsMu sync.Mutex
var bankingDryRunRowsByDSN = map[string]bankingDryRunRowSet{}

func newBankingDryRunDB(t *testing.T, opts ...bankingDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: bankingDryRunConnPool{}}), &gorm.Config{
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

func withBankingDryRunFixtures(fixtures bankingDryRunFixtures) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var countIndex int
		err := db.Callback().Query().After("gorm:query").Register(bankingDryRunCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *models.BankAccount:
				if fixtures.bankAccount != nil {
					*dest = *fixtures.bankAccount
					tx.RowsAffected = 1
				}
			case *[]models.BankAccount:
				*dest = append([]models.BankAccount(nil), fixtures.bankAccounts...)
				tx.RowsAffected = int64(len(fixtures.bankAccounts))
			case *BankMatchRule:
				if fixtures.bankMatchRule != nil {
					*dest = *fixtures.bankMatchRule
					tx.RowsAffected = 1
				}
			case *[]BankMatchRule:
				*dest = append([]BankMatchRule(nil), fixtures.bankMatchRules...)
				tx.RowsAffected = int64(len(fixtures.bankMatchRules))
			case *[]models.BankTransaction:
				*dest = append([]models.BankTransaction(nil), fixtures.transactions...)
				tx.RowsAffected = int64(len(fixtures.transactions))
			case *models.BankTransaction:
				if fixtures.transaction != nil {
					*dest = *fixtures.transaction
					tx.RowsAffected = 1
				}
			case *models.BankReconciliation:
				if fixtures.reconciliation != nil {
					*dest = *fixtures.reconciliation
					tx.RowsAffected = 1
				}
			case *[]models.BankReconciliation:
				*dest = append([]models.BankReconciliation(nil), fixtures.reconciliations...)
				tx.RowsAffected = int64(len(fixtures.reconciliations))
			case *models.BankStatementImport:
				if fixtures.importRecord != nil {
					*dest = *fixtures.importRecord
					tx.RowsAffected = 1
				}
			case *[]models.BankStatementImport:
				*dest = append([]models.BankStatementImport(nil), fixtures.imports...)
				tx.RowsAffected = int64(len(fixtures.imports))
			case *int64:
				count := int64(0)
				if len(fixtures.counts) > 0 {
					count = fixtures.counts[len(fixtures.counts)-1]
					if countIndex < len(fixtures.counts) {
						count = fixtures.counts[countIndex]
					}
					countIndex++
				}
				*dest = count
				tx.RowsAffected = 1
			case *[]string:
				*dest = append([]string(nil), fixtures.paymentNumbers...)
				tx.RowsAffected = int64(len(fixtures.paymentNumbers))
			}
		})
		require.NoError(t, err)
	}
}

func withBankingDryRunQueryError(expectedErr error) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(bankingDryRunCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withBankingDryRunRowError(expectedErr error) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().Before("gorm:row").Register(bankingDryRunCallbackName(t, "row_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withBankingDryRunScanRows(rowSets ...bankingDryRunRowSet) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(bankingDryRunCallbackName(t, "scan_rows"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing banking dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newBankingDryRunSQLRows(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		require.NoError(t, err)
	}
}

func withBankingDryRunUpdateRows(rows ...int64) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(bankingDryRunCallbackName(t, "update_rows"), func(tx *gorm.DB) {
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

func withBankingDryRunUpdateError(expectedErr error) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(bankingDryRunCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withBankingDryRunDeleteRows(rows ...int64) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Delete().After("gorm:delete").Register(bankingDryRunCallbackName(t, "delete_rows"), func(tx *gorm.DB) {
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

func withBankingDryRunCreateCapture(capture func(*gorm.DB)) bankingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().After("gorm:create").Register(bankingDryRunCallbackName(t, "create_capture"), func(tx *gorm.DB) {
			if capture != nil {
				capture(tx)
			}
			if tx.RowsAffected == 0 {
				tx.RowsAffected = 1
			}
		})
		require.NoError(t, err)
	}
}

func bankingDryRunCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "banking_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func newBankingDryRunSQLRows(t *testing.T, rowSet bankingDryRunRowSet) *sql.Rows {
	t.Helper()

	bankingDryRunRowsDriverOnce.Do(func() {
		sql.Register("banking_dryrun_rows", bankingDryRunRowsDriver{})
	})

	dsn := fmt.Sprintf("banking-dry-run-rows-%d", atomic.AddUint64(&bankingDryRunRowsDSNID, 1))
	bankingDryRunRowsMu.Lock()
	bankingDryRunRowsByDSN[dsn] = rowSet
	bankingDryRunRowsMu.Unlock()

	db, err := sql.Open("banking_dryrun_rows", dsn)
	require.NoError(t, err)
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		bankingDryRunRowsMu.Lock()
		delete(bankingDryRunRowsByDSN, dsn)
		bankingDryRunRowsMu.Unlock()
	})

	return rows
}

type bankingDryRunRowsDriver struct{}

func (bankingDryRunRowsDriver) Open(name string) (driver.Conn, error) {
	return bankingDryRunRowsConn{dsn: name}, nil
}

type bankingDryRunRowsConn struct {
	dsn string
}

func (bankingDryRunRowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("banking dry-run rows do not prepare statements")
}

func (bankingDryRunRowsConn) Close() error {
	return nil
}

func (bankingDryRunRowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("banking dry-run rows do not begin transactions")
}

func (c bankingDryRunRowsConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	bankingDryRunRowsMu.Lock()
	rowSet, ok := bankingDryRunRowsByDSN[c.dsn]
	bankingDryRunRowsMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("banking dry-run row set %q not found", c.dsn)
	}
	return &bankingDryRunSQLRows{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type bankingDryRunSQLRows struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *bankingDryRunSQLRows) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*bankingDryRunSQLRows) Close() error {
	return nil
}

func (r *bankingDryRunSQLRows) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}

func TestNewServiceWithGORMUsesRepository(t *testing.T) {
	db := newBankingDryRunDB(t)

	service := NewServiceWithGORM(db)

	require.NotNil(t, service)
	repo, ok := service.repo.(*GORMRepository)
	require.True(t, ok)
	assert.Same(t, db, repo.db)
	assert.Nil(t, service.accounts)
}

func TestGORMRepositoryDryRunListOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "11111111-1111-1111-1111-111111111111"
	accountID := "22222222-2222-2222-2222-222222222222"
	transactionID := "33333333-3333-3333-3333-333333333333"
	now := time.Date(2026, time.June, 25, 10, 0, 0, 0, time.UTC)
	minAmount := decimal.NewFromInt(50)
	maxAmount := decimal.NewFromInt(150)
	ruleAccountID := accountID
	active := true
	repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunFixtures(bankingDryRunFixtures{
		bankAccounts: []models.BankAccount{
			{
				ID:            accountID,
				TenantID:      tenantID,
				Name:          "Operating account",
				AccountNumber: "EE471000001020145685",
				BankName:      "Demo Bank",
				SwiftCode:     "DEMOEE2X",
				Currency:      "EUR",
				IsDefault:     true,
				IsActive:      true,
				CreatedAt:     now,
			},
		},
		bankMatchRules: []BankMatchRule{
			{
				ID:            "rule-1",
				TenantID:      tenantID,
				BankAccountID: &ruleAccountID,
				Name:          "Stripe payouts",
				Priority:      10,
				MatchField:    BankMatchFieldDescription,
				Pattern:       "stripe",
				MinConfidence: 0.8,
				IsActive:      true,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
		transactions: []models.BankTransaction{
			{
				ID:              transactionID,
				TenantID:        tenantID,
				BankAccountID:   accountID,
				TransactionDate: now,
				Amount:          models.Decimal{Decimal: decimal.NewFromInt(125)},
				Currency:        "EUR",
				Description:     "Invoice payment",
				Reference:       "INV-100",
				Status:          models.TransactionStatusUnmatched,
				FollowUpStatus:  models.TransactionFollowUpReadyToMatch,
				ImportedAt:      now,
			},
		},
	})))

	accounts, err := repo.ListBankAccounts(ctx, schemaName, tenantID, &BankAccountFilter{
		IsActive: &active,
		Currency: "EUR",
	})
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, accountID, accounts[0].ID)
	assert.True(t, accounts[0].IsDefault)

	rules, err := repo.ListBankMatchRules(ctx, schemaName, tenantID, &BankMatchRuleFilter{
		BankAccountID: accountID,
		ActiveOnly:    true,
		IncludeGlobal: true,
	})
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, "Stripe payouts", rules[0].Name)

	transactions, err := repo.ListTransactions(ctx, schemaName, tenantID, &TransactionFilter{
		BankAccountID: accountID,
		Status:        StatusUnmatched,
		FromDate:      &now,
		ToDate:        &now,
		MinAmount:     &minAmount,
		MaxAmount:     &maxAmount,
	})
	require.NoError(t, err)
	require.Len(t, transactions, 1)
	assert.Equal(t, transactionID, transactions[0].ID)
	assert.True(t, transactions[0].Amount.Equal(decimal.NewFromInt(125)))
	assert.Equal(t, FollowUpReadyToMatch, transactions[0].FollowUpStatus)
}

func TestGORMRepositoryDryRunListBankMatchRulesReturnsEmptySlice(t *testing.T) {
	repo := NewGORMRepository(newBankingDryRunDB(t))

	rules, err := repo.ListBankMatchRules(context.Background(), "tenant_banking", "tenant-1", nil)

	require.NoError(t, err)
	assert.NotNil(t, rules)
	assert.Empty(t, rules)
}

func TestGORMRepositoryDryRunBasicRepositoryOperations(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "11111111-1111-1111-1111-111111111111"
	accountID := "22222222-2222-2222-2222-222222222222"
	transactionID := "33333333-3333-3333-3333-333333333333"
	reconciliationID := "44444444-4444-4444-4444-444444444444"
	importID := "55555555-5555-5555-5555-555555555555"
	ruleID := "66666666-6666-6666-6666-666666666666"
	now := time.Date(2026, time.June, 25, 11, 0, 0, 0, time.UTC)
	repo := NewGORMRepository(newBankingDryRunDB(t,
		withBankingDryRunFixtures(bankingDryRunFixtures{
			bankAccount: &models.BankAccount{
				ID:            accountID,
				TenantID:      tenantID,
				Name:          "Operating account",
				AccountNumber: "EE471000001020145685",
				Currency:      "EUR",
				IsActive:      true,
				CreatedAt:     now,
			},
			bankMatchRule: &BankMatchRule{
				ID:            ruleID,
				TenantID:      tenantID,
				Name:          "Reference match",
				MatchField:    BankMatchFieldReference,
				Pattern:       "INV",
				MinConfidence: 0.75,
				IsActive:      true,
				CreatedAt:     now,
				UpdatedAt:     now,
			},
			reconciliation: &models.BankReconciliation{
				ID:             reconciliationID,
				TenantID:       tenantID,
				BankAccountID:  accountID,
				StatementDate:  now,
				OpeningBalance: models.Decimal{Decimal: decimal.NewFromInt(1000)},
				ClosingBalance: models.Decimal{Decimal: decimal.NewFromInt(1125)},
				Status:         models.ReconciliationInProgress,
				CreatedAt:      now,
				CreatedBy:      "77777777-7777-7777-7777-777777777777",
			},
			reconciliations: []models.BankReconciliation{
				{
					ID:             reconciliationID,
					TenantID:       tenantID,
					BankAccountID:  accountID,
					StatementDate:  now,
					OpeningBalance: models.Decimal{Decimal: decimal.NewFromInt(1000)},
					ClosingBalance: models.Decimal{Decimal: decimal.NewFromInt(1125)},
					Status:         models.ReconciliationInProgress,
					CreatedAt:      now,
					CreatedBy:      "77777777-7777-7777-7777-777777777777",
				},
			},
			importRecord: &models.BankStatementImport{
				ID:                  importID,
				TenantID:            tenantID,
				BankAccountID:       accountID,
				FileName:            "statement.csv",
				TransactionsMatched: 2,
				CreatedAt:           now,
			},
			imports: []models.BankStatementImport{
				{
					ID:                  importID,
					TenantID:            tenantID,
					BankAccountID:       accountID,
					FileName:            "statement.csv",
					TransactionsMatched: 2,
					CreatedAt:           now,
				},
			},
			counts: []int64{2},
		}),
		withBankingDryRunUpdateRows(1),
		withBankingDryRunDeleteRows(1),
	))
	account := &BankAccount{
		ID:            accountID,
		TenantID:      tenantID,
		Name:          "Operating account",
		AccountNumber: "EE471000001020145685",
		Currency:      "EUR",
		IsActive:      true,
		CreatedAt:     now,
	}
	rule := &BankMatchRule{
		ID:            ruleID,
		TenantID:      tenantID,
		Name:          "Reference match",
		MatchField:    BankMatchFieldReference,
		Pattern:       "INV",
		MinConfidence: 0.75,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	transaction := &BankTransaction{
		ID:              transactionID,
		TenantID:        tenantID,
		BankAccountID:   accountID,
		TransactionDate: now,
		Amount:          decimal.NewFromInt(125),
		Currency:        "EUR",
		Status:          StatusUnmatched,
		ImportedAt:      now,
	}
	reconciliation := &BankReconciliation{
		ID:             reconciliationID,
		TenantID:       tenantID,
		BankAccountID:  accountID,
		StatementDate:  now,
		OpeningBalance: decimal.NewFromInt(1000),
		ClosingBalance: decimal.NewFromInt(1125),
		Status:         ReconciliationInProgress,
		CreatedAt:      now,
		CreatedBy:      "77777777-7777-7777-7777-777777777777",
	}
	importRecord := &BankStatementImport{
		ID:                  importID,
		TenantID:            tenantID,
		BankAccountID:       accountID,
		FileName:            "statement.csv",
		TransactionsMatched: 2,
		CreatedAt:           now,
	}

	require.NoError(t, repo.CreateBankAccount(ctx, schemaName, account))
	gotAccount, err := repo.GetBankAccount(ctx, schemaName, tenantID, accountID)
	require.NoError(t, err)
	assert.Equal(t, accountID, gotAccount.ID)
	require.NoError(t, repo.UpdateBankAccount(ctx, schemaName, account))
	require.NoError(t, repo.UnsetDefaultAccounts(ctx, schemaName, tenantID))
	transactionCount, err := repo.CountTransactionsForAccount(ctx, schemaName, accountID)
	require.NoError(t, err)
	assert.Equal(t, 2, transactionCount)
	require.NoError(t, repo.DeleteBankAccount(ctx, schemaName, tenantID, accountID))

	require.NoError(t, repo.CreateBankMatchRule(ctx, schemaName, rule))
	gotRule, err := repo.GetBankMatchRule(ctx, schemaName, tenantID, ruleID)
	require.NoError(t, err)
	assert.Equal(t, ruleID, gotRule.ID)
	require.NoError(t, repo.UpdateBankMatchRule(ctx, schemaName, rule))
	require.NoError(t, repo.DeleteBankMatchRule(ctx, schemaName, tenantID, ruleID))

	require.NoError(t, repo.CreateTransaction(ctx, schemaName, transaction))
	require.NoError(t, repo.MatchTransaction(ctx, schemaName, tenantID, transactionID, "88888888-8888-8888-8888-888888888888"))
	require.NoError(t, repo.UnmatchTransaction(ctx, schemaName, tenantID, transactionID))

	require.NoError(t, repo.CreateReconciliation(ctx, schemaName, reconciliation))
	gotReconciliation, err := repo.GetReconciliation(ctx, schemaName, tenantID, reconciliationID)
	require.NoError(t, err)
	assert.Equal(t, reconciliationID, gotReconciliation.ID)
	reconciliations, err := repo.ListReconciliations(ctx, schemaName, tenantID, accountID)
	require.NoError(t, err)
	require.Len(t, reconciliations, 1)
	assert.Equal(t, reconciliationID, reconciliations[0].ID)
	require.NoError(t, repo.AddTransactionToReconciliation(ctx, schemaName, tenantID, transactionID, reconciliationID))

	require.NoError(t, repo.CreateImportRecord(ctx, schemaName, importRecord))
	require.NoError(t, repo.IncrementLatestImportMatchedCount(ctx, schemaName, tenantID, accountID, 1))
	imports, err := repo.GetImportHistory(ctx, schemaName, tenantID, accountID)
	require.NoError(t, err)
	require.Len(t, imports, 1)
	assert.Equal(t, importID, imports[0].ID)
}

func TestGORMRepositoryDryRunCalculateAccountBalance(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	accountID := "22222222-2222-2222-2222-222222222222"

	t.Run("scans decimal sum", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunScanRows(bankingDryRunRowSet{
			columns: []string{"balance"},
			values:  [][]driver.Value{{"987.65"}},
		})))

		balance, err := repo.CalculateAccountBalance(ctx, schemaName, accountID)

		require.NoError(t, err)
		assert.True(t, balance.Equal(decimal.RequireFromString("987.65")))
	})

	t.Run("wraps scan errors", func(t *testing.T) {
		expectedErr := errors.New("balance scan failed")
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunRowError(expectedErr)))

		balance, err := repo.CalculateAccountBalance(ctx, schemaName, accountID)

		assert.True(t, balance.Equal(decimal.Zero))
		require.ErrorContains(t, err, "calculate balance")
		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunUpdateTransactionReview(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "11111111-1111-1111-1111-111111111111"
	transactionID := "33333333-3333-3333-3333-333333333333"
	reviewedBy := "44444444-4444-4444-4444-444444444444"
	reviewedAt := time.Date(2026, time.June, 25, 11, 30, 0, 0, time.UTC)
	reviewNote := "Evidence requested"
	followUp := FollowUpEvidenceRequired
	repo := NewGORMRepository(newBankingDryRunDB(t,
		withBankingDryRunUpdateRows(1),
		withBankingDryRunFixtures(bankingDryRunFixtures{
			transaction: &models.BankTransaction{
				ID:              transactionID,
				TenantID:        tenantID,
				BankAccountID:   "22222222-2222-2222-2222-222222222222",
				TransactionDate: reviewedAt,
				Amount:          models.Decimal{Decimal: decimal.NewFromInt(75)},
				Currency:        "EUR",
				Status:          models.TransactionStatusUnmatched,
				FollowUpStatus:  models.TransactionFollowUpEvidenceRequired,
				ReviewNote:      reviewNote,
				ReviewedBy:      &reviewedBy,
				ReviewedAt:      &reviewedAt,
				ImportedAt:      reviewedAt,
			},
		}),
	))

	transaction, err := repo.UpdateTransactionReview(ctx, schemaName, tenantID, transactionID, TransactionReviewUpdate{
		FollowUpStatus: &followUp,
		ReviewNote:     &reviewNote,
		ReviewedBy:     reviewedBy,
		ReviewedAt:     reviewedAt,
	})

	require.NoError(t, err)
	require.NotNil(t, transaction)
	assert.Equal(t, transactionID, transaction.ID)
	assert.Equal(t, FollowUpEvidenceRequired, transaction.FollowUpStatus)
	assert.Equal(t, reviewNote, transaction.ReviewNote)
	assert.Equal(t, &reviewedBy, transaction.ReviewedBy)
}

func TestGORMRepositoryDryRunCreatePaymentFromTransaction(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "11111111-1111-1111-1111-111111111111"
	userID := "22222222-2222-2222-2222-222222222222"
	transactionDate := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)
	var createdPayment models.Payment
	repo := NewGORMRepository(newBankingDryRunDB(t,
		withBankingDryRunFixtures(bankingDryRunFixtures{
			paymentNumbers: []string{"OUT-00003", "OUT-00008"},
		}),
		withBankingDryRunCreateCapture(func(tx *gorm.DB) {
			payment, ok := tx.Statement.Dest.(*models.Payment)
			if ok {
				createdPayment = *payment
			}
		}),
		withBankingDryRunUpdateRows(1),
	))
	transaction := &BankTransaction{
		ID:              "33333333-3333-3333-3333-333333333333",
		TenantID:        tenantID,
		BankAccountID:   "44444444-4444-4444-4444-444444444444",
		TransactionDate: transactionDate,
		Amount:          decimal.NewFromInt(-225),
		Description:     "Supplier payment",
		Reference:       "BILL-42",
		Status:          StatusUnmatched,
	}

	paymentID, err := repo.CreatePaymentFromTransaction(ctx, schemaName, tenantID, userID, transaction)

	require.NoError(t, err)
	require.NotEmpty(t, paymentID)
	assert.Equal(t, paymentID, createdPayment.ID)
	assert.Equal(t, tenantID, createdPayment.TenantID)
	assert.Equal(t, models.PaymentTypeMade, createdPayment.PaymentType)
	assert.Equal(t, "OUT-00009", createdPayment.PaymentNumber)
	assert.Equal(t, "EUR", createdPayment.Currency)
	assert.True(t, createdPayment.Amount.Decimal.Equal(decimal.NewFromInt(225)))
	assert.Equal(t, "BILL-42", createdPayment.Reference)
	assert.Equal(t, userID, createdPayment.CreatedBy)
}

func TestGORMRepositoryDryRunIsTransactionDuplicate(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "11111111-1111-1111-1111-111111111111"
	accountID := "22222222-2222-2222-2222-222222222222"
	date := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)

	t.Run("external id match", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunFixtures(bankingDryRunFixtures{
			counts: []int64{1},
		})))

		duplicate, err := repo.IsTransactionDuplicate(ctx, schemaName, tenantID, accountID, date, decimal.NewFromInt(100), "external-1")

		require.NoError(t, err)
		assert.True(t, duplicate)
	})

	t.Run("date and amount match", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunFixtures(bankingDryRunFixtures{
			transactions: []models.BankTransaction{
				{
					ID:              "33333333-3333-3333-3333-333333333333",
					TenantID:        tenantID,
					BankAccountID:   accountID,
					TransactionDate: date,
					Amount:          models.Decimal{Decimal: decimal.NewFromInt(100)},
				},
			},
		})))

		duplicate, err := repo.IsTransactionDuplicate(ctx, schemaName, tenantID, accountID, date, decimal.NewFromInt(100), "")

		require.NoError(t, err)
		assert.True(t, duplicate)
	})

	t.Run("no match", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunFixtures(bankingDryRunFixtures{
			transactions: []models.BankTransaction{
				{
					ID:              "33333333-3333-3333-3333-333333333333",
					TenantID:        tenantID,
					BankAccountID:   accountID,
					TransactionDate: date.AddDate(0, 0, -1),
					Amount:          models.Decimal{Decimal: decimal.NewFromInt(99)},
				},
			},
		})))

		duplicate, err := repo.IsTransactionDuplicate(ctx, schemaName, tenantID, accountID, date, decimal.NewFromInt(100), "")

		require.NoError(t, err)
		assert.False(t, duplicate)
	})
}

func TestGORMRepositoryDryRunCompleteReconciliation(t *testing.T) {
	repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunUpdateRows(1, 2)))

	err := repo.CompleteReconciliation(
		context.Background(),
		"tenant_banking",
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	)

	require.NoError(t, err)
}

func TestGORMRepositoryDryRunCompleteReconciliationNoRows(t *testing.T) {
	repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunUpdateRows(0)))

	err := repo.CompleteReconciliation(
		context.Background(),
		"tenant_banking",
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
	)

	assert.ErrorIs(t, err, ErrReconciliationAlreadyDone)
}

func TestGORMRepositoryDryRunErrorPaths(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_banking"
	tenantID := "11111111-1111-1111-1111-111111111111"
	accountID := "22222222-2222-2222-2222-222222222222"
	expectedErr := errors.New("dry-run query failed")

	t.Run("list transactions query error", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

		transactions, err := repo.ListTransactions(ctx, schemaName, tenantID, nil)

		require.Nil(t, transactions)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("duplicate external id query error", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunQueryError(expectedErr)))

		duplicate, err := repo.IsTransactionDuplicate(ctx, schemaName, tenantID, accountID, time.Now(), decimal.NewFromInt(1), "external-1")

		assert.False(t, duplicate)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("complete reconciliation update error", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunUpdateError(expectedErr)))

		err := repo.CompleteReconciliation(ctx, schemaName, tenantID, "33333333-3333-3333-3333-333333333333")

		assert.ErrorIs(t, err, expectedErr)
	})
}

func TestGORMRepositoryDryRunListPaymentMatchCandidatesErrors(t *testing.T) {
	ctx := context.Background()
	tenantID := "11111111-1111-1111-1111-111111111111"

	t.Run("invalid schema", func(t *testing.T) {
		repo := NewGORMRepository(newBankingDryRunDB(t))

		candidates, err := repo.ListPaymentMatchCandidates(ctx, "tenant-banking", tenantID, payments.PaymentTypeReceived, decimal.NewFromInt(125), 5)

		require.Nil(t, candidates)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid SQL identifier")
	})

	t.Run("scan error", func(t *testing.T) {
		expectedErr := errors.New("dry-run scan failed")
		repo := NewGORMRepository(newBankingDryRunDB(t, withBankingDryRunRowError(expectedErr)))

		candidates, err := repo.ListPaymentMatchCandidates(ctx, "tenant_banking", tenantID, payments.PaymentTypeReceived, decimal.NewFromInt(125), 5)

		require.Nil(t, candidates)
		assert.ErrorIs(t, err, expectedErr)
	})
}
