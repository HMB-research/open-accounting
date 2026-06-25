package quotes

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
	"github.com/HMB-research/open-accounting/internal/importrefs"
	"github.com/HMB-research/open-accounting/internal/inventory"
)

type quoteImportRow struct {
	rowNumber int
	values    map[string]string
}

type quoteImportContactRef struct {
	id        string
	code      string
	regCode   string
	vatNumber string
	email     string
	name      string
}

type quoteImportLine struct {
	description     string
	quantity        decimal.Decimal
	unit            string
	unitPrice       decimal.Decimal
	discountPercent decimal.Decimal
	vatRate         decimal.Decimal
	productID       *string
}

type quoteImportHeader struct {
	id             string
	quoteNumber    string
	contactRef     quoteImportContactRef
	quoteDate      time.Time
	validUntil     *time.Time
	currency       string
	exchangeRate   decimal.Decimal
	notes          string
	explicitStatus QuoteStatus
}

type quoteImportParsedRow struct {
	header quoteImportHeader
	line   quoteImportLine
}

type quoteImportGroup struct {
	header       quoteImportHeader
	lines        []quoteImportLine
	rowCount     int
	firstRow     int
	conflictRow  int
	conflictText string
}

type quoteImportContactLookup struct {
	byID        map[string]contacts.Contact
	byCode      map[string]contacts.Contact
	byRegCode   map[string]contacts.Contact
	byVATNumber map[string]contacts.Contact
	byEmail     map[string]contacts.Contact
	byName      map[string]contacts.Contact
}

var quoteImportHeaderAliases = map[string]string{
	"id":                 "id",
	"quote_id":           "id",
	"quote_number":       "quote_number",
	"quotation_number":   "quote_number",
	"offer_number":       "quote_number",
	"number":             "quote_number",
	"quote_no":           "quote_number",
	"quote_no.":          "quote_number",
	"contact_id":         "contact_id",
	"customer_id":        "contact_id",
	"contact_code":       "contact_code",
	"customer_code":      "contact_code",
	"contact_reg_code":   "contact_reg_code",
	"contact_vat_number": "contact_vat_number",
	"vat_number":         "contact_vat_number",
	"contact_email":      "contact_email",
	"email":              "contact_email",
	"contact_name":       "contact_name",
	"customer_name":      "contact_name",
	"quote_date":         "quote_date",
	"date":               "quote_date",
	"valid_until":        "valid_until",
	"valid_to":           "valid_until",
	"expiry_date":        "valid_until",
	"expires_at":         "valid_until",
	"currency":           "currency",
	"exchange_rate":      "exchange_rate",
	"notes":              "notes",
	"status":             "status",
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
	"product_id":         "product_id",
	"product_code":       "product_code",
	"sku":                "product_code",
	"item_code":          "product_code",
}

var quoteImportStatusAliases = map[string]QuoteStatus{
	"draft":       QuoteStatusDraft,
	"mustand":     QuoteStatusDraft,
	"sent":        QuoteStatusSent,
	"issued":      QuoteStatusSent,
	"saadetud":    QuoteStatusSent,
	"accepted":    QuoteStatusAccepted,
	"approved":    QuoteStatusAccepted,
	"rejected":    QuoteStatusRejected,
	"declined":    QuoteStatusRejected,
	"expired":     QuoteStatusExpired,
	"converted":   QuoteStatusConverted,
	"convertedto": QuoteStatusConverted,
}

// ImportCSV imports historical quotes from grouped CSV rows. Each row represents one quote line.
func (s *Service) ImportCSV(
	ctx context.Context,
	tenantID, schemaName string,
	existingContacts []contacts.Contact,
	existingProducts []inventory.Product,
	req *ImportQuotesRequest,
) (*ImportQuotesResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseQuoteImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no quotes found in CSV")
	}

	existingQuotes, err := s.repo.List(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing quotes: %w", err)
	}

	existingKeys := make(map[string]struct{}, len(existingQuotes))
	existingIDs := make(map[string]struct{}, len(existingQuotes))
	for _, quote := range existingQuotes {
		if key := normalizedQuoteImportKey(quote.QuoteNumber); key != "" {
			existingKeys[key] = struct{}{}
		}
		if key := normalizedQuoteImportKey(quote.ID); key != "" {
			existingIDs[key] = struct{}{}
		}
	}

	result := &ImportQuotesResult{
		FileName: req.FileName,
		Errors:   []ImportQuotesRowError{},
	}

	contactLookup := buildQuoteImportContactLookup(existingContacts)
	productLookup := importrefs.NewProductLookup(existingProducts)
	groupOrder := make([]string, 0)
	groups := make(map[string]*quoteImportGroup)

	for _, row := range rows {
		result.RowsProcessed++

		parsed, err := parseQuoteImportDataRow(row, productLookup)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportQuotesRowError{
				Row:         row.rowNumber,
				QuoteNumber: strings.TrimSpace(row.values["quote_number"]),
				Message:     err.Error(),
			})
			continue
		}

		key := normalizedQuoteImportKey(parsed.header.quoteNumber)
		group, ok := groups[key]
		if !ok {
			group = &quoteImportGroup{
				header:   parsed.header,
				firstRow: row.rowNumber,
			}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		} else if conflict := mergeQuoteImportGroup(group, parsed.header, row.rowNumber); conflict != "" && group.conflictText == "" {
			group.conflictRow = row.rowNumber
			group.conflictText = conflict
		}

		group.lines = append(group.lines, parsed.line)
		group.rowCount++
	}

	now := normalizeQuoteImportDate(time.Now())
	for _, key := range groupOrder {
		group := groups[key]

		if group.conflictText != "" {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportQuotesRowError{
				Row:         group.conflictRow,
				QuoteNumber: group.header.quoteNumber,
				Message:     group.conflictText,
			})
			continue
		}

		if _, exists := existingKeys[key]; exists {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportQuotesRowError{
				Row:         group.firstRow,
				QuoteNumber: group.header.quoteNumber,
				Message:     fmt.Sprintf("quote_number %q already exists", group.header.quoteNumber),
			})
			continue
		}
		if idKey := normalizedQuoteImportKey(group.header.id); idKey != "" {
			if _, exists := existingIDs[idKey]; exists {
				result.RowsSkipped += group.rowCount
				result.Errors = append(result.Errors, ImportQuotesRowError{
					Row:         group.firstRow,
					QuoteNumber: group.header.quoteNumber,
					Message:     fmt.Sprintf("id %q already exists", group.header.id),
				})
				continue
			}
		}

		contact, err := contactLookup.find(group.header.contactRef)
		if err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportQuotesRowError{
				Row:         group.firstRow,
				QuoteNumber: group.header.quoteNumber,
				Message:     err.Error(),
			})
			continue
		}

		quote, err := buildImportedQuote(tenantID, req.UserID, contact.ID, group, now)
		if err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportQuotesRowError{
				Row:         group.firstRow,
				QuoteNumber: group.header.quoteNumber,
				Message:     err.Error(),
			})
			continue
		}

		if err := s.repo.Create(ctx, schemaName, quote); err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportQuotesRowError{
				Row:         group.firstRow,
				QuoteNumber: group.header.quoteNumber,
				Message:     err.Error(),
			})
			continue
		}

		existingKeys[key] = struct{}{}
		if idKey := normalizedQuoteImportKey(quote.ID); idKey != "" {
			existingIDs[idKey] = struct{}{}
		}
		result.QuotesCreated++
		result.LinesImported += len(quote.Lines)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseQuoteImportRows(content string) ([]quoteImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectQuoteImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	required := map[string]bool{
		"quote_number":     false,
		"quote_date":       false,
		"line_description": false,
		"quantity":         false,
		"unit_price":       false,
		"vat_rate":         false,
	}
	hasContactColumn := false

	for i, header := range headers {
		canonical := canonicalQuoteImportHeader(header)
		canonicalHeaders[i] = canonical
		if _, ok := required[canonical]; ok {
			required[canonical] = true
		}
		switch canonical {
		case "contact_id", "contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name":
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

	rows := make([]quoteImportRow, 0)
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

		rows = append(rows, quoteImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func parseQuoteImportDataRow(row quoteImportRow, productLookup importrefs.ProductLookup) (*quoteImportParsedRow, error) {
	quoteNumber := strings.TrimSpace(row.values["quote_number"])
	if quoteNumber == "" {
		return nil, fmt.Errorf("quote_number is required")
	}

	id := strings.TrimSpace(row.values["id"])
	if id != "" {
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("invalid id")
		}
		id = parsedID.String()
	}

	contactID, err := parseOptionalQuoteImportUUID("contact_id", row.values["contact_id"])
	if err != nil {
		return nil, err
	}
	contactRef := quoteImportContactRef{
		id:        contactID,
		code:      strings.TrimSpace(row.values["contact_code"]),
		regCode:   strings.TrimSpace(row.values["contact_reg_code"]),
		vatNumber: strings.TrimSpace(row.values["contact_vat_number"]),
		email:     strings.TrimSpace(row.values["contact_email"]),
		name:      strings.TrimSpace(row.values["contact_name"]),
	}
	if contactRef.id == "" && contactRef.code == "" && contactRef.regCode == "" && contactRef.vatNumber == "" && contactRef.email == "" && contactRef.name == "" {
		return nil, fmt.Errorf("a contact identifier is required")
	}

	quoteDate, err := parseQuoteImportDate(row.values["quote_date"], "quote_date")
	if err != nil {
		return nil, err
	}

	var validUntil *time.Time
	if value := strings.TrimSpace(row.values["valid_until"]); value != "" {
		parsed, err := parseQuoteImportDate(value, "valid_until")
		if err != nil {
			return nil, err
		}
		validUntil = &parsed
	}

	currency := strings.ToUpper(strings.TrimSpace(row.values["currency"]))
	if currency == "" {
		currency = "EUR"
	}

	exchangeRate := decimal.NewFromInt(1)
	if value := strings.TrimSpace(row.values["exchange_rate"]); value != "" {
		exchangeRate, err = decimal.NewFromString(normalizeQuoteImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid exchange_rate")
		}
		if exchangeRate.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("exchange_rate must be greater than zero")
		}
	}

	explicitStatus := QuoteStatus("")
	if value := strings.TrimSpace(row.values["status"]); value != "" {
		explicitStatus, err = parseQuoteImportStatus(value)
		if err != nil {
			return nil, err
		}
	}

	description := strings.TrimSpace(row.values["line_description"])
	if description == "" {
		return nil, fmt.Errorf("line_description is required")
	}

	quantity, err := parseQuoteImportDecimal(row.values["quantity"], "quantity")
	if err != nil {
		return nil, err
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}

	unitPrice, err := parseQuoteImportDecimal(row.values["unit_price"], "unit_price")
	if err != nil {
		return nil, err
	}
	if unitPrice.IsNegative() {
		return nil, fmt.Errorf("unit_price cannot be negative")
	}

	discountPercent := decimal.Zero
	if value := strings.TrimSpace(row.values["discount_percent"]); value != "" {
		discountPercent, err = decimal.NewFromString(normalizeQuoteImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid discount_percent")
		}
		if discountPercent.IsNegative() || discountPercent.GreaterThan(decimal.NewFromInt(100)) {
			return nil, fmt.Errorf("discount_percent must be between 0 and 100")
		}
	}

	vatRate, err := parseQuoteImportDecimal(row.values["vat_rate"], "vat_rate")
	if err != nil {
		return nil, err
	}
	if vatRate.IsNegative() {
		return nil, fmt.Errorf("vat_rate cannot be negative")
	}

	productID, err := productLookup.ResolveID(row.values["product_id"], row.values["product_code"])
	if err != nil {
		return nil, err
	}

	return &quoteImportParsedRow{
		header: quoteImportHeader{
			id:             id,
			quoteNumber:    quoteNumber,
			contactRef:     contactRef,
			quoteDate:      quoteDate,
			validUntil:     validUntil,
			currency:       currency,
			exchangeRate:   exchangeRate,
			notes:          strings.TrimSpace(row.values["notes"]),
			explicitStatus: explicitStatus,
		},
		line: quoteImportLine{
			description:     description,
			quantity:        quantity,
			unit:            strings.TrimSpace(row.values["unit"]),
			unitPrice:       unitPrice,
			discountPercent: discountPercent,
			vatRate:         vatRate,
			productID:       productID,
		},
	}, nil
}

func mergeQuoteImportGroup(group *quoteImportGroup, next quoteImportHeader, rowNumber int) string {
	if conflict := mergeQuoteImportOptionalString(&group.header.id, next.id, "id"); conflict != "" {
		return conflict
	}
	if !normalizeQuoteImportDate(group.header.quoteDate).Equal(normalizeQuoteImportDate(next.quoteDate)) {
		return "quote_date must be consistent for each quote_number"
	}
	if conflict := mergeQuoteImportOptionalDate(&group.header.validUntil, next.validUntil, "valid_until"); conflict != "" {
		return conflict
	}
	if group.header.currency != next.currency {
		return "currency must be consistent for each quote_number"
	}
	if !group.header.exchangeRate.Equal(next.exchangeRate) {
		return "exchange_rate must be consistent for each quote_number"
	}
	if conflict := mergeQuoteImportContactRef(&group.header.contactRef, next.contactRef); conflict != "" {
		return conflict
	}
	if conflict := mergeQuoteImportOptionalString(&group.header.notes, next.notes, "notes"); conflict != "" {
		return conflict
	}
	if next.explicitStatus != "" {
		if group.header.explicitStatus == "" {
			group.header.explicitStatus = next.explicitStatus
		} else if group.header.explicitStatus != next.explicitStatus {
			return fmt.Sprintf("status must be consistent for each quote_number (row %d)", rowNumber)
		}
	}

	return ""
}

func mergeQuoteImportContactRef(target *quoteImportContactRef, next quoteImportContactRef) string {
	if conflict := mergeQuoteImportOptionalString(&target.id, next.id, "contact_id"); conflict != "" {
		return conflict
	}
	if conflict := mergeQuoteImportOptionalString(&target.code, next.code, "contact_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeQuoteImportOptionalString(&target.regCode, next.regCode, "contact_reg_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeQuoteImportOptionalString(&target.vatNumber, next.vatNumber, "contact_vat_number"); conflict != "" {
		return conflict
	}
	if conflict := mergeQuoteImportOptionalString(&target.email, next.email, "contact_email"); conflict != "" {
		return conflict
	}
	if conflict := mergeQuoteImportOptionalString(&target.name, next.name, "contact_name"); conflict != "" {
		return conflict
	}
	return ""
}

func mergeQuoteImportOptionalString(target *string, next, field string) string {
	if strings.TrimSpace(next) == "" {
		return ""
	}
	if strings.TrimSpace(*target) == "" {
		*target = strings.TrimSpace(next)
		return ""
	}
	if strings.TrimSpace(*target) != strings.TrimSpace(next) {
		return fmt.Sprintf("%s must be consistent for each quote_number", field)
	}
	return ""
}

func mergeQuoteImportOptionalDate(target **time.Time, next *time.Time, field string) string {
	if next == nil {
		return ""
	}
	if *target == nil {
		value := *next
		*target = &value
		return ""
	}
	if !normalizeQuoteImportDate(**target).Equal(normalizeQuoteImportDate(*next)) {
		return fmt.Sprintf("%s must be consistent for each quote_number", field)
	}
	return ""
}

func buildImportedQuote(
	tenantID, userID, contactID string,
	group *quoteImportGroup,
	now time.Time,
) (*Quote, error) {
	quoteID := group.header.id
	if quoteID == "" {
		quoteID = uuid.New().String()
	}

	quote := &Quote{
		ID:           quoteID,
		TenantID:     tenantID,
		QuoteNumber:  group.header.quoteNumber,
		ContactID:    contactID,
		QuoteDate:    group.header.quoteDate,
		ValidUntil:   group.header.validUntil,
		Currency:     group.header.currency,
		ExchangeRate: group.header.exchangeRate,
		Status:       deriveQuoteImportStatus(group.header.explicitStatus, group.header.validUntil, now),
		Notes:        group.header.notes,
		CreatedAt:    time.Now(),
		CreatedBy:    userID,
		UpdatedAt:    time.Now(),
	}

	for index, line := range group.lines {
		quoteLine := QuoteLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			QuoteID:         quote.ID,
			LineNumber:      index + 1,
			Description:     line.description,
			Quantity:        line.quantity,
			Unit:            line.unit,
			UnitPrice:       line.unitPrice,
			DiscountPercent: line.discountPercent,
			VATRate:         line.vatRate,
			ProductID:       line.productID,
		}
		quoteLine.Calculate()
		quote.Lines = append(quote.Lines, quoteLine)
	}

	quote.Calculate()
	if err := quote.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return quote, nil
}

func deriveQuoteImportStatus(explicitStatus QuoteStatus, validUntil *time.Time, now time.Time) QuoteStatus {
	if explicitStatus != "" {
		return explicitStatus
	}
	if validUntil != nil && normalizeQuoteImportDate(*validUntil).Before(now) {
		return QuoteStatusExpired
	}
	return QuoteStatusDraft
}

func buildQuoteImportContactLookup(existingContacts []contacts.Contact) *quoteImportContactLookup {
	lookup := &quoteImportContactLookup{
		byID:        make(map[string]contacts.Contact),
		byCode:      make(map[string]contacts.Contact),
		byRegCode:   make(map[string]contacts.Contact),
		byVATNumber: make(map[string]contacts.Contact),
		byEmail:     make(map[string]contacts.Contact),
		byName:      make(map[string]contacts.Contact),
	}

	for _, contact := range existingContacts {
		if key := normalizedQuoteImportKey(contact.ID); key != "" {
			lookup.byID[key] = contact
		}
		if key := normalizedQuoteImportKey(contact.Code); key != "" {
			lookup.byCode[key] = contact
		}
		if key := normalizedQuoteImportKey(contact.RegCode); key != "" {
			lookup.byRegCode[key] = contact
		}
		if key := normalizedQuoteImportKey(contact.VATNumber); key != "" {
			lookup.byVATNumber[key] = contact
		}
		if key := normalizedQuoteImportKey(contact.Email); key != "" {
			lookup.byEmail[key] = contact
		}
		if key := normalizedQuoteImportKey(contact.Name); key != "" {
			lookup.byName[key] = contact
		}
	}

	return lookup
}

func (l *quoteImportContactLookup) find(ref quoteImportContactRef) (*contacts.Contact, error) {
	if key := normalizedQuoteImportKey(ref.id); key != "" {
		if contact, ok := l.byID[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_id %q was not found", ref.id)
	}
	if key := normalizedQuoteImportKey(ref.code); key != "" {
		if contact, ok := l.byCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_code %q was not found", ref.code)
	}
	if key := normalizedQuoteImportKey(ref.regCode); key != "" {
		if contact, ok := l.byRegCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_reg_code %q was not found", ref.regCode)
	}
	if key := normalizedQuoteImportKey(ref.vatNumber); key != "" {
		if contact, ok := l.byVATNumber[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_vat_number %q was not found", ref.vatNumber)
	}
	if key := normalizedQuoteImportKey(ref.email); key != "" {
		if contact, ok := l.byEmail[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_email %q was not found", ref.email)
	}
	if key := normalizedQuoteImportKey(ref.name); key != "" {
		if contact, ok := l.byName[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_name %q was not found", ref.name)
	}
	return nil, fmt.Errorf("a contact identifier is required")
}

func parseQuoteImportStatus(raw string) (QuoteStatus, error) {
	normalized := normalizedQuoteImportKey(raw)
	if normalized == "" {
		return "", nil
	}
	if status, ok := quoteImportStatusAliases[normalized]; ok {
		return status, nil
	}

	candidate := QuoteStatus(strings.ToUpper(strings.TrimSpace(raw)))
	switch candidate {
	case QuoteStatusDraft, QuoteStatusSent, QuoteStatusAccepted, QuoteStatusRejected, QuoteStatusExpired, QuoteStatusConverted:
		return candidate, nil
	default:
		return "", fmt.Errorf("invalid status %q", raw)
	}
}

func parseQuoteImportDate(value, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD", field)
	}
	return normalizeQuoteImportDate(parsed), nil
}

func parseQuoteImportDecimal(value, field string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(normalizeQuoteImportDecimal(value))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

func parseOptionalQuoteImportUUID(field, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%s must be a valid UUID", field)
	}
	return parsedID.String(), nil
}

func canonicalQuoteImportHeader(value string) string {
	normalized := normalizedQuoteImportKey(value)
	if canonical, ok := quoteImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return ""
}

func detectQuoteImportDelimiter(content string) rune {
	switch {
	case strings.Contains(content, "\t"):
		return '\t'
	case strings.Contains(content, ";"):
		return ';'
	default:
		return ','
	}
}

func normalizedQuoteImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeQuoteImportDecimal(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	return normalized
}

func normalizeQuoteImportDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}
