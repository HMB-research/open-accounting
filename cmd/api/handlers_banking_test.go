package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// mockBankingRepository implements banking.Repository for testing
type mockBankingRepository struct {
	accounts        map[string]*banking.BankAccount
	transactions    map[string]*banking.BankTransaction
	reconciliations map[string]*banking.BankReconciliation
	imports         map[string][]banking.BankStatementImport
	txCount         map[string]int // accountID -> transaction count

	createAccErr     error
	getAccErr        error
	listAccErr       error
	updateAccErr     error
	deleteAccErr     error
	listTxErr        error
	getTxErr         error
	matchErr         error
	unmatchErr       error
	createTxErr      error
	createRecErr     error
	getRecErr        error
	listRecErr       error
	completeRecErr   error
	importRecordErr  error
	importHistoryErr error
}

func newMockBankingRepository() *mockBankingRepository {
	return &mockBankingRepository{
		accounts:        make(map[string]*banking.BankAccount),
		transactions:    make(map[string]*banking.BankTransaction),
		reconciliations: make(map[string]*banking.BankReconciliation),
		imports:         make(map[string][]banking.BankStatementImport),
		txCount:         make(map[string]int),
	}
}

func (m *mockBankingRepository) CreateBankAccount(ctx context.Context, schemaName string, account *banking.BankAccount) error {
	if m.createAccErr != nil {
		return m.createAccErr
	}
	m.accounts[account.ID] = account
	return nil
}

func (m *mockBankingRepository) GetBankAccount(ctx context.Context, schemaName, tenantID, accountID string) (*banking.BankAccount, error) {
	if m.getAccErr != nil {
		return nil, m.getAccErr
	}
	if acc, ok := m.accounts[accountID]; ok && acc.TenantID == tenantID {
		return acc, nil
	}
	return nil, banking.ErrBankAccountNotFound
}

func (m *mockBankingRepository) ListBankAccounts(ctx context.Context, schemaName, tenantID string, filter *banking.BankAccountFilter) ([]banking.BankAccount, error) {
	if m.listAccErr != nil {
		return nil, m.listAccErr
	}
	var result []banking.BankAccount
	for _, acc := range m.accounts {
		if acc.TenantID != tenantID {
			continue
		}
		if filter != nil && filter.IsActive != nil && acc.IsActive != *filter.IsActive {
			continue
		}
		result = append(result, *acc)
	}
	return result, nil
}

func (m *mockBankingRepository) UpdateBankAccount(ctx context.Context, schemaName string, account *banking.BankAccount) error {
	if m.updateAccErr != nil {
		return m.updateAccErr
	}
	m.accounts[account.ID] = account
	return nil
}

func (m *mockBankingRepository) DeleteBankAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	if m.deleteAccErr != nil {
		return m.deleteAccErr
	}
	if _, ok := m.accounts[accountID]; !ok {
		return banking.ErrBankAccountNotFound
	}
	delete(m.accounts, accountID)
	return nil
}

func (m *mockBankingRepository) UnsetDefaultAccounts(ctx context.Context, schemaName, tenantID string) error {
	for _, acc := range m.accounts {
		if acc.TenantID == tenantID {
			acc.IsDefault = false
		}
	}
	return nil
}

func (m *mockBankingRepository) CountTransactionsForAccount(ctx context.Context, schemaName, accountID string) (int, error) {
	return m.txCount[accountID], nil
}

func (m *mockBankingRepository) CalculateAccountBalance(ctx context.Context, schemaName, accountID string) (decimal.Decimal, error) {
	balance := decimal.Zero
	for _, tx := range m.transactions {
		if tx.BankAccountID == accountID {
			balance = balance.Add(tx.Amount)
		}
	}
	return balance, nil
}

func (m *mockBankingRepository) ListTransactions(ctx context.Context, schemaName, tenantID string, filter *banking.TransactionFilter) ([]banking.BankTransaction, error) {
	if m.listTxErr != nil {
		return nil, m.listTxErr
	}
	var result []banking.BankTransaction
	for _, tx := range m.transactions {
		if tx.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.BankAccountID != "" && tx.BankAccountID != filter.BankAccountID {
				continue
			}
			if filter.Status != "" && tx.Status != filter.Status {
				continue
			}
		}
		result = append(result, *tx)
	}
	return result, nil
}

func (m *mockBankingRepository) GetTransaction(ctx context.Context, schemaName, tenantID, transactionID string) (*banking.BankTransaction, error) {
	if m.getTxErr != nil {
		return nil, m.getTxErr
	}
	if tx, ok := m.transactions[transactionID]; ok && tx.TenantID == tenantID {
		return tx, nil
	}
	return nil, banking.ErrTransactionNotFound
}

func (m *mockBankingRepository) MatchTransaction(ctx context.Context, schemaName, tenantID, transactionID, paymentID string) error {
	if m.matchErr != nil {
		return m.matchErr
	}
	tx, ok := m.transactions[transactionID]
	if !ok || tx.TenantID != tenantID || tx.Status != banking.StatusUnmatched {
		return banking.ErrTransactionAlreadyMatched
	}
	tx.MatchedPaymentID = &paymentID
	tx.Status = banking.StatusMatched
	return nil
}

func (m *mockBankingRepository) UnmatchTransaction(ctx context.Context, schemaName, tenantID, transactionID string) error {
	if m.unmatchErr != nil {
		return m.unmatchErr
	}
	tx, ok := m.transactions[transactionID]
	if !ok || tx.TenantID != tenantID || tx.Status != banking.StatusMatched {
		return banking.ErrTransactionNotMatched
	}
	tx.MatchedPaymentID = nil
	tx.Status = banking.StatusUnmatched
	return nil
}

func (m *mockBankingRepository) CreateTransaction(ctx context.Context, schemaName string, t *banking.BankTransaction) error {
	if m.createTxErr != nil {
		return m.createTxErr
	}
	m.transactions[t.ID] = t
	m.txCount[t.BankAccountID]++
	return nil
}

func (m *mockBankingRepository) IsTransactionDuplicate(ctx context.Context, schemaName, tenantID, bankAccountID string, date time.Time, amount decimal.Decimal, externalID string) (bool, error) {
	for _, tx := range m.transactions {
		if tx.TenantID == tenantID && tx.BankAccountID == bankAccountID {
			if externalID != "" && tx.ExternalID == externalID {
				return true, nil
			}
			if tx.TransactionDate.Equal(date) && tx.Amount.Equal(amount) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *mockBankingRepository) CreateReconciliation(ctx context.Context, schemaName string, r *banking.BankReconciliation) error {
	if m.createRecErr != nil {
		return m.createRecErr
	}
	m.reconciliations[r.ID] = r
	return nil
}

func (m *mockBankingRepository) GetReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) (*banking.BankReconciliation, error) {
	if m.getRecErr != nil {
		return nil, m.getRecErr
	}
	if rec, ok := m.reconciliations[reconciliationID]; ok && rec.TenantID == tenantID {
		return rec, nil
	}
	return nil, banking.ErrReconciliationNotFound
}

func (m *mockBankingRepository) ListReconciliations(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]banking.BankReconciliation, error) {
	if m.listRecErr != nil {
		return nil, m.listRecErr
	}
	var result []banking.BankReconciliation
	for _, rec := range m.reconciliations {
		if rec.TenantID == tenantID && rec.BankAccountID == bankAccountID {
			result = append(result, *rec)
		}
	}
	return result, nil
}

func (m *mockBankingRepository) CompleteReconciliation(ctx context.Context, schemaName, tenantID, reconciliationID string) error {
	if m.completeRecErr != nil {
		return m.completeRecErr
	}
	rec, ok := m.reconciliations[reconciliationID]
	if !ok || rec.TenantID != tenantID || rec.Status != banking.ReconciliationInProgress {
		return banking.ErrReconciliationAlreadyDone
	}
	now := time.Now()
	rec.Status = banking.ReconciliationCompleted
	rec.CompletedAt = &now
	return nil
}

func (m *mockBankingRepository) AddTransactionToReconciliation(ctx context.Context, schemaName, tenantID, transactionID, reconciliationID string) error {
	tx, ok := m.transactions[transactionID]
	if !ok || tx.TenantID != tenantID {
		return banking.ErrTransactionNotFound
	}
	tx.ReconciliationID = &reconciliationID
	return nil
}

func (m *mockBankingRepository) CreateImportRecord(ctx context.Context, schemaName string, imp *banking.BankStatementImport) error {
	if m.importRecordErr != nil {
		return m.importRecordErr
	}
	m.imports[imp.BankAccountID] = append(m.imports[imp.BankAccountID], *imp)
	return nil
}

func (m *mockBankingRepository) GetImportHistory(ctx context.Context, schemaName, tenantID, bankAccountID string) ([]banking.BankStatementImport, error) {
	if m.importHistoryErr != nil {
		return nil, m.importHistoryErr
	}
	return m.imports[bankAccountID], nil
}

func setupBankingTestHandlers() (*Handlers, *mockBankingRepository, *mockTenantRepository) {
	bankingRepo := newMockBankingRepository()
	bankingSvc := banking.NewServiceWithRepository(bankingRepo)

	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{
		bankingService: bankingSvc,
		tenantService:  tenantSvc,
	}
	return h, bankingRepo, tenantRepo
}

func TestListBankAccounts(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.accounts["acc-1"] = &banking.BankAccount{
		ID:       "acc-1",
		TenantID: "tenant-1",
		Name:     "Main Account",
		IsActive: true,
	}
	repo.accounts["acc-2"] = &banking.BankAccount{
		ID:       "acc-2",
		TenantID: "tenant-1",
		Name:     "Inactive Account",
		IsActive: false,
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "list all accounts",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "list active only",
			query:      "?active_only=true",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ListBankAccounts(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []banking.BankAccount
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestCreateBankAccount(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid account",
			body: map[string]interface{}{
				"name":           "Business Account",
				"account_number": "123456789",
				"bank_name":      "Test Bank",
				"currency":       "EUR",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"account_number": "123456789",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "required",
		},
		{
			name: "missing account number",
			body: map[string]interface{}{
				"name": "Business Account",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "required",
		},
		{
			name:       "invalid JSON",
			body:       nil,
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, tenantRepo := setupBankingTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("{invalid")
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CreateBankAccount(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}

			if tt.wantStatus == http.StatusCreated {
				var result banking.BankAccount
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.ID)
				assert.Equal(t, "Business Account", result.Name)
			}
		})
	}
}

func TestGetBankAccount(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.accounts["acc-1"] = &banking.BankAccount{
		ID:            "acc-1",
		TenantID:      "tenant-1",
		Name:          "Main Account",
		AccountNumber: "123456789",
	}

	tests := []struct {
		name       string
		accountID  string
		wantStatus int
	}{
		{
			name:       "existing account",
			accountID:  "acc-1",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-existent account",
			accountID:  "acc-999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/"+tt.accountID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": tt.accountID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetBankAccount(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result banking.BankAccount
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Equal(t, tt.accountID, result.ID)
			}
		})
	}
}

func TestUpdateBankAccount(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.accounts["acc-1"] = &banking.BankAccount{
		ID:            "acc-1",
		TenantID:      "tenant-1",
		Name:          "Original Name",
		AccountNumber: "123456789",
		IsActive:      true,
	}

	tests := []struct {
		name       string
		accountID  string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name:      "update name",
			accountID: "acc-1",
			body: map[string]interface{}{
				"name": "Updated Name",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "update active status",
			accountID: "acc-1",
			body: map[string]interface{}{
				"is_active": false,
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/bank-accounts/"+tt.accountID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": tt.accountID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.UpdateBankAccount(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestDeleteBankAccount(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mockBankingRepository)
		accountID  string
		wantStatus int
	}{
		{
			name: "delete account without transactions",
			setupRepo: func(repo *mockBankingRepository) {
				repo.accounts["acc-1"] = &banking.BankAccount{
					ID:       "acc-1",
					TenantID: "tenant-1",
					Name:     "Empty Account",
				}
				repo.txCount["acc-1"] = 0
			},
			accountID:  "acc-1",
			wantStatus: http.StatusNoContent,
		},
		{
			name: "delete account with transactions rejected",
			setupRepo: func(repo *mockBankingRepository) {
				repo.accounts["acc-2"] = &banking.BankAccount{
					ID:       "acc-2",
					TenantID: "tenant-1",
					Name:     "Account With Transactions",
				}
				repo.txCount["acc-2"] = 5
			},
			accountID:  "acc-2",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupBankingTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			req := httptest.NewRequest(http.MethodDelete, "/tenants/tenant-1/bank-accounts/"+tt.accountID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": tt.accountID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.DeleteBankAccount(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestListBankTransactions(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.transactions["tx-1"] = &banking.BankTransaction{
		ID:            "tx-1",
		TenantID:      "tenant-1",
		BankAccountID: "acc-1",
		Status:        banking.StatusUnmatched,
		Amount:        decimal.NewFromInt(100),
	}
	repo.transactions["tx-2"] = &banking.BankTransaction{
		ID:            "tx-2",
		TenantID:      "tenant-1",
		BankAccountID: "acc-1",
		Status:        banking.StatusMatched,
		Amount:        decimal.NewFromInt(50),
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "list all transactions",
			query:      "",
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "filter by status UNMATCHED",
			query:      "?status=UNMATCHED",
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/acc-1/transactions"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ListBankTransactions(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []banking.BankTransaction
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestGetBankTransaction(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.transactions["tx-1"] = &banking.BankTransaction{
		ID:            "tx-1",
		TenantID:      "tenant-1",
		BankAccountID: "acc-1",
		Amount:        decimal.NewFromInt(100),
	}

	tests := []struct {
		name          string
		transactionID string
		wantStatus    int
	}{
		{
			name:          "existing transaction",
			transactionID: "tx-1",
			wantStatus:    http.StatusOK,
		},
		{
			name:          "non-existent transaction",
			transactionID: "tx-999",
			wantStatus:    http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/bank-transactions/"+tt.transactionID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "transactionID": tt.transactionID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetBankTransaction(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestMatchBankTransaction(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mockBankingRepository)
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid match",
			setupRepo: func(repo *mockBankingRepository) {
				repo.transactions["tx-1"] = &banking.BankTransaction{
					ID:       "tx-1",
					TenantID: "tenant-1",
					Status:   banking.StatusUnmatched,
					Amount:   decimal.NewFromInt(100),
				}
			},
			body: map[string]interface{}{
				"payment_id": "payment-1",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing payment_id",
			setupRepo: func(repo *mockBankingRepository) {
				repo.transactions["tx-1"] = &banking.BankTransaction{
					ID:       "tx-1",
					TenantID: "tenant-1",
					Status:   banking.StatusUnmatched,
				}
			},
			body:       map[string]interface{}{},
			wantStatus: http.StatusBadRequest,
			wantErr:    "Payment ID",
		},
		{
			name: "already matched",
			setupRepo: func(repo *mockBankingRepository) {
				paymentID := "already-matched"
				repo.transactions["tx-1"] = &banking.BankTransaction{
					ID:               "tx-1",
					TenantID:         "tenant-1",
					Status:           banking.StatusMatched,
					MatchedPaymentID: &paymentID,
				}
			},
			body: map[string]interface{}{
				"payment_id": "payment-2",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupBankingTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/tx-1/match", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "transactionID": "tx-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.MatchBankTransaction(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}
		})
	}
}

func TestUnmatchBankTransaction(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mockBankingRepository)
		wantStatus int
	}{
		{
			name: "valid unmatch",
			setupRepo: func(repo *mockBankingRepository) {
				paymentID := "payment-1"
				repo.transactions["tx-1"] = &banking.BankTransaction{
					ID:               "tx-1",
					TenantID:         "tenant-1",
					Status:           banking.StatusMatched,
					MatchedPaymentID: &paymentID,
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "not matched",
			setupRepo: func(repo *mockBankingRepository) {
				repo.transactions["tx-1"] = &banking.BankTransaction{
					ID:       "tx-1",
					TenantID: "tenant-1",
					Status:   banking.StatusUnmatched,
				}
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupBankingTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/tx-1/unmatch", nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "transactionID": "tx-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.UnmatchBankTransaction(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestImportBankTransactions(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "empty transactions",
			body: map[string]interface{}{
				"transactions": []map[string]string{},
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    "No transactions",
		},
		{
			name:       "invalid JSON",
			body:       nil,
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid",
		},
		// Note: Valid import tests require database integration testing
		// as ImportTransactions uses database transactions directly
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupBankingTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			repo.accounts["acc-1"] = &banking.BankAccount{
				ID:       "acc-1",
				TenantID: "tenant-1",
				Currency: "EUR",
			}

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("{invalid")
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/acc-1/import", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.ImportBankTransactions(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}
		})
	}
}

func TestListReconciliations(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.reconciliations["rec-1"] = &banking.BankReconciliation{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		BankAccountID: "acc-1",
		Status:        banking.ReconciliationCompleted,
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/acc-1/reconciliations", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.ListReconciliations(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result []banking.BankReconciliation
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestCreateReconciliation(t *testing.T) {
	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantErr    string
	}{
		{
			name: "valid reconciliation",
			body: map[string]interface{}{
				"statement_date":  "2026-01-31",
				"opening_balance": "1000.00",
				"closing_balance": "1500.00",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid JSON",
			body:       nil,
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupBankingTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			repo.accounts["acc-1"] = &banking.BankAccount{
				ID:       "acc-1",
				TenantID: "tenant-1",
			}

			var body []byte
			if tt.body != nil {
				body, _ = json.Marshal(tt.body)
			} else {
				body = []byte("{invalid")
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/bank-accounts/acc-1/reconciliation", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CreateReconciliation(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantErr != "" {
				assert.Contains(t, rr.Body.String(), tt.wantErr)
			}

			if tt.wantStatus == http.StatusCreated {
				var result banking.BankReconciliation
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.ID)
			}
		})
	}
}

func TestGetReconciliation(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.reconciliations["rec-1"] = &banking.BankReconciliation{
		ID:            "rec-1",
		TenantID:      "tenant-1",
		BankAccountID: "acc-1",
		Status:        banking.ReconciliationInProgress,
	}

	tests := []struct {
		name             string
		reconciliationID string
		wantStatus       int
	}{
		{
			name:             "existing reconciliation",
			reconciliationID: "rec-1",
			wantStatus:       http.StatusOK,
		},
		{
			name:             "non-existent reconciliation",
			reconciliationID: "rec-999",
			wantStatus:       http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reconciliations/"+tt.reconciliationID, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "reconciliationID": tt.reconciliationID})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetReconciliation(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestCompleteReconciliation(t *testing.T) {
	tests := []struct {
		name       string
		setupRepo  func(*mockBankingRepository)
		wantStatus int
	}{
		{
			name: "valid completion",
			setupRepo: func(repo *mockBankingRepository) {
				repo.reconciliations["rec-1"] = &banking.BankReconciliation{
					ID:            "rec-1",
					TenantID:      "tenant-1",
					BankAccountID: "acc-1",
					Status:        banking.ReconciliationInProgress,
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "already completed",
			setupRepo: func(repo *mockBankingRepository) {
				now := time.Now()
				repo.reconciliations["rec-1"] = &banking.BankReconciliation{
					ID:            "rec-1",
					TenantID:      "tenant-1",
					BankAccountID: "acc-1",
					Status:        banking.ReconciliationCompleted,
					CompletedAt:   &now,
				}
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo, tenantRepo := setupBankingTestHandlers()

			tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
				ID:         "tenant-1",
				SchemaName: "tenant_test",
			}

			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/reconciliations/rec-1/complete", nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "reconciliationID": "rec-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.CompleteReconciliation(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestGetImportHistory(t *testing.T) {
	h, repo, tenantRepo := setupBankingTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	repo.imports["acc-1"] = []banking.BankStatementImport{
		{
			ID:                   "imp-1",
			TenantID:             "tenant-1",
			BankAccountID:        "acc-1",
			FileName:             "statement.csv",
			TransactionsImported: 10,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/acc-1/import-history", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "accountID": "acc-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetImportHistory(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result []banking.BankStatementImport
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}
