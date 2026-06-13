package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/reports"
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

func (m *mockYearEndAccountingRepository) UpdateAccount(ctx context.Context, schemaName string, a *accounting.Account) error {
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
	h.accountingService = accounting.NewServiceWithRepository(accountingRepo)
	return h, repo, accountingRepo
}

func attachYearEndInventoryFixture(h *Handlers, unitCost decimal.Decimal) *mockInventoryRepository {
	inventoryRepo := newMockInventoryRepository()
	h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
	inventoryRepo.products[apiInventoryStockProductID] = &inventory.Product{
		ID:             apiInventoryStockProductID,
		TenantID:       "tenant-1",
		Code:           "SKU-001",
		Name:           "Widget",
		ProductType:    inventory.ProductTypeGoods,
		PurchasePrice:  unitCost,
		CurrentStock:   decimal.NewFromInt(4),
		TrackInventory: true,
		IsActive:       true,
	}
	inventoryRepo.warehouses[apiInventoryStockWarehouseID] = &inventory.Warehouse{
		ID:       apiInventoryStockWarehouseID,
		TenantID: "tenant-1",
		Code:     "MAIN",
		Name:     "Main warehouse",
		IsActive: true,
	}
	inventoryRepo.stockLevels[apiInventoryStockLevelKey(apiInventoryStockProductID, apiInventoryStockWarehouseID)] = &inventory.StockLevel{
		ID:           "stock-1",
		TenantID:     "tenant-1",
		ProductID:    apiInventoryStockProductID,
		WarehouseID:  apiInventoryStockWarehouseID,
		Quantity:     decimal.NewFromInt(4),
		ReservedQty:  decimal.Zero,
		AvailableQty: decimal.NewFromInt(4),
	}
	if unitCost.GreaterThan(decimal.Zero) {
		inventoryRepo.movements[apiInventoryStockProductID] = []inventory.InventoryMovement{{
			ID:           "movement-1",
			TenantID:     "tenant-1",
			ProductID:    apiInventoryStockProductID,
			WarehouseID:  apiInventoryStockWarehouseID,
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.NewFromInt(4),
			UnitCost:     unitCost,
			TotalCost:    unitCost.Mul(decimal.NewFromInt(4)),
			MovementDate: time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC),
		}}
	}
	return inventoryRepo
}

func seedYearEndAccountingReady(accountingRepo *mockYearEndAccountingRepository) {
	accountingRepo.accounts["retained"] = &accounting.Account{
		ID:          "retained",
		TenantID:    "tenant-1",
		Code:        "3200",
		Name:        "Retained Earnings",
		AccountType: accounting.AccountTypeEquity,
		IsActive:    true,
	}
	accountingRepo.periodBalances = []accounting.AccountBalance{{
		AccountID:     "revenue-1",
		AccountCode:   "4100",
		AccountName:   "Sales Revenue",
		AccountType:   accounting.AccountTypeRevenue,
		CreditBalance: decimal.NewFromInt(1000),
		NetBalance:    decimal.NewFromInt(1000),
	}}
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

func TestGetYearEndCloseStatusIncludesInventoryCostingReview(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	attachYearEndInventoryFixture(h, decimal.NewFromInt(6))

	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	seedYearEndAccountingReady(accountingRepo)

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-status?period_end_date=2025-12-31&inventory_valuation_method=weighted-average", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.GetYearEndCloseStatus(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp accounting.YearEndCloseStatus
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.NotNil(t, resp.InventoryCostingReview)
	assert.Equal(t, inventory.InventoryValuationMethodWeightedAverage, resp.InventoryCostingReview.ValuationMethod)
	assert.True(t, resp.InventoryCostingReview.Ready)
	assert.Equal(t, 1, resp.InventoryCostingReview.LineCount)
	assert.True(t, resp.InventoryCostingReview.TotalValue.Equal(decimal.NewFromInt(24)))
	assert.Equal(t, 0, resp.InventoryCostingReview.BlockingExceptionLineCount)
	assert.True(t, resp.CarryForwardReady)
}

func TestGetYearEndCloseStatusUsesTenantInventoryPolicyWhenOmitted(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	attachYearEndInventoryFixture(h, decimal.NewFromInt(6))

	settings := tenant.DefaultSettings()
	settings.PeriodLockDate = stringPtr("2025-12-31")
	settings.InventoryValuationMethod = tenant.InventoryValuationMethodWeightedAverage
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		Name:       "Tenant",
		Slug:       "tenant",
		SchemaName: "tenant_tenant",
		Settings:   settings,
	}
	seedYearEndAccountingReady(accountingRepo)

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
	require.NotNil(t, resp.InventoryCostingReview)
	assert.Equal(t, inventory.InventoryValuationMethodWeightedAverage, resp.InventoryCostingReview.ValuationMethod)
	assert.True(t, resp.InventoryCostingReview.Ready)
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

func TestCreateYearEndCarryForwardRequiresInventoryCostingReady(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	attachYearEndInventoryFixture(h, decimal.Zero)

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
	seedYearEndAccountingReady(accountingRepo)

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]any{
		"period_end_date":            "2025-12-31",
		"inventory_valuation_method": "standard-cost",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateYearEndCarryForward(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "response body: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "inventory costing review has 1 blocking exception lines")
}

func TestGetAnnualReport(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	reportsRepo := reports.NewMockRepository()
	h.reportsService = reports.NewServiceWithRepository(reportsRepo)

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
			AccountID:    "asset-1",
			AccountCode:  "1000",
			AccountName:  "Bank",
			AccountType:  accounting.AccountTypeAsset,
			DebitBalance: decimal.NewFromInt(600),
			NetBalance:   decimal.NewFromInt(600),
		},
		{
			AccountID:     "equity-1",
			AccountCode:   "3200",
			AccountName:   "Retained Earnings",
			AccountType:   accounting.AccountTypeEquity,
			CreditBalance: decimal.NewFromInt(600),
			NetBalance:    decimal.NewFromInt(-600),
		},
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
			DebitBalance: decimal.NewFromInt(400),
			NetBalance:   decimal.NewFromInt(400),
		},
	}
	reportsRepo.JournalEntries = []reports.JournalEntryWithLines{{
		ID:        "je-1",
		EntryDate: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC),
		Lines: []reports.JournalLine{
			{AccountCode: "1000", AccountName: "Bank", AccountType: "ASSET", Debit: decimal.NewFromInt(600)},
			{AccountCode: "4100", AccountName: "Sales Revenue", AccountType: "REVENUE", Credit: decimal.NewFromInt(600)},
		},
	}}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/reports/annual?period_end_date=2025-12-31&cash_flow_method=direct", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.GetAnnualReport(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp reports.AnnualReport
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.NotNil(t, resp.CloseStatus)
	require.NotNil(t, resp.TrialBalance)
	require.NotNil(t, resp.BalanceSheet)
	require.NotNil(t, resp.IncomeStatement)
	require.NotNil(t, resp.CashFlowStatement)
	assert.Equal(t, "2025", resp.FiscalYearLabel)
	assert.Equal(t, "2025-01-01", resp.FiscalYearStartDate)
	assert.Equal(t, "2025-12-31", resp.FiscalYearEndDate)
	assert.True(t, resp.IncomeStatement.NetIncome.Equal(decimal.NewFromInt(600)))
	assert.Equal(t, reports.CashFlowMethodDirect, resp.CashFlowStatement.Method)
	assert.True(t, resp.CashFlowStatement.NetCashChange.Equal(decimal.NewFromInt(600)))
}

func TestGetYearEndCloseAuditEvidenceIncludesClosePackDocuments(t *testing.T) {
	h, repo, accountingRepo := setupTenantAccountingHandlers()
	docRepo := newMockDocumentRepository()
	store, err := documents.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	h.documentsService = documents.NewService(docRepo, store)

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
	}
	entityID, err := accounting.YearEndCloseEvidenceEntityID("tenant-1", "2025-12-31")
	require.NoError(t, err)
	storageKey := "tenant-1/year-end/doc-close-pack.pdf"
	docRepo.docs["doc-close-pack"] = &documents.Document{
		ID:           "doc-close-pack",
		TenantID:     "tenant-1",
		EntityType:   documents.EntityTypeYearEndClose,
		EntityID:     entityID,
		DocumentType: documents.DocumentTypeClosePack,
		FileName:     "close-pack.pdf",
		ContentType:  "application/pdf",
		FileSize:     4096,
		StorageKey:   storageKey,
		ReviewStatus: documents.ReviewStatusApproved,
		UploadedBy:   "user-1",
		CreatedAt:    time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC),
	}
	require.NoError(t, store.Save(context.Background(), storageKey, bytes.NewBufferString("close pack pdf")))

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-evidence?period_end_date=2025-12-31", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.GetYearEndCloseAuditEvidence(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp accounting.YearEndCloseAuditEvidence
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.NotNil(t, resp.Pack)
	require.NotNil(t, resp.Pack.Status)
	require.NotNil(t, resp.EvidencePolicy)
	assert.True(t, resp.EvidencePolicy.Compliant)
	require.Len(t, resp.Documents, 1)
	assert.Equal(t, "close-pack.pdf", resp.Documents[0].FileName)
	assert.Equal(t, entityID, resp.Pack.Status.ClosePackEvidenceEntityID)

	archiveReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-archive?period_end_date=2025-12-31", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	archiveReq = withURLParams(archiveReq, map[string]string{"tenantID": "tenant-1"})
	archiveResp := httptest.NewRecorder()

	h.DownloadYearEndCloseAuditArchive(archiveResp, archiveReq)

	require.Equal(t, http.StatusOK, archiveResp.Code, "response body: %s", archiveResp.Body.String())
	assert.Equal(t, "application/zip", archiveResp.Header().Get("Content-Type"))
	reader, err := zip.NewReader(bytes.NewReader(archiveResp.Body.Bytes()), int64(archiveResp.Body.Len()))
	require.NoError(t, err)
	entries := map[string]string{}
	for _, file := range reader.File {
		rc, err := file.Open()
		require.NoError(t, err)
		payload, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		entries[file.Name] = string(payload)
	}
	assert.Contains(t, entries, "manifest.json")
	assert.Contains(t, entries["manifest.json"], "close-pack.pdf")
	assert.Equal(t, "close pack pdf", entries["documents/doc-close-pack-close-pack.pdf"])
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
