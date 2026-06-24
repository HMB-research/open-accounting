package plugin

import (
	"context"
	"net/url"
	"testing"
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
