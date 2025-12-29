package payroll

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// GenerateTSD generates a TSD declaration from a payroll run
func (s *Service) GenerateTSD(ctx context.Context, schemaName, tenantID, payrollRunID string) (*TSDDeclaration, error) {
	// Get the payroll run
	run, err := s.GetPayrollRun(ctx, schemaName, tenantID, payrollRunID)
	if err != nil {
		return nil, err
	}

	if run.Status != PayrollApproved && run.Status != PayrollPaid {
		return nil, fmt.Errorf("payroll run must be APPROVED or PAID to generate TSD")
	}

	// Get payslips with employee data
	payslips, err := s.GetPayslipsWithEmployees(ctx, schemaName, tenantID, payrollRunID)
	if err != nil {
		return nil, err
	}

	if len(payslips) == 0 {
		return nil, fmt.Errorf("no payslips found for this payroll run")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Check for existing TSD for this period
	var existingID string
	checkQuery := fmt.Sprintf(`
		SELECT id FROM %s.tsd_declarations
		WHERE tenant_id = $1 AND period_year = $2 AND period_month = $3
	`, schemaName)
	err = tx.QueryRow(ctx, checkQuery, tenantID, run.PeriodYear, run.PeriodMonth).Scan(&existingID)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("check existing TSD: %w", err)
	}

	// Delete existing TSD if found
	if existingID != "" {
		deleteRowsQuery := fmt.Sprintf(`DELETE FROM %s.tsd_rows WHERE declaration_id = $1`, schemaName)
		_, _ = tx.Exec(ctx, deleteRowsQuery, existingID)

		deleteQuery := fmt.Sprintf(`DELETE FROM %s.tsd_declarations WHERE id = $1`, schemaName)
		_, _ = tx.Exec(ctx, deleteQuery, existingID)
	}

	// Create TSD declaration
	tsd := &TSDDeclaration{
		ID:           uuid.New().String(),
		TenantID:     tenantID,
		PeriodYear:   run.PeriodYear,
		PeriodMonth:  run.PeriodMonth,
		PayrollRunID: payrollRunID,
		Status:       TSDDraft,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Calculate totals and create rows
	var rows []TSDRow
	for _, ps := range payslips {
		if ps.Employee == nil {
			continue
		}

		row := TSDRow{
			ID:             uuid.New().String(),
			TenantID:       tenantID,
			DeclarationID:  tsd.ID,
			EmployeeID:     ps.EmployeeID,
			PersonalCode:   ps.Employee.PersonalCode,
			FirstName:      ps.Employee.FirstName,
			LastName:       ps.Employee.LastName,
			PaymentType:    "10", // Regular salary
			GrossPayment:   ps.GrossSalary,
			BasicExemption: ps.BasicExemptionApplied,
			TaxableAmount:  ps.TaxableIncome,
			IncomeTax:      ps.IncomeTax,
			SocialTax:      ps.SocialTax,
			UnemploymentER: ps.UnemploymentInsuranceER,
			UnemploymentEE: ps.UnemploymentInsuranceEE,
			FundedPension:  ps.FundedPension,
			CreatedAt:      time.Now(),
		}

		// Insert TSD row
		insertRowQuery := fmt.Sprintf(`
			INSERT INTO %s.tsd_rows (
				id, tenant_id, declaration_id, employee_id, personal_code, first_name, last_name,
				payment_type, gross_payment, basic_exemption, taxable_amount,
				income_tax, social_tax, unemployment_insurance_employer, unemployment_insurance_employee,
				funded_pension, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		`, schemaName)

		_, err = tx.Exec(ctx, insertRowQuery,
			row.ID, row.TenantID, row.DeclarationID, row.EmployeeID,
			row.PersonalCode, row.FirstName, row.LastName, row.PaymentType,
			row.GrossPayment, row.BasicExemption, row.TaxableAmount,
			row.IncomeTax, row.SocialTax, row.UnemploymentER, row.UnemploymentEE,
			row.FundedPension, row.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert TSD row: %w", err)
		}

		rows = append(rows, row)

		// Accumulate totals
		tsd.TotalPayments = tsd.TotalPayments.Add(row.GrossPayment)
		tsd.TotalIncomeTax = tsd.TotalIncomeTax.Add(row.IncomeTax)
		tsd.TotalSocialTax = tsd.TotalSocialTax.Add(row.SocialTax)
		tsd.TotalUnemploymentER = tsd.TotalUnemploymentER.Add(row.UnemploymentER)
		tsd.TotalUnemploymentEE = tsd.TotalUnemploymentEE.Add(row.UnemploymentEE)
		tsd.TotalFundedPension = tsd.TotalFundedPension.Add(row.FundedPension)
	}

	// Insert TSD declaration
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s.tsd_declarations (
			id, tenant_id, period_year, period_month, payroll_run_id,
			total_payments, total_income_tax, total_social_tax,
			total_unemployment_employer, total_unemployment_employee, total_funded_pension,
			status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, schemaName)

	_, err = tx.Exec(ctx, insertQuery,
		tsd.ID, tsd.TenantID, tsd.PeriodYear, tsd.PeriodMonth, tsd.PayrollRunID,
		tsd.TotalPayments, tsd.TotalIncomeTax, tsd.TotalSocialTax,
		tsd.TotalUnemploymentER, tsd.TotalUnemploymentEE, tsd.TotalFundedPension,
		tsd.Status, tsd.CreatedAt, tsd.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert TSD declaration: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	tsd.Rows = rows
	return tsd, nil
}

// GetTSD retrieves a TSD declaration by period
func (s *Service) GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDDeclaration, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, payroll_run_id,
			total_payments, total_income_tax, total_social_tax,
			total_unemployment_employer, total_unemployment_employee, total_funded_pension,
			status, submitted_at, emta_reference, created_at, updated_at
		FROM %s.tsd_declarations
		WHERE tenant_id = $1 AND period_year = $2 AND period_month = $3
	`, schemaName)

	var tsd TSDDeclaration
	err := s.db.QueryRow(ctx, query, tenantID, year, month).Scan(
		&tsd.ID, &tsd.TenantID, &tsd.PeriodYear, &tsd.PeriodMonth, &tsd.PayrollRunID,
		&tsd.TotalPayments, &tsd.TotalIncomeTax, &tsd.TotalSocialTax,
		&tsd.TotalUnemploymentER, &tsd.TotalUnemploymentEE, &tsd.TotalFundedPension,
		&tsd.Status, &tsd.SubmittedAt, &tsd.EMTAReference, &tsd.CreatedAt, &tsd.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("TSD declaration not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get TSD: %w", err)
	}

	// Load rows
	rows, err := s.GetTSDRows(ctx, schemaName, tenantID, tsd.ID)
	if err != nil {
		return nil, err
	}
	tsd.Rows = rows

	return &tsd, nil
}

// GetTSDRows retrieves all rows for a TSD declaration
func (s *Service) GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]TSDRow, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, declaration_id, employee_id, personal_code, first_name, last_name,
			payment_type, gross_payment, basic_exemption, taxable_amount,
			income_tax, social_tax, unemployment_insurance_employer, unemployment_insurance_employee,
			funded_pension, created_at
		FROM %s.tsd_rows
		WHERE tenant_id = $1 AND declaration_id = $2
		ORDER BY last_name, first_name
	`, schemaName)

	rows, err := s.db.Query(ctx, query, tenantID, declarationID)
	if err != nil {
		return nil, fmt.Errorf("get TSD rows: %w", err)
	}
	defer rows.Close()

	var tsdRows []TSDRow
	for rows.Next() {
		var row TSDRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.DeclarationID, &row.EmployeeID,
			&row.PersonalCode, &row.FirstName, &row.LastName, &row.PaymentType,
			&row.GrossPayment, &row.BasicExemption, &row.TaxableAmount,
			&row.IncomeTax, &row.SocialTax, &row.UnemploymentER, &row.UnemploymentEE,
			&row.FundedPension, &row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan TSD row: %w", err)
		}
		tsdRows = append(tsdRows, row)
	}

	return tsdRows, nil
}

// ListTSD lists all TSD declarations for a tenant
func (s *Service) ListTSD(ctx context.Context, schemaName, tenantID string) ([]TSDDeclaration, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, payroll_run_id,
			total_payments, total_income_tax, total_social_tax,
			total_unemployment_employer, total_unemployment_employee, total_funded_pension,
			status, submitted_at, emta_reference, created_at, updated_at
		FROM %s.tsd_declarations
		WHERE tenant_id = $1
		ORDER BY period_year DESC, period_month DESC
	`, schemaName)

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list TSD: %w", err)
	}
	defer rows.Close()

	var declarations []TSDDeclaration
	for rows.Next() {
		var tsd TSDDeclaration
		if err := rows.Scan(
			&tsd.ID, &tsd.TenantID, &tsd.PeriodYear, &tsd.PeriodMonth, &tsd.PayrollRunID,
			&tsd.TotalPayments, &tsd.TotalIncomeTax, &tsd.TotalSocialTax,
			&tsd.TotalUnemploymentER, &tsd.TotalUnemploymentEE, &tsd.TotalFundedPension,
			&tsd.Status, &tsd.SubmittedAt, &tsd.EMTAReference, &tsd.CreatedAt, &tsd.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan TSD: %w", err)
		}
		declarations = append(declarations, tsd)
	}

	return declarations, nil
}

// GetPayslipsWithEmployees retrieves payslips with employee data
func (s *Service) GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]Payslip, error) {
	query := fmt.Sprintf(`
		SELECT p.id, p.tenant_id, p.payroll_run_id, p.employee_id,
			p.gross_salary, p.taxable_income, p.income_tax, p.unemployment_insurance_employee,
			p.funded_pension, p.other_deductions, p.net_salary, p.social_tax,
			p.unemployment_insurance_employer, p.total_employer_cost, p.basic_exemption_applied,
			p.payment_status, p.paid_at, p.created_at,
			e.id, e.first_name, e.last_name, e.personal_code, e.email
		FROM %s.payslips p
		JOIN %s.employees e ON e.id = p.employee_id
		WHERE p.tenant_id = $1 AND p.payroll_run_id = $2
		ORDER BY e.last_name, e.first_name
	`, schemaName, schemaName)

	rows, err := s.db.Query(ctx, query, tenantID, payrollRunID)
	if err != nil {
		return nil, fmt.Errorf("get payslips: %w", err)
	}
	defer rows.Close()

	var payslips []Payslip
	for rows.Next() {
		var ps Payslip
		var emp Employee
		if err := rows.Scan(
			&ps.ID, &ps.TenantID, &ps.PayrollRunID, &ps.EmployeeID,
			&ps.GrossSalary, &ps.TaxableIncome, &ps.IncomeTax, &ps.UnemploymentInsuranceEE,
			&ps.FundedPension, &ps.OtherDeductions, &ps.NetSalary, &ps.SocialTax,
			&ps.UnemploymentInsuranceER, &ps.TotalEmployerCost, &ps.BasicExemptionApplied,
			&ps.PaymentStatus, &ps.PaidAt, &ps.CreatedAt,
			&emp.ID, &emp.FirstName, &emp.LastName, &emp.PersonalCode, &emp.Email,
		); err != nil {
			return nil, fmt.Errorf("scan payslip: %w", err)
		}
		ps.Employee = &emp
		payslips = append(payslips, ps)
	}

	return payslips, nil
}

// CalculateTaxPreview calculates tax preview for a given gross salary
func CalculateTaxPreview(grossSalary decimal.Decimal, applyBasicExemption bool, fundedPensionRate decimal.Decimal) TaxCalculation {
	basicExemption := decimal.Zero
	if applyBasicExemption {
		basicExemption = DefaultBasicExemption
	}
	return CalculateEstonianTaxes(grossSalary, basicExemption, fundedPensionRate)
}
