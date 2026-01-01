//go:build gorm

package payroll

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// GORMRepository implements Repository using GORM
type GORMRepository struct {
	db *gorm.DB
}

// NewGORMRepository creates a new GORM payroll repository
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

// BeginTx is not supported in GORM implementation
// Use GORM's Transaction method instead
func (r *GORMRepository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return nil, fmt.Errorf("BeginTx is not supported in GORM implementation; use internal transactions")
}

// WithTx is not supported in GORM implementation
func (r *GORMRepository) WithTx(tx pgx.Tx) Repository {
	return r // Return self - transactions are handled internally
}

// CreateEmployee inserts a new employee
func (r *GORMRepository) CreateEmployee(ctx context.Context, schemaName string, emp *Employee) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	empModel := employeeToModel(emp)
	if err := db.Create(empModel).Error; err != nil {
		return fmt.Errorf("create employee: %w", err)
	}
	return nil
}

// GetEmployee retrieves an employee by ID
func (r *GORMRepository) GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var empModel models.Employee
	err := db.Where("tenant_id = ? AND id = ?", tenantID, employeeID).First(&empModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrEmployeeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get employee: %w", err)
	}

	return modelToEmployee(&empModel), nil
}

// ListEmployees returns employees for a tenant
func (r *GORMRepository) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]Employee, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	query := db.Where("tenant_id = ?", tenantID)
	if activeOnly {
		query = query.Where("is_active = ?", true)
	}
	query = query.Order("last_name, first_name")

	var empModels []models.Employee
	if err := query.Find(&empModels).Error; err != nil {
		return nil, fmt.Errorf("list employees: %w", err)
	}

	employees := make([]Employee, len(empModels))
	for i, em := range empModels {
		employees[i] = *modelToEmployee(&em)
	}

	return employees, nil
}

// UpdateEmployee updates an existing employee
func (r *GORMRepository) UpdateEmployee(ctx context.Context, schemaName string, emp *Employee) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.Employee{}).
		Where("tenant_id = ? AND id = ?", emp.TenantID, emp.ID).
		Updates(map[string]interface{}{
			"employee_number":        emp.EmployeeNumber,
			"first_name":             emp.FirstName,
			"last_name":              emp.LastName,
			"personal_code":          emp.PersonalCode,
			"email":                  emp.Email,
			"phone":                  emp.Phone,
			"address":                emp.Address,
			"bank_account":           emp.BankAccount,
			"end_date":               emp.EndDate,
			"position":               emp.Position,
			"department":             emp.Department,
			"employment_type":        emp.EmploymentType,
			"apply_basic_exemption":  emp.ApplyBasicExemption,
			"basic_exemption_amount": emp.BasicExemptionAmount.String(),
			"funded_pension_rate":    emp.FundedPensionRate.String(),
			"is_active":              emp.IsActive,
			"updated_at":             emp.UpdatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update employee: %w", result.Error)
	}
	return nil
}

// EndCurrentBaseSalary ends an existing base salary
func (r *GORMRepository) EndCurrentBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, effectiveTo time.Time) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	return db.Model(&models.SalaryComponent{}).
		Where("tenant_id = ? AND employee_id = ? AND component_type = ? AND effective_to IS NULL",
			tenantID, employeeID, "BASE_SALARY").
		Update("effective_to", effectiveTo).Error
}

// CreateSalaryComponent inserts a new salary component
func (r *GORMRepository) CreateSalaryComponent(ctx context.Context, schemaName string, comp *SalaryComponent) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	compModel := salaryComponentToModel(comp)
	if err := db.Create(compModel).Error; err != nil {
		return fmt.Errorf("create salary component: %w", err)
	}
	return nil
}

// GetCurrentSalary returns the current salary for an employee
func (r *GORMRepository) GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var result struct {
		Total models.Decimal
	}
	err := db.Model(&models.SalaryComponent{}).
		Select("COALESCE(SUM(amount), 0) as total").
		Where("tenant_id = ? AND employee_id = ? AND is_recurring = ?", tenantID, employeeID, true).
		Where("effective_from <= CURRENT_DATE").
		Where("effective_to IS NULL OR effective_to >= CURRENT_DATE").
		Scan(&result).Error
	if err != nil {
		return decimal.Zero, fmt.Errorf("get current salary: %w", err)
	}

	return result.Total.Decimal, nil
}

// CreatePayrollRun inserts a new payroll run
func (r *GORMRepository) CreatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	runModel := payrollRunToModel(run)
	if err := db.Create(runModel).Error; err != nil {
		return fmt.Errorf("create payroll run: %w", err)
	}
	return nil
}

// GetPayrollRun retrieves a payroll run by ID
func (r *GORMRepository) GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	var runModel models.PayrollRun
	err := db.Where("tenant_id = ? AND id = ?", tenantID, runID).First(&runModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPayrollRunNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get payroll run: %w", err)
	}

	return modelToPayrollRun(&runModel), nil
}

// ListPayrollRuns lists payroll runs for a tenant
func (r *GORMRepository) ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]PayrollRun, error) {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	query := db.Where("tenant_id = ?", tenantID)
	if year > 0 {
		query = query.Where("period_year = ?", year)
	}
	query = query.Order("period_year DESC, period_month DESC")

	var runModels []models.PayrollRun
	if err := query.Find(&runModels).Error; err != nil {
		return nil, fmt.Errorf("list payroll runs: %w", err)
	}

	runs := make([]PayrollRun, len(runModels))
	for i, rm := range runModels {
		runs[i] = *modelToPayrollRun(&rm)
	}

	return runs, nil
}

// UpdatePayrollRun updates a payroll run
func (r *GORMRepository) UpdatePayrollRun(ctx context.Context, schemaName string, run *PayrollRun) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	return db.Model(&models.PayrollRun{}).
		Where("id = ?", run.ID).
		Updates(map[string]interface{}{
			"status":              run.Status,
			"total_gross":         run.TotalGross.String(),
			"total_net":           run.TotalNet.String(),
			"total_employer_cost": run.TotalEmployerCost.String(),
			"updated_at":          time.Now(),
		}).Error
}

// ApprovePayrollRun approves a payroll run
func (r *GORMRepository) ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	result := db.Model(&models.PayrollRun{}).
		Where("tenant_id = ? AND id = ? AND status = ?", tenantID, runID, PayrollCalculated).
		Updates(map[string]interface{}{
			"status":      PayrollApproved,
			"approved_by": approverID,
			"approved_at": time.Now(),
			"updated_at":  time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("approve payroll run: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrPayrollRunNotFound
	}
	return nil
}

// DeletePayslipsByRunID deletes all payslips for a run
func (r *GORMRepository) DeletePayslipsByRunID(ctx context.Context, schemaName, runID string) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	return db.Where("payroll_run_id = ?", runID).Delete(&models.Payslip{}).Error
}

// CreatePayslip inserts a new payslip
func (r *GORMRepository) CreatePayslip(ctx context.Context, schemaName string, payslip *Payslip) error {
	db := database.TenantDB(r.db, schemaName).WithContext(ctx)

	payslipModel := payslipToModel(payslip)
	if err := db.Create(payslipModel).Error; err != nil {
		return fmt.Errorf("create payslip: %w", err)
	}
	return nil
}

// Conversion helpers

func modelToEmployee(m *models.Employee) *Employee {
	return &Employee{
		ID:                   m.ID,
		TenantID:             m.TenantID,
		EmployeeNumber:       m.EmployeeNumber,
		FirstName:            m.FirstName,
		LastName:             m.LastName,
		PersonalCode:         m.PersonalCode,
		Email:                m.Email,
		Phone:                m.Phone,
		Address:              m.Address,
		BankAccount:          m.BankAccount,
		StartDate:            m.StartDate,
		EndDate:              m.EndDate,
		Position:             m.Position,
		Department:           m.Department,
		EmploymentType:       EmploymentType(m.EmploymentType),
		TaxResidency:         m.TaxResidency,
		ApplyBasicExemption:  m.ApplyBasicExemption,
		BasicExemptionAmount: m.BasicExemptionAmount.Decimal,
		FundedPensionRate:    m.FundedPensionRate.Decimal,
		IsActive:             m.IsActive,
		CreatedAt:            m.CreatedAt,
		UpdatedAt:            m.UpdatedAt,
	}
}

func employeeToModel(e *Employee) *models.Employee {
	return &models.Employee{
		ID:                   e.ID,
		TenantID:             e.TenantID,
		EmployeeNumber:       e.EmployeeNumber,
		FirstName:            e.FirstName,
		LastName:             e.LastName,
		PersonalCode:         e.PersonalCode,
		Email:                e.Email,
		Phone:                e.Phone,
		Address:              e.Address,
		BankAccount:          e.BankAccount,
		StartDate:            e.StartDate,
		EndDate:              e.EndDate,
		Position:             e.Position,
		Department:           e.Department,
		EmploymentType:       models.EmploymentType(e.EmploymentType),
		TaxResidency:         e.TaxResidency,
		ApplyBasicExemption:  e.ApplyBasicExemption,
		BasicExemptionAmount: models.Decimal{Decimal: e.BasicExemptionAmount},
		FundedPensionRate:    models.Decimal{Decimal: e.FundedPensionRate},
		IsActive:             e.IsActive,
		CreatedAt:            e.CreatedAt,
		UpdatedAt:            e.UpdatedAt,
	}
}

func salaryComponentToModel(s *SalaryComponent) *models.SalaryComponent {
	return &models.SalaryComponent{
		ID:            s.ID,
		TenantID:      s.TenantID,
		EmployeeID:    s.EmployeeID,
		ComponentType: s.ComponentType,
		Name:          s.Name,
		Amount:        models.Decimal{Decimal: s.Amount},
		IsTaxable:     s.IsTaxable,
		IsRecurring:   s.IsRecurring,
		EffectiveFrom: s.EffectiveFrom,
		EffectiveTo:   s.EffectiveTo,
		CreatedAt:     s.CreatedAt,
	}
}

func modelToPayrollRun(m *models.PayrollRun) *PayrollRun {
	return &PayrollRun{
		ID:                m.ID,
		TenantID:          m.TenantID,
		PeriodYear:        m.PeriodYear,
		PeriodMonth:       m.PeriodMonth,
		Status:            PayrollStatus(m.Status),
		PaymentDate:       m.PaymentDate,
		TotalGross:        m.TotalGross.Decimal,
		TotalNet:          m.TotalNet.Decimal,
		TotalEmployerCost: m.TotalEmployerCost.Decimal,
		Notes:             m.Notes,
		CreatedBy:         m.CreatedBy,
		ApprovedBy:        m.ApprovedBy,
		ApprovedAt:        m.ApprovedAt,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

func payrollRunToModel(r *PayrollRun) *models.PayrollRun {
	return &models.PayrollRun{
		ID:                r.ID,
		TenantID:          r.TenantID,
		PeriodYear:        r.PeriodYear,
		PeriodMonth:       r.PeriodMonth,
		Status:            models.PayrollStatus(r.Status),
		PaymentDate:       r.PaymentDate,
		TotalGross:        models.Decimal{Decimal: r.TotalGross},
		TotalNet:          models.Decimal{Decimal: r.TotalNet},
		TotalEmployerCost: models.Decimal{Decimal: r.TotalEmployerCost},
		Notes:             r.Notes,
		CreatedBy:         r.CreatedBy,
		ApprovedBy:        r.ApprovedBy,
		ApprovedAt:        r.ApprovedAt,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

func payslipToModel(p *Payslip) *models.Payslip {
	return &models.Payslip{
		ID:                      p.ID,
		TenantID:                p.TenantID,
		PayrollRunID:            p.PayrollRunID,
		EmployeeID:              p.EmployeeID,
		GrossSalary:             models.Decimal{Decimal: p.GrossSalary},
		TaxableIncome:           models.Decimal{Decimal: p.TaxableIncome},
		IncomeTax:               models.Decimal{Decimal: p.IncomeTax},
		UnemploymentInsuranceEE: models.Decimal{Decimal: p.UnemploymentInsuranceEE},
		FundedPension:           models.Decimal{Decimal: p.FundedPension},
		OtherDeductions:         models.Decimal{Decimal: p.OtherDeductions},
		NetSalary:               models.Decimal{Decimal: p.NetSalary},
		SocialTax:               models.Decimal{Decimal: p.SocialTax},
		UnemploymentInsuranceER: models.Decimal{Decimal: p.UnemploymentInsuranceER},
		TotalEmployerCost:       models.Decimal{Decimal: p.TotalEmployerCost},
		BasicExemptionApplied:   models.Decimal{Decimal: p.BasicExemptionApplied},
		PaymentStatus:           p.PaymentStatus,
		PaidAt:                  p.PaidAt,
		CreatedAt:               p.CreatedAt,
	}
}
