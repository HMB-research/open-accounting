package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type mockAccountingRepository struct {
	accounts       map[string]*accounting.Account
	journalEntries map[string]*accounting.JournalEntry

	accountBalance       decimal.Decimal
	trialBalances        []accounting.AccountBalance
	periodBalances       []accounting.AccountBalance
	listAccountsErr      error
	createAccountErr     error
	getAccountErr        error
	getJournalErr        error
	createJournalErr     error
	updateJournalErr     error
	getBalanceErr        error
	getTrialBalanceErr   error
	getPeriodBalancesErr error
	voidJournalErr       error
}

func newMockAccountingRepository() *mockAccountingRepository {
	return &mockAccountingRepository{
		accounts:       make(map[string]*accounting.Account),
		journalEntries: make(map[string]*accounting.JournalEntry),
	}
}

func (m *mockAccountingRepository) GetAccountByID(ctx context.Context, schemaName, tenantID, accountID string) (*accounting.Account, error) {
	if m.getAccountErr != nil {
		return nil, m.getAccountErr
	}
	account, ok := m.accounts[accountID]
	if !ok || account.TenantID != tenantID {
		return nil, assert.AnError
	}
	return account, nil
}

func (m *mockAccountingRepository) ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.Account, error) {
	if m.listAccountsErr != nil {
		return nil, m.listAccountsErr
	}
	result := make([]accounting.Account, 0, len(m.accounts))
	for _, account := range m.accounts {
		if account.TenantID != tenantID {
			continue
		}
		if activeOnly && !account.IsActive {
			continue
		}
		result = append(result, *account)
	}
	return result, nil
}

func (m *mockAccountingRepository) CreateAccount(ctx context.Context, schemaName string, account *accounting.Account) error {
	if m.createAccountErr != nil {
		return m.createAccountErr
	}
	m.accounts[account.ID] = account
	return nil
}

func (m *mockAccountingRepository) GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*accounting.JournalEntry, error) {
	if m.getJournalErr != nil {
		return nil, m.getJournalErr
	}
	entry, ok := m.journalEntries[entryID]
	if !ok || entry.TenantID != tenantID {
		return nil, assert.AnError
	}
	return entry, nil
}

func (m *mockAccountingRepository) ListJournalEntries(ctx context.Context, schemaName, tenantID string, limit int) ([]accounting.JournalEntry, error) {
	if m.getJournalErr != nil {
		return nil, m.getJournalErr
	}

	result := make([]accounting.JournalEntry, 0, len(m.journalEntries))
	for _, entry := range m.journalEntries {
		if entry.TenantID != tenantID {
			continue
		}
		result = append(result, *entry)
	}
	return result, nil
}

func (m *mockAccountingRepository) GetJournalEntryBySource(ctx context.Context, schemaName, tenantID, sourceType, sourceID string) (*accounting.JournalEntry, error) {
	if m.getJournalErr != nil {
		return nil, m.getJournalErr
	}
	for _, entry := range m.journalEntries {
		if entry.TenantID != tenantID || entry.SourceType != sourceType || entry.Status == accounting.StatusVoided || entry.SourceID == nil || *entry.SourceID != sourceID {
			continue
		}
		return entry, nil
	}
	return nil, nil
}

func (m *mockAccountingRepository) CreateJournalEntry(ctx context.Context, schemaName string, je *accounting.JournalEntry) error {
	if m.createJournalErr != nil {
		return m.createJournalErr
	}
	je.EntryNumber = "JE-00001"
	m.journalEntries[je.ID] = je
	return nil
}

func (m *mockAccountingRepository) CreateJournalEntryTx(ctx context.Context, schemaName string, tx pgx.Tx, je *accounting.JournalEntry) error {
	return m.CreateJournalEntry(ctx, schemaName, je)
}

func (m *mockAccountingRepository) UpdateJournalEntryStatus(ctx context.Context, schemaName, tenantID, entryID string, status accounting.JournalEntryStatus, userID string) error {
	if m.updateJournalErr != nil {
		return m.updateJournalErr
	}
	entry, ok := m.journalEntries[entryID]
	if !ok || entry.TenantID != tenantID {
		return assert.AnError
	}
	entry.Status = status
	if status == accounting.StatusPosted {
		now := time.Now()
		entry.PostedAt = &now
		entry.PostedBy = &userID
	}
	return nil
}

func (m *mockAccountingRepository) GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error) {
	if m.getBalanceErr != nil {
		return decimal.Zero, m.getBalanceErr
	}
	if m.accountBalance.IsZero() {
		return decimal.NewFromInt(1000), nil
	}
	return m.accountBalance, nil
}

func (m *mockAccountingRepository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]accounting.AccountBalance, error) {
	if m.getTrialBalanceErr != nil {
		return nil, m.getTrialBalanceErr
	}
	return m.trialBalances, nil
}

func (m *mockAccountingRepository) GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]accounting.AccountBalance, error) {
	if m.getPeriodBalancesErr != nil {
		return nil, m.getPeriodBalancesErr
	}
	return m.periodBalances, nil
}

func (m *mockAccountingRepository) VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string, reversal *accounting.JournalEntry) error {
	if m.voidJournalErr != nil {
		return m.voidJournalErr
	}
	entry, ok := m.journalEntries[entryID]
	if !ok || entry.TenantID != tenantID {
		return assert.AnError
	}
	entry.Status = accounting.StatusVoided
	now := time.Now()
	entry.VoidedAt = &now
	entry.VoidedBy = &userID
	entry.VoidReason = reason
	if reversal != nil {
		reversal.EntryNumber = "JE-00002"
		m.journalEntries[reversal.ID] = reversal
	}
	return nil
}

func setupAccountingTestHandlers() (*Handlers, *mockTenantRepository, *mockAccountingRepository) {
	tenantRepo := newMockTenantRepository()
	accountingRepo := newMockAccountingRepository()

	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)
	accountingSvc := accounting.NewServiceWithRepo(nil, accountingRepo)
	tokenSvc := auth.NewTokenService("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour)

	h := &Handlers{
		tenantService:     tenantSvc,
		accountingService: accountingSvc,
		tokenService:      tokenSvc,
	}

	return h, tenantRepo, accountingRepo
}

func TestListJournalEntries(t *testing.T) {
	h, _, accountingRepo := setupAccountingTestHandlers()
	accountingRepo.journalEntries["je-1"] = &accounting.JournalEntry{
		ID:          "je-1",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00001",
		Description: "Opening balance",
		Status:      accounting.StatusPosted,
		CreatedBy:   "user-1",
	}
	accountingRepo.journalEntries["je-2"] = &accounting.JournalEntry{
		ID:          "je-2",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00002",
		Description: "Sales accrual",
		Status:      accounting.StatusDraft,
		CreatedBy:   "user-1",
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/journal-entries?limit=10", nil, createTestClaims("user-1", "acc@example.com", "tenant-1", "admin"))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ListJournalEntries(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp []accounting.JournalEntry
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp, 2)
}

func TestListJournalEntriesRejectsInvalidLimit(t *testing.T) {
	h, _, _ := setupAccountingTestHandlers()

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/journal-entries?limit=500", nil, createTestClaims("user-1", "acc@example.com", "tenant-1", "admin"))
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ListJournalEntries(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportAccounts(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository, *mockAccountingRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, accounting.ImportAccountsResult)
	}{
		{
			name:     "imports valid accounts",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"file_name": "accounts.csv",
				"csv_content": "code,name,account_type,parent_code\n" +
					"1100,Cash,ASSET,\n" +
					"1110,Cash Drawer,ASSET,1100\n",
			},
			setupMock: func(tr *mockTenantRepository, ar *mockAccountingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp accounting.ImportAccountsResult) {
				assert.Equal(t, "accounts.csv", resp.FileName)
				assert.Equal(t, 2, resp.RowsProcessed)
				assert.Equal(t, 2, resp.AccountsCreated)
				assert.Equal(t, 0, resp.RowsSkipped)
				assert.Empty(t, resp.Errors)
			},
		},
		{
			name:     "defaults file name and reports duplicates",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"csv_content": "code,name,account_type\n1000,Cash,ASSET\n1000,Duplicate Cash,ASSET\n",
			},
			setupMock: func(tr *mockTenantRepository, ar *mockAccountingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ar.accounts["acc-1"] = &accounting.Account{
					ID:          "acc-1",
					TenantID:    "tenant-1",
					Code:        "1000",
					Name:        "Cash",
					AccountType: accounting.AccountTypeAsset,
					IsActive:    true,
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp accounting.ImportAccountsResult) {
				assert.Equal(t, "accounts_import.csv", resp.FileName)
				assert.Equal(t, 2, resp.RowsProcessed)
				assert.Equal(t, 0, resp.AccountsCreated)
				assert.Equal(t, 2, resp.RowsSkipped)
				require.Len(t, resp.Errors, 2)
				assert.Contains(t, resp.Errors[0].Message, "duplicate")
			},
		},
		{
			name:     "rejects missing csv content",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{"file_name": "accounts.csv"},
			setupMock: func(tr *mockTenantRepository, ar *mockAccountingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "csv_content is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, accountingRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/accounts/import", tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.ImportAccounts(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp accounting.ImportAccountsResult
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestImportOpeningBalances(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository, *mockAccountingRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, accounting.ImportOpeningBalancesResult)
	}{
		{
			name:     "imports balanced opening balances",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"file_name":   "opening-balances.csv",
				"entry_date":  "2026-01-01",
				"description": "Imported opening balances",
				"reference":   "OB-2026",
				"csv_content": "account_code,debit,credit,description\n1000,1000.00,0,Cash opening balance\n3000,0,1000.00,Equity opening balance\n",
			},
			setupMock: func(tr *mockTenantRepository, ar *mockAccountingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ar.accounts["acc-1000"] = &accounting.Account{
					ID:          "acc-1000",
					TenantID:    "tenant-1",
					Code:        "1000",
					Name:        "Cash",
					AccountType: accounting.AccountTypeAsset,
					IsActive:    true,
				}
				ar.accounts["acc-3000"] = &accounting.Account{
					ID:          "acc-3000",
					TenantID:    "tenant-1",
					Code:        "3000",
					Name:        "Owner Equity",
					AccountType: accounting.AccountTypeEquity,
					IsActive:    true,
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp accounting.ImportOpeningBalancesResult) {
				assert.Equal(t, "opening-balances.csv", resp.FileName)
				assert.Equal(t, 2, resp.RowsProcessed)
				assert.Equal(t, 2, resp.LinesImported)
				assert.True(t, resp.TotalDebit.Equal(decimal.NewFromInt(1000)))
				assert.True(t, resp.TotalCredit.Equal(decimal.NewFromInt(1000)))
				require.NotNil(t, resp.JournalEntry)
				assert.Equal(t, accounting.StatusPosted, resp.JournalEntry.Status)
				assert.Equal(t, "OPENING_BALANCE", resp.JournalEntry.SourceType)
			},
		},
		{
			name:     "rejects missing entry date",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"csv_content": "account_code,debit,credit\n1000,100.00,0\n3000,0,100.00\n",
			},
			setupMock: func(tr *mockTenantRepository, ar *mockAccountingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "entry_date is required",
		},
		{
			name:     "rejects unbalanced opening balances",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"entry_date":  "2026-01-01",
				"csv_content": "account_code,debit,credit\n1000,100.00,0\n3000,0,90.00\n",
			},
			setupMock: func(tr *mockTenantRepository, ar *mockAccountingRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				ar.accounts["acc-1000"] = &accounting.Account{
					ID:          "acc-1000",
					TenantID:    "tenant-1",
					Code:        "1000",
					Name:        "Cash",
					AccountType: accounting.AccountTypeAsset,
					IsActive:    true,
				}
				ar.accounts["acc-3000"] = &accounting.Account{
					ID:          "acc-3000",
					TenantID:    "tenant-1",
					Code:        "3000",
					Name:        "Owner Equity",
					AccountType: accounting.AccountTypeEquity,
					IsActive:    true,
				}
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "opening balances do not balance",
		},
		{
			name:     "rejects locked periods",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"entry_date":  "2026-01-15",
				"csv_content": "account_code,debit,credit\n1000,100.00,0\n3000,0,100.00\n",
			},
			setupMock: func(tr *mockTenantRepository, ar *mockAccountingRepository) {
				lockedTenant := tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				lockDate := "2026-01-31"
				lockedTenant.Settings.PeriodLockDate = &lockDate
			},
			wantStatus:     http.StatusConflict,
			wantErrContain: "period locked through 2026-01-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, accountingRepo := setupAccountingTestHandlers()
			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, accountingRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/journal-entries/import-opening-balances", tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.ImportOpeningBalances(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp accounting.ImportOpeningBalancesResult
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}
