package payroll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportEmployeesCSV_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		FileName: "employees.csv",
		CSVContent: "employee_number,first_name,last_name,personal_code,email,start_date,employment_type,base_salary,salary_effective_from\n" +
			"EMP-001,Mari,Maasikas,49001010001,mari@example.com,2026-01-15,FULL_TIME,3200.00,2026-01-15\n" +
			"EMP-002,Juhan,Tamm,49001010002,juhan@example.com,2026-02-01,PART_TIME,,\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "employees.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 2, result.EmployeesCreated)
	assert.Equal(t, 1, result.SalariesCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	require.Len(t, repo.Employees, 2)
	assert.Equal(t, "Mari", repo.Employees["emp-1"].FirstName)
	assert.Equal(t, EmploymentFullTime, repo.Employees["emp-1"].EmploymentType)
	assert.True(t, repo.Employees["emp-1"].ApplyBasicExemption)
	assert.True(t, repo.Salaries["emp-1"].Equal(decimal.RequireFromString("3200.00")))
	assert.Equal(t, EmploymentPartTime, repo.Employees["emp-3"].EmploymentType)
}

func TestImportEmployeesCSV_SkipsDuplicatesAndInvalidRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["existing-1"] = &Employee{
		ID:             "existing-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-001",
		FirstName:      "Existing",
		LastName:       "Person",
		PersonalCode:   "49001010001",
		Email:          "existing@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		FileName: "employees.csv",
		CSVContent: "employee_number,first_name,last_name,personal_code,email,start_date,employment_type,base_salary\n" +
			"EMP-001,Duplicate,Number,49001010003,duplicate-number@example.com,2026-01-15,FULL_TIME,2500.00\n" +
			"EMP-002,Duplicate,Code,49001010001,duplicate-code@example.com,2026-01-16,FULL_TIME,2500.00\n" +
			"EMP-003,Invalid,Salary,49001010004,invalid-salary@example.com,2026-01-17,FULL_TIME,0\n" +
			"EMP-004,Valid,Employee,49001010005,valid@example.com,2026-01-18,FULL_TIME,2800.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 4, result.RowsProcessed)
	assert.Equal(t, 1, result.EmployeesCreated)
	assert.Equal(t, 1, result.SalariesCreated)
	assert.Equal(t, 3, result.RowsSkipped)
	require.Len(t, result.Errors, 3)
	assert.Contains(t, result.Errors[0].Message, "employee_number")
	assert.Contains(t, result.Errors[1].Message, "personal_code")
	assert.Contains(t, result.Errors[2].Message, "base_salary must be greater than zero")
}

func TestImportEmployeesCSV_RejectsMissingHeaders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	_, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		CSVContent: "first_name,last_name\nMari,Maasikas\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required first_name, last_name, or start_date column")
}

func TestImportPayrollHistoryCSV_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-existing-1"] = &Employee{
		ID:             "emp-existing-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-existing-2"] = &Employee{
		ID:             "emp-existing-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-101",
		FirstName:      "Juhan",
		LastName:       "Tamm",
		PersonalCode:   "49001010002",
		Email:          "juhan@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		FileName: "payroll-history.csv",
		CSVContent: "period_year,period_month,status,payment_date,notes,employee_number,name,gross_salary,income_tax,unemployment_insurance_employee,funded_pension,other_deductions,net_salary,social_tax,unemployment_insurance_employer,total_employer_cost,basic_exemption_applied,payment_status,paid_at\n" +
			"2025,12,DECLARED,2026-01-05,Imported December payroll,EMP-100,,3200.00,550.00,51.20,64.00,0,2534.80,1056.00,25.60,4281.60,50.00,PAID,2026-01-05\n" +
			"2025,12,DECLARED,2026-01-05,Imported December payroll,,Juhan Tamm,2800.00,420.00,44.80,56.00,10.00,2269.20,924.00,22.40,3746.40,40.00,PAID,2026-01-05\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "payroll-history.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.PayrollRunsCreated)
	assert.Equal(t, 2, result.PayslipsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	require.Len(t, repo.PayrollRuns, 1)
	var importedRun *PayrollRun
	for _, run := range repo.PayrollRuns {
		importedRun = run
	}
	require.NotNil(t, importedRun)
	assert.Equal(t, PayrollDeclared, importedRun.Status)
	assert.Equal(t, decimal.RequireFromString("6000.00"), importedRun.TotalGross)
	assert.Equal(t, decimal.RequireFromString("4804.00"), importedRun.TotalNet)
	assert.Equal(t, decimal.RequireFromString("8028.00"), importedRun.TotalEmployerCost)
	assert.Equal(t, "user-1", importedRun.ApprovedBy)
	require.NotNil(t, importedRun.ApprovedAt)
	assert.Equal(t, "2026-01-05", importedRun.ApprovedAt.Format("2006-01-02"))

	require.Len(t, repo.Payslips, 2)
	assert.Equal(t, decimal.RequireFromString("0"), repo.Payslips[0].OtherDeductions)
	require.NotNil(t, repo.Payslips[0].PaidAt)
	assert.Equal(t, "PAID", repo.Payslips[0].PaymentStatus)
	assert.Equal(t, "hist-1", repo.Payslips[0].PayrollRunID)
}

func TestImportPayrollHistoryCSV_MatchesAlternateEmployeeIdentifiersAndDerivesAmounts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-personal"] = &Employee{
		ID:             "emp-personal",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-200",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-email"] = &Employee{
		ID:             "emp-email",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-201",
		FirstName:      "Liis",
		LastName:       "Lepp",
		PersonalCode:   "49001010002",
		Email:          "liis@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-name"] = &Employee{
		ID:             "emp-name",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-202",
		FirstName:      "Kati",
		LastName:       "Kask",
		PersonalCode:   "49001010003",
		Email:          "kati@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		FileName: "payroll-history.csv",
		CSVContent: "period_year,period_month,status,personal_code,email,first_name,last_name,gross_salary,income_tax,unemployment_insurance_employee,funded_pension,other_deductions,social_tax,unemployment_insurance_employer\n" +
			"2025,8,APPROVED,49001010001,,, ,3000.00,500.00,48.00,60.00,10.00,990.00,24.00\n" +
			"2025,8,APPROVED,,liis@example.com,,,2500.00,400.00,40.00,50.00,0.00,825.00,20.00\n" +
			"2025,8,APPROVED,,,Kati,Kask,2000.00,300.00,32.00,40.00,5.00,660.00,16.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 3, result.RowsProcessed)
	assert.Equal(t, 1, result.PayrollRunsCreated)
	assert.Equal(t, 3, result.PayslipsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	require.Len(t, repo.PayrollRuns, 1)
	run := repo.PayrollRuns["hist-1"]
	require.NotNil(t, run)
	assert.Equal(t, PayrollApproved, run.Status)
	assert.Nil(t, run.PaymentDate)
	assert.Equal(t, "user-1", run.ApprovedBy)
	require.NotNil(t, run.ApprovedAt)
	assert.Equal(t, decimal.RequireFromString("7500.00"), run.TotalGross)
	assert.Equal(t, decimal.RequireFromString("6015.00"), run.TotalNet)
	assert.Equal(t, decimal.RequireFromString("10035.00"), run.TotalEmployerCost)

	require.Len(t, repo.Payslips, 3)
	assert.Equal(t, "emp-personal", repo.Payslips[0].EmployeeID)
	assert.Equal(t, "PENDING", repo.Payslips[0].PaymentStatus)
	assert.Equal(t, decimal.RequireFromString("3000.00"), repo.Payslips[0].TaxableIncome)
	assert.Equal(t, decimal.RequireFromString("2382.00"), repo.Payslips[0].NetSalary)
	assert.Equal(t, decimal.RequireFromString("4014.00"), repo.Payslips[0].TotalEmployerCost)
	assert.Equal(t, "emp-email", repo.Payslips[1].EmployeeID)
	assert.Equal(t, "emp-name", repo.Payslips[2].EmployeeID)
}

func TestImportPayrollHistoryCSV_RejectsMismatchedIdentifiersAndGroupInconsistency(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-1"] = &Employee{
		ID:             "emp-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-2"] = &Employee{
		ID:             "emp-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-101",
		FirstName:      "Liis",
		LastName:       "Lepp",
		PersonalCode:   "49001010002",
		Email:          "liis@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		CSVContent: "period_year,period_month,status,payment_date,notes,employee_number,email,gross_salary\n" +
			"2025,7,PAID,2025-08-05,Batch A,EMP-100,,1000.00\n" +
			"2025,7,PAID,2025-08-05,Batch A,EMP-100,,900.00\n" +
			"2025,7,PAID,2025-08-05,Batch A,EMP-100,liis@example.com,800.00\n" +
			"2025,7,APPROVED,2025-08-05,Batch A,EMP-101,,700.00\n" +
			"2025,7,PAID,2025-08-06,Batch A,EMP-101,,600.00\n" +
			"2025,7,PAID,2025-08-05,Batch B,EMP-101,,500.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 6, result.RowsProcessed)
	assert.Equal(t, 1, result.PayrollRunsCreated)
	assert.Equal(t, 1, result.PayslipsCreated)
	assert.Equal(t, 5, result.RowsSkipped)
	require.Len(t, result.Errors, 5)
	assert.Contains(t, result.Errors[0].Message, "employee already has a payslip in this payroll period")
	assert.Contains(t, result.Errors[1].Message, "employee identifiers do not match the same employee")
	assert.Contains(t, result.Errors[2].Message, "status must be consistent for each payroll period")
	assert.Contains(t, result.Errors[3].Message, "payment_date must be consistent for each payroll period")
	assert.Contains(t, result.Errors[4].Message, "notes must be consistent for each payroll period")
}

func TestImportPayrollHistoryCSV_ReportsTransactionErrorsPerGroup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-1"] = &Employee{
		ID:             "emp-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-2"] = &Employee{
		ID:             "emp-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-101",
		FirstName:      "Liis",
		LastName:       "Lepp",
		PersonalCode:   "49001010002",
		Email:          "liis@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.CreatePayslipErr = errors.New("write failed")
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		CSVContent: "period_year,period_month,status,employee_number,gross_salary\n" +
			"2025,6,PAID,EMP-100,1000.00\n" +
			"2025,6,PAID,EMP-101,900.00\n",
	})
	require.NoError(t, err)

	assert.Zero(t, result.PayrollRunsCreated)
	assert.Zero(t, result.PayslipsCreated)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "create payslip: write failed")
	assert.Contains(t, result.Errors[1].Message, "create payslip: write failed")
}

func TestImportPayrollHistoryCSV_SkipsInvalidRowsAndExistingPeriods(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-existing-1"] = &Employee{
		ID:             "emp-existing-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          "mari@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.PayrollRuns["run-existing"] = &PayrollRun{
		ID:          "run-existing",
		TenantID:    "tenant-1",
		PeriodYear:  2025,
		PeriodMonth: 12,
		Status:      PayrollPaid,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		CSVContent: "period_year,period_month,status,employee_number,gross_salary,notes\n" +
			"2025,12,PAID,EMP-100,3200.00,Already exists\n" +
			"2025,11,PAID,EMP-999,3100.00,Missing employee\n" +
			"2025,10,PAID,EMP-100,-10.00,Invalid gross\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 3, result.RowsProcessed)
	assert.Zero(t, result.PayrollRunsCreated)
	assert.Zero(t, result.PayslipsCreated)
	assert.Equal(t, 3, result.RowsSkipped)
	require.Len(t, result.Errors, 3)
	assert.Contains(t, result.Errors[0].Message, "employee_number \"EMP-999\" not found")
	assert.Contains(t, result.Errors[1].Message, "gross_salary must be zero or greater")
	assert.Contains(t, result.Errors[2].Message, "payroll run already exists for 2025-12")
}

func TestImportPayrollHistoryCSV_RejectsMissingHeaders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	_, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		CSVContent: "employee_number,gross_salary\nEMP-001,1000\n",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required period_year, period_month, or gross_salary column")
}
