//go:build integration

package invoicing

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_UpdateOverdueStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, type, name, created_at, updated_at)
		VALUES ($1, $2, 'customer', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	// Create an overdue invoice (due date in the past, still in draft/sent status)
	overdueInvoiceID := uuid.New().String()
	pastDate := time.Now().AddDate(0, 0, -10) // 10 days ago

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, type, contact_id, issue_date, due_date, currency, subtotal, tax_total, total, status, created_at, updated_at)
		VALUES ($1, $2, 'INV-001', 'sales', $3, $4, $5, 'EUR', 100.00, 20.00, 120.00, 'sent', NOW(), NOW())
	`, overdueInvoiceID, tenant.ID, contactID, pastDate, pastDate)
	if err != nil {
		t.Fatalf("failed to create overdue invoice: %v", err)
	}

	// Create a non-overdue invoice (due date in the future)
	currentInvoiceID := uuid.New().String()
	futureDate := time.Now().AddDate(0, 0, 30) // 30 days from now

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, type, contact_id, issue_date, due_date, currency, subtotal, tax_total, total, status, created_at, updated_at)
		VALUES ($1, $2, 'INV-002', 'sales', $3, NOW(), $4, 'EUR', 200.00, 40.00, 240.00, 'sent', NOW(), NOW())
	`, currentInvoiceID, tenant.ID, contactID, futureDate)
	if err != nil {
		t.Fatalf("failed to create current invoice: %v", err)
	}

	// Run UpdateOverdueStatus
	count, err := repo.UpdateOverdueStatus(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("UpdateOverdueStatus failed: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 invoice updated to overdue, got %d", count)
	}

	// Verify the overdue invoice was updated
	var status string
	err = pool.QueryRow(ctx, `
		SELECT status FROM `+tenant.SchemaName+`.invoices WHERE id = $1
	`, overdueInvoiceID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query invoice status: %v", err)
	}

	if status != "overdue" {
		t.Errorf("expected status 'overdue', got '%s'", status)
	}

	// Verify the non-overdue invoice was not changed
	err = pool.QueryRow(ctx, `
		SELECT status FROM `+tenant.SchemaName+`.invoices WHERE id = $1
	`, currentInvoiceID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query current invoice status: %v", err)
	}

	if status != "sent" {
		t.Errorf("expected status 'sent' for current invoice, got '%s'", status)
	}
}

func TestPostgresRepository_Create(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, type, name, created_at, updated_at)
		VALUES ($1, $2, 'customer', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	invoice := &Invoice{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		InvoiceNumber: "INV-TEST-001",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     contactID,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(100),
		VATAmount:     decimal.NewFromFloat(20),
		Total:         decimal.NewFromFloat(120),
		Status:        StatusDraft,
	}

	err = repo.Create(ctx, tenant.SchemaName, invoice)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify invoice was created
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, invoice.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.InvoiceNumber != invoice.InvoiceNumber {
		t.Errorf("expected invoice number %s, got %s", invoice.InvoiceNumber, retrieved.InvoiceNumber)
	}
}
