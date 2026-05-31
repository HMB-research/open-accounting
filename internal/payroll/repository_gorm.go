package payroll

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
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

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

// WithTransaction runs fn inside a GORM-backed transaction.
func (r *GORMRepository) WithTransaction(ctx context.Context, fn func(txRepo Repository) error) error {
	if r.db == nil {
		return fmt.Errorf("database connection not available")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GORMRepository{db: tx})
	})
}

// CreateEmployee inserts a new employee
func (r *GORMRepository) CreateEmployee(ctx context.Context, schemaName string, emp *Employee) error {
	db, err := r.tenantTable(ctx, schemaName, "employees")
	if err != nil {
		return err
	}

	empModel := employeeToModel(emp)
	if err := db.Create(empModel).Error; err != nil {
		return fmt.Errorf("create employee: %w", err)
	}
	return nil
}

// GetEmployee retrieves an employee by ID
func (r *GORMRepository) GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*Employee, error) {
	db, err := r.tenantTable(ctx, schemaName, "employees")
	if err != nil {
		return nil, err
	}

	var empModel models.Employee
	err = db.Where("tenant_id = ? AND id = ?", tenantID, employeeID).First(&empModel).Error
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
	db, err := r.tenantTable(ctx, schemaName, "employees")
	if err != nil {
		return nil, err
	}

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
	db, err := r.tenantTable(ctx, schemaName, "employees")
	if err != nil {
		return err
	}

	result := db.Where("tenant_id = ? AND id = ?", emp.TenantID, emp.ID).
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
	db, err := r.tenantTable(ctx, schemaName, "salary_components")
	if err != nil {
		return err
	}

	return db.Where("tenant_id = ? AND employee_id = ? AND component_type = ? AND effective_to IS NULL",
		tenantID, employeeID, "BASE_SALARY").
		Update("effective_to", effectiveTo).Error
}

// CreateSalaryComponent inserts a new salary component
func (r *GORMRepository) CreateSalaryComponent(ctx context.Context, schemaName string, comp *SalaryComponent) error {
	db, err := r.tenantTable(ctx, schemaName, "salary_components")
	if err != nil {
		return err
	}

	compModel := salaryComponentToModel(comp)
	if err := db.Create(compModel).Error; err != nil {
		return fmt.Errorf("create salary component: %w", err)
	}
	return nil
}

// ListSalaryComponents returns salary components for an employee.
func (r *GORMRepository) ListSalaryComponents(ctx context.Context, schemaName, tenantID, employeeID string, activeOn *time.Time) ([]SalaryComponent, error) {
	db, err := r.tenantTable(ctx, schemaName, "salary_components")
	if err != nil {
		return nil, err
	}

	query := db.Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID)
	if activeOn != nil {
		query = query.Where("effective_from <= ?", *activeOn).
			Where("effective_to IS NULL OR effective_to >= ?", *activeOn)
	}
	query = query.Order("effective_from DESC, created_at DESC, name")

	var compModels []models.SalaryComponent
	if err := query.Find(&compModels).Error; err != nil {
		return nil, fmt.Errorf("list salary components: %w", err)
	}

	components := make([]SalaryComponent, len(compModels))
	for i, compModel := range compModels {
		components[i] = *modelToSalaryComponent(&compModel)
	}
	return components, nil
}

// GetCurrentSalary returns the current salary for an employee
func (r *GORMRepository) GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error) {
	db, err := r.tenantTable(ctx, schemaName, "salary_components")
	if err != nil {
		return decimal.Zero, err
	}

	var result struct {
		Total models.Decimal
	}
	err = db.Select("COALESCE(SUM(amount), 0) as total").
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
	db, err := r.tenantTable(ctx, schemaName, "payroll_runs")
	if err != nil {
		return err
	}

	runModel := payrollRunToModel(run)
	if err := db.Create(runModel).Error; err != nil {
		return fmt.Errorf("create payroll run: %w", err)
	}
	return nil
}

// GetPayrollRun retrieves a payroll run by ID
func (r *GORMRepository) GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*PayrollRun, error) {
	db, err := r.tenantTable(ctx, schemaName, "payroll_runs")
	if err != nil {
		return nil, err
	}

	var runModel models.PayrollRun
	err = db.Where("tenant_id = ? AND id = ?", tenantID, runID).First(&runModel).Error
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
	db, err := r.tenantTable(ctx, schemaName, "payroll_runs")
	if err != nil {
		return nil, err
	}

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
	db, err := r.tenantTable(ctx, schemaName, "payroll_runs")
	if err != nil {
		return err
	}

	return db.Where("id = ?", run.ID).
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
	db, err := r.tenantTable(ctx, schemaName, "payroll_runs")
	if err != nil {
		return err
	}

	result := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, runID, PayrollCalculated).
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
	db, err := r.tenantTable(ctx, schemaName, "payslips")
	if err != nil {
		return err
	}

	return db.Where("payroll_run_id = ?", runID).Delete(&models.Payslip{}).Error
}

// CreatePayslip inserts a new payslip
func (r *GORMRepository) CreatePayslip(ctx context.Context, schemaName string, payslip *Payslip) error {
	db, err := r.tenantTable(ctx, schemaName, "payslips")
	if err != nil {
		return err
	}

	payslipModel := payslipToModel(payslip)
	if err := db.Create(payslipModel).Error; err != nil {
		return fmt.Errorf("create payslip: %w", err)
	}
	return nil
}

// GetPayslipsWithEmployees retrieves payslips with employee data.
func (r *GORMRepository) GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]Payslip, error) {
	payslipsTable, err := database.QualifiedTable(schemaName, "payslips")
	if err != nil {
		return nil, err
	}
	employeesTable, err := database.QualifiedTable(schemaName, "employees")
	if err != nil {
		return nil, err
	}

	var rows []struct {
		models.Payslip
		EmployeeFirstName    string
		EmployeeLastName     string
		EmployeePersonalCode string
		EmployeeEmail        string
	}
	if err := r.db.WithContext(ctx).Table(payslipsTable+" AS p").
		Select(`
			p.*,
			e.first_name AS employee_first_name,
			e.last_name AS employee_last_name,
			e.personal_code AS employee_personal_code,
			e.email AS employee_email
		`).
		Joins("JOIN "+employeesTable+" AS e ON e.id = p.employee_id").
		Where("p.tenant_id = ? AND p.payroll_run_id = ?", tenantID, payrollRunID).
		Order("e.last_name, e.first_name").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get payslips: %w", err)
	}

	payslips := make([]Payslip, len(rows))
	for i := range rows {
		payslip := *modelToPayslip(&rows[i].Payslip)
		payslip.Employee = &Employee{
			ID:           payslip.EmployeeID,
			FirstName:    rows[i].EmployeeFirstName,
			LastName:     rows[i].EmployeeLastName,
			PersonalCode: rows[i].EmployeePersonalCode,
			Email:        rows[i].EmployeeEmail,
		}
		payslips[i] = payslip
	}
	return payslips, nil
}

// DeleteTSDByPeriod removes an existing TSD declaration and cascaded rows for a period.
func (r *GORMRepository) DeleteTSDByPeriod(ctx context.Context, schemaName, tenantID string, year, month int) error {
	db, err := r.tenantTable(ctx, schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	if err := db.Where("tenant_id = ? AND period_year = ? AND period_month = ?", tenantID, year, month).
		Delete(&models.TSDDeclaration{}).Error; err != nil {
		return fmt.Errorf("delete TSD declaration: %w", err)
	}
	return nil
}

// CreateTSDDeclaration inserts a TSD declaration.
func (r *GORMRepository) CreateTSDDeclaration(ctx context.Context, schemaName string, declaration *TSDDeclaration) error {
	db, err := r.tenantTable(ctx, schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	if err := db.Create(tsdDeclarationToModel(declaration)).Error; err != nil {
		return fmt.Errorf("create TSD declaration: %w", err)
	}
	return nil
}

// CreateTSDRows inserts TSD declaration rows.
func (r *GORMRepository) CreateTSDRows(ctx context.Context, schemaName string, rows []TSDRow) error {
	if len(rows) == 0 {
		return nil
	}
	db, err := r.tenantTable(ctx, schemaName, "tsd_rows")
	if err != nil {
		return err
	}
	rowModels := make([]models.TSDRow, len(rows))
	for i := range rows {
		rowModels[i] = *tsdRowToModel(&rows[i])
	}
	if err := db.Create(&rowModels).Error; err != nil {
		return fmt.Errorf("create TSD rows: %w", err)
	}
	return nil
}

// GetTSD retrieves a TSD declaration by period.
func (r *GORMRepository) GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDDeclaration, error) {
	db, err := r.tenantTable(ctx, schemaName, "tsd_declarations")
	if err != nil {
		return nil, err
	}
	var declarationModel models.TSDDeclaration
	err = db.Where("tenant_id = ? AND period_year = ? AND period_month = ?", tenantID, year, month).
		First(&declarationModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTSDDeclarationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get TSD: %w", err)
	}

	declaration := modelToTSDDeclaration(&declarationModel)
	declaration.Rows, err = r.GetTSDRows(ctx, schemaName, tenantID, declaration.ID)
	if err != nil {
		return nil, err
	}
	return declaration, nil
}

// GetTSDRows retrieves all rows for a TSD declaration.
func (r *GORMRepository) GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]TSDRow, error) {
	db, err := r.tenantTable(ctx, schemaName, "tsd_rows")
	if err != nil {
		return nil, err
	}
	var rowModels []models.TSDRow
	if err := db.Where("tenant_id = ? AND declaration_id = ?", tenantID, declarationID).
		Order("last_name, first_name").
		Find(&rowModels).Error; err != nil {
		return nil, fmt.Errorf("get TSD rows: %w", err)
	}
	rows := make([]TSDRow, len(rowModels))
	for i := range rowModels {
		rows[i] = *modelToTSDRow(&rowModels[i])
	}
	return rows, nil
}

// ListTSD lists all TSD declarations for a tenant.
func (r *GORMRepository) ListTSD(ctx context.Context, schemaName, tenantID string) ([]TSDDeclaration, error) {
	db, err := r.tenantTable(ctx, schemaName, "tsd_declarations")
	if err != nil {
		return nil, err
	}
	var declarationModels []models.TSDDeclaration
	if err := db.Where("tenant_id = ?", tenantID).
		Order("period_year DESC, period_month DESC").
		Find(&declarationModels).Error; err != nil {
		return nil, fmt.Errorf("list TSD: %w", err)
	}
	declarations := make([]TSDDeclaration, len(declarationModels))
	for i := range declarationModels {
		declarations[i] = *modelToTSDDeclaration(&declarationModels[i])
	}
	return declarations, nil
}

// MarkTSDSubmitted marks a TSD declaration as submitted to e-MTA.
func (r *GORMRepository) MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string, submittedAt time.Time) error {
	db, err := r.tenantTable(ctx, schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	result := db.Where("tenant_id = ? AND id = ?", tenantID, declarationID).
		Updates(map[string]interface{}{
			"status":         TSDSubmitted,
			"submitted_at":   submittedAt,
			"emta_reference": emtaReference,
			"updated_at":     submittedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("mark TSD submitted: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTSDDeclarationNotFound
	}
	return nil
}

// UpdateTSDStatus updates a TSD declaration status.
func (r *GORMRepository) UpdateTSDStatus(ctx context.Context, schemaName, tenantID, declarationID string, status TSDStatus, updatedAt time.Time) error {
	db, err := r.tenantTable(ctx, schemaName, "tsd_declarations")
	if err != nil {
		return err
	}
	result := db.Where("tenant_id = ? AND id = ?", tenantID, declarationID).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update TSD status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrTSDDeclarationNotFound
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

func modelToSalaryComponent(m *models.SalaryComponent) *SalaryComponent {
	return &SalaryComponent{
		ID:            m.ID,
		TenantID:      m.TenantID,
		EmployeeID:    m.EmployeeID,
		ComponentType: m.ComponentType,
		Name:          m.Name,
		Amount:        m.Amount.Decimal,
		IsTaxable:     m.IsTaxable,
		IsRecurring:   m.IsRecurring,
		EffectiveFrom: m.EffectiveFrom,
		EffectiveTo:   m.EffectiveTo,
		CreatedAt:     m.CreatedAt,
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
		CreatedBy:         stringValue(m.CreatedBy),
		ApprovedBy:        stringValue(m.ApprovedBy),
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
		CreatedBy:         stringPtrIfNotBlank(r.CreatedBy),
		ApprovedBy:        stringPtrIfNotBlank(r.ApprovedBy),
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

func modelToPayslip(m *models.Payslip) *Payslip {
	return &Payslip{
		ID:                      m.ID,
		TenantID:                m.TenantID,
		PayrollRunID:            m.PayrollRunID,
		EmployeeID:              m.EmployeeID,
		GrossSalary:             m.GrossSalary.Decimal,
		TaxableIncome:           m.TaxableIncome.Decimal,
		IncomeTax:               m.IncomeTax.Decimal,
		UnemploymentInsuranceEE: m.UnemploymentInsuranceEE.Decimal,
		FundedPension:           m.FundedPension.Decimal,
		OtherDeductions:         m.OtherDeductions.Decimal,
		NetSalary:               m.NetSalary.Decimal,
		SocialTax:               m.SocialTax.Decimal,
		UnemploymentInsuranceER: m.UnemploymentInsuranceER.Decimal,
		TotalEmployerCost:       m.TotalEmployerCost.Decimal,
		BasicExemptionApplied:   m.BasicExemptionApplied.Decimal,
		PaymentStatus:           m.PaymentStatus,
		PaidAt:                  m.PaidAt,
		CreatedAt:               m.CreatedAt,
	}
}

func tsdDeclarationToModel(t *TSDDeclaration) *models.TSDDeclaration {
	return &models.TSDDeclaration{
		ID:                  t.ID,
		TenantID:            t.TenantID,
		PeriodYear:          t.PeriodYear,
		PeriodMonth:         t.PeriodMonth,
		PayrollRunID:        stringPtrIfNotBlank(t.PayrollRunID),
		TotalPayments:       models.Decimal{Decimal: t.TotalPayments},
		TotalIncomeTax:      models.Decimal{Decimal: t.TotalIncomeTax},
		TotalSocialTax:      models.Decimal{Decimal: t.TotalSocialTax},
		TotalUnemploymentER: models.Decimal{Decimal: t.TotalUnemploymentER},
		TotalUnemploymentEE: models.Decimal{Decimal: t.TotalUnemploymentEE},
		TotalFundedPension:  models.Decimal{Decimal: t.TotalFundedPension},
		Status:              string(t.Status),
		SubmittedAt:         t.SubmittedAt,
		EMTAReference:       t.EMTAReference,
		CreatedAt:           t.CreatedAt,
		UpdatedAt:           t.UpdatedAt,
	}
}

func modelToTSDDeclaration(m *models.TSDDeclaration) *TSDDeclaration {
	return &TSDDeclaration{
		ID:                  m.ID,
		TenantID:            m.TenantID,
		PeriodYear:          m.PeriodYear,
		PeriodMonth:         m.PeriodMonth,
		PayrollRunID:        stringValue(m.PayrollRunID),
		TotalPayments:       m.TotalPayments.Decimal,
		TotalIncomeTax:      m.TotalIncomeTax.Decimal,
		TotalSocialTax:      m.TotalSocialTax.Decimal,
		TotalUnemploymentER: m.TotalUnemploymentER.Decimal,
		TotalUnemploymentEE: m.TotalUnemploymentEE.Decimal,
		TotalFundedPension:  m.TotalFundedPension.Decimal,
		Status:              TSDStatus(m.Status),
		SubmittedAt:         m.SubmittedAt,
		EMTAReference:       m.EMTAReference,
		CreatedAt:           m.CreatedAt,
		UpdatedAt:           m.UpdatedAt,
	}
}

func stringPtrIfNotBlank(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func tsdRowToModel(t *TSDRow) *models.TSDRow {
	return &models.TSDRow{
		ID:             t.ID,
		TenantID:       t.TenantID,
		DeclarationID:  t.DeclarationID,
		EmployeeID:     t.EmployeeID,
		PersonalCode:   t.PersonalCode,
		FirstName:      t.FirstName,
		LastName:       t.LastName,
		PaymentType:    t.PaymentType,
		GrossPayment:   models.Decimal{Decimal: t.GrossPayment},
		BasicExemption: models.Decimal{Decimal: t.BasicExemption},
		TaxableAmount:  models.Decimal{Decimal: t.TaxableAmount},
		IncomeTax:      models.Decimal{Decimal: t.IncomeTax},
		SocialTax:      models.Decimal{Decimal: t.SocialTax},
		UnemploymentER: models.Decimal{Decimal: t.UnemploymentER},
		UnemploymentEE: models.Decimal{Decimal: t.UnemploymentEE},
		FundedPension:  models.Decimal{Decimal: t.FundedPension},
		CreatedAt:      t.CreatedAt,
	}
}

func modelToTSDRow(m *models.TSDRow) *TSDRow {
	return &TSDRow{
		ID:             m.ID,
		TenantID:       m.TenantID,
		DeclarationID:  m.DeclarationID,
		EmployeeID:     m.EmployeeID,
		PersonalCode:   m.PersonalCode,
		FirstName:      m.FirstName,
		LastName:       m.LastName,
		PaymentType:    m.PaymentType,
		GrossPayment:   m.GrossPayment.Decimal,
		BasicExemption: m.BasicExemption.Decimal,
		TaxableAmount:  m.TaxableAmount.Decimal,
		IncomeTax:      m.IncomeTax.Decimal,
		SocialTax:      m.SocialTax.Decimal,
		UnemploymentER: m.UnemploymentER.Decimal,
		UnemploymentEE: m.UnemploymentEE.Decimal,
		FundedPension:  m.FundedPension.Decimal,
		CreatedAt:      m.CreatedAt,
	}
}
