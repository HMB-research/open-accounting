package banking

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/shopspring/decimal"
)

func TestBankingMatcherWave8AutoMatchSkipsLookupProblems(t *testing.T) {
	for _, tt := range []struct {
		name       string
		candidates func(context.Context, string, string, payments.PaymentType, decimal.Decimal, int) ([]PaymentForMatching, error)
	}{
		{
			name: "candidate lookup error",
			candidates: func(context.Context, string, string, payments.PaymentType, decimal.Decimal, int) ([]PaymentForMatching, error) {
				return nil, errors.New("payments offline")
			},
		},
		{
			name: "no suggestions",
			candidates: func(context.Context, string, string, payments.PaymentType, decimal.Decimal, int) ([]PaymentForMatching, error) {
				return nil, nil
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockRepository()
			service := NewServiceWithRepository(repo)
			repo.transactions["tx-skip"] = &BankTransaction{
				ID:              "tx-skip",
				TenantID:        testTenantID,
				BankAccountID:   testBankID,
				TransactionDate: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
				Amount:          decimal.NewFromInt(100),
				Status:          StatusUnmatched,
			}
			repo.ListPaymentMatchCandidatesFn = tt.candidates

			matched, err := service.AutoMatchTransactions(context.Background(), testSchemaName, testTenantID, testBankID, 0.6)

			if err != nil {
				t.Fatalf("AutoMatchTransactions() error = %v", err)
			}
			if matched != 0 {
				t.Fatalf("matched = %d, want 0", matched)
			}
		})
	}
}

func TestBankingMatcherWave8RuleAndCreatePaymentBranches(t *testing.T) {
	bankAccountID := testBankID
	otherBankID := "bank-other"
	transaction := &BankTransaction{
		BankAccountID:       bankAccountID,
		Description:         "Card settlement",
		Reference:           "RF-1",
		CounterpartyName:    "Acme",
		CounterpartyAccount: "EE123",
		TransactionDate:     time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
		Amount:              decimal.NewFromInt(10),
		Status:              StatusUnmatched,
	}

	rules := []BankMatchRule{
		{Pattern: "card", MatchField: BankMatchFieldDescription, IsActive: false},
		{Pattern: "card", MatchField: BankMatchFieldDescription, IsActive: true, BankAccountID: &otherBankID},
		{Pattern: "ee123", MatchField: BankMatchFieldCounterpartyAccount, IsActive: true, BankAccountID: &bankAccountID},
	}

	rule := firstBankMatchRuleForTransaction(transaction, bankAccountID, rules)
	if rule == nil || rule.Pattern != "ee123" {
		t.Fatalf("firstBankMatchRuleForTransaction() = %#v, want counterparty account rule", rule)
	}
	if bankMatchRuleMatchesTransaction(nil, transaction) {
		t.Fatalf("nil rule should not match")
	}
	if bankMatchRuleMatchesTransaction(&BankMatchRule{Pattern: "  "}, transaction) {
		t.Fatalf("blank pattern should not match")
	}
	if !bankMatchRuleMatchesTransaction(&BankMatchRule{Pattern: "acme", MatchField: BankMatchFieldCounterpartyName}, transaction) {
		t.Fatalf("counterparty name rule should match")
	}

	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	if _, err := service.CreatePaymentFromTransaction(context.Background(), testSchemaName, testTenantID, "user-1", "missing"); err == nil {
		t.Fatalf("expected missing transaction error")
	}

	createErrorTransaction := *transaction
	createErrorTransaction.ID = "tx-create-error"
	createErrorTransaction.TenantID = testTenantID
	repo.transactions["tx-create-error"] = &createErrorTransaction
	repo.CreatePaymentFromTransactionFn = func(context.Context, string, string, string, *BankTransaction) (string, error) {
		return "", errors.New("payment insert failed")
	}
	_, err := service.CreatePaymentFromTransaction(context.Background(), testSchemaName, testTenantID, "user-1", "tx-create-error")
	if err == nil || !strings.Contains(err.Error(), "create payment from transaction") {
		t.Fatalf("CreatePaymentFromTransaction() error = %v, want wrapped create error", err)
	}
}
