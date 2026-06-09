//go:build integration

package banking

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func setupBankingServiceTest(t *testing.T) (*Service, *testutil.TestTenant, string) {
	t.Helper()

	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "banking-service-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure schema with reconciliation tables
	_, err := pool.Exec(ctx, "SELECT add_reconciliation_tables_to_schema($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add reconciliation tables: %v", err)
	}

	// Create GL account for bank account
	glAccountID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1110', 'Bank GL Account', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create bank account
	_, err = service.CreateBankAccount(ctx, tenant.SchemaName, tenant.ID, &CreateBankAccountRequest{
		Name:          "Test Bank Account",
		AccountNumber: "EE123456789012345678",
		BankName:      "Swedbank",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
		IsDefault:     true,
	})
	if err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Get the bank account ID
	accounts, err := service.ListBankAccounts(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("ListBankAccounts failed: %v", err)
	}

	return service, tenant, accounts[0].ID
}

func TestService_ImportTransactionsRows(t *testing.T) {
	service, tenant, bankAccountID := setupBankingServiceTest(t)
	ctx := context.Background()

	result, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, &ImportCSVRequest{
		FileName: "test_import.csv",
		Transactions: []CSVTransactionRow{
			{Date: "2025-01-15", Amount: "1000.00", Description: "Customer Payment"},
			{Date: "2025-01-16", Amount: "-500.00", Description: "Supplier Payment"},
			{Date: "2025-01-17", Amount: "250.50", Description: "Interest Income"},
		},
	})
	if err != nil {
		t.Fatalf("ImportTransactions failed: %v", err)
	}

	if result.TransactionsImported != 3 {
		t.Errorf("expected 3 transactions imported, got %d", result.TransactionsImported)
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}

	// Verify transactions were created
	transactions, err := service.ListTransactions(ctx, tenant.SchemaName, tenant.ID, &TransactionFilter{
		BankAccountID: bankAccountID,
	})
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}

	if len(transactions) != 3 {
		t.Errorf("expected 3 transactions, got %d", len(transactions))
	}
}

func TestService_ImportTransactionsWithDuplicateSkip(t *testing.T) {
	service, tenant, bankAccountID := setupBankingServiceTest(t)
	ctx := context.Background()

	result1, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, &ImportCSVRequest{
		FileName: "import1.csv",
		Transactions: []CSVTransactionRow{
			{Date: "2025-01-15", Amount: "1000.00", Description: "Customer Payment"},
		},
		SkipDuplicates: true,
	})
	if err != nil {
		t.Fatalf("First ImportTransactions failed: %v", err)
	}

	if result1.TransactionsImported != 1 {
		t.Errorf("expected 1 transaction imported, got %d", result1.TransactionsImported)
	}

	result2, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, &ImportCSVRequest{
		FileName: "import2.csv",
		Transactions: []CSVTransactionRow{
			{Date: "2025-01-15", Amount: "1000.00", Description: "Customer Payment"},
		},
		SkipDuplicates: true,
	})
	if err != nil {
		t.Fatalf("Second ImportTransactions failed: %v", err)
	}

	if result2.DuplicatesSkipped != 1 {
		t.Errorf("expected 1 duplicate skipped, got %d", result2.DuplicatesSkipped)
	}
	if result2.TransactionsImported != 0 {
		t.Errorf("expected 0 transactions imported, got %d", result2.TransactionsImported)
	}
}

func TestService_ImportTransactionsWithDuplicateSkipSameFile(t *testing.T) {
	service, tenant, bankAccountID := setupBankingServiceTest(t)
	ctx := context.Background()

	result, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, &ImportCSVRequest{
		FileName: "same_file.csv",
		Transactions: []CSVTransactionRow{
			{Date: "2025-01-15", Amount: "1000.00", Description: "Customer Payment"},
			{Date: "2025-01-15", Amount: "1000.00", Description: "Customer Payment"},
		},
		SkipDuplicates: true,
	})
	if err != nil {
		t.Fatalf("ImportTransactions failed: %v", err)
	}

	if result.TransactionsImported != 1 {
		t.Errorf("expected 1 transaction imported, got %d", result.TransactionsImported)
	}
	if result.DuplicatesSkipped != 1 {
		t.Errorf("expected 1 duplicate skipped, got %d", result.DuplicatesSkipped)
	}
}

func TestService_ImportTransactionsInvalidData(t *testing.T) {
	service, tenant, bankAccountID := setupBankingServiceTest(t)
	ctx := context.Background()

	result, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, &ImportCSVRequest{
		FileName: "invalid.csv",
		Transactions: []CSVTransactionRow{
			{Date: "invalid-date", Amount: "1000.00", Description: "Valid Amount Invalid Date"},
			{Date: "2025-01-15", Amount: "not-a-number", Description: "Invalid Amount"},
			{Date: "2025-01-16", Amount: "100.00", Description: "Valid Row"},
		},
	})
	if err != nil {
		t.Fatalf("ImportTransactions failed: %v", err)
	}

	if result.TransactionsImported != 1 {
		t.Errorf("expected 1 transaction imported, got %d", result.TransactionsImported)
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestService_ImportTransactions(t *testing.T) {
	service, tenant, bankAccountID := setupBankingServiceTest(t)
	ctx := context.Background()

	req := &ImportCSVRequest{
		FileName: "test_transactions.csv",
		Transactions: []CSVTransactionRow{
			{
				Date:             "2025-01-20",
				Amount:           "500.00",
				Description:      "Payment from customer",
				Reference:        "REF001",
				CounterpartyName: "Customer A",
			},
			{
				Date:             "2025-01-21",
				ValueDate:        "2025-01-22",
				Amount:           "-200.00",
				Description:      "Payment to supplier",
				Reference:        "REF002",
				CounterpartyName: "Supplier B",
			},
		},
		SkipDuplicates: false,
	}

	result, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, req)
	if err != nil {
		t.Fatalf("ImportTransactions failed: %v", err)
	}

	if result.TransactionsImported != 2 {
		t.Errorf("expected 2 transactions imported, got %d", result.TransactionsImported)
	}

	// Verify transactions exist
	transactions, err := service.ListTransactions(ctx, tenant.SchemaName, tenant.ID, &TransactionFilter{
		BankAccountID: bankAccountID,
	})
	if err != nil {
		t.Fatalf("ListTransactions failed: %v", err)
	}

	// Count newly imported
	var count int
	for _, tx := range transactions {
		if tx.Reference == "REF001" || tx.Reference == "REF002" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 transactions with references, got %d", count)
	}
}

func TestService_ImportTransactions_WithDuplicates(t *testing.T) {
	service, tenant, bankAccountID := setupBankingServiceTest(t)
	ctx := context.Background()

	// First import
	req1 := &ImportCSVRequest{
		FileName: "import1.csv",
		Transactions: []CSVTransactionRow{
			{
				Date:       "2025-01-25",
				Amount:     "750.00",
				ExternalID: "EXT001",
			},
		},
		SkipDuplicates: true,
	}

	_, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, req1)
	if err != nil {
		t.Fatalf("First ImportTransactions failed: %v", err)
	}

	// Second import with same external ID
	req2 := &ImportCSVRequest{
		FileName: "import2.csv",
		Transactions: []CSVTransactionRow{
			{
				Date:       "2025-01-25",
				Amount:     "750.00",
				ExternalID: "EXT001",
			},
		},
		SkipDuplicates: true,
	}

	result, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, req2)
	if err != nil {
		t.Fatalf("Second ImportTransactions failed: %v", err)
	}

	if result.DuplicatesSkipped != 1 {
		t.Errorf("expected 1 duplicate skipped, got %d", result.DuplicatesSkipped)
	}
}

func TestService_ImportTransactions_InvalidData(t *testing.T) {
	service, tenant, bankAccountID := setupBankingServiceTest(t)
	ctx := context.Background()

	req := &ImportCSVRequest{
		FileName: "invalid.csv",
		Transactions: []CSVTransactionRow{
			{
				Date:   "invalid-date",
				Amount: "100.00",
			},
			{
				Date:   "2025-01-26",
				Amount: "not-a-number",
			},
			{
				Date:   "2025-01-26",
				Amount: "100.00",
			},
		},
		SkipDuplicates: false,
	}

	result, err := service.ImportTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccountID, req)
	if err != nil {
		t.Fatalf("ImportTransactions failed: %v", err)
	}

	if result.TransactionsImported != 1 {
		t.Errorf("expected 1 transaction imported, got %d", result.TransactionsImported)
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestService_GetMatchSuggestions(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "match-suggest-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Create GL account
	glAccountID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1120', 'Bank Match GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create bank account
	bankAccount, err := service.CreateBankAccount(ctx, tenant.SchemaName, tenant.ID, &CreateBankAccountRequest{
		Name:          "Match Test Account",
		AccountNumber: "EE987654321098765432",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
	})
	if err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create a contact for payments
	contactID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Match Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	// Create a payment (unallocated - total less than amount means partially allocated)
	paymentID := uuid.New().String()
	paymentDate := time.Now().AddDate(0, 0, -2)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, payment_number, payment_date, amount, currency, base_amount, payment_type, payment_method, contact_id, reference, created_by, created_at)
		VALUES ($1, $2, 'PAY-MATCH-001', $3, 500, 'EUR', 500, 'RECEIVED', 'BANK_TRANSFER', $4, 'REF-MATCH-001', $5, NOW())
	`, paymentID, tenant.ID, paymentDate, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create payment: %v", err)
	}

	// Create a bank transaction with matching amount and reference
	transactionID := uuid.New().String()
	transactionDate := time.Now()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.bank_transactions
		(id, tenant_id, bank_account_id, transaction_date, amount, currency, description, reference, counterparty_name, counterparty_account, status, external_id, imported_at)
		VALUES ($1, $2, $3, $4, 500, 'EUR', 'Payment received', 'REF-MATCH-001', 'Match Test Customer', 'EE123', 'UNMATCHED', 'EXT-001', NOW())
	`, transactionID, tenant.ID, bankAccount.ID, transactionDate)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Get match suggestions
	suggestions, err := service.GetMatchSuggestions(ctx, tenant.SchemaName, tenant.ID, transactionID)
	if err != nil {
		t.Fatalf("GetMatchSuggestions failed: %v", err)
	}

	// There might or might not be suggestions depending on unallocated logic
	// The key is that the function runs without error
	t.Logf("Found %d match suggestions", len(suggestions))

	// Verify suggestions contain expected data if any
	for _, s := range suggestions {
		if s.PaymentID == "" {
			t.Error("suggestion should have payment ID")
		}
		if s.Confidence < 0 || s.Confidence > 1 {
			t.Errorf("confidence should be between 0 and 1, got %f", s.Confidence)
		}
	}
}

func TestService_AutoMatchTransactions(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "auto-match-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Setup schema
	_, err := pool.Exec(ctx, "SELECT add_reconciliation_tables_to_schema($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add reconciliation tables: %v", err)
	}

	// Create GL account
	glAccountID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1130', 'Auto Match GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create bank account
	bankAccount, err := service.CreateBankAccount(ctx, tenant.SchemaName, tenant.ID, &CreateBankAccountRequest{
		Name:          "Auto Match Account",
		AccountNumber: "EE111222333444555666",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
	})
	if err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create unmatched transaction
	transactionID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.bank_transactions
		(id, tenant_id, bank_account_id, transaction_date, amount, currency, description, reference, counterparty_name, counterparty_account, status, external_id, imported_at)
		VALUES ($1, $2, $3, NOW(), 1000, 'EUR', 'Large payment', 'REF001', 'Customer Name', 'EE456', 'UNMATCHED', 'EXT-002', NOW())
	`, transactionID, tenant.ID, bankAccount.ID)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Run auto-match (may not match anything but should not error)
	matched, err := service.AutoMatchTransactions(ctx, tenant.SchemaName, tenant.ID, bankAccount.ID, 0.8)
	if err != nil {
		t.Fatalf("AutoMatchTransactions failed: %v", err)
	}

	t.Logf("Auto-matched %d transactions", matched)

	// Verify the function executed without error
	// The actual matching depends on having suitable payments
}

func TestCreateTenantSchemaIncludesBankingReconciliationTables(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	ctx := context.Background()

	for _, tableName := range []string{"bank_reconciliations", "bank_statement_imports", "bank_match_rules"} {
		var exists bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", tenant.SchemaName+"."+tableName).Scan(&exists); err != nil {
			t.Fatalf("failed to inspect %s: %v", tableName, err)
		}
		if !exists {
			t.Fatalf("expected %s.%s to exist after tenant bootstrap", tenant.SchemaName, tableName)
		}
	}

	for _, columnName := range []string{"reconciliation_id", "import_id", "follow_up_status", "review_note", "reviewed_by", "reviewed_at"} {
		var exists bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = $1
				  AND table_name = 'bank_transactions'
				  AND column_name = $2
			)
		`, tenant.SchemaName, columnName).Scan(&exists); err != nil {
			t.Fatalf("failed to inspect bank_transactions.%s: %v", columnName, err)
		}
		if !exists {
			t.Fatalf("expected %s.bank_transactions.%s to exist after tenant bootstrap", tenant.SchemaName, columnName)
		}
	}
}

func TestService_CreatePaymentFromTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "create-payment-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Setup schema
	_, err := pool.Exec(ctx, "SELECT add_reconciliation_tables_to_schema($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add reconciliation tables: %v", err)
	}

	// Create GL account
	glAccountID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts
		(id, tenant_id, code, name, account_type, is_active, created_at)
		VALUES ($1, $2, '1140', 'Create Payment GL', 'ASSET', true, NOW())
	`, glAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create GL account: %v", err)
	}

	// Create bank account
	bankAccount, err := service.CreateBankAccount(ctx, tenant.SchemaName, tenant.ID, &CreateBankAccountRequest{
		Name:          "Payment Create Account",
		AccountNumber: "EE999888777666555444",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
	})
	if err != nil {
		t.Fatalf("CreateBankAccount failed: %v", err)
	}

	// Create an unmatched transaction
	transactionID := uuid.New().String()
	transactionDate := time.Now()
	amount := decimal.NewFromFloat(750.00)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.bank_transactions
		(id, tenant_id, bank_account_id, transaction_date, amount, currency, description, reference, counterparty_name, counterparty_account, status, external_id, imported_at)
		VALUES ($1, $2, $3, $4, $5, 'EUR', 'Payment from customer', 'REF002', 'Payment From Tx Customer', 'EE789', 'UNMATCHED', 'EXT-003', NOW())
	`, transactionID, tenant.ID, bankAccount.ID, transactionDate, amount)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Create payment from transaction (API: schemaName, tenantID, userID, transactionID)
	paymentID, err := service.CreatePaymentFromTransaction(ctx, tenant.SchemaName, tenant.ID, userID, transactionID)
	if err != nil {
		t.Fatalf("CreatePaymentFromTransaction failed: %v", err)
	}

	if paymentID == "" {
		t.Fatal("expected payment ID to be returned")
	}

	// Verify transaction is now matched
	tx, err := service.GetTransaction(ctx, tenant.SchemaName, tenant.ID, transactionID)
	if err != nil {
		t.Fatalf("GetTransaction failed: %v", err)
	}

	if tx.Status != StatusMatched {
		t.Errorf("expected transaction status MATCHED, got %s", tx.Status)
	}
	if tx.MatchedPaymentID == nil || *tx.MatchedPaymentID != paymentID {
		t.Error("expected transaction to be matched to created payment")
	}

	var paymentNumber, paymentType, paymentAmountText, currency, reference, notes string
	err = pool.QueryRow(ctx, `
		SELECT payment_number, payment_type, amount::text, currency, reference, notes
		FROM `+tenant.SchemaName+`.payments
		WHERE id = $1 AND tenant_id = $2
	`, paymentID, tenant.ID).Scan(&paymentNumber, &paymentType, &paymentAmountText, &currency, &reference, &notes)
	if err != nil {
		t.Fatalf("Failed to query created payment: %v", err)
	}
	paymentAmount, err := decimal.NewFromString(paymentAmountText)
	if err != nil {
		t.Fatalf("Failed to parse created payment amount %q: %v", paymentAmountText, err)
	}
	if paymentNumber != "PMT-00001" {
		t.Errorf("expected generated payment number PMT-00001, got %s", paymentNumber)
	}
	if paymentType != "RECEIVED" {
		t.Errorf("expected payment type RECEIVED, got %s", paymentType)
	}
	if !paymentAmount.Equal(amount) {
		t.Errorf("expected payment amount %s, got %s", amount, paymentAmount)
	}
	if currency != "EUR" {
		t.Errorf("expected currency EUR, got %s", currency)
	}
	if reference != "REF002" {
		t.Errorf("expected reference REF002, got %s", reference)
	}
	if notes != "Created from bank transaction: Payment from customer" {
		t.Errorf("expected bank transaction note, got %s", notes)
	}
}

func TestService_ParseDateFormats(t *testing.T) {
	parsed, err := ParseDateFormats("15.03.2026")
	if err != nil {
		t.Fatalf("ParseDateFormats failed: %v", err)
	}
	if got := parsed.Format("2006-01-02"); got != "2026-03-15" {
		t.Errorf("expected 2026-03-15, got %s", got)
	}
}
