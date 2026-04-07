package payroll

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type employeeImportRow struct {
	rowNumber int
	values    map[string]string
}

type employeeImportRecord struct {
	createRequest        CreateEmployeeRequest
	employeeName         string
	employeeNumber       string
	endDate              *time.Time
	isActive             *bool
	baseSalary           *decimal.Decimal
	salaryEffectiveFrom  *time.Time
	duplicateEmployeeKey string
}

type employeeImportIndexes struct {
	employeeNumbers map[string]string
	personalCodes   map[string]string
	emails          map[string]string
	nameStartDates  map[string]string
}

var employeeImportHeaderAliases = map[string]string{
	"employee_number":        "employee_number",
	"number":                 "employee_number",
	"employee_no":            "employee_number",
	"employee_id":            "employee_number",
	"first_name":             "first_name",
	"firstname":              "first_name",
	"given_name":             "first_name",
	"last_name":              "last_name",
	"lastname":               "last_name",
	"surname":                "last_name",
	"family_name":            "last_name",
	"personal_code":          "personal_code",
	"isikukood":              "personal_code",
	"email":                  "email",
	"phone":                  "phone",
	"telephone":              "phone",
	"address":                "address",
	"bank_account":           "bank_account",
	"iban":                   "bank_account",
	"start_date":             "start_date",
	"employment_start":       "start_date",
	"end_date":               "end_date",
	"employment_end":         "end_date",
	"position":               "position",
	"title":                  "position",
	"department":             "department",
	"team":                   "department",
	"employment_type":        "employment_type",
	"type":                   "employment_type",
	"apply_basic_exemption":  "apply_basic_exemption",
	"basic_exemption":        "apply_basic_exemption",
	"basic_exemption_amount": "basic_exemption_amount",
	"funded_pension_rate":    "funded_pension_rate",
	"pension_rate":           "funded_pension_rate",
	"base_salary":            "base_salary",
	"salary":                 "base_salary",
	"gross_salary":           "base_salary",
	"salary_effective_from":  "salary_effective_from",
	"effective_from":         "salary_effective_from",
	"is_active":              "is_active",
	"active":                 "is_active",
}

var employeeImportEmploymentTypeAliases = map[string]EmploymentType{
	"":               EmploymentFullTime,
	"full_time":      EmploymentFullTime,
	"full-time":      EmploymentFullTime,
	"full time":      EmploymentFullTime,
	"tais":           EmploymentFullTime,
	"part_time":      EmploymentPartTime,
	"part-time":      EmploymentPartTime,
	"part time":      EmploymentPartTime,
	"osaline":        EmploymentPartTime,
	"contract":       EmploymentContract,
	"contractor":     EmploymentContract,
	"work_order":     EmploymentContract,
	"too_vott":       EmploymentContract,
	"too_votuleping": EmploymentContract,
}

var employeeImportBoolAliases = map[string]bool{
	"1":     true,
	"0":     false,
	"true":  true,
	"false": false,
	"yes":   true,
	"no":    false,
	"y":     true,
	"n":     false,
	"ja":    true,
	"ei":    false,
}

// ImportEmployeesCSV imports employees and optional base salaries from CSV content.
func (s *Service) ImportEmployeesCSV(ctx context.Context, schemaName, tenantID string, req *ImportEmployeesRequest) (*ImportEmployeesResult, error) {
	if strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseEmployeeImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no employees found in CSV")
	}

	existingEmployees, err := s.repo.ListEmployees(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing employees: %w", err)
	}

	indexes := buildEmployeeImportIndexes(existingEmployees)
	result := &ImportEmployeesResult{
		FileName: req.FileName,
		Errors:   []ImportEmployeesRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		record, err := buildEmployeeImportRecord(row)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportEmployeesRowError{
				Row:          row.rowNumber,
				EmployeeName: employeeImportDisplayName(row.values["first_name"], row.values["last_name"]),
				Message:      err.Error(),
			})
			continue
		}

		if duplicateReason := indexes.findDuplicate(record); duplicateReason != "" {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportEmployeesRowError{
				Row:            row.rowNumber,
				EmployeeName:   record.employeeName,
				EmployeeNumber: record.employeeNumber,
				Message:        duplicateReason,
			})
			continue
		}

		tx, err := s.repo.BeginTx(ctx)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportEmployeesRowError{
				Row:            row.rowNumber,
				EmployeeName:   record.employeeName,
				EmployeeNumber: record.employeeNumber,
				Message:        fmt.Sprintf("begin transaction: %v", err),
			})
			continue
		}

		txRepo := s.repo.WithTx(tx)
		txService := &Service{
			repo: txRepo,
			uuid: s.uuid,
		}

		employee, err := txService.CreateEmployee(ctx, schemaName, tenantID, &record.createRequest)
		if err != nil {
			_ = tx.Rollback(ctx)
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportEmployeesRowError{
				Row:            row.rowNumber,
				EmployeeName:   record.employeeName,
				EmployeeNumber: record.employeeNumber,
				Message:        err.Error(),
			})
			continue
		}

		if record.endDate != nil || record.isActive != nil {
			updateReq := &UpdateEmployeeRequest{
				EndDate:  record.endDate,
				IsActive: record.isActive,
			}
			if _, err := txService.UpdateEmployee(ctx, schemaName, tenantID, employee.ID, updateReq); err != nil {
				_ = tx.Rollback(ctx)
				result.RowsSkipped++
				result.Errors = append(result.Errors, ImportEmployeesRowError{
					Row:            row.rowNumber,
					EmployeeName:   record.employeeName,
					EmployeeNumber: record.employeeNumber,
					Message:        err.Error(),
				})
				continue
			}
		}

		salaryCreated := false
		if record.baseSalary != nil {
			effectiveFrom := record.createRequest.StartDate
			if record.salaryEffectiveFrom != nil {
				effectiveFrom = *record.salaryEffectiveFrom
			}

			if err := txService.SetBaseSalary(ctx, schemaName, tenantID, employee.ID, *record.baseSalary, effectiveFrom); err != nil {
				_ = tx.Rollback(ctx)
				result.RowsSkipped++
				result.Errors = append(result.Errors, ImportEmployeesRowError{
					Row:            row.rowNumber,
					EmployeeName:   record.employeeName,
					EmployeeNumber: record.employeeNumber,
					Message:        err.Error(),
				})
				continue
			}
			salaryCreated = true
		}

		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportEmployeesRowError{
				Row:            row.rowNumber,
				EmployeeName:   record.employeeName,
				EmployeeNumber: record.employeeNumber,
				Message:        fmt.Sprintf("commit transaction: %v", err),
			})
			continue
		}

		result.EmployeesCreated++
		if salaryCreated {
			result.SalariesCreated++
		}
		indexes.add(record)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseEmployeeImportRows(content string) ([]employeeImportRow, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectEmployeeImportDelimiter(trimmed)
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
	hasFirstName := false
	hasLastName := false
	hasStartDate := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalEmployeeImportHeader(header)
		switch canonicalHeaders[i] {
		case "first_name":
			hasFirstName = true
		case "last_name":
			hasLastName = true
		case "start_date":
			hasStartDate = true
		}
	}

	if !hasFirstName || !hasLastName || !hasStartDate {
		return nil, fmt.Errorf("missing required first_name, last_name, or start_date column")
	}

	rows := make([]employeeImportRow, 0)
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

		rows = append(rows, employeeImportRow{
			rowNumber: rowNumber,
			values:    rowValues,
		})
	}

	return rows, nil
}

func buildEmployeeImportRecord(row employeeImportRow) (*employeeImportRecord, error) {
	firstName := strings.TrimSpace(row.values["first_name"])
	lastName := strings.TrimSpace(row.values["last_name"])
	if firstName == "" || lastName == "" {
		return nil, fmt.Errorf("first_name and last_name are required")
	}

	startDate, err := parseEmployeeImportDate(row.values["start_date"], "start_date")
	if err != nil {
		return nil, err
	}

	employmentType, err := parseEmployeeImportEmploymentType(row.values["employment_type"])
	if err != nil {
		return nil, err
	}

	applyBasicExemption := true
	if value := strings.TrimSpace(row.values["apply_basic_exemption"]); value != "" {
		parsed, err := parseEmployeeImportBool(value, "apply_basic_exemption")
		if err != nil {
			return nil, err
		}
		applyBasicExemption = parsed
	}

	basicExemptionAmount := decimal.Zero
	if applyBasicExemption {
		basicExemptionAmount = DefaultBasicExemption
	}
	if value := strings.TrimSpace(row.values["basic_exemption_amount"]); value != "" {
		parsed, err := parseEmployeeImportDecimal(value, "basic_exemption_amount")
		if err != nil {
			return nil, err
		}
		if parsed.IsNegative() {
			return nil, fmt.Errorf("basic_exemption_amount must be zero or greater")
		}
		basicExemptionAmount = parsed
	}

	fundedPensionRate := FundedPensionRateDefault
	if value := strings.TrimSpace(row.values["funded_pension_rate"]); value != "" {
		parsed, err := parseEmployeeImportDecimal(value, "funded_pension_rate")
		if err != nil {
			return nil, err
		}
		if parsed.IsNegative() || parsed.GreaterThan(decimal.NewFromInt(1)) {
			return nil, fmt.Errorf("funded_pension_rate must be between 0 and 1")
		}
		fundedPensionRate = parsed
	}

	var endDate *time.Time
	if value := strings.TrimSpace(row.values["end_date"]); value != "" {
		parsed, err := parseEmployeeImportDate(value, "end_date")
		if err != nil {
			return nil, err
		}
		if parsed.Before(startDate) {
			return nil, fmt.Errorf("end_date cannot be before start_date")
		}
		endDate = &parsed
	}

	var isActive *bool
	if value := strings.TrimSpace(row.values["is_active"]); value != "" {
		parsed, err := parseEmployeeImportBool(value, "is_active")
		if err != nil {
			return nil, err
		}
		isActive = &parsed
	} else if endDate != nil {
		active := !normalizeEmployeeImportDate(*endDate).Before(normalizeEmployeeImportDate(time.Now()))
		isActive = &active
	}

	var baseSalary *decimal.Decimal
	if value := strings.TrimSpace(row.values["base_salary"]); value != "" {
		parsed, err := parseEmployeeImportDecimal(value, "base_salary")
		if err != nil {
			return nil, err
		}
		if !parsed.GreaterThan(decimal.Zero) {
			return nil, fmt.Errorf("base_salary must be greater than zero")
		}
		baseSalary = &parsed
	}

	var salaryEffectiveFrom *time.Time
	if value := strings.TrimSpace(row.values["salary_effective_from"]); value != "" {
		if baseSalary == nil {
			return nil, fmt.Errorf("salary_effective_from requires base_salary")
		}
		parsed, err := parseEmployeeImportDate(value, "salary_effective_from")
		if err != nil {
			return nil, err
		}
		salaryEffectiveFrom = &parsed
	}

	record := &employeeImportRecord{
		createRequest: CreateEmployeeRequest{
			EmployeeNumber:       strings.TrimSpace(row.values["employee_number"]),
			FirstName:            firstName,
			LastName:             lastName,
			PersonalCode:         strings.TrimSpace(row.values["personal_code"]),
			Email:                strings.TrimSpace(row.values["email"]),
			Phone:                strings.TrimSpace(row.values["phone"]),
			Address:              strings.TrimSpace(row.values["address"]),
			BankAccount:          strings.TrimSpace(row.values["bank_account"]),
			StartDate:            startDate,
			Position:             strings.TrimSpace(row.values["position"]),
			Department:           strings.TrimSpace(row.values["department"]),
			EmploymentType:       employmentType,
			ApplyBasicExemption:  applyBasicExemption,
			BasicExemptionAmount: basicExemptionAmount,
			FundedPensionRate:    fundedPensionRate,
		},
		employeeName:   employeeImportDisplayName(firstName, lastName),
		employeeNumber: strings.TrimSpace(row.values["employee_number"]),
		endDate:        endDate,
		isActive:       isActive,
		baseSalary:     baseSalary,
	}
	record.salaryEffectiveFrom = salaryEffectiveFrom
	record.duplicateEmployeeKey = employeeImportNameStartKey(firstName, lastName, startDate)
	return record, nil
}

func buildEmployeeImportIndexes(employees []Employee) *employeeImportIndexes {
	indexes := &employeeImportIndexes{
		employeeNumbers: make(map[string]string, len(employees)),
		personalCodes:   make(map[string]string, len(employees)),
		emails:          make(map[string]string, len(employees)),
		nameStartDates:  make(map[string]string, len(employees)),
	}

	for _, employee := range employees {
		record := &employeeImportRecord{
			createRequest: CreateEmployeeRequest{
				EmployeeNumber: employee.EmployeeNumber,
				FirstName:      employee.FirstName,
				LastName:       employee.LastName,
				PersonalCode:   employee.PersonalCode,
				Email:          employee.Email,
				StartDate:      employee.StartDate,
			},
			employeeName:         employee.FullName(),
			employeeNumber:       employee.EmployeeNumber,
			duplicateEmployeeKey: employeeImportNameStartKey(employee.FirstName, employee.LastName, employee.StartDate),
		}
		indexes.add(record)
	}

	return indexes
}

func (i *employeeImportIndexes) findDuplicate(record *employeeImportRecord) string {
	if value := normalizeEmployeeImportValue(record.createRequest.EmployeeNumber); value != "" {
		if existing, ok := i.employeeNumbers[value]; ok {
			return fmt.Sprintf("employee_number %q already exists (%s)", record.createRequest.EmployeeNumber, existing)
		}
	}
	if value := normalizeEmployeeImportValue(record.createRequest.PersonalCode); value != "" {
		if existing, ok := i.personalCodes[value]; ok {
			return fmt.Sprintf("personal_code %q already exists (%s)", record.createRequest.PersonalCode, existing)
		}
	}
	if value := normalizeEmployeeImportEmail(record.createRequest.Email); value != "" {
		if existing, ok := i.emails[value]; ok {
			return fmt.Sprintf("email %q already exists (%s)", record.createRequest.Email, existing)
		}
	}
	if record.duplicateEmployeeKey != "" {
		if existing, ok := i.nameStartDates[record.duplicateEmployeeKey]; ok {
			return fmt.Sprintf("employee %q with start_date %s already exists (%s)", record.employeeName, record.createRequest.StartDate.Format("2006-01-02"), existing)
		}
	}
	return ""
}

func (i *employeeImportIndexes) add(record *employeeImportRecord) {
	label := record.employeeName
	if record.employeeNumber != "" {
		label = fmt.Sprintf("%s / %s", record.employeeNumber, record.employeeName)
	}

	if value := normalizeEmployeeImportValue(record.createRequest.EmployeeNumber); value != "" {
		i.employeeNumbers[value] = label
	}
	if value := normalizeEmployeeImportValue(record.createRequest.PersonalCode); value != "" {
		i.personalCodes[value] = label
	}
	if value := normalizeEmployeeImportEmail(record.createRequest.Email); value != "" {
		i.emails[value] = label
	}
	if record.duplicateEmployeeKey != "" {
		i.nameStartDates[record.duplicateEmployeeKey] = label
	}
}

func canonicalEmployeeImportHeader(header string) string {
	normalized := strings.ToLower(strings.TrimSpace(header))
	if canonical, ok := employeeImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func detectEmployeeImportDelimiter(content string) rune {
	firstLine := content
	if index := strings.IndexRune(content, '\n'); index >= 0 {
		firstLine = content[:index]
	}

	candidates := []rune{',', ';', '\t'}
	bestDelimiter := ','
	bestCount := -1
	for _, candidate := range candidates {
		count := strings.Count(firstLine, string(candidate))
		if count > bestCount {
			bestCount = count
			bestDelimiter = candidate
		}
	}

	return bestDelimiter
}

func parseEmployeeImportEmploymentType(value string) (EmploymentType, error) {
	normalized := normalizeEmployeeImportValue(strings.ReplaceAll(value, "-", "_"))
	if employmentType, ok := employeeImportEmploymentTypeAliases[normalized]; ok {
		return employmentType, nil
	}
	return "", fmt.Errorf("invalid employment_type %q", value)
}

func parseEmployeeImportBool(value, field string) (bool, error) {
	normalized := normalizeEmployeeImportValue(value)
	if parsed, ok := employeeImportBoolAliases[normalized]; ok {
		return parsed, nil
	}
	return false, fmt.Errorf("invalid %s %q", field, value)
}

func parseEmployeeImportDecimal(value, field string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(normalizeEmployeeImportDecimal(value))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return parsed, nil
}

func parseEmployeeImportDate(value, field string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	layouts := []string{
		"2006-01-02",
		time.RFC3339,
		"02.01.2006",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return normalizeEmployeeImportDate(parsed), nil
		}
	}

	return time.Time{}, fmt.Errorf("%s must be in YYYY-MM-DD format", field)
}

func normalizeEmployeeImportDate(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func normalizeEmployeeImportDecimal(value string) string {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, " ", "")
	if strings.Contains(normalized, ",") && !strings.Contains(normalized, ".") {
		normalized = strings.ReplaceAll(normalized, ",", ".")
	}
	return normalized
}

func normalizeEmployeeImportValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeEmployeeImportEmail(value string) string {
	return normalizeEmployeeImportValue(value)
}

func employeeImportDisplayName(firstName, lastName string) string {
	return strings.TrimSpace(strings.TrimSpace(firstName) + " " + strings.TrimSpace(lastName))
}

func employeeImportNameStartKey(firstName, lastName string, startDate time.Time) string {
	name := normalizeEmployeeImportValue(employeeImportDisplayName(firstName, lastName))
	if name == "" || startDate.IsZero() {
		return ""
	}
	return name + "|" + startDate.Format("2006-01-02")
}
