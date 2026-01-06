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

// Repository defines the contract for payroll data access
type Repository interface {
	// Employee operations
	CreateEmployee(ctx context.Context, schemaName string, emp *Employee) error
	GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error)
	ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error)
	UpdateEmployee(ctx context.Context, schemaName string, emp *Employee) error

	// Salary component operations
	EndCurrentBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, effectiveTo time.Time) error
	CreateSalaryComponent(ctx context.Context, schemaName string, comp *SalaryComponent) error
	GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error)

	// Payroll run operations
	CreatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error
	GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error)
	ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]PayrollRun, error)
	UpdatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error
	ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error

	// Payslip operations
	DeletePayslipsByRunID(ctx context.Context, schemaName, runID string) error
	CreatePayslip(ctx context.Context, schemaName string, payslip *Payslip) error

	// Transaction support
	BeginTx(ctx context.Context) (pgx.Tx, error)
	WithTx(tx pgx.Tx) Repository
}

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// BeginTx starts a new transaction
func (r *PostgresRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// WithTx returns a new repository that uses the given transaction
func (r *PostgresRepository) WithTx(tx pgx.Tx) Repository {
	return &PostgresRepository{pool: r.pool, tx: tx}
}

func (r *PostgresRepository) exec(ctx context.Context, query string, args ...interface{}) error {
	if r.tx != nil {
		_, err := r.tx.Exec(ctx, query, args...)
		return err
	}
	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *PostgresRepository) queryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	if r.tx != nil {
		return r.tx.QueryRow(ctx, query, args...)
	}
	return r.pool.QueryRow(ctx, query, args...)
}

func (r *PostgresRepository) query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if r.tx != nil {
		return r.tx.Query(ctx, query, args...)
	}
	return r.pool.Query(ctx, query, args...)
}

// CreateEmployee inserts a new employee
func (r *PostgresRepository) CreateEmployee(ctx context.Context, schemaName string, emp *Employee) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.employees (
			id, tenant_id, employee_number, first_name, last_name, personal_code,
			email, phone, address, bank_account, start_date, position, department,
			employment_type, tax_residency, apply_basic_exemption, basic_exemption_amount,
			funded_pension_rate, is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
	`, schemaName)

	return r.exec(ctx, query,
		emp.ID, emp.TenantID, emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.PersonalCode,
		emp.Email, emp.Phone, emp.Address, emp.BankAccount, emp.StartDate, emp.Position, emp.Department,
		emp.EmploymentType, emp.TaxResidency, emp.ApplyBasicExemption, emp.BasicExemptionAmount,
		emp.FundedPensionRate, emp.IsActive, emp.CreatedAt, emp.UpdatedAt,
	)
}

// GetEmployee retrieves an employee by ID
func (r *PostgresRepository) GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, COALESCE(employee_number, ''), first_name, last_name, COALESCE(personal_code, ''),
			COALESCE(email, ''), COALESCE(phone, ''), COALESCE(address, ''), COALESCE(bank_account, ''), start_date, end_date, COALESCE(position, ''), COALESCE(department, ''),
			employment_type, tax_residency, apply_basic_exemption, basic_exemption_amount,
			funded_pension_rate, is_active, created_at, updated_at
		FROM %s.employees
		WHERE tenant_id = $1 AND id = $2
	`, schemaName)

	var emp Employee
	err := r.queryRow(ctx, query, tenantID, employeeID).Scan(
		&emp.ID, &emp.TenantID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.PersonalCode,
		&emp.Email, &emp.Phone, &emp.Address, &emp.BankAccount, &emp.StartDate, &emp.EndDate,
		&emp.Position, &emp.Department, &emp.EmploymentType, &emp.TaxResidency,
		&emp.ApplyBasicExemption, &emp.BasicExemptionAmount, &emp.FundedPensionRate,
		&emp.IsActive, &emp.CreatedAt, &emp.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrEmployeeNotFound
	}
	if err != nil {
		return nil, err
	}

	return &emp, nil
}

// ListEmployees returns employees for a tenant
func (r *PostgresRepository) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, COALESCE(employee_number, ''), first_name, last_name, COALESCE(personal_code, ''),
			COALESCE(email, ''), COALESCE(phone, ''), COALESCE(address, ''), COALESCE(bank_account, ''), start_date, end_date, COALESCE(position, ''), COALESCE(department, ''),
			employment_type, tax_residency, apply_basic_exemption, basic_exemption_amount,
			funded_pension_rate, is_active, created_at, updated_at
		FROM %s.employees
		WHERE tenant_id = $1
	`, schemaName)

	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY last_name, first_name"

	rows, err := r.query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	employees := []Employee{}
	for rows.Next() {
		var emp Employee
		if err := rows.Scan(
			&emp.ID, &emp.TenantID, &emp.EmployeeNumber, &emp.FirstName, &emp.LastName, &emp.PersonalCode,
			&emp.Email, &emp.Phone, &emp.Address, &emp.BankAccount, &emp.StartDate, &emp.EndDate,
			&emp.Position, &emp.Department, &emp.EmploymentType, &emp.TaxResidency,
			&emp.ApplyBasicExemption, &emp.BasicExemptionAmount, &emp.FundedPensionRate,
			&emp.IsActive, &emp.CreatedAt, &emp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		employees = append(employees, emp)
	}

	return employees, nil
}

// UpdateEmployee updates an existing employee
func (r *PostgresRepository) UpdateEmployee(ctx context.Context, schemaName string, emp *Employee) error {
	query := fmt.Sprintf(`
		UPDATE %s.employees SET
			employee_number = $1, first_name = $2, last_name = $3, personal_code = $4,
			email = $5, phone = $6, address = $7, bank_account = $8, end_date = $9,
			position = $10, department = $11, employment_type = $12,
			apply_basic_exemption = $13, basic_exemption_amount = $14, funded_pension_rate = $15,
			is_active = $16, updated_at = $17
		WHERE tenant_id = $18 AND id = $19
	`, schemaName)

	return r.exec(ctx, query,
		emp.EmployeeNumber, emp.FirstName, emp.LastName, emp.PersonalCode,
		emp.Email, emp.Phone, emp.Address, emp.BankAccount, emp.EndDate,
		emp.Position, emp.Department, emp.EmploymentType,
		emp.ApplyBasicExemption, emp.BasicExemptionAmount, emp.FundedPensionRate,
		emp.IsActive, emp.UpdatedAt,
		emp.TenantID, emp.ID,
	)
}

// EndCurrentBaseSalary ends an existing base salary
func (r *PostgresRepository) EndCurrentBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, effectiveTo time.Time) error {
	query := fmt.Sprintf(`
		UPDATE %s.salary_components
		SET effective_to = $1
		WHERE tenant_id = $2 AND employee_id = $3 AND component_type = 'BASE_SALARY' AND effective_to IS NULL
	`, schemaName)
	return r.exec(ctx, query, effectiveTo, tenantID, employeeID)
}

// CreateSalaryComponent inserts a new salary component
func (r *PostgresRepository) CreateSalaryComponent(ctx context.Context, schemaName string, comp *SalaryComponent) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.salary_components (id, tenant_id, employee_id, component_type, name, amount, is_taxable, is_recurring, effective_from, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
	`, schemaName)

	return r.exec(ctx, query, comp.ID, comp.TenantID, comp.EmployeeID, comp.ComponentType, comp.Name, comp.Amount, comp.IsTaxable, comp.IsRecurring, comp.EffectiveFrom)
}

// GetCurrentSalary returns the current salary for an employee
func (r *PostgresRepository) GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error) {
	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(amount), 0)
		FROM %s.salary_components
		WHERE tenant_id = $1 AND employee_id = $2 AND is_recurring = true
			AND effective_from <= CURRENT_DATE
			AND (effective_to IS NULL OR effective_to >= CURRENT_DATE)
	`, schemaName)

	var salary decimal.Decimal
	err := r.queryRow(ctx, query, tenantID, employeeID).Scan(&salary)
	return salary, err
}

// CreatePayrollRun inserts a new payroll run
func (r *PostgresRepository) CreatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.payroll_runs (id, tenant_id, period_year, period_month, status, payment_date, notes, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, schemaName)

	return r.exec(ctx, query,
		run.ID, run.TenantID, run.PeriodYear, run.PeriodMonth, run.Status,
		run.PaymentDate, run.Notes, run.CreatedBy, run.CreatedAt, run.UpdatedAt,
	)
}

// GetPayrollRun retrieves a payroll run by ID
func (r *PostgresRepository) GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, status, payment_date,
			total_gross, total_net, total_employer_cost, COALESCE(notes, ''),
			created_by, approved_by, approved_at, created_at, updated_at
		FROM %s.payroll_runs
		WHERE tenant_id = $1 AND id = $2
	`, schemaName)

	var run PayrollRun
	var createdBy, approvedBy *string
	err := r.queryRow(ctx, query, tenantID, runID).Scan(
		&run.ID, &run.TenantID, &run.PeriodYear, &run.PeriodMonth, &run.Status, &run.PaymentDate,
		&run.TotalGross, &run.TotalNet, &run.TotalEmployerCost, &run.Notes,
		&createdBy, &approvedBy, &run.ApprovedAt, &run.CreatedAt, &run.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrPayrollRunNotFound
	}
	if err != nil {
		return nil, err
	}

	if createdBy != nil {
		run.CreatedBy = *createdBy
	}
	if approvedBy != nil {
		run.ApprovedBy = *approvedBy
	}

	return &run, nil
}

// ListPayrollRuns lists payroll runs for a tenant
func (r *PostgresRepository) ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]PayrollRun, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, status, payment_date,
			total_gross, total_net, total_employer_cost, COALESCE(notes, ''),
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

	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []PayrollRun{}
	for rows.Next() {
		var run PayrollRun
		var createdBy, approvedBy *string
		if err := rows.Scan(
			&run.ID, &run.TenantID, &run.PeriodYear, &run.PeriodMonth, &run.Status, &run.PaymentDate,
			&run.TotalGross, &run.TotalNet, &run.TotalEmployerCost, &run.Notes,
			&createdBy, &approvedBy, &run.ApprovedAt, &run.CreatedAt, &run.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if createdBy != nil {
			run.CreatedBy = *createdBy
		}
		if approvedBy != nil {
			run.ApprovedBy = *approvedBy
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// UpdatePayrollRun updates a payroll run
func (r *PostgresRepository) UpdatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error {
	query := fmt.Sprintf(`
		UPDATE %s.payroll_runs
		SET status = $1, total_gross = $2, total_net = $3, total_employer_cost = $4, updated_at = NOW()
		WHERE id = $5
	`, schemaName)

	return r.exec(ctx, query, run.Status, run.TotalGross, run.TotalNet, run.TotalEmployerCost, run.ID)
}

// ApprovePayrollRun approves a payroll run
func (r *PostgresRepository) ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.payroll_runs
		SET status = $1, approved_by = $2, approved_at = NOW(), updated_at = NOW()
		WHERE tenant_id = $3 AND id = $4 AND status = $5
	`, schemaName)

	if r.tx != nil {
		result, err := r.tx.Exec(ctx, query, PayrollApproved, approverID, tenantID, runID, PayrollCalculated)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrPayrollRunNotFound
		}
		return nil
	}

	result, err := r.pool.Exec(ctx, query, PayrollApproved, approverID, tenantID, runID, PayrollCalculated)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrPayrollRunNotFound
	}
	return nil
}

// DeletePayslipsByRunID deletes all payslips for a run
func (r *PostgresRepository) DeletePayslipsByRunID(ctx context.Context, schemaName, runID string) error {
	query := fmt.Sprintf(`DELETE FROM %s.payslips WHERE payroll_run_id = $1`, schemaName)
	return r.exec(ctx, query, runID)
}

// CreatePayslip inserts a new payslip
func (r *PostgresRepository) CreatePayslip(ctx context.Context, schemaName string, payslip *Payslip) error {
	query := fmt.Sprintf(`
		INSERT INTO %s.payslips (
			id, tenant_id, payroll_run_id, employee_id, gross_salary, taxable_income,
			income_tax, unemployment_insurance_employee, funded_pension, net_salary,
			social_tax, unemployment_insurance_employer, total_employer_cost,
			basic_exemption_applied, payment_status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, schemaName)

	return r.exec(ctx, query,
		payslip.ID, payslip.TenantID, payslip.PayrollRunID, payslip.EmployeeID,
		payslip.GrossSalary, payslip.TaxableIncome, payslip.IncomeTax,
		payslip.UnemploymentInsuranceEE, payslip.FundedPension, payslip.NetSalary,
		payslip.SocialTax, payslip.UnemploymentInsuranceER, payslip.TotalEmployerCost,
		payslip.BasicExemptionApplied, payslip.PaymentStatus, payslip.CreatedAt,
	)
}

// Error definitions
var (
	ErrEmployeeNotFound   = fmt.Errorf("employee not found")
	ErrPayrollRunNotFound = fmt.Errorf("payroll run not found")
)

// UUIDGenerator interface for generating UUIDs (for testing)
type UUIDGenerator interface {
	New() string
}

// DefaultUUIDGenerator uses google/uuid
type DefaultUUIDGenerator struct{}

func (g *DefaultUUIDGenerator) New() string {
	return uuid.New().String()
}
