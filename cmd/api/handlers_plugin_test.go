package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type pluginHandlerRepository struct {
	registries    map[uuid.UUID]*plugin.Registry
	plugins       map[uuid.UUID]*plugin.Plugin
	tenantPlugins map[string]*plugin.TenantPlugin

	listRegistriesErr error
	listPluginsErr    error
}

func newPluginHandlerRepository() *pluginHandlerRepository {
	return &pluginHandlerRepository{
		registries:    make(map[uuid.UUID]*plugin.Registry),
		plugins:       make(map[uuid.UUID]*plugin.Plugin),
		tenantPlugins: make(map[string]*plugin.TenantPlugin),
	}
}

func (r *pluginHandlerRepository) ListRegistries(ctx context.Context) ([]plugin.Registry, error) {
	if r.listRegistriesErr != nil {
		return nil, r.listRegistriesErr
	}
	result := make([]plugin.Registry, 0, len(r.registries))
	for _, registry := range r.registries {
		result = append(result, *registry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (r *pluginHandlerRepository) GetRegistry(ctx context.Context, id uuid.UUID) (*plugin.Registry, error) {
	registry, ok := r.registries[id]
	if !ok {
		return nil, fmt.Errorf("registry not found")
	}
	copyRegistry := *registry
	return &copyRegistry, nil
}

func (r *pluginHandlerRepository) CreateRegistry(ctx context.Context, name, url, description string) (*plugin.Registry, error) {
	now := time.Now()
	registry := &plugin.Registry{
		ID:          uuid.New(),
		Name:        name,
		URL:         url,
		Description: description,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.registries[registry.ID] = registry
	return registry, nil
}

func (r *pluginHandlerRepository) DeleteRegistry(ctx context.Context, id uuid.UUID) (int64, error) {
	registry, ok := r.registries[id]
	if !ok || registry.IsOfficial {
		return 0, nil
	}
	delete(r.registries, id)
	return 1, nil
}

func (r *pluginHandlerRepository) UpdateRegistryLastSynced(ctx context.Context, id uuid.UUID) error {
	registry, ok := r.registries[id]
	if !ok {
		return fmt.Errorf("registry not found")
	}
	now := time.Now()
	registry.LastSyncedAt = &now
	registry.UpdatedAt = now
	return nil
}

func (r *pluginHandlerRepository) ListPlugins(ctx context.Context) ([]plugin.Plugin, error) {
	if r.listPluginsErr != nil {
		return nil, r.listPluginsErr
	}
	result := make([]plugin.Plugin, 0, len(r.plugins))
	for _, installedPlugin := range r.plugins {
		result = append(result, *installedPlugin)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (r *pluginHandlerRepository) GetPlugin(ctx context.Context, id uuid.UUID) (*plugin.Plugin, error) {
	installedPlugin, ok := r.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin not found")
	}
	copyPlugin := *installedPlugin
	return &copyPlugin, nil
}

func (r *pluginHandlerRepository) GetPluginByName(ctx context.Context, name string) (*plugin.Plugin, error) {
	for _, installedPlugin := range r.plugins {
		if installedPlugin.Name == name {
			copyPlugin := *installedPlugin
			return &copyPlugin, nil
		}
	}
	return nil, fmt.Errorf("plugin not found")
}

func (r *pluginHandlerRepository) CreatePlugin(ctx context.Context, installedPlugin *plugin.Plugin) error {
	r.plugins[installedPlugin.ID] = installedPlugin
	return nil
}

func (r *pluginHandlerRepository) UpdatePlugin(ctx context.Context, installedPlugin *plugin.Plugin) error {
	if _, ok := r.plugins[installedPlugin.ID]; !ok {
		return fmt.Errorf("plugin not found")
	}
	r.plugins[installedPlugin.ID] = installedPlugin
	return nil
}

func (r *pluginHandlerRepository) DeletePlugin(ctx context.Context, id uuid.UUID) (int64, error) {
	if _, ok := r.plugins[id]; !ok {
		return 0, nil
	}
	delete(r.plugins, id)
	return 1, nil
}

func (r *pluginHandlerRepository) ListTenantPlugins(ctx context.Context, tenantID uuid.UUID) ([]plugin.TenantPlugin, error) {
	result := make([]plugin.TenantPlugin, 0)
	for _, tenantPlugin := range r.tenantPlugins {
		if tenantPlugin.TenantID != tenantID {
			continue
		}
		copyTenantPlugin := *tenantPlugin
		if installedPlugin, ok := r.plugins[tenantPlugin.PluginID]; ok {
			copyPlugin := *installedPlugin
			copyTenantPlugin.Plugin = &copyPlugin
		}
		result = append(result, copyTenantPlugin)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].PluginID.String() < result[j].PluginID.String()
	})
	return result, nil
}

func (r *pluginHandlerRepository) GetTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (*plugin.TenantPlugin, error) {
	tenantPlugin, ok := r.tenantPlugins[pluginTenantKey(tenantID, pluginID)]
	if !ok {
		return nil, fmt.Errorf("tenant plugin not found")
	}
	copyTenantPlugin := *tenantPlugin
	if installedPlugin, ok := r.plugins[tenantPlugin.PluginID]; ok {
		copyPlugin := *installedPlugin
		copyTenantPlugin.Plugin = &copyPlugin
	}
	return &copyTenantPlugin, nil
}

func (r *pluginHandlerRepository) CreateTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	now := time.Now()
	r.tenantPlugins[pluginTenantKey(tenantID, pluginID)] = &plugin.TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return nil
}

func (r *pluginHandlerRepository) EnableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	key := pluginTenantKey(tenantID, pluginID)
	now := time.Now()
	if tenantPlugin, ok := r.tenantPlugins[key]; ok {
		tenantPlugin.IsEnabled = true
		tenantPlugin.Settings = settings
		tenantPlugin.EnabledAt = &now
		tenantPlugin.UpdatedAt = now
		return nil
	}
	r.tenantPlugins[key] = &plugin.TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return nil
}

func (r *pluginHandlerRepository) DisableTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) (int64, error) {
	tenantPlugin, ok := r.tenantPlugins[pluginTenantKey(tenantID, pluginID)]
	if !ok {
		return 0, nil
	}
	tenantPlugin.IsEnabled = false
	tenantPlugin.UpdatedAt = time.Now()
	return 1, nil
}

func (r *pluginHandlerRepository) GetTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID) (json.RawMessage, error) {
	if tenantPlugin, ok := r.tenantPlugins[pluginTenantKey(tenantID, pluginID)]; ok {
		return tenantPlugin.Settings, nil
	}
	return json.RawMessage("{}"), nil
}

func (r *pluginHandlerRepository) UpdateTenantPluginSettings(ctx context.Context, tenantID, pluginID uuid.UUID, settings json.RawMessage) error {
	tenantPlugin, ok := r.tenantPlugins[pluginTenantKey(tenantID, pluginID)]
	if !ok {
		return fmt.Errorf("tenant plugin not found")
	}
	tenantPlugin.Settings = settings
	tenantPlugin.UpdatedAt = time.Now()
	return nil
}

func (r *pluginHandlerRepository) DeleteTenantPlugin(ctx context.Context, tenantID, pluginID uuid.UUID) error {
	delete(r.tenantPlugins, pluginTenantKey(tenantID, pluginID))
	return nil
}

func (r *pluginHandlerRepository) IsPluginEnabledForTenant(ctx context.Context, tenantID, pluginID uuid.UUID) (bool, error) {
	tenantPlugin, ok := r.tenantPlugins[pluginTenantKey(tenantID, pluginID)]
	return ok && tenantPlugin.IsEnabled, nil
}

func (r *pluginHandlerRepository) ListEnabledPlugins(ctx context.Context) ([]plugin.Plugin, error) {
	result := make([]plugin.Plugin, 0)
	for _, installedPlugin := range r.plugins {
		if installedPlugin.State == plugin.StateEnabled {
			result = append(result, *installedPlugin)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (r *pluginHandlerRepository) InsertPluginReturning(
	ctx context.Context,
	manifest *plugin.Manifest,
	repoURL string,
	repoType plugin.RepositoryType,
	manifestJSON []byte,
) (*plugin.Plugin, error) {
	now := time.Now()
	installedPlugin := &plugin.Plugin{
		ID:             uuid.New(),
		Name:           manifest.Name,
		DisplayName:    manifest.DisplayName,
		Description:    manifest.Description,
		Version:        manifest.Version,
		RepositoryURL:  repoURL,
		RepositoryType: repoType,
		Author:         manifest.Author,
		License:        manifest.License,
		HomepageURL:    manifest.Homepage,
		State:          plugin.StateInstalled,
		Manifest:       json.RawMessage(manifestJSON),
		InstalledAt:    now,
		UpdatedAt:      now,
	}
	r.plugins[installedPlugin.ID] = installedPlugin
	return installedPlugin, nil
}

func (r *pluginHandlerRepository) CountEnabledTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) (int, error) {
	count := 0
	for _, tenantPlugin := range r.tenantPlugins {
		if tenantPlugin.PluginID == pluginID && tenantPlugin.IsEnabled {
			count++
		}
	}
	return count, nil
}

func (r *pluginHandlerRepository) UpdatePluginState(
	ctx context.Context,
	pluginID uuid.UUID,
	state plugin.PluginState,
	permissions []string,
) error {
	installedPlugin, ok := r.plugins[pluginID]
	if !ok {
		return fmt.Errorf("plugin not found")
	}
	installedPlugin.State = state
	installedPlugin.GrantedPermissions = permissions
	installedPlugin.UpdatedAt = time.Now()
	return nil
}

func (r *pluginHandlerRepository) DisableAllTenantsForPlugin(ctx context.Context, pluginID uuid.UUID) error {
	for _, tenantPlugin := range r.tenantPlugins {
		if tenantPlugin.PluginID == pluginID {
			tenantPlugin.IsEnabled = false
			tenantPlugin.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (r *pluginHandlerRepository) GetTenantPluginsWithAll(ctx context.Context, tenantID uuid.UUID) ([]plugin.TenantPlugin, error) {
	result := make([]plugin.TenantPlugin, 0)
	for _, installedPlugin := range r.plugins {
		if installedPlugin.State != plugin.StateEnabled {
			continue
		}
		copyPlugin := *installedPlugin
		tenantPlugin := plugin.TenantPlugin{
			TenantID:  tenantID,
			PluginID:  installedPlugin.ID,
			IsEnabled: false,
			Plugin:    &copyPlugin,
		}
		if existing, ok := r.tenantPlugins[pluginTenantKey(tenantID, installedPlugin.ID)]; ok {
			tenantPlugin.ID = existing.ID
			tenantPlugin.IsEnabled = existing.IsEnabled
			tenantPlugin.Settings = existing.Settings
			tenantPlugin.EnabledAt = existing.EnabledAt
			tenantPlugin.CreatedAt = existing.CreatedAt
			tenantPlugin.UpdatedAt = existing.UpdatedAt
		}
		result = append(result, tenantPlugin)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Plugin.Name < result[j].Plugin.Name
	})
	return result, nil
}

func pluginTenantKey(tenantID, pluginID uuid.UUID) string {
	return tenantID.String() + ":" + pluginID.String()
}

func setupPluginTestHandlers(t *testing.T) (*Handlers, *pluginHandlerRepository) {
	t.Helper()
	repo := newPluginHandlerRepository()
	return &Handlers{
		pluginService: plugin.NewServiceWithRepository(repo, nil, t.TempDir()),
	}, repo
}

func TestPluginListHandlers(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	registryID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	repo.registries[registryID] = &plugin.Registry{
		ID:          registryID,
		Name:        "Official marketplace",
		URL:         "https://github.com/HMB-research/open-accounting-plugins",
		Description: "Curated accounting plugins",
		IsOfficial:  true,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "lhv-bank-import",
		DisplayName:        "LHV Bank Import",
		Description:        "Imports LHV account statements",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/open-accounting-lhv",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"banking:write"},
		Manifest:           json.RawMessage(`{"name":"lhv-bank-import","version":"1.0.0"}`),
		InstalledAt:        now,
		UpdatedAt:          now,
	}

	registryReq := httptest.NewRequest(http.MethodGet, "/admin/plugin-registries", nil)
	registryResp := httptest.NewRecorder()
	h.ListPluginRegistries(registryResp, registryReq)

	require.Equal(t, http.StatusOK, registryResp.Code)
	var registries []plugin.Registry
	require.NoError(t, json.NewDecoder(registryResp.Body).Decode(&registries))
	require.Len(t, registries, 1)
	assert.Equal(t, registryID, registries[0].ID)
	assert.Equal(t, "Official marketplace", registries[0].Name)
	assert.True(t, registries[0].IsOfficial)

	pluginsReq := httptest.NewRequest(http.MethodGet, "/admin/plugins", nil)
	pluginsResp := httptest.NewRecorder()
	h.ListPlugins(pluginsResp, pluginsReq)

	require.Equal(t, http.StatusOK, pluginsResp.Code)
	var installedPlugins []plugin.Plugin
	require.NoError(t, json.NewDecoder(pluginsResp.Body).Decode(&installedPlugins))
	require.Len(t, installedPlugins, 1)
	assert.Equal(t, pluginID, installedPlugins[0].ID)
	assert.Equal(t, "lhv-bank-import", installedPlugins[0].Name)
	assert.Equal(t, plugin.StateEnabled, installedPlugins[0].State)
	assert.Equal(t, []string{"banking:write"}, installedPlugins[0].GrantedPermissions)
}

func TestPluginListHandlersRepositoryErrors(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)

	repo.listRegistriesErr = fmt.Errorf("database unavailable")
	registryReq := httptest.NewRequest(http.MethodGet, "/admin/plugin-registries", nil)
	registryResp := httptest.NewRecorder()
	h.ListPluginRegistries(registryResp, registryReq)

	require.Equal(t, http.StatusInternalServerError, registryResp.Code)
	assert.JSONEq(t, `{"error":"Failed to list registries"}`, registryResp.Body.String())

	repo.listPluginsErr = fmt.Errorf("database unavailable")
	pluginsReq := httptest.NewRequest(http.MethodGet, "/admin/plugins", nil)
	pluginsResp := httptest.NewRecorder()
	h.ListPlugins(pluginsResp, pluginsReq)

	require.Equal(t, http.StatusInternalServerError, pluginsResp.Code)
	assert.JSONEq(t, `{"error":"Failed to list plugins"}`, pluginsResp.Body.String())
}

func TestPluginEnableDisableHandlersReturnUpdatedPlugin(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:             pluginID,
		Name:           "demo-bank-import",
		DisplayName:    "Demo Bank Import",
		Description:    "Demo plugin for tenant workflow coverage",
		Version:        "1.0.0",
		RepositoryURL:  "https://github.com/HMB-research/open-accounting-demo-bank-import",
		RepositoryType: plugin.RepoGitHub,
		State:          plugin.StateInstalled,
		Manifest: json.RawMessage(`{
			"name":"demo-bank-import",
			"display_name":"Demo Bank Import",
			"version":"1.0.0",
			"permissions":["banking:read"]
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	enableReq := withURLParams(
		httptest.NewRequest(
			http.MethodPost,
			"/admin/plugins/"+pluginID.String()+"/enable",
			strings.NewReader(`{"granted_permissions":["banking:read"]}`),
		),
		map[string]string{"id": pluginID.String()},
	)
	enableResp := httptest.NewRecorder()
	h.EnablePlugin(enableResp, enableReq)

	require.Equal(t, http.StatusOK, enableResp.Code)
	var enabled plugin.Plugin
	require.NoError(t, json.NewDecoder(enableResp.Body).Decode(&enabled))
	assert.Equal(t, pluginID, enabled.ID)
	assert.Equal(t, plugin.StateEnabled, enabled.State)
	assert.Equal(t, []string{"banking:read"}, enabled.GrantedPermissions)

	disableReq := withURLParams(
		httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/disable", nil),
		map[string]string{"id": pluginID.String()},
	)
	disableResp := httptest.NewRecorder()
	h.DisablePlugin(disableResp, disableReq)

	require.Equal(t, http.StatusOK, disableResp.Code)
	var disabled plugin.Plugin
	require.NoError(t, json.NewDecoder(disableResp.Body).Decode(&disabled))
	assert.Equal(t, pluginID, disabled.ID)
	assert.Equal(t, plugin.StateDisabled, disabled.State)
	assert.Empty(t, disabled.GrantedPermissions)
}

func TestTenantPluginEnableHandlerReturnsUpdatedTenantPlugin(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := "user-1"

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Demo Tenant", "demo-tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{{
		TenantID: tenantID.String(),
		UserID:   userID,
		Role:     tenant.RoleOwner,
		IsActive: true,
	}}
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:             pluginID,
		Name:           "demo-bank-import",
		DisplayName:    "Demo Bank Import",
		Description:    "Demo plugin for tenant workflow coverage",
		Version:        "1.0.0",
		RepositoryURL:  "https://github.com/HMB-research/open-accounting-demo-bank-import",
		RepositoryType: plugin.RepoGitHub,
		State:          plugin.StateEnabled,
		GrantedPermissions: []string{
			"banking:read",
		},
		Manifest: json.RawMessage(`{
			"name":"demo-bank-import",
			"display_name":"Demo Bank Import",
			"version":"1.0.0",
			"permissions":["banking:read"]
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	req := makeAuthenticatedRequest(
		http.MethodPost,
		"/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/enable",
		map[string]json.RawMessage{"settings": json.RawMessage(`{"account":"1000"}`)},
		&auth.Claims{UserID: userID, TenantID: tenantID.String(), Role: tenant.RoleOwner},
	)
	req = withURLParams(req, map[string]string{"tenantID": tenantID.String(), "pluginID": pluginID.String()})
	resp := httptest.NewRecorder()
	h.EnableTenantPlugin(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var enabled plugin.TenantPlugin
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&enabled))
	assert.Equal(t, tenantID, enabled.TenantID)
	assert.Equal(t, pluginID, enabled.PluginID)
	assert.True(t, enabled.IsEnabled)
	assert.JSONEq(t, `{"account":"1000"}`, string(enabled.Settings))
	require.NotNil(t, enabled.Plugin)
	assert.Equal(t, "demo-bank-import", enabled.Plugin.Name)
}

func TestInvokeTenantPluginRouteProxiesDeclaredRuntimeRoute(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)

	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	userID := "user-1"
	var runtimeTenantHeader string
	var runtimePluginHeader string
	var runtimeBody map[string]string

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/routes/status", r.URL.Path)
		runtimeTenantHeader = r.Header.Get("X-Open-Accounting-Tenant-ID")
		runtimePluginHeader = r.Header.Get("X-Open-Accounting-Plugin-ID")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&runtimeBody))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"handled"}`))
	}))
	defer runtimeServer.Close()

	tenantRepo := newMockTenantRepository()
	tenantRepo.addTestTenant(tenantID.String(), "Demo Tenant", "demo-tenant")
	tenantRepo.tenantUsers[tenantID.String()] = []tenant.TenantUser{{
		TenantID: tenantID.String(),
		UserID:   userID,
		Role:     tenant.RoleAdmin,
		IsActive: true,
	}}
	h.tenantService = tenant.NewServiceWithRepository(tenantRepo)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:                 pluginID,
		Name:               "runtime-plugin",
		DisplayName:        "Runtime Plugin",
		Description:        "Demo runtime plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/HMB-research/open-accounting-runtime-plugin",
		RepositoryType:     plugin.RepoGitHub,
		State:              plugin.StateEnabled,
		GrantedPermissions: []string{"routes:register"},
		Manifest: json.RawMessage(fmt.Sprintf(`{
			"name":"runtime-plugin",
			"display_name":"Runtime Plugin",
			"version":"1.0.0",
			"permissions":["routes:register"],
			"backend":{
				"package":"./backend",
				"entry":"main",
				"runtime":"http",
				"base_url":%q,
				"routes":[{"method":"POST","path":"/status","handler":"/routes/status"}]
			}
		}`, runtimeServer.URL)),
		InstalledAt: now,
		UpdatedAt:   now,
	}
	repo.tenantPlugins[pluginTenantKey(tenantID, pluginID)] = &plugin.TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	req := makeAuthenticatedRequest(
		http.MethodPost,
		"/tenants/"+tenantID.String()+"/plugins/"+pluginID.String()+"/runtime/status",
		map[string]string{"ping": "pong"},
		&auth.Claims{UserID: userID, TenantID: tenantID.String(), Role: tenant.RoleAdmin},
	)
	req = withURLParams(req, map[string]string{
		"tenantID": tenantID.String(),
		"pluginID": pluginID.String(),
		"*":        "status",
	})
	resp := httptest.NewRecorder()
	h.InvokeTenantPluginRoute(resp, req)

	require.Equal(t, http.StatusAccepted, resp.Code)
	assert.JSONEq(t, `{"status":"handled"}`, resp.Body.String())
	assert.Equal(t, tenantID.String(), runtimeTenantHeader)
	assert.Equal(t, pluginID.String(), runtimePluginHeader)
	assert.Equal(t, "pong", runtimeBody["ping"])
}

func TestPluginRuntimeStatusHandlers(t *testing.T) {
	h, repo := setupPluginTestHandlers(t)

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	pluginID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	repo.plugins[pluginID] = &plugin.Plugin{
		ID:             pluginID,
		Name:           "runtime-plugin",
		DisplayName:    "Runtime Plugin",
		Version:        "1.0.0",
		RepositoryURL:  "https://github.com/HMB-research/open-accounting-runtime-plugin",
		RepositoryType: plugin.RepoGitHub,
		State:          plugin.StateEnabled,
		Manifest: json.RawMessage(`{
			"name":"runtime-plugin",
			"display_name":"Runtime Plugin",
			"version":"1.0.0",
			"backend":{
				"runtime":"http",
				"base_url":"http://127.0.0.1:39999",
				"routes":[{"method":"GET","path":"/status","handler":"/routes/status"}]
			}
		}`),
		InstalledAt: now,
		UpdatedAt:   now,
	}

	statusReq := withURLParams(
		httptest.NewRequest(http.MethodGet, "/admin/plugins/"+pluginID.String()+"/runtime", nil),
		map[string]string{"id": pluginID.String()},
	)
	statusResp := httptest.NewRecorder()
	h.GetPluginRuntimeStatus(statusResp, statusReq)

	require.Equal(t, http.StatusOK, statusResp.Code)
	var status plugin.PluginRuntimeStatus
	require.NoError(t, json.NewDecoder(statusResp.Body).Decode(&status))
	assert.Equal(t, pluginID, status.PluginID)
	assert.Equal(t, plugin.BackendRuntimeHTTP, status.Runtime)
	assert.Equal(t, plugin.RuntimeStateExternal, status.State)
	assert.Equal(t, "Runtime Plugin", status.DisplayName)

	restartReq := withURLParams(
		httptest.NewRequest(http.MethodPost, "/admin/plugins/"+pluginID.String()+"/runtime/restart", nil),
		map[string]string{"id": pluginID.String()},
	)
	restartResp := httptest.NewRecorder()
	h.RestartPluginRuntime(restartResp, restartReq)

	require.Equal(t, http.StatusBadRequest, restartResp.Code)
	assert.Contains(t, restartResp.Body.String(), "does not use a supervised package runtime")
}
