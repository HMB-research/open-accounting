package expenses

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type expenseImportRow struct {
	rowNumber int
	values    map[string]string
}

var expenseImportHeaderAliases = map[string]string{
	"expense_number":     "expense_number",
	"expense_no":         "expense_number",
	"number":             "expense_number",
	"date":               "expense_date",
	"expense_date":       "expense_date",
	"merchant":           "merchant",
	"supplier":           "merchant",
	"vendor":             "merchant",
	"description":        "description",
	"notes":              "description",
	"employee_id":        "employee_id",
	"contact_id":         "contact_id",
	"expense_account_id": "expense_account_id",
	"expense_account":    "expense_account_id",
	"payment_account_id": "payment_account_id",
	"payment_account":    "payment_account_id",
	"amount":             "amount",
	"currency":           "currency",
	"exchange_rate":      "exchange_rate",
	"requires_receipt":   "requires_receipt",
	"receipt_required":   "requires_receipt",
	"status":             "status",
	"rejection_reason":   "rejection_reason",
	"submitted_at":       "submitted_at",
	"approved_at":        "approved_at",
	"rejected_at":        "rejected_at",
}

func (s *Service) ImportExpensesCSV(ctx context.Context, schemaName, tenantID string, req *ImportExpensesRequest) (*ImportExpensesResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}

	rows, err := parseExpenseImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no expenses found in CSV")
	}

	result := &ImportExpensesResult{
		FileName: req.FileName,
		Errors:   []ImportExpensesRowError{},
	}
	usedNumbers := map[string]bool{}

	for _, row := range rows {
		result.RowsProcessed++

		expense, err := s.buildExpenseFromImportRow(ctx, schemaName, tenantID, userID, row, req.LockDate, usedNumbers)
		if err != nil {
			appendExpenseImportRowError(result, row, err)
			continue
		}
		if err := s.repo.Create(ctx, schemaName, expense); err != nil {
			appendExpenseImportRowError(result, row, err)
			continue
		}

		usedNumbers[normalizedExpenseImportKey(expense.ExpenseNumber)] = true
		result.ExpensesCreated++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	return result, nil
}

func (s *Service) buildExpenseFromImportRow(
	ctx context.Context,
	schemaName, tenantID, userID string,
	row expenseImportRow,
	lockDate *time.Time,
	usedNumbers map[string]bool,
) (*Expense, error) {
	expenseDate, err := parseExpenseImportDate(row.values["expense_date"], "expense_date")
	if err != nil {
		return nil, err
	}
	if lockDate != nil && !expenseDate.After(*lockDate) {
		return nil, fmt.Errorf("period locked through %s", lockDate.Format("2006-01-02"))
	}

	merchant := strings.TrimSpace(row.values["merchant"])
	if merchant == "" {
		return nil, fmt.Errorf("merchant is required")
	}
	expenseAccountID := strings.TrimSpace(row.values["expense_account_id"])
	if expenseAccountID == "" {
		return nil, fmt.Errorf("expense_account_id is required")
	}
	paymentAccountID := strings.TrimSpace(row.values["payment_account_id"])
	if paymentAccountID == "" {
		return nil, fmt.Errorf("payment_account_id is required")
	}

	amount, err := parseExpenseImportDecimal(row.values["amount"], "amount")
	if err != nil {
		return nil, err
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("amount must be positive")
	}
	currency, err := normalizeCurrency(row.values["currency"])
	if err != nil {
		return nil, err
	}
	exchangeRate, err := normalizeExchangeRate(decimal.Zero)
	if strings.TrimSpace(row.values["exchange_rate"]) != "" {
		exchangeRate, err = parseExpenseImportDecimal(row.values["exchange_rate"], "exchange_rate")
		if err == nil && exchangeRate.LessThanOrEqual(decimal.Zero) {
			err = fmt.Errorf("exchange_rate must be positive")
		}
	}
	if err != nil {
		return nil, err
	}

	status, err := parseExpenseImportStatus(row.values["status"])
	if err != nil {
		return nil, err
	}
	requiresReceipt := true
	if strings.TrimSpace(row.values["requires_receipt"]) != "" {
		requiresReceipt, err = parseExpenseImportBool(row.values["requires_receipt"], "requires_receipt")
		if err != nil {
			return nil, err
		}
	}

	expenseNumber := strings.TrimSpace(row.values["expense_number"])
	if expenseNumber == "" {
		expenseNumber, err = s.repo.GenerateNumber(ctx, schemaName, tenantID)
		if err != nil {
			return nil, fmt.Errorf("generate expense number: %w", err)
		}
	}
	numberKey := normalizedExpenseImportKey(expenseNumber)
	if numberKey == "" {
		return nil, fmt.Errorf("expense_number is required")
	}
	if usedNumbers[numberKey] {
		return nil, fmt.Errorf("duplicate expense_number %q in import file", expenseNumber)
	}

	now := s.now()
	expense := &Expense{
		ID:               uuid.NewString(),
		TenantID:         tenantID,
		ExpenseNumber:    expenseNumber,
		ExpenseDate:      expenseDate,
		Merchant:         merchant,
		Description:      strings.TrimSpace(row.values["description"]),
		EmployeeID:       stringPtrOrNil(row.values["employee_id"]),
		ContactID:        stringPtrOrNil(row.values["contact_id"]),
		ExpenseAccountID: expenseAccountID,
		PaymentAccountID: paymentAccountID,
		Amount:           amount,
		Currency:         currency,
		ExchangeRate:     exchangeRate,
		BaseAmount:       amount.Mul(exchangeRate).Round(2),
		RequiresReceipt:  requiresReceipt,
		Status:           status,
		CreatedAt:        now,
		CreatedBy:        userID,
		UpdatedAt:        now,
	}
	if err := applyExpenseImportStatusMetadata(expense, row, userID, now); err != nil {
		return nil, err
	}
	return expense, nil
}

func applyExpenseImportStatusMetadata(expense *Expense, row expenseImportRow, userID string, now time.Time) error {
	switch expense.Status {
	case StatusDraft:
		return nil
	case StatusSubmitted:
		submittedAt, err := parseOptionalExpenseImportDateTime(row.values["submitted_at"], now)
		if err != nil {
			return fmt.Errorf("submitted_at: %w", err)
		}
		expense.SubmittedAt = &submittedAt
		expense.SubmittedBy = &userID
	case StatusApproved:
		submittedAt, err := parseOptionalExpenseImportDateTime(row.values["submitted_at"], now)
		if err != nil {
			return fmt.Errorf("submitted_at: %w", err)
		}
		approvedAt, err := parseOptionalExpenseImportDateTime(row.values["approved_at"], now)
		if err != nil {
			return fmt.Errorf("approved_at: %w", err)
		}
		expense.SubmittedAt = &submittedAt
		expense.SubmittedBy = &userID
		expense.ApprovedAt = &approvedAt
		expense.ApprovedBy = &userID
	case StatusRejected:
		reason := strings.TrimSpace(row.values["rejection_reason"])
		if reason == "" {
			return fmt.Errorf("rejection_reason is required for rejected expenses")
		}
		submittedAt, err := parseOptionalExpenseImportDateTime(row.values["submitted_at"], now)
		if err != nil {
			return fmt.Errorf("submitted_at: %w", err)
		}
		rejectedAt, err := parseOptionalExpenseImportDateTime(row.values["rejected_at"], now)
		if err != nil {
			return fmt.Errorf("rejected_at: %w", err)
		}
		expense.SubmittedAt = &submittedAt
		expense.SubmittedBy = &userID
		expense.RejectedAt = &rejectedAt
		expense.RejectedBy = &userID
		expense.RejectionReason = reason
	}
	return nil
}

func parseExpenseImportRows(content string) ([]expenseImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectExpenseImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("csv file is empty")
		}
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	for i, header := range headers {
		canonicalHeaders[i] = canonicalExpenseImportHeader(header)
	}

	rows := []expenseImportRow{}
	rowNumber := 1
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("parse csv row %d: %w", rowNumber+1, err)
		}
		rowNumber++
		values := map[string]string{}
		blank := true
		for i, header := range canonicalHeaders {
			if header == "" {
				continue
			}
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				blank = false
			}
			values[header] = value
		}
		if blank {
			continue
		}
		rows = append(rows, expenseImportRow{rowNumber: rowNumber, values: values})
	}
	return rows, nil
}

func appendExpenseImportRowError(result *ImportExpensesResult, row expenseImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportExpensesRowError{
		Row:           row.rowNumber,
		ExpenseNumber: strings.TrimSpace(row.values["expense_number"]),
		Merchant:      strings.TrimSpace(row.values["merchant"]),
		Message:       err.Error(),
	})
}

func parseExpenseImportStatus(value string) (ExpenseStatus, error) {
	normalized := normalizedExpenseImportKey(value)
	switch normalized {
	case "", "draft":
		return StatusDraft, nil
	case "submitted":
		return StatusSubmitted, nil
	case "approved":
		return StatusApproved, nil
	case "rejected":
		return StatusRejected, nil
	case "posted":
		return "", fmt.Errorf("posted expenses must be imported as approved and posted through the expense workflow")
	default:
		return "", fmt.Errorf("invalid status %q", value)
	}
}

func parseExpenseImportDate(value, field string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("%s is required", field)
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid %s", field)
}

func parseOptionalExpenseImportDateTime(value string, fallback time.Time) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp")
}

func parseExpenseImportDecimal(value, field string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, fmt.Errorf("%s is required", field)
	}
	parsed, err := decimal.NewFromString(normalizeExpenseImportDecimal(trimmed))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

func parseExpenseImportBool(value, field string) (bool, error) {
	switch normalizedExpenseImportKey(value) {
	case "true", "t", "yes", "y", "1":
		return true, nil
	case "false", "f", "no", "n", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s", field)
	}
}

func canonicalExpenseImportHeader(value string) string {
	normalized := normalizedExpenseImportKey(value)
	if canonical, ok := expenseImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func detectExpenseImportDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexAny(content, "\r\n"); idx >= 0 {
		firstLine = content[:idx]
	}
	best := ','
	bestCount := -1
	for _, delimiter := range []rune{',', ';', '\t'} {
		count := strings.Count(firstLine, string(delimiter))
		if count > bestCount {
			best = delimiter
			bestCount = count
		}
	}
	return best
}

func normalizedExpenseImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, " ", "_")))
}

func normalizeExpenseImportDecimal(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
}

func stringPtrOrNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
