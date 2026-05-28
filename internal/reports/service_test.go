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

func TestGenerateCashFlowStatementRejectsInvertedPeriod(t *testing.T) {
	svc := NewServiceWithRepository(NewMockRepository())

	_, err := svc.GenerateCashFlowStatement(context.Background(), "tenant-1", "schema_tenant1", &CashFlowRequest{
		StartDate: "2024-02-01",
		EndDate:   "2024-01-31",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "end date must be on or after start date")
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
				{AccountCode: "2400", AccountType: "LIABILITY", AccountName: "Short-term Loan", Debit: decimal.Zero, Credit: decimal.NewFromFloat(10000)},
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

func TestCashFlowDefaultChartOperatingClassifications(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "receipt",
			EntryDate:   time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
			Description: "Customer receipt",
			Lines: []JournalLine{
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.NewFromInt(1000), Credit: decimal.Zero},
				{AccountCode: "1200", AccountType: "ASSET", AccountName: "Accounts Receivable", Debit: decimal.Zero, Credit: decimal.NewFromInt(1000)},
			},
		},
		{
			ID:          "supplier-payment",
			EntryDate:   time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC),
			Description: "Supplier payment",
			Lines: []JournalLine{
				{AccountCode: "2100", AccountType: "LIABILITY", AccountName: "Accounts Payable", Debit: decimal.NewFromInt(400), Credit: decimal.Zero},
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.Zero, Credit: decimal.NewFromInt(400)},
			},
		},
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"})

	require.NoError(t, err)
	assert.Equal(t, "1000", cashFlowAmount(result.OperatingActivities, CFOperReceipts).String())
	assert.Equal(t, "-400", cashFlowAmount(result.OperatingActivities, CFOperPayments).String())
	assert.Equal(t, "600", result.TotalOperating.String())
	assert.True(t, result.TotalFinancing.IsZero(), "accounts payable should not be classified as financing")
}

func TestCashFlowOperatingSubcategories(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "salary",
			EntryDate:   time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC),
			Description: "Salary payment",
			Lines: []JournalLine{
				{AccountCode: "5200", AccountType: "EXPENSE", AccountName: "Salary Expenses", Debit: decimal.NewFromInt(700), Credit: decimal.Zero},
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.Zero, Credit: decimal.NewFromInt(700)},
			},
		},
		{
			ID:          "tax",
			EntryDate:   time.Date(2024, 1, 13, 0, 0, 0, 0, time.UTC),
			Description: "VAT payment",
			Lines: []JournalLine{
				{AccountCode: "2200", AccountType: "LIABILITY", AccountName: "VAT Payable", Debit: decimal.NewFromInt(200), Credit: decimal.Zero},
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.Zero, Credit: decimal.NewFromInt(200)},
			},
		},
		{
			ID:          "interest",
			EntryDate:   time.Date(2024, 1, 14, 0, 0, 0, 0, time.UTC),
			Description: "Interest payment",
			Lines: []JournalLine{
				{AccountCode: "5700", AccountType: "EXPENSE", AccountName: "Interest Expense", Debit: decimal.NewFromInt(50), Credit: decimal.Zero},
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.Zero, Credit: decimal.NewFromInt(50)},
			},
		},
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"})

	require.NoError(t, err)
	assert.Equal(t, "-700", cashFlowAmount(result.OperatingActivities, CFOperWages).String())
	assert.Equal(t, "-200", cashFlowAmount(result.OperatingActivities, CFOperTaxes).String())
	assert.Equal(t, "-50", cashFlowAmount(result.OperatingActivities, CFOperInterestPd).String())
	assert.Equal(t, "-950", result.TotalOperating.String())
}

func TestCashFlowFinancingSubcategories(t *testing.T) {
	ctx := context.Background()
	mockRepo := NewMockRepository()
	svc := NewServiceWithRepository(mockRepo)

	mockRepo.JournalEntries = []JournalEntryWithLines{
		{
			ID:          "loan",
			EntryDate:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			Description: "Loan received",
			Lines: []JournalLine{
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.NewFromInt(1000), Credit: decimal.Zero},
				{AccountCode: "2400", AccountType: "LIABILITY", AccountName: "Short-term Loans", Debit: decimal.Zero, Credit: decimal.NewFromInt(1000)},
			},
		},
		{
			ID:          "share-capital",
			EntryDate:   time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
			Description: "Share capital contribution",
			Lines: []JournalLine{
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.NewFromInt(500), Credit: decimal.Zero},
				{AccountCode: "3100", AccountType: "EQUITY", AccountName: "Share Capital", Debit: decimal.Zero, Credit: decimal.NewFromInt(500)},
			},
		},
		{
			ID:          "dividends",
			EntryDate:   time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
			Description: "Dividends paid",
			Lines: []JournalLine{
				{AccountCode: "3200", AccountType: "EQUITY", AccountName: "Retained Earnings", Debit: decimal.NewFromInt(200), Credit: decimal.Zero},
				{AccountCode: "1100", AccountType: "ASSET", AccountName: "Cash and Bank", Debit: decimal.Zero, Credit: decimal.NewFromInt(200)},
			},
		},
	}

	result, err := svc.GenerateCashFlowStatement(ctx, "tenant-1", "schema_tenant1", &CashFlowRequest{StartDate: "2024-01-01", EndDate: "2024-01-31"})

	require.NoError(t, err)
	assert.Equal(t, "1000", cashFlowAmount(result.FinancingActivities, CFFinLoansRcvd).String())
	assert.Equal(t, "500", cashFlowAmount(result.FinancingActivities, CFFinShares).String())
	assert.Equal(t, "-200", cashFlowAmount(result.FinancingActivities, CFFinDividendsPd).String())
	assert.Equal(t, "1300", result.TotalFinancing.String())
	assert.Empty(t, result.OperatingActivities[:len(result.OperatingActivities)-1], "financing entries should not appear as operating lines")
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

func cashFlowAmount(items []CashFlowItem, code string) decimal.Decimal {
	for _, item := range items {
		if item.Code == code {
			return item.Amount
		}
	}
	return decimal.Zero
}

func TestGetBalanceConfirmationSummary(t *testing.T) {
	ctx := context.Background()

	t.Run("returns receivable balance summary", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		mockRepo.ContactBalances = []ContactBalance{
			{
				ContactID:    "contact-1",
				ContactName:  "Customer A",
				ContactCode:  "CUST-001",
				Balance:      decimal.NewFromFloat(1500.00),
				InvoiceCount: 2,
			},
			{
				ContactID:    "contact-2",
				ContactName:  "Customer B",
				ContactCode:  "CUST-002",
				Balance:      decimal.NewFromFloat(2500.00),
				InvoiceCount: 3,
			},
		}

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "schema_tenant1", req)

		require.NoError(t, err)
		assert.Equal(t, BalanceTypeReceivable, result.Type)
		assert.Equal(t, "2024-01-31", result.AsOfDate)
		assert.True(t, result.TotalBalance.Equal(decimal.NewFromFloat(4000.00)))
		assert.Equal(t, 2, result.ContactCount)
		assert.Equal(t, 5, result.InvoiceCount)
		assert.Len(t, result.Contacts, 2)
	})

	t.Run("returns payable balance summary", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		mockRepo.ContactBalances = []ContactBalance{
			{
				ContactID:    "supplier-1",
				ContactName:  "Supplier X",
				ContactCode:  "SUP-001",
				Balance:      decimal.NewFromFloat(3000.00),
				InvoiceCount: 1,
			},
		}

		req := &BalanceConfirmationRequest{
			Type:     "PAYABLE",
			AsOfDate: "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "schema_tenant1", req)

		require.NoError(t, err)
		assert.Equal(t, BalanceTypePayable, result.Type)
		assert.True(t, result.TotalBalance.Equal(decimal.NewFromFloat(3000.00)))
		assert.Equal(t, 1, result.ContactCount)
		assert.Equal(t, 1, result.InvoiceCount)
	})

	t.Run("returns error for invalid date format", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "invalid-date",
		}

		result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "schema_tenant1", req)

		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid as_of_date")
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)
		mockRepo.GetContactBalancesErr = assert.AnError

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "schema_tenant1", req)

		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "get contact balances")
	})

	t.Run("handles empty contact list", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmationSummary(ctx, "tenant-1", "schema_tenant1", req)

		require.NoError(t, err)
		assert.True(t, result.TotalBalance.IsZero())
		assert.Equal(t, 0, result.ContactCount)
		assert.Equal(t, 0, result.InvoiceCount)
		assert.Empty(t, result.Contacts)
	})
}

func TestGetBalanceConfirmation(t *testing.T) {
	ctx := context.Background()

	t.Run("returns individual balance confirmation", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		mockRepo.Contact = ContactInfo{
			ID:    "contact-1",
			Name:  "Customer A",
			Code:  "CUST-001",
			Email: "customer@example.com",
		}

		mockRepo.ContactInvoices = []BalanceInvoice{
			{
				InvoiceID:         "inv-1",
				InvoiceNumber:     "INV-001",
				InvoiceDate:       "2024-01-15",
				DueDate:           "2024-02-15",
				TotalAmount:       decimal.NewFromFloat(1000.00),
				AmountPaid:        decimal.NewFromFloat(500.00),
				OutstandingAmount: decimal.NewFromFloat(500.00),
				Currency:          "EUR",
				DaysOverdue:       0,
			},
			{
				InvoiceID:         "inv-2",
				InvoiceNumber:     "INV-002",
				InvoiceDate:       "2024-01-20",
				DueDate:           "2024-02-20",
				TotalAmount:       decimal.NewFromFloat(750.00),
				AmountPaid:        decimal.Zero,
				OutstandingAmount: decimal.NewFromFloat(750.00),
				Currency:          "EUR",
				DaysOverdue:       0,
			},
		}

		req := &BalanceConfirmationRequest{
			ContactID: "contact-1",
			Type:      "RECEIVABLE",
			AsOfDate:  "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "schema_tenant1", req)

		require.NoError(t, err)
		assert.Equal(t, "contact-1", result.ContactID)
		assert.Equal(t, "Customer A", result.ContactName)
		assert.Equal(t, "CUST-001", result.ContactCode)
		assert.Equal(t, "customer@example.com", result.ContactEmail)
		assert.Equal(t, BalanceTypeReceivable, result.Type)
		assert.Equal(t, "2024-01-31", result.AsOfDate)
		assert.True(t, result.TotalBalance.Equal(decimal.NewFromFloat(1250.00)))
		assert.Len(t, result.Invoices, 2)
	})

	t.Run("returns payable balance confirmation", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		mockRepo.Contact = ContactInfo{
			ID:   "supplier-1",
			Name: "Supplier X",
			Code: "SUP-001",
		}

		mockRepo.ContactInvoices = []BalanceInvoice{
			{
				InvoiceID:         "pinv-1",
				InvoiceNumber:     "PINV-001",
				OutstandingAmount: decimal.NewFromFloat(2000.00),
			},
		}

		req := &BalanceConfirmationRequest{
			ContactID: "supplier-1",
			Type:      "PAYABLE",
			AsOfDate:  "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "schema_tenant1", req)

		require.NoError(t, err)
		assert.Equal(t, BalanceTypePayable, result.Type)
		assert.True(t, result.TotalBalance.Equal(decimal.NewFromFloat(2000.00)))
	})

	t.Run("returns error when contact_id missing", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			Type:     "RECEIVABLE",
			AsOfDate: "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "schema_tenant1", req)

		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "contact_id is required")
	})

	t.Run("returns error for invalid date format", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		req := &BalanceConfirmationRequest{
			ContactID: "contact-1",
			Type:      "RECEIVABLE",
			AsOfDate:  "bad-date",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "schema_tenant1", req)

		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid as_of_date")
	})

	t.Run("returns error when contact not found", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)
		mockRepo.GetContactErr = assert.AnError

		req := &BalanceConfirmationRequest{
			ContactID: "nonexistent",
			Type:      "RECEIVABLE",
			AsOfDate:  "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "schema_tenant1", req)

		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "get contact")
	})

	t.Run("returns error when invoices query fails", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)
		mockRepo.Contact = ContactInfo{ID: "contact-1", Name: "Test"}
		mockRepo.GetContactInvoicesErr = assert.AnError

		req := &BalanceConfirmationRequest{
			ContactID: "contact-1",
			Type:      "RECEIVABLE",
			AsOfDate:  "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "schema_tenant1", req)

		assert.Nil(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "get contact invoices")
	})

	t.Run("handles contact with no invoices", func(t *testing.T) {
		mockRepo := NewMockRepository()
		svc := NewServiceWithRepository(mockRepo)

		mockRepo.Contact = ContactInfo{
			ID:   "contact-1",
			Name: "Customer A",
		}

		req := &BalanceConfirmationRequest{
			ContactID: "contact-1",
			Type:      "RECEIVABLE",
			AsOfDate:  "2024-01-31",
		}

		result, err := svc.GetBalanceConfirmation(ctx, "tenant-1", "schema_tenant1", req)

		require.NoError(t, err)
		assert.True(t, result.TotalBalance.IsZero())
		assert.Empty(t, result.Invoices)
	})
}
