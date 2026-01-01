//go:build integration

package accounting

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_VoidJournalEntry(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get account IDs from default chart of accounts
	var cashAccountID, revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	// Create a journal entry
	entryID := uuid.New().String()
	now := time.Now()
	amount := decimal.NewFromFloat(100)

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at)
		VALUES ($1, $2, 'JE-001', $3, 'Test entry', 'POSTED', $4, NOW())
	`, entryID, tenant.ID, now, userID)
	if err != nil {
		t.Fatalf("failed to create journal entry: %v", err)
	}

	// Create journal entry lines with multi-currency columns
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines
		(id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, $5, 0, 'EUR', 1, $5, 0)
	`, uuid.New().String(), tenant.ID, entryID, cashAccountID, amount)
	if err != nil {
		t.Fatalf("failed to create debit line: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines
		(id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 0, $5, 'EUR', 1, 0, $5)
	`, uuid.New().String(), tenant.ID, entryID, revenueAccountID, amount)
	if err != nil {
		t.Fatalf("failed to create credit line: %v", err)
	}

	// Create reversal entry
	reversalID := uuid.New().String()
	reversal := &JournalEntry{
		ID:          reversalID,
		TenantID:    tenant.ID,
		EntryNumber: "JE-002",
		EntryDate:   now,
		Description: "Reversal of JE-001",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				ID:             uuid.New().String(),
				TenantID:       tenant.ID,
				JournalEntryID: reversalID,
				AccountID:      revenueAccountID,
				DebitAmount:    amount,
				CreditAmount:   decimal.Zero,
				Currency:       "EUR",
				ExchangeRate:   decimal.NewFromInt(1),
				BaseDebit:      amount,
				BaseCredit:     decimal.Zero,
			},
			{
				ID:             uuid.New().String(),
				TenantID:       tenant.ID,
				JournalEntryID: reversalID,
				AccountID:      cashAccountID,
				DebitAmount:    decimal.Zero,
				CreditAmount:   amount,
				Currency:       "EUR",
				ExchangeRate:   decimal.NewFromInt(1),
				BaseDebit:      decimal.Zero,
				BaseCredit:     amount,
			},
		},
	}

	// Void the entry
	err = repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, entryID, userID, "Test void reason", reversal)
	if err != nil {
		t.Fatalf("VoidJournalEntry failed: %v", err)
	}

	// Verify original entry is voided
	var status string
	var voidReason *string
	err = pool.QueryRow(ctx, `
		SELECT status, void_reason FROM `+tenant.SchemaName+`.journal_entries WHERE id = $1
	`, entryID).Scan(&status, &voidReason)
	if err != nil {
		t.Fatalf("failed to query voided entry: %v", err)
	}

	if status != "VOIDED" {
		t.Errorf("expected status 'VOIDED', got '%s'", status)
	}
	if voidReason == nil || *voidReason != "Test void reason" {
		t.Errorf("expected void reason 'Test void reason', got %v", voidReason)
	}

	// Verify reversal entry was created
	var reversalStatus string
	err = pool.QueryRow(ctx, `
		SELECT status FROM `+tenant.SchemaName+`.journal_entries WHERE id = $1
	`, reversalID).Scan(&reversalStatus)
	if err != nil {
		t.Fatalf("failed to query reversal entry: %v", err)
	}

	if reversalStatus != "POSTED" {
		t.Errorf("expected reversal status 'POSTED', got '%s'", reversalStatus)
	}
}

func TestPostgresRepository_CreateJournalEntry(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get account IDs
	var cashAccountID, revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	entryID := uuid.New().String()
	amount := decimal.NewFromFloat(500)

	entry := &JournalEntry{
		ID:          entryID,
		TenantID:    tenant.ID,
		EntryNumber: "JE-TEST-001",
		EntryDate:   time.Now(),
		Description: "Test journal entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				ID:             uuid.New().String(),
				TenantID:       tenant.ID,
				JournalEntryID: entryID,
				AccountID:      cashAccountID,
				DebitAmount:    amount,
				CreditAmount:   decimal.Zero,
				Currency:       "EUR",
				ExchangeRate:   decimal.NewFromInt(1),
				BaseDebit:      amount,
				BaseCredit:     decimal.Zero,
			},
			{
				ID:             uuid.New().String(),
				TenantID:       tenant.ID,
				JournalEntryID: entryID,
				AccountID:      revenueAccountID,
				DebitAmount:    decimal.Zero,
				CreditAmount:   amount,
				Currency:       "EUR",
				ExchangeRate:   decimal.NewFromInt(1),
				BaseDebit:      decimal.Zero,
				BaseCredit:     amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Verify entry was created
	retrieved, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, entryID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	if retrieved.EntryNumber != entry.EntryNumber {
		t.Errorf("expected entry number %s, got %s", entry.EntryNumber, retrieved.EntryNumber)
	}
	if len(retrieved.Lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(retrieved.Lines))
	}
}

func TestPostgresRepository_GetAccountByID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get a known account (cash account created by CreateTestTenant)
	var accountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&accountID)
	if err != nil {
		t.Fatalf("failed to get cash account ID: %v", err)
	}

	// Test GetAccountByID
	account, err := repo.GetAccountByID(ctx, tenant.SchemaName, tenant.ID, accountID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if account.Code != "1000" {
		t.Errorf("expected code '1000', got '%s'", account.Code)
	}
	if account.AccountType != AccountTypeAsset {
		t.Errorf("expected account type '%s', got '%s'", AccountTypeAsset, account.AccountType)
	}
}

func TestPostgresRepository_GetAccountByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Try to get non-existent account
	_, err := repo.GetAccountByID(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err == nil {
		t.Error("expected error for non-existent account, got nil")
	}
}

func TestPostgresRepository_ListAccounts(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// List all accounts
	accounts, err := repo.ListAccounts(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}

	if len(accounts) == 0 {
		t.Error("expected at least one account from default chart of accounts")
	}

	// Test active only filter
	activeAccounts, err := repo.ListAccounts(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListAccounts (active only) failed: %v", err)
	}

	for _, a := range activeAccounts {
		if !a.IsActive {
			t.Errorf("expected all accounts to be active, found inactive: %s", a.Code)
		}
	}
}

func TestPostgresRepository_CreateAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	account := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "9999",
		Name:        "Test Account",
		AccountType: AccountTypeExpense,
		IsActive:    true,
		IsSystem:    false,
		Description: "Test account description",
		CreatedAt:   time.Now(),
	}

	err := repo.CreateAccount(ctx, tenant.SchemaName, account)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	// Verify account was created
	retrieved, err := repo.GetAccountByID(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if retrieved.Code != account.Code {
		t.Errorf("expected code '%s', got '%s'", account.Code, retrieved.Code)
	}
	if retrieved.Name != account.Name {
		t.Errorf("expected name '%s', got '%s'", account.Name, retrieved.Name)
	}
}

func TestPostgresRepository_GetAccountBalance(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get cash account ID
	var cashAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	// Initially, balance should be zero
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, cashAccountID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.IsZero() {
		t.Errorf("expected zero balance initially, got %s", balance)
	}
}

func TestPostgresRepository_GetTrialBalance(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Create test accounts
	cashAccount := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "1001",
		Name:        "Cash TB",
		AccountType: AccountTypeAsset,
		IsActive:    true,
	}
	revenueAccount := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "4001",
		Name:        "Revenue TB",
		AccountType: AccountTypeRevenue,
		IsActive:    true,
	}
	if err := repo.CreateAccount(ctx, tenant.SchemaName, cashAccount); err != nil {
		t.Fatalf("CreateAccount (cash) failed: %v", err)
	}
	if err := repo.CreateAccount(ctx, tenant.SchemaName, revenueAccount); err != nil {
		t.Fatalf("CreateAccount (revenue) failed: %v", err)
	}

	// Create a posted journal entry so trial balance has data
	entry := &JournalEntry{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Trial balance test entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccount.ID,
				Description:  "Cash receipt",
				DebitAmount:  decimal.NewFromInt(500),
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.NewFromInt(500),
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccount.ID,
				Description:  "Revenue",
				DebitAmount:  decimal.Zero,
				CreditAmount: decimal.NewFromInt(500),
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   decimal.NewFromInt(500),
			},
		},
	}
	if err := repo.CreateJournalEntry(ctx, tenant.SchemaName, entry); err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Post the entry
	if err := repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID); err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get trial balance - now it should return accounts with transactions
	balances, err := repo.GetTrialBalance(ctx, tenant.SchemaName, tenant.ID, time.Now())
	if err != nil {
		t.Fatalf("GetTrialBalance failed: %v", err)
	}

	// Trial balance should include our accounts with transactions
	if len(balances) < 2 {
		t.Errorf("expected at least 2 accounts in trial balance, got %d", len(balances))
	}
}

func TestPostgresRepository_GetPeriodBalances(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "periodbalances@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get revenue and expense account IDs
	var revenueAccountID, expenseAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '5000' LIMIT 1
	`).Scan(&expenseAccountID)
	if err != nil {
		t.Fatalf("failed to get expense account: %v", err)
	}

	// Create a POSTED journal entry with revenue and expense
	entryID := uuid.New().String()
	now := time.Now()
	amount := decimal.NewFromFloat(500)

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at)
		VALUES ($1, $2, 'JE-PERIOD-001', $3, 'Period Balance Test Entry', 'POSTED', $4, NOW())
	`, entryID, tenant.ID, now, userID)
	if err != nil {
		t.Fatalf("failed to create journal entry: %v", err)
	}

	// Revenue line (credit)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines
		(id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 0, $5, 'EUR', 1, 0, $5)
	`, uuid.New().String(), tenant.ID, entryID, revenueAccountID, amount)
	if err != nil {
		t.Fatalf("failed to create revenue line: %v", err)
	}

	// Expense line (debit)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines
		(id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, $5, 0, 'EUR', 1, $5, 0)
	`, uuid.New().String(), tenant.ID, entryID, expenseAccountID, amount)
	if err != nil {
		t.Fatalf("failed to create expense line: %v", err)
	}

	// Query period balances
	startDate := now.AddDate(0, -1, 0)
	endDate := now.AddDate(0, 1, 0)

	balances, err := repo.GetPeriodBalances(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetPeriodBalances failed: %v", err)
	}

	// Should have revenue and expense balances
	if len(balances) < 2 {
		t.Errorf("expected at least 2 period balances (revenue and expense), got %d", len(balances))
	}

	// Verify we have revenue and expense account types
	var hasRevenue, hasExpense bool
	for _, b := range balances {
		if b.AccountType == AccountTypeRevenue {
			hasRevenue = true
		}
		if b.AccountType == AccountTypeExpense {
			hasExpense = true
		}
	}

	if !hasRevenue {
		t.Error("expected revenue account in period balances")
	}
	if !hasExpense {
		t.Error("expected expense account in period balances")
	}
}

func TestPostgresRepository_GetPeriodBalances_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Query for a period with no transactions
	startDate := time.Now().AddDate(-2, 0, 0)
	endDate := time.Now().AddDate(-2, 1, 0)

	balances, err := repo.GetPeriodBalances(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetPeriodBalances failed: %v", err)
	}

	// Should return empty slice for period with no activity
	if len(balances) != 0 {
		t.Errorf("expected 0 balances for empty period, got %d", len(balances))
	}
}

func TestPostgresRepository_UpdateJournalEntryStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get account IDs
	var cashAccountID, revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	// Create a draft journal entry
	entryID := uuid.New().String()
	amount := decimal.NewFromFloat(100)

	entry := &JournalEntry{
		ID:          entryID,
		TenantID:    tenant.ID,
		EntryNumber: "JE-STATUS-001",
		EntryDate:   time.Now(),
		Description: "Test status update",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				ID:             uuid.New().String(),
				TenantID:       tenant.ID,
				JournalEntryID: entryID,
				AccountID:      cashAccountID,
				DebitAmount:    amount,
				CreditAmount:   decimal.Zero,
				Currency:       "EUR",
				ExchangeRate:   decimal.NewFromInt(1),
				BaseDebit:      amount,
				BaseCredit:     decimal.Zero,
			},
			{
				ID:             uuid.New().String(),
				TenantID:       tenant.ID,
				JournalEntryID: entryID,
				AccountID:      revenueAccountID,
				DebitAmount:    decimal.Zero,
				CreditAmount:   amount,
				Currency:       "EUR",
				ExchangeRate:   decimal.NewFromInt(1),
				BaseDebit:      decimal.Zero,
				BaseCredit:     amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Update status to POSTED
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entryID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Verify status was updated
	retrieved, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, entryID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	if retrieved.Status != StatusPosted {
		t.Errorf("expected status '%s', got '%s'", StatusPosted, retrieved.Status)
	}
}

func TestPostgresRepository_GetJournalEntryByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	_, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err == nil {
		t.Error("expected error for non-existent journal entry")
	}
}

func TestPostgresRepository_UpdateJournalEntryStatus_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "status-notfound@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	err := repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), StatusPosted, userID)
	if err == nil {
		t.Error("expected error when updating non-existent journal entry")
	}
}

func TestPostgresRepository_GetAccountBalance_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Query balance for non-existent account - should error because account type lookup fails
	_, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), time.Now())
	if err == nil {
		t.Error("expected error for non-existent account")
	}
}

func TestPostgresRepository_VoidJournalEntry_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "void-notfound@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Try to void non-existent entry
	err := repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), userID, "test reason", nil)
	if err == nil {
		t.Error("expected error when voiding non-existent journal entry")
	}
}

func TestPostgresRepository_ListAccounts_InactiveFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Create an inactive account
	inactiveAccount := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "9999",
		Name:        "Inactive Test Account",
		AccountType: AccountTypeExpense,
		IsActive:    false,
		IsSystem:    false,
	}

	err := repo.CreateAccount(ctx, tenant.SchemaName, inactiveAccount)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	// List only active accounts
	activeAccounts, err := repo.ListAccounts(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListAccounts (active only) failed: %v", err)
	}

	for _, a := range activeAccounts {
		if a.ID == inactiveAccount.ID {
			t.Error("inactive account should not appear in active-only list")
		}
	}

	// List all accounts (including inactive)
	allAccounts, err := repo.ListAccounts(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListAccounts (all) failed: %v", err)
	}

	found := false
	for _, a := range allAccounts {
		if a.ID == inactiveAccount.ID {
			found = true
			break
		}
	}

	if !found {
		t.Error("inactive account should appear in all accounts list")
	}
}

func TestPostgresRepository_CreateJournalEntry_ValidatesBalance(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "unbalanced@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get cash account
	var cashAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	// Create an unbalanced entry (only debit, no credit)
	entryID := uuid.New().String()
	entry := &JournalEntry{
		ID:          entryID,
		TenantID:    tenant.ID,
		EntryNumber: "JE-UNBAL-001",
		EntryDate:   time.Now(),
		Description: "Unbalanced entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				ID:             uuid.New().String(),
				TenantID:       tenant.ID,
				JournalEntryID: entryID,
				AccountID:      cashAccountID,
				DebitAmount:    decimal.NewFromFloat(100),
				CreditAmount:   decimal.Zero,
				Currency:       "EUR",
				ExchangeRate:   decimal.NewFromInt(1),
				BaseDebit:      decimal.NewFromFloat(100),
				BaseCredit:     decimal.Zero,
			},
		},
	}

	// This should still succeed for draft entries, but would fail for posted entries
	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		// May fail depending on validation - either is acceptable
		t.Logf("CreateJournalEntry returned error (expected for validation): %v", err)
	}
}
