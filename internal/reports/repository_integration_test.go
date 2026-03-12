package reports

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestPostgresRepository_ReportQueries(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "reports-integration@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	salesContactID := uuid.New().String()
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
			(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, email, is_active, created_at, updated_at)
		VALUES
			($1, $2, 'C-REP-1', 'Receivable Contact', 'CUSTOMER', 'EE', 14, 0, 'receivable@example.com', true, NOW(), NOW())
	`, salesContactID, tenant.ID); err != nil {
		t.Fatalf("failed to create sales contact: %v", err)
	}

	purchaseContactID := uuid.New().String()
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts
			(id, tenant_id, code, name, contact_type, country_code, payment_terms_days, credit_limit, email, is_active, created_at, updated_at)
		VALUES
			($1, $2, 'S-REP-1', 'Payable Contact', 'SUPPLIER', 'EE', 14, 0, 'payable@example.com', true, NOW(), NOW())
	`, purchaseContactID, tenant.ID); err != nil {
		t.Fatalf("failed to create purchase contact: %v", err)
	}

	salesInvoiceID := uuid.New().String()
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
			(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency,
			 subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES
			($1, $2, 'INV-REP-001', 'SALES', $3, DATE '2025-01-10', DATE '2025-01-31', 'EUR',
			 100, 20, 120, 20, 'SENT', $4, NOW(), NOW())
	`, salesInvoiceID, tenant.ID, salesContactID, userID); err != nil {
		t.Fatalf("failed to create sales invoice: %v", err)
	}

	purchaseInvoiceID := uuid.New().String()
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.invoices
			(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency,
			 subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
		VALUES
			($1, $2, 'BILL-REP-001', 'PURCHASE', $3, DATE '2025-01-12', DATE '2025-01-25', 'EUR',
			 200, 40, 240, 100, 'SENT', $4, NOW(), NOW())
	`, purchaseInvoiceID, tenant.ID, purchaseContactID, userID); err != nil {
		t.Fatalf("failed to create purchase invoice: %v", err)
	}

	var cashAccountID, revenueAccountID string
	if err := pool.QueryRow(ctx, `SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '1000' LIMIT 1`).Scan(&cashAccountID); err != nil {
		t.Fatalf("failed to find cash account: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM `+tenant.SchemaName+`.accounts WHERE code = '3000' LIMIT 1`).Scan(&revenueAccountID); err != nil {
		t.Fatalf("failed to find revenue account: %v", err)
	}

	entryID := uuid.New().String()
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entries
			(id, tenant_id, entry_number, entry_date, description, status, created_by, created_at)
		VALUES
			($1, $2, 'JE-REP-001', DATE '2025-01-20', 'Cash sale', 'POSTED', $3, NOW())
	`, entryID, tenant.ID, userID); err != nil {
		t.Fatalf("failed to create journal entry: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.journal_entry_lines
			(id, tenant_id, journal_entry_id, account_id, debit_amount, credit_amount, currency, exchange_rate, base_debit, base_credit)
		VALUES
			($1, $2, $3, $4, 120, 0, 'EUR', 1, 120, 0),
			($5, $2, $3, $6, 0, 120, 'EUR', 1, 0, 120)
	`, uuid.New().String(), tenant.ID, entryID, cashAccountID, uuid.New().String(), revenueAccountID); err != nil {
		t.Fatalf("failed to create journal entry lines: %v", err)
	}

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("journal entries and cash balance", func(t *testing.T) {
		entries, err := repo.GetJournalEntriesForPeriod(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
		if err != nil {
			t.Fatalf("GetJournalEntriesForPeriod failed: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 journal entry, got %d", len(entries))
		}
		if len(entries[0].Lines) != 2 {
			t.Fatalf("expected 2 journal lines, got %d", len(entries[0].Lines))
		}

		balance, err := repo.GetCashAccountBalance(ctx, tenant.SchemaName, tenant.ID, endDate)
		if err != nil {
			t.Fatalf("GetCashAccountBalance failed: %v", err)
		}
		if !balance.Equal(decimal.NewFromInt(120)) {
			t.Fatalf("expected cash balance 120, got %s", balance)
		}
	})

	t.Run("contact balances and invoices", func(t *testing.T) {
		receivables, err := repo.GetOutstandingInvoicesByContact(ctx, tenant.SchemaName, tenant.ID, "SALES", endDate)
		if err != nil {
			t.Fatalf("GetOutstandingInvoicesByContact sales failed: %v", err)
		}
		if len(receivables) != 1 {
			t.Fatalf("expected 1 receivable contact, got %d", len(receivables))
		}
		if !receivables[0].Balance.Equal(decimal.NewFromInt(100)) {
			t.Fatalf("expected receivable balance 100, got %s", receivables[0].Balance)
		}

		payables, err := repo.GetOutstandingInvoicesByContact(ctx, tenant.SchemaName, tenant.ID, "PURCHASE", endDate)
		if err != nil {
			t.Fatalf("GetOutstandingInvoicesByContact purchase failed: %v", err)
		}
		if len(payables) != 1 {
			t.Fatalf("expected 1 payable contact, got %d", len(payables))
		}
		if !payables[0].Balance.Equal(decimal.NewFromInt(140)) {
			t.Fatalf("expected payable balance 140, got %s", payables[0].Balance)
		}

		invoices, err := repo.GetContactInvoices(ctx, tenant.SchemaName, tenant.ID, salesContactID, "SALES", endDate)
		if err != nil {
			t.Fatalf("GetContactInvoices failed: %v", err)
		}
		if len(invoices) != 1 {
			t.Fatalf("expected 1 contact invoice, got %d", len(invoices))
		}
		if !invoices[0].OutstandingAmount.Equal(decimal.NewFromInt(100)) {
			t.Fatalf("expected outstanding amount 100, got %s", invoices[0].OutstandingAmount)
		}

		contact, err := repo.GetContact(ctx, tenant.SchemaName, tenant.ID, salesContactID)
		if err != nil {
			t.Fatalf("GetContact failed: %v", err)
		}
		if contact.Email != "receivable@example.com" {
			t.Fatalf("expected contact email to round-trip, got %q", contact.Email)
		}
	})
}

func TestNewServiceUsesPostgresRepository(t *testing.T) {
	pool := testutil.SetupTestDB(t)

	service := NewService(pool)
	if service == nil {
		t.Fatal("expected service")
	}
	if _, ok := service.repo.(*PostgresRepository); !ok {
		t.Fatalf("expected PostgresRepository, got %T", service.repo)
	}
}
