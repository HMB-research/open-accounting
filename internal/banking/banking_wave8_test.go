package banking

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestBankingWave8ImportAccountCodeResolutionErrors(t *testing.T) {
	service := NewServiceWithRepository(NewMockRepository())
	service.accounts = fakeBankingAccountLister{err: errors.New("ledger unavailable")}

	_, err := service.ImportBankAccounts(context.Background(), testSchemaName, testTenantID, &ImportBankAccountsRequest{
		Rows: []CSVBankAccountRow{{
			Name:          "Operating",
			AccountNumber: "EE123",
			GLAccountCode: "1000",
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "list accounts for bank account import") {
		t.Fatalf("ImportBankAccounts() error = %v, want account code lookup failure", err)
	}
}

func TestBankingWave8MatchRuleValidationAndRepositoryErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("create repository error after defaults", func(t *testing.T) {
		repo := NewMockRepository()
		repo.CreateBankMatchRuleFn = func(context.Context, string, *BankMatchRule) error {
			return errors.New("create failed")
		}
		service := NewServiceWithRepository(repo)

		_, err := service.CreateBankMatchRule(ctx, testSchemaName, testTenantID, &CreateBankMatchRuleRequest{
			Name:       "Defaulted rule",
			MatchField: BankMatchFieldReference,
			Pattern:    "INV-",
		})
		if err == nil || !strings.Contains(err.Error(), "create failed") {
			t.Fatalf("CreateBankMatchRule() error = %v, want repository error", err)
		}
	})

	t.Run("invalid updated match field", func(t *testing.T) {
		repo := NewMockRepository()
		ruleID := "rule-1"
		repo.matchRules[ruleID] = &BankMatchRule{
			ID:              ruleID,
			TenantID:        testTenantID,
			Name:            "Rule",
			MatchField:      BankMatchFieldDescription,
			Pattern:         "invoice",
			MinConfidence:   0.7,
			MaxDateDiffDays: 3,
			IsActive:        true,
		}
		service := NewServiceWithRepository(repo)
		badField := BankMatchField("bad")

		_, err := service.UpdateBankMatchRule(ctx, testSchemaName, testTenantID, ruleID, &UpdateBankMatchRuleRequest{MatchField: &badField})
		if err == nil || !strings.Contains(err.Error(), "invalid bank match field") {
			t.Fatalf("UpdateBankMatchRule() error = %v, want invalid match field", err)
		}
	})

	t.Run("bank account lookup error", func(t *testing.T) {
		repo := NewMockRepository()
		repo.GetBankAccountFn = func(context.Context, string, string, string) (*BankAccount, error) {
			return nil, errors.New("account lookup failed")
		}
		service := NewServiceWithRepository(repo)
		bankAccountID := "11111111-1111-4111-8111-111111111111"

		_, err := service.normalizeBankMatchRuleAccount(ctx, testSchemaName, testTenantID, &bankAccountID)
		if err == nil || !strings.Contains(err.Error(), "account lookup failed") {
			t.Fatalf("normalizeBankMatchRuleAccount() error = %v, want lookup failure", err)
		}
	})
}

func TestBankingWave8ReviewAndUUIDHelperErrors(t *testing.T) {
	blank := "   "
	id, err := normalizeOptionalBankAccountUUIDPtr(&blank, "gl_account_id")
	if err != nil || id != nil {
		t.Fatalf("normalizeOptionalBankAccountUUIDPtr(blank) = %#v, %v", id, err)
	}

	repo := NewMockRepository()
	repo.UpdateTransactionReviewFn = func(context.Context, string, string, string, TransactionReviewUpdate) (*BankTransaction, error) {
		return nil, errors.New("review update failed")
	}
	note := "needs receipt"
	_, err = NewServiceWithRepository(repo).UpdateTransactionReview(context.Background(), testSchemaName, testTenantID, "tx-1", "reviewer-1", &UpdateTransactionReviewRequest{
		ReviewNote: &note,
	})
	if err == nil || !strings.Contains(err.Error(), "review update failed") {
		t.Fatalf("UpdateTransactionReview() error = %v, want repository error", err)
	}
}
