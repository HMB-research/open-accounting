package plugin

import (
	"context"
	"encoding/json"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wave7Runtime struct {
	statusValue PluginRuntimeStatus
	routeBody   string
	routePath   string
}

func (r *wave7Runtime) invokeHook(context.Context, uuid.UUID, string, HookConfig, Event) error {
	return nil
}

func (r *wave7Runtime) invokeRoute(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ RouteConfig, _ string, requestPath string, _ string, _ http.Header, body io.Reader) (*RuntimeRouteResponse, error) {
	r.routePath = requestPath
	if body != nil {
		data, _ := io.ReadAll(body)
		r.routeBody = string(data)
	}
	return &RuntimeRouteResponse{StatusCode: http.StatusAccepted, Body: []byte("accepted")}, nil
}

func (r *wave7Runtime) close(context.Context) error {
	return nil
}

func (r *wave7Runtime) status() PluginRuntimeStatus {
	return r.statusValue
}

func TestServiceWave7ConstructorPanicsOnUnreachablePool(t *testing.T) {
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	defer pool.Close()

	var panicValue any
	func() {
		defer func() {
			panicValue = recover()
		}()
		_ = NewService(pool, t.TempDir())
	}()
	require.NotNil(t, panicValue)
	panicErr, ok := panicValue.(error)
	require.True(t, ok, "panic value should be an error, got %T", panicValue)
	assert.Contains(t, panicErr.Error(), "create plugin GORM repository")
}

func TestServiceWave7InvokeTenantPluginRouteBranches(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	manifest := Manifest{Backend: &BackendConfig{
		Runtime: BackendRuntimePackage,
		Routes:  []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}},
	}}
	manifestJSON, err := json.Marshal(manifest)
	require.NoError(t, err)

	t.Run("tenant enablement lookup error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.isEnabledForTenantErr = errors.New("tenant lookup failed")
		service := NewServiceWithRepository(repo, nil, t.TempDir())

		response, err := service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodGet, "/status", "", nil, nil)

		assert.Nil(t, response)
		require.ErrorContains(t, err, "tenant lookup failed")
	})

	t.Run("plugin lookup error after tenant is enabled", func(t *testing.T) {
		repo := NewMockRepository()
		repo.tenantPlugins[tenantID.String()+":"+pluginID.String()] = &TenantPlugin{TenantID: tenantID, PluginID: pluginID, IsEnabled: true}
		service := NewServiceWithRepository(repo, nil, t.TempDir())

		response, err := service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodGet, "/status", "", nil, nil)

		assert.Nil(t, response)
		require.ErrorContains(t, err, "plugin not found")
	})

	t.Run("plugin state must be enabled", func(t *testing.T) {
		repo := NewMockRepository()
		repo.plugins[pluginID] = &Plugin{ID: pluginID, Name: "route-plugin", State: StateInstalled, Manifest: manifestJSON}
		repo.tenantPlugins[tenantID.String()+":"+pluginID.String()] = &TenantPlugin{TenantID: tenantID, PluginID: pluginID, IsEnabled: true}
		service := NewServiceWithRepository(repo, nil, t.TempDir())

		response, err := service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodGet, "/status", "", nil, nil)

		assert.Nil(t, response)
		assert.ErrorIs(t, err, ErrPluginNotEnabled)
	})

	t.Run("invalid manifest and missing route", func(t *testing.T) {
		repo := NewMockRepository()
		repo.plugins[pluginID] = &Plugin{ID: pluginID, Name: "route-plugin", State: StateEnabled, Manifest: []byte("{")}
		repo.tenantPlugins[tenantID.String()+":"+pluginID.String()] = &TenantPlugin{TenantID: tenantID, PluginID: pluginID, IsEnabled: true}
		service := NewServiceWithRepository(repo, nil, t.TempDir())

		response, err := service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodGet, "/status", "", nil, nil)
		assert.Nil(t, response)
		require.ErrorContains(t, err, "failed to parse manifest")

		repo.plugins[pluginID].Manifest = manifestJSON
		response, err = service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodPost, "/missing", "", nil, nil)
		assert.Nil(t, response)
		assert.ErrorIs(t, err, ErrPluginRouteNotFound)
	})

	t.Run("package runtime must be loaded", func(t *testing.T) {
		repo := NewMockRepository()
		repo.plugins[pluginID] = &Plugin{ID: pluginID, Name: "route-plugin", State: StateEnabled, Manifest: manifestJSON}
		repo.tenantPlugins[tenantID.String()+":"+pluginID.String()] = &TenantPlugin{TenantID: tenantID, PluginID: pluginID, IsEnabled: true}
		service := NewServiceWithRepository(repo, nil, t.TempDir())

		response, err := service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodGet, "/status", "fresh=1", http.Header{"X-Test": []string{"1"}}, nil)

		assert.Nil(t, response)
		assert.ErrorIs(t, err, ErrPluginRuntimeUnavailable)
	})

	t.Run("loaded package runtime receives normalized empty body route", func(t *testing.T) {
		repo := NewMockRepository()
		plugin := &Plugin{ID: pluginID, Name: "route-plugin", DisplayName: "Route Plugin", State: StateEnabled, Manifest: manifestJSON}
		repo.plugins[pluginID] = plugin
		repo.tenantPlugins[tenantID.String()+":"+pluginID.String()] = &TenantPlugin{TenantID: tenantID, PluginID: pluginID, IsEnabled: true}
		service := NewServiceWithRepository(repo, nil, t.TempDir())
		runtime := &wave7Runtime{statusValue: PluginRuntimeStatus{State: RuntimeStateRunning, Runtime: BackendRuntimePackage}}
		service.plugins[plugin.Name] = &LoadedPlugin{Plugin: plugin, Manifest: &manifest, Runtime: runtime}

		response, err := service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodGet, "status", "", nil, nil)

		require.NoError(t, err)
		require.NotNil(t, response)
		assert.Equal(t, http.StatusAccepted, response.StatusCode)
		assert.Equal(t, "/status", runtime.routePath)
		assert.Empty(t, runtime.routeBody)
	})
}

func TestServiceWave7RuntimeStatusAndFailureBranches(t *testing.T) {
	pluginID := uuid.New()
	plugin := &Plugin{ID: pluginID, Name: "runtime-plugin", DisplayName: "Runtime Plugin", State: StateEnabled}
	httpManifest := &Manifest{Backend: &BackendConfig{
		Runtime: BackendRuntimeHTTP,
		BaseURL: "http://127.0.0.1:9876",
		Routes:  []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}},
	}}
	service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
	runtime := &wave7Runtime{statusValue: PluginRuntimeStatus{
		State:   RuntimeStateRunning,
		Health:  RuntimeHealthHealthy,
		Runtime: BackendRuntimeHTTP,
	}}
	service.plugins[plugin.Name] = &LoadedPlugin{Plugin: plugin, Manifest: httpManifest, Runtime: runtime}

	status := service.runtimeStatusForPlugin(plugin, httpManifest)
	assert.Equal(t, RuntimeStateRunning, status.State)
	assert.Equal(t, "Runtime Plugin", status.DisplayName)
	assert.Equal(t, 1, status.RouteCount)

	legacy := baseRuntimeStatus(plugin, &Manifest{Backend: &BackendConfig{
		Hooks: []HookConfig{{Event: EventInvoiceCreated, Handler: "/hooks/invoices"}},
	}})
	assert.Equal(t, "legacy", legacy.Runtime)
	assert.Equal(t, RuntimeStateNotConfigured, legacy.State)

	service.recordRuntimeFailure(nil, httpManifest, errors.New("ignored"), packageRuntimeStats{})
	service.recordRuntimeFailure(plugin, httpManifest, nil, packageRuntimeStats{})
	assert.Empty(t, service.runtimeFailures)
}

func TestGitWave7CloneAndFixtureErrorBranches(t *testing.T) {
	t.Run("plugin directory cannot be created", func(t *testing.T) {
		parentFile := filepath.Join(t.TempDir(), "not-a-directory")
		require.NoError(t, os.WriteFile(parentFile, []byte("x"), 0600))
		service := NewServiceWithRepository(NewMockRepository(), nil, filepath.Join(parentFile, "plugins"))

		pluginPath, err := service.cloneRepository(context.Background(), "https://github.com/owner/repo")

		assert.Empty(t, pluginPath)
		require.ErrorContains(t, err, "failed to create plugins directory")
	})

	t.Run("demo fixture cleans up when git init fails", func(t *testing.T) {
		binDir := t.TempDir()
		gitPath := filepath.Join(binDir, "git")
		require.NoError(t, os.WriteFile(gitPath, []byte("#!/bin/sh\necho fixture git failed >&2\nexit 42\n"), 0700))
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		cloneURL, cleanup, err := createDemoInstallFixtureRepository(context.Background())

		assert.Empty(t, cloneURL)
		assert.Nil(t, cleanup)
		require.ErrorContains(t, err, "fixture git failed")
	})

	t.Run("fetch registry rejects unparsable URLs", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

		index, err := service.FetchRegistryIndex(context.Background(), "https://github.com/%zz/repo")

		assert.Nil(t, index)
		require.ErrorContains(t, err, "invalid registry URL")
	})
}

func TestPackageRuntimeWave7StopAndExitBranches(t *testing.T) {
	runtimeURL := &url.URL{Scheme: "http", Host: "127.0.0.1:4567"}

	t.Run("close returns nil after interrupting running process", func(t *testing.T) {
		cmd := exec.Command("sh", "-c", "sleep 10")
		runtime := &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: runtimeURL},
			pluginName:        "sleepy-runtime",
			cmd:               cmd,
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
		}
		require.NoError(t, cmd.Start())
		go runtime.wait()
		t.Cleanup(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		})

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := runtime.close(ctx)

		require.NoError(t, err)
		assert.Equal(t, RuntimeStateStopped, runtime.status().State)
	})

	t.Run("unintentional clean exit is recorded as process exited", func(t *testing.T) {
		runtime := &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: runtimeURL},
			cmd:               &exec.Cmd{},
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
			backoff:           time.Hour,
		}

		runtime.markExited(nil)

		status := runtime.status()
		assert.Equal(t, RuntimeStateBackoff, status.State)
		assert.Equal(t, "process exited", status.LastExitError)
		assert.Equal(t, 1, status.CrashCount)
	})

	t.Run("require running falls back to state when status message is empty", func(t *testing.T) {
		runtime := &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: runtimeURL},
			pluginName:        "exited-runtime",
			cmd:               &exec.Cmd{},
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
		}
		now := time.Now().UTC().Add(-time.Minute)
		runtime.exitedAt = &now

		err := runtime.requireRunning()

		require.ErrorIs(t, err, ErrPluginRuntimeUnavailable)
		assert.True(t, strings.Contains(err.Error(), string(RuntimeStateExited)) || strings.Contains(err.Error(), "runtime process exited"))
	})
}
