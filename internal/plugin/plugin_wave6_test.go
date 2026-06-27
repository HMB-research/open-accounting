package plugin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginWave6ServiceRuntimeBranches(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	pluginDir := t.TempDir()
	service := NewService(nil, pluginDir)
	require.NotNil(t, service)
	assert.Equal(t, pluginDir, service.pluginDir)

	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{ID: pluginID, Name: "bad-manifest", State: StateEnabled, Manifest: []byte("{")}
	service = NewServiceWithRepository(repo, nil, pluginDir)
	status, err := service.GetPluginRuntimeStatus(ctx, pluginID)
	assert.Nil(t, status)
	require.ErrorContains(t, err, "failed to parse manifest")

	packageManifest := Manifest{Backend: &BackendConfig{
		Runtime:    BackendRuntimePackage,
		Package:    "backend",
		Executable: "bin/runtime",
		Routes:     []RouteConfig{{Method: "GET", Path: "/status", Handler: "/status"}},
	}}
	packageManifestJSON, err := json.Marshal(packageManifest)
	require.NoError(t, err)
	repo.plugins[pluginID] = &Plugin{ID: pluginID, Name: "package-plugin", State: StateInstalled, Manifest: packageManifestJSON}
	status, err = service.RestartPluginRuntime(ctx, pluginID)
	assert.Nil(t, status)
	require.ErrorIs(t, err, ErrPluginNotEnabled)

	httpManifest := Manifest{Backend: &BackendConfig{
		Runtime: BackendRuntimeHTTP,
		BaseURL: "http://127.0.0.1:3000",
		Routes:  []RouteConfig{{Method: "GET", Path: "/status", Handler: "/status"}},
	}}
	httpManifestJSON, err := json.Marshal(httpManifest)
	require.NoError(t, err)
	repo.plugins[pluginID] = &Plugin{ID: pluginID, Name: "http-plugin", State: StateEnabled, Manifest: httpManifestJSON}
	status, err = service.RestartPluginRuntime(ctx, pluginID)
	assert.Nil(t, status)
	require.ErrorIs(t, err, ErrPluginRuntimeUnsupported)

	failedPlugin := &Plugin{ID: pluginID, Name: "failed-plugin", State: StateFailed}
	runtimeStatus := service.runtimeStatusForPlugin(failedPlugin, &packageManifest)
	assert.Equal(t, RuntimeStateFailed, runtimeStatus.State)
	assert.Equal(t, RuntimeHealthUnhealthy, runtimeStatus.Health)
	assert.Equal(t, "plugin failed to load", runtimeStatus.Message)

	service.supervisePackageRuntime(nil, &packageManifest, nil)
	runtime, err := service.backendRuntimeForPluginWithStats(&Plugin{Name: "frontend-only"}, &Manifest{Backend: &BackendConfig{}}, packageRuntimeStats{})
	require.NoError(t, err)
	assert.Nil(t, runtime)
}

func TestPluginWave6GitFixtureAndInstallErrors(t *testing.T) {
	t.Run("fixture temp dir error", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "tmp-file")
		require.NoError(t, os.WriteFile(tmpFile, []byte("not a directory"), 0600))
		t.Setenv("TMPDIR", tmpFile)

		cloneURL, cleanup, err := createDemoInstallFixtureRepository(context.Background())

		assert.Empty(t, cloneURL)
		assert.Nil(t, cleanup)
		require.ErrorContains(t, err, "create demo plugin fixture")
	})

	t.Run("git command error is wrapped", func(t *testing.T) {
		err := runDemoInstallFixtureGit(context.Background(), t.TempDir(), "definitely-not-a-git-command")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "git definitely-not-a-git-command failed")
	})

	t.Run("install rejects invalid manifest after clone", func(t *testing.T) {
		installWave6FakePluginGit(t, `name: Invalid_Name
display_name: Invalid Plugin
version: 1.0.0
`, true)
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

		installed, err := service.InstallPlugin(context.Background(), "https://github.com/owner/repo")

		assert.Nil(t, installed)
		require.ErrorContains(t, err, "invalid manifest")
		assert.ErrorContains(t, err, "plugin name must be lowercase")
	})
}

func TestPluginWave6PackageRuntimeExecutableMissingPaths(t *testing.T) {
	pluginRoot := t.TempDir()

	_, _, err := resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{Package: "backend", Executable: "bin/runtime"})
	require.ErrorContains(t, err, "backend.package")
	assert.ErrorIs(t, err, ErrPluginRuntimeUnavailable)

	backendDir := filepath.Join(pluginRoot, "backend")
	require.NoError(t, os.MkdirAll(backendDir, 0750))
	_, _, err = resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{Package: "backend", Executable: "bin/runtime"})
	require.ErrorContains(t, err, "backend.executable")
	assert.ErrorIs(t, err, ErrPluginRuntimeUnavailable)
}

func installWave6FakePluginGit(t *testing.T, manifest string, includeLicense bool) {
	t.Helper()

	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	licenseScript := ""
	if includeLicense {
		licenseScript = "echo MIT > \"$target/LICENSE\"\n"
	}
	script := `#!/bin/sh
if [ "$1" = "clone" ]; then
  target="$5"
  mkdir -p "$target"
  cat > "$target/plugin.yaml" <<'YAML'
` + manifest + `YAML
  ` + licenseScript + `  exit 0
fi
exit 1
`
	require.NoError(t, os.WriteFile(gitPath, []byte(script), 0700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
