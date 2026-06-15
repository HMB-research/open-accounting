package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/plugin"
)

func TestService_CreateEndpointValidationAndSanitization(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	endpoint, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   " CRM ",
		URL:    "https://example.com/hooks",
		Events: []string{"invoice.created", "invoice.created", "payment.received"},
		Secret: "super-secret",
	})
	require.NoError(t, err)

	assert.Equal(t, "CRM", endpoint.Name)
	assert.Equal(t, []string{"invoice.created", "payment.received"}, endpoint.Events)
	assert.True(t, endpoint.SecretSet)
	assert.Empty(t, endpoint.Secret)
	assert.True(t, endpoint.IsActive)

	stored := repo.endpoints[endpoint.ID]
	assert.Equal(t, "super-secret", stored.Secret)

	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Bad",
		URL:    "ftp://example.com/hooks",
		Events: []string{"invoice.created"},
	})
	require.ErrorContains(t, err, "url must use http or https")

	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Bad",
		URL:    "https://example.com/hooks",
		Events: []string{"unknown.event"},
	})
	require.ErrorContains(t, err, "unsupported event_type")
}

func TestService_ListAndGetEndpointsSanitizeSecrets(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	active, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Active",
		URL:    "https://example.com/active",
		Events: []string{plugin.EventInvoiceCreated},
		Secret: "active-secret",
	})
	require.NoError(t, err)
	inactive := false
	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:     "Inactive",
		URL:      "https://example.com/inactive",
		Events:   []string{plugin.EventPaymentReceived},
		Secret:   "inactive-secret",
		IsActive: &inactive,
	})
	require.NoError(t, err)

	endpoints, err := service.ListEndpoints(context.Background(), "tenant-1", false)
	require.NoError(t, err)
	require.Len(t, endpoints, 2)
	for _, endpoint := range endpoints {
		assert.Empty(t, endpoint.Secret)
		assert.True(t, endpoint.SecretSet)
	}

	activeEndpoints, err := service.ListEndpoints(context.Background(), "tenant-1", true)
	require.NoError(t, err)
	require.Len(t, activeEndpoints, 1)
	assert.Equal(t, active.ID, activeEndpoints[0].ID)

	endpoint, err := service.GetEndpoint(context.Background(), "tenant-1", active.ID)
	require.NoError(t, err)
	assert.Empty(t, endpoint.Secret)
	assert.True(t, endpoint.SecretSet)
}

func TestService_UpdateEndpointValidationAndSanitization(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	endpoint, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "CRM",
		URL:    "https://example.com/hooks",
		Events: []string{plugin.EventInvoiceCreated},
		Secret: "old-secret",
	})
	require.NoError(t, err)

	updatedName := " Updated CRM "
	updatedURL := " https://example.com/updated "
	updatedSecret := "new-secret"
	inactive := false
	updated, err := service.UpdateEndpoint(context.Background(), "tenant-1", endpoint.ID, &UpdateEndpointRequest{
		Name:     &updatedName,
		URL:      &updatedURL,
		Events:   []string{plugin.EventPaymentReceived, plugin.EventPaymentReceived},
		Secret:   &updatedSecret,
		IsActive: &inactive,
	})
	require.NoError(t, err)

	assert.Equal(t, "Updated CRM", updated.Name)
	assert.Equal(t, "https://example.com/updated", updated.URL)
	assert.Equal(t, []string{plugin.EventPaymentReceived}, updated.Events)
	assert.False(t, updated.IsActive)
	assert.Empty(t, updated.Secret)
	assert.True(t, updated.SecretSet)
	assert.Equal(t, "new-secret", repo.endpoints[endpoint.ID].Secret)

	_, err = service.UpdateEndpoint(context.Background(), "tenant-1", endpoint.ID, nil)
	require.ErrorContains(t, err, "request is required")

	blankName := " "
	_, err = service.UpdateEndpoint(context.Background(), "tenant-1", endpoint.ID, &UpdateEndpointRequest{Name: &blankName})
	require.ErrorContains(t, err, "name is required")

	_, err = service.UpdateEndpoint(context.Background(), "tenant-1", endpoint.ID, &UpdateEndpointRequest{Events: []string{"unknown.event"}})
	require.ErrorContains(t, err, "unsupported event_type")
}

func TestService_DeleteEndpointAndListDeliveries(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "CRM",
		URL:    server.URL,
		Events: []string{plugin.EventInvoiceCreated},
	})
	require.NoError(t, err)

	for range 2 {
		_, err = service.DispatchTest(context.Background(), "tenant-1", endpoint.ID, &TestDeliveryRequest{
			EventType: plugin.EventInvoiceCreated,
			Payload:   json.RawMessage(`{"source":"test"}`),
		})
		require.NoError(t, err)
	}

	deliveries, err := service.ListDeliveries(context.Background(), "tenant-1", endpoint.ID, 1)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	deliveries, err = service.ListDeliveries(context.Background(), "tenant-1", endpoint.ID, 0)
	require.NoError(t, err)
	require.Len(t, deliveries, 2)

	_, err = service.ListDeliveries(context.Background(), "tenant-1", endpoint.ID, 201)
	require.ErrorContains(t, err, "limit must be between 1 and 200")

	require.NoError(t, service.DeleteEndpoint(context.Background(), "tenant-1", endpoint.ID))
	err = service.DeleteEndpoint(context.Background(), "tenant-1", endpoint.ID)
	require.ErrorContains(t, err, "webhook endpoint not found")
}

func TestService_DispatchSignedWebhookAndRecordsDelivery(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	var receivedSignature string
	var receivedEvent string
	var receivedTenant string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSignature = r.Header.Get("X-Open-Accounting-Signature")
		receivedEvent = r.Header.Get("X-Open-Accounting-Event")
		receivedTenant = r.Header.Get("X-Open-Accounting-Tenant-ID")
		var err error
		receivedBody, err = ioReadAll(r)
		require.NoError(t, err)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	endpoint, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "CRM",
		URL:    server.URL,
		Events: []string{"invoice.created"},
		Secret: "secret",
	})
	require.NoError(t, err)

	result, err := service.Dispatch(context.Background(), Event{
		ID:       "evt-1",
		Type:     "invoice.created",
		TenantID: "tenant-1",
		Data:     json.RawMessage(`{"invoice_id":"inv-1"}`),
	})
	require.NoError(t, err)
	require.Len(t, result.Deliveries, 1)

	assert.Equal(t, "invoice.created", receivedEvent)
	assert.Equal(t, "tenant-1", receivedTenant)
	assert.Equal(t, expectedSignature("secret", receivedBody), receivedSignature)
	assert.JSONEq(t, `{"id":"evt-1","type":"invoice.created","tenant_id":"tenant-1","data":{"invoice_id":"inv-1"},"created_at":"2026-01-02T03:04:05Z"}`, string(receivedBody))

	delivery := result.Deliveries[0]
	assert.Equal(t, endpoint.ID, delivery.EndpointID)
	assert.Equal(t, DeliveryStatusSucceeded, delivery.Status)
	assert.Equal(t, http.StatusAccepted, delivery.StatusCode)
	assert.Contains(t, delivery.ResponseBody, "ok")
	require.Len(t, repo.deliveries, 1)
	assert.Equal(t, DeliveryStatusSucceeded, repo.deliveries[0].Status)
	require.NotNil(t, repo.endpoints[endpoint.ID].LastDeliveryAt)
}

func TestService_DispatchSkipsUnsubscribedAndRecordsFailure(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("failed"))
	}))
	defer server.Close()

	_, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Contacts",
		URL:    server.URL,
		Events: []string{"contact.created"},
	})
	require.NoError(t, err)
	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Invoices",
		URL:    server.URL,
		Events: []string{"invoice.created"},
	})
	require.NoError(t, err)

	result, err := service.Dispatch(context.Background(), Event{
		Type:     "invoice.created",
		TenantID: "tenant-1",
		Data:     json.RawMessage(`{"invoice_id":"inv-1"}`),
	})
	require.NoError(t, err)

	require.Len(t, result.Deliveries, 1)
	assert.Equal(t, DeliveryStatusFailed, result.Deliveries[0].Status)
	assert.Equal(t, http.StatusInternalServerError, result.Deliveries[0].StatusCode)
	assert.Contains(t, result.Deliveries[0].Error, "HTTP 500")
	require.Len(t, repo.deliveries, 1)
}

func TestService_DispatchTestUsesWebhookTestEvent(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "webhook.test", r.Header.Get("X-Open-Accounting-Event"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Any",
		URL:    server.URL,
		Events: []string{"invoice.created"},
	})
	require.NoError(t, err)

	result, err := service.DispatchTest(context.Background(), "tenant-1", endpoint.ID, &TestDeliveryRequest{})
	require.NoError(t, err)
	assert.Equal(t, "webhook.test", result.Event.Type)
	require.Len(t, result.Deliveries, 1)
	assert.Equal(t, DeliveryStatusSucceeded, result.Deliveries[0].Status)
}

func TestService_DispatchAsyncSendsSubscribedEvent(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "Async",
		URL:    server.URL,
		Events: []string{plugin.EventPaymentReceived},
	})
	require.NoError(t, err)

	service.DispatchAsync(Event{
		Type:     plugin.EventPaymentReceived,
		TenantID: "tenant-1",
		Data:     json.RawMessage(`{"payment_id":"pay-1"}`),
	})

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async webhook delivery")
	}
}

func TestService_RegisterPluginHooksDispatchesPluginEvents(t *testing.T) {
	repo := newMemoryRepository()
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	tenantID := uuid.New()
	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, plugin.EventInvoiceCreated, r.Header.Get("X-Open-Accounting-Event"))
		assert.Equal(t, tenantID.String(), r.Header.Get("X-Open-Accounting-Tenant-ID"))
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := service.CreateEndpoint(context.Background(), tenantID.String(), &CreateEndpointRequest{
		Name:   "Plugin bridge",
		URL:    server.URL,
		Events: []string{plugin.EventInvoiceCreated},
	})
	require.NoError(t, err)

	registry := plugin.NewHookRegistry()
	service.RegisterPluginHooks(registry)
	assert.Equal(t, 1, registry.GetHandlerCount(plugin.EventInvoiceCreated))

	err = registry.Emit(context.Background(), plugin.Event{
		Type:     plugin.EventInvoiceCreated,
		TenantID: tenantID,
		Data:     json.RawMessage(`{"invoice_id":"inv-1"}`),
		Time:     fixedWebhookTime(),
	})
	require.NoError(t, err)

	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for plugin webhook delivery")
	}
	require.Len(t, repo.deliveries, 1)
	assert.Equal(t, DeliveryStatusSucceeded, repo.deliveries[0].Status)
}

func TestListEventTypesReturnsSortedCopy(t *testing.T) {
	events := ListEventTypes()
	require.NotEmpty(t, events)
	assert.True(t, sortStringsAreAscending(events))
	assert.Contains(t, events, plugin.EventInvoiceCreated)

	events[0] = "mutated"
	assert.NotEqual(t, "mutated", ListEventTypes()[0])
}

func fixedWebhookTime() time.Time {
	return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
}

func expectedSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func ioReadAll(r *http.Request) ([]byte, error) {
	defer func() {
		_ = r.Body.Close()
	}()
	return io.ReadAll(r.Body)
}

func sortStringsAreAscending(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] > values[i] {
			return false
		}
	}
	return true
}
