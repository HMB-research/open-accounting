//go:build integration

package reports

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestGORMRepository_ReportQueries(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "reports-integration@example.com")
	repo := NewGORMRepository(pool)
	ctx := context.Background()

	gormDB, err := database.NewGormDBFromPool(ctx, pool)
	if err != nil {
		t.Fatalf("failed to create gorm db: %v", err)
	}
	tenantTable := func(tableName string) *gorm.DB {
		t.Helper()
		table, err := database.TenantTable(gormDB, tenant.SchemaName, tableName)
		if err != nil {
			t.Fatalf("failed to qualify %s table: %v", tableName, err)
		}
		return table
	}

	salesContactID := uuid.New().String()
	now := time.Now()
	if err := tenantTable("contacts").Create(&models.Contact{
		ID:               salesContactID,
		TenantID:         tenant.ID,
		Code:             "C-REP-1",
		Name:             "Receivable Contact",
		ContactType:      models.ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      models.DecimalZero(),
		Email:            "receivable@example.com",
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("failed to create sales contact: %v", err)
	}

	purchaseContactID := uuid.New().String()
	if err := tenantTable("contacts").Create(&models.Contact{
		ID:               purchaseContactID,
		TenantID:         tenant.ID,
		Code:             "S-REP-1",
		Name:             "Payable Contact",
		ContactType:      models.ContactTypeSupplier,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      models.DecimalZero(),
		Email:            "payable@example.com",
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("failed to create purchase contact: %v", err)
	}

	salesInvoiceID := uuid.New().String()
	if err := tenantTable("invoices").Create(&models.Invoice{
		ID:            salesInvoiceID,
		TenantID:      tenant.ID,
		InvoiceNumber: "INV-REP-001",
		InvoiceType:   models.InvoiceTypeSales,
		ContactID:     salesContactID,
		IssueDate:     time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
		DueDate:       time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		Currency:      "EUR",
		ExchangeRate:  models.NewDecimal(decimal.NewFromInt(1)),
		Subtotal:      models.NewDecimal(decimal.NewFromInt(100)),
		VATAmount:     models.NewDecimal(decimal.NewFromInt(20)),
		Total:         models.NewDecimal(decimal.NewFromInt(120)),
		BaseSubtotal:  models.NewDecimal(decimal.NewFromInt(100)),
		BaseVATAmount: models.NewDecimal(decimal.NewFromInt(20)),
		BaseTotal:     models.NewDecimal(decimal.NewFromInt(120)),
		AmountPaid:    models.NewDecimal(decimal.NewFromInt(20)),
		Status:        models.InvoiceStatusSent,
		CreatedBy:     userID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("failed to create sales invoice: %v", err)
	}

	purchaseInvoiceID := uuid.New().String()
	if err := tenantTable("invoices").Create(&models.Invoice{
		ID:            purchaseInvoiceID,
		TenantID:      tenant.ID,
		InvoiceNumber: "BILL-REP-001",
		InvoiceType:   models.InvoiceTypePurchase,
		ContactID:     purchaseContactID,
		IssueDate:     time.Date(2025, 1, 12, 0, 0, 0, 0, time.UTC),
		DueDate:       time.Date(2025, 1, 25, 0, 0, 0, 0, time.UTC),
		Currency:      "EUR",
		ExchangeRate:  models.NewDecimal(decimal.NewFromInt(1)),
		Subtotal:      models.NewDecimal(decimal.NewFromInt(200)),
		VATAmount:     models.NewDecimal(decimal.NewFromInt(40)),
		Total:         models.NewDecimal(decimal.NewFromInt(240)),
		BaseSubtotal:  models.NewDecimal(decimal.NewFromInt(200)),
		BaseVATAmount: models.NewDecimal(decimal.NewFromInt(40)),
		BaseTotal:     models.NewDecimal(decimal.NewFromInt(240)),
		AmountPaid:    models.NewDecimal(decimal.NewFromInt(100)),
		Status:        models.InvoiceStatusSent,
		CreatedBy:     userID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("failed to create purchase invoice: %v", err)
	}

	var cashAccount, revenueAccount models.Account
	if err := tenantTable("accounts").Select("id").Where("tenant_id = ? AND code = ?", tenant.ID, "1000").Take(&cashAccount).Error; err != nil {
		t.Fatalf("failed to find cash account: %v", err)
	}
	if err := tenantTable("accounts").Select("id").Where("tenant_id = ? AND code = ?", tenant.ID, "3000").Take(&revenueAccount).Error; err != nil {
		t.Fatalf("failed to find revenue account: %v", err)
	}

	entryID := uuid.New().String()
	if err := tenantTable("journal_entries").Create(&models.JournalEntry{
		ID:          entryID,
		TenantID:    tenant.ID,
		EntryNumber: "JE-REP-001",
		EntryDate:   time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC),
		Description: "Cash sale",
		Status:      models.JournalStatusPosted,
		CreatedBy:   userID,
		CreatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("failed to create journal entry: %v", err)
	}

	if err := tenantTable("journal_entry_lines").Create([]models.JournalEntryLine{
		{
			ID:             uuid.New().String(),
			TenantID:       tenant.ID,
			JournalEntryID: entryID,
			AccountID:      cashAccount.ID,
			DebitAmount:    models.NewDecimal(decimal.NewFromInt(120)),
			CreditAmount:   models.DecimalZero(),
			Currency:       "EUR",
			ExchangeRate:   models.NewDecimal(decimal.NewFromInt(1)),
			BaseDebit:      models.NewDecimal(decimal.NewFromInt(120)),
			BaseCredit:     models.DecimalZero(),
		},
		{
			ID:             uuid.New().String(),
			TenantID:       tenant.ID,
			JournalEntryID: entryID,
			AccountID:      revenueAccount.ID,
			DebitAmount:    models.DecimalZero(),
			CreditAmount:   models.NewDecimal(decimal.NewFromInt(120)),
			Currency:       "EUR",
			ExchangeRate:   models.NewDecimal(decimal.NewFromInt(1)),
			BaseDebit:      models.DecimalZero(),
			BaseCredit:     models.NewDecimal(decimal.NewFromInt(120)),
		},
	}).Error; err != nil {
		t.Fatalf("failed to create journal entry lines: %v", err)
	}

	paymentID := uuid.New().String()
	if err := tenantTable("payments").Create(&models.Payment{
		ID:            paymentID,
		TenantID:      tenant.ID,
		PaymentNumber: "PAY-REP-001",
		PaymentType:   models.PaymentTypeReceived,
		ContactID:     &salesContactID,
		PaymentDate:   time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC),
		Amount:        models.NewDecimal(decimal.NewFromInt(20)),
		Currency:      "EUR",
		ExchangeRate:  models.NewDecimal(decimal.NewFromInt(1)),
		BaseAmount:    models.NewDecimal(decimal.NewFromInt(20)),
		CreatedBy:     userID,
		CreatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	productID := uuid.New().String()
	if err := tenantTable("products").Create(map[string]interface{}{
		"id":             productID,
		"tenant_id":      tenant.ID,
		"code":           "PRD-REP-1",
		"name":           "Report Product",
		"description":    "Product for report integration tests",
		"product_type":   "GOODS",
		"unit":           "pcs",
		"purchase_price": decimal.NewFromInt(30),
		"sale_price":     decimal.NewFromInt(50),
		"vat_rate":       decimal.NewFromInt(20),
		"is_active":      true,
		"created_at":     now,
		"updated_at":     now,
	}).Error; err != nil {
		t.Fatalf("failed to create product: %v", err)
	}

	if err := tenantTable("invoice_lines").Create(&models.InvoiceLine{
		ID:              uuid.New().String(),
		TenantID:        tenant.ID,
		InvoiceID:       salesInvoiceID,
		LineNumber:      1,
		Description:     "Report product",
		Quantity:        models.NewDecimal(decimal.NewFromInt(2)),
		Unit:            "pcs",
		UnitPrice:       models.NewDecimal(decimal.NewFromInt(50)),
		DiscountPercent: models.DecimalZero(),
		VATRate:         models.NewDecimal(decimal.NewFromInt(20)),
		LineSubtotal:    models.NewDecimal(decimal.NewFromInt(100)),
		LineVAT:         models.NewDecimal(decimal.NewFromInt(20)),
		LineTotal:       models.NewDecimal(decimal.NewFromInt(120)),
		ProductID:       &productID,
	}).Error; err != nil {
		t.Fatalf("failed to create invoice line: %v", err)
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

	t.Run("cash flow mapping settings", func(t *testing.T) {
		updated, err := repo.UpdateCashFlowMappingOverrides(ctx, tenant.ID, CashFlowMappingOverrides{
			OperatingAccountCodes: []string{"PREPAY"},
			InvestingAccountCodes: []string{"CAPEX-1"},
			FinancingAccountCodes: []string{"FOUNDERS"},
		})
		if err != nil {
			t.Fatalf("UpdateCashFlowMappingOverrides failed: %v", err)
		}
		if len(updated.InvestingAccountCodes) != 1 || updated.InvestingAccountCodes[0] != "CAPEX-1" {
			t.Fatalf("expected CAPEX-1 investing mapping, got %#v", updated.InvestingAccountCodes)
		}

		mapping, err := repo.GetCashFlowMappingOverrides(ctx, tenant.ID)
		if err != nil {
			t.Fatalf("GetCashFlowMappingOverrides failed: %v", err)
		}
		if len(mapping.FinancingAccountCodes) != 1 || mapping.FinancingAccountCodes[0] != "FOUNDERS" {
			t.Fatalf("expected FOUNDERS financing mapping, got %#v", mapping.FinancingAccountCodes)
		}
	})

	t.Run("contact statement entries", func(t *testing.T) {
		opening, err := repo.GetContactStatementOpeningBalance(ctx, tenant.SchemaName, tenant.ID, salesContactID, "SALES", "RECEIVED", startDate)
		if err != nil {
			t.Fatalf("GetContactStatementOpeningBalance failed: %v", err)
		}
		if !opening.IsZero() {
			t.Fatalf("expected zero opening balance, got %s", opening)
		}

		entries, err := repo.GetContactStatementEntries(ctx, tenant.SchemaName, tenant.ID, salesContactID, "SALES", "RECEIVED", startDate, endDate)
		if err != nil {
			t.Fatalf("GetContactStatementEntries failed: %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected invoice and payment statement entries, got %d", len(entries))
		}
		if entries[0].DocumentType != "INVOICE" || !entries[0].StatementAmount.Equal(decimal.NewFromInt(120)) {
			t.Fatalf("unexpected invoice statement entry: %+v", entries[0])
		}
		if entries[1].DocumentType != "PAYMENT" || !entries[1].StatementAmount.Equal(decimal.NewFromInt(-20)) {
			t.Fatalf("unexpected payment statement entry: %+v", entries[1])
		}
	})

	t.Run("sales margin lines", func(t *testing.T) {
		lines, err := repo.GetSalesMarginLines(ctx, tenant.SchemaName, tenant.ID, startDate, endDate)
		if err != nil {
			t.Fatalf("GetSalesMarginLines failed: %v", err)
		}
		if len(lines) != 1 {
			t.Fatalf("expected 1 sales margin line, got %d", len(lines))
		}
		if lines[0].ProductCode != "PRD-REP-1" {
			t.Fatalf("expected product code PRD-REP-1, got %q", lines[0].ProductCode)
		}
		if !lines[0].Revenue.Equal(decimal.NewFromInt(100)) || !lines[0].Cost.Equal(decimal.NewFromInt(60)) {
			t.Fatalf("unexpected margin line amounts: %+v", lines[0])
		}
	})
}

func TestNewServiceUsesGORMRepository(t *testing.T) {
	pool := testutil.SetupTestDB(t)

	service := NewService(pool)
	if service == nil {
		t.Fatal("expected service")
	}
	if _, ok := service.repo.(*GORMRepository); !ok {
		t.Fatalf("expected GORMRepository, got %T", service.repo)
	}
}
