package invoicing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestInterestServiceLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	service := NewInterestService(pool)
	ctx := context.Background()

	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Interest Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "interest-service@example.com")
	invoiceID := uuid.New().String()
	dueDate := time.Now().AddDate(0, 0, -10)

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-INT-001', 'SALES', $3, NOW(), $4, 'EUR', 100.00, 20.00, 120.00, 20.00, 'OVERDUE', $5, NOW(), NOW())
	`, invoiceID, tenant.ID, contactID, dueDate, userID)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}

	result, err := service.CalculateInterest(ctx, tenant.SchemaName, tenant.ID, invoiceID, 0.001, time.Now())
	if err != nil {
		t.Fatalf("CalculateInterest failed: %v", err)
	}
	if result.InvoiceID != invoiceID || !result.OutstandingAmount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("unexpected interest result: %+v", result)
	}
	if result.DaysOverdue <= 0 || result.TotalInterest.LessThanOrEqual(decimal.Zero) {
		t.Fatalf("expected positive overdue interest, got %+v", result)
	}

	saved, err := service.SaveInterestCalculation(ctx, tenant.SchemaName, result)
	if err != nil {
		t.Fatalf("SaveInterestCalculation failed: %v", err)
	}
	if saved.ID == "" {
		t.Fatal("expected saved interest calculation ID")
	}

	latest, err := service.GetLatestInterest(ctx, tenant.SchemaName, invoiceID)
	if err != nil {
		t.Fatalf("GetLatestInterest failed: %v", err)
	}
	if latest == nil || latest.InvoiceID != invoiceID {
		t.Fatalf("unexpected latest interest: %+v", latest)
	}

	history, err := service.ListInterestHistory(ctx, tenant.SchemaName, invoiceID)
	if err != nil {
		t.Fatalf("ListInterestHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected one interest history entry, got %d", len(history))
	}

	allResults, err := service.CalculateInterestForOverdueInvoices(ctx, tenant.SchemaName, tenant.ID, 0.001)
	if err != nil {
		t.Fatalf("CalculateInterestForOverdueInvoices failed: %v", err)
	}
	if len(allResults) != 1 || allResults[0].InvoiceID != invoiceID {
		t.Fatalf("unexpected overdue interest results: %+v", allResults)
	}
}
