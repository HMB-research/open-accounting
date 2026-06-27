package orders

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

type orderImportRow struct {
	rowNumber int
	values    map[string]string
}

type orderImportContactRef struct {
	id        string
	code      string
	regCode   string
	vatNumber string
	email     string
	name      string
}

type orderImportLine struct {
	description     string
	quantity        decimal.Decimal
	unit            string
	unitPrice       decimal.Decimal
	discountPercent decimal.Decimal
	vatRate         decimal.Decimal
	productID       *string
}

type orderImportHeader struct {
	orderNumber      string
	contactRef       orderImportContactRef
	orderDate        time.Time
	expectedDelivery *time.Time
	currency         string
	exchangeRate     decimal.Decimal
	notes            string
	quoteID          *string
	quoteNumber      string
	explicitStatus   OrderStatus
}

type orderImportParsedRow struct {
	header orderImportHeader
	line   orderImportLine
}

type orderImportGroup struct {
	header       orderImportHeader
	lines        []orderImportLine
	rowCount     int
	firstRow     int
	conflictRow  int
	conflictText string
}

type orderImportContactLookup struct {
	byID        map[string]contacts.Contact
	byCode      map[string]contacts.Contact
	byRegCode   map[string]contacts.Contact
	byVATNumber map[string]contacts.Contact
	byEmail     map[string]contacts.Contact
	byName      map[string]contacts.Contact
}

type orderImportQuoteLookup struct {
	byNumber         map[string]string
	duplicateNumbers map[string]bool
}

var orderImportHeaderAliases = map[string]string{
	"order_number":       "order_number",
	"sales_order":        "order_number",
	"sales_order_no":     "order_number",
	"number":             "order_number",
	"order_no":           "order_number",
	"order_no.":          "order_number",
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
	"order_date":         "order_date",
	"date":               "order_date",
	"expected_delivery":  "expected_delivery",
	"delivery_date":      "expected_delivery",
	"currency":           "currency",
	"exchange_rate":      "exchange_rate",
	"notes":              "notes",
	"quote_id":           "quote_id",
	"quote_number":       "quote_number",
	"quotation_number":   "quote_number",
	"offer_number":       "quote_number",
	"quote_no":           "quote_number",
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

var orderImportStatusAliases = map[string]OrderStatus{
	"pending":    OrderStatusPending,
	"open":       OrderStatusPending,
	"confirmed":  OrderStatusConfirmed,
	"processing": OrderStatusProcessing,
	"shipped":    OrderStatusShipped,
	"delivered":  OrderStatusDelivered,
	"canceled":   OrderStatusCanceled,
}

// ImportCSV imports historical orders from grouped CSV rows. Each row represents one order line.
func (s *Service) ImportCSV(
	ctx context.Context,
	tenantID, schemaName string,
	existingContacts []contacts.Contact,
	existingProducts []inventory.Product,
	req *ImportOrdersRequest,
) (*ImportOrdersResult, error) {
	return s.ImportCSVWithQuoteReferences(ctx, tenantID, schemaName, existingContacts, existingProducts, nil, req)
}

// ImportCSVWithQuoteReferences imports historical orders and resolves quote_number values against known quotes.
func (s *Service) ImportCSVWithQuoteReferences(
	ctx context.Context,
	tenantID, schemaName string,
	existingContacts []contacts.Contact,
	existingProducts []inventory.Product,
	existingQuotes []ImportQuoteReference,
	req *ImportOrdersRequest,
) (*ImportOrdersResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseOrderImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no orders found in CSV")
	}

	existingOrders, err := s.repo.List(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing orders: %w", err)
	}

	existingKeys := make(map[string]struct{}, len(existingOrders))
	for _, order := range existingOrders {
		if key := normalizedOrderImportKey(order.OrderNumber); key != "" {
			existingKeys[key] = struct{}{}
		}
	}

	result := &ImportOrdersResult{
		FileName: req.FileName,
		Errors:   []ImportOrdersRowError{},
	}
	contactLookup := buildOrderImportContactLookup(existingContacts)
	productLookup := importrefs.NewProductLookup(existingProducts)
	quoteLookup := buildOrderImportQuoteLookup(existingQuotes)
	groupOrder := make([]string, 0)
	groups := make(map[string]*orderImportGroup)

	for _, row := range rows {
		result.RowsProcessed++

		parsed, err := parseOrderImportDataRow(row, productLookup)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportOrdersRowError{
				Row:         row.rowNumber,
				OrderNumber: strings.TrimSpace(row.values["order_number"]),
				Message:     err.Error(),
			})
			continue
		}

		key := normalizedOrderImportKey(parsed.header.orderNumber)
		group, ok := groups[key]
		if !ok {
			group = &orderImportGroup{
				header:   parsed.header,
				firstRow: row.rowNumber,
			}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		} else if conflict := mergeOrderImportGroup(group, parsed.header, row.rowNumber); conflict != "" && group.conflictText == "" {
			group.conflictRow = row.rowNumber
			group.conflictText = conflict
		}

		group.lines = append(group.lines, parsed.line)
		group.rowCount++
	}

	for _, key := range groupOrder {
		group := groups[key]
		if group.conflictText != "" {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportOrdersRowError{
				Row:         group.conflictRow,
				OrderNumber: group.header.orderNumber,
				Message:     group.conflictText,
			})
			continue
		}
		if _, exists := existingKeys[key]; exists {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportOrdersRowError{
				Row:         group.firstRow,
				OrderNumber: group.header.orderNumber,
				Message:     fmt.Sprintf("order_number %q already exists", group.header.orderNumber),
			})
			continue
		}

		contact, err := contactLookup.find(group.header.contactRef)
		if err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportOrdersRowError{
				Row:         group.firstRow,
				OrderNumber: group.header.orderNumber,
				Message:     err.Error(),
			})
			continue
		}

		if err := quoteLookup.resolve(&group.header); err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportOrdersRowError{
				Row:         group.firstRow,
				OrderNumber: group.header.orderNumber,
				Message:     err.Error(),
			})
			continue
		}

		order, err := buildImportedOrder(tenantID, req.UserID, contact.ID, group)
		if err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportOrdersRowError{
				Row:         group.firstRow,
				OrderNumber: group.header.orderNumber,
				Message:     err.Error(),
			})
			continue
		}

		if err := s.repo.Create(ctx, schemaName, order); err != nil {
			result.RowsSkipped += group.rowCount
			result.Errors = append(result.Errors, ImportOrdersRowError{
				Row:         group.firstRow,
				OrderNumber: group.header.orderNumber,
				Message:     err.Error(),
			})
			continue
		}

		existingKeys[key] = struct{}{}
		result.OrdersCreated++
		result.LinesImported += len(order.Lines)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	return result, nil
}

func parseOrderImportRows(content string) ([]orderImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectOrderImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	required := map[string]bool{
		"order_number":     false,
		"order_date":       false,
		"line_description": false,
		"quantity":         false,
		"unit_price":       false,
		"vat_rate":         false,
	}
	hasContactColumn := false

	for i, header := range headers {
		canonical := canonicalOrderImportHeader(header)
		canonicalHeaders[i] = canonical
		if _, ok := required[canonical]; ok {
			required[canonical] = true
		}
		switch canonical {
		case "contact_id", "contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name":
			hasContactColumn = true
		}
	}

	for _, column := range []string{"order_number", "order_date", "line_description", "quantity", "unit_price", "vat_rate"} {
		if !required[column] {
			return nil, fmt.Errorf("missing required %s column", column)
		}
	}
	if !hasContactColumn {
		return nil, fmt.Errorf("missing contact identifier column")
	}

	rows := make([]orderImportRow, 0)
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
		rows = append(rows, orderImportRow{
			rowNumber: rowNumber,
			values:    values,
		})
	}

	return rows, nil
}

func parseOrderImportDataRow(row orderImportRow, productLookup importrefs.ProductLookup) (*orderImportParsedRow, error) {
	orderNumber := strings.TrimSpace(row.values["order_number"])
	if orderNumber == "" {
		return nil, fmt.Errorf("order_number is required")
	}

	contactID, err := parseOptionalOrderImportUUID("contact_id", row.values["contact_id"])
	if err != nil {
		return nil, err
	}
	contactRef := orderImportContactRef{
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

	orderDate, err := parseOrderImportDate(row.values["order_date"], "order_date")
	if err != nil {
		return nil, err
	}

	var expectedDelivery *time.Time
	if value := strings.TrimSpace(row.values["expected_delivery"]); value != "" {
		parsed, err := parseOrderImportDate(value, "expected_delivery")
		if err != nil {
			return nil, err
		}
		expectedDelivery = &parsed
	}

	currency := strings.ToUpper(strings.TrimSpace(row.values["currency"]))
	if currency == "" {
		currency = "EUR"
	}

	exchangeRate := decimal.NewFromInt(1)
	if value := strings.TrimSpace(row.values["exchange_rate"]); value != "" {
		exchangeRate, err = decimal.NewFromString(normalizeOrderImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid exchange_rate")
		}
		if exchangeRate.LessThanOrEqual(decimal.Zero) {
			return nil, fmt.Errorf("exchange_rate must be greater than zero")
		}
	}

	explicitStatus := OrderStatus("")
	if value := strings.TrimSpace(row.values["status"]); value != "" {
		explicitStatus, err = parseOrderImportStatus(value)
		if err != nil {
			return nil, err
		}
	}

	var quoteID *string
	if value := strings.TrimSpace(row.values["quote_id"]); value != "" {
		parsedID, err := uuid.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("quote_id must be a valid UUID")
		}
		canonicalID := parsedID.String()
		quoteID = &canonicalID
	}
	quoteNumber := strings.TrimSpace(row.values["quote_number"])

	description := strings.TrimSpace(row.values["line_description"])
	if description == "" {
		return nil, fmt.Errorf("line_description is required")
	}

	quantity, err := parseOrderImportDecimal(row.values["quantity"], "quantity")
	if err != nil {
		return nil, err
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("quantity must be greater than zero")
	}

	unitPrice, err := parseOrderImportDecimal(row.values["unit_price"], "unit_price")
	if err != nil {
		return nil, err
	}
	if unitPrice.IsNegative() {
		return nil, fmt.Errorf("unit_price cannot be negative")
	}

	discountPercent := decimal.Zero
	if value := strings.TrimSpace(row.values["discount_percent"]); value != "" {
		discountPercent, err = decimal.NewFromString(normalizeOrderImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid discount_percent")
		}
		if discountPercent.IsNegative() || discountPercent.GreaterThan(decimal.NewFromInt(100)) {
			return nil, fmt.Errorf("discount_percent must be between 0 and 100")
		}
	}

	vatRate, err := parseOrderImportDecimal(row.values["vat_rate"], "vat_rate")
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

	return &orderImportParsedRow{
		header: orderImportHeader{
			orderNumber:      orderNumber,
			contactRef:       contactRef,
			orderDate:        orderDate,
			expectedDelivery: expectedDelivery,
			currency:         currency,
			exchangeRate:     exchangeRate,
			notes:            strings.TrimSpace(row.values["notes"]),
			quoteID:          quoteID,
			quoteNumber:      quoteNumber,
			explicitStatus:   explicitStatus,
		},
		line: orderImportLine{
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

func mergeOrderImportGroup(group *orderImportGroup, next orderImportHeader, rowNumber int) string {
	if !normalizeOrderImportDate(group.header.orderDate).Equal(normalizeOrderImportDate(next.orderDate)) {
		return "order_date must be consistent for each order_number"
	}
	if conflict := mergeOrderImportOptionalDate(&group.header.expectedDelivery, next.expectedDelivery, "expected_delivery"); conflict != "" {
		return conflict
	}
	if group.header.currency != next.currency {
		return "currency must be consistent for each order_number"
	}
	if !group.header.exchangeRate.Equal(next.exchangeRate) {
		return "exchange_rate must be consistent for each order_number"
	}
	if conflict := mergeOrderImportContactRef(&group.header.contactRef, next.contactRef); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalString(&group.header.notes, next.notes, "notes"); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalStringPtr(&group.header.quoteID, next.quoteID, "quote_id"); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalString(&group.header.quoteNumber, next.quoteNumber, "quote_number"); conflict != "" {
		return conflict
	}
	if next.explicitStatus != "" {
		if group.header.explicitStatus == "" {
			group.header.explicitStatus = next.explicitStatus
		} else if group.header.explicitStatus != next.explicitStatus {
			return fmt.Sprintf("status must be consistent for each order_number (row %d)", rowNumber)
		}
	}
	return ""
}

func mergeOrderImportContactRef(target *orderImportContactRef, next orderImportContactRef) string {
	if conflict := mergeOrderImportOptionalString(&target.id, next.id, "contact_id"); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalString(&target.code, next.code, "contact_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalString(&target.regCode, next.regCode, "contact_reg_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalString(&target.vatNumber, next.vatNumber, "contact_vat_number"); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalString(&target.email, next.email, "contact_email"); conflict != "" {
		return conflict
	}
	if conflict := mergeOrderImportOptionalString(&target.name, next.name, "contact_name"); conflict != "" {
		return conflict
	}
	return ""
}

func mergeOrderImportOptionalString(target *string, next, field string) string {
	if strings.TrimSpace(next) == "" {
		return ""
	}
	if strings.TrimSpace(*target) == "" {
		*target = strings.TrimSpace(next)
		return ""
	}
	if strings.TrimSpace(*target) != strings.TrimSpace(next) {
		return fmt.Sprintf("%s must be consistent for each order_number", field)
	}
	return ""
}

func mergeOrderImportOptionalStringPtr(target **string, next *string, field string) string {
	if next == nil || strings.TrimSpace(*next) == "" {
		return ""
	}
	if *target == nil || strings.TrimSpace(**target) == "" {
		value := strings.TrimSpace(*next)
		*target = &value
		return ""
	}
	if strings.TrimSpace(**target) != strings.TrimSpace(*next) {
		return fmt.Sprintf("%s must be consistent for each order_number", field)
	}
	return ""
}

func mergeOrderImportOptionalDate(target **time.Time, next *time.Time, field string) string {
	if next == nil {
		return ""
	}
	if *target == nil {
		value := *next
		*target = &value
		return ""
	}
	if !normalizeOrderImportDate(**target).Equal(normalizeOrderImportDate(*next)) {
		return fmt.Sprintf("%s must be consistent for each order_number", field)
	}
	return ""
}

func buildImportedOrder(tenantID, userID, contactID string, group *orderImportGroup) (*Order, error) {
	order := &Order{
		ID:               uuid.New().String(),
		TenantID:         tenantID,
		OrderNumber:      group.header.orderNumber,
		ContactID:        contactID,
		OrderDate:        group.header.orderDate,
		ExpectedDelivery: group.header.expectedDelivery,
		Currency:         group.header.currency,
		ExchangeRate:     group.header.exchangeRate,
		Status:           deriveOrderImportStatus(group.header.explicitStatus),
		Notes:            group.header.notes,
		QuoteID:          group.header.quoteID,
		CreatedAt:        time.Now(),
		CreatedBy:        userID,
		UpdatedAt:        time.Now(),
	}

	for index, line := range group.lines {
		orderLine := OrderLine{
			ID:              uuid.New().String(),
			TenantID:        tenantID,
			OrderID:         order.ID,
			LineNumber:      index + 1,
			Description:     line.description,
			Quantity:        line.quantity,
			Unit:            line.unit,
			UnitPrice:       line.unitPrice,
			DiscountPercent: line.discountPercent,
			VATRate:         line.vatRate,
			ProductID:       line.productID,
		}
		orderLine.Calculate()
		order.Lines = append(order.Lines, orderLine)
	}

	order.Calculate()
	if err := order.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return order, nil
}

func deriveOrderImportStatus(explicitStatus OrderStatus) OrderStatus {
	if explicitStatus != "" {
		return explicitStatus
	}
	return OrderStatusPending
}

func buildOrderImportContactLookup(existingContacts []contacts.Contact) *orderImportContactLookup {
	lookup := &orderImportContactLookup{
		byID:        make(map[string]contacts.Contact),
		byCode:      make(map[string]contacts.Contact),
		byRegCode:   make(map[string]contacts.Contact),
		byVATNumber: make(map[string]contacts.Contact),
		byEmail:     make(map[string]contacts.Contact),
		byName:      make(map[string]contacts.Contact),
	}
	for _, contact := range existingContacts {
		if key := normalizedOrderImportKey(contact.ID); key != "" {
			lookup.byID[key] = contact
		}
		if key := normalizedOrderImportKey(contact.Code); key != "" {
			lookup.byCode[key] = contact
		}
		if key := normalizedOrderImportKey(contact.RegCode); key != "" {
			lookup.byRegCode[key] = contact
		}
		if key := normalizedOrderImportKey(contact.VATNumber); key != "" {
			lookup.byVATNumber[key] = contact
		}
		if key := normalizedOrderImportKey(contact.Email); key != "" {
			lookup.byEmail[key] = contact
		}
		if key := normalizedOrderImportKey(contact.Name); key != "" {
			lookup.byName[key] = contact
		}
	}
	return lookup
}

func buildOrderImportQuoteLookup(existingQuotes []ImportQuoteReference) orderImportQuoteLookup {
	lookup := orderImportQuoteLookup{
		byNumber:         make(map[string]string),
		duplicateNumbers: make(map[string]bool),
	}
	for _, quote := range existingQuotes {
		id := strings.TrimSpace(quote.ID)
		if id == "" {
			continue
		}
		if key := normalizedOrderImportKey(quote.QuoteNumber); key != "" {
			if _, exists := lookup.byNumber[key]; exists {
				lookup.duplicateNumbers[key] = true
				continue
			}
			lookup.byNumber[key] = id
		}
	}
	return lookup
}

func (l orderImportQuoteLookup) resolve(header *orderImportHeader) error {
	if header.quoteID != nil || strings.TrimSpace(header.quoteNumber) == "" {
		return nil
	}
	key := normalizedOrderImportKey(header.quoteNumber)
	if l.duplicateNumbers[key] {
		return fmt.Errorf("quote_number %q is ambiguous", header.quoteNumber)
	}
	quoteID, ok := l.byNumber[key]
	if !ok {
		return fmt.Errorf("quote_number %q was not found", header.quoteNumber)
	}
	header.quoteID = &quoteID
	return nil
}

func (l *orderImportContactLookup) find(ref orderImportContactRef) (*contacts.Contact, error) {
	if key := normalizedOrderImportKey(ref.id); key != "" {
		if contact, ok := l.byID[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_id %q was not found", ref.id)
	}
	if key := normalizedOrderImportKey(ref.code); key != "" {
		if contact, ok := l.byCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_code %q was not found", ref.code)
	}
	if key := normalizedOrderImportKey(ref.regCode); key != "" {
		if contact, ok := l.byRegCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_reg_code %q was not found", ref.regCode)
	}
	if key := normalizedOrderImportKey(ref.vatNumber); key != "" {
		if contact, ok := l.byVATNumber[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_vat_number %q was not found", ref.vatNumber)
	}
	if key := normalizedOrderImportKey(ref.email); key != "" {
		if contact, ok := l.byEmail[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_email %q was not found", ref.email)
	}
	if key := normalizedOrderImportKey(ref.name); key != "" {
		if contact, ok := l.byName[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_name %q was not found", ref.name)
	}
	return nil, fmt.Errorf("a contact identifier is required")
}

func parseOrderImportStatus(raw string) (OrderStatus, error) {
	normalized := normalizedOrderImportKey(raw)
	if normalized == "" {
		return "", nil
	}
	if status, ok := orderImportStatusAliases[normalized]; ok {
		return status, nil
	}

	candidate := OrderStatus(strings.ToUpper(strings.TrimSpace(raw)))
	switch candidate {
	case OrderStatusPending, OrderStatusConfirmed, OrderStatusProcessing, OrderStatusShipped, OrderStatusDelivered, OrderStatusCanceled:
		return candidate, nil
	default:
		return "", fmt.Errorf("invalid status %q", raw)
	}
}

func parseOrderImportDate(value, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD", field)
	}
	return normalizeOrderImportDate(parsed), nil
}

func parseOrderImportDecimal(value, field string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(normalizeOrderImportDecimal(value))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

func parseOptionalOrderImportUUID(field, value string) (string, error) {
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

func canonicalOrderImportHeader(value string) string {
	normalized := normalizedOrderImportKey(value)
	if canonical, ok := orderImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return ""
}

func detectOrderImportDelimiter(content string) rune {
	switch {
	case strings.Contains(content, "\t"):
		return '\t'
	case strings.Contains(content, ";"):
		return ';'
	default:
		return ','
	}
}

func normalizedOrderImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeOrderImportDecimal(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	return normalized
}

func normalizeOrderImportDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}
