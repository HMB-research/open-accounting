package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Test base.go exports
func TestDecimalExports(t *testing.T) {
	// Test NewDecimal
	d := NewDecimal(decimal.NewFromInt(100))
	if d.Decimal.IntPart() != 100 {
		t.Errorf("expected 100, got %d", d.Decimal.IntPart())
	}

	// Test NewDecimalFromFloat
	df := NewDecimalFromFloat(123.45)
	if df.Decimal.InexactFloat64() != 123.45 {
		t.Errorf("expected 123.45, got %f", df.Decimal.InexactFloat64())
	}

	// Test NewDecimalFromString
	ds, err := NewDecimalFromString("999.99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ds.Decimal.String() != "999.99" {
		t.Errorf("expected 999.99, got %s", ds.Decimal.String())
	}

	// Test DecimalZero
	if !DecimalZero().Decimal.IsZero() {
		t.Error("DecimalZero should be zero")
	}
}

func TestTenantModel(t *testing.T) {
	now := time.Now()
	m := TenantModel{
		ID:        "test-id",
		TenantID:  "tenant-id",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if m.ID != "test-id" {
		t.Errorf("expected test-id, got %s", m.ID)
	}
	if m.TenantID != "tenant-id" {
		t.Errorf("expected tenant-id, got %s", m.TenantID)
	}
}

func TestPublicModel(t *testing.T) {
	now := time.Now()
	m := PublicModel{
		ID:        "public-id",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if m.ID != "public-id" {
		t.Errorf("expected public-id, got %s", m.ID)
	}
}

// Test contact.go
func TestContactType_Constants(t *testing.T) {
	tests := []struct {
		ct       ContactType
		expected string
	}{
		{ContactTypeCustomer, "CUSTOMER"},
		{ContactTypeSupplier, "SUPPLIER"},
		{ContactTypeBoth, "BOTH"},
	}

	for _, tt := range tests {
		if string(tt.ct) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.ct))
		}
	}
}

func TestContact_TableName(t *testing.T) {
	c := Contact{}
	if c.TableName() != "contacts" {
		t.Errorf("expected contacts, got %s", c.TableName())
	}
}

func TestContact_Fields(t *testing.T) {
	now := time.Now()
	accountID := "acc-123"
	c := Contact{
		ID:               "contact-id",
		TenantID:         "tenant-id",
		Code:             "C001",
		Name:             "Test Customer",
		ContactType:      ContactTypeCustomer,
		RegCode:          "12345678",
		VATNumber:        "EE123456789",
		Email:            "test@example.com",
		Phone:            "+372 555 1234",
		AddressLine1:     "Test Street 1",
		AddressLine2:     "Apt 2",
		City:             "Tallinn",
		PostalCode:       "10111",
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      NewDecimalFromFloat(1000.00),
		DefaultAccountID: &accountID,
		IsActive:         true,
		Notes:            "Test notes",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if c.ID != "contact-id" {
		t.Errorf("expected contact-id, got %s", c.ID)
	}
	if c.Name != "Test Customer" {
		t.Errorf("expected Test Customer, got %s", c.Name)
	}
	if c.ContactType != ContactTypeCustomer {
		t.Errorf("expected CUSTOMER, got %s", c.ContactType)
	}
	if *c.DefaultAccountID != accountID {
		t.Errorf("expected %s, got %s", accountID, *c.DefaultAccountID)
	}
}

// Test invoice.go types
func TestInvoiceType_Constants(t *testing.T) {
	tests := []struct {
		it       InvoiceType
		expected string
	}{
		{InvoiceTypeSales, "SALES"},
		{InvoiceTypePurchase, "PURCHASE"},
		{InvoiceTypeCreditNote, "CREDIT_NOTE"},
	}

	for _, tt := range tests {
		if string(tt.it) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.it))
		}
	}
}

func TestInvoiceStatus_Constants(t *testing.T) {
	tests := []struct {
		is       InvoiceStatus
		expected string
	}{
		{InvoiceStatusDraft, "DRAFT"},
		{InvoiceStatusSent, "SENT"},
		{InvoiceStatusPartiallyPaid, "PARTIALLY_PAID"},
		{InvoiceStatusPaid, "PAID"},
		{InvoiceStatusOverdue, "OVERDUE"},
		{InvoiceStatusVoided, "VOIDED"},
	}

	for _, tt := range tests {
		if string(tt.is) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.is))
		}
	}
}

func TestInvoice_TableName(t *testing.T) {
	i := Invoice{}
	if i.TableName() != "invoices" {
		t.Errorf("expected invoices, got %s", i.TableName())
	}
}

func TestInvoiceLine_TableName(t *testing.T) {
	il := InvoiceLine{}
	if il.TableName() != "invoice_lines" {
		t.Errorf("expected invoice_lines, got %s", il.TableName())
	}
}

func TestInvoice_Fields(t *testing.T) {
	now := time.Now()
	userID := "user-123"
	i := Invoice{
		ID:            "inv-id",
		TenantID:      "tenant-id",
		InvoiceNumber: "INV-001",
		InvoiceType:   InvoiceTypeSales,
		ContactID:     "contact-id",
		IssueDate:     now,
		DueDate:       now.AddDate(0, 0, 30),
		Currency:      "EUR",
		ExchangeRate:  NewDecimalFromFloat(1.0),
		Subtotal:      NewDecimalFromFloat(100.00),
		VATAmount:     NewDecimalFromFloat(20.00),
		Total:         NewDecimalFromFloat(120.00),
		Status:        InvoiceStatusDraft,
		CreatedBy:     userID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if i.InvoiceNumber != "INV-001" {
		t.Errorf("expected INV-001, got %s", i.InvoiceNumber)
	}
	if i.InvoiceType != InvoiceTypeSales {
		t.Errorf("expected SALES, got %s", i.InvoiceType)
	}
}

// Test payment.go types
func TestPaymentType_Constants(t *testing.T) {
	tests := []struct {
		pt       PaymentType
		expected string
	}{
		{PaymentTypeReceived, "RECEIVED"},
		{PaymentTypeMade, "MADE"},
	}

	for _, tt := range tests {
		if string(tt.pt) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.pt))
		}
	}
}

func TestPayment_TableName(t *testing.T) {
	p := Payment{}
	if p.TableName() != "payments" {
		t.Errorf("expected payments, got %s", p.TableName())
	}
}

func TestPaymentAllocation_TableName(t *testing.T) {
	pa := PaymentAllocation{}
	if pa.TableName() != "payment_allocations" {
		t.Errorf("expected payment_allocations, got %s", pa.TableName())
	}
}

// Test accounting.go types
func TestAccountType_Constants(t *testing.T) {
	tests := []struct {
		at       AccountType
		expected string
	}{
		{AccountTypeAsset, "ASSET"},
		{AccountTypeLiability, "LIABILITY"},
		{AccountTypeEquity, "EQUITY"},
		{AccountTypeRevenue, "REVENUE"},
		{AccountTypeExpense, "EXPENSE"},
	}

	for _, tt := range tests {
		if string(tt.at) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.at))
		}
	}
}

func TestAccountType_IsDebitNormal(t *testing.T) {
	tests := []struct {
		at       AccountType
		expected bool
	}{
		{AccountTypeAsset, true},
		{AccountTypeExpense, true},
		{AccountTypeLiability, false},
		{AccountTypeEquity, false},
		{AccountTypeRevenue, false},
	}

	for _, tt := range tests {
		if tt.at.IsDebitNormal() != tt.expected {
			t.Errorf("expected IsDebitNormal(%s) = %v, got %v", tt.at, tt.expected, tt.at.IsDebitNormal())
		}
	}
}

func TestJournalEntryStatus_Constants(t *testing.T) {
	tests := []struct {
		jes      JournalEntryStatus
		expected string
	}{
		{JournalStatusDraft, "DRAFT"},
		{JournalStatusPosted, "POSTED"},
		{JournalStatusVoided, "VOIDED"},
	}

	for _, tt := range tests {
		if string(tt.jes) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.jes))
		}
	}
}

func TestAccount_TableName(t *testing.T) {
	a := Account{}
	if a.TableName() != "accounts" {
		t.Errorf("expected accounts, got %s", a.TableName())
	}
}

func TestJournalEntry_TableName(t *testing.T) {
	je := JournalEntry{}
	if je.TableName() != "journal_entries" {
		t.Errorf("expected journal_entries, got %s", je.TableName())
	}
}

func TestJournalEntryLine_TableName(t *testing.T) {
	jel := JournalEntryLine{}
	if jel.TableName() != "journal_entry_lines" {
		t.Errorf("expected journal_entry_lines, got %s", jel.TableName())
	}
}

// Test banking.go types
func TestTransactionStatus_Constants(t *testing.T) {
	tests := []struct {
		ts       TransactionStatus
		expected string
	}{
		{TransactionStatusUnmatched, "UNMATCHED"},
		{TransactionStatusMatched, "MATCHED"},
		{TransactionStatusReconciled, "RECONCILED"},
	}

	for _, tt := range tests {
		if string(tt.ts) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.ts))
		}
	}
}

func TestReconciliationStatus_Constants(t *testing.T) {
	tests := []struct {
		rs       ReconciliationStatus
		expected string
	}{
		{ReconciliationInProgress, "IN_PROGRESS"},
		{ReconciliationCompleted, "COMPLETED"},
	}

	for _, tt := range tests {
		if string(tt.rs) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.rs))
		}
	}
}

func TestBankAccount_TableName(t *testing.T) {
	ba := BankAccount{}
	if ba.TableName() != "bank_accounts" {
		t.Errorf("expected bank_accounts, got %s", ba.TableName())
	}
}

func TestBankTransaction_TableName(t *testing.T) {
	bt := BankTransaction{}
	if bt.TableName() != "bank_transactions" {
		t.Errorf("expected bank_transactions, got %s", bt.TableName())
	}
}

func TestBankReconciliation_TableName(t *testing.T) {
	br := BankReconciliation{}
	if br.TableName() != "bank_reconciliations" {
		t.Errorf("expected bank_reconciliations, got %s", br.TableName())
	}
}

func TestBankStatementImport_TableName(t *testing.T) {
	bsi := BankStatementImport{}
	if bsi.TableName() != "bank_statement_imports" {
		t.Errorf("expected bank_statement_imports, got %s", bsi.TableName())
	}
}

// Test email.go types
func TestTemplateType_Constants(t *testing.T) {
	tests := []struct {
		tt       TemplateType
		expected string
	}{
		{TemplateInvoiceSend, "INVOICE_SEND"},
		{TemplatePaymentReceipt, "PAYMENT_RECEIPT"},
		{TemplateOverdueReminder, "OVERDUE_REMINDER"},
	}

	for _, tc := range tests {
		if string(tc.tt) != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, string(tc.tt))
		}
	}
}

func TestEmailStatus_Constants(t *testing.T) {
	tests := []struct {
		es       EmailStatus
		expected string
	}{
		{EmailStatusPending, "PENDING"},
		{EmailStatusSent, "SENT"},
		{EmailStatusFailed, "FAILED"},
	}

	for _, tt := range tests {
		if string(tt.es) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.es))
		}
	}
}

func TestEmailTemplate_TableName(t *testing.T) {
	et := EmailTemplate{}
	if et.TableName() != "email_templates" {
		t.Errorf("expected email_templates, got %s", et.TableName())
	}
}

func TestEmailLog_TableName(t *testing.T) {
	el := EmailLog{}
	if el.TableName() != "email_log" {
		t.Errorf("expected email_log, got %s", el.TableName())
	}
}

// Test payroll.go types
func TestEmploymentType_Constants(t *testing.T) {
	tests := []struct {
		et       EmploymentType
		expected string
	}{
		{EmploymentFullTime, "FULL_TIME"},
		{EmploymentPartTime, "PART_TIME"},
		{EmploymentContract, "CONTRACT"},
	}

	for _, tt := range tests {
		if string(tt.et) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.et))
		}
	}
}

func TestPayrollStatus_Constants(t *testing.T) {
	tests := []struct {
		ps       PayrollStatus
		expected string
	}{
		{PayrollDraft, "DRAFT"},
		{PayrollCalculated, "CALCULATED"},
		{PayrollApproved, "APPROVED"},
		{PayrollPaid, "PAID"},
		{PayrollDeclared, "DECLARED"},
	}

	for _, tt := range tests {
		if string(tt.ps) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.ps))
		}
	}
}

func TestEmployee_TableName(t *testing.T) {
	e := Employee{}
	if e.TableName() != "employees" {
		t.Errorf("expected employees, got %s", e.TableName())
	}
}

func TestSalaryComponent_TableName(t *testing.T) {
	sc := SalaryComponent{}
	if sc.TableName() != "salary_components" {
		t.Errorf("expected salary_components, got %s", sc.TableName())
	}
}

func TestPayrollRun_TableName(t *testing.T) {
	pr := PayrollRun{}
	if pr.TableName() != "payroll_runs" {
		t.Errorf("expected payroll_runs, got %s", pr.TableName())
	}
}

func TestPayslip_TableName(t *testing.T) {
	p := Payslip{}
	if p.TableName() != "payslips" {
		t.Errorf("expected payslips, got %s", p.TableName())
	}
}

// Test plugin.go types
func TestPluginState_Constants(t *testing.T) {
	tests := []struct {
		ps       PluginState
		expected string
	}{
		{PluginStateInstalled, "installed"},
		{PluginStateEnabled, "enabled"},
		{PluginStateDisabled, "disabled"},
		{PluginStateFailed, "failed"},
	}

	for _, tt := range tests {
		if string(tt.ps) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.ps))
		}
	}
}

func TestRepositoryType_Constants(t *testing.T) {
	tests := []struct {
		rt       RepositoryType
		expected string
	}{
		{RepoGitHub, "github"},
		{RepoGitLab, "gitlab"},
	}

	for _, tt := range tests {
		if string(tt.rt) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.rt))
		}
	}
}

func TestPluginRegistry_TableName(t *testing.T) {
	pr := PluginRegistry{}
	if pr.TableName() != "plugin_registries" {
		t.Errorf("expected plugin_registries, got %s", pr.TableName())
	}
}

func TestPlugin_TableName(t *testing.T) {
	p := Plugin{}
	if p.TableName() != "plugins" {
		t.Errorf("expected plugins, got %s", p.TableName())
	}
}

func TestTenantPlugin_TableName(t *testing.T) {
	tp := TenantPlugin{}
	if tp.TableName() != "tenant_plugins" {
		t.Errorf("expected tenant_plugins, got %s", tp.TableName())
	}
}

func TestPluginMigration_TableName(t *testing.T) {
	pm := PluginMigration{}
	if pm.TableName() != "plugin_migrations" {
		t.Errorf("expected plugin_migrations, got %s", pm.TableName())
	}
}

// Test recurring.go types
func TestFrequency_Constants(t *testing.T) {
	tests := []struct {
		f        Frequency
		expected string
	}{
		{FrequencyWeekly, "WEEKLY"},
		{FrequencyBiweekly, "BIWEEKLY"},
		{FrequencyMonthly, "MONTHLY"},
		{FrequencyQuarterly, "QUARTERLY"},
		{FrequencyYearly, "YEARLY"},
	}

	for _, tt := range tests {
		if string(tt.f) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.f))
		}
	}
}

func TestRecurringInvoice_TableName(t *testing.T) {
	ri := RecurringInvoice{}
	if ri.TableName() != "recurring_invoices" {
		t.Errorf("expected recurring_invoices, got %s", ri.TableName())
	}
}

func TestRecurringInvoiceLine_TableName(t *testing.T) {
	ril := RecurringInvoiceLine{}
	if ril.TableName() != "recurring_invoice_lines" {
		t.Errorf("expected recurring_invoice_lines, got %s", ril.TableName())
	}
}

// Test tax.go types
func TestKMDDeclaration_TableName(t *testing.T) {
	kmd := KMDDeclaration{}
	if kmd.TableName() != "kmd_declarations" {
		t.Errorf("expected kmd_declarations, got %s", kmd.TableName())
	}
}

func TestKMDRow_TableName(t *testing.T) {
	kr := KMDRow{}
	if kr.TableName() != "kmd_rows" {
		t.Errorf("expected kmd_rows, got %s", kr.TableName())
	}
}

// Test tenant.go types
func TestTenant_TableName(t *testing.T) {
	tn := Tenant{}
	if tn.TableName() != "tenants" {
		t.Errorf("expected tenants, got %s", tn.TableName())
	}
}

func TestUser_TableName(t *testing.T) {
	u := User{}
	if u.TableName() != "users" {
		t.Errorf("expected users, got %s", u.TableName())
	}
}

func TestTenantUserModel_TableName(t *testing.T) {
	tu := TenantUserModel{}
	if tu.TableName() != "tenant_users" {
		t.Errorf("expected tenant_users, got %s", tu.TableName())
	}
}

func TestUserInvitation_TableName(t *testing.T) {
	ui := UserInvitation{}
	if ui.TableName() != "user_invitations" {
		t.Errorf("expected user_invitations, got %s", ui.TableName())
	}
}

// Test model field values
func TestInvoice_WithLines(t *testing.T) {
	inv := Invoice{
		ID:            "inv-1",
		InvoiceNumber: "INV-001",
		Lines: []InvoiceLine{
			{ID: "line-1", LineNumber: 1, Description: "Item 1"},
			{ID: "line-2", LineNumber: 2, Description: "Item 2"},
		},
	}

	if len(inv.Lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(inv.Lines))
	}
	if inv.Lines[0].Description != "Item 1" {
		t.Errorf("expected Item 1, got %s", inv.Lines[0].Description)
	}
}

func TestPayment_Fields(t *testing.T) {
	now := time.Now()
	p := Payment{
		ID:            "pay-1",
		TenantID:      "tenant-1",
		PaymentNumber: "PAY-001",
		PaymentType:   PaymentTypeReceived,
		PaymentDate:   now,
		Amount:        NewDecimalFromFloat(500.00),
		Currency:      "EUR",
		ExchangeRate:  NewDecimalFromFloat(1.0),
		BaseAmount:    NewDecimalFromFloat(500.00),
		CreatedBy:     "user-1",
		CreatedAt:     now,
	}

	if p.PaymentNumber != "PAY-001" {
		t.Errorf("expected PAY-001, got %s", p.PaymentNumber)
	}
	if p.PaymentType != PaymentTypeReceived {
		t.Errorf("expected RECEIVED, got %s", p.PaymentType)
	}
}

func TestEmployee_Fields(t *testing.T) {
	now := time.Now()
	startDate := now.AddDate(-1, 0, 0)
	e := Employee{
		ID:                   "emp-1",
		TenantID:             "tenant-1",
		EmployeeNumber:       "EMP001",
		FirstName:            "John",
		LastName:             "Doe",
		PersonalCode:         "12345678901",
		Email:                "john@example.com",
		StartDate:            startDate,
		Position:             "Developer",
		EmploymentType:       EmploymentFullTime,
		TaxResidency:         "EE",
		ApplyBasicExemption:  true,
		BasicExemptionAmount: NewDecimalFromFloat(700.00),
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if e.FirstName != "John" {
		t.Errorf("expected John, got %s", e.FirstName)
	}
	if e.EmploymentType != EmploymentFullTime {
		t.Errorf("expected FULL_TIME, got %s", e.EmploymentType)
	}
}

func TestRecurringInvoice_Fields(t *testing.T) {
	now := time.Now()
	ri := RecurringInvoice{
		ID:                 "ri-1",
		TenantID:           "tenant-1",
		Name:               "Monthly Subscription",
		ContactID:          "contact-1",
		Frequency:          FrequencyMonthly,
		StartDate:          now,
		NextGenerationDate: now.AddDate(0, 1, 0),
		PaymentTermsDays:   14,
		IsActive:           true,
		CreatedBy:          "user-1",
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if ri.Frequency != FrequencyMonthly {
		t.Errorf("expected MONTHLY, got %s", ri.Frequency)
	}
	if ri.Name != "Monthly Subscription" {
		t.Errorf("expected Monthly Subscription, got %s", ri.Name)
	}
}

func TestTenant_Fields(t *testing.T) {
	now := time.Now()
	tn := Tenant{
		ID:         "tenant-1",
		Name:       "Test Company",
		Slug:       "test-company",
		SchemaName: "tenant_test_company",
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if tn.Name != "Test Company" {
		t.Errorf("expected Test Company, got %s", tn.Name)
	}
	if tn.SchemaName != "tenant_test_company" {
		t.Errorf("expected tenant_test_company, got %s", tn.SchemaName)
	}
}

func TestBankAccount_Fields(t *testing.T) {
	now := time.Now()
	ba := BankAccount{
		ID:            "ba-1",
		TenantID:      "tenant-1",
		Name:          "Main Account",
		AccountNumber: "EE123456789012345678",
		BankName:      "Test Bank",
		SwiftCode:     "TESTEE2X",
		Currency:      "EUR",
		IsDefault:     true,
		IsActive:      true,
		CreatedAt:     now,
	}

	if ba.AccountNumber != "EE123456789012345678" {
		t.Errorf("expected EE123456789012345678, got %s", ba.AccountNumber)
	}
	if !ba.IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestPlugin_Fields(t *testing.T) {
	now := time.Now()
	p := Plugin{
		ID:             uuid.New(),
		Name:           "test-plugin",
		DisplayName:    "Test Plugin",
		Version:        "1.0.0",
		Description:    "A test plugin",
		RepositoryURL:  "https://github.com/test/test-plugin",
		RepositoryType: RepoGitHub,
		State:          PluginStateInstalled,
		InstalledAt:    now,
		UpdatedAt:      now,
	}

	if p.DisplayName != "Test Plugin" {
		t.Errorf("expected Test Plugin, got %s", p.DisplayName)
	}
	if p.State != PluginStateInstalled {
		t.Errorf("expected installed, got %s", p.State)
	}
}

func TestJournalEntry_Fields(t *testing.T) {
	now := time.Now()
	je := JournalEntry{
		ID:          "je-1",
		TenantID:    "tenant-1",
		EntryNumber: "JE-001",
		EntryDate:   now,
		Description: "Test entry",
		Status:      JournalStatusDraft,
		CreatedBy:   "user-1",
		CreatedAt:   now,
	}

	if je.EntryNumber != "JE-001" {
		t.Errorf("expected JE-001, got %s", je.EntryNumber)
	}
	if je.Status != JournalStatusDraft {
		t.Errorf("expected DRAFT, got %s", je.Status)
	}
}

func TestJournalEntryLine_Fields(t *testing.T) {
	jel := JournalEntryLine{
		ID:             "jel-1",
		TenantID:       "tenant-1",
		JournalEntryID: "je-1",
		AccountID:      "acc-1",
		Description:    "Test line",
		DebitAmount:    NewDecimalFromFloat(100.00),
		CreditAmount:   DecimalZero(),
		Currency:       "EUR",
		ExchangeRate:   NewDecimalFromFloat(1.0),
		BaseDebit:      NewDecimalFromFloat(100.00),
		BaseCredit:     DecimalZero(),
	}

	if jel.Description != "Test line" {
		t.Errorf("expected Test line, got %s", jel.Description)
	}
	if jel.DebitAmount.Decimal.InexactFloat64() != 100.00 {
		t.Errorf("expected 100.00, got %f", jel.DebitAmount.Decimal.InexactFloat64())
	}
}

func TestAccountBalance_Fields(t *testing.T) {
	ab := AccountBalance{
		AccountID:     "acc-1",
		AccountCode:   "1000",
		AccountName:   "Cash",
		AccountType:   AccountTypeAsset,
		DebitBalance:  NewDecimalFromFloat(1000.00),
		CreditBalance: DecimalZero(),
		NetBalance:    NewDecimalFromFloat(1000.00),
	}

	if ab.AccountCode != "1000" {
		t.Errorf("expected 1000, got %s", ab.AccountCode)
	}
	if ab.AccountType != AccountTypeAsset {
		t.Errorf("expected ASSET, got %s", ab.AccountType)
	}
}

func TestBankTransaction_Fields(t *testing.T) {
	now := time.Now()
	bt := BankTransaction{
		ID:              "bt-1",
		TenantID:        "tenant-1",
		BankAccountID:   "ba-1",
		TransactionDate: now,
		Amount:          NewDecimalFromFloat(250.00),
		Currency:        "EUR",
		Description:     "Payment received",
		Status:          TransactionStatusUnmatched,
		ImportedAt:      now,
	}

	if bt.Description != "Payment received" {
		t.Errorf("expected Payment received, got %s", bt.Description)
	}
	if bt.Status != TransactionStatusUnmatched {
		t.Errorf("expected UNMATCHED, got %s", bt.Status)
	}
}

func TestBankReconciliation_Fields(t *testing.T) {
	now := time.Now()
	br := BankReconciliation{
		ID:             "br-1",
		TenantID:       "tenant-1",
		BankAccountID:  "ba-1",
		StatementDate:  now,
		OpeningBalance: NewDecimalFromFloat(1000.00),
		ClosingBalance: NewDecimalFromFloat(1500.00),
		Status:         ReconciliationInProgress,
		CreatedBy:      "user-1",
		CreatedAt:      now,
	}

	if br.Status != ReconciliationInProgress {
		t.Errorf("expected IN_PROGRESS, got %s", br.Status)
	}
}

func TestPayrollRun_Fields(t *testing.T) {
	now := time.Now()
	pr := PayrollRun{
		ID:                "pr-1",
		TenantID:          "tenant-1",
		PeriodYear:        2024,
		PeriodMonth:       1,
		Status:            PayrollDraft,
		TotalGross:        NewDecimalFromFloat(5000.00),
		TotalNet:          NewDecimalFromFloat(3500.00),
		TotalEmployerCost: NewDecimalFromFloat(6500.00),
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if pr.PeriodYear != 2024 {
		t.Errorf("expected 2024, got %d", pr.PeriodYear)
	}
	if pr.Status != PayrollDraft {
		t.Errorf("expected DRAFT, got %s", pr.Status)
	}
}

func TestPayslip_Fields(t *testing.T) {
	now := time.Now()
	ps := Payslip{
		ID:                      "ps-1",
		TenantID:                "tenant-1",
		PayrollRunID:            "pr-1",
		EmployeeID:              "emp-1",
		GrossSalary:             NewDecimalFromFloat(2500.00),
		TaxableIncome:           NewDecimalFromFloat(2500.00),
		IncomeTax:               NewDecimalFromFloat(500.00),
		UnemploymentInsuranceEE: NewDecimalFromFloat(40.00),
		FundedPension:           NewDecimalFromFloat(50.00),
		NetSalary:               NewDecimalFromFloat(1910.00),
		SocialTax:               NewDecimalFromFloat(825.00),
		UnemploymentInsuranceER: NewDecimalFromFloat(20.00),
		TotalEmployerCost:       NewDecimalFromFloat(3345.00),
		PaymentStatus:           "PENDING",
		CreatedAt:               now,
	}

	if ps.GrossSalary.Decimal.InexactFloat64() != 2500.00 {
		t.Errorf("expected 2500.00, got %f", ps.GrossSalary.Decimal.InexactFloat64())
	}
	if ps.PaymentStatus != "PENDING" {
		t.Errorf("expected PENDING, got %s", ps.PaymentStatus)
	}
}

func TestKMDDeclaration_Fields(t *testing.T) {
	now := time.Now()
	kmd := KMDDeclaration{
		ID:             "kmd-1",
		TenantID:       "tenant-1",
		Year:           2024,
		Month:          1,
		Status:         "DRAFT",
		TotalOutputVAT: NewDecimalFromFloat(2000.00),
		TotalInputVAT:  NewDecimalFromFloat(500.00),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if kmd.Year != 2024 {
		t.Errorf("expected 2024, got %d", kmd.Year)
	}
	if kmd.Status != "DRAFT" {
		t.Errorf("expected DRAFT, got %s", kmd.Status)
	}
}

func TestEmailTemplate_Fields(t *testing.T) {
	now := time.Now()
	et := EmailTemplate{
		ID:           "et-1",
		TenantID:     "tenant-1",
		TemplateType: TemplateInvoiceSend,
		Subject:      "Invoice {{.InvoiceNumber}}",
		BodyHTML:     "<p>Hello</p>",
		BodyText:     "Hello",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if et.TemplateType != TemplateInvoiceSend {
		t.Errorf("expected INVOICE_SEND, got %s", et.TemplateType)
	}
}

func TestEmailLog_Fields(t *testing.T) {
	now := time.Now()
	el := EmailLog{
		ID:             "el-1",
		TenantID:       "tenant-1",
		EmailType:      "INVOICE_SEND",
		RecipientEmail: "test@example.com",
		RecipientName:  "Test User",
		Subject:        "Invoice INV-001",
		Status:         EmailStatusPending,
		CreatedAt:      now,
	}

	if el.Status != EmailStatusPending {
		t.Errorf("expected PENDING, got %s", el.Status)
	}
}

func TestUser_Fields(t *testing.T) {
	now := time.Now()
	u := User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hashed",
		Name:         "Test User",
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if u.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", u.Email)
	}
}

func TestTenantUserModel_Fields(t *testing.T) {
	now := time.Now()
	invitedBy := "user-0"
	tu := TenantUserModel{
		TenantID:  "tenant-1",
		UserID:    "user-1",
		Role:      "ADMIN",
		IsDefault: true,
		InvitedBy: &invitedBy,
		InvitedAt: &now,
		CreatedAt: now,
	}

	if tu.Role != "ADMIN" {
		t.Errorf("expected ADMIN, got %s", tu.Role)
	}
	if !tu.IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestUserInvitation_Fields(t *testing.T) {
	now := time.Now()
	ui := UserInvitation{
		ID:        "inv-1",
		TenantID:  "tenant-1",
		Email:     "invited@example.com",
		Role:      "MEMBER",
		InvitedBy: "user-1",
		Token:     "abc123",
		ExpiresAt: now.AddDate(0, 0, 7),
		CreatedAt: now,
	}

	if ui.Email != "invited@example.com" {
		t.Errorf("expected invited@example.com, got %s", ui.Email)
	}
}

func TestSalaryComponent_Fields(t *testing.T) {
	now := time.Now()
	sc := SalaryComponent{
		ID:            "sc-1",
		TenantID:      "tenant-1",
		EmployeeID:    "emp-1",
		ComponentType: "SALARY",
		Name:          "Base Salary",
		Amount:        NewDecimalFromFloat(2500.00),
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: now,
		CreatedAt:     now,
	}

	if sc.ComponentType != "SALARY" {
		t.Errorf("expected SALARY, got %s", sc.ComponentType)
	}
}

func TestPluginRegistry_Fields(t *testing.T) {
	now := time.Now()
	pr := PluginRegistry{
		ID:          uuid.New(),
		Name:        "Official Registry",
		URL:         "https://plugins.example.com",
		Description: "Official plugin registry",
		IsOfficial:  true,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if pr.Name != "Official Registry" {
		t.Errorf("expected Official Registry, got %s", pr.Name)
	}
}

func TestTenantPlugin_Fields(t *testing.T) {
	now := time.Now()
	tp := TenantPlugin{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		PluginID:  uuid.New(),
		IsEnabled: true,
		EnabledAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if !tp.IsEnabled {
		t.Error("expected IsEnabled to be true")
	}
}

func TestPluginMigration_Fields(t *testing.T) {
	now := time.Now()
	pm := PluginMigration{
		ID:        uuid.New(),
		PluginID:  uuid.New(),
		Version:   "1.0.0",
		Filename:  "001_initial.sql",
		AppliedAt: now,
		Checksum:  "abc123def456",
	}

	if pm.Version != "1.0.0" {
		t.Errorf("expected 1.0.0, got %s", pm.Version)
	}
}

func TestRecurringInvoiceLine_Fields(t *testing.T) {
	ril := RecurringInvoiceLine{
		ID:                 "ril-1",
		RecurringInvoiceID: "ri-1",
		LineNumber:         1,
		Description:        "Monthly service fee",
		Quantity:           NewDecimalFromFloat(1.00),
		Unit:               "pcs",
		UnitPrice:          NewDecimalFromFloat(99.00),
		DiscountPercent:    DecimalZero(),
		VATRate:            NewDecimalFromFloat(22.00),
	}

	if ril.Description != "Monthly service fee" {
		t.Errorf("expected Monthly service fee, got %s", ril.Description)
	}
}

func TestKMDRow_Fields(t *testing.T) {
	kr := KMDRow{
		ID:            "kr-1",
		DeclarationID: "kmd-1",
		Code:          "1",
		Description:   "Taxable sales at 22%",
		TaxBase:       NewDecimalFromFloat(10000.00),
		TaxAmount:     NewDecimalFromFloat(2200.00),
	}

	if kr.Code != "1" {
		t.Errorf("expected 1, got %s", kr.Code)
	}
}

func TestBankStatementImport_Fields(t *testing.T) {
	now := time.Now()
	bsi := BankStatementImport{
		ID:                   "bsi-1",
		TenantID:             "tenant-1",
		BankAccountID:        "ba-1",
		FileName:             "statement_2024_01.csv",
		TransactionsImported: 50,
		TransactionsMatched:  45,
		DuplicatesSkipped:    5,
		CreatedAt:            now,
	}

	if bsi.TransactionsImported != 50 {
		t.Errorf("expected 50, got %d", bsi.TransactionsImported)
	}
}

func TestPaymentAllocation_Fields(t *testing.T) {
	now := time.Now()
	pa := PaymentAllocation{
		ID:        "pa-1",
		TenantID:  "tenant-1",
		PaymentID: "pay-1",
		InvoiceID: "inv-1",
		Amount:    NewDecimalFromFloat(100.00),
		CreatedAt: now,
	}

	if pa.PaymentID != "pay-1" {
		t.Errorf("expected pay-1, got %s", pa.PaymentID)
	}
}

func TestAccount_Fields(t *testing.T) {
	now := time.Now()
	parentID := "acc-parent"
	a := Account{
		ID:          "acc-1",
		TenantID:    "tenant-1",
		Code:        "1000",
		Name:        "Cash",
		AccountType: AccountTypeAsset,
		ParentID:    &parentID,
		IsActive:    true,
		IsSystem:    false,
		Description: "Cash account",
		CreatedAt:   now,
	}

	if a.Code != "1000" {
		t.Errorf("expected 1000, got %s", a.Code)
	}
	if a.AccountType != AccountTypeAsset {
		t.Errorf("expected ASSET, got %s", a.AccountType)
	}
}

func TestInvoiceLine_Fields(t *testing.T) {
	il := InvoiceLine{
		ID:              "il-1",
		TenantID:        "tenant-1",
		InvoiceID:       "inv-1",
		LineNumber:      1,
		Description:     "Consulting services",
		Quantity:        NewDecimalFromFloat(10.00),
		Unit:            "hours",
		UnitPrice:       NewDecimalFromFloat(100.00),
		DiscountPercent: NewDecimalFromFloat(5.00),
		VATRate:         NewDecimalFromFloat(22.00),
		LineSubtotal:    NewDecimalFromFloat(950.00),
		LineVAT:         NewDecimalFromFloat(209.00),
		LineTotal:       NewDecimalFromFloat(1159.00),
	}

	if il.Description != "Consulting services" {
		t.Errorf("expected Consulting services, got %s", il.Description)
	}
}
