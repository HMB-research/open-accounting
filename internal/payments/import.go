package payments

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contactrefs"
)

type paymentImportRow struct {
	rowNumber int
	values    map[string]string
}

var paymentImportHeaderAliases = map[string]string{
	"payment_number":     "payment_number",
	"number":             "payment_number",
	"payment_no":         "payment_number",
	"type":               "payment_type",
	"payment_type":       "payment_type",
	"date":               "payment_date",
	"payment_date":       "payment_date",
	"contact_id":         "contact_id",
	"customer_id":        "contact_id",
	"supplier_id":        "contact_id",
	"contact_code":       "contact_code",
	"customer_code":      "contact_code",
	"supplier_code":      "contact_code",
	"contact_reg_code":   "contact_reg_code",
	"contact_vat_number": "contact_vat_number",
	"vat_number":         "contact_vat_number",
	"contact_email":      "contact_email",
	"email":              "contact_email",
	"contact_name":       "contact_name",
	"customer_name":      "contact_name",
	"supplier_name":      "contact_name",
	"amount":             "amount",
	"currency":           "currency",
	"exchange_rate":      "exchange_rate",
	"method":             "payment_method",
	"payment_method":     "payment_method",
	"bank_account":       "bank_account",
	"reference":          "reference",
	"notes":              "notes",
	"description":        "notes",
	"invoice_id":         "invoice_id",
	"invoice_number":     "invoice_number",
	"invoice_no":         "invoice_number",
	"allocation_amount":  "allocation_amount",
	"allocated_amount":   "allocation_amount",
}

// ImportPaymentsCSV imports historical payments from CSV content.
func (s *Service) ImportPaymentsCSV(ctx context.Context, tenantID, schemaName string, req *ImportPaymentsRequest) (*ImportPaymentsResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parsePaymentImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no payments found in CSV")
	}

	existingPayments, err := s.repo.List(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing payments: %w", err)
	}
	usedNumbers := make(map[string]string, len(existingPayments)+len(rows))
	for _, payment := range existingPayments {
		key := normalizedPaymentImportKey(payment.PaymentNumber)
		if key == "" {
			continue
		}
		usedNumbers[key] = payment.ID
	}

	result := &ImportPaymentsResult{
		FileName: req.FileName,
		Errors:   []ImportPaymentsRowError{},
	}
	contactLookup, err := s.paymentImportContactLookup(ctx, schemaName, tenantID, rows)
	if err != nil {
		return nil, err
	}

	for _, row := range rows {
		result.RowsProcessed++

		payment, allocation, err := s.buildPaymentFromImportRow(ctx, tenantID, schemaName, row, req.UserID, req.LockDate, contactLookup)
		if err != nil {
			appendPaymentImportRowError(result, row, err)
			continue
		}

		if payment.PaymentNumber == "" {
			if err := s.assignImportedPaymentNumber(ctx, schemaName, tenantID, payment, usedNumbers); err != nil {
				appendPaymentImportRowError(result, row, err)
				continue
			}
		}

		numberKey := normalizedPaymentImportKey(payment.PaymentNumber)
		if existingID, exists := usedNumbers[numberKey]; exists {
			appendPaymentImportRowError(result, row, fmt.Errorf("duplicate payment_number %q matches existing payment %q", payment.PaymentNumber, existingID))
			continue
		}

		if err := s.repo.Create(ctx, schemaName, payment); err != nil {
			appendPaymentImportRowError(result, row, err)
			continue
		}

		if allocation != nil {
			allocation.PaymentID = payment.ID
			if err := s.repo.CreateAllocation(ctx, schemaName, allocation); err != nil {
				appendPaymentImportRowError(result, row, fmt.Errorf("insert allocation: %w", err))
				continue
			}
			payment.Allocations = append(payment.Allocations, *allocation)
			if s.invoicing != nil {
				if err := s.invoicing.RecordPayment(ctx, tenantID, schemaName, allocation.InvoiceID, allocation.Amount); err != nil {
					fmt.Printf("warning: failed to update invoice %s payment amount: %v\n", allocation.InvoiceID, err)
				}
			}
		}

		usedNumbers[numberKey] = payment.ID
		result.PaymentsCreated++
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func (s *Service) assignImportedPaymentNumber(ctx context.Context, schemaName, tenantID string, payment *Payment, usedNumbers map[string]string) error {
	for attempt := 0; attempt < 100; attempt++ {
		seq, err := s.repo.GetNextPaymentNumber(ctx, schemaName, tenantID, payment.PaymentType)
		if err != nil {
			return fmt.Errorf("generate payment number: %w", err)
		}
		candidate := FormatPaymentNumber(payment.PaymentType, seq)
		if _, exists := usedNumbers[normalizedPaymentImportKey(candidate)]; exists {
			continue
		}
		payment.PaymentNumber = candidate
		return nil
	}

	return fmt.Errorf("generate payment number: exhausted duplicate attempts")
}

func appendPaymentImportRowError(result *ImportPaymentsResult, row paymentImportRow, err error) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportPaymentsRowError{
		Row:           row.rowNumber,
		PaymentNumber: strings.TrimSpace(row.values["payment_number"]),
		Reference:     strings.TrimSpace(row.values["reference"]),
		Message:       err.Error(),
	})
}

func parsePaymentImportRows(content string) ([]paymentImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectPaymentImportDelimiter(trimmed)
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
	required := map[string]bool{
		"payment_type": false,
		"payment_date": false,
		"amount":       false,
	}
	for i, header := range headers {
		canonicalHeaders[i] = canonicalPaymentImportHeader(header)
		if _, ok := required[canonicalHeaders[i]]; ok {
			required[canonicalHeaders[i]] = true
		}
	}

	var missing []string
	for _, name := range []string{"payment_type", "payment_date", "amount"} {
		if !required[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required columns: %s", strings.Join(missing, ", "))
	}

	rows := make([]paymentImportRow, 0)
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
		values := make(map[string]string, len(canonicalHeaders))
		isBlank := true
		for i, header := range canonicalHeaders {
			if header == "" {
				continue
			}
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				isBlank = false
			}
			values[header] = value
		}
		if isBlank {
			continue
		}
		rows = append(rows, paymentImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func (s *Service) buildPaymentFromImportRow(
	ctx context.Context,
	tenantID string,
	schemaName string,
	row paymentImportRow,
	userID string,
	lockDate *time.Time,
	contactLookup contactrefs.ContactLookup,
) (*Payment, *PaymentAllocation, error) {
	paymentType, err := parsePaymentImportType(row.values["payment_type"])
	if err != nil {
		return nil, nil, err
	}
	paymentDate, err := parsePaymentImportDate(row.values["payment_date"])
	if err != nil {
		return nil, nil, err
	}
	if lockDate != nil && !normalizePaymentImportDate(paymentDate).After(normalizePaymentImportDate(*lockDate)) {
		return nil, nil, fmt.Errorf("period locked through %s; payment_date %s must be later", lockDate.Format("2006-01-02"), paymentDate.Format("2006-01-02"))
	}
	amount, err := parsePaymentImportPositiveDecimal("amount", row.values["amount"])
	if err != nil {
		return nil, nil, err
	}
	exchangeRate, err := parsePaymentImportOptionalPositiveDecimal("exchange_rate", row.values["exchange_rate"], decimal.NewFromInt(1))
	if err != nil {
		return nil, nil, err
	}
	contactID, err := resolveOptionalPaymentImportContactID(row, contactLookup)
	if err != nil {
		return nil, nil, err
	}

	currency := strings.ToUpper(strings.TrimSpace(row.values["currency"]))
	if currency == "" {
		currency = "EUR"
	}
	now := time.Now()
	payment := &Payment{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		PaymentNumber: strings.TrimSpace(row.values["payment_number"]),
		PaymentType:   paymentType,
		ContactID:     contactID,
		PaymentDate:   paymentDate,
		Amount:        amount,
		Currency:      currency,
		ExchangeRate:  exchangeRate,
		BaseAmount:    amount.Mul(exchangeRate).Round(2),
		PaymentMethod: strings.TrimSpace(row.values["payment_method"]),
		BankAccount:   strings.TrimSpace(row.values["bank_account"]),
		Reference:     strings.TrimSpace(row.values["reference"]),
		Notes:         strings.TrimSpace(row.values["notes"]),
		CreatedAt:     now,
		CreatedBy:     userID,
	}

	allocation, err := s.buildPaymentImportAllocation(ctx, tenantID, schemaName, row, payment.ID, amount, now)
	if err != nil {
		return nil, nil, err
	}

	return payment, allocation, nil
}

func (s *Service) buildPaymentImportAllocation(
	ctx context.Context,
	tenantID string,
	schemaName string,
	row paymentImportRow,
	paymentID string,
	paymentAmount decimal.Decimal,
	now time.Time,
) (*PaymentAllocation, error) {
	invoiceID, err := s.resolvePaymentImportInvoiceID(ctx, tenantID, schemaName, row)
	if err != nil {
		return nil, err
	}
	allocationValue := strings.TrimSpace(row.values["allocation_amount"])
	if invoiceID == "" && allocationValue == "" {
		return nil, nil
	}
	if invoiceID == "" {
		return nil, fmt.Errorf("invoice_id or invoice_number is required when allocation_amount is provided")
	}

	amount := paymentAmount
	if allocationValue != "" {
		parsed, err := parsePaymentImportPositiveDecimal("allocation_amount", allocationValue)
		if err != nil {
			return nil, err
		}
		amount = parsed
	}
	if amount.GreaterThan(paymentAmount) {
		return nil, fmt.Errorf("allocation_amount exceeds payment amount")
	}

	return &PaymentAllocation{
		ID:        uuid.New().String(),
		TenantID:  tenantID,
		PaymentID: paymentID,
		InvoiceID: invoiceID,
		Amount:    amount,
		CreatedAt: now,
	}, nil
}

func (s *Service) resolvePaymentImportInvoiceID(ctx context.Context, tenantID, schemaName string, row paymentImportRow) (string, error) {
	if invoiceID := strings.TrimSpace(row.values["invoice_id"]); invoiceID != "" {
		parsedID, err := uuid.Parse(invoiceID)
		if err != nil {
			return "", fmt.Errorf("invoice_id must be a valid UUID")
		}
		return parsedID.String(), nil
	}

	invoiceNumber := strings.TrimSpace(row.values["invoice_number"])
	if invoiceNumber == "" {
		return "", nil
	}
	if s.invoicing == nil {
		return "", fmt.Errorf("invoice_number %q cannot be resolved without invoicing service", invoiceNumber)
	}

	invoiceID, err := s.invoicing.ResolveInvoiceIDByNumber(ctx, tenantID, schemaName, invoiceNumber)
	if err != nil {
		return "", fmt.Errorf("resolve invoice_number %q: %w", invoiceNumber, err)
	}
	return invoiceID, nil
}

func (s *Service) paymentImportContactLookup(ctx context.Context, schemaName, tenantID string, rows []paymentImportRow) (contactrefs.ContactLookup, error) {
	usesContactReferences := false
	for _, row := range rows {
		if hasPaymentImportContactLookupReference(row) {
			usesContactReferences = true
			break
		}
	}
	if !usesContactReferences {
		return contactrefs.ContactLookup{}, nil
	}
	if s.contacts == nil {
		return contactrefs.ContactLookup{}, fmt.Errorf("contact service is required to resolve payment contact references")
	}

	contacts, err := s.contacts.List(ctx, tenantID, schemaName, nil)
	if err != nil {
		return contactrefs.ContactLookup{}, fmt.Errorf("list contacts for payment import: %w", err)
	}
	return contactrefs.NewContactLookup(contacts), nil
}

func resolveOptionalPaymentImportContactID(row paymentImportRow, contactLookup contactrefs.ContactLookup) (*string, error) {
	return contactLookup.ResolveID(row.values["contact_id"], paymentImportContactReferences(row)...)
}

func hasPaymentImportContactLookupReference(row paymentImportRow) bool {
	for _, ref := range paymentImportContactReferences(row) {
		if strings.TrimSpace(ref.Value) != "" {
			return true
		}
	}
	return false
}

func paymentImportContactReferences(row paymentImportRow) []contactrefs.Reference {
	return []contactrefs.Reference{
		{Field: "contact_code", Value: row.values["contact_code"]},
		{Field: "contact_reg_code", Value: row.values["contact_reg_code"]},
		{Field: "contact_vat_number", Value: row.values["contact_vat_number"]},
		{Field: "contact_email", Value: row.values["contact_email"]},
		{Field: "contact_name", Value: row.values["contact_name"]},
	}
}

func parsePaymentImportType(value string) (PaymentType, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	switch PaymentType(normalized) {
	case PaymentTypeReceived, PaymentTypeMade:
		return PaymentType(normalized), nil
	default:
		return "", fmt.Errorf("invalid payment_type %q", value)
	}
}

func parsePaymentImportDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("payment_date is required")
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("payment_date must be YYYY-MM-DD or RFC3339")
}

func parsePaymentImportPositiveDecimal(field, value string) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return decimal.Zero, fmt.Errorf("%s is required", field)
	}
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%s must be a decimal", field)
	}
	if parsed.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("%s must be positive", field)
	}
	return parsed, nil
}

func parsePaymentImportOptionalPositiveDecimal(field, value string, fallback decimal.Decimal) (decimal.Decimal, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return parsePaymentImportPositiveDecimal(field, value)
}

func detectPaymentImportDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexByte(content, '\n'); idx >= 0 {
		firstLine = content[:idx]
	}

	bestDelimiter := ','
	bestCount := strings.Count(firstLine, string(bestDelimiter))
	for _, delimiter := range []rune{';', '\t'} {
		count := strings.Count(firstLine, string(delimiter))
		if count > bestCount {
			bestDelimiter = delimiter
			bestCount = count
		}
	}
	return bestDelimiter
}

func canonicalPaymentImportHeader(header string) string {
	normalized := normalizedPaymentImportHeader(header)
	if canonical, ok := paymentImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedPaymentImportHeader(header string) string {
	normalized := strings.TrimSpace(strings.ToLower(header))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", ".", "_")
	return replacer.Replace(normalized)
}

func normalizedPaymentImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizePaymentImportDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}
