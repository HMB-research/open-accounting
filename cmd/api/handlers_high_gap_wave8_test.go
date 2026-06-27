package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestHighGapWave8CoreAuthTenantRefreshBranches(t *testing.T) {
	t.Run("login records tenant access denial and suspension before token issue", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/login", map[string]string{
			"email":     "user@example.com",
			"password":  "password123",
			"tenant_id": "missing-tenant",
		}, nil)
		rr := httptest.NewRecorder()
		h.Login(rr, req)
		require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Access denied to tenant")

		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{
			TenantID:  "tenant-1",
			UserID:    "user-1",
			Role:      tenant.RoleAdmin,
			IsActive:  false,
			CreatedAt: time.Now(),
		}}
		req = makeAuthenticatedRequest(http.MethodPost, "/auth/login", map[string]string{
			"email":     "user@example.com",
			"password":  "password123",
			"tenant_id": "tenant-1",
		}, nil)
		rr = httptest.NewRecorder()
		h.Login(rr, req)
		require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Tenant access is suspended")
	})

	t.Run("refresh maps tenant access denial", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
		refreshToken := createMockRefreshSession(t, h, "user-1")

		req := makeAuthenticatedRequest(http.MethodPost, "/auth/refresh", map[string]string{
			"refresh_token": refreshToken,
			"tenant_id":     "missing-tenant",
		}, nil)
		rr := httptest.NewRecorder()
		h.RefreshToken(rr, req)

		require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Access denied to tenant")
	})
}

type wave8PluginRepository struct {
	*pluginHandlerRepository
	getTenantPluginsErr error
	getSettingsErr      error
	updateSettingsErr   error
	getPluginCalls      int
	failGetPluginAtCall int
}

func (r *wave8PluginRepository) GetTenantPluginsWithAll(ctx context.Context, tenantID uuid.UUID) ([]plugin.TenantPlugin, error) {
	if r.getTenantPluginsErr != nil {
		return nil, r.getTenantPluginsErr
	}
	return r.pluginHandlerRepository.GetTenantPluginsWithAll(ctx, tenantID)
}

func (r *wave8PluginRepository) GetTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID) (json.RawMessage, error) {
	if r.getSettingsErr != nil {
		return nil, r.getSettingsErr
	}
	return r.pluginHandlerRepository.GetTenantPluginSettings(ctx, tenantID, pluginID)
}

func (r *wave8PluginRepository) UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	if r.updateSettingsErr != nil {
		return r.updateSettingsErr
	}
	return r.pluginHandlerRepository.UpdateTenantPluginSettings(ctx, tenantID, pluginID, settings)
}

func (r *wave8PluginRepository) GetPlugin(ctx context.Context, id uuid.UUID) (*plugin.Plugin, error) {
	r.getPluginCalls++
	if r.failGetPluginAtCall > 0 && r.getPluginCalls >= r.failGetPluginAtCall {
		return nil, errors.New("plugin reload failed")
	}
	return r.pluginHandlerRepository.GetPlugin(ctx, id)
}

func wave8PluginHandlers(t *testing.T) (*Handlers, *wave8PluginRepository, uuid.UUID, uuid.UUID, string) {
	t.Helper()

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := "user-1"
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	repo := &wave8PluginRepository{pluginHandlerRepository: newPluginHandlerRepository()}
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "wave8-plugin",
		DisplayName:        "Wave8 Plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/wave8-plugin",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"routes:register"},
		Manifest: json.RawMessage(`{
			"name":"wave8-plugin",
			"display_name":"Wave8 Plugin",
			"version":"1.0.0",
			"permissions":["routes:register"]
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Wave8 Tenant", "wave8-tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{{
		TenantID: tenantID.String(),
		UserID:   userID,
		Role:     tenant.RoleAdmin,
		IsActive: true,
	}}

	h := &Handlers{
		pluginService: plugin.NewServiceWithRepository(repo, nil, t.TempDir()),
		tenantService: tenant.NewServiceWithRepository(tenantRepo),
	}
	return h, repo, tenantID, pluginID, userID
}

func TestHighGapWave8PluginHandlerBranches(t *testing.T) {
	t.Run("registry success paths", func(t *testing.T) {
		h, repo := setupPluginTestHandlers(t)

		addReq := makeAuthenticatedRequest(http.MethodPost, "/admin/plugin-registries", plugin.CreateRegistryRequest{
			Name:        "Community",
			URL:         "https://github.com/HMB-research/community-plugins",
			Description: "Community-maintained plugins",
		}, nil)
		addResp := httptest.NewRecorder()
		h.AddPluginRegistry(addResp, addReq)
		require.Equal(t, http.StatusCreated, addResp.Code, addResp.Body.String())

		var created plugin.Registry
		require.NoError(t, json.NewDecoder(addResp.Body).Decode(&created))
		require.NotEqual(t, uuid.Nil, created.ID)

		require.Equal(t, "Community", repo.registries[created.ID].Name)
	})

	t.Run("admin plugin reload failures after state mutation", func(t *testing.T) {
		h, repo, _, pluginID, _ := wave8PluginHandlers(t)
		repo.plugins[pluginID].State = plugin.StateInstalled
		repo.plugins[pluginID].GrantedPermissions = nil
		repo.failGetPluginAtCall = 2

		enableReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/enable", plugin.EnablePluginRequest{
			GrantedPermissions: []string{"routes:register"},
		}, nil), map[string]string{"id": pluginID.String()})
		enableResp := httptest.NewRecorder()
		h.EnablePlugin(enableResp, enableReq)
		require.Equal(t, http.StatusInternalServerError, enableResp.Code, enableResp.Body.String())
		assert.Contains(t, enableResp.Body.String(), "Failed to load plugin")

		h, repo, _, pluginID, _ = wave8PluginHandlers(t)
		repo.failGetPluginAtCall = 2
		disableReq := withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/disable", nil), map[string]string{"id": pluginID.String()})
		disableResp := httptest.NewRecorder()
		h.DisablePlugin(disableResp, disableReq)
		require.Equal(t, http.StatusInternalServerError, disableResp.Code, disableResp.Body.String())
		assert.Contains(t, disableResp.Body.String(), "Failed to load plugin")
	})

	t.Run("tenant plugin repository errors and validation", func(t *testing.T) {
		h, repo, tenantID, pluginID, userID := wave8PluginHandlers(t)
		claims := createTestClaims(userID, "user@example.com", tenantID.String(), tenant.RoleAdmin)

		repo.getTenantPluginsErr = errors.New("tenant plugin store down")
		listReq := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins", nil, claims), map[string]string{"tenantID": tenantID.String()})
		listResp := httptest.NewRecorder()
		h.ListTenantPlugins(listResp, listReq)
		require.Equal(t, http.StatusInternalServerError, listResp.Code, listResp.Body.String())
		assert.Contains(t, listResp.Body.String(), "Failed to list plugins")

		enableReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable", map[string]json.RawMessage{
			"settings": json.RawMessage(`{"account":"1000"}`),
		}, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
		enableResp := httptest.NewRecorder()
		h.EnableTenantPlugin(enableResp, enableReq)
		require.Equal(t, http.StatusInternalServerError, enableResp.Code, enableResp.Body.String())
		assert.Contains(t, enableResp.Body.String(), "Failed to load tenant plugin")

		repo.getTenantPluginsErr = nil
		disableReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/disable", nil, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
		disableResp := httptest.NewRecorder()
		h.DisableTenantPlugin(disableResp, disableReq)
		require.Equal(t, http.StatusOK, disableResp.Code, disableResp.Body.String())

		delete(repo.tenantPlugins, pluginTenantKey(tenantID, pluginID))
		disableResp = httptest.NewRecorder()
		h.DisableTenantPlugin(disableResp, disableReq)
		require.Equal(t, http.StatusBadRequest, disableResp.Code, disableResp.Body.String())
		assert.Contains(t, disableResp.Body.String(), "plugin not found for tenant")

		repo.getSettingsErr = errors.New("settings unavailable")
		settingsReq := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", nil, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
		settingsResp := httptest.NewRecorder()
		h.GetTenantPluginSettings(settingsResp, settingsReq)
		require.Equal(t, http.StatusNotFound, settingsResp.Code, settingsResp.Body.String())
		assert.Contains(t, settingsResp.Body.String(), "settings unavailable")

		repo.getSettingsErr = nil
		repo.updateSettingsErr = errors.New("settings write failed")
		updateReq := withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", map[string]string{"account": "2000"}, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
		updateResp := httptest.NewRecorder()
		h.UpdateTenantPluginSettings(updateResp, updateReq)
		require.Equal(t, http.StatusBadRequest, updateResp.Code, updateResp.Body.String())
		assert.Contains(t, updateResp.Body.String(), "settings write failed")
	})
}

func TestHighGapWave8BusinessBankingAndOrderBranches(t *testing.T) {
	t.Run("banking handlers map remaining validation and service errors", func(t *testing.T) {
		h, repo, tenantRepo := setupBankingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Bank Tenant", "bank-tenant")

		repo.importHistoryErr = errors.New("history unavailable")
		req := wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/bank-accounts/bank-1/import-history", nil, map[string]string{"tenantID": "tenant-1", "accountID": "bank-1"})
		resp := httptest.NewRecorder()
		h.GetImportHistory(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "Failed to get import history")

		repo.getTxErr = errors.New("suggestions unavailable")
		req = wave7BusinessRequest(http.MethodGet, "/tenants/tenant-1/bank-transactions/tx-1/suggestions", nil, map[string]string{"tenantID": "tenant-1", "transactionID": "tx-1"})
		resp = httptest.NewRecorder()
		h.GetMatchSuggestions(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "Failed to get match suggestions")

		req = wave7BusinessRawRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/tx-1/match", "{", map[string]string{"tenantID": "tenant-1", "transactionID": "tx-1"})
		resp = httptest.NewRecorder()
		h.MatchBankTransaction(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "Invalid request body")

		req = wave7BusinessRequest(http.MethodPost, "/tenants/tenant-1/bank-transactions/tx-1/match", banking.MatchTransactionRequest{}, map[string]string{"tenantID": "tenant-1", "transactionID": "tx-1"})
		resp = httptest.NewRecorder()
		h.MatchBankTransaction(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		assert.Contains(t, resp.Body.String(), "Payment ID is required")
	})

	t.Run("order pick list ready and reservation skip paths", func(t *testing.T) {
		repo := newMockOrdersRepository()
		inventoryRepo := newMockInventoryRepository()
		h := wave7OrderStockHandlers(repo, inventoryRepo)
		productID, _, warehouseID := wave7SeedOrderStockFixture(repo, inventoryRepo)
		repo.orders["order-1"].Lines = []orders.OrderLine{{
			ID:          "line-ready",
			LineNumber:  1,
			Description: "Reserved item",
			Quantity:    decimal.NewFromInt(3),
			ProductID:   &productID,
		}}
		repo.stockReservations[orderStockReservationKey("order-1", productID, warehouseID)] = &orders.OrderStockReservation{
			ID:          "reservation-1",
			TenantID:    "tenant-1",
			OrderID:     "order-1",
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    decimal.NewFromInt(5),
			Status:      orders.OrderStockReservationStatusReserved,
		}

		pickList, err := h.buildOrderPickList(context.Background(), "tenant-1", "tenant_tenant", "order-1", warehouseID)
		require.NoError(t, err)
		require.True(t, pickList.Ready)
		require.Len(t, pickList.Lines, 1)
		assert.Equal(t, orders.OrderPickListLineStatusReady, pickList.Lines[0].Status)
		assert.True(t, pickList.Lines[0].PickQty.Equal(decimal.NewFromInt(3)))

		result, err := h.applyOrderStockReservation(context.Background(), "tenant-1", "tenant_tenant", orders.OrderStockReservationActionReserve, "", "user-1", &orders.OrderStockCheck{
			OrderID:     "order-empty",
			OrderNumber: "ORD-EMPTY",
			WarehouseID: warehouseID,
			Lines: []orders.OrderStockCheckLine{
				{LineID: "manual", Status: orders.OrderStockLineStatusAvailable},
				{LineID: "missing", ProductID: productID, Status: orders.OrderStockLineStatusProductNotFound},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Empty(t, result.Lines)
	})
}

func TestHighGapWave8MigrationStepBranches(t *testing.T) {
	t.Run("direct executor maps malformed parser inputs", func(t *testing.T) {
		h := &Handlers{bankingService: banking.NewServiceWithRepository(newMockBankingRepository())}
		executor := &handlerMigrationStepExecutor{h: h}

		result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
			cutover.MigrationExecutionStep{Kind: cutover.KindBankAccounts},
			cutover.BundleFile{Kind: cutover.KindBankAccounts, FileName: "bank-accounts.csv", CSVContent: "not,a,bank,account\n"},
			&cutover.ExecuteMigrationRequest{},
		)
		require.Error(t, err)
		assert.Nil(t, result)

		result, err = executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
			cutover.MigrationExecutionStep{Kind: cutover.KindBankTransactions},
			cutover.BundleFile{Kind: cutover.KindBankTransactions, FileName: "bank.csv", CSVContent: "not,a,transaction\n"},
			&cutover.ExecuteMigrationRequest{BankTransactionAccountID: "bank-1"},
		)
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("direct executor maps reference and period-lock dependencies", func(t *testing.T) {
		contactsRepo := newMockContactsRepository()
		contactsRepo.listErr = errors.New("contacts unavailable")
		h := &Handlers{contactsService: contacts.NewServiceWithRepository(contactsRepo)}
		executor := &handlerMigrationStepExecutor{h: h}

		result, err := executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
			cutover.MigrationExecutionStep{Kind: cutover.KindEInvoices},
			cutover.BundleFile{Kind: cutover.KindEInvoices, FileName: "einvoice.xml", XMLContent: "<Invoice/>"},
			&cutover.ExecuteMigrationRequest{EInvoiceInvoiceType: "sales"},
		)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "load contacts")

		tenantRepo := newMockTenantRepository()
		tenantRepo.getTenantErr = errors.New("tenant unavailable")
		h = &Handlers{tenantService: tenant.NewServiceWithRepository(tenantRepo)}
		executor = &handlerMigrationStepExecutor{h: h}
		result, err = executor.ExecuteMigrationStep(context.Background(), "tenant-1", "tenant_tenant", "user-1",
			cutover.MigrationExecutionStep{Kind: cutover.KindJournalEntries},
			cutover.BundleFile{Kind: cutover.KindJournalEntries, FileName: "journal.csv", CSVContent: "entry_reference,entry_date,account_code,debit,credit\nJE-1,2026-01-31,1000,10,0\n"},
			&cutover.ExecuteMigrationRequest{},
		)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "tenant unavailable")
	})
}

func TestHighGapWave8TaxBranches(t *testing.T) {
	t.Run("KMD history import and accepted status errors", func(t *testing.T) {
		h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
		tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")

		errBody := invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleImportKMDHistory, taxHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tax/kmd/import-history",
			tax.ImportKMDHistoryRequest{CSVContent: "year,row_code\n2026,1\n"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.NotEmpty(t, errBody["error"])

		taxRepo.getDecl = &tax.KMDDeclaration{ID: "decl-accepted-missing", TenantID: "tenant-1", Year: 2026, Month: 2}
		installApprovedEvidenceDocuments(t, h, documents.Document{
			ID:           "doc-kmd-accepted-approved",
			EntityType:   documents.EntityTypeKMD,
			EntityID:     "decl-accepted-missing",
			DocumentType: documents.DocumentTypeTaxSupport,
		})
		taxRepo.statusErr = tax.ErrKMDDeclarationNotFound
		errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusNotFound, h.HandleMarkKMDAccepted, taxHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tax/kmd/2026/2/accept",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
		))
		assert.Equal(t, "Declaration not found", errBody["error"])
	})
}

func TestHighGapWave8YearEndBranches(t *testing.T) {
	t.Run("pack and audit attach inventory failures", func(t *testing.T) {
		h, _, _, _ := setupWave6YearEndReady(t)
		attachYearEndInventoryFixture(h, decimal.NewFromInt(5))

		req := wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-pack?period_end_date=2025-12-31&inventory_valuation_method=bad-method", nil, "tenant-1")
		rec := httptest.NewRecorder()
		h.GetYearEndClosePack(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "invalid valuation method")

		req = wave6YearEndRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-evidence?period_end_date=2025-12-31&inventory_valuation_method=bad-method", nil, "tenant-1")
		rec = httptest.NewRecorder()
		h.GetYearEndCloseAuditEvidence(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "invalid valuation method")
	})

	t.Run("close-pack evidence and inventory helpers cover empty and blocking reviews", func(t *testing.T) {
		h, _, _, tenantRecord := setupWave6YearEndReady(t)
		docRepo := newMockDocumentRepository()
		h.documentsService = documents.NewService(docRepo, nil)
		status := &accounting.YearEndCloseStatus{
			IsFiscalYearEnd:           true,
			CarryForwardReady:         true,
			ClosePackEvidenceEntityID: "year-end-close:tenant-1:2025-12-31",
		}
		require.NoError(t, h.attachYearEndCloseEvidenceStatus(context.Background(), tenantRecord.SchemaName, tenantRecord.ID, status))
		require.NotNil(t, status.ClosePackEvidence)
		assert.False(t, status.ClosePackEvidence.Compliant)
		assert.False(t, status.CarryForwardReady)

		attachYearEndInventoryFixture(h, decimal.Zero)
		err := h.requireYearEndInventoryCostingReady(context.Background(), tenantRecord.SchemaName, tenantRecord.ID, tenantRecord.Settings.FiscalYearStart, "2025-12-31", "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "inventory costing review")

		status = &accounting.YearEndCloseStatus{IsFiscalYearEnd: true, CarryForwardReady: true}
		err = h.attachYearEndInventoryCostingReview(context.Background(), tenantRecord.SchemaName, tenantRecord.ID, "", status)
		require.NoError(t, err)
		require.NotNil(t, status.InventoryCostingReview)
		assert.False(t, status.CarryForwardReady)
	})
}

func TestHighGapWave8ReportCSVUnreachableErrorBranches(t *testing.T) {
	_, err := trialBalanceCSV(&accounting.TrialBalance{
		Accounts: []accounting.AccountBalance{{
			AccountCode:  "1000",
			AccountName:  "Cash",
			AccountType:  accounting.AccountTypeAsset,
			DebitBalance: decimal.NewFromInt(1),
			NetBalance:   decimal.NewFromInt(1),
		}},
		TotalDebits: decimal.NewFromInt(1),
	})
	require.NoError(t, err)
	assert.Contains(t, strings.TrimSpace(accountBalanceCSVRow(accounting.AccountBalance{AccountCode: "1000", AccountName: "Cash"})[1]), "Cash")
}
