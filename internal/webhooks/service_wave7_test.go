package webhooks

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/plugin"
)

func TestWebhooksWave7NewServiceNilPoolUsesDefaults(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Fatal("NewService(nil) returned nil")
	}
	if service.repo != nil {
		t.Fatalf("NewService(nil).repo = %#v, want nil", service.repo)
	}
	if service.httpClient == nil || service.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("NewService(nil).httpClient = %#v, want default timeout", service.httpClient)
	}

	withDefaultClient := NewServiceWithRepository(newMemoryRepository(), nil)
	if withDefaultClient.httpClient == nil || withDefaultClient.httpClient.Timeout != defaultHTTPTimeout {
		t.Fatalf("NewServiceWithRepository(nil client).httpClient = %#v, want default timeout", withDefaultClient.httpClient)
	}
	customClient := &http.Client{Timeout: time.Second}
	withCustomClient := NewServiceWithRepository(newMemoryRepository(), customClient)
	if withCustomClient.httpClient != customClient {
		t.Fatalf("NewServiceWithRepository(custom).httpClient = %#v, want custom client", withCustomClient.httpClient)
	}
}

func TestWebhooksWave7DispatchSkipsUnsubscribedEndpoints(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime
	_, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Payments only",
		URL:    "https://example.com/hooks",
		Events: []string{plugin.EventPaymentReceived},
	})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}

	result, err := service.Dispatch(context.Background(), Event{
		Type:     plugin.EventInvoiceCreated,
		TenantID: "tenant-1",
		Data:     []byte(`{"invoice_id":"inv-1"}`),
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(result.Deliveries) != 0 {
		t.Fatalf("Dispatch() deliveries = %#v, want none for unsubscribed endpoint", result.Deliveries)
	}
}
