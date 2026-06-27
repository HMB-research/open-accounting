package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryNilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	registryID := uuid.New()
	repositoryPlugin := &Plugin{
		ID:                 pluginID,
		Name:               "demo-plugin",
		DisplayName:        "Demo Plugin",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/example/demo-plugin",
		RepositoryType:     RepoGitHub,
		State:              StateInstalled,
		GrantedPermissions: []string{"read:invoices"},
		Manifest:           json.RawMessage(`{"name":"demo-plugin"}`),
	}
	manifest := &Manifest{
		Name:        "demo-plugin",
		DisplayName: "Demo Plugin",
		Version:     "1.0.0",
	}

	repositories := []struct {
		name string
		repo *GORMRepository
	}{
		{name: "nil receiver"},
		{name: "nil gorm database", repo: NewGORMRepository(nil)},
	}

	tests := []struct {
		name string
		run  func(t *testing.T, repo *GORMRepository) error
	}{
		{
			name: "dbWithContext",
			run: func(t *testing.T, repo *GORMRepository) error {
				db, err := repo.dbWithContext(ctx)
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "ListRegistries",
			run: func(t *testing.T, repo *GORMRepository) error {
				registries, err := repo.ListRegistries(ctx)
				assert.Nil(t, registries)
				return err
			},
		},
		{
			name: "GetRegistry",
			run: func(t *testing.T, repo *GORMRepository) error {
				registry, err := repo.GetRegistry(ctx, registryID)
				assert.Nil(t, registry)
				return err
			},
		},
		{
			name: "CreateRegistry",
			run: func(t *testing.T, repo *GORMRepository) error {
				registry, err := repo.CreateRegistry(ctx, "demo", "https://plugins.example.com", "Demo")
				assert.Nil(t, registry)
				return err
			},
		},
		{
			name: "DeleteRegistry",
			run: func(t *testing.T, repo *GORMRepository) error {
				rows, err := repo.DeleteRegistry(ctx, registryID)
				assert.Zero(t, rows)
				return err
			},
		},
		{
			name: "UpdateRegistryLastSynced",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdateRegistryLastSynced(ctx, registryID)
			},
		},
		{
			name: "ListPlugins",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugins, err := repo.ListPlugins(ctx)
				assert.Nil(t, plugins)
				return err
			},
		},
		{
			name: "GetPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugin, err := repo.GetPlugin(ctx, pluginID)
				assert.Nil(t, plugin)
				return err
			},
		},
		{
			name: "GetPluginByName",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugin, err := repo.GetPluginByName(ctx, "demo-plugin")
				assert.Nil(t, plugin)
				return err
			},
		},
		{
			name: "CreatePlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.CreatePlugin(ctx, repositoryPlugin)
			},
		},
		{
			name: "UpdatePlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdatePlugin(ctx, repositoryPlugin)
			},
		},
		{
			name: "DeletePlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				rows, err := repo.DeletePlugin(ctx, pluginID)
				assert.Zero(t, rows)
				return err
			},
		},
		{
			name: "ListTenantPlugins",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugins, err := repo.ListTenantPlugins(ctx, tenantID)
				assert.Nil(t, plugins)
				return err
			},
		},
		{
			name: "GetTenantPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugin, err := repo.GetTenantPlugin(ctx, tenantID, pluginID)
				assert.Nil(t, plugin)
				return err
			},
		},
		{
			name: "CreateTenantPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.CreateTenantPlugin(ctx, tenantID, pluginID, json.RawMessage(`{"enabled":true}`))
			},
		},
		{
			name: "EnableTenantPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.EnableTenantPlugin(ctx, tenantID, pluginID, json.RawMessage(`{"enabled":true}`))
			},
		},
		{
			name: "DisableTenantPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				rows, err := repo.DisableTenantPlugin(ctx, tenantID, pluginID)
				assert.Zero(t, rows)
				return err
			},
		},
		{
			name: "GetTenantPluginSettings",
			run: func(t *testing.T, repo *GORMRepository) error {
				settings, err := repo.GetTenantPluginSettings(ctx, tenantID, pluginID)
				assert.Nil(t, settings)
				return err
			},
		},
		{
			name: "UpdateTenantPluginSettings",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdateTenantPluginSettings(ctx, tenantID, pluginID, json.RawMessage(`{"enabled":false}`))
			},
		},
		{
			name: "DeleteTenantPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.DeleteTenantPlugin(ctx, tenantID, pluginID)
			},
		},
		{
			name: "IsPluginEnabledForTenant",
			run: func(t *testing.T, repo *GORMRepository) error {
				enabled, err := repo.IsPluginEnabledForTenant(ctx, tenantID, pluginID)
				assert.False(t, enabled)
				return err
			},
		},
		{
			name: "ListEnabledPlugins",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugins, err := repo.ListEnabledPlugins(ctx)
				assert.Nil(t, plugins)
				return err
			},
		},
		{
			name: "InsertPluginReturning",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugin, err := repo.InsertPluginReturning(ctx, manifest, "https://github.com/example/demo-plugin", RepoGitHub, json.RawMessage(`{"name":"demo-plugin"}`))
				assert.Nil(t, plugin)
				return err
			},
		},
		{
			name: "CountEnabledTenantsForPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				count, err := repo.CountEnabledTenantsForPlugin(ctx, pluginID)
				assert.Zero(t, count)
				return err
			},
		},
		{
			name: "UpdatePluginState",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.UpdatePluginState(ctx, pluginID, StateEnabled, []string{"read:invoices"})
			},
		},
		{
			name: "DisableAllTenantsForPlugin",
			run: func(t *testing.T, repo *GORMRepository) error {
				return repo.DisableAllTenantsForPlugin(ctx, pluginID)
			},
		},
		{
			name: "GetTenantPluginsWithAll",
			run: func(t *testing.T, repo *GORMRepository) error {
				plugins, err := repo.GetTenantPluginsWithAll(ctx, tenantID)
				assert.Nil(t, plugins)
				return err
			},
		},
	}

	for _, repository := range repositories {
		t.Run(repository.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := tt.run(t, repository.repo)
					require.Error(t, err)
					assert.Contains(t, err.Error(), "plugin repository database is not configured")
				})
			}
		})
	}
}
