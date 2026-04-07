package payroll

import (
	"context"
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
