//go:build integration

package banking

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

// ensureReconciliationSchema calls the migration function to add reconciliation tables and columns
func ensureReconciliationSchema(t *testing.T, pool *pgxpool.Pool, schemaName string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, "SELECT add_reconciliation_tables_to_schema($1)", schemaName)
	if err != nil {
		t.Fatalf("Failed to add reconciliation tables to schema: %v", err)
	}
}

func TestRepository_CreateAndGetBankAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a GL account first (bank accounts reference GL accounts)
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1110', 'Bank Account GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	account := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		Name:          "Main Business Account",
		AccountNumber: "EE123456789012345678",
		BankName:      "Swedbank",
		SwiftCode:     "HABAEE2X",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsDefault:     true,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	err = repo.CreateBankAccount(ctx, tenant.SchemaName, account)
	if err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	retrieved, err := repo.GetBankAccount(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err != nil {
		t.Fatalf("GetBankAccount failed: %v", err)
	}

	if retrieved.Name != account.Name {
		t.Errorf("expected name %s, got %s", account.Name, retrieved.Name)
	}
	if retrieved.AccountNumber != account.AccountNumber {
		t.Errorf("expected account number %s, got %s", account.AccountNumber, retrieved.AccountNumber)
	}
	if retrieved.Currency != account.Currency {
		t.Errorf("expected currency %s, got %s", account.Currency, retrieved.Currency)
	}
}

func TestRepository_ListBankAccounts(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1001', 'Bank Account 2', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create multiple bank accounts
	accounts := []BankAccount{
		{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			Name:          "Active Account 1",
			AccountNumber: "EE111111111111111111",
			BankName:      "Swedbank",
			Currency:      "EUR",
			GLAccountID:   strPtr(glAccountID),
			IsActive:      true,
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			Name:          "Active Account 2",
			AccountNumber: "EE222222222222222222",
			BankName:      "LHV",
			Currency:      "EUR",
			GLAccountID:   strPtr(glAccountID),
			IsActive:      true,
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			Name:          "Inactive Account",
			AccountNumber: "EE333333333333333333",
			BankName:      "SEB",
			Currency:      "EUR",
			GLAccountID:   strPtr(glAccountID),
			IsActive:      false,
			CreatedAt:     time.Now(),
		},
	}

	for _, acc := range accounts {
		if err := repo.CreateBankAccount(ctx, tenant.SchemaName, &acc); err != nil {
			t.Fatalf("CreateBankAccount failed: %v", err)
		}
	}

	// List all accounts
	all, err := repo.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("ListBankAccounts failed: %v", err)
	}
	if len(all) < 3 {
		t.Errorf("expected at least 3 accounts, got %d", len(all))
	}

	// List active accounts only
	activeFilter := &BankAccountFilter{IsActive: boolPtr(true)}
	active, err := repo.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, activeFilter)
	if err != nil {
		t.Fatalf("ListBankAccounts with filter failed: %v", err)
	}
	for _, acc := range active {
		if !acc.IsActive {
			t.Error("expected only active accounts")
		}
	}
}

func TestRepository_UpdateBankAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1002', 'Bank Account 3', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	account := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		Name:          "Original Name",
		AccountNumber: "EE444444444444444444",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsDefault:     false,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, account); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Update the account
	account.Name = "Updated Name"
	account.IsDefault = true

	if err := repo.UpdateBankAccount(ctx, tenant.SchemaName, account); err != nil {
		t.Fatalf("UpdateBankAccount failed: %v", err)
	}

	retrieved, err := repo.GetBankAccount(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err != nil {
		t.Fatalf("GetBankAccount failed: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", retrieved.Name)
	}
	if !retrieved.IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestRepository_DeleteBankAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1003', 'Bank Account 4', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	account := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		Name:          "To Delete",
		AccountNumber: "EE555555555555555555",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, account); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Delete the account (should soft-delete)
	if err := repo.DeleteBankAccount(ctx, tenant.SchemaName, tenant.ID, account.ID); err != nil {
		t.Fatalf("DeleteBankAccount failed: %v", err)
	}

	// Verify deletion - should either not find or be inactive
	retrieved, err := repo.GetBankAccount(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err == nil && retrieved.IsActive {
		t.Error("expected account to be deleted or inactive")
	}
}

func TestRepository_GetBankAccount_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetBankAccount(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrBankAccountNotFound {
		t.Errorf("expected ErrBankAccountNotFound, got %v", err)
	}
}

func TestRepository_CreateTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	// Ensure reconciliation schema is in place (adds reconciliation_id column to bank_transactions)
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1004', 'Bank Account 5', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Transaction Test Account",
		AccountNumber: "EE666666666666666666",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a transaction
	transaction := &BankTransaction{
		ID:              uuid.New().String(),
		TenantID:        tenant.ID,
		BankAccountID:   bankAccountID,
		TransactionDate: time.Now(),
		Amount:          decimal.NewFromFloat(1000.00),
		Currency:        "EUR",
		Description:     "Test transaction",
		Reference:       "REF123",
		Status:          StatusUnmatched,
		ExternalID:      "EXT123",
		ImportedAt:      time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// List transactions to verify
	filter := &TransactionFilter{
		BankAccountID: bankAccountID,
	}
	transactions, err := repo.ListTransactions(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}

	found := false
	for _, tx := range transactions {
		if tx.ID == transaction.ID {
			found = true
			if !tx.Amount.Equal(transaction.Amount) {
				t.Errorf("expected amount %s, got %s", transaction.Amount, tx.Amount)
			}
		}
	}
	if !found {
		t.Error("created transaction not found in list")
	}
}

func TestRepository_IsTransactionDuplicate(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1005', 'Bank Account 6', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Duplicate Test Account",
		AccountNumber: "EE777777777777777777",
		BankName:      "LHV",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a transaction
	txDate := time.Now()
	txAmount := decimal.NewFromFloat(500.00)
	externalID := "EXT456"

	transaction := &BankTransaction{
		ID:              uuid.New().String(),
		TenantID:        tenant.ID,
		BankAccountID:   bankAccountID,
		TransactionDate: txDate,
		Amount:          txAmount,
		Currency:        "EUR",
		Description:     "Original transaction",
		ExternalID:      externalID,
		Status:          StatusUnmatched,
		ImportedAt:      time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Check for duplicate by external ID - should be true
	isDuplicate, err := repo.IsTransactionDuplicate(ctx, tenant.SchemaName, tenant.ID, bankAccountID, txDate, txAmount, externalID)
	if err != nil {
		t.Fatalf("IsTransactionDuplicate failed: %v", err)
	}
	if !isDuplicate {
		t.Error("expected transaction to be marked as duplicate (same external ID)")
	}

	// Check for duplicate by date+amount - should still be true even with different external ID
	// The duplicate detection also checks date+amount as a fallback
	isDuplicateByDateAmount, err := repo.IsTransactionDuplicate(ctx, tenant.SchemaName, tenant.ID, bankAccountID, txDate, txAmount, "DIFFERENT123")
	if err != nil {
		t.Fatalf("IsTransactionDuplicate failed: %v", err)
	}
	if !isDuplicateByDateAmount {
		t.Error("expected transaction to be duplicate (same date+amount)")
	}

	// Check for non-duplicate - different date and amount
	differentDate := txDate.AddDate(0, 0, -1)
	differentAmount := decimal.NewFromFloat(999.99)
	isNotDuplicate, err := repo.IsTransactionDuplicate(ctx, tenant.SchemaName, tenant.ID, bankAccountID, differentDate, differentAmount, "UNIQUE123")
	if err != nil {
		t.Fatalf("IsTransactionDuplicate failed: %v", err)
	}
	if isNotDuplicate {
		t.Error("expected transaction with different date and amount to not be duplicate")
	}
}

func TestRepository_CalculateAccountBalance(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1006', 'Bank Account 7', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Balance Test Account",
		AccountNumber: "EE888888888888888888",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create some transactions
	transactions := []BankTransaction{
		{ID: uuid.New().String(), TenantID: tenant.ID, BankAccountID: bankAccountID, TransactionDate: time.Now(), Amount: decimal.NewFromFloat(1000.00), Currency: "EUR", Description: "Deposit 1", Status: StatusUnmatched, ImportedAt: time.Now()},
		{ID: uuid.New().String(), TenantID: tenant.ID, BankAccountID: bankAccountID, TransactionDate: time.Now(), Amount: decimal.NewFromFloat(-200.00), Currency: "EUR", Description: "Withdrawal 1", Status: StatusUnmatched, ImportedAt: time.Now()},
		{ID: uuid.New().String(), TenantID: tenant.ID, BankAccountID: bankAccountID, TransactionDate: time.Now(), Amount: decimal.NewFromFloat(500.00), Currency: "EUR", Description: "Deposit 2", Status: StatusUnmatched, ImportedAt: time.Now()},
	}

	for _, tx := range transactions {
		if err := repo.CreateTransaction(ctx, tenant.SchemaName, &tx); err != nil {
			t.Fatalf("CreateTransaction failed: %v", err)
		}
	}

	// Calculate balance
	balance, err := repo.CalculateAccountBalance(ctx, tenant.SchemaName, bankAccountID)
	if err != nil {
		t.Fatalf("CalculateAccountBalance failed: %v", err)
	}

	expectedBalance := decimal.NewFromFloat(1300.00) // 1000 - 200 + 500
	if !balance.Equal(expectedBalance) {
		t.Errorf("expected balance %s, got %s", expectedBalance, balance)
	}
}

func TestRepository_ReconciliationLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-recon-test@example.com")

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account with unique code
	glAccountID := uuid.New().String()
	glCode := "1100" + uuid.New().String()[:4] // Make unique
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Recon GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Reconciliation Test Account",
		AccountNumber: "EE101010101010101010",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a reconciliation
	reconID := uuid.New().String()
	recon := &BankReconciliation{
		ID:             reconID,
		TenantID:       tenant.ID,
		BankAccountID:  bankAccountID,
		StatementDate:  time.Now(),
		OpeningBalance: decimal.NewFromFloat(1000),
		ClosingBalance: decimal.NewFromFloat(1500),
		Status:         ReconciliationInProgress,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
	}

	if err := repo.CreateReconciliation(ctx, tenant.SchemaName, recon); err != nil {
		t.Fatalf("CreateReconciliation failed: %v", err)
	}

	// Get reconciliation
	retrieved, err := repo.GetReconciliation(ctx, tenant.SchemaName, tenant.ID, reconID)
	if err != nil {
		t.Fatalf("GetReconciliation failed: %v", err)
	}

	if retrieved.Status != ReconciliationInProgress {
		t.Errorf("expected status IN_PROGRESS, got %s", retrieved.Status)
	}
	if !retrieved.OpeningBalance.Equal(recon.OpeningBalance) {
		t.Errorf("expected opening balance %s, got %s", recon.OpeningBalance, retrieved.OpeningBalance)
	}

	// List reconciliations
	reconciliations, err := repo.ListReconciliations(ctx, tenant.SchemaName, tenant.ID, bankAccountID)
	if err != nil {
		t.Fatalf("ListReconciliations failed: %v", err)
	}

	if len(reconciliations) != 1 {
		t.Errorf("expected 1 reconciliation, got %d", len(reconciliations))
	}

	// Complete reconciliation
	if err := repo.CompleteReconciliation(ctx, tenant.SchemaName, tenant.ID, reconID); err != nil {
		t.Fatalf("CompleteReconciliation failed: %v", err)
	}

	// Verify completion
	completed, err := repo.GetReconciliation(ctx, tenant.SchemaName, tenant.ID, reconID)
	if err != nil {
		t.Fatalf("GetReconciliation after completion failed: %v", err)
	}

	if completed.Status != ReconciliationCompleted {
		t.Errorf("expected status COMPLETED, got %s", completed.Status)
	}
	if completed.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestRepository_GetTransaction_MatchUnmatch(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-match-test@example.com")

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account with unique code
	glAccountID := uuid.New().String()
	glCode := "1101" + uuid.New().String()[:4] // Make unique
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Match GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Match Test Account",
		AccountNumber: "EE202020202020202020",
		BankName:      "LHV",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a transaction
	txID := uuid.New().String()
	transaction := &BankTransaction{
		ID:              txID,
		TenantID:        tenant.ID,
		BankAccountID:   bankAccountID,
		TransactionDate: time.Now(),
		Amount:          decimal.NewFromFloat(500),
		Currency:        "EUR",
		Description:     "Match test transaction",
		Status:          StatusUnmatched,
		ImportedAt:      time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Get transaction
	retrieved, err := repo.GetTransaction(ctx, tenant.SchemaName, tenant.ID, txID)
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}

	if retrieved.Status != StatusUnmatched {
		t.Errorf("expected status UNMATCHED, got %s", retrieved.Status)
	}

	// Create a contact first (required for payment)
	contactID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Match Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	// Create a payment to match against (note: payment_type is RECEIVED/MADE, no status column)
	paymentID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, payment_number, payment_date, amount, currency, base_amount, payment_type, payment_method, contact_id, created_by, created_at)
		VALUES ($1, $2, 'PAY-MATCH-001', NOW(), 500, 'EUR', 500, 'RECEIVED', 'BANK_TRANSFER', $3, $4, NOW())
	`, paymentID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	// Match the transaction
	if err := repo.MatchTransaction(ctx, tenant.SchemaName, tenant.ID, txID, paymentID); err != nil {
		t.Fatalf("MatchTransaction failed: %v", err)
	}

	// Verify matched status
	matched, err := repo.GetTransaction(ctx, tenant.SchemaName, tenant.ID, txID)
	if err != nil {
		t.Fatalf("GetTransaction after match failed: %v", err)
	}

	if matched.Status != StatusMatched {
		t.Errorf("expected status MATCHED, got %s", matched.Status)
	}
	if matched.MatchedPaymentID == nil || *matched.MatchedPaymentID != paymentID {
		t.Error("expected matched payment ID to be set")
	}

	// Unmatch the transaction
	if err := repo.UnmatchTransaction(ctx, tenant.SchemaName, tenant.ID, txID); err != nil {
		t.Fatalf("UnmatchTransaction failed: %v", err)
	}

	// Verify unmatched status
	unmatched, err := repo.GetTransaction(ctx, tenant.SchemaName, tenant.ID, txID)
	if err != nil {
		t.Fatalf("GetTransaction after unmatch failed: %v", err)
	}

	if unmatched.Status != StatusUnmatched {
		t.Errorf("expected status UNMATCHED, got %s", unmatched.Status)
	}
	if unmatched.MatchedPaymentID != nil {
		t.Error("expected matched payment ID to be nil after unmatch")
	}
}

func TestRepository_AddTransactionToReconciliation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-addtorecon-test@example.com")

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account with unique code
	glAccountID := uuid.New().String()
	glCode := "1102" + uuid.New().String()[:4] // Make unique
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Add Recon GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Add To Recon Test Account",
		AccountNumber: "EE303030303030303030",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a reconciliation
	reconID := uuid.New().String()
	recon := &BankReconciliation{
		ID:             reconID,
		TenantID:       tenant.ID,
		BankAccountID:  bankAccountID,
		StatementDate:  time.Now(),
		OpeningBalance: decimal.NewFromFloat(0),
		ClosingBalance: decimal.NewFromFloat(1000),
		Status:         ReconciliationInProgress,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
	}

	if err := repo.CreateReconciliation(ctx, tenant.SchemaName, recon); err != nil {
		t.Fatalf("CreateReconciliation failed: %v", err)
	}

	// Create a transaction
	txID := uuid.New().String()
	transaction := &BankTransaction{
		ID:              txID,
		TenantID:        tenant.ID,
		BankAccountID:   bankAccountID,
		TransactionDate: time.Now(),
		Amount:          decimal.NewFromFloat(1000),
		Currency:        "EUR",
		Description:     "Recon add test transaction",
		Status:          StatusUnmatched,
		ImportedAt:      time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Add transaction to reconciliation
	if err := repo.AddTransactionToReconciliation(ctx, tenant.SchemaName, tenant.ID, txID, reconID); err != nil {
		t.Fatalf("AddTransactionToReconciliation failed: %v", err)
	}

	// Verify transaction was added (check reconciliation_id)
	var assignedReconID *string
	err = pool.QueryRow(ctx, `
		SELECT reconciliation_id FROM `+tenant.SchemaName+`.bank_transactions WHERE id = $1
	`, txID).Scan(&assignedReconID)
	if err != nil {
		t.Fatalf("Failed to query transaction: %v", err)
	}

	if assignedReconID == nil || *assignedReconID != reconID {
		t.Error("expected transaction to be assigned to reconciliation")
	}
}

func TestRepository_ListTransactionsWithFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1200" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Filter Test GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Filter Test Account",
		AccountNumber: "EE404040404040404040",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create transactions with different dates and amounts
	now := time.Now()
	lastWeek := now.AddDate(0, 0, -7)
	lastMonth := now.AddDate(0, -1, 0)

	transactions := []struct {
		date   time.Time
		amount float64
		status TransactionStatus
	}{
		{now, 100.00, StatusUnmatched},
		{now, 500.00, StatusMatched},
		{lastWeek, 200.00, StatusUnmatched},
		{lastMonth, 1000.00, StatusReconciled},
	}

	for i, tx := range transactions {
		transaction := &BankTransaction{
			ID:              uuid.New().String(),
			TenantID:        tenant.ID,
			BankAccountID:   bankAccountID,
			TransactionDate: tx.date,
			Amount:          decimal.NewFromFloat(tx.amount),
			Currency:        "EUR",
			Description:     "Filter test " + string(rune('0'+i+1)),
			Status:          tx.status,
			ImportedAt:      time.Now(),
		}
		if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
			t.Fatalf("CreateTransaction failed: %v", err)
		}
	}

	// Test filter by status
	t.Run("FilterByStatus", func(t *testing.T) {
		filter := &TransactionFilter{Status: StatusUnmatched}
		results, err := repo.ListTransactions(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("ListTransactions failed: %v", err)
		}
		for _, r := range results {
			if r.Status != StatusUnmatched {
				t.Errorf("expected status UNMATCHED, got %s", r.Status)
			}
		}
	})

	// Test filter by date range
	t.Run("FilterByDateRange", func(t *testing.T) {
		fromDate := now.AddDate(0, 0, -10)
		toDate := now.AddDate(0, 0, 1)
		filter := &TransactionFilter{FromDate: &fromDate, ToDate: &toDate}
		results, err := repo.ListTransactions(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("ListTransactions failed: %v", err)
		}
		// Should include now and lastWeek transactions, not lastMonth
		if len(results) < 2 {
			t.Errorf("expected at least 2 transactions in date range, got %d", len(results))
		}
	})

	// Test filter by amount range
	t.Run("FilterByAmountRange", func(t *testing.T) {
		minAmount := decimal.NewFromFloat(150)
		maxAmount := decimal.NewFromFloat(600)
		filter := &TransactionFilter{MinAmount: &minAmount, MaxAmount: &maxAmount}
		results, err := repo.ListTransactions(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("ListTransactions failed: %v", err)
		}
		// Should include 200 and 500 transactions
		for _, r := range results {
			if r.Amount.LessThan(minAmount) || r.Amount.GreaterThan(maxAmount) {
				t.Errorf("transaction amount %s is outside range [%s, %s]", r.Amount, minAmount, maxAmount)
			}
		}
	})

	// Test combined filters
	t.Run("CombinedFilters", func(t *testing.T) {
		minAmount := decimal.NewFromFloat(50)
		filter := &TransactionFilter{
			BankAccountID: bankAccountID,
			Status:        StatusUnmatched,
			MinAmount:     &minAmount,
		}
		results, err := repo.ListTransactions(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("ListTransactions failed: %v", err)
		}
		for _, r := range results {
			if r.BankAccountID != bankAccountID {
				t.Errorf("expected bank account ID %s, got %s", bankAccountID, r.BankAccountID)
			}
			if r.Status != StatusUnmatched {
				t.Errorf("expected status UNMATCHED, got %s", r.Status)
			}
			if r.Amount.LessThan(minAmount) {
				t.Errorf("expected amount >= %s, got %s", minAmount, r.Amount)
			}
		}
	})
}

func TestRepository_GetTransaction_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetTransaction(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrTransactionNotFound {
		t.Errorf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestRepository_GetReconciliation_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetReconciliation(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrReconciliationNotFound {
		t.Errorf("expected ErrReconciliationNotFound, got %v", err)
	}
}

func TestRepository_UnsetDefaultAccounts(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1104', 'Bank Unset GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create two default accounts
	for i := 1; i <= 2; i++ {
		account := &BankAccount{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			Name:          "Default Account " + string(rune('0'+i)),
			AccountNumber: "EE50505050505050505" + string(rune('0'+i)),
			BankName:      "Swedbank",
			Currency:      "EUR",
			GLAccountID:   strPtr(glAccountID),
			IsDefault:     true,
			IsActive:      true,
			CreatedAt:     time.Now(),
		}

		if err := repo.CreateBankAccount(ctx, tenant.SchemaName, account); err != nil {
			t.Fatalf("CreateBankAccount failed: %v", err)
		}
	}

	// Unset all default accounts
	if err := repo.UnsetDefaultAccounts(ctx, tenant.SchemaName, tenant.ID); err != nil {
		t.Fatalf("UnsetDefaultAccounts failed: %v", err)
	}

	// Verify no default accounts remain
	accounts, err := repo.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("ListBankAccounts failed: %v", err)
	}

	for _, acc := range accounts {
		if acc.IsDefault {
			t.Error("expected no default accounts after UnsetDefaultAccounts")
		}
	}
}

func TestRepository_CountTransactionsForAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	// Ensure reconciliation schema is in place
	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1007', 'Bank Account 8', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Count Test Account",
		AccountNumber: "EE999999999999999999",
		BankName:      "Nordea",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Initially should be 0
	count, err := repo.CountTransactionsForAccount(ctx, tenant.SchemaName, bankAccountID)
	if err != nil {
		t.Fatalf("CountTransactionsForAccount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 transactions initially, got %d", count)
	}

	// Create 3 transactions
	for i := 0; i < 3; i++ {
		tx := &BankTransaction{
			ID:              uuid.New().String(),
			TenantID:        tenant.ID,
			BankAccountID:   bankAccountID,
			TransactionDate: time.Now(),
			Amount:          decimal.NewFromFloat(100.00),
			Currency:        "EUR",
			Description:     "Test transaction",
			Status:          StatusUnmatched,
			ImportedAt:      time.Now(),
		}
		if err := repo.CreateTransaction(ctx, tenant.SchemaName, tx); err != nil {
			t.Fatalf("CreateTransaction failed: %v", err)
		}
	}

	// Should now be 3
	count, err = repo.CountTransactionsForAccount(ctx, tenant.SchemaName, bankAccountID)
	if err != nil {
		t.Fatalf("CountTransactionsForAccount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 transactions, got %d", count)
	}
}

func TestRepository_DeleteBankAccount_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to delete a non-existent account
	err := repo.DeleteBankAccount(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrBankAccountNotFound {
		t.Errorf("expected ErrBankAccountNotFound, got %v", err)
	}
}

func TestRepository_MatchTransaction_AlreadyMatched(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-match-err-test@example.com")

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1300" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Match Err GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Match Error Test Account",
		AccountNumber: "EE606060606060606060",
		BankName:      "LHV",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a transaction that's already matched
	txID := uuid.New().String()
	paymentID := uuid.New().String()

	// Create contact for payment
	contactID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Match Error Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	// Create payment
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, payment_number, payment_date, amount, currency, base_amount, payment_type, payment_method, contact_id, created_by, created_at)
		VALUES ($1, $2, 'PAY-MATCH-ERR-001', NOW(), 500, 'EUR', 500, 'RECEIVED', 'BANK_TRANSFER', $3, $4, NOW())
	`, paymentID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	// Create transaction with MATCHED status
	transaction := &BankTransaction{
		ID:               txID,
		TenantID:         tenant.ID,
		BankAccountID:    bankAccountID,
		TransactionDate:  time.Now(),
		Amount:           decimal.NewFromFloat(500),
		Currency:         "EUR",
		Description:      "Already matched transaction",
		Status:           StatusMatched,
		MatchedPaymentID: &paymentID,
		ImportedAt:       time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Try to match again - should fail
	anotherPaymentID := uuid.New().String()
	err = repo.MatchTransaction(ctx, tenant.SchemaName, tenant.ID, txID, anotherPaymentID)
	if err != ErrTransactionAlreadyMatched {
		t.Errorf("expected ErrTransactionAlreadyMatched, got %v", err)
	}
}

func TestRepository_MatchTransaction_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to match a non-existent transaction
	err := repo.MatchTransaction(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), uuid.New().String())
	if err != ErrTransactionAlreadyMatched {
		t.Errorf("expected ErrTransactionAlreadyMatched for non-existent transaction, got %v", err)
	}
}

func TestRepository_UnmatchTransaction_NotMatched(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1301" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Unmatch Err GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Unmatch Error Test Account",
		AccountNumber: "EE707070707070707070",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create an unmatched transaction
	txID := uuid.New().String()
	transaction := &BankTransaction{
		ID:              txID,
		TenantID:        tenant.ID,
		BankAccountID:   bankAccountID,
		TransactionDate: time.Now(),
		Amount:          decimal.NewFromFloat(300),
		Currency:        "EUR",
		Description:     "Unmatched transaction",
		Status:          StatusUnmatched,
		ImportedAt:      time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Try to unmatch - should fail because it's not matched
	err = repo.UnmatchTransaction(ctx, tenant.SchemaName, tenant.ID, txID)
	if err != ErrTransactionNotMatched {
		t.Errorf("expected ErrTransactionNotMatched, got %v", err)
	}
}

func TestRepository_UnmatchTransaction_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to unmatch a non-existent transaction
	err := repo.UnmatchTransaction(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrTransactionNotMatched {
		t.Errorf("expected ErrTransactionNotMatched for non-existent transaction, got %v", err)
	}
}

func TestRepository_CompleteReconciliation_AlreadyCompleted(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-recon-complete-err@example.com")

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1302" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Recon Complete Err GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Complete Recon Error Test Account",
		AccountNumber: "EE808080808080808080",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create reconciliation
	reconID := uuid.New().String()
	recon := &BankReconciliation{
		ID:             reconID,
		TenantID:       tenant.ID,
		BankAccountID:  bankAccountID,
		StatementDate:  time.Now(),
		OpeningBalance: decimal.NewFromFloat(1000),
		ClosingBalance: decimal.NewFromFloat(1500),
		Status:         ReconciliationInProgress,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
	}

	if err := repo.CreateReconciliation(ctx, tenant.SchemaName, recon); err != nil {
		t.Fatalf("CreateReconciliation failed: %v", err)
	}

	// Complete it once
	if err := repo.CompleteReconciliation(ctx, tenant.SchemaName, tenant.ID, reconID); err != nil {
		t.Fatalf("First CompleteReconciliation failed: %v", err)
	}

	// Try to complete again - should fail
	err = repo.CompleteReconciliation(ctx, tenant.SchemaName, tenant.ID, reconID)
	if err != ErrReconciliationAlreadyDone {
		t.Errorf("expected ErrReconciliationAlreadyDone, got %v", err)
	}
}

func TestRepository_CompleteReconciliation_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to complete a non-existent reconciliation
	err := repo.CompleteReconciliation(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrReconciliationAlreadyDone {
		t.Errorf("expected ErrReconciliationAlreadyDone for non-existent reconciliation, got %v", err)
	}
}

func TestRepository_AddTransactionToReconciliation_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to add a non-existent transaction to reconciliation
	err := repo.AddTransactionToReconciliation(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), uuid.New().String())
	if err != ErrTransactionNotFound {
		t.Errorf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestRepository_ListBankAccounts_CurrencyFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account
	glAccountID := uuid.New().String()
	glCode := "1303" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Currency Filter GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create accounts with different currencies
	eurAccount := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		Name:          "EUR Account",
		AccountNumber: "EE909090909090909090",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	usdAccount := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		Name:          "USD Account",
		AccountNumber: "US909090909090909090",
		BankName:      "Chase",
		Currency:      "USD",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, eurAccount); err != nil {
		t.Fatalf("CreateBankAccount EUR failed: %v", err)
	}
	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, usdAccount); err != nil {
		t.Fatalf("CreateBankAccount USD failed: %v", err)
	}

	// Filter by EUR currency
	filter := &BankAccountFilter{Currency: "EUR"}
	eurAccounts, err := repo.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("ListBankAccounts with EUR filter failed: %v", err)
	}

	for _, acc := range eurAccounts {
		if acc.Currency != "EUR" {
			t.Errorf("expected currency EUR, got %s", acc.Currency)
		}
	}

	// Filter by USD currency
	filter = &BankAccountFilter{Currency: "USD"}
	usdAccounts, err := repo.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("ListBankAccounts with USD filter failed: %v", err)
	}

	for _, acc := range usdAccounts {
		if acc.Currency != "USD" {
			t.Errorf("expected currency USD, got %s", acc.Currency)
		}
	}
}

func TestRepository_ListBankAccounts_EmptyResult(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// List accounts for a tenant with no accounts
	accounts, err := repo.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("ListBankAccounts failed: %v", err)
	}

	if accounts == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}
}

func TestRepository_ListTransactions_EmptyResult(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// List transactions for a tenant with no transactions
	transactions, err := repo.ListTransactions(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}

	if transactions == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(transactions) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(transactions))
	}
}

func TestRepository_ListReconciliations_EmptyResult(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a bank account but no reconciliations
	glAccountID := uuid.New().String()
	glCode := "1304" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Empty Recon GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Empty Recon Test Account",
		AccountNumber: "EE101010101010101011",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// List reconciliations for account with none
	reconciliations, err := repo.ListReconciliations(ctx, tenant.SchemaName, tenant.ID, bankAccountID)
	if err != nil {
		t.Fatalf("ListReconciliations failed: %v", err)
	}

	if reconciliations == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(reconciliations) != 0 {
		t.Errorf("expected 0 reconciliations, got %d", len(reconciliations))
	}
}

func TestRepository_CalculateAccountBalance_EmptyAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1305" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Empty Balance GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Empty Balance Test Account",
		AccountNumber: "EE111111111111111112",
		BankName:      "Nordea",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Calculate balance for account with no transactions
	balance, err := repo.CalculateAccountBalance(ctx, tenant.SchemaName, bankAccountID)
	if err != nil {
		t.Fatalf("CalculateAccountBalance failed: %v", err)
	}

	if !balance.Equal(decimal.Zero) {
		t.Errorf("expected balance 0, got %s", balance)
	}
}

func TestRepository_CreateTransaction_WithAllFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1306" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank All Fields GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "All Fields Test Account",
		AccountNumber: "EE121212121212121212",
		BankName:      "LHV",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a transaction with all fields populated
	txID := uuid.New().String()
	valueDate := time.Now().Add(-24 * time.Hour)
	transaction := &BankTransaction{
		ID:                  txID,
		TenantID:            tenant.ID,
		BankAccountID:       bankAccountID,
		TransactionDate:     time.Now(),
		ValueDate:           &valueDate,
		Amount:              decimal.NewFromFloat(1234.56),
		Currency:            "EUR",
		Description:         "Full transaction with all fields",
		Reference:           "REF-ALL-FIELDS-001",
		CounterpartyName:    "Test Counterparty Ltd",
		CounterpartyAccount: "EE999999999999999999",
		Status:              StatusUnmatched,
		ExternalID:          "EXT-ALL-FIELDS-001",
		ImportedAt:          time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := repo.GetTransaction(ctx, tenant.SchemaName, tenant.ID, txID)
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}

	if retrieved.Description != transaction.Description {
		t.Errorf("expected description %q, got %q", transaction.Description, retrieved.Description)
	}
	if retrieved.Reference != transaction.Reference {
		t.Errorf("expected reference %q, got %q", transaction.Reference, retrieved.Reference)
	}
	if retrieved.CounterpartyName != transaction.CounterpartyName {
		t.Errorf("expected counterparty name %q, got %q", transaction.CounterpartyName, retrieved.CounterpartyName)
	}
	if retrieved.CounterpartyAccount != transaction.CounterpartyAccount {
		t.Errorf("expected counterparty account %q, got %q", transaction.CounterpartyAccount, retrieved.CounterpartyAccount)
	}
	if retrieved.ExternalID != transaction.ExternalID {
		t.Errorf("expected external ID %q, got %q", transaction.ExternalID, retrieved.ExternalID)
	}
	if retrieved.ValueDate == nil {
		t.Error("expected value date to be set")
	}
}

func TestRepository_ListBankAccounts_CombinedFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account
	glAccountID := uuid.New().String()
	glCode := "1307" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Combined Filter GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create accounts with different combinations
	accounts := []BankAccount{
		{ID: uuid.New().String(), TenantID: tenant.ID, Name: "EUR Active", AccountNumber: "EE131313131313131313", BankName: "Swedbank", Currency: "EUR", GLAccountID: strPtr(glAccountID), IsActive: true, CreatedAt: time.Now()},
		{ID: uuid.New().String(), TenantID: tenant.ID, Name: "EUR Inactive", AccountNumber: "EE141414141414141414", BankName: "LHV", Currency: "EUR", GLAccountID: strPtr(glAccountID), IsActive: false, CreatedAt: time.Now()},
		{ID: uuid.New().String(), TenantID: tenant.ID, Name: "USD Active", AccountNumber: "US151515151515151515", BankName: "Chase", Currency: "USD", GLAccountID: strPtr(glAccountID), IsActive: true, CreatedAt: time.Now()},
		{ID: uuid.New().String(), TenantID: tenant.ID, Name: "USD Inactive", AccountNumber: "US161616161616161616", BankName: "BOA", Currency: "USD", GLAccountID: strPtr(glAccountID), IsActive: false, CreatedAt: time.Now()},
	}

	for _, acc := range accounts {
		if err := repo.CreateBankAccount(ctx, tenant.SchemaName, &acc); err != nil {
			t.Fatalf("CreateBankAccount failed: %v", err)
		}
	}

	// Filter by both EUR and active
	filter := &BankAccountFilter{Currency: "EUR", IsActive: boolPtr(true)}
	results, err := repo.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("ListBankAccounts failed: %v", err)
	}

	for _, acc := range results {
		if acc.Currency != "EUR" {
			t.Errorf("expected currency EUR, got %s", acc.Currency)
		}
		if !acc.IsActive {
			t.Error("expected active account")
		}
	}
}

func TestRepository_IsTransactionDuplicate_NoExternalID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1308" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank No External ID GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "No External ID Test Account",
		AccountNumber: "EE171717171717171717",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a transaction without external ID
	txDate := time.Now().Truncate(24 * time.Hour) // Truncate to day for consistent comparison
	txAmount := decimal.NewFromFloat(750.00)

	transaction := &BankTransaction{
		ID:              uuid.New().String(),
		TenantID:        tenant.ID,
		BankAccountID:   bankAccountID,
		TransactionDate: txDate,
		Amount:          txAmount,
		Currency:        "EUR",
		Description:     "Transaction without external ID",
		Status:          StatusUnmatched,
		ExternalID:      "", // No external ID
		ImportedAt:      time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Check for duplicate without external ID - should use date+amount fallback
	isDuplicate, err := repo.IsTransactionDuplicate(ctx, tenant.SchemaName, tenant.ID, bankAccountID, txDate, txAmount, "")
	if err != nil {
		t.Fatalf("IsTransactionDuplicate failed: %v", err)
	}
	if !isDuplicate {
		t.Error("expected transaction to be duplicate (same date+amount)")
	}

	// Check with different amount - should not be duplicate
	differentAmount := decimal.NewFromFloat(750.01)
	isNotDuplicate, err := repo.IsTransactionDuplicate(ctx, tenant.SchemaName, tenant.ID, bankAccountID, txDate, differentAmount, "")
	if err != nil {
		t.Fatalf("IsTransactionDuplicate failed: %v", err)
	}
	if isNotDuplicate {
		t.Error("expected transaction with different amount to not be duplicate")
	}
}

func TestRepository_CompleteReconciliation_UpdatesMatchedTransactions(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-recon-tx-update@example.com")

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1309" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Recon TX Update GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Recon TX Update Test Account",
		AccountNumber: "EE181818181818181818",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create reconciliation
	reconID := uuid.New().String()
	recon := &BankReconciliation{
		ID:             reconID,
		TenantID:       tenant.ID,
		BankAccountID:  bankAccountID,
		StatementDate:  time.Now(),
		OpeningBalance: decimal.NewFromFloat(0),
		ClosingBalance: decimal.NewFromFloat(1000),
		Status:         ReconciliationInProgress,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
	}

	if err := repo.CreateReconciliation(ctx, tenant.SchemaName, recon); err != nil {
		t.Fatalf("CreateReconciliation failed: %v", err)
	}

	// Create contact for payment
	contactID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Recon Update Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	// Create payment
	paymentID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, payment_number, payment_date, amount, currency, base_amount, payment_type, payment_method, contact_id, created_by, created_at)
		VALUES ($1, $2, 'PAY-RECON-UPD-001', NOW(), 1000, 'EUR', 1000, 'RECEIVED', 'BANK_TRANSFER', $3, $4, NOW())
	`, paymentID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	// Create a matched transaction with reconciliation_id
	txID := uuid.New().String()
	transaction := &BankTransaction{
		ID:               txID,
		TenantID:         tenant.ID,
		BankAccountID:    bankAccountID,
		TransactionDate:  time.Now(),
		Amount:           decimal.NewFromFloat(1000),
		Currency:         "EUR",
		Description:      "Matched transaction for recon",
		Status:           StatusMatched,
		MatchedPaymentID: &paymentID,
		ReconciliationID: &reconID,
		ImportedAt:       time.Now(),
	}

	if err := repo.CreateTransaction(ctx, tenant.SchemaName, transaction); err != nil {
		t.Fatalf("CreateTransaction failed: %v", err)
	}

	// Complete reconciliation - should update transaction status to RECONCILED
	if err := repo.CompleteReconciliation(ctx, tenant.SchemaName, tenant.ID, reconID); err != nil {
		t.Fatalf("CompleteReconciliation failed: %v", err)
	}

	// Verify transaction status was updated
	updatedTx, err := repo.GetTransaction(ctx, tenant.SchemaName, tenant.ID, txID)
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}

	if updatedTx.Status != StatusReconciled {
		t.Errorf("expected status RECONCILED, got %s", updatedTx.Status)
	}
}

func TestRepository_BankAccount_NullGLAccountID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a bank account without GL account ID
	account := &BankAccount{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		Name:          "Account Without GL",
		AccountNumber: "EE191919191919191919",
		BankName:      "Nordea",
		Currency:      "EUR",
		GLAccountID:   nil, // No GL account
		IsDefault:     false,
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, account); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Retrieve and verify
	retrieved, err := repo.GetBankAccount(ctx, tenant.SchemaName, tenant.ID, account.ID)
	if err != nil {
		t.Fatalf("GetBankAccount failed: %v", err)
	}

	if retrieved.GLAccountID != nil {
		t.Error("expected GLAccountID to be nil")
	}
}

func TestRepository_ListTransactions_NoFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1310" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank No Filter GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "No Filter Test Account",
		AccountNumber: "EE202020202020202021",
		BankName:      "LHV",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create multiple transactions
	for i := 0; i < 5; i++ {
		tx := &BankTransaction{
			ID:              uuid.New().String(),
			TenantID:        tenant.ID,
			BankAccountID:   bankAccountID,
			TransactionDate: time.Now().AddDate(0, 0, -i),
			Amount:          decimal.NewFromFloat(float64(100 * (i + 1))),
			Currency:        "EUR",
			Description:     fmt.Sprintf("Transaction %d", i+1),
			Status:          StatusUnmatched,
			ImportedAt:      time.Now(),
		}
		if err := repo.CreateTransaction(ctx, tenant.SchemaName, tx); err != nil {
			t.Fatalf("CreateTransaction failed: %v", err)
		}
	}

	// List with nil filter - should return all
	transactions, err := repo.ListTransactions(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}

	if len(transactions) < 5 {
		t.Errorf("expected at least 5 transactions, got %d", len(transactions))
	}
}

func TestRepository_MultipleReconciliations_SameBankAccount(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-multi-recon@example.com")

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1311" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Multi Recon GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Multi Recon Test Account",
		AccountNumber: "EE212121212121212121",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create multiple reconciliations for the same bank account
	for i := 0; i < 3; i++ {
		reconID := uuid.New().String()
		recon := &BankReconciliation{
			ID:             reconID,
			TenantID:       tenant.ID,
			BankAccountID:  bankAccountID,
			StatementDate:  time.Now().AddDate(0, -i, 0),
			OpeningBalance: decimal.NewFromFloat(float64(1000 * i)),
			ClosingBalance: decimal.NewFromFloat(float64(1000 * (i + 1))),
			Status:         ReconciliationInProgress,
			CreatedAt:      time.Now(),
			CreatedBy:      userID,
		}

		if err := repo.CreateReconciliation(ctx, tenant.SchemaName, recon); err != nil {
			t.Fatalf("CreateReconciliation %d failed: %v", i, err)
		}
	}

	// List reconciliations - should return all 3
	reconciliations, err := repo.ListReconciliations(ctx, tenant.SchemaName, tenant.ID, bankAccountID)
	if err != nil {
		t.Fatalf("ListReconciliations failed: %v", err)
	}

	if len(reconciliations) != 3 {
		t.Errorf("expected 3 reconciliations, got %d", len(reconciliations))
	}

	// Verify ordering (most recent first)
	for i := 1; i < len(reconciliations); i++ {
		if reconciliations[i].StatementDate.After(reconciliations[i-1].StatementDate) {
			t.Error("expected reconciliations to be ordered by statement date descending")
		}
	}
}

func TestRepository_CreateImportRecord(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1312" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Import Test GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Import Test Account",
		AccountNumber: "EE222222222222222222",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create an import record
	importID := uuid.New().String()
	importRecord := &BankStatementImport{
		ID:                   importID,
		TenantID:             tenant.ID,
		BankAccountID:        bankAccountID,
		FileName:             "test_statement.csv",
		TransactionsImported: 10,
		TransactionsMatched:  5,
		DuplicatesSkipped:    2,
		CreatedAt:            time.Now(),
	}

	if err := repo.CreateImportRecord(ctx, tenant.SchemaName, importRecord); err != nil {
		t.Fatalf("CreateImportRecord failed: %v", err)
	}

	// Verify the import record was created by querying it directly
	var fileName string
	var txImported, txMatched, dupsSkipped int
	err = pool.QueryRow(ctx, `
		SELECT file_name, transactions_imported, transactions_matched, duplicates_skipped
		FROM `+tenant.SchemaName+`.bank_statement_imports
		WHERE id = $1
	`, importID).Scan(&fileName, &txImported, &txMatched, &dupsSkipped)
	if err != nil {
		t.Fatalf("Failed to query import record: %v", err)
	}

	if fileName != importRecord.FileName {
		t.Errorf("expected file name %q, got %q", importRecord.FileName, fileName)
	}
	if txImported != importRecord.TransactionsImported {
		t.Errorf("expected transactions imported %d, got %d", importRecord.TransactionsImported, txImported)
	}
	if txMatched != importRecord.TransactionsMatched {
		t.Errorf("expected transactions matched %d, got %d", importRecord.TransactionsMatched, txMatched)
	}
	if dupsSkipped != importRecord.DuplicatesSkipped {
		t.Errorf("expected duplicates skipped %d, got %d", importRecord.DuplicatesSkipped, dupsSkipped)
	}
}

func TestRepository_GetImportHistory(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1313" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Import History GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Import History Test Account",
		AccountNumber: "EE232323232323232323",
		BankName:      "LHV",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create multiple import records
	for i := 0; i < 5; i++ {
		importRecord := &BankStatementImport{
			ID:                   uuid.New().String(),
			TenantID:             tenant.ID,
			BankAccountID:        bankAccountID,
			FileName:             fmt.Sprintf("statement_%d.csv", i+1),
			TransactionsImported: 10 + i,
			TransactionsMatched:  5 + i,
			DuplicatesSkipped:    i,
			CreatedAt:            time.Now().Add(time.Duration(-i) * time.Hour),
		}

		if err := repo.CreateImportRecord(ctx, tenant.SchemaName, importRecord); err != nil {
			t.Fatalf("CreateImportRecord %d failed: %v", i, err)
		}
	}

	// Get import history
	history, err := repo.GetImportHistory(ctx, tenant.SchemaName, tenant.ID, bankAccountID)
	if err != nil {
		t.Fatalf("GetImportHistory failed: %v", err)
	}

	if len(history) != 5 {
		t.Errorf("expected 5 import records, got %d", len(history))
	}

	// Verify ordering (most recent first)
	for i := 1; i < len(history); i++ {
		if history[i].CreatedAt.After(history[i-1].CreatedAt) {
			t.Error("expected import history to be ordered by created_at descending")
		}
	}

	// Verify data integrity
	for _, imp := range history {
		if imp.TenantID != tenant.ID {
			t.Errorf("expected tenant ID %s, got %s", tenant.ID, imp.TenantID)
		}
		if imp.BankAccountID != bankAccountID {
			t.Errorf("expected bank account ID %s, got %s", bankAccountID, imp.BankAccountID)
		}
		if imp.FileName == "" {
			t.Error("expected file name to be set")
		}
	}
}

func TestRepository_GetImportHistory_EmptyResult(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1314" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Empty Import GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Empty Import History Account",
		AccountNumber: "EE242424242424242424",
		BankName:      "SEB",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Get import history for account with no imports
	history, err := repo.GetImportHistory(ctx, tenant.SchemaName, tenant.ID, bankAccountID)
	if err != nil {
		t.Fatalf("GetImportHistory failed: %v", err)
	}

	if history == nil {
		t.Error("expected empty slice, got nil")
	}

	if len(history) != 0 {
		t.Errorf("expected 0 import records, got %d", len(history))
	}
}

func TestRepository_GetImportHistory_Limit(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	ensureReconciliationSchema(t, pool, tenant.SchemaName)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create GL account and bank account
	glAccountID := uuid.New().String()
	glCode := "1315" + uuid.New().String()[:4]
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, $3, 'Bank Import Limit GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID, glCode)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	bankAccountID := uuid.New().String()
	bankAccount := &BankAccount{
		ID:            bankAccountID,
		TenantID:      tenant.ID,
		Name:          "Import Limit Test Account",
		AccountNumber: "EE252525252525252525",
		BankName:      "Nordea",
		Currency:      "EUR",
		GLAccountID:   strPtr(glAccountID),
		IsActive:      true,
		CreatedAt:     time.Now(),
	}

	if err := repo.CreateBankAccount(ctx, tenant.SchemaName, bankAccount); err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create 55 import records (more than the 50 limit)
	for i := 0; i < 55; i++ {
		importRecord := &BankStatementImport{
			ID:                   uuid.New().String(),
			TenantID:             tenant.ID,
			BankAccountID:        bankAccountID,
			FileName:             fmt.Sprintf("statement_%d.csv", i+1),
			TransactionsImported: 10,
			TransactionsMatched:  5,
			DuplicatesSkipped:    0,
			CreatedAt:            time.Now().Add(time.Duration(-i) * time.Minute),
		}

		if err := repo.CreateImportRecord(ctx, tenant.SchemaName, importRecord); err != nil {
			t.Fatalf("CreateImportRecord %d failed: %v", i, err)
		}
	}

	// Get import history - should be limited to 50
	history, err := repo.GetImportHistory(ctx, tenant.SchemaName, tenant.ID, bankAccountID)
	if err != nil {
		t.Fatalf("GetImportHistory failed: %v", err)
	}

	if len(history) != 50 {
		t.Errorf("expected 50 import records (limit), got %d", len(history))
	}
}
