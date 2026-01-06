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
