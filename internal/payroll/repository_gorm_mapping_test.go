package payroll

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepositoryNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	now := time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC)
	employee := &Employee{
		ID:             "employee-1",
		TenantID:       tenantID,
		EmployeeNumber: "EMP-001",
		FirstName:      "Mari",
		LastName:       "Mets",
		StartDate:      now,
	}
	component := &SalaryComponent{
		ID:            "component-1",
		TenantID:      tenantID,
		EmployeeID:    employee.ID,
		ComponentType: SalaryComponentBaseSalary,
		Name:          "Base salary",
		Amount:        decimal.NewFromInt(2500),
		EffectiveFrom: now,
	}
	run := &PayrollRun{
		ID:          "run-1",
		TenantID:    tenantID,
		PeriodYear:  2026,
		PeriodMonth: 6,
		Status:      PayrollCalculated,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	payslip := &Payslip{
		ID:           "payslip-1",
		TenantID:     tenantID,
		PayrollRunID: run.ID,
		EmployeeID:   employee.ID,
		CreatedAt:    now,
	}
	declaration := &TSDDeclaration{
		ID:          "tsd-1",
		TenantID:    tenantID,
		PeriodYear:  2026,
		PeriodMonth: 6,
		Status:      TSDDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	row := TSDRow{
		ID:            "tsd-row-1",
		TenantID:      tenantID,
		DeclarationID: declaration.ID,
		EmployeeID:    employee.ID,
		PaymentType:   PaymentTypeSalary,
		CreatedAt:     now,
	}

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	err := repo.CreateTSDRows(ctx, schemaName, nil)
	require.NoError(t, err)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "dbWithContext",
			run: func(t *testing.T) error {
				db, err := repo.dbWithContext(ctx)
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				db, err := repo.tenantTable(ctx, schemaName, "employees")
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "WithTransaction",
			run: func(t *testing.T) error {
				called := false
				err := repo.WithTransaction(ctx, func(txRepo Repository) error {
					called = true
					return nil
				})
				assert.False(t, called)
				return err
			},
		},
		{
			name: "CreateEmployee",
			run: func(t *testing.T) error {
				return repo.CreateEmployee(ctx, schemaName, employee)
			},
		},
		{
			name: "GetEmployee",
			run: func(t *testing.T) error {
				gotEmployee, err := repo.GetEmployee(ctx, schemaName, tenantID, employee.ID)
				assert.Nil(t, gotEmployee)
				return err
			},
		},
		{
			name: "ListEmployees",
			run: func(t *testing.T) error {
				employees, err := repo.ListEmployees(ctx, schemaName, tenantID, true)
				assert.Nil(t, employees)
				return err
			},
		},
		{
			name: "UpdateEmployee",
			run: func(t *testing.T) error {
				return repo.UpdateEmployee(ctx, schemaName, employee)
			},
		},
		{
			name: "EndCurrentBaseSalary",
			run: func(t *testing.T) error {
				return repo.EndCurrentBaseSalary(ctx, schemaName, tenantID, employee.ID, now)
			},
		},
		{
			name: "CreateSalaryComponent",
			run: func(t *testing.T) error {
				return repo.CreateSalaryComponent(ctx, schemaName, component)
			},
		},
		{
			name: "ListSalaryComponents",
			run: func(t *testing.T) error {
				components, err := repo.ListSalaryComponents(ctx, schemaName, tenantID, employee.ID, &now)
				assert.Nil(t, components)
				return err
			},
		},
		{
			name: "GetCurrentSalary",
			run: func(t *testing.T) error {
				salary, err := repo.GetCurrentSalary(ctx, schemaName, tenantID, employee.ID)
				assert.True(t, salary.IsZero())
				return err
			},
		},
		{
			name: "CreatePayrollRun",
			run: func(t *testing.T) error {
				return repo.CreatePayrollRun(ctx, schemaName, run)
			},
		},
		{
			name: "GetPayrollRun",
			run: func(t *testing.T) error {
				gotRun, err := repo.GetPayrollRun(ctx, schemaName, tenantID, run.ID)
				assert.Nil(t, gotRun)
				return err
			},
		},
		{
			name: "ListPayrollRuns",
			run: func(t *testing.T) error {
				runs, err := repo.ListPayrollRuns(ctx, schemaName, tenantID, 2026)
				assert.Nil(t, runs)
				return err
			},
		},
		{
			name: "UpdatePayrollRun",
			run: func(t *testing.T) error {
				return repo.UpdatePayrollRun(ctx, schemaName, run)
			},
		},
		{
			name: "ApprovePayrollRun",
			run: func(t *testing.T) error {
				return repo.ApprovePayrollRun(ctx, schemaName, tenantID, run.ID, "approver-1")
			},
		},
		{
			name: "DeletePayslipsByRunID",
			run: func(t *testing.T) error {
				return repo.DeletePayslipsByRunID(ctx, schemaName, run.ID)
			},
		},
		{
			name: "CreatePayslip",
			run: func(t *testing.T) error {
				return repo.CreatePayslip(ctx, schemaName, payslip)
			},
		},
		{
			name: "GetPayslipsWithEmployees",
			run: func(t *testing.T) error {
				payslips, err := repo.GetPayslipsWithEmployees(ctx, schemaName, tenantID, run.ID)
				assert.Nil(t, payslips)
				return err
			},
		},
		{
			name: "DeleteTSDByPeriod",
			run: func(t *testing.T) error {
				return repo.DeleteTSDByPeriod(ctx, schemaName, tenantID, 2026, 6)
			},
		},
		{
			name: "CreateTSDDeclaration",
			run: func(t *testing.T) error {
				return repo.CreateTSDDeclaration(ctx, schemaName, declaration)
			},
		},
		{
			name: "CreateTSDRows",
			run: func(t *testing.T) error {
				return repo.CreateTSDRows(ctx, schemaName, []TSDRow{row})
			},
		},
		{
			name: "GetTSD",
			run: func(t *testing.T) error {
				gotDeclaration, err := repo.GetTSD(ctx, schemaName, tenantID, 2026, 6)
				assert.Nil(t, gotDeclaration)
				return err
			},
		},
		{
			name: "GetTSDRows",
			run: func(t *testing.T) error {
				rows, err := repo.GetTSDRows(ctx, schemaName, tenantID, declaration.ID)
				assert.Nil(t, rows)
				return err
			},
		},
		{
			name: "ListTSD",
			run: func(t *testing.T) error {
				declarations, err := repo.ListTSD(ctx, schemaName, tenantID, TSDListFilter{Year: 2026, Month: 6})
				assert.Nil(t, declarations)
				return err
			},
		},
		{
			name: "MarkTSDSubmitted",
			run: func(t *testing.T) error {
				return repo.MarkTSDSubmitted(ctx, schemaName, tenantID, declaration.ID, "EMTA-REF-1", now)
			},
		},
		{
			name: "UpdateTSDStatus",
			run: func(t *testing.T) error {
				return repo.UpdateTSDStatus(ctx, schemaName, tenantID, declaration.ID, TSDSubmitted, now)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "payroll repository database is not configured")
		})
	}
}

func TestEmployeeModelMappings(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	endDate := now.AddDate(1, 0, 0)
	employee := &Employee{
		ID:                   uuid.NewString(),
		TenantID:             uuid.NewString(),
		EmployeeNumber:       "EMP-100",
		FirstName:            "Marta",
		LastName:             "Tamm",
		PersonalCode:         "49001010010",
		Email:                "marta@example.com",
		Phone:                "+3725550100",
		Address:              "Payroll 1",
		BankAccount:          "EE383800853212345678",
		StartDate:            now.AddDate(-2, 0, 0),
		EndDate:              &endDate,
		Position:             "Accountant",
		Department:           "Finance",
		EmploymentType:       EmploymentPartTime,
		TaxResidency:         "EE",
		ApplyBasicExemption:  true,
		BasicExemptionAmount: decimal.NewFromInt(500),
		FundedPensionRate:    decimal.NewFromFloat(0.04),
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now.Add(time.Hour),
	}

	model := employeeToModel(employee)

	assert.Equal(t, employee.ID, model.ID)
	assert.Equal(t, employee.TenantID, model.TenantID)
	assert.Equal(t, employee.EmployeeNumber, model.EmployeeNumber)
	assert.Equal(t, employee.FirstName, model.FirstName)
	assert.Equal(t, employee.LastName, model.LastName)
	assert.Equal(t, employee.PersonalCode, model.PersonalCode)
	assert.Equal(t, employee.Email, model.Email)
	assert.Equal(t, employee.Phone, model.Phone)
	assert.Equal(t, employee.Address, model.Address)
	assert.Equal(t, employee.BankAccount, model.BankAccount)
	assert.Equal(t, employee.StartDate, model.StartDate)
	assert.Equal(t, employee.EndDate, model.EndDate)
	assert.Equal(t, employee.Position, model.Position)
	assert.Equal(t, employee.Department, model.Department)
	assert.Equal(t, models.EmploymentType(employee.EmploymentType), model.EmploymentType)
	assert.Equal(t, employee.TaxResidency, model.TaxResidency)
	assert.Equal(t, employee.ApplyBasicExemption, model.ApplyBasicExemption)
	requireDecimalEqual(t, model.BasicExemptionAmount.Decimal, employee.BasicExemptionAmount)
	requireDecimalEqual(t, model.FundedPensionRate.Decimal, employee.FundedPensionRate)
	assert.Equal(t, employee.IsActive, model.IsActive)
	assert.Equal(t, employee.CreatedAt, model.CreatedAt)
	assert.Equal(t, employee.UpdatedAt, model.UpdatedAt)

	roundTrip := modelToEmployee(model)
	assert.Equal(t, employee.ID, roundTrip.ID)
	assert.Equal(t, employee.TenantID, roundTrip.TenantID)
	assert.Equal(t, employee.EmployeeNumber, roundTrip.EmployeeNumber)
	assert.Equal(t, employee.FirstName, roundTrip.FirstName)
	assert.Equal(t, employee.LastName, roundTrip.LastName)
	assert.Equal(t, employee.EmploymentType, roundTrip.EmploymentType)
	requireDecimalEqual(t, roundTrip.BasicExemptionAmount, employee.BasicExemptionAmount)
	requireDecimalEqual(t, roundTrip.FundedPensionRate, employee.FundedPensionRate)
	assert.Equal(t, employee.IsActive, roundTrip.IsActive)

	values := employeeCreateValues(employee)
	require.Len(t, values, 22)
	assert.Equal(t, employee.ID, values["id"])
	assert.Equal(t, employee.TenantID, values["tenant_id"])
	assert.Equal(t, employee.EmployeeNumber, values["employee_number"])
	assert.Equal(t, employee.FirstName, values["first_name"])
	assert.Equal(t, models.EmploymentType(employee.EmploymentType), values["employment_type"])
	assert.Equal(t, employee.EndDate, values["end_date"])
	requireDecimalEqual(t, values["basic_exemption_amount"].(models.Decimal).Decimal, employee.BasicExemptionAmount)
	requireDecimalEqual(t, values["funded_pension_rate"].(models.Decimal).Decimal, employee.FundedPensionRate)
	assert.Equal(t, employee.UpdatedAt, values["updated_at"])
}

func TestSalaryComponentModelMappings(t *testing.T) {
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	effectiveTo := now.AddDate(0, 6, 0)
	component := &SalaryComponent{
		ID:            uuid.NewString(),
		TenantID:      uuid.NewString(),
		EmployeeID:    uuid.NewString(),
		ComponentType: SalaryComponentBonus,
		Name:          "Quarterly bonus",
		Amount:        decimal.NewFromFloat(1250.75),
		IsTaxable:     true,
		IsRecurring:   false,
		EffectiveFrom: now,
		EffectiveTo:   &effectiveTo,
		CreatedAt:     now.Add(time.Hour),
	}

	model := salaryComponentToModel(component)

	assert.Equal(t, component.ID, model.ID)
	assert.Equal(t, component.TenantID, model.TenantID)
	assert.Equal(t, component.EmployeeID, model.EmployeeID)
	assert.Equal(t, component.ComponentType, model.ComponentType)
	assert.Equal(t, component.Name, model.Name)
	requireDecimalEqual(t, model.Amount.Decimal, component.Amount)
	assert.Equal(t, component.IsTaxable, model.IsTaxable)
	assert.Equal(t, component.IsRecurring, model.IsRecurring)
	assert.Equal(t, component.EffectiveFrom, model.EffectiveFrom)
	assert.Equal(t, component.EffectiveTo, model.EffectiveTo)
	assert.Equal(t, component.CreatedAt, model.CreatedAt)

	roundTrip := modelToSalaryComponent(model)
	assert.Equal(t, component.ID, roundTrip.ID)
	assert.Equal(t, component.TenantID, roundTrip.TenantID)
	assert.Equal(t, component.EmployeeID, roundTrip.EmployeeID)
	assert.Equal(t, component.ComponentType, roundTrip.ComponentType)
	requireDecimalEqual(t, roundTrip.Amount, component.Amount)
	assert.Equal(t, component.EffectiveTo, roundTrip.EffectiveTo)

	values := salaryComponentCreateValues(component)
	require.Len(t, values, 11)
	assert.Equal(t, component.ID, values["id"])
	assert.Equal(t, component.EmployeeID, values["employee_id"])
	assert.Equal(t, component.ComponentType, values["component_type"])
	requireDecimalEqual(t, values["amount"].(models.Decimal).Decimal, component.Amount)
	assert.Equal(t, component.EffectiveTo, values["effective_to"])
}

func TestPayrollRunModelMappings(t *testing.T) {
	now := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	paymentDate := now.AddDate(0, 0, 5)
	approvedAt := now.Add(time.Hour)
	run := &PayrollRun{
		ID:                uuid.NewString(),
		TenantID:          uuid.NewString(),
		PeriodYear:        2026,
		PeriodMonth:       3,
		Status:            PayrollApproved,
		PaymentDate:       &paymentDate,
		TotalGross:        decimal.NewFromInt(5000),
		TotalNet:          decimal.NewFromInt(3820),
		TotalEmployerCost: decimal.NewFromInt(6670),
		Notes:             "March payroll",
		CreatedBy:         uuid.NewString(),
		ApprovedBy:        uuid.NewString(),
		ApprovedAt:        &approvedAt,
		CreatedAt:         now,
		UpdatedAt:         now.Add(2 * time.Hour),
	}

	model := payrollRunToModel(run)

	assert.Equal(t, run.ID, model.ID)
	assert.Equal(t, run.TenantID, model.TenantID)
	assert.Equal(t, run.PeriodYear, model.PeriodYear)
	assert.Equal(t, run.PeriodMonth, model.PeriodMonth)
	assert.Equal(t, models.PayrollStatus(run.Status), model.Status)
	assert.Equal(t, run.PaymentDate, model.PaymentDate)
	requireDecimalEqual(t, model.TotalGross.Decimal, run.TotalGross)
	requireDecimalEqual(t, model.TotalNet.Decimal, run.TotalNet)
	requireDecimalEqual(t, model.TotalEmployerCost.Decimal, run.TotalEmployerCost)
	assert.Equal(t, run.Notes, model.Notes)
	require.NotNil(t, model.CreatedBy)
	require.NotNil(t, model.ApprovedBy)
	assert.Equal(t, run.CreatedBy, *model.CreatedBy)
	assert.Equal(t, run.ApprovedBy, *model.ApprovedBy)
	assert.Equal(t, run.ApprovedAt, model.ApprovedAt)

	roundTrip := modelToPayrollRun(model)
	assert.Equal(t, run.ID, roundTrip.ID)
	assert.Equal(t, run.Status, roundTrip.Status)
	requireDecimalEqual(t, roundTrip.TotalGross, run.TotalGross)
	requireDecimalEqual(t, roundTrip.TotalNet, run.TotalNet)
	requireDecimalEqual(t, roundTrip.TotalEmployerCost, run.TotalEmployerCost)
	assert.Equal(t, run.CreatedBy, roundTrip.CreatedBy)
	assert.Equal(t, run.ApprovedBy, roundTrip.ApprovedBy)
	assert.Equal(t, run.ApprovedAt, roundTrip.ApprovedAt)

	values := payrollRunCreateValues(run)
	require.Len(t, values, 15)
	assert.Equal(t, run.ID, values["id"])
	assert.Equal(t, models.PayrollStatus(run.Status), values["status"])
	requireDecimalEqual(t, values["total_gross"].(models.Decimal).Decimal, run.TotalGross)
	require.NotNil(t, values["created_by"])
	assert.Equal(t, run.CreatedBy, *values["created_by"].(*string))
	assert.Equal(t, run.ApprovedAt, values["approved_at"])
}

func TestPayslipModelMappings(t *testing.T) {
	now := time.Date(2026, 4, 5, 6, 7, 8, 0, time.UTC)
	paidAt := now.AddDate(0, 0, 3)
	payslip := &Payslip{
		ID:                      uuid.NewString(),
		TenantID:                uuid.NewString(),
		PayrollRunID:            uuid.NewString(),
		EmployeeID:              uuid.NewString(),
		GrossSalary:             decimal.NewFromInt(3000),
		TaxableIncome:           decimal.NewFromInt(2300),
		IncomeTax:               decimal.NewFromInt(506),
		UnemploymentInsuranceEE: decimal.NewFromInt(48),
		FundedPension:           decimal.NewFromInt(60),
		OtherDeductions:         decimal.NewFromInt(25),
		NetSalary:               decimal.NewFromInt(2361),
		SocialTax:               decimal.NewFromInt(990),
		UnemploymentInsuranceER: decimal.NewFromInt(24),
		TotalEmployerCost:       decimal.NewFromInt(4014),
		BasicExemptionApplied:   decimal.NewFromInt(700),
		PaymentStatus:           "PAID",
		PaidAt:                  &paidAt,
		CreatedAt:               now,
	}

	model := payslipToModel(payslip)

	assert.Equal(t, payslip.ID, model.ID)
	assert.Equal(t, payslip.TenantID, model.TenantID)
	assert.Equal(t, payslip.PayrollRunID, model.PayrollRunID)
	assert.Equal(t, payslip.EmployeeID, model.EmployeeID)
	requireDecimalEqual(t, model.GrossSalary.Decimal, payslip.GrossSalary)
	requireDecimalEqual(t, model.TaxableIncome.Decimal, payslip.TaxableIncome)
	requireDecimalEqual(t, model.IncomeTax.Decimal, payslip.IncomeTax)
	requireDecimalEqual(t, model.UnemploymentInsuranceEE.Decimal, payslip.UnemploymentInsuranceEE)
	requireDecimalEqual(t, model.FundedPension.Decimal, payslip.FundedPension)
	requireDecimalEqual(t, model.OtherDeductions.Decimal, payslip.OtherDeductions)
	requireDecimalEqual(t, model.NetSalary.Decimal, payslip.NetSalary)
	requireDecimalEqual(t, model.SocialTax.Decimal, payslip.SocialTax)
	requireDecimalEqual(t, model.UnemploymentInsuranceER.Decimal, payslip.UnemploymentInsuranceER)
	requireDecimalEqual(t, model.TotalEmployerCost.Decimal, payslip.TotalEmployerCost)
	requireDecimalEqual(t, model.BasicExemptionApplied.Decimal, payslip.BasicExemptionApplied)
	assert.Equal(t, payslip.PaymentStatus, model.PaymentStatus)
	assert.Equal(t, payslip.PaidAt, model.PaidAt)

	roundTrip := modelToPayslip(model)
	assert.Equal(t, payslip.ID, roundTrip.ID)
	requireDecimalEqual(t, roundTrip.GrossSalary, payslip.GrossSalary)
	requireDecimalEqual(t, roundTrip.NetSalary, payslip.NetSalary)
	requireDecimalEqual(t, roundTrip.TotalEmployerCost, payslip.TotalEmployerCost)
	assert.Equal(t, payslip.PaymentStatus, roundTrip.PaymentStatus)
	assert.Equal(t, payslip.PaidAt, roundTrip.PaidAt)

	values := payslipCreateValues(payslip)
	require.Len(t, values, 18)
	assert.Equal(t, payslip.ID, values["id"])
	assert.Equal(t, payslip.PayrollRunID, values["payroll_run_id"])
	requireDecimalEqual(t, values["gross_salary"].(models.Decimal).Decimal, payslip.GrossSalary)
	requireDecimalEqual(t, values["unemployment_insurance_employer"].(models.Decimal).Decimal, payslip.UnemploymentInsuranceER)
	assert.Equal(t, payslip.PaymentStatus, values["payment_status"])
}

func TestTSDDeclarationModelMappings(t *testing.T) {
	now := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)
	submittedAt := now.Add(time.Hour)
	declaration := &TSDDeclaration{
		ID:                  uuid.NewString(),
		TenantID:            uuid.NewString(),
		PeriodYear:          2026,
		PeriodMonth:         5,
		PayrollRunID:        uuid.NewString(),
		TotalPayments:       decimal.NewFromInt(3000),
		TotalIncomeTax:      decimal.NewFromInt(506),
		TotalSocialTax:      decimal.NewFromInt(990),
		TotalUnemploymentER: decimal.NewFromInt(24),
		TotalUnemploymentEE: decimal.NewFromInt(48),
		TotalFundedPension:  decimal.NewFromInt(60),
		Status:              TSDSubmitted,
		SubmittedAt:         &submittedAt,
		EMTAReference:       "TSD-2026-05",
		CreatedAt:           now,
		UpdatedAt:           now.Add(2 * time.Hour),
	}

	model := tsdDeclarationToModel(declaration)

	assert.Equal(t, declaration.ID, model.ID)
	assert.Equal(t, declaration.TenantID, model.TenantID)
	assert.Equal(t, declaration.PeriodYear, model.PeriodYear)
	assert.Equal(t, declaration.PeriodMonth, model.PeriodMonth)
	require.NotNil(t, model.PayrollRunID)
	assert.Equal(t, declaration.PayrollRunID, *model.PayrollRunID)
	requireDecimalEqual(t, model.TotalPayments.Decimal, declaration.TotalPayments)
	requireDecimalEqual(t, model.TotalIncomeTax.Decimal, declaration.TotalIncomeTax)
	requireDecimalEqual(t, model.TotalSocialTax.Decimal, declaration.TotalSocialTax)
	requireDecimalEqual(t, model.TotalUnemploymentER.Decimal, declaration.TotalUnemploymentER)
	requireDecimalEqual(t, model.TotalUnemploymentEE.Decimal, declaration.TotalUnemploymentEE)
	requireDecimalEqual(t, model.TotalFundedPension.Decimal, declaration.TotalFundedPension)
	assert.Equal(t, string(declaration.Status), model.Status)
	assert.Equal(t, declaration.SubmittedAt, model.SubmittedAt)
	assert.Equal(t, declaration.EMTAReference, model.EMTAReference)

	roundTrip := modelToTSDDeclaration(model)
	assert.Equal(t, declaration.ID, roundTrip.ID)
	assert.Equal(t, declaration.PayrollRunID, roundTrip.PayrollRunID)
	requireDecimalEqual(t, roundTrip.TotalPayments, declaration.TotalPayments)
	requireDecimalEqual(t, roundTrip.TotalUnemploymentEE, declaration.TotalUnemploymentEE)
	assert.Equal(t, declaration.Status, roundTrip.Status)
	assert.Equal(t, declaration.SubmittedAt, roundTrip.SubmittedAt)

	values := tsdDeclarationCreateValues(declaration)
	require.Len(t, values, 16)
	assert.Equal(t, declaration.ID, values["id"])
	assert.Equal(t, declaration.PeriodMonth, values["period_month"])
	require.NotNil(t, values["payroll_run_id"])
	assert.Equal(t, declaration.PayrollRunID, *values["payroll_run_id"].(*string))
	requireDecimalEqual(t, values["total_payments"].(models.Decimal).Decimal, declaration.TotalPayments)
	assert.Equal(t, string(declaration.Status), values["status"])
	assert.Equal(t, declaration.EMTAReference, values["emta_reference"])
}

func TestTSDRowModelMappings(t *testing.T) {
	now := time.Date(2026, 6, 7, 8, 9, 10, 0, time.UTC)
	row := &TSDRow{
		ID:             uuid.NewString(),
		TenantID:       uuid.NewString(),
		DeclarationID:  uuid.NewString(),
		EmployeeID:     uuid.NewString(),
		PersonalCode:   "49001010010",
		FirstName:      "Marta",
		LastName:       "Tamm",
		PaymentType:    PaymentTypeSalary,
		GrossPayment:   decimal.NewFromInt(3000),
		BasicExemption: decimal.NewFromInt(700),
		TaxableAmount:  decimal.NewFromInt(2300),
		IncomeTax:      decimal.NewFromInt(506),
		SocialTax:      decimal.NewFromInt(990),
		UnemploymentER: decimal.NewFromInt(24),
		UnemploymentEE: decimal.NewFromInt(48),
		FundedPension:  decimal.NewFromInt(60),
		CreatedAt:      now,
	}

	model := tsdRowToModel(row)

	assert.Equal(t, row.ID, model.ID)
	assert.Equal(t, row.TenantID, model.TenantID)
	assert.Equal(t, row.DeclarationID, model.DeclarationID)
	assert.Equal(t, row.EmployeeID, model.EmployeeID)
	assert.Equal(t, row.PersonalCode, model.PersonalCode)
	assert.Equal(t, row.FirstName, model.FirstName)
	assert.Equal(t, row.LastName, model.LastName)
	assert.Equal(t, row.PaymentType, model.PaymentType)
	requireDecimalEqual(t, model.GrossPayment.Decimal, row.GrossPayment)
	requireDecimalEqual(t, model.BasicExemption.Decimal, row.BasicExemption)
	requireDecimalEqual(t, model.TaxableAmount.Decimal, row.TaxableAmount)
	requireDecimalEqual(t, model.IncomeTax.Decimal, row.IncomeTax)
	requireDecimalEqual(t, model.SocialTax.Decimal, row.SocialTax)
	requireDecimalEqual(t, model.UnemploymentER.Decimal, row.UnemploymentER)
	requireDecimalEqual(t, model.UnemploymentEE.Decimal, row.UnemploymentEE)
	requireDecimalEqual(t, model.FundedPension.Decimal, row.FundedPension)

	roundTrip := modelToTSDRow(model)
	assert.Equal(t, row.ID, roundTrip.ID)
	assert.Equal(t, row.PaymentType, roundTrip.PaymentType)
	requireDecimalEqual(t, roundTrip.GrossPayment, row.GrossPayment)
	requireDecimalEqual(t, roundTrip.BasicExemption, row.BasicExemption)
	requireDecimalEqual(t, roundTrip.UnemploymentER, row.UnemploymentER)
	requireDecimalEqual(t, roundTrip.UnemploymentEE, row.UnemploymentEE)

	values := tsdRowCreateValues(row)
	require.Len(t, values, 17)
	assert.Equal(t, row.ID, values["id"])
	assert.Equal(t, row.DeclarationID, values["declaration_id"])
	assert.Equal(t, row.PaymentType, values["payment_type"])
	requireDecimalEqual(t, values["gross_payment"].(models.Decimal).Decimal, row.GrossPayment)
	requireDecimalEqual(t, values["funded_pension"].(models.Decimal).Decimal, row.FundedPension)
}

func TestPayrollStringPointerHelpers(t *testing.T) {
	require.Nil(t, stringPtrIfNotBlank(""))

	value := stringPtrIfNotBlank("user-id")
	require.NotNil(t, value)
	assert.Equal(t, "user-id", *value)

	assert.Equal(t, "", stringValue(nil))
	assert.Equal(t, "user-id", stringValue(value))
}
