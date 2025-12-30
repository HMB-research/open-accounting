package plugin

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestPluginStateConstants(t *testing.T) {
	tests := []struct {
		state    PluginState
		expected string
	}{
		{StateInstalled, "installed"},
		{StateEnabled, "enabled"},
		{StateDisabled, "disabled"},
		{StateFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.state))
		})
	}
}

func TestRepositoryTypeConstants(t *testing.T) {
	tests := []struct {
		repoType RepositoryType
		expected string
	}{
		{RepoGitHub, "github"},
		{RepoGitLab, "gitlab"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.repoType))
		})
	}
}

func TestRegistry_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	lastSynced := now.Add(-24 * time.Hour)

	reg := Registry{
		ID:           uuid.New(),
		Name:         "official",
		URL:          "https://plugins.openaccounting.io",
		Description:  "Official plugin registry",
		IsOfficial:   true,
		IsActive:     true,
		LastSyncedAt: &lastSynced,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(reg)
	assert.NoError(t, err)

	var parsed Registry
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, reg.ID, parsed.ID)
	assert.Equal(t, reg.Name, parsed.Name)
	assert.Equal(t, reg.URL, parsed.URL)
	assert.Equal(t, reg.IsOfficial, parsed.IsOfficial)
	assert.True(t, parsed.IsActive)
}

func TestPlugin_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	manifest := json.RawMessage(`{"version": "1.0.0", "name": "test-plugin"}`)

	plugin := Plugin{
		ID:                 uuid.New(),
		Name:               "test-plugin",
		DisplayName:        "Test Plugin",
		Description:        "A test plugin for testing",
		Version:            "1.0.0",
		RepositoryURL:      "https://github.com/test/test-plugin",
		RepositoryType:     RepoGitHub,
		Author:             "Test Author",
		License:            "MIT",
		HomepageURL:        "https://test.example.com",
		State:              StateEnabled,
		GrantedPermissions: []string{"read:invoices", "write:contacts"},
		Manifest:           manifest,
		InstalledAt:        now,
		UpdatedAt:          now,
	}

	data, err := json.Marshal(plugin)
	assert.NoError(t, err)

	var parsed Plugin
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, plugin.ID, parsed.ID)
	assert.Equal(t, plugin.Name, parsed.Name)
	assert.Equal(t, plugin.DisplayName, parsed.DisplayName)
	assert.Equal(t, plugin.Version, parsed.Version)
	assert.Equal(t, plugin.RepositoryType, parsed.RepositoryType)
	assert.Equal(t, plugin.State, parsed.State)
	assert.Equal(t, plugin.GrantedPermissions, parsed.GrantedPermissions)
}

func TestTenantPlugin_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	enabledAt := now.Add(-1 * time.Hour)
	settings := json.RawMessage(`{"theme": "dark", "notifications": true}`)

	tp := TenantPlugin{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		PluginID:  uuid.New(),
		IsEnabled: true,
		Settings:  settings,
		EnabledAt: &enabledAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(tp)
	assert.NoError(t, err)

	var parsed TenantPlugin
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, tp.ID, parsed.ID)
	assert.Equal(t, tp.TenantID, parsed.TenantID)
	assert.Equal(t, tp.PluginID, parsed.PluginID)
	assert.True(t, parsed.IsEnabled)
}

func TestPluginMigration_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	migration := PluginMigration{
		ID:        uuid.New(),
		PluginID:  uuid.New(),
		Version:   "1.0.0",
		Filename:  "001_initial.sql",
		AppliedAt: now,
		Checksum:  "sha256:abc123",
	}

	data, err := json.Marshal(migration)
	assert.NoError(t, err)

	var parsed PluginMigration
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, migration.ID, parsed.ID)
	assert.Equal(t, migration.Version, parsed.Version)
	assert.Equal(t, migration.Filename, parsed.Filename)
	assert.Equal(t, migration.Checksum, parsed.Checksum)
}

func TestPluginInfo_JSONSerialization(t *testing.T) {
	info := PluginInfo{
		Name:        "analytics-plugin",
		DisplayName: "Analytics Dashboard",
		Description: "Advanced analytics for your accounting data",
		Repository:  "https://github.com/openaccounting/analytics-plugin",
		Version:     "2.1.0",
		Author:      "Open Accounting Team",
		License:     "Apache-2.0",
		Tags:        []string{"analytics", "dashboard", "reporting"},
		Downloads:   1500,
		Stars:       42,
	}

	data, err := json.Marshal(info)
	assert.NoError(t, err)

	var parsed PluginInfo
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, info.Name, parsed.Name)
	assert.Equal(t, info.DisplayName, parsed.DisplayName)
	assert.Equal(t, info.Tags, parsed.Tags)
	assert.Equal(t, info.Downloads, parsed.Downloads)
	assert.Equal(t, info.Stars, parsed.Stars)
}

func TestRegistryIndex_JSONSerialization(t *testing.T) {
	index := RegistryIndex{
		Version: 1,
		Plugins: []PluginInfo{
			{
				Name:        "plugin-1",
				DisplayName: "Plugin One",
				Version:     "1.0.0",
			},
			{
				Name:        "plugin-2",
				DisplayName: "Plugin Two",
				Version:     "2.0.0",
			},
		},
	}

	data, err := json.Marshal(index)
	assert.NoError(t, err)

	var parsed RegistryIndex
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, index.Version, parsed.Version)
	assert.Len(t, parsed.Plugins, 2)
	assert.Equal(t, "plugin-1", parsed.Plugins[0].Name)
	assert.Equal(t, "plugin-2", parsed.Plugins[1].Name)
}

func TestCreateRegistryRequest_JSONSerialization(t *testing.T) {
	req := CreateRegistryRequest{
		Name:        "custom-registry",
		URL:         "https://custom.example.com/plugins",
		Description: "My custom plugin registry",
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var parsed CreateRegistryRequest
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, req.Name, parsed.Name)
	assert.Equal(t, req.URL, parsed.URL)
	assert.Equal(t, req.Description, parsed.Description)
}

func TestInstallPluginRequest_JSONSerialization(t *testing.T) {
	req := InstallPluginRequest{
		RepositoryURL: "https://github.com/test/my-plugin",
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var parsed InstallPluginRequest
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, req.RepositoryURL, parsed.RepositoryURL)
}

func TestEnablePluginRequest_JSONSerialization(t *testing.T) {
	req := EnablePluginRequest{
		GrantedPermissions: []string{"read:invoices", "write:contacts", "read:reports"},
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var parsed EnablePluginRequest
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, req.GrantedPermissions, parsed.GrantedPermissions)
	assert.Len(t, parsed.GrantedPermissions, 3)
}

func TestTenantPluginSettingsRequest_JSONSerialization(t *testing.T) {
	settings := json.RawMessage(`{"apiKey": "test-key", "enabled": true}`)
	req := TenantPluginSettingsRequest{
		Settings: settings,
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)

	var parsed TenantPluginSettingsRequest
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.JSONEq(t, string(settings), string(parsed.Settings))
}

func TestPluginSearchResult_JSONSerialization(t *testing.T) {
	result := PluginSearchResult{
		Plugin: PluginInfo{
			Name:        "search-result-plugin",
			DisplayName: "Search Result Plugin",
			Version:     "1.0.0",
		},
		Registry: "official",
	}

	data, err := json.Marshal(result)
	assert.NoError(t, err)

	var parsed PluginSearchResult
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, result.Plugin.Name, parsed.Plugin.Name)
	assert.Equal(t, result.Registry, parsed.Registry)
}

func TestPlugin_WithJoinedPlugin(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	plugin := &Plugin{
		ID:          uuid.New(),
		Name:        "parent-plugin",
		DisplayName: "Parent Plugin",
		Version:     "1.0.0",
	}

	tp := TenantPlugin{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		PluginID:  plugin.ID,
		IsEnabled: true,
		CreatedAt: now,
		UpdatedAt: now,
		Plugin:    plugin,
	}

	assert.NotNil(t, tp.Plugin)
	assert.Equal(t, plugin.Name, tp.Plugin.Name)
	assert.Equal(t, plugin.ID, tp.PluginID)
}

func TestPluginInfo_EmptyTags(t *testing.T) {
	info := PluginInfo{
		Name:    "minimal-plugin",
		Version: "1.0.0",
	}

	assert.Nil(t, info.Tags)
	assert.Empty(t, info.Description)
	assert.Empty(t, info.Author)
}

func TestRegistry_OptionalFields(t *testing.T) {
	reg := Registry{
		ID:         uuid.New(),
		Name:       "test",
		URL:        "https://example.com",
		IsOfficial: false,
		IsActive:   true,
	}

	// LastSyncedAt should be nil for never synced
	assert.Nil(t, reg.LastSyncedAt)
	assert.Empty(t, reg.Description)
}
