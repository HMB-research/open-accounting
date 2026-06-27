package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

func TestCoreWave4AdminAndTenantPermissionMiddleware(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "owner-1", Role: tenant.RoleOwner, IsActive: true},
		{TenantID: "tenant-1", UserID: "viewer-1", Role: tenant.RoleViewer, IsActive: true},
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	tests := []struct {
		name       string
		handler    http.Handler
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "instance admin requires auth claims",
			handler:    h.RequireInstanceAdmin(next),
			request:    httptest.NewRequest(http.MethodGet, "/admin/plugins", nil),
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Authentication required",
		},
		{
			name:       "instance admin requires tenant context",
			handler:    h.RequireInstanceAdmin(next),
			request:    makeAuthenticatedRequest(http.MethodGet, "/admin/plugins", nil, createTestClaims("owner-1", "owner@example.com", "", "")),
			wantStatus: http.StatusForbidden,
			wantBody:   "Tenant admin context required",
		},
		{
			name:    "instance admin accepts tenant header",
			handler: h.RequireInstanceAdmin(next),
			request: func() *http.Request {
				req := makeAuthenticatedRequest(http.MethodGet, "/admin/plugins", nil, createTestClaims("owner-1", "owner@example.com", "", ""))
				req.Header.Set("X-Tenant-ID", "tenant-1")
				return req
			}(),
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "tenant permission requires auth claims",
			handler:    h.RequireTenantPermission(func(tenant.RolePermissions) bool { return true })(next),
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Authentication required",
		},
		{
			name:       "tenant permission rejects nil check",
			handler:    h.RequireTenantPermission(nil)(next),
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1", nil, createTestClaims("owner-1", "owner@example.com", "", "")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Insufficient permissions",
		},
		{
			name:       "tenant permission rejects insufficient role",
			handler:    h.RequireTenantPermission(func(perms tenant.RolePermissions) bool { return perms.CanManageSettings })(next),
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1", nil, createTestClaims("viewer-1", "viewer@example.com", "", "")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Insufficient permissions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			tt.handler.ServeHTTP(resp, tt.request)
			require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
			if tt.wantBody != "" {
				require.Contains(t, resp.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestCoreWave4AuthLoginRefreshAndPasswordBranches(t *testing.T) {
	t.Run("login rejects suspended tenant membership", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
		repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{
			TenantID:  "tenant-1",
			UserID:    "user-1",
			Role:      tenant.RoleAdmin,
			IsActive:  false,
			CreatedAt: time.Now(),
		}}

		req := makeAuthenticatedRequest(http.MethodPost, "/login", map[string]string{
			"email":     "user@example.com",
			"password":  "password123",
			"tenant_id": "tenant-1",
		}, nil)
		resp := httptest.NewRecorder()

		h.Login(resp, req)

		require.Equal(t, http.StatusForbidden, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Tenant access is suspended")
	})

	t.Run("login reports refresh session creation failures", func(t *testing.T) {
		h, repo := setupAuthTestHandlers()
		repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
		h.refreshSessionService.(*mockRefreshSessionService).createErr = errors.New("store unavailable")

		req := makeAuthenticatedRequest(http.MethodPost, "/login", map[string]string{
			"email":    "user@example.com",
			"password": "password123",
		}, nil)
		resp := httptest.NewRecorder()

		h.Login(resp, req)

		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Failed to create refresh session")
	})

	t.Run("refresh token maps user and session failures", func(t *testing.T) {
		t.Run("service unavailable", func(t *testing.T) {
			h, repo := setupAuthTestHandlers()
			repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
			refreshToken, _, err := h.generateRefreshTokenWithClaims("user-1")
			require.NoError(t, err)
			h.refreshSessionService = nil

			req := makeAuthenticatedRequest(http.MethodPost, "/refresh", map[string]string{"refresh_token": refreshToken}, nil)
			resp := httptest.NewRecorder()
			h.RefreshToken(resp, req)

			require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), "Refresh session service unavailable")
		})

		t.Run("user missing", func(t *testing.T) {
			h, _ := setupAuthTestHandlers()
			refreshToken, _, err := h.generateRefreshTokenWithClaims("missing-user")
			require.NoError(t, err)

			req := makeAuthenticatedRequest(http.MethodPost, "/refresh", map[string]string{"refresh_token": refreshToken}, nil)
			resp := httptest.NewRecorder()
			h.RefreshToken(resp, req)

			require.Equal(t, http.StatusUnauthorized, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), "User not found")
		})

		t.Run("tenant membership suspended", func(t *testing.T) {
			h, repo := setupAuthTestHandlers()
			repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
			repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
			repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{
				TenantID:  "tenant-1",
				UserID:    "user-1",
				Role:      tenant.RoleAdmin,
				IsActive:  false,
				CreatedAt: time.Now(),
			}}
			refreshToken, _, err := h.generateRefreshTokenWithClaims("user-1")
			require.NoError(t, err)

			req := makeAuthenticatedRequest(http.MethodPost, "/refresh", map[string]string{
				"refresh_token": refreshToken,
				"tenant_id":     "tenant-1",
			}, nil)
			resp := httptest.NewRecorder()
			h.RefreshToken(resp, req)

			require.Equal(t, http.StatusForbidden, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), "Tenant access is suspended")
		})

		t.Run("invalid old session", func(t *testing.T) {
			h, repo := setupAuthTestHandlers()
			repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
			refreshToken, _, err := h.generateRefreshTokenWithClaims("user-1")
			require.NoError(t, err)

			req := makeAuthenticatedRequest(http.MethodPost, "/refresh", map[string]string{"refresh_token": refreshToken}, nil)
			resp := httptest.NewRecorder()
			h.RefreshToken(resp, req)

			require.Equal(t, http.StatusUnauthorized, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), "Invalid refresh token")
		})

		t.Run("rotation storage error", func(t *testing.T) {
			h, repo := setupAuthTestHandlers()
			repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
			refreshToken, _, err := h.generateRefreshTokenWithClaims("user-1")
			require.NoError(t, err)
			h.refreshSessionService.(*mockRefreshSessionService).rotateErr = errors.New("rotate failed")

			req := makeAuthenticatedRequest(http.MethodPost, "/refresh", map[string]string{"refresh_token": refreshToken}, nil)
			resp := httptest.NewRecorder()
			h.RefreshToken(resp, req)

			require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), "Failed to rotate refresh session")
		})
	})

	t.Run("change password validation and revoke failures", func(t *testing.T) {
		tests := []struct {
			name       string
			handler    func() *Handlers
			request    func(*Handlers) *http.Request
			wantStatus int
			wantBody   string
		}{
			{
				name: "requires auth claims",
				handler: func() *Handlers {
					h, _ := setupAuthTestHandlers()
					return h
				},
				request: func(*Handlers) *http.Request {
					return httptest.NewRequest(http.MethodPost, "/auth/password", nil)
				},
				wantStatus: http.StatusUnauthorized,
				wantBody:   "Authentication required",
			},
			{
				name: "rejects invalid json",
				handler: func() *Handlers {
					h, _ := setupAuthTestHandlers()
					return h
				},
				request: func(*Handlers) *http.Request {
					return httptest.NewRequest(http.MethodPost, "/auth/password", strings.NewReader("{")).WithContext(contextWithClaims(context.Background(), createTestClaims("user-1", "user@example.com", "", "")))
				},
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name: "requires refresh session service",
				handler: func() *Handlers {
					h, _ := setupAuthTestHandlers()
					h.refreshSessionService = nil
					return h
				},
				request: func(*Handlers) *http.Request {
					return makeAuthenticatedRequest(http.MethodPost, "/auth/password", map[string]string{
						"current_password": "password123",
						"new_password":     "newpassword123",
					}, createTestClaims("user-1", "user@example.com", "", ""))
				},
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Refresh session service unavailable",
			},
			{
				name: "maps current password failure to unauthorized",
				handler: func() *Handlers {
					h, repo := setupAuthTestHandlers()
					repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
					return h
				},
				request: func(*Handlers) *http.Request {
					return makeAuthenticatedRequest(http.MethodPost, "/auth/password", map[string]string{
						"current_password": "wrongpassword",
						"new_password":     "newpassword123",
					}, createTestClaims("user-1", "user@example.com", "", ""))
				},
				wantStatus: http.StatusUnauthorized,
				wantBody:   "current password is incorrect",
			},
			{
				name: "reports revoke all failure after password change",
				handler: func() *Handlers {
					h, repo := setupAuthTestHandlers()
					repo.addTestUser("user-1", "user@example.com", "User One", "password123", true)
					h.refreshSessionService = &failingRefreshSessionService{
						mockRefreshSessionService: newMockRefreshSessionService(),
						revokeAllErr:              errors.New("revoke failed"),
					}
					return h
				},
				request: func(*Handlers) *http.Request {
					return makeAuthenticatedRequest(http.MethodPost, "/auth/password", map[string]string{
						"current_password": "password123",
						"new_password":     "newpassword123",
					}, createTestClaims("user-1", "user@example.com", "", ""))
				},
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to revoke refresh sessions",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h := tt.handler()
				resp := httptest.NewRecorder()
				h.ChangePassword(resp, tt.request(h))
				require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
				require.Contains(t, resp.Body.String(), tt.wantBody)
			})
		}
	})
}

func TestCoreWave4TenantAndAccountingErrorBranches(t *testing.T) {
	t.Run("list my tenants maps repository failure", func(t *testing.T) {
		baseRepo := newMockTenantRepository()
		h := &Handlers{tenantService: tenant.NewServiceWithRepository(&listUserTenantsErrorRepository{
			mockTenantRepository: baseRepo,
			err:                  errors.New("membership lookup failed"),
		})}
		req := makeAuthenticatedRequest(http.MethodGet, "/tenants/my", nil, createTestClaims("user-1", "user@example.com", "", ""))
		resp := httptest.NewRecorder()

		h.ListMyTenants(resp, req)

		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Failed to list tenants")
	})

	t.Run("api tokens cannot create tenants", func(t *testing.T) {
		h, _ := setupTenantTestHandlers()
		claims := createTestClaims("user-1", "user@example.com", "", "")
		claims.TokenKind = auth.TokenKindAPIToken
		req := makeAuthenticatedRequest(http.MethodPost, "/tenants", map[string]string{"name": "Tenant", "slug": "tenant"}, claims)
		resp := httptest.NewRecorder()

		h.CreateTenant(resp, req)

		require.Equal(t, http.StatusForbidden, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "API tokens cannot create tenants")
	})

	t.Run("get tenant maps missing tenant after membership check", func(t *testing.T) {
		h, repo := setupTenantTestHandlers()
		repo.tenantUsers["tenant-1"] = []tenant.TenantUser{{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleAdmin, IsActive: true}}
		req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1", nil, createTestClaims("user-1", "user@example.com", "", "")), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()

		h.GetTenant(resp, req)

		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Tenant not found")
	})

	t.Run("accounting list and hierarchy map repository failures", func(t *testing.T) {
		h, _, repo := setupAccountingTestHandlers()
		repo.listAccountsErr = errors.New("accounts unavailable")

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/accounts", nil), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.ListAccounts(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Failed to list accounts")

		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/accounts/hierarchy", nil), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.GetAccountHierarchy(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Failed to get account hierarchy")
	})

	t.Run("import opening balances validates auth body and dates", func(t *testing.T) {
		h, tenantRepo, _ := setupAccountingTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")

		req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import-opening-balances", nil), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.ImportOpeningBalances(resp, req)
		require.Equal(t, http.StatusUnauthorized, resp.Code, resp.Body.String())

		claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)
		for _, tt := range []struct {
			name string
			body map[string]string
			want string
		}{
			{name: "requires csv", body: map[string]string{"entry_date": "2026-01-01"}, want: "csv_content is required"},
			{name: "requires entry date", body: map[string]string{"csv_content": "code,debit,credit\n1000,1,0\n"}, want: "entry_date is required"},
			{name: "validates entry date", body: map[string]string{"csv_content": "code,debit,credit\n1000,1,0\n", "entry_date": "2026/01/01"}, want: "entry_date must be in YYYY-MM-DD format"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries/import-opening-balances", tt.body, claims), map[string]string{"tenantID": "tenant-1"})
				resp := httptest.NewRecorder()
				h.ImportOpeningBalances(resp, req)
				require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
				require.Contains(t, resp.Body.String(), tt.want)
			})
		}
	})

	t.Run("journal handlers map validation and service failures", func(t *testing.T) {
		h, _, repo := setupAccountingTestHandlers()
		claims := createTestClaims("user-1", "user@example.com", "tenant-1", tenant.RoleOwner)

		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/journal-entries?limit=0", nil), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.ListJournalEntries(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Limit must be between 1 and 200")

		repo.getJournalErr = errors.New("journal unavailable")
		req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/journal-entries", nil), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.ListJournalEntries(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "journal unavailable")

		req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/journal-entries", map[string]any{"lines": []map[string]any{{"account_id": "cash"}}}, claims), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.CreateJournalEntry(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "At least 2 lines required")
	})
}

func TestCoreWave4APITokenWebhookRecurringAndReminderBranches(t *testing.T) {
	t.Run("api token create and tenant revoke internal errors", func(t *testing.T) {
		claims := &auth.Claims{UserID: "user-1", Email: "user@example.com", TenantID: "tenant-1", Role: tenant.RoleOwner}
		repo := &erroringAPITokenRepository{
			mockAPITokenRepository: newMockAPITokenRepository(),
			createErr:              errors.New("create failed"),
		}
		h := &Handlers{apiTokenService: apitoken.NewServiceWithRepository(repo)}
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/api-tokens", map[string]string{"name": "CLI"}, claims), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.CreateAPIToken(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "create failed")

		h, tenantRepo, tokenRepo := setupTenantUserAPITokenHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		tenantRepo.addTestUser("user-2", "target@example.com", "Target", "password123", true)
		tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
			{TenantID: "tenant-1", UserID: "admin-1", Role: tenant.RoleAdmin, IsActive: true},
			{TenantID: "tenant-1", UserID: "user-2", Role: tenant.RoleViewer, IsActive: true},
		}
		h.apiTokenService = apitoken.NewServiceWithRepository(&erroringAPITokenRepository{
			mockAPITokenRepository: tokenRepo,
			revokeErr:              errors.New("storage failed"),
		})
		req = withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/api-tokens/token-1", nil, &auth.Claims{UserID: "admin-1", Email: "admin@example.com", TenantID: "tenant-1", Role: tenant.RoleAdmin}), map[string]string{"tenantID": "tenant-1", "userID": "user-2", "tokenID": "token-1"})
		resp = httptest.NewRecorder()
		h.RevokeTenantUserAPIToken(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Failed to revoke API token")
	})

	t.Run("webhook handlers map validation and service errors", func(t *testing.T) {
		h, _, repo := setupWebhookHandlers("")
		h.webhookService = webhooks.NewServiceWithRepository(&erroringWebhookRepository{
			memoryWebhookRepository: repo,
			listErr:                 errors.New("list failed"),
		}, nil)
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/webhooks", nil), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.ListWebhookEndpoints(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Failed to list webhook endpoints")

		h, _, repo = setupWebhookHandlers("")
		active := true
		endpoint, err := h.webhookService.CreateEndpoint(context.Background(), "tenant-1", &webhooks.CreateEndpointRequest{
			Name:     "CRM",
			URL:      "https://example.com/webhook",
			Events:   []string{"invoice.created"},
			IsActive: &active,
		})
		require.NoError(t, err)
		repo.defaultEndpointID = endpoint.ID

		for _, tt := range []struct {
			name       string
			handler    func(http.ResponseWriter, *http.Request)
			req        *http.Request
			wantStatus int
			wantBody   string
		}{
			{
				name:       "create rejects invalid json",
				handler:    h.CreateWebhookEndpoint,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/webhooks", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "get maps missing endpoint",
				handler:    h.GetWebhookEndpoint,
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/webhooks/missing", nil), map[string]string{"tenantID": "tenant-1", "webhookID": "missing"}),
				wantStatus: http.StatusNotFound,
				wantBody:   "Webhook endpoint not found",
			},
			{
				name:       "update rejects invalid json",
				handler:    h.UpdateWebhookEndpoint,
				req:        withURLParams(httptest.NewRequest(http.MethodPut, "/tenants/tenant-1/webhooks/"+endpoint.ID, strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "webhookID": endpoint.ID}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
			{
				name:       "deliveries reject invalid limit",
				handler:    h.ListWebhookDeliveries,
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/webhooks/"+endpoint.ID+"/deliveries?limit=201", nil), map[string]string{"tenantID": "tenant-1", "webhookID": endpoint.ID}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Limit must be between 1 and 200",
			},
			{
				name:       "test rejects invalid json",
				handler:    h.TestWebhookEndpoint,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/webhooks/"+endpoint.ID+"/test", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "webhookID": endpoint.ID}),
				wantStatus: http.StatusBadRequest,
				wantBody:   "Invalid request body",
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				resp := httptest.NewRecorder()
				tt.handler(resp, tt.req)
				require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
				require.Contains(t, resp.Body.String(), tt.wantBody)
			})
		}
	})

	t.Run("recurring handlers cover validation and repository errors", func(t *testing.T) {
		h, tenantRepo, recurringRepo, _ := setupRecurringTestHandlers()
		tenantRepo.addTestTenant("tenant-1", "Recurring Tenant", "recurring-tenant")
		claims := &auth.Claims{UserID: "user-1", Email: "user@example.com", TenantID: "tenant-1", Role: tenant.RoleOwner}

		recurringRepo.listErr = errors.New("list failed")
		req := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/recurring-invoices", nil), map[string]string{"tenantID": "tenant-1"})
		resp := httptest.NewRecorder()
		h.ListRecurringInvoices(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "Failed to list recurring invoices")
		recurringRepo.listErr = nil

		req = withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.CreateRecurringInvoice(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())

		req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/import", map[string]string{"csv_content": ""}, claims), map[string]string{"tenantID": "tenant-1"})
		resp = httptest.NewRecorder()
		h.ImportRecurringInvoices(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "csv_content is required")

		recurringRepo.setActiveErr = errors.New("status failed")
		req = withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/recurring-invoices/rec-1/pause", nil, claims), map[string]string{"tenantID": "tenant-1", "recurringID": "rec-1"})
		resp = httptest.NewRecorder()
		h.PauseRecurringInvoice(resp, req)
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		require.Contains(t, resp.Body.String(), "status failed")
	})

	t.Run("reminder rule handlers map service errors", func(t *testing.T) {
		h, tenantRepo, _, _, _, baseRepo := setupMiscHandlers()
		tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
		errorRepo := &erroringReminderRuleRepository{
			mockReminderRuleRepository: baseRepo,
			listErr:                    errors.New("list failed"),
			getErr:                     errors.New("get failed"),
			createErr:                  errors.New("create failed"),
			updateErr:                  errors.New("update failed"),
			deleteErr:                  errors.New("delete failed"),
			activeErr:                  errors.New("active failed"),
		}
		h.automatedReminderService = invoicing.NewAutomatedReminderServiceWithRepository(errorRepo, nil)

		tests := []struct {
			name       string
			handler    func(http.ResponseWriter, *http.Request)
			req        *http.Request
			wantStatus int
			wantBody   string
		}{
			{
				name:       "list failure",
				handler:    h.ListReminderRules,
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reminder-rules", nil), map[string]string{"tenantID": "tenant-1"}),
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to list rules",
			},
			{
				name:       "get failure",
				handler:    h.GetReminderRule,
				req:        withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/reminder-rules/rule-1", nil), map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"}),
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to get rule",
			},
			{
				name:       "create repository failure",
				handler:    h.CreateReminderRule,
				req:        withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/reminder-rules", map[string]any{"name": "Overdue", "trigger_type": invoicing.TriggerAfterDue, "days_offset": 7}, nil), map[string]string{"tenantID": "tenant-1"}),
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to create rule",
			},
			{
				name:       "update repository failure",
				handler:    h.UpdateReminderRule,
				req:        withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/reminder-rules/rule-1", map[string]string{"name": "Updated"}, nil), map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"}),
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to update rule",
			},
			{
				name:       "delete repository failure",
				handler:    h.DeleteReminderRule,
				req:        withURLParams(httptest.NewRequest(http.MethodDelete, "/tenants/tenant-1/reminder-rules/rule-1", nil), map[string]string{"tenantID": "tenant-1", "ruleID": "rule-1"}),
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to delete rule",
			},
			{
				name:       "trigger active rules failure",
				handler:    h.TriggerReminders,
				req:        withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/reminder-rules/trigger", nil), map[string]string{"tenantID": "tenant-1"}),
				wantStatus: http.StatusInternalServerError,
				wantBody:   "Failed to process reminders",
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
}

type listUserTenantsErrorRepository struct {
	*mockTenantRepository
	err error
}

func (r *listUserTenantsErrorRepository) ListUserTenants(ctx context.Context, userID string) ([]tenant.TenantMembership, error) {
	return nil, r.err
}

type erroringAPITokenRepository struct {
	*mockAPITokenRepository
	listErr   error
	createErr error
	revokeErr error
}

func (r *erroringAPITokenRepository) CreateToken(ctx context.Context, token *apitoken.APIToken, tokenHash string) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.mockAPITokenRepository.CreateToken(ctx, token, tokenHash)
}

func (r *erroringAPITokenRepository) ListTokens(ctx context.Context, userID, tenantID string) ([]apitoken.APIToken, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.mockAPITokenRepository.ListTokens(ctx, userID, tenantID)
}

func (r *erroringAPITokenRepository) RevokeToken(ctx context.Context, userID, tenantID, tokenID string, revokedAt time.Time) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	return r.mockAPITokenRepository.RevokeToken(ctx, userID, tenantID, tokenID, revokedAt)
}

type erroringWebhookRepository struct {
	*memoryWebhookRepository
	listErr error
}

func (r *erroringWebhookRepository) ListEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]webhooks.Endpoint, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.memoryWebhookRepository.ListEndpoints(ctx, tenantID, activeOnly)
}

type erroringReminderRuleRepository struct {
	*mockReminderRuleRepository
	listErr   error
	activeErr error
	getErr    error
	createErr error
	updateErr error
	deleteErr error
}

func (r *erroringReminderRuleRepository) ListRules(ctx context.Context, schemaName, tenantID string) ([]invoicing.ReminderRule, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.mockReminderRuleRepository.ListRules(ctx, schemaName, tenantID)
}

func (r *erroringReminderRuleRepository) ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]invoicing.ReminderRule, error) {
	if r.activeErr != nil {
		return nil, r.activeErr
	}
	return r.mockReminderRuleRepository.ListActiveRules(ctx, schemaName, tenantID)
}

func (r *erroringReminderRuleRepository) GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*invoicing.ReminderRule, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.mockReminderRuleRepository.GetRule(ctx, schemaName, tenantID, ruleID)
}

func (r *erroringReminderRuleRepository) CreateRule(ctx context.Context, schemaName string, rule *invoicing.ReminderRule) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.mockReminderRuleRepository.CreateRule(ctx, schemaName, rule)
}

func (r *erroringReminderRuleRepository) UpdateRule(ctx context.Context, schemaName string, rule *invoicing.ReminderRule) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.mockReminderRuleRepository.UpdateRule(ctx, schemaName, rule)
}

func (r *erroringReminderRuleRepository) DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	return r.mockReminderRuleRepository.DeleteRule(ctx, schemaName, tenantID, ruleID)
}
