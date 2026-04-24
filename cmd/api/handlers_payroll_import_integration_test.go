package main

import (
	"context"
	"net/http"
	"testing"
	"time"

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

func TestImportPayrollHistoryHandlerIntegration(t *testing.T) {
	h, tenant, claims, _ := setupPayrollIntegrationHandlers(t)

	invokeJSON[payroll.ImportEmployeesResult](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ImportEmployees(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/employees/import", map[string]any{
		"file_name": "employees.csv",
		"csv_content": "employee_number,first_name,last_name,personal_code,email,start_date,employment_type\n" +
			"EMP-200,Mari,Maasikas,49001010001,mari@example.com,2025-01-15,FULL_TIME\n" +
			"EMP-201,Juhan,Tamm,49001010002,juhan@example.com,2025-01-15,FULL_TIME\n",
	}, claims), map[string]string{"tenantID": tenant.ID}))

	result := invokeJSON[payroll.ImportPayrollHistoryResult](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ImportPayrollHistory(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/payroll-runs/import-history", map[string]any{
		"file_name": "payroll-history.csv",
		"csv_content": "period_year,period_month,status,payment_date,notes,employee_number,gross_salary,income_tax,unemployment_insurance_employee,funded_pension,net_salary,social_tax,unemployment_insurance_employer,total_employer_cost,basic_exemption_applied\n" +
			"2025,12,PAID,2026-01-05,Imported run,EMP-200,3200.00,550.00,51.20,64.00,2534.80,1056.00,25.60,4281.60,50.00\n" +
			"2025,12,PAID,2026-01-05,Imported run,EMP-201,2800.00,420.00,44.80,56.00,2279.20,924.00,22.40,3746.40,0\n",
	}, claims), map[string]string{"tenantID": tenant.ID}))

	require.Equal(t, 2, result.RowsProcessed)
	require.Equal(t, 1, result.PayrollRunsCreated)
	require.Equal(t, 2, result.PayslipsCreated)
	require.Zero(t, result.RowsSkipped)

	runs := invokeJSON[[]payroll.PayrollRun](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListPayrollRuns(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/payroll-runs?year=2025", nil, claims), map[string]string{"tenantID": tenant.ID}))

	require.Len(t, runs, 1)
	require.Equal(t, payroll.PayrollPaid, runs[0].Status)
	require.True(t, runs[0].TotalGross.Equal(decimal.RequireFromString("6000")))

	payslips := invokeJSON[[]payroll.Payslip](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetPayslips(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/payroll-runs/"+runs[0].ID+"/payslips", nil, claims), map[string]string{"tenantID": tenant.ID, "runID": runs[0].ID}))

	require.Len(t, payslips, 2)
	require.Equal(t, "PAID", payslips[0].PaymentStatus)
}

func TestImportLeaveBalancesHandlerIntegration(t *testing.T) {
	h, tenant, claims, pool := setupPayrollIntegrationHandlers(t)

	employee := invokeJSON[payroll.Employee](t, http.StatusCreated, func(w http.ResponseWriter, r *http.Request) {
		h.CreateEmployee(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/employees", payroll.CreateEmployeeRequest{
		EmployeeNumber:       "EMP-300",
		FirstName:            "Liis",
		LastName:             "Puhkus",
		StartDate:            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EmploymentType:       payroll.EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: payroll.DefaultBasicExemption,
		FundedPensionRate:    payroll.FundedPensionRateDefault,
	}, claims), map[string]string{"tenantID": tenant.ID}))

	absenceType := insertTenantAbsenceType(t, pool, tenant)

	result := invokeJSON[payroll.ImportLeaveBalancesResult](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ImportLeaveBalances(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/leave-balances/import", map[string]any{
		"file_name": "leave-balances.csv",
		"csv_content": "year,employee_number,absence_type_code,entitled_days,carryover_days,used_days,pending_days,notes\n" +
			"2025,EMP-300," + absenceType.Code + ",28,2,4,1,Imported leave balance\n",
	}, claims), map[string]string{"tenantID": tenant.ID}))

	require.Equal(t, 1, result.RowsProcessed)
	require.Equal(t, 1, result.LeaveBalancesCreated)
	require.Zero(t, result.LeaveBalancesUpdated)
	require.Zero(t, result.RowsSkipped)

	balances := invokeJSON[[]payroll.LeaveBalance](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListLeaveBalances(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/employees/"+employee.ID+"/leave-balances?year=2025", nil, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID}))

	require.Len(t, balances, 1)
	require.True(t, balances[0].RemainingDays.Equal(decimal.NewFromInt(25)))
	require.Equal(t, "Imported leave balance", balances[0].Notes)
}
