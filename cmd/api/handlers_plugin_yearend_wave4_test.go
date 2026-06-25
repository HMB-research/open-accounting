package main

import (
	"archive/zip"
	"bytes"
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
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestPluginWave4AdminHandlerBranches(t *testing.T) {
	t.Run("admin management maps service errors", func(t *testing.T) {
		h, repo := setupPluginTestHandlers(t)
		now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
		registryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
		pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
		repo.registries[registryID] = &plugin.Registry{
			ID:         registryID,
			Name:       "Official",
			URL:        "https://github.com/HMB-research/open-accounting-plugins",
			IsOfficial: true,
			IsActive:   true,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		repo.plugins[pluginID] = &plugin.Plugin{
			ID:             pluginID,
			Name:           "managed-plugin",
			DisplayName:    "Managed Plugin",
			Version:        "1.0.0",
			RepositoryURL:  "https://github.com/HMB-research/managed-plugin",
			RepositoryType: plugin.RepoGitHub,
			State:          plugin.StateInstalled,
			Manifest:       json.RawMessage(`{"name":"managed-plugin","version":"1.0.0"}`),
			InstalledAt:    now,
			UpdatedAt:      now,
		}

		tests := []struct {
			name       string
			handler    func(http.ResponseWriter, *http.Request)
			req        *http.Request
			wantStatus int
			wantBody   string
		}{
			{
				name:       "remove official registry",
				handler:    h.RemovePluginRegistry,
				req:        withURLParams(httptest.NewRequest(http.MethodDelete, "/admin/plugin-registries/"+registryID.String(), nil), map[string]string{"id": registryID.String()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "cannot be removed",
			},
			{
				name:       "sync missing registry",
				handler:    h.SyncPluginRegistry,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugin-registries/"+uuid.NewString()+"/sync", nil), map[string]string{"id": uuid.NewString()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "registry not found",
			},
			{
				name:       "enable rejects invalid JSON",
				handler:    h.EnablePlugin,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/enable", strings.NewReader("{")), map[string]string{"id": pluginID.String()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "enable missing plugin",
				handler:    h.EnablePlugin,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+uuid.NewString()+"/enable", strings.NewReader(`{"granted_permissions":[]}`)), map[string]string{"id": uuid.NewString()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "plugin not found",
			},
			{
				name:       "disable missing plugin",
				handler:    h.DisablePlugin,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+uuid.NewString()+"/disable", nil), map[string]string{"id": uuid.NewString()}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "plugin not found",
			},
			{
				name:       "get missing plugin",
				handler:    h.GetPlugin,
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/admin/plugins/"+uuid.NewString(), nil), map[string]string{"id": uuid.NewString()}),
				wantStatus: http.StatusNotFound,
				wantBody:   "plugin not found",
			},
			{
				name:       "runtime status missing plugin",
				handler:    h.GetPluginRuntimeStatus,
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/admin/plugins/"+uuid.NewString()+"/runtime", nil), map[string]string{"id": uuid.NewString()}),
				wantStatus: http.StatusNotFound,
				wantBody:   "plugin not found",
			},
			{
				name:       "restart missing plugin",
				handler:    h.RestartPluginRuntime,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+uuid.NewString()+"/runtime/restart", nil), map[string]string{"id": uuid.NewString()}),
				wantStatus: http.StatusNotFound,
				wantBody:   "plugin not found",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := httptest.NewRecorder()
				tt.handler(resp, tt.req)
				require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
				require.Contains(t, resp.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("restart maps disabled plugin runtime error", func(t *testing.T) {
		h, repo := setupPluginTestHandlers(t)
		pluginID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
		repo.plugins[pluginID] = &plugin.Plugin{
			ID:             pluginID,
			Name:           "disabled-plugin",
			DisplayName:    "Disabled Plugin",
			Version:        "1.0.0",
			RepositoryURL:  "https://github.com/HMB-research/disabled-plugin",
			RepositoryType: plugin.RepoGitHub,
			State:          plugin.StateDisabled,
			Manifest:       json.RawMessage(`{"name":"disabled-plugin","version":"1.0.0","backend":{"runtime":"http","base_url":"http://127.0.0.1:1"}}`),
			InstalledAt:    time.Now(),
			UpdatedAt:      time.Now(),
		}

		req := withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/runtime/restart", nil), map[string]string{"id": pluginID.String()})
		resp := httptest.NewRecorder()
		h.RestartPluginRuntime(resp, req)

		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "plugin is not enabled")
	})
}

func TestPluginWave4TenantHandlerBranches(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := "user-1"
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Plugin Tenant", "plugin-tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{{
		TenantID: tenantID.String(),
		UserID:   userID,
		Role:     tenant.RoleAdmin,
		IsActive: true,
	}}
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "tenant-plugin",
		DisplayName:        "Tenant Plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/tenant-plugin",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"routes:register"},
		Manifest: json.RawMessage(`{
			"name":"tenant-plugin",
			"version":"1.0.0",
			"permissions":["routes:register"],
			"backend":{"runtime":"http","base_url":"http://127.0.0.1:1","routes":[{"method":"GET","path":"/status","handler":"/status"}]}
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	claims := &auth.Claims{UserID: userID, TenantID: tenantID.String(), Role: tenant.RoleAdmin}
	viewerClaims := &auth.Claims{UserID: "viewer-1", TenantID: tenantID.String(), Role: tenant.RoleViewer}

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		req        *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "enable requires authentication",
			handler:    h.EnableTenantPlugin,
			req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable", nil), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()}),
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Not authenticated",
		},
		{
			name:       "enable rejects invalid tenant id",
			handler:    h.EnableTenantPlugin,
			req:        withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/bad/plugins/"+pluginID.String()+"/enable", nil, claims), map[string]string{"tenantID": "bad", "pluginID": pluginID.String()}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid tenant ID",
		},
		{
			name:       "enable rejects invalid plugin id",
			handler:    h.EnableTenantPlugin,
			req:        withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/bad/enable", nil, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid plugin ID",
		},
		{
			name:       "enable requires admin role",
			handler:    h.EnableTenantPlugin,
			req:        withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable", nil, viewerClaims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Admin access required",
		},
		{
			name:       "settings requires access",
			handler:    h.GetTenantPluginSettings,
			req:        withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", nil, &auth.Claims{UserID: "other", TenantID: tenantID.String()}), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Access denied",
		},
		{
			name:       "update settings rejects invalid json",
			handler:    h.UpdateTenantPluginSettings,
			req:        withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", strings.NewReader("{")).WithContext(contextWithClaims(context.Background(), claims)), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "invoke rejects invalid tenant id",
			handler:    h.InvokeTenantPluginRoute,
			req:        withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/bad/plugins/"+pluginID.String()+"/runtime/status", nil, claims), map[string]string{"tenantID": "bad", "pluginID": pluginID.String(), "*": "status"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid tenant ID",
		},
		{
			name:       "invoke requires access",
			handler:    h.InvokeTenantPluginRoute,
			req:        withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status", nil, &auth.Claims{UserID: "other", TenantID: tenantID.String()}), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "status"}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Access denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			tt.handler(resp, tt.req)
			require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), tt.wantBody)
		})
	}

	enableReq := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable", nil, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	enableResp := httptest.NewRecorder()
	h.EnableTenantPlugin(enableResp, enableReq)
	require.Equal(t, http.StatusOK, enableResp.Code, enableResp.Body.String())

	repo.plugins[pluginID].Manifest = json.RawMessage(`{"name":"tenant-plugin","version":"1.0.0","permissions":["routes:register"],"backend":{"runtime":"http","base_url":"http://127.0.0.1:1","routes":[{"method":"POST","path":"/status","handler":"/status"}]}}`)
	routeReq := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status", nil, claims), map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "status"})
	routeResp := httptest.NewRecorder()
	h.InvokeTenantPluginRoute(routeResp, routeReq)
	require.Equal(t, http.StatusNotFound, routeResp.Code, routeResp.Body.String())
	require.Contains(t, routeResp.Body.String(), "Plugin route is not registered")
}

func TestYearEndWave4ValidationAndErrorBranches(t *testing.T) {
	t.Run("read handlers require period end date", func(t *testing.T) {
		h, _, _ := setupTenantAccountingHandlers()
		handlers := []struct {
			name    string
			handler func(http.ResponseWriter, *http.Request)
			path    string
		}{
			{name: "status", handler: h.GetYearEndCloseStatus, path: "/tenants/tenant-1/year-end-close-status"},
			{name: "pack", handler: h.GetYearEndClosePack, path: "/tenants/tenant-1/year-end-close-pack"},
			{name: "audit evidence", handler: h.GetYearEndCloseAuditEvidence, path: "/tenants/tenant-1/year-end-close-audit-evidence"},
			{name: "audit archive", handler: h.DownloadYearEndCloseAuditArchive, path: "/tenants/tenant-1/year-end-close-audit-archive"},
		}

		for _, tt := range handlers {
			t.Run(tt.name, func(t *testing.T) {
				req := withURLParams(makeAuthenticatedRequest(http.MethodGet, tt.path, nil, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
				resp := httptest.NewRecorder()
				tt.handler(resp, req)
				require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
				require.Contains(t, resp.Body.String(), "period end date is required")
			})
		}
	})

	t.Run("status and pack map tenant and accounting errors", func(t *testing.T) {
		h, repo, accountingRepo := setupTenantAccountingHandlers()
		req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-status?period_end_date=2025-12-31", nil, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.GetYearEndCloseStatus(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Tenant not found")

		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		accountingRepo.periodBalanceErr = errors.New("period end date is invalid")
		resp = httptest.NewRecorder()
		h.GetYearEndCloseStatus(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "period end date")

		resp = httptest.NewRecorder()
		h.GetYearEndClosePack(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "period end date")
	})

	t.Run("audit archive requires document service", func(t *testing.T) {
		h, repo, _ := setupTenantAccountingHandlers()
		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-archive?period_end_date=2025-12-31", nil, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()

		h.DownloadYearEndCloseAuditArchive(resp, req)

		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Document storage is not configured")
	})

	t.Run("carry forward handlers validate auth and request body", func(t *testing.T) {
		h, repo, _ := setupTenantAccountingHandlers()
		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")

		req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", nil), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.CreateYearEndCarryForward(resp, req)
		require.Equal(t, http.StatusUnauthorized, resp.Code, resp.Body.String())

		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "viewer-1", Role: tenant.RoleViewer, IsActive: true}}
		req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", map[string]string{}, createTestClaims("viewer-1", "viewer@example.com", "tenant-1", tenant.RoleViewer)), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.CreateYearEndCarryForward(resp, req)
		require.Equal(t, http.StatusForbidden, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Insufficient permissions")

		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "owner-1", Role: tenant.RoleOwner, IsActive: true}}
		req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward", strings.NewReader("{")).WithContext(contextWithClaims(context.Background(), createTestClaims("owner-1", "owner@example.com", "tenant-1", tenant.RoleOwner))), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.CreateYearEndCarryForward(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Invalid request body")

		req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward/reverse", strings.NewReader("{")).WithContext(contextWithClaims(context.Background(), createTestClaims("owner-1", "owner@example.com", "tenant-1", tenant.RoleOwner))), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.ReverseYearEndCarryForward(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Invalid request body")
	})

	t.Run("reverse maps tenant and workflow errors", func(t *testing.T) {
		h, repo, accountingRepo := setupTenantAccountingHandlers()
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "owner-1", Role: tenant.RoleOwner, IsActive: true}}
		reverseReq := func() *http.Request {
			return withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/year-end-carry-forward/reverse", accounting.ReverseYearEndCarryForwardRequest{
				PeriodEndDate: "2025-12-31",
				Reason:        "Correction",
			}, createTestClaims("owner-1", "owner@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		}
		req := reverseReq()
		resp := httptest.NewRecorder()
		h.ReverseYearEndCarryForward(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())

		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		accountingRepo.getJournalErr = errors.New("carry-forward does not exist")
		req = reverseReq()
		resp = httptest.NewRecorder()
		h.ReverseYearEndCarryForward(resp, req)
		require.Equal(t, http.StatusConflict, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "carry-forward does not exist")
	})
}

func TestYearEndWave4AuditArchiveAndInventoryHelpers(t *testing.T) {
	t.Run("audit evidence includes attached documents and archive contents", func(t *testing.T) {
		h, repo, accountingRepo := setupTenantAccountingHandlers()
		docRepo := newMockDocumentRepository()
		store, err := documents.NewLocalStore(t.TempDir())
		require.NoError(t, err)
		h.documentsService = documents.NewService(docRepo, store)
		tenantRecord := repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		seedYearEndAccountingReady(accountingRepo)
		entityID, err := accounting.YearEndCloseEvidenceEntityID("tenant-1", "2025-12-31")
		require.NoError(t, err)
		storageKey := "tenant-1/year-end/close-pack.pdf"
		docRepo.docs["close-pack"] = &documents.Document{
			ID:           "close-pack",
			TenantID:     "tenant-1",
			EntityType:   documents.EntityTypeYearEndClose,
			EntityID:     entityID,
			DocumentType: documents.DocumentTypeClosePack,
			FileName:     "Close Pack 2025.pdf",
			ContentType:  "application/pdf",
			FileSize:     11,
			StorageKey:   storageKey,
			ReviewStatus: documents.ReviewStatusApproved,
			UploadedBy:   "user-1",
			CreatedAt:    time.Now(),
		}
		require.NoError(t, store.Save(context.Background(), storageKey, bytes.NewBufferString("hello world")))

		audit, err := h.buildYearEndCloseAuditEvidence(context.Background(), tenantRecord, "2025-12-31", inventory.InventoryValuationMethodStandardCost)
		require.NoError(t, err)
		require.Len(t, audit.Documents, 1)

		archive, err := h.buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)
		require.NoError(t, err)
		reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		require.NoError(t, err)
		names := make([]string, 0, len(reader.File))
		for _, file := range reader.File {
			names = append(names, file.Name)
		}
		require.Contains(t, names, "manifest.json")
		require.Contains(t, names, "documents/close-pack-Close_Pack_2025.pdf")
	})

	t.Run("safe archive filenames normalize blank and unsafe names", func(t *testing.T) {
		require.Equal(t, "file", safeArchiveFileName(" ../ "))
		require.Equal(t, "Close_Pack_2025.pdf", safeArchiveFileName("Close Pack 2025.pdf"))
	})

	t.Run("inventory costing review counts all blocking categories", func(t *testing.T) {
		h, _, _ := setupTenantAccountingHandlers()
		inventoryRepo := newMockInventoryRepository()
		h.inventoryService = inventory.NewServiceWithRepository(inventoryRepo)
		inventoryRepo.products["product-1"] = &inventory.Product{
			ID:             "product-1",
			TenantID:       "tenant-1",
			Code:           "SKU-NEG",
			Name:           "Negative stock",
			ProductType:    inventory.ProductTypeGoods,
			TrackInventory: true,
			IsActive:       true,
			CurrentStock:   decimal.NewFromInt(-2),
			PurchasePrice:  decimal.NewFromInt(5),
		}
		inventoryRepo.warehouses["warehouse-1"] = &inventory.Warehouse{ID: "warehouse-1", TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}
		inventoryRepo.stockLevels[apiInventoryStockLevelKey("product-1", "warehouse-1")] = &inventory.StockLevel{
			ID:           "stock-negative",
			TenantID:     "tenant-1",
			ProductID:    "product-1",
			WarehouseID:  "warehouse-1",
			Quantity:     decimal.NewFromInt(-2),
			ReservedQty:  decimal.NewFromInt(1),
			AvailableQty: decimal.NewFromInt(-3),
		}
		inventoryRepo.movements["product-1"] = []inventory.InventoryMovement{{
			ID:           "movement-negative",
			TenantID:     "tenant-1",
			ProductID:    "product-1",
			WarehouseID:  "warehouse-1",
			MovementType: inventory.MovementTypeIn,
			Quantity:     decimal.NewFromInt(-2),
			UnitCost:     decimal.NewFromInt(5),
			TotalCost:    decimal.NewFromInt(-10),
			MovementDate: time.Now(),
		}}

		review, err := h.yearEndInventoryCostingReview(context.Background(), "tenant_tenant", "tenant-1", inventory.InventoryValuationMethodStandardCost)
		require.NoError(t, err)
		require.False(t, review.Ready)
		require.Equal(t, 1, review.NegativeQuantityLineCount)
		require.Equal(t, 1, review.NegativeAvailableLineCount)
		require.Equal(t, 1, review.NegativeValueLineCount)
		require.Equal(t, 1, review.BlockingExceptionLineCount)
	})

	t.Run("archive builder returns document open errors", func(t *testing.T) {
		h, repo, _ := setupTenantAccountingHandlers()
		h.documentsService = documents.NewService(newMockDocumentRepository(), nil)
		tenantRecord := repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		audit := &accounting.YearEndCloseAuditEvidence{Documents: []documents.Document{{ID: "missing"}}}

		_, err := h.buildYearEndCloseAuditArchive(context.Background(), tenantRecord, audit)
		require.Error(t, err)
	})

	t.Run("audit archive handler downloads zip", func(t *testing.T) {
		h, repo, accountingRepo := setupTenantAccountingHandlers()
		docRepo := newMockDocumentRepository()
		store, err := documents.NewLocalStore(t.TempDir())
		require.NoError(t, err)
		h.documentsService = documents.NewService(docRepo, store)
		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		seedYearEndAccountingReady(accountingRepo)
		entityID, err := accounting.YearEndCloseEvidenceEntityID("tenant-1", "2025-12-31")
		require.NoError(t, err)
		storageKey := "tenant-1/year-end/close-pack.pdf"
		docRepo.docs["close-pack"] = &documents.Document{
			ID:           "close-pack",
			TenantID:     "tenant-1",
			EntityType:   documents.EntityTypeYearEndClose,
			EntityID:     entityID,
			DocumentType: documents.DocumentTypeClosePack,
			FileName:     "close-pack.pdf",
			ContentType:  "application/pdf",
			FileSize:     7,
			StorageKey:   storageKey,
			ReviewStatus: documents.ReviewStatusApproved,
			UploadedBy:   "user-1",
			CreatedAt:    time.Now(),
		}
		require.NoError(t, store.Save(context.Background(), storageKey, bytes.NewBufferString("content")))

		req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/year-end-close-audit-archive?period_end_date=2025-12-31", nil, createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.DownloadYearEndCloseAuditArchive(resp, req)

		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		require.Equal(t, "application/zip", resp.Header().Get("Content-Type"))
		require.Contains(t, resp.Header().Get("Content-Disposition"), "year-end-close-audit-2025-12-31.zip")
		_, err = zip.NewReader(bytes.NewReader(resp.Body.Bytes()), int64(resp.Body.Len()))
		require.NoError(t, err)
	})
}
