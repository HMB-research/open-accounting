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

// TestCreateAccount_AutoGeneratesIDAndCreatedAt verifies that ID and CreatedAt
// are automatically populated when not provided
func TestCreateAccount_AutoGeneratesIDAndCreatedAt(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	account := &Account{
		TenantID:    tenant.ID,
		Code:        "AUTO-001",
		Name:        "Auto-Generated Account",
		AccountType: AccountTypeAsset,
		IsActive:    true,
		IsSystem:    false,
	}

	err := repo.CreateAccount(ctx, tenant.SchemaName, account)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	// Verify ID was auto-generated
	if account.ID == "" {
		t.Error("expected ID to be auto-generated")
	}

	// Verify CreatedAt was auto-generated
	if account.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be auto-generated")
	}

	// Verify account can be retrieved
	retrieved, err := repo.GetAccountByID(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if retrieved.Code != "AUTO-001" {
		t.Errorf("expected code AUTO-001, got %s", retrieved.Code)
	}
}

// TestCreateAccount_DuplicateCode verifies error handling for duplicate account codes
func TestCreateAccount_DuplicateCode(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	account1 := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "DUP-001",
		Name:        "First Account",
		AccountType: AccountTypeAsset,
		IsActive:    true,
	}

	err := repo.CreateAccount(ctx, tenant.SchemaName, account1)
	if err != nil {
		t.Fatalf("first CreateAccount failed: %v", err)
	}

	// Try to create duplicate
	account2 := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "DUP-001", // Same code
		Name:        "Duplicate Account",
		AccountType: AccountTypeAsset,
		IsActive:    true,
	}

	err = repo.CreateAccount(ctx, tenant.SchemaName, account2)
	if err == nil {
		t.Error("expected error for duplicate account code")
	}
}

// TestUpdateJournalEntryStatus_ToVoided tests that VOIDED status is rejected
func TestUpdateJournalEntryStatus_ToVoided(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "voided-status@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Try to update to VOIDED status (should fail)
	err := repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), StatusVoided, userID)
	if err == nil {
		t.Error("expected error when trying to update to VOIDED status")
	}

	expectedMsg := "use VoidJournalEntry method to void entries"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

// TestUpdateJournalEntryStatus_InvalidStatus tests invalid status transition
func TestUpdateJournalEntryStatus_InvalidStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "invalid-status@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Try to update to an invalid status
	err := repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), "INVALID", userID)
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

// TestUpdateJournalEntryStatus_AlreadyPosted tests that re-posting fails
func TestUpdateJournalEntryStatus_AlreadyPosted(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "repost@example.com")
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

	// Create and post a journal entry
	entryID := uuid.New().String()
	amount := decimal.NewFromFloat(100)

	entry := &JournalEntry{
		ID:          entryID,
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Test entry for re-post",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Post it
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entryID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("first UpdateJournalEntryStatus failed: %v", err)
	}

	// Try to post again - should fail because it's already posted
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entryID, StatusPosted, userID)
	if err == nil {
		t.Error("expected error when trying to re-post already posted entry")
	}
}

// TestVoidJournalEntry_DraftEntry tests that voiding a draft entry fails
func TestVoidJournalEntry_DraftEntry(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "void-draft@example.com")
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

	// Create a DRAFT journal entry
	entryID := uuid.New().String()
	amount := decimal.NewFromFloat(100)

	entry := &JournalEntry{
		ID:          entryID,
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Draft entry to void",
		Status:      StatusDraft, // Draft, not posted
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Try to void the draft entry - should fail
	reversal := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Reversal",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    revenueAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    cashAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, entryID, userID, "test void", reversal)
	if err == nil {
		t.Error("expected error when voiding a draft entry")
	}
}

// TestGetAccountBalance_CreditNormalAccount tests balance calculation for liability/equity/revenue
func TestGetAccountBalance_CreditNormalAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "creditnormal@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get revenue account (credit-normal)
	var revenueAccountID, cashAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	// Create and post a journal entry with revenue
	amount := decimal.NewFromFloat(500)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Revenue transaction",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get revenue account balance (should be positive since credits > debits for credit-normal)
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, revenueAccountID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.Equal(amount) {
		t.Errorf("expected balance %s, got %s", amount.String(), balance.String())
	}
}

// TestGetTrialBalance_Empty tests trial balance with no posted transactions
func TestGetTrialBalance_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get trial balance with no posted transactions
	balances, err := repo.GetTrialBalance(ctx, tenant.SchemaName, tenant.ID, time.Now())
	if err != nil {
		t.Fatalf("GetTrialBalance failed: %v", err)
	}

	// Should return empty slice when no posted transactions
	if len(balances) != 0 {
		t.Errorf("expected 0 balances for tenant with no transactions, got %d", len(balances))
	}
}

// TestGetTrialBalance_FutureDate tests trial balance excludes future transactions
func TestGetTrialBalance_FutureDate(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "futuredate@example.com")
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

	// Create a posted entry dated in the future
	futureDate := time.Now().AddDate(1, 0, 0) // 1 year in future
	amount := decimal.NewFromFloat(1000)

	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   futureDate,
		Description: "Future dated entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get trial balance as of today - should not include the future entry
	balances, err := repo.GetTrialBalance(ctx, tenant.SchemaName, tenant.ID, time.Now())
	if err != nil {
		t.Fatalf("GetTrialBalance failed: %v", err)
	}

	// Should be empty since only transaction is future-dated
	if len(balances) != 0 {
		t.Errorf("expected 0 balances when only future transactions exist, got %d", len(balances))
	}
}

// TestCreateJournalEntry_AutoGeneratesIDAndCreatedAt verifies auto-generation
func TestCreateJournalEntry_AutoGeneratesIDAndCreatedAt(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "autogen@example.com")
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

	amount := decimal.NewFromFloat(100)

	// Create entry without ID or CreatedAt
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Auto-generated ID test",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Verify ID was auto-generated
	if entry.ID == "" {
		t.Error("expected ID to be auto-generated")
	}

	// Verify CreatedAt was auto-generated
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be auto-generated")
	}

	// Verify EntryNumber was auto-generated
	if entry.EntryNumber == "" {
		t.Error("expected EntryNumber to be auto-generated")
	}

	// Verify lines have IDs populated
	for i, line := range entry.Lines {
		if line.ID == "" {
			t.Errorf("expected line %d to have auto-generated ID", i)
		}
	}
}

// TestCreateJournalEntry_LineAutoGeneration verifies line IDs are auto-generated
func TestCreateJournalEntry_LineAutoGeneration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "lineautogen@example.com")
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

	amount := decimal.NewFromFloat(100)

	// Create entry with lines that have no IDs
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Line auto-gen test",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				// No ID - should be auto-generated
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				// No ID - should be auto-generated
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Retrieve and verify lines have proper data
	retrieved, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	if len(retrieved.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(retrieved.Lines))
	}

	for i, line := range retrieved.Lines {
		if line.ID == "" {
			t.Errorf("line %d should have an ID", i)
		}
		if line.TenantID != tenant.ID {
			t.Errorf("line %d should have tenant ID %s, got %s", i, tenant.ID, line.TenantID)
		}
		if line.JournalEntryID != entry.ID {
			t.Errorf("line %d should have journal entry ID %s, got %s", i, entry.ID, line.JournalEntryID)
		}
	}
}

// TestGetAccountBalance_DebitNormalAccount tests balance for asset/expense accounts
func TestGetAccountBalance_DebitNormalAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "debitnormal@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get cash account (debit-normal asset)
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

	// Create and post a journal entry
	amount := decimal.NewFromFloat(750)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Cash receipt",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get cash account balance (debit-normal: debits - credits)
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, cashAccountID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.Equal(amount) {
		t.Errorf("expected balance %s, got %s", amount.String(), balance.String())
	}
}

// TestGetAccountBalance_MultipleTransactions tests cumulative balance
func TestGetAccountBalance_MultipleTransactions(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "multitx@example.com")
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

	// Create multiple transactions
	amounts := []decimal.Decimal{
		decimal.NewFromFloat(100),
		decimal.NewFromFloat(250),
		decimal.NewFromFloat(175),
	}

	expectedTotal := decimal.Zero
	for _, amount := range amounts {
		entry := &JournalEntry{
			TenantID:    tenant.ID,
			EntryDate:   time.Now(),
			Description: "Multi-tx test",
			Status:      StatusDraft,
			CreatedBy:   userID,
			Lines: []JournalEntryLine{
				{
					AccountID:    cashAccountID,
					DebitAmount:  amount,
					CreditAmount: decimal.Zero,
					Currency:     "EUR",
					ExchangeRate: decimal.NewFromInt(1),
					BaseDebit:    amount,
					BaseCredit:   decimal.Zero,
				},
				{
					AccountID:    revenueAccountID,
					DebitAmount:  decimal.Zero,
					CreditAmount: amount,
					Currency:     "EUR",
					ExchangeRate: decimal.NewFromInt(1),
					BaseDebit:    decimal.Zero,
					BaseCredit:   amount,
				},
			},
		}

		err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
		if err != nil {
			t.Fatalf("CreateJournalEntry failed: %v", err)
		}

		err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
		if err != nil {
			t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
		}

		expectedTotal = expectedTotal.Add(amount)
	}

	// Get balance
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, cashAccountID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.Equal(expectedTotal) {
		t.Errorf("expected balance %s, got %s", expectedTotal.String(), balance.String())
	}
}

// TestListAccounts_EmptyTenant tests listing accounts from an empty tenant
func TestListAccounts_EmptyTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Default chart of accounts should have accounts
	accounts, err := repo.ListAccounts(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}

	// Should have default chart of accounts
	if len(accounts) == 0 {
		t.Error("expected default chart of accounts, got empty list")
	}
}

// TestVoidJournalEntry_AlreadyVoided tests that voiding an already voided entry fails
func TestVoidJournalEntry_AlreadyVoided(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "alreadyvoided@example.com")
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

	// Create and post a journal entry
	amount := decimal.NewFromFloat(100)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Entry to void twice",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// First void
	reversal1 := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "First reversal",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    revenueAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    cashAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, entry.ID, userID, "first void", reversal1)
	if err != nil {
		t.Fatalf("first VoidJournalEntry failed: %v", err)
	}

	// Try to void again - should fail
	reversal2 := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Second reversal",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    revenueAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    cashAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, entry.ID, userID, "second void", reversal2)
	if err == nil {
		t.Error("expected error when voiding already voided entry")
	}
}

// TestGetPeriodBalances_OutsidePeriod tests that transactions outside period are excluded
func TestGetPeriodBalances_OutsidePeriod(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "outsideperiod@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get account IDs
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

	// Create a POSTED journal entry dated outside our query period
	amount := decimal.NewFromFloat(500)
	pastDate := time.Now().AddDate(-1, 0, 0) // 1 year ago

	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   pastDate,
		Description: "Old transaction",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    expenseAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Query for current month only - should not include the old transaction
	startDate := time.Now().AddDate(0, 0, -7) // 7 days ago
	endDate := time.Now().AddDate(0, 0, 7)    // 7 days from now

	balances, err := repo.GetPeriodBalances(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetPeriodBalances failed: %v", err)
	}

	// Should be empty since transaction is outside the period
	if len(balances) != 0 {
		t.Errorf("expected 0 balances for period with no transactions, got %d", len(balances))
	}
}

// TestCreateAccount_WithParentID tests creating an account with a parent
func TestCreateAccount_WithParentID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get an existing account to use as parent
	var parentID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&parentID)
	if err != nil {
		t.Fatalf("failed to get parent account: %v", err)
	}

	// Create child account
	childAccount := &Account{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		Code:        "1000-01",
		Name:        "Child Cash Account",
		AccountType: AccountTypeAsset,
		ParentID:    &parentID,
		IsActive:    true,
		IsSystem:    false,
	}

	err = repo.CreateAccount(ctx, tenant.SchemaName, childAccount)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	// Verify parent ID was stored
	retrieved, err := repo.GetAccountByID(ctx, tenant.SchemaName, tenant.ID, childAccount.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if retrieved.ParentID == nil {
		t.Error("expected parent ID to be set")
	} else if *retrieved.ParentID != parentID {
		t.Errorf("expected parent ID %s, got %s", parentID, *retrieved.ParentID)
	}
}

// TestGetJournalEntryByID_WithLines verifies lines are loaded correctly
func TestGetJournalEntryByID_WithLines(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "withlines@example.com")
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

	// Create entry with multiple lines
	amount := decimal.NewFromFloat(123.45)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Entry with lines test",
		Reference:   "REF-123",
		SourceType:  "TEST",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				Description:  "Cash line",
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				Description:  "Revenue line",
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	if retrieved.Reference != "REF-123" {
		t.Errorf("expected reference REF-123, got %s", retrieved.Reference)
	}

	if retrieved.SourceType != "TEST" {
		t.Errorf("expected source type TEST, got %s", retrieved.SourceType)
	}

	if len(retrieved.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(retrieved.Lines))
	}

	// Verify line details
	for _, line := range retrieved.Lines {
		if line.Currency != "EUR" {
			t.Errorf("expected currency EUR, got %s", line.Currency)
		}
		if !line.ExchangeRate.Equal(decimal.NewFromInt(1)) {
			t.Errorf("expected exchange rate 1, got %s", line.ExchangeRate.String())
		}
	}
}

// TestVoidJournalEntry_CreatesReversal verifies reversal entry is properly created
func TestVoidJournalEntry_CreatesReversal(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "reversaltest@example.com")
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

	// Create and post original entry
	amount := decimal.NewFromFloat(200)
	original := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Original entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, original)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, original.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Create reversal
	reversalID := uuid.New().String()
	reversal := &JournalEntry{
		ID:          reversalID,
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Reversal of original",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    revenueAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    cashAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, original.ID, userID, "Test void reason", reversal)
	if err != nil {
		t.Fatalf("VoidJournalEntry failed: %v", err)
	}

	// Verify reversal was created
	retrievedReversal, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, reversalID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID (reversal) failed: %v", err)
	}

	if retrievedReversal.Description != "Reversal of original" {
		t.Errorf("expected description 'Reversal of original', got '%s'", retrievedReversal.Description)
	}

	if len(retrievedReversal.Lines) != 2 {
		t.Errorf("expected 2 lines in reversal, got %d", len(retrievedReversal.Lines))
	}

	// Verify original is voided
	retrievedOriginal, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, original.ID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID (original) failed: %v", err)
	}

	if retrievedOriginal.Status != StatusVoided {
		t.Errorf("expected status VOIDED, got %s", retrievedOriginal.Status)
	}

	if retrievedOriginal.VoidReason != "Test void reason" {
		t.Errorf("expected void reason 'Test void reason', got '%s'", retrievedOriginal.VoidReason)
	}

	if retrievedOriginal.VoidedAt == nil {
		t.Error("expected VoidedAt to be set")
	}

	if retrievedOriginal.VoidedBy == nil || *retrievedOriginal.VoidedBy != userID {
		t.Error("expected VoidedBy to be set to userID")
	}
}

// TestGetAccountBalance_AsOfPastDate tests balance calculation for a past date
func TestGetAccountBalance_AsOfPastDate(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "pastdate@example.com")
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

	// Create entry dated today
	amount := decimal.NewFromFloat(300)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Today's entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get balance as of yesterday - should be zero
	yesterday := time.Now().AddDate(0, 0, -1)
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, cashAccountID, yesterday)
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.IsZero() {
		t.Errorf("expected zero balance for past date, got %s", balance.String())
	}

	// Get balance as of today - should include the transaction
	balance, err = repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, cashAccountID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.Equal(amount) {
		t.Errorf("expected balance %s, got %s", amount.String(), balance.String())
	}
}

// TestGetAccountBalance_ExcludesDraftEntries verifies draft entries don't affect balance
func TestGetAccountBalance_ExcludesDraftEntries(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "excludedraft@example.com")
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

	// Create a draft entry (don't post it)
	amount := decimal.NewFromFloat(1000)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Draft entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    cashAccountID,
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    revenueAccountID,
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Don't post - leave as draft

	// Get balance - should be zero since draft entries are excluded
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, cashAccountID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.IsZero() {
		t.Errorf("expected zero balance (draft excluded), got %s", balance.String())
	}
}

// TestNewRepository verifies repository constructor
func TestNewRepository(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewRepository(pool)

	if repo == nil {
		t.Error("expected non-nil repository")
	}
}

// TestCreateJournalEntry_WithInvalidAccountID tests foreign key constraint
func TestCreateJournalEntry_WithInvalidAccountID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "invalidaccount@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Try to create entry with non-existent account ID
	amount := decimal.NewFromFloat(100)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Entry with bad account",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{
				AccountID:    uuid.New().String(), // Non-existent account
				DebitAmount:  amount,
				CreditAmount: decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    amount,
				BaseCredit:   decimal.Zero,
			},
			{
				AccountID:    uuid.New().String(), // Non-existent account
				DebitAmount:  decimal.Zero,
				CreditAmount: amount,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
				BaseDebit:    decimal.Zero,
				BaseCredit:   amount,
			},
		},
	}

	err := repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err == nil {
		t.Error("expected error for invalid account ID")
	}
}

// TestListAccounts_SortedByCode verifies accounts are sorted by code
func TestListAccounts_SortedByCode(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	accounts, err := repo.ListAccounts(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListAccounts failed: %v", err)
	}

	// Verify accounts are sorted by code
	for i := 1; i < len(accounts); i++ {
		if accounts[i-1].Code > accounts[i].Code {
			t.Errorf("accounts not sorted by code: %s > %s", accounts[i-1].Code, accounts[i].Code)
		}
	}
}

// TestGetTrialBalance_AllAccountTypes tests trial balance with all account types
func TestGetTrialBalance_AllAccountTypes(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "alltypes@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get various account types
	var assetID, liabilityID, revenueID, expenseID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&assetID)
	if err != nil {
		t.Fatalf("failed to get asset account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '2000' LIMIT 1
	`).Scan(&liabilityID)
	if err != nil {
		t.Fatalf("failed to get liability account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '5000' LIMIT 1
	`).Scan(&expenseID)
	if err != nil {
		t.Fatalf("failed to get expense account: %v", err)
	}

	// Create complex journal entry with all account types
	amount := decimal.NewFromFloat(1000)

	// Entry: Debit Asset, Credit Liability (e.g., taking a loan)
	entry1 := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Loan received",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: assetID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: liabilityID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry1)
	if err != nil {
		t.Fatalf("CreateJournalEntry (loan) failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry1.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus (loan) failed: %v", err)
	}

	// Entry: Debit Expense, Credit Asset (e.g., paying for supplies)
	entry2 := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Supplies expense",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: expenseID, DebitAmount: amount.Div(decimal.NewFromInt(2)), CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount.Div(decimal.NewFromInt(2)), BaseCredit: decimal.Zero},
			{AccountID: assetID, DebitAmount: decimal.Zero, CreditAmount: amount.Div(decimal.NewFromInt(2)), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount.Div(decimal.NewFromInt(2))},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry2)
	if err != nil {
		t.Fatalf("CreateJournalEntry (expense) failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry2.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus (expense) failed: %v", err)
	}

	// Get trial balance
	balances, err := repo.GetTrialBalance(ctx, tenant.SchemaName, tenant.ID, time.Now())
	if err != nil {
		t.Fatalf("GetTrialBalance failed: %v", err)
	}

	// Verify we have accounts with activity
	hasAsset := false
	hasLiability := false
	hasExpense := false
	for _, b := range balances {
		switch b.AccountType {
		case AccountTypeAsset:
			hasAsset = true
		case AccountTypeLiability:
			hasLiability = true
		case AccountTypeExpense:
			hasExpense = true
		}
	}

	if !hasAsset {
		t.Error("expected asset account in trial balance")
	}
	if !hasLiability {
		t.Error("expected liability account in trial balance")
	}
	if !hasExpense {
		t.Error("expected expense account in trial balance")
	}
}

// TestGetAccountBalance_LiabilityAccount tests balance calculation for liability accounts
func TestGetAccountBalance_LiabilityAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "liability@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get liability and asset accounts
	var liabilityID, assetID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '2000' LIMIT 1
	`).Scan(&liabilityID)
	if err != nil {
		t.Fatalf("failed to get liability account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&assetID)
	if err != nil {
		t.Fatalf("failed to get asset account: %v", err)
	}

	// Create entry: debit asset (increase), credit liability (increase)
	amount := decimal.NewFromFloat(5000)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Liability test",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: assetID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: liabilityID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get liability balance (credit-normal: credits - debits = 5000)
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, liabilityID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.Equal(amount) {
		t.Errorf("expected liability balance %s, got %s", amount.String(), balance.String())
	}
}

// TestGetAccountBalance_EquityAccount tests balance calculation for equity accounts
func TestGetAccountBalance_EquityAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "equity@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get equity and asset accounts
	var equityID, assetID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '3000' LIMIT 1
	`).Scan(&equityID)
	if err != nil {
		t.Fatalf("failed to get equity account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&assetID)
	if err != nil {
		t.Fatalf("failed to get asset account: %v", err)
	}

	// Create entry: debit asset, credit equity (owner contribution)
	amount := decimal.NewFromFloat(10000)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Owner contribution",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: assetID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: equityID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get equity balance (credit-normal: credits - debits = 10000)
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, equityID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.Equal(amount) {
		t.Errorf("expected equity balance %s, got %s", amount.String(), balance.String())
	}
}

// TestGetAccountBalance_ExpenseAccount tests balance for expense accounts
func TestGetAccountBalance_ExpenseAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "expense@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get expense and asset accounts
	var expenseID, assetID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '5000' LIMIT 1
	`).Scan(&expenseID)
	if err != nil {
		t.Fatalf("failed to get expense account: %v", err)
	}

	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&assetID)
	if err != nil {
		t.Fatalf("failed to get asset account: %v", err)
	}

	// Create entry: debit expense, credit asset (paying an expense)
	amount := decimal.NewFromFloat(250)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Expense payment",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: expenseID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: assetID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get expense balance (debit-normal: debits - credits = 250)
	balance, err := repo.GetAccountBalance(ctx, tenant.SchemaName, tenant.ID, expenseID, time.Now())
	if err != nil {
		t.Fatalf("GetAccountBalance failed: %v", err)
	}

	if !balance.Equal(amount) {
		t.Errorf("expected expense balance %s, got %s", amount.String(), balance.String())
	}
}

// TestCreateAccount_WithDescription tests account with description
func TestCreateAccount_WithDescription(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	account := &Account{
		TenantID:    tenant.ID,
		Code:        "DESC-001",
		Name:        "Described Account",
		AccountType: AccountTypeAsset,
		IsActive:    true,
		IsSystem:    false,
		Description: "This is a detailed description for the account",
	}

	err := repo.CreateAccount(ctx, tenant.SchemaName, account)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	retrieved, err := repo.GetAccountByID(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if retrieved.Description != account.Description {
		t.Errorf("expected description '%s', got '%s'", account.Description, retrieved.Description)
	}
}

// TestCreateJournalEntry_TransactionRollback tests that transaction is rolled back on error
func TestCreateJournalEntry_TransactionRollback(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "rollback@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get a valid account
	var cashID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}

	// Create entry where first line is valid but second line has invalid account
	// This should cause a rollback after the entry is inserted but before lines complete
	amount := decimal.NewFromFloat(100)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Rollback test",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: cashID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: uuid.New().String(), DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount}, // Invalid account
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err == nil {
		t.Error("expected error for invalid line account")
	}

	// Verify the journal entry was NOT created (transaction was rolled back)
	var count int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM `+tenant.SchemaName+`.journal_entries WHERE id = $1
	`, entry.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to check for entry: %v", err)
	}

	if count != 0 {
		t.Error("expected journal entry to be rolled back, but it exists")
	}
}

// TestVoidJournalEntry_ReversalFailure tests rollback when reversal creation fails
func TestVoidJournalEntry_ReversalFailure(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "voidreversalfail@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get accounts
	var cashID, revenueID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	// Create and post a valid entry
	amount := decimal.NewFromFloat(100)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Original for void failure test",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: cashID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: revenueID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Try to void with a reversal that has an invalid account
	reversal := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Reversal with bad account",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: uuid.New().String(), DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero}, // Invalid
			{AccountID: cashID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, entry.ID, userID, "test void", reversal)
	if err == nil {
		t.Error("expected error when reversal has invalid account")
	}

	// Verify the original entry was NOT voided (transaction was rolled back)
	var status string
	err = pool.QueryRow(ctx, `
		SELECT status FROM `+tenant.SchemaName+`.journal_entries WHERE id = $1
	`, entry.ID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to check entry status: %v", err)
	}

	if status != "POSTED" {
		t.Errorf("expected entry to still be POSTED after rollback, got %s", status)
	}
}

// TestGetJournalEntryByID_VerifiesAllFields tests that all fields are loaded
func TestGetJournalEntryByID_VerifiesAllFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "allfields@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get accounts
	var cashID, revenueID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	sourceID := uuid.New().String()
	amount := decimal.NewFromFloat(999.99)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Full field test entry",
		Reference:   "REF-TEST-001",
		SourceType:  "TEST",
		SourceID:    &sourceID,
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: cashID, Description: "Debit line desc", DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "USD", ExchangeRate: decimal.NewFromFloat(0.92), BaseDebit: amount.Mul(decimal.NewFromFloat(0.92)), BaseCredit: decimal.Zero},
			{AccountID: revenueID, Description: "Credit line desc", DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "USD", ExchangeRate: decimal.NewFromFloat(0.92), BaseDebit: decimal.Zero, BaseCredit: amount.Mul(decimal.NewFromFloat(0.92))},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Retrieve and verify all fields
	retrieved, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	// Verify entry fields
	if retrieved.Reference != "REF-TEST-001" {
		t.Errorf("expected reference REF-TEST-001, got %s", retrieved.Reference)
	}
	if retrieved.SourceType != "TEST" {
		t.Errorf("expected source type TEST, got %s", retrieved.SourceType)
	}
	if retrieved.SourceID == nil || *retrieved.SourceID != sourceID {
		t.Errorf("expected source ID to be set")
	}

	// Verify line fields
	for i, line := range retrieved.Lines {
		if line.Currency != "USD" {
			t.Errorf("line %d: expected currency USD, got %s", i, line.Currency)
		}
		if !line.ExchangeRate.Equal(decimal.NewFromFloat(0.92)) {
			t.Errorf("line %d: expected exchange rate 0.92, got %s", i, line.ExchangeRate.String())
		}
	}
}

// TestCreateAccount_SystemAccount tests creating a system account
func TestCreateAccount_SystemAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	account := &Account{
		TenantID:    tenant.ID,
		Code:        "SYS-001",
		Name:        "System Account",
		AccountType: AccountTypeAsset,
		IsActive:    true,
		IsSystem:    true, // Mark as system account
	}

	err := repo.CreateAccount(ctx, tenant.SchemaName, account)
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	retrieved, err := repo.GetAccountByID(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err != nil {
		t.Fatalf("GetAccountByID failed: %v", err)
	}

	if !retrieved.IsSystem {
		t.Error("expected IsSystem to be true")
	}
}

// TestGetPeriodBalances_IncludesOnlyRevenueAndExpense tests that only income statement accounts are returned
func TestGetPeriodBalances_IncludesOnlyRevenueAndExpense(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "periodincomes@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get account IDs
	var assetID, revenueID, expenseID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&assetID)
	if err != nil {
		t.Fatalf("failed to get asset account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '5000' LIMIT 1
	`).Scan(&expenseID)
	if err != nil {
		t.Fatalf("failed to get expense account: %v", err)
	}

	// Create entry with asset, revenue, and expense
	amount := decimal.NewFromFloat(300)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Multi-type entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: assetID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: revenueID, DebitAmount: decimal.Zero, CreditAmount: amount.Div(decimal.NewFromInt(2)), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount.Div(decimal.NewFromInt(2))},
			{AccountID: expenseID, DebitAmount: decimal.Zero, CreditAmount: amount.Div(decimal.NewFromInt(2)), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount.Div(decimal.NewFromInt(2))},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get period balances - should only include revenue and expense
	startDate := time.Now().AddDate(0, 0, -1)
	endDate := time.Now().AddDate(0, 0, 1)

	balances, err := repo.GetPeriodBalances(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetPeriodBalances failed: %v", err)
	}

	// Verify no asset accounts in period balances
	for _, b := range balances {
		if b.AccountType == AccountTypeAsset {
			t.Errorf("asset account should not be in period balances: %s", b.AccountCode)
		}
		if b.AccountType == AccountTypeLiability {
			t.Errorf("liability account should not be in period balances: %s", b.AccountCode)
		}
		if b.AccountType == AccountTypeEquity {
			t.Errorf("equity account should not be in period balances: %s", b.AccountCode)
		}
	}
}

// TestTrialBalance_VerifiesDebitAndCreditColumns tests the debit/credit columns
func TestTrialBalance_VerifiesDebitAndCreditColumns(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tbcolumns@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get accounts
	var cashID, revenueID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	// Create posted entry
	amount := decimal.NewFromFloat(1500)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Trial balance column test",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: cashID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: revenueID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Get trial balance
	balances, err := repo.GetTrialBalance(ctx, tenant.SchemaName, tenant.ID, time.Now())
	if err != nil {
		t.Fatalf("GetTrialBalance failed: %v", err)
	}

	// Find our accounts and verify debit/credit columns
	for _, b := range balances {
		if b.AccountID == cashID {
			if !b.DebitBalance.Equal(amount) {
				t.Errorf("cash debit balance: expected %s, got %s", amount.String(), b.DebitBalance.String())
			}
			if !b.CreditBalance.IsZero() {
				t.Errorf("cash credit balance: expected 0, got %s", b.CreditBalance.String())
			}
		}
		if b.AccountID == revenueID {
			if !b.DebitBalance.IsZero() {
				t.Errorf("revenue debit balance: expected 0, got %s", b.DebitBalance.String())
			}
			if !b.CreditBalance.Equal(amount) {
				t.Errorf("revenue credit balance: expected %s, got %s", amount.String(), b.CreditBalance.String())
			}
		}
	}
}

// TestVoidJournalEntry_VerifiesVoidFields tests that void fields are properly set
func TestVoidJournalEntry_VerifiesVoidFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "voidfields@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get accounts
	var cashID, revenueID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}

	// Create and post entry
	amount := decimal.NewFromFloat(200)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Entry to void",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: cashID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: revenueID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}
	err = repo.UpdateJournalEntryStatus(ctx, tenant.SchemaName, tenant.ID, entry.ID, StatusPosted, userID)
	if err != nil {
		t.Fatalf("UpdateJournalEntryStatus failed: %v", err)
	}

	// Void the entry
	reversal := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Reversal",
		Status:      StatusPosted,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: revenueID, DebitAmount: amount, CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount, BaseCredit: decimal.Zero},
			{AccountID: cashID, DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	voidReason := "Test void with specific reason"
	err = repo.VoidJournalEntry(ctx, tenant.SchemaName, tenant.ID, entry.ID, userID, voidReason, reversal)
	if err != nil {
		t.Fatalf("VoidJournalEntry failed: %v", err)
	}

	// Retrieve voided entry and verify fields
	voided, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	if voided.Status != StatusVoided {
		t.Errorf("expected VOIDED status, got %s", voided.Status)
	}
	if voided.VoidReason != voidReason {
		t.Errorf("expected void reason '%s', got '%s'", voidReason, voided.VoidReason)
	}
	if voided.VoidedAt == nil {
		t.Error("expected VoidedAt to be set")
	}
	if voided.VoidedBy == nil || *voided.VoidedBy != userID {
		t.Error("expected VoidedBy to match user ID")
	}
}

// TestGetJournalEntryByID_VerifiesLines tests that lines are loaded in order
func TestGetJournalEntryByID_VerifiesLines(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "verifylines@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")
	repo := NewRepository(pool)
	ctx := context.Background()

	// Get accounts
	var cashID, revenueID, expenseID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashID)
	if err != nil {
		t.Fatalf("failed to get cash account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '4000' LIMIT 1
	`).Scan(&revenueID)
	if err != nil {
		t.Fatalf("failed to get revenue account: %v", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '5000' LIMIT 1
	`).Scan(&expenseID)
	if err != nil {
		t.Fatalf("failed to get expense account: %v", err)
	}

	// Create entry with 3 lines
	amount := decimal.NewFromFloat(100)
	entry := &JournalEntry{
		TenantID:    tenant.ID,
		EntryDate:   time.Now(),
		Description: "Multi-line entry",
		Status:      StatusDraft,
		CreatedBy:   userID,
		Lines: []JournalEntryLine{
			{AccountID: cashID, Description: "First line", DebitAmount: amount.Mul(decimal.NewFromInt(2)), CreditAmount: decimal.Zero, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: amount.Mul(decimal.NewFromInt(2)), BaseCredit: decimal.Zero},
			{AccountID: revenueID, Description: "Second line", DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
			{AccountID: expenseID, Description: "Third line", DebitAmount: decimal.Zero, CreditAmount: amount, Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.Zero, BaseCredit: amount},
		},
	}

	err = repo.CreateJournalEntry(ctx, tenant.SchemaName, entry)
	if err != nil {
		t.Fatalf("CreateJournalEntry failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := repo.GetJournalEntryByID(ctx, tenant.SchemaName, tenant.ID, entry.ID)
	if err != nil {
		t.Fatalf("GetJournalEntryByID failed: %v", err)
	}

	if len(retrieved.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(retrieved.Lines))
	}

	// Verify all lines have proper parent relationship
	for i, line := range retrieved.Lines {
		if line.JournalEntryID != entry.ID {
			t.Errorf("line %d has wrong journal entry ID: got %s, want %s", i, line.JournalEntryID, entry.ID)
		}
		if line.TenantID != tenant.ID {
			t.Errorf("line %d has wrong tenant ID: got %s, want %s", i, line.TenantID, tenant.ID)
		}
	}
}
