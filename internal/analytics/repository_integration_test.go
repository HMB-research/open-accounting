//go:build integration

package analytics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_GetReceivablesSummary(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'C001', 'Test Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a SALES invoice (receivable)
	invoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-AR-001', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 100, 20, 120, 0, 'SENT', $4, NOW(), NOW())
	`, invoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create test invoice: %v", err)
	}

	// Get receivables summary
	total, overdue, err := repo.GetReceivablesSummary(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetReceivablesSummary failed: %v", err)
	}

	expectedTotal := decimal.NewFromFloat(120)
	if !total.Equal(expectedTotal) {
		t.Errorf("expected total %s, got %s", expectedTotal, total)
	}
	if !overdue.Equal(decimal.Zero) {
		t.Errorf("expected overdue 0, got %s", overdue)
	}
}

func TestPostgresRepository_GetPayablesSummary(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-payables-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a vendor contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'V001', 'Test Vendor', 'SUPPLIER', 'EE', 30, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a PURCHASE invoice (payable)
	invoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'BILL-AP-001', 'PURCHASE', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 200, 40, 240, 100, 'SENT', $4, NOW(), NOW())
	`, invoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create test invoice: %v", err)
	}

	// Get payables summary (total - amount_paid = 240 - 100 = 140)
	total, overdue, err := repo.GetPayablesSummary(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetPayablesSummary failed: %v", err)
	}

	expectedTotal := decimal.NewFromFloat(140)
	if !total.Equal(expectedTotal) {
		t.Errorf("expected total %s, got %s", expectedTotal, total)
	}
	if !overdue.Equal(decimal.Zero) {
		t.Errorf("expected overdue 0, got %s", overdue)
	}
}

func TestPostgresRepository_GetInvoiceCounts(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-counts-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'C002', 'Invoice Count Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create invoices with different statuses
	invoices := []struct {
		number  string
		status  string
		overdue bool
	}{
		{"INV-COUNT-001", "DRAFT", false},
		{"INV-COUNT-002", "SENT", false},
		{"INV-COUNT-003", "SENT", true}, // overdue (due date in the past)
		{"INV-COUNT-004", "PARTIALLY_PAID", false},
	}

	for _, inv := range invoices {
		invoiceID := uuid.New().String()
		dueDate := "NOW() + INTERVAL '30 days'"
		if inv.overdue {
			dueDate = "NOW() - INTERVAL '10 days'"
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices
			(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, 'SALES', $4, NOW(), `+dueDate+`, 'EUR', 100, 20, 120, 0, $5, $6, NOW(), NOW())
		`, invoiceID, tenant.ID, inv.number, contactID, inv.status, userID)
		if err != nil {
			t.Fatalf("Failed to create test invoice %s: %v", inv.number, err)
		}
	}

	// Get invoice counts
	draft, pending, overdue, err := repo.GetInvoiceCounts(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetInvoiceCounts failed: %v", err)
	}

	if draft != 1 {
		t.Errorf("expected 1 draft invoice, got %d", draft)
	}
	if pending != 3 { // SENT + PARTIALLY_PAID (including overdue SENT)
		t.Errorf("expected 3 pending invoices, got %d", pending)
	}
	if overdue != 1 {
		t.Errorf("expected 1 overdue invoice, got %d", overdue)
	}
}

func TestPostgresRepository_GetRevenueExpenses_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now

	// Get revenue/expenses for period with no data
	revenue, expenses, err := repo.GetRevenueExpenses(ctx, tenant.SchemaName, start, end)
	if err != nil {
		t.Fatalf("GetRevenueExpenses failed: %v", err)
	}

	// Should return zeros for empty data
	if !revenue.Equal(decimal.Zero) {
		t.Errorf("expected revenue 0, got %s", revenue)
	}
	if !expenses.Equal(decimal.Zero) {
		t.Errorf("expected expenses 0, got %s", expenses)
	}
}

func TestPostgresRepository_GetMonthlyRevenueExpenses_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get monthly data with no journal entries
	data, err := repo.GetMonthlyRevenueExpenses(ctx, tenant.SchemaName, 6)
	if err != nil {
		t.Fatalf("GetMonthlyRevenueExpenses failed: %v", err)
	}

	// Should have 6 months of data
	if len(data) != 6 {
		t.Errorf("expected 6 months of data, got %d", len(data))
	}

	// All values should be zero
	for _, month := range data {
		if !month.Revenue.Equal(decimal.Zero) {
			t.Errorf("expected revenue 0 for %s, got %s", month.Label, month.Revenue)
		}
		if !month.Expenses.Equal(decimal.Zero) {
			t.Errorf("expected expenses 0 for %s, got %s", month.Label, month.Expenses)
		}
	}
}

func TestPostgresRepository_GetMonthlyCashFlow_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get monthly cash flow with no payments
	data, err := repo.GetMonthlyCashFlow(ctx, tenant.SchemaName, 6)
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow failed: %v", err)
	}

	// Should have 6 months of data
	if len(data) != 6 {
		t.Errorf("expected 6 months of data, got %d", len(data))
	}

	// All values should be zero
	for _, month := range data {
		if !month.Inflows.Equal(decimal.Zero) {
			t.Errorf("expected inflows 0 for %s, got %s", month.Label, month.Inflows)
		}
		if !month.Outflows.Equal(decimal.Zero) {
			t.Errorf("expected outflows 0 for %s, got %s", month.Label, month.Outflows)
		}
	}
}

func TestPostgresRepository_GetAgingByContact_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get aging data for receivables (SALES invoices)
	aging, err := repo.GetAgingByContact(ctx, tenant.SchemaName, "SALES")
	if err != nil {
		t.Fatalf("GetAgingByContact failed: %v", err)
	}

	// Should be empty with no invoices
	if len(aging) != 0 {
		t.Errorf("expected 0 aging records, got %d", len(aging))
	}
}

func TestPostgresRepository_GetTopCustomers_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get top customers with no data
	customers, err := repo.GetTopCustomers(ctx, tenant.SchemaName, 10)
	if err != nil {
		t.Fatalf("GetTopCustomers failed: %v", err)
	}

	// Should be empty with no paid invoices
	if len(customers) != 0 {
		t.Errorf("expected 0 customers, got %d", len(customers))
	}
}

func TestPostgresRepository_GetAgingByContact_WithData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-aging-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a customer contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'AGING-C001', 'Aging Test Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create invoices with different due dates to test aging buckets
	invoices := []struct {
		number  string
		dueDays int // negative means overdue
		amount  float64
	}{
		{"INV-AGING-001", 10, 100},   // current (due in future)
		{"INV-AGING-002", -15, 200},  // 1-30 days overdue
		{"INV-AGING-003", -45, 300},  // 31-60 days overdue
		{"INV-AGING-004", -75, 400},  // 61-90 days overdue
		{"INV-AGING-005", -120, 500}, // 90+ days overdue
	}

	for _, inv := range invoices {
		invoiceID := uuid.New().String()
		_, err = pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices
			(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, 'SALES', $4, NOW(), NOW() + INTERVAL '`+fmt.Sprintf("%d days", inv.dueDays)+`', 'EUR', $5, 0, $5, 0, 'SENT', $6, NOW(), NOW())
		`, invoiceID, tenant.ID, inv.number, contactID, inv.amount, userID)
		if err != nil {
			t.Fatalf("Failed to create test invoice %s: %v", inv.number, err)
		}
	}

	// Get aging data for receivables (SALES invoices)
	aging, err := repo.GetAgingByContact(ctx, tenant.SchemaName, "SALES")
	if err != nil {
		t.Fatalf("GetAgingByContact failed: %v", err)
	}

	// Should have one contact with aging data
	if len(aging) != 1 {
		t.Errorf("expected 1 aging record, got %d", len(aging))
	}

	if len(aging) > 0 {
		// Verify the contact
		if aging[0].ContactID != contactID {
			t.Errorf("expected contact ID %s, got %s", contactID, aging[0].ContactID)
		}
		// Verify current bucket (due in future)
		if !aging[0].Current.Equal(decimal.NewFromFloat(100)) {
			t.Errorf("expected current 100, got %s", aging[0].Current)
		}
		// Verify 1-30 days bucket
		if !aging[0].Days1to30.Equal(decimal.NewFromFloat(200)) {
			t.Errorf("expected days 1-30 = 200, got %s", aging[0].Days1to30)
		}
		// Verify total
		expectedTotal := decimal.NewFromFloat(1500) // 100+200+300+400+500
		if !aging[0].Total.Equal(expectedTotal) {
			t.Errorf("expected total %s, got %s", expectedTotal, aging[0].Total)
		}
	}
}

func TestPostgresRepository_GetTopCustomers_WithData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-top-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create three customers with different invoice totals
	customers := []struct {
		code   string
		name   string
		amount float64
	}{
		{"TOP-C001", "Top Customer One", 1000},
		{"TOP-C002", "Top Customer Two", 500},
		{"TOP-C003", "Top Customer Three", 2000},
	}

	for _, cust := range customers {
		contactID := uuid.New().String()
		_, err := pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.contacts
			(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
		`, contactID, tenant.ID, cust.code, cust.name)
		if err != nil {
			t.Fatalf("Failed to create test contact %s: %v", cust.code, err)
		}

		// Create invoice for this customer
		invoiceID := uuid.New().String()
		_, err = pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices
			(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
			VALUES ($1, $2, 'INV-TOP-'||$3, 'SALES', $4, NOW(), NOW() + INTERVAL '30 days', 'EUR', $5, 0, $5, 0, 'SENT', $6, NOW(), NOW())
		`, invoiceID, tenant.ID, cust.code, contactID, cust.amount, userID)
		if err != nil {
			t.Fatalf("Failed to create test invoice for %s: %v", cust.code, err)
		}
	}

	// Get top 3 customers
	topCustomers, err := repo.GetTopCustomers(ctx, tenant.SchemaName, 3)
	if err != nil {
		t.Fatalf("GetTopCustomers failed: %v", err)
	}

	// Should have 3 customers
	if len(topCustomers) != 3 {
		t.Errorf("expected 3 customers, got %d", len(topCustomers))
	}

	// First should be the one with highest amount (2000)
	if len(topCustomers) > 0 {
		if topCustomers[0].Name != "Top Customer Three" {
			t.Errorf("expected first customer to be 'Top Customer Three', got '%s'", topCustomers[0].Name)
		}
		if !topCustomers[0].Amount.Equal(decimal.NewFromFloat(2000)) {
			t.Errorf("expected first customer amount 2000, got %s", topCustomers[0].Amount)
		}
	}
}

func TestPostgresRepository_GetMonthlyRevenueExpenses_WithData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-monthly-rev-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// We need to create journal entries for revenue/expenses
	// First we need GL accounts for revenue and expenses

	// Get revenue account (4000 range)
	var revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code LIKE '4%' AND account_type = 'REVENUE' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Skipf("No revenue account found (4xxx REVENUE type): %v - skipping test", err)
	}

	// Get expense account (5000 range)
	var expenseAccountID string
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code LIKE '5%' AND account_type = 'EXPENSE' LIMIT 1
	`).Scan(&expenseAccountID)
	if err != nil {
		t.Skipf("No expense account found (5xxx EXPENSE type): %v - skipping test", err)
	}

	// Get cash account (1000 range) for the other side of entries
	var cashAccountID string
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Skipf("No cash account found: %v - skipping test", err)
	}

	// Create a POSTED journal entry for revenue this month
	entryID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at)
		VALUES ($1, $2, 'JE-REV-001', NOW(), 'Test revenue entry', 'POSTED', $3, NOW())
	`, entryID, tenant.ID, userID)
	if err != nil {
		t.Fatalf("Failed to create journal entry: %v", err)
	}

	// Revenue is credited (credit_amount), debit goes to cash
	// Need base_debit/base_credit columns as well (with currency = EUR, exchange_rate = 1)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 1000, 0, 'EUR', 1, 1000, 0)
	`, uuid.New().String(), tenant.ID, entryID, cashAccountID)
	if err != nil {
		t.Fatalf("Failed to create debit line: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 0, 1000, 'EUR', 1, 0, 1000)
	`, uuid.New().String(), tenant.ID, entryID, revenueAccountID)
	if err != nil {
		t.Fatalf("Failed to create credit line: %v", err)
	}

	// Get monthly revenue/expenses for last 6 months
	data, err := repo.GetMonthlyRevenueExpenses(ctx, tenant.SchemaName, 6)
	if err != nil {
		t.Fatalf("GetMonthlyRevenueExpenses failed: %v", err)
	}

	// Should have 6 months of data
	if len(data) != 6 {
		t.Errorf("expected 6 months of data, got %d", len(data))
	}

	// At least one month should have revenue (current month)
	hasRevenue := false
	for _, month := range data {
		if month.Revenue.GreaterThan(decimal.Zero) {
			hasRevenue = true
			break
		}
	}
	if !hasRevenue {
		t.Error("expected at least one month to have revenue")
	}
}

func TestPostgresRepository_GetMonthlyCashFlow_WithData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-cashflow-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a customer contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'CF-C001', 'Cash Flow Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create payments (RECEIVED = inflow, MADE = outflow)
	paymentID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, payment_number, payment_type, contact_id, amount, currency, exchange_rate, base_amount, payment_date, created_by, created_at)
		VALUES ($1, $2, 'PMT-CF-001', 'RECEIVED', $3, 500, 'EUR', 1, 500, NOW(), $4, NOW())
	`, paymentID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create received payment: %v", err)
	}

	// Create outflow payment (need supplier contact)
	supplierID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'CF-S001', 'Cash Flow Supplier', 'SUPPLIER', 'EE', 14, 0, true, NOW(), NOW())
	`, supplierID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create supplier contact: %v", err)
	}

	paymentID2 := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.payments
		(id, tenant_id, payment_number, payment_type, contact_id, amount, currency, exchange_rate, base_amount, payment_date, created_by, created_at)
		VALUES ($1, $2, 'PMT-CF-002', 'MADE', $3, 200, 'EUR', 1, 200, NOW(), $4, NOW())
	`, paymentID2, tenant.ID, supplierID, userID)
	if err != nil {
		t.Fatalf("Failed to create made payment: %v", err)
	}

	// Get monthly cash flow
	data, err := repo.GetMonthlyCashFlow(ctx, tenant.SchemaName, 6)
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow failed: %v", err)
	}

	// Should have 6 months of data
	if len(data) != 6 {
		t.Errorf("expected 6 months of data, got %d", len(data))
	}

	// At least one month should have inflows and outflows
	hasInflows := false
	hasOutflows := false
	for _, month := range data {
		if month.Inflows.GreaterThan(decimal.Zero) {
			hasInflows = true
		}
		if month.Outflows.GreaterThan(decimal.Zero) {
			hasOutflows = true
		}
	}
	if !hasInflows {
		t.Error("expected at least one month to have inflows")
	}
	if !hasOutflows {
		t.Error("expected at least one month to have outflows")
	}
}

func TestNewPostgresRepository(t *testing.T) {
	pool := testutil.SetupTestDB(t)

	repo := NewPostgresRepository(pool)
	if repo == nil {
		t.Fatal("NewPostgresRepository returned nil")
	}
	if repo.pool != pool {
		t.Error("repository pool does not match provided pool")
	}
}

func TestPostgresRepository_GetRevenueExpenses_WithData(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-revexp-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get revenue account (4000 range)
	var revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code LIKE '4%' AND account_type = 'REVENUE' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Skipf("No revenue account found (4xxx REVENUE type): %v - skipping test", err)
	}

	// Get expense account (5000 range)
	var expenseAccountID string
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code LIKE '5%' AND account_type = 'EXPENSE' LIMIT 1
	`).Scan(&expenseAccountID)
	if err != nil {
		t.Skipf("No expense account found (5xxx EXPENSE type): %v - skipping test", err)
	}

	// Get cash account for the other side of entries
	var cashAccountID string
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Skipf("No cash account found: %v - skipping test", err)
	}

	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now.AddDate(0, 0, 1)

	// Create a POSTED journal entry for revenue
	revenueEntryID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at)
		VALUES ($1, $2, 'JE-REVEXP-001', $3, 'Test revenue entry', 'POSTED', $4, NOW())
	`, revenueEntryID, tenant.ID, now, userID)
	if err != nil {
		t.Fatalf("Failed to create revenue journal entry: %v", err)
	}

	// Revenue is credited, debit goes to cash (1000 revenue)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 1000, 0, 'EUR', 1, 1000, 0)
	`, uuid.New().String(), tenant.ID, revenueEntryID, cashAccountID)
	if err != nil {
		t.Fatalf("Failed to create debit line for revenue: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 0, 1000, 'EUR', 1, 0, 1000)
	`, uuid.New().String(), tenant.ID, revenueEntryID, revenueAccountID)
	if err != nil {
		t.Fatalf("Failed to create credit line for revenue: %v", err)
	}

	// Create a POSTED journal entry for expense
	expenseEntryID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at)
		VALUES ($1, $2, 'JE-REVEXP-002', $3, 'Test expense entry', 'POSTED', $4, NOW())
	`, expenseEntryID, tenant.ID, now, userID)
	if err != nil {
		t.Fatalf("Failed to create expense journal entry: %v", err)
	}

	// Expense is debited, credit goes from cash (300 expense)
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 300, 0, 'EUR', 1, 300, 0)
	`, uuid.New().String(), tenant.ID, expenseEntryID, expenseAccountID)
	if err != nil {
		t.Fatalf("Failed to create debit line for expense: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 0, 300, 'EUR', 1, 0, 300)
	`, uuid.New().String(), tenant.ID, expenseEntryID, cashAccountID)
	if err != nil {
		t.Fatalf("Failed to create credit line for expense: %v", err)
	}

	// Get revenue/expenses for the period
	revenue, expenses, err := repo.GetRevenueExpenses(ctx, tenant.SchemaName, start, end)
	if err != nil {
		t.Fatalf("GetRevenueExpenses failed: %v", err)
	}

	expectedRevenue := decimal.NewFromInt(1000)
	if !revenue.Equal(expectedRevenue) {
		t.Errorf("expected revenue %s, got %s", expectedRevenue, revenue)
	}

	expectedExpenses := decimal.NewFromInt(300)
	if !expenses.Equal(expectedExpenses) {
		t.Errorf("expected expenses %s, got %s", expectedExpenses, expenses)
	}
}

func TestPostgresRepository_GetRevenueExpenses_DraftNotIncluded(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-draft-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Get revenue account
	var revenueAccountID string
	err := pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code LIKE '4%' AND account_type = 'REVENUE' LIMIT 1
	`).Scan(&revenueAccountID)
	if err != nil {
		t.Skipf("No revenue account found: %v - skipping test", err)
	}

	// Get cash account
	var cashAccountID string
	err = pool.QueryRow(ctx, `
		SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1
	`).Scan(&cashAccountID)
	if err != nil {
		t.Skipf("No cash account found: %v - skipping test", err)
	}

	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now.AddDate(0, 0, 1)

	// Create a DRAFT journal entry (should NOT be included)
	draftEntryID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
		(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at)
		VALUES ($1, $2, 'JE-DRAFT-001', $3, 'Draft entry - should not count', 'DRAFT', $4, NOW())
	`, draftEntryID, tenant.ID, now, userID)
	if err != nil {
		t.Fatalf("Failed to create draft journal entry: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 500, 0, 'EUR', 1, 500, 0)
	`, uuid.New().String(), tenant.ID, draftEntryID, cashAccountID)
	if err != nil {
		t.Fatalf("Failed to create debit line: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines (id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES ($1, $2, $3, $4, 0, 500, 'EUR', 1, 0, 500)
	`, uuid.New().String(), tenant.ID, draftEntryID, revenueAccountID)
	if err != nil {
		t.Fatalf("Failed to create credit line: %v", err)
	}

	// Get revenue/expenses - should be zero because entry is DRAFT
	revenue, expenses, err := repo.GetRevenueExpenses(ctx, tenant.SchemaName, start, end)
	if err != nil {
		t.Fatalf("GetRevenueExpenses failed: %v", err)
	}

	if !revenue.Equal(decimal.Zero) {
		t.Errorf("expected revenue 0 for draft entries, got %s", revenue)
	}
	if !expenses.Equal(decimal.Zero) {
		t.Errorf("expected expenses 0 for draft entries, got %s", expenses)
	}
}

func TestPostgresRepository_GetReceivablesSummary_WithOverdue(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-ar-overdue-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'AR-OVR-001', 'AR Overdue Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a current SALES invoice (due in future)
	currentInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-AR-CURR', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 100, 0, 100, 0, 'SENT', $4, NOW(), NOW())
	`, currentInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create current invoice: %v", err)
	}

	// Create an overdue SALES invoice (due date in the past)
	overdueInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-AR-OVRD', 'SALES', $3, NOW() - INTERVAL '60 days', NOW() - INTERVAL '30 days', 'EUR', 200, 0, 200, 50, 'PARTIALLY_PAID', $4, NOW(), NOW())
	`, overdueInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create overdue invoice: %v", err)
	}

	// Get receivables summary
	total, overdue, err := repo.GetReceivablesSummary(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetReceivablesSummary failed: %v", err)
	}

	// Total: current (100-0=100) + overdue (200-50=150) = 250
	expectedTotal := decimal.NewFromInt(250)
	if !total.Equal(expectedTotal) {
		t.Errorf("expected total %s, got %s", expectedTotal, total)
	}

	// Overdue: 200-50=150
	expectedOverdue := decimal.NewFromInt(150)
	if !overdue.Equal(expectedOverdue) {
		t.Errorf("expected overdue %s, got %s", expectedOverdue, overdue)
	}
}

func TestPostgresRepository_GetReceivablesSummary_ExcludesPaidAndVoided(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-ar-exclude-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'AR-EXC-001', 'AR Exclude Test Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a PAID invoice (should be excluded)
	paidInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-PAID-001', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 500, 0, 500, 500, 'PAID', $4, NOW(), NOW())
	`, paidInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create paid invoice: %v", err)
	}

	// Create a VOIDED invoice (should be excluded)
	voidedInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-VOID-001', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 300, 0, 300, 0, 'VOIDED', $4, NOW(), NOW())
	`, voidedInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create voided invoice: %v", err)
	}

	// Get receivables summary - should be zero because PAID and VOIDED are excluded
	total, overdue, err := repo.GetReceivablesSummary(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetReceivablesSummary failed: %v", err)
	}

	if !total.Equal(decimal.Zero) {
		t.Errorf("expected total 0 (PAID/VOIDED excluded), got %s", total)
	}
	if !overdue.Equal(decimal.Zero) {
		t.Errorf("expected overdue 0, got %s", overdue)
	}
}

func TestPostgresRepository_GetPayablesSummary_WithOverdue(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-ap-overdue-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a vendor contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'AP-OVR-001', 'AP Overdue Vendor', 'SUPPLIER', 'EE', 30, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a current PURCHASE invoice (due in future)
	currentInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'BILL-AP-CURR', 'PURCHASE', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 400, 0, 400, 0, 'SENT', $4, NOW(), NOW())
	`, currentInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create current bill: %v", err)
	}

	// Create an overdue PURCHASE invoice (due date in the past)
	overdueInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'BILL-AP-OVRD', 'PURCHASE', $3, NOW() - INTERVAL '60 days', NOW() - INTERVAL '30 days', 'EUR', 600, 0, 600, 100, 'PARTIALLY_PAID', $4, NOW(), NOW())
	`, overdueInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create overdue bill: %v", err)
	}

	// Get payables summary
	total, overdue, err := repo.GetPayablesSummary(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetPayablesSummary failed: %v", err)
	}

	// Total: current (400-0=400) + overdue (600-100=500) = 900
	expectedTotal := decimal.NewFromInt(900)
	if !total.Equal(expectedTotal) {
		t.Errorf("expected total %s, got %s", expectedTotal, total)
	}

	// Overdue: 600-100=500
	expectedOverdue := decimal.NewFromInt(500)
	if !overdue.Equal(expectedOverdue) {
		t.Errorf("expected overdue %s, got %s", expectedOverdue, overdue)
	}
}

func TestPostgresRepository_GetPayablesSummary_ExcludesPaidAndVoided(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-ap-exclude-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a vendor contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'AP-EXC-001', 'AP Exclude Test Vendor', 'SUPPLIER', 'EE', 30, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a PAID bill (should be excluded)
	paidBillID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'BILL-PAID-001', 'PURCHASE', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 800, 0, 800, 800, 'PAID', $4, NOW(), NOW())
	`, paidBillID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create paid bill: %v", err)
	}

	// Create a VOIDED bill (should be excluded)
	voidedBillID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'BILL-VOID-001', 'PURCHASE', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 700, 0, 700, 0, 'VOIDED', $4, NOW(), NOW())
	`, voidedBillID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create voided bill: %v", err)
	}

	// Get payables summary - should be zero because PAID and VOIDED are excluded
	total, overdue, err := repo.GetPayablesSummary(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetPayablesSummary failed: %v", err)
	}

	if !total.Equal(decimal.Zero) {
		t.Errorf("expected total 0 (PAID/VOIDED excluded), got %s", total)
	}
	if !overdue.Equal(decimal.Zero) {
		t.Errorf("expected overdue 0, got %s", overdue)
	}
}

func TestPostgresRepository_GetTopCustomers_Limit(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-toplimit-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create 5 customers with different invoice totals
	customers := []struct {
		code   string
		name   string
		amount float64
	}{
		{"LIM-C001", "Limit Customer One", 100},
		{"LIM-C002", "Limit Customer Two", 200},
		{"LIM-C003", "Limit Customer Three", 300},
		{"LIM-C004", "Limit Customer Four", 400},
		{"LIM-C005", "Limit Customer Five", 500},
	}

	for _, cust := range customers {
		contactID := uuid.New().String()
		_, err := pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.contacts
			(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
		`, contactID, tenant.ID, cust.code, cust.name)
		if err != nil {
			t.Fatalf("Failed to create test contact %s: %v", cust.code, err)
		}

		invoiceID := uuid.New().String()
		_, err = pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices
			(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
			VALUES ($1, $2, 'INV-LIM-'||$3, 'SALES', $4, NOW(), NOW() + INTERVAL '30 days', 'EUR', $5, 0, $5, 0, 'SENT', $6, NOW(), NOW())
		`, invoiceID, tenant.ID, cust.code, contactID, cust.amount, userID)
		if err != nil {
			t.Fatalf("Failed to create test invoice for %s: %v", cust.code, err)
		}
	}

	// Get top 2 customers (should only return 2)
	topCustomers, err := repo.GetTopCustomers(ctx, tenant.SchemaName, 2)
	if err != nil {
		t.Fatalf("GetTopCustomers failed: %v", err)
	}

	if len(topCustomers) != 2 {
		t.Errorf("expected 2 customers with limit=2, got %d", len(topCustomers))
	}

	// First should be highest amount (500)
	if len(topCustomers) > 0 && topCustomers[0].Name != "Limit Customer Five" {
		t.Errorf("expected first customer to be 'Limit Customer Five', got '%s'", topCustomers[0].Name)
	}

	// Second should be second highest (400)
	if len(topCustomers) > 1 && topCustomers[1].Name != "Limit Customer Four" {
		t.Errorf("expected second customer to be 'Limit Customer Four', got '%s'", topCustomers[1].Name)
	}
}

func TestPostgresRepository_GetTopCustomers_ExcludesVoided(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-topvoided-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a customer
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'VOID-C001', 'Voided Invoice Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a voided invoice (should be excluded from total)
	voidedInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-VOID-TOP', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 1000, 0, 1000, 0, 'VOIDED', $4, NOW(), NOW())
	`, voidedInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create voided invoice: %v", err)
	}

	// Create a valid invoice
	validInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-VALID-TOP', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 250, 0, 250, 0, 'SENT', $4, NOW(), NOW())
	`, validInvoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create valid invoice: %v", err)
	}

	// Get top customers
	topCustomers, err := repo.GetTopCustomers(ctx, tenant.SchemaName, 10)
	if err != nil {
		t.Fatalf("GetTopCustomers failed: %v", err)
	}

	if len(topCustomers) != 1 {
		t.Fatalf("expected 1 customer, got %d", len(topCustomers))
	}

	// Amount should only include the valid invoice (250), not the voided one (1000)
	expectedAmount := decimal.NewFromInt(250)
	if !topCustomers[0].Amount.Equal(expectedAmount) {
		t.Errorf("expected amount %s (voided excluded), got %s", expectedAmount, topCustomers[0].Amount)
	}

	// Count should be 1 (only the valid invoice)
	if topCustomers[0].Count != 1 {
		t.Errorf("expected count 1, got %d", topCustomers[0].Count)
	}
}

func TestPostgresRepository_GetTopCustomers_IncludesBothContactType(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-topboth-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact with contact_type = 'BOTH' (customer and supplier)
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'BOTH-C001', 'Both Type Contact', 'BOTH', 'EE', 14, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create a SALES invoice
	invoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-BOTH-001', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 750, 0, 750, 0, 'SENT', $4, NOW(), NOW())
	`, invoiceID, tenant.ID, contactID, userID)
	if err != nil {
		t.Fatalf("Failed to create invoice: %v", err)
	}

	// Get top customers - should include 'BOTH' type contacts
	topCustomers, err := repo.GetTopCustomers(ctx, tenant.SchemaName, 10)
	if err != nil {
		t.Fatalf("GetTopCustomers failed: %v", err)
	}

	if len(topCustomers) != 1 {
		t.Fatalf("expected 1 customer (BOTH type), got %d", len(topCustomers))
	}

	if topCustomers[0].Name != "Both Type Contact" {
		t.Errorf("expected 'Both Type Contact', got '%s'", topCustomers[0].Name)
	}

	expectedAmount := decimal.NewFromInt(750)
	if !topCustomers[0].Amount.Equal(expectedAmount) {
		t.Errorf("expected amount %s, got %s", expectedAmount, topCustomers[0].Amount)
	}
}

func TestPostgresRepository_GetAgingByContact_Payables(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-aging-ap-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a vendor contact
	contactID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'AGING-V001', 'Aging Test Vendor', 'SUPPLIER', 'EE', 30, 0, true, NOW(), NOW())
	`, contactID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create test contact: %v", err)
	}

	// Create PURCHASE invoices with different due dates
	invoices := []struct {
		number  string
		dueDays int
		amount  float64
	}{
		{"BILL-AGING-001", 10, 100},   // current
		{"BILL-AGING-002", -20, 200},  // 1-30 days overdue
		{"BILL-AGING-003", -50, 300},  // 31-60 days overdue
		{"BILL-AGING-004", -80, 400},  // 61-90 days overdue
		{"BILL-AGING-005", -100, 500}, // 90+ days overdue
	}

	for _, inv := range invoices {
		invoiceID := uuid.New().String()
		_, err = pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices
			(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
			VALUES ($1, $2, $3, 'PURCHASE', $4, NOW(), NOW() + INTERVAL '`+fmt.Sprintf("%d days", inv.dueDays)+`', 'EUR', $5, 0, $5, 0, 'SENT', $6, NOW(), NOW())
		`, invoiceID, tenant.ID, inv.number, contactID, inv.amount, userID)
		if err != nil {
			t.Fatalf("Failed to create test bill %s: %v", inv.number, err)
		}
	}

	// Get aging data for payables (PURCHASE invoices)
	aging, err := repo.GetAgingByContact(ctx, tenant.SchemaName, "PURCHASE")
	if err != nil {
		t.Fatalf("GetAgingByContact failed: %v", err)
	}

	if len(aging) != 1 {
		t.Errorf("expected 1 aging record, got %d", len(aging))
	}

	if len(aging) > 0 {
		// Verify totals
		expectedTotal := decimal.NewFromFloat(1500) // 100+200+300+400+500
		if !aging[0].Total.Equal(expectedTotal) {
			t.Errorf("expected total %s, got %s", expectedTotal, aging[0].Total)
		}

		// Verify current bucket
		if !aging[0].Current.Equal(decimal.NewFromFloat(100)) {
			t.Errorf("expected current 100, got %s", aging[0].Current)
		}

		// Verify 90+ bucket
		if !aging[0].Days90Plus.Equal(decimal.NewFromFloat(500)) {
			t.Errorf("expected days 90+ = 500, got %s", aging[0].Days90Plus)
		}
	}
}

func TestPostgresRepository_GetMonthlyRevenueExpenses_DifferentMonths(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Test with 3 months
	data3, err := repo.GetMonthlyRevenueExpenses(ctx, tenant.SchemaName, 3)
	if err != nil {
		t.Fatalf("GetMonthlyRevenueExpenses(3) failed: %v", err)
	}
	if len(data3) != 3 {
		t.Errorf("expected 3 months of data, got %d", len(data3))
	}

	// Test with 12 months
	data12, err := repo.GetMonthlyRevenueExpenses(ctx, tenant.SchemaName, 12)
	if err != nil {
		t.Fatalf("GetMonthlyRevenueExpenses(12) failed: %v", err)
	}
	if len(data12) != 12 {
		t.Errorf("expected 12 months of data, got %d", len(data12))
	}
}

func TestPostgresRepository_GetMonthlyCashFlow_DifferentMonths(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Test with 3 months
	data3, err := repo.GetMonthlyCashFlow(ctx, tenant.SchemaName, 3)
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow(3) failed: %v", err)
	}
	if len(data3) != 3 {
		t.Errorf("expected 3 months of data, got %d", len(data3))
	}

	// Test with 12 months
	data12, err := repo.GetMonthlyCashFlow(ctx, tenant.SchemaName, 12)
	if err != nil {
		t.Fatalf("GetMonthlyCashFlow(12) failed: %v", err)
	}
	if len(data12) != 12 {
		t.Errorf("expected 12 months of data, got %d", len(data12))
	}
}

func TestPostgresRepository_GetInvoiceCounts_OnlyCountsSales(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "analytics-counts-sales-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contacts
	customerID := uuid.New().String()
	_, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'CNT-CUST', 'Sales Count Customer', 'CUSTOMER', 'EE', 14, 0, true, NOW(), NOW())
	`, customerID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create customer contact: %v", err)
	}

	supplierID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
		(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, is_active, created_at, updated_at)
		VALUES ($1, $2, 'CNT-SUPP', 'Sales Count Supplier', 'SUPPLIER', 'EE', 30, 0, true, NOW(), NOW())
	`, supplierID, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to create supplier contact: %v", err)
	}

	// Create a SALES invoice with DRAFT status
	salesInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'INV-SALES-CNT', 'SALES', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 100, 0, 100, 0, 'DRAFT', $4, NOW(), NOW())
	`, salesInvoiceID, tenant.ID, customerID, userID)
	if err != nil {
		t.Fatalf("Failed to create sales invoice: %v", err)
	}

	// Create a PURCHASE invoice with DRAFT status (should NOT be counted)
	purchaseInvoiceID := uuid.New().String()
	_, err = pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
		(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency, subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES ($1, $2, 'BILL-PURCHASE-CNT', 'PURCHASE', $3, NOW(), NOW() + INTERVAL '30 days', 'EUR', 200, 0, 200, 0, 'DRAFT', $4, NOW(), NOW())
	`, purchaseInvoiceID, tenant.ID, supplierID, userID)
	if err != nil {
		t.Fatalf("Failed to create purchase invoice: %v", err)
	}

	// Get invoice counts
	draft, pending, overdue, err := repo.GetInvoiceCounts(ctx, tenant.SchemaName)
	if err != nil {
		t.Fatalf("GetInvoiceCounts failed: %v", err)
	}

	// Should only count SALES invoices, not PURCHASE
	if draft != 1 {
		t.Errorf("expected 1 draft (SALES only), got %d", draft)
	}
	if pending != 0 {
		t.Errorf("expected 0 pending, got %d", pending)
	}
	if overdue != 0 {
		t.Errorf("expected 0 overdue, got %d", overdue)
	}
}
