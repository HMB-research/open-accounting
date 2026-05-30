package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/webhooks"
)

func setupWebhookHandlers(targetURL string) (*Handlers, *webhooks.Service, *memoryWebhookRepository) {
	repo := newMemoryWebhookRepository()
	service := webhooks.NewServiceWithRepository(repo, nil)
	h := &Handlers{webhookService: service}
	if targetURL != "" {
		active := true
		endpoint, _ := service.CreateEndpoint(testCtx(), "tenant-1", &webhooks.CreateEndpointRequest{
			Name:     "CRM",
			URL:      targetURL,
			Events:   []string{"invoice.created"},
			Secret:   "secret",
			IsActive: &active,
		})
		repo.defaultEndpointID = endpoint.ID
	}
	return h, service, repo
}

func TestWebhookHandlers(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "webhook.test", r.Header.Get("X-Open-Accounting-Event"))
		assert.NotEmpty(t, r.Header.Get("X-Open-Accounting-Signature"))
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer target.Close()

	h, _, repo := setupWebhookHandlers(target.URL)

	t.Run("list event types", func(t *testing.T) {
		req := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/webhooks/events", nil, nil), map[string]string{"tenantID": "tenant-1"})
		w := httptest.NewRecorder()

		h.ListWebhookEventTypes(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var events []string
		require.NoError(t, json.NewDecoder(w.Body).Decode(&events))
		assert.Contains(t, events, "invoice.created")
		assert.Contains(t, events, "webhook.test")
	})

	t.Run("create and list endpoint", func(t *testing.T) {
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/webhooks", webhooks.CreateEndpointRequest{
			Name:   "Accounting Bot",
			URL:    target.URL,
			Events: []string{"payment.received"},
			Secret: "hidden",
		}, nil), map[string]string{"tenantID": "tenant-1"})
		w := httptest.NewRecorder()

		h.CreateWebhookEndpoint(w, req)

		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		var endpoint webhooks.Endpoint
		require.NoError(t, json.NewDecoder(w.Body).Decode(&endpoint))
		assert.Equal(t, "Accounting Bot", endpoint.Name)
		assert.True(t, endpoint.SecretSet)
		assert.Empty(t, endpoint.Secret)

		req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/webhooks", nil, nil), map[string]string{"tenantID": "tenant-1"})
		w = httptest.NewRecorder()
		h.ListWebhookEndpoints(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var endpoints []webhooks.Endpoint
		require.NoError(t, json.NewDecoder(w.Body).Decode(&endpoints))
		assert.Len(t, endpoints, 2)
	})

	t.Run("test delivery and list attempts", func(t *testing.T) {
		req := withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/webhooks/"+repo.defaultEndpointID+"/test", webhooks.TestDeliveryRequest{}, nil), map[string]string{
			"tenantID":  "tenant-1",
			"webhookID": repo.defaultEndpointID,
		})
		w := httptest.NewRecorder()

		h.TestWebhookEndpoint(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var result webhooks.DeliveryResult
		require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
		assert.Equal(t, "webhook.test", result.Event.Type)
		require.Len(t, result.Deliveries, 1)
		assert.Equal(t, webhooks.DeliveryStatusSucceeded, result.Deliveries[0].Status)

		req = withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/webhooks/"+repo.defaultEndpointID+"/deliveries?limit=10", nil, nil), map[string]string{
			"tenantID":  "tenant-1",
			"webhookID": repo.defaultEndpointID,
		})
		w = httptest.NewRecorder()
		h.ListWebhookDeliveries(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		var deliveries []webhooks.Delivery
		require.NoError(t, json.NewDecoder(w.Body).Decode(&deliveries))
		assert.NotEmpty(t, deliveries)
	})

	t.Run("delete endpoint", func(t *testing.T) {
		req := withURLParams(makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/webhooks/"+repo.defaultEndpointID, nil, nil), map[string]string{
			"tenantID":  "tenant-1",
			"webhookID": repo.defaultEndpointID,
		})
		w := httptest.NewRecorder()

		h.DeleteWebhookEndpoint(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func testCtx() context.Context {
	return context.Background()
}

type memoryWebhookRepository struct {
	endpoints         map[string]*webhooks.Endpoint
	deliveries        []webhooks.Delivery
	defaultEndpointID string
}

func newMemoryWebhookRepository() *memoryWebhookRepository {
	return &memoryWebhookRepository{endpoints: make(map[string]*webhooks.Endpoint)}
}

func (r *memoryWebhookRepository) ListEndpoints(_ context.Context, tenantID string, activeOnly bool) ([]webhooks.Endpoint, error) {
	var endpoints []webhooks.Endpoint
	for _, endpoint := range r.endpoints {
		if endpoint.TenantID != tenantID {
			continue
		}
		if activeOnly && !endpoint.IsActive {
			continue
		}
		copyEndpoint := *endpoint
		endpoints = append(endpoints, copyEndpoint)
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Name < endpoints[j].Name
	})
	return endpoints, nil
}

func (r *memoryWebhookRepository) GetEndpoint(_ context.Context, tenantID, endpointID string) (*webhooks.Endpoint, error) {
	endpoint, ok := r.endpoints[endpointID]
	if !ok || endpoint.TenantID != tenantID {
		return nil, fmt.Errorf("webhook endpoint not found")
	}
	copyEndpoint := *endpoint
	return &copyEndpoint, nil
}

func (r *memoryWebhookRepository) CreateEndpoint(_ context.Context, endpoint *webhooks.Endpoint) error {
	copyEndpoint := *endpoint
	r.endpoints[endpoint.ID] = &copyEndpoint
	return nil
}

func (r *memoryWebhookRepository) UpdateEndpoint(_ context.Context, endpoint *webhooks.Endpoint) error {
	copyEndpoint := *endpoint
	r.endpoints[endpoint.ID] = &copyEndpoint
	return nil
}

func (r *memoryWebhookRepository) DeleteEndpoint(_ context.Context, tenantID, endpointID string) (int64, error) {
	endpoint, ok := r.endpoints[endpointID]
	if !ok || endpoint.TenantID != tenantID {
		return 0, nil
	}
	delete(r.endpoints, endpointID)
	return 1, nil
}

func (r *memoryWebhookRepository) CreateDelivery(_ context.Context, delivery *webhooks.Delivery) error {
	r.deliveries = append([]webhooks.Delivery{*delivery}, r.deliveries...)
	return nil
}

func (r *memoryWebhookRepository) ListDeliveries(_ context.Context, tenantID, endpointID string, limit int) ([]webhooks.Delivery, error) {
	var deliveries []webhooks.Delivery
	for _, delivery := range r.deliveries {
		if delivery.TenantID == tenantID && delivery.EndpointID == endpointID {
			deliveries = append(deliveries, delivery)
		}
	}
	if limit > 0 && len(deliveries) > limit {
		deliveries = deliveries[:limit]
	}
	return deliveries, nil
}

func (r *memoryWebhookRepository) UpdateEndpointLastDelivery(_ context.Context, tenantID, endpointID string, deliveredAt time.Time) error {
	endpoint, ok := r.endpoints[endpointID]
	if !ok || endpoint.TenantID != tenantID {
		return fmt.Errorf("webhook endpoint not found")
	}
	endpoint.LastDeliveryAt = &deliveredAt
	return nil
}
