package plugin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
	"gorm.io/gorm"
)

var (
	wave10DefaultPluginAbs                    = pluginAbs
	wave10DefaultPluginEnviron                = pluginEnviron
	wave10DefaultPluginMkdirAll               = pluginMkdirAll
	wave10DefaultPluginRel                    = pluginRel
	wave10DefaultPluginRemoveAll              = pluginRemoveAll
	wave10DefaultPluginStat                   = pluginStat
	wave10DefaultPluginWriteFile              = pluginWriteFile
	wave10DefaultNewRegistryIndexRequest      = newRegistryIndexRequest
	wave10DefaultFetchRegistryIndexDo         = fetchRegistryIndexDo
	wave10DefaultNewRuntimeHTTPRequest        = newRuntimeHTTPRequest
	wave10DefaultPackageRuntimeAfter          = packageRuntimeAfter
	wave10DefaultPackageRuntimeCommandContext = packageRuntimeCommandContext
	wave10DefaultPackageRuntimeListen         = packageRuntimeListen
	wave10DefaultPackageRuntimeProcessKill    = packageRuntimeProcessKill
	wave10DefaultPackageRuntimeProcessSignal  = packageRuntimeProcessSignal
	wave10DefaultNewPluginGormDBFromPool      = newPluginGormDBFromPool
)

func TestPluginWave10ServiceManifestAndHookEdges(t *testing.T) {
	t.Run("NewService uses repository from pool on successful gorm setup", func(t *testing.T) {
		restorePluginWave10Seams(t)
		newPluginGormDBFromPool = func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
			return &gorm.DB{}, nil
		}

		service := NewService(&pgxpool.Pool{}, t.TempDir())

		if service.repo == nil {
			t.Fatal("NewService should install a repository when pool setup succeeds")
		}
		if service.pluginDir == "" || service.hooks == nil || service.plugins == nil || service.runtimeFailures == nil {
			t.Fatalf("service was not fully initialized: %#v", service)
		}
	})

	t.Run("InstallPlugin wraps clone and manifest JSON errors", func(t *testing.T) {
		binDir := t.TempDir()
		gitPath := filepath.Join(binDir, "git")
		script := "#!/bin/sh\necho clone failed >&2\nexit 44\n"
		if err := os.WriteFile(gitPath, []byte(script), 0700); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

		installed, err := service.InstallPlugin(context.Background(), "https://github.com/owner/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to clone repository") {
			t.Fatalf("InstallPlugin clone error = %v, want wrapped clone error", err)
		}
		if installed != nil {
			t.Fatalf("installed = %#v, want nil", installed)
		}

		installWave6FakePluginGit(t, `name: wave10-json-plugin
display_name: Wave 10 JSON Plugin
version: 1.0.0
license: MIT
settings:
  type: object
  properties:
    bad_default:
      type: number
      default: .inf
`, true)
		service = NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

		installed, err = service.InstallPlugin(context.Background(), "https://github.com/owner/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to serialize manifest") {
			t.Fatalf("InstallPlugin JSON error = %v, want serialization error", err)
		}
		if installed != nil {
			t.Fatalf("installed = %#v, want nil", installed)
		}
	})

	t.Run("RestartPluginRuntime returns repository lookup errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.getPluginErr = errors.New("repository unavailable")
		service := NewServiceWithRepository(repo, nil, t.TempDir())

		status, err := service.RestartPluginRuntime(context.Background(), uuid.New())

		if err == nil || !strings.Contains(err.Error(), "repository unavailable") {
			t.Fatalf("RestartPluginRuntime error = %v, want repository error", err)
		}
		if status != nil {
			t.Fatalf("status = %#v, want nil", status)
		}
	})

	t.Run("legacy routes without runtime report route execution error", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
		runtime, err := service.backendRuntimeForPluginWithStats(&Plugin{Name: "route-only"}, &Manifest{
			Backend: &BackendConfig{
				Routes: []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}},
			},
		}, packageRuntimeStats{})

		if runtime != nil {
			t.Fatalf("runtime = %#v, want nil", runtime)
		}
		if err == nil || !strings.Contains(err.Error(), "backend routes are declared") {
			t.Fatalf("backendRuntimeForPluginWithStats error = %v, want route runtime error", err)
		}
	})

	t.Run("base runtime status returns early for backend without work", func(t *testing.T) {
		status := baseRuntimeStatus(&Plugin{ID: uuid.New(), Name: "quiet-plugin"}, &Manifest{
			Backend: &BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: "http://127.0.0.1:9000"},
		})

		if status.State != RuntimeStateNotConfigured || status.Health != RuntimeHealthNotApplicable {
			t.Fatalf("status = %+v, want not configured/not applicable", status)
		}
		if status.Runtime != BackendRuntimeHTTP {
			t.Fatalf("runtime = %q, want %q", status.Runtime, BackendRuntimeHTTP)
		}
	})

	t.Run("backend validators surface nested relative path errors", func(t *testing.T) {
		if err := validateLegacyBackendConfig(&BackendConfig{Package: "../backend", Entry: "main"}); err == nil || !strings.Contains(err.Error(), "backend.package") {
			t.Fatalf("legacy backend error = %v, want package path error", err)
		}
		if err := validateHTTPBackendRuntime(&BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: "http://127.0.0.1:9000", Package: "../backend"}); err == nil || !strings.Contains(err.Error(), "backend.package") {
			t.Fatalf("http backend error = %v, want package path error", err)
		}
		if err := validatePackageBackendRuntime(&BackendConfig{Runtime: BackendRuntimePackage}); err == nil || !strings.Contains(err.Error(), "backend.package is required") {
			t.Fatalf("package backend error = %v, want required package error", err)
		}
	})

	t.Run("EmitAsync logs handler errors without blocking caller", func(t *testing.T) {
		registry := NewHookRegistry()
		done := make(chan struct{})
		registry.Register(EventWebhookTest, func(context.Context, Event) error {
			close(done)
			return errors.New("async handler failed")
		})

		registry.EmitAsync(Event{Type: EventWebhookTest})

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("async handler was not invoked")
		}
	})
}

func TestPluginWave10GitAndRegistryEdges(t *testing.T) {
	t.Run("clone reports existing target removal errors", func(t *testing.T) {
		restorePluginWave10Seams(t)
		pluginDir := t.TempDir()
		targetDir := filepath.Join(pluginDir, "owner-repo")
		if err := os.MkdirAll(targetDir, 0750); err != nil {
			t.Fatalf("create target: %v", err)
		}
		pluginRemoveAll = func(path string) error {
			if path == targetDir {
				return errors.New("remove denied")
			}
			return os.RemoveAll(path)
		}
		service := NewServiceWithRepository(NewMockRepository(), nil, pluginDir)

		pluginPath, err := service.cloneRepository(context.Background(), "https://github.com/owner/repo")

		if err == nil || !strings.Contains(err.Error(), "failed to remove existing plugin directory") {
			t.Fatalf("cloneRepository error = %v, want remove error", err)
		}
		if pluginPath != "" {
			t.Fatalf("pluginPath = %q, want empty", pluginPath)
		}
	})

	t.Run("clone cleanup warnings are exercised for invalid cloned contents", func(t *testing.T) {
		restorePluginWave10Seams(t)
		pluginRemoveAll = func(string) error { return errors.New("cleanup denied") }
		installWave10FakeCloneGit(t, "")
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

		pluginPath, err := service.cloneRepository(context.Background(), "https://github.com/owner/repo")
		if err == nil || !strings.Contains(err.Error(), "plugin.yaml") {
			t.Fatalf("missing manifest error = %v, want plugin.yaml error", err)
		}
		if pluginPath != "" {
			t.Fatalf("pluginPath = %q, want empty", pluginPath)
		}

		restorePluginWave10Seams(t)
		pluginRemoveAll = func(string) error { return errors.New("cleanup denied") }
		installWave10FakeCloneGit(t, "manifest-only")
		service = NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
		pluginPath, err = service.cloneRepository(context.Background(), "https://github.com/owner/repo")
		if err == nil || !strings.Contains(err.Error(), "LICENSE") {
			t.Fatalf("missing license error = %v, want license error", err)
		}
		if pluginPath != "" {
			t.Fatalf("pluginPath = %q, want empty", pluginPath)
		}
	})

	t.Run("demo fixture filesystem errors are wrapped", func(t *testing.T) {
		restorePluginWave10Seams(t)
		pluginMkdirAll = func(string, os.FileMode) error { return errors.New("mkdir denied") }
		cloneURL, cleanup, err := createDemoInstallFixtureRepository(context.Background())
		if err == nil || !strings.Contains(err.Error(), "create demo plugin repository") {
			t.Fatalf("mkdir fixture error = %v, want repository error", err)
		}
		if cloneURL != "" || cleanup != nil {
			t.Fatalf("cloneURL=%q cleanup nil=%t, want empty true", cloneURL, cleanup == nil)
		}
	})

	t.Run("demo fixture write and cleanup errors are wrapped", func(t *testing.T) {
		restorePluginWave10Seams(t)
		pluginWriteFile = func(path string, data []byte, perm os.FileMode) error {
			if filepath.Base(path) == "plugin.yaml" {
				return errors.New("manifest write denied")
			}
			return os.WriteFile(path, data, perm)
		}
		cloneURL, cleanup, err := createDemoInstallFixtureRepository(context.Background())
		if err == nil || !strings.Contains(err.Error(), "write demo plugin manifest") {
			t.Fatalf("manifest write error = %v, want manifest write error", err)
		}
		if cloneURL != "" || cleanup != nil {
			t.Fatalf("cloneURL=%q cleanup nil=%t, want empty true", cloneURL, cleanup == nil)
		}

		restorePluginWave10Seams(t)
		pluginWriteFile = func(path string, data []byte, perm os.FileMode) error {
			if filepath.Base(path) == "LICENSE" {
				return errors.New("license write denied")
			}
			return os.WriteFile(path, data, perm)
		}
		cloneURL, cleanup, err = createDemoInstallFixtureRepository(context.Background())
		if err == nil || !strings.Contains(err.Error(), "write demo plugin license") {
			t.Fatalf("license write error = %v, want license write error", err)
		}
		if cloneURL != "" || cleanup != nil {
			t.Fatalf("cloneURL=%q cleanup nil=%t, want empty true", cloneURL, cleanup == nil)
		}

		restorePluginWave10Seams(t)
		var tempDir string
		pluginRemoveAll = func(path string) error {
			tempDir = path
			return errors.New("cleanup denied")
		}
		cloneURL, cleanup, err = createDemoInstallFixtureRepository(context.Background())
		if err != nil {
			t.Fatalf("createDemoInstallFixtureRepository() error = %v", err)
		}
		if cleanup == nil {
			t.Fatal("cleanup should not be nil")
		}
		cleanup()
		if tempDir == "" {
			t.Fatal("cleanup did not call pluginRemoveAll")
		}
		_ = os.RemoveAll(tempDir)
	})

	t.Run("demo fixture add errors are wrapped", func(t *testing.T) {
		binDir := t.TempDir()
		gitPath := filepath.Join(binDir, "git")
		script := `#!/bin/sh
for arg in "$@"; do
  if [ "$arg" = "add" ]; then
    echo fixture add failed >&2
    exit 45
  fi
done
exit 0
`
		if err := os.WriteFile(gitPath, []byte(script), 0700); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
		t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

		cloneURL, cleanup, err := createDemoInstallFixtureRepository(context.Background())

		if err == nil || !strings.Contains(err.Error(), "fixture add failed") {
			t.Fatalf("add fixture error = %v, want add error", err)
		}
		if cloneURL != "" || cleanup != nil {
			t.Fatalf("cloneURL=%q cleanup nil=%t, want empty true", cloneURL, cleanup == nil)
		}
	})

	t.Run("updateRepository validates absolute paths", func(t *testing.T) {
		pluginDir := t.TempDir()
		writeWave10PluginManifest(t, filepath.Join(pluginDir, "owner-repo"), "pull-plugin")
		service := NewServiceWithRepository(NewMockRepository(), nil, pluginDir)

		restorePluginWave10Seams(t)
		pluginAbs = func(string) (string, error) { return "", errors.New("bad plugin path") }
		if err := service.updateRepository(context.Background(), "pull-plugin"); err == nil || !strings.Contains(err.Error(), "invalid plugin path") {
			t.Fatalf("plugin path abs error = %v, want invalid plugin path", err)
		}

		restorePluginWave10Seams(t)
		calls := 0
		pluginAbs = func(path string) (string, error) {
			calls++
			if calls == 2 {
				return "", errors.New("bad plugin dir")
			}
			return filepath.Abs(path)
		}
		if err := service.updateRepository(context.Background(), "pull-plugin"); err == nil || !strings.Contains(err.Error(), "invalid plugin directory") {
			t.Fatalf("plugin dir abs error = %v, want invalid plugin directory", err)
		}

		restorePluginWave10Seams(t)
		calls = 0
		pluginAbs = func(path string) (string, error) {
			calls++
			if calls == 1 {
				return "/outside/plugin", nil
			}
			return "/plugins", nil
		}
		if err := service.updateRepository(context.Background(), "pull-plugin"); err == nil || !strings.Contains(err.Error(), "outside the plugins directory") {
			t.Fatalf("outside path error = %v, want outside directory error", err)
		}
	})

	t.Run("registry request, close, read, and sync errors", func(t *testing.T) {
		restorePluginWave10Seams(t)
		newRegistryIndexRequest = func(context.Context, string, string, io.Reader) (*http.Request, error) {
			return nil, errors.New("request denied")
		}
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
		index, err := service.FetchRegistryIndex(context.Background(), "https://github.com/owner/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to create request") {
			t.Fatalf("request error = %v, want create request error", err)
		}
		if index != nil {
			t.Fatalf("index = %#v, want nil", index)
		}

		restorePluginWave10Seams(t)
		fetchRegistryIndexDo = func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       &wave10ReadCloser{reader: strings.NewReader("version: 1\nplugins: []\n"), closeErr: errors.New("close denied")},
			}, nil
		}
		index, err = service.FetchRegistryIndex(context.Background(), "https://github.com/owner/repo")
		if err != nil {
			t.Fatalf("FetchRegistryIndex with close error returned error = %v", err)
		}
		if index == nil || index.Version != 1 {
			t.Fatalf("index = %#v, want parsed version 1", index)
		}

		restorePluginWave10Seams(t)
		fetchRegistryIndexDo = func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: &wave10ReadCloser{readErr: errors.New("read denied")}}, nil
		}
		index, err = service.FetchRegistryIndex(context.Background(), "https://github.com/owner/repo")
		if err == nil || !strings.Contains(err.Error(), "failed to read registry index") {
			t.Fatalf("read error = %v, want read registry error", err)
		}
		if index != nil {
			t.Fatalf("index = %#v, want nil", index)
		}

		repo := NewMockRepository()
		registryID := uuid.New()
		repo.registries[registryID] = &Registry{ID: registryID, Name: "bad-registry", URL: "https://example.com/owner/repo", IsActive: true}
		service = NewServiceWithRepository(repo, nil, t.TempDir())
		if err := service.SyncRegistry(context.Background(), registryID); err == nil || !strings.Contains(err.Error(), "failed to sync registry bad-registry") {
			t.Fatalf("SyncRegistry error = %v, want wrapped fetch error", err)
		}
	})
}

func TestPluginWave10RuntimeHTTPEdges(t *testing.T) {
	pluginID := uuid.New()
	baseURL := &url.URL{Scheme: "http", Host: "127.0.0.1:1234"}

	t.Run("invokeHook handles request creation and client errors", func(t *testing.T) {
		restorePluginWave10Seams(t)
		newRuntimeHTTPRequest = func(context.Context, string, string, io.Reader) (*http.Request, error) {
			return nil, errors.New("request build failed")
		}
		client := &runtimeHTTPClient{baseURL: baseURL, client: http.DefaultClient}
		err := client.invokeHook(context.Background(), pluginID, "hook-plugin", HookConfig{Handler: "/hook"}, Event{Type: EventWebhookTest})
		if err == nil || !strings.Contains(err.Error(), "request build failed") {
			t.Fatalf("invokeHook request error = %v, want build error", err)
		}

		restorePluginWave10Seams(t)
		client = &runtimeHTTPClient{
			baseURL: baseURL,
			client: &http.Client{Transport: wave10RoundTripper(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("runtime unavailable")
			})},
		}
		err = client.invokeHook(context.Background(), pluginID, "hook-plugin", HookConfig{Handler: "/hook"}, Event{Type: EventWebhookTest})
		if err == nil || !strings.Contains(err.Error(), "runtime unavailable") {
			t.Fatalf("invokeHook client error = %v, want runtime error", err)
		}
	})

	t.Run("invokeRoute handles response read errors", func(t *testing.T) {
		client := &runtimeHTTPClient{
			baseURL: baseURL,
			client: &http.Client{Transport: wave10RoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: &wave10ReadCloser{readErr: errors.New("body read failed")}}, nil
			})},
		}

		response, err := client.invokeRoute(context.Background(), pluginID, uuid.New(), RouteConfig{Handler: "/route"}, http.MethodGet, "/route", "", nil, strings.NewReader(""))

		if err == nil || !strings.Contains(err.Error(), "body read failed") {
			t.Fatalf("invokeRoute read error = %v, want body read error", err)
		}
		if response != nil {
			t.Fatalf("response = %#v, want nil", response)
		}
	})
}

func TestPluginWave10PackageRuntimeEdges(t *testing.T) {
	t.Run("environment skips malformed and duplicate entries", func(t *testing.T) {
		restorePluginWave10Seams(t)
		pluginEnviron = func() []string {
			return []string{
				"BROKEN",
				"PATH=/first",
				"PATH=/second",
				"TMPDIR=/tmp/runtime",
				"AWS_SECRET_ACCESS_KEY=secret",
			}
		}

		env := packageRuntimeEnvironment(&Plugin{ID: uuid.New(), Name: "env-plugin"}, "127.0.0.1:1", "http://127.0.0.1:1")

		if countWave10Env(env, "PATH=") != 1 {
			t.Fatalf("env PATH count = %d, want 1: %v", countWave10Env(env, "PATH="), env)
		}
		if !containsWave10Env(env, "PATH=/first") || containsWave10Env(env, "PATH=/second") {
			t.Fatalf("env did not preserve first PATH only: %v", env)
		}
		if containsWave10Env(env, "BROKEN") || containsWave10Env(env, "AWS_SECRET_ACCESS_KEY=secret") {
			t.Fatalf("env leaked malformed or secret values: %v", env)
		}
	})

	t.Run("resolve helpers expose OS errors", func(t *testing.T) {
		restorePluginWave10Seams(t)
		pluginAbs = func(string) (string, error) { return "", errors.New("abs denied") }
		_, _, err := resolvePackageRuntimeExecutable(t.TempDir(), &BackendConfig{Package: "backend", Executable: "bin/runtime"})
		if err == nil || !strings.Contains(err.Error(), "invalid plugin path") {
			t.Fatalf("resolve abs error = %v, want invalid plugin path", err)
		}

		restorePluginWave10Seams(t)
		pluginRoot := t.TempDir()
		writeRuntimeExecutable(t, pluginRoot, 0750)
		pluginStat = func(string) (os.FileInfo, error) {
			return nil, errors.New("stat denied")
		}
		_, _, err = resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{Package: "backend", Executable: "bin/runtime"})
		if err == nil || !strings.Contains(err.Error(), "backend.executable") {
			t.Fatalf("resolve stat error = %v, want executable access error", err)
		}

		restorePluginWave10Seams(t)
		pluginRel = func(string, string) (string, error) { return "", errors.New("rel denied") }
		if pathWithinDir("/parent", "/parent/child", false) {
			t.Fatal("pathWithinDir should be false when filepath.Rel fails")
		}
	})

	t.Run("reserve loopback address and startup expose listen and parse errors", func(t *testing.T) {
		restorePluginWave10Seams(t)
		packageRuntimeListen = func(context.Context, string, string) (net.Listener, error) {
			return nil, errors.New("listen denied")
		}
		addr, err := reserveLoopbackRuntimeAddress()
		if err == nil || !strings.Contains(err.Error(), "listen denied") {
			t.Fatalf("reserveLoopbackRuntimeAddress error = %v, want listen error", err)
		}
		if addr != "" {
			t.Fatalf("addr = %q, want empty", addr)
		}

		pluginID := uuid.New()
		service, plugin, manifest := newWave10PackageRuntimeService(t, pluginID, "listen-plugin")
		runtime, err := service.startPackageBackendRuntimeWithStats(plugin, manifest, packageRuntimeStats{})
		if err == nil || !strings.Contains(err.Error(), "reserve package runtime loopback address") {
			t.Fatalf("start listen error = %v, want reserve error", err)
		}
		if runtime != nil {
			t.Fatalf("runtime = %#v, want nil", runtime)
		}

		restorePluginWave10Seams(t)
		service, plugin, manifest = newWave10PackageRuntimeService(t, pluginID, "bad-address-plugin")
		packageRuntimeListen = func(context.Context, string, string) (net.Listener, error) {
			return wave10Listener{addr: wave10Addr("bad host")}, nil
		}
		runtime, err = service.startPackageBackendRuntimeWithStats(plugin, manifest, packageRuntimeStats{})
		if err == nil || !strings.Contains(err.Error(), "invalid URL escape") && !strings.Contains(err.Error(), "invalid character") {
			t.Fatalf("start parse base URL error = %v, want parse error", err)
		}
		if runtime != nil {
			t.Fatalf("runtime = %#v, want nil", runtime)
		}
	})

	t.Run("startup exposes resolve and process start errors", func(t *testing.T) {
		pluginDir := t.TempDir()
		writeWave10PluginManifest(t, filepath.Join(pluginDir, "owner-missing-runtime"), "missing-runtime")
		service := NewServiceWithRepository(NewMockRepository(), nil, pluginDir)
		plugin := &Plugin{ID: uuid.New(), Name: "missing-runtime"}
		manifest := &Manifest{Backend: &BackendConfig{Runtime: BackendRuntimePackage, Package: "backend", Executable: "bin/runtime", Routes: []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}}}}

		runtime, err := service.startPackageBackendRuntimeWithStats(plugin, manifest, packageRuntimeStats{})
		if err == nil || !strings.Contains(err.Error(), "backend.package") {
			t.Fatalf("start resolve error = %v, want backend package error", err)
		}
		if runtime != nil {
			t.Fatalf("runtime = %#v, want nil", runtime)
		}

		restorePluginWave10Seams(t)
		service, plugin, manifest = newWave10PackageRuntimeService(t, uuid.New(), "start-error-plugin")
		packageRuntimeCommandContext = func(ctx context.Context, name string, arg ...string) *exec.Cmd {
			return exec.CommandContext(ctx, filepath.Join(t.TempDir(), "missing-runtime"), arg...)
		}
		runtime, err = service.startPackageBackendRuntimeWithStats(plugin, manifest, packageRuntimeStats{})
		if err == nil || !strings.Contains(err.Error(), "start package runtime") {
			t.Fatalf("start process error = %v, want process start error", err)
		}
		if runtime != nil {
			t.Fatalf("runtime = %#v, want nil", runtime)
		}
	})

	t.Run("waitForReady handles pre-canceled contexts and request errors", func(t *testing.T) {
		baseURL, err := parseLoopbackRuntimeBaseURL("http://127.0.0.1:1")
		if err != nil {
			t.Fatalf("parse base URL: %v", err)
		}
		runtime := &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: baseURL},
			pluginName:        "canceled-runtime",
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err = runtime.waitForReady(ctx)
		if err == nil || !strings.Contains(err.Error(), "did not expose loopback health endpoint") {
			t.Fatalf("waitForReady canceled error = %v, want timeout error", err)
		}

		restorePluginWave10Seams(t)
		newRuntimeHTTPRequest = func(context.Context, string, string, io.Reader) (*http.Request, error) {
			return nil, errors.New("request denied")
		}
		runtime = &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: baseURL},
			pluginName:        "request-runtime",
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
		}
		err = runtime.waitForReady(context.Background())
		if err == nil || !strings.Contains(err.Error(), "request denied") {
			t.Fatalf("waitForReady request error = %v, want request error", err)
		}
	})

	t.Run("stop handles process done, signal failure, kill, and shutdown timeout", func(t *testing.T) {
		restorePluginWave10Seams(t)
		runtime := &packageRuntimeProcess{
			cmd:        &exec.Cmd{Process: &os.Process{Pid: 12345}},
			output:     newRuntimeProcessLogBuffer(64),
			exited:     make(chan struct{}),
			pluginName: "process-done-runtime",
		}
		packageRuntimeProcessSignal = func(*os.Process, os.Signal) error {
			runtime.exitMu.Lock()
			runtime.exitErr = errors.New("process already done")
			runtime.exitMu.Unlock()
			close(runtime.exited)
			return os.ErrProcessDone
		}

		err := runtime.stop(context.Background())
		if err == nil || !strings.Contains(err.Error(), "process already done") {
			t.Fatalf("stop process done error = %v, want exit error", err)
		}

		restorePluginWave10Seams(t)
		killCount := 0
		runtime = &packageRuntimeProcess{
			cmd:        &exec.Cmd{Process: &os.Process{Pid: 12346}},
			output:     newRuntimeProcessLogBuffer(64),
			exited:     make(chan struct{}),
			pluginName: "stubborn-runtime",
		}
		packageRuntimeProcessSignal = func(*os.Process, os.Signal) error {
			return errors.New("signal denied")
		}
		packageRuntimeProcessKill = func(*os.Process) error {
			killCount++
			return nil
		}
		packageRuntimeAfter = func(time.Duration) <-chan time.Time {
			ch := make(chan time.Time, 1)
			ch <- time.Now()
			return ch
		}
		canceled, cancel := context.WithCancel(context.Background())
		cancel()

		err = runtime.stop(canceled)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("stop canceled error = %v, want context.Canceled", err)
		}
		if killCount != 2 {
			t.Fatalf("killCount = %d, want 2", killCount)
		}
	})

	t.Run("supervisor exits when runtime is replaced and records restart failures", func(t *testing.T) {
		pluginID := uuid.New()
		service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())
		service.runtimeRestartBackoff = time.Nanosecond
		plugin := &Plugin{ID: pluginID, Name: "replaced-runtime", State: StateEnabled}
		manifest := &Manifest{Backend: &BackendConfig{Runtime: BackendRuntimePackage, Package: "backend", Executable: "bin/runtime", Routes: []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}}}}
		replaced := newWave10PackageRuntimeProcess(pluginID, plugin.Name)
		service.mu.Lock()
		service.plugins[plugin.Name] = &LoadedPlugin{Plugin: plugin, Manifest: manifest, Runtime: &wave10Runtime{}}
		service.mu.Unlock()

		service.supervisePackageRuntime(plugin, manifest, replaced)
		close(replaced.exited)
		time.Sleep(10 * time.Millisecond)

		failing := newWave10PackageRuntimeProcess(pluginID, "failing-runtime")
		failing.markExited(errors.New("boom"))
		plugin = &Plugin{ID: pluginID, Name: "failing-runtime", State: StateEnabled}
		service.mu.Lock()
		service.plugins[plugin.Name] = &LoadedPlugin{Plugin: plugin, Manifest: manifest, Runtime: failing}
		service.mu.Unlock()

		service.supervisePackageRuntime(plugin, manifest, failing)
		close(failing.exited)

		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			service.mu.RLock()
			_, ok := service.runtimeFailures[pluginID]
			service.mu.RUnlock()
			if ok {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("supervisor did not record restart failure")
	})
}

type wave10Runtime struct{}

func (r *wave10Runtime) invokeHook(context.Context, uuid.UUID, string, HookConfig, Event) error {
	return nil
}

func (r *wave10Runtime) invokeRoute(context.Context, uuid.UUID, uuid.UUID, RouteConfig, string, string, string, http.Header, io.Reader) (*RuntimeRouteResponse, error) {
	return nil, nil
}

func (r *wave10Runtime) close(context.Context) error {
	return nil
}

func (r *wave10Runtime) status() PluginRuntimeStatus {
	return PluginRuntimeStatus{State: RuntimeStateRunning}
}

type wave10ReadCloser struct {
	reader   *strings.Reader
	readErr  error
	closeErr error
}

func (r *wave10ReadCloser) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	if r.reader == nil {
		return 0, io.EOF
	}
	return r.reader.Read(p)
}

func (r *wave10ReadCloser) Close() error {
	return r.closeErr
}

type wave10RoundTripper func(*http.Request) (*http.Response, error)

func (f wave10RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type wave10Addr string

func (a wave10Addr) Network() string {
	return "tcp"
}

func (a wave10Addr) String() string {
	return string(a)
}

type wave10Listener struct {
	addr net.Addr
}

func (l wave10Listener) Accept() (net.Conn, error) {
	return nil, errors.New("unused listener")
}

func (l wave10Listener) Close() error {
	return nil
}

func (l wave10Listener) Addr() net.Addr {
	return l.addr
}

func restorePluginWave10Seams(t *testing.T) {
	t.Helper()

	resetPluginWave10Seams()
	t.Cleanup(resetPluginWave10Seams)
}

func resetPluginWave10Seams() {
	pluginAbs = wave10DefaultPluginAbs
	pluginEnviron = wave10DefaultPluginEnviron
	pluginMkdirAll = wave10DefaultPluginMkdirAll
	pluginRel = wave10DefaultPluginRel
	pluginRemoveAll = wave10DefaultPluginRemoveAll
	pluginStat = wave10DefaultPluginStat
	pluginWriteFile = wave10DefaultPluginWriteFile
	newRegistryIndexRequest = wave10DefaultNewRegistryIndexRequest
	fetchRegistryIndexDo = wave10DefaultFetchRegistryIndexDo
	newRuntimeHTTPRequest = wave10DefaultNewRuntimeHTTPRequest
	packageRuntimeAfter = wave10DefaultPackageRuntimeAfter
	packageRuntimeCommandContext = wave10DefaultPackageRuntimeCommandContext
	packageRuntimeListen = wave10DefaultPackageRuntimeListen
	packageRuntimeProcessKill = wave10DefaultPackageRuntimeProcessKill
	packageRuntimeProcessSignal = wave10DefaultPackageRuntimeProcessSignal
	newPluginGormDBFromPool = wave10DefaultNewPluginGormDBFromPool
}

func installWave10FakeCloneGit(t *testing.T, mode string) {
	t.Helper()

	binDir := t.TempDir()
	gitPath := filepath.Join(binDir, "git")
	body := ""
	switch mode {
	case "manifest-only":
		body = "cat > \"$target/plugin.yaml\" <<'YAML'\nname: clone-plugin\nversion: 1.0.0\nYAML\n"
	case "complete":
		body = "cat > \"$target/plugin.yaml\" <<'YAML'\nname: clone-plugin\nversion: 1.0.0\nYAML\necho MIT > \"$target/LICENSE\"\n"
	}
	script := `#!/bin/sh
if [ "$1" = "clone" ]; then
  target="$5"
  mkdir -p "$target"
  ` + body + `  exit 0
fi
exit 1
`
	if err := os.WriteFile(gitPath, []byte(script), 0700); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writeWave10PluginManifest(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatalf("create plugin dir: %v", err)
	}
	manifest := fmt.Sprintf("name: %s\nversion: 1.0.0\n", name)
	if err := os.WriteFile(filepath.Join(dir, "plugin.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}
}

func newWave10PackageRuntimeService(t *testing.T, pluginID uuid.UUID, pluginName string) (*Service, *Plugin, *Manifest) {
	t.Helper()
	pluginDir := t.TempDir()
	pluginRoot := filepath.Join(pluginDir, "owner-"+pluginName)
	writeWave10PluginManifest(t, pluginRoot, pluginName)
	writeRuntimeExecutable(t, pluginRoot, 0750)
	service := NewServiceWithRepository(NewMockRepository(), nil, pluginDir)
	plugin := &Plugin{ID: pluginID, Name: pluginName, State: StateEnabled}
	manifest := &Manifest{Backend: &BackendConfig{
		Runtime:    BackendRuntimePackage,
		Package:    "backend",
		Executable: "bin/runtime",
		Routes:     []RouteConfig{{Method: http.MethodGet, Path: "/status", Handler: "/status"}},
	}}
	return service, plugin, manifest
}

func newWave10PackageRuntimeProcess(pluginID uuid.UUID, pluginName string) *packageRuntimeProcess {
	return &packageRuntimeProcess{
		runtimeHTTPClient: &runtimeHTTPClient{baseURL: &url.URL{Scheme: "http", Host: "127.0.0.1:1"}},
		pluginID:          pluginID,
		pluginName:        pluginName,
		cmd:               &exec.Cmd{},
		output:            newRuntimeProcessLogBuffer(64),
		backoff:           time.Minute,
		exited:            make(chan struct{}),
		startedAt:         time.Now().UTC(),
	}
}

func containsWave10Env(env []string, value string) bool {
	for _, entry := range env {
		if entry == value {
			return true
		}
	}
	return false
}

func countWave10Env(env []string, prefix string) int {
	count := 0
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			count++
		}
	}
	return count
}
