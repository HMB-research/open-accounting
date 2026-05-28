package main

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type mockYearEndAccountingRepository struct {
	accounts       map[string]*accounting.Account
	journalEntries map[string]*accounting.JournalEntry
	periodBalances []accounting.AccountBalance

	getJournalErr    error
	createJournalErr error
	updateStatusErr  error
	periodBalanceErr error
}

func newMockYearEndAccountingRepository() *mockYearEndAccountingRepository {
	return &mockYearEndAccountingRepository{
		accounts:       make(map[string]*accounting.Account),
		journalEntries: make(map[string]*accounting.JournalEntry),
	}
}

func (m *mockYearEndAccountingRepository) GetAccountByID(ctx context.Context, schemaName, tenantID, accountID string) (*accounting.Account, error) {
	account, ok := m.accounts[accountID]
	if !ok || account.TenantID != tenantID {
		return nil, errors.New("account not found")
	}
	return account, nil
}

func (m *mockYearEndAccountingRepository) ListAccounts(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]accounting.Account, error) {
	result := make([]accounting.Account, 0, len(m.accounts))
	for _, account := range m.accounts {
		if account.TenantID != tenantID {
			continue
		}
		result = append(result, *account)
	}
	return result, nil
}

func (m *mockYearEndAccountingRepository) CreateAccount(ctx context.Context, schemaName string, a *accounting.Account) error {
	m.accounts[a.ID] = a
	return nil
}

func (m *mockYearEndAccountingRepository) ListJournalEntries(ctx context.Context, schemaName, tenantID string, limit int) ([]accounting.JournalEntry, error) {
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

func (m *mockYearEndAccountingRepository) GetJournalEntryByID(ctx context.Context, schemaName, tenantID, entryID string) (*accounting.JournalEntry, error) {
	if m.getJournalErr != nil {
		return nil, m.getJournalErr
	}
	entry, ok := m.journalEntries[entryID]
	if !ok || entry.TenantID != tenantID {
		return nil, errors.New("journal entry not found")
	}
	return entry, nil
}

func (m *mockYearEndAccountingRepository) GetJournalEntryBySource(ctx context.Context, schemaName, tenantID, sourceType, sourceID string) (*accounting.JournalEntry, error) {
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

func (m *mockYearEndAccountingRepository) CreateJournalEntry(ctx context.Context, schemaName string, je *accounting.JournalEntry) error {
	if m.createJournalErr != nil {
		return m.createJournalErr
	}
	je.EntryNumber = "JE-00100"
	m.journalEntries[je.ID] = je
	return nil
}

func (m *mockYearEndAccountingRepository) CreateJournalEntryTx(ctx context.Context, schemaName string, tx pgx.Tx, je *accounting.JournalEntry) error {
	return m.CreateJournalEntry(ctx, schemaName, je)
}

func (m *mockYearEndAccountingRepository) UpdateJournalEntryStatus(ctx context.Context, schemaName, tenantID, entryID string, status accounting.JournalEntryStatus, userID string) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	entry, ok := m.journalEntries[entryID]
	if !ok || entry.TenantID != tenantID {
		return errors.New("journal entry not found")
	}
	entry.Status = status
	return nil
}

func (m *mockYearEndAccountingRepository) GetAccountBalance(ctx context.Context, schemaName, tenantID, accountID string, asOfDate time.Time) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (m *mockYearEndAccountingRepository) GetTrialBalance(ctx context.Context, schemaName, tenantID string, asOfDate time.Time) ([]accounting.AccountBalance, error) {
	if m.periodBalanceErr != nil {
		return nil, m.periodBalanceErr
	}
	return m.periodBalances, nil
}

func (m *mockYearEndAccountingRepository) GetPeriodBalances(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]accounting.AccountBalance, error) {
	if m.periodBalanceErr != nil {
		return nil, m.periodBalanceErr
	}
	return m.periodBalances, nil
}

func (m *mockYearEndAccountingRepository) VoidJournalEntry(ctx context.Context, schemaName, tenantID, entryID, userID, reason string, reversal *accounting.JournalEntry) error {
	entry, ok := m.journalEntries[entryID]
	if !ok || entry.TenantID != tenantID {
		return errors.New("entry not found or not in posted status")
	}
	if entry.Status != accounting.StatusPosted {
		return errors.New("entry not found or not in posted status")
	}
	entry.Status = accounting.StatusVoided
	entry.VoidReason = reason
	if reversal != nil {
		reversal.EntryNumber = "JE-00101"
		m.journalEntries[reversal.ID] = reversal
	}
	return nil
}

func setupTenantAccountingHandlers() (*Handlers, *mockTenantRepository, *mockYearEndAccountingRepository) {
	h, repo := setupTenantTestHandlers()
	accountingRepo := newMockYearEndAccountingRepository()
	h.accountingService = accounting.NewServiceWithRepo(nil, accountingRepo)
	return h, repo, accountingRepo
}

func TestGetYearEndCloseStatus(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleViewer},
	}
	accountingRepo.accounts["retained"] = &accounting.Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: accounting.AccountTypeEquity,
		IsActive:    true,
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   accounting.AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-status?period_end_date=2025-12-31", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.GetYearEndCloseStatus(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp accounting.YearEndCloseStatus
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.True(t, resp.IsFiscalYearEnd)
	assert.True(t, resp.PeriodClosed)
	assert.True(t, resp.CarryForwardReady)
	assert.Equal(t, "2026-01-01", resp.CarryForwardDate)
}

func TestGetYearEndCloseStatusIncludesClosePackEvidence(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	docRepo := newMockDocumentRepository()
	h.documentsService = documents.NewService(docRepo, nil)

	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	accountingRepo.accounts["retained"] = &accounting.Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: accounting.AccountTypeEquity,
		IsActive:    true,
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   accounting.AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-status?period_end_date=2025-12-31", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.GetYearEndCloseStatus(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp accounting.YearEndCloseStatus
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.NotEmpty(t, resp.ClosePackEvidenceEntityID)
	require.NotNil(t, resp.ClosePackEvidence)
	assert.False(t, resp.ClosePackEvidence.Compliant)
	assert.False(t, resp.CarryForwardReady)
}

func TestGetYearEndClosePack(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	accountingRepo.accounts["retained"] = &accounting.Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: accounting.AccountTypeEquity,
		IsActive:    true,
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   accounting.AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(-1000),
		},
		{
			AccountID:    "expense-1",
			AccountCode:  "5100",
			AccountName:  "Operating Expenses",
			AccountType:  accounting.AccountTypeExpense,
			DebitBalance: decimal.NewFromInt(250),
			NetBalance:   decimal.NewFromInt(250),
		},
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-pack?period_end_date=2025-12-31", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.GetYearEndClosePack(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp accounting.YearEndClosePack
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Status)
	require.NotNil(t, resp.TrialBalance)
	require.NotNil(t, resp.BalanceSheet)
	require.NotNil(t, resp.IncomeStatement)
	assert.Equal(t, "2025", resp.Status.FiscalYearLabel)
	assert.True(t, resp.IncomeStatement.NetIncome.Equal(decimal.NewFromInt(750)))
	assert.True(t, resp.TrialBalance.TotalDebits.Equal(decimal.NewFromInt(250)))
	assert.True(t, resp.TrialBalance.TotalCredits.Equal(decimal.NewFromInt(1000)))
}

func TestCreateYearEndCarryForward(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}
	accountingRepo.accounts["retained"] = &accounting.Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: accounting.AccountTypeEquity,
		IsActive:    true,
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   accounting.AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
		{
			AccountID:    "expense-1",
			AccountCode:  "5100",
			AccountName:  "Salary Expenses",
			AccountType:  accounting.AccountTypeExpense,
			DebitBalance: decimal.NewFromInt(400),
			NetBalance:   decimal.NewFromInt(400),
		},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]interface{}{
		"period_end_date": "2025-12-31",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateYearEndCarryForward(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp accounting.YearEndCarryForwardResult
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.NotNil(t, resp.JournalEntry)
	assert.Equal(t, accounting.SourceTypeYearEndCarryForward, resp.JournalEntry.SourceType)
	assert.Equal(t, accounting.StatusPosted, resp.JournalEntry.Status)
	require.NotNil(t, resp.Status)
	require.NotNil(t, resp.Status.ExistingCarryForward)
	assert.Equal(t, resp.JournalEntry.ID, resp.Status.ExistingCarryForward.ID)
}

func TestCreateYearEndCarryForwardRequiresApprovedClosePackEvidence(t *testing.T) {
	h, repo, _ := setupTenantAccountingHandlers()
	docRepo := newMockDocumentRepository()
	h.documentsService = documents.NewService(docRepo, nil)

	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]interface{}{
		"period_end_date": "2025-12-31",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateYearEndCarryForward(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "response body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "approved close-pack evidence is required")
}

func TestCreateYearEndCarryForwardRequiresClosedYear(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-11-30")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}
	accountingRepo.accounts["retained"] = &accounting.Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: accounting.AccountTypeEquity,
		IsActive:    true,
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   accounting.AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]interface{}{
		"period_end_date": "2025-12-31",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateYearEndCarryForward(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "response body: %s", w.Body.String())
	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "fiscal year must be closed")
}

func TestReverseYearEndCarryForward(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}
	accountingRepo.accounts["retained"] = &accounting.Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: accounting.AccountTypeEquity,
		IsActive:    true,
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{
		{
			AccountID:     "revenue-1",
			AccountCode:   "4100",
			AccountName:   "Sales Revenue",
			AccountType:   accounting.AccountTypeRevenue,
			CreditBalance: decimal.NewFromInt(1000),
			NetBalance:    decimal.NewFromInt(1000),
		},
	}
	fiscalYearEndDate, err := time.Parse("2006-01-02", "2025-12-31")
	require.NoError(t, err)
	sourceID := accounting.YearEndCarryForwardSourceID("tenant-1", fiscalYearEndDate)
	accountingRepo.journalEntries["carry-forward"] = &accounting.JournalEntry{
		ID:          "carry-forward",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00088",
		EntryDate:   fiscalYearEndDate.AddDate(0, 0, 1),
		Description: "Year-end carry-forward",
		Reference:   "CF-20251231",
		SourceType:  accounting.SourceTypeYearEndCarryForward,
		SourceID:    &sourceID,
		Status:      accounting.StatusPosted,
		Lines: []accounting.JournalEntryLine{
			{
				AccountID:    "revenue-1",
				DebitAmount:  decimal.NewFromInt(1000),
				BaseDebit:    decimal.NewFromInt(1000),
				CreditAmount: decimal.Zero,
				BaseCredit:   decimal.Zero,
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
			},
			{
				AccountID:    "retained",
				DebitAmount:  decimal.Zero,
				BaseDebit:    decimal.Zero,
				CreditAmount: decimal.NewFromInt(1000),
				BaseCredit:   decimal.NewFromInt(1000),
				Currency:     "EUR",
				ExchangeRate: decimal.NewFromInt(1),
			},
		},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward/reverse", map[string]interface{}{
		"period_end_date": "2025-12-31",
		"reason":          "Late supplier accrual",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ReverseYearEndCarryForward(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp accounting.YearEndCarryForwardReversalResult
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.NotNil(t, resp.ReversalJournalEntry)
	assert.Equal(t, accounting.SourceTypeYearEndCarryForwardReversal, resp.ReversalJournalEntry.SourceType)
	assert.Equal(t, "2026-01-01", resp.ReversalJournalEntry.EntryDate.Format("2006-01-02"))
	assert.Equal(t, accounting.StatusVoided, accountingRepo.journalEntries["carry-forward"].Status)
	require.NotNil(t, resp.Status)
	assert.Nil(t, resp.Status.ExistingCarryForward)
	assert.True(t, resp.Status.CarryForwardReady)
}

func TestReopenPeriodRejectsYearEndCarryForward(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}
	fiscalYearEndDate, err := time.Parse("2006-01-02", "2025-12-31")
	require.NoError(t, err)
	sourceID := accounting.YearEndCarryForwardSourceID("tenant-1", fiscalYearEndDate)
	accountingRepo.journalEntries["carry-forward"] = &accounting.JournalEntry{
		ID:          "carry-forward",
		TenantID:    "tenant-1",
		EntryNumber: "JE-00088",
		EntryDate:   fiscalYearEndDate.AddDate(0, 0, 1),
		Description: "Year-end carry-forward",
		SourceType:  accounting.SourceTypeYearEndCarryForward,
		SourceID:    &sourceID,
		Status:      accounting.StatusPosted,
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", map[string]interface{}{
		"period_end_date": "2025-12-31",
		"note":            "Need to revise year-end",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ReopenPeriod(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "response body: %s", w.Body.String())
	var resp map[string]string
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "cannot reopen a fiscal year")
}
