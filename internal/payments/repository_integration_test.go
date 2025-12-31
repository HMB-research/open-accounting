//go:build integration

package payments

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_GetUnallocatedPayments(t *testing.T) {
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

	// Create cash account
	cashAccountID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts (id, tenant_id, code, name, type, is_system, created_at, updated_at)
		VALUES ($1, $2, '1001', 'Cash', 'asset', false, NOW(), NOW())
	`, cashAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create cash account: %v", err)
	}

	// Create test payments - one fully allocated, one unallocated
	unallocatedPaymentID := uuid.New().String()
	allocatedPaymentID := uuid.New().String()

	// Unallocated payment
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, type, contact_id, amount, currency, payment_date, account_id, allocated_amount, status, created_at, updated_at)
		VALUES ($1, $2, 'received', $3, 1000.00, 'EUR', $4, $5, 0.00, 'completed', NOW(), NOW())
	`, unallocatedPaymentID, tenant.ID, contactID, time.Now(), cashAccountID)
	if err != nil {
		t.Fatalf("failed to create unallocated payment: %v", err)
	}

	// Fully allocated payment
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, type, contact_id, amount, currency, payment_date, account_id, allocated_amount, status, created_at, updated_at)
		VALUES ($1, $2, 'received', $3, 500.00, 'EUR', $4, $5, 500.00, 'completed', NOW(), NOW())
	`, allocatedPaymentID, tenant.ID, contactID, time.Now(), cashAccountID)
	if err != nil {
		t.Fatalf("failed to create allocated payment: %v", err)
	}

	// Test GetUnallocatedPayments
	payments, err := repo.GetUnallocatedPayments(ctx, tenant.SchemaName, tenant.ID, PaymentTypeReceived)
	if err != nil {
		t.Fatalf("GetUnallocatedPayments failed: %v", err)
	}

	// Should only return the unallocated payment
	if len(payments) != 1 {
		t.Errorf("expected 1 unallocated payment, got %d", len(payments))
	}

	if len(payments) > 0 {
		if payments[0].ID != unallocatedPaymentID {
			t.Errorf("expected payment ID %s, got %s", unallocatedPaymentID, payments[0].ID)
		}
		expectedAmount := decimal.NewFromFloat(1000.00)
		if !payments[0].Amount.Equal(expectedAmount) {
			t.Errorf("expected amount %s, got %s", expectedAmount, payments[0].Amount)
		}
	}
}

func TestPostgresRepository_CreatePayment(t *testing.T) {
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

	// Create cash account
	cashAccountID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.accounts (id, tenant_id, code, name, type, is_system, created_at, updated_at)
		VALUES ($1, $2, '1001', 'Cash', 'asset', false, NOW(), NOW())
	`, cashAccountID, tenant.ID)
	if err != nil {
		t.Fatalf("failed to create cash account: %v", err)
	}

	payment := &Payment{
		ID:              uuid.New().String(),
		TenantID:        tenant.ID,
		Type:            PaymentTypeReceived,
		ContactID:       contactID,
		Amount:          decimal.NewFromFloat(250.50),
		Currency:        "EUR",
		PaymentDate:     time.Now(),
		AccountID:       cashAccountID,
		Reference:       "TEST-001",
		Description:     "Test payment",
		AllocatedAmount: decimal.Zero,
		Status:          "completed",
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
