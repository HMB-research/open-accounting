package payroll

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type tsdHistoryImportRow struct {
	rowNumber int
	values    map[string]string
}

type tsdHistoryImportGroup struct {
	periodYear    int
	periodMonth   int
	status        TSDStatus
	submittedAt   *time.Time
	emtaReference string
	records       []*tsdHistoryImportRecord
	employeeIDs   map[string]int
}

type tsdHistoryImportRecord struct {
	rowNumber      int
	periodYear     int
	periodMonth    int
	status         TSDStatus
	submittedAt    *time.Time
	emtaReference  string
	employeeName   string
	employeeNumber string
	row            TSDRow
}

var tsdHistoryImportHeaderAliases = map[string]string{
	"period_year":                     "period_year",
	"declaration_year":                "period_year",
	"tsd_year":                        "period_year",
	"year":                            "period_year",
	"period_month":                    "period_month",
	"declaration_month":               "period_month",
	"tsd_month":                       "period_month",
	"month":                           "period_month",
	"status":                          "status",
	"declaration_status":              "status",
	"submitted_at":                    "submitted_at",
	"submitted_date":                  "submitted_at",
	"submission_date":                 "submitted_at",
	"emta_reference":                  "emta_reference",
	"emta_ref":                        "emta_reference",
	"submission_reference":            "emta_reference",
	"employee_number":                 "employee_number",
	"employee_no":                     "employee_number",
	"employee_id":                     "employee_number",
	"personal_code":                   "personal_code",
	"isikukood":                       "personal_code",
	"email":                           "email",
	"e_mail":                          "email",
	"name":                            "name",
	"first_name":                      "first_name",
	"last_name":                       "last_name",
	"payment_type":                    "payment_type",
	"payment_code":                    "payment_type",
	"tsd_payment_type":                "payment_type",
	"gross_payment":                   "gross_payment",
	"gross_salary":                    "gross_payment",
	"gross":                           "gross_payment",
	"basic_exemption":                 "basic_exemption",
	"basic_exemption_applied":         "basic_exemption",
	"taxable_amount":                  "taxable_amount",
	"taxable_income":                  "taxable_amount",
	"income_tax":                      "income_tax",
	"social_tax":                      "social_tax",
	"unemployment_insurance_employer": "unemployment_insurance_employer",
	"unemployment_employer":           "unemployment_insurance_employer",
	"unemployment_insurance_er":       "unemployment_insurance_employer",
	"unemployment_insurance_employee": "unemployment_insurance_employee",
	"unemployment_employee":           "unemployment_insurance_employee",
	"unemployment_insurance_ee":       "unemployment_insurance_employee",
	"funded_pension":                  "funded_pension",
	"pension":                         "funded_pension",
}

var tsdHistoryImportStatusAliases = map[string]TSDStatus{
	"":          TSDDraft,
	"draft":     TSDDraft,
	"submitted": TSDSubmitted,
	"filed":     TSDSubmitted,
	"accepted":  TSDAccepted,
	"approved":  TSDAccepted,
	"confirmed": TSDAccepted,
	"rejected":  TSDRejected,
}

// ImportTSDHistoryCSV imports historical TSD declarations and rows from CSV.
func (s *Service) ImportTSDHistoryCSV(
	ctx context.Context,
	schemaName, tenantID string,
	req *ImportTSDHistoryRequest,
) (*ImportTSDHistoryResult, error) {
	if req == nil || strings.TrimSpace(req.CSVContent) == "" {
		return nil, fmt.Errorf("csv_content is required")
	}

	rows, err := parseTSDHistoryImportRows(req.CSVContent)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no TSD rows found in CSV")
	}

	employees, err := s.repo.ListEmployees(ctx, schemaName, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("list existing employees: %w", err)
	}
	indexes := buildPayrollHistoryEmployeeIndexes(employees)

	result := &ImportTSDHistoryResult{
		FileName: req.FileName,
		Errors:   []ImportTSDHistoryRowError{},
	}
	groups := make(map[string]*tsdHistoryImportGroup)
	for _, row := range rows {
		result.RowsProcessed++

		record, err := buildTSDHistoryImportRecord(row, indexes)
		if err != nil {
			result.RowsSkipped++
			result.Errors = append(result.Errors, ImportTSDHistoryRowError{
				Row:            row.rowNumber,
				PeriodYear:     parseOptionalInt(row.values["period_year"]),
				PeriodMonth:    parseOptionalInt(row.values["period_month"]),
				EmployeeName:   payrollHistoryImportEmployeeName(row.values),
				EmployeeNumber: strings.TrimSpace(row.values["employee_number"]),
				Message:        err.Error(),
			})
			continue
		}

		key := payrollHistoryGroupKey(record.periodYear, record.periodMonth)
		group, ok := groups[key]
		if !ok {
			group = &tsdHistoryImportGroup{
				periodYear:    record.periodYear,
				periodMonth:   record.periodMonth,
				status:        record.status,
				submittedAt:   record.submittedAt,
				emtaReference: record.emtaReference,
				records:       []*tsdHistoryImportRecord{},
				employeeIDs:   map[string]int{},
			}
			groups[key] = group
		}

		if message := validateTSDHistoryGroupConsistency(group, record); message != "" {
			appendTSDHistoryRowError(result, record, message)
			continue
		}

		if previousRow, duplicate := group.employeeIDs[record.row.EmployeeID]; duplicate {
			appendTSDHistoryRowError(result, record, fmt.Sprintf("employee already has a TSD row in this period (row %d)", previousRow))
			continue
		}
		group.employeeIDs[record.row.EmployeeID] = record.rowNumber
		group.records = append(group.records, record)
	}

	groupKeys := make([]string, 0, len(groups))
	for key := range groups {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)

	for _, key := range groupKeys {
		group := groups[key]
		if len(group.records) == 0 {
			continue
		}

		existing, err := s.repo.GetTSD(ctx, schemaName, tenantID, group.periodYear, group.periodMonth)
		if err != nil && !errors.Is(err, ErrTSDDeclarationNotFound) {
			return nil, fmt.Errorf("check existing TSD declaration for %04d-%02d: %w", group.periodYear, group.periodMonth, err)
		}
		if existing != nil {
			for _, record := range group.records {
				appendTSDHistoryRowError(result, record, fmt.Sprintf("TSD declaration already exists for %04d-%02d", group.periodYear, group.periodMonth))
			}
			continue
		}

		declaration := buildTSDHistoryDeclaration(tenantID, group)
		declaration.ID = s.uuid.New()
		rows := make([]TSDRow, 0, len(group.records))
		for _, record := range group.records {
			row := record.row
			row.ID = s.uuid.New()
			row.TenantID = tenantID
			row.DeclarationID = declaration.ID
			row.CreatedAt = time.Now()
			rows = append(rows, row)
		}

		err = s.repo.WithTransaction(ctx, func(txRepo Repository) error {
			if err := txRepo.CreateTSDDeclaration(ctx, schemaName, declaration); err != nil {
				return fmt.Errorf("create TSD declaration: %w", err)
			}
			if err := txRepo.CreateTSDRows(ctx, schemaName, rows); err != nil {
				return fmt.Errorf("create TSD rows: %w", err)
			}
			return nil
		})
		if err != nil {
			for _, record := range group.records {
				appendTSDHistoryRowError(result, record, err.Error())
			}
			continue
		}

		result.DeclarationsCreated++
		result.RowsImported += len(rows)
	}

	if len(result.Errors) == 0 {
		result.Errors = nil
	}
	return result, nil
}

func parseTSDHistoryImportRows(content string) ([]tsdHistoryImportRow, error) {
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
		canonicalHeaders[i] = canonicalTSDHistoryImportHeader(header)
		switch canonicalHeaders[i] {
		case "period_year":
			hasYear = true
		case "period_month":
			hasMonth = true
		case "gross_payment":
			hasGross = true
		}
	}
	if !hasYear || !hasMonth || !hasGross {
		return nil, fmt.Errorf("missing required period_year, period_month, or gross_payment column")
	}

	rows := make([]tsdHistoryImportRow, 0)
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
		rows = append(rows, tsdHistoryImportRow{rowNumber: rowNumber, values: values})
	}
	return rows, nil
}

func buildTSDHistoryImportRecord(row tsdHistoryImportRow, indexes *payrollHistoryEmployeeIndexes) (*tsdHistoryImportRecord, error) {
	periodYear, err := parsePayrollHistoryImportYear(row.values["period_year"])
	if err != nil {
		return nil, err
	}
	periodMonth, err := parsePayrollHistoryImportMonth(row.values["period_month"])
	if err != nil {
		return nil, err
	}
	status, err := parseTSDHistoryImportStatus(row.values["status"])
	if err != nil {
		return nil, err
	}

	var submittedAt *time.Time
	if value := strings.TrimSpace(row.values["submitted_at"]); value != "" {
		parsed, err := parseEmployeeImportDate(value, "submitted_at")
		if err != nil {
			return nil, err
		}
		submittedAt = &parsed
	}
	if submittedAt == nil && (status == TSDSubmitted || status == TSDAccepted) {
		now := normalizeEmployeeImportDate(time.Now())
		submittedAt = &now
	}

	employee, employeeName, err := findPayrollHistoryEmployee(row.values, indexes)
	if err != nil {
		return nil, err
	}
	employeeNumber := strings.TrimSpace(row.values["employee_number"])
	if employeeNumber == "" {
		employeeNumber = employee.EmployeeNumber
	}

	grossPayment, err := parseRequiredPayrollHistoryDecimal(row.values["gross_payment"], "gross_payment")
	if err != nil {
		return nil, err
	}
	if !grossPayment.GreaterThan(decimal.Zero) {
		return nil, fmt.Errorf("gross_payment must be greater than zero")
	}
	basicExemption, err := parseOptionalPayrollHistoryDecimal(row.values["basic_exemption"], "basic_exemption")
	if err != nil {
		return nil, err
	}
	taxableAmount, err := parseOptionalPayrollHistoryDecimal(row.values["taxable_amount"], "taxable_amount")
	if err != nil {
		return nil, err
	}
	if taxableAmount.IsZero() && strings.TrimSpace(row.values["taxable_amount"]) == "" {
		taxableAmount = grossPayment.Sub(basicExemption)
		if taxableAmount.IsNegative() {
			taxableAmount = decimal.Zero
		}
	}
	incomeTax, err := parseOptionalPayrollHistoryDecimal(row.values["income_tax"], "income_tax")
	if err != nil {
		return nil, err
	}
	socialTax, err := parseOptionalPayrollHistoryDecimal(row.values["social_tax"], "social_tax")
	if err != nil {
		return nil, err
	}
	unemploymentER, err := parseOptionalPayrollHistoryDecimal(row.values["unemployment_insurance_employer"], "unemployment_insurance_employer")
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

	paymentType := strings.TrimSpace(row.values["payment_type"])
	if paymentType == "" {
		paymentType = "10"
	}

	return &tsdHistoryImportRecord{
		rowNumber:      row.rowNumber,
		periodYear:     periodYear,
		periodMonth:    periodMonth,
		status:         status,
		submittedAt:    submittedAt,
		emtaReference:  strings.TrimSpace(row.values["emta_reference"]),
		employeeName:   employeeName,
		employeeNumber: employeeNumber,
		row: TSDRow{
			EmployeeID:     employee.ID,
			PersonalCode:   employee.PersonalCode,
			FirstName:      employee.FirstName,
			LastName:       employee.LastName,
			PaymentType:    paymentType,
			GrossPayment:   grossPayment,
			BasicExemption: basicExemption,
			TaxableAmount:  taxableAmount,
			IncomeTax:      incomeTax,
			SocialTax:      socialTax,
			UnemploymentER: unemploymentER,
			UnemploymentEE: unemploymentEE,
			FundedPension:  fundedPension,
		},
	}, nil
}

func validateTSDHistoryGroupConsistency(group *tsdHistoryImportGroup, record *tsdHistoryImportRecord) string {
	if group.status != record.status {
		return "status must be consistent for each TSD period"
	}
	if !payrollHistoryDatesEqual(group.submittedAt, record.submittedAt) {
		return "submitted_at must be consistent for each TSD period"
	}
	if group.emtaReference != record.emtaReference {
		return "emta_reference must be consistent for each TSD period"
	}
	return ""
}

func buildTSDHistoryDeclaration(tenantID string, group *tsdHistoryImportGroup) *TSDDeclaration {
	declaration := &TSDDeclaration{
		TenantID:      tenantID,
		PeriodYear:    group.periodYear,
		PeriodMonth:   group.periodMonth,
		Status:        group.status,
		SubmittedAt:   group.submittedAt,
		EMTAReference: group.emtaReference,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	for _, record := range group.records {
		declaration.TotalPayments = declaration.TotalPayments.Add(record.row.GrossPayment)
		declaration.TotalIncomeTax = declaration.TotalIncomeTax.Add(record.row.IncomeTax)
		declaration.TotalSocialTax = declaration.TotalSocialTax.Add(record.row.SocialTax)
		declaration.TotalUnemploymentER = declaration.TotalUnemploymentER.Add(record.row.UnemploymentER)
		declaration.TotalUnemploymentEE = declaration.TotalUnemploymentEE.Add(record.row.UnemploymentEE)
		declaration.TotalFundedPension = declaration.TotalFundedPension.Add(record.row.FundedPension)
	}
	return declaration
}

func parseTSDHistoryImportStatus(value string) (TSDStatus, error) {
	status, ok := tsdHistoryImportStatusAliases[normalizeEmployeeImportValue(value)]
	if !ok {
		return "", fmt.Errorf("status must be DRAFT, SUBMITTED, ACCEPTED, or REJECTED")
	}
	return status, nil
}

func canonicalTSDHistoryImportHeader(header string) string {
	normalized := strings.ToLower(strings.TrimSpace(header))
	if canonical, ok := tsdHistoryImportHeaderAliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func appendTSDHistoryRowError(result *ImportTSDHistoryResult, record *tsdHistoryImportRecord, message string) {
	result.RowsSkipped++
	result.Errors = append(result.Errors, ImportTSDHistoryRowError{
		Row:            record.rowNumber,
		PeriodYear:     record.periodYear,
		PeriodMonth:    record.periodMonth,
		EmployeeName:   record.employeeName,
		EmployeeNumber: record.employeeNumber,
		Message:        message,
	})
}
