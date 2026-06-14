package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	packageRuntimeHealthPath      = "/__open_accounting/health"
	packageRuntimeStartupTimeout  = 2 * time.Second
	packageRuntimeProbeTimeout    = 200 * time.Millisecond
	packageRuntimeProbeInterval   = 25 * time.Millisecond
	packageRuntimeShutdownTimeout = 2 * time.Second
	packageRuntimeLogLimit        = 4096
)

type pluginBackendRuntime interface {
	invokeHook(ctx context.Context, pluginID uuid.UUID, pluginName string, hook HookConfig, event Event) error
	invokeRoute(ctx context.Context, pluginID uuid.UUID, tenantID uuid.UUID, route RouteConfig, method string, requestPath string, rawQuery string, sourceHeader http.Header, body io.Reader) (*RuntimeRouteResponse, error)
	close(ctx context.Context) error
}

func (c *runtimeHTTPClient) close(context.Context) error {
	return nil
}

type packageRuntimeProcess struct {
	*runtimeHTTPClient

	pluginName string
	cmd        *exec.Cmd
	output     *runtimeProcessLogBuffer

	exited  chan struct{}
	exitMu  sync.RWMutex
	exitErr error

	stopOnce sync.Once
	stopErr  error
}

func (s *Service) startPackageBackendRuntime(plugin *Plugin, manifest *Manifest) (*packageRuntimeProcess, error) {
	if plugin == nil {
		return nil, fmt.Errorf("%w: package runtime plugin is nil", ErrPluginRuntimeUnavailable)
	}
	if manifest == nil || manifest.Backend == nil {
		return nil, nil
	}
	if err := validatePackageBackendRuntime(manifest.Backend); err != nil {
		return nil, err
	}

	pluginRoot := s.getPluginPath(plugin.Name)
	if pluginRoot == "" {
		return nil, fmt.Errorf("%w: package runtime plugin %q is not installed under the plugin directory", ErrPluginRuntimeUnavailable, plugin.Name)
	}

	backendDir, executablePath, err := resolvePackageRuntimeExecutable(pluginRoot, manifest.Backend)
	if err != nil {
		return nil, err
	}

	listenAddr, err := reserveLoopbackRuntimeAddress()
	if err != nil {
		return nil, fmt.Errorf("%w: reserve package runtime loopback address: %v", ErrPluginRuntimeUnavailable, err)
	}
	baseURL, err := parseLoopbackRuntimeBaseURL("http://" + listenAddr)
	if err != nil {
		return nil, err
	}

	output := newRuntimeProcessLogBuffer(packageRuntimeLogLimit)
	cmd := exec.CommandContext(context.Background(), executablePath)
	cmd.Dir = backendDir
	cmd.Env = append(os.Environ(),
		"OPEN_ACCOUNTING_PLUGIN_ID="+plugin.ID.String(),
		"OPEN_ACCOUNTING_PLUGIN_NAME="+plugin.Name,
		"OPEN_ACCOUNTING_RUNTIME_ADDR="+listenAddr,
		"OPEN_ACCOUNTING_RUNTIME_BASE_URL="+baseURL.String(),
		"OPEN_ACCOUNTING_RUNTIME_HEALTH_PATH="+packageRuntimeHealthPath,
	)
	cmd.Stdout = output
	cmd.Stderr = output

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: start package runtime for plugin %q: %v", ErrPluginRuntimeUnavailable, plugin.Name, err)
	}

	runtime := &packageRuntimeProcess{
		runtimeHTTPClient: &runtimeHTTPClient{
			baseURL: baseURL,
			client:  &http.Client{Timeout: 10 * time.Second},
		},
		pluginName: plugin.Name,
		cmd:        cmd,
		output:     output,
		exited:     make(chan struct{}),
	}
	go runtime.wait()

	startupCtx, cancel := context.WithTimeout(context.Background(), packageRuntimeStartupTimeout)
	defer cancel()
	if err := runtime.waitForReady(startupCtx); err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), packageRuntimeShutdownTimeout)
		_ = runtime.close(stopCtx)
		stopCancel()
		return nil, err
	}

	return runtime, nil
}

func resolvePackageRuntimeExecutable(pluginRoot string, backend *BackendConfig) (string, string, error) {
	absPluginRoot, err := filepath.Abs(pluginRoot)
	if err != nil {
		return "", "", fmt.Errorf("invalid plugin path: %w", err)
	}
	realPluginRoot, err := filepath.EvalSymlinks(absPluginRoot)
	if err != nil {
		return "", "", fmt.Errorf("%w: plugin directory is not accessible: %v", ErrPluginRuntimeUnavailable, err)
	}

	backendDir := filepath.Join(realPluginRoot, filepath.Clean(strings.TrimSpace(backend.Package)))
	realBackendDir, err := filepath.EvalSymlinks(backendDir)
	if err != nil {
		return "", "", fmt.Errorf("%w: backend.package %q is not accessible: %v", ErrPluginRuntimeUnavailable, backend.Package, err)
	}
	if !pathWithinDir(realPluginRoot, realBackendDir, true) {
		return "", "", fmt.Errorf("backend.package must stay within the plugin package")
	}

	executablePath := filepath.Join(realBackendDir, filepath.Clean(strings.TrimSpace(backend.Executable)))
	realExecutablePath, err := filepath.EvalSymlinks(executablePath)
	if err != nil {
		return "", "", fmt.Errorf("%w: backend.executable %q is not accessible: %v", ErrPluginRuntimeUnavailable, backend.Executable, err)
	}
	if !pathWithinDir(realBackendDir, realExecutablePath, false) {
		return "", "", fmt.Errorf("backend.executable must stay within the plugin package")
	}

	info, err := os.Stat(realExecutablePath)
	if err != nil {
		return "", "", fmt.Errorf("%w: backend.executable %q is not accessible: %v", ErrPluginRuntimeUnavailable, backend.Executable, err)
	}
	if !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("backend.executable must point to a regular file")
	}
	if info.Mode().Perm()&0111 == 0 {
		return "", "", fmt.Errorf("backend.executable must point to an executable file")
	}

	return realBackendDir, realExecutablePath, nil
}

func pathWithinDir(parent, child string, allowEqual bool) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return allowEqual
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func reserveLoopbackRuntimeAddress() (string, error) {
	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = listener.Close()
	}()
	return listener.Addr().String(), nil
}

func (r *packageRuntimeProcess) wait() {
	err := r.cmd.Wait()
	r.exitMu.Lock()
	r.exitErr = err
	r.exitMu.Unlock()
	close(r.exited)
}

func (r *packageRuntimeProcess) waitForReady(ctx context.Context) error {
	probeClient := &http.Client{Timeout: packageRuntimeProbeTimeout}
	healthURL := r.handlerURL(packageRuntimeHealthPath, "")
	var lastProbe error

	for {
		select {
		case <-r.exited:
			return fmt.Errorf("%w: package runtime for plugin %q exited before exposing %s: %v%s", ErrPluginRuntimeUnavailable, r.pluginName, packageRuntimeHealthPath, r.getExitErr(), r.outputSuffix())
		case <-ctx.Done():
			return fmt.Errorf("%w: package runtime for plugin %q did not expose loopback health endpoint %s within %s (last probe: %v)%s", ErrPluginRuntimeUnavailable, r.pluginName, packageRuntimeHealthPath, packageRuntimeStartupTimeout, lastProbe, r.outputSuffix())
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			return err
		}
		resp, err := probeClient.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxRuntimeBodySize))
			_ = resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				return nil
			}
			lastProbe = fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
		} else {
			lastProbe = err
		}

		timer := time.NewTimer(packageRuntimeProbeInterval)
		select {
		case <-r.exited:
			timer.Stop()
			return fmt.Errorf("%w: package runtime for plugin %q exited before exposing %s: %v%s", ErrPluginRuntimeUnavailable, r.pluginName, packageRuntimeHealthPath, r.getExitErr(), r.outputSuffix())
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%w: package runtime for plugin %q did not expose loopback health endpoint %s within %s (last probe: %v)%s", ErrPluginRuntimeUnavailable, r.pluginName, packageRuntimeHealthPath, packageRuntimeStartupTimeout, lastProbe, r.outputSuffix())
		case <-timer.C:
		}
	}
}

func (r *packageRuntimeProcess) close(ctx context.Context) error {
	r.stopOnce.Do(func() {
		r.stopErr = r.stop(ctx)
	})
	return r.stopErr
}

func (r *packageRuntimeProcess) stop(ctx context.Context) error {
	select {
	case <-r.exited:
		return r.getExitErr()
	default:
	}

	signaled := false
	if r.cmd.Process != nil {
		if err := r.cmd.Process.Signal(os.Interrupt); err == nil {
			signaled = true
		} else if !errors.Is(err, os.ErrProcessDone) {
			_ = r.cmd.Process.Kill()
		}
	}

	select {
	case <-r.exited:
		if signaled {
			return nil
		}
		return r.getExitErr()
	case <-ctx.Done():
		if r.cmd.Process != nil {
			_ = r.cmd.Process.Kill()
		}
		select {
		case <-r.exited:
		case <-time.After(packageRuntimeShutdownTimeout):
		}
		return ctx.Err()
	}
}

func (r *packageRuntimeProcess) getExitErr() error {
	r.exitMu.RLock()
	defer r.exitMu.RUnlock()
	return r.exitErr
}

func (r *packageRuntimeProcess) outputSuffix() string {
	output := r.output.String()
	if output == "" {
		return ""
	}
	return "; runtime output: " + output
}

type runtimeProcessLogBuffer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	limit int
}

func newRuntimeProcessLogBuffer(limit int) *runtimeProcessLogBuffer {
	return &runtimeProcessLogBuffer{limit: limit}
}

func (b *runtimeProcessLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data := append(append([]byte(nil), b.buf.Bytes()...), p...)
	if b.limit > 0 && len(data) > b.limit {
		data = data[len(data)-b.limit:]
	}
	b.buf.Reset()
	_, _ = b.buf.Write(data)
	return len(p), nil
}

func (b *runtimeProcessLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(b.buf.String())
}
