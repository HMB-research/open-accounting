package payroll

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides payroll operations
type Service struct {
	db   *pgxpool.Pool
	repo Repository
	uuid UUIDGenerator
}

// NewService creates a new payroll service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:   db,
		repo: NewPostgresRepository(db),
		uuid: &DefaultUUIDGenerator{},
	}
}

// NewServiceWithRepository creates a new payroll service with a custom repository (for testing)
func NewServiceWithRepository(repo Repository, uuidGen UUIDGenerator) *Service {
	return &Service{
		repo: repo,
		uuid: uuidGen,
	}
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
		ID:                   s.uuid.New(),
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

	if err := s.repo.CreateEmployee(ctx, schemaName, emp); err != nil {
		return nil, fmt.Errorf("create employee: %w", err)
	}

	return emp, nil
}

// GetEmployee retrieves an employee by ID
func (s *Service) GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error) {
	emp, err := s.repo.GetEmployee(ctx, schemaName, tenantID, employeeID)
	if err == ErrEmployeeNotFound {
		return nil, fmt.Errorf("employee not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}
	return emp, nil
}

// ListEmployees returns all active employees for a tenant
func (s *Service) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error) {
	employees, err := s.repo.ListEmployees(ctx, schemaName, tenantID, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
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

	if err := s.repo.UpdateEmployee(ctx, schemaName, emp); err != nil {
		return nil, fmt.Errorf("update employee: %w", err)
	}

	return emp, nil
}

// =============================================================================
// SALARY COMPONENT OPERATIONS
// =============================================================================

// SetBaseSalary sets or updates an employee's base salary
func (s *Service) SetBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, amount decimal.Decimal, effectiveFrom time.Time) error {
	// End any existing base salary (ignore errors - may not exist)
	_ = s.repo.EndCurrentBaseSalary(ctx, schemaName, tenantID, employeeID, effectiveFrom.AddDate(0, 0, -1))

	// Create new base salary
	comp := &SalaryComponent{
		ID:            s.uuid.New(),
		TenantID:      tenantID,
		EmployeeID:    employeeID,
		ComponentType: "BASE_SALARY",
		Name:          "Base Salary",
		Amount:        amount,
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: effectiveFrom,
		CreatedAt:     time.Now(),
	}

	if err := s.repo.CreateSalaryComponent(ctx, schemaName, comp); err != nil {
		return fmt.Errorf("set base salary: %w", err)
	}

	return nil
}

// GetCurrentSalary returns the current salary for an employee
func (s *Service) GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error) {
	salary, err := s.repo.GetCurrentSalary(ctx, schemaName, tenantID, employeeID)
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
		ID:          s.uuid.New(),
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

	if err := s.repo.CreatePayrollRun(ctx, schemaName, run); err != nil {
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

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txRepo := s.repo.WithTx(tx)

	// Delete any existing payslips for this run
	_ = txRepo.DeletePayslipsByRunID(ctx, schemaName, payrollRunID)

	var totalGross, totalNet, totalEmployerCost decimal.Decimal
	payslips := make([]Payslip, 0, len(employees))

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
			ID:                      s.uuid.New(),
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

		if err := txRepo.CreatePayslip(ctx, schemaName, &payslip); err != nil {
			return nil, fmt.Errorf("insert payslip: %w", err)
		}

		totalGross = totalGross.Add(calc.GrossSalary)
		totalNet = totalNet.Add(calc.NetSalary)
		totalEmployerCost = totalEmployerCost.Add(calc.TotalEmployerCost)
		payslips = append(payslips, payslip)
	}

	// Update payroll run totals and status
	run.Status = PayrollCalculated
	run.TotalGross = totalGross
	run.TotalNet = totalNet
	run.TotalEmployerCost = totalEmployerCost

	if err := txRepo.UpdatePayrollRun(ctx, schemaName, run); err != nil {
		return nil, fmt.Errorf("update payroll run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	run.Payslips = payslips

	return run, nil
}

// GetPayrollRun retrieves a payroll run by ID
func (s *Service) GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error) {
	run, err := s.repo.GetPayrollRun(ctx, schemaName, tenantID, runID)
	if err == ErrPayrollRunNotFound {
		return nil, fmt.Errorf("payroll run not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get payroll run: %w", err)
	}
	return run, nil
}

// ListPayrollRuns lists payroll runs for a tenant
func (s *Service) ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]PayrollRun, error) {
	runs, err := s.repo.ListPayrollRuns(ctx, schemaName, tenantID, year)
	if err != nil {
		return nil, fmt.Errorf("list payroll runs: %w", err)
	}
	return runs, nil
}

// ApprovePayrollRun approves a calculated payroll run
func (s *Service) ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error {
	if err := s.repo.ApprovePayrollRun(ctx, schemaName, tenantID, runID, approverID); err != nil {
		if err == ErrPayrollRunNotFound {
			return fmt.Errorf("payroll run not found or not in CALCULATED status")
		}
		return fmt.Errorf("approve payroll run: %w", err)
	}
	return nil
}
