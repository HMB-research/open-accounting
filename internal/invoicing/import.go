package invoicing

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

type invoiceImportRow struct {
	rowNumber int
	values    map[string]string
}

type invoiceImportContactRef struct {
	code    string
	regCode string
	email   string
	name    string
}

type invoiceImportLine struct {
	description     string
	quantity        decimal.Decimal
	unit            string
	unitPrice       decimal.Decimal
	discountPercent decimal.Decimal
	vatRate         decimal.Decimal
}

type invoiceImportHeader struct {
	invoiceNumber       string
	invoiceType         InvoiceType
	contactRef          invoiceImportContactRef
	issueDate           time.Time
	dueDate             time.Time
	currency            string
	exchangeRate        decimal.Decimal
	reference           string
	notes               string
	explicitStatus      InvoiceStatus
	amountPaid          decimal.Decimal
	amountPaidSpecified bool
}

type invoiceImportParsedRow struct {
	header invoiceImportHeader
	line   invoiceImportLine
}

type invoiceImportGroup struct {
	header       invoiceImportHeader
	lines        []invoiceImportLine
	rowCount     int
	firstRow     int
	conflictRow  int
	conflictText string
}

type invoiceImportContactLookup struct {
	byCode    map[string]contacts.Contact
	byRegCode map[string]contacts.Contact
	byEmail   map[string]contacts.Contact
	byName    map[string]contacts.Contact
}

var invoiceImportHeaderAliases = map[string]string{
	"invoice_number":     "invoice_number",
	"number":             "invoice_number",
	"invoice_no":         "invoice_number",
	"invoice_no.":        "invoice_number",
	"invoice_type":       "invoice_type",
	"type":               "invoice_type",
	"contact_code":       "contact_code",
	"customer_code":      "contact_code",
	"supplier_code":      "contact_code",
	"contact_reg_code":   "contact_reg_code",
	"contact_vat_number": "contact_reg_code",
	"contact_email":      "contact_email",
	"email":              "contact_email",
	"contact_name":       "contact_name",
	"customer_name":      "contact_name",
	"supplier_name":      "contact_name",
	"issue_date":         "issue_date",
	"invoice_date":       "issue_date",
	"due_date":           "due_date",
	"currency":           "currency",
	"exchange_rate":      "exchange_rate",
	"reference":          "reference",
	"notes":              "notes",
	"status":             "status",
	"amount_paid":        "amount_paid",
	"paid_amount":        "amount_paid",
	"line_description":   "line_description",
	"description":        "line_description",
	"quantity":           "quantity",
	"qty":                "quantity",
	"unit":               "unit",
	"unit_price":         "unit_price",
	"price":              "unit_price",
	"discount_percent":   "discount_percent",
	"discount":           "discount_percent",
	"vat_rate":           "vat_rate",
	"vat":                "vat_rate",
}

var invoiceImportTypeAliases = map[string]InvoiceType{
	"sales":            InvoiceTypeSales,
	"sale":             InvoiceTypeSales,
	"salesinvoice":     InvoiceTypeSales,
	"sales_invoice":    InvoiceTypeSales,
	"sales invoice":    InvoiceTypeSales,
	"myygiarve":        InvoiceTypeSales,
	"purchase":         InvoiceTypePurchase,
	"purchaseinvoice":  InvoiceTypePurchase,
	"purchase_invoice": InvoiceTypePurchase,
	"purchase invoice": InvoiceTypePurchase,
	"bill":             InvoiceTypePurchase,
	"ostuarve":         InvoiceTypePurchase,
	"credit_note":      InvoiceTypeCreditNote,
	"creditnote":       InvoiceTypeCreditNote,
	"credit note":      InvoiceTypeCreditNote,
	"kreeditarve":      InvoiceTypeCreditNote,
}

var invoiceImportStatusAliases = map[string]InvoiceStatus{
	"draft":            StatusDraft,
	"mustand":          StatusDraft,
	"sent":             StatusSent,
	"issued":           StatusSent,
	"open":             StatusSent,
	"saadetud":         StatusSent,
	"partially_paid":   StatusPartiallyPaid,
	"partial":          StatusPartiallyPaid,
	"osaline":          StatusPartiallyPaid,
	"paid":             StatusPaid,
	"makstud":          StatusPaid,
	"overdue":          StatusOverdue,
	"tahtaja_uletanud": StatusOverdue,
	"voided":           StatusVoided,
	"void":             StatusVoided,
	"tuhistatud":       StatusVoided,
}

// ImportCSV imports invoices from grouped CSV rows. Each row represents one invoice line.
func (s *Service) ImportCSV(
	ctx context.Context,
	tenantID, schemaName string,
	existingContacts []contacts.Contact,
	req *ImportInvoicesRequest,
	validateDate func(time.Time) error,
) (*ImportInvoicesResult, error) {
	if strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseInvoiceImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no invoices found in CSV")
	}

	existingInvoices, err := s.repo.List(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing invoices: %w", err)
	}

	existingKeys := make(map[string]struct{}, len(existingInvoices))
	for _, invoice := range existingInvoices {
		existingKeys[normalizedInvoiceImportGroupKey(invoice.InvoiceNumber, invoice.InvoiceType)] = struct{}{}
	}

	result := &ImportInvoicesResult{
		FileName: req.FileName,
		Errors:   []ImportInvoicesRowError{},
	}

	contactLookup := buildInvoiceImportContactLookup(existingContacts)
	groupOrder := make([]string, 0)
	groups := make(map[string]*invoiceImportGroup)

	for _, row := range rows {
		result.RowsProcessed++

		parsed, err := parseInvoiceImportDataRow(row)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           row.rowNumber,
				InvoiceNumber: strings.TrimSpace(row.values["invoice_number"]),
				Message:       err.Error(),
			})
			continue
		}

		key := normalizedInvoiceImportGroupKey(parsed.header.invoiceNumber, parsed.header.invoiceType)
		group, ok := groups[key]
		if !ok {
			group = &invoiceImportGroup{
				header:   parsed.header,
				firstRow: row.rowNumber,
			}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		} else if conflict := mergeInvoiceImportGroup(group, parsed.header, row.rowNumber); conflict != "" && group.conflictText == "" {
			group.conflictRow = row.rowNumber
			group.conflictText = conflict
		}

		group.lines = append(group.lines, parsed.line)
		group.rowCount++
	}

	now := normalizeInvoiceImportDate(time.Now())
	for _, key := range groupOrder {
		group := groups[key]

		if group.conflictText != "" {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           group.conflictRow,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       group.conflictText,
			})
			continue
		}

		if _, exists := existingKeys[key]; exists {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           group.firstRow,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       fmt.Sprintf("invoice_number %q already exists for invoice_type %s", group.header.invoiceNumber, group.header.invoiceType),
			})
			continue
		}

		contact, err := contactLookup.find(group.header.contactRef)
		if err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           group.firstRow,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       err.Error(),
			})
			continue
		}

		if validateDate != nil {
			if err := validateDate(group.header.issueDate); err != nil {
				result.RowsSkipped += group.rowCount
				result.Errors = append(result.Errors, ImportInvoicesRowError{
					Row:           group.firstRow,
					InvoiceNumber: group.header.invoiceNumber,
					Message:       err.Error(),
				})
				continue
			}
		}

		invoice, err := buildImportedInvoice(tenantID, req.UserID, contact.ID, group, now)
		if err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           group.firstRow,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       err.Error(),
			})
			continue
		}

		if err := s.repo.Create(ctx, schemaName, invoice); err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportInvoicesRowError{
				Row:           group.firstRow,
				InvoiceNumber: group.header.invoiceNumber,
				Message:       err.Error(),
			})
			continue
		}

		existingKeys[key] = struct{}{}
		result.InvoicesCreated++
		result.LinesImported += len(invoice.Lines)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseInvoiceImportRows(content string) ([]invoiceImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectInvoiceImportDelimiter(trimmed)
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
		"invoice_number":   false,
		"invoice_type":     false,
		"issue_date":       false,
		"due_date":         false,
		"line_description": false,
		"quantity":         false,
		"unit_price":       false,
		"vat_rate":         false,
	}
	hasContactColumn := false

	for i, header := range headers {
		canonical := canonicalInvoiceImportHeader(header)
		canonicalHeaders[i] = canonical
		if _, ok := required[canonical]; ok {
			required[canonical] = true
		}
		switch canonical {
		case "contact_code", "contact_reg_code", "contact_email", "contact_name":
			hasContactColumn = true
		}
	}

	for column, exists := range required {
		if !exists {
			return nil, fmt.Errorf("missing required %s column", column)
		}
	}
	if !hasContactColumn {
		return nil, fmt.Errorf("missing contact identifier column")
	}

	rows := make([]invoiceImportRow, 0)
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

		rows = append(rows, invoiceImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func parseInvoiceImportDataRow(row invoiceImportRow) (*invoiceImportParsedRow, error) {
	invoiceNumber := strings.TrimSpace(row.values["invoice_number"])
	if invoiceNumber == "" {
		return nil, fmt.Errorf("invoice_number is required")
	}

	invoiceType, err := parseInvoiceImportType(row.values["invoice_type"])
	if err != nil {
		return nil, err
	}

	contactRef := invoiceImportContactRef{
		code:    strings.TrimSpace(row.values["contact_code"]),
		regCode: strings.TrimSpace(row.values["contact_reg_code"]),
		email:   strings.TrimSpace(row.values["contact_email"]),
		name:    strings.TrimSpace(row.values["contact_name"]),
	}
	if contactRef.code == "" && contactRef.regCode == "" && contactRef.email == "" && contactRef.name == "" {
		return nil, fmt.Errorf("a contact identifier is required")
	}

	issueDate, err := parseInvoiceImportDate(row.values["issue_date"], "issue_date")
	if err != nil {
		return nil, err
	}
	dueDate, err := parseInvoiceImportDate(row.values["due_date"], "due_date")
	if err != nil {
		return nil, err
	}

	currency := strings.ToUpper(strings.TrimSpace(row.values["currency"]))
	if currency == "" {
		currency = "EUR"
	}

	exchangeRate := decimal.NewFromInt(1)
	if value := strings.TrimSpace(row.values["exchange_rate"]); value != "" {
		exchangeRate, err = decimal.NewFromString(normalizeInvoiceImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid exchange_rate")
		}
		if exchangeRate.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("exchange_rate must be greater than zero")
		}
	}

	explicitStatus := InvoiceStatus("")
	if value := strings.TrimSpace(row.values["status"]); value != "" {
		explicitStatus, err = parseInvoiceImportStatus(value)
		if err != nil {
			return nil, err
		}
	}

	amountPaid := decimal.Zero
	amountPaidSpecified := false
	if value := strings.TrimSpace(row.values["amount_paid"]); value != "" {
		amountPaid, err = decimal.NewFromString(normalizeInvoiceImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid amount_paid")
		}
		if amountPaid.IsNegative() {
			return nil, fmt.Errorf("amount_paid cannot be negative")
		}
		amountPaidSpecified = true
	}

	description := strings.TrimSpace(row.values["line_description"])
	if description == "" {
		return nil, fmt.Errorf("line_description is required")
	}

	quantity, err := parseInvoiceImportDecimal(row.values["quantity"], "quantity")
	if err != nil {
		return nil, err
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}

	unitPrice, err := parseInvoiceImportDecimal(row.values["unit_price"], "unit_price")
	if err != nil {
		return nil, err
	}
	if unitPrice.IsNegative() {
		return nil, fmt.Errorf("unit_price cannot be negative")
	}

	discountPercent := decimal.Zero
	if value := strings.TrimSpace(row.values["discount_percent"]); value != "" {
		discountPercent, err = decimal.NewFromString(normalizeInvoiceImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid discount_percent")
		}
		if discountPercent.IsNegative() || discountPercent.GreaterThan(decimal.NewFromInt(100)) {
			return nil, fmt.Errorf("discount_percent must be between 0 and 100")
		}
	}

	vatRate, err := parseInvoiceImportDecimal(row.values["vat_rate"], "vat_rate")
	if err != nil {
		return nil, err
	}
	if vatRate.IsNegative() {
		return nil, fmt.Errorf("vat_rate cannot be negative")
	}

	return &invoiceImportParsedRow{
		header: invoiceImportHeader{
			invoiceNumber:       invoiceNumber,
			invoiceType:         invoiceType,
			contactRef:          contactRef,
			issueDate:           issueDate,
			dueDate:             dueDate,
			currency:            currency,
			exchangeRate:        exchangeRate,
			reference:           strings.TrimSpace(row.values["reference"]),
			notes:               strings.TrimSpace(row.values["notes"]),
			explicitStatus:      explicitStatus,
			amountPaid:          amountPaid,
			amountPaidSpecified: amountPaidSpecified,
		},
		line: invoiceImportLine{
			description:     description,
			quantity:        quantity,
			unit:            strings.TrimSpace(row.values["unit"]),
			unitPrice:       unitPrice,
			discountPercent: discountPercent,
			vatRate:         vatRate,
		},
	}, nil
}

func mergeInvoiceImportGroup(group *invoiceImportGroup, next invoiceImportHeader, rowNumber int) string {
	if group.header.invoiceType != next.invoiceType {
		return "invoice_type must be consistent for each invoice_number"
	}
	if !normalizeInvoiceImportDate(group.header.issueDate).Equal(normalizeInvoiceImportDate(next.issueDate)) {
		return "issue_date must be consistent for each invoice_number"
	}
	if !normalizeInvoiceImportDate(group.header.dueDate).Equal(normalizeInvoiceImportDate(next.dueDate)) {
		return "due_date must be consistent for each invoice_number"
	}
	if group.header.currency != next.currency {
		return "currency must be consistent for each invoice_number"
	}
	if !group.header.exchangeRate.Equal(next.exchangeRate) {
		return "exchange_rate must be consistent for each invoice_number"
	}
	if conflict := mergeInvoiceImportContactRef(&group.header.contactRef, next.contactRef); conflict != "" {
		return conflict
	}
	if conflict := mergeInvoiceImportOptionalString(&group.header.reference, next.reference, "reference"); conflict != "" {
		return conflict
	}
	if conflict := mergeInvoiceImportOptionalString(&group.header.notes, next.notes, "notes"); conflict != "" {
		return conflict
	}
	if next.explicitStatus != "" {
		if group.header.explicitStatus == "" {
			group.header.explicitStatus = next.explicitStatus
		} else if group.header.explicitStatus != next.explicitStatus {
			return "status must be consistent for each invoice_number"
		}
	}
	if next.amountPaidSpecified {
		if !group.header.amountPaidSpecified {
			group.header.amountPaidSpecified = true
			group.header.amountPaid = next.amountPaid
		} else if !group.header.amountPaid.Equal(next.amountPaid) {
			return fmt.Sprintf("amount_paid must be consistent for each invoice_number (row %d)", rowNumber)
		}
	}

	return ""
}

func mergeInvoiceImportContactRef(target *invoiceImportContactRef, next invoiceImportContactRef) string {
	if conflict := mergeInvoiceImportOptionalString(&target.code, next.code, "contact_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeInvoiceImportOptionalString(&target.regCode, next.regCode, "contact_reg_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeInvoiceImportOptionalString(&target.email, next.email, "contact_email"); conflict != "" {
		return conflict
	}
	if conflict := mergeInvoiceImportOptionalString(&target.name, next.name, "contact_name"); conflict != "" {
		return conflict
	}
	return ""
}

func mergeInvoiceImportOptionalString(target *string, next, field string) string {
	if strings.TrimSpace(next) == "" {
		return ""
	}
	if strings.TrimSpace(*target) == "" {
		*target = strings.TrimSpace(next)
		return ""
	}
	if strings.TrimSpace(*target) != strings.TrimSpace(next) {
		return fmt.Sprintf("%s must be consistent for each invoice_number", field)
	}
	return ""
}

func buildImportedInvoice(
	tenantID, userID, contactID string,
	group *invoiceImportGroup,
	now time.Time,
) (*Invoice, error) {
	invoice := &Invoice{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		InvoiceNumber: group.header.invoiceNumber,
		InvoiceType:   group.header.invoiceType,
		ContactID:     contactID,
		IssueDate:     group.header.issueDate,
		DueDate:       group.header.dueDate,
		Currency:      group.header.currency,
		ExchangeRate:  group.header.exchangeRate,
		Reference:     group.header.reference,
		Notes:         group.header.notes,
		CreatedAt:     time.Now(),
		CreatedBy:     userID,
		UpdatedAt:     time.Now(),
	}

	for index, line := range group.lines {
		invoiceLine := InvoiceLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			InvoiceID:       invoice.ID,
			LineNumber:      index + 1,
			Description:     line.description,
			Quantity:        line.quantity,
			Unit:            line.unit,
			UnitPrice:       line.unitPrice,
			DiscountPercent: line.discountPercent,
			VATRate:         line.vatRate,
		}
		invoiceLine.Calculate()
		invoice.Lines = append(invoice.Lines, invoiceLine)
	}

	invoice.Calculate()

	status, amountPaid, err := deriveInvoiceImportStatus(
		group.header.explicitStatus,
		group.header.amountPaid,
		group.header.amountPaidSpecified,
		invoice.Total,
		group.header.dueDate,
		now,
	)
	if err != nil {
		return nil, err
	}

	invoice.Status = status
	invoice.AmountPaid = amountPaid

	if err := invoice.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return invoice, nil
}

func deriveInvoiceImportStatus(
	explicitStatus InvoiceStatus,
	amountPaid decimal.Decimal,
	amountPaidSpecified bool,
	total decimal.Decimal,
	dueDate time.Time,
	now time.Time,
) (InvoiceStatus, decimal.Decimal, error) {
	if amountPaid.GreaterThan(total) {
		return "", decimal.Zero, fmt.Errorf("amount_paid cannot exceed invoice total")
	}

	if explicitStatus != "" {
		switch explicitStatus {
		case StatusDraft, StatusSent, StatusOverdue, StatusVoided:
			if !amountPaid.IsZero() {
				return "", decimal.Zero, fmt.Errorf("amount_paid must be zero when status is %s", explicitStatus)
			}
			return explicitStatus, decimal.Zero, nil
		case StatusPartiallyPaid:
			if !amountPaidSpecified || amountPaid.LessThanOrEqual(decimal.Zero) || !amountPaid.LessThan(total) {
				return "", decimal.Zero, fmt.Errorf("amount_paid must be greater than zero and less than total when status is PARTIALLY_PAID")
			}
			return explicitStatus, amountPaid, nil
		case StatusPaid:
			if !amountPaidSpecified {
				amountPaid = total
			}
			if !amountPaid.Equal(total) {
				return "", decimal.Zero, fmt.Errorf("amount_paid must equal total when status is PAID")
			}
			return explicitStatus, amountPaid, nil
		default:
			return "", decimal.Zero, fmt.Errorf("invalid status %q", explicitStatus)
		}
	}

	if amountPaidSpecified {
		if amountPaid.Equal(total) {
			return StatusPaid, total, nil
		}
		if amountPaid.GreaterThan(decimal.Zero) {
			return StatusPartiallyPaid, amountPaid, nil
		}
	}

	if normalizeInvoiceImportDate(dueDate).Before(now) {
		return StatusOverdue, decimal.Zero, nil
	}

	return StatusSent, decimal.Zero, nil
}

func buildInvoiceImportContactLookup(existingContacts []contacts.Contact) *invoiceImportContactLookup {
	lookup := &invoiceImportContactLookup{
		byCode:    make(map[string]contacts.Contact),
		byRegCode: make(map[string]contacts.Contact),
		byEmail:   make(map[string]contacts.Contact),
		byName:    make(map[string]contacts.Contact),
	}

	for _, contact := range existingContacts {
		if key := normalizedInvoiceImportKey(contact.Code); key != "" {
			lookup.byCode[key] = contact
		}
		if key := normalizedInvoiceImportKey(contact.RegCode); key != "" {
			lookup.byRegCode[key] = contact
		}
		if key := normalizedInvoiceImportKey(contact.Email); key != "" {
			lookup.byEmail[key] = contact
		}
		if key := normalizedInvoiceImportKey(contact.Name); key != "" {
			lookup.byName[key] = contact
		}
	}

	return lookup
}

func (l *invoiceImportContactLookup) find(ref invoiceImportContactRef) (*contacts.Contact, error) {
	if key := normalizedInvoiceImportKey(ref.code); key != "" {
		if contact, ok := l.byCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_code %q was not found", ref.code)
	}
	if key := normalizedInvoiceImportKey(ref.regCode); key != "" {
		if contact, ok := l.byRegCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_reg_code %q was not found", ref.regCode)
	}
	if key := normalizedInvoiceImportKey(ref.email); key != "" {
		if contact, ok := l.byEmail[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_email %q was not found", ref.email)
	}
	if key := normalizedInvoiceImportKey(ref.name); key != "" {
		if contact, ok := l.byName[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_name %q was not found", ref.name)
	}
	return nil, fmt.Errorf("a contact identifier is required")
}

func parseInvoiceImportType(raw string) (InvoiceType, error) {
	normalized := normalizedInvoiceImportKey(raw)
	if normalized == "" {
		return "", fmt.Errorf("invoice_type is required")
	}
	if invoiceType, ok := invoiceImportTypeAliases[normalized]; ok {
		return invoiceType, nil
	}

	candidate := InvoiceType(strings.ToUpper(strings.TrimSpace(raw)))
	switch candidate {
	case InvoiceTypeSales, InvoiceTypePurchase, InvoiceTypeCreditNote:
		return candidate, nil
	default:
		return "", fmt.Errorf("invalid invoice_type %q", raw)
	}
}

func parseInvoiceImportStatus(raw string) (InvoiceStatus, error) {
	normalized := normalizedInvoiceImportKey(raw)
	if normalized == "" {
		return "", nil
	}
	if status, ok := invoiceImportStatusAliases[normalized]; ok {
		return status, nil
	}

	candidate := InvoiceStatus(strings.ToUpper(strings.TrimSpace(raw)))
	switch candidate {
	case StatusDraft, StatusSent, StatusPartiallyPaid, StatusPaid, StatusOverdue, StatusVoided:
		return candidate, nil
	default:
		return "", fmt.Errorf("invalid status %q", raw)
	}
}

func parseInvoiceImportDate(value, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD", field)
	}
	return normalizeInvoiceImportDate(parsed), nil
}

func parseInvoiceImportDecimal(value, field string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(normalizeInvoiceImportDecimal(value))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

func canonicalInvoiceImportHeader(value string) string {
	normalized := normalizedInvoiceImportKey(value)
	if canonical, ok := invoiceImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return ""
}

func detectInvoiceImportDelimiter(content string) rune {
	switch {
	case strings.Contains(content, "\t"):
		return '\t'
	case strings.Contains(content, ";"):
		return ';'
	default:
		return ','
	}
}

func normalizedInvoiceImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizedInvoiceImportGroupKey(invoiceNumber string, invoiceType InvoiceType) string {
	return fmt.Sprintf("%s|%s", normalizedInvoiceImportKey(invoiceNumber), invoiceType)
}

func normalizeInvoiceImportDecimal(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	return normalized
}

func normalizeInvoiceImportDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}
