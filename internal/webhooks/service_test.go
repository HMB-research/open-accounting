package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

	badURL := "ftp://example.com/hooks"
	_, err = service.UpdateEndpoint(context.Background(), "tenant-1", endpoint.ID, &UpdateEndpointRequest{URL: &badURL})
	require.ErrorContains(t, err, "url must use http or https")

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
	service.httpClient = server.Client()
	service.validateTarget = func(context.Context, string) error { return nil }

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
	service.httpClient = server.Client()
	service.validateTarget = func(context.Context, string) error { return nil }

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
	assert.Empty(t, delivery.ResponseBody)
	encodedDelivery, err := json.Marshal(delivery)
	require.NoError(t, err)
	assert.NotContains(t, string(encodedDelivery), "response_body")
	require.Len(t, repo.deliveries, 1)
	assert.Equal(t, DeliveryStatusSucceeded, repo.deliveries[0].Status)
	assert.Empty(t, repo.deliveries[0].ResponseBody)
	require.NotNil(t, repo.endpoints[endpoint.ID].LastDeliveryAt)
}

func TestService_DefaultClientBlocksPrivateWebhookTargets(t *testing.T) {
	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := newMemoryRepository()
	endpoint := seededEndpoint("endpoint-1", "tenant-1", server.URL)
	repo.endpoints[endpoint.ID] = endpoint
	service := NewServiceWithRepository(repo, nil)
	service.now = fixedWebhookTime

	result, err := service.Dispatch(context.Background(), Event{
		ID:       "event-1",
		Type:     plugin.EventInvoiceCreated,
		TenantID: "tenant-1",
		Data:     json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	require.Len(t, result.Deliveries, 1)
	assert.Equal(t, DeliveryStatusFailed, result.Deliveries[0].Status)
	assert.Contains(t, result.Deliveries[0].Error, "private or reserved")

	select {
	case <-received:
		t.Fatal("private webhook target received a request")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestService_DispatchDoesNotFollowRedirects(t *testing.T) {
	var requests int
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return nil },
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://127.0.0.1:8080/internal"}},
				Body:       io.NopCloser(strings.NewReader("redirect")),
				Request:    req,
			}, nil
		}),
	}

	repo := newMemoryRepository()
	endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://93.184.216.34/events")
	repo.endpoints[endpoint.ID] = endpoint
	service := NewServiceWithRepository(repo, client)
	service.now = fixedWebhookTime

	result, err := service.Dispatch(context.Background(), Event{
		ID:       "event-1",
		Type:     plugin.EventInvoiceCreated,
		TenantID: "tenant-1",
		Data:     json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	require.Len(t, result.Deliveries, 1)
	assert.Equal(t, DeliveryStatusFailed, result.Deliveries[0].Status)
	assert.Equal(t, http.StatusFound, result.Deliveries[0].StatusCode)
	assert.Empty(t, result.Deliveries[0].ResponseBody)
	assert.Equal(t, 1, requests)
}

func TestWebhookTargetValidation(t *testing.T) {
	err := validateWebhookTarget(context.Background(), "http://127.0.0.1/hooks")
	require.ErrorContains(t, err, "private or reserved")

	require.NoError(t, validateWebhookTarget(context.Background(), "https://93.184.216.34/hooks"))
}

func TestResolvePublicWebhookHost(t *testing.T) {
	ctx := context.Background()
	publicIP := net.ParseIP("93.184.216.34")

	t.Run("rejects an empty host", func(t *testing.T) {
		_, err := resolvePublicWebhookHost(ctx, " ", func(context.Context, string) ([]net.IPAddr, error) {
			t.Fatal("lookup must not be called for an empty host")
			return nil, nil
		})
		require.ErrorContains(t, err, "url host is required")
	})

	t.Run("wraps lookup failures", func(t *testing.T) {
		_, err := resolvePublicWebhookHost(ctx, "hooks.example", func(context.Context, string) ([]net.IPAddr, error) {
			return nil, assert.AnError
		})
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("rejects empty lookup results", func(t *testing.T) {
		_, err := resolvePublicWebhookHost(ctx, "hooks.example", func(context.Context, string) ([]net.IPAddr, error) {
			return nil, nil
		})
		require.ErrorContains(t, err, "did not resolve")
	})

	t.Run("rejects private addresses", func(t *testing.T) {
		_, err := resolvePublicWebhookHost(ctx, "hooks.example", func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		})
		require.ErrorContains(t, err, "private or reserved")
	})

	t.Run("returns public addresses", func(t *testing.T) {
		addresses, err := resolvePublicWebhookHost(ctx, " hooks.example ", func(_ context.Context, host string) ([]net.IPAddr, error) {
			assert.Equal(t, "hooks.example", host)
			return []net.IPAddr{{IP: publicIP}}, nil
		})
		require.NoError(t, err)
		require.Equal(t, []net.IPAddr{{IP: publicIP}}, addresses)
	})
}

func TestDialWebhookTarget(t *testing.T) {
	ctx := context.Background()
	publicIP := net.ParseIP("93.184.216.34")

	t.Run("rejects malformed dial addresses", func(t *testing.T) {
		_, err := dialWebhookTarget(ctx, "tcp", "malformed", nil, nil)
		require.ErrorContains(t, err, "invalid webhook dial address")
	})

	t.Run("returns the final dial error", func(t *testing.T) {
		_, err := dialWebhookTarget(
			ctx,
			"tcp",
			"hooks.example:443",
			func(context.Context, string) ([]net.IPAddr, error) { return []net.IPAddr{{IP: publicIP}}, nil },
			func(_ context.Context, network, address string) (net.Conn, error) {
				assert.Equal(t, "tcp", network)
				assert.Equal(t, "93.184.216.34:443", address)
				return nil, assert.AnError
			},
		)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("refuses a private address returned at dial time", func(t *testing.T) {
		_, err := dialWebhookTarget(
			ctx,
			"tcp",
			"hooks.example:443",
			func(context.Context, string) ([]net.IPAddr, error) {
				return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
			},
			func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("private resolved address must never be dialed")
				return nil, nil
			},
		)
		require.ErrorContains(t, err, "private or reserved")
	})

	t.Run("dials only validated resolved addresses", func(t *testing.T) {
		connection, err := dialWebhookTarget(
			ctx,
			"tcp",
			"hooks.example:443",
			func(context.Context, string) ([]net.IPAddr, error) { return []net.IPAddr{{IP: publicIP}}, nil },
			func(context.Context, string, string) (net.Conn, error) {
				client, server := net.Pipe()
				_ = server.Close()
				return client, nil
			},
		)
		require.NoError(t, err)
		require.NoError(t, connection.Close())
	})

	t.Run("production dialer rejects private literal targets", func(t *testing.T) {
		_, err := dialPublicWebhookTarget(ctx, "tcp", "127.0.0.1:443")
		require.ErrorContains(t, err, "private or reserved")
	})
}

func TestRejectWebhookRedirect(t *testing.T) {
	require.ErrorIs(t, rejectWebhookRedirect(nil, nil), http.ErrUseLastResponse)
}

func TestNewWebhookHTTPClientFallsBackWhenDefaultTransportIsNotHTTP(t *testing.T) {
	originalTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, assert.AnError
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	client := newWebhookHTTPClient()
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, defaultHTTPTimeout, client.Timeout)
	assert.NotNil(t, transport.DialContext)
}

func TestIsPublicWebhookIPRejectsSpecialUseRanges(t *testing.T) {
	assert.True(t, isPublicWebhookIP(net.ParseIP("93.184.216.34")))
	assert.False(t, isPublicWebhookIP(net.ParseIP("100.64.0.1")))
	assert.False(t, isPublicWebhookIP(net.ParseIP("198.18.0.1")))
}

func TestSanitizeEndpoint(t *testing.T) {
	endpoint := &Endpoint{Secret: "super-secret"}
	sanitizeEndpoint(endpoint)
	assert.True(t, endpoint.SecretSet)
	assert.Empty(t, endpoint.Secret)
}

func TestService_DeliverUsesDefaultTargetValidation(t *testing.T) {
	repo := newMemoryRepository()
	endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://93.184.216.34/hooks")
	repo.endpoints[endpoint.ID] = endpoint
	service := NewServiceWithRepository(repo, &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
		}),
	})
	service.now = fixedWebhookTime
	service.validateTarget = nil

	delivery, err := service.deliver(context.Background(), endpoint, Event{
		ID:       "event-1",
		Type:     plugin.EventInvoiceCreated,
		TenantID: "tenant-1",
		Data:     json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	assert.Equal(t, DeliveryStatusSucceeded, delivery.Status)
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
	service.httpClient = server.Client()
	service.validateTarget = func(context.Context, string) error { return nil }

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
	service.httpClient = server.Client()
	service.validateTarget = func(context.Context, string) error { return nil }

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
	service.httpClient = server.Client()
	service.validateTarget = func(context.Context, string) error { return nil }

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
	service.httpClient = server.Client()
	service.validateTarget = func(context.Context, string) error { return nil }

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

func TestService_NewServiceWithNilPoolUsesDefaultClient(t *testing.T) {
	service := NewService(nil)
	require.NotNil(t, service)
	assert.Nil(t, service.repo)
	require.NotNil(t, service.httpClient)
	assert.Equal(t, defaultHTTPTimeout, service.httpClient.Timeout)
}

func TestService_RegisterPluginHooksNilRegistry(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), nil)
	require.NotPanics(t, func() {
		service.RegisterPluginHooks(nil)
	})
}

func TestService_NormalizeEventValidationAndDefaults(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), nil)
	service.now = fixedWebhookTime

	event, err := service.normalizeEvent(Event{
		Type:     " invoice.created ",
		TenantID: " tenant-1 ",
	})
	require.NoError(t, err)
	assert.Equal(t, plugin.EventInvoiceCreated, event.Type)
	assert.Equal(t, "tenant-1", event.TenantID)
	assert.NotEmpty(t, event.ID)
	assert.Equal(t, json.RawMessage(`{}`), event.Data)
	assert.Equal(t, fixedWebhookTime(), event.CreatedAt)

	_, err = service.normalizeEvent(Event{TenantID: "tenant-1"})
	require.ErrorContains(t, err, "event_type is required")

	_, err = service.normalizeEvent(Event{Type: "unknown.event", TenantID: "tenant-1"})
	require.ErrorContains(t, err, "unsupported event_type")

	_, err = service.normalizeEvent(Event{Type: plugin.EventInvoiceCreated})
	require.ErrorContains(t, err, "tenant_id is required")

	_, err = service.normalizeEvent(Event{
		Type:     plugin.EventInvoiceCreated,
		TenantID: "tenant-1",
		Data:     json.RawMessage(`{invalid}`),
	})
	require.ErrorContains(t, err, "payload must be valid JSON")
}

func TestService_CreateEndpointRejectsRequiredFields(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), nil)

	_, err := service.CreateEndpoint(context.Background(), "tenant-1", nil)
	require.ErrorContains(t, err, "request is required")

	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "CRM",
		URL:    "https://example.com/hooks",
		Events: []string{" ", ""},
	})
	require.ErrorContains(t, err, "at least one event is required")

	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "CRM",
		URL:    "",
		Events: []string{plugin.EventInvoiceCreated},
	})
	require.ErrorContains(t, err, "url is required")

	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "CRM",
		URL:    "://bad-url",
		Events: []string{plugin.EventInvoiceCreated},
	})
	require.ErrorContains(t, err, "invalid url")

	_, err = service.CreateEndpoint(context.Background(), "tenant-1", &CreateEndpointRequest{
		Name:   "",
		URL:    "https://example.com/hooks",
		Events: []string{plugin.EventInvoiceCreated},
	})
	require.ErrorContains(t, err, "name is required")
}

func TestValidateWebhookURLRejectsMissingHost(t *testing.T) {
	err := validateWebhookURL("http:/hooks")
	require.ErrorContains(t, err, "url host is required")
}

func TestService_PropagatesRepositoryErrors(t *testing.T) {
	ctx := context.Background()
	validRequest := &CreateEndpointRequest{
		Name:   "CRM",
		URL:    "https://example.com/hooks",
		Events: []string{plugin.EventInvoiceCreated},
	}

	t.Run("list endpoints", func(t *testing.T) {
		repo := newErroringRepository()
		repo.listErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		_, err := service.ListEndpoints(ctx, "tenant-1", true)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("get endpoint", func(t *testing.T) {
		repo := newErroringRepository()
		repo.getErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		_, err := service.GetEndpoint(ctx, "tenant-1", "endpoint-1")
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("create endpoint", func(t *testing.T) {
		repo := newErroringRepository()
		repo.createErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		_, err := service.CreateEndpoint(ctx, "tenant-1", validRequest)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("update endpoint get", func(t *testing.T) {
		repo := newErroringRepository()
		repo.getErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		name := "Updated"
		_, err := service.UpdateEndpoint(ctx, "tenant-1", "endpoint-1", &UpdateEndpointRequest{Name: &name})
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("update endpoint save", func(t *testing.T) {
		repo := newErroringRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://example.com/hooks")
		repo.endpoints[endpoint.ID] = endpoint
		repo.updateErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		name := "Updated"
		_, err := service.UpdateEndpoint(ctx, "tenant-1", endpoint.ID, &UpdateEndpointRequest{Name: &name})
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("delete endpoint", func(t *testing.T) {
		repo := newErroringRepository()
		repo.deleteErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		err := service.DeleteEndpoint(ctx, "tenant-1", "endpoint-1")
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("list deliveries endpoint lookup", func(t *testing.T) {
		repo := newErroringRepository()
		repo.getErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		_, err := service.ListDeliveries(ctx, "tenant-1", "endpoint-1", 10)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("list deliveries", func(t *testing.T) {
		repo := newErroringRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://example.com/hooks")
		repo.endpoints[endpoint.ID] = endpoint
		repo.listDeliveriesErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		_, err := service.ListDeliveries(ctx, "tenant-1", endpoint.ID, 10)
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("dispatch list endpoints", func(t *testing.T) {
		repo := newErroringRepository()
		repo.listErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		_, err := service.Dispatch(ctx, Event{Type: plugin.EventInvoiceCreated, TenantID: "tenant-1"})
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("dispatch test endpoint lookup", func(t *testing.T) {
		repo := newErroringRepository()
		repo.getErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)

		_, err := service.DispatchTest(ctx, "tenant-1", "endpoint-1", nil)
		require.ErrorIs(t, err, assert.AnError)
	})
}

func TestService_DeliveryFailureEdges(t *testing.T) {
	ctx := context.Background()

	t.Run("event encoding error is returned", func(t *testing.T) {
		repo := newMemoryRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://example.com/hooks")
		repo.endpoints[endpoint.ID] = endpoint
		service := NewServiceWithRepository(repo, nil)

		_, err := service.deliver(ctx, endpoint, Event{
			ID:       "event-1",
			Type:     plugin.EventInvoiceCreated,
			TenantID: "tenant-1",
			Data:     json.RawMessage(`{invalid}`),
		})
		require.ErrorContains(t, err, "encode webhook event")
		require.Empty(t, repo.deliveries)
	})

	t.Run("invalid endpoint URL is recorded", func(t *testing.T) {
		repo := newMemoryRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "http://[::1")
		repo.endpoints[endpoint.ID] = endpoint
		service := NewServiceWithRepository(repo, nil)
		service.now = fixedWebhookTime
		service.validateTarget = func(context.Context, string) error { return nil }

		delivery, err := service.deliver(ctx, endpoint, Event{
			ID:       "event-1",
			Type:     plugin.EventInvoiceCreated,
			TenantID: "tenant-1",
			Data:     json.RawMessage(`{}`),
		})
		require.NoError(t, err)
		assert.Equal(t, DeliveryStatusFailed, delivery.Status)
		assert.NotEmpty(t, delivery.Error)
		require.Len(t, repo.deliveries, 1)
	})

	t.Run("http client error is recorded", func(t *testing.T) {
		repo := newMemoryRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://example.com/hooks")
		repo.endpoints[endpoint.ID] = endpoint
		service := NewServiceWithRepository(repo, &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return nil, assert.AnError
			}),
		})
		service.now = fixedWebhookTime
		service.validateTarget = func(context.Context, string) error { return nil }

		delivery, err := service.deliver(ctx, endpoint, Event{
			ID:       "event-1",
			Type:     plugin.EventInvoiceCreated,
			TenantID: "tenant-1",
			Data:     json.RawMessage(`{}`),
		})
		require.NoError(t, err)
		assert.Equal(t, DeliveryStatusFailed, delivery.Status)
		assert.Contains(t, delivery.Error, assert.AnError.Error())
	})

	t.Run("response body read error is recorded", func(t *testing.T) {
		repo := newMemoryRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://example.com/hooks")
		repo.endpoints[endpoint.ID] = endpoint
		service := NewServiceWithRepository(repo, &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       errorReadCloser{},
				}, nil
			}),
		})
		service.now = fixedWebhookTime
		service.validateTarget = func(context.Context, string) error { return nil }

		delivery, err := service.deliver(ctx, endpoint, Event{
			ID:       "event-1",
			Type:     plugin.EventInvoiceCreated,
			TenantID: "tenant-1",
			Data:     json.RawMessage(`{}`),
		})
		require.NoError(t, err)
		assert.Equal(t, DeliveryStatusFailed, delivery.Status)
		assert.Contains(t, delivery.Error, assert.AnError.Error())
	})

	t.Run("create delivery error is returned", func(t *testing.T) {
		repo := newErroringRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "http://[::1")
		repo.endpoints[endpoint.ID] = endpoint
		repo.createDeliveryErr = assert.AnError
		service := NewServiceWithRepository(repo, nil)
		service.now = fixedWebhookTime

		_, err := service.deliver(ctx, endpoint, Event{
			ID:       "event-1",
			Type:     plugin.EventInvoiceCreated,
			TenantID: "tenant-1",
			Data:     json.RawMessage(`{}`),
		})
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("update endpoint last delivery error is returned", func(t *testing.T) {
		repo := newErroringRepository()
		endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://example.com/hooks")
		repo.endpoints[endpoint.ID] = endpoint
		repo.updateLastDeliveryErr = assert.AnError
		service := NewServiceWithRepository(repo, &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusAccepted,
					Body:       io.NopCloser(strings.NewReader("accepted")),
				}, nil
			}),
		})
		service.now = fixedWebhookTime
		service.validateTarget = func(context.Context, string) error { return nil }

		_, err := service.deliver(ctx, endpoint, Event{
			ID:       "event-1",
			Type:     plugin.EventInvoiceCreated,
			TenantID: "tenant-1",
			Data:     json.RawMessage(`{}`),
		})
		require.ErrorIs(t, err, assert.AnError)
	})
}

func TestService_DispatchRejectsInvalidEvent(t *testing.T) {
	service := NewServiceWithRepository(newMemoryRepository(), nil)

	_, err := service.Dispatch(context.Background(), Event{TenantID: "tenant-1"})
	require.ErrorContains(t, err, "event_type is required")
}

func TestService_DispatchTestRejectsInvalidOverrideEvent(t *testing.T) {
	repo := newMemoryRepository()
	endpoint := seededEndpoint("endpoint-1", "tenant-1", "https://example.com/hooks")
	repo.endpoints[endpoint.ID] = endpoint
	service := NewServiceWithRepository(repo, nil)

	_, err := service.DispatchTest(context.Background(), "tenant-1", endpoint.ID, &TestDeliveryRequest{
		EventType: "unknown.event",
		Payload:   json.RawMessage(`{}`),
	})
	require.ErrorContains(t, err, "unsupported event_type")
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

func seededEndpoint(id, tenantID, url string) *Endpoint {
	return &Endpoint{
		ID:        id,
		TenantID:  tenantID,
		Name:      "CRM",
		URL:       url,
		Events:    []string{plugin.EventInvoiceCreated},
		Secret:    "secret",
		IsActive:  true,
		CreatedAt: fixedWebhookTime(),
		UpdatedAt: fixedWebhookTime(),
	}
}

type erroringRepository struct {
	*memoryRepository

	listErr               error
	getErr                error
	createErr             error
	updateErr             error
	deleteErr             error
	createDeliveryErr     error
	listDeliveriesErr     error
	updateLastDeliveryErr error
}

func newErroringRepository() *erroringRepository {
	return &erroringRepository{memoryRepository: newMemoryRepository()}
}

func (r *erroringRepository) ListEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]Endpoint, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.memoryRepository.ListEndpoints(ctx, tenantID, activeOnly)
}

func (r *erroringRepository) GetEndpoint(ctx context.Context, tenantID, endpointID string) (*Endpoint, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.memoryRepository.GetEndpoint(ctx, tenantID, endpointID)
}

func (r *erroringRepository) CreateEndpoint(ctx context.Context, endpoint *Endpoint) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.memoryRepository.CreateEndpoint(ctx, endpoint)
}

func (r *erroringRepository) UpdateEndpoint(ctx context.Context, endpoint *Endpoint) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.memoryRepository.UpdateEndpoint(ctx, endpoint)
}

func (r *erroringRepository) DeleteEndpoint(ctx context.Context, tenantID, endpointID string) (int64, error) {
	if r.deleteErr != nil {
		return 0, r.deleteErr
	}
	return r.memoryRepository.DeleteEndpoint(ctx, tenantID, endpointID)
}

func (r *erroringRepository) CreateDelivery(ctx context.Context, delivery *Delivery) error {
	if r.createDeliveryErr != nil {
		return r.createDeliveryErr
	}
	return r.memoryRepository.CreateDelivery(ctx, delivery)
}

func (r *erroringRepository) ListDeliveries(ctx context.Context, tenantID, endpointID string, limit int) ([]Delivery, error) {
	if r.listDeliveriesErr != nil {
		return nil, r.listDeliveriesErr
	}
	return r.memoryRepository.ListDeliveries(ctx, tenantID, endpointID, limit)
}

func (r *erroringRepository) UpdateEndpointLastDelivery(ctx context.Context, tenantID, endpointID string, deliveredAt time.Time) error {
	if r.updateLastDeliveryErr != nil {
		return r.updateLastDeliveryErr
	}
	return r.memoryRepository.UpdateEndpointLastDelivery(ctx, tenantID, endpointID, deliveredAt)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReadCloser struct{}

func (errorReadCloser) Read([]byte) (int, error) {
	return 0, assert.AnError
}

func (errorReadCloser) Close() error {
	return nil
}
