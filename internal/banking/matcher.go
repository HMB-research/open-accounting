package banking

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// MatcherConfig configures the matching algorithm
type MatcherConfig struct {
	// ExactAmountBonus is the confidence boost for exact amount matches
	ExactAmountBonus float64
	// DateProximityWeight is how much date proximity affects confidence
	DateProximityWeight float64
	// ReferenceMatchWeight is how much reference matching affects confidence
	ReferenceMatchWeight float64
	// NameMatchWeight is how much counterparty name matching affects confidence
	NameMatchWeight float64
	// MinConfidence is the minimum confidence to return a suggestion
	MinConfidence float64
	// MaxDateDiff is the maximum days difference to consider a match
	MaxDateDiff int
}

// DefaultMatcherConfig returns sensible default matching configuration
func DefaultMatcherConfig() MatcherConfig {
	return MatcherConfig{
		ExactAmountBonus:     0.5,
		DateProximityWeight:  0.2,
		ReferenceMatchWeight: 0.2,
		NameMatchWeight:      0.1,
		MinConfidence:        0.3,
		MaxDateDiff:          7,
	}
}

// PaymentForMatching is the payment data needed for matching
type PaymentForMatching struct {
	ID            string
	PaymentNumber string
	PaymentDate   time.Time
	Amount        decimal.Decimal
	ContactName   string
	Reference     string
}

// GetMatchSuggestions finds potential payment matches for a bank transaction
func (s *Service) GetMatchSuggestions(ctx context.Context, schemaName, tenantID, transactionID string) ([]MatchSuggestion, error) {
	// Get the transaction
	transaction, err := s.GetTransaction(ctx, schemaName, tenantID, transactionID)
	if err != nil {
		return nil, err
	}

	// Get unallocated payments
	payments, err := s.getUnallocatedPayments(ctx, schemaName, tenantID, transaction.Amount)
	if err != nil {
		return nil, fmt.Errorf("get unallocated payments: %w", err)
	}

	config := DefaultMatcherConfig()
	suggestions := matchPayments(transaction, payments, config)

	// Sort by confidence descending
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Confidence > suggestions[j].Confidence
	})

	// Limit to top 5 suggestions
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions, nil
}

// AutoMatchTransactions attempts to auto-match unmatched transactions
func (s *Service) AutoMatchTransactions(ctx context.Context, schemaName, tenantID, bankAccountID string, minConfidence float64) (int, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return 0, fmt.Errorf("ensure schema: %w", err)
	}

	// Get unmatched transactions
	filter := &TransactionFilter{
		BankAccountID: bankAccountID,
		Status:        StatusUnmatched,
	}
	transactions, err := s.ListTransactions(ctx, schemaName, tenantID, filter)
	if err != nil {
		return 0, fmt.Errorf("list transactions: %w", err)
	}

	matched := 0
	config := DefaultMatcherConfig()
	config.MinConfidence = minConfidence

	for _, transaction := range transactions {
		// Get potential matches
		payments, err := s.getUnallocatedPayments(ctx, schemaName, tenantID, transaction.Amount)
		if err != nil {
			continue
		}

		suggestions := matchPayments(&transaction, payments, config)
		if len(suggestions) == 0 {
			continue
		}

		// Only auto-match if confidence is high enough and there's a clear winner
		best := suggestions[0]
		if best.Confidence >= minConfidence {
			// Check if there's a significant gap to second best
			if len(suggestions) > 1 && suggestions[1].Confidence > best.Confidence*0.9 {
				// Too close, don't auto-match
				continue
			}

			err = s.MatchTransaction(ctx, schemaName, tenantID, transaction.ID, best.PaymentID)
			if err == nil {
				matched++
			}
		}
	}

	// Update import stats with matched count
	if matched > 0 {
		_, _ = s.db.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.bank_statement_imports
			SET transactions_matched = transactions_matched + $1
			WHERE bank_account_id = $2
			ORDER BY created_at DESC
			LIMIT 1
		`, schemaName), matched, bankAccountID)
	}

	return matched, nil
}

func (s *Service) getUnallocatedPayments(ctx context.Context, schemaName, tenantID string, amount decimal.Decimal) ([]PaymentForMatching, error) {
	// Query payments that haven't been fully allocated
	// Look for payments where amount matches (or close to) and have remaining unallocated amount
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT p.id, p.payment_number, p.payment_date, p.amount, COALESCE(c.name, '') as contact_name, COALESCE(p.reference, '') as reference
		FROM %s.payments p
		LEFT JOIN %s.contacts c ON p.contact_id = c.id
		WHERE p.tenant_id = $1
		AND p.amount - COALESCE((
			SELECT SUM(pa.amount) FROM %s.payment_allocations pa WHERE pa.payment_id = p.id
		), 0) > 0
		AND NOT EXISTS (
			SELECT 1 FROM %s.bank_transactions bt WHERE bt.matched_payment_id = p.id
		)
		ORDER BY ABS(p.amount - $2) ASC
		LIMIT 20
	`, schemaName, schemaName, schemaName, schemaName), tenantID, amount.Abs())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var payments []PaymentForMatching
	for rows.Next() {
		var p PaymentForMatching
		if err := rows.Scan(&p.ID, &p.PaymentNumber, &p.PaymentDate, &p.Amount, &p.ContactName, &p.Reference); err != nil {
			return nil, err
		}
		payments = append(payments, p)
	}

	return payments, nil
}

func matchPayments(transaction *BankTransaction, payments []PaymentForMatching, config MatcherConfig) []MatchSuggestion {
	var suggestions []MatchSuggestion

	transAmount := transaction.Amount.Abs()

	for _, payment := range payments {
		confidence := 0.0
		var reasons []string

		// Amount matching
		paymentAmount := payment.Amount.Abs()
		if transAmount.Equal(paymentAmount) {
			confidence += config.ExactAmountBonus
			reasons = append(reasons, "exact amount")
		} else {
			// Calculate how close the amounts are
			diff := transAmount.Sub(paymentAmount).Abs()
			percentDiff := diff.Div(paymentAmount).InexactFloat64()
			if percentDiff < 0.01 {
				confidence += config.ExactAmountBonus * 0.8
				reasons = append(reasons, "amount within 1%")
			} else if percentDiff < 0.05 {
				confidence += config.ExactAmountBonus * 0.5
				reasons = append(reasons, "amount within 5%")
			}
		}

		// Date proximity
		daysDiff := math.Abs(transaction.TransactionDate.Sub(payment.PaymentDate).Hours() / 24)
		if daysDiff <= float64(config.MaxDateDiff) {
			// Linear decay: full weight at 0 days, 0 at MaxDateDiff
			dateScore := config.DateProximityWeight * (1 - daysDiff/float64(config.MaxDateDiff))
			confidence += dateScore
			if daysDiff == 0 {
				reasons = append(reasons, "same date")
			} else if daysDiff <= 2 {
				reasons = append(reasons, "date within 2 days")
			}
		}

		// Reference matching
		if transaction.Reference != "" && payment.Reference != "" {
			refSimilarity := calculateStringSimilarity(
				normalizeReference(transaction.Reference),
				normalizeReference(payment.Reference),
			)
			if refSimilarity > 0.8 {
				confidence += config.ReferenceMatchWeight
				reasons = append(reasons, "reference match")
			} else if refSimilarity > 0.5 {
				confidence += config.ReferenceMatchWeight * 0.5
				reasons = append(reasons, "partial reference match")
			}
		}

		// Counterparty name matching
		if transaction.CounterpartyName != "" && payment.ContactName != "" {
			nameSimilarity := calculateStringSimilarity(
				normalizeName(transaction.CounterpartyName),
				normalizeName(payment.ContactName),
			)
			if nameSimilarity > 0.7 {
				confidence += config.NameMatchWeight
				reasons = append(reasons, "name match")
			} else if nameSimilarity > 0.4 {
				confidence += config.NameMatchWeight * 0.5
				reasons = append(reasons, "partial name match")
			}
		}

		// Check if description contains payment number
		if strings.Contains(strings.ToLower(transaction.Description), strings.ToLower(payment.PaymentNumber)) {
			confidence += 0.1
			reasons = append(reasons, "payment number in description")
		}

		if confidence >= config.MinConfidence {
			// Normalize confidence to 0-1 range
			if confidence > 1.0 {
				confidence = 1.0
			}

			suggestions = append(suggestions, MatchSuggestion{
				PaymentID:     payment.ID,
				PaymentNumber: payment.PaymentNumber,
				PaymentDate:   payment.PaymentDate,
				Amount:        payment.Amount,
				ContactName:   payment.ContactName,
				Reference:     payment.Reference,
				Confidence:    confidence,
				MatchReason:   strings.Join(reasons, ", "),
			})
		}
	}

	return suggestions
}

// normalizeReference cleans up a reference for comparison
func normalizeReference(ref string) string {
	// Remove common prefixes/suffixes
	ref = strings.TrimSpace(ref)
	ref = strings.ToLower(ref)

	// Remove non-alphanumeric characters
	re := regexp.MustCompile(`[^a-z0-9]`)
	ref = re.ReplaceAllString(ref, "")

	return ref
}

// normalizeName cleans up a name for comparison
func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)

	// Remove common business suffixes
	suffixes := []string{" oü", " as", " ou", " llc", " ltd", " inc", " gmbh", " ag"}
	for _, suffix := range suffixes {
		name = strings.TrimSuffix(name, suffix)
	}

	// Remove extra spaces
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")

	return strings.TrimSpace(name)
}

// calculateStringSimilarity calculates Jaro-Winkler similarity between two strings
func calculateStringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	// Simple implementation using Levenshtein distance ratio
	maxLen := float64(max(len(s1), len(s2)))
	distance := float64(levenshteinDistance(s1, s2))

	return 1.0 - (distance / maxLen)
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	m := len(s1)
	n := len(s2)
	matrix := make([][]int, m+1)
	for i := range matrix {
		matrix[i] = make([]int, n+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[m][n]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CreatePaymentFromTransaction creates a new payment from a bank transaction
func (s *Service) CreatePaymentFromTransaction(ctx context.Context, schemaName, tenantID, userID, transactionID string) (string, error) {
	// Get the transaction
	transaction, err := s.GetTransaction(ctx, schemaName, tenantID, transactionID)
	if err != nil {
		return "", err
	}

	if transaction.Status != StatusUnmatched {
		return "", fmt.Errorf("transaction is already matched")
	}

	// Determine payment type based on amount sign
	paymentType := "RECEIVED"
	if transaction.Amount.IsNegative() {
		paymentType = "MADE"
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Generate payment number
	var paymentNumber string
	prefix := "PMT"
	if paymentType == "MADE" {
		prefix = "PAY"
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(MAX(CAST(SUBSTRING(payment_number FROM '%s-([0-9]+)') AS INTEGER)), 0) + 1
		FROM %s.payments
		WHERE tenant_id = $1 AND payment_number LIKE '%s-%%'
	`, prefix, schemaName, prefix), tenantID).Scan(&paymentNumber)
	if err != nil {
		return "", fmt.Errorf("generate payment number: %w", err)
	}
	paymentNumber = fmt.Sprintf("%s-%06d", prefix, parseIntOrDefault(paymentNumber, 1))

	// Create payment
	paymentID := ""
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.payments (tenant_id, payment_number, payment_type, payment_date, amount, currency, exchange_rate, base_amount, reference, notes, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, 1, $5, $7, $8, $9)
		RETURNING id
	`, schemaName), tenantID, paymentNumber, paymentType, transaction.TransactionDate,
		transaction.Amount.Abs(), transaction.Currency, transaction.Reference,
		fmt.Sprintf("Created from bank transaction: %s", transaction.Description), userID).Scan(&paymentID)
	if err != nil {
		return "", fmt.Errorf("create payment: %w", err)
	}

	// Link transaction to payment
	_, err = tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.bank_transactions
		SET matched_payment_id = $1, status = 'MATCHED'
		WHERE id = $2 AND tenant_id = $3
	`, schemaName), paymentID, transactionID, tenantID)
	if err != nil {
		return "", fmt.Errorf("link transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit: %w", err)
	}

	return paymentID, nil
}
