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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
