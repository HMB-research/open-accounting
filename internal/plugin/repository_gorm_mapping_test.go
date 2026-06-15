package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelToRegistry(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	updatedAt := createdAt.Add(2 * time.Hour)
	lastSyncedAt := createdAt.Add(time.Hour)
	registryID := uuid.New()

	model := &models.PluginRegistry{
		ID:           registryID,
		Name:         "official",
		URL:          "https://plugins.example.com",
		Description:  "Official plugin registry",
		IsOfficial:   true,
		IsActive:     true,
		LastSyncedAt: &lastSyncedAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}

	registry := modelToRegistry(model)

	assert.Equal(t, registryID, registry.ID)
	assert.Equal(t, "official", registry.Name)
	assert.Equal(t, "https://plugins.example.com", registry.URL)
	assert.Equal(t, "Official plugin registry", registry.Description)
	assert.True(t, registry.IsOfficial)
	assert.True(t, registry.IsActive)
	require.NotNil(t, registry.LastSyncedAt)
	assert.Equal(t, lastSyncedAt, *registry.LastSyncedAt)
	assert.Equal(t, createdAt, registry.CreatedAt)
	assert.Equal(t, updatedAt, registry.UpdatedAt)
}

func TestPluginModelMappings(t *testing.T) {
	installedAt := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	updatedAt := installedAt.Add(3 * time.Hour)
	pluginID := uuid.New()
	manifest := json.RawMessage(`{"name":"payroll-sync","version":"1.2.3"}`)

	model := &models.Plugin{
		ID:                 pluginID,
		Name:               "payroll-sync",
		DisplayName:        "Payroll Sync",
		Description:        "Imports payroll activity",
		Version:            "1.2.3",
		RepositoryURL:      "https://github.com/example/payroll-sync",
		RepositoryType:     models.RepoGitHub,
		Author:             "Open Accounting",
		License:            "MIT",
		HomepageURL:        "https://example.com/payroll-sync",
		State:              models.PluginStateEnabled,
		GrantedPermissions: pq.StringArray{"read:payroll", "write:journal"},
		Manifest:           manifest,
		InstalledAt:        installedAt,
		UpdatedAt:          updatedAt,
	}

	domainPlugin := modelToPlugin(model)

	assert.Equal(t, pluginID, domainPlugin.ID)
	assert.Equal(t, "payroll-sync", domainPlugin.Name)
	assert.Equal(t, "Payroll Sync", domainPlugin.DisplayName)
	assert.Equal(t, "Imports payroll activity", domainPlugin.Description)
	assert.Equal(t, "1.2.3", domainPlugin.Version)
	assert.Equal(t, "https://github.com/example/payroll-sync", domainPlugin.RepositoryURL)
	assert.Equal(t, RepoGitHub, domainPlugin.RepositoryType)
	assert.Equal(t, "Open Accounting", domainPlugin.Author)
	assert.Equal(t, "MIT", domainPlugin.License)
	assert.Equal(t, "https://example.com/payroll-sync", domainPlugin.HomepageURL)
	assert.Equal(t, StateEnabled, domainPlugin.State)
	assert.Equal(t, []string{"read:payroll", "write:journal"}, domainPlugin.GrantedPermissions)
	assert.Equal(t, manifest, domainPlugin.Manifest)
	assert.Equal(t, installedAt, domainPlugin.InstalledAt)
	assert.Equal(t, updatedAt, domainPlugin.UpdatedAt)

	roundTripModel := pluginToModel(&domainPlugin)

	assert.Equal(t, pluginID, roundTripModel.ID)
	assert.Equal(t, "payroll-sync", roundTripModel.Name)
	assert.Equal(t, "Payroll Sync", roundTripModel.DisplayName)
	assert.Equal(t, "Imports payroll activity", roundTripModel.Description)
	assert.Equal(t, "1.2.3", roundTripModel.Version)
	assert.Equal(t, "https://github.com/example/payroll-sync", roundTripModel.RepositoryURL)
	assert.Equal(t, models.RepoGitHub, roundTripModel.RepositoryType)
	assert.Equal(t, "Open Accounting", roundTripModel.Author)
	assert.Equal(t, "MIT", roundTripModel.License)
	assert.Equal(t, "https://example.com/payroll-sync", roundTripModel.HomepageURL)
	assert.Equal(t, models.PluginStateEnabled, roundTripModel.State)
	assert.Equal(t, []string{"read:payroll", "write:journal"}, []string(roundTripModel.GrantedPermissions))
	assert.Equal(t, manifest, roundTripModel.Manifest)
	assert.Equal(t, installedAt, roundTripModel.InstalledAt)
	assert.Equal(t, updatedAt, roundTripModel.UpdatedAt)
}

func TestModelToTenantPluginWithPlugin(t *testing.T) {
	createdAt := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	updatedAt := createdAt.Add(4 * time.Hour)
	enabledAt := createdAt.Add(time.Hour)
	pluginID := uuid.New()
	tenantID := uuid.New()
	tenantPluginID := uuid.New()
	settings := json.RawMessage(`{"payrollProvider":"csv"}`)
	manifest := json.RawMessage(`{"name":"payroll-sync"}`)

	model := &models.TenantPlugin{
		ID:        tenantPluginID,
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &enabledAt,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		Plugin: &models.Plugin{
			ID:                 pluginID,
			Name:               "payroll-sync",
			DisplayName:        "Payroll Sync",
			Description:        "Imports payroll activity",
			Version:            "1.2.3",
			RepositoryURL:      "https://github.com/example/payroll-sync",
			RepositoryType:     models.RepoGitLab,
			Author:             "Open Accounting",
			License:            "Apache-2.0",
			HomepageURL:        "https://example.com/payroll-sync",
			State:              models.PluginStateInstalled,
			GrantedPermissions: pq.StringArray{"read:payroll"},
			Manifest:           manifest,
			InstalledAt:        createdAt.Add(-24 * time.Hour),
			UpdatedAt:          updatedAt.Add(-24 * time.Hour),
		},
	}

	tenantPlugin := modelToTenantPlugin(model)

	assert.Equal(t, tenantPluginID, tenantPlugin.ID)
	assert.Equal(t, tenantID, tenantPlugin.TenantID)
	assert.Equal(t, pluginID, tenantPlugin.PluginID)
	assert.True(t, tenantPlugin.IsEnabled)
	assert.Equal(t, settings, tenantPlugin.Settings)
	require.NotNil(t, tenantPlugin.EnabledAt)
	assert.Equal(t, enabledAt, *tenantPlugin.EnabledAt)
	assert.Equal(t, createdAt, tenantPlugin.CreatedAt)
	assert.Equal(t, updatedAt, tenantPlugin.UpdatedAt)

	require.NotNil(t, tenantPlugin.Plugin)
	assert.Equal(t, pluginID, tenantPlugin.Plugin.ID)
	assert.Equal(t, "payroll-sync", tenantPlugin.Plugin.Name)
	assert.Equal(t, "Payroll Sync", tenantPlugin.Plugin.DisplayName)
	assert.Equal(t, RepoGitLab, tenantPlugin.Plugin.RepositoryType)
	assert.Equal(t, StateInstalled, tenantPlugin.Plugin.State)
	assert.Equal(t, []string{"read:payroll"}, tenantPlugin.Plugin.GrantedPermissions)
	assert.Equal(t, manifest, tenantPlugin.Plugin.Manifest)
}

func TestModelToTenantPluginWithoutPlugin(t *testing.T) {
	model := &models.TenantPlugin{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		PluginID:  uuid.New(),
		IsEnabled: false,
		Settings:  json.RawMessage(`{}`),
		CreatedAt: time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 5, 7, 7, 8, 0, time.UTC),
	}

	tenantPlugin := modelToTenantPlugin(model)

	assert.Equal(t, model.ID, tenantPlugin.ID)
	assert.Equal(t, model.TenantID, tenantPlugin.TenantID)
	assert.Equal(t, model.PluginID, tenantPlugin.PluginID)
	assert.False(t, tenantPlugin.IsEnabled)
	assert.Nil(t, tenantPlugin.EnabledAt)
	assert.Nil(t, tenantPlugin.Plugin)
}
