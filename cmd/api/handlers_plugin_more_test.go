package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func TestPluginAdminHandlersValidationAndErrorBranches(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	registryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo.registries[registryID] = &plugin.Registry{
		ID:        registryID,
		Name:      "Community",
		URL:       "https://github.com/HMB-research/community-plugins",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:             pluginID,
		Name:           "installed-plugin",
		DisplayName:    "Installed Plugin",
		Version:        "1.0.0",
		RepositoryURL:  "https://github.com/HMB-research/installed-plugin",
		RepositoryType: plugin.RepoGitHub,
		State:          plugin.StateInstalled,
		Manifest:       json.RawMessage(`{"name":"installed-plugin","version":"1.0.0"}`),
		InstalledAt:    now,
		UpdatedAt:      now,
	}

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "add registry rejects invalid JSON",
			handler:    h.AddPluginRegistry,
			request:    httptest.NewRequest(http.MethodPost, "/admin/plugin-registries", strings.NewReader("{")),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "add registry requires name and URL",
			handler:    h.AddPluginRegistry,
			request:    makeAuthenticatedRequest(http.MethodPost, "/admin/plugin-registries", map[string]string{"name": "Missing URL"}, nil),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Name and URL are required",
		},
		{
			name:       "add registry rejects unsupported URL",
			handler:    h.AddPluginRegistry,
			request:    makeAuthenticatedRequest(http.MethodPost, "/admin/plugin-registries", plugin.CreateRegistryRequest{Name: "Bad", URL: "https://example.com/plugins"}, nil),
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid registry URL",
		},
		{
			name:       "remove registry rejects invalid ID",
			handler:    h.RemovePluginRegistry,
			request:    withURLParams(httptest.NewRequest(http.MethodDelete, "/admin/plugin-registries/bad", nil), map[string]string{"id": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid registry ID",
		},
		{
			name:       "sync registry rejects invalid ID",
			handler:    h.SyncPluginRegistry,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugin-registries/bad/sync", nil), map[string]string{"id": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid registry ID",
		},
		{
			name:       "search requires query",
			handler:    h.SearchPlugins,
			request:    httptest.NewRequest(http.MethodGet, "/admin/plugins/search", nil),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Search query is required",
		},
		{
			name:       "install rejects invalid JSON",
			handler:    h.InstallPlugin,
			request:    httptest.NewRequest(http.MethodPost, "/admin/plugins/install", strings.NewReader("{")),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid request body",
		},
		{
			name:       "install requires repository URL",
			handler:    h.InstallPlugin,
			request:    makeAuthenticatedRequest(http.MethodPost, "/admin/plugins/install", plugin.InstallPluginRequest{}, nil),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Repository URL is required",
		},
		{
			name:       "install rejects unsupported repository URL",
			handler:    h.InstallPlugin,
			request:    makeAuthenticatedRequest(http.MethodPost, "/admin/plugins/install", plugin.InstallPluginRequest{RepositoryURL: "https://example.com/plugin.git"}, nil),
			wantStatus: http.StatusBadRequest,
			wantBody:   "invalid repository URL",
		},
		{
			name:       "uninstall rejects invalid ID",
			handler:    h.UninstallPlugin,
			request:    withURLParams(httptest.NewRequest(http.MethodDelete, "/admin/plugins/bad", nil), map[string]string{"id": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid plugin ID",
		},
		{
			name:       "get plugin rejects invalid ID",
			handler:    h.GetPlugin,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/admin/plugins/bad", nil), map[string]string{"id": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid plugin ID",
		},
		{
			name:       "runtime status rejects invalid ID",
			handler:    h.GetPluginRuntimeStatus,
			request:    withURLParams(httptest.NewRequest(http.MethodGet, "/admin/plugins/bad/runtime", nil), map[string]string{"id": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid plugin ID",
		},
		{
			name:       "restart rejects invalid ID",
			handler:    h.RestartPluginRuntime,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/admin/plugins/bad/runtime/restart", nil), map[string]string{"id": "bad"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid plugin ID",
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

	removeReq := withURLParams(httptest.NewRequest(http.MethodDelete, "/admin/plugin-registries/"+registryID.String(), nil), map[string]string{"id": registryID.String()})
	removeResp := httptest.NewRecorder()
	h.RemovePluginRegistry(removeResp, removeReq)
	require.Equal(t, http.StatusNoContent, removeResp.Code, removeResp.Body.String())

	searchReq := httptest.NewRequest(http.MethodGet, "/admin/plugins/search?q=bank", nil)
	searchResp := httptest.NewRecorder()
	h.SearchPlugins(searchResp, searchReq)
	require.Equal(t, http.StatusOK, searchResp.Code, searchResp.Body.String())
	require.JSONEq(t, `null`, searchResp.Body.String())

	getReq := withURLParams(httptest.NewRequest(http.MethodGet, "/admin/plugins/"+pluginID.String(), nil), map[string]string{"id": pluginID.String()})
	getResp := httptest.NewRecorder()
	h.GetPlugin(getResp, getReq)
	require.Equal(t, http.StatusOK, getResp.Code, getResp.Body.String())

	uninstallReq := withURLParams(httptest.NewRequest(http.MethodDelete, "/admin/plugins/"+pluginID.String(), nil), map[string]string{"id": pluginID.String()})
	uninstallResp := httptest.NewRecorder()
	h.UninstallPlugin(uninstallResp, uninstallReq)
	require.Equal(t, http.StatusNoContent, uninstallResp.Code, uninstallResp.Body.String())
}

func TestTenantPluginHandlersValidationAndRuntimeErrors(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := "user-1"
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Demo Tenant", "demo-tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{{
		TenantID: tenantID.String(),
		UserID:   userID,
		Role:     tenant.RoleAdmin,
		IsActive: true,
	}}
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)

	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "runtime-plugin",
		DisplayName:        "Runtime Plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/runtime-plugin",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"routes:register"},
		Manifest: json.RawMessage(`{
			"name":"runtime-plugin",
			"display_name":"Runtime Plugin",
			"version":"1.0.0",
			"permissions":["routes:register"],
			"backend":{
				"runtime":"http",
				"base_url":"http://127.0.0.1:1",
				"routes":[{"method":"GET","path":"/status","handler":"/routes/status"}]
			}
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	claims := &auth.Claims{UserID: userID, TenantID: tenantID.String(), Role: tenant.RoleAdmin}
	unauthReq := withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins", nil), map[string]string{"tenantID": tenantID.String()})
	unauthResp := httptest.NewRecorder()
	h.ListTenantPlugins(unauthResp, unauthReq)
	require.Equal(t, http.StatusUnauthorized, unauthResp.Code, unauthResp.Body.String())

	badTenantReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/bad/plugins", nil, claims)
	badTenantReq = withURLParams(badTenantReq, map[string]string{"tenantID": "bad"})
	badTenantResp := httptest.NewRecorder()
	h.ListTenantPlugins(badTenantResp, badTenantReq)
	require.Equal(t, http.StatusBadRequest, badTenantResp.Code, badTenantResp.Body.String())

	badPluginReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/bad/disable", nil, claims)
	badPluginReq = withURLParams(badPluginReq, map[string]string{"tenantID": tenantID.String(), "pluginID": "bad"})
	badPluginResp := httptest.NewRecorder()
	h.DisableTenantPlugin(badPluginResp, badPluginReq)
	require.Equal(t, http.StatusBadRequest, badPluginResp.Code, badPluginResp.Body.String())

	viewerReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/disable", nil, &auth.Claims{UserID: "viewer-1", TenantID: tenantID.String(), Role: tenant.RoleViewer})
	viewerReq = withURLParams(viewerReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	viewerResp := httptest.NewRecorder()
	h.DisableTenantPlugin(viewerResp, viewerReq)
	require.Equal(t, http.StatusForbidden, viewerResp.Code, viewerResp.Body.String())

	notEnabledReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status", nil, claims)
	notEnabledReq = withURLParams(notEnabledReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "status"})
	notEnabledResp := httptest.NewRecorder()
	h.InvokeTenantPluginRoute(notEnabledResp, notEnabledReq)
	require.Equal(t, http.StatusForbidden, notEnabledResp.Code, notEnabledResp.Body.String())
	require.Contains(t, notEnabledResp.Body.String(), "Plugin is not enabled")

	repo.tenantPlugins[pluginTenantKey(tenantID, pluginID)] = &plugin.TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  json.RawMessage(`{"account":"1000"}`),
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	settingsReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", nil, claims)
	settingsReq = withURLParams(settingsReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	settingsResp := httptest.NewRecorder()
	h.GetTenantPluginSettings(settingsResp, settingsReq)
	require.Equal(t, http.StatusOK, settingsResp.Code, settingsResp.Body.String())
	require.JSONEq(t, `{"account":"1000"}`, settingsResp.Body.String())

	updateReq := makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/settings", map[string]string{"account": "2000"}, claims)
	updateReq = withURLParams(updateReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	updateResp := httptest.NewRecorder()
	h.UpdateTenantPluginSettings(updateResp, updateReq)
	require.Equal(t, http.StatusOK, updateResp.Code, updateResp.Body.String())
	require.JSONEq(t, `{"account":"2000"}`, string(repo.tenantPlugins[pluginTenantKey(tenantID, pluginID)].Settings))

	routeMissingReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/missing", nil, claims)
	routeMissingReq = withURLParams(routeMissingReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "missing"})
	routeMissingResp := httptest.NewRecorder()
	h.InvokeTenantPluginRoute(routeMissingResp, routeMissingReq)
	require.Equal(t, http.StatusNotFound, routeMissingResp.Code, routeMissingResp.Body.String())

	repo.plugins[pluginID].Manifest = json.RawMessage(`{
		"name":"runtime-plugin",
		"display_name":"Runtime Plugin",
		"version":"1.0.0",
		"backend":{
			"runtime":"package",
			"routes":[{"method":"GET","path":"/status","handler":"/routes/status"}]
		}
	}`)
	runtimeUnavailableReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status", nil, claims)
	runtimeUnavailableReq = withURLParams(runtimeUnavailableReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String(), "*": "status"})
	runtimeUnavailableResp := httptest.NewRecorder()
	h.InvokeTenantPluginRoute(runtimeUnavailableResp, runtimeUnavailableReq)
	require.Equal(t, http.StatusBadGateway, runtimeUnavailableResp.Code, runtimeUnavailableResp.Body.String())

	disableReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/disable", nil, claims)
	disableReq = withURLParams(disableReq, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	disableResp := httptest.NewRecorder()
	h.DisableTenantPlugin(disableResp, disableReq)
	require.Equal(t, http.StatusOK, disableResp.Code, disableResp.Body.String())
}
