package webhooks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepository_WebhookLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newWebhookGORMRepository(t, pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	endpoint := &Endpoint{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		Name:      "Billing Events",
		URL:       "https://example.com/webhooks",
		Events:    []string{"invoice.created", "payment.received"},
		Secret:    "secret-value",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateEndpoint(ctx, endpoint); err != nil {
		t.Fatalf("CreateEndpoint failed: %v", err)
	}

	listed, err := repo.ListEndpoints(ctx, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEndpoints failed: %v", err)
	}
	if len(listed) != 1 || listed[0].Name != endpoint.Name || !listed[0].SecretSet {
		t.Fatalf("unexpected listed endpoints: %+v", listed)
	}

	endpoint.Name = "Updated Billing Events"
	endpoint.Events = []string{"invoice.created"}
	endpoint.IsActive = false
	endpoint.UpdatedAt = now.Add(time.Minute)
	if err := repo.UpdateEndpoint(ctx, endpoint); err != nil {
		t.Fatalf("UpdateEndpoint failed: %v", err)
	}

	active, err := repo.ListEndpoints(ctx, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEndpoints active failed: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected inactive endpoint to be filtered, got %+v", active)
	}

	retrieved, err := repo.GetEndpoint(ctx, tenant.ID, endpoint.ID)
	if err != nil {
		t.Fatalf("GetEndpoint failed: %v", err)
	}
	if retrieved.Name != endpoint.Name || len(retrieved.Events) != 1 || retrieved.Events[0] != "invoice.created" {
		t.Fatalf("unexpected retrieved endpoint: %+v", retrieved)
	}

	deliveredAt := now.Add(2 * time.Minute)
	if err := repo.UpdateEndpointLastDelivery(ctx, tenant.ID, endpoint.ID, deliveredAt); err != nil {
		t.Fatalf("UpdateEndpointLastDelivery failed: %v", err)
	}

	requestBody := json.RawMessage(`{"id":"evt_1"}`)
	delivery := &Delivery{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		EndpointID:    endpoint.ID,
		EventID:       "evt_1",
		EventType:     "invoice.created",
		Status:        DeliveryStatusSucceeded,
		StatusCode:    204,
		AttemptNumber: 1,
		RequestBody:   requestBody,
		ResponseBody:  "accepted",
		DeliveredAt:   deliveredAt,
		CreatedAt:     deliveredAt,
	}
	if err := repo.CreateDelivery(ctx, delivery); err != nil {
		t.Fatalf("CreateDelivery failed: %v", err)
	}

	deliveries, err := repo.ListDeliveries(ctx, tenant.ID, endpoint.ID, 10)
	if err != nil {
		t.Fatalf("ListDeliveries failed: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("unexpected deliveries: %+v", deliveries)
	}
	var deliveredPayload map[string]string
	if err := json.Unmarshal(deliveries[0].RequestBody, &deliveredPayload); err != nil {
		t.Fatalf("unmarshal delivered request body: %v", err)
	}
	if deliveries[0].StatusCode != 204 || deliveredPayload["id"] != "evt_1" {
		t.Fatalf("unexpected deliveries: %+v", deliveries)
	}

	affected, err := repo.DeleteEndpoint(ctx, tenant.ID, endpoint.ID)
	if err != nil {
		t.Fatalf("DeleteEndpoint failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected one deleted endpoint, got %d", affected)
	}
}

func newWebhookGORMRepository(t *testing.T, pool *pgxpool.Pool) *GORMRepository {
	t.Helper()

	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		t.Fatalf("create gorm repository: %v", err)
	}
	return NewGORMRepository(gormDB)
}
