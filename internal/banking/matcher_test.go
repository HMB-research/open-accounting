package banking

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestDefaultMatcherConfig(t *testing.T) {
	config := DefaultMatcherConfig()

	if config.ExactAmountBonus != 0.5 {
		t.Errorf("ExactAmountBonus = %f, want 0.5", config.ExactAmountBonus)
	}
	if config.DateProximityWeight != 0.2 {
		t.Errorf("DateProximityWeight = %f, want 0.2", config.DateProximityWeight)
	}
	if config.ReferenceMatchWeight != 0.2 {
		t.Errorf("ReferenceMatchWeight = %f, want 0.2", config.ReferenceMatchWeight)
	}
	if config.NameMatchWeight != 0.1 {
		t.Errorf("NameMatchWeight = %f, want 0.1", config.NameMatchWeight)
	}
	if config.MinConfidence != 0.3 {
		t.Errorf("MinConfidence = %f, want 0.3", config.MinConfidence)
	}
	if config.MaxDateDiff != 7 {
		t.Errorf("MaxDateDiff = %d, want 7", config.MaxDateDiff)
	}
}

func TestNormalizeReference(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple reference", "INV-123", "inv123"},
		{"With spaces", "  INV 123  ", "inv123"},
		{"With special chars", "INV-123/456", "inv123456"},
		{"Mixed case", "InVoIcE#456", "invoice456"},
		{"Numbers only", "123456", "123456"},
		{"Empty", "", ""},
		{"Only special chars", "---", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeReference(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeReference(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple name", "John Doe", "john doe"},
		{"With OÜ suffix", "Acme OÜ", "acme"},
		{"With AS suffix", "Company AS", "company"},
		{"With LLC suffix", "BigCorp LLC", "bigcorp"},
		{"With Ltd suffix", "SmallCo Ltd", "smallco"},
		{"With GmbH suffix", "German GmbH", "german"},
		{"Extra spaces", "  Company   Name  ", "company name"},
		{"Mixed case", "MixedCase Company", "mixedcase company"},
		{"Empty", "", ""},
		{"Lowercase suffix", "company oü", "company"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCalculateStringSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		s1        string
		s2        string
		minExpect float64
		maxExpect float64
	}{
		{"Identical strings", "hello", "hello", 1.0, 1.0},
		{"Completely different", "abc", "xyz", 0.0, 0.4},
		{"Empty strings", "", "", 1.0, 1.0},
		{"One empty", "hello", "", 0.0, 0.0},
		{"Similar strings", "hello", "hallo", 0.7, 1.0},
		{"Substring", "test", "testing", 0.5, 1.0},
		{"Case matters", "ABC", "abc", 0.0, 0.4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateStringSimilarity(tt.s1, tt.s2)
			if result < tt.minExpect || result > tt.maxExpect {
				t.Errorf("calculateStringSimilarity(%q, %q) = %f, want between %f and %f",
					tt.s1, tt.s2, result, tt.minExpect, tt.maxExpect)
			}
		})
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{"Identical", "hello", "hello", 0},
		{"One insertion", "hello", "helo", 1},
		{"One deletion", "hello", "helloo", 1},
		{"One substitution", "hello", "hallo", 1},
		{"Empty first", "", "abc", 3},
		{"Empty second", "abc", "", 3},
		{"Both empty", "", "", 0},
		{"Completely different", "abc", "xyz", 3},
		{"Multiple edits", "kitten", "sitting", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := levenshteinDistance(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, result, tt.expected)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, c  int
		expected int
	}{
		{1, 2, 3, 1},
		{3, 2, 1, 1},
		{2, 1, 3, 1},
		{5, 5, 5, 5},
		{-1, 0, 1, -1},
		{0, -1, 1, -1},
	}

	for _, tt := range tests {
		result := min(tt.a, tt.b, tt.c)
		if result != tt.expected {
			t.Errorf("min(%d, %d, %d) = %d, want %d", tt.a, tt.b, tt.c, result, tt.expected)
		}
	}
}

func TestMax(t *testing.T) {
	tests := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 2},
		{2, 1, 2},
		{5, 5, 5},
		{-1, 0, 0},
		{-5, -3, -3},
	}

	for _, tt := range tests {
		result := max(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestMatchPayments(t *testing.T) {
	config := DefaultMatcherConfig()
	now := time.Now()

	transaction := &BankTransaction{
		ID:               "trans-1",
		TransactionDate:  now,
		Amount:           decimal.NewFromFloat(100.00),
		Currency:         "EUR",
		Description:      "Payment for INV-001",
		Reference:        "INV-001",
		CounterpartyName: "Acme OÜ",
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-1",
			PaymentNumber: "PMT-001",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
			ContactName:   "Acme OÜ",
			Reference:     "INV-001",
		},
		{
			ID:            "pay-2",
			PaymentNumber: "PMT-002",
			PaymentDate:   now.AddDate(0, 0, -10),
			Amount:        decimal.NewFromFloat(50.00),
			ContactName:   "Other Company",
			Reference:     "OTHER-REF",
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	if len(suggestions) == 0 {
		t.Fatal("Expected at least one suggestion")
	}

	// First suggestion should be the exact match
	best := suggestions[0]
	if best.PaymentID != "pay-1" {
		t.Errorf("Best match PaymentID = %q, want %q", best.PaymentID, "pay-1")
	}

	if best.Confidence < 0.7 {
		t.Errorf("Best match confidence = %f, want >= 0.7", best.Confidence)
	}

	if best.MatchReason == "" {
		t.Error("Best match should have a reason")
	}
}

func TestMatchPayments_ExactAmountMatch(t *testing.T) {
	config := DefaultMatcherConfig()
	now := time.Now()

	transaction := &BankTransaction{
		ID:              "trans-1",
		TransactionDate: now,
		Amount:          decimal.NewFromFloat(250.50),
		Currency:        "EUR",
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-exact",
			PaymentNumber: "PMT-001",
			PaymentDate:   now.AddDate(0, 0, -3),
			Amount:        decimal.NewFromFloat(250.50),
		},
		{
			ID:            "pay-close",
			PaymentNumber: "PMT-002",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(252.00),
		},
		{
			ID:            "pay-far",
			PaymentNumber: "PMT-003",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	// Should find exact match first
	found := false
	for _, s := range suggestions {
		if s.PaymentID == "pay-exact" {
			found = true
			if s.Confidence < config.ExactAmountBonus {
				t.Errorf("Exact amount match confidence = %f, want >= %f", s.Confidence, config.ExactAmountBonus)
			}
			break
		}
	}
	if !found {
		t.Error("Expected to find pay-exact in suggestions")
	}
}

func TestMatchPayments_DateProximity(t *testing.T) {
	config := DefaultMatcherConfig()
	now := time.Now()

	transaction := &BankTransaction{
		ID:              "trans-1",
		TransactionDate: now,
		Amount:          decimal.NewFromFloat(100.00),
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-today",
			PaymentNumber: "PMT-001",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
		},
		{
			ID:            "pay-week",
			PaymentNumber: "PMT-002",
			PaymentDate:   now.AddDate(0, 0, -7),
			Amount:        decimal.NewFromFloat(100.00),
		},
		{
			ID:            "pay-old",
			PaymentNumber: "PMT-003",
			PaymentDate:   now.AddDate(0, 0, -30),
			Amount:        decimal.NewFromFloat(100.00),
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	// Should prefer same-day payment
	if len(suggestions) < 2 {
		t.Fatal("Expected at least 2 suggestions")
	}

	var todayConf, weekConf float64
	for _, s := range suggestions {
		if s.PaymentID == "pay-today" {
			todayConf = s.Confidence
		}
		if s.PaymentID == "pay-week" {
			weekConf = s.Confidence
		}
	}

	if todayConf <= weekConf {
		t.Errorf("Same-day confidence (%f) should be higher than week-old (%f)", todayConf, weekConf)
	}
}

func TestMatchPayments_ReferenceMatch(t *testing.T) {
	config := DefaultMatcherConfig()
	now := time.Now()

	transaction := &BankTransaction{
		ID:              "trans-1",
		TransactionDate: now,
		Amount:          decimal.NewFromFloat(100.00),
		Reference:       "INV-12345",
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-match",
			PaymentNumber: "PMT-001",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
			Reference:     "INV-12345",
		},
		{
			ID:            "pay-partial",
			PaymentNumber: "PMT-002",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
			Reference:     "INV-123",
		},
		{
			ID:            "pay-no-ref",
			PaymentNumber: "PMT-003",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
			Reference:     "",
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	// Should have reference match with highest confidence
	if len(suggestions) == 0 {
		t.Fatal("Expected suggestions")
	}

	best := suggestions[0]
	if best.PaymentID != "pay-match" {
		t.Errorf("Best match should be pay-match (exact ref), got %s", best.PaymentID)
	}
}

func TestMatchPayments_NameMatch(t *testing.T) {
	config := DefaultMatcherConfig()
	now := time.Now()

	transaction := &BankTransaction{
		ID:               "trans-1",
		TransactionDate:  now,
		Amount:           decimal.NewFromFloat(100.00),
		CounterpartyName: "Acme Corporation OÜ",
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-exact-name",
			PaymentNumber: "PMT-001",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
			ContactName:   "Acme Corporation OÜ",
		},
		{
			ID:            "pay-similar-name",
			PaymentNumber: "PMT-002",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
			ContactName:   "ACME Corp",
		},
		{
			ID:            "pay-diff-name",
			PaymentNumber: "PMT-003",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
			ContactName:   "Different Company",
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	// Verify name matching affects confidence
	if len(suggestions) == 0 {
		t.Fatal("Expected suggestions")
	}

	var exactNameConf, diffNameConf float64
	for _, s := range suggestions {
		if s.PaymentID == "pay-exact-name" {
			exactNameConf = s.Confidence
		}
		if s.PaymentID == "pay-diff-name" {
			diffNameConf = s.Confidence
		}
	}

	if exactNameConf <= diffNameConf {
		t.Errorf("Exact name match confidence (%f) should be higher than different name (%f)",
			exactNameConf, diffNameConf)
	}
}

func TestMatchPayments_PaymentNumberInDescription(t *testing.T) {
	config := DefaultMatcherConfig()
	now := time.Now()

	transaction := &BankTransaction{
		ID:              "trans-1",
		TransactionDate: now,
		Amount:          decimal.NewFromFloat(100.00),
		Description:     "Payment for PMT-001 from client",
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-1",
			PaymentNumber: "PMT-001",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
		},
		{
			ID:            "pay-2",
			PaymentNumber: "PMT-002",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00),
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	if len(suggestions) < 2 {
		t.Fatal("Expected at least 2 suggestions")
	}

	// Payment PMT-001 should have higher confidence due to number in description
	var pay1Conf, pay2Conf float64
	for _, s := range suggestions {
		if s.PaymentID == "pay-1" {
			pay1Conf = s.Confidence
		}
		if s.PaymentID == "pay-2" {
			pay2Conf = s.Confidence
		}
	}

	if pay1Conf <= pay2Conf {
		t.Errorf("Payment in description confidence (%f) should be higher than other (%f)",
			pay1Conf, pay2Conf)
	}
}

func TestMatchPayments_MinConfidenceFilter(t *testing.T) {
	config := DefaultMatcherConfig()
	config.MinConfidence = 0.8 // High threshold
	now := time.Now()

	transaction := &BankTransaction{
		ID:              "trans-1",
		TransactionDate: now.AddDate(0, 0, -30), // Old date
		Amount:          decimal.NewFromFloat(100.00),
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-1",
			PaymentNumber: "PMT-001",
			PaymentDate:   now,                         // Different date
			Amount:        decimal.NewFromFloat(75.00), // Different amount
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	// Should filter out low confidence matches
	if len(suggestions) > 0 {
		for _, s := range suggestions {
			if s.Confidence < config.MinConfidence {
				t.Errorf("Got suggestion with confidence %f below min %f", s.Confidence, config.MinConfidence)
			}
		}
	}
}

func TestMatchPayments_NegativeAmount(t *testing.T) {
	config := DefaultMatcherConfig()
	now := time.Now()

	// Negative transaction (outgoing payment)
	transaction := &BankTransaction{
		ID:              "trans-1",
		TransactionDate: now,
		Amount:          decimal.NewFromFloat(-100.00),
	}

	payments := []PaymentForMatching{
		{
			ID:            "pay-1",
			PaymentNumber: "PMT-001",
			PaymentDate:   now,
			Amount:        decimal.NewFromFloat(100.00), // Positive amount
		},
	}

	suggestions := matchPayments(transaction, payments, config)

	// Should match by absolute value
	if len(suggestions) == 0 {
		t.Error("Expected match for negative amount transaction with positive payment")
	}
}

func TestTransactionStatusValues(t *testing.T) {
	statuses := []struct {
		status   TransactionStatus
		expected string
	}{
		{StatusUnmatched, "UNMATCHED"},
		{StatusMatched, "MATCHED"},
		{StatusReconciled, "RECONCILED"},
	}

	for _, tt := range statuses {
		if string(tt.status) != tt.expected {
			t.Errorf("Status = %q, want %q", tt.status, tt.expected)
		}
	}
}

func TestReconciliationStatusValues(t *testing.T) {
	statuses := []struct {
		status   ReconciliationStatus
		expected string
	}{
		{ReconciliationInProgress, "IN_PROGRESS"},
		{ReconciliationCompleted, "COMPLETED"},
	}

	for _, tt := range statuses {
		if string(tt.status) != tt.expected {
			t.Errorf("Status = %q, want %q", tt.status, tt.expected)
		}
	}
}
