package banking

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/shopspring/decimal"
)

func TestParseDateFormatsUnit(t *testing.T) {
	tests := []struct {
		name  string
		value string
		year  int
		month time.Month
		day   int
	}{
		{name: "iso date", value: "2026-03-15", year: 2026, month: time.March, day: 15},
		{name: "estonian dotted date", value: " 15.03.2026 ", year: 2026, month: time.March, day: 15},
		{name: "us slash date", value: "03/15/2026", year: 2026, month: time.March, day: 15},
		{name: "european slash date", value: "15/03/2026", year: 2026, month: time.March, day: 15},
		{name: "rfc3339 date", value: "2026-03-15T10:30:00Z", year: 2026, month: time.March, day: 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseDateFormats(tt.value)
			if err != nil {
				t.Fatalf("ParseDateFormats() error = %v", err)
			}
			if parsed.Year() != tt.year || parsed.Month() != tt.month || parsed.Day() != tt.day {
				t.Fatalf("parsed date = %s, want %04d-%02d-%02d", parsed.Format(time.RFC3339), tt.year, tt.month, tt.day)
			}
		})
	}

	if _, err := ParseDateFormats("not-a-date"); err == nil || !strings.Contains(err.Error(), "unable to parse date") {
		t.Fatalf("expected parse error for invalid date, got %v", err)
	}
}

func TestPaymentTypeForTransactionAmountUnit(t *testing.T) {
	tests := []struct {
		name string
		amt  decimal.Decimal
		want payments.PaymentType
	}{
		{name: "incoming", amt: decimal.NewFromInt(100), want: payments.PaymentTypeReceived},
		{name: "zero", amt: decimal.Zero, want: payments.PaymentTypeReceived},
		{name: "outgoing", amt: decimal.NewFromInt(-100), want: payments.PaymentTypeMade},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paymentTypeForTransactionAmount(tt.amt); got != tt.want {
				t.Fatalf("paymentTypeForTransactionAmount() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestService_GetMatchSuggestionsRanksAndLimitsUnit(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()
	txDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	repo.transactions["tx-suggest"] = &BankTransaction{
		ID:               "tx-suggest",
		TenantID:         testTenantID,
		BankAccountID:    testBankID,
		TransactionDate:  txDate,
		Amount:           decimal.NewFromInt(100),
		Description:      "Incoming settlement PAY-001",
		Reference:        "RF 123-456",
		CounterpartyName: "Acme OÜ",
		Status:           StatusUnmatched,
	}

	var gotType payments.PaymentType
	var gotAmount decimal.Decimal
	var gotLimit int
	repo.ListPaymentMatchCandidatesFn = func(_ context.Context, _, _ string, paymentType payments.PaymentType, amount decimal.Decimal, limit int) ([]PaymentForMatching, error) {
		gotType = paymentType
		gotAmount = amount
		gotLimit = limit
		return []PaymentForMatching{
			{
				ID:            "payment-best",
				PaymentNumber: "PAY-001",
				PaymentDate:   txDate,
				Amount:        decimal.NewFromInt(100),
				ContactName:   "Acme OU",
				Reference:     "RF123456",
			},
			{ID: "payment-2", PaymentNumber: "PAY-002", PaymentDate: txDate, Amount: decimal.NewFromInt(100)},
			{ID: "payment-3", PaymentNumber: "PAY-003", PaymentDate: txDate, Amount: decimal.NewFromInt(100)},
			{ID: "payment-4", PaymentNumber: "PAY-004", PaymentDate: txDate, Amount: decimal.NewFromInt(100)},
			{ID: "payment-5", PaymentNumber: "PAY-005", PaymentDate: txDate, Amount: decimal.NewFromInt(100)},
			{ID: "payment-6", PaymentNumber: "PAY-006", PaymentDate: txDate, Amount: decimal.NewFromInt(100)},
		}, nil
	}

	suggestions, err := service.GetMatchSuggestions(ctx, testSchemaName, testTenantID, "tx-suggest")
	if err != nil {
		t.Fatalf("GetMatchSuggestions() error = %v", err)
	}
	if gotType != payments.PaymentTypeReceived {
		t.Fatalf("payment type = %s, want %s", gotType, payments.PaymentTypeReceived)
	}
	if !gotAmount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("candidate amount = %s, want 100", gotAmount)
	}
	if gotLimit != 20 {
		t.Fatalf("candidate limit = %d, want 20", gotLimit)
	}
	if len(suggestions) != 5 {
		t.Fatalf("expected top 5 suggestions, got %d", len(suggestions))
	}
	if suggestions[0].PaymentID != "payment-best" {
		t.Fatalf("expected best suggestion first, got %#v", suggestions[0])
	}
	if suggestions[0].Confidence != 1.0 {
		t.Fatalf("expected best suggestion to be capped at 1.0 confidence, got %.2f", suggestions[0].Confidence)
	}
	if !strings.Contains(suggestions[0].MatchReason, "reference match") || !strings.Contains(suggestions[0].MatchReason, "name match") {
		t.Fatalf("expected reference and name match reasons, got %q", suggestions[0].MatchReason)
	}
	for i := 1; i < len(suggestions); i++ {
		if suggestions[i-1].Confidence < suggestions[i].Confidence {
			t.Fatalf("suggestions are not sorted by confidence: %#v", suggestions)
		}
	}
}

func TestService_AutoMatchTransactionsSortsRuleSuggestionsUnit(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()
	txDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	bankAccountID := testBankID

	repo.transactions["tx-auto"] = &BankTransaction{
		ID:               "tx-auto",
		TenantID:         testTenantID,
		BankAccountID:    bankAccountID,
		TransactionDate:  txDate,
		Amount:           decimal.NewFromInt(100),
		Description:      "Incoming PAY-LOW",
		Reference:        "INV-100",
		CounterpartyName: "Acme OÜ",
		Status:           StatusUnmatched,
	}
	repo.ListBankMatchRulesFn = func(_ context.Context, _, _ string, filter *BankMatchRuleFilter) ([]BankMatchRule, error) {
		if filter == nil || !filter.ActiveOnly || !filter.IncludeGlobal || filter.BankAccountID != bankAccountID {
			t.Fatalf("unexpected match rule filter: %#v", filter)
		}
		return []BankMatchRule{
			{
				ID:                 "rule-1",
				TenantID:           testTenantID,
				BankAccountID:      &bankAccountID,
				MatchField:         BankMatchFieldReference,
				Pattern:            "inv-",
				MinConfidence:      0.75,
				MaxDateDiffDays:    3,
				RequireExactAmount: true,
				IsActive:           true,
			},
		}, nil
	}

	var incremented int
	repo.IncrementLatestImportMatchedFn = func(_ context.Context, _, _, accountID string, matchedCount int) error {
		if accountID != bankAccountID {
			t.Fatalf("increment account = %s, want %s", accountID, bankAccountID)
		}
		incremented = matchedCount
		return nil
	}
	repo.ListPaymentMatchCandidatesFn = func(_ context.Context, _, _ string, paymentType payments.PaymentType, amount decimal.Decimal, limit int) ([]PaymentForMatching, error) {
		if paymentType != payments.PaymentTypeReceived {
			t.Fatalf("payment type = %s, want received", paymentType)
		}
		if !amount.Equal(decimal.NewFromInt(100)) || limit != 20 {
			t.Fatalf("unexpected candidate query amount=%s limit=%d", amount, limit)
		}
		return []PaymentForMatching{
			{
				ID:            "payment-low",
				PaymentNumber: "PAY-LOW",
				PaymentDate:   txDate,
				Amount:        decimal.NewFromInt(100),
			},
			{
				ID:            "payment-best",
				PaymentNumber: "PAY-BEST",
				PaymentDate:   txDate,
				Amount:        decimal.NewFromInt(100),
				ContactName:   "Acme OU",
				Reference:     "INV-100",
			},
			{
				ID:            "payment-filtered",
				PaymentNumber: "PAY-FILTERED",
				PaymentDate:   txDate,
				Amount:        decimal.NewFromInt(99),
				ContactName:   "Acme OU",
				Reference:     "INV-100",
			},
		}, nil
	}

	matched, err := service.AutoMatchTransactions(ctx, testSchemaName, testTenantID, bankAccountID, 0.6)
	if err != nil {
		t.Fatalf("AutoMatchTransactions() error = %v", err)
	}
	if matched != 1 {
		t.Fatalf("matched count = %d, want 1", matched)
	}
	if incremented != 1 {
		t.Fatalf("incremented matched count = %d, want 1", incremented)
	}
	if repo.transactions["tx-auto"].MatchedPaymentID == nil || *repo.transactions["tx-auto"].MatchedPaymentID != "payment-best" {
		t.Fatalf("expected highest-confidence payment to be matched, got %#v", repo.transactions["tx-auto"].MatchedPaymentID)
	}
}

func TestService_AutoMatchTransactionsSkipsAmbiguousSuggestionsUnit(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()
	txDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	repo.transactions["tx-ambiguous"] = &BankTransaction{
		ID:               "tx-ambiguous",
		TenantID:         testTenantID,
		BankAccountID:    testBankID,
		TransactionDate:  txDate,
		Amount:           decimal.NewFromInt(100),
		Reference:        "INV-200",
		CounterpartyName: "Beta AS",
		Status:           StatusUnmatched,
	}
	repo.ListPaymentMatchCandidatesFn = func(context.Context, string, string, payments.PaymentType, decimal.Decimal, int) ([]PaymentForMatching, error) {
		return []PaymentForMatching{
			{
				ID:          "payment-a",
				PaymentDate: txDate,
				Amount:      decimal.NewFromInt(100),
				ContactName: "Beta",
				Reference:   "INV-200",
			},
			{
				ID:          "payment-b",
				PaymentDate: txDate,
				Amount:      decimal.NewFromInt(100),
				ContactName: "Beta",
				Reference:   "INV-200",
			},
		}, nil
	}

	incrementCalled := false
	repo.IncrementLatestImportMatchedFn = func(context.Context, string, string, string, int) error {
		incrementCalled = true
		return nil
	}

	matched, err := service.AutoMatchTransactions(ctx, testSchemaName, testTenantID, testBankID, 0.6)
	if err != nil {
		t.Fatalf("AutoMatchTransactions() error = %v", err)
	}
	if matched != 0 {
		t.Fatalf("matched count = %d, want 0 for ambiguous suggestions", matched)
	}
	if repo.transactions["tx-ambiguous"].MatchedPaymentID != nil {
		t.Fatalf("ambiguous transaction should remain unmatched")
	}
	if incrementCalled {
		t.Fatalf("import matched count should not be incremented when nothing matched")
	}
}

func TestMatcherSimilarityHelpersUnit(t *testing.T) {
	if similarity := calculateStringSimilarity("invoice", "invoce"); similarity <= 0.7 || similarity >= 1.0 {
		t.Fatalf("expected fuzzy similarity between 0.7 and 1.0, got %.2f", similarity)
	}
	if similarity := calculateStringSimilarity("", "invoice"); similarity != 0 {
		t.Fatalf("expected empty string similarity 0, got %.2f", similarity)
	}
	if distance := levenshteinDistance("kitten", "sitting"); distance != 3 {
		t.Fatalf("levenshteinDistance() = %d, want 3", distance)
	}
	if distance := levenshteinDistance("", "abc"); distance != 3 {
		t.Fatalf("levenshteinDistance() empty lhs = %d, want 3", distance)
	}
	if distance := levenshteinDistance("abc", ""); distance != 3 {
		t.Fatalf("levenshteinDistance() empty rhs = %d, want 3", distance)
	}
	if got := min(3, 2, 1); got != 1 {
		t.Fatalf("min(3, 2, 1) = %d, want 1", got)
	}
	if got := min(1, 3, 2); got != 1 {
		t.Fatalf("min(1, 3, 2) = %d, want 1", got)
	}
	if got := min(2, 1, 3); got != 1 {
		t.Fatalf("min(2, 1, 3) = %d, want 1", got)
	}
	if got := max(7, 5); got != 7 {
		t.Fatalf("max(7, 5) = %d, want 7", got)
	}
	if got := max(5, 7); got != 7 {
		t.Fatalf("max(5, 7) = %d, want 7", got)
	}
}

func TestService_CreatePaymentFromTransactionGuardsStatusUnit(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	ctx := context.Background()
	txDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

	repo.transactions["tx-already"] = &BankTransaction{
		ID:              "tx-already",
		TenantID:        testTenantID,
		BankAccountID:   testBankID,
		TransactionDate: txDate,
		Amount:          decimal.NewFromInt(100),
		Status:          StatusMatched,
	}
	if _, err := service.CreatePaymentFromTransaction(ctx, testSchemaName, testTenantID, "user-1", "tx-already"); err == nil || !strings.Contains(err.Error(), "already matched") {
		t.Fatalf("expected already matched error, got %v", err)
	}

	repo.transactions["tx-create-payment"] = &BankTransaction{
		ID:              "tx-create-payment",
		TenantID:        testTenantID,
		BankAccountID:   testBankID,
		TransactionDate: txDate,
		Amount:          decimal.NewFromInt(-42),
		Status:          StatusUnmatched,
	}
	paymentID, err := service.CreatePaymentFromTransaction(ctx, testSchemaName, testTenantID, "user-1", "tx-create-payment")
	if err != nil {
		t.Fatalf("CreatePaymentFromTransaction() error = %v", err)
	}
	if paymentID != "payment-tx-create-payment" {
		t.Fatalf("payment ID = %s, want payment-tx-create-payment", paymentID)
	}
	if repo.transactions["tx-create-payment"].Status != StatusMatched {
		t.Fatalf("transaction status = %s, want matched", repo.transactions["tx-create-payment"].Status)
	}
}
