package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestRuntimeHTTPClientStatus(t *testing.T) {
	var nilClient *runtimeHTTPClient
	status := nilClient.status()

	if status.Runtime != BackendRuntimeHTTP {
		t.Fatalf("nil status runtime = %q, want %q", status.Runtime, BackendRuntimeHTTP)
	}
	if status.State != RuntimeStateExternal {
		t.Fatalf("nil status state = %q, want %q", status.State, RuntimeStateExternal)
	}
	if status.Health != RuntimeHealthUnknown {
		t.Fatalf("nil status health = %q, want %q", status.Health, RuntimeHealthUnknown)
	}
	if status.BaseURL != "" {
		t.Fatalf("nil status base URL = %q, want empty", status.BaseURL)
	}

	baseURL := &url.URL{Scheme: "http", Host: "127.0.0.1:4321"}
	client := &runtimeHTTPClient{baseURL: baseURL}
	status = client.status()

	if status.BaseURL != "http://127.0.0.1:4321" {
		t.Fatalf("status base URL = %q, want http://127.0.0.1:4321", status.BaseURL)
	}
	if status.Message != "external HTTP runtime is operator-managed" {
		t.Fatalf("status message = %q, want operator-managed message", status.Message)
	}
}

func TestRuntimeHTTPClientClose(t *testing.T) {
	var client *runtimeHTTPClient
	if err := client.close(context.Background()); err != nil {
		t.Fatalf("close() error = %v, want nil", err)
	}
}

func TestNewRuntimeHTTPClientAndParseLoopbackBaseURL(t *testing.T) {
	client, err := newRuntimeHTTPClient(nil)
	if err != nil {
		t.Fatalf("newRuntimeHTTPClient(nil) error = %v, want nil", err)
	}
	if client != nil {
		t.Fatalf("newRuntimeHTTPClient(nil) = %+v, want nil", client)
	}

	client, err = newRuntimeHTTPClient(&BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: " http://127.0.0.1:8123/runtime?ignored=true#frag "})
	if err != nil {
		t.Fatalf("newRuntimeHTTPClient(valid) error = %v", err)
	}
	if got := client.baseURL.String(); got != "http://127.0.0.1:8123/runtime" {
		t.Fatalf("base URL = %q, want query and fragment stripped", got)
	}

	tests := []struct {
		name    string
		backend *BackendConfig
		wantErr error
	}{
		{
			name:    "empty runtime",
			backend: &BackendConfig{},
			wantErr: ErrPluginRuntimeUnavailable,
		},
		{
			name:    "unsupported runtime",
			backend: &BackendConfig{Runtime: "docker", BaseURL: "http://127.0.0.1:8123"},
			wantErr: ErrPluginRuntimeUnsupported,
		},
		{
			name:    "invalid URL",
			backend: &BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: "http://%"},
		},
		{
			name:    "missing host",
			backend: &BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: "http:///runtime"},
		},
		{
			name:    "non HTTP scheme",
			backend: &BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: "https://127.0.0.1:8123"},
		},
		{
			name:    "non loopback host",
			backend: &BackendConfig{Runtime: BackendRuntimeHTTP, BaseURL: "http://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newRuntimeHTTPClient(tt.backend)
			if err == nil {
				t.Fatalf("newRuntimeHTTPClient() error = nil, got client %+v", got)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuntimeHTTPClientInvokeHookAndRoute(t *testing.T) {
	pluginID := uuid.New()
	tenantID := uuid.New()
	var sawRoute bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/runtime/hooks/invoice":
			if r.Method != http.MethodPost {
				t.Errorf("hook method = %s, want POST", r.Method)
			}
			if got := r.Header.Get("X-Open-Accounting-Plugin-ID"); got != pluginID.String() {
				t.Errorf("hook plugin id header = %q, want %q", got, pluginID)
			}
			var payload runtimeHookPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decode hook payload: %v", err)
			}
			if payload.Handler != "/hooks/invoice" || payload.Event.Type != EventInvoiceCreated {
				t.Errorf("hook payload = %+v, want handler and event", payload)
			}
			w.WriteHeader(http.StatusAccepted)
		case "/runtime/hooks/fail":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, "failed")
		case "/runtime/routes/status":
			sawRoute = true
			if r.Method != http.MethodPut {
				t.Errorf("route method = %s, want PUT", r.Method)
			}
			if r.URL.RawQuery != "debug=true" {
				t.Errorf("route raw query = %q, want debug=true", r.URL.RawQuery)
			}
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("authorization header leaked to runtime: %q", got)
			}
			if got := r.Header.Get("Connection"); got != "" {
				t.Errorf("connection header leaked to runtime: %q", got)
			}
			if got := r.Header.Values("X-Request-ID"); len(got) != 2 {
				t.Errorf("X-Request-ID values = %v, want two copied values", got)
			}
			if got := r.Header.Get("X-Open-Accounting-Tenant-ID"); got != tenantID.String() {
				t.Errorf("tenant id header = %q, want %q", got, tenantID)
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != "request body" {
				t.Errorf("route body = %q, want request body", body)
			}
			w.Header().Set("X-Runtime-Result", "ok")
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, "route ok")
		case "/runtime/routes/large":
			_, _ = io.WriteString(w, strings.Repeat("x", maxRuntimeBodySize+1))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseURL, err := parseLoopbackRuntimeBaseURL(server.URL + "/runtime?drop=true#fragment")
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	client := &runtimeHTTPClient{baseURL: baseURL, client: server.Client()}

	if got := client.handlerURL("", "debug=true"); got != server.URL+"/runtime/?debug=true" {
		t.Fatalf("empty handler URL = %q, want runtime root with query", got)
	}

	cloned := cloneRuntimeResponseHeaders(http.Header{
		"Te":       {"trailers"},
		"X-Result": {"one", "two"},
	})
	if got := cloned.Get("Te"); got != "" {
		t.Fatalf("hop-by-hop TE header copied: %q", got)
	}
	if got := cloned.Values("X-Result"); len(got) != 2 {
		t.Fatalf("X-Result values = %v, want two values", got)
	}

	err = client.invokeHook(context.Background(), pluginID, "invoice-plugin", HookConfig{Handler: "/hooks/invoice"}, Event{
		Type: EventInvoiceCreated,
		Data: json.RawMessage(`{`),
	})
	if err == nil {
		t.Fatal("expected invokeHook JSON marshal error")
	}

	err = client.invokeHook(context.Background(), pluginID, "invoice-plugin", HookConfig{Handler: "/hooks/invoice"}, Event{Type: EventInvoiceCreated})
	if err != nil {
		t.Fatalf("invokeHook(success) error = %v", err)
	}
	err = client.invokeHook(context.Background(), pluginID, "invoice-plugin", HookConfig{Handler: "/hooks/fail"}, Event{Type: EventInvoiceCreated})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("invokeHook(fail) error = %v, want status 500", err)
	}

	resp, err := client.invokeRoute(
		context.Background(),
		pluginID,
		tenantID,
		RouteConfig{Handler: "routes/status"},
		http.MethodPut,
		"/status",
		"debug=true",
		http.Header{
			"Authorization": {"Bearer secret"},
			"Connection":    {"keep-alive"},
			"X-Request-ID":  {"req-1", "req-2"},
		},
		bytes.NewBufferString("request body"),
	)
	if err != nil {
		t.Fatalf("invokeRoute(success) error = %v", err)
	}
	if !sawRoute {
		t.Fatal("runtime route was not invoked")
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if string(resp.Body) != "route ok" {
		t.Fatalf("body = %q, want route ok", resp.Body)
	}
	if got := resp.Header.Get("X-Runtime-Result"); got != "ok" {
		t.Fatalf("runtime result header = %q, want ok", got)
	}
	if got := resp.Header.Get("Connection"); got != "" {
		t.Fatalf("hop-by-hop response header was copied: %q", got)
	}

	_, err = client.invokeRoute(context.Background(), pluginID, tenantID, RouteConfig{Handler: "/routes/status"}, http.MethodPost, "/status", "", nil, strings.NewReader(strings.Repeat("x", maxRuntimeBodySize+1)))
	if err == nil || !strings.Contains(err.Error(), "request body exceeds") {
		t.Fatalf("invokeRoute(large request) error = %v, want request size error", err)
	}

	_, err = client.invokeRoute(context.Background(), pluginID, tenantID, RouteConfig{Handler: "/routes/status"}, http.MethodPost, "/status", "", nil, failingRuntimeReader{})
	if err == nil || !strings.Contains(err.Error(), "runtime read failed") {
		t.Fatalf("invokeRoute(read error) error = %v, want body read error", err)
	}

	_, err = client.invokeRoute(context.Background(), pluginID, tenantID, RouteConfig{Handler: "/routes/status"}, "bad\nmethod", "/status", "", nil, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected invokeRoute invalid method error")
	}

	_, err = client.invokeRoute(context.Background(), pluginID, tenantID, RouteConfig{Handler: "/routes/large"}, http.MethodGet, "/large", "", nil, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "response body exceeds") {
		t.Fatalf("invokeRoute(large response) error = %v, want response size error", err)
	}
}

type failingRuntimeReader struct{}

func (failingRuntimeReader) Read([]byte) (int, error) {
	return 0, errors.New("runtime read failed")
}
