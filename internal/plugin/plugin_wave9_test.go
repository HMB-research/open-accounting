package plugin

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wave9Runtime struct {
	closeErr    error
	statusValue PluginRuntimeStatus
}

func (r *wave9Runtime) invokeHook(context.Context, uuid.UUID, string, HookConfig, Event) error {
	return nil
}

func (r *wave9Runtime) invokeRoute(context.Context, uuid.UUID, uuid.UUID, RouteConfig, string, string, string, http.Header, io.Reader) (*RuntimeRouteResponse, error) {
	return &RuntimeRouteResponse{StatusCode: http.StatusOK}, nil
}

func (r *wave9Runtime) close(context.Context) error {
	return r.closeErr
}

func (r *wave9Runtime) status() PluginRuntimeStatus {
	return r.statusValue
}

func TestPluginWave9DemoFixtureCloneAndCommitErrors(t *testing.T) {
	t.Run("cloneRepository wraps demo fixture creation errors", func(t *testing.T) {
		binDir := t.TempDir()
		gitPath := filepath.Join(binDir, "git")
		require.NoError(t, os.WriteFile(gitPath, []byte("#!/bin/sh\necho fixture init failed >&2\nexit 42\n"), 0700))
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		t.Setenv("DEMO_MODE", "true")
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

		pluginPath, err := service.cloneRepository(context.Background(), DemoInstallFixtureRepositoryURL)

		assert.Empty(t, pluginPath)
		require.ErrorContains(t, err, "fixture init failed")
	})

	t.Run("demo fixture reports commit errors after init and add", func(t *testing.T) {
		binDir := t.TempDir()
		gitPath := filepath.Join(binDir, "git")
		script := `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "commit" ]; then
    echo fixture commit failed >&2
    exit 43
  fi
done
exit 0
`
		require.NoError(t, os.WriteFile(gitPath, []byte(script), 0700))
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		cloneURL, cleanup, err := createDemoInstallFixtureRepository(context.Background())

		assert.Empty(t, cloneURL)
		assert.Nil(t, cleanup)
		require.ErrorContains(t, err, "fixture commit failed")
	})
}

func TestPluginWave9RuntimeStatusAndLoadEdges(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
	pluginID := uuid.New()
	plugin := &Plugin{ID: pluginID, Name: "wave9-plugin", DisplayName: "Wave 9", State: StateEnabled}
	manifest := &Manifest{Backend: &BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: "http://127.0.0.1:9876", Routes: []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}}}}
	previous := &wave9Runtime{closeErr: errors.New("close failed"), statusValue: PluginRuntimeStatus{State: RuntimeStateRunning, Runtime: BackendRuntimeHTTP}}
	service.plugins[plugin.Name] = &LoadedPlugin{Plugin: plugin, Manifest: manifest, Runtime: previous}

	require.NoError(t, service.loadPluginWithRuntimeStats(plugin, &Manifest{}, packageRuntimeStats{}))
	loaded, ok := service.GetLoadedPlugin(plugin.Name)
	require.True(t, ok)
	assert.Nil(t, loaded.Runtime)

	service.runtimeFailures = nil
	service.recordRuntimeFailure(plugin, manifest, errors.New("boot failed"), packageRuntimeStats{RestartCount: 2, CrashCount: 3})
	status := service.runtimeStatusForPlugin(plugin, manifest)
	assert.Equal(t, RuntimeStateExternal, status.State)
	assert.Equal(t, BackendRuntimeHTTP, status.Runtime)
	assert.NotNil(t, service.runtimeFailures)

	base := baseRuntimeStatus(plugin, nil)
	assert.Equal(t, RuntimeStateNotConfigured, base.State)
	assert.Equal(t, "plugin does not declare backend runtime work", base.Message)
}

func TestPluginWave9RestartPackageRuntimeStartFailure(t *testing.T) {
	ctx := context.Background()
	pluginID := uuid.New()
	manifestJSON := jsonRaw(t, `{
		"name":"package-plugin",
		"display_name":"Package Plugin",
		"version":"1.0.0",
		"backend":{
			"runtime":"package",
			"package":"backend",
			"executable":"bin/runtime",
			"routes":[{"method":"GET","path":"/status","handler":"/status"}]
		}
	}`)
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:          pluginID,
		Name:        "package-plugin",
		DisplayName: "Package Plugin",
		State:       StateEnabled,
		Manifest:    manifestJSON,
	}
	service := NewServiceWithRepository(repo, nil, t.TempDir())

	status, err := service.RestartPluginRuntime(ctx, pluginID)

	assert.Nil(t, status)
	require.ErrorIs(t, err, ErrPluginRuntimeUnavailable)
	require.ErrorContains(t, err, "not installed")
}

func TestPackageRuntimeWave9RequestAndStatusEdges(t *testing.T) {
	runtimeURL := &url.URL{Scheme: "http", Host: "%zz"}
	runtime := &packageRuntimeProcess{
		runtimeHTTPClient: &runtimeHTTPClient{baseURL: runtimeURL},
		pluginName:        "bad-url-runtime",
		output:            newRuntimeProcessLogBuffer(64),
		exited:            make(chan struct{}),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := runtime.waitForReady(ctx)
	require.Error(t, err)

	assert.Empty(t, runtime.outputSuffix())

	cmd := exec.Command("sh", "-c", "sleep 1")
	running := &packageRuntimeProcess{
		runtimeHTTPClient: &runtimeHTTPClient{baseURL: &url.URL{Scheme: "http", Host: "127.0.0.1:4567"}},
		pluginID:          uuid.New(),
		pluginName:        "running-runtime",
		cmd:               cmd,
		output:            newRuntimeProcessLogBuffer(64),
		exited:            make(chan struct{}),
		startedAt:         time.Now().UTC(),
	}
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	status := running.status()
	require.NotNil(t, status.PID)
	assert.Positive(t, *status.PID)
}

func TestPluginWave9UpdateRepositoryGitPullError(t *testing.T) {
	pluginDir := t.TempDir()
	repoDir := filepath.Join(pluginDir, "owner-repo")
	require.NoError(t, os.MkdirAll(repoDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "plugin.yaml"), []byte("name: pull-plugin\nversion: 1.0.0\n"), 0600))
	service := NewServiceWithRepository(NewMockRepository(), nil, pluginDir)

	err := service.updateRepository(context.Background(), "pull-plugin")

	require.ErrorContains(t, err, "git pull failed")
}

func jsonRaw(t *testing.T, value string) []byte {
	t.Helper()
	return []byte(strings.TrimSpace(value))
}
