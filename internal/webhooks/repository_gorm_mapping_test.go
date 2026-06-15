package webhooks

import (
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/lib/pq"
)

func TestWebhookEndpointModelMappings(t *testing.T) {
	lastDeliveryAt := time.Date(2026, 6, 11, 9, 15, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)

	model := &models.WebhookEndpoint{
		ID:             "endpoint-1",
		TenantID:       "tenant-1",
		Name:           "Accounting Events",
		URL:            "https://example.com/accounting/webhook",
		Events:         pq.StringArray{"invoice.created", "payment.received"},
		Secret:         "stored-secret",
		IsActive:       true,
		LastDeliveryAt: &lastDeliveryAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	endpoint := modelToEndpoint(model)

	if endpoint.ID != model.ID ||
		endpoint.TenantID != model.TenantID ||
		endpoint.Name != model.Name ||
		endpoint.URL != model.URL ||
		len(endpoint.Events) != len(model.Events) ||
		endpoint.Events[0] != model.Events[0] ||
		endpoint.Events[1] != model.Events[1] ||
		endpoint.Secret != model.Secret ||
		!endpoint.SecretSet ||
		endpoint.IsActive != model.IsActive ||
		endpoint.LastDeliveryAt != model.LastDeliveryAt ||
		!endpoint.CreatedAt.Equal(model.CreatedAt) ||
		!endpoint.UpdatedAt.Equal(model.UpdatedAt) {
		t.Fatalf("modelToEndpoint() = %#v, want fields from %#v", endpoint, model)
	}

	roundTrip := endpointToModel(endpoint)

	if roundTrip.ID != endpoint.ID ||
		roundTrip.TenantID != endpoint.TenantID ||
		roundTrip.Name != endpoint.Name ||
		roundTrip.URL != endpoint.URL ||
		len(roundTrip.Events) != len(endpoint.Events) ||
		roundTrip.Events[0] != endpoint.Events[0] ||
		roundTrip.Events[1] != endpoint.Events[1] ||
		roundTrip.Secret != endpoint.Secret ||
		roundTrip.IsActive != endpoint.IsActive ||
		roundTrip.LastDeliveryAt != endpoint.LastDeliveryAt ||
		!roundTrip.CreatedAt.Equal(endpoint.CreatedAt) ||
		!roundTrip.UpdatedAt.Equal(endpoint.UpdatedAt) {
		t.Fatalf("endpointToModel() = %#v, want fields from %#v", roundTrip, endpoint)
	}

	withoutSecret := modelToEndpoint(&models.WebhookEndpoint{})
	if withoutSecret.SecretSet {
		t.Fatalf("modelToEndpoint() SecretSet = true for an empty stored secret")
	}
}

func TestWebhookDeliveryModelMappings(t *testing.T) {
	deliveredAt := time.Date(2026, 6, 11, 12, 30, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 11, 12, 31, 0, 0, time.UTC)
	statusCode := 202

	model := &models.WebhookDelivery{
		ID:            "delivery-1",
		TenantID:      "tenant-1",
		EndpointID:    "endpoint-1",
		EventID:       "event-1",
		EventType:     "invoice.created",
		Status:        DeliveryStatusSucceeded,
		StatusCode:    &statusCode,
		AttemptNumber: 2,
		RequestBody:   []byte(`{"id":"invoice-1"}`),
		ResponseBody:  "accepted",
		Error:         "",
		DeliveredAt:   deliveredAt,
		CreatedAt:     createdAt,
	}

	delivery := modelToDelivery(model)

	if delivery.ID != model.ID ||
		delivery.TenantID != model.TenantID ||
		delivery.EndpointID != model.EndpointID ||
		delivery.EventID != model.EventID ||
		delivery.EventType != model.EventType ||
		delivery.Status != model.Status ||
		delivery.StatusCode != *model.StatusCode ||
		delivery.AttemptNumber != model.AttemptNumber ||
		string(delivery.RequestBody) != string(model.RequestBody) ||
		delivery.ResponseBody != model.ResponseBody ||
		delivery.Error != model.Error ||
		!delivery.DeliveredAt.Equal(model.DeliveredAt) ||
		!delivery.CreatedAt.Equal(model.CreatedAt) {
		t.Fatalf("modelToDelivery() = %#v, want fields from %#v", delivery, model)
	}

	roundTrip := deliveryToModel(delivery)

	if roundTrip.ID != delivery.ID ||
		roundTrip.TenantID != delivery.TenantID ||
		roundTrip.EndpointID != delivery.EndpointID ||
		roundTrip.EventID != delivery.EventID ||
		roundTrip.EventType != delivery.EventType ||
		roundTrip.Status != delivery.Status ||
		roundTrip.StatusCode == nil ||
		*roundTrip.StatusCode != delivery.StatusCode ||
		roundTrip.AttemptNumber != delivery.AttemptNumber ||
		string(roundTrip.RequestBody) != string(delivery.RequestBody) ||
		roundTrip.ResponseBody != delivery.ResponseBody ||
		roundTrip.Error != delivery.Error ||
		!roundTrip.DeliveredAt.Equal(delivery.DeliveredAt) ||
		!roundTrip.CreatedAt.Equal(delivery.CreatedAt) {
		t.Fatalf("deliveryToModel() = %#v, want fields from %#v", roundTrip, delivery)
	}
}

func TestWebhookDeliveryModelMappingsHandleMissingStatusCode(t *testing.T) {
	model := deliveryToModel(&Delivery{})
	if model.StatusCode != nil {
		t.Fatalf("deliveryToModel() StatusCode = %v, want nil for zero status code", *model.StatusCode)
	}

	delivery := modelToDelivery(&models.WebhookDelivery{})
	if delivery.StatusCode != 0 {
		t.Fatalf("modelToDelivery() StatusCode = %d, want 0 for nil stored status code", delivery.StatusCode)
	}
}
