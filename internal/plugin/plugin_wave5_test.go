package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wave5BackendRuntime struct {
	closeErr    error
	statusValue PluginRuntimeStatus
}

func (r *wave5BackendRuntime) invokeHook(context.Context, uuid.UUID, string, HookConfig, Event) error {
	return nil
}

func (r *wave5BackendRuntime) invokeRoute(context.Context, uuid.UUID, uuid.UUID, RouteConfig, string, string, string, http.Header, io.Reader) (*RuntimeRouteResponse, error) {
	return &RuntimeRouteResponse{StatusCode: http.StatusAccepted, Body: []byte("ok")}, nil
}

func (r *wave5BackendRuntime) close(context.Context) error {
	return r.closeErr
}

func (r *wave5BackendRuntime) status() PluginRuntimeStatus {
	return r.statusValue
}

func TestPluginWave5RuntimeRouteAndStatusEdges(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
	pluginID := uuid.New()
	pluginValue := &Plugin{
		ID:          pluginID,
		Name:        "wave5-plugin",
		DisplayName: "Wave 5 Plugin",
		State:       StateEnabled,
	}

	runtime, err := service.runtimeForTenantPluginRoute(pluginValue, &Manifest{})
	require.NoError(t, err)
	assert.Nil(t, runtime)

	_, ok := findRuntimeRoute(&Manifest{}, http.MethodGet, "/status")
	assert.False(t, ok)

	manifest := &Manifest{Backend: &BackendConfig{
		Routes: []RouteConfig{{Method: "GET", Path: "/status", Handler: "/routes/status"}},
	}}
	_, ok = findRuntimeRoute(manifest, http.MethodPost, "/status")
	assert.False(t, ok)

	packageManifest := &Manifest{Backend: &BackendConfig{
		Runtime: BackendRuntimePackage,
		Routes:  []RouteConfig{{Method: "GET", Path: "/status", Handler: "/routes/status"}},
	}}
	runtime, err = service.runtimeForTenantPluginRoute(pluginValue, packageManifest)
	assert.Nil(t, runtime)
	require.ErrorIs(t, err, ErrPluginRuntimeUnavailable)

	service.recordRuntimeFailure(pluginValue, packageManifest, errors.New("boot failed"), packageRuntimeStats{RestartCount: 2, CrashCount: 3})
	status := service.runtimeStatusForPlugin(pluginValue, packageManifest)
	assert.Equal(t, RuntimeStateFailed, status.State)
	assert.Equal(t, 2, status.RestartCount)
	assert.Equal(t, 3, status.CrashCount)
	assert.Equal(t, "boot failed", status.LastError)

	service.recordRuntimeFailure(pluginValue, packageManifest, nil, packageRuntimeStats{})
	assert.Equal(t, RuntimeStateFailed, service.runtimeFailures[pluginID].State)
}

func TestPluginWave5InvokeTenantPluginRouteUsesEmptyBody(t *testing.T) {
	ctx := context.Background()
	tenantID := uuid.New()
	pluginID := uuid.New()
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer server.Close()

	manifest := Manifest{
		Name:        "route-plugin",
		DisplayName: "Route Plugin",
		Version:     "1.0.0",
		Backend: &BackendConfig{
			Runtime: BackendRuntimeHTTP,
			BaseURL: server.URL,
			Routes:  []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	require.NoError(t, err)

	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:       pluginID,
		Name:     manifest.Name,
		State:    StateEnabled,
		Manifest: manifestJSON,
	}
	repo.tenantPlugins[tenantID.String()+":"+pluginID.String()] = &TenantPlugin{
		ID:        uuid.New(),
		TenantID:  tenantID,
		PluginID:  pluginID,
		IsEnabled: true,
	}
	service := NewServiceWithRepository(repo, nil, t.TempDir())

	resp, err := service.InvokeTenantPluginRoute(ctx, tenantID, pluginID, http.MethodGet, "status", "", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Empty(t, body)
}

func TestPluginWave5UnloadAndLoadEnabledPluginErrorBranches(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
	service.unloadPlugin("missing-plugin")

	pluginID := uuid.New()
	closeErr := errors.New("close failed")
	service.plugins["loaded-plugin"] = &LoadedPlugin{
		Plugin: &Plugin{ID: pluginID, Name: "loaded-plugin"},
		Runtime: &wave5BackendRuntime{
			closeErr: closeErr,
			statusValue: PluginRuntimeStatus{
				State:  RuntimeStateRunning,
				Health: RuntimeHealthHealthy,
			},
		},
	}
	service.runtimeFailures[pluginID] = PluginRuntimeStatus{State: RuntimeStateFailed}

	service.unloadPlugin("loaded-plugin")

	_, loaded := service.plugins["loaded-plugin"]
	assert.False(t, loaded)
	_, failed := service.runtimeFailures[pluginID]
	assert.False(t, failed)

	repo := NewMockRepository()
	badManifest := Manifest{
		Name: "bad-runtime",
		Backend: &BackendConfig{
			Runtime: BackendRuntimeHTTP,
			BaseURL: "http://example.com",
			Routes:  []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}},
		},
	}
	manifestJSON, err := json.Marshal(badManifest)
	require.NoError(t, err)
	enabledID := uuid.New()
	repo.plugins[enabledID] = &Plugin{
		ID:       enabledID,
		Name:     badManifest.Name,
		State:    StateEnabled,
		Manifest: manifestJSON,
	}
	service = NewServiceWithRepository(repo, nil, t.TempDir())

	require.NoError(t, service.LoadEnabledPlugins(context.Background()))
	status := service.runtimeFailures[enabledID]
	assert.Equal(t, RuntimeStateFailed, status.State)
	assert.Contains(t, status.LastError, "loopback")
}
