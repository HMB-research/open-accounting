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
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	// Create a test user for created_by field
	userID := testutil.CreateTestUser(t, pool, "invoicing-test@example.com")

	// Create an overdue invoice (due date in the past, still in draft/sent status)
	overdueInvoiceID := uuid.New().String()
	pastDate := time.Now().AddDate(0, 0, -10) // 10 days ago

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-001', 'SALES', $3, $4, $5, 'EUR', 100.00, 20.00, 120.00, 'SENT', $6, NOW(), NOW())
	`, overdueInvoiceID, tenant.ID, contactID, pastDate, pastDate, userID)
	if err != nil {
		t.Fatalf("failed to create overdue invoice: %v", err)
	}

	// Create a non-overdue invoice (due date in the future)
	currentInvoiceID := uuid.New().String()
	futureDate := time.Now().AddDate(0, 0, 30) // 30 days from now

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-002', 'SALES', $3, NOW(), $4, 'EUR', 200.00, 40.00, 240.00, 'SENT', $5, NOW(), NOW())
	`, currentInvoiceID, tenant.ID, contactID, futureDate, userID)
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

	if status != "OVERDUE" {
		t.Errorf("expected status 'OVERDUE', got '%s'", status)
	}

	// Verify the non-overdue invoice was not changed
	err = pool.QueryRow(ctx, `
		SELECT status FROM `+tenant.SchemaName+`.invoices WHERE id = $1
	`, currentInvoiceID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to query current invoice status: %v", err)
	}

	if status != "SENT" {
		t.Errorf("expected status 'SENT' for current invoice, got '%s'", status)
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
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	// Create a test user for created_by field
	userID := testutil.CreateTestUser(t, pool, "invoicing-create-test@example.com")

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
		CreatedBy:     userID,
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

func TestPostgresRepository_List(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "invoicing-list-test@example.com")

	// Create multiple invoices
	for i := 1; i <= 3; i++ {
		invoice := &Invoice{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			InvoiceNumber: "INV-LIST-" + string(rune('0'+i)),
			InvoiceType:   InvoiceTypeSales,
			ContactID:     contactID,
			IssueDate:     time.Now(),
			DueDate:       time.Now().AddDate(0, 0, 30),
			Currency:      "EUR",
			Subtotal:      decimal.NewFromInt(int64(i * 100)),
			VATAmount:     decimal.NewFromInt(int64(i * 20)),
			Total:         decimal.NewFromInt(int64(i * 120)),
			Status:        StatusDraft,
			CreatedBy:     userID,
		}
		if err := repo.Create(ctx, tenant.SchemaName, invoice); err != nil {
			t.Fatalf("Create failed for invoice %d: %v", i, err)
		}
	}

	// List all invoices
	invoices, err := repo.List(ctx, tenant.SchemaName, tenant.ID, &InvoiceFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(invoices) < 3 {
		t.Errorf("expected at least 3 invoices, got %d", len(invoices))
	}
}

func TestPostgresRepository_UpdateStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "invoicing-status-test@example.com")

	invoice := &Invoice{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		InvoiceNumber: "INV-STATUS-001",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     contactID,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(100),
		VATAmount:     decimal.NewFromFloat(20),
		Total:         decimal.NewFromFloat(120),
		Status:        StatusDraft,
		CreatedBy:     userID,
	}

	if err := repo.Create(ctx, tenant.SchemaName, invoice); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update status to SENT
	if err := repo.UpdateStatus(ctx, tenant.SchemaName, tenant.ID, invoice.ID, StatusSent); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Verify status was updated
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, invoice.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Status != StatusSent {
		t.Errorf("expected status %s, got %s", StatusSent, retrieved.Status)
	}
}

func TestPostgresRepository_UpdatePayment(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "invoicing-payment-test@example.com")

	invoice := &Invoice{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		InvoiceNumber: "INV-PAY-001",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     contactID,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(100),
		VATAmount:     decimal.NewFromFloat(20),
		Total:         decimal.NewFromFloat(120),
		AmountPaid:    decimal.Zero,
		Status:        StatusSent,
		CreatedBy:     userID,
	}

	if err := repo.Create(ctx, tenant.SchemaName, invoice); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update payment
	paidAmount := decimal.NewFromFloat(50)
	if err := repo.UpdatePayment(ctx, tenant.SchemaName, tenant.ID, invoice.ID, paidAmount, StatusPartiallyPaid); err != nil {
		t.Fatalf("UpdatePayment failed: %v", err)
	}

	// Verify payment was updated
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, invoice.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if !retrieved.AmountPaid.Equal(paidAmount) {
		t.Errorf("expected amount paid %s, got %s", paidAmount, retrieved.AmountPaid)
	}

	if retrieved.Status != StatusPartiallyPaid {
		t.Errorf("expected status %s, got %s", StatusPartiallyPaid, retrieved.Status)
	}
}

func TestPostgresRepository_GenerateNumber(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "invoicing-gennumber-test@example.com")

	// Generate first invoice number - should be INV-00001 (no existing invoices)
	num1, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID, InvoiceTypeSales)
	if err != nil {
		t.Fatalf("GenerateNumber failed: %v", err)
	}

	if num1 == "" {
		t.Error("expected non-empty invoice number")
	}

	// Create an invoice with that number
	invoice := &Invoice{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		InvoiceNumber: num1,
		InvoiceType:   InvoiceTypeSales,
		ContactID:     contactID,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(100),
		VATAmount:     decimal.NewFromFloat(20),
		Total:         decimal.NewFromFloat(120),
		Status:        StatusDraft,
		CreatedBy:     userID,
	}

	if err := repo.Create(ctx, tenant.SchemaName, invoice); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Generate second number - should be INV-00002 now
	num2, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID, InvoiceTypeSales)
	if err != nil {
		t.Fatalf("GenerateNumber failed: %v", err)
	}

	if num2 == "" {
		t.Error("expected non-empty invoice number")
	}

	// They should be different (sequential)
	if num1 == num2 {
		t.Errorf("expected different numbers after creating invoice, got same: %s", num1)
	}

	// Test different invoice type gets different prefix
	purchaseNum, err := repo.GenerateNumber(ctx, tenant.SchemaName, tenant.ID, InvoiceTypePurchase)
	if err != nil {
		t.Fatalf("GenerateNumber for purchase failed: %v", err)
	}

	if purchaseNum == "" {
		t.Error("expected non-empty purchase number")
	}

	// Should have BILL prefix
	if purchaseNum != "BILL-00001" {
		t.Errorf("expected BILL-00001, got %s", purchaseNum)
	}
}

func TestPostgresRepository_CreateWithLines(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "invoicing-lines-test@example.com")

	invoiceID := uuid.New().String()
	invoice := &Invoice{
		ID:            invoiceID,
		TenantID:      tenant.ID,
		InvoiceNumber: "INV-LINES-001",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     contactID,
		IssueDate:     time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(200),
		VATAmount:     decimal.NewFromFloat(40),
		Total:         decimal.NewFromFloat(240),
		Status:        StatusDraft,
		CreatedBy:     userID,
		Lines: []InvoiceLine{
			{
				ID:           uuid.New().String(),
				TenantID:     tenant.ID,
				LineNumber:   1,
				Description:  "Service A",
				Quantity:     decimal.NewFromInt(1),
				UnitPrice:    decimal.NewFromFloat(100),
				VATRate:      decimal.NewFromFloat(20),
				LineSubtotal: decimal.NewFromFloat(100),
				LineVAT:      decimal.NewFromFloat(20),
				LineTotal:    decimal.NewFromFloat(120),
			},
			{
				ID:           uuid.New().String(),
				TenantID:     tenant.ID,
				LineNumber:   2,
				Description:  "Service B",
				Quantity:     decimal.NewFromInt(1),
				UnitPrice:    decimal.NewFromFloat(100),
				VATRate:      decimal.NewFromFloat(20),
				LineSubtotal: decimal.NewFromFloat(100),
				LineVAT:      decimal.NewFromFloat(20),
				LineTotal:    decimal.NewFromFloat(120),
			},
		},
	}

	err = repo.Create(ctx, tenant.SchemaName, invoice)
	if err != nil {
		t.Fatalf("Create with lines failed: %v", err)
	}

	// Verify invoice and lines were created
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, invoiceID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if len(retrieved.Lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(retrieved.Lines))
	}

	if len(retrieved.Lines) > 0 && retrieved.Lines[0].Description != "Service A" {
		t.Errorf("expected first line description 'Service A', got '%s'", retrieved.Lines[0].Description)
	}
}

func TestPostgresRepository_GetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to get a non-existent invoice
	_, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrInvoiceNotFound {
		t.Errorf("expected ErrInvoiceNotFound, got %v", err)
	}
}

func TestPostgresRepository_ListWithFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contacts
	customerID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Customer A', NOW(), NOW())
	`, customerID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create customer contact: %v", err)
	}

	supplierID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'SUPPLIER', 'Supplier B', NOW(), NOW())
	`, supplierID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create supplier contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "invoicing-filters-test@example.com")

	// Create invoices with different types and statuses
	invoices := []struct {
		number      string
		invoiceType InvoiceType
		contactID   string
		status      InvoiceStatus
	}{
		{"INV-FILT-001", InvoiceTypeSales, customerID, StatusDraft},
		{"INV-FILT-002", InvoiceTypeSales, customerID, StatusSent},
		{"BILL-FILT-001", InvoiceTypePurchase, supplierID, StatusDraft},
	}

	for _, inv := range invoices {
		invoice := &Invoice{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			InvoiceNumber: inv.number,
			InvoiceType:   inv.invoiceType,
			ContactID:     inv.contactID,
			IssueDate:     time.Now(),
			DueDate:       time.Now().AddDate(0, 0, 30),
			Currency:      "EUR",
			Subtotal:      decimal.NewFromFloat(100),
			VATAmount:     decimal.NewFromFloat(20),
			Total:         decimal.NewFromFloat(120),
			Status:        inv.status,
			CreatedBy:     userID,
		}
		if err := repo.Create(ctx, tenant.SchemaName, invoice); err != nil {
			t.Fatalf("Create failed for %s: %v", inv.number, err)
		}
	}

	// Test filter by invoice type
	t.Run("FilterByType", func(t *testing.T) {
		filter := &InvoiceFilter{InvoiceType: InvoiceTypeSales}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		for _, r := range results {
			if r.InvoiceType != InvoiceTypeSales {
				t.Errorf("expected invoice type SALES, got %s", r.InvoiceType)
			}
		}
	})

	// Test filter by status
	t.Run("FilterByStatus", func(t *testing.T) {
		filter := &InvoiceFilter{Status: StatusDraft}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		for _, r := range results {
			if r.Status != StatusDraft {
				t.Errorf("expected status DRAFT, got %s", r.Status)
			}
		}
	})

	// Test filter by contact ID
	t.Run("FilterByContactID", func(t *testing.T) {
		filter := &InvoiceFilter{ContactID: customerID}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		for _, r := range results {
			if r.ContactID != customerID {
				t.Errorf("expected contact ID %s, got %s", customerID, r.ContactID)
			}
		}
	})
}

func TestPostgresRepository_UpdateStatus_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to update status of non-existent invoice
	err := repo.UpdateStatus(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), StatusSent)
	if err != ErrInvoiceNotFound {
		t.Errorf("expected ErrInvoiceNotFound, got %v", err)
	}
}

func TestPostgresRepository_UpdatePayment_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Try to update payment on non-existent invoice
	err := repo.UpdatePayment(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), decimal.NewFromFloat(50), StatusPartiallyPaid)
	if err != ErrInvoiceNotFound {
		t.Errorf("expected ErrInvoiceNotFound, got %v", err)
	}
}

func TestPostgresRepository_ListWithDateRangeAndSearch(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Search Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "invoicing-daterange-test@example.com")

	// Create invoices with different dates
	pastDate := time.Now().AddDate(0, -1, 0)  // 1 month ago
	futureDate := time.Now().AddDate(0, 1, 0) // 1 month from now

	// Invoice in the past
	pastInvoice := &Invoice{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		InvoiceNumber: "INV-PAST-001",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     contactID,
		IssueDate:     pastDate,
		DueDate:       pastDate.AddDate(0, 0, 30),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(100),
		VATAmount:     decimal.NewFromFloat(20),
		Total:         decimal.NewFromFloat(120),
		Status:        StatusDraft,
		Reference:     "REF-SEARCHME-123",
		CreatedBy:     userID,
	}
	if err := repo.Create(ctx, tenant.SchemaName, pastInvoice); err != nil {
		t.Fatalf("Create past invoice failed: %v", err)
	}

	// Invoice in the future
	futureInvoice := &Invoice{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		InvoiceNumber: "INV-FUTURE-001",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     contactID,
		IssueDate:     futureDate,
		DueDate:       futureDate.AddDate(0, 0, 30),
		Currency:      "EUR",
		Subtotal:      decimal.NewFromFloat(200),
		VATAmount:     decimal.NewFromFloat(40),
		Total:         decimal.NewFromFloat(240),
		Status:        StatusDraft,
		CreatedBy:     userID,
	}
	if err := repo.Create(ctx, tenant.SchemaName, futureInvoice); err != nil {
		t.Fatalf("Create future invoice failed: %v", err)
	}

	// Test filter by FromDate
	t.Run("FilterByFromDate", func(t *testing.T) {
		today := time.Now()
		filter := &InvoiceFilter{FromDate: &today}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		// Should include future invoice
		found := false
		for _, r := range results {
			if r.ID == futureInvoice.ID {
				found = true
			}
		}
		if !found {
			t.Error("expected future invoice in results when filtering from today")
		}
	})

	// Test filter by ToDate
	t.Run("FilterByToDate", func(t *testing.T) {
		today := time.Now()
		filter := &InvoiceFilter{ToDate: &today}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		// Should include past invoice
		found := false
		for _, r := range results {
			if r.ID == pastInvoice.ID {
				found = true
			}
		}
		if !found {
			t.Error("expected past invoice in results when filtering to today")
		}
	})

	// Test filter by Search (invoice number)
	t.Run("FilterBySearchNumber", func(t *testing.T) {
		filter := &InvoiceFilter{Search: "PAST"}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		found := false
		for _, r := range results {
			if r.InvoiceNumber == "INV-PAST-001" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find invoice with 'PAST' in number")
		}
	})

	// Test filter by Search (reference)
	t.Run("FilterBySearchReference", func(t *testing.T) {
		filter := &InvoiceFilter{Search: "SEARCHME"}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		found := false
		for _, r := range results {
			if r.Reference == "REF-SEARCHME-123" {
				found = true
			}
		}
		if !found {
			t.Error("expected to find invoice with 'SEARCHME' in reference")
		}
	})
}
