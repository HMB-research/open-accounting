//go:build integration

package payroll

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestPostgresRepository_EmployeeOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee
	employee := &Employee{
		ID:                   uuid.New().String(),
		TenantID:             tenant.ID,
		EmployeeNumber:       "EMP-001",
		FirstName:            "John",
		LastName:             "Doe",
		PersonalCode:         "38901234567",
		Email:                "john.doe@example.com",
		Phone:                "+372 555 1234",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		Position:             "Developer",
		Department:           "Engineering",
		EmploymentType:       EmploymentFullTime,
		TaxResidency:         "EE",
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
		IsActive:             true,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	err = repo.CreateEmployee(ctx, tenant.SchemaName, employee)
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Get employee by ID
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

	// List employees (activeOnly = false)
	employees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListEmployees failed: %v", err)
	}

	if len(employees) < 1 {
		t.Errorf("expected at least 1 employee, got %d", len(employees))
	}

	// List employees (activeOnly = true)
	activeEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEmployees (active) failed: %v", err)
	}

	if len(activeEmployees) < 1 {
		t.Errorf("expected at least 1 active employee, got %d", len(activeEmployees))
	}

	// Update employee
	employee.Position = "Senior Developer"
	employee.Department = "Platform"
	err = repo.UpdateEmployee(ctx, tenant.SchemaName, employee)
	if err != nil {
		t.Fatalf("UpdateEmployee failed: %v", err)
	}

	// Verify update
	updated, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee after update failed: %v", err)
	}

	if updated.Position != "Senior Developer" {
		t.Errorf("expected position 'Senior Developer', got '%s'", updated.Position)
	}
}

func TestPostgresRepository_SalaryOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee first
	employeeID := uuid.New().String()
	employee := &Employee{
		ID:                   employeeID,
		TenantID:             tenant.ID,
		EmployeeNumber:       "EMP-SAL-001",
		FirstName:            "Jane",
		LastName:             "Smith",
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

	err = repo.CreateEmployee(ctx, tenant.SchemaName, employee)
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Create salary component
	salaryComponent := &SalaryComponent{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		EmployeeID:    employeeID,
		ComponentType: "BASE_SALARY",
		Name:          "Base Salary",
		Amount:        decimal.NewFromFloat(3000.00),
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
		CreatedAt:     time.Now(),
	}

	err = repo.CreateSalaryComponent(ctx, tenant.SchemaName, salaryComponent)
	if err != nil {
		t.Fatalf("CreateSalaryComponent failed: %v", err)
	}

	// Get current salary
	currentSalary, err := repo.GetCurrentSalary(ctx, tenant.SchemaName, tenant.ID, employeeID)
	if err != nil {
		t.Fatalf("GetCurrentSalary failed: %v", err)
	}

	if !currentSalary.Equal(decimal.NewFromFloat(3000.00)) {
		t.Errorf("expected current salary 3000.00, got %s", currentSalary)
	}

	// End current base salary (prepare for new one)
	err = repo.EndCurrentBaseSalary(ctx, tenant.SchemaName, tenant.ID, employeeID, time.Now())
	if err != nil {
		t.Fatalf("EndCurrentBaseSalary failed: %v", err)
	}
}

func TestPostgresRepository_PayrollRunOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "payroll-run-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create payroll run
	now := time.Now()

	run := &PayrollRun{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
		Status:      PayrollDraft,
		TotalGross:  decimal.Zero,
		TotalNet:    decimal.Zero,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = repo.CreatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Get payroll run
	retrieved, err := repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun failed: %v", err)
	}

	if retrieved.Status != PayrollDraft {
		t.Errorf("expected status DRAFT, got %s", retrieved.Status)
	}

	// List payroll runs (pass 0 to not filter by year)
	runs, err := repo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, 0)
	if err != nil {
		t.Fatalf("ListPayrollRuns failed: %v", err)
	}

	if len(runs) < 1 {
		t.Errorf("expected at least 1 payroll run, got %d", len(runs))
	}

	// Update payroll run
	run.Status = PayrollCalculated
	run.TotalGross = decimal.NewFromFloat(6000.00)
	run.TotalNet = decimal.NewFromFloat(4500.00)
	run.TotalEmployerCost = decimal.NewFromFloat(7980.00)

	err = repo.UpdatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("UpdatePayrollRun failed: %v", err)
	}

	// Verify update
	updated, err := repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun after update failed: %v", err)
	}

	if updated.Status != PayrollCalculated {
		t.Errorf("expected status CALCULATED, got %s", updated.Status)
	}
	if !updated.TotalGross.Equal(decimal.NewFromFloat(6000.00)) {
		t.Errorf("expected total gross 6000.00, got %s", updated.TotalGross)
	}

	// Approve payroll run
	err = repo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	// Verify approval
	approved, err := repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun after approval failed: %v", err)
	}

	if approved.Status != PayrollApproved {
		t.Errorf("expected status APPROVED, got %s", approved.Status)
	}
	if approved.ApprovedBy != userID {
		t.Errorf("expected approved_by %s, got %s", userID, approved.ApprovedBy)
	}
}

func TestPostgresRepository_PayslipOperations(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "payslip-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee first
	employeeID := uuid.New().String()
	employee := &Employee{
		ID:                   employeeID,
		TenantID:             tenant.ID,
		EmployeeNumber:       "EMP-SLIP-001",
		FirstName:            "Bob",
		LastName:             "Wilson",
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

	err = repo.CreateEmployee(ctx, tenant.SchemaName, employee)
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Create payroll run
	now := time.Now()
	runID := uuid.New().String()

	run := &PayrollRun{
		ID:          runID,
		TenantID:    tenant.ID,
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
		Status:      PayrollCalculated,
		TotalGross:  decimal.Zero,
		TotalNet:    decimal.Zero,
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = repo.CreatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Create payslip
	payslip := &Payslip{
		ID:                      uuid.New().String(),
		TenantID:                tenant.ID,
		PayrollRunID:            runID,
		EmployeeID:              employeeID,
		GrossSalary:             decimal.NewFromFloat(3000.00),
		TaxableIncome:           decimal.NewFromFloat(2500.00),
		IncomeTax:               decimal.NewFromFloat(506.00),
		UnemploymentInsuranceEE: decimal.NewFromFloat(48.00),
		FundedPension:           decimal.NewFromFloat(60.00),
		OtherDeductions:         decimal.Zero,
		NetSalary:               decimal.NewFromFloat(2386.00),
		SocialTax:               decimal.NewFromFloat(990.00),
		UnemploymentInsuranceER: decimal.NewFromFloat(24.00),
		TotalEmployerCost:       decimal.NewFromFloat(4014.00),
		BasicExemptionApplied:   DefaultBasicExemption,
		PaymentStatus:           "PENDING",
		CreatedAt:               now,
	}

	err = repo.CreatePayslip(ctx, tenant.SchemaName, payslip)
	if err != nil {
		t.Fatalf("CreatePayslip failed: %v", err)
	}

	// Delete payslips by run ID (cleanup before run deletion)
	err = repo.DeletePayslipsByRunID(ctx, tenant.SchemaName, runID)
	if err != nil {
		t.Fatalf("DeletePayslipsByRunID failed: %v", err)
	}
}

func TestPostgresRepository_GetEmployee_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Try to get non-existent employee
	_, err = repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrEmployeeNotFound {
		t.Errorf("expected ErrEmployeeNotFound, got %v", err)
	}
}

func TestPostgresRepository_GetPayrollRun_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Try to get non-existent payroll run
	_, err = repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound, got %v", err)
	}
}

func TestPostgresRepository_ApprovePayrollRun_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "approve-notfound-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Try to approve non-existent payroll run
	err = repo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), userID)
	if err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound, got %v", err)
	}
}

func TestPostgresRepository_ListPayrollRuns_WithYear(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "list-runs-year-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	now := time.Now()
	currentYear := now.Year()
	lastYear := currentYear - 1

	// Create payroll runs for different years
	for _, year := range []int{currentYear, lastYear} {
		run := &PayrollRun{
			ID:          uuid.New().String(),
			TenantID:    tenant.ID,
			PeriodYear:  year,
			PeriodMonth: 1,
			Status:      PayrollDraft,
			TotalGross:  decimal.Zero,
			TotalNet:    decimal.Zero,
			CreatedBy:   userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		err = repo.CreatePayrollRun(ctx, tenant.SchemaName, run)
		if err != nil {
			t.Fatalf("CreatePayrollRun for year %d failed: %v", year, err)
		}
	}

	// List only current year
	runs, err := repo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, currentYear)
	if err != nil {
		t.Fatalf("ListPayrollRuns failed: %v", err)
	}

	for _, r := range runs {
		if r.PeriodYear != currentYear {
			t.Errorf("expected year %d, got %d", currentYear, r.PeriodYear)
		}
	}
}

func TestPostgresRepository_ListEmployees_InactiveOnly(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create an inactive employee
	inactiveEmployee := &Employee{
		ID:                   uuid.New().String(),
		TenantID:             tenant.ID,
		EmployeeNumber:       "EMP-INACTIVE-001",
		FirstName:            "Inactive",
		LastName:             "Employee",
		StartDate:            time.Now().AddDate(-2, 0, 0),
		EndDate:              timePtr(time.Now().AddDate(-1, 0, 0)),
		EmploymentType:       EmploymentFullTime,
		TaxResidency:         "EE",
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
		IsActive:             false,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	err = repo.CreateEmployee(ctx, tenant.SchemaName, inactiveEmployee)
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// List all employees (including inactive)
	allEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListEmployees (all) failed: %v", err)
	}

	// List only active employees
	activeEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEmployees (active only) failed: %v", err)
	}

	// All employees should include more than or equal to active
	if len(allEmployees) < len(activeEmployees) {
		t.Error("expected all employees to include at least as many as active employees")
	}

	// Verify inactive employee is in all but not in active
	foundInAll := false
	foundInActive := false
	for _, e := range allEmployees {
		if e.ID == inactiveEmployee.ID {
			foundInAll = true
			break
		}
	}
	for _, e := range activeEmployees {
		if e.ID == inactiveEmployee.ID {
			foundInActive = true
			break
		}
	}

	if !foundInAll {
		t.Error("expected inactive employee in all employees list")
	}
	if foundInActive {
		t.Error("expected inactive employee NOT in active employees list")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
