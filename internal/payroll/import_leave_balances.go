package payroll

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type leaveBalanceImportRow struct {
	rowNumber int
	values    map[string]string
}

type leaveBalanceImportRecord struct {
	rowNumber       int
	year            int
	employeeID      string
	employeeName    string
	employeeNumber  string
	absenceTypeID   string
	absenceTypeCode string
	entitledDays    decimal.Decimal
	carryoverDays   decimal.Decimal
	usedDays        decimal.Decimal
	pendingDays     decimal.Decimal
	notes           string
}

type leaveBalanceAbsenceTypeIndexes struct {
	ids   map[string]*AbsenceType
	codes map[string]*AbsenceType
	names map[string][]*AbsenceType
}

var leaveBalanceImportHeaderAliases = map[string]string{
	"year":                 "year",
	"period_year":          "year",
	"employee_number":      "employee_number",
	"employee_no":          "employee_number",
	"employee_id":          "employee_number",
	"personal_code":        "personal_code",
	"isikukood":            "personal_code",
	"email":                "email",
	"first_name":           "first_name",
	"last_name":            "last_name",
	"absence_type_id":      "absence_type_id",
	"absence_type_code":    "absence_type_code",
	"absence_code":         "absence_type_code",
	"leave_type_code":      "absence_type_code",
	"type_code":            "absence_type_code",
	"absence_type":         "absence_type",
	"absence_type_name":    "absence_type",
	"leave_type":           "absence_type",
	"leave_type_name":      "absence_type",
	"type":                 "absence_type",
	"entitled_days":        "entitled_days",
	"entitlement":          "entitled_days",
	"annual_entitlement":   "entitled_days",
	"carryover_days":       "carryover_days",
	"carry_over_days":      "carryover_days",
	"carried_forward_days": "carryover_days",
	"used_days":            "used_days",
	"taken_days":           "used_days",
	"pending_days":         "pending_days",
	"reserved_days":        "pending_days",
	"notes":                "notes",
}

// ImportLeaveBalancesCSV imports or updates leave balances from CSV.
func (s *AbsenceService) ImportLeaveBalancesCSV(ctx context.Context, schemaName, tenantID string, req *ImportLeaveBalancesRequest) (*ImportLeaveBalancesResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseLeaveBalanceImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no leave balance rows found in CSV")
	}

	employees, err := s.repo.ListEmployees(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing employees: %w", err)
	}
	employeeIndexes := buildPayrollHistoryEmployeeIndexes(employees)

	absenceTypes, err := s.repo.ListAbsenceTypes(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list absence types: %w", err)
	}
	absenceTypeIndexes := buildLeaveBalanceAbsenceTypeIndexes(absenceTypes)

	result := &ImportLeaveBalancesResult{
		FileName: req.FileName,
		Errors:   []ImportLeaveBalanceRowError{},
	}

	for _, row := range rows {
		result.RowsProcessed++

		record, err := buildLeaveBalanceImportRecord(row, employeeIndexes, absenceTypeIndexes)
		if err != nil {
			appendLeaveBalanceRowError(result, row, nil, err.Error())
			continue
		}

		now := time.Now()
		remainingDays := calculateLeaveBalanceRemaining(record.entitledDays, record.carryoverDays, record.usedDays, record.pendingDays)
		existing, err := s.repo.GetLeaveBalance(ctx, schemaName, tenantID, record.employeeID, record.absenceTypeID, record.year)
		switch {
		case err == nil && existing != nil:
			existing.EntitledDays = record.entitledDays
			existing.CarryoverDays = record.carryoverDays
			existing.UsedDays = record.usedDays
			existing.PendingDays = record.pendingDays
			existing.RemainingDays = remainingDays
			existing.Notes = record.notes
			existing.UpdatedAt = now
			if err := s.repo.UpdateLeaveBalance(ctx, schemaName, existing); err != nil {
				appendLeaveBalanceRowError(result, row, record, fmt.Sprintf("update leave balance: %v", err))
				continue
			}
			result.LeaveBalancesUpdated++
		case errors.Is(err, ErrLeaveBalanceNotFound):
			balance := &LeaveBalance{
				ID:            s.uuid.New(),
				TenantID:      tenantID,
				EmployeeID:    record.employeeID,
				AbsenceTypeID: record.absenceTypeID,
				Year:          record.year,
				EntitledDays:  record.entitledDays,
				CarryoverDays: record.carryoverDays,
				UsedDays:      record.usedDays,
				PendingDays:   record.pendingDays,
				RemainingDays: remainingDays,
				Notes:         record.notes,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := s.repo.CreateLeaveBalance(ctx, schemaName, balance); err != nil {
				appendLeaveBalanceRowError(result, row, record, fmt.Sprintf("create leave balance: %v", err))
				continue
			}
			result.LeaveBalancesCreated++
		default:
			appendLeaveBalanceRowError(result, row, record, fmt.Sprintf("get leave balance: %v", err))
		}
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parseLeaveBalanceImportRows(content string) ([]leaveBalanceImportRow, error) {
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
	hasYear := false
	hasAbsenceType := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalLeaveBalanceImportHeader(header)
		switch canonicalHeaders[i] {
		case "year":
			hasYear = true
		case "absence_type_id", "absence_type_code", "absence_type":
			hasAbsenceType = true
		}
	}

	if !hasYear {
		return nil, fmt.Errorf("missing required year column")
	}
	if !hasAbsenceType {
		return nil, fmt.Errorf("missing required absence_type_code, absence_type, or absence_type_id column")
	}

	rows := make([]leaveBalanceImportRow, 0)
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
		rows = append(rows, leaveBalanceImportRow{
			rowNumber: rowNumber,
			values:    rowValues,
		})
	}

	return rows, nil
}

func buildLeaveBalanceImportRecord(
	row leaveBalanceImportRow,
	employeeIndexes *payrollHistoryEmployeeIndexes,
	absenceTypeIndexes *leaveBalanceAbsenceTypeIndexes,
) (*leaveBalanceImportRecord, error) {
	year, err := parsePayrollHistoryImportYear(row.values["year"])
	if err != nil {
		return nil, err
	}

	employee, employeeName, err := findPayrollHistoryEmployee(row.values, employeeIndexes)
	if err != nil {
		return nil, err
	}

	absenceType, err := findLeaveBalanceAbsenceType(row.values, absenceTypeIndexes)
	if err != nil {
		return nil, err
	}

	entitledDays, err := parseOptionalPayrollHistoryDecimal(row.values["entitled_days"], "entitled_days")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(row.values["entitled_days"]) == "" {
		entitledDays = absenceType.DefaultDaysPerYear
	}

	carryoverDays, err := parseOptionalPayrollHistoryDecimal(row.values["carryover_days"], "carryover_days")
	if err != nil {
		return nil, err
	}
	usedDays, err := parseOptionalPayrollHistoryDecimal(row.values["used_days"], "used_days")
	if err != nil {
		return nil, err
	}
	pendingDays, err := parseOptionalPayrollHistoryDecimal(row.values["pending_days"], "pending_days")
	if err != nil {
		return nil, err
	}

	employeeNumber := strings.TrimSpace(row.values["employee_number"])
	if employeeNumber == "" {
		employeeNumber = employee.EmployeeNumber
	}

	return &leaveBalanceImportRecord{
		rowNumber:       row.rowNumber,
		year:            year,
		employeeID:      employee.ID,
		employeeName:    employeeName,
		employeeNumber:  employeeNumber,
		absenceTypeID:   absenceType.ID,
		absenceTypeCode: absenceType.Code,
		entitledDays:    entitledDays,
		carryoverDays:   carryoverDays,
		usedDays:        usedDays,
		pendingDays:     pendingDays,
		notes:           strings.TrimSpace(row.values["notes"]),
	}, nil
}

func buildLeaveBalanceAbsenceTypeIndexes(absenceTypes []AbsenceType) *leaveBalanceAbsenceTypeIndexes {
	indexes := &leaveBalanceAbsenceTypeIndexes{
		ids:   make(map[string]*AbsenceType, len(absenceTypes)),
		codes: make(map[string]*AbsenceType, len(absenceTypes)),
		names: make(map[string][]*AbsenceType, len(absenceTypes)),
	}

	for i := range absenceTypes {
		absenceType := &absenceTypes[i]
		if value := normalizeEmployeeImportValue(absenceType.ID); value != "" {
			indexes.ids[value] = absenceType
		}
		if value := normalizeEmployeeImportValue(absenceType.Code); value != "" {
			indexes.codes[value] = absenceType
		}
		if value := normalizeEmployeeImportValue(absenceType.Name); value != "" {
			indexes.names[value] = append(indexes.names[value], absenceType)
		}
		if value := normalizeEmployeeImportValue(absenceType.NameET); value != "" {
			indexes.names[value] = append(indexes.names[value], absenceType)
		}
	}

	return indexes
}

func findLeaveBalanceAbsenceType(values map[string]string, indexes *leaveBalanceAbsenceTypeIndexes) (*AbsenceType, error) {
	candidates := make(map[string]*AbsenceType)

	if typeID := strings.TrimSpace(values["absence_type_id"]); typeID != "" {
		match, ok := indexes.ids[normalizeEmployeeImportValue(typeID)]
		if !ok {
			return nil, fmt.Errorf("absence_type_id %q not found", typeID)
		}
		candidates[match.ID] = match
	}
	if code := strings.TrimSpace(values["absence_type_code"]); code != "" {
		match, ok := indexes.codes[normalizeEmployeeImportValue(code)]
		if !ok {
			return nil, fmt.Errorf("absence_type_code %q not found", code)
		}
		candidates[match.ID] = match
	}
	if name := strings.TrimSpace(values["absence_type"]); name != "" {
		matches := indexes.names[normalizeEmployeeImportValue(name)]
		if len(matches) == 0 {
			return nil, fmt.Errorf("absence_type %q not found", name)
		}
		if len(matches) > 1 && len(candidates) == 0 {
			return nil, fmt.Errorf("absence_type %q matches multiple types; use absence_type_code", name)
		}
		if len(matches) == 1 {
			candidates[matches[0].ID] = matches[0]
		} else {
			matchedCandidate := false
			for _, match := range matches {
				if _, ok := candidates[match.ID]; ok {
					matchedCandidate = true
					break
				}
			}
			if !matchedCandidate {
				return nil, fmt.Errorf("absence type identifiers do not match the same type")
			}
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("absence_type_code, absence_type, or absence_type_id is required")
	}
	if len(candidates) > 1 {
		return nil, fmt.Errorf("absence type identifiers do not match the same type")
	}

	for _, absenceType := range candidates {
		return absenceType, nil
	}

	return nil, fmt.Errorf("absence type not found")
}

func appendLeaveBalanceRowError(result *ImportLeaveBalancesResult, row leaveBalanceImportRow, record *leaveBalanceImportRecord, message string) {
	result.RowsSkipped++
	rowError := ImportLeaveBalanceRowError{
		Row:             row.rowNumber,
		Year:            parseOptionalInt(row.values["year"]),
		EmployeeName:    employeeImportDisplayName(row.values["first_name"], row.values["last_name"]),
		EmployeeNumber:  strings.TrimSpace(row.values["employee_number"]),
		AbsenceTypeCode: strings.TrimSpace(row.values["absence_type_code"]),
		Message:         message,
	}
	if record != nil {
		rowError.Year = record.year
		rowError.EmployeeName = record.employeeName
		rowError.EmployeeNumber = record.employeeNumber
		rowError.AbsenceTypeCode = record.absenceTypeCode
	}
	result.Errors = append(result.Errors, rowError)
}

func canonicalLeaveBalanceImportHeader(header string) string {
	normalized := strings.ToLower(strings.TrimSpace(header))
	if canonical, ok := leaveBalanceImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func calculateLeaveBalanceRemaining(entitledDays, carryoverDays, usedDays, pendingDays decimal.Decimal) decimal.Decimal {
	return entitledDays.Add(carryoverDays).Sub(usedDays).Sub(pendingDays)
}
