//go:build integration

package quotes

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestRepository_CreateAndGetQuote(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact for the quote
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, name, contact_type, is_active, created_at, updated_at)
		VALUES ($1, $2, 'Test Customer', 'CUSTOMER', true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create contact: %v", err)
	}

	quoteDate := time.Now()
	validUntil := time.Now().AddDate(0, 0, 30)
	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-00001",
		ContactID:    contactID,
		QuoteDate:    quoteDate,
		ValidUntil:   &validUntil,
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(500.00),
		VATAmount:    decimal.NewFromFloat(100.00),
		Total:        decimal.NewFromFloat(600.00),
		Notes:        "Test quote",
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
		Lines: []QuoteLine{
			{
				ID:              uuid.New().String(),
				TenantID:        tenant.ID,
				LineNumber:      1,
				Description:     "Consulting services",
				Quantity:        decimal.NewFromInt(10),
				Unit:            "hr",
				UnitPrice:       decimal.NewFromFloat(50.00),
				DiscountPercent: decimal.Zero,
				VATRate:         decimal.NewFromFloat(20.00),
				LineSubtotal:    decimal.NewFromFloat(500.00),
				LineVAT:         decimal.NewFromFloat(100.00),
				LineTotal:       decimal.NewFromFloat(600.00),
			},
		},
	}

	err = repo.Create(ctx, tenant.SchemaName, quote)
	if err != nil {
		t.Fatalf("Create quote failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, quote.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.QuoteNumber != quote.QuoteNumber {
		t.Errorf("expected quote number %s, got %s", quote.QuoteNumber, retrieved.QuoteNumber)
	}
	if retrieved.ContactID != quote.ContactID {
		t.Errorf("expected contact ID %s, got %s", quote.ContactID, retrieved.ContactID)
	}
	if !retrieved.Total.Equal(quote.Total) {
		t.Errorf("expected total %s, got %s", quote.Total, retrieved.Total)
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

	// Use valid UUID format that doesn't exist
	nonExistentID := uuid.New().String()
	_, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, nonExistentID)
	if err != ErrQuoteNotFound {
		t.Errorf("expected ErrQuoteNotFound, got %v", err)
	}
}

func TestRepository_ListQuotes(t *testing.T) {
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

	// Create multiple quotes
	for i := 0; i < 3; i++ {
		quote := &Quote{
			ID:           uuid.New().String(),
			TenantID:     tenant.ID,
			QuoteNumber:  uuid.New().String()[:10],
			ContactID:    contactID,
			QuoteDate:    time.Now(),
			Status:       QuoteStatusDraft,
			Currency:     "EUR",
			ExchangeRate: decimal.NewFromInt(1),
			Subtotal:     decimal.NewFromFloat(float64(100 * (i + 1))),
			VATAmount:    decimal.NewFromFloat(float64(20 * (i + 1))),
			Total:        decimal.NewFromFloat(float64(120 * (i + 1))),
			CreatedAt:    time.Now(),
			CreatedBy:    uuid.New().String(),
			UpdatedAt:    time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, quote); err != nil {
			t.Fatalf("Create quote failed: %v", err)
		}
	}

	quotes, err := repo.List(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(quotes) != 3 {
		t.Errorf("expected 3 quotes, got %d", len(quotes))
	}
}

func TestRepository_ListQuotes_WithFilter(t *testing.T) {
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

	// Create draft quote
	draftQuote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-DRAFT",
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, draftQuote); err != nil {
		t.Fatalf("Create draft quote failed: %v", err)
	}

	// Create sent quote
	sentQuote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-SENT",
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusSent,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(200.00),
		VATAmount:    decimal.NewFromFloat(40.00),
		Total:        decimal.NewFromFloat(240.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, sentQuote); err != nil {
		t.Fatalf("Create sent quote failed: %v", err)
	}

	// Filter by status
	filter := &QuoteFilter{Status: QuoteStatusSent}
	quotes, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}

	if len(quotes) != 1 {
		t.Errorf("expected 1 sent quote, got %d", len(quotes))
	}
	if quotes[0].Status != QuoteStatusSent {
		t.Errorf("expected status %s, got %s", QuoteStatusSent, quotes[0].Status)
	}
}

func TestRepository_UpdateQuote(t *testing.T) {
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

	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-UPDATE",
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update
	quote.Notes = "Updated notes"
	quote.Subtotal = decimal.NewFromFloat(150.00)
	quote.VATAmount = decimal.NewFromFloat(30.00)
	quote.Total = decimal.NewFromFloat(180.00)

	if err := repo.Update(ctx, tenant.SchemaName, quote); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, quote.ID)
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

	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-STATUS",
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update status
	if err := repo.UpdateStatus(ctx, tenant.SchemaName, tenant.ID, quote.ID, QuoteStatusSent); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, quote.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Status != QuoteStatusSent {
		t.Errorf("expected status %s, got %s", QuoteStatusSent, retrieved.Status)
	}
}

func TestRepository_DeleteQuote(t *testing.T) {
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

	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-DELETE",
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, quote.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.GetByID(ctx, tenant.SchemaName, tenant.ID, quote.ID)
	if err != ErrQuoteNotFound {
		t.Errorf("expected ErrQuoteNotFound after deletion, got %v", err)
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
	if num != "Q-00001" {
		t.Errorf("expected 'Q-00001', got '%s'", num)
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

	// Create a quote with this number
	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  num,
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Second number
	num2, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("GenerateNumber (second) failed: %v", err)
	}
	if num2 != "Q-00002" {
		t.Errorf("expected 'Q-00002', got '%s'", num2)
	}
}

func TestRepository_SetConvertedToOrder(t *testing.T) {
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

	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-CONVERT-ORDER",
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusAccepted,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create order first (required by foreign key constraint)
	orderID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.orders (id, tenant_id, order_number, contact_id, order_date, status, currency, exchange_rate, subtotal, vat_amount, total, created_at, created_by, updated_at)
		VALUES ($1, $2, 'ORD-001', $3, NOW(), 'PENDING', 'EUR', 1, 100.00, 20.00, 120.00, NOW(), $4, NOW())
	`, orderID, tenant.ID, contactID, uuid.New().String())
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	if err := repo.SetConvertedToOrder(ctx, tenant.SchemaName, tenant.ID, quote.ID, orderID); err != nil {
		t.Fatalf("SetConvertedToOrder failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, quote.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ConvertedToOrderID == nil || *retrieved.ConvertedToOrderID != orderID {
		t.Errorf("expected converted to order ID %s, got %v", orderID, retrieved.ConvertedToOrderID)
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

	quote := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-CONVERT-INV",
		ContactID:    contactID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusAccepted,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	invoiceID := uuid.New().String()
	if err := repo.SetConvertedToInvoice(ctx, tenant.SchemaName, tenant.ID, quote.ID, invoiceID); err != nil {
		t.Fatalf("SetConvertedToInvoice failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, quote.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ConvertedToInvoiceID == nil || *retrieved.ConvertedToInvoiceID != invoiceID {
		t.Errorf("expected converted to invoice ID %s, got %v", invoiceID, retrieved.ConvertedToInvoiceID)
	}
}

func TestRepository_ListQuotes_WithContactFilter(t *testing.T) {
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

	// Create quotes for different contacts
	quote1 := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-C1",
		ContactID:    contact1ID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(100.00),
		VATAmount:    decimal.NewFromFloat(20.00),
		Total:        decimal.NewFromFloat(120.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote1); err != nil {
		t.Fatalf("Create quote 1 failed: %v", err)
	}

	quote2 := &Quote{
		ID:           uuid.New().String(),
		TenantID:     tenant.ID,
		QuoteNumber:  "QUO-C2",
		ContactID:    contact2ID,
		QuoteDate:    time.Now(),
		Status:       QuoteStatusDraft,
		Currency:     "EUR",
		ExchangeRate: decimal.NewFromInt(1),
		Subtotal:     decimal.NewFromFloat(200.00),
		VATAmount:    decimal.NewFromFloat(40.00),
		Total:        decimal.NewFromFloat(240.00),
		CreatedAt:    time.Now(),
		CreatedBy:    uuid.New().String(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, quote2); err != nil {
		t.Fatalf("Create quote 2 failed: %v", err)
	}

	// Filter by contact
	filter := &QuoteFilter{ContactID: contact1ID}
	quotes, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List with contact filter failed: %v", err)
	}

	if len(quotes) != 1 {
		t.Errorf("expected 1 quote for contact 1, got %d", len(quotes))
	}
	if quotes[0].ContactID != contact1ID {
		t.Errorf("expected contact ID %s, got %s", contact1ID, quotes[0].ContactID)
	}
}
