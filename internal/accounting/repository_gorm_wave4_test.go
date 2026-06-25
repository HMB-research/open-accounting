package accounting

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestGORMRepositoryWave4NilDatabaseMethodGuards(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	now := time.Date(2026, time.June, 25, 12, 0, 0, 0, time.UTC)

	account := &Account{ID: "account-1", TenantID: "tenant-1", Code: "1000", Name: "Cash", AccountType: AccountTypeAsset}
	entry := &JournalEntry{ID: "entry-1", TenantID: "tenant-1", EntryDate: now, Status: StatusDraft}
	template := &JournalEntryTemplate{ID: "template-1", TenantID: "tenant-1", Name: "Monthly accrual"}

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "GetAccountByID",
			run: func(t *testing.T) error {
				got, err := repo.GetAccountByID(ctx, "tenant_schema", "tenant-1", account.ID)
				if got != nil {
					t.Fatalf("GetAccountByID() account = %#v, want nil", got)
				}
				return err
			},
		},
		{
			name: "ListAccounts",
			run: func(t *testing.T) error {
				got, err := repo.ListAccounts(ctx, "tenant_schema", "tenant-1", true)
				if got != nil {
					t.Fatalf("ListAccounts() accounts = %#v, want nil", got)
				}
				return err
			},
		},
		{name: "CreateAccount", run: func(t *testing.T) error { return repo.CreateAccount(ctx, "tenant_schema", account) }},
		{name: "UpdateAccount", run: func(t *testing.T) error { return repo.UpdateAccount(ctx, "tenant_schema", account) }},
		{
			name: "GetJournalEntryByID",
			run: func(t *testing.T) error {
				got, err := repo.GetJournalEntryByID(ctx, "tenant_schema", "tenant-1", entry.ID)
				if got != nil {
					t.Fatalf("GetJournalEntryByID() entry = %#v, want nil", got)
				}
				return err
			},
		},
		{
			name: "GetJournalEntryBySource",
			run: func(t *testing.T) error {
				got, err := repo.GetJournalEntryBySource(ctx, "tenant_schema", "tenant-1", "IMPORT", "source-1")
				if got != nil {
					t.Fatalf("GetJournalEntryBySource() entry = %#v, want nil", got)
				}
				return err
			},
		},
		{
			name: "ListJournalEntries",
			run: func(t *testing.T) error {
				got, err := repo.ListJournalEntries(ctx, "tenant_schema", "tenant-1", 10)
				if got != nil {
					t.Fatalf("ListJournalEntries() entries = %#v, want nil", got)
				}
				return err
			},
		},
		{name: "CreateJournalEntryTemplate", run: func(t *testing.T) error { return repo.CreateJournalEntryTemplate(ctx, "tenant_schema", template) }},
		{
			name: "ListJournalEntryTemplates",
			run: func(t *testing.T) error {
				got, err := repo.ListJournalEntryTemplates(ctx, "tenant_schema", "tenant-1", true)
				if got != nil {
					t.Fatalf("ListJournalEntryTemplates() templates = %#v, want nil", got)
				}
				return err
			},
		},
		{
			name: "GetJournalEntryTemplateByID",
			run: func(t *testing.T) error {
				got, err := repo.GetJournalEntryTemplateByID(ctx, "tenant_schema", "tenant-1", template.ID)
				if got != nil {
					t.Fatalf("GetJournalEntryTemplateByID() template = %#v, want nil", got)
				}
				return err
			},
		},
		{
			name: "GetDueJournalEntryTemplateIDs",
			run: func(t *testing.T) error {
				got, err := repo.GetDueJournalEntryTemplateIDs(ctx, "tenant_schema", "tenant-1", now)
				if got != nil {
					t.Fatalf("GetDueJournalEntryTemplateIDs() ids = %#v, want nil", got)
				}
				return err
			},
		},
		{name: "UpdateJournalEntryTemplateAfterGeneration", run: func(t *testing.T) error {
			return repo.UpdateJournalEntryTemplateAfterGeneration(ctx, "tenant_schema", "tenant-1", template.ID, now.AddDate(0, 1, 0), now)
		}},
		{
			name: "countJournalEntryTemplateLines",
			run: func(t *testing.T) error {
				got, err := repo.countJournalEntryTemplateLines(ctx, "tenant_schema", []string{template.ID})
				if got != nil {
					t.Fatalf("countJournalEntryTemplateLines() counts = %#v, want nil", got)
				}
				return err
			},
		},
		{name: "CreateJournalEntry", run: func(t *testing.T) error { return repo.CreateJournalEntry(ctx, "tenant_schema", entry) }},
		{name: "UpdateJournalEntryStatus", run: func(t *testing.T) error {
			return repo.UpdateJournalEntryStatus(ctx, "tenant_schema", "tenant-1", entry.ID, StatusPosted, "user-1")
		}},
		{
			name: "GetAccountBalance",
			run: func(t *testing.T) error {
				_, err := repo.GetAccountBalance(ctx, "tenant_schema", "tenant-1", account.ID, now)
				return err
			},
		},
		{
			name: "GetTrialBalance",
			run: func(t *testing.T) error {
				got, err := repo.GetTrialBalance(ctx, "tenant_schema", "tenant-1", now)
				if got != nil {
					t.Fatalf("GetTrialBalance() balances = %#v, want nil", got)
				}
				return err
			},
		},
		{
			name: "GetPeriodBalances",
			run: func(t *testing.T) error {
				got, err := repo.GetPeriodBalances(ctx, "tenant_schema", "tenant-1", now.AddDate(0, -1, 0), now)
				if got != nil {
					t.Fatalf("GetPeriodBalances() balances = %#v, want nil", got)
				}
				return err
			},
		},
		{name: "VoidJournalEntry", run: func(t *testing.T) error {
			return repo.VoidJournalEntry(ctx, "tenant_schema", "tenant-1", entry.ID, "user-1", "mistake", &JournalEntry{})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if err == nil {
				t.Fatal("expected not-configured error")
			}
			if !strings.Contains(err.Error(), "accounting repository database is not configured") {
				t.Fatalf("error = %q, want accounting repository database is not configured", err)
			}
		})
	}
}

func TestGORMRepositoryWave4ScanBackedBalances(t *testing.T) {
	ctx := context.Background()
	asOfDate := time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC)
	tenantID := "tenant-1"
	liabilityAccount := models.Account{
		ID:          "liability-1",
		TenantID:    tenantID,
		Code:        "2100",
		Name:        "Accounts payable",
		AccountType: models.AccountTypeLiability,
		IsActive:    true,
	}

	repo := NewGORMRepository(newAccountingDryRunDB(t,
		withAccountingDryRunFixtures(accountingDryRunFixture{accounts: []models.Account{liabilityAccount}}),
		withAccountingDryRunScanRowsWave4(accountingDryRunRowSetWave4{
			columns: []string{"debit_sum", "credit_sum"},
			values:  [][]driver.Value{{"25.00", "125.00"}},
		}),
	))

	balance, err := repo.GetAccountBalance(ctx, "tenant_schema", tenantID, liabilityAccount.ID, asOfDate)
	if err != nil {
		t.Fatalf("GetAccountBalance() error = %v", err)
	}
	if !balance.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("GetAccountBalance() = %s, want 100", balance)
	}

	repo = NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunScanRowsWave4(
		accountingDryRunRowSetWave4{
			columns: []string{"account_id", "account_code", "account_name", "account_type", "total_debits", "total_credits", "net_balance"},
			values: [][]driver.Value{{
				"expense-1", "5000", "Supplies", string(AccountTypeExpense), "75.00", "10.00", "65.00",
			}},
		},
		accountingDryRunRowSetWave4{
			columns: []string{"account_id", "account_code", "account_name", "account_type", "total_debits", "total_credits", "net_balance"},
			values: [][]driver.Value{{
				"revenue-1", "4000", "Sales revenue", string(AccountTypeRevenue), "5.00", "95.00", "90.00",
			}},
		},
	)))

	trialBalance, err := repo.GetTrialBalance(ctx, "tenant_schema", tenantID, asOfDate)
	if err != nil {
		t.Fatalf("GetTrialBalance() error = %v", err)
	}
	if len(trialBalance) != 1 || trialBalance[0].AccountCode != "5000" || !trialBalance[0].NetBalance.Equal(decimal.NewFromInt(65)) {
		t.Fatalf("GetTrialBalance() = %#v, want supplies balance", trialBalance)
	}

	periodBalances, err := repo.GetPeriodBalances(ctx, "tenant_schema", tenantID, asOfDate.AddDate(0, -1, 0), asOfDate)
	if err != nil {
		t.Fatalf("GetPeriodBalances() error = %v", err)
	}
	if len(periodBalances) != 1 || periodBalances[0].AccountCode != "4000" || !periodBalances[0].NetBalance.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("GetPeriodBalances() = %#v, want sales revenue balance", periodBalances)
	}
}

func TestGORMRepositoryWave4CreateJournalEntryScanSuccess(t *testing.T) {
	ctx := context.Background()
	entry := &JournalEntry{
		TenantID:    "tenant-1",
		EntryDate:   time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC),
		Description: "Wave 4 accrual",
		Status:      StatusDraft,
		CreatedBy:   "user-1",
		Lines: []JournalEntryLine{
			{
				AccountID:    "expense-1",
				Description:  "Debit",
				DebitAmount:  decimal.NewFromInt(50),
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.NewFromInt(50),
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    "payable-1",
				Description:  "Credit",
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromInt(50),
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   decimal.NewFromInt(50),
			},
		},
	}
	repo := NewGORMRepository(newAccountingDryRunDB(t, withAccountingDryRunScanRowsWave4(accountingDryRunRowSetWave4{
		columns: []string{"sequence"},
		values:  [][]driver.Value{{int64(42)}},
	})))

	if err := repo.CreateJournalEntry(ctx, "tenant_schema", entry); err != nil {
		t.Fatalf("CreateJournalEntry() error = %v", err)
	}
	if entry.ID == "" || entry.CreatedAt.IsZero() {
		t.Fatalf("CreateJournalEntry() did not populate ID/CreatedAt: %#v", entry)
	}
	if entry.EntryNumber != "JE-00042" {
		t.Fatalf("EntryNumber = %q, want JE-00042", entry.EntryNumber)
	}
	for i, line := range entry.Lines {
		if line.TenantID != entry.TenantID || line.JournalEntryID != entry.ID || line.ID == "" {
			t.Fatalf("line %d after create = %#v, want tenant, journal entry, and id populated", i, line)
		}
	}
}

type accountingDryRunRowSetWave4 struct {
	columns []string
	values  [][]driver.Value
}

var accountingDryRunRowsWave4ID uint64
var accountingDryRunRowsWave4DriverOnce sync.Once
var accountingDryRunRowsWave4Mu sync.Mutex
var accountingDryRunRowsWave4ByDSN = map[string]accountingDryRunRowSetWave4{}

func withAccountingDryRunScanRowsWave4(rowSets ...accountingDryRunRowSetWave4) accountingDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()

		var index int
		err := db.Callback().Row().After("gorm:row").Register(accountingDryRunCallbackName("wave4_scan_rows"), func(tx *gorm.DB) {
			if index >= len(rowSets) {
				tx.AddError(fmt.Errorf("missing accounting wave4 dry-run row set %d", index))
				return
			}
			rowSet := rowSets[index]
			index++
			tx.Statement.Dest = newAccountingDryRunSQLRowsWave4(t, rowSet)
			tx.RowsAffected = int64(len(rowSet.values))
		})
		if err != nil {
			t.Fatalf("register scan rows callback: %v", err)
		}
	}
}

func newAccountingDryRunSQLRowsWave4(t *testing.T, rowSet accountingDryRunRowSetWave4) *sql.Rows {
	t.Helper()

	accountingDryRunRowsWave4DriverOnce.Do(func() {
		sql.Register("accounting_wave4_dryrun_rows", accountingDryRunRowsWave4Driver{})
	})

	dsn := fmt.Sprintf("accounting-wave4-dry-run-rows-%d", atomic.AddUint64(&accountingDryRunRowsWave4ID, 1))
	accountingDryRunRowsWave4Mu.Lock()
	accountingDryRunRowsWave4ByDSN[dsn] = rowSet
	accountingDryRunRowsWave4Mu.Unlock()

	db, err := sql.Open("accounting_wave4_dryrun_rows", dsn)
	if err != nil {
		t.Fatalf("open accounting wave4 dry-run rows: %v", err)
	}
	rows, err := db.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Fatalf("query accounting wave4 dry-run rows: %v", err)
	}

	t.Cleanup(func() {
		_ = rows.Close()
		_ = db.Close()
		accountingDryRunRowsWave4Mu.Lock()
		delete(accountingDryRunRowsWave4ByDSN, dsn)
		accountingDryRunRowsWave4Mu.Unlock()
	})

	return rows
}

type accountingDryRunRowsWave4Driver struct{}

func (accountingDryRunRowsWave4Driver) Open(name string) (driver.Conn, error) {
	return accountingDryRunRowsWave4Conn{dsn: name}, nil
}

type accountingDryRunRowsWave4Conn struct {
	dsn string
}

func (accountingDryRunRowsWave4Conn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("accounting wave4 dry-run rows do not prepare statements")
}

func (accountingDryRunRowsWave4Conn) Close() error {
	return nil
}

func (accountingDryRunRowsWave4Conn) Begin() (driver.Tx, error) {
	return nil, fmt.Errorf("accounting wave4 dry-run rows do not begin transactions")
}

func (c accountingDryRunRowsWave4Conn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	accountingDryRunRowsWave4Mu.Lock()
	rowSet, ok := accountingDryRunRowsWave4ByDSN[c.dsn]
	accountingDryRunRowsWave4Mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("accounting wave4 dry-run row set %q not found", c.dsn)
	}
	return &accountingDryRunSQLRowsWave4{
		columns: append([]string(nil), rowSet.columns...),
		values:  append([][]driver.Value(nil), rowSet.values...),
	}, nil
}

type accountingDryRunSQLRowsWave4 struct {
	columns []string
	values  [][]driver.Value
	index   int
}

func (r *accountingDryRunSQLRowsWave4) Columns() []string {
	return append([]string(nil), r.columns...)
}

func (*accountingDryRunSQLRowsWave4) Close() error {
	return nil
}

func (r *accountingDryRunSQLRowsWave4) Next(dest []driver.Value) error {
	if r.index >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.index])
	r.index++
	return nil
}
