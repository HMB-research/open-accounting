package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginWave8InstallPluginLoadAndInsertErrors(t *testing.T) {
	t.Run("load manifest yaml errors after clone", func(t *testing.T) {
		installWave6FakePluginGit(t, ":\n", true)
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

		installed, err := service.InstallPlugin(context.Background(), "https://github.com/owner/repo")

		assert.Nil(t, installed)
		require.ErrorContains(t, err, "failed to load manifest")
	})

	t.Run("insert plugin errors are wrapped", func(t *testing.T) {
		installWave6FakePluginGit(t, `name: wave8-plugin
display_name: Wave 8 Plugin
version: 1.0.0
license: MIT
`, true)
		repo := NewMockRepository()
		repo.createPluginErr = errors.New("insert failed")
		service := NewServiceWithRepository(repo, nil, t.TempDir())

		installed, err := service.InstallPlugin(context.Background(), "https://github.com/owner/repo")

		assert.Nil(t, installed)
		require.ErrorContains(t, err, "failed to insert plugin")
		assert.ErrorContains(t, err, "insert failed")
	})
}

func TestPluginWave8RuntimeStatusBranches(t *testing.T) {
	pluginID := uuid.New()
	plugin := &Plugin{ID: pluginID, Name: "runtime-plugin", DisplayName: "Runtime Plugin", State: StateEnabled}
	manifest := &Manifest{Backend: &BackendConfig{
		Runtime: BackendRuntimePackage,
		Hooks:   []HookConfig{{Event: EventInvoiceCreated, Handler: "/hooks/invoices"}},
		Routes:  []RouteConfig{{Method: "GET", Path: "/status", Handler: "/status"}},
	}}
	service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

	status := service.runtimeStatusForPlugin(plugin, manifest)
	assert.Equal(t, RuntimeStateNotLoaded, status.State)
	assert.Equal(t, RuntimeHealthUnknown, status.Health)
	assert.Equal(t, "package runtime is not loaded", status.Message)

	failed := status
	failed.State = RuntimeStateFailed
	failed.Health = RuntimeHealthUnhealthy
	failed.Message = "runtime failed to start"
	service.runtimeFailures[pluginID] = failed

	status = service.runtimeStatusForPlugin(plugin, manifest)
	assert.Equal(t, RuntimeStateFailed, status.State)
	assert.Equal(t, "runtime failed to start", status.Message)
}

func TestPluginWave8RestartRuntimeAndManifestErrors(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil, t.TempDir())

	status, err := service.GetPluginRuntimeStatus(ctx, pluginID)
	assert.Nil(t, status)
	require.ErrorContains(t, err, "plugin not found")

	plugin := &Plugin{ID: pluginID, Name: "broken-manifest", State: StateEnabled, Manifest: []byte("{")}
	repo.plugins[pluginID] = plugin
	status, err = service.RestartPluginRuntime(ctx, pluginID)
	assert.Nil(t, status)
	require.ErrorContains(t, err, "failed to parse manifest")
}

func TestPluginWave8ParsePluginManifestNil(t *testing.T) {
	manifest, err := parsePluginManifest(nil)

	assert.Nil(t, manifest)
	require.ErrorContains(t, err, "plugin is nil")
}

func TestPluginWave8ManifestJSONForInstallIsSerializable(t *testing.T) {
	manifest := Manifest{Name: "serializable", DisplayName: "Serializable", Version: "1.0.0"}
	data, err := manifest.ToJSON()

	require.NoError(t, err)
	var decoded Manifest
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, manifest.Name, decoded.Name)
}
