package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPluginWave4NewServiceNilPoolInitializesRuntimeState(t *testing.T) {
	service := NewService(nil, t.TempDir())

	if service == nil {
		t.Fatal("NewService(nil) returned nil")
	}
	if service.repo != nil {
		t.Fatalf("repo = %#v, want nil for nil pool", service.repo)
	}
	if service.hooks == nil || service.plugins == nil || service.runtimeFailures == nil {
		t.Fatalf("service maps/hooks not initialized: %#v", service)
	}
	if service.runtimeRestartBackoff != packageRuntimeCrashBackoff {
		t.Fatalf("runtimeRestartBackoff = %s, want %s", service.runtimeRestartBackoff, packageRuntimeCrashBackoff)
	}
}

func TestPluginWave4CreateDemoInstallFixtureRepositoryCreatesCleanGitRepo(t *testing.T) {
	cloneURL, cleanup, err := createDemoInstallFixtureRepository(context.Background())
	if err != nil {
		t.Fatalf("createDemoInstallFixtureRepository() error = %v", err)
	}
	u, err := url.Parse(cloneURL)
	if err != nil {
		cleanup()
		t.Fatalf("fixture clone URL did not parse: %v", err)
	}
	repoDir := u.Path
	tempDir := filepath.Dir(repoDir)
	defer cleanup()

	for _, name := range []string{"plugin.yaml", "LICENSE", ".git"} {
		if _, err := os.Stat(filepath.Join(repoDir, name)); err != nil {
			t.Fatalf("fixture repo missing %s: %v", name, err)
		}
	}

	cleanup()
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("cleanup left temp dir behind: %v", err)
	}
}

func TestPluginWave4RuntimeStatusAndRestartValidationEdges(t *testing.T) {
	ctx := context.Background()
	pluginID := mustPluginWave4UUID(t, "11111111-1111-4111-8111-111111111111")
	manifest := Manifest{
		Name:        "runtime-plugin",
		DisplayName: "Runtime Plugin",
		Version:     "1.0.0",
		Backend: &BackendConfig{
			Runtime:    BackendRuntimePackage,
			Package:    "backend",
			Executable: "bin/runtime",
			Routes:     []RouteConfig{{Method: "GET", Path: "/status", Handler: "status"}},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	repo := NewMockRepository()
	repo.plugins[pluginID] = &Plugin{
		ID:          pluginID,
		Name:        manifest.Name,
		DisplayName: manifest.DisplayName,
		State:       StateFailed,
		Manifest:    manifestJSON,
	}
	service := NewServiceWithRepository(repo, nil, t.TempDir())

	status, err := service.GetPluginRuntimeStatus(ctx, pluginID)
	if err != nil {
		t.Fatalf("GetPluginRuntimeStatus() error = %v", err)
	}
	if status.State != RuntimeStateFailed || status.Health != RuntimeHealthUnhealthy {
		t.Fatalf("status = %+v, want failed/unhealthy", status)
	}

	repo.plugins[pluginID].State = StateInstalled
	_, err = service.RestartPluginRuntime(ctx, pluginID)
	if !errors.Is(err, ErrPluginNotEnabled) {
		t.Fatalf("RestartPluginRuntime() error = %v, want ErrPluginNotEnabled", err)
	}

	httpManifest := manifest
	httpManifest.Backend = &BackendConfig{
		Runtime: BackendRuntimeHTTP,
		BaseURL: "http://127.0.0.1:8080",
		Routes:  []RouteConfig{{Method: "GET", Path: "/status", Handler: "status"}},
	}
	httpManifestJSON, err := json.Marshal(httpManifest)
	if err != nil {
		t.Fatalf("marshal http manifest: %v", err)
	}
	repo.plugins[pluginID].State = StateEnabled
	repo.plugins[pluginID].Manifest = httpManifestJSON
	_, err = service.RestartPluginRuntime(ctx, pluginID)
	if !errors.Is(err, ErrPluginRuntimeUnsupported) || !strings.Contains(err.Error(), "does not use a supervised package runtime") {
		t.Fatalf("RestartPluginRuntime() error = %v, want unsupported package runtime error", err)
	}
}

func mustPluginWave4UUID(t *testing.T, value string) uuid.UUID {
	t.Helper()
	parsed, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid %q: %v", value, err)
	}
	return parsed
}

func TestPluginWave4PackageRuntimeStopReturnsCanceledContext(t *testing.T) {
	runtime := &packageRuntimeProcess{
		cmd:        &exec.Cmd{},
		exited:     make(chan struct{}),
		pluginName: "stuck-runtime",
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(runtime.exited)
	}()

	err := runtime.stop(ctx)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("stop() error = %v, want context.Canceled", err)
	}
}
