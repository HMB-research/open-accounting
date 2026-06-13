package payroll

import (
	"context"
	"errors"
	"strings"
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

func TestImportEmployeesCSV_ParsesAliasesBooleansEndDatesAndDuplicateRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		FileName: "employees-aliases.csv",
		CSVContent: "number;given_name;surname;isikukood;email;telephone;iban;employment_start;employment_end;title;team;type;basic_exemption;basic_exemption_amount;pension_rate;salary;effective_from;active\n" +
			"EMP-010;Mari;Mets;49001010010;mari.alias@example.com;+37255550000;EE471000001020145685;01.02.2026;2030-12-31;Engineer;Payroll;part-time;ei;0;0,04;4500,50;2026-02-15T00:00:00Z;ja\n" +
			";Mari;Mets;49001010011;duplicate@example.com;;;01.02.2026;;;;full_time;yes;700;;;;\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "employees-aliases.csv", result.FileName)
	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.EmployeesCreated)
	assert.Equal(t, 1, result.SalariesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `employee "Mari Mets" with start_date 2026-02-01 already exists`)

	require.Len(t, repo.Employees, 1)
	employee := repo.Employees["emp-1"]
	require.NotNil(t, employee)
	assert.Equal(t, "EMP-010", employee.EmployeeNumber)
	assert.Equal(t, "Mari", employee.FirstName)
	assert.Equal(t, "Mets", employee.LastName)
	assert.Equal(t, "49001010010", employee.PersonalCode)
	assert.Equal(t, "mari.alias@example.com", employee.Email)
	assert.Equal(t, "+37255550000", employee.Phone)
	assert.Equal(t, "EE471000001020145685", employee.BankAccount)
	assert.Equal(t, "2026-02-01", employee.StartDate.Format("2006-01-02"))
	require.NotNil(t, employee.EndDate)
	assert.Equal(t, "2030-12-31", employee.EndDate.Format("2006-01-02"))
	assert.Equal(t, "Engineer", employee.Position)
	assert.Equal(t, "Payroll", employee.Department)
	assert.Equal(t, EmploymentPartTime, employee.EmploymentType)
	assert.False(t, employee.ApplyBasicExemption)
	assert.True(t, employee.BasicExemptionAmount.IsZero())
	assert.Equal(t, decimal.RequireFromString("0.04"), employee.FundedPensionRate)
	assert.True(t, employee.IsActive)
	assert.Equal(t, decimal.RequireFromString("4500.50"), repo.Salaries["emp-1"])
}

func TestImportEmployeesCSV_ReportsAliasValidationErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		CSVContent: "first_name,last_name,start_date,apply_basic_exemption,end_date,funded_pension_rate,salary_effective_from,base_salary\n" +
			"Bad,Bool,2026-01-01,maybe,,,,\n" +
			"Bad,End,2026-01-10,,2026-01-09,,,\n" +
			"Bad,Pension,2026-01-01,,,1.2,,\n" +
			"Bad,Effective,2026-01-01,,,,2026-01-01,\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 4, result.RowsProcessed)
	assert.Zero(t, result.EmployeesCreated)
	assert.Zero(t, result.SalariesCreated)
	assert.Equal(t, 4, result.RowsSkipped)
	require.Len(t, result.Errors, 4)
	assert.Contains(t, result.Errors[0].Message, `invalid apply_basic_exemption "maybe"`)
	assert.Contains(t, result.Errors[1].Message, "end_date cannot be before start_date")
	assert.Contains(t, result.Errors[2].Message, "funded_pension_rate must be between 0 and 1")
	assert.Contains(t, result.Errors[3].Message, "salary_effective_from requires base_salary")
	assert.Empty(t, repo.Employees)
}

func TestImportEmployeesCSV_ReportsExtendedValidationErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		CSVContent: strings.Join([]string{
			"first_name,last_name,start_date,employment_type,basic_exemption_amount,funded_pension_rate,end_date,is_active,base_salary,salary_effective_from",
			",Missing,2026-01-01,FULL_TIME,,,,,,",
			"Bad,Start,not-a-date,FULL_TIME,,,,,,",
			"Bad,Type,2026-01-01,intern,,,,,,",
			"Bad,BasicParse,2026-01-01,,not-a-decimal,,,,,",
			"Bad,BasicNegative,2026-01-01,,-1,,,,,",
			"Bad,PensionParse,2026-01-01,,,not-a-decimal,,,,",
			"Bad,PensionNegative,2026-01-01,,,-0.01,,,,",
			"Bad,EndParse,2026-01-01,,,,not-a-date,,,",
			"Bad,Active,2026-01-01,,,,,maybe,,",
			"Bad,SalaryParse,2026-01-01,,,,,,not-a-decimal,",
			"Bad,SalaryEffectiveParse,2026-01-01,,,,,,1000,not-a-date",
		}, "\n") + "\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 11, result.RowsProcessed)
	assert.Zero(t, result.EmployeesCreated)
	assert.Zero(t, result.SalariesCreated)
	assert.Equal(t, 11, result.RowsSkipped)
	require.Len(t, result.Errors, 11)

	expectedMessages := []string{
		"first_name and last_name are required",
		"start_date must be in YYYY-MM-DD format",
		`invalid employment_type "intern"`,
		"invalid basic_exemption_amount",
		"basic_exemption_amount must be zero or greater",
		"invalid funded_pension_rate",
		"funded_pension_rate must be between 0 and 1",
		"end_date must be in YYYY-MM-DD format",
		`invalid is_active "maybe"`,
		"invalid base_salary",
		"salary_effective_from must be in YYYY-MM-DD format",
	}
	for i, expected := range expectedMessages {
		assert.Contains(t, result.Errors[i].Message, expected)
	}
	assert.Empty(t, repo.Employees)
}

func TestImportEmployeesCSV_SkipsEmailDuplicatesAndIgnoresUnknownHeaders(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["existing-1"] = &Employee{
		ID:             "existing-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-001",
		FirstName:      "Existing",
		LastName:       "Person",
		Email:          "Existing@Example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		CSVContent: "first_name,last_name,start_date,email,external_id,\n" +
			"Duplicate,Email,2026-01-01,existing@example.com,legacy-1,ignored\n" +
			"Valid,Worker,2026-01-02,valid@example.com,legacy-2,ignored\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.EmployeesCreated)
	assert.Zero(t, result.SalariesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, `email "existing@example.com" already exists`)
	require.Len(t, repo.Employees, 2)
	assert.Equal(t, "Valid", repo.Employees["emp-1"].FirstName)
}

func TestImportEmployeesCSV_DerivesInactiveStatusFromPastEndDate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

	result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
		CSVContent: "first_name,last_name,start_date,end_date\n" +
			"Mari,Past,2019-01-01,2020-01-01\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.EmployeesCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)
	require.Len(t, repo.Employees, 1)
	employee := repo.Employees["emp-1"]
	require.NotNil(t, employee.EndDate)
	assert.Equal(t, "2020-01-01", employee.EndDate.Format("2006-01-02"))
	assert.False(t, employee.IsActive)
}

func TestImportEmployeesCSV_ReportsTransactionFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupRepo func(*MockRepository)
		content   string
		wantErr   string
	}{
		{
			name: "create employee failure",
			setupRepo: func(repo *MockRepository) {
				repo.CreateEmployeeErr = errors.New("create failed")
			},
			content: "first_name,last_name,start_date\n" +
				"Mari,Create,2026-01-01\n",
			wantErr: "create employee: create failed",
		},
		{
			name: "update employee failure",
			setupRepo: func(repo *MockRepository) {
				repo.UpdateEmployeeErr = errors.New("update failed")
			},
			content: "first_name,last_name,start_date,is_active\n" +
				"Mari,Update,2026-01-01,false\n",
			wantErr: "update employee: update failed",
		},
		{
			name: "base salary failure",
			setupRepo: func(repo *MockRepository) {
				repo.CreateSalaryComponentErr = errors.New("salary failed")
			},
			content: "first_name,last_name,start_date,base_salary\n" +
				"Mari,Salary,2026-01-01,2500\n",
			wantErr: "set base salary: salary failed",
		},
		{
			name: "transaction failure",
			setupRepo: func(repo *MockRepository) {
				repo.WithTransactionErr = errors.New("transaction unavailable")
			},
			content: "first_name,last_name,start_date\n" +
				"Mari,Transaction,2026-01-01\n",
			wantErr: "transaction unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := NewMockRepository()
			tt.setupRepo(repo)
			service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

			result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
				CSVContent: tt.content,
			})
			require.NoError(t, err)

			assert.Equal(t, 1, result.RowsProcessed)
			assert.Zero(t, result.EmployeesCreated)
			assert.Zero(t, result.SalariesCreated)
			assert.Equal(t, 1, result.RowsSkipped)
			require.Len(t, result.Errors, 1)
			assert.Contains(t, result.Errors[0].Message, tt.wantErr)
		})
	}
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

func TestImportEmployeesCSV_RejectsNilOrEmptyRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := NewServiceWithRepository(NewMockRepository(), &MockUUIDGenerator{prefix: "emp"})

	tests := []struct {
		name string
		req  *ImportEmployeesRequest
	}{
		{name: "nil request"},
		{name: "empty content", req: &ImportEmployeesRequest{CSVContent: " \n\t "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", tt.req)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "csv_content is required")
		})
	}
}

func TestImportEmployeesCSV_RejectsNoRowsAndRepositoryFailures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("no nonblank employee rows", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), &MockUUIDGenerator{prefix: "emp"})

		result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
			CSVContent: "first_name,last_name,start_date\n,,\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no employees found in CSV")
	})

	t.Run("list existing employees failure", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListEmployeesErr = errors.New("list failed")
		service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "emp"})

		result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
			CSVContent: "first_name,last_name,start_date\nMari,Maasikas,2026-01-01\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "list existing employees: list failed")
	})

	t.Run("malformed header", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), &MockUUIDGenerator{prefix: "emp"})

		result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
			CSVContent: "\"first_name,last_name,start_date\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parse csv header")
	})

	t.Run("malformed row", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), &MockUUIDGenerator{prefix: "emp"})

		result, err := service.ImportEmployeesCSV(ctx, "tenant_schema", "tenant-1", &ImportEmployeesRequest{
			CSVContent: "first_name,last_name,start_date\n\"Mari,Maasikas,2026-01-01\n",
		})

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parse csv row 2")
	})
}

func TestEmployeeImportNormalizationHelpers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "legacy_column", canonicalEmployeeImportHeader(" Legacy_Column "))
	assert.Empty(t, employeeImportNameStartKey(" ", " ", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.Empty(t, employeeImportNameStartKey("OnlyFirst", "", time.Time{}))
	assert.Equal(
		t,
		"mari mets|2026-01-01",
		employeeImportNameStartKey(" Mari ", " Mets ", time.Date(2026, 1, 1, 12, 30, 0, 0, time.Local)),
	)
}

func TestParseEmployeeImportRowsRejectsEmptyContent(t *testing.T) {
	t.Parallel()

	rows, err := parseEmployeeImportRows("")
	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "csv_content is required")
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

func TestImportPayrollHistoryCSV_AcceptsMigrationAliasesAndCanceledPaymentStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-300"] = &Employee{
		ID:             "emp-300",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-300",
		FirstName:      "Marta",
		LastName:       "Mets",
		PersonalCode:   "49001010300",
		Email:          "marta@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		FileName: "payroll-history-aliases.csv",
		CSVContent: "payroll_year,payroll_month,run_status,pay_date,employee_no,gross,income_tax,unemployment_employee,pension,net,social_tax,unemployment_employer,employer_cost,basic_exemption_applied,payment_status\n" +
			"2025,9,paid,2025-10-05,EMP-300,1200.00,100.00,19.20,24.00,1056.80,396.00,9.60,1605.60,0,canceled\n",
	})
	require.NoError(t, err)

	assert.Equal(t, "payroll-history-aliases.csv", result.FileName)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.PayrollRunsCreated)
	assert.Equal(t, 1, result.PayslipsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	run := repo.PayrollRuns["hist-1"]
	require.NotNil(t, run)
	assert.Equal(t, PayrollPaid, run.Status)
	require.NotNil(t, run.PaymentDate)
	assert.Equal(t, "2025-10-05", run.PaymentDate.Format("2006-01-02"))
	assert.Equal(t, "user-1", run.ApprovedBy)
	require.NotNil(t, run.ApprovedAt)
	assert.Equal(t, "2025-10-05", run.ApprovedAt.Format("2006-01-02"))
	assert.True(t, run.TotalGross.Equal(decimal.RequireFromString("1200.00")))
	assert.True(t, run.TotalNet.Equal(decimal.RequireFromString("1056.80")))
	assert.True(t, run.TotalEmployerCost.Equal(decimal.RequireFromString("1605.60")))

	require.Len(t, repo.Payslips, 1)
	payslip := repo.Payslips[0]
	assert.Equal(t, "hist-2", payslip.ID)
	assert.Equal(t, "hist-1", payslip.PayrollRunID)
	assert.Equal(t, "emp-300", payslip.EmployeeID)
	assert.Equal(t, "CANCELLED", payslip.PaymentStatus) //nolint:misspell // Existing API/database spelling.
	assert.Nil(t, payslip.PaidAt)
	assert.True(t, payslip.NetSalary.Equal(decimal.RequireFromString("1056.80")))
	assert.True(t, payslip.TotalEmployerCost.Equal(decimal.RequireFromString("1605.60")))
}

func TestImportPayrollHistoryCSV_DefaultsPaidRunWithBlankImporter(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-blank-user"] = &Employee{
		ID:             "emp-blank-user",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-350",
		FirstName:      "Marten",
		LastName:       "Muru",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "", &ImportPayrollHistoryRequest{
		CSVContent: "period_year,period_month,status,employee_number,gross_salary,basic_exemption_applied,payment_status\n" +
			"2025,7,,EMP-350,1000.00,1200.00,\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 1, result.RowsProcessed)
	assert.Equal(t, 1, result.PayrollRunsCreated)
	assert.Equal(t, 1, result.PayslipsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)

	run := repo.PayrollRuns["hist-1"]
	require.NotNil(t, run)
	assert.Equal(t, PayrollPaid, run.Status)
	assert.Empty(t, run.CreatedBy)
	assert.Empty(t, run.ApprovedBy)
	assert.Nil(t, run.PaymentDate)
	assert.NotNil(t, run.ApprovedAt)

	require.Len(t, repo.Payslips, 1)
	payslip := repo.Payslips[0]
	assert.Equal(t, "PAID", payslip.PaymentStatus)
	assert.Nil(t, payslip.PaidAt)
	assert.True(t, payslip.TaxableIncome.IsZero())
	assert.True(t, payslip.NetSalary.Equal(decimal.RequireFromString("1000.00")))
	assert.True(t, payslip.TotalEmployerCost.Equal(decimal.RequireFromString("1000.00")))
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

func TestImportPayrollHistoryCSV_RejectsAmbiguousNameIdentifierConflicts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-1"] = &Employee{
		ID:             "emp-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-2"] = &Employee{
		ID:             "emp-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-200",
		FirstName:      "Duplicate",
		LastName:       "Name",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-3"] = &Employee{
		ID:             "emp-3",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-201",
		FirstName:      "Duplicate",
		LastName:       "Name",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		CSVContent: "period_year,period_month,status,employee_number,name,first_name,last_name,gross_salary\n" +
			"2025,5,PAID,EMP-100,Duplicate Name,,,1000.00\n" +
			"2025,5,PAID,EMP-100,,Duplicate,Name,900.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.RowsProcessed)
	assert.Zero(t, result.PayrollRunsCreated)
	assert.Zero(t, result.PayslipsCreated)
	assert.Equal(t, 2, result.RowsSkipped)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "employee identifiers do not match the same employee")
	assert.Contains(t, result.Errors[1].Message, "employee identifiers do not match the same employee")
}

func TestImportPayrollHistoryCSV_AllowsAmbiguousNameWithMatchingExplicitIdentifier(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-1"] = &Employee{
		ID:             "emp-1",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-200",
		FirstName:      "Duplicate",
		LastName:       "Name",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	repo.Employees["emp-2"] = &Employee{
		ID:             "emp-2",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-201",
		FirstName:      "Duplicate",
		LastName:       "Name",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		CSVContent: "period_year,period_month,status,employee_number,name,first_name,last_name,gross_salary\n" +
			"2025,5,PAID,EMP-200,Duplicate Name,,,1000.00\n" +
			"2025,5,PAID,EMP-201,,Duplicate,Name,900.00\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 2, result.RowsProcessed)
	assert.Equal(t, 1, result.PayrollRunsCreated)
	assert.Equal(t, 2, result.PayslipsCreated)
	assert.Zero(t, result.RowsSkipped)
	assert.Nil(t, result.Errors)
	require.Len(t, repo.Payslips, 2)
	assert.Equal(t, "emp-1", repo.Payslips[0].EmployeeID)
	assert.Equal(t, "emp-2", repo.Payslips[1].EmployeeID)
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

func TestImportPayrollHistoryCSV_RejectsMalformedPeriodStatusAndAmountRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMockRepository()
	repo.Employees["emp-100"] = &Employee{
		ID:             "emp-100",
		TenantID:       "tenant-1",
		EmployeeNumber: "EMP-100",
		FirstName:      "Mari",
		LastName:       "Maasikas",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "hist"})

	result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", &ImportPayrollHistoryRequest{
		CSVContent: "period_year,period_month,status,employee_number,gross_salary,taxable_income,income_tax\n" +
			"twenty,5,PAID,EMP-100,1000.00,,\n" +
			"2025,13,PAID,EMP-100,1000.00,,\n" +
			"2025,5,CALCULATED,EMP-100,1000.00,,\n" +
			"2025,5,PAID,EMP-100,,,\n" +
			"2025,5,PAID,EMP-100,0.00,,\n" +
			"2025,5,PAID,EMP-100,1000.00,,-1.00\n" +
			"2025,5,PAID,EMP-100,1000.00,not-a-decimal,\n",
	})
	require.NoError(t, err)

	assert.Equal(t, 7, result.RowsProcessed)
	assert.Zero(t, result.PayrollRunsCreated)
	assert.Zero(t, result.PayslipsCreated)
	assert.Equal(t, 7, result.RowsSkipped)
	require.Len(t, result.Errors, 7)
	assert.Contains(t, result.Errors[0].Message, "period_year must be between 2020 and 2100")
	assert.Contains(t, result.Errors[1].Message, "period_month must be between 1 and 12")
	assert.Contains(t, result.Errors[2].Message, "status must be APPROVED, PAID, or DECLARED")
	assert.Contains(t, result.Errors[3].Message, "gross_salary is required")
	assert.Contains(t, result.Errors[4].Message, "gross_salary must be greater than zero")
	assert.Contains(t, result.Errors[5].Message, "income_tax must be zero or greater")
	assert.Contains(t, result.Errors[6].Message, "invalid taxable_income")
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

func TestImportPayrollHistoryCSV_RejectsNilOrEmptyRequest(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := NewServiceWithRepository(NewMockRepository(), &MockUUIDGenerator{prefix: "hist"})

	tests := []struct {
		name string
		req  *ImportPayrollHistoryRequest
	}{
		{name: "nil request"},
		{name: "empty content", req: &ImportPayrollHistoryRequest{CSVContent: " \n\t "}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ImportPayrollHistoryCSV(ctx, "tenant_schema", "tenant-1", "user-1", tt.req)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "csv_content is required")
		})
	}
}
