package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestAPITokenHandlersValidationAndErrorBranches(t *testing.T) {
	h, repo := setupAPITokenHandlers()
	claims := &auth.Claims{UserID: "user-1", Email: "user@example.com", TenantID: "tenant-1", Role: tenant.RoleOwner}

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "list requires authentication",
			handler:    h.ListAPITokens,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/api-tokens", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Not authenticated",
		},
		{
			name:       "create requires authentication",
			handler:    h.CreateAPIToken,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/api-tokens", nil), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Not authenticated",
		},
		{
			name:       "revoke requires authentication",
			handler:    h.RevokeAPIToken,
			request:    withURLParams(httptest.NewRequest(http.MethodDelete, "/tenants/tenant-1/api-tokens/token-1", nil), map[string]string{"tenantID": "tenant-1", "tokenID": "token-1"}),
			wantStatus: http.StatusUnauthorized,
			wantBody:   "Not authenticated",
		},
		{
			name:       "create rejects invalid JSON",
			handler:    h.CreateAPIToken,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/api-tokens", strings.NewReader("{")).WithContext(contextWithClaims(httptest.NewRequest(http.MethodPost, "/", nil).Context(), claims)), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "create requires name",
			handler:    h.CreateAPIToken,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/api-tokens", map[string]string{"name": "  "}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "name is required",
		},
		{
			name:       "create requires future expiry",
			handler:    h.CreateAPIToken,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/api-tokens", map[string]any{"name": "old", "expires_at": time.Now().Add(-time.Hour)}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "expires_at must be in the future",
		},
		{
			name:       "revoke maps missing token to bad request",
			handler:    h.RevokeAPIToken,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/api-tokens/missing", nil, claims), map[string]string{"tenantID": "tenant-1", "tokenID": "missing"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "api token not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			tt.handler(resp, tt.request)
			require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), tt.wantBody)
		})
	}

	h.apiTokenService = apitoken.NewServiceWithRepository(nil)
	req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/api-tokens", nil, claims), map[string]string{"tenantID": "tenant-1"})
	resp := httptest.NewRecorder()
	h.ListAPITokens(resp, req)
	require.Equal(t, http.StatusInternalServerError, resp.Code, resp.Body.String())
	require.Contains(t, resp.Body.String(), "Failed to list API tokens")

	require.Empty(t, repo.tokens)
}

func TestTenantUserAPITokenHandlersAuthorizationAndErrors(t *testing.T) {
	h, tenantRepo, apiTokenRepo := setupTenantUserAPITokenHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	tenantRepo.addTestUser("user-2", "target@example.com", "Target", "password", true)
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "admin-1", Role: tenant.RoleAdmin, IsActive: true, CreatedAt: time.Now()},
		{TenantID: "tenant-1", UserID: "user-2", Role: tenant.RoleViewer, IsActive: true, CreatedAt: time.Now()},
	}
	adminClaims := &auth.Claims{UserID: "admin-1", Email: "admin@example.com", TenantID: "tenant-1", Role: tenant.RoleAdmin}

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "list rejects non-admin claims",
			handler:    h.ListTenantUserAPITokens,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/user-2/api-tokens", nil, &auth.Claims{UserID: "viewer-1", TenantID: "tenant-1", Role: tenant.RoleViewer}), map[string]string{"tenantID": "tenant-1", "userID": "user-2"}),
			wantStatus: http.StatusForbidden,
			wantBody:   "Permission denied",
		},
		{
			name:       "list requires user id",
			handler:    h.ListTenantUserAPITokens,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users//api-tokens", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": ""}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "User id is required",
		},
		{
			name:       "list maps missing membership to not found",
			handler:    h.ListTenantUserAPITokens,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/missing/api-tokens", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": "missing"}),
			wantStatus: http.StatusNotFound,
			wantBody:   "User not found in tenant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			tt.handler(resp, tt.request)
			require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), tt.wantBody)
		})
	}

	h.apiTokenService = nil
	nilServiceReq := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/user-2/api-tokens", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
	nilServiceResp := httptest.NewRecorder()
	h.ListTenantUserAPITokens(nilServiceResp, nilServiceReq)
	require.Equal(t, http.StatusInternalServerError, nilServiceResp.Code, nilServiceResp.Body.String())
	require.Contains(t, nilServiceResp.Body.String(), "API token service unavailable")

	h.apiTokenService = apitoken.NewServiceWithRepository(apiTokenRepo)
	emptyTokenReq := withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/api-tokens/", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": "user-2", "tokenID": "  "})
	emptyTokenResp := httptest.NewRecorder()
	h.RevokeTenantUserAPIToken(emptyTokenResp, emptyTokenReq)
	require.Equal(t, http.StatusBadRequest, emptyTokenResp.Code, emptyTokenResp.Body.String())
	require.Contains(t, emptyTokenResp.Body.String(), "Token id is required")

	missingTokenReq := withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/api-tokens/missing", nil, adminClaims), map[string]string{"tenantID": "tenant-1", "userID": "user-2", "tokenID": "missing"})
	missingTokenResp := httptest.NewRecorder()
	h.RevokeTenantUserAPIToken(missingTokenResp, missingTokenReq)
	require.Equal(t, http.StatusNotFound, missingTokenResp.Code, missingTokenResp.Body.String())
	require.Contains(t, missingTokenResp.Body.String(), "API token not found")
}
