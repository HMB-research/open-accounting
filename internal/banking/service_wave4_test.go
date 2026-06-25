package banking

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestServiceWave4UpdateBankMatchRuleAppliesAllOptionalFields(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()

	repo.accounts[testBankID] = &BankAccount{
		ID:       testBankID,
		TenantID: testTenantID,
		Name:     "Operating bank",
	}
	repo.matchRules["rule-1"] = &BankMatchRule{
		ID:              "rule-1",
		TenantID:        testTenantID,
		Name:            "Old rule",
		Priority:        100,
		MatchField:      BankMatchFieldDescription,
		Pattern:         "old",
		MinConfidence:   0.7,
		MaxDateDiffDays: 5,
		IsActive:        true,
	}

	name := "Invoice reference"
	priority := 7
	matchField := BankMatchFieldReference
	pattern := "INV-"
	minConfidence := 0.92
	maxDateDiffDays := 12
	requireExactAmount := true
	isActive := false
	bankAccountID := testBankID

	updated, err := service.UpdateBankMatchRule(ctx, testSchemaName, testTenantID, "rule-1", &UpdateBankMatchRuleRequest{
		BankAccountID:      &bankAccountID,
		Name:               &name,
		Priority:           &priority,
		MatchField:         &matchField,
		Pattern:            &pattern,
		MinConfidence:      &minConfidence,
		MaxDateDiffDays:    &maxDateDiffDays,
		RequireExactAmount: &requireExactAmount,
		IsActive:           &isActive,
	})
	if err != nil {
		t.Fatalf("UpdateBankMatchRule() error = %v", err)
	}

	if updated.BankAccountID == nil || *updated.BankAccountID != testBankID {
		t.Fatalf("BankAccountID = %#v, want %s", updated.BankAccountID, testBankID)
	}
	if updated.Name != name ||
		updated.Priority != priority ||
		updated.MatchField != matchField ||
		updated.Pattern != pattern ||
		updated.MinConfidence != minConfidence ||
		updated.MaxDateDiffDays != maxDateDiffDays ||
		updated.RequireExactAmount != requireExactAmount ||
		updated.IsActive != isActive {
		t.Fatalf("updated rule = %#v, want all optional fields applied", updated)
	}
	if updated.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero, want timestamp")
	}
}

func TestServiceWave4BankMatchRuleValidationBranches(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()
	repo.matchRules["rule-1"] = &BankMatchRule{
		ID:              "rule-1",
		TenantID:        testTenantID,
		Name:            "Reference",
		MatchField:      BankMatchFieldReference,
		Pattern:         "INV",
		MinConfidence:   0.8,
		MaxDateDiffDays: 5,
		IsActive:        true,
	}

	empty := "  "
	_, err := service.UpdateBankMatchRule(ctx, testSchemaName, testTenantID, "rule-1", &UpdateBankMatchRuleRequest{Name: &empty})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("UpdateBankMatchRule(empty name) error = %v, want name is required", err)
	}

	badDays := 91
	_, err = service.UpdateBankMatchRule(ctx, testSchemaName, testTenantID, "rule-1", &UpdateBankMatchRuleRequest{MaxDateDiffDays: &badDays})
	if err == nil || !strings.Contains(err.Error(), "max date diff days") {
		t.Fatalf("UpdateBankMatchRule(bad days) error = %v, want max date diff days", err)
	}
}

func TestServiceWave4ListTransactionsPropagatesRepositoryError(t *testing.T) {
	expectedErr := errors.New("list transactions failed")
	service := NewServiceWithRepository(&MockRepository{
		ListTransactionsFn: func(context.Context, string, string, *TransactionFilter) ([]BankTransaction, error) {
			return nil, expectedErr
		},
	})

	transactions, err := service.ListTransactions(context.Background(), testSchemaName, testTenantID, nil)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("ListTransactions() error = %v, want %v", err, expectedErr)
	}
	if transactions != nil {
		t.Fatalf("ListTransactions() transactions = %#v, want nil", transactions)
	}
}

func TestHydrateTransactionDerivedFieldsWave4NilAndDefaults(t *testing.T) {
	hydrateTransactionDerivedFields(nil)

	transaction := &BankTransaction{
		ID:            "tx-1",
		TenantID:      testTenantID,
		BankAccountID: testBankID,
		Amount:        decimal.NewFromInt(-25),
		Status:        StatusUnmatched,
	}
	hydrateTransactionDerivedFields(transaction)

	if transaction.FollowUpStatus != FollowUpNone {
		t.Fatalf("FollowUpStatus = %q, want %q", transaction.FollowUpStatus, FollowUpNone)
	}
	if transaction.RemediationActions == nil {
		t.Fatal("RemediationActions is nil, want derived actions slice")
	}
}
