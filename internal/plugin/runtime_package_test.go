package plugin

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResolvePackageRuntimeExecutable(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		pluginRoot := t.TempDir()
		executablePath := writeRuntimeExecutable(t, pluginRoot, 0750)

		backendDir, executable, err := resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{
			Package:    "backend",
			Executable: "bin/runtime",
		})

		if err != nil {
			t.Fatalf("resolvePackageRuntimeExecutable() error = %v", err)
		}
		if backendDir != filepath.Dir(filepath.Dir(executablePath)) {
			t.Fatalf("backend dir = %q, want %q", backendDir, filepath.Dir(filepath.Dir(executablePath)))
		}
		if executable != executablePath {
			t.Fatalf("executable = %q, want %q", executable, executablePath)
		}
	})

	t.Run("missing plugin root", func(t *testing.T) {
		_, _, err := resolvePackageRuntimeExecutable(filepath.Join(t.TempDir(), "missing"), &BackendConfig{Package: "backend", Executable: "bin/runtime"})
		if err == nil || !strings.Contains(err.Error(), "plugin directory is not accessible") {
			t.Fatalf("error = %v, want inaccessible plugin directory", err)
		}
	})

	t.Run("package symlink escapes plugin", func(t *testing.T) {
		pluginRoot := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(pluginRoot, "backend")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		_, _, err := resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{Package: "backend", Executable: "runtime"})
		if err == nil || !strings.Contains(err.Error(), "backend.package must stay within") {
			t.Fatalf("error = %v, want package escape error", err)
		}
	})

	t.Run("executable symlink escapes backend", func(t *testing.T) {
		pluginRoot := t.TempDir()
		backendBin := filepath.Join(pluginRoot, "backend", "bin")
		if err := os.MkdirAll(backendBin, 0750); err != nil {
			t.Fatalf("create backend dir: %v", err)
		}
		outsideRuntime := filepath.Join(t.TempDir(), "runtime")
		if err := os.WriteFile(outsideRuntime, []byte("#!/bin/sh\n"), 0750); err != nil {
			t.Fatalf("write outside runtime: %v", err)
		}
		if err := os.Symlink(outsideRuntime, filepath.Join(backendBin, "runtime")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		_, _, err := resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{Package: "backend", Executable: "bin/runtime"})
		if err == nil || !strings.Contains(err.Error(), "backend.executable must stay within") {
			t.Fatalf("error = %v, want executable escape error", err)
		}
	})

	t.Run("executable must be a regular executable file", func(t *testing.T) {
		pluginRoot := t.TempDir()
		backendBin := filepath.Join(pluginRoot, "backend", "bin")
		if err := os.MkdirAll(filepath.Join(backendBin, "runtime"), 0750); err != nil {
			t.Fatalf("create runtime dir: %v", err)
		}
		_, _, err := resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{Package: "backend", Executable: "bin/runtime"})
		if err == nil || !strings.Contains(err.Error(), "regular file") {
			t.Fatalf("error = %v, want regular file error", err)
		}

		pluginRoot = t.TempDir()
		writeRuntimeExecutable(t, pluginRoot, 0600)
		_, _, err = resolvePackageRuntimeExecutable(pluginRoot, &BackendConfig{Package: "backend", Executable: "bin/runtime"})
		if err == nil || !strings.Contains(err.Error(), "executable file") {
			t.Fatalf("error = %v, want executable bit error", err)
		}
	})
}

func TestPackageRuntimeEnvironmentFiltersHostVariables(t *testing.T) {
	pluginID := uuid.New()
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("DATABASE_URL", "postgres://secret")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("PATH", os.Getenv("PATH"))

	env := packageRuntimeEnvironment(&Plugin{ID: pluginID, Name: "runtime-plugin"}, "127.0.0.1:1234", "http://127.0.0.1:1234")
	values := map[string]string{}
	for _, kv := range env {
		key, value, ok := strings.Cut(kv, "=")
		if ok {
			values[key] = value
		}
	}

	if values["OPEN_ACCOUNTING_PLUGIN_ID"] != pluginID.String() {
		t.Fatalf("plugin id env = %q, want %q", values["OPEN_ACCOUNTING_PLUGIN_ID"], pluginID)
	}
	if values["OPEN_ACCOUNTING_PLUGIN_NAME"] != "runtime-plugin" {
		t.Fatalf("plugin name env = %q, want runtime-plugin", values["OPEN_ACCOUNTING_PLUGIN_NAME"])
	}
	if values["OPEN_ACCOUNTING_RUNTIME_ADDR"] != "127.0.0.1:1234" {
		t.Fatalf("runtime addr env = %q, want 127.0.0.1:1234", values["OPEN_ACCOUNTING_RUNTIME_ADDR"])
	}
	if _, ok := values["AWS_SECRET_ACCESS_KEY"]; ok {
		t.Fatal("AWS_SECRET_ACCESS_KEY leaked into package runtime environment")
	}
	if _, ok := values["DATABASE_URL"]; ok {
		t.Fatal("DATABASE_URL leaked into package runtime environment")
	}
	if _, ok := values["JWT_SECRET"]; ok {
		t.Fatal("JWT_SECRET leaked into package runtime environment")
	}
	if _, ok := values["PATH"]; !ok {
		t.Fatal("PATH should be preserved for package runtime lookup")
	}
}

func TestStartPackageBackendRuntimeWithStatsGuards(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository(), nil, t.TempDir())

	runtime, err := service.startPackageBackendRuntimeWithStats(nil, &Manifest{}, packageRuntimeStats{})
	if err == nil || runtime != nil || !errors.Is(err, ErrPluginRuntimeUnavailable) {
		t.Fatalf("nil plugin runtime = %+v, %v; want unavailable error", runtime, err)
	}

	runtime, err = service.startPackageBackendRuntimeWithStats(&Plugin{Name: "frontend-only"}, nil, packageRuntimeStats{})
	if err != nil || runtime != nil {
		t.Fatalf("nil manifest runtime = %+v, %v; want nil nil", runtime, err)
	}

	runtime, err = service.startPackageBackendRuntimeWithStats(&Plugin{Name: "frontend-only"}, &Manifest{}, packageRuntimeStats{})
	if err != nil || runtime != nil {
		t.Fatalf("manifest without backend runtime = %+v, %v; want nil nil", runtime, err)
	}

	runtime, err = service.startPackageBackendRuntimeWithStats(&Plugin{Name: "bad-runtime"}, &Manifest{
		Backend: &BackendConfig{Runtime: BackendRuntimePackage, BaseURL: "http://127.0.0.1:3000", Package: "backend", Executable: "bin/runtime"},
	}, packageRuntimeStats{})
	if err == nil || runtime != nil || !strings.Contains(err.Error(), "backend.base_url") {
		t.Fatalf("invalid backend runtime = %+v, %v; want backend.base_url error", runtime, err)
	}

	runtime, err = service.startPackageBackendRuntimeWithStats(&Plugin{Name: "missing-plugin"}, &Manifest{
		Backend: &BackendConfig{Runtime: BackendRuntimePackage, Package: "backend", Executable: "bin/runtime"},
	}, packageRuntimeStats{})
	if err == nil || runtime != nil || !errors.Is(err, ErrPluginRuntimeUnavailable) || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("missing plugin runtime = %+v, %v; want not installed unavailable error", runtime, err)
	}
}

func TestPackageRuntimeWaitForReady(t *testing.T) {
	t.Run("healthy endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != packageRuntimeHealthPath {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()
		baseURL, err := parseLoopbackRuntimeBaseURL(server.URL)
		if err != nil {
			t.Fatalf("parse server URL: %v", err)
		}
		runtime := &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: baseURL},
			pluginName:        "ready-plugin",
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.waitForReady(ctx); err != nil {
			t.Fatalf("waitForReady() error = %v", err)
		}
		status := runtime.status()
		if status.LastHealthError != "" || status.LastHealthCheckAt == nil {
			t.Fatalf("status after ready probe = %+v", status)
		}
	})

	t.Run("exited before ready", func(t *testing.T) {
		baseURL, err := parseLoopbackRuntimeBaseURL("http://127.0.0.1:1")
		if err != nil {
			t.Fatalf("parse base URL: %v", err)
		}
		runtime := &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: baseURL},
			pluginName:        "exited-plugin",
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
		}
		runtime.exitErr = errors.New("exit status 42")
		close(runtime.exited)

		err = runtime.waitForReady(context.Background())
		if err == nil || !errors.Is(err, ErrPluginRuntimeUnavailable) || !strings.Contains(err.Error(), "exit status 42") {
			t.Fatalf("waitForReady exited error = %v, want exit status unavailable error", err)
		}
	})

	t.Run("context timeout records last probe", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()
		baseURL, err := parseLoopbackRuntimeBaseURL(server.URL)
		if err != nil {
			t.Fatalf("parse server URL: %v", err)
		}
		runtime := &packageRuntimeProcess{
			runtimeHTTPClient: &runtimeHTTPClient{baseURL: baseURL},
			pluginName:        "timeout-plugin",
			output:            newRuntimeProcessLogBuffer(64),
			exited:            make(chan struct{}),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
		defer cancel()
		err = runtime.waitForReady(ctx)
		if err == nil || !errors.Is(err, ErrPluginRuntimeUnavailable) || !strings.Contains(err.Error(), "last probe") {
			t.Fatalf("waitForReady timeout error = %v, want last probe unavailable error", err)
		}
		if status := runtime.status(); !strings.Contains(status.LastHealthError, "status 503") {
			t.Fatalf("last health error = %q, want status 503", status.LastHealthError)
		}
	})
}

func TestPackageRuntimeProcessStatusTransitions(t *testing.T) {
	runtimeURL := &url.URL{Scheme: "http", Host: "127.0.0.1:4567"}
	runtime := &packageRuntimeProcess{
		runtimeHTTPClient: &runtimeHTTPClient{baseURL: runtimeURL},
		pluginID:          uuid.New(),
		pluginName:        "runtime-plugin",
		cmd:               &exec.Cmd{},
		output:            newRuntimeProcessLogBuffer(64),
		backoff:           time.Minute,
		exited:            make(chan struct{}),
		startedAt:         time.Now().UTC(),
	}

	status := runtime.status()
	if status.State != RuntimeStateStarting {
		t.Fatalf("initial state = %s, want starting", status.State)
	}
	if err := runtime.requireRunning(); err == nil || !strings.Contains(err.Error(), "starting") {
		t.Fatalf("requireRunning while starting = %v, want starting error", err)
	}
	if err := runtime.invokeHook(context.Background(), uuid.New(), "runtime-plugin", HookConfig{}, Event{}); err == nil || !strings.Contains(err.Error(), "starting") {
		t.Fatalf("invokeHook while starting = %v, want starting error", err)
	}
	if resp, err := runtime.invokeRoute(context.Background(), uuid.New(), uuid.New(), RouteConfig{}, http.MethodGet, "/", "", nil, nil); err == nil || resp != nil || !strings.Contains(err.Error(), "starting") {
		t.Fatalf("invokeRoute while starting = %+v, %v; want starting error", resp, err)
	}

	runtime.markReady()
	status = runtime.status()
	if status.State != RuntimeStateRunning || status.Health != RuntimeHealthHealthy {
		t.Fatalf("ready status = %+v, want running healthy", status)
	}
	if err := runtime.requireRunning(); err != nil {
		t.Fatalf("requireRunning after ready error = %v", err)
	}

	runtime.markHealthProbe(errors.New("health down"))
	status = runtime.status()
	if status.State != RuntimeStateRunning || status.Health != RuntimeHealthUnhealthy || status.Message != "last health probe failed" {
		t.Fatalf("health failure status = %+v", status)
	}

	_, _ = runtime.output.Write([]byte("first line\nsecond line"))
	runtime.markExited(errors.New("exit status 2"))
	status = runtime.status()
	if status.State != RuntimeStateBackoff {
		t.Fatalf("crashed state = %s, want backoff", status.State)
	}
	if status.CrashCount != 1 {
		t.Fatalf("crash count = %d, want 1", status.CrashCount)
	}
	if status.BackoffUntil == nil {
		t.Fatal("expected backoff deadline")
	}
	if status.LastOutput != "first line\nsecond line" {
		t.Fatalf("last output = %q, want trimmed runtime output", status.LastOutput)
	}
	if err := runtime.requireRunning(); err == nil || !strings.Contains(err.Error(), "until") {
		t.Fatalf("requireRunning after crash = %v, want backoff error", err)
	}

	stopped := &packageRuntimeProcess{
		runtimeHTTPClient: &runtimeHTTPClient{baseURL: runtimeURL},
		pluginID:          uuid.New(),
		pluginName:        "stopped-plugin",
		cmd:               &exec.Cmd{},
		output:            newRuntimeProcessLogBuffer(64),
		exited:            make(chan struct{}),
		startedAt:         time.Now().UTC(),
	}
	stopped.markIntentionalStop()
	stopped.markExited(nil)
	status = stopped.status()
	if status.State != RuntimeStateStopped || status.Health != RuntimeHealthNotApplicable {
		t.Fatalf("stopped status = %+v, want stopped/not applicable", status)
	}
	if err := stopped.requireRunning(); err == nil || !strings.Contains(err.Error(), "runtime stopped") {
		t.Fatalf("requireRunning stopped = %v, want stopped error", err)
	}

	alreadyExited := &packageRuntimeProcess{
		cmd:        &exec.Cmd{},
		output:     newRuntimeProcessLogBuffer(64),
		exited:     make(chan struct{}),
		pluginName: "exited-plugin",
	}
	alreadyExited.exitErr = errors.New("process already exited")
	close(alreadyExited.exited)
	if err := alreadyExited.stop(context.Background()); err == nil || !strings.Contains(err.Error(), "process already exited") {
		t.Fatalf("stop already exited = %v, want exit error", err)
	}
}

func TestRuntimeProcessLogBufferKeepsTail(t *testing.T) {
	limited := newRuntimeProcessLogBuffer(5)
	n, err := limited.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("limited Write error = %v", err)
	}
	if n != 6 {
		t.Fatalf("limited Write n = %d, want 6", n)
	}
	if got := limited.String(); got != "bcdef" {
		t.Fatalf("limited buffer = %q, want bcdef", got)
	}

	unlimited := newRuntimeProcessLogBuffer(0)
	_, _ = unlimited.Write([]byte("abcdef"))
	if got := unlimited.String(); got != "abcdef" {
		t.Fatalf("unlimited buffer = %q, want abcdef", got)
	}

	if timePtr(time.Time{}) != nil {
		t.Fatal("timePtr(zero) should be nil")
	}
	if cloneTimePtr(nil) != nil {
		t.Fatal("cloneTimePtr(nil) should be nil")
	}
	if cloneIntPtr(nil) != nil {
		t.Fatal("cloneIntPtr(nil) should be nil")
	}
}

func TestPackageRuntimePathAndAddressHelpers(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0750); err != nil {
		t.Fatalf("create child: %v", err)
	}
	sibling := filepath.Join(filepath.Dir(parent), "sibling")

	if !pathWithinDir(parent, parent, true) {
		t.Fatal("parent should be within itself when equality is allowed")
	}
	if pathWithinDir(parent, parent, false) {
		t.Fatal("parent should not be within itself when equality is disallowed")
	}
	if !pathWithinDir(parent, child, false) {
		t.Fatal("child should be within parent")
	}
	if pathWithinDir(parent, sibling, false) {
		t.Fatal("sibling should not be within parent")
	}

	addr, err := reserveLoopbackRuntimeAddress()
	if err != nil {
		t.Fatalf("reserveLoopbackRuntimeAddress() error = %v", err)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("reserved address %q did not split: %v", addr, err)
	}
	if host != "127.0.0.1" || port == "" {
		t.Fatalf("reserved address = %q, want 127.0.0.1 with dynamic port", addr)
	}

	_, err = parseLoopbackRuntimeBaseURL(" ")
	if err == nil || !strings.Contains(err.Error(), "backend.base_url is required") {
		t.Fatalf("parse blank base URL = %v, want required error", err)
	}
}

func writeRuntimeExecutable(t *testing.T, pluginRoot string, mode os.FileMode) string {
	t.Helper()
	executablePath := filepath.Join(pluginRoot, "backend", "bin", "runtime")
	if err := os.MkdirAll(filepath.Dir(executablePath), 0750); err != nil {
		t.Fatalf("create runtime dir: %v", err)
	}
	if err := os.WriteFile(executablePath, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatalf("write runtime executable: %v", err)
	}
	realPath, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		t.Fatalf("eval runtime executable: %v", err)
	}
	return realPath
}
