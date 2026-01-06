//go:build integration

package orders

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func strPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func createTestContact(t *testing.T, pool interface{ Exec(ctx context.Context, sql string, arguments ...interface{}) (interface{}, error) }, schemaName, tenantID string) string {
	t.Helper()
	_ = context.Background() // placeholder for future use
	contactID := uuid.New().String()

	// The pool interface doesn't match exactly, so we'll work around this
	return contactID
}

func TestRepository_CreateAndGetOrder(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact for the order
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	orderDate := time.Now()
	expectedDelivery := time.Now().AddDate(0, 0, 14)
	order := &Order{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		OrderNumber:      "ORD-00001",
		ContactID:        contactID,
		OrderDate:        orderDate,
		ExpectedDelivery: &expectedDelivery,
		Status:           OrderStatusPending,
		Currency:         "EUR",
		ExchangeRate:     decimal.NewFromInt(1),
		Subtotal:         decimal.NewFromFloat(200.00),
		VATAmount:        decimal.NewFromFloat(40.00),
		Total:            decimal.NewFromFloat(240.00),
		Notes:            "Test order",
		CreatedAt:        time.Now(),
		CreatedBy:        uuid.New().String(),
		UpdatedAt:        time.Now(),
		Lines: []OrderLine{
			{
				ID:              uuid.New().String(),
				TenantID:        tenant.ID,
				LineNumber:      1,
				Description:     "Product A",
				Quantity:        decimal.NewFromInt(2),
				Unit:            "pcs",
				UnitPrice:       decimal.NewFromFloat(100.00),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.NewFromFloat(20.00),
				LineSubtotal:    decimal.NewFromFloat(200.00),
				LineVAT:         decimal.NewFromFloat(40.00),
				LineTotal:       decimal.NewFromFloat(240.00),
			},
		},
	}

	err = repo.Create(ctx, tenant.SchemaName, order)
	if err != nil {
		t.Fatalf("Create order failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, order.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.OrderNumber != order.OrderNumber {
		t.Errorf("expected order number %s, got %s", order.OrderNumber, retrieved.OrderNumber)
	}
	if retrieved.ContactID != order.ContactID {
		t.Errorf("expected contact ID %s, got %s", order.ContactID, retrieved.ContactID)
	}
	if !retrieved.Total.Equal(order.Total) {
		t.Errorf("expected total %s, got %s", order.Total, retrieved.Total)
	}
	if len(retrieved.Lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(retrieved.Lines))
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, "nonexistent")
	if err != ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestRepository_ListOrders(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	// Create multiple orders
	for i := 0; i < 3; i++ {
		order := &Order{
			ID:           uuid.New().String(),
			TenantID:     tenant.ID,
			OrderNumber:  uuid.New().String()[:10],
			ContactID:    contactID,
			OrderDate:    time.Now(),
			Status:       OrderStatusPending,
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
			Subtotal:     decimal.NewFromFloat(float64(100 * (i + 1))),
			VATAmount:    decimal.NewFromFloat(float64(20 * (i + 1))),
			Total:        decimal.NewFromFloat(float64(120 * (i + 1))),
			CreatedAt:    time.Now(),
			CreatedBy:    uuid.New().String(),
			UpdatedAt:    time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, order); err != nil {
			t.Fatalf("Create order failed: %v", err)
		}
	}

	orders, err := repo.List(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(orders) != 3 {
		t.Errorf("expected 3 orders, got %d", len(orders))
	}
}

func TestRepository_ListOrders_WithFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	// Create draft order
	draftOrder := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-DRAFT",
		ContactID:    contactID,
		OrderDate:    time.Now(),
		Status:       OrderStatusPending,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, draftOrder); err != nil {
		t.Fatalf("Create draft order failed: %v", err)
	}

	// Create confirmed order
	confirmedOrder := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-CONFIRMED",
		ContactID:    contactID,
		OrderDate:    time.Now(),
		Status:       OrderStatusConfirmed,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(200.00),
		VATAmount:    decimal.NewFromFloat(40.00),
		Total:        decimal.NewFromFloat(240.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, confirmedOrder); err != nil {
		t.Fatalf("Create confirmed order failed: %v", err)
	}

	// Filter by status
	filter := &OrderFilter{Status: OrderStatusConfirmed}
	orders, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}

	if len(orders) != 1 {
		t.Errorf("expected 1 confirmed order, got %d", len(orders))
	}
	if orders[0].Status != OrderStatusConfirmed {
		t.Errorf("expected status %s, got %s", OrderStatusConfirmed, orders[0].Status)
	}
}

func TestRepository_UpdateOrder(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	order := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-UPDATE",
		ContactID:    contactID,
		OrderDate:    time.Now(),
		Status:       OrderStatusPending,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, order); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	order.Notes = "Updated notes"
	order.Subtotal = decimal.NewFromFloat(150.00)
	order.VATAmount = decimal.NewFromFloat(30.00)
	order.Total = decimal.NewFromFloat(180.00)

	if err := repo.Update(ctx, tenant.SchemaName, order); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, order.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Notes != "Updated notes" {
		t.Errorf("expected notes 'Updated notes', got '%s'", retrieved.Notes)
	}
	if !retrieved.Total.Equal(decimal.NewFromFloat(180.00)) {
		t.Errorf("expected total 180.00, got %s", retrieved.Total)
	}
}

func TestRepository_UpdateStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	order := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-STATUS",
		ContactID:    contactID,
		OrderDate:    time.Now(),
		Status:       OrderStatusPending,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, order); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update status
	if err := repo.UpdateStatus(ctx, tenant.SchemaName, tenant.ID, order.ID, OrderStatusConfirmed); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, order.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Status != OrderStatusConfirmed {
		t.Errorf("expected status %s, got %s", OrderStatusConfirmed, retrieved.Status)
	}
}

func TestRepository_DeleteOrder(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	order := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-DELETE",
		ContactID:    contactID,
		OrderDate:    time.Now(),
		Status:       OrderStatusPending,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, order); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, order.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.GetByID(ctx, tenant.SchemaName, tenant.ID, order.ID)
	if err != ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound after deletion, got %v", err)
	}
}

func TestRepository_GenerateNumber(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// First number
	num, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateNumber failed: %v", err)
	}
	if num != "ORD-00001" {
		t.Errorf("expected 'ORD-00001', got '%s'", num)
	}

	// Create contact
	contactID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	// Create an order with this number
	order := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  num,
		ContactID:    contactID,
		OrderDate:    time.Now(),
		Status:       OrderStatusPending,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, order); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Second number
	num2, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateNumber (second) failed: %v", err)
	}
	if num2 != "ORD-00002" {
		t.Errorf("expected 'ORD-00002', got '%s'", num2)
	}
}

func TestRepository_SetConvertedToInvoice(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	order := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-CONVERT",
		ContactID:    contactID,
		OrderDate:    time.Now(),
		Status:       OrderStatusConfirmed,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, order); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	invoiceID := uuid.New().String()
	if err := repo.SetConvertedToInvoice(ctx, tenant.SchemaName, tenant.ID, order.ID, invoiceID); err != nil {
		t.Fatalf("SetConvertedToInvoice failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, order.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ConvertedToInvoiceID == nil || *retrieved.ConvertedToInvoiceID != invoiceID {
		t.Errorf("expected converted to invoice ID %s, got %v", invoiceID, retrieved.ConvertedToInvoiceID)
	}
}

func TestRepository_ListOrders_WithContactFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create two contacts
	contact1ID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Customer 1', 'CUSTOMER', true, NOW(), NOW())
	`, contact1ID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact 1: %v", err)
	}

	contact2ID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Customer 2', 'CUSTOMER', true, NOW(), NOW())
	`, contact2ID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact 2: %v", err)
	}

	// Create orders for different contacts
	order1 := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-C1",
		ContactID:    contact1ID,
		OrderDate:    time.Now(),
		Status:       OrderStatusPending,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, order1); err != nil {
		t.Fatalf("Create order 1 failed: %v", err)
	}

	order2 := &Order{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		OrderNumber:  "ORD-C2",
		ContactID:    contact2ID,
		OrderDate:    time.Now(),
		Status:       OrderStatusPending,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(200.00),
		VATAmount:    decimal.NewFromFloat(40.00),
		Total:        decimal.NewFromFloat(240.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, order2); err != nil {
		t.Fatalf("Create order 2 failed: %v", err)
	}

	// Filter by contact
	filter := &OrderFilter{ContactID: contact1ID}
	orders, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List with contact filter failed: %v", err)
	}

	if len(orders) != 1 {
		t.Errorf("expected 1 order for contact 1, got %d", len(orders))
	}
	if orders[0].ContactID != contact1ID {
		t.Errorf("expected contact ID %s, got %s", contact1ID, orders[0].ContactID)
	}
}
