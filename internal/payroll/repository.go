package payroll

import (
	"context"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
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
	ListSalaryComponents(ctx context.Context, schemaName, tenantID, employeeID string, activeOn *time.Time) ([]SalaryComponent, error)
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

	// TSD operations
	GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]Payslip, error)
	DeleteTSDByPeriod(ctx context.Context, schemaName, tenantID string, year, month int) error
	CreateTSDDeclaration(ctx context.Context, schemaName string, declaration *TSDDeclaration) error
	CreateTSDRows(ctx context.Context, schemaName string, rows []TSDRow) error
	GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDDeclaration, error)
	GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]TSDRow, error)
	ListTSD(ctx context.Context, schemaName, tenantID string) ([]TSDDeclaration, error)
	MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string, submittedAt time.Time) error
	UpdateTSDStatus(ctx context.Context, schemaName, tenantID, declarationID string, status TSDStatus, updatedAt time.Time) error

	// Transaction support
	WithTransaction(ctx context.Context, fn func(txRepo Repository) error) error
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

// WithTransaction runs fn inside a repository-backed transaction.
func (r *PostgresRepository) WithTransaction(ctx context.Context, fn func(txRepo Repository) error) error {
	if r.tx != nil {
		return fn(r)
	}
	tx, err := r.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(r.WithTx(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
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
		INSERT INTO %s.salary_components (id, tenant_id, employee_id, component_type, name, amount, is_taxable, is_recurring, effective_from, effective_to, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
	`, schemaName)

	return r.exec(ctx, query, comp.ID, comp.TenantID, comp.EmployeeID, comp.ComponentType, comp.Name, comp.Amount, comp.IsTaxable, comp.IsRecurring, comp.EffectiveFrom, comp.EffectiveTo)
}

// ListSalaryComponents returns salary components for an employee.
func (r *PostgresRepository) ListSalaryComponents(ctx context.Context, schemaName, tenantID, employeeID string, activeOn *time.Time) ([]SalaryComponent, error) {
	query := fmt.Sprintf(`
		SELECT id, tenant_id, employee_id, component_type, name, amount, is_taxable, is_recurring, effective_from, effective_to, created_at
		FROM %s.salary_components
		WHERE tenant_id = $1 AND employee_id = $2
	`, schemaName)
	args := []interface{}{tenantID, employeeID}
	if activeOn != nil {
		query += " AND effective_from <= $3 AND (effective_to IS NULL OR effective_to >= $3)"
		args = append(args, *activeOn)
	}
	query += " ORDER BY effective_from DESC, created_at DESC, name"

	rows, err := r.query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	components := []SalaryComponent{}
	for rows.Next() {
		var comp SalaryComponent
		if err := rows.Scan(
			&comp.ID, &comp.TenantID, &comp.EmployeeID, &comp.ComponentType, &comp.Name,
			&comp.Amount, &comp.IsTaxable, &comp.IsRecurring, &comp.EffectiveFrom,
			&comp.EffectiveTo, &comp.CreatedAt,
		); err != nil {
			return nil, err
		}
		components = append(components, comp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return components, nil
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
		INSERT INTO %s.payroll_runs (
			id, tenant_id, period_year, period_month, status, payment_date,
			total_gross, total_net, total_employer_cost, notes,
			created_by, approved_by, approved_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`, schemaName)

	return r.exec(ctx, query,
		run.ID, run.TenantID, run.PeriodYear, run.PeriodMonth, run.Status,
		run.PaymentDate, run.TotalGross, run.TotalNet, run.TotalEmployerCost,
		run.Notes, run.CreatedBy, nullIfBlank(run.ApprovedBy), run.ApprovedAt,
		run.CreatedAt, run.UpdatedAt,
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
			income_tax, unemployment_insurance_employee, funded_pension, other_deductions, net_salary,
			social_tax, unemployment_insurance_employer, total_employer_cost,
			basic_exemption_applied, payment_status, paid_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`, schemaName)

	return r.exec(ctx, query,
		payslip.ID, payslip.TenantID, payslip.PayrollRunID, payslip.EmployeeID,
		payslip.GrossSalary, payslip.TaxableIncome, payslip.IncomeTax,
		payslip.UnemploymentInsuranceEE, payslip.FundedPension, payslip.OtherDeductions, payslip.NetSalary,
		payslip.SocialTax, payslip.UnemploymentInsuranceER, payslip.TotalEmployerCost,
		payslip.BasicExemptionApplied, payslip.PaymentStatus, payslip.PaidAt, payslip.CreatedAt,
	)
}

func payrollTable(schemaName, tableName string) (string, error) {
	return database.QualifiedTable(schemaName, tableName)
}

// GetPayslipsWithEmployees retrieves payslips with employee data.
func (r *PostgresRepository) GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]Payslip, error) {
	payslipsTable, err := payrollTable(schemaName, "payslips")
	if err != nil {
		return nil, err
	}
	employeesTable, err := payrollTable(schemaName, "employees")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT p.id, p.tenant_id, p.payroll_run_id, p.employee_id,
			p.gross_salary, p.taxable_income, p.income_tax, p.unemployment_insurance_employee,
			p.funded_pension, p.other_deductions, p.net_salary, p.social_tax,
			p.unemployment_insurance_employer, p.total_employer_cost, p.basic_exemption_applied,
			p.payment_status, p.paid_at, p.created_at,
			e.id, e.first_name, e.last_name, e.personal_code, e.email
		FROM %s p
		JOIN %s e ON e.id = p.employee_id
		WHERE p.tenant_id = $1 AND p.payroll_run_id = $2
		ORDER BY e.last_name, e.first_name
	`, payslipsTable, employeesTable)

	rows, err := r.query(ctx, query, tenantID, payrollRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payslips := []Payslip{}
	for rows.Next() {
		var payslip Payslip
		var employee Employee
		if err := rows.Scan(
			&payslip.ID, &payslip.TenantID, &payslip.PayrollRunID, &payslip.EmployeeID,
			&payslip.GrossSalary, &payslip.TaxableIncome, &payslip.IncomeTax, &payslip.UnemploymentInsuranceEE,
			&payslip.FundedPension, &payslip.OtherDeductions, &payslip.NetSalary, &payslip.SocialTax,
			&payslip.UnemploymentInsuranceER, &payslip.TotalEmployerCost, &payslip.BasicExemptionApplied,
			&payslip.PaymentStatus, &payslip.PaidAt, &payslip.CreatedAt,
			&employee.ID, &employee.FirstName, &employee.LastName, &employee.PersonalCode, &employee.Email,
		); err != nil {
			return nil, err
		}
		payslip.Employee = &employee
		payslips = append(payslips, payslip)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return payslips, nil
}

// DeleteTSDByPeriod removes an existing TSD declaration and cascaded rows for a period.
func (r *PostgresRepository) DeleteTSDByPeriod(ctx context.Context, schemaName, tenantID string, year, month int) error {
	declarationsTable, err := payrollTable(schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE tenant_id = $1 AND period_year = $2 AND period_month = $3
	`, declarationsTable)
	return r.exec(ctx, query, tenantID, year, month)
}

// CreateTSDDeclaration inserts a TSD declaration.
func (r *PostgresRepository) CreateTSDDeclaration(ctx context.Context, schemaName string, declaration *TSDDeclaration) error {
	declarationsTable, err := payrollTable(schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		INSERT INTO %s (
			id, tenant_id, period_year, period_month, payroll_run_id,
			total_payments, total_income_tax, total_social_tax,
			total_unemployment_employer, total_unemployment_employee, total_funded_pension,
			status, submitted_at, emta_reference, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, declarationsTable)

	return r.exec(ctx, query,
		declaration.ID, declaration.TenantID, declaration.PeriodYear, declaration.PeriodMonth,
		nullIfBlank(declaration.PayrollRunID), declaration.TotalPayments, declaration.TotalIncomeTax,
		declaration.TotalSocialTax, declaration.TotalUnemploymentER, declaration.TotalUnemploymentEE,
		declaration.TotalFundedPension, declaration.Status, declaration.SubmittedAt,
		nullIfBlank(declaration.EMTAReference), declaration.CreatedAt, declaration.UpdatedAt,
	)
}

// CreateTSDRows inserts TSD declaration rows.
func (r *PostgresRepository) CreateTSDRows(ctx context.Context, schemaName string, rows []TSDRow) error {
	if len(rows) == 0 {
		return nil
	}
	rowsTable, err := payrollTable(schemaName, "tsd_rows")
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		INSERT INTO %s (
			id, tenant_id, declaration_id, employee_id, personal_code, first_name, last_name,
			payment_type, gross_payment, basic_exemption, taxable_amount,
			income_tax, social_tax, unemployment_insurance_employer, unemployment_insurance_employee,
			funded_pension, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, rowsTable)

	for i := range rows {
		row := rows[i]
		if err := r.exec(ctx, query,
			row.ID, row.TenantID, row.DeclarationID, row.EmployeeID,
			row.PersonalCode, row.FirstName, row.LastName, row.PaymentType,
			row.GrossPayment, row.BasicExemption, row.TaxableAmount,
			row.IncomeTax, row.SocialTax, row.UnemploymentER, row.UnemploymentEE,
			row.FundedPension, row.CreatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetTSD retrieves a TSD declaration by period.
func (r *PostgresRepository) GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDDeclaration, error) {
	declarationsTable, err := payrollTable(schemaName, "tsd_declarations")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, COALESCE(payroll_run_id::text, ''),
			total_payments, total_income_tax, total_social_tax,
			total_unemployment_employer, total_unemployment_employee, total_funded_pension,
			status, submitted_at, COALESCE(emta_reference, ''), created_at, updated_at
		FROM %s
		WHERE tenant_id = $1 AND period_year = $2 AND period_month = $3
	`, declarationsTable)

	var declaration TSDDeclaration
	err = r.queryRow(ctx, query, tenantID, year, month).Scan(
		&declaration.ID, &declaration.TenantID, &declaration.PeriodYear, &declaration.PeriodMonth,
		&declaration.PayrollRunID, &declaration.TotalPayments, &declaration.TotalIncomeTax,
		&declaration.TotalSocialTax, &declaration.TotalUnemploymentER, &declaration.TotalUnemploymentEE,
		&declaration.TotalFundedPension, &declaration.Status, &declaration.SubmittedAt,
		&declaration.EMTAReference, &declaration.CreatedAt, &declaration.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrTSDDeclarationNotFound
	}
	if err != nil {
		return nil, err
	}

	declaration.Rows, err = r.GetTSDRows(ctx, schemaName, tenantID, declaration.ID)
	if err != nil {
		return nil, err
	}
	return &declaration, nil
}

// GetTSDRows retrieves all rows for a TSD declaration.
func (r *PostgresRepository) GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]TSDRow, error) {
	rowsTable, err := payrollTable(schemaName, "tsd_rows")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT id, tenant_id, declaration_id, employee_id, personal_code, first_name, last_name,
			payment_type, gross_payment, basic_exemption, taxable_amount,
			income_tax, social_tax, unemployment_insurance_employer, unemployment_insurance_employee,
			funded_pension, created_at
		FROM %s
		WHERE tenant_id = $1 AND declaration_id = $2
		ORDER BY last_name, first_name
	`, rowsTable)

	rows, err := r.query(ctx, query, tenantID, declarationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tsdRows := []TSDRow{}
	for rows.Next() {
		var row TSDRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.DeclarationID, &row.EmployeeID,
			&row.PersonalCode, &row.FirstName, &row.LastName, &row.PaymentType,
			&row.GrossPayment, &row.BasicExemption, &row.TaxableAmount,
			&row.IncomeTax, &row.SocialTax, &row.UnemploymentER, &row.UnemploymentEE,
			&row.FundedPension, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		tsdRows = append(tsdRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tsdRows, nil
}

// ListTSD lists all TSD declarations for a tenant.
func (r *PostgresRepository) ListTSD(ctx context.Context, schemaName, tenantID string) ([]TSDDeclaration, error) {
	declarationsTable, err := payrollTable(schemaName, "tsd_declarations")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		SELECT id, tenant_id, period_year, period_month, COALESCE(payroll_run_id::text, ''),
			total_payments, total_income_tax, total_social_tax,
			total_unemployment_employer, total_unemployment_employee, total_funded_pension,
			status, submitted_at, COALESCE(emta_reference, ''), created_at, updated_at
		FROM %s
		WHERE tenant_id = $1
		ORDER BY period_year DESC, period_month DESC
	`, declarationsTable)

	rows, err := r.query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	declarations := []TSDDeclaration{}
	for rows.Next() {
		var declaration TSDDeclaration
		if err := rows.Scan(
			&declaration.ID, &declaration.TenantID, &declaration.PeriodYear, &declaration.PeriodMonth,
			&declaration.PayrollRunID, &declaration.TotalPayments, &declaration.TotalIncomeTax,
			&declaration.TotalSocialTax, &declaration.TotalUnemploymentER, &declaration.TotalUnemploymentEE,
			&declaration.TotalFundedPension, &declaration.Status, &declaration.SubmittedAt,
			&declaration.EMTAReference, &declaration.CreatedAt, &declaration.UpdatedAt,
		); err != nil {
			return nil, err
		}
		declarations = append(declarations, declaration)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return declarations, nil
}

// MarkTSDSubmitted marks a TSD declaration as submitted to e-MTA.
func (r *PostgresRepository) MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string, submittedAt time.Time) error {
	declarationsTable, err := payrollTable(schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		UPDATE %s
		SET status = $1, submitted_at = $2, emta_reference = $3, updated_at = $4
		WHERE tenant_id = $5 AND id = $6
	`, declarationsTable)
	return r.exec(ctx, query, TSDSubmitted, submittedAt, emtaReference, submittedAt, tenantID, declarationID)
}

// UpdateTSDStatus updates a TSD declaration status.
func (r *PostgresRepository) UpdateTSDStatus(ctx context.Context, schemaName, tenantID, declarationID string, status TSDStatus, updatedAt time.Time) error {
	declarationsTable, err := payrollTable(schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	query := fmt.Sprintf(`
		UPDATE %s
		SET status = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4
	`, declarationsTable)
	return r.exec(ctx, query, status, updatedAt, tenantID, declarationID)
}

// Error definitions
var (
	ErrEmployeeNotFound       = fmt.Errorf("employee not found")
	ErrPayrollRunNotFound     = fmt.Errorf("payroll run not found")
	ErrTSDDeclarationNotFound = fmt.Errorf("TSD declaration not found")
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

func nullIfBlank(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
