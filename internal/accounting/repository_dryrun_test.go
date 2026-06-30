package accounting

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
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

type accountingDryRunConnPool struct{}

func (accountingDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run accounting tests should not prepare statements")
}

func (accountingDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run accounting tests should not execute statements")
}

func (accountingDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run accounting tests should not query rows")
}

func (accountingDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (accountingDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &accountingDryRunTx{}, nil
}

type accountingDryRunTx struct {
	accountingDryRunConnPool
}

func (*accountingDryRunTx) Commit() error {
	return nil
}

func (*accountingDryRunTx) Rollback() error {
	return nil
}

type accountingDryRunDBOption func(t *testing.T, db *gorm.DB)

type accountingDryRunFixture struct {
	accounts          []models.Account
	accountIndex      int
	balanceAggregates []accountingBalanceAggregate
	balanceIndex      int

	journalEntries    []models.JournalEntry
	journalEntryIndex int
	journalEntryLines []models.JournalEntryLine
	sequenceNumber    int

	templates          []models.JournalEntryTemplate
	templateIndex      int
	templateLines      []models.JournalEntryTemplateLine
	templateLineCounts []accountingTemplateLineCount
	dueTemplateIDs     []string

	reportBalances []accountingReportBalance

	costCenters     []models.CostCenter
	costCenterIndex int
	costAllocations []accountingCostAllocationRow
	expenseTotals   []decimal.Decimal
	expenseIndex    int

	counts     []accountingCountResult
	countIndex int
}

type accountingBalanceAggregate struct {
	debits  decimal.Decimal
	credits decimal.Decimal
}

type accountingReportBalance struct {
	accountID    string
	accountCode  string
	accountName  string
	accountType  string
	totalDebits  decimal.Decimal
	totalCredits decimal.Decimal
	netBalance   decimal.Decimal
}

type accountingTemplateLineCount struct {
	templateID string
	lineCount  int
}

type accountingCostAllocationRow struct {
	allocation     models.CostAllocation
	costCenterCode string
	costCenterName string
}

type accountingCountResult struct {
	value int64
	err   error
}

var accountingDryRunCallbackID uint64

func newAccountingDryRunDB(t *testing.T, opts ...accountingDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: accountingDryRunConnPool{}}), &gorm.Config{
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

func withAccountingDryRunFixtures(fixture accountingDryRunFixture) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(accountingDryRunCallbackName("query_fixtures"), func(tx *gorm.DB) {
			if populateAccountingDryRunDest(tx, tx.Statement.Dest, &fixture) {
				if tx.RowsAffected == 0 {
					tx.RowsAffected = 1
				}
			}
		})
		require.NoError(t, err)
	}
}

func withAccountingDryRunQueryError(expectedErr error) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().Before("gorm:query").Register(accountingDryRunCallbackName("query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAccountingDryRunCreateError(expectedErr error) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Create().Before("gorm:create").Register(accountingDryRunCallbackName("create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAccountingDryRunUpdateRows(rows ...int64) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Update().After("gorm:update").Register(accountingDryRunCallbackName("update_rows"), func(tx *gorm.DB) {
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

func withAccountingDryRunUpdateError(expectedErr error) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Update().Before("gorm:update").Register(accountingDryRunCallbackName("update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAccountingDryRunExecRows(rows ...int64) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Raw().After("gorm:raw").Register(accountingDryRunCallbackName("exec_rows"), func(tx *gorm.DB) {
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

func withAccountingDryRunExecError(expectedErr error) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Raw().Before("gorm:raw").Register(accountingDryRunCallbackName("exec_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAccountingDryRunCapturedQueries(queries *[]string) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Query().After("gorm:query").Register(accountingDryRunCallbackName("capture_query_sql"), func(tx *gorm.DB) {
			*queries = append(*queries, tx.Statement.SQL.String())
		})
		require.NoError(t, err)
	}
}

func withAccountingDryRunCapturedRows(rows *[]string) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		err := db.Callback().Row().After("gorm:row").Register(accountingDryRunCallbackName("capture_row_sql"), func(tx *gorm.DB) {
			*rows = append(*rows, tx.Statement.SQL.String())
		})
		require.NoError(t, err)
	}
}

func accountingDryRunCallbackName(suffix string) string {
	id := atomic.AddUint64(&accountingDryRunCallbackID, 1)
	return fmt.Sprintf("accounting_dryrun:%d:%s", id, suffix)
}

func populateAccountingDryRunDest(tx *gorm.DB, dest any, fixture *accountingDryRunFixture) bool {
	switch typed := dest.(type) {
	case *models.Account:
		if len(fixture.accounts) == 0 {
			return false
		}
		*typed = fixture.accounts[fixture.nextAccountIndex()]
		return true
	case *[]models.Account:
		*typed = append((*typed)[:0], fixture.accounts...)
		return true
	case *models.JournalEntry:
		if len(fixture.journalEntries) == 0 {
			return false
		}
		*typed = fixture.journalEntries[fixture.nextJournalEntryIndex()]
		return true
	case *[]models.JournalEntry:
		*typed = append((*typed)[:0], fixture.journalEntries...)
		return true
	case *[]models.JournalEntryLine:
		*typed = append((*typed)[:0], fixture.journalEntryLines...)
		return true
	case *models.JournalEntryTemplate:
		if len(fixture.templates) == 0 {
			return false
		}
		*typed = fixture.templates[fixture.nextTemplateIndex()]
		return true
	case *[]models.JournalEntryTemplate:
		*typed = append((*typed)[:0], fixture.templates...)
		return true
	case *[]models.JournalEntryTemplateLine:
		*typed = append((*typed)[:0], fixture.templateLines...)
		return true
	case *[]string:
		*typed = append((*typed)[:0], fixture.dueTemplateIDs...)
		return true
	case *models.CostCenter:
		if len(fixture.costCenters) == 0 {
			return false
		}
		*typed = fixture.costCenters[fixture.nextCostCenterIndex()]
		return true
	case *[]models.CostCenter:
		*typed = append((*typed)[:0], fixture.costCenters...)
		return true
	case *int:
		*typed = fixture.sequenceNumber
		return true
	case *int64:
		count := fixture.nextCount()
		if count.err != nil {
			tx.AddError(count.err)
			return true
		}
		*typed = count.value
		return true
	default:
		return populateAccountingReflectDest(dest, fixture)
	}
}

func (f *accountingDryRunFixture) nextAccountIndex() int {
	index := f.accountIndex
	if index >= len(f.accounts) {
		index = len(f.accounts) - 1
	}
	f.accountIndex++
	return index
}

func (f *accountingDryRunFixture) nextJournalEntryIndex() int {
	index := f.journalEntryIndex
	if index >= len(f.journalEntries) {
		index = len(f.journalEntries) - 1
	}
	f.journalEntryIndex++
	return index
}

func (f *accountingDryRunFixture) nextTemplateIndex() int {
	index := f.templateIndex
	if index >= len(f.templates) {
		index = len(f.templates) - 1
	}
	f.templateIndex++
	return index
}

func (f *accountingDryRunFixture) nextCostCenterIndex() int {
	index := f.costCenterIndex
	if index >= len(f.costCenters) {
		index = len(f.costCenters) - 1
	}
	f.costCenterIndex++
	return index
}

func (f *accountingDryRunFixture) nextBalanceAggregate() accountingBalanceAggregate {
	if len(f.balanceAggregates) == 0 {
		return accountingBalanceAggregate{}
	}
	index := f.balanceIndex
	if index >= len(f.balanceAggregates) {
		index = len(f.balanceAggregates) - 1
	}
	f.balanceIndex++
	return f.balanceAggregates[index]
}

func (f *accountingDryRunFixture) nextExpenseTotal() decimal.Decimal {
	if len(f.expenseTotals) == 0 {
		return decimal.Zero
	}
	index := f.expenseIndex
	if index >= len(f.expenseTotals) {
		index = len(f.expenseTotals) - 1
	}
	f.expenseIndex++
	return f.expenseTotals[index]
}

func (f *accountingDryRunFixture) nextCount() accountingCountResult {
	if len(f.counts) == 0 {
		return accountingCountResult{}
	}
	index := f.countIndex
	if index >= len(f.counts) {
		index = len(f.counts) - 1
	}
	f.countIndex++
	return f.counts[index]
}

func populateAccountingReflectDest(dest any, fixture *accountingDryRunFixture) bool {
	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return false
	}

	target := value.Elem()
	switch target.Kind() {
	case reflect.Struct:
		if setAccountingBalanceAggregate(target, fixture.nextBalanceAggregate()) {
			return true
		}
		if setAccountingExpenseTotal(target, fixture.nextExpenseTotal()) {
			return true
		}
	case reflect.Slice:
		if setAccountingTemplateLineCountRows(target, fixture.templateLineCounts) {
			return true
		}
		if setAccountingReportBalanceRows(target, fixture.reportBalances) {
			return true
		}
		if setAccountingCostAllocationRows(target, fixture.costAllocations) {
			return true
		}
	}
	return false
}

func setAccountingBalanceAggregate(target reflect.Value, aggregate accountingBalanceAggregate) bool {
	if !target.FieldByName("DebitSum").IsValid() || !target.FieldByName("CreditSum").IsValid() {
		return false
	}
	setModelDecimalField(target, "DebitSum", aggregate.debits)
	setModelDecimalField(target, "CreditSum", aggregate.credits)
	return true
}

func setAccountingExpenseTotal(target reflect.Value, total decimal.Decimal) bool {
	if !target.FieldByName("Total").IsValid() {
		return false
	}
	setModelDecimalField(target, "Total", total)
	return true
}

func setAccountingTemplateLineCountRows(target reflect.Value, counts []accountingTemplateLineCount) bool {
	elemType := target.Type().Elem()
	if _, ok := elemType.FieldByName("TemplateID"); !ok {
		return false
	}
	if _, ok := elemType.FieldByName("LineCount"); !ok {
		return false
	}

	rows := reflect.MakeSlice(target.Type(), len(counts), len(counts))
	for i, count := range counts {
		row := rows.Index(i)
		setStringField(row, "TemplateID", count.templateID)
		setIntField(row, "LineCount", count.lineCount)
	}
	target.Set(rows)
	return true
}

func setAccountingReportBalanceRows(target reflect.Value, balances []accountingReportBalance) bool {
	elemType := target.Type().Elem()
	if _, ok := elemType.FieldByName("AccountID"); !ok {
		return false
	}
	if _, ok := elemType.FieldByName("NetBalance"); !ok {
		return false
	}

	rows := reflect.MakeSlice(target.Type(), len(balances), len(balances))
	for i, balance := range balances {
		row := rows.Index(i)
		setStringField(row, "AccountID", balance.accountID)
		setStringField(row, "AccountCode", balance.accountCode)
		setStringField(row, "AccountName", balance.accountName)
		setStringField(row, "AccountType", balance.accountType)
		setModelDecimalField(row, "TotalDebits", balance.totalDebits)
		setModelDecimalField(row, "TotalCredits", balance.totalCredits)
		setModelDecimalField(row, "NetBalance", balance.netBalance)
	}
	target.Set(rows)
	return true
}

func setAccountingCostAllocationRows(target reflect.Value, allocations []accountingCostAllocationRow) bool {
	elemType := target.Type().Elem()
	allocationField, ok := elemType.FieldByName("CostAllocation")
	if !ok || allocationField.Type != reflect.TypeOf(models.CostAllocation{}) {
		return false
	}

	rows := reflect.MakeSlice(target.Type(), len(allocations), len(allocations))
	for i, allocation := range allocations {
		row := rows.Index(i)
		field := row.FieldByName("CostAllocation")
		if field.CanSet() {
			field.Set(reflect.ValueOf(allocation.allocation))
		}
		setStringField(row, "CostCenterCode", allocation.costCenterCode)
		setStringField(row, "CostCenterName", allocation.costCenterName)
	}
	target.Set(rows)
	return true
}

func setStringField(target reflect.Value, name string, value string) {
	field := target.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
		field.SetString(value)
	}
}

func setIntField(target reflect.Value, name string, value int) {
	field := target.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.Int {
		field.SetInt(int64(value))
	}
}

func setModelDecimalField(target reflect.Value, name string, value decimal.Decimal) {
	field := target.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Type() == reflect.TypeOf(models.Decimal{}) {
		field.Set(reflect.ValueOf(models.NewDecimal(value)))
	}
}

func TestRepositoryConstructorsNilPoolAndTenantGuards(t *testing.T) {
	ctx := context.Background()

	repo := NewRepository(nil)
	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	table, err := repo.tenantTable(ctx, "tenant_schema", "accounts")
	require.Error(t, err)
	assert.Nil(t, table)
	assert.Contains(t, err.Error(), "accounting repository database is not configured")

	aliasTable, err := repo.tenantTableAlias(ctx, "tenant_schema", "journal_entries", "je")
	require.Error(t, err)
	assert.Nil(t, aliasTable)
	assert.Contains(t, err.Error(), "accounting repository database is not configured")

	dryRepo := NewGORMRepository(newAccountingDryRunDB(t))
	aliasTable, err = dryRepo.tenantTableAlias(ctx, "tenant_schema", "journal_entries", "je")
	require.NoError(t, err)
	assert.NotNil(t, aliasTable)

	aliasTable, err = dryRepo.tenantTableAlias(ctx, "bad.schema", "journal_entries", "je")
	require.Error(t, err)
	assert.Nil(t, aliasTable)
	assert.Contains(t, err.Error(), "invalid SQL identifier")

	aliasTable, err = dryRepo.tenantTableAlias(ctx, "tenant_schema", "journal_entries", "bad alias")
	require.Error(t, err)
	assert.Nil(t, aliasTable)
	assert.Contains(t, err.Error(), "invalid SQL identifier")

	costCenterRepo := NewCostCenterRepository(nil)
	require.NotNil(t, costCenterRepo)
	assert.Nil(t, costCenterRepo.db)

	costCenterTable, err := costCenterRepo.tenantTable(ctx, "tenant_schema", "cost_centers")
	require.Error(t, err)
	assert.Nil(t, costCenterTable)
	assert.Contains(t, err.Error(), "cost center repository database is not configured")
}

func TestGORMRepositoryDryRunAccountOperations(t *testing.T) {
	ctx := context.Background()
	createdAt := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.UTC)
	accountModel := models.Account{
		ID:          "account-1",
		TenantID:    "tenant-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: models.AccountTypeAsset,
		IsActive:    true,
		CreatedAt:   createdAt,
	}
	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{accounts: []models.Account{accountModel}}),
		withAccountingDryRunUpdateRows(1),
	))

	account, err := repo.GetAccountByID(ctx, "tenant_schema", "tenant-1", accountModel.ID)
	require.NoError(t, err)
	assert.Equal(t, "Cash", account.Name)

	accounts, err := repo.ListAccounts(ctx, "tenant_schema", "tenant-1", true)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "1000", accounts[0].Code)

	newAccount := &Account{TenantID: "tenant-1", Code: "1100", Name: "Bank", AccountType: AccountTypeAsset, IsActive: true}
	require.NoError(t, repo.CreateAccount(ctx, "tenant_schema", newAccount))
	assert.NotEmpty(t, newAccount.ID)
	assert.False(t, newAccount.CreatedAt.IsZero())

	require.NoError(t, repo.UpdateAccount(ctx, "tenant_schema", account))
}

func TestGORMRepositoryDryRunAccountErrors(t *testing.T) {
	ctx := context.Background()
	account := &Account{ID: "account-1", TenantID: "tenant-1", Code: "1000", Name: "Cash", AccountType: AccountTypeAsset}

	t.Run("get not found", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(gorm.ErrRecordNotFound)))
		got, err := repo.GetAccountByID(ctx, "tenant_schema", "tenant-1", account.ID)
		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "account not found")
	})

	t.Run("list wraps query error", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(assert.AnError)))
		got, err := repo.ListAccounts(ctx, "tenant_schema", "tenant-1", false)
		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "list accounts")
	})

	t.Run("create wraps create error", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunCreateError(assert.AnError)))
		err := repo.CreateAccount(ctx, "tenant_schema", account)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "create account")
	})

	t.Run("update wraps update error", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateError(assert.AnError)))
		err := repo.UpdateAccount(ctx, "tenant_schema", account)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "update account")
	})

	t.Run("update reports missing account", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateRows(0)))
		err := repo.UpdateAccount(ctx, "tenant_schema", account)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account not found")
	})
}

func TestGORMRepositoryDryRunJournalEntryOperations(t *testing.T) {
	ctx := context.Background()
	entryDate := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	entryModel := models.JournalEntry{
		ID:          "entry-1",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00001",
		EntryDate:   entryDate,
		Description: "Accrual",
		Status:      models.JournalStatusDraft,
		CreatedAt:   entryDate,
		CreatedBy:   "user-1",
	}
	lineModel := models.JournalEntryLine{
		ID:             "line-1",
		TenantID:       "tenant-1",
		JournalEntryID: "entry-1",
		AccountID:      "account-1",
		DebitAmount:    models.NewDecimal(decimal.NewFromInt(25)),
		Currency:       "EUR",
		ExchangeRate:   models.NewDecimal(decimal.NewFromInt(1)),
		BaseDebit:      models.NewDecimal(decimal.NewFromInt(25)),
	}
	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{
			journalEntries:    []models.JournalEntry{entryModel, entryModel, entryModel},
			journalEntryLines: []models.JournalEntryLine{lineModel},
			sequenceNumber:    12,
		}),
		withAccountingDryRunUpdateRows(1),
	))

	entry, err := repo.GetJournalEntryByID(ctx, "tenant_schema", "tenant-1", entryModel.ID)
	require.NoError(t, err)
	require.Len(t, entry.Lines, 1)
	assert.Equal(t, "line-1", entry.Lines[0].ID)

	sourceEntry, err := repo.GetJournalEntryBySource(ctx, "tenant_schema", "tenant-1", "IMPORT", "source-1")
	require.NoError(t, err)
	require.NotNil(t, sourceEntry)
	assert.Equal(t, "JE-00001", sourceEntry.EntryNumber)

	entries, err := repo.ListJournalEntries(ctx, "tenant_schema", "tenant-1", 0)
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	require.Len(t, entries[len(entries)-1].Lines, 1)

	newEntry := &JournalEntry{
		TenantID:    "tenant-1",
		EntryDate:   entryDate,
		Description: "Dry-run entry",
		Status:      StatusDraft,
		CreatedBy:   "user-1",
		Lines: []JournalEntryLine{{
			AccountID:      "account-1",
			DebitAmount:    decimal.NewFromInt(25),
			Currency:       "EUR",
			ExchangeRate:   decimal.NewFromInt(1),
			BaseDebit:      decimal.NewFromInt(25),
			BaseCredit:     decimal.Zero,
			CreditAmount:   decimal.Zero,
			Description:    "Debit",
			JournalEntryID: "",
		}},
	}
	err = repo.CreateJournalEntry(ctx, "tenant_schema", newEntry)
	requireAccountingDryRunScanError(t, err, "generate entry number")

	require.NoError(t, repo.UpdateJournalEntryStatus(ctx, "tenant_schema", "tenant-1", "entry-1", StatusPosted, "user-1", "Dry-run posting reason"))
}

func TestGORMRepositoryDryRunJournalEntryErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("get by source returns nil for missing source", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(gorm.ErrRecordNotFound)))
		entry, err := repo.GetJournalEntryBySource(ctx, "tenant_schema", "tenant-1", "IMPORT", "source-1")
		require.NoError(t, err)
		assert.Nil(t, entry)
	})

	t.Run("list wraps line query errors", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t,
			withAccountingDryRunFixtures(accountingDryRunFixture{journalEntries: []models.JournalEntry{{ID: "entry-1", TenantID: "tenant-1"}}}),
			withAccountingDryRunQueryError(assert.AnError),
		))
		entries, err := repo.ListJournalEntries(ctx, "tenant_schema", "tenant-1", 10)
		require.Error(t, err)
		assert.Nil(t, entries)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("create wraps create errors", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t,
			withAccountingDryRunFixtures(accountingDryRunFixture{sequenceNumber: 1}),
			withAccountingDryRunCreateError(assert.AnError),
		))
		err := repo.CreateJournalEntry(ctx, "tenant_schema", &JournalEntry{TenantID: "tenant-1"})
		requireAccountingDryRunScanError(t, err, "generate entry number")
	})

	t.Run("posting missing draft returns transition error", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateRows(0)))
		err := repo.UpdateJournalEntryStatus(ctx, "tenant_schema", "tenant-1", "entry-1", StatusPosted, "user-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "entry not found or invalid status transition")
	})

	t.Run("voided status requires void method", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t))
		err := repo.UpdateJournalEntryStatus(ctx, "tenant_schema", "tenant-1", "entry-1", StatusVoided, "user-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "use VoidJournalEntry method")
	})

	t.Run("invalid status transition", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t))
		err := repo.UpdateJournalEntryStatus(ctx, "tenant_schema", "tenant-1", "entry-1", JournalEntryStatus("ARCHIVED"), "user-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")
	})
}

func TestGORMRepositoryDryRunJournalEntryTemplates(t *testing.T) {
	ctx := context.Background()
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	nextDate := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, time.June, 1, 8, 0, 0, 0, time.UTC)
	templateModel := models.JournalEntryTemplate{
		ID:                 "template-1",
		TenantID:           "tenant-1",
		Name:               "Monthly accrual",
		IsActive:           true,
		Frequency:          string(JournalEntryTemplateFrequencyMonthly),
		StartDate:          &startDate,
		NextGenerationDate: &nextDate,
		CreatedAt:          createdAt,
		CreatedBy:          "user-1",
		UpdatedAt:          createdAt,
	}
	lineModel := models.JournalEntryTemplateLine{
		ID:           "template-line-1",
		TemplateID:   "template-1",
		LineNumber:   1,
		AccountID:    "account-1",
		DebitAmount:  models.NewDecimal(decimal.NewFromInt(10)),
		Currency:     "EUR",
		ExchangeRate: models.NewDecimal(decimal.NewFromInt(1)),
	}
	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{
			templates:          []models.JournalEntryTemplate{templateModel, templateModel},
			templateLines:      []models.JournalEntryTemplateLine{lineModel},
			templateLineCounts: []accountingTemplateLineCount{{templateID: "template-1", lineCount: 1}},
			dueTemplateIDs:     []string{"template-1"},
		}),
		withAccountingDryRunUpdateRows(1),
	))

	template := &JournalEntryTemplate{
		TenantID: "tenant-1",
		Name:     "Monthly accrual",
		IsActive: true,
		Lines: []JournalEntryTemplateLine{{
			AccountID:    "account-1",
			DebitAmount:  decimal.NewFromInt(10),
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
		}},
	}
	require.NoError(t, repo.CreateJournalEntryTemplate(ctx, "tenant_schema", template))
	assert.NotEmpty(t, template.ID)
	assert.False(t, template.CreatedAt.IsZero())
	assert.Equal(t, 1, template.LineCount)
	assert.Equal(t, template.ID, template.Lines[0].TemplateID)
	assert.Equal(t, 1, template.Lines[0].LineNumber)

	templates, err := repo.ListJournalEntryTemplates(ctx, "tenant_schema", "tenant-1", true)
	requireAccountingDryRunScanError(t, err, "count journal entry template lines")
	assert.Nil(t, templates)

	gotTemplate, err := repo.GetJournalEntryTemplateByID(ctx, "tenant_schema", "tenant-1", "template-1")
	require.NoError(t, err)
	require.Len(t, gotTemplate.Lines, 1)
	assert.Equal(t, "template-line-1", gotTemplate.Lines[0].ID)

	dueIDs, err := repo.GetDueJournalEntryTemplateIDs(ctx, "tenant_schema", "tenant-1", nextDate)
	require.NoError(t, err)
	assert.Equal(t, []string{"template-1"}, dueIDs)

	require.NoError(t, repo.UpdateJournalEntryTemplateAfterGeneration(ctx, "tenant_schema", "tenant-1", "template-1", nextDate, createdAt))

	counts, err := repo.countJournalEntryTemplateLines(ctx, "tenant_schema", []string{"template-1"})
	requireAccountingDryRunScanError(t, err, "count journal entry template lines")
	assert.Nil(t, counts)

	emptyCounts, err := repo.countJournalEntryTemplateLines(ctx, "tenant_schema", nil)
	require.NoError(t, err)
	assert.Empty(t, emptyCounts)
}

func TestGORMRepositoryDryRunJournalEntryTemplateErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("create wraps line create errors", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunCreateError(assert.AnError)))
		err := repo.CreateJournalEntryTemplate(ctx, "tenant_schema", &JournalEntryTemplate{
			TenantID: "tenant-1",
			Name:     "Template",
			Lines:    []JournalEntryTemplateLine{{AccountID: "account-1"}},
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "insert journal entry template")
	})

	t.Run("get by id not found", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(gorm.ErrRecordNotFound)))
		template, err := repo.GetJournalEntryTemplateByID(ctx, "tenant_schema", "tenant-1", "template-1")
		require.Error(t, err)
		assert.Nil(t, template)
		assert.Contains(t, err.Error(), "journal entry template not found")
	})

	t.Run("update reports missing template", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateRows(0)))
		err := repo.UpdateJournalEntryTemplateAfterGeneration(ctx, "tenant_schema", "tenant-1", "template-1", time.Now(), time.Now())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "journal entry template not found")
	})
}

func TestGORMRepositoryDryRunBalances(t *testing.T) {
	ctx := context.Background()
	asOfDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
	assetAccount := models.Account{ID: "asset-1", TenantID: "tenant-1", Code: "1000", Name: "Cash", AccountType: models.AccountTypeAsset, IsActive: true}
	revenueAccount := models.Account{ID: "revenue-1", TenantID: "tenant-1", Code: "4000", Name: "Revenue", AccountType: models.AccountTypeRevenue, IsActive: true}
	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
		accounts: []models.Account{assetAccount, revenueAccount},
		balanceAggregates: []accountingBalanceAggregate{
			{debits: decimal.NewFromInt(125), credits: decimal.NewFromInt(25)},
			{debits: decimal.NewFromInt(10), credits: decimal.NewFromInt(80)},
		},
		reportBalances: []accountingReportBalance{{
			accountID:    "asset-1",
			accountCode:  "1000",
			accountName:  "Cash",
			accountType:  string(AccountTypeAsset),
			totalDebits:  decimal.NewFromInt(125),
			totalCredits: decimal.NewFromInt(25),
			netBalance:   decimal.NewFromInt(100),
		}},
	})))

	assetBalance, err := repo.GetAccountBalance(ctx, "tenant_schema", "tenant-1", "asset-1", asOfDate)
	requireAccountingDryRunScanError(t, err, "get account balance")
	assert.True(t, assetBalance.IsZero())

	revenueBalance, err := repo.GetAccountBalance(ctx, "tenant_schema", "tenant-1", "revenue-1", asOfDate)
	requireAccountingDryRunScanError(t, err, "get account balance")
	assert.True(t, revenueBalance.IsZero())

	trialBalance, err := repo.GetTrialBalance(ctx, "tenant_schema", "tenant-1", asOfDate)
	requireAccountingDryRunScanError(t, err, "get trial balance")
	assert.Nil(t, trialBalance)

	periodBalances, err := repo.GetPeriodBalances(ctx, "tenant_schema", "tenant-1", asOfDate.AddDate(0, -1, 0), asOfDate)
	requireAccountingDryRunScanError(t, err, "get period balances")
	assert.Nil(t, periodBalances)
}

func TestGORMRepositoryReportBalanceQueriesStartFromPostedJournalEntries(t *testing.T) {
	ctx := context.Background()
	asOfDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
	rowQueries := []string{}
	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunCapturedRows(&rowQueries)))

	trialBalance, err := repo.GetTrialBalance(ctx, "tenant_schema", "tenant-1", asOfDate)
	requireAccountingDryRunScanError(t, err, "get trial balance")
	assert.Nil(t, trialBalance)
	require.NotEmpty(t, rowQueries)
	trialQuery := rowQueries[len(rowQueries)-1]
	assert.Contains(t, trialQuery, `FROM "tenant_schema"."journal_entries" AS "je"`)
	assert.Contains(t, trialQuery, `JOIN "tenant_schema"."journal_entry_lines" AS jel ON jel.journal_entry_id = je.id AND jel.tenant_id = je.tenant_id`)
	assert.Contains(t, trialQuery, `JOIN "tenant_schema"."accounts" AS a ON a.id = jel.account_id AND a.tenant_id = jel.tenant_id`)
	assert.Contains(t, trialQuery, `je.entry_date <=`)
	assert.Contains(t, trialQuery, `je.status =`)
	assert.NotContains(t, trialQuery, `LEFT JOIN`)
	assert.NotContains(t, trialQuery, `je.id IS NULL`)

	periodBalances, err := repo.GetPeriodBalances(ctx, "tenant_schema", "tenant-1", asOfDate.AddDate(0, -1, 0), asOfDate)
	requireAccountingDryRunScanError(t, err, "get period balances")
	assert.Nil(t, periodBalances)
	require.Len(t, rowQueries, 2)
	periodQuery := rowQueries[len(rowQueries)-1]
	assert.Contains(t, periodQuery, `FROM "tenant_schema"."journal_entries" AS "je"`)
	assert.Contains(t, periodQuery, `JOIN "tenant_schema"."journal_entry_lines" AS jel ON jel.journal_entry_id = je.id AND jel.tenant_id = je.tenant_id`)
	assert.Contains(t, periodQuery, `JOIN "tenant_schema"."accounts" AS a ON a.id = jel.account_id AND a.tenant_id = jel.tenant_id`)
	assert.Contains(t, periodQuery, `je.entry_date >=`)
	assert.Contains(t, periodQuery, `je.entry_date <=`)
	assert.Contains(t, periodQuery, `je.status =`)
	assert.NotContains(t, periodQuery, `LEFT JOIN`)
	assert.NotContains(t, periodQuery, `je.id IS NULL`)
}

func TestGORMRepositoryDryRunBalanceErrors(t *testing.T) {
	ctx := context.Background()
	asOfDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)

	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(assert.AnError)))

	balance, err := repo.GetAccountBalance(ctx, "tenant_schema", "tenant-1", "account-1", asOfDate)
	require.Error(t, err)
	assert.True(t, balance.IsZero())
	assert.ErrorIs(t, err, assert.AnError)

	trialBalance, err := repo.GetTrialBalance(ctx, "tenant_schema", "tenant-1", asOfDate)
	require.Error(t, err)
	assert.Nil(t, trialBalance)
	assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)

	periodBalances, err := repo.GetPeriodBalances(ctx, "tenant_schema", "tenant-1", asOfDate.AddDate(0, -1, 0), asOfDate)
	require.Error(t, err)
	assert.Nil(t, periodBalances)
	assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)
}

func TestGORMRepositoryDryRunVoidJournalEntry(t *testing.T) {
	ctx := context.Background()
	reversal := &JournalEntry{
		TenantID:    "tenant-1",
		EntryDate:   time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC),
		Description: "Reversal",
		Status:      StatusDraft,
		CreatedBy:   "user-1",
	}
	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{sequenceNumber: 99}),
		withAccountingDryRunUpdateRows(1),
	))

	err := repo.VoidJournalEntry(ctx, "tenant_schema", "tenant-1", "entry-1", "user-1", "duplicate", reversal)
	requireAccountingDryRunScanError(t, err, "generate entry number")
}

func TestGORMRepositoryDryRunVoidJournalEntryErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("update error", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateError(assert.AnError)))
		err := repo.VoidJournalEntry(ctx, "tenant_schema", "tenant-1", "entry-1", "user-1", "duplicate", &JournalEntry{})
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "mark entry as voided")
	})

	t.Run("missing posted entry", func(t *testing.T) {
		repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateRows(0)))
		err := repo.VoidJournalEntry(ctx, "tenant_schema", "tenant-1", "entry-1", "user-1", "duplicate", &JournalEntry{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "entry not found or not in posted status")
	})
}

func TestCostCenterGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.UTC)
	budget := models.NewDecimal(decimal.NewFromInt(5000))
	costCenterModel := models.CostCenter{
		ID:           "cc-1",
		TenantID:     "tenant-1",
		Code:         "OPS",
		Name:         "Operations",
		IsActive:     true,
		BudgetAmount: &budget,
		BudgetPeriod: string(BudgetPeriodAnnual),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	allocationModel := models.CostAllocation{
		ID:                 "allocation-1",
		TenantID:           "tenant-1",
		CostCenterID:       "cc-1",
		JournalEntryLineID: "line-1",
		Amount:             models.NewDecimal(decimal.NewFromInt(125)),
		AllocationDate:     now,
		CreatedAt:          now,
	}
	repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{
			costCenters:   []models.CostCenter{costCenterModel},
			expenseTotals: []decimal.Decimal{decimal.NewFromInt(125)},
			costAllocations: []accountingCostAllocationRow{{
				allocation:     allocationModel,
				costCenterCode: "OPS",
				costCenterName: "Operations",
			}},
			counts: []accountingCountResult{{value: 0}, {value: 0}},
		}),
		withAccountingDryRunUpdateRows(1),
		withAccountingDryRunExecRows(1),
	))

	costCenter, err := repo.GetByID(ctx, "tenant_schema", "tenant-1", "cc-1")
	require.NoError(t, err)
	assert.Equal(t, "OPS", costCenter.Code)

	costCenters, err := repo.List(ctx, "tenant_schema", "tenant-1", true)
	require.NoError(t, err)
	require.Len(t, costCenters, 1)
	assert.Equal(t, "Operations", costCenters[0].Name)

	newCostCenter := &CostCenter{TenantID: "tenant-1", Code: "ADMIN", Name: "Administration", IsActive: true}
	require.NoError(t, repo.Create(ctx, "tenant_schema", newCostCenter))
	assert.NotEmpty(t, newCostCenter.ID)
	assert.Equal(t, BudgetPeriodAnnual, newCostCenter.BudgetPeriod)
	assert.False(t, newCostCenter.CreatedAt.IsZero())
	assert.False(t, newCostCenter.UpdatedAt.IsZero())

	require.NoError(t, repo.Update(ctx, "tenant_schema", costCenter))
	require.NoError(t, repo.Delete(ctx, "tenant_schema", "tenant-1", "cc-1"))

	total, err := repo.GetExpensesByPeriod(ctx, "tenant_schema", "tenant-1", "cc-1", now.AddDate(0, 0, -1), now)
	requireAccountingDryRunScanError(t, err, "get expenses")
	assert.True(t, total.IsZero())

	allocation := &CostAllocation{
		TenantID:           "tenant-1",
		CostCenterID:       "cc-1",
		JournalEntryLineID: "line-1",
		Amount:             decimal.NewFromInt(125),
		AllocationDate:     now,
	}
	require.NoError(t, repo.CreateAllocation(ctx, "tenant_schema", allocation))
	assert.NotEmpty(t, allocation.ID)
	assert.False(t, allocation.CreatedAt.IsZero())

	allocations, err := repo.ListAllocations(ctx, "tenant_schema", "tenant-1", CostAllocationFilters{
		CostCenterID:       " cc-1 ",
		JournalEntryLineID: " line-1 ",
		StartDate:          &now,
		EndDate:            &now,
	})
	requireAccountingDryRunScanError(t, err, "list cost allocations")
	assert.Nil(t, allocations)
}

func TestCostCenterGORMRepositoryWave11ScanSuccessPaths(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.UTC)
	repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunScanRowsWave4(
		accountingDryRunRowSetWave4{
			columns: []string{"total"},
			values:  [][]driver.Value{{"125.50"}},
		},
		accountingDryRunRowSetWave4{
			columns: []string{
				"id",
				"tenant_id",
				"cost_center_id",
				"journal_entry_line_id",
				"amount",
				"allocation_date",
				"created_at",
				"cost_center_code",
				"cost_center_name",
			},
			values: [][]driver.Value{{
				"allocation-1",
				"tenant-1",
				"cc-1",
				"line-1",
				"125.50",
				now,
				now,
				"OPS",
				"Operations",
			}},
		},
	)))

	total, err := repo.GetExpensesByPeriod(ctx, "tenant_schema", "tenant-1", "cc-1", now.AddDate(0, 0, -1), now)
	require.NoError(t, err)
	assert.True(t, decimal.RequireFromString("125.50").Equal(total))

	allocations, err := repo.ListAllocations(ctx, "tenant_schema", "tenant-1", CostAllocationFilters{})
	require.NoError(t, err)
	require.Len(t, allocations, 1)
	assert.Equal(t, "OPS", allocations[0].CostCenterCode)
	assert.Equal(t, "Operations", allocations[0].CostCenterName)
	assert.True(t, decimal.RequireFromString("125.50").Equal(allocations[0].Amount))
}

func requireAccountingDryRunScanError(t *testing.T, err error, operation string) {
	t.Helper()

	require.Error(t, err)
	assert.Contains(t, err.Error(), operation)
	assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)
}

func TestCostCenterGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 1, 9, 0, 0, 0, time.UTC)
	costCenter := &CostCenter{ID: "cc-1", TenantID: "tenant-1", Code: "OPS", Name: "Operations", IsActive: true}

	t.Run("get not found", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(gorm.ErrRecordNotFound)))
		got, err := repo.GetByID(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "cost center not found")
	})

	t.Run("get wraps query error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(assert.AnError)))
		got, err := repo.GetByID(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "get cost center")
	})

	t.Run("list wraps query error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(assert.AnError)))
		got, err := repo.List(ctx, "tenant_schema", "tenant-1", false)
		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "list cost centers")
	})

	t.Run("create wraps create error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunCreateError(assert.AnError)))
		err := repo.Create(ctx, "tenant_schema", costCenter)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "create cost center")
	})

	t.Run("update wraps update error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateError(assert.AnError)))
		err := repo.Update(ctx, "tenant_schema", costCenter)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "update cost center")
	})

	t.Run("update reports missing cost center", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunUpdateRows(0)))
		err := repo.Update(ctx, "tenant_schema", costCenter)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cost center not found")
	})

	t.Run("delete rejects children", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
			counts: []accountingCountResult{{value: 2}},
		})))
		err := repo.Delete(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot delete cost center with 2 children")
	})

	t.Run("delete rejects invalid schema", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t))
		err := repo.Delete(ctx, "bad.schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "qualify cost center tables")
		assert.Contains(t, err.Error(), "invalid SQL identifier")
	})

	t.Run("delete wraps child count error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
			counts: []accountingCountResult{{err: assert.AnError}},
		})))
		err := repo.Delete(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "check children")
	})

	t.Run("delete wraps allocation count error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
			counts: []accountingCountResult{{value: 0}, {err: assert.AnError}},
		})))
		err := repo.Delete(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "check allocations")
	})

	t.Run("delete rejects allocations", func(t *testing.T) {
		var queries []string
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunFixtures(accountingDryRunFixture{
			counts: []accountingCountResult{{value: 0}, {value: 3}},
		}), withAccountingDryRunCapturedQueries(&queries)))
		err := repo.Delete(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot delete cost center with 3 allocations")
		require.Len(t, queries, 2)
		assert.Contains(t, queries[0], `FROM "tenant_schema"."cost_centers"`)
		assert.Contains(t, queries[1], `FROM "tenant_schema"."cost_allocations"`)
	})

	t.Run("delete wraps delete error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t,
			withAccountingDryRunFixtures(accountingDryRunFixture{counts: []accountingCountResult{{value: 0}, {value: 0}}}),
			withAccountingDryRunExecError(assert.AnError),
		))
		err := repo.Delete(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "delete cost center")
	})

	t.Run("delete reports missing cost center", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t,
			withAccountingDryRunFixtures(accountingDryRunFixture{counts: []accountingCountResult{{value: 0}, {value: 0}}}),
			withAccountingDryRunExecRows(0),
		))
		err := repo.Delete(ctx, "tenant_schema", "tenant-1", "cc-1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cost center not found")
	})

	t.Run("get expenses wraps query error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(assert.AnError)))
		total, err := repo.GetExpensesByPeriod(ctx, "tenant_schema", "tenant-1", "cc-1", now, now)
		require.Error(t, err)
		assert.True(t, total.IsZero())
		assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)
		assert.Contains(t, err.Error(), "get expenses")
	})

	t.Run("create allocation wraps create error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunCreateError(assert.AnError)))
		err := repo.CreateAllocation(ctx, "tenant_schema", &CostAllocation{
			TenantID:           "tenant-1",
			CostCenterID:       "cc-1",
			JournalEntryLineID: "line-1",
			Amount:             decimal.NewFromInt(1),
			AllocationDate:     now,
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "create cost allocation")
	})

	t.Run("list allocations wraps query error", func(t *testing.T) {
		repo := NewCostCenterGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunQueryError(assert.AnError)))
		allocations, err := repo.ListAllocations(ctx, "tenant_schema", "tenant-1", CostAllocationFilters{})
		require.Error(t, err)
		assert.Nil(t, allocations)
		assert.ErrorIs(t, err, gorm.ErrDryRunModeUnsupported)
		assert.Contains(t, err.Error(), "list cost allocations")
	})
}
