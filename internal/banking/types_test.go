package banking

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestTransactionStatus_Constants(t *testing.T) {
	tests := []struct {
		status   TransactionStatus
		expected string
	}{
		{StatusUnmatched, "UNMATCHED"},
		{StatusMatched, "MATCHED"},
		{StatusReconciled, "RECONCILED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestReconciliationStatus_Constants(t *testing.T) {
	tests := []struct {
		status   ReconciliationStatus
		expected string
	}{
		{ReconciliationInProgress, "IN_PROGRESS"},
		{ReconciliationCompleted, "COMPLETED"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestBankAccount_Fields(t *testing.T) {
	now := time.Now()
	glAccountID := "gl-123"
	ba := BankAccount{
		ID:            "ba-1",
		TenantID:      "tenant-1",
		Name:          "Main Account",
		AccountNumber: "EE123456789012345678",
		BankName:      "Test Bank",
		SwiftCode:     "TESTEE2X",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
		IsDefault:     true,
		IsActive:      true,
		CreatedAt:     now,
		Balance:       decimal.NewFromFloat(1000.00),
	}

	assert.Equal(t, "ba-1", ba.ID)
	assert.Equal(t, "Main Account", ba.Name)
	assert.Equal(t, "EE123456789012345678", ba.AccountNumber)
	assert.Equal(t, "Test Bank", ba.BankName)
	assert.Equal(t, "EUR", ba.Currency)
	assert.True(t, ba.IsDefault)
	assert.True(t, ba.IsActive)
	assert.Equal(t, "gl-123", *ba.GLAccountID)
}

func TestBankTransaction_Fields(t *testing.T) {
	now := time.Now()
	valueDate := now.AddDate(0, 0, 1)
	matchedPaymentID := "pay-123"
	journalEntryID := "je-123"
	reconciliationID := "rec-123"

	bt := BankTransaction{
		ID:                  "bt-1",
		TenantID:            "tenant-1",
		BankAccountID:       "ba-1",
		TransactionDate:     now,
		ValueDate:           &valueDate,
		Amount:              decimal.NewFromFloat(250.00),
		Currency:            "EUR",
		Description:         "Payment received",
		Reference:           "REF123",
		CounterpartyName:    "John Doe",
		CounterpartyAccount: "EE987654321098765432",
		Status:              StatusUnmatched,
		MatchedPaymentID:    &matchedPaymentID,
		JournalEntryID:      &journalEntryID,
		ReconciliationID:    &reconciliationID,
		ImportedAt:          now,
		ExternalID:          "ext-123",
	}

	assert.Equal(t, "bt-1", bt.ID)
	assert.Equal(t, "ba-1", bt.BankAccountID)
	assert.Equal(t, StatusUnmatched, bt.Status)
	assert.Equal(t, "Payment received", bt.Description)
	assert.Equal(t, "John Doe", bt.CounterpartyName)
	assert.NotNil(t, bt.ValueDate)
}

func TestBankReconciliation_Fields(t *testing.T) {
	now := time.Now()
	completedAt := now.AddDate(0, 0, 1)

	br := BankReconciliation{
		ID:             "br-1",
		TenantID:       "tenant-1",
		BankAccountID:  "ba-1",
		StatementDate:  now,
		OpeningBalance: decimal.NewFromFloat(1000.00),
		ClosingBalance: decimal.NewFromFloat(1500.00),
		Status:         ReconciliationInProgress,
		CompletedAt:    &completedAt,
		CreatedAt:      now,
		CreatedBy:      "user-1",
	}

	assert.Equal(t, "br-1", br.ID)
	assert.Equal(t, ReconciliationInProgress, br.Status)
	assert.Equal(t, "user-1", br.CreatedBy)
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

	assert.Equal(t, "bsi-1", bsi.ID)
	assert.Equal(t, "statement_2024_01.csv", bsi.FileName)
	assert.Equal(t, 50, bsi.TransactionsImported)
	assert.Equal(t, 45, bsi.TransactionsMatched)
	assert.Equal(t, 5, bsi.DuplicatesSkipped)
}

func TestMatchSuggestion_Fields(t *testing.T) {
	now := time.Now()

	ms := MatchSuggestion{
		PaymentID:     "pay-1",
		PaymentNumber: "PAY-001",
		PaymentDate:   now,
		Amount:        decimal.NewFromFloat(100.00),
		ContactName:   "Test Customer",
		Reference:     "REF-001",
		Confidence:    0.85,
		MatchReason:   "Exact amount match",
	}

	assert.Equal(t, "pay-1", ms.PaymentID)
	assert.Equal(t, "PAY-001", ms.PaymentNumber)
	assert.Equal(t, 0.85, ms.Confidence)
	assert.Equal(t, "Exact amount match", ms.MatchReason)
}

func TestCreateBankAccountRequest_Fields(t *testing.T) {
	glAccountID := "gl-123"

	req := CreateBankAccountRequest{
		Name:          "New Account",
		AccountNumber: "EE123456789",
		BankName:      "Test Bank",
		SwiftCode:     "TESTEE2X",
		Currency:      "EUR",
		GLAccountID:   &glAccountID,
		IsDefault:     true,
	}

	assert.Equal(t, "New Account", req.Name)
	assert.Equal(t, "EE123456789", req.AccountNumber)
	assert.True(t, req.IsDefault)
}

func TestUpdateBankAccountRequest_Fields(t *testing.T) {
	glAccountID := "gl-456"
	isActive := true
	isDefault := false

	req := UpdateBankAccountRequest{
		Name:        "Updated Account",
		BankName:    "New Bank Name",
		SwiftCode:   "NEWEE2X",
		GLAccountID: &glAccountID,
		IsActive:    &isActive,
		IsDefault:   &isDefault,
	}

	assert.Equal(t, "Updated Account", req.Name)
	assert.Equal(t, "New Bank Name", req.BankName)
	assert.True(t, *req.IsActive)
	assert.False(t, *req.IsDefault)
}

func TestImportCSVRequest_Fields(t *testing.T) {
	rows := []CSVTransactionRow{
		{
			Date:        "2024-01-15",
			Amount:      "100.00",
			Description: "Test transaction",
		},
	}

	req := ImportCSVRequest{
		FileName:       "test.csv",
		Transactions:   rows,
		SkipDuplicates: true,
	}

	assert.Equal(t, "test.csv", req.FileName)
	assert.Len(t, req.Transactions, 1)
	assert.True(t, req.SkipDuplicates)
}

func TestCSVTransactionRow_Fields(t *testing.T) {
	row := CSVTransactionRow{
		Date:                "2024-01-15",
		ValueDate:           "2024-01-16",
		Amount:              "250.50",
		Description:         "Wire transfer",
		Reference:           "REF123",
		CounterpartyName:    "John Doe",
		CounterpartyAccount: "EE123456",
		ExternalID:          "ext-001",
	}

	assert.Equal(t, "2024-01-15", row.Date)
	assert.Equal(t, "2024-01-16", row.ValueDate)
	assert.Equal(t, "250.50", row.Amount)
	assert.Equal(t, "Wire transfer", row.Description)
}

func TestImportResult_Fields(t *testing.T) {
	result := ImportResult{
		ImportID:             "imp-1",
		TransactionsImported: 10,
		TransactionsMatched:  8,
		DuplicatesSkipped:    2,
		Errors:               []string{"Error 1", "Error 2"},
	}

	assert.Equal(t, "imp-1", result.ImportID)
	assert.Equal(t, 10, result.TransactionsImported)
	assert.Len(t, result.Errors, 2)
}

func TestCreateReconciliationRequest_Fields(t *testing.T) {
	req := CreateReconciliationRequest{
		StatementDate:  "2024-01-31",
		OpeningBalance: decimal.NewFromFloat(1000.00),
		ClosingBalance: decimal.NewFromFloat(1500.00),
	}

	assert.Equal(t, "2024-01-31", req.StatementDate)
	assert.True(t, req.OpeningBalance.Equal(decimal.NewFromFloat(1000.00)))
	assert.True(t, req.ClosingBalance.Equal(decimal.NewFromFloat(1500.00)))
}

func TestMatchTransactionRequest_Fields(t *testing.T) {
	req := MatchTransactionRequest{
		PaymentID: "pay-123",
	}

	assert.Equal(t, "pay-123", req.PaymentID)
}

func TestTransactionFilter_Fields(t *testing.T) {
	now := time.Now()
	fromDate := now.AddDate(0, -1, 0)
	toDate := now
	minAmount := decimal.NewFromFloat(10.00)
	maxAmount := decimal.NewFromFloat(1000.00)

	filter := TransactionFilter{
		BankAccountID: "ba-1",
		Status:        StatusUnmatched,
		FromDate:      &fromDate,
		ToDate:        &toDate,
		MinAmount:     &minAmount,
		MaxAmount:     &maxAmount,
	}

	assert.Equal(t, "ba-1", filter.BankAccountID)
	assert.Equal(t, StatusUnmatched, filter.Status)
	assert.NotNil(t, filter.FromDate)
	assert.NotNil(t, filter.ToDate)
	assert.NotNil(t, filter.MinAmount)
	assert.NotNil(t, filter.MaxAmount)
}

func TestBankAccountFilter_Fields(t *testing.T) {
	isActive := true

	filter := BankAccountFilter{
		IsActive: &isActive,
		Currency: "EUR",
	}

	assert.True(t, *filter.IsActive)
	assert.Equal(t, "EUR", filter.Currency)
}
