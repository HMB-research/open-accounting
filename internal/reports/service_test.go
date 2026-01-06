package reports

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCashFlowStatement(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Setup mock data
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Cash sale",
			Lines: []JournalLine{
				{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.NewFromFloat(1000), Credit: decimal.Zero},
				{AccountCode: "4000", AccountType: "REVENUE", Debit: decimal.Zero, Credit: decimal.NewFromFloat(1000)},
			},
		},
		{
			ID:          "je-2",
			EntryDate:   time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC),
			Description: "Supplier payment",
			Lines: []JournalLine{
				{AccountCode: "2000", AccountType: "LIABILITY", Debit: decimal.NewFromFloat(500), Credit: decimal.Zero},
				{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.Zero, Credit: decimal.NewFromFloat(500)},
			},
		},
	}

	req := &CashFlowRequest{
		StartDate: "2024-01-01",
		EndDate:   "2024-01-31",
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "2024-01-01", result.StartDate)
	assert.Equal(t, "2024-01-31", result.EndDate)

	// Operating activities should show net cash from sales and payments
	assert.NotEmpty(t, result.OperatingActivities)
}

func TestCashFlowOperatingActivities(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Cash received from customers
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Cash from customer",
			Lines: []JournalLine{
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.NewFromFloat(5000), Credit: decimal.Zero},
				{AccountCode: "1200", AccountType: "ASSET", AccountName: "Accounts Receivable", Debit: decimal.Zero, Credit: decimal.NewFromFloat(5000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have cash received from customers
	var cashFromCustomers decimal.Decimal
	for _, item := range result.OperatingActivities {
		if item.Code == "CF_OPER_RECEIPTS" {
			cashFromCustomers = item.Amount
			break
		}
	}
	assert.True(t, cashFromCustomers.GreaterThan(decimal.Zero), "Should have positive cash from customers")
}

func TestCashFlowInvestingActivities(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Purchase of fixed asset (cash decreases, asset increases)
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Computer purchase",
			Lines: []JournalLine{
				{AccountCode: "1500", AccountType: "ASSET", AccountName: "Fixed Assets", Debit: decimal.NewFromFloat(2000), Credit: decimal.Zero},
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.Zero, Credit: decimal.NewFromFloat(2000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have fixed asset purchase (negative cash flow)
	var fixedAssetsCash decimal.Decimal
	for _, item := range result.InvestingActivities {
		if item.Code == "CF_INV_FIXED_ASSETS" {
			fixedAssetsCash = item.Amount
			break
		}
	}
	assert.True(t, fixedAssetsCash.LessThan(decimal.Zero), "Fixed asset purchase should show negative cash flow")
}

func TestCashFlowFinancingActivities(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Loan received
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Bank loan received",
			Lines: []JournalLine{
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.NewFromFloat(10000), Credit: decimal.Zero},
				{AccountCode: "2000", AccountType: "LIABILITY", AccountName: "Bank Loan", Debit: decimal.Zero, Credit: decimal.NewFromFloat(10000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have loan proceeds (positive cash flow)
	var loanCash decimal.Decimal
	for _, item := range result.FinancingActivities {
		if item.Code == "CF_FIN_LOANS_RCVD" {
			loanCash = item.Amount
			break
		}
	}
	assert.True(t, loanCash.GreaterThan(decimal.Zero), "Loan received should show positive cash flow")
}

func TestCashFlowOpeningClosingBalance(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Set opening cash balance
	mockRepo.CashBalance = decimal.NewFromFloat(5000)

	// Cash received
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Cash sale",
			Lines: []JournalLine{
				{AccountCode: "1000", AccountType: "ASSET", Debit: decimal.NewFromFloat(1000), Credit: decimal.Zero},
				{AccountCode: "4000", AccountType: "REVENUE", Debit: decimal.Zero, Credit: decimal.NewFromFloat(1000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)
	assert.Equal(t, decimal.NewFromFloat(5000).String(), result.OpeningCash.String())
	// Closing = Opening + Net Change
	assert.Equal(t, result.OpeningCash.Add(result.NetCashChange).String(), result.ClosingCash.String())
}

// MockRepository tests

func TestNewMockRepository(t *testing.T) {
	repo := NewMockRepository()
	require.NotNil(t, repo)
	assert.Empty(t, repo.JournalEntries)
	assert.True(t, repo.CashBalance.IsZero())
	assert.Empty(t, repo.ContactBalances)
	assert.Empty(t, repo.ContactInvoices)
}

func TestMockRepository_GetOutstandingInvoicesByContact(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()

	// Test empty result
	contacts, err := repo.GetOutstandingInvoicesByContact(ctx, "test_schema", "tenant-1", "SALES", time.Now())
	require.NoError(t, err)
	assert.Empty(t, contacts)

	// Add mock contact balances
	repo.ContactBalances = []ContactBalance{
		{
			ContactID:     "contact-1",
			ContactName:   "Customer A",
			ContactCode:   "CUST-001",
			ContactEmail:  "customer.a@test.com",
			Balance:       decimal.NewFromInt(5000),
			InvoiceCount:  3,
			OldestInvoice: "2024-01-15",
		},
		{
			ContactID:    "contact-2",
			ContactName:  "Customer B",
			Balance:      decimal.NewFromInt(2500),
			InvoiceCount: 1,
		},
	}

	contacts, err = repo.GetOutstandingInvoicesByContact(ctx, "test_schema", "tenant-1", "SALES", time.Now())
	require.NoError(t, err)
	assert.Len(t, contacts, 2)
	assert.Equal(t, "contact-1", contacts[0].ContactID)
	assert.Equal(t, "Customer A", contacts[0].ContactName)
	assert.True(t, contacts[0].Balance.Equal(decimal.NewFromInt(5000)))
}

func TestMockRepository_GetOutstandingInvoicesByContact_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetContactBalancesErr = assert.AnError

	contacts, err := repo.GetOutstandingInvoicesByContact(ctx, "test_schema", "tenant-1", "SALES", time.Now())
	require.Error(t, err)
	assert.Nil(t, contacts)
}

func TestMockRepository_GetContactInvoices(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()

	// Test empty result
	invoices, err := repo.GetContactInvoices(ctx, "test_schema", "tenant-1", "contact-1", "SALES", time.Now())
	require.NoError(t, err)
	assert.Empty(t, invoices)

	// Add mock invoices
	repo.ContactInvoices = []BalanceInvoice{
		{
			InvoiceID:         "inv-1",
			InvoiceNumber:     "INV-2024-001",
			InvoiceDate:       "2024-01-15",
			DueDate:           "2024-02-15",
			TotalAmount:       decimal.NewFromInt(1000),
			AmountPaid:        decimal.NewFromInt(200),
			OutstandingAmount: decimal.NewFromInt(800),
			Currency:          "EUR",
			DaysOverdue:       30,
		},
		{
			InvoiceID:         "inv-2",
			InvoiceNumber:     "INV-2024-002",
			InvoiceDate:       "2024-02-01",
			DueDate:           "2024-03-01",
			TotalAmount:       decimal.NewFromInt(500),
			AmountPaid:        decimal.Zero,
			OutstandingAmount: decimal.NewFromInt(500),
			Currency:          "EUR",
			DaysOverdue:       15,
		},
	}

	invoices, err = repo.GetContactInvoices(ctx, "test_schema", "tenant-1", "contact-1", "SALES", time.Now())
	require.NoError(t, err)
	assert.Len(t, invoices, 2)
	assert.Equal(t, "INV-2024-001", invoices[0].InvoiceNumber)
	assert.True(t, invoices[0].OutstandingAmount.Equal(decimal.NewFromInt(800)))
}

func TestMockRepository_GetContactInvoices_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetContactInvoicesErr = assert.AnError

	invoices, err := repo.GetContactInvoices(ctx, "test_schema", "tenant-1", "contact-1", "SALES", time.Now())
	require.Error(t, err)
	assert.Nil(t, invoices)
}

func TestMockRepository_GetContact(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()

	// Test empty/default result
	contact, err := repo.GetContact(ctx, "test_schema", "tenant-1", "contact-1")
	require.NoError(t, err)
	assert.Empty(t, contact.ID)

	// Set mock contact
	repo.Contact = ContactInfo{
		ID:    "contact-1",
		Name:  "Test Customer",
		Code:  "CUST-001",
		Email: "test@example.com",
	}

	contact, err = repo.GetContact(ctx, "test_schema", "tenant-1", "contact-1")
	require.NoError(t, err)
	assert.Equal(t, "contact-1", contact.ID)
	assert.Equal(t, "Test Customer", contact.Name)
	assert.Equal(t, "CUST-001", contact.Code)
	assert.Equal(t, "test@example.com", contact.Email)
}

func TestMockRepository_GetContact_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetContactErr = assert.AnError

	contact, err := repo.GetContact(ctx, "test_schema", "tenant-1", "contact-1")
	require.Error(t, err)
	assert.Empty(t, contact.ID)
}

func TestMockRepository_GetCashAccountBalance(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()

	// Test zero balance
	balance, err := repo.GetCashAccountBalance(ctx, "test_schema", "tenant-1", time.Now())
	require.NoError(t, err)
	assert.True(t, balance.IsZero())

	// Set mock balance
	repo.CashBalance = decimal.NewFromFloat(12500.50)

	balance, err = repo.GetCashAccountBalance(ctx, "test_schema", "tenant-1", time.Now())
	require.NoError(t, err)
	assert.True(t, balance.Equal(decimal.NewFromFloat(12500.50)))
}

func TestMockRepository_GetCashAccountBalance_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.GetCashBalanceErr = assert.AnError

	balance, err := repo.GetCashAccountBalance(ctx, "test_schema", "tenant-1", time.Now())
	require.Error(t, err)
	assert.True(t, balance.IsZero())
}

// Balance Confirmation Service tests

func TestGetBalanceConfirmationSummary_Receivables(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Setup mock contact balances
	mockRepo.ContactBalances = []ContactBalance{
		{
			ContactID:     "contact-1",
			ContactName:   "Customer A",
			Balance:       decimal.NewFromInt(5000),
			InvoiceCount:  3,
			OldestInvoice: "2024-01-15",
		},
		{
			ContactID:    "contact-2",
			ContactName:  "Customer B",
			Balance:      decimal.NewFromInt(2500),
			InvoiceCount: 2,
		},
	}

	req := &BalanceConfirmationRequest{
		Type:     "RECEIVABLE",
		AsOfDate: "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, BalanceTypeReceivable, result.Type)
	assert.Equal(t, "2024-03-31", result.AsOfDate)
	assert.True(t, result.TotalBalance.Equal(decimal.NewFromInt(7500)))
	assert.Equal(t, 2, result.ContactCount)
	assert.Equal(t, 5, result.InvoiceCount)
	assert.Len(t, result.Contacts, 2)
}

func TestGetBalanceConfirmationSummary_Payables(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Setup mock contact balances for payables
	mockRepo.ContactBalances = []ContactBalance{
		{
			ContactID:    "supplier-1",
			ContactName:  "Supplier X",
			Balance:      decimal.NewFromInt(8000),
			InvoiceCount: 4,
		},
	}

	req := &BalanceConfirmationRequest{
		Type:     "PAYABLE",
		AsOfDate: "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, BalanceTypePayable, result.Type)
	assert.True(t, result.TotalBalance.Equal(decimal.NewFromInt(8000)))
	assert.Equal(t, 1, result.ContactCount)
}

func TestGetBalanceConfirmationSummary_Empty(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	req := &BalanceConfirmationRequest{
		Type:     "RECEIVABLE",
		AsOfDate: "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.TotalBalance.IsZero())
	assert.Equal(t, 0, result.ContactCount)
	assert.Equal(t, 0, result.InvoiceCount)
}

func TestGetBalanceConfirmationSummary_InvalidDate(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	req := &BalanceConfirmationRequest{
		Type:     "RECEIVABLE",
		AsOfDate: "invalid-date",
	}

	result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid as_of_date")
}

func TestGetBalanceConfirmationSummary_RepoError(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	mockRepo.GetContactBalancesErr = assert.AnError
	svc := NewServiceWithRepository(mockRepo)

	req := &BalanceConfirmationRequest{
		Type:     "RECEIVABLE",
		AsOfDate: "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGetBalanceConfirmation_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Setup mock contact
	mockRepo.Contact = ContactInfo{
		ID:    "contact-1",
		Name:  "Test Customer",
		Code:  "CUST-001",
		Email: "test@example.com",
	}

	// Setup mock invoices
	mockRepo.ContactInvoices = []BalanceInvoice{
		{
			InvoiceID:         "inv-1",
			InvoiceNumber:     "INV-2024-001",
			InvoiceDate:       "2024-01-15",
			DueDate:           "2024-02-15",
			TotalAmount:       decimal.NewFromInt(1000),
			AmountPaid:        decimal.NewFromInt(200),
			OutstandingAmount: decimal.NewFromInt(800),
			Currency:          "EUR",
			DaysOverdue:       30,
		},
		{
			InvoiceID:         "inv-2",
			InvoiceNumber:     "INV-2024-002",
			InvoiceDate:       "2024-02-01",
			DueDate:           "2024-03-01",
			TotalAmount:       decimal.NewFromInt(500),
			AmountPaid:        decimal.Zero,
			OutstandingAmount: decimal.NewFromInt(500),
			Currency:          "EUR",
			DaysOverdue:       15,
		},
	}

	req := &BalanceConfirmationRequest{
		ContactID: "contact-1",
		Type:      "RECEIVABLE",
		AsOfDate:  "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "contact-1", result.ContactID)
	assert.Equal(t, "Test Customer", result.ContactName)
	assert.Equal(t, "CUST-001", result.ContactCode)
	assert.Equal(t, "test@example.com", result.ContactEmail)
	assert.Equal(t, BalanceTypeReceivable, result.Type)
	assert.Equal(t, "2024-03-31", result.AsOfDate)
	assert.True(t, result.TotalBalance.Equal(decimal.NewFromInt(1300)))
	assert.Len(t, result.Invoices, 2)
}

func TestGetBalanceConfirmation_Payable(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	mockRepo.Contact = ContactInfo{
		ID:   "supplier-1",
		Name: "Test Supplier",
	}

	mockRepo.ContactInvoices = []BalanceInvoice{
		{
			InvoiceID:         "pinv-1",
			OutstandingAmount: decimal.NewFromInt(3000),
		},
	}

	req := &BalanceConfirmationRequest{
		ContactID: "supplier-1",
		Type:      "PAYABLE",
		AsOfDate:  "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, BalanceTypePayable, result.Type)
	assert.True(t, result.TotalBalance.Equal(decimal.NewFromInt(3000)))
}

func TestGetBalanceConfirmation_MissingContactID(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	req := &BalanceConfirmationRequest{
		ContactID: "", // Empty
		Type:      "RECEIVABLE",
		AsOfDate:  "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "contact_id is required")
}

func TestGetBalanceConfirmation_InvalidDate(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	req := &BalanceConfirmationRequest{
		ContactID: "contact-1",
		Type:      "RECEIVABLE",
		AsOfDate:  "not-a-date",
	}

	result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid as_of_date")
}

func TestGetBalanceConfirmation_ContactNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	mockRepo.GetContactErr = assert.AnError
	svc := NewServiceWithRepository(mockRepo)

	req := &BalanceConfirmationRequest{
		ContactID: "nonexistent",
		Type:      "RECEIVABLE",
		AsOfDate:  "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGetBalanceConfirmation_InvoicesError(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	mockRepo.Contact = ContactInfo{ID: "contact-1", Name: "Test"}
	mockRepo.GetContactInvoicesErr = assert.AnError
	svc := NewServiceWithRepository(mockRepo)

	req := &BalanceConfirmationRequest{
		ContactID: "contact-1",
		Type:      "RECEIVABLE",
		AsOfDate:  "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

	require.Error(t, err)
	assert.Nil(t, result)
}

func TestGetBalanceConfirmation_NoInvoices(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	mockRepo.Contact = ContactInfo{
		ID:   "contact-1",
		Name: "Customer with no outstanding",
	}
	// No invoices

	req := &BalanceConfirmationRequest{
		ContactID: "contact-1",
		Type:      "RECEIVABLE",
		AsOfDate:  "2024-03-31",
	}

	result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "test_schema", req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.TotalBalance.IsZero())
	assert.Empty(t, result.Invoices)
}

// Test account classification helper functions

func TestIsFixedAssetAccount(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"1500", true},  // Fixed assets
		{"1550", true},  // Fixed assets
		{"1599", true},  // Fixed assets
		{"1000", false}, // Cash
		{"1200", false}, // Receivables
		{"2000", false}, // Liabilities
		{"15", false},   // Too short
		{"", false},     // Empty
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := isFixedAssetAccount(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLoanAccount(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"2000", true},  // Short-term loans
		{"2050", true},  // Short-term loans
		{"2500", true},  // Long-term loans
		{"2599", true},  // Long-term loans
		{"1000", false}, // Cash
		{"3000", false}, // Equity
		{"20", false},   // Too short
		{"", false},     // Empty
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := isLoanAccount(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsDividendAccount(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"3000", true},  // Equity
		{"3100", true},  // Equity
		{"3999", true},  // Equity
		{"1000", false}, // Assets
		{"2000", false}, // Liabilities
		{"4000", false}, // Revenue
		{"3", false},    // Too short
		{"", false},     // Empty
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := isDividendAccount(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test types and constants

func TestBalanceConfirmationTypes(t *testing.T) {
	assert.Equal(t, BalanceConfirmationType("RECEIVABLE"), BalanceTypeReceivable)
	assert.Equal(t, BalanceConfirmationType("PAYABLE"), BalanceTypePayable)
}

func TestCashFlowCodeConstants(t *testing.T) {
	// Operating activities
	assert.Equal(t, "CF_OPER_RECEIPTS", CFOperReceipts)
	assert.Equal(t, "CF_OPER_PAYMENTS", CFOperPayments)
	assert.Equal(t, "CF_OPER_WAGES", CFOperWages)
	assert.Equal(t, "CF_OPER_TAXES", CFOperTaxes)
	assert.Equal(t, "CF_OPER_TOTAL", CFOperTotal)

	// Investing activities
	assert.Equal(t, "CF_INV_FIXED_ASSETS", CFInvFixedAssets)
	assert.Equal(t, "CF_INV_TOTAL", CFInvTotal)

	// Financing activities
	assert.Equal(t, "CF_FIN_LOANS_RCVD", CFFinLoansRcvd)
	assert.Equal(t, "CF_FIN_DIVIDENDS_PD", CFFinDividendsPd)
	assert.Equal(t, "CF_FIN_TOTAL", CFFinTotal)
}

// Additional cash flow error tests

func TestGenerateCashFlowStatement_InvalidStartDate(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	req := &CashFlowRequest{
		StartDate: "invalid",
		EndDate:   "2024-01-31",
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid start date")
}

func TestGenerateCashFlowStatement_InvalidEndDate(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	req := &CashFlowRequest{
		StartDate: "2024-01-01",
		EndDate:   "invalid",
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid end date")
}

func TestGenerateCashFlowStatement_JournalEntriesError(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	mockRepo.GetEntriesErr = assert.AnError
	svc := NewServiceWithRepository(mockRepo)

	req := &CashFlowRequest{
		StartDate: "2024-01-01",
		EndDate:   "2024-01-31",
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get journal entries")
}

func TestGenerateCashFlowStatement_CashBalanceError(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	mockRepo.GetCashBalanceErr = assert.AnError
	svc := NewServiceWithRepository(mockRepo)

	req := &CashFlowRequest{
		StartDate: "2024-01-01",
		EndDate:   "2024-01-31",
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "get opening cash")
}

func TestCashFlowFinancingActivities_LoanRepayment(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Loan repayment (cash decreases, loan liability decreases)
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Loan repayment",
			Lines: []JournalLine{
				{AccountCode: "2000", AccountType: "LIABILITY", AccountName: "Bank Loan", Debit: decimal.NewFromFloat(5000), Credit: decimal.Zero},
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.Zero, Credit: decimal.NewFromFloat(5000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have loan repayment (negative cash flow)
	var loanCash decimal.Decimal
	for _, item := range result.FinancingActivities {
		if item.Code == "CF_FIN_LOANS_REPAID" || item.Code == CFFinLoansRcvd {
			loanCash = item.Amount
			break
		}
	}
	assert.True(t, loanCash.LessThan(decimal.Zero), "Loan repayment should show negative cash flow")
}

func TestCashFlowFinancingActivities_DividendsPaid(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Dividend payment (cash decreases, equity decreases)
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Dividend payment",
			Lines: []JournalLine{
				{AccountCode: "3000", AccountType: "EQUITY", AccountName: "Dividends", Debit: decimal.NewFromFloat(2000), Credit: decimal.Zero},
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.Zero, Credit: decimal.NewFromFloat(2000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have dividend payment
	var dividendCash decimal.Decimal
	for _, item := range result.FinancingActivities {
		if item.Code == "CF_FIN_DIVIDENDS_PD" {
			dividendCash = item.Amount
			break
		}
	}
	assert.True(t, dividendCash.LessThan(decimal.Zero), "Dividend payment should show negative cash flow")
}

func TestCashFlowOperatingActivities_ExpensePayment(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Cash payment for expense
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Office supplies",
			Lines: []JournalLine{
				{AccountCode: "5000", AccountType: "EXPENSE", AccountName: "Office Expenses", Debit: decimal.NewFromFloat(500), Credit: decimal.Zero},
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.Zero, Credit: decimal.NewFromFloat(500)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have payments to suppliers
	var payments decimal.Decimal
	for _, item := range result.OperatingActivities {
		if item.Code == CFOperPayments {
			payments = item.Amount
			break
		}
	}
	assert.True(t, payments.LessThan(decimal.Zero), "Expense payment should show negative cash flow")
}

func TestCashFlowOperatingActivities_LiabilityPayment(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// Cash payment to supplier (accounts payable)
	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "je-1",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Supplier payment",
			Lines: []JournalLine{
				{AccountCode: "2100", AccountType: "LIABILITY", AccountName: "Accounts Payable", Debit: decimal.NewFromFloat(1000), Credit: decimal.Zero},
				{AccountCode: "1000", AccountType: "ASSET", AccountName: "Cash", Debit: decimal.Zero, Credit: decimal.NewFromFloat(1000)},
			},
		},
	}

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)

	// Should have payments to suppliers (non-loan liabilities skip the financing filter)
	// Since 2100 is NOT in loan accounts (20xx or 25xx), it should be classified as operating
	var payments decimal.Decimal
	for _, item := range result.OperatingActivities {
		if item.Code == CFOperPayments {
			payments = item.Amount
			break
		}
	}
	assert.True(t, payments.LessThan(decimal.Zero), "Liability payment should show negative cash flow")
}

func TestCashFlowStatement_EmptyJournalEntries(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	// No journal entries
	mockRepo.JournalEntries = []JournalEntryWithLines{}
	mockRepo.CashBalance = decimal.NewFromFloat(5000)

	req := &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"}
	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", req)

	require.NoError(t, err)
	assert.Equal(t, decimal.NewFromFloat(5000).String(), result.OpeningCash.String())
	assert.Equal(t, decimal.NewFromFloat(5000).String(), result.ClosingCash.String())
	assert.True(t, result.NetCashChange.IsZero())
}

func TestIsCashAccount(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
	}{
		{"1000", true},  // Cash
		{"1050", true},  // Cash
		{"1099", true},  // Cash
		{"1100", false}, // Not cash
		{"1200", false}, // Receivables
		{"10", false},   // Too short
		{"", false},     // Empty
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			result := isCashAccount(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}
