package cutover

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
)

type fileSpec struct {
	aliases        map[string]string
	requiredGroups [][]string
}

type parsedFile struct {
	kind     FileKind
	fileName string
	headers  []string
	rows     []parsedRow
}

type parsedRow struct {
	number int
	values map[string]string
}

type bundleIndexes struct {
	files     map[FileKind]bool
	accounts  map[string]bool
	contacts  map[string]bool
	employees map[string]bool
	invoices  map[string]bool
}

var fileSpecs = map[FileKind]fileSpec{
	KindAccounts: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"account_code": "code",
			"account_name": "name",
			"account_type": "account_type",
			"type":         "account_type",
			"parent_code":  "parent_code",
		}),
		requiredGroups: [][]string{{"code"}, {"name"}, {"account_type"}},
	},
	KindContacts: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"contact_name":      "name",
			"company":           "name",
			"company_name":      "name",
			"contact_code":      "code",
			"customer_code":     "code",
			"supplier_code":     "code",
			"contact_type":      "contact_type",
			"type":              "contact_type",
			"reg_code":          "reg_code",
			"registration_code": "reg_code",
			"registry_code":     "reg_code",
			"contact_email":     "email",
			"e_mail":            "email",
		}),
		requiredGroups: [][]string{{"name"}},
	},
	KindEmployees: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"employee_number":       "employee_number",
			"employee_no":           "employee_number",
			"employee_code":         "employee_number",
			"employee_id":           "employee_number",
			"personal_code":         "personal_code",
			"isikukood":             "personal_code",
			"e_mail":                "email",
			"base_salary":           "base_salary",
			"salary_effective_from": "salary_effective_from",
		}),
		requiredGroups: [][]string{{"first_name", "name"}, {"last_name", "name"}},
	},
	KindExpenses: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"expense_number":     "expense_number",
			"expense_no":         "expense_number",
			"number":             "expense_number",
			"date":               "expense_date",
			"expense_date":       "expense_date",
			"supplier":           "merchant",
			"vendor":             "merchant",
			"merchant":           "merchant",
			"notes":              "description",
			"employee_id":        "employee_id",
			"contact_id":         "contact_id",
			"expense_account_id": "expense_account_id",
			"payment_account_id": "payment_account_id",
			"requires_receipt":   "requires_receipt",
			"receipt_required":   "requires_receipt",
		}),
		requiredGroups: [][]string{
			{"expense_date"},
			{"merchant"},
			{"expense_account_id"},
			{"payment_account_id"},
			{"amount"},
		},
	},
	KindInvoices: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"invoice_number":     "invoice_number",
			"number":             "invoice_number",
			"invoice_no":         "invoice_number",
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
			"line_description":   "line_description",
			"description":        "line_description",
			"qty":                "quantity",
			"price":              "unit_price",
			"vat":                "vat_rate",
		}),
		requiredGroups: [][]string{
			{"invoice_number"},
			{"issue_date"},
			{"contact_code", "contact_reg_code", "contact_email", "contact_name"},
			{"line_description"},
			{"quantity"},
			{"unit_price"},
			{"vat_rate"},
		},
	},
	KindPayments: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"payment_number":    "payment_number",
			"payment_no":        "payment_number",
			"number":            "payment_number",
			"type":              "payment_type",
			"payment_type":      "payment_type",
			"date":              "payment_date",
			"payment_date":      "payment_date",
			"invoice_id":        "invoice_id",
			"invoice_number":    "invoice_number",
			"allocation_amount": "allocation_amount",
			"allocated_amount":  "allocation_amount",
		}),
		requiredGroups: [][]string{{"payment_type"}, {"payment_date"}, {"amount"}},
	},
	KindPayrollHistory: {
		aliases: mergeAliases(employeeReferenceAliases(), map[string]string{
			"period_year":   "period_year",
			"payroll_year":  "period_year",
			"year":          "period_year",
			"period_month":  "period_month",
			"payroll_month": "period_month",
			"month":         "period_month",
			"gross":         "gross_salary",
			"gross_salary":  "gross_salary",
		}),
		requiredGroups: [][]string{
			{"period_year"},
			{"period_month"},
			{"employee_number", "personal_code", "email", "first_name", "name"},
			{"gross_salary"},
		},
	},
	KindLeaveBalances: {
		aliases: mergeAliases(employeeReferenceAliases(), map[string]string{
			"year":              "year",
			"absence_type":      "absence_type",
			"absence_type_code": "absence_type_code",
			"type_code":         "absence_type_code",
			"entitled":          "entitled_days",
			"entitled_days":     "entitled_days",
		}),
		requiredGroups: [][]string{
			{"year"},
			{"employee_number", "personal_code", "email", "first_name", "name"},
			{"absence_type_code", "absence_type", "absence_type_id"},
		},
	},
	KindOpeningBalances: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"account_code": "account_code",
			"account":      "account_code",
			"description":  "description",
		}),
		requiredGroups: [][]string{{"account_code"}, {"debit", "credit"}},
	},
	KindJournalEntries: {
		aliases: mergeAliases(commonAliases(), map[string]string{
			"entry_reference":   "entry_reference",
			"reference":         "entry_reference",
			"entry_date":        "entry_date",
			"date":              "entry_date",
			"account_code":      "account_code",
			"account":           "account_code",
			"entry_description": "entry_description",
			"line_description":  "line_description",
		}),
		requiredGroups: [][]string{{"entry_reference"}, {"entry_date"}, {"account_code"}, {"debit", "credit"}},
	},
}

func ValidateBundle(req *ValidateBundleRequest) (*BundleValidationReport, error) {
	if req == nil || len(req.Files) == 0 {
		return nil, fmt.Errorf("at least one migration file is required")
	}

	report := &BundleValidationReport{}
	parsed := make([]parsedFile, 0, len(req.Files))
	for _, file := range req.Files {
		spec, ok := fileSpecs[file.Kind]
		if !ok {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.Kind,
				FileName: displayFileName(file),
				Message:  fmt.Sprintf("unsupported migration file kind %q", file.Kind),
			})
			continue
		}

		parsedFile, validation, err := parseBundleFile(file, spec)
		report.Files = append(report.Files, validation)
		if err != nil {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.Kind,
				FileName: validation.FileName,
				Message:  err.Error(),
			})
			continue
		}

		for _, missing := range validation.MissingColumns {
			report.addIssue(ValidationIssue{
				Severity: SeverityError,
				Kind:     file.Kind,
				FileName: validation.FileName,
				Message:  "missing required column group: " + missing,
			})
		}
		report.Summary.RowsValidated += len(parsedFile.rows)
		parsed = append(parsed, parsedFile)
	}

	report.Summary.FilesValidated = len(report.Files)
	indexes := buildIndexes(parsed)
	for _, file := range parsed {
		validateReferences(report, indexes, file)
	}

	sort.SliceStable(report.Issues, func(i, j int) bool {
		if report.Issues[i].Severity != report.Issues[j].Severity {
			return report.Issues[i].Severity < report.Issues[j].Severity
		}
		if report.Issues[i].FileName != report.Issues[j].FileName {
			return report.Issues[i].FileName < report.Issues[j].FileName
		}
		return report.Issues[i].Row < report.Issues[j].Row
	})

	report.Summary.Ready = report.Summary.ErrorCount == 0
	return report, nil
}

func parseBundleFile(file BundleFile, spec fileSpec) (parsedFile, FileValidation, error) {
	fileName := displayFileName(file)
	validation := FileValidation{
		Kind:     file.Kind,
		FileName: fileName,
	}

	trimmed := strings.TrimPrefix(strings.TrimSpace(file.CSVContent), "\ufeff")
	if trimmed == "" {
		return parsedFile{}, validation, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return parsedFile{}, validation, fmt.Errorf("csv file is empty")
		}
		return parsedFile{}, validation, fmt.Errorf("parse csv header: %w", err)
	}

	canonicalHeaders := make([]string, len(headers))
	headerSet := map[string]bool{}
	for i, header := range headers {
		canonical := canonicalHeader(spec.aliases, header)
		canonicalHeaders[i] = canonical
		if canonical != "" {
			headerSet[canonical] = true
			validation.Headers = append(validation.Headers, canonical)
		}
	}
	validation.MissingColumns = missingRequiredGroups(spec.requiredGroups, headerSet)

	rows := []parsedRow{}
	rowNumber := 1
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return parsedFile{}, validation, fmt.Errorf("parse csv row %d: %w", rowNumber+1, err)
		}
		rowNumber++

		values := make(map[string]string, len(canonicalHeaders))
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
		rows = append(rows, parsedRow{number: rowNumber, values: values})
	}

	validation.Rows = len(rows)
	return parsedFile{kind: file.Kind, fileName: fileName, headers: validation.Headers, rows: rows}, validation, nil
}

func buildIndexes(files []parsedFile) bundleIndexes {
	indexes := bundleIndexes{
		files:     map[FileKind]bool{},
		accounts:  map[string]bool{},
		contacts:  map[string]bool{},
		employees: map[string]bool{},
		invoices:  map[string]bool{},
	}
	for _, file := range files {
		indexes.files[file.kind] = true
		for _, row := range file.rows {
			switch file.kind {
			case KindAccounts:
				addIndexValue(indexes.accounts, row.values["code"])
			case KindContacts:
				addIndexValue(indexes.contacts, row.values["code"])
				addIndexValue(indexes.contacts, row.values["reg_code"])
				addIndexValue(indexes.contacts, row.values["email"])
				addIndexValue(indexes.contacts, row.values["name"])
			case KindEmployees:
				addEmployeeIndexValues(indexes.employees, row.values)
			case KindInvoices:
				addIndexValue(indexes.invoices, row.values["invoice_number"])
				addIndexValue(indexes.invoices, row.values["invoice_id"])
				addIndexValue(indexes.invoices, row.values["id"])
			}
		}
	}
	return indexes
}

func validateReferences(report *BundleValidationReport, indexes bundleIndexes, file parsedFile) {
	for _, row := range file.rows {
		switch file.kind {
		case KindInvoices:
			checkTargetReference(report, indexes.files[KindContacts], indexes.contacts, file, row, KindContacts,
				[]string{"contact_code", "contact_reg_code", "contact_email", "contact_name"})
		case KindPayments:
			checkTargetReference(report, indexes.files[KindInvoices], indexes.invoices, file, row, KindInvoices,
				[]string{"invoice_number"})
		case KindPayrollHistory, KindLeaveBalances:
			checkEmployeeReference(report, indexes, file, row)
		case KindOpeningBalances, KindJournalEntries:
			checkTargetReference(report, indexes.files[KindAccounts], indexes.accounts, file, row, KindAccounts,
				[]string{"account_code"})
		}
	}
}

func checkEmployeeReference(report *BundleValidationReport, indexes bundleIndexes, file parsedFile, row parsedRow) {
	if !indexes.files[KindEmployees] {
		return
	}
	values := []string{
		row.values["employee_number"],
		row.values["personal_code"],
		row.values["email"],
		employeeName(row.values),
	}
	checkReferenceValues(report, indexes.employees, file, row, KindEmployees, "employee", values)
}

func checkTargetReference(
	report *BundleValidationReport,
	targetPresent bool,
	targetIndex map[string]bool,
	file parsedFile,
	row parsedRow,
	targetKind FileKind,
	fields []string,
) {
	if !targetPresent {
		return
	}
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		values = append(values, row.values[field])
	}
	checkReferenceValues(report, targetIndex, file, row, targetKind, strings.Join(fields, "/"), values)
}

func checkReferenceValues(
	report *BundleValidationReport,
	targetIndex map[string]bool,
	file parsedFile,
	row parsedRow,
	targetKind FileKind,
	field string,
	values []string,
) {
	var firstValue string
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if firstValue == "" {
			firstValue = trimmed
		}
		if targetIndex[normalizedValue(trimmed)] {
			return
		}
	}
	if firstValue == "" {
		return
	}
	report.addIssue(ValidationIssue{
		Severity:   SeverityError,
		Kind:       file.kind,
		FileName:   file.fileName,
		Row:        row.number,
		Field:      field,
		Value:      firstValue,
		TargetKind: targetKind,
		Message:    fmt.Sprintf("%s reference %q was not found in %s file", field, firstValue, targetKind),
	})
}

func (r *BundleValidationReport) addIssue(issue ValidationIssue) {
	r.Issues = append(r.Issues, issue)
	switch issue.Severity {
	case SeverityWarning:
		r.Summary.WarningCount++
	default:
		r.Summary.ErrorCount++
	}
}

func missingRequiredGroups(groups [][]string, headerSet map[string]bool) []string {
	var missing []string
	for _, group := range groups {
		found := false
		for _, column := range group {
			if headerSet[column] {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, strings.Join(group, "|"))
		}
	}
	return missing
}

func addEmployeeIndexValues(index map[string]bool, values map[string]string) {
	addIndexValue(index, values["employee_number"])
	addIndexValue(index, values["personal_code"])
	addIndexValue(index, values["email"])
	addIndexValue(index, employeeName(values))
}

func addIndexValue(index map[string]bool, value string) {
	key := normalizedValue(value)
	if key != "" {
		index[key] = true
	}
}

func employeeName(values map[string]string) string {
	if name := strings.TrimSpace(values["name"]); name != "" {
		return name
	}
	return strings.TrimSpace(strings.Join([]string{values["first_name"], values["last_name"]}, " "))
}

func displayFileName(file BundleFile) string {
	if strings.TrimSpace(file.FileName) != "" {
		return strings.TrimSpace(file.FileName)
	}
	return string(file.Kind) + ".csv"
}

func canonicalHeader(aliases map[string]string, value string) string {
	normalized := normalizedHeader(value)
	if canonical, ok := aliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func normalizedHeader(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(value)), "\ufeff")
	var builder strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(builder.String(), "_")
}

func normalizedValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func detectDelimiter(content string) rune {
	firstLine := content
	if idx := strings.IndexAny(content, "\r\n"); idx >= 0 {
		firstLine = content[:idx]
	}
	candidates := []rune{',', ';', '\t'}
	best := ','
	bestCount := -1
	for _, candidate := range candidates {
		count := strings.Count(firstLine, string(candidate))
		if count > bestCount {
			best = candidate
			bestCount = count
		}
	}
	return best
}

func commonAliases() map[string]string {
	return map[string]string{
		"id":          "id",
		"code":        "code",
		"name":        "name",
		"email":       "email",
		"amount":      "amount",
		"debit":       "debit",
		"credit":      "credit",
		"currency":    "currency",
		"status":      "status",
		"description": "description",
	}
}

func employeeReferenceAliases() map[string]string {
	return mergeAliases(commonAliases(), map[string]string{
		"employee_number": "employee_number",
		"employee_no":     "employee_number",
		"employee_code":   "employee_number",
		"employee_id":     "employee_number",
		"personal_code":   "personal_code",
		"isikukood":       "personal_code",
		"e_mail":          "email",
		"first_name":      "first_name",
		"last_name":       "last_name",
	})
}

func mergeAliases(base map[string]string, extra map[string]string) map[string]string {
	merged := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		merged[normalizedHeader(key)] = value
	}
	for key, value := range extra {
		merged[normalizedHeader(key)] = value
	}
	return merged
}
