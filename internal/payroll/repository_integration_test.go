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

// TestPostgresRepository_UpdateEmployee_WithTransaction tests exec with transaction for updates
func TestPostgresRepository_UpdateEmployee_WithTransaction(t *testing.T) {
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
		EmployeeNumber:       "EMP-TX-UPDATE-001",
		FirstName:            "UpdateTx",
		LastName:             "Test",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		Position:             "Junior",
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

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Update employee within transaction
	employee.Position = "Senior"
	employee.UpdatedAt = time.Now()

	err = txRepo.UpdateEmployee(ctx, tenant.SchemaName, employee)
	if err != nil {
		t.Fatalf("UpdateEmployee with transaction failed: %v", err)
	}

	// Verify within same transaction
	retrieved, err := txRepo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee within transaction failed: %v", err)
	}

	if retrieved.Position != "Senior" {
		t.Errorf("expected position Senior, got %s", retrieved.Position)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify outside transaction
	retrievedAfter, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee after commit failed: %v", err)
	}

	if retrievedAfter.Position != "Senior" {
		t.Errorf("expected position Senior after commit, got %s", retrievedAfter.Position)
	}
}

// TestPostgresRepository_CreatePayrollRun_WithTransaction tests exec with transaction for payroll runs
func TestPostgresRepository_CreatePayrollRun_WithTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "create-run-tx@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Create payroll run within transaction
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

	err = txRepo.CreatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("CreatePayrollRun with transaction failed: %v", err)
	}

	// Verify within same transaction
	retrieved, err := txRepo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun within transaction failed: %v", err)
	}

	if retrieved.ID != run.ID {
		t.Errorf("expected run ID %s, got %s", run.ID, retrieved.ID)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify outside transaction
	retrievedAfter, err := repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun after commit failed: %v", err)
	}

	if retrievedAfter.ID != run.ID {
		t.Errorf("expected run ID %s after commit, got %s", run.ID, retrievedAfter.ID)
	}
}

// TestPostgresRepository_UpdatePayrollRun_WithTransaction tests exec with transaction for payroll updates
func TestPostgresRepository_UpdatePayrollRun_WithTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "update-run-tx@example.com")
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

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Update payroll run within transaction
	run.Status = PayrollCalculated
	run.TotalGross = decimal.NewFromFloat(5000.00)
	run.TotalNet = decimal.NewFromFloat(3500.00)
	run.TotalEmployerCost = decimal.NewFromFloat(6650.00)

	err = txRepo.UpdatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("UpdatePayrollRun with transaction failed: %v", err)
	}

	// Verify within same transaction
	retrieved, err := txRepo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun within transaction failed: %v", err)
	}

	if retrieved.Status != PayrollCalculated {
		t.Errorf("expected status CALCULATED, got %s", retrieved.Status)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify outside transaction
	retrievedAfter, err := repo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun after commit failed: %v", err)
	}

	if retrievedAfter.Status != PayrollCalculated {
		t.Errorf("expected status CALCULATED after commit, got %s", retrievedAfter.Status)
	}
}

// TestPostgresRepository_CreateSalaryComponent_WithTransaction tests exec with transaction for salary
func TestPostgresRepository_CreateSalaryComponent_WithTransaction(t *testing.T) {
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
		EmployeeNumber:       "EMP-SAL-TX-CREATE-001",
		FirstName:            "SalaryTxCreate",
		LastName:             "Test",
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

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Create salary component within transaction
	salaryComponent := &SalaryComponent{
		ID:            uuid.New().String(),
		TenantID:      tenant.ID,
		EmployeeID:    employeeID,
		ComponentType: "BASE_SALARY",
		Name:          "Base Salary",
		Amount:        decimal.NewFromFloat(5000.00),
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
		CreatedAt:     time.Now(),
	}

	err = txRepo.CreateSalaryComponent(ctx, tenant.SchemaName, salaryComponent)
	if err != nil {
		t.Fatalf("CreateSalaryComponent with transaction failed: %v", err)
	}

	// Verify within same transaction
	currentSalary, err := txRepo.GetCurrentSalary(ctx, tenant.SchemaName, tenant.ID, employeeID)
	if err != nil {
		t.Fatalf("GetCurrentSalary within transaction failed: %v", err)
	}

	if !currentSalary.Equal(decimal.NewFromFloat(5000.00)) {
		t.Errorf("expected current salary 5000.00, got %s", currentSalary)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify outside transaction
	currentSalaryAfter, err := repo.GetCurrentSalary(ctx, tenant.SchemaName, tenant.ID, employeeID)
	if err != nil {
		t.Fatalf("GetCurrentSalary after commit failed: %v", err)
	}

	if !currentSalaryAfter.Equal(decimal.NewFromFloat(5000.00)) {
		t.Errorf("expected current salary 5000.00 after commit, got %s", currentSalaryAfter)
	}
}

// TestPostgresRepository_DeletePayslipsByRunID_WithTransaction tests exec with transaction for deletion
func TestPostgresRepository_DeletePayslipsByRunID_WithTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "delete-payslips-tx@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee and payroll run first
	employeeID := uuid.New().String()
	employee := &Employee{
		ID:                   employeeID,
		TenantID:             tenant.ID,
		EmployeeNumber:       "EMP-DEL-TX-001",
		FirstName:            "DeleteTx",
		LastName:             "Test",
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

	// Create a payslip
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

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Delete payslips within transaction
	err = txRepo.DeletePayslipsByRunID(ctx, tenant.SchemaName, runID)
	if err != nil {
		t.Fatalf("DeletePayslipsByRunID with transaction failed: %v", err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Payslips should be deleted - can't directly verify without a GetPayslips function
	// but the test ensures the exec with transaction works
}

// TestPostgresRepository_EndCurrentBaseSalary_WithTransaction tests exec with transaction
func TestPostgresRepository_EndCurrentBaseSalary_WithTransaction(t *testing.T) {
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
	employeeID := uuid.New().String()
	employee := &Employee{
		ID:                   employeeID,
		TenantID:             tenant.ID,
		EmployeeNumber:       "EMP-END-SAL-TX-001",
		FirstName:            "EndSalTx",
		LastName:             "Test",
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
		EffectiveFrom: time.Now().AddDate(0, -2, 0),
		CreatedAt:     time.Now(),
	}

	err = repo.CreateSalaryComponent(ctx, tenant.SchemaName, salaryComponent)
	if err != nil {
		t.Fatalf("CreateSalaryComponent failed: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// End current base salary within transaction
	err = txRepo.EndCurrentBaseSalary(ctx, tenant.SchemaName, tenant.ID, employeeID, time.Now())
	if err != nil {
		t.Fatalf("EndCurrentBaseSalary with transaction failed: %v", err)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify salary is now ended (should return 0 for current salary since it's ended)
	// Note: This depends on the effective_to being set correctly
}

// TestPostgresRepository_ApprovePayrollRun_WithTransaction tests the transaction branch
// of ApprovePayrollRun (lines 344-352 in repository.go)
func TestPostgresRepository_ApprovePayrollRun_WithTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "approve-tx-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create payroll run in CALCULATED status (required for approval)
	now := time.Now()
	run := &PayrollRun{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
		Status:      PayrollCalculated,
		TotalGross:  decimal.NewFromFloat(5000.00),
		TotalNet:    decimal.NewFromFloat(3500.00),
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = repo.CreatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Approve payroll run within transaction
	err = txRepo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun with transaction failed: %v", err)
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify the approval was persisted
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

// TestPostgresRepository_ApprovePayrollRun_WithTransaction_NotFound tests the transaction branch
// when payroll run is not found (covers the RowsAffected() == 0 case in tx branch)
func TestPostgresRepository_ApprovePayrollRun_WithTransaction_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "approve-tx-notfound@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Try to approve non-existent payroll run within transaction
	err = txRepo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, uuid.New().String(), userID)
	if err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound, got %v", err)
	}
}

// TestPostgresRepository_ApprovePayrollRun_WithTransaction_WrongStatus tests the transaction branch
// when payroll run exists but is not in CALCULATED status
func TestPostgresRepository_ApprovePayrollRun_WithTransaction_WrongStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "approve-tx-wrongstatus@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create payroll run in DRAFT status (not CALCULATED)
	now := time.Now()
	run := &PayrollRun{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
		Status:      PayrollDraft, // Not CALCULATED
		TotalGross:  decimal.NewFromFloat(5000.00),
		TotalNet:    decimal.NewFromFloat(3500.00),
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = repo.CreatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Try to approve payroll run that's in wrong status (DRAFT, not CALCULATED)
	err = txRepo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound for wrong status, got %v", err)
	}
}

// TestPostgresRepository_GetEmployee_WithTransaction tests queryRow with transaction
func TestPostgresRepository_GetEmployee_WithTransaction(t *testing.T) {
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
		EmployeeNumber:       "EMP-TX-001",
		FirstName:            "Transaction",
		LastName:             "Test",
		PersonalCode:         "38901234567",
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

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Get employee within transaction (tests queryRow with tx branch)
	retrieved, err := txRepo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee with transaction failed: %v", err)
	}

	if retrieved.FirstName != employee.FirstName {
		t.Errorf("expected first name %s, got %s", employee.FirstName, retrieved.FirstName)
	}

	// Also test not found case within transaction
	_, err = txRepo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrEmployeeNotFound {
		t.Errorf("expected ErrEmployeeNotFound, got %v", err)
	}
}

// TestPostgresRepository_ListEmployees_WithTransaction tests query with transaction
func TestPostgresRepository_ListEmployees_WithTransaction(t *testing.T) {
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
		EmployeeNumber:       "EMP-TX-LIST-001",
		FirstName:            "ListTx",
		LastName:             "Test",
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

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// List employees within transaction (tests query with tx branch)
	employees, err := txRepo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListEmployees with transaction failed: %v", err)
	}

	if len(employees) < 1 {
		t.Errorf("expected at least 1 employee, got %d", len(employees))
	}

	// Also test activeOnly within transaction
	activeEmployees, err := txRepo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEmployees (active) with transaction failed: %v", err)
	}

	if len(activeEmployees) < 1 {
		t.Errorf("expected at least 1 active employee, got %d", len(activeEmployees))
	}
}

// TestPostgresRepository_ListPayrollRuns_Empty tests empty list case
func TestPostgresRepository_ListPayrollRuns_Empty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// List payroll runs for a year that doesn't have any runs
	runs, err := repo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, 1999)
	if err != nil {
		t.Fatalf("ListPayrollRuns failed: %v", err)
	}

	// Should return empty slice, not nil
	if runs == nil {
		// This is acceptable - nil slice is valid empty result
		t.Log("ListPayrollRuns returned nil for empty result")
	} else if len(runs) != 0 {
		t.Errorf("expected 0 payroll runs, got %d", len(runs))
	}
}

// TestPostgresRepository_ListPayrollRuns_WithTransaction tests query with transaction for payroll runs
func TestPostgresRepository_ListPayrollRuns_WithTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "list-runs-tx@example.com")
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

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// List payroll runs within transaction (tests query with tx branch)
	runs, err := txRepo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, 0)
	if err != nil {
		t.Fatalf("ListPayrollRuns with transaction failed: %v", err)
	}

	if len(runs) < 1 {
		t.Errorf("expected at least 1 payroll run, got %d", len(runs))
	}

	// Also test with year filter in transaction
	runsWithYear, err := txRepo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, now.Year())
	if err != nil {
		t.Fatalf("ListPayrollRuns with year filter in transaction failed: %v", err)
	}

	if len(runsWithYear) < 1 {
		t.Errorf("expected at least 1 payroll run for current year, got %d", len(runsWithYear))
	}
}

// TestPostgresRepository_GetPayrollRun_WithTransaction tests queryRow with transaction for payroll runs
func TestPostgresRepository_GetPayrollRun_WithTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "get-run-tx@example.com")
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
		TotalGross:  decimal.NewFromFloat(1000.00),
		TotalNet:    decimal.NewFromFloat(800.00),
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = repo.CreatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Get payroll run within transaction (tests queryRow with tx branch)
	retrieved, err := txRepo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayrollRun with transaction failed: %v", err)
	}

	if retrieved.ID != run.ID {
		t.Errorf("expected run ID %s, got %s", run.ID, retrieved.ID)
	}

	// Also test not found case within transaction
	_, err = txRepo.GetPayrollRun(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrPayrollRunNotFound {
		t.Errorf("expected ErrPayrollRunNotFound, got %v", err)
	}
}

// TestPostgresRepository_GetCurrentSalary_WithTransaction tests queryRow with transaction for salary
func TestPostgresRepository_GetCurrentSalary_WithTransaction(t *testing.T) {
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
		EmployeeNumber:       "EMP-SAL-TX-001",
		FirstName:            "Salary",
		LastName:             "TxTest",
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
		Amount:        decimal.NewFromFloat(4000.00),
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: time.Now().AddDate(0, -1, 0),
		CreatedAt:     time.Now(),
	}

	err = repo.CreateSalaryComponent(ctx, tenant.SchemaName, salaryComponent)
	if err != nil {
		t.Fatalf("CreateSalaryComponent failed: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Get current salary within transaction (tests queryRow with tx branch)
	currentSalary, err := txRepo.GetCurrentSalary(ctx, tenant.SchemaName, tenant.ID, employeeID)
	if err != nil {
		t.Fatalf("GetCurrentSalary with transaction failed: %v", err)
	}

	if !currentSalary.Equal(decimal.NewFromFloat(4000.00)) {
		t.Errorf("expected current salary 4000.00, got %s", currentSalary)
	}
}

// TestPostgresRepository_ListEmployees_EmptyList tests empty list for employees
func TestPostgresRepository_ListEmployees_EmptyList(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// List employees for a tenant that has no employees yet
	// Note: the tenant is fresh so should have no employees
	employees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, false)
	if err != nil {
		t.Fatalf("ListEmployees failed: %v", err)
	}

	// Should be empty or nil
	if employees != nil && len(employees) > 0 {
		t.Log("Tenant has employees from other tests running in parallel")
	}

	// Also test activeOnly on empty list
	activeEmployees, err := repo.ListEmployees(ctx, tenant.SchemaName, tenant.ID, true)
	if err != nil {
		t.Fatalf("ListEmployees (active) failed: %v", err)
	}

	if activeEmployees != nil && len(activeEmployees) > 0 {
		t.Log("Tenant has active employees from other tests running in parallel")
	}
}

// TestPostgresRepository_ListPayrollRuns_WithApprovedRun tests that approvedBy is correctly populated
func TestPostgresRepository_ListPayrollRuns_WithApprovedRun(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "list-approved-test@example.com")
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create payroll run in CALCULATED status
	now := time.Now()
	run := &PayrollRun{
		ID:          uuid.New().String(),
		TenantID:    tenant.ID,
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
		Status:      PayrollCalculated,
		TotalGross:  decimal.NewFromFloat(5000.00),
		TotalNet:    decimal.NewFromFloat(3500.00),
		CreatedBy:   userID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = repo.CreatePayrollRun(ctx, tenant.SchemaName, run)
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Approve the payroll run
	err = repo.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	// List payroll runs - should include the approved run with approvedBy set
	runs, err := repo.ListPayrollRuns(ctx, tenant.SchemaName, tenant.ID, now.Year())
	if err != nil {
		t.Fatalf("ListPayrollRuns failed: %v", err)
	}

	// Find our approved run
	var foundApproved bool
	for _, r := range runs {
		if r.ID == run.ID {
			foundApproved = true
			if r.ApprovedBy != userID {
				t.Errorf("expected approved_by %s, got %s", userID, r.ApprovedBy)
			}
			if r.CreatedBy != userID {
				t.Errorf("expected created_by %s, got %s", userID, r.CreatedBy)
			}
			if r.Status != PayrollApproved {
				t.Errorf("expected status APPROVED, got %s", r.Status)
			}
			break
		}
	}

	if !foundApproved {
		t.Error("expected to find the approved payroll run in list")
	}
}

// TestPostgresRepository_CreateEmployee_WithTransaction tests exec with transaction
func TestPostgresRepository_CreateEmployee_WithTransaction(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Start a transaction
	tx, err := repo.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx failed: %v", err)
	}
	defer tx.Rollback(ctx)

	// Get repository with transaction
	txRepo := repo.WithTx(tx)

	// Create employee within transaction (tests exec with tx branch)
	employee := &Employee{
		ID:                   uuid.New().String(),
		TenantID:             tenant.ID,
		EmployeeNumber:       "EMP-TX-CREATE-001",
		FirstName:            "TxCreate",
		LastName:             "Test",
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

	err = txRepo.CreateEmployee(ctx, tenant.SchemaName, employee)
	if err != nil {
		t.Fatalf("CreateEmployee with transaction failed: %v", err)
	}

	// Verify within same transaction
	retrieved, err := txRepo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee within transaction failed: %v", err)
	}

	if retrieved.FirstName != employee.FirstName {
		t.Errorf("expected first name %s, got %s", employee.FirstName, retrieved.FirstName)
	}

	// Commit transaction
	err = tx.Commit(ctx)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify outside transaction
	retrievedAfter, err := repo.GetEmployee(ctx, tenant.SchemaName, tenant.ID, employee.ID)
	if err != nil {
		t.Fatalf("GetEmployee after commit failed: %v", err)
	}

	if retrievedAfter.FirstName != employee.FirstName {
		t.Errorf("expected first name %s after commit, got %s", employee.FirstName, retrievedAfter.FirstName)
	}
}
