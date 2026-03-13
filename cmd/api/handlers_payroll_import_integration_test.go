package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/payroll"
)

func TestImportEmployeesHandlerIntegration(t *testing.T) {
	h, tenant, claims, pool := setupPayrollIntegrationHandlers(t)

	result := invokeJSON[payroll.ImportEmployeesResult](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ImportEmployees(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/employees/import", map[string]any{
		"file_name": "employees.csv",
		"csv_content": "employee_number,first_name,last_name,personal_code,email,start_date,employment_type,base_salary,salary_effective_from\n" +
			"EMP-100,Mari,Maasikas,49001010001,mari@example.com,2026-01-15,FULL_TIME,3200.00,2026-01-15\n" +
			"EMP-101,Juhan,Tamm,49001010002,juhan@example.com,2026-02-01,PART_TIME,,\n",
	}, claims), map[string]string{"tenantID": tenant.ID}))

	require.Equal(t, 2, result.RowsProcessed)
	require.Equal(t, 2, result.EmployeesCreated)
	require.Equal(t, 1, result.SalariesCreated)
	require.Zero(t, result.RowsSkipped)

	employees := invokeJSON[[]payroll.Employee](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListEmployees(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/employees", nil, claims), map[string]string{"tenantID": tenant.ID}))

	require.Len(t, employees, 2)

	var importedEmployee payroll.Employee
	for _, employee := range employees {
		if employee.EmployeeNumber == "EMP-100" {
			importedEmployee = employee
			break
		}
	}
	require.NotEmpty(t, importedEmployee.ID)

	salaryService := payroll.NewService(pool)
	salary, err := salaryService.GetCurrentSalary(context.Background(), tenant.SchemaName, tenant.ID, importedEmployee.ID)
	require.NoError(t, err)
	require.True(t, salary.Equal(decimal.RequireFromString("3200")))
}
