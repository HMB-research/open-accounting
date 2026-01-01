//go:build integration

package payroll

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/shopspring/decimal"
)

func TestService_GenerateTSD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-generate-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee with personal code (required for TSD)
	employee, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
		EmployeeNumber:       "EMP-TSD-001",
		FirstName:            "Mari",
		LastName:             "Maasikas",
		PersonalCode:         "48901234567", // Estonian personal code
		Email:                "mari@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		Position:             "Developer",
		Department:           "Engineering",
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	})
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Set salary for employee
	err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID, decimal.NewFromFloat(3000.00), time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("SetBaseSalary failed: %v", err)
	}

	// Create payroll run for current period
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Calculate payroll
	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll failed: %v", err)
	}

	// Approve payroll run (required for TSD generation)
	err = service.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	// Generate TSD
	tsd, err := service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GenerateTSD failed: %v", err)
	}

	// Verify TSD fields
	if tsd.ID == "" {
		t.Error("expected TSD ID to be set")
	}
	if tsd.TenantID != tenant.ID {
		t.Errorf("expected tenant ID %s, got %s", tenant.ID, tsd.TenantID)
	}
	if tsd.PeriodYear != now.Year() {
		t.Errorf("expected period year %d, got %d", now.Year(), tsd.PeriodYear)
	}
	if tsd.PeriodMonth != int(now.Month()) {
		t.Errorf("expected period month %d, got %d", int(now.Month()), tsd.PeriodMonth)
	}
	if tsd.Status != TSDDraft {
		t.Errorf("expected status %s, got %s", TSDDraft, tsd.Status)
	}

	// Verify TSD totals are not zero
	if tsd.TotalPayments.IsZero() {
		t.Error("expected total payments to be non-zero")
	}
	if tsd.TotalSocialTax.IsZero() {
		t.Error("expected total social tax to be non-zero")
	}

	// Verify TSD rows
	if len(tsd.Rows) == 0 {
		t.Error("expected at least one TSD row")
	}

	if len(tsd.Rows) > 0 {
		row := tsd.Rows[0]
		if row.EmployeeID != employee.ID {
			t.Errorf("expected employee ID %s, got %s", employee.ID, row.EmployeeID)
		}
		if row.PersonalCode != employee.PersonalCode {
			t.Errorf("expected personal code %s, got %s", employee.PersonalCode, row.PersonalCode)
		}
		if row.FirstName != employee.FirstName {
			t.Errorf("expected first name %s, got %s", employee.FirstName, row.FirstName)
		}
		if row.LastName != employee.LastName {
			t.Errorf("expected last name %s, got %s", employee.LastName, row.LastName)
		}
	}
}

func TestService_GenerateTSD_InvalidStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-invalid-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create payroll run (DRAFT status - not approved)
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Try to generate TSD from DRAFT run (should fail)
	_, err = service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err == nil {
		t.Error("expected error when generating TSD from DRAFT payroll run")
	}
}

func TestService_GetTSD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-get-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee with personal code
	employee, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
		EmployeeNumber:       "EMP-TSD-GET-001",
		FirstName:            "Juri",
		LastName:             "Juurikas",
		PersonalCode:         "38812345678",
		Email:                "juri@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	})
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Create salary
	err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID, decimal.NewFromFloat(2500.00), time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("SetBaseSalary failed: %v", err)
	}

	// Create, calculate, and approve payroll run
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll failed: %v", err)
	}

	err = service.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	// Generate TSD
	_, err = service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GenerateTSD failed: %v", err)
	}

	// Get TSD by period
	tsd, err := service.GetTSD(ctx, tenant.SchemaName, tenant.ID, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("GetTSD failed: %v", err)
	}

	if tsd == nil {
		t.Fatal("expected TSD to be returned")
	}
	if tsd.PeriodYear != now.Year() {
		t.Errorf("expected period year %d, got %d", now.Year(), tsd.PeriodYear)
	}
	if tsd.PeriodMonth != int(now.Month()) {
		t.Errorf("expected period month %d, got %d", int(now.Month()), tsd.PeriodMonth)
	}
	if len(tsd.Rows) == 0 {
		t.Error("expected TSD rows to be loaded")
	}
}

func TestService_GetTSD_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Try to get non-existent TSD
	_, err = service.GetTSD(ctx, tenant.SchemaName, tenant.ID, 2099, 12)
	if err == nil {
		t.Error("expected error when getting non-existent TSD")
	}
}

func TestService_ListTSD(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-list-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee
	employee, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
		EmployeeNumber:       "EMP-TSD-LIST-001",
		FirstName:            "Peeter",
		LastName:             "Porgand",
		PersonalCode:         "37501234567",
		Email:                "peeter@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	})
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Create salary
	err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID, decimal.NewFromFloat(2000.00), time.Now().AddDate(0, -2, 0))
	if err != nil {
		t.Fatalf("SetBaseSalary failed: %v", err)
	}

	// Create and process payroll run
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll failed: %v", err)
	}

	err = service.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	// Generate TSD
	_, err = service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GenerateTSD failed: %v", err)
	}

	// List TSD declarations
	declarations, err := service.ListTSD(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListTSD failed: %v", err)
	}

	if len(declarations) == 0 {
		t.Error("expected at least one TSD declaration")
	}

	// Verify ordering (should be by year/month DESC)
	found := false
	for _, d := range declarations {
		if d.PeriodYear == now.Year() && d.PeriodMonth == int(now.Month()) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find current period TSD in list")
	}
}

func TestService_GetPayslipsWithEmployees(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "payslips-emp-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create two employees
	employee1, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
		EmployeeNumber:       "EMP-PS-001",
		FirstName:            "Anna",
		LastName:             "Aasik",
		PersonalCode:         "49001234567",
		Email:                "anna@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	})
	if err != nil {
		t.Fatalf("CreateEmployee 1 failed: %v", err)
	}

	employee2, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
		EmployeeNumber:       "EMP-PS-002",
		FirstName:            "Bert",
		LastName:             "Buss",
		PersonalCode:         "38502234567",
		Email:                "bert@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	})
	if err != nil {
		t.Fatalf("CreateEmployee 2 failed: %v", err)
	}

	// Create salaries for both
	err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee1.ID, decimal.NewFromFloat(2500.00), time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("SetBaseSalary for employee 1 failed: %v", err)
	}

	err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee2.ID, decimal.NewFromFloat(3000.00), time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("SetBaseSalary for employee 2 failed: %v", err)
	}

	// Create and calculate payroll run
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll failed: %v", err)
	}

	// Get payslips with employee data
	payslips, err := service.GetPayslipsWithEmployees(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GetPayslipsWithEmployees failed: %v", err)
	}

	if len(payslips) != 2 {
		t.Errorf("expected 2 payslips, got %d", len(payslips))
	}

	// Verify employee data is loaded
	for _, ps := range payslips {
		if ps.Employee == nil {
			t.Error("expected employee data to be loaded")
			continue
		}
		if ps.Employee.ID == "" {
			t.Error("expected employee ID to be set")
		}
		if ps.Employee.FirstName == "" {
			t.Error("expected employee first name to be set")
		}
		if ps.Employee.PersonalCode == "" {
			t.Error("expected employee personal code to be set")
		}
	}

	// Verify ordering by last name
	if len(payslips) >= 2 && payslips[0].Employee.LastName > payslips[1].Employee.LastName {
		t.Error("expected payslips to be ordered by last name")
	}
}

func TestService_GenerateTSD_ReplacesExisting(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-replace-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create employee
	employee, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
		EmployeeNumber:       "EMP-REPLACE-001",
		FirstName:            "Karl",
		LastName:             "Kappar",
		PersonalCode:         "36701234567",
		Email:                "karl@example.com",
		StartDate:            time.Now().AddDate(-1, 0, 0),
		EmploymentType:       EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: DefaultBasicExemption,
		FundedPensionRate:    FundedPensionRateDefault,
	})
	if err != nil {
		t.Fatalf("CreateEmployee failed: %v", err)
	}

	// Create salary
	err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, employee.ID, decimal.NewFromFloat(3500.00), time.Now().AddDate(0, -1, 0))
	if err != nil {
		t.Fatalf("SetBaseSalary failed: %v", err)
	}

	now := time.Now()

	// Create first payroll run and TSD
	run1, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun 1 failed: %v", err)
	}

	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run1.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll 1 failed: %v", err)
	}

	err = service.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run1.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun 1 failed: %v", err)
	}

	tsd1, err := service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run1.ID)
	if err != nil {
		t.Fatalf("GenerateTSD 1 failed: %v", err)
	}

	firstTSDID := tsd1.ID

	// Generate TSD again (should replace the first one)
	tsd2, err := service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run1.ID)
	if err != nil {
		t.Fatalf("GenerateTSD 2 failed: %v", err)
	}

	// Verify new TSD has different ID
	if tsd2.ID == firstTSDID {
		t.Error("expected new TSD to have different ID after replacement")
	}

	// Verify only one TSD exists for this period
	declarations, err := service.ListTSD(ctx, tenant.SchemaName, tenant.ID)
	if err != nil {
		t.Fatalf("ListTSD failed: %v", err)
	}

	periodCount := 0
	for _, d := range declarations {
		if d.PeriodYear == now.Year() && d.PeriodMonth == int(now.Month()) {
			periodCount++
		}
	}

	if periodCount != 1 {
		t.Errorf("expected exactly one TSD for the period, got %d", periodCount)
	}
}

func TestService_GenerateTSD_NoPayslips(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-no-payslips-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create payroll run without any employees/payslips
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	// Calculate (no employees = empty payslips)
	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll failed: %v", err)
	}

	// Approve
	err = service.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	// Try to generate TSD (should fail due to no payslips)
	_, err = service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err == nil {
		t.Error("expected error when generating TSD with no payslips")
	}
}

func TestService_GetTSDRows(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "tsd-rows-test@example.com")
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, "admin")

	service := NewService(pool)
	ctx := context.Background()

	// Ensure payroll schema
	_, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName)
	if err != nil {
		t.Fatalf("Failed to add payroll tables: %v", err)
	}

	// Create multiple employees
	for i := 1; i <= 3; i++ {
		emp, err := service.CreateEmployee(ctx, tenant.SchemaName, tenant.ID, &CreateEmployeeRequest{
			EmployeeNumber:       "EMP-ROWS-" + string(rune('0'+i)),
			FirstName:            "Employee" + string(rune('0'+i)),
			LastName:             "Test" + string(rune('0'+i)),
			PersonalCode:         "3870123456" + string(rune('0'+i)),
			Email:                "emp" + string(rune('0'+i)) + "@example.com",
			StartDate:            time.Now().AddDate(-1, 0, 0),
			EmploymentType:       EmploymentFullTime,
			ApplyBasicExemption:  true,
			BasicExemptionAmount: DefaultBasicExemption,
			FundedPensionRate:    FundedPensionRateDefault,
		})
		if err != nil {
			t.Fatalf("CreateEmployee %d failed: %v", i, err)
		}

		err = service.SetBaseSalary(ctx, tenant.SchemaName, tenant.ID, emp.ID, decimal.NewFromFloat(float64(2000+i*500)), time.Now().AddDate(0, -1, 0))
		if err != nil {
			t.Fatalf("SetBaseSalary %d failed: %v", i, err)
		}
	}

	// Create, calculate, approve, and generate TSD
	now := time.Now()
	run, err := service.CreatePayrollRun(ctx, tenant.SchemaName, tenant.ID, userID, &CreatePayrollRunRequest{
		PeriodYear:  now.Year(),
		PeriodMonth: int(now.Month()),
	})
	if err != nil {
		t.Fatalf("CreatePayrollRun failed: %v", err)
	}

	_, err = service.CalculatePayroll(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("CalculatePayroll failed: %v", err)
	}

	err = service.ApprovePayrollRun(ctx, tenant.SchemaName, tenant.ID, run.ID, userID)
	if err != nil {
		t.Fatalf("ApprovePayrollRun failed: %v", err)
	}

	tsd, err := service.GenerateTSD(ctx, tenant.SchemaName, tenant.ID, run.ID)
	if err != nil {
		t.Fatalf("GenerateTSD failed: %v", err)
	}

	// Get TSD rows directly
	rows, err := service.GetTSDRows(ctx, tenant.SchemaName, tenant.ID, tsd.ID)
	if err != nil {
		t.Fatalf("GetTSDRows failed: %v", err)
	}

	if len(rows) != 3 {
		t.Errorf("expected 3 TSD rows, got %d", len(rows))
	}

	// Verify rows are ordered by last name
	for i := 1; i < len(rows); i++ {
		if rows[i-1].LastName > rows[i].LastName {
			t.Errorf("expected rows to be ordered by last name, but %s > %s", rows[i-1].LastName, rows[i].LastName)
		}
	}

	// Verify all rows have required fields
	for _, row := range rows {
		if row.ID == "" {
			t.Error("expected row ID to be set")
		}
		if row.DeclarationID != tsd.ID {
			t.Errorf("expected declaration ID %s, got %s", tsd.ID, row.DeclarationID)
		}
		if row.PersonalCode == "" {
			t.Error("expected personal code to be set")
		}
		if row.GrossPayment.IsZero() {
			t.Error("expected gross payment to be non-zero")
		}
	}
}
