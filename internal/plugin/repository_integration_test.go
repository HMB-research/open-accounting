//go:build integration

package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
)

func TestPostgresRepository_PluginLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a plugin
	manifest := &Manifest{
		Name:        "test-plugin",
		DisplayName: "Test Plugin",
		Description: "A test plugin for integration testing",
		Version:     "1.0.0",
		Author:      "Test Author",
		License:     "MIT",
		Homepage:    "https://example.com",
	}
	manifestJSON, _ := json.Marshal(manifest)

	plugin, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/test/test-plugin", RepoGitHub, manifestJSON)
	if err != nil {
		t.Fatalf("InsertPluginReturning failed: %v", err)
	}

	if plugin.Name != manifest.Name {
		t.Errorf("expected name %s, got %s", manifest.Name, plugin.Name)
	}
	if plugin.State != StateInstalled {
		t.Errorf("expected state %s, got %s", StateInstalled, plugin.State)
	}

	// Get plugin by ID
	retrieved, err := repo.GetPlugin(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}
	if retrieved.DisplayName != manifest.DisplayName {
		t.Errorf("expected display name %s, got %s", manifest.DisplayName, retrieved.DisplayName)
	}

	// Get plugin by name
	byName, err := repo.GetPluginByName(ctx, manifest.Name)
	if err != nil {
		t.Fatalf("GetPluginByName failed: %v", err)
	}
	if byName.ID != plugin.ID {
		t.Errorf("expected ID %s, got %s", plugin.ID, byName.ID)
	}

	// Update plugin state
	err = repo.UpdatePluginState(ctx, plugin.ID, StateEnabled, []string{"read:invoices"})
	if err != nil {
		t.Fatalf("UpdatePluginState failed: %v", err)
	}

	// Verify state update
	updated, err := repo.GetPlugin(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("GetPlugin after update failed: %v", err)
	}
	if updated.State != StateEnabled {
		t.Errorf("expected state %s, got %s", StateEnabled, updated.State)
	}

	// List enabled plugins
	enabledPlugins, err := repo.ListEnabledPlugins(ctx)
	if err != nil {
		t.Fatalf("ListEnabledPlugins failed: %v", err)
	}

	found := false
	for _, p := range enabledPlugins {
		if p.ID == plugin.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("plugin not found in enabled plugins list")
	}

	// Delete plugin
	affected, err := repo.DeletePlugin(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("DeletePlugin failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}
}

func TestPostgresRepository_TenantPluginOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a plugin first
	manifest := &Manifest{
		Name:        "tenant-test-plugin",
		DisplayName: "Tenant Test Plugin",
		Description: "Plugin for tenant tests",
		Version:     "1.0.0",
		Author:      "Test Author",
		License:     "MIT",
	}
	manifestJSON, _ := json.Marshal(manifest)

	plugin, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/test/tenant-plugin", RepoGitHub, manifestJSON)
	if err != nil {
		t.Fatalf("InsertPluginReturning failed: %v", err)
	}

	// Enable for instance level first
	err = repo.UpdatePluginState(ctx, plugin.ID, StateEnabled, []string{"read:invoices"})
	if err != nil {
		t.Fatalf("UpdatePluginState failed: %v", err)
	}

	// Parse tenant ID
	tenantUUID, _ := uuid.Parse(tenant.ID)

	// Enable plugin for tenant
	settings := json.RawMessage(`{"key": "value"}`)
	err = repo.EnableTenantPlugin(ctx, tenantUUID, plugin.ID, settings)
	if err != nil {
		t.Fatalf("EnableTenantPlugin failed: %v", err)
	}

	// Check if enabled
	enabled, err := repo.IsPluginEnabledForTenant(ctx, tenantUUID, plugin.ID)
	if err != nil {
		t.Fatalf("IsPluginEnabledForTenant failed: %v", err)
	}
	if !enabled {
		t.Error("expected plugin to be enabled for tenant")
	}

	// Get settings
	retrievedSettings, err := repo.GetTenantPluginSettings(ctx, tenantUUID, plugin.ID)
	if err != nil {
		t.Fatalf("GetTenantPluginSettings failed: %v", err)
	}
	if string(retrievedSettings) != string(settings) {
		t.Errorf("expected settings %s, got %s", settings, retrievedSettings)
	}

	// Count enabled tenants
	count, err := repo.CountEnabledTenantsForPlugin(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("CountEnabledTenantsForPlugin failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 enabled tenant, got %d", count)
	}

	// Get tenant plugins with all
	tenantPlugins, err := repo.GetTenantPluginsWithAll(ctx, tenantUUID)
	if err != nil {
		t.Fatalf("GetTenantPluginsWithAll failed: %v", err)
	}

	found := false
	for _, tp := range tenantPlugins {
		if tp.Plugin.ID == plugin.ID {
			found = true
			if !tp.IsEnabled {
				t.Error("expected plugin to be enabled")
			}
			break
		}
	}
	if !found {
		t.Error("plugin not found in tenant plugins list")
	}

	// Disable for tenant
	affected, err := repo.DisableTenantPlugin(ctx, tenantUUID, plugin.ID)
	if err != nil {
		t.Fatalf("DisableTenantPlugin failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}

	// Cleanup
	_, _ = repo.DeletePlugin(ctx, plugin.ID)
}

func TestPostgresRepository_ListPlugins(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a couple of plugins
	for i := 1; i <= 2; i++ {
		manifest := &Manifest{
			Name:        "list-test-plugin-" + string(rune('0'+i)),
			DisplayName: "List Test Plugin " + string(rune('0'+i)),
			Description: "Plugin for list test",
			Version:     "1.0.0",
			Author:      "Test Author",
			License:     "MIT",
		}
		manifestJSON, _ := json.Marshal(manifest)

		_, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/test/list-plugin-"+string(rune('0'+i)), RepoGitHub, manifestJSON)
		if err != nil {
			t.Fatalf("InsertPluginReturning failed: %v", err)
		}
	}

	// List all plugins
	plugins, err := repo.ListPlugins(ctx)
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}

	if len(plugins) < 2 {
		t.Errorf("expected at least 2 plugins, got %d", len(plugins))
	}

	// Cleanup
	for _, p := range plugins {
		_, _ = repo.DeletePlugin(ctx, p.ID)
	}
}

func TestPostgresRepository_CreateAndUpdatePlugin(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create plugin via CreatePlugin (not InsertPluginReturning)
	pluginID := uuid.New()
	plugin := &Plugin{
		ID:               pluginID,
		Name:             "create-update-test-plugin",
		DisplayName:      "Create Update Test Plugin",
		Description:      "Plugin for create/update test",
		Version:          "1.0.0",
		RepositoryURL:    "https://github.com/test/create-update-plugin",
		RepositoryType:   RepoGitHub,
		Author:           "Test Author",
		License:          "MIT",
		HomepageURL:      "https://example.com",
		State:            StateInstalled,
		GrantedPermissions: []string{},
		Manifest:         json.RawMessage(`{"name":"create-update-test-plugin"}`),
	}

	err := repo.CreatePlugin(ctx, plugin)
	if err != nil {
		t.Fatalf("CreatePlugin failed: %v", err)
	}

	// Verify created
	retrieved, err := repo.GetPlugin(ctx, pluginID)
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}

	if retrieved.Name != plugin.Name {
		t.Errorf("expected name %s, got %s", plugin.Name, retrieved.Name)
	}

	// Update plugin
	plugin.State = StateEnabled
	plugin.GrantedPermissions = []string{"read:invoices", "write:contacts"}
	err = repo.UpdatePlugin(ctx, plugin)
	if err != nil {
		t.Fatalf("UpdatePlugin failed: %v", err)
	}

	// Verify update
	updated, err := repo.GetPlugin(ctx, pluginID)
	if err != nil {
		t.Fatalf("GetPlugin after update failed: %v", err)
	}

	if updated.State != StateEnabled {
		t.Errorf("expected state %s, got %s", StateEnabled, updated.State)
	}

	// Cleanup
	_, _ = repo.DeletePlugin(ctx, pluginID)
}

func TestPostgresRepository_TenantPluginCRUD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a plugin
	manifest := &Manifest{
		Name:        "tenant-crud-plugin",
		DisplayName: "Tenant CRUD Plugin",
		Description: "Plugin for tenant CRUD tests",
		Version:     "1.0.0",
		Author:      "Test Author",
		License:     "MIT",
	}
	manifestJSON, _ := json.Marshal(manifest)

	plugin, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/test/tenant-crud-plugin", RepoGitHub, manifestJSON)
	if err != nil {
		t.Fatalf("InsertPluginReturning failed: %v", err)
	}

	// Enable for instance level first
	err = repo.UpdatePluginState(ctx, plugin.ID, StateEnabled, []string{"read:invoices"})
	if err != nil {
		t.Fatalf("UpdatePluginState failed: %v", err)
	}

	tenantUUID, _ := uuid.Parse(tenant.ID)

	// Test CreateTenantPlugin
	settings := json.RawMessage(`{"feature_x": true}`)
	err = repo.CreateTenantPlugin(ctx, tenantUUID, plugin.ID, settings)
	if err != nil {
		t.Fatalf("CreateTenantPlugin failed: %v", err)
	}

	// Test GetTenantPlugin
	tp, err := repo.GetTenantPlugin(ctx, tenantUUID, plugin.ID)
	if err != nil {
		t.Fatalf("GetTenantPlugin failed: %v", err)
	}

	if tp.Plugin.Name != manifest.Name {
		t.Errorf("expected plugin name %s, got %s", manifest.Name, tp.Plugin.Name)
	}

	// Test ListTenantPlugins
	tenantPlugins, err := repo.ListTenantPlugins(ctx, tenantUUID)
	if err != nil {
		t.Fatalf("ListTenantPlugins failed: %v", err)
	}

	if len(tenantPlugins) != 1 {
		t.Errorf("expected 1 tenant plugin, got %d", len(tenantPlugins))
	}

	// Test UpdateTenantPluginSettings
	newSettings := json.RawMessage(`{"feature_x": false, "feature_y": true}`)
	err = repo.UpdateTenantPluginSettings(ctx, tenantUUID, plugin.ID, newSettings)
	if err != nil {
		t.Fatalf("UpdateTenantPluginSettings failed: %v", err)
	}

	// Verify settings update
	retrievedSettings, err := repo.GetTenantPluginSettings(ctx, tenantUUID, plugin.ID)
	if err != nil {
		t.Fatalf("GetTenantPluginSettings failed: %v", err)
	}
	if string(retrievedSettings) != string(newSettings) {
		t.Errorf("expected settings %s, got %s", newSettings, retrievedSettings)
	}

	// Test DeleteTenantPlugin
	err = repo.DeleteTenantPlugin(ctx, tenantUUID, plugin.ID)
	if err != nil {
		t.Fatalf("DeleteTenantPlugin failed: %v", err)
	}

	// Cleanup
	_, _ = repo.DeletePlugin(ctx, plugin.ID)
}

func TestPostgresRepository_DisableAllTenantsForPlugin(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a plugin
	manifest := &Manifest{
		Name:        "disable-all-plugin",
		DisplayName: "Disable All Plugin",
		Description: "Plugin for disable all test",
		Version:     "1.0.0",
		Author:      "Test Author",
		License:     "MIT",
	}
	manifestJSON, _ := json.Marshal(manifest)

	plugin, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/test/disable-all-plugin", RepoGitHub, manifestJSON)
	if err != nil {
		t.Fatalf("InsertPluginReturning failed: %v", err)
	}

	// Enable for instance level
	err = repo.UpdatePluginState(ctx, plugin.ID, StateEnabled, []string{})
	if err != nil {
		t.Fatalf("UpdatePluginState failed: %v", err)
	}

	tenantUUID, _ := uuid.Parse(tenant.ID)

	// Enable for tenant
	err = repo.EnableTenantPlugin(ctx, tenantUUID, plugin.ID, json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("EnableTenantPlugin failed: %v", err)
	}

	// Disable all tenants
	err = repo.DisableAllTenantsForPlugin(ctx, plugin.ID)
	if err != nil {
		t.Fatalf("DisableAllTenantsForPlugin failed: %v", err)
	}

	// Verify disabled - use GetTenantPlugin to check is_enabled field
	tp, err := repo.GetTenantPlugin(ctx, tenantUUID, plugin.ID)
	if err != nil {
		t.Fatalf("GetTenantPlugin failed: %v", err)
	}
	if tp.IsEnabled {
		t.Error("expected plugin to be disabled for tenant (is_enabled should be false)")
	}

	// Cleanup
	_, _ = repo.DeletePlugin(ctx, plugin.ID)
}

func TestPostgresRepository_RegistryOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create registry
	registry, err := repo.CreateRegistry(ctx, "Test Registry", "https://github.com/test/registry", "A test registry")
	if err != nil {
		t.Fatalf("CreateRegistry failed: %v", err)
	}

	if registry.Name != "Test Registry" {
		t.Errorf("expected name 'Test Registry', got '%s'", registry.Name)
	}
	if registry.IsOfficial {
		t.Error("new registry should not be official")
	}

	// Get registry
	retrieved, err := repo.GetRegistry(ctx, registry.ID)
	if err != nil {
		t.Fatalf("GetRegistry failed: %v", err)
	}
	if retrieved.URL != registry.URL {
		t.Errorf("expected URL %s, got %s", registry.URL, retrieved.URL)
	}

	// Update last synced
	err = repo.UpdateRegistryLastSynced(ctx, registry.ID)
	if err != nil {
		t.Fatalf("UpdateRegistryLastSynced failed: %v", err)
	}

	// Verify last synced is updated
	updated, err := repo.GetRegistry(ctx, registry.ID)
	if err != nil {
		t.Fatalf("GetRegistry after sync failed: %v", err)
	}
	if updated.LastSyncedAt == nil {
		t.Error("expected last synced to be set")
	}

	// List registries
	registries, err := repo.ListRegistries(ctx)
	if err != nil {
		t.Fatalf("ListRegistries failed: %v", err)
	}

	found := false
	for _, r := range registries {
		if r.ID == registry.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("registry not found in list")
	}

	// Delete registry
	affected, err := repo.DeleteRegistry(ctx, registry.ID)
	if err != nil {
		t.Fatalf("DeleteRegistry failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}
}
