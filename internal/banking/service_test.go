package banking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// MockRepository implements Repository for testing
type MockRepository struct {
	accounts        map[string]*BankAccount
	transactions    map[string]*BankTransaction
	reconciliations map[string]*BankReconciliation
	imports         map[string]*BankStatementImport

	// Function overrides for custom behavior
	CreateBankAccountFn              func(ctx context.Context, schemaName string, account *BankAccount) error
	GetBankAccountFn                 func(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error)
	ListBankAccountsFn               func(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error)
	UpdateBankAccountFn              func(ctx context.Context, schemaName string, account *BankAccount) error
	DeleteBankAccountFn              func(ctx context.Context, schemaName, tenantID, accountID string) error
	UnsetDefaultAccountsFn           func(ctx context.Context, schemaName, tenantID string) error
	CountTransactionsForAccountFn    func(ctx context.Context, schemaName, accountID string) (int, error)
	CalculateAccountBalanceFn        func(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error)
	ListTransactionsFn               func(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error)
	GetTransactionFn                 func(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error)
	MatchTransactionFn               func(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error
	UnmatchTransactionFn             func(ctx context.Context, schemaName, tenantID, transactionID string) error
	CreateReconciliationFn           func(ctx context.Context, schemaName string, r *BankReconciliation) error
	GetReconciliationFn              func(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error)
	ListReconciliationsFn            func(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error)
	CompleteReconciliationFn         func(ctx context.Context, schemaName, tenantID, reconciliationID string) error
	AddTransactionToReconciliationFn func(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error
	GetImportHistoryFn               func(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error)
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		accounts:        make(map[string]*BankAccount),
		transactions:    make(map[string]*BankTransaction),
		reconciliations: make(map[string]*BankReconciliation),
		imports:         make(map[string]*BankStatementImport),
	}
}

func (m *MockRepository) CreateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error {
	if m.CreateBankAccountFn != nil {
		return m.CreateBankAccountFn(ctx, schemaName, account)
	}
	m.accounts[account.ID] = account
	return nil
}

func (m *MockRepository) GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*BankAccount, error) {
	if m.GetBankAccountFn != nil {
		return m.GetBankAccountFn(ctx, schemaName, tenantID, accountID)
	}
	account, ok := m.accounts[accountID]
	if !ok || account.TenantID != tenantID {
		return nil, ErrBankAccountNotFound
	}
	return account, nil
}

func (m *MockRepository) ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error) {
	if m.ListBankAccountsFn != nil {
		return m.ListBankAccountsFn(ctx, schemaName, tenantID, filter)
	}
	var accounts []BankAccount
	for _, a := range m.accounts {
		if a.TenantID == tenantID {
			if filter != nil {
				if filter.IsActive != nil && a.IsActive != *filter.IsActive {
					continue
				}
				if filter.Currency != "" && a.Currency != filter.Currency {
					continue
				}
			}
			accounts = append(accounts, *a)
		}
	}
	return accounts, nil
}

func (m *MockRepository) UpdateBankAccount(ctx context.Context, schemaName string, account *BankAccount) error {
	if m.UpdateBankAccountFn != nil {
		return m.UpdateBankAccountFn(ctx, schemaName, account)
	}
	m.accounts[account.ID] = account
	return nil
}

func (m *MockRepository) DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	if m.DeleteBankAccountFn != nil {
		return m.DeleteBankAccountFn(ctx, schemaName, tenantID, accountID)
	}
	if _, ok := m.accounts[accountID]; !ok {
		return ErrBankAccountNotFound
	}
	delete(m.accounts, accountID)
	return nil
}

func (m *MockRepository) UnsetDefaultAccounts(ctx context.Context, schemaName, tenantID string) error {
	if m.UnsetDefaultAccountsFn != nil {
		return m.UnsetDefaultAccountsFn(ctx, schemaName, tenantID)
	}
	for _, a := range m.accounts {
		if a.TenantID == tenantID {
			a.IsDefault = false
		}
	}
	return nil
}

func (m *MockRepository) CountTransactionsForAccount(ctx context.Context, schemaName, accountID string) (int, error) {
	if m.CountTransactionsForAccountFn != nil {
		return m.CountTransactionsForAccountFn(ctx, schemaName, accountID)
	}
	count := 0
	for _, t := range m.transactions {
		if t.BankAccountID == accountID {
			count++
		}
	}
	return count, nil
}

func (m *MockRepository) CalculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error) {
	if m.CalculateAccountBalanceFn != nil {
		return m.CalculateAccountBalanceFn(ctx, schemaName, accountID)
	}
	balance := decimal.Zero
	for _, t := range m.transactions {
		if t.BankAccountID == accountID {
			balance = balance.Add(t.Amount)
		}
	}
	return balance, nil
}

func (m *MockRepository) ListTransactions(ctx context.Context, schemaName, tenantID string, filter *TransactionFilter) ([]BankTransaction, error) {
	if m.ListTransactionsFn != nil {
		return m.ListTransactionsFn(ctx, schemaName, tenantID, filter)
	}
	var transactions []BankTransaction
	for _, t := range m.transactions {
		if t.TenantID == tenantID {
			if filter != nil {
				if filter.BankAccountID != "" && t.BankAccountID != filter.BankAccountID {
					continue
				}
				if filter.Status != "" && t.Status != filter.Status {
					continue
				}
			}
			transactions = append(transactions, *t)
		}
	}
	return transactions, nil
}

func (m *MockRepository) GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*BankTransaction, error) {
	if m.GetTransactionFn != nil {
		return m.GetTransactionFn(ctx, schemaName, tenantID, transactionID)
	}
	t, ok := m.transactions[transactionID]
	if !ok || t.TenantID != tenantID {
		return nil, ErrTransactionNotFound
	}
	return t, nil
}

func (m *MockRepository) MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error {
	if m.MatchTransactionFn != nil {
		return m.MatchTransactionFn(ctx, schemaName, tenantID, transactionID, paymentID)
	}
	t, ok := m.transactions[transactionID]
	if !ok || t.TenantID != tenantID || t.Status != "UNMATCHED" {
		return ErrTransactionAlreadyMatched
	}
	t.Status = "MATCHED"
	t.MatchedPaymentID = &paymentID
	return nil
}

func (m *MockRepository) UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error {
	if m.UnmatchTransactionFn != nil {
		return m.UnmatchTransactionFn(ctx, schemaName, tenantID, transactionID)
	}
	t, ok := m.transactions[transactionID]
	if !ok || t.TenantID != tenantID || t.Status != "MATCHED" {
		return ErrTransactionNotMatched
	}
	t.Status = "UNMATCHED"
	t.MatchedPaymentID = nil
	return nil
}

func (m *MockRepository) CreateTransaction(ctx context.Context, schemaName string, t *BankTransaction) error {
	m.transactions[t.ID] = t
	return nil
}

func (m *MockRepository) IsTransactionDuplicate(ctx context.Context, schemaName, tenantID, bankAccountID string, date time.Time, amount decimal.Decimal, externalID string) (bool, error) {
	for _, t := range m.transactions {
		if t.TenantID == tenantID && t.BankAccountID == bankAccountID {
			if externalID != "" && t.ExternalID == externalID {
				return true, nil
			}
			if t.TransactionDate.Equal(date) && t.Amount.Equal(amount) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *MockRepository) CreateReconciliation(ctx context.Context, schemaName string, r *BankReconciliation) error {
	if m.CreateReconciliationFn != nil {
		return m.CreateReconciliationFn(ctx, schemaName, r)
	}
	m.reconciliations[r.ID] = r
	return nil
}

func (m *MockRepository) GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*BankReconciliation, error) {
	if m.GetReconciliationFn != nil {
		return m.GetReconciliationFn(ctx, schemaName, tenantID, reconciliationID)
	}
	r, ok := m.reconciliations[reconciliationID]
	if !ok || r.TenantID != tenantID {
		return nil, ErrReconciliationNotFound
	}
	return r, nil
}

func (m *MockRepository) ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankReconciliation, error) {
	if m.ListReconciliationsFn != nil {
		return m.ListReconciliationsFn(ctx, schemaName, tenantID, bankAccountID)
	}
	var reconciliations []BankReconciliation
	for _, r := range m.reconciliations {
		if r.TenantID == tenantID && r.BankAccountID == bankAccountID {
			reconciliations = append(reconciliations, *r)
		}
	}
	return reconciliations, nil
}

func (m *MockRepository) CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error {
	if m.CompleteReconciliationFn != nil {
		return m.CompleteReconciliationFn(ctx, schemaName, tenantID, reconciliationID)
	}
	r, ok := m.reconciliations[reconciliationID]
	if !ok || r.TenantID != tenantID || r.Status != ReconciliationInProgress {
		return ErrReconciliationAlreadyDone
	}
	r.Status = ReconciliationCompleted
	return nil
}

func (m *MockRepository) AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error {
	if m.AddTransactionToReconciliationFn != nil {
		return m.AddTransactionToReconciliationFn(ctx, schemaName, tenantID, transactionID, reconciliationID)
	}
	t, ok := m.transactions[transactionID]
	if !ok || t.TenantID != tenantID {
		return ErrTransactionNotFound
	}
	t.ReconciliationID = &reconciliationID
	return nil
}

func (m *MockRepository) CreateImportRecord(ctx context.Context, schemaName string, imp *BankStatementImport) error {
	m.imports[imp.ID] = imp
	return nil
}

func (m *MockRepository) GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]BankStatementImport, error) {
	if m.GetImportHistoryFn != nil {
		return m.GetImportHistoryFn(ctx, schemaName, tenantID, bankAccountID)
	}
	var imports []BankStatementImport
	for _, i := range m.imports {
		if i.TenantID == tenantID && i.BankAccountID == bankAccountID {
			imports = append(imports, *i)
		}
	}
	return imports, nil
}

// Test helpers
const (
	testTenantID   = "tenant-123"
	testSchemaName = "test_schema"
)

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	if service == nil {
		t.Error("NewServiceWithRepository should return a non-nil service")
		return
	}
	if service.repo == nil {
		t.Error("Service should have a repository")
	}
}

func TestService_CreateBankAccount(t *testing.T) {
	tests := []struct {
		name      string
		req       *CreateBankAccountRequest
		wantErr   bool
		checkFunc func(t *testing.T, account *BankAccount)
	}{
		{
			name: "valid bank account",
			req: &CreateBankAccountRequest{
				Name:          "Main Account",
				AccountNumber: "EE123456789012345678",
				BankName:      "Test Bank",
				SwiftCode:     "TESTEE22",
				Currency:      "EUR",
				IsDefault:     false,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, account *BankAccount) {
				if account.Name != "Main Account" {
					t.Errorf("expected name 'Main Account', got '%s'", account.Name)
				}
				if account.Currency != "EUR" {
					t.Errorf("expected currency 'EUR', got '%s'", account.Currency)
				}
				if !account.IsActive {
					t.Error("expected account to be active")
				}
			},
		},
		{
			name: "default currency",
			req: &CreateBankAccountRequest{
				Name:          "Second Account",
				AccountNumber: "EE987654321098765432",
				BankName:      "Another Bank",
				Currency:      "", // Should default to EUR
			},
			wantErr: false,
			checkFunc: func(t *testing.T, account *BankAccount) {
				if account.Currency != "EUR" {
					t.Errorf("expected default currency 'EUR', got '%s'", account.Currency)
				}
			},
		},
		{
			name: "default account unsets others",
			req: &CreateBankAccountRequest{
				Name:      "Default Account",
				BankName:  "Test Bank",
				IsDefault: true,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, account *BankAccount) {
				if !account.IsDefault {
					t.Error("expected account to be default")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			service := NewServiceWithRepository(repo)
			ctx := context.Background()

			account, err := service.CreateBankAccount(ctx, testSchemaName, testTenantID, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateBankAccount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, account)
			}
		})
	}
}

func TestService_CreateBankAccount_RepositoryError(t *testing.T) {
	repo := NewMockRepository()
	repo.CreateBankAccountFn = func(ctx context.Context, schemaName string, account *BankAccount) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.CreateBankAccount(ctx, testSchemaName, testTenantID, &CreateBankAccountRequest{
		Name:     "Test Account",
		BankName: "Test Bank",
	})
	if err == nil {
		t.Error("expected error from repository")
	}
}

func TestService_GetBankAccount(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	// Create an account first
	account := &BankAccount{
		ID:       "acc-123",
		TenantID: testTenantID,
		Name:     "Test Account",
		Currency: "EUR",
		IsActive: true,
	}
	repo.accounts[account.ID] = account

	// Add some transactions for balance calculation
	repo.transactions["tx-1"] = &BankTransaction{
		ID:            "tx-1",
		TenantID:      testTenantID,
		BankAccountID: "acc-123",
		Amount:        decimal.NewFromFloat(100.50),
	}
	repo.transactions["tx-2"] = &BankTransaction{
		ID:            "tx-2",
		TenantID:      testTenantID,
		BankAccountID: "acc-123",
		Amount:        decimal.NewFromFloat(-50.25),
	}

	// Test retrieval
	result, err := service.GetBankAccount(ctx, testSchemaName, testTenantID, "acc-123")
	if err != nil {
		t.Fatalf("GetBankAccount() error = %v", err)
	}
	if result.ID != account.ID {
		t.Errorf("expected ID %s, got %s", account.ID, result.ID)
	}

	expectedBalance := decimal.NewFromFloat(50.25)
	if !result.Balance.Equal(expectedBalance) {
		t.Errorf("expected balance %s, got %s", expectedBalance, result.Balance)
	}
}

func TestService_GetBankAccount_NotFound(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.GetBankAccount(ctx, testSchemaName, testTenantID, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent account")
	}
}

func TestService_ListBankAccounts(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	// Create some accounts
	repo.accounts["acc-1"] = &BankAccount{ID: "acc-1", TenantID: testTenantID, Name: "Account 1", Currency: "EUR", IsActive: true}
	repo.accounts["acc-2"] = &BankAccount{ID: "acc-2", TenantID: testTenantID, Name: "Account 2", Currency: "USD", IsActive: true}
	repo.accounts["acc-3"] = &BankAccount{ID: "acc-3", TenantID: testTenantID, Name: "Account 3", Currency: "EUR", IsActive: false}
	repo.accounts["other"] = &BankAccount{ID: "other", TenantID: "other-tenant", Name: "Other", Currency: "EUR", IsActive: true}

	// Test listing all
	accounts, err := service.ListBankAccounts(ctx, testSchemaName, testTenantID, nil)
	if err != nil {
		t.Fatalf("ListBankAccounts() error = %v", err)
	}
	if len(accounts) != 3 {
		t.Errorf("expected 3 accounts, got %d", len(accounts))
	}

	// Test with filter
	active := true
	accounts, err = service.ListBankAccounts(ctx, testSchemaName, testTenantID, &BankAccountFilter{IsActive: &active})
	if err != nil {
		t.Fatalf("ListBankAccounts() with filter error = %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("expected 2 active accounts, got %d", len(accounts))
	}

	// Test currency filter
	accounts, err = service.ListBankAccounts(ctx, testSchemaName, testTenantID, &BankAccountFilter{Currency: "EUR"})
	if err != nil {
		t.Fatalf("ListBankAccounts() with currency filter error = %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("expected 2 EUR accounts, got %d", len(accounts))
	}
}

func TestService_UpdateBankAccount(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	// Create an account
	repo.accounts["acc-123"] = &BankAccount{
		ID:        "acc-123",
		TenantID:  testTenantID,
		Name:      "Original Name",
		BankName:  "Original Bank",
		SwiftCode: "ORIG1234",
		Currency:  "EUR",
		IsActive:  true,
		IsDefault: false,
	}

	// Update the account
	isDefault := true
	isActive := false
	glID := "gl-123"
	result, err := service.UpdateBankAccount(ctx, testSchemaName, testTenantID, "acc-123", &UpdateBankAccountRequest{
		Name:        "Updated Name",
		BankName:    "Updated Bank",
		SwiftCode:   "UPDT5678",
		GLAccountID: &glID,
		IsDefault:   &isDefault,
		IsActive:    &isActive,
	})
	if err != nil {
		t.Fatalf("UpdateBankAccount() error = %v", err)
	}

	if result.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", result.Name)
	}
	if result.BankName != "Updated Bank" {
		t.Errorf("expected bank name 'Updated Bank', got '%s'", result.BankName)
	}
	if !result.IsDefault {
		t.Error("expected account to be default")
	}
	if result.IsActive {
		t.Error("expected account to be inactive")
	}
}

func TestService_UpdateBankAccount_NotFound(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.UpdateBankAccount(ctx, testSchemaName, testTenantID, "nonexistent", &UpdateBankAccountRequest{Name: "Test"})
	if err == nil {
		t.Error("expected error for non-existent account")
	}
}

func TestService_DeleteBankAccount(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	// Create an account
	repo.accounts["acc-123"] = &BankAccount{ID: "acc-123", TenantID: testTenantID}

	// Delete it
	err := service.DeleteBankAccount(ctx, testSchemaName, testTenantID, "acc-123")
	if err != nil {
		t.Fatalf("DeleteBankAccount() error = %v", err)
	}

	// Verify deletion
	if _, ok := repo.accounts["acc-123"]; ok {
		t.Error("account should have been deleted")
	}
}

func TestService_DeleteBankAccount_WithTransactions(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	// Create an account with transactions
	repo.accounts["acc-123"] = &BankAccount{ID: "acc-123", TenantID: testTenantID}
	repo.transactions["tx-1"] = &BankTransaction{ID: "tx-1", TenantID: testTenantID, BankAccountID: "acc-123"}

	// Try to delete
	err := service.DeleteBankAccount(ctx, testSchemaName, testTenantID, "acc-123")
	if err == nil {
		t.Error("expected error when deleting account with transactions")
	}
	if err != ErrAccountHasTransactions {
		t.Errorf("expected ErrAccountHasTransactions, got %v", err)
	}
}

func TestService_ListTransactions(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	// Create some transactions
	repo.transactions["tx-1"] = &BankTransaction{ID: "tx-1", TenantID: testTenantID, BankAccountID: "acc-1", Status: "UNMATCHED"}
	repo.transactions["tx-2"] = &BankTransaction{ID: "tx-2", TenantID: testTenantID, BankAccountID: "acc-1", Status: "MATCHED"}
	repo.transactions["tx-3"] = &BankTransaction{ID: "tx-3", TenantID: testTenantID, BankAccountID: "acc-2", Status: "UNMATCHED"}
	repo.transactions["other"] = &BankTransaction{ID: "other", TenantID: "other-tenant", BankAccountID: "acc-1", Status: "UNMATCHED"}

	// List all
	transactions, err := service.ListTransactions(ctx, testSchemaName, testTenantID, nil)
	if err != nil {
		t.Fatalf("ListTransactions() error = %v", err)
	}
	if len(transactions) != 3 {
		t.Errorf("expected 3 transactions, got %d", len(transactions))
	}

	// Filter by account
	transactions, err = service.ListTransactions(ctx, testSchemaName, testTenantID, &TransactionFilter{BankAccountID: "acc-1"})
	if err != nil {
		t.Fatalf("ListTransactions() with account filter error = %v", err)
	}
	if len(transactions) != 2 {
		t.Errorf("expected 2 transactions for acc-1, got %d", len(transactions))
	}

	// Filter by status
	transactions, err = service.ListTransactions(ctx, testSchemaName, testTenantID, &TransactionFilter{Status: "UNMATCHED"})
	if err != nil {
		t.Fatalf("ListTransactions() with status filter error = %v", err)
	}
	if len(transactions) != 2 {
		t.Errorf("expected 2 unmatched transactions, got %d", len(transactions))
	}
}

func TestService_GetTransaction(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.transactions["tx-123"] = &BankTransaction{
		ID:          "tx-123",
		TenantID:    testTenantID,
		Description: "Test Transaction",
		Amount:      decimal.NewFromFloat(100.00),
	}

	result, err := service.GetTransaction(ctx, testSchemaName, testTenantID, "tx-123")
	if err != nil {
		t.Fatalf("GetTransaction() error = %v", err)
	}
	if result.ID != "tx-123" {
		t.Errorf("expected ID 'tx-123', got '%s'", result.ID)
	}
}

func TestService_GetTransaction_NotFound(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.GetTransaction(ctx, testSchemaName, testTenantID, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent transaction")
	}
}

func TestService_MatchTransaction(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.transactions["tx-123"] = &BankTransaction{
		ID:       "tx-123",
		TenantID: testTenantID,
		Status:   "UNMATCHED",
	}

	err := service.MatchTransaction(ctx, testSchemaName, testTenantID, "tx-123", "payment-456")
	if err != nil {
		t.Fatalf("MatchTransaction() error = %v", err)
	}

	// Verify the transaction was matched
	if repo.transactions["tx-123"].Status != "MATCHED" {
		t.Error("expected transaction status to be MATCHED")
	}
	if repo.transactions["tx-123"].MatchedPaymentID == nil || *repo.transactions["tx-123"].MatchedPaymentID != "payment-456" {
		t.Error("expected matched payment ID to be set")
	}
}

func TestService_MatchTransaction_AlreadyMatched(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	paymentID := "existing-payment"
	repo.transactions["tx-123"] = &BankTransaction{
		ID:               "tx-123",
		TenantID:         testTenantID,
		Status:           "MATCHED",
		MatchedPaymentID: &paymentID,
	}

	err := service.MatchTransaction(ctx, testSchemaName, testTenantID, "tx-123", "new-payment")
	if err == nil {
		t.Error("expected error for already matched transaction")
	}
}

func TestService_UnmatchTransaction(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	paymentID := "payment-456"
	repo.transactions["tx-123"] = &BankTransaction{
		ID:               "tx-123",
		TenantID:         testTenantID,
		Status:           "MATCHED",
		MatchedPaymentID: &paymentID,
	}

	err := service.UnmatchTransaction(ctx, testSchemaName, testTenantID, "tx-123")
	if err != nil {
		t.Fatalf("UnmatchTransaction() error = %v", err)
	}

	// Verify the transaction was unmatched
	if repo.transactions["tx-123"].Status != "UNMATCHED" {
		t.Error("expected transaction status to be UNMATCHED")
	}
	if repo.transactions["tx-123"].MatchedPaymentID != nil {
		t.Error("expected matched payment ID to be nil")
	}
}

func TestService_UnmatchTransaction_NotMatched(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.transactions["tx-123"] = &BankTransaction{
		ID:       "tx-123",
		TenantID: testTenantID,
		Status:   "UNMATCHED",
	}

	err := service.UnmatchTransaction(ctx, testSchemaName, testTenantID, "tx-123")
	if err == nil {
		t.Error("expected error for unmatched transaction")
	}
}

func TestService_CreateReconciliation(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	result, err := service.CreateReconciliation(ctx, testSchemaName, testTenantID, "acc-123", "user-456", &CreateReconciliationRequest{
		StatementDate:  "2024-01-15",
		OpeningBalance: decimal.NewFromFloat(1000.00),
		ClosingBalance: decimal.NewFromFloat(1500.00),
	})
	if err != nil {
		t.Fatalf("CreateReconciliation() error = %v", err)
	}

	if result.BankAccountID != "acc-123" {
		t.Errorf("expected bank account ID 'acc-123', got '%s'", result.BankAccountID)
	}
	if result.Status != ReconciliationInProgress {
		t.Errorf("expected status IN_PROGRESS, got '%s'", result.Status)
	}
	if !result.OpeningBalance.Equal(decimal.NewFromFloat(1000.00)) {
		t.Errorf("expected opening balance 1000, got %s", result.OpeningBalance)
	}
}

func TestService_CreateReconciliation_InvalidDate(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.CreateReconciliation(ctx, testSchemaName, testTenantID, "acc-123", "user-456", &CreateReconciliationRequest{
		StatementDate:  "invalid-date",
		OpeningBalance: decimal.NewFromFloat(1000.00),
		ClosingBalance: decimal.NewFromFloat(1500.00),
	})
	if err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestService_GetReconciliation(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.reconciliations["rec-123"] = &BankReconciliation{
		ID:             "rec-123",
		TenantID:       testTenantID,
		BankAccountID:  "acc-456",
		Status:         ReconciliationInProgress,
		OpeningBalance: decimal.NewFromFloat(1000.00),
	}

	result, err := service.GetReconciliation(ctx, testSchemaName, testTenantID, "rec-123")
	if err != nil {
		t.Fatalf("GetReconciliation() error = %v", err)
	}
	if result.ID != "rec-123" {
		t.Errorf("expected ID 'rec-123', got '%s'", result.ID)
	}
}

func TestService_GetReconciliation_NotFound(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.GetReconciliation(ctx, testSchemaName, testTenantID, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent reconciliation")
	}
}

func TestService_ListReconciliations(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.reconciliations["rec-1"] = &BankReconciliation{ID: "rec-1", TenantID: testTenantID, BankAccountID: "acc-123"}
	repo.reconciliations["rec-2"] = &BankReconciliation{ID: "rec-2", TenantID: testTenantID, BankAccountID: "acc-123"}
	repo.reconciliations["rec-3"] = &BankReconciliation{ID: "rec-3", TenantID: testTenantID, BankAccountID: "acc-456"}
	repo.reconciliations["other"] = &BankReconciliation{ID: "other", TenantID: "other-tenant", BankAccountID: "acc-123"}

	reconciliations, err := service.ListReconciliations(ctx, testSchemaName, testTenantID, "acc-123")
	if err != nil {
		t.Fatalf("ListReconciliations() error = %v", err)
	}
	if len(reconciliations) != 2 {
		t.Errorf("expected 2 reconciliations, got %d", len(reconciliations))
	}
}

func TestService_CompleteReconciliation(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.reconciliations["rec-123"] = &BankReconciliation{
		ID:       "rec-123",
		TenantID: testTenantID,
		Status:   ReconciliationInProgress,
	}

	err := service.CompleteReconciliation(ctx, testSchemaName, testTenantID, "rec-123")
	if err != nil {
		t.Fatalf("CompleteReconciliation() error = %v", err)
	}

	if repo.reconciliations["rec-123"].Status != ReconciliationCompleted {
		t.Error("expected reconciliation to be completed")
	}
}

func TestService_CompleteReconciliation_AlreadyCompleted(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.reconciliations["rec-123"] = &BankReconciliation{
		ID:       "rec-123",
		TenantID: testTenantID,
		Status:   ReconciliationCompleted,
	}

	err := service.CompleteReconciliation(ctx, testSchemaName, testTenantID, "rec-123")
	if err == nil {
		t.Error("expected error for already completed reconciliation")
	}
}

func TestService_AddTransactionToReconciliation(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.transactions["tx-123"] = &BankTransaction{ID: "tx-123", TenantID: testTenantID}
	repo.reconciliations["rec-456"] = &BankReconciliation{ID: "rec-456", TenantID: testTenantID}

	err := service.AddTransactionToReconciliation(ctx, testSchemaName, testTenantID, "tx-123", "rec-456")
	if err != nil {
		t.Fatalf("AddTransactionToReconciliation() error = %v", err)
	}

	if repo.transactions["tx-123"].ReconciliationID == nil || *repo.transactions["tx-123"].ReconciliationID != "rec-456" {
		t.Error("expected transaction to be added to reconciliation")
	}
}

func TestService_AddTransactionToReconciliation_NotFound(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	err := service.AddTransactionToReconciliation(ctx, testSchemaName, testTenantID, "nonexistent", "rec-456")
	if err == nil {
		t.Error("expected error for non-existent transaction")
	}
}

func TestService_GetImportHistory(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.imports["imp-1"] = &BankStatementImport{ID: "imp-1", TenantID: testTenantID, BankAccountID: "acc-123", FileName: "file1.csv"}
	repo.imports["imp-2"] = &BankStatementImport{ID: "imp-2", TenantID: testTenantID, BankAccountID: "acc-123", FileName: "file2.csv"}
	repo.imports["imp-3"] = &BankStatementImport{ID: "imp-3", TenantID: testTenantID, BankAccountID: "acc-456", FileName: "file3.csv"}
	repo.imports["other"] = &BankStatementImport{ID: "other", TenantID: "other-tenant", BankAccountID: "acc-123", FileName: "other.csv"}

	imports, err := service.GetImportHistory(ctx, testSchemaName, testTenantID, "acc-123")
	if err != nil {
		t.Fatalf("GetImportHistory() error = %v", err)
	}
	if len(imports) != 2 {
		t.Errorf("expected 2 imports, got %d", len(imports))
	}
}

func TestService_ListBankAccounts_Error(t *testing.T) {
	repo := NewMockRepository()
	repo.ListBankAccountsFn = func(ctx context.Context, schemaName, tenantID string, filter *BankAccountFilter) ([]BankAccount, error) {
		return nil, errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.ListBankAccounts(ctx, testSchemaName, testTenantID, nil)
	if err == nil {
		t.Error("expected error")
	}
}

func TestService_UpdateBankAccount_UnsetDefaultError(t *testing.T) {
	repo := NewMockRepository()
	repo.accounts["acc-123"] = &BankAccount{
		ID:        "acc-123",
		TenantID:  testTenantID,
		IsDefault: false,
	}
	repo.UnsetDefaultAccountsFn = func(ctx context.Context, schemaName, tenantID string) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	isDefault := true
	_, err := service.UpdateBankAccount(ctx, testSchemaName, testTenantID, "acc-123", &UpdateBankAccountRequest{
		IsDefault: &isDefault,
	})
	if err == nil {
		t.Error("expected error when unset defaults fails")
	}
}

func TestService_UpdateBankAccount_UpdateError(t *testing.T) {
	repo := NewMockRepository()
	repo.accounts["acc-123"] = &BankAccount{
		ID:       "acc-123",
		TenantID: testTenantID,
	}
	repo.UpdateBankAccountFn = func(ctx context.Context, schemaName string, account *BankAccount) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.UpdateBankAccount(ctx, testSchemaName, testTenantID, "acc-123", &UpdateBankAccountRequest{
		Name: "New Name",
	})
	if err == nil {
		t.Error("expected error when update fails")
	}
}

func TestService_DeleteBankAccount_CountError(t *testing.T) {
	repo := NewMockRepository()
	repo.accounts["acc-123"] = &BankAccount{
		ID:       "acc-123",
		TenantID: testTenantID,
	}
	repo.CountTransactionsForAccountFn = func(ctx context.Context, schemaName, accountID string) (int, error) {
		return 0, errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	err := service.DeleteBankAccount(ctx, testSchemaName, testTenantID, "acc-123")
	if err == nil {
		t.Error("expected error when count fails")
	}
}

func TestService_CreateReconciliation_RepoError(t *testing.T) {
	repo := NewMockRepository()
	repo.CreateReconciliationFn = func(ctx context.Context, schemaName string, r *BankReconciliation) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.CreateReconciliation(ctx, testSchemaName, testTenantID, "acc-123", "user-456", &CreateReconciliationRequest{
		StatementDate:  "2024-01-15",
		OpeningBalance: decimal.NewFromFloat(1000.00),
		ClosingBalance: decimal.NewFromFloat(1500.00),
	})
	if err == nil {
		t.Error("expected error when create fails")
	}
}

func TestService_CreateBankAccount_RepoError(t *testing.T) {
	repo := NewMockRepository()
	repo.CreateBankAccountFn = func(ctx context.Context, schemaName string, account *BankAccount) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.CreateBankAccount(ctx, testSchemaName, testTenantID, &CreateBankAccountRequest{
		Name:          "Test Account",
		AccountNumber: "EE123456789",
		BankName:      "Test Bank",
		Currency:      "EUR",
	})
	if err == nil {
		t.Error("expected error when create fails")
	}
}

func TestService_CreateBankAccount_UnsetDefaultError(t *testing.T) {
	repo := NewMockRepository()
	repo.UnsetDefaultAccountsFn = func(ctx context.Context, schemaName, tenantID string) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	_, err := service.CreateBankAccount(ctx, testSchemaName, testTenantID, &CreateBankAccountRequest{
		Name:          "Test Account",
		AccountNumber: "EE123456789",
		BankName:      "Test Bank",
		Currency:      "EUR",
		IsDefault:     true,
	})
	if err == nil {
		t.Error("expected error when unset defaults fails")
	}
}

// Test error types
func TestErrorTypes(t *testing.T) {
	if ErrBankAccountNotFound == nil {
		t.Error("ErrBankAccountNotFound should not be nil")
	}
	if ErrTransactionNotFound == nil {
		t.Error("ErrTransactionNotFound should not be nil")
	}
	if ErrReconciliationNotFound == nil {
		t.Error("ErrReconciliationNotFound should not be nil")
	}
	if ErrAccountHasTransactions == nil {
		t.Error("ErrAccountHasTransactions should not be nil")
	}
	if ErrTransactionAlreadyMatched == nil {
		t.Error("ErrTransactionAlreadyMatched should not be nil")
	}
	if ErrTransactionNotMatched == nil {
		t.Error("ErrTransactionNotMatched should not be nil")
	}
	if ErrReconciliationAlreadyDone == nil {
		t.Error("ErrReconciliationAlreadyDone should not be nil")
	}
}
