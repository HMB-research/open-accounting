//go:build integration

package payments

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func TestGORMRepository_CreatePayment(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := uuid.New().String()
	payment := &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		PaymentNumber: "PMT-001",
		PaymentType:   PaymentTypeReceived,
		ContactID:     &contactID,
		Amount:        decimal.NewFromFloat(250.50),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.NewFromFloat(250.50),
		PaymentDate:   time.Now(),
		Reference:     "TEST-001",
		Notes:         "Test payment",
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}

	err = repo.Create(ctx, tenant.SchemaName, payment)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify payment was created
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, payment.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Reference != payment.Reference {
		t.Errorf("expected reference %s, got %s", payment.Reference, retrieved.Reference)
	}
	if !retrieved.Amount.Equal(payment.Amount) {
		t.Errorf("expected amount %s, got %s", payment.Amount, retrieved.Amount)
	}
}

func TestGORMRepository_List(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
	ctx := context.Background()

	// Create test contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := uuid.New().String()

	// Create two payments
	for i := 1; i <= 2; i++ {
		payment := &Payment{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			PaymentNumber: "PMT-00" + string(rune('0'+i)),
			PaymentType:   PaymentTypeReceived,
			ContactID:     &contactID,
			Amount:        decimal.NewFromFloat(float64(100 * i)),
			Currency:      "EUR",
			ExchangeRate:  decimal.NewFromInt(1),
			BaseAmount:    decimal.NewFromFloat(float64(100 * i)),
			PaymentDate:   time.Now(),
			CreatedAt:     time.Now(),
			CreatedBy:     userID,
		}
		if err := repo.Create(ctx, tenant.SchemaName, payment); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// List payments
	payments, err := repo.List(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(payments) < 2 {
		t.Errorf("expected at least 2 payments, got %d", len(payments))
	}
}

func TestGORMRepository_CreateAllocation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
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

	userID := testutil.CreateTestUser(t, pool, "payments-alloc-test@example.com")

	// Create a payment
	paymentID := uuid.New().String()
	payment := &Payment{
		ID:            paymentID,
		TenantID:      tenant.ID,
		PaymentNumber: "PMT-ALLOC-001",
		PaymentType:   PaymentTypeReceived,
		ContactID:     &contactID,
		Amount:        decimal.NewFromFloat(100),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.NewFromFloat(100),
		PaymentDate:   time.Now(),
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}

	if err := repo.Create(ctx, tenant.SchemaName, payment); err != nil {
		t.Fatalf("Create payment failed: %v", err)
	}

	// Create an invoice to allocate to
	invoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-ALLOC-001', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 100, 20, 120, 'SENT', $4, NOW(), NOW())
	`, invoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("failed to create test invoice: %v", err)
	}

	// Create allocation
	allocation := &PaymentAllocation{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		PaymentID: paymentID,
		InvoiceID: invoiceID,
		Amount:    decimal.NewFromFloat(50),
		CreatedAt: time.Now(),
	}

	if err := repo.CreateAllocation(ctx, tenant.SchemaName, allocation); err != nil {
		t.Fatalf("CreateAllocation failed: %v", err)
	}

	// Verify allocation
	allocations, err := repo.GetAllocations(ctx, tenant.SchemaName, tenant.ID, paymentID)
	if err != nil {
		t.Fatalf("GetAllocations failed: %v", err)
	}

	if len(allocations) != 1 {
		t.Errorf("expected 1 allocation, got %d", len(allocations))
	}

	if !allocations[0].Amount.Equal(allocation.Amount) {
		t.Errorf("expected allocation amount %s, got %s", allocation.Amount, allocations[0].Amount)
	}
}

func TestGORMRepository_CreateReversal(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
	ctx := context.Background()

	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Reversal Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create contact: %v", err)
	}
	userID := testutil.CreateTestUser(t, pool, "payments-reversal-test@example.com")
	invoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-REV-001', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 100, 20, 120, 60, 'PARTIALLY_PAID', $4, NOW(), NOW())
	`, invoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("failed to create invoice: %v", err)
	}

	original := &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		PaymentNumber: "PMT-REV-001",
		PaymentType:   PaymentTypeReceived,
		ContactID:     &contactID,
		Amount:        decimal.RequireFromString("120.00"),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.RequireFromString("120.00"),
		PaymentDate:   time.Now(),
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}
	if err := repo.Create(ctx, tenant.SchemaName, original); err != nil {
		t.Fatalf("Create original failed: %v", err)
	}
	originalAllocation := &PaymentAllocation{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		PaymentID: original.ID,
		InvoiceID: invoiceID,
		Amount:    decimal.RequireFromString("60.00"),
		CreatedAt: time.Now(),
	}
	if err := repo.CreateAllocation(ctx, tenant.SchemaName, originalAllocation); err != nil {
		t.Fatalf("CreateAllocation original failed: %v", err)
	}

	reversalOfPaymentID := original.ID
	reversal := &Payment{
		ID:                  uuid.New().String(),
		TenantID:            tenant.ID,
		PaymentNumber:       "OUT-REV-001",
		PaymentType:         PaymentTypeMade,
		ContactID:           &contactID,
		Amount:              original.Amount,
		Currency:            "EUR",
		ExchangeRate:        decimal.NewFromInt(1),
		BaseAmount:          original.BaseAmount,
		PaymentDate:         time.Now(),
		Reference:           "REVERSAL-PMT-REV-001",
		ReversalOfPaymentID: &reversalOfPaymentID,
		ReversalReason:      "Duplicate bank import",
		CreatedAt:           time.Now(),
		CreatedBy:           userID,
	}
	reversalAllocations := []PaymentAllocation{{
		ID:        uuid.New().String(),
		TenantID:  tenant.ID,
		PaymentID: reversal.ID,
		InvoiceID: invoiceID,
		Amount:    originalAllocation.Amount,
		CreatedAt: time.Now(),
	}}
	reversedAt := time.Now()
	if err := repo.CreateReversal(ctx, tenant.SchemaName, original.ID, reversal, reversalAllocations, reversedAt, userID, "Duplicate bank import"); err != nil {
		t.Fatalf("CreateReversal failed: %v", err)
	}

	retrievedOriginal, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, original.ID)
	if err != nil {
		t.Fatalf("GetByID original failed: %v", err)
	}
	if retrievedOriginal.ReversedByPaymentID == nil || *retrievedOriginal.ReversedByPaymentID != reversal.ID {
		t.Fatalf("expected original reversed_by_payment_id %s, got %v", reversal.ID, retrievedOriginal.ReversedByPaymentID)
	}
	if retrievedOriginal.ReversalReason != "Duplicate bank import" {
		t.Errorf("expected original reversal reason, got %q", retrievedOriginal.ReversalReason)
	}

	retrievedReversal, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, reversal.ID)
	if err != nil {
		t.Fatalf("GetByID reversal failed: %v", err)
	}
	if retrievedReversal.ReversalOfPaymentID == nil || *retrievedReversal.ReversalOfPaymentID != original.ID {
		t.Fatalf("expected reversal_of_payment_id %s, got %v", original.ID, retrievedReversal.ReversalOfPaymentID)
	}
	allocations, err := repo.GetAllocations(ctx, tenant.SchemaName, tenant.ID, reversal.ID)
	if err != nil {
		t.Fatalf("GetAllocations reversal failed: %v", err)
	}
	if len(allocations) != 1 {
		t.Fatalf("expected one reversal allocation, got %d", len(allocations))
	}
	if allocations[0].InvoiceID != invoiceID {
		t.Errorf("expected invoice allocation %s, got %s", invoiceID, allocations[0].InvoiceID)
	}
	if !allocations[0].Amount.Equal(decimal.RequireFromString("60.00")) {
		t.Errorf("expected allocation amount 60.00, got %s", allocations[0].Amount)
	}

	err = repo.CreateReversal(ctx, tenant.SchemaName, original.ID, &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		PaymentNumber: "OUT-REV-002",
		PaymentType:   PaymentTypeMade,
		Amount:        original.Amount,
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    original.BaseAmount,
		PaymentDate:   time.Now(),
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}, nil, time.Now(), userID, "Second reversal")
	if err != ErrPaymentAlreadyReversed {
		t.Fatalf("expected ErrPaymentAlreadyReversed, got %v", err)
	}
}

func TestGORMRepository_GetNextPaymentNumber(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
	ctx := context.Background()

	// Get first payment number
	num1, err := repo.GetNextPaymentNumber(ctx, tenant.SchemaName, tenant.ID, PaymentTypeReceived)
	if err != nil {
		t.Fatalf("GetNextPaymentNumber failed: %v", err)
	}

	if num1 == 0 {
		t.Error("expected non-zero payment number")
	}

	// Create a payment with that number
	contactID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "payments-nextnum-test@example.com")

	paymentNumber := "PMT-" + string(rune('0'+num1))
	payment := &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		PaymentNumber: paymentNumber,
		PaymentType:   PaymentTypeReceived,
		ContactID:     &contactID,
		Amount:        decimal.NewFromFloat(100),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.NewFromFloat(100),
		PaymentDate:   time.Now(),
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}

	if err := repo.Create(ctx, tenant.SchemaName, payment); err != nil {
		t.Fatalf("Create payment failed: %v", err)
	}

	// Get second number - should be different
	num2, err := repo.GetNextPaymentNumber(ctx, tenant.SchemaName, tenant.ID, PaymentTypeReceived)
	if err != nil {
		t.Fatalf("GetNextPaymentNumber failed: %v", err)
	}

	if num1 == num2 {
		t.Errorf("expected different payment numbers, got same: %d", num1)
	}
}

func TestPaymentNumberSequence(t *testing.T) {
	tests := []struct {
		name          string
		paymentNumber string
		prefix        string
		want          int
		wantOK        bool
	}{
		{name: "generated payment number", paymentNumber: "PMT-00042", prefix: "PMT", want: 42, wantOK: true},
		{name: "generated outgoing number", paymentNumber: "OUT-00007", prefix: "OUT", want: 7, wantOK: true},
		{name: "ignores non-matching prefix", paymentNumber: "OUT-00007", prefix: "PMT", wantOK: false},
		{name: "preserves legacy leading digit extraction", paymentNumber: "PMT-00008-adjusted", prefix: "PMT", want: 8, wantOK: true},
		{name: "rejects missing digits", paymentNumber: "PMT-adjusted", prefix: "PMT", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := paymentNumberSequence(tt.paymentNumber, tt.prefix)
			if ok != tt.wantOK {
				t.Fatalf("expected ok=%v, got %v", tt.wantOK, ok)
			}
			if got != tt.want {
				t.Fatalf("expected sequence %d, got %d", tt.want, got)
			}
		})
	}
}

func TestGORMRepository_GetUnallocatedPayments(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
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

	userID := testutil.CreateTestUser(t, pool, "payments-unalloc-test@example.com")

	// Create a payment that should be unallocated
	payment := &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		PaymentNumber: "PMT-UNALLOC-001",
		PaymentType:   PaymentTypeReceived,
		ContactID:     &contactID,
		Amount:        decimal.NewFromFloat(200),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.NewFromFloat(200),
		PaymentDate:   time.Now(),
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}

	if err := repo.Create(ctx, tenant.SchemaName, payment); err != nil {
		t.Fatalf("Create payment failed: %v", err)
	}

	// Get unallocated payments (by payment type, not contact ID)
	unallocated, err := repo.GetUnallocatedPayments(ctx, tenant.SchemaName, tenant.ID, PaymentTypeReceived)
	if err != nil {
		t.Fatalf("GetUnallocatedPayments failed: %v", err)
	}

	if len(unallocated) < 1 {
		t.Errorf("expected at least 1 unallocated payment, got %d", len(unallocated))
	}

	// Verify our payment is in the list
	found := false
	for _, p := range unallocated {
		if p.ID == payment.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find our payment in unallocated list")
	}
}

func TestGORMRepository_ListWithFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
	ctx := context.Background()

	// Create test contacts
	customerID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, customerID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test customer: %v", err)
	}

	supplierID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'SUPPLIER', 'Test Supplier', NOW(), NOW())
	`, supplierID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test supplier: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "payments-filter-test@example.com")

	// Create payments with different types and dates
	now := time.Now()
	lastWeek := now.AddDate(0, 0, -7)
	lastMonth := now.AddDate(0, -1, 0)

	payments := []struct {
		paymentType PaymentType
		contactID   string
		date        time.Time
	}{
		{PaymentTypeReceived, customerID, now},
		{PaymentTypeReceived, customerID, lastWeek},
		{PaymentTypeMade, supplierID, now},
		{PaymentTypeMade, supplierID, lastMonth},
	}

	for i, p := range payments {
		payment := &Payment{
			ID:            uuid.New().String(),
			TenantID:      tenant.ID,
			PaymentNumber: "PMT-FILTER-" + string(rune('0'+i+1)),
			PaymentType:   p.paymentType,
			ContactID:     &p.contactID,
			Amount:        decimal.NewFromFloat(float64((i + 1) * 100)),
			Currency:      "EUR",
			ExchangeRate:  decimal.NewFromInt(1),
			BaseAmount:    decimal.NewFromFloat(float64((i + 1) * 100)),
			PaymentDate:   p.date,
			CreatedAt:     time.Now(),
			CreatedBy:     userID,
		}
		if err := repo.Create(ctx, tenant.SchemaName, payment); err != nil {
			t.Fatalf("Create payment failed: %v", err)
		}
	}

	// Test filter by payment type
	t.Run("FilterByPaymentType", func(t *testing.T) {
		filter := &PaymentFilter{PaymentType: PaymentTypeReceived}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		for _, r := range results {
			if r.PaymentType != PaymentTypeReceived {
				t.Errorf("expected payment type RECEIVED, got %s", r.PaymentType)
			}
		}
	})

	// Test filter by contact ID
	t.Run("FilterByContactID", func(t *testing.T) {
		filter := &PaymentFilter{ContactID: customerID}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		for _, r := range results {
			if *r.ContactID != customerID {
				t.Errorf("expected contact ID %s, got %s", customerID, *r.ContactID)
			}
		}
	})

	// Test filter by date range
	t.Run("FilterByDateRange", func(t *testing.T) {
		fromDate := now.AddDate(0, 0, -10) // 10 days ago
		toDate := now.AddDate(0, 0, 1)     // tomorrow
		filter := &PaymentFilter{FromDate: &fromDate, ToDate: &toDate}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		// Should include payments from now and lastWeek, but not lastMonth
		if len(results) < 2 {
			t.Errorf("expected at least 2 payments in date range, got %d", len(results))
		}
	})

	// Test combined filters
	t.Run("CombinedFilters", func(t *testing.T) {
		fromDate := now.AddDate(0, 0, -1) // yesterday
		filter := &PaymentFilter{
			PaymentType: PaymentTypeReceived,
			ContactID:   customerID,
			FromDate:    &fromDate,
		}
		results, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		// Should only match the first payment (RECEIVED, customerID, today)
		if len(results) != 1 {
			t.Errorf("expected 1 payment with combined filters, got %d", len(results))
		}
	})
}

func TestGORMRepository_GetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
	ctx := context.Background()

	// Try to get a non-existent payment
	_, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrPaymentNotFound {
		t.Errorf("expected ErrPaymentNotFound, got %v", err)
	}
}

func TestGORMRepository_GetAllocations_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPaymentsGORMRepository(t, pool)
	ctx := context.Background()

	// Create a contact and payment with no allocations
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Test Customer', NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create test contact: %v", err)
	}

	userID := testutil.CreateTestUser(t, pool, "payments-empty-alloc-test@example.com")

	paymentID := uuid.New().String()
	payment := &Payment{
		ID:            paymentID,
		TenantID:      tenant.ID,
		PaymentNumber: "PMT-EMPTY-ALLOC-001",
		PaymentType:   PaymentTypeReceived,
		ContactID:     &contactID,
		Amount:        decimal.NewFromFloat(100),
		Currency:      "EUR",
		ExchangeRate:  decimal.NewFromInt(1),
		BaseAmount:    decimal.NewFromFloat(100),
		PaymentDate:   time.Now(),
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
	}

	if err := repo.Create(ctx, tenant.SchemaName, payment); err != nil {
		t.Fatalf("Create payment failed: %v", err)
	}

	// Get allocations for payment with no allocations
	allocations, err := repo.GetAllocations(ctx, tenant.SchemaName, tenant.ID, paymentID)
	if err != nil {
		t.Fatalf("GetAllocations failed: %v", err)
	}

	if len(allocations) != 0 {
		t.Errorf("expected 0 allocations for new payment, got %d", len(allocations))
	}
}

func newPaymentsGORMRepository(t *testing.T, pool *pgxpool.Pool) *GORMRepository {
	t.Helper()

	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		t.Fatalf("create gorm repository: %v", err)
	}
	return NewGORMRepository(gormDB)
}
