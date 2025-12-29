package payroll

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides payroll operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new payroll service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// =============================================================================
// EMPLOYEE OPERATIONS
// =============================================================================

// CreateEmployee creates a new employee
func (s *Service) CreateEmployee(ctx context.Context, schemaName, tenantID string, req *CreateEmployeeRequest) (*Employee, error) {
	if req.FirstName == "" || req.LastName == "" {
		return nil, fmt.Errorf("first name and last name are required")
	}
	if req.StartDate.IsZero() {
		return nil, fmt.Errorf("start date is required")
	}

	// Set defaults
	if req.EmploymentType == "" {
		req.EmploymentType = EmploymentFullTime
	}
	if req.BasicExemptionAmount.IsZero() && req.ApplyBasicExemption {
		req.BasicExemptionAmount = DefaultBasicExemption
	}

	emp := &Employee{
		ID:                   uuid.New().String(),
		TenantID:             tenantID,
		EmployeeNumber:       req.EmployeeNumber,
		FirstName:            req.FirstName,
		LastName:             req.LastName,
		PersonalCode:         req.PersonalCode,
		Email:                req.Email,
		Phone:                req.Phone,
		Address:              req.Address,
		BankAccount:          req.BankAccount,
		StartDate:            req.StartDate,
		Position:             req.Position,
		Department:           req.Department,
		EmploymentType:       req.EmploymentType,
		TaxResidency:         "EE",
		ApplyBasicExemption:  req.ApplyBasicExemption,
		BasicExemptionAmount: req.BasicExemptionAmount,
		FundedPensionRate:    req.FundedPensionRate,
		IsActive:             true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.employees (
			id, tenant_id, employee_number, first_name, last_name, personal_code,
			email, phone, address, bank_account, start_date, position, department,
			employment_type, tax_residency, apply_basic_exemption, basic_exemption_amount,
			funded_pension_rate, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
	`, schemaName)

	_, err := s.db.Exec(ctx, query,
		emp.ID, emp.TenantID, emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.PersonalCode,
		emp.Email, emp.Phone, emp.Address, emp.BankAccount, emp.StartDate, emp.Position, emp.Department,
		emp.EmploymentType, emp.TaxResidency, emp.ApplyBasicExemption, emp.BasicExemptionAmount,
		emp.FundedPensionRate, emp.IsActive, emp.CreatedAt, emp.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}

	return emp, nil
}

// GetEmployee retrieves an employee by ID
func (s *Service) GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, employee_number, first_name, last_name, personal_code,
			email, phone, address, bank_account, start_date, end_date, position, department,
			employment_type, tax_residency, apply_basic_exemption, basic_exemption_amount,
			funded_pension_rate, is_active, created_at, updated_at
		FROM %s.employees
		WHERE tenant_id = $1 AND id = $2
	`, schemaName)

	var emp Employee
	err := s.db.QueryRow(ctx, query, tenantID, employeeID).Scan(
		&emp.ID, &emp.TenantID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.PersonalCode,
		&emp.Email, &emp.Phone, &emp.Address, &emp.BankAccount, &emp.StartDate, &emp.EndDate,
		&emp.Position, &emp.Department, &emp.EmploymentType, &emp.TaxResidency,
		&emp.ApplyBasicExemption, &emp.BasicExemptionAmount, &emp.FundedPensionRate,
		&emp.IsActive, &emp.CreatedAt, &emp.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("employee not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}

	return &emp, nil
}

// ListEmployees returns all active employees for a tenant
func (s *Service) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, employee_number, first_name, last_name, personal_code,
			email, phone, address, bank_account, start_date, end_date, position, department,
			employment_type, tax_residency, apply_basic_exemption, basic_exemption_amount,
			funded_pension_rate, is_active, created_at, updated_at
		FROM %s.employees
		WHERE tenant_id = $1
	`, schemaName)

	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY last_name, first_name"

	rows, err := s.db.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}
	defer rows.Close()

	var employees []Employee
	for rows.Next() {
		var emp Employee
		if err := rows.Scan(
			&emp.ID, &emp.TenantID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.PersonalCode,
			&emp.Email, &emp.Phone, &emp.Address, &emp.BankAccount, &emp.StartDate, &emp.EndDate,
			&emp.Position, &emp.Department, &emp.EmploymentType, &emp.TaxResidency,
			&emp.ApplyBasicExemption, &emp.BasicExemptionAmount, &emp.FundedPensionRate,
			&emp.IsActive, &emp.CreatedAt, &emp.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		employees = append(employees, emp)
	}

	return employees, nil
}

// UpdateEmployee updates an existing employee
func (s *Service) UpdateEmployee(ctx context.Context, schemaName, tenantID, employeeID string, req *UpdateEmployeeRequest) (*Employee, error) {
	// Get the existing employee first
	emp, err := s.GetEmployee(ctx, schemaName, tenantID, employeeID)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.EmployeeNumber != "" {
		emp.EmployeeNumber = req.EmployeeNumber
	}
	if req.FirstName != "" {
		emp.FirstName = req.FirstName
	}
	if req.LastName != "" {
		emp.LastName = req.LastName
	}
	if req.PersonalCode != "" {
		emp.PersonalCode = req.PersonalCode
	}
	if req.Email != "" {
		emp.Email = req.Email
	}
	if req.Phone != "" {
		emp.Phone = req.Phone
	}
	if req.Address != "" {
		emp.Address = req.Address
	}
	if req.BankAccount != "" {
		emp.BankAccount = req.BankAccount
	}
	if req.EndDate != nil {
		emp.EndDate = req.EndDate
	}
	if req.Position != "" {
		emp.Position = req.Position
	}
	if req.Department != "" {
		emp.Department = req.Department
	}
	if req.EmploymentType != "" {
		emp.EmploymentType = req.EmploymentType
	}
	if req.ApplyBasicExemption != nil {
		emp.ApplyBasicExemption = *req.ApplyBasicExemption
	}
	if req.BasicExemptionAmount != nil {
		emp.BasicExemptionAmount = *req.BasicExemptionAmount
	}
	if req.FundedPensionRate != nil {
		emp.FundedPensionRate = *req.FundedPensionRate
	}
	if req.IsActive != nil {
		emp.IsActive = *req.IsActive
	}

	emp.UpdatedAt = time.Now()

	query := fmt.Sprintf(`
		UPDATE %s.employees SET
			employee_number = $1, first_name = $2, last_name = $3, personal_code = $4,
			email = $5, phone = $6, address = $7, bank_account = $8, end_date = $9,
			position = $10, department = $11, employment_type = $12,
			apply_basic_exemption = $13, basic_exemption_amount = $14, funded_pension_rate = $15,
			is_active = $16, updated_at = $17
		WHERE tenant_id = $18 AND id = $19
	`, schemaName)

	_, err = s.db.Exec(ctx, query,
		emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.PersonalCode,
		emp.Email, emp.Phone, emp.Address, emp.BankAccount, emp.EndDate,
		emp.Position, emp.Department, emp.EmploymentType,
		emp.ApplyBasicExemption, emp.BasicExemptionAmount, emp.FundedPensionRate,
		emp.IsActive, emp.UpdatedAt,
		tenantID, employeeID,
	)
	if err != nil {
		return nil, fmt.Errorf("update employee: %w", err)
	}

	return emp, nil
}

// =============================================================================
// SALARY COMPONENT OPERATIONS
// =============================================================================

// SetBaseSalary sets or updates an employee's base salary
func (s *Service) SetBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, amount decimal.Decimal, effectiveFrom time.Time) error {
	// End any existing base salary
	endQuery := fmt.Sprintf(`
		UPDATE %s.salary_components
		SET effective_to = $1
		WHERE tenant_id = $2 AND employee_id = $3 AND component_type = 'BASE_SALARY' AND effective_to IS NULL
	`, schemaName)
	_, _ = s.db.Exec(ctx, endQuery, effectiveFrom.AddDate(0, 0, -1), tenantID, employeeID)

	// Create new base salary
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s.salary_components (id, tenant_id, employee_id, component_type, name, amount, is_taxable, is_recurring, effective_from, created_at)
		VALUES ($1, $2, $3, 'BASE_SALARY', 'Base Salary', $4, true, true, $5, NOW())
	`, schemaName)

	_, err := s.db.Exec(ctx, insertQuery, uuid.New().String(), tenantID, employeeID, amount, effectiveFrom)
	if err != nil {
		return fmt.Errorf("set base salary: %w", err)
	}

	return nil
}

// GetCurrentSalary returns the current salary for an employee
func (s *Service) GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(amount), 0)
		FROM %s.salary_components
		WHERE tenant_id = $1 AND employee_id = $2 AND is_recurring = true
			AND effective_from <= CURRENT_DATE
			AND (effective_to IS NULL OR effective_to >= CURRENT_DATE)
	`, schemaName)

	var salary decimal.Decimal
	err := s.db.QueryRow(ctx, query, tenantID, employeeID).Scan(&salary)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get current salary: %w", err)
	}

	return salary, nil
}

// =============================================================================
// PAYROLL RUN OPERATIONS
// =============================================================================

// CreatePayrollRun creates a new payroll run for a period
func (s *Service) CreatePayrollRun(ctx context.Context, schemaName, tenantID, userID string, req *CreatePayrollRunRequest) (*PayrollRun, error) {
	if req.PeriodYear < 2020 || req.PeriodYear > 2100 {
		return nil, fmt.Errorf("invalid period year")
	}
	if req.PeriodMonth < 1 || req.PeriodMonth > 12 {
		return nil, fmt.Errorf("invalid period month")
	}

	run := &PayrollRun{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		PeriodYear:  req.PeriodYear,
		PeriodMonth: req.PeriodMonth,
		Status:      PayrollDraft,
		PaymentDate: req.PaymentDate,
		Notes:       req.Notes,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	query := fmt.Sprintf(`
		INSERT INTO %s.payroll_runs (id, tenant_id, period_year, period_month, status, payment_date, notes, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, schemaName)

	_, err := s.db.Exec(ctx, query,
		run.ID, run.TenantID, run.PeriodYear, run.PeriodMonth, run.Status,
		run.PaymentDate, run.Notes, run.CreatedBy, run.CreatedAt, run.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create payroll run: %w", err)
	}

	return run, nil
}

// CalculatePayroll calculates payroll for all active employees in a run
func (s *Service) CalculatePayroll(ctx context.Context, schemaName, tenantID, payrollRunID string) (*PayrollRun, error) {
	// Get the payroll run
	run, err := s.GetPayrollRun(ctx, schemaName, tenantID, payrollRunID)
	if err != nil {
		return nil, err
	}

	if run.Status != PayrollDraft {
		return nil, fmt.Errorf("payroll run must be in DRAFT status to calculate")
	}

	// Get all active employees
	employees, err := s.ListEmployees(ctx, schemaName, tenantID, true)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Delete any existing payslips for this run
	deleteQuery := fmt.Sprintf(`DELETE FROM %s.payslips WHERE payroll_run_id = $1`, schemaName)
	_, _ = tx.Exec(ctx, deleteQuery, payrollRunID)

	var totalGross, totalNet, totalEmployerCost decimal.Decimal
	var payslips []Payslip

	for _, emp := range employees {
		// Get current salary
		salary, err := s.GetCurrentSalary(ctx, schemaName, tenantID, emp.ID)
		if err != nil || salary.IsZero() {
			continue // Skip employees without salary
		}

		// Calculate taxes
		basicExemption := decimal.Zero
		if emp.ApplyBasicExemption {
			basicExemption = emp.BasicExemptionAmount
		}
		calc := CalculateEstonianTaxes(salary, basicExemption, emp.FundedPensionRate)

		// Create payslip
		payslip := Payslip{
			ID:                      uuid.New().String(),
			TenantID:                tenantID,
			PayrollRunID:            payrollRunID,
			EmployeeID:              emp.ID,
			GrossSalary:             calc.GrossSalary,
			TaxableIncome:           calc.TaxableIncome,
			IncomeTax:               calc.IncomeTax,
			UnemploymentInsuranceEE: calc.UnemploymentEE,
			FundedPension:           calc.FundedPension,
			NetSalary:               calc.NetSalary,
			SocialTax:               calc.SocialTax,
			UnemploymentInsuranceER: calc.UnemploymentER,
			TotalEmployerCost:       calc.TotalEmployerCost,
			BasicExemptionApplied:   basicExemption,
			PaymentStatus:           "PENDING",
			CreatedAt:               time.Now(),
		}

		insertQuery := fmt.Sprintf(`
			INSERT INTO %s.payslips (
				id, tenant_id, payroll_run_id, employee_id, gross_salary, taxable_income,
				income_tax, unemployment_insurance_employee, funded_pension, net_salary,
				social_tax, unemployment_insurance_employer, total_employer_cost,
				basic_exemption_applied, payment_status, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		`, schemaName)

		_, err = tx.Exec(ctx, insertQuery,
			payslip.ID, payslip.TenantID, payslip.PayrollRunID, payslip.EmployeeID,
			payslip.GrossSalary, payslip.TaxableIncome, payslip.IncomeTax,
			payslip.UnemploymentInsuranceEE, payslip.FundedPension, payslip.NetSalary,
			payslip.SocialTax, payslip.UnemploymentInsuranceER, payslip.TotalEmployerCost,
			payslip.BasicExemptionApplied, payslip.PaymentStatus, payslip.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert payslip: %w", err)
		}

		totalGross = totalGross.Add(calc.GrossSalary)
		totalNet = totalNet.Add(calc.NetSalary)
		totalEmployerCost = totalEmployerCost.Add(calc.TotalEmployerCost)
		payslips = append(payslips, payslip)
	}

	// Update payroll run totals and status
	updateQuery := fmt.Sprintf(`
		UPDATE %s.payroll_runs
		SET status = $1, total_gross = $2, total_net = $3, total_employer_cost = $4, updated_at = NOW()
		WHERE id = $5
	`, schemaName)

	_, err = tx.Exec(ctx, updateQuery, PayrollCalculated, totalGross, totalNet, totalEmployerCost, payrollRunID)
	if err != nil {
		return nil, fmt.Errorf("update payroll run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	run.Status = PayrollCalculated
	run.TotalGross = totalGross
	run.TotalNet = totalNet
	run.TotalEmployerCost = totalEmployerCost
	run.Payslips = payslips

	return run, nil
}

// GetPayrollRun retrieves a payroll run by ID
func (s *Service) GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, status, payment_date,
			total_gross, total_net, total_employer_cost, notes,
			created_by, approved_by, approved_at, created_at, updated_at
		FROM %s.payroll_runs
		WHERE tenant_id = $1 AND id = $2
	`, schemaName)

	var run PayrollRun
	err := s.db.QueryRow(ctx, query, tenantID, runID).Scan(
		&run.ID, &run.TenantID, &run.PeriodYear, &run.PeriodMonth, &run.Status, &run.PaymentDate,
		&run.TotalGross, &run.TotalNet, &run.TotalEmployerCost, &run.Notes,
		&run.CreatedBy, &run.ApprovedBy, &run.ApprovedAt, &run.CreatedAt, &run.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("payroll run not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get payroll run: %w", err)
	}

	return &run, nil
}

// ListPayrollRuns lists payroll runs for a tenant
func (s *Service) ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]PayrollRun, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, status, payment_date,
			total_gross, total_net, total_employer_cost, notes,
			created_by, approved_by, approved_at, created_at, updated_at
		FROM %s.payroll_runs
		WHERE tenant_id = $1
	`, schemaName)

	args := []interface{}{tenantID}
	if year > 0 {
		query += " AND period_year = $2"
		args = append(args, year)
	}
	query += " ORDER BY period_year DESC, period_month DESC"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list payroll runs: %w", err)
	}
	defer rows.Close()

	var runs []PayrollRun
	for rows.Next() {
		var run PayrollRun
		if err := rows.Scan(
			&run.ID, &run.TenantID, &run.PeriodYear, &run.PeriodMonth, &run.Status, &run.PaymentDate,
			&run.TotalGross, &run.TotalNet, &run.TotalEmployerCost, &run.Notes,
			&run.CreatedBy, &run.ApprovedBy, &run.ApprovedAt, &run.CreatedAt, &run.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan payroll run: %w", err)
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// ApprovePayrollRun approves a calculated payroll run
func (s *Service) ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.payroll_runs
		SET status = $1, approved_by = $2, approved_at = NOW(), updated_at = NOW()
		WHERE tenant_id = $3 AND id = $4 AND status = $5
	`, schemaName)

	result, err := s.db.Exec(ctx, query, PayrollApproved, approverID, tenantID, runID, PayrollCalculated)
	if err != nil {
		return fmt.Errorf("approve payroll run: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("payroll run not found or not in CALCULATED status")
	}

	return nil
}
