package contacts

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"
)

type importRow struct {
	rowNumber int
	values    map[string]string
}

type importIndexes struct {
	codes    map[string]string
	regCodes map[string]string
	emails   map[string]string
	names    map[string]string
}

var contactImportHeaderAliases = map[string]string{
	"name":               "name",
	"contact_name":       "name",
	"company":            "name",
	"company_name":       "name",
	"contact_type":       "contact_type",
	"type":               "contact_type",
	"role":               "contact_type",
	"code":               "code",
	"contact_code":       "code",
	"reg_code":           "reg_code",
	"registration_code":  "reg_code",
	"registry_code":      "reg_code",
	"vat_number":         "vat_number",
	"vat":                "vat_number",
	"vat_no":             "vat_number",
	"email":              "email",
	"e_mail":             "email",
	"phone":              "phone",
	"telephone":          "phone",
	"address":            "address_line1",
	"address_line1":      "address_line1",
	"address_line_1":     "address_line1",
	"street":             "address_line1",
	"street_address":     "address_line1",
	"address_line2":      "address_line2",
	"address_line_2":     "address_line2",
	"city":               "city",
	"postal_code":        "postal_code",
	"postcode":           "postal_code",
	"zip":                "postal_code",
	"zip_code":           "postal_code",
	"country":            "country_code",
	"country_code":       "country_code",
	"payment_terms_days": "payment_terms_days",
	"payment_days":       "payment_terms_days",
	"terms_days":         "payment_terms_days",
	"credit_limit":       "credit_limit",
	"notes":              "notes",
}

var contactImportTypeAliases = map[string]ContactType{
	"":         ContactTypeCustomer,
	"customer": ContactTypeCustomer,
	"client":   ContactTypeCustomer,
	"klient":   ContactTypeCustomer,
	"supplier": ContactTypeSupplier,
	"vendor":   ContactTypeSupplier,
	"tarnija":  ContactTypeSupplier,
	"both":     ContactTypeBoth,
	"molemad":  ContactTypeBoth,
}

// ImportCSV imports contacts from CSV content and skips duplicate or invalid rows.
func (s *Service) ImportCSV(ctx context.Context, tenantID, schemaName string, req *ImportContactsRequest) (*ImportContactsResult, error) {
	if strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no contacts found in CSV")
	}

	existingContacts, err := s.repo.List(ctx, schemaName, tenantID, nil)
	if err != nil {
		return nil, fmt.Errorf("list existing contacts: %w", err)
	}

	indexes := buildImportIndexes(existingContacts)
	result := &ImportContactsResult{
		FileName: req.FileName,
		Errors:   []ImportContactsRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		createReq, err := buildCreateRequestFromImportRow(row)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportContactsRowError{
				Row:     row.rowNumber,
				Name:    strings.TrimSpace(row.values["name"]),
				Message: err.Error(),
			})
			continue
		}

		if duplicateReason := indexes.findDuplicate(*createReq); duplicateReason != "" {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportContactsRowError{
				Row:     row.rowNumber,
				Name:    createReq.Name,
				Message: duplicateReason,
			})
			continue
		}

		if _, err := s.Create(ctx, tenantID, schemaName, createReq); err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportContactsRowError{
				Row:     row.rowNumber,
				Name:    createReq.Name,
				Message: err.Error(),
			})
			continue
		}

		result.ContactsCreated++
		indexes.add(*createReq)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseImportRows(content string) ([]importRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectImportDelimiter(trimmed)
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
	hasNameColumn := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalImportHeader(header)
		if canonicalHeaders[i] == "name" {
			hasNameColumn = true
		}
	}

	if !hasNameColumn {
		return nil, fmt.Errorf("missing required name column")
	}

	rows := make([]importRow, 0)
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
		rowValues := make(map[string]string, len(canonicalHeaders))
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
			rowValues[header] = value
		}

		if isBlank {
			continue
		}

		rows = append(rows, importRow{
			rowNumber: rowNumber,
			values:    rowValues,
		})
	}

	return rows, nil
}

func buildCreateRequestFromImportRow(row importRow) (*CreateContactRequest, error) {
	name := strings.TrimSpace(row.values["name"])
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	contactType, err := parseImportContactType(row.values["contact_type"])
	if err != nil {
		return nil, err
	}

	paymentTermsDays := 14
	if value := strings.TrimSpace(row.values["payment_terms_days"]); value != "" {
		parsed, err := parseImportInt(value, "payment_terms_days")
		if err != nil {
			return nil, err
		}
		if parsed < 0 {
			return nil, fmt.Errorf("payment_terms_days must be zero or greater")
		}
		paymentTermsDays = parsed
	}

	countryCode := strings.ToUpper(strings.TrimSpace(row.values["country_code"]))
	if countryCode == "" {
		countryCode = "EE"
	}
	if len(countryCode) != 2 {
		return nil, fmt.Errorf("country_code must be a 2-letter code")
	}

	creditLimit := decimal.Zero
	if value := strings.TrimSpace(row.values["credit_limit"]); value != "" {
		parsed, err := decimal.NewFromString(normalizeImportDecimal(value))
		if err != nil {
			return nil, fmt.Errorf("invalid credit_limit")
		}
		creditLimit = parsed
	}

	return &CreateContactRequest{
		Code:             strings.TrimSpace(row.values["code"]),
		Name:             name,
		ContactType:      contactType,
		RegCode:          strings.TrimSpace(row.values["reg_code"]),
		VATNumber:        strings.TrimSpace(row.values["vat_number"]),
		Email:            strings.TrimSpace(row.values["email"]),
		Phone:            strings.TrimSpace(row.values["phone"]),
		AddressLine1:     strings.TrimSpace(row.values["address_line1"]),
		AddressLine2:     strings.TrimSpace(row.values["address_line2"]),
		City:             strings.TrimSpace(row.values["city"]),
		PostalCode:       strings.TrimSpace(row.values["postal_code"]),
		CountryCode:      countryCode,
		PaymentTermsDays: paymentTermsDays,
		CreditLimit:      creditLimit,
		Notes:            strings.TrimSpace(row.values["notes"]),
	}, nil
}

func buildImportIndexes(existing []Contact) *importIndexes {
	indexes := &importIndexes{
		codes:    make(map[string]string),
		regCodes: make(map[string]string),
		emails:   make(map[string]string),
		names:    make(map[string]string),
	}
	for _, contact := range existing {
		indexes.add(CreateContactRequest{
			Code:        contact.Code,
			Name:        contact.Name,
			RegCode:     contact.RegCode,
			Email:       contact.Email,
			ContactType: contact.ContactType,
		})
	}
	return indexes
}

func (i *importIndexes) add(contact CreateContactRequest) {
	if key := normalizedImportKey(contact.Code); key != "" {
		i.codes[key] = contact.Name
	}
	if key := normalizedImportKey(contact.RegCode); key != "" {
		i.regCodes[key] = contact.Name
	}
	if key := normalizedImportKey(contact.Email); key != "" {
		i.emails[key] = contact.Name
	}
	if key := normalizedImportKey(contact.Name); key != "" {
		i.names[key] = contact.Name
	}
}

func (i *importIndexes) findDuplicate(contact CreateContactRequest) string {
	if key := normalizedImportKey(contact.Code); key != "" {
		if existingName, ok := i.codes[key]; ok {
			return fmt.Sprintf("duplicate code %q matches existing contact %q", contact.Code, existingName)
		}
	}
	if key := normalizedImportKey(contact.RegCode); key != "" {
		if existingName, ok := i.regCodes[key]; ok {
			return fmt.Sprintf("duplicate reg_code %q matches existing contact %q", contact.RegCode, existingName)
		}
	}
	if key := normalizedImportKey(contact.Email); key != "" {
		if existingName, ok := i.emails[key]; ok {
			return fmt.Sprintf("duplicate email %q matches existing contact %q", contact.Email, existingName)
		}
	}
	if key := normalizedImportKey(contact.Name); key != "" {
		if existingName, ok := i.names[key]; ok {
			return fmt.Sprintf("duplicate name %q matches existing contact %q", contact.Name, existingName)
		}
	}
	return ""
}

func parseImportContactType(value string) (ContactType, error) {
	normalized := normalizedImportKey(value)
	if contactType, ok := contactImportTypeAliases[normalized]; ok {
		return contactType, nil
	}
	return "", fmt.Errorf("invalid contact_type %q", value)
}

func detectImportDelimiter(content string) rune {
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

func canonicalImportHeader(header string) string {
	normalized := normalizedImportHeader(header)
	if canonical, ok := contactImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedImportHeader(header string) string {
	normalized := strings.TrimSpace(strings.ToLower(header))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", ".", "_")
	return replacer.Replace(normalized)
}

func normalizedImportKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseImportInt(value string, fieldName string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s", fieldName)
	}
	return parsed, nil
}

func normalizeImportDecimal(value string) string {
	trimmed := strings.TrimSpace(value)
	if strings.Contains(trimmed, ",") && !strings.Contains(trimmed, ".") {
		return strings.ReplaceAll(trimmed, ",", ".")
	}
	return strings.ReplaceAll(trimmed, ",", "")
}
