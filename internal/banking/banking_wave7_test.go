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

func TestBankingWave7ConstructorAndServiceValidationBranches(t *testing.T) {
	if svc := NewService(nil); svc == nil || svc.repo != nil {
		t.Fatalf("NewService(nil) = %#v, want empty service", svc)
	}

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	inactive := false
	rule, err := service.CreateBankMatchRule(ctx, testSchemaName, testTenantID, &CreateBankMatchRuleRequest{
		Name:            "  Inactive rule  ",
		MatchField:      BankMatchFieldDescription,
		Pattern:         " invoice ",
		IsActive:        &inactive,
		MinConfidence:   0.8,
		MaxDateDiffDays: 3,
	})
	if err != nil {
		t.Fatalf("CreateBankMatchRule() error = %v", err)
	}
	if rule.IsActive || rule.Name != "Inactive rule" || rule.Pattern != "invoice" {
		t.Fatalf("CreateBankMatchRule() = %#v, want trimmed inactive rule", rule)
	}

	_, err = service.UpdateBankMatchRule(ctx, testSchemaName, testTenantID, rule.ID, nil)
	if err == nil || !strings.Contains(err.Error(), "bank match rule request is required") {
		t.Fatalf("UpdateBankMatchRule(nil) error = %v", err)
	}
	blankPattern := " "
	_, err = service.UpdateBankMatchRule(ctx, testSchemaName, testTenantID, rule.ID, &UpdateBankMatchRuleRequest{Pattern: &blankPattern})
	if err == nil || !strings.Contains(err.Error(), "pattern is required") {
		t.Fatalf("UpdateBankMatchRule(blank pattern) error = %v", err)
	}
	longNote := strings.Repeat("x", 2001)
	_, err = service.UpdateTransactionReview(ctx, testSchemaName, testTenantID, "tx-1", "reviewer-1", &UpdateTransactionReviewRequest{ReviewNote: &longNote})
	if err == nil || !strings.Contains(err.Error(), "review note must be 2000 characters or less") {
		t.Fatalf("UpdateTransactionReview(long note) error = %v", err)
	}
	_, err = service.CreateReconciliation(ctx, testSchemaName, testTenantID, "acc-1", "user-1", &CreateReconciliationRequest{StatementDate: "bad"})
	if err == nil || !strings.Contains(err.Error(), "invalid statement date") {
		t.Fatalf("CreateReconciliation(bad date) error = %v", err)
	}
}

func TestBankingWave7GetBankAccountIgnoresBalanceError(t *testing.T) {
	repo := NewMockRepository()
	repo.accounts["acc-1"] = &BankAccount{ID: "acc-1", TenantID: testTenantID, AccountNumber: "EE1", Currency: "EUR"}
	repo.CalculateAccountBalanceFn = func(context.Context, string, string) (decimal.Decimal, error) {
		return decimal.Zero, errors.New("balance failed")
	}
	account, err := NewServiceWithRepository(repo).GetBankAccount(context.Background(), testSchemaName, testTenantID, "acc-1")
	if err != nil {
		t.Fatalf("GetBankAccount() error = %v", err)
	}
	if !account.Balance.IsZero() {
		t.Fatalf("GetBankAccount() balance = %s, want zero when balance calculation fails", account.Balance)
	}
}

func TestBankingWave7MatchSuggestionErrorsAndLimit(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	_, err := service.GetMatchSuggestions(ctx, testSchemaName, testTenantID, "missing")
	if err == nil {
		t.Fatal("GetMatchSuggestions() missing transaction error = nil")
	}

	repo.transactions["tx-1"] = &BankTransaction{
		ID:              "tx-1",
		TenantID:        testTenantID,
		BankAccountID:   testBankID,
		TransactionDate: time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
		Amount:          decimal.NewFromInt(-100),
		Status:          StatusUnmatched,
	}
	repo.ListPaymentMatchCandidatesFn = func(context.Context, string, string, payments.PaymentType, decimal.Decimal, int) ([]PaymentForMatching, error) {
		return nil, errors.New("candidates failed")
	}
	_, err = service.GetMatchSuggestions(ctx, testSchemaName, testTenantID, "tx-1")
	if err == nil || !strings.Contains(err.Error(), "get unallocated payments") {
		t.Fatalf("GetMatchSuggestions() candidates error = %v", err)
	}

	repo.ListPaymentMatchCandidatesFn = func(context.Context, string, string, payments.PaymentType, decimal.Decimal, int) ([]PaymentForMatching, error) {
		candidates := make([]PaymentForMatching, 0, 6)
		for i := 0; i < 6; i++ {
			candidates = append(candidates, PaymentForMatching{
				ID:            "payment-" + string(rune('a'+i)),
				PaymentNumber: "PMT-1",
				PaymentDate:   time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
				Amount:        decimal.NewFromInt(100),
				ContactName:   "Acme",
				Reference:     "INV-1",
			})
		}
		return candidates, nil
	}
	repo.transactions["tx-1"].CounterpartyName = "Acme"
	repo.transactions["tx-1"].Reference = "INV-1"
	repo.transactions["tx-1"].Description = "Payment PMT-1"
	suggestions, err := service.GetMatchSuggestions(ctx, testSchemaName, testTenantID, "tx-1")
	if err != nil {
		t.Fatalf("GetMatchSuggestions() error = %v", err)
	}
	if len(suggestions) != 5 {
		t.Fatalf("GetMatchSuggestions() len = %d, want 5", len(suggestions))
	}
}

func TestBankingWave7AutoMatchErrorBranches(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	repo.ListTransactionsFn = func(context.Context, string, string, *TransactionFilter) ([]BankTransaction, error) {
		return nil, errors.New("list failed")
	}
	_, err := service.AutoMatchTransactions(ctx, testSchemaName, testTenantID, testBankID, 0.6)
	if err == nil || !strings.Contains(err.Error(), "list transactions") {
		t.Fatalf("AutoMatchTransactions() list error = %v", err)
	}

	repo = NewMockRepository()
	service = NewServiceWithRepository(repo)
	repo.transactions["tx-1"] = &BankTransaction{ID: "tx-1", TenantID: testTenantID, BankAccountID: testBankID, Status: StatusUnmatched, Amount: decimal.NewFromInt(-100)}
	repo.ListBankMatchRulesFn = func(context.Context, string, string, *BankMatchRuleFilter) ([]BankMatchRule, error) {
		return nil, errors.New("rules failed")
	}
	_, err = service.AutoMatchTransactions(ctx, testSchemaName, testTenantID, testBankID, 0.6)
	if err == nil || !strings.Contains(err.Error(), "list bank match rules") {
		t.Fatalf("AutoMatchTransactions() rules error = %v", err)
	}

	repo = NewMockRepository()
	service = NewServiceWithRepository(repo)
	repo.transactions["tx-1"] = &BankTransaction{
		ID:               "tx-1",
		TenantID:         testTenantID,
		BankAccountID:    testBankID,
		Status:           StatusUnmatched,
		TransactionDate:  time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
		Amount:           decimal.NewFromInt(-100),
		Reference:        "INV-1",
		Description:      "Payment PMT-1",
		CounterpartyName: "Acme",
	}
	repo.ListPaymentMatchCandidatesFn = func(context.Context, string, string, payments.PaymentType, decimal.Decimal, int) ([]PaymentForMatching, error) {
		return []PaymentForMatching{{
			ID:            "payment-1",
			PaymentNumber: "PMT-1",
			PaymentDate:   time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC),
			Amount:        decimal.NewFromInt(100),
			ContactName:   "Acme",
			Reference:     "INV-1",
		}}, nil
	}
	repo.IncrementLatestImportMatchedFn = func(context.Context, string, string, string, int) error {
		return errors.New("increment failed")
	}
	matched, err := service.AutoMatchTransactions(ctx, testSchemaName, testTenantID, testBankID, 0.6)
	if matched != 1 || err == nil || !strings.Contains(err.Error(), "update import matched count") {
		t.Fatalf("AutoMatchTransactions() matched=%d error=%v, want increment error after one match", matched, err)
	}
}
