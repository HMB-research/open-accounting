package webhooks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/plugin"
)

type errorWebhookRepository struct {
	*memoryRepository
	createDeliveryErr     error
	updateLastDeliveryErr error
}

func (r *errorWebhookRepository) CreateDelivery(ctx context.Context, delivery *Delivery) error {
	if r.createDeliveryErr != nil {
		return r.createDeliveryErr
	}
	return r.memoryRepository.CreateDelivery(ctx, delivery)
}

func (r *errorWebhookRepository) UpdateEndpointLastDelivery(ctx context.Context, tenantID, endpointID string, deliveredAt time.Time) error {
	if r.updateLastDeliveryErr != nil {
		return r.updateLastDeliveryErr
	}
	return r.memoryRepository.UpdateEndpointLastDelivery(ctx, tenantID, endpointID, deliveredAt)
}

func TestWebhooksWave8DispatchPropagatesDeliveryPersistenceErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := &errorWebhookRepository{
		memoryRepository:  newMemoryRepository(),
		createDeliveryErr: errors.New("delivery insert failed"),
	}
	repo.endpoints["endpoint-1"] = &Endpoint{
		ID:       "endpoint-1",
		TenantID: "tenant-1",
		Name:     "CRM",
		URL:      server.URL,
		Events:   []string{plugin.EventInvoiceCreated},
		IsActive: true,
	}

	service := NewServiceWithRepository(repo, server.Client())
	service.validateTarget = func(context.Context, string) error { return nil }
	_, err := service.Dispatch(context.Background(), Event{
		Type:     plugin.EventInvoiceCreated,
		TenantID: "tenant-1",
	})
	if err == nil || !strings.Contains(err.Error(), "delivery insert failed") {
		t.Fatalf("Dispatch() error = %v, want delivery persistence failure", err)
	}
}

func TestWebhooksWave8DispatchTestPropagatesLastDeliveryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := &errorWebhookRepository{
		memoryRepository:      newMemoryRepository(),
		updateLastDeliveryErr: errors.New("last delivery failed"),
	}
	repo.endpoints["endpoint-1"] = &Endpoint{
		ID:       "endpoint-1",
		TenantID: "tenant-1",
		Name:     "CRM",
		URL:      server.URL,
		Events:   []string{plugin.EventInvoiceCreated},
		IsActive: true,
	}

	service := NewServiceWithRepository(repo, server.Client())
	service.validateTarget = func(context.Context, string) error { return nil }
	_, err := service.DispatchTest(context.Background(), "tenant-1", "endpoint-1", &TestDeliveryRequest{
		EventType: plugin.EventInvoiceCreated,
	})
	if err == nil || !strings.Contains(err.Error(), "last delivery failed") {
		t.Fatalf("DispatchTest() error = %v, want last delivery failure", err)
	}
}
