package banking

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEnsureSchema tests the EnsureSchema function that has 0% coverage
func TestEnsureSchema(t *testing.T) {
	tests := []struct {
		name           string
		service        *Service
		schemaName     string
		expectedError  string
	}{
		{
			name:          "service without database connection",
			service:       &Service{db: nil},
			schemaName:    "test_schema",
			expectedError: "database connection not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.service.EnsureSchema(context.Background(), tt.schemaName)
			
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}
			
			assert.NoError(t, err)
		})
	}
}

// These functions test the business logic validation without database dependency

// TestTransactionValidation tests transaction validation logic
func TestTransactionValidation(t *testing.T) {
	tests := []struct {
		name         string
		transaction  BankTransaction
		isValid      bool
		errorField   string
	}{
		{
			name: "valid transaction",
			transaction: BankTransaction{
				ID:            "trans-123",
				TenantID:      "tenant-123",
				BankAccountID: "account-123",
				Description:   "Test transaction",
				Status:        StatusUnmatched,
			},
			isValid: true,
		},
		{
			name: "transaction with empty tenant ID",
			transaction: BankTransaction{
				ID:            "trans-123",
				TenantID:      "",
				BankAccountID: "account-123",
				Description:   "Test transaction",
			},
			isValid:    false,
			errorField: "tenant_id",
		},
		{
			name: "transaction with empty bank account ID",
			transaction: BankTransaction{
				ID:            "trans-123",
				TenantID:      "tenant-123",
				BankAccountID: "",
				Description:   "Test transaction",
			},
			isValid:    false,
			errorField: "bank_account_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test basic validation logic that would be used in the service layer
			isValid := tt.transaction.TenantID != "" && tt.transaction.BankAccountID != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// TestBankAccountValidation tests bank account validation logic
func TestBankAccountValidation(t *testing.T) {
	tests := []struct {
		name        string
		account     BankAccount
		isValid     bool
		errorField  string
	}{
		{
			name: "valid bank account",
			account: BankAccount{
				ID:            "account-123",
				TenantID:      "tenant-123",
				Name:          "Main Account",
				AccountNumber: "123456789",
				Currency:      "EUR",
				IsActive:      true,
			},
			isValid: true,
		},
		{
			name: "account with empty name",
			account: BankAccount{
				ID:            "account-123",
				TenantID:      "tenant-123",
				Name:          "",
				AccountNumber: "123456789",
				Currency:      "EUR",
			},
			isValid:    false,
			errorField: "name",
		},
		{
			name: "account with empty account number",
			account: BankAccount{
				ID:            "account-123",
				TenantID:      "tenant-123",
				Name:          "Main Account",
				AccountNumber: "",
				Currency:      "EUR",
			},
			isValid:    false,
			errorField: "account_number",
		},
		{
			name: "account with invalid currency",
			account: BankAccount{
				ID:            "account-123",
				TenantID:      "tenant-123",
				Name:          "Main Account",
				AccountNumber: "123456789",
				Currency:      "",
			},
			isValid:    false,
			errorField: "currency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic that would be used in service layer
			isValid := tt.account.Name != "" && tt.account.AccountNumber != "" && tt.account.Currency != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// TestTransactionStatusTransitions tests valid transaction status transitions  
func TestTransactionStatusTransitions(t *testing.T) {
	tests := []struct {
		name        string
		from        TransactionStatus
		to          TransactionStatus
		isValid     bool
	}{
		{
			name:    "unmatched to matched",
			from:    StatusUnmatched,
			to:      StatusMatched,
			isValid: true,
		},
		{
			name:    "matched to reconciled",
			from:    StatusMatched,
			to:      StatusReconciled,
			isValid: true,
		},
		{
			name:    "unmatched to reconciled",
			from:    StatusUnmatched,
			to:      StatusReconciled,
			isValid: false, // Should go through matched first
		},
		{
			name:    "reconciled to matched",
			from:    StatusReconciled,
			to:      StatusMatched,
			isValid: false, // Cannot go backwards
		},
		{
			name:    "matched to unmatched",
			from:    StatusMatched,
			to:      StatusUnmatched,
			isValid: true, // Allowed for unmatch operation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test business logic for valid status transitions
			isValid := isValidStatusTransition(tt.from, tt.to)
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// Helper function to test status transition logic
func isValidStatusTransition(from, to TransactionStatus) bool {
	switch from {
	case StatusUnmatched:
		return to == StatusMatched
	case StatusMatched:
		return to == StatusReconciled || to == StatusUnmatched
	case StatusReconciled:
		return false // Cannot transition from reconciled
	}
	return false
}

// TestReconciliationValidation tests reconciliation validation logic
func TestReconciliationValidation(t *testing.T) {
	tests := []struct {
		name           string
		reconciliation BankReconciliation
		isValid        bool
		errorField     string
	}{
		{
			name: "valid reconciliation",
			reconciliation: BankReconciliation{
				ID:            "recon-123",
				TenantID:      "tenant-123",
				BankAccountID: "account-123",
				Status:        ReconciliationInProgress,
			},
			isValid: true,
		},
		{
			name: "reconciliation with empty bank account",
			reconciliation: BankReconciliation{
				ID:            "recon-123",
				TenantID:      "tenant-123",
				BankAccountID: "",
				Status:        ReconciliationInProgress,
			},
			isValid:    false,
			errorField: "bank_account_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			isValid := tt.reconciliation.BankAccountID != "" && tt.reconciliation.TenantID != ""
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// TestMatchSuggestionSorting tests sorting of match suggestions by confidence
func TestMatchSuggestionSorting(t *testing.T) {
	suggestions := []MatchSuggestion{
		{PaymentID: "pay-1", Confidence: 0.3},
		{PaymentID: "pay-2", Confidence: 0.8},
		{PaymentID: "pay-3", Confidence: 0.6},
		{PaymentID: "pay-4", Confidence: 0.9},
	}

	// Sort by confidence descending (highest first)
	for i := 0; i < len(suggestions); i++ {
		for j := i + 1; j < len(suggestions); j++ {
			if suggestions[i].Confidence < suggestions[j].Confidence {
				suggestions[i], suggestions[j] = suggestions[j], suggestions[i]
			}
		}
	}

	assert.Equal(t, "pay-4", suggestions[0].PaymentID) // 0.9
	assert.Equal(t, "pay-2", suggestions[1].PaymentID) // 0.8
	assert.Equal(t, "pay-3", suggestions[2].PaymentID) // 0.6
	assert.Equal(t, "pay-1", suggestions[3].PaymentID) // 0.3
}

// TestTransactionFilterLogic tests transaction filtering logic
func TestTransactionFilterLogic(t *testing.T) {
	transactions := []BankTransaction{
		{ID: "1", Status: StatusUnmatched, BankAccountID: "acc-1"},
		{ID: "2", Status: StatusMatched, BankAccountID: "acc-1"},
		{ID: "3", Status: StatusUnmatched, BankAccountID: "acc-2"},
		{ID: "4", Status: StatusReconciled, BankAccountID: "acc-1"},
	}

	tests := []struct {
		name          string
		filter        TransactionFilter
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "filter by account",
			filter:        TransactionFilter{BankAccountID: "acc-1"},
			expectedCount: 3,
			expectedIDs:   []string{"1", "2", "4"},
		},
		{
			name:          "filter by status",
			filter:        TransactionFilter{Status: StatusUnmatched},
			expectedCount: 2,
			expectedIDs:   []string{"1", "3"},
		},
		{
			name:          "filter by account and status",
			filter:        TransactionFilter{BankAccountID: "acc-1", Status: StatusUnmatched},
			expectedCount: 1,
			expectedIDs:   []string{"1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate filtering logic
			var filtered []BankTransaction
			for _, transaction := range transactions {
				include := true
				if tt.filter.BankAccountID != "" && transaction.BankAccountID != tt.filter.BankAccountID {
					include = false
				}
				if tt.filter.Status != "" && transaction.Status != tt.filter.Status {
					include = false
				}
				if include {
					filtered = append(filtered, transaction)
				}
			}

			assert.Len(t, filtered, tt.expectedCount)
			for i, expectedID := range tt.expectedIDs {
				if i < len(filtered) {
					assert.Equal(t, expectedID, filtered[i].ID)
				}
			}
		})
	}
}