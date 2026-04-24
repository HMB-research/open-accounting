package payroll

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type payrollHistoryImportRow struct {
	rowNumber int
	values    map[string]string
}

type payrollHistoryImportGroup struct {
	periodYear  int
	periodMonth int
	status      PayrollStatus
	paymentDate *time.Time
	notes       string
	records     []*payrollHistoryImportRecord
	employeeIDs map[string]int
}

type payrollHistoryImportRecord struct {
	rowNumber      int
	periodYear     int
	periodMonth    int
	status         PayrollStatus
	paymentDate    *time.Time
	notes          string
	employeeID     string
	employeeName   string
	employeeNumber string
	payslip        Payslip
}

type payrollHistoryEmployeeIndexes struct {
	employeeNumbers map[string]*Employee
	personalCodes   map[string]*Employee
	emails          map[string]*Employee
	names           map[string][]*Employee
}

var payrollHistoryImportHeaderAliases = map[string]string{
	"period_year":                     "period_year",
	"year":                            "period_year",
	"payroll_year":                    "period_year",
	"period_month":                    "period_month",
	"month":                           "period_month",
	"payroll_month":                   "period_month",
	"status":                          "status",
	"run_status":                      "status",
	"payment_date":                    "payment_date",
	"pay_date":                        "payment_date",
	"notes":                           "notes",
	"employee_number":                 "employee_number",
	"employee_no":                     "employee_number",
	"employee_id":                     "employee_number",
	"personal_code":                   "personal_code",
	"isikukood":                       "personal_code",
	"email":                           "email",
	"first_name":                      "first_name",
	"last_name":                       "last_name",
	"gross_salary":                    "gross_salary",
	"gross":                           "gross_salary",
	"taxable_income":                  "taxable_income",
	"income_tax":                      "income_tax",
	"unemployment_insurance_employee": "unemployment_insurance_employee",
	"unemployment_employee":           "unemployment_insurance_employee",
	"unemployment_insurance_ee":       "unemployment_insurance_employee",
	"funded_pension":                  "funded_pension",
	"pension":                         "funded_pension",
	"other_deductions":                "other_deductions",
	"net_salary":                      "net_salary",
	"net":                             "net_salary",
	"social_tax":                      "social_tax",
	"unemployment_insurance_employer": "unemployment_insurance_employer",
	"unemployment_employer":           "unemployment_insurance_employer",
	"unemployment_insurance_er":       "unemployment_insurance_employer",
	"total_employer_cost":             "total_employer_cost",
	"employer_cost":                   "total_employer_cost",
	"basic_exemption_applied":         "basic_exemption_applied",
	"payment_status":                  "payment_status",
	"paid_at":                         "paid_at",
}

var payrollHistoryImportStatusAliases = map[string]PayrollStatus{
	"approved": PayrollApproved,
	"paid":     PayrollPaid,
	"declared": PayrollDeclared,
}

var payrollHistoryImportPaymentStatusAliases = map[string]string{
	"pending":   "PENDING",
	"paid":      "PAID",
	"cancelled": "CANCELLED", //nolint:misspell // External payment status values use existing API/database spelling.
	"canceled":  "CANCELLED", //nolint:misspell // External payment status values use existing API/database spelling.
}

// ImportPayrollHistoryCSV imports finalized historical payroll runs and payslips from CSV.
func (s *Service) ImportPayrollHistoryCSV(
	ctx context.Context,
	schemaName, tenantID, userID string,
	req *ImportPayrollHistoryRequest,
) (*ImportPayrollHistoryResult, error) {
	if strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parsePayrollHistoryImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no payroll rows found in CSV")
	}

	employees, err := s.repo.ListEmployees(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing employees: %w", err)
	}
	indexes := buildPayrollHistoryEmployeeIndexes(employees)

	result := &ImportPayrollHistoryResult{
		FileName: req.FileName,
		Errors:   []ImportPayrollHistoryRowError{},
	}

	groups := make(map[string]*payrollHistoryImportGroup)
	yearSet := make(map[int]struct{})

	for _, row := range rows {
		result.RowsProcessed++

		record, err := buildPayrollHistoryImportRecord(row, indexes)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportPayrollHistoryRowError{
				Row:            row.rowNumber,
				PeriodYear:     parseOptionalInt(row.values["period_year"]),
				PeriodMonth:    parseOptionalInt(row.values["period_month"]),
				EmployeeName:   employeeImportDisplayName(row.values["first_name"], row.values["last_name"]),
				EmployeeNumber: strings.TrimSpace(row.values["employee_number"]),
				Message:        err.Error(),
			})
			continue
		}

		key := payrollHistoryGroupKey(record.periodYear, record.periodMonth)
		group, ok := groups[key]
		if !ok {
			group = &payrollHistoryImportGroup{
				periodYear:  record.periodYear,
				periodMonth: record.periodMonth,
				status:      record.status,
				paymentDate: record.paymentDate,
				notes:       record.notes,
				records:     []*payrollHistoryImportRecord{},
				employeeIDs: map[string]int{},
			}
			groups[key] = group
		}

		if message := validatePayrollHistoryGroupConsistency(group, record); message != "" {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportPayrollHistoryRowError{
				Row:            record.rowNumber,
				PeriodYear:     record.periodYear,
				PeriodMonth:    record.periodMonth,
				EmployeeName:   record.employeeName,
				EmployeeNumber: record.employeeNumber,
				Message:        message,
			})
			continue
		}

		if previousRow, duplicate := group.employeeIDs[record.employeeID]; duplicate {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportPayrollHistoryRowError{
				Row:            record.rowNumber,
				PeriodYear:     record.periodYear,
				PeriodMonth:    record.periodMonth,
				EmployeeName:   record.employeeName,
				EmployeeNumber: record.employeeNumber,
				Message:        fmt.Sprintf("employee already has a payslip in this payroll period (row %d)", previousRow),
			})
			continue
		}

		group.employeeIDs[record.employeeID] = record.rowNumber
		group.records = append(group.records, record)
		yearSet[record.periodYear] = struct{}{}
	}

	existingRuns := make(map[string]struct{})
	for year := range yearSet {
		runs, err := s.repo.ListPayrollRuns(ctx, schemaName, tenantID, year)
		if err != nil {
			return nil, fmt.Errorf("list existing payroll runs for %d: %w", year, err)
		}
		for _, run := range runs {
			existingRuns[payrollHistoryGroupKey(run.PeriodYear, run.PeriodMonth)] = struct{}{}
		}
	}

	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)

	for _, key := range groupKeys {
		group := groups[key]
		if _, exists := existingRuns[key]; exists {
			for _, record := range group.records {
				result.RowsSkipped++
				result.Errors = append(result.Errors, ImportPayrollHistoryRowError{
					Row:            record.rowNumber,
					PeriodYear:     record.periodYear,
					PeriodMonth:    record.periodMonth,
					EmployeeName:   record.employeeName,
					EmployeeNumber: record.employeeNumber,
					Message:        fmt.Sprintf("payroll run already exists for %04d-%02d", record.periodYear, record.periodMonth),
				})
			}
			continue
		}

		totalGross := decimal.Zero
		totalNet := decimal.Zero
		totalEmployerCost := decimal.Zero
		for _, record := range group.records {
			totalGross = totalGross.Add(record.payslip.GrossSalary)
			totalNet = totalNet.Add(record.payslip.NetSalary)
			totalEmployerCost = totalEmployerCost.Add(record.payslip.TotalEmployerCost)
		}

		tx, err := s.repo.BeginTx(ctx)
		if err != nil {
			for _, record := range group.records {
				result.RowsSkipped++
				result.Errors = append(result.Errors, ImportPayrollHistoryRowError{
					Row:            record.rowNumber,
					PeriodYear:     record.periodYear,
					PeriodMonth:    record.periodMonth,
					EmployeeName:   record.employeeName,
					EmployeeNumber: record.employeeNumber,
					Message:        fmt.Sprintf("begin transaction: %v", err),
				})
			}
			continue
		}

		txRepo := s.repo.WithTx(tx)
		approvedAt := payrollHistoryApprovedAt(group.status, group.paymentDate)
		run := &PayrollRun{
			ID:                s.uuid.New(),
			TenantID:          tenantID,
			PeriodYear:        group.periodYear,
			PeriodMonth:       group.periodMonth,
			Status:            group.status,
			PaymentDate:       group.paymentDate,
			TotalGross:        totalGross,
			TotalNet:          totalNet,
			TotalEmployerCost: totalEmployerCost,
			Notes:             group.notes,
			CreatedBy:         userID,
			ApprovedBy:        payrollHistoryApprovedBy(group.status, userID),
			ApprovedAt:        approvedAt,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		if err := txRepo.CreatePayrollRun(ctx, schemaName, run); err != nil {
			_ = tx.Rollback(ctx)
			appendPayrollHistoryGroupError(result, group, fmt.Sprintf("create payroll run: %v", err))
			continue
		}

		groupFailed := false
		for _, record := range group.records {
			payslip := record.payslip
			payslip.ID = s.uuid.New()
			payslip.TenantID = tenantID
			payslip.PayrollRunID = run.ID
			payslip.EmployeeID = record.employeeID

			if err := txRepo.CreatePayslip(ctx, schemaName, &payslip); err != nil {
				_ = tx.Rollback(ctx)
				appendPayrollHistoryGroupError(result, group, fmt.Sprintf("create payslip: %v", err))
				groupFailed = true
				break
			}
		}
		if groupFailed {
			continue
		}

		if err := tx.Commit(ctx); err != nil {
			_ = tx.Rollback(ctx)
			appendPayrollHistoryGroupError(result, group, fmt.Sprintf("commit transaction: %v", err))
			continue
		}

		existingRuns[key] = struct{}{}
		result.PayrollRunsCreated++
		result.PayslipsCreated += len(group.records)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}

	return result, nil
}

func parsePayrollHistoryImportRows(content string) ([]payrollHistoryImportRow, error) {
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
	hasMonth := false
	hasGross := false
	for i, header := range headers {
		canonicalHeaders[i] = canonicalPayrollHistoryImportHeader(header)
		switch canonicalHeaders[i] {
		case "period_year":
			hasYear = true
		case "period_month":
			hasMonth = true
		case "gross_salary":
			hasGross = true
		}
	}

	if !hasYear || !hasMonth || !hasGross {
		return nil, fmt.Errorf("missing required period_year, period_month, or gross_salary column")
	}

	rows := make([]payrollHistoryImportRow, 0)
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

		rows = append(rows, payrollHistoryImportRow{
			rowNumber: rowNumber,
			values:    rowValues,
		})
	}

	return rows, nil
}

func buildPayrollHistoryImportRecord(row payrollHistoryImportRow, indexes *payrollHistoryEmployeeIndexes) (*payrollHistoryImportRecord, error) {
	periodYear, err := parsePayrollHistoryImportYear(row.values["period_year"])
	if err != nil {
		return nil, err
	}
	periodMonth, err := parsePayrollHistoryImportMonth(row.values["period_month"])
	if err != nil {
		return nil, err
	}
	status, err := parsePayrollHistoryImportStatus(row.values["status"])
	if err != nil {
		return nil, err
	}

	var paymentDate *time.Time
	if value := strings.TrimSpace(row.values["payment_date"]); value != "" {
		parsed, err := parseEmployeeImportDate(value, "payment_date")
		if err != nil {
			return nil, err
		}
		paymentDate = &parsed
	}

	employee, employeeName, err := findPayrollHistoryEmployee(row.values, indexes)
	if err != nil {
		return nil, err
	}

	employeeNumber := strings.TrimSpace(row.values["employee_number"])
	if employeeNumber == "" {
		employeeNumber = employee.EmployeeNumber
	}

	basicExemptionApplied, err := parseOptionalPayrollHistoryDecimal(row.values["basic_exemption_applied"], "basic_exemption_applied")
	if err != nil {
		return nil, err
	}

	grossSalary, err := parseRequiredPayrollHistoryDecimal(row.values["gross_salary"], "gross_salary")
	if err != nil {
		return nil, err
	}
	if !grossSalary.GreaterThan(decimal.Zero) {
		return nil, fmt.Errorf("gross_salary must be greater than zero")
	}

	taxableIncome, err := parseOptionalPayrollHistoryDecimal(row.values["taxable_income"], "taxable_income")
	if err != nil {
		return nil, err
	}
	if taxableIncome.IsZero() && strings.TrimSpace(row.values["taxable_income"]) == "" {
		taxableIncome = grossSalary.Sub(basicExemptionApplied)
		if taxableIncome.IsNegative() {
			taxableIncome = decimal.Zero
		}
	}

	incomeTax, err := parseOptionalPayrollHistoryDecimal(row.values["income_tax"], "income_tax")
	if err != nil {
		return nil, err
	}
	unemploymentEE, err := parseOptionalPayrollHistoryDecimal(row.values["unemployment_insurance_employee"], "unemployment_insurance_employee")
	if err != nil {
		return nil, err
	}
	fundedPension, err := parseOptionalPayrollHistoryDecimal(row.values["funded_pension"], "funded_pension")
	if err != nil {
		return nil, err
	}
	otherDeductions, err := parseOptionalPayrollHistoryDecimal(row.values["other_deductions"], "other_deductions")
	if err != nil {
		return nil, err
	}
	netSalary, err := parseOptionalPayrollHistoryDecimal(row.values["net_salary"], "net_salary")
	if err != nil {
		return nil, err
	}
	if netSalary.IsZero() && strings.TrimSpace(row.values["net_salary"]) == "" {
		netSalary = grossSalary.Sub(incomeTax).Sub(unemploymentEE).Sub(fundedPension).Sub(otherDeductions)
	}

	socialTax, err := parseOptionalPayrollHistoryDecimal(row.values["social_tax"], "social_tax")
	if err != nil {
		return nil, err
	}
	unemploymentER, err := parseOptionalPayrollHistoryDecimal(row.values["unemployment_insurance_employer"], "unemployment_insurance_employer")
	if err != nil {
		return nil, err
	}
	totalEmployerCost, err := parseOptionalPayrollHistoryDecimal(row.values["total_employer_cost"], "total_employer_cost")
	if err != nil {
		return nil, err
	}
	if totalEmployerCost.IsZero() && strings.TrimSpace(row.values["total_employer_cost"]) == "" {
		totalEmployerCost = grossSalary.Add(socialTax).Add(unemploymentER)
	}

	paymentStatus, err := parsePayrollHistoryPaymentStatus(row.values["payment_status"], status)
	if err != nil {
		return nil, err
	}

	var paidAt *time.Time
	if value := strings.TrimSpace(row.values["paid_at"]); value != "" {
		parsed, err := parseEmployeeImportDate(value, "paid_at")
		if err != nil {
			return nil, err
		}
		paidAt = &parsed
	} else if paymentStatus == "PAID" && paymentDate != nil {
		paidAt = paymentDate
	}

	return &payrollHistoryImportRecord{
		rowNumber:      row.rowNumber,
		periodYear:     periodYear,
		periodMonth:    periodMonth,
		status:         status,
		paymentDate:    paymentDate,
		notes:          strings.TrimSpace(row.values["notes"]),
		employeeID:     employee.ID,
		employeeName:   employeeName,
		employeeNumber: employeeNumber,
		payslip: Payslip{
			EmployeeID:              employee.ID,
			GrossSalary:             grossSalary,
			TaxableIncome:           taxableIncome,
			IncomeTax:               incomeTax,
			UnemploymentInsuranceEE: unemploymentEE,
			FundedPension:           fundedPension,
			OtherDeductions:         otherDeductions,
			NetSalary:               netSalary,
			SocialTax:               socialTax,
			UnemploymentInsuranceER: unemploymentER,
			TotalEmployerCost:       totalEmployerCost,
			BasicExemptionApplied:   basicExemptionApplied,
			PaymentStatus:           paymentStatus,
			PaidAt:                  paidAt,
			CreatedAt:               time.Now(),
		},
	}, nil
}

func buildPayrollHistoryEmployeeIndexes(employees []Employee) *payrollHistoryEmployeeIndexes {
	indexes := &payrollHistoryEmployeeIndexes{
		employeeNumbers: make(map[string]*Employee, len(employees)),
		personalCodes:   make(map[string]*Employee, len(employees)),
		emails:          make(map[string]*Employee, len(employees)),
		names:           make(map[string][]*Employee, len(employees)),
	}

	for i := range employees {
		employee := &employees[i]
		if value := normalizeEmployeeImportValue(employee.EmployeeNumber); value != "" {
			indexes.employeeNumbers[value] = employee
		}
		if value := normalizeEmployeeImportValue(employee.PersonalCode); value != "" {
			indexes.personalCodes[value] = employee
		}
		if value := normalizeEmployeeImportEmail(employee.Email); value != "" {
			indexes.emails[value] = employee
		}
		if value := payrollHistoryNameKey(employee.FirstName, employee.LastName); value != "" {
			indexes.names[value] = append(indexes.names[value], employee)
		}
	}

	return indexes
}

func findPayrollHistoryEmployee(values map[string]string, indexes *payrollHistoryEmployeeIndexes) (*Employee, string, error) {
	candidates := make(map[string]*Employee)

	if employeeNumber := strings.TrimSpace(values["employee_number"]); employeeNumber != "" {
		match, ok := indexes.employeeNumbers[normalizeEmployeeImportValue(employeeNumber)]
		if !ok {
			return nil, "", fmt.Errorf("employee_number %q not found", employeeNumber)
		}
		candidates[match.ID] = match
	}
	if personalCode := strings.TrimSpace(values["personal_code"]); personalCode != "" {
		match, ok := indexes.personalCodes[normalizeEmployeeImportValue(personalCode)]
		if !ok {
			return nil, "", fmt.Errorf("personal_code %q not found", personalCode)
		}
		candidates[match.ID] = match
	}
	if email := strings.TrimSpace(values["email"]); email != "" {
		match, ok := indexes.emails[normalizeEmployeeImportEmail(email)]
		if !ok {
			return nil, "", fmt.Errorf("email %q not found", email)
		}
		candidates[match.ID] = match
	}

	firstName := strings.TrimSpace(values["first_name"])
	lastName := strings.TrimSpace(values["last_name"])
	if firstName != "" || lastName != "" {
		if firstName == "" || lastName == "" {
			return nil, "", fmt.Errorf("first_name and last_name must both be provided when matching by name")
		}

		matches := indexes.names[payrollHistoryNameKey(firstName, lastName)]
		if len(matches) == 0 {
			return nil, "", fmt.Errorf("employee %q not found", employeeImportDisplayName(firstName, lastName))
		}
		if len(matches) > 1 && len(candidates) == 0 {
			return nil, "", fmt.Errorf("employee %q matches multiple employees; use employee_number or personal_code", employeeImportDisplayName(firstName, lastName))
		}
		if len(matches) == 1 {
			candidates[matches[0].ID] = matches[0]
		}
	}

	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("employee_number, personal_code, email, or first_name/last_name is required")
	}
	if len(candidates) > 1 {
		return nil, "", fmt.Errorf("employee identifiers do not match the same employee")
	}

	for _, employee := range candidates {
		return employee, employee.FullName(), nil
	}

	return nil, "", fmt.Errorf("employee not found")
}

func validatePayrollHistoryGroupConsistency(group *payrollHistoryImportGroup, record *payrollHistoryImportRecord) string {
	if group.status != record.status {
		return "status must be consistent for each payroll period"
	}
	if !payrollHistoryDatesEqual(group.paymentDate, record.paymentDate) {
		return "payment_date must be consistent for each payroll period"
	}
	if group.notes != record.notes {
		return "notes must be consistent for each payroll period"
	}
	return ""
}

func appendPayrollHistoryGroupError(result *ImportPayrollHistoryResult, group *payrollHistoryImportGroup, message string) {
	for _, record := range group.records {
		result.RowsSkipped++
		result.Errors = append(result.Errors, ImportPayrollHistoryRowError{
			Row:            record.rowNumber,
			PeriodYear:     record.periodYear,
			PeriodMonth:    record.periodMonth,
			EmployeeName:   record.employeeName,
			EmployeeNumber: record.employeeNumber,
			Message:        message,
		})
	}
}

func canonicalPayrollHistoryImportHeader(header string) string {
	normalized := strings.ToLower(strings.TrimSpace(header))
	if canonical, ok := payrollHistoryImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func parsePayrollHistoryImportYear(value string) (int, error) {
	year, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || year < 2020 || year > 2100 {
		return 0, fmt.Errorf("period_year must be between 2020 and 2100")
	}
	return year, nil
}

func parsePayrollHistoryImportMonth(value string) (int, error) {
	month, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || month < 1 || month > 12 {
		return 0, fmt.Errorf("period_month must be between 1 and 12")
	}
	return month, nil
}

func parsePayrollHistoryImportStatus(value string) (PayrollStatus, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return PayrollPaid, nil
	}

	status, ok := payrollHistoryImportStatusAliases[normalizeEmployeeImportValue(trimmed)]
	if !ok {
		return "", fmt.Errorf("status must be APPROVED, PAID, or DECLARED")
	}
	return status, nil
}

func parsePayrollHistoryPaymentStatus(value string, runStatus PayrollStatus) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		if runStatus == PayrollApproved {
			return "PENDING", nil
		}
		return "PAID", nil
	}

	status, ok := payrollHistoryImportPaymentStatusAliases[normalizeEmployeeImportValue(trimmed)]
	if !ok {
		return "", fmt.Errorf("payment_status must be PENDING, PAID, or CANCELLED") //nolint:misspell // Existing API/database spelling.
	}
	return status, nil
}

func parseRequiredPayrollHistoryDecimal(value, field string) (decimal.Decimal, error) {
	parsed, err := parseOptionalPayrollHistoryDecimal(value, field)
	if err != nil {
		return decimal.Zero, err
	}
	if strings.TrimSpace(value) == "" {
		return decimal.Zero, fmt.Errorf("%s is required", field)
	}
	return parsed, nil
}

func parseOptionalPayrollHistoryDecimal(value, field string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, nil
	}
	parsed, err := parseEmployeeImportDecimal(trimmed, field)
	if err != nil {
		return decimal.Zero, err
	}
	if parsed.IsNegative() {
		return decimal.Zero, fmt.Errorf("%s must be zero or greater", field)
	}
	return parsed, nil
}

func parseOptionalInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return parsed
}

func payrollHistoryGroupKey(year, month int) string {
	return fmt.Sprintf("%04d-%02d", year, month)
}

func payrollHistoryNameKey(firstName, lastName string) string {
	return normalizeEmployeeImportValue(employeeImportDisplayName(firstName, lastName))
}

func payrollHistoryDatesEqual(left, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return normalizeEmployeeImportDate(*left).Equal(normalizeEmployeeImportDate(*right))
	}
}

func payrollHistoryApprovedBy(status PayrollStatus, userID string) string {
	if userID == "" {
		return ""
	}
	if status == PayrollApproved || status == PayrollPaid || status == PayrollDeclared {
		return userID
	}
	return ""
}

func payrollHistoryApprovedAt(status PayrollStatus, paymentDate *time.Time) *time.Time {
	if status != PayrollApproved && status != PayrollPaid && status != PayrollDeclared {
		return nil
	}
	if paymentDate != nil {
		approvedAt := normalizeEmployeeImportDate(*paymentDate)
		return &approvedAt
	}
	now := normalizeEmployeeImportDate(time.Now())
	return &now
}
