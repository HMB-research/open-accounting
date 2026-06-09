//go:build integration

package payroll

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

func newPayrollGORMRepository(t *testing.T, pool *pgxpool.Pool) *GORMRepository {
	t.Helper()

	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		t.Fatalf("failed to create GORM DB: %v", err)
	}
	return NewGORMRepository(gormDB)
}

func TestGORMRepository_EmployeeOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPayrollGORMRepository(t, pool)
	ctx := context.Background()

	employee := testEmployee(tenant.ID, "EMP-001")
	employee.FirstName = "John"
	employee.LastName = "Doe"
	employee.Email = "john.doe@example.com"
	employee.Position = "Developer"
	employee.Department = "Engineering"

	if err := repo.CreateEmployee(ctx, tenant.SchemaName, employee); err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	retrieved, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee failed: %v", err)
	}
	if retrieved.FirstName != employee.FirstName {
		t.Errorf("expected first name %s, got %s", employee.FirstName, retrieved.FirstName)
	}
	if retrieved.Email != employee.Email {
		t.Errorf("expected email %s, got %s", employee.Email, retrieved.Email)
	}

	employees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListEmployees failed: %v", err)
	}
	if len(employees) != 1 {
		t.Fatalf("expected 1 employee, got %d", len(employees))
	}

	employee.Position = "Senior Developer"
	employee.Department = "Platform"
	employee.UpdatedAt = time.Now()
	if err := repo.UpdateEmployee(ctx, tenant.SchemaName, employee); err != nil {
		t.Fatalf("UpdateEmployee failed: %v", err)
	}

	updated, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee after update failed: %v", err)
	}
	if updated.Position != "Senior Developer" {
		t.Errorf("expected position Senior Developer, got %s", updated.Position)
	}
}

func TestGORMRepository_SalaryOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := newPayrollGORMRepository(t, pool)
	ctx := context.Background()

	employee := testEmployee(tenant.ID, "EMP-SAL-001")
	if err := repo.CreateEmployee(ctx, tenant.SchemaName, employee); err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	component := testSalaryComponent(tenant.ID, employee.ID, decimal.NewFromInt(3000))
	if err := repo.CreateSalaryComponent(ctx, tenant.SchemaName, component); err != nil {
		t.Fatalf("CreateSalaryComponent failed: %v", err)
	}

	currentSalary, err := repo.GetCurrentSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetCurrentSalary failed: %v", err)
	}
	if !currentSalary.Equal(decimal.NewFromInt(3000)) {
		t.Errorf("expected current salary 3000, got %s", currentSalary)
	}

	activeOn := time.Now()
	components, err := repo.ListSalaryComponents(ctx, tenant.SchemaName, tenant.ID, employee.ID, &activeOn)
	if err != nil {
		t.Fatalf("ListSalaryComponents failed: %v", err)
	}
	if len(components) != 1 {
		t.Fatalf("expected 1 salary component, got %d", len(components))
	}

	if err := repo.EndCurrentBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID, time.Now().AddDate(0, 0, -1)); err != nil {
		t.Fatalf("EndCurrentBaseSalary failed: %v", err)
	}
	currentSalary, err = repo.GetCurrentSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetCurrentSalary after ending salary failed: %v", err)
	}
	if !currentSalary.IsZero() {
		t.Errorf("expected current salary 0 after ending salary, got %s", currentSalary)
	}
}

func TestGORMRepository_PayrollRunOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "payroll-run-gorm-test@example.com")
	repo := newPayrollGORMRepository(t, pool)
	ctx := context.Background()

	now := time.Now()
	run := testPayrollRun(tenant.ID, userID, now)
	if err := repo.CreatePayrollRun(ctx, tenant.SchemaName, run); err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	retrieved, err := repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun failed: %v", err)
	}
	if retrieved.Status != PayrollDraft {
		t.Errorf("expected status DRAFT, got %s", retrieved.Status)
	}

	run.Status = PayrollCalculated
	run.TotalGross = decimal.NewFromInt(6000)
	run.TotalNet = decimal.NewFromInt(4500)
	run.TotalEmployerCost = decimal.NewFromInt(7980)
	if err := repo.UpdatePayrollRun(ctx, tenant.SchemaName, run); err != nil {
		t.Fatalf("UpdatePayrollRun failed: %v", err)
	}

	if err := repo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID); err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	runs, err := repo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, now.Year())
	if err != nil {
		t.Fatalf("ListPayrollRuns failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 payroll run, got %d", len(runs))
	}
	if runs[0].Status != PayrollApproved {
		t.Errorf("expected status APPROVED, got %s", runs[0].Status)
	}
	if runs[0].ApprovedBy != userID {
		t.Errorf("expected approved_by %s, got %s", userID, runs[0].ApprovedBy)
	}
}

func TestGORMRepository_PayslipOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "payslip-gorm-test@example.com")
	repo := newPayrollGORMRepository(t, pool)
	ctx := context.Background()

	employee := testEmployee(tenant.ID, "EMP-SLIP-001")
	if err := repo.CreateEmployee(ctx, tenant.SchemaName, employee); err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	now := time.Now()
	run := testPayrollRun(tenant.ID, userID, now)
	run.Status = PayrollCalculated
	if err := repo.CreatePayrollRun(ctx, tenant.SchemaName, run); err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	payslip := testPayslip(tenant.ID, run.ID, employee.ID, now)
	if err := repo.CreatePayslip(ctx, tenant.SchemaName, payslip); err != nil {
		t.Fatalf("CreatePayslip failed: %v", err)
	}

	payslips, err := repo.GetPayslipsWithEmployees(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayslipsWithEmployees failed: %v", err)
	}
	if len(payslips) != 1 {
		t.Fatalf("expected 1 payslip, got %d", len(payslips))
	}
	if payslips[0].Employee == nil || payslips[0].Employee.LastName != employee.LastName {
		t.Fatalf("expected joined employee data, got %+v", payslips[0].Employee)
	}

	if err := repo.DeletePayslipsByRunID(ctx, tenant.SchemaName, run.ID); err != nil {
		t.Fatalf("DeletePayslipsByRunID failed: %v", err)
	}
	payslips, err = repo.GetPayslipsWithEmployees(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayslipsWithEmployees after delete failed: %v", err)
	}
	if len(payslips) != 0 {
		t.Errorf("expected 0 payslips after delete, got %d", len(payslips))
	}
}

func TestGORMRepository_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "not-found-gorm-test@example.com")
	repo := newPayrollGORMRepository(t, pool)
	ctx := context.Background()

	if _, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, uuid.New().String()); err != ErrEmployeeNotFound {
		t.Errorf("expected ErrEmployeeNotFound, got %v", err)
	}
	if _, err := repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, uuid.New().String()); err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound, got %v", err)
	}
	if err := repo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), userID); err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound, got %v", err)
	}

	run := testPayrollRun(tenant.ID, userID, time.Now())
	run.Status = PayrollDraft
	if err := repo.CreatePayrollRun(ctx, tenant.SchemaName, run); err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}
	if err := repo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID); err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound for wrong status, got %v", err)
	}
}

func TestGORMRepository_ListFiltersAndEmptyLists(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "list-filter-gorm-test@example.com")
	repo := newPayrollGORMRepository(t, pool)
	ctx := context.Background()

	inactiveEmployee := testEmployee(tenant.ID, "EMP-INACTIVE-001")
	inactiveEmployee.IsActive = false
	if err := repo.CreateEmployee(ctx, tenant.SchemaName, inactiveEmployee); err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	allEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListEmployees failed: %v", err)
	}
	activeEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEmployees active failed: %v", err)
	}
	if len(allEmployees) != 1 {
		t.Fatalf("expected 1 employee, got %d", len(allEmployees))
	}
	if len(activeEmployees) != 0 {
		t.Fatalf("expected 0 active employees, got %d", len(activeEmployees))
	}

	now := time.Now()
	for _, year := range []int{now.Year(), now.Year() - 1} {
		run := testPayrollRun(tenant.ID, userID, now)
		run.PeriodYear = year
		if err := repo.CreatePayrollRun(ctx, tenant.SchemaName, run); err != nil {
			t.Fatalf("CreatePayrollRun for year %d failed: %v", year, err)
		}
	}

	runs, err := repo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, now.Year())
	if err != nil {
		t.Fatalf("ListPayrollRuns failed: %v", err)
	}
	if len(runs) != 1 || runs[0].PeriodYear != now.Year() {
		t.Fatalf("expected one current-year run, got %+v", runs)
	}

	runs, err = repo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, 1999)
	if err != nil {
		t.Fatalf("ListPayrollRuns empty failed: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("expected 0 payroll runs, got %d", len(runs))
	}
}

func TestGORMRepository_WithTransaction_CommitsAndRollsBack(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "payroll-gorm-transaction@example.com")
	repo := newPayrollGORMRepository(t, pool)
	ctx := context.Background()

	employee := testEmployee(tenant.ID, "EMP-GORM-TX-COMMIT")
	run := testPayrollRun(tenant.ID, userID, time.Now())
	component := testSalaryComponent(tenant.ID, employee.ID, decimal.NewFromInt(5000))
	payslip := testPayslip(tenant.ID, run.ID, employee.ID, time.Now())

	if err := repo.WithTransaction(ctx, func(txRepo Repository) error {
		if err := txRepo.CreateEmployee(ctx, tenant.SchemaName, employee); err != nil {
			return err
		}
		if err := txRepo.CreatePayrollRun(ctx, tenant.SchemaName, run); err != nil {
			return err
		}
		if err := txRepo.CreateSalaryComponent(ctx, tenant.SchemaName, component); err != nil {
			return err
		}
		if err := txRepo.CreatePayslip(ctx, tenant.SchemaName, payslip); err != nil {
			return err
		}
		return txRepo.DeletePayslipsByRunID(ctx, tenant.SchemaName, run.ID)
	}); err != nil {
		t.Fatalf("WithTransaction commit failed: %v", err)
	}

	if _, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID); err != nil {
		t.Fatalf("expected committed employee: %v", err)
	}
	currentSalary, err := repo.GetCurrentSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetCurrentSalary after transaction failed: %v", err)
	}
	if !currentSalary.Equal(decimal.NewFromInt(5000)) {
		t.Errorf("expected current salary 5000, got %s", currentSalary)
	}
	payslips, err := repo.GetPayslipsWithEmployees(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayslipsWithEmployees after transaction failed: %v", err)
	}
	if len(payslips) != 0 {
		t.Errorf("expected deleted payslips, got %d", len(payslips))
	}

	rolledBackEmployee := testEmployee(tenant.ID, "EMP-GORM-TX-ROLLBACK")
	expectedErr := fmt.Errorf("force rollback")
	err = repo.WithTransaction(ctx, func(txRepo Repository) error {
		if err := txRepo.CreateEmployee(ctx, tenant.SchemaName, rolledBackEmployee); err != nil {
			return err
		}
		return expectedErr
	})
	if err != expectedErr {
		t.Fatalf("expected rollback error %v, got %v", expectedErr, err)
	}
	if _, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, rolledBackEmployee.ID); err != ErrEmployeeNotFound {
		t.Fatalf("expected employee insert to roll back, got %v", err)
	}
}

func testEmployee(tenantID, employeeNumber string) *Employee {
	return &Employee{
		ID:                   uuid.New().String(),
		TenantID:             tenantID,
		EmployeeNumber:       employeeNumber,
		FirstName:            "Test",
		LastName:             employeeNumber,
		PersonalCode:         "38901234567",
		Email:                employeeNumber + "@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		EmploymentType:       EmploymentFullTime,
		TaxResidency:         "EE",
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
		IsActive:             true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
}

func testSalaryComponent(tenantID, employeeID string, amount decimal.Decimal) *SalaryComponent {
	return &SalaryComponent{
		ID:            uuid.New().String(),
		TenantID:      tenantID,
		EmployeeID:    employeeID,
		ComponentType: SalaryComponentBaseSalary,
		Name:          "Base Salary",
		Amount:        amount,
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
		CreatedAt:     time.Now(),
	}
}

func testPayrollRun(tenantID, userID string, now time.Time) *PayrollRun {
	return &PayrollRun{
		ID:          uuid.New().String(),
		TenantID:    tenantID,
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
		Status:      PayrollDraft,
		TotalGross:  decimal.Zero,
		TotalNet:    decimal.Zero,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func testPayslip(tenantID, runID, employeeID string, now time.Time) *Payslip {
	return &Payslip{
		ID:                      uuid.New().String(),
		TenantID:                tenantID,
		PayrollRunID:            runID,
		EmployeeID:              employeeID,
		GrossSalary:             decimal.NewFromInt(3000),
		TaxableIncome:           decimal.NewFromInt(2500),
		IncomeTax:               decimal.NewFromInt(506),
		UnemploymentInsuranceEE: decimal.NewFromInt(48),
		FundedPension:           decimal.NewFromInt(60),
		OtherDeductions:         decimal.Zero,
		NetSalary:               decimal.NewFromInt(2386),
		SocialTax:               decimal.NewFromInt(990),
		UnemploymentInsuranceER: decimal.NewFromInt(24),
		TotalEmployerCost:       decimal.NewFromInt(4014),
		BasicExemptionApplied:   DefaultBasicExemption,
		PaymentStatus:           "PENDING",
		CreatedAt:               now,
	}
}
