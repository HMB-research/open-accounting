package recurring

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/importrefs"
	"github.com/HMB-research/open-accounting/internal/inventory"
)

type recurringImportRow struct {
	rowNumber int
	values    map[string]string
}

type recurringImportContactRef struct {
	id        string
	code      string
	regCode   string
	vatNumber string
	email     string
	name      string
}

type recurringImportLine struct {
	description     string
	quantity        decimal.Decimal
	unit            string
	unitPrice       decimal.Decimal
	discountPercent decimal.Decimal
	vatRate         decimal.Decimal
	accountID       *string
	productID       *string
}

type recurringImportHeader struct {
	name                   string
	contactRef             recurringImportContactRef
	invoiceType            string
	currency               string
	frequency              Frequency
	startDate              time.Time
	endDate                *time.Time
	nextGenerationDate     time.Time
	paymentTermsDays       int
	reference              string
	notes                  string
	isActive               bool
	lastGeneratedAt        *time.Time
	generatedCount         int
	sendEmailOnGeneration  bool
	emailTemplateType      string
	recipientEmailOverride string
	attachPDFToEmail       bool
	emailSubjectOverride   string
	emailMessage           string
}

type recurringImportParsedRow struct {
	header recurringImportHeader
	line   recurringImportLine
}

type recurringImportGroup struct {
	header       recurringImportHeader
	lines        []recurringImportLine
	rowCount     int
	firstRow     int
	conflictRow  int
	conflictText string
}

type recurringImportContactLookup struct {
	byID        map[string]contacts.Contact
	byCode      map[string]contacts.Contact
	byRegCode   map[string]contacts.Contact
	byVATNumber map[string]contacts.Contact
	byEmail     map[string]contacts.Contact
	byName      map[string]contacts.Contact
}

var recurringImportHeaderAliases = map[string]string{
	"name":                     "name",
	"template":                 "name",
	"template_name":            "name",
	"recurring_name":           "name",
	"contact_id":               "contact_id",
	"customer_id":              "contact_id",
	"contact_code":             "contact_code",
	"customer_code":            "contact_code",
	"contact_reg_code":         "contact_reg_code",
	"contact_vat_number":       "contact_vat_number",
	"vat_number":               "contact_vat_number",
	"contact_email":            "contact_email",
	"email":                    "contact_email",
	"contact_name":             "contact_name",
	"customer_name":            "contact_name",
	"invoice_type":             "invoice_type",
	"type":                     "invoice_type",
	"currency":                 "currency",
	"frequency":                "frequency",
	"start_date":               "start_date",
	"end_date":                 "end_date",
	"next_generation_date":     "next_generation_date",
	"next_date":                "next_generation_date",
	"payment_terms_days":       "payment_terms_days",
	"payment_terms":            "payment_terms_days",
	"reference":                "reference",
	"notes":                    "notes",
	"is_active":                "is_active",
	"active":                   "is_active",
	"last_generated_at":        "last_generated_at",
	"generated_count":          "generated_count",
	"send_email_on_generation": "send_email_on_generation",
	"send_email":               "send_email_on_generation",
	"email_template_type":      "email_template_type",
	"recipient_email_override": "recipient_email_override",
	"recipient_email":          "recipient_email_override",
	"attach_pdf_to_email":      "attach_pdf_to_email",
	"attach_pdf":               "attach_pdf_to_email",
	"email_subject_override":   "email_subject_override",
	"email_subject":            "email_subject_override",
	"email_message":            "email_message",
	"line_description":         "line_description",
	"description":              "line_description",
	"quantity":                 "quantity",
	"qty":                      "quantity",
	"unit":                     "unit",
	"unit_price":               "unit_price",
	"price":                    "unit_price",
	"discount_percent":         "discount_percent",
	"discount":                 "discount_percent",
	"vat_rate":                 "vat_rate",
	"vat":                      "vat_rate",
	"account_id":               "account_id",
	"product_id":               "product_id",
	"product_code":             "product_code",
	"sku":                      "product_code",
	"item_code":                "product_code",
}

// ImportCSV imports recurring invoice templates from grouped CSV rows.
func (s *Service) ImportCSV(
	ctx context.Context,
	tenantID, schemaName string,
	existingContacts []contacts.Contact,
	existingProducts []inventory.Product,
	req *ImportRecurringInvoicesRequest,
) (*ImportRecurringInvoicesResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}
	if err := s.requireRepository(); err != nil {
		return nil, err
	}

	rows, err := parseRecurringImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no recurring invoices found in CSV")
	}

	existingTemplates, err := s.repo.List(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing recurring invoices: %w", err)
	}
	existingKeys := make(map[string]struct{}, len(existingTemplates))
	for _, template := range existingTemplates {
		if key := normalizedRecurringImportKey(template.Name); key != "" {
			existingKeys[key] = struct{}{}
		}
	}

	result := &ImportRecurringInvoicesResult{
		FileName: req.FileName,
		Errors:   []ImportRecurringInvoicesRowError{},
	}
	contactLookup := buildRecurringImportContactLookup(existingContacts)
	productLookup := importrefs.NewProductLookup(existingProducts)
	groupOrder := make([]string, 0)
	groups := make(map[string]*recurringImportGroup)

	for _, row := range rows {
		result.RowsProcessed++
		parsed, err := parseRecurringImportDataRow(row, productLookup)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportRecurringInvoicesRowError{
				Row:      row.rowNumber,
				Template: strings.TrimSpace(row.values["name"]),
				Message:  err.Error(),
			})
			continue
		}

		key := normalizedRecurringImportKey(parsed.header.name)
		group, ok := groups[key]
		if !ok {
			group = &recurringImportGroup{
				header:   parsed.header,
				firstRow: row.rowNumber,
			}
			groups[key] = group
			groupOrder = append(groupOrder, key)
		} else if conflict := mergeRecurringImportGroup(group, parsed.header); conflict != "" && group.conflictText == "" {
			group.conflictRow = row.rowNumber
			group.conflictText = conflict
		}
		group.lines = append(group.lines, parsed.line)
		group.rowCount++
	}

	for _, key := range groupOrder {
		group := groups[key]
		if group.conflictText != "" {
			appendRecurringImportGroupError(result, group, group.conflictRow, group.conflictText)
			continue
		}
		if _, exists := existingKeys[key]; exists {
			appendRecurringImportGroupError(result, group, group.firstRow, fmt.Sprintf("template %q already exists", group.header.name))
			continue
		}

		contact, err := contactLookup.find(group.header.contactRef)
		if err != nil {
			appendRecurringImportGroupError(result, group, group.firstRow, err.Error())
			continue
		}

		template, err := buildImportedRecurringInvoice(tenantID, req.UserID, contact.ID, group)
		if err != nil {
			appendRecurringImportGroupError(result, group, group.firstRow, err.Error())
			continue
		}

		if err := s.repo.Create(ctx, schemaName, template); err != nil {
			appendRecurringImportGroupError(result, group, group.firstRow, err.Error())
			continue
		}
		lineInsertFailed := false
		for i := range template.Lines {
			line := template.Lines[i]
			if err := s.repo.CreateLine(ctx, schemaName, &line); err != nil {
				appendRecurringImportGroupError(result, group, group.firstRow, fmt.Sprintf("create recurring invoice line: %v", err))
				lineInsertFailed = true
				break
			}
		}
		if lineInsertFailed {
			continue
		}

		existingKeys[key] = struct{}{}
		result.TemplatesCreated++
		result.LinesImported += len(template.Lines)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	return result, nil
}

func appendRecurringImportGroupError(result *ImportRecurringInvoicesResult, group *recurringImportGroup, row int, message string) {
	result.RowsSkipped += group.rowCount
	result.Errors = append(result.Errors, ImportRecurringInvoicesRowError{
		Row:      row,
		Template: group.header.name,
		Message:  message,
	})
}

func parseRecurringImportRows(content string) ([]recurringImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectRecurringImportDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	required := map[string]bool{
		"name":             false,
		"frequency":        false,
		"start_date":       false,
		"line_description": false,
		"quantity":         false,
		"unit_price":       false,
		"vat_rate":         false,
	}
	hasContactColumn := false
	for i, header := range headers {
		canonical := canonicalRecurringImportHeader(header)
		canonicalHeaders[i] = canonical
		if _, ok := required[canonical]; ok {
			required[canonical] = true
		}
		switch canonical {
		case "contact_id", "contact_code", "contact_reg_code", "contact_vat_number", "contact_email", "contact_name":
			hasContactColumn = true
		}
	}
	for _, column := range []string{"name", "frequency", "start_date", "line_description", "quantity", "unit_price", "vat_rate"} {
		if !required[column] {
			return nil, fmt.Errorf("missing required %s column", column)
		}
	}
	if !hasContactColumn {
		return nil, fmt.Errorf("missing contact identifier column")
	}

	rows := make([]recurringImportRow, 0)
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
		rows = append(rows, recurringImportRow{rowNumber: rowNumber, values: values})
	}
	return rows, nil
}

func parseRecurringImportDataRow(row recurringImportRow, productLookup importrefs.ProductLookup) (*recurringImportParsedRow, error) {
	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	contactID, err := parseOptionalRecurringImportUUID("contact_id", row.values["contact_id"])
	if err != nil {
		return nil, err
	}
	contactIDValue := ""
	if contactID != nil {
		contactIDValue = *contactID
	}
	contactRef := recurringImportContactRef{
		id:        contactIDValue,
		code:      strings.TrimSpace(row.values["contact_code"]),
		regCode:   strings.TrimSpace(row.values["contact_reg_code"]),
		vatNumber: strings.TrimSpace(row.values["contact_vat_number"]),
		email:     strings.TrimSpace(row.values["contact_email"]),
		name:      strings.TrimSpace(row.values["contact_name"]),
	}
	if contactRef.id == "" && contactRef.code == "" && contactRef.regCode == "" && contactRef.vatNumber == "" && contactRef.email == "" && contactRef.name == "" {
		return nil, fmt.Errorf("a contact identifier is required")
	}

	frequency, err := parseRecurringImportFrequency(row.values["frequency"])
	if err != nil {
		return nil, err
	}
	startDate, err := parseRecurringImportDate(row.values["start_date"], "start_date")
	if err != nil {
		return nil, err
	}
	var endDate *time.Time
	if value := strings.TrimSpace(row.values["end_date"]); value != "" {
		parsed, err := parseRecurringImportDate(value, "end_date")
		if err != nil {
			return nil, err
		}
		endDate = &parsed
	}
	nextGenerationDate := startDate
	if value := strings.TrimSpace(row.values["next_generation_date"]); value != "" {
		nextGenerationDate, err = parseRecurringImportDate(value, "next_generation_date")
		if err != nil {
			return nil, err
		}
	}
	var lastGeneratedAt *time.Time
	if value := strings.TrimSpace(row.values["last_generated_at"]); value != "" {
		parsed, err := parseRecurringImportDate(value, "last_generated_at")
		if err != nil {
			return nil, err
		}
		lastGeneratedAt = &parsed
	}

	paymentTermsDays, err := parseRecurringImportOptionalNonNegativeInt("payment_terms_days", row.values["payment_terms_days"], 14)
	if err != nil {
		return nil, err
	}
	generatedCount, err := parseRecurringImportOptionalNonNegativeInt("generated_count", row.values["generated_count"], 0)
	if err != nil {
		return nil, err
	}
	isActive, err := parseRecurringImportOptionalBool("is_active", row.values["is_active"], true)
	if err != nil {
		return nil, err
	}
	sendEmail, err := parseRecurringImportOptionalBool("send_email_on_generation", row.values["send_email_on_generation"], false)
	if err != nil {
		return nil, err
	}
	attachPDF, err := parseRecurringImportOptionalBool("attach_pdf_to_email", row.values["attach_pdf_to_email"], true)
	if err != nil {
		return nil, err
	}

	line, err := parseRecurringImportLine(row, productLookup)
	if err != nil {
		return nil, err
	}

	invoiceType := strings.ToUpper(strings.TrimSpace(row.values["invoice_type"]))
	if invoiceType == "" {
		invoiceType = "SALES"
	}
	currency := strings.ToUpper(strings.TrimSpace(row.values["currency"]))
	if currency == "" {
		currency = "EUR"
	}
	emailTemplateType := strings.TrimSpace(row.values["email_template_type"])
	if emailTemplateType == "" {
		emailTemplateType = "INVOICE_SEND"
	}

	return &recurringImportParsedRow{
		header: recurringImportHeader{
			name:                   name,
			contactRef:             contactRef,
			invoiceType:            invoiceType,
			currency:               currency,
			frequency:              frequency,
			startDate:              startDate,
			endDate:                endDate,
			nextGenerationDate:     nextGenerationDate,
			paymentTermsDays:       paymentTermsDays,
			reference:              strings.TrimSpace(row.values["reference"]),
			notes:                  strings.TrimSpace(row.values["notes"]),
			isActive:               isActive,
			lastGeneratedAt:        lastGeneratedAt,
			generatedCount:         generatedCount,
			sendEmailOnGeneration:  sendEmail,
			emailTemplateType:      emailTemplateType,
			recipientEmailOverride: strings.TrimSpace(row.values["recipient_email_override"]),
			attachPDFToEmail:       attachPDF,
			emailSubjectOverride:   strings.TrimSpace(row.values["email_subject_override"]),
			emailMessage:           strings.TrimSpace(row.values["email_message"]),
		},
		line: line,
	}, nil
}

func parseRecurringImportLine(row recurringImportRow, productLookup importrefs.ProductLookup) (recurringImportLine, error) {
	description := strings.TrimSpace(row.values["line_description"])
	if description == "" {
		return recurringImportLine{}, fmt.Errorf("line_description is required")
	}
	quantity, err := parseRecurringImportDecimal(row.values["quantity"], "quantity")
	if err != nil {
		return recurringImportLine{}, err
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return recurringImportLine{}, fmt.Errorf("quantity must be greater than zero")
	}
	unitPrice, err := parseRecurringImportDecimal(row.values["unit_price"], "unit_price")
	if err != nil {
		return recurringImportLine{}, err
	}
	if unitPrice.IsNegative() {
		return recurringImportLine{}, fmt.Errorf("unit_price cannot be negative")
	}
	discountPercent := decimal.Zero
	if value := strings.TrimSpace(row.values["discount_percent"]); value != "" {
		discountPercent, err = decimal.NewFromString(normalizeRecurringImportDecimal(value))
		if err != nil {
			return recurringImportLine{}, fmt.Errorf("invalid discount_percent")
		}
		if discountPercent.IsNegative() || discountPercent.GreaterThan(decimal.NewFromInt(100)) {
			return recurringImportLine{}, fmt.Errorf("discount_percent must be between 0 and 100")
		}
	}
	vatRate, err := parseRecurringImportDecimal(row.values["vat_rate"], "vat_rate")
	if err != nil {
		return recurringImportLine{}, err
	}
	if vatRate.IsNegative() {
		return recurringImportLine{}, fmt.Errorf("vat_rate cannot be negative")
	}
	productID, err := productLookup.ResolveID(row.values["product_id"], row.values["product_code"])
	if err != nil {
		return recurringImportLine{}, err
	}
	accountID, err := parseOptionalRecurringImportUUID("account_id", row.values["account_id"])
	if err != nil {
		return recurringImportLine{}, err
	}
	return recurringImportLine{
		description:     description,
		quantity:        quantity,
		unit:            strings.TrimSpace(row.values["unit"]),
		unitPrice:       unitPrice,
		discountPercent: discountPercent,
		vatRate:         vatRate,
		accountID:       accountID,
		productID:       productID,
	}, nil
}

func mergeRecurringImportGroup(group *recurringImportGroup, next recurringImportHeader) string {
	if conflict := mergeRecurringImportContactRef(&group.header.contactRef, next.contactRef); conflict != "" {
		return conflict
	}
	if group.header.invoiceType != next.invoiceType {
		return "invoice_type must be consistent for each template"
	}
	if group.header.currency != next.currency {
		return "currency must be consistent for each template"
	}
	if group.header.frequency != next.frequency {
		return "frequency must be consistent for each template"
	}
	if !normalizeRecurringImportDate(group.header.startDate).Equal(normalizeRecurringImportDate(next.startDate)) {
		return "start_date must be consistent for each template"
	}
	for _, conflict := range []string{
		mergeRecurringImportOptionalDate(&group.header.endDate, next.endDate, "end_date"),
		mergeRecurringImportOptionalDateValue(&group.header.nextGenerationDate, next.nextGenerationDate, "next_generation_date"),
		mergeRecurringImportOptionalDate(&group.header.lastGeneratedAt, next.lastGeneratedAt, "last_generated_at"),
		mergeRecurringImportOptionalString(&group.header.reference, next.reference, "reference"),
		mergeRecurringImportOptionalString(&group.header.notes, next.notes, "notes"),
		mergeRecurringImportOptionalString(&group.header.emailTemplateType, next.emailTemplateType, "email_template_type"),
		mergeRecurringImportOptionalString(&group.header.recipientEmailOverride, next.recipientEmailOverride, "recipient_email_override"),
		mergeRecurringImportOptionalString(&group.header.emailSubjectOverride, next.emailSubjectOverride, "email_subject_override"),
		mergeRecurringImportOptionalString(&group.header.emailMessage, next.emailMessage, "email_message"),
	} {
		if conflict != "" {
			return conflict
		}
	}
	if group.header.paymentTermsDays != next.paymentTermsDays {
		return "payment_terms_days must be consistent for each template"
	}
	if group.header.isActive != next.isActive {
		return "is_active must be consistent for each template"
	}
	if group.header.generatedCount != next.generatedCount {
		return "generated_count must be consistent for each template"
	}
	if group.header.sendEmailOnGeneration != next.sendEmailOnGeneration {
		return "send_email_on_generation must be consistent for each template"
	}
	if group.header.attachPDFToEmail != next.attachPDFToEmail {
		return "attach_pdf_to_email must be consistent for each template"
	}
	return ""
}

func mergeRecurringImportContactRef(target *recurringImportContactRef, next recurringImportContactRef) string {
	if conflict := mergeRecurringImportOptionalString(&target.id, next.id, "contact_id"); conflict != "" {
		return conflict
	}
	if conflict := mergeRecurringImportOptionalString(&target.code, next.code, "contact_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeRecurringImportOptionalString(&target.regCode, next.regCode, "contact_reg_code"); conflict != "" {
		return conflict
	}
	if conflict := mergeRecurringImportOptionalString(&target.vatNumber, next.vatNumber, "contact_vat_number"); conflict != "" {
		return conflict
	}
	if conflict := mergeRecurringImportOptionalString(&target.email, next.email, "contact_email"); conflict != "" {
		return conflict
	}
	if conflict := mergeRecurringImportOptionalString(&target.name, next.name, "contact_name"); conflict != "" {
		return conflict
	}
	return ""
}

func buildImportedRecurringInvoice(tenantID, userID, contactID string, group *recurringImportGroup) (*RecurringInvoice, error) {
	template := &RecurringInvoice{
		ID:                     uuid.New().String(),
		TenantID:               tenantID,
		Name:                   group.header.name,
		ContactID:              contactID,
		InvoiceType:            group.header.invoiceType,
		Currency:               group.header.currency,
		Frequency:              group.header.frequency,
		StartDate:              group.header.startDate,
		EndDate:                group.header.endDate,
		NextGenerationDate:     group.header.nextGenerationDate,
		PaymentTermsDays:       group.header.paymentTermsDays,
		Reference:              group.header.reference,
		Notes:                  group.header.notes,
		IsActive:               group.header.isActive,
		LastGeneratedAt:        group.header.lastGeneratedAt,
		GeneratedCount:         group.header.generatedCount,
		CreatedAt:              time.Now(),
		CreatedBy:              userID,
		UpdatedAt:              time.Now(),
		SendEmailOnGeneration:  group.header.sendEmailOnGeneration,
		EmailTemplateType:      group.header.emailTemplateType,
		RecipientEmailOverride: group.header.recipientEmailOverride,
		AttachPDFToEmail:       group.header.attachPDFToEmail,
		EmailSubjectOverride:   group.header.emailSubjectOverride,
		EmailMessage:           group.header.emailMessage,
	}
	for index, line := range group.lines {
		template.Lines = append(template.Lines, RecurringInvoiceLine{
			ID:                 uuid.New().String(),
			RecurringInvoiceID: template.ID,
			LineNumber:         index + 1,
			Description:        line.description,
			Quantity:           line.quantity,
			Unit:               line.unit,
			UnitPrice:          line.unitPrice,
			DiscountPercent:    line.discountPercent,
			VATRate:            line.vatRate,
			AccountID:          line.accountID,
			ProductID:          line.productID,
		})
	}
	if err := template.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	return template, nil
}

func buildRecurringImportContactLookup(existingContacts []contacts.Contact) *recurringImportContactLookup {
	lookup := &recurringImportContactLookup{
		byID:        make(map[string]contacts.Contact),
		byCode:      make(map[string]contacts.Contact),
		byRegCode:   make(map[string]contacts.Contact),
		byVATNumber: make(map[string]contacts.Contact),
		byEmail:     make(map[string]contacts.Contact),
		byName:      make(map[string]contacts.Contact),
	}
	for _, contact := range existingContacts {
		if key := normalizedRecurringImportKey(contact.ID); key != "" {
			lookup.byID[key] = contact
		}
		if key := normalizedRecurringImportKey(contact.Code); key != "" {
			lookup.byCode[key] = contact
		}
		if key := normalizedRecurringImportKey(contact.RegCode); key != "" {
			lookup.byRegCode[key] = contact
		}
		if key := normalizedRecurringImportKey(contact.VATNumber); key != "" {
			lookup.byVATNumber[key] = contact
		}
		if key := normalizedRecurringImportKey(contact.Email); key != "" {
			lookup.byEmail[key] = contact
		}
		if key := normalizedRecurringImportKey(contact.Name); key != "" {
			lookup.byName[key] = contact
		}
	}
	return lookup
}

func (l *recurringImportContactLookup) find(ref recurringImportContactRef) (*contacts.Contact, error) {
	if key := normalizedRecurringImportKey(ref.id); key != "" {
		if contact, ok := l.byID[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_id %q was not found", ref.id)
	}
	if key := normalizedRecurringImportKey(ref.code); key != "" {
		if contact, ok := l.byCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_code %q was not found", ref.code)
	}
	if key := normalizedRecurringImportKey(ref.regCode); key != "" {
		if contact, ok := l.byRegCode[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_reg_code %q was not found", ref.regCode)
	}
	if key := normalizedRecurringImportKey(ref.vatNumber); key != "" {
		if contact, ok := l.byVATNumber[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_vat_number %q was not found", ref.vatNumber)
	}
	if key := normalizedRecurringImportKey(ref.email); key != "" {
		if contact, ok := l.byEmail[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_email %q was not found", ref.email)
	}
	if key := normalizedRecurringImportKey(ref.name); key != "" {
		if contact, ok := l.byName[key]; ok {
			return &contact, nil
		}
		return nil, fmt.Errorf("contact_name %q was not found", ref.name)
	}
	return nil, fmt.Errorf("a contact identifier is required")
}

func parseRecurringImportFrequency(raw string) (Frequency, error) {
	candidate := Frequency(strings.ToUpper(strings.TrimSpace(raw)))
	switch candidate {
	case FrequencyWeekly, FrequencyBiweekly, FrequencyMonthly, FrequencyQuarterly, FrequencyYearly:
		return candidate, nil
	default:
		return "", fmt.Errorf("invalid frequency %q", raw)
	}
}

func parseRecurringImportDate(value, field string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD", field)
	}
	return normalizeRecurringImportDate(parsed), nil
}

func parseRecurringImportDecimal(value, field string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(normalizeRecurringImportDecimal(value))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

func parseRecurringImportOptionalNonNegativeInt(field, value string, defaultValue int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", field)
	}
	return parsed, nil
}

func parseRecurringImportOptionalBool(field, value string, defaultValue bool) (bool, error) {
	normalized := normalizedRecurringImportKey(value)
	if normalized == "" {
		return defaultValue, nil
	}
	switch normalized {
	case "true", "t", "yes", "y", "1":
		return true, nil
	case "false", "f", "no", "n", "0":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be true or false", field)
	}
}

func mergeRecurringImportOptionalString(target *string, next, field string) string {
	if strings.TrimSpace(next) == "" {
		return ""
	}
	if strings.TrimSpace(*target) == "" {
		*target = strings.TrimSpace(next)
		return ""
	}
	if strings.TrimSpace(*target) != strings.TrimSpace(next) {
		return fmt.Sprintf("%s must be consistent for each template", field)
	}
	return ""
}

func mergeRecurringImportOptionalDate(target **time.Time, next *time.Time, field string) string {
	if next == nil {
		return ""
	}
	if *target == nil {
		value := *next
		*target = &value
		return ""
	}
	if !normalizeRecurringImportDate(**target).Equal(normalizeRecurringImportDate(*next)) {
		return fmt.Sprintf("%s must be consistent for each template", field)
	}
	return ""
}

func mergeRecurringImportOptionalDateValue(target *time.Time, next time.Time, field string) string {
	if !normalizeRecurringImportDate(*target).Equal(normalizeRecurringImportDate(next)) {
		return fmt.Sprintf("%s must be consistent for each template", field)
	}
	return ""
}

func parseOptionalRecurringImportUUID(field, value string) (*string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsedID, err := uuid.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s must be a valid UUID", field)
	}
	canonicalID := parsedID.String()
	return &canonicalID, nil
}

func canonicalRecurringImportHeader(value string) string {
	normalized := normalizedRecurringImportKey(value)
	if canonical, ok := recurringImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return ""
}

func detectRecurringImportDelimiter(content string) rune {
	switch {
	case strings.Contains(content, "\t"):
		return '\t'
	case strings.Contains(content, ";"):
		return ';'
	default:
		return ','
	}
}

func normalizedRecurringImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeRecurringImportDecimal(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	return normalized
}

func normalizeRecurringImportDate(value time.Time) time.Time {
	utcValue := value.UTC()
	return time.Date(utcValue.Year(), utcValue.Month(), utcValue.Day(), 0, 0, 0, 0, time.UTC)
}
