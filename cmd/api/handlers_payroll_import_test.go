package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/payroll"
)

func TestPayrollImportHandlers(t *testing.T) {
	t.Run("imports employees with default file name", func(t *testing.T) {
		h, repo, _ := setupPayrollImportHandlerTest(t)

		result := invokePayrollImportJSON[payroll.ImportEmployeesResult](t, http.StatusOK, h.ImportEmployees, payrollImportRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/import",
			map[string]any{
				"csv_content": "employee_number,first_name,last_name,start_date,base_salary\nEMP-100,Mari,Maasikas,2026-01-15,3200.00\n",
			},
		))

		require.Equal(t, "employees_import.csv", result.FileName)
		require.Equal(t, 1, result.RowsProcessed)
		require.Equal(t, 1, result.EmployeesCreated)
		require.Equal(t, 1, result.SalariesCreated)
		require.Len(t, repo.employees, 1)
		require.Len(t, repo.salaryComponents, 1)
	})

	t.Run("imports payroll history with default file name", func(t *testing.T) {
		h, repo, _ := setupPayrollImportHandlerTest(t)
		repo.seedEmployee(payrollImportEmployee("emp-1", "EMP-200"))

		result := invokePayrollImportJSON[payroll.ImportPayrollHistoryResult](t, http.StatusOK, h.ImportPayrollHistory, payrollImportRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs/import-history",
			map[string]any{
				"csv_content": "period_year,period_month,employee_number,gross_salary\n2025,12,EMP-200,3200.00\n",
			},
		))

		require.Equal(t, "payroll-history.csv", result.FileName)
		require.Equal(t, 1, result.RowsProcessed)
		require.Equal(t, 1, result.PayrollRunsCreated)
		require.Equal(t, 1, result.PayslipsCreated)
		require.Len(t, repo.payrollRuns, 1)
		require.Len(t, repo.payslips, 1)
	})

	t.Run("imports TSD history with default file name", func(t *testing.T) {
		h, repo, _ := setupPayrollImportHandlerTest(t)
		repo.seedEmployee(payrollImportEmployee("emp-1", "EMP-250"))

		result := invokePayrollImportJSON[payroll.ImportTSDHistoryResult](t, http.StatusOK, h.ImportTSDHistory, payrollImportRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/import-history",
			map[string]any{
				"csv_content": "year,month,employee_number,gross_payment\n2025,12,EMP-250,3200.00\n",
			},
		))

		require.Equal(t, "tsd-history.csv", result.FileName)
		require.Equal(t, 1, result.RowsProcessed)
		require.Equal(t, 1, result.DeclarationsCreated)
		require.Equal(t, 1, result.RowsImported)
		require.Len(t, repo.tsdDeclarations, 1)
		require.Len(t, repo.tsdRows, 1)
	})

	t.Run("imports leave balances with default file name", func(t *testing.T) {
		h, _, absenceRepo := setupPayrollImportHandlerTest(t)
		absenceRepo.Employees["emp-1"] = payrollImportEmployee("emp-1", "EMP-300")
		absenceRepo.AbsenceTypes["absence-1"] = &payroll.AbsenceType{
			ID:                 "absence-1",
			TenantID:           "tenant-1",
			Code:               "ANNUAL_LEAVE",
			Name:               "Annual leave",
			DefaultDaysPerYear: decimal.NewFromInt(28),
			IsActive:           true,
		}

		result := invokePayrollImportJSON[payroll.ImportLeaveBalancesResult](t, http.StatusOK, h.ImportLeaveBalances, payrollImportRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-balances/import",
			map[string]any{
				"csv_content": "year,employee_number,absence_type_code\n2025,EMP-300,ANNUAL_LEAVE\n",
			},
		))

		require.Equal(t, "leave-balances.csv", result.FileName)
		require.Equal(t, 1, result.RowsProcessed)
		require.Equal(t, 1, result.LeaveBalancesCreated)
		require.Len(t, absenceRepo.LeaveBalances, 1)
	})
}

func TestPayrollImportHandlersRejectBadRequests(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{name: "employees", path: "/tenants/tenant-1/employees/import"},
		{name: "payroll history", path: "/tenants/tenant-1/payroll-runs/import-history"},
		{name: "TSD history", path: "/tenants/tenant-1/tsd/import-history"},
		{name: "leave balances", path: "/tenants/tenant-1/leave-balances/import"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" invalid JSON", func(t *testing.T) {
			h, _, _ := setupPayrollImportHandlerTest(t)
			tt.handler = payrollImportHandlerByName(h, tt.name)

			req := httptest.NewRequest(http.MethodPost, tt.path, http.NoBody)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})

			body := invokePayrollImportJSON[map[string]string](t, http.StatusBadRequest, tt.handler, req)
			require.Equal(t, "Invalid request body", body["error"])
		})

		t.Run(tt.name+" missing CSV content", func(t *testing.T) {
			h, _, _ := setupPayrollImportHandlerTest(t)
			tt.handler = payrollImportHandlerByName(h, tt.name)

			body := invokePayrollImportJSON[map[string]string](t, http.StatusBadRequest, tt.handler, payrollImportRequest(
				http.MethodPost,
				tt.path,
				map[string]any{"csv_content": "  "},
			))
			require.Equal(t, "csv_content is required", body["error"])
		})
	}
}

func TestPayrollBusinessHandlersEmployeesAndSalaryComponents(t *testing.T) {
	h, _, _ := setupPayrollImportHandlerTest(t)
	startDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	created := invokePayrollImportJSON[payroll.Employee](t, http.StatusCreated, h.CreateEmployee, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/employees",
		payroll.CreateEmployeeRequest{
			EmployeeNumber:      "EMP-500",
			FirstName:           "Mari",
			LastName:            "Maasikas",
			PersonalCode:        "49001010500",
			Email:               "mari.handler@example.com",
			StartDate:           startDate,
			EmploymentType:      payroll.EmploymentFullTime,
			ApplyBasicExemption: true,
			FundedPensionRate:   decimal.RequireFromString("0.02"),
		},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "payroll-1", created.ID)
	require.Equal(t, payroll.DefaultBasicExemption.String(), created.BasicExemptionAmount.String())

	activeEmployees := invokePayrollImportJSON[[]payroll.Employee](t, http.StatusOK, h.ListEmployees, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/employees?active_only=true",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, activeEmployees, 1)
	require.Equal(t, created.ID, activeEmployees[0].ID)

	got := invokePayrollImportJSON[payroll.Employee](t, http.StatusOK, h.GetEmployee, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/employees/"+created.ID,
		nil,
		map[string]string{"tenantID": "tenant-1", "employeeID": created.ID},
	))
	require.Equal(t, "Mari", got.FirstName)

	inactive := false
	updated := invokePayrollImportJSON[payroll.Employee](t, http.StatusOK, h.UpdateEmployee, payrollHandlerRequest(
		http.MethodPut,
		"/tenants/tenant-1/employees/"+created.ID,
		payroll.UpdateEmployeeRequest{
			LastName: "Kask",
			IsActive: &inactive,
		},
		map[string]string{"tenantID": "tenant-1", "employeeID": created.ID},
	))
	require.Equal(t, "Kask", updated.LastName)
	require.False(t, updated.IsActive)

	salaryResponse := invokePayrollImportJSON[map[string]string](t, http.StatusOK, h.SetBaseSalary, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/employees/"+created.ID+"/salary",
		map[string]any{
			"amount":         "3200.00",
			"effective_from": startDate,
		},
		map[string]string{"tenantID": "tenant-1", "employeeID": created.ID},
	))
	require.Equal(t, "salary updated", salaryResponse["status"])

	oneOff := false
	component := invokePayrollImportJSON[payroll.SalaryComponent](t, http.StatusCreated, h.AddSalaryComponent, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/employees/"+created.ID+"/salary-components",
		payroll.CreateSalaryComponentRequest{
			ComponentType: payroll.SalaryComponentBonus,
			Name:          "Quarterly bonus",
			Amount:        decimal.RequireFromString("450.00"),
			IsRecurring:   &oneOff,
			EffectiveFrom: startDate,
		},
		map[string]string{"tenantID": "tenant-1", "employeeID": created.ID},
	))
	require.Equal(t, "payroll-3", component.ID)
	require.Equal(t, payroll.SalaryComponentBonus, component.ComponentType)
	require.False(t, component.IsRecurring)

	components := invokePayrollImportJSON[[]payroll.SalaryComponent](t, http.StatusOK, h.ListSalaryComponents, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/employees/"+created.ID+"/salary-components?active_on=2026-03-01",
		nil,
		map[string]string{"tenantID": "tenant-1", "employeeID": created.ID},
	))
	require.Len(t, components, 2)
	require.Equal(t, payroll.SalaryComponentBaseSalary, components[0].ComponentType)
	require.Equal(t, payroll.SalaryComponentBonus, components[1].ComponentType)
}

func TestPayrollBusinessHandlersRunLifecycleAndTSD(t *testing.T) {
	h, repo, _ := setupPayrollImportHandlerTest(t)
	employee := payrollImportEmployee("emp-600", "EMP-600")
	employee.ApplyBasicExemption = true
	employee.BasicExemptionAmount = payroll.DefaultBasicExemption
	employee.FundedPensionRate = decimal.RequireFromString("0.02")
	repo.seedEmployee(employee)
	repo.salaryComponents = append(repo.salaryComponents, payroll.SalaryComponent{
		ID:            "salary-1",
		TenantID:      "tenant-1",
		EmployeeID:    employee.ID,
		ComponentType: payroll.SalaryComponentBaseSalary,
		Name:          "Base salary",
		Amount:        decimal.RequireFromString("3000.00"),
		IsTaxable:     true,
		IsRecurring:   true,
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	createdRun := invokePayrollImportJSON[payroll.PayrollRun](t, http.StatusCreated, h.CreatePayrollRun, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/payroll-runs",
		payroll.CreatePayrollRunRequest{
			PeriodYear:  2026,
			PeriodMonth: 3,
			Notes:       "March payroll",
		},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "payroll-1", createdRun.ID)
	require.Equal(t, payroll.PayrollDraft, createdRun.Status)

	runs := invokePayrollImportJSON[[]payroll.PayrollRun](t, http.StatusOK, h.ListPayrollRuns, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/payroll-runs?year=2026",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, runs, 1)
	require.Equal(t, createdRun.ID, runs[0].ID)

	gotRun := invokePayrollImportJSON[payroll.PayrollRun](t, http.StatusOK, h.GetPayrollRun, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/payroll-runs/"+createdRun.ID,
		nil,
		map[string]string{"tenantID": "tenant-1", "runID": createdRun.ID},
	))
	require.Equal(t, "March payroll", gotRun.Notes)

	calculated := invokePayrollImportJSON[payroll.PayrollRun](t, http.StatusOK, h.CalculatePayroll, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/payroll-runs/"+createdRun.ID+"/calculate",
		nil,
		map[string]string{"tenantID": "tenant-1", "runID": createdRun.ID},
	))
	require.Equal(t, payroll.PayrollCalculated, calculated.Status)
	require.Len(t, calculated.Payslips, 1)
	require.True(t, calculated.TotalGross.Equal(decimal.RequireFromString("3000.00")))

	payslips := invokePayrollImportJSON[[]payroll.Payslip](t, http.StatusOK, h.GetPayslips, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/payroll-runs/"+createdRun.ID+"/payslips",
		nil,
		map[string]string{"tenantID": "tenant-1", "runID": createdRun.ID},
	))
	require.Len(t, payslips, 1)
	require.Equal(t, employee.ID, payslips[0].EmployeeID)

	approvalResponse := invokePayrollImportJSON[map[string]string](t, http.StatusOK, h.ApprovePayroll, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/payroll-runs/"+createdRun.ID+"/approve",
		nil,
		map[string]string{"tenantID": "tenant-1", "runID": createdRun.ID},
	))
	require.Equal(t, "approved", approvalResponse["status"])

	processedRun := invokePayrollImportJSON[payroll.PayrollRun](t, http.StatusCreated, h.CreatePayrollRun, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/payroll-runs",
		payroll.CreatePayrollRunRequest{
			PeriodYear:  2026,
			PeriodMonth: 4,
			Notes:       "April payroll",
		},
		map[string]string{"tenantID": "tenant-1"},
	))
	processResult := invokePayrollImportJSON[payroll.PayrollRunProcessResult](t, http.StatusOK, h.ProcessPayrollRun, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/payroll-runs/"+processedRun.ID+"/process",
		payroll.ProcessPayrollRunRequest{Approve: true},
		map[string]string{"tenantID": "tenant-1", "runID": processedRun.ID},
	))
	require.True(t, processResult.Approved)
	require.Equal(t, 1, processResult.PayslipCount)
	require.Equal(t, payroll.PayrollApproved, processResult.PayrollRun.Status)

	tsd := invokePayrollImportJSON[payroll.TSDDeclaration](t, http.StatusOK, h.GenerateTSD, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/payroll-runs/"+processedRun.ID+"/tsd",
		nil,
		map[string]string{"tenantID": "tenant-1", "runID": processedRun.ID},
	))
	require.Equal(t, payroll.TSDDraft, tsd.Status)
	require.Equal(t, 2026, tsd.PeriodYear)
	require.Equal(t, 4, tsd.PeriodMonth)
	require.Len(t, tsd.Rows, 1)
	require.True(t, tsd.TotalPayments.Equal(decimal.RequireFromString("3000.00")))
}

func TestPayrollBusinessHandlersTSDPeriodActions(t *testing.T) {
	h, repo, _ := setupPayrollImportHandlerTest(t)
	repo.tsdDeclarations["tsd-1"] = &payroll.TSDDeclaration{
		ID:            "tsd-1",
		TenantID:      "tenant-1",
		PeriodYear:    2026,
		PeriodMonth:   3,
		Status:        payroll.TSDDraft,
		TotalPayments: decimal.RequireFromString("3000.00"),
	}

	declarations := invokePayrollImportJSON[[]payroll.TSDDeclaration](t, http.StatusOK, h.ListTSD, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tsd?year=2026&month=3",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, declarations, 1)
	require.Equal(t, "tsd-1", declarations[0].ID)

	got := invokePayrollImportJSON[payroll.TSDDeclaration](t, http.StatusOK, h.GetTSD, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tsd/2026/3",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
	))
	require.Equal(t, payroll.TSDDraft, got.Status)

	submitted := invokePayrollImportJSON[map[string]string](t, http.StatusOK, h.MarkTSDSubmitted, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tsd/2026/3/submit",
		map[string]any{"emta_reference": "EMTA-2026-03"},
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
	))
	require.Equal(t, "submitted", submitted["status"])
	require.Equal(t, payroll.TSDSubmitted, repo.tsdDeclarations["tsd-1"].Status)
	require.Equal(t, "EMTA-2026-03", repo.tsdDeclarations["tsd-1"].EMTAReference)

	accepted := invokePayrollImportJSON[map[string]string](t, http.StatusOK, h.MarkTSDAccepted, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tsd/2026/3/accept",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
	))
	require.Equal(t, "accepted", accepted["status"])
	require.Equal(t, payroll.TSDAccepted, repo.tsdDeclarations["tsd-1"].Status)

	rejected := invokePayrollImportJSON[map[string]string](t, http.StatusOK, h.MarkTSDRejected, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tsd/2026/3/reject",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
	))
	require.Equal(t, "rejected", rejected["status"])
	require.Equal(t, payroll.TSDRejected, repo.tsdDeclarations["tsd-1"].Status)

	errorBody := invokePayrollImportJSON[map[string]string](t, http.StatusBadRequest, h.ListTSD, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tsd?month=13",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "invalid month", errorBody["error"])
}

func TestLeaveBusinessHandlersBalanceAndRecordWorkflow(t *testing.T) {
	h, _, absenceRepo := setupPayrollImportHandlerTest(t)
	employee := payrollImportEmployee("emp-700", "EMP-700")
	absenceRepo.Employees[employee.ID] = employee
	absenceRepo.AbsenceTypes["type-annual"] = &payroll.AbsenceType{
		ID:                 "type-annual",
		TenantID:           "tenant-1",
		Code:               "ANNUAL_LEAVE",
		Name:               "Annual leave",
		DefaultDaysPerYear: decimal.NewFromInt(28),
		IsActive:           true,
	}
	absenceRepo.LeaveBalances["tenant-1-emp-700-type-annual-2026"] = &payroll.LeaveBalance{
		ID:            "balance-2026",
		TenantID:      "tenant-1",
		EmployeeID:    employee.ID,
		AbsenceTypeID: "type-annual",
		Year:          2026,
		EntitledDays:  decimal.NewFromInt(28),
		CarryoverDays: decimal.NewFromInt(2),
		UsedDays:      decimal.NewFromInt(4),
		PendingDays:   decimal.Zero,
		RemainingDays: decimal.NewFromInt(26),
	}

	types := invokePayrollImportJSON[[]payroll.AbsenceType](t, http.StatusOK, h.ListAbsenceTypes, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/absence-types?active_only=true",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, types, 1)
	require.Equal(t, "ANNUAL_LEAVE", types[0].Code)

	absenceType := invokePayrollImportJSON[payroll.AbsenceType](t, http.StatusOK, h.GetAbsenceType, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/absence-types/type-annual",
		nil,
		map[string]string{"tenantID": "tenant-1", "typeID": "type-annual"},
	))
	require.Equal(t, "Annual leave", absenceType.Name)

	balances := invokePayrollImportJSON[[]payroll.LeaveBalance](t, http.StatusOK, h.ListLeaveBalances, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/employees/emp-700/leave-balances?year=2026",
		nil,
		map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID},
	))
	require.Len(t, balances, 1)
	require.True(t, balances[0].RemainingDays.Equal(decimal.NewFromInt(26)))

	balancesByYear := invokePayrollImportJSON[[]payroll.LeaveBalance](t, http.StatusOK, h.GetLeaveBalancesByYear, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/employees/emp-700/leave-balances/2026",
		nil,
		map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID, "year": "2026"},
	))
	require.Len(t, balancesByYear, 1)

	entitled := decimal.NewFromInt(30)
	carryover := decimal.NewFromInt(3)
	updated := invokePayrollImportJSON[payroll.LeaveBalance](t, http.StatusOK, h.UpdateLeaveBalance, payrollHandlerRequest(
		http.MethodPut,
		"/tenants/tenant-1/employees/emp-700/leave-balances/2026/type-annual",
		payroll.UpdateLeaveBalanceRequest{
			EntitledDays:  &entitled,
			CarryoverDays: &carryover,
			Notes:         "Adjusted entitlement",
		},
		map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID, "year": "2026", "typeID": "type-annual"},
	))
	require.True(t, updated.RemainingDays.Equal(decimal.NewFromInt(29)))
	require.Equal(t, "Adjusted entitlement", updated.Notes)

	initialized := invokePayrollImportJSON[[]payroll.LeaveBalance](t, http.StatusOK, h.InitializeLeaveBalances, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/employees/emp-700/leave-balances/2027/initialize",
		nil,
		map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID, "year": "2027"},
	))
	require.Len(t, initialized, 1)
	require.Equal(t, "leave-1", initialized[0].ID)
	require.True(t, initialized[0].RemainingDays.Equal(decimal.NewFromInt(28)))

	start := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	createdRecord := invokePayrollImportJSON[payroll.LeaveRecord](t, http.StatusCreated, h.CreateLeaveRecord, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/leave-records",
		payroll.CreateLeaveRecordRequest{
			EmployeeID:    employee.ID,
			AbsenceTypeID: "type-annual",
			StartDate:     start,
			EndDate:       end,
			TotalDays:     decimal.NewFromInt(2),
			WorkingDays:   decimal.NewFromInt(2),
			Notes:         "Summer leave",
		},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "leave-2", createdRecord.ID)
	require.Equal(t, payroll.LeavePending, createdRecord.Status)
	require.Equal(t, "user-1", createdRecord.RequestedBy)
	require.True(t, absenceRepo.LeaveBalances["tenant-1-emp-700-type-annual-2026"].PendingDays.Equal(decimal.NewFromInt(2)))

	records := invokePayrollImportJSON[[]payroll.LeaveRecord](t, http.StatusOK, h.ListLeaveRecords, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/leave-records?employee_id=emp-700&year=2026",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, records, 1)
	require.Equal(t, createdRecord.ID, records[0].ID)

	gotRecord := invokePayrollImportJSON[payroll.LeaveRecord](t, http.StatusOK, h.GetLeaveRecord, payrollHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/leave-records/"+createdRecord.ID,
		nil,
		map[string]string{"tenantID": "tenant-1", "recordID": createdRecord.ID},
	))
	require.Equal(t, "Summer leave", gotRecord.Notes)

	approved := invokePayrollImportJSON[payroll.LeaveRecord](t, http.StatusOK, h.ApproveLeaveRecord, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/leave-records/"+createdRecord.ID+"/approve",
		nil,
		map[string]string{"tenantID": "tenant-1", "recordID": createdRecord.ID},
	))
	require.Equal(t, payroll.LeaveApproved, approved.Status)
	require.Equal(t, "user-1", approved.ApprovedBy)
	require.True(t, absenceRepo.LeaveBalances["tenant-1-emp-700-type-annual-2026"].PendingDays.IsZero())
	require.True(t, absenceRepo.LeaveBalances["tenant-1-emp-700-type-annual-2026"].UsedDays.Equal(decimal.NewFromInt(6)))

	canceled := invokePayrollImportJSON[payroll.LeaveRecord](t, http.StatusOK, h.CancelLeaveRecord, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/leave-records/"+createdRecord.ID+"/cancel",
		nil,
		map[string]string{"tenantID": "tenant-1", "recordID": createdRecord.ID},
	))
	require.Equal(t, payroll.LeaveCanceled, canceled.Status)
	require.True(t, absenceRepo.LeaveBalances["tenant-1-emp-700-type-annual-2026"].UsedDays.Equal(decimal.NewFromInt(4)))

	rejectRecord := invokePayrollImportJSON[payroll.LeaveRecord](t, http.StatusCreated, h.CreateLeaveRecord, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/leave-records",
		payroll.CreateLeaveRecordRequest{
			EmployeeID:    employee.ID,
			AbsenceTypeID: "type-annual",
			StartDate:     time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			EndDate:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			TotalDays:     decimal.NewFromInt(1),
			WorkingDays:   decimal.NewFromInt(1),
		},
		map[string]string{"tenantID": "tenant-1"},
	))
	rejected := invokePayrollImportJSON[payroll.LeaveRecord](t, http.StatusOK, h.RejectLeaveRecord, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/leave-records/"+rejectRecord.ID+"/reject",
		payroll.RejectLeaveRequest{Reason: "Coverage conflict"},
		map[string]string{"tenantID": "tenant-1", "recordID": rejectRecord.ID},
	))
	require.Equal(t, payroll.LeaveRejected, rejected.Status)
	require.Equal(t, "user-1", rejected.RejectedBy)
	require.Equal(t, "Coverage conflict", rejected.RejectionReason)
	require.True(t, absenceRepo.LeaveBalances["tenant-1-emp-700-type-annual-2026"].PendingDays.IsZero())
}

func setupPayrollImportHandlerTest(t *testing.T) (*Handlers, *payrollImportHandlerRepository, *payroll.MockAbsenceRepository) {
	t.Helper()

	h, tenantRepo := setupTenantTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")

	repo := newPayrollImportHandlerRepository()
	h.payrollService = payroll.NewServiceWithRepository(repo, &payrollImportHandlerIDGenerator{prefix: "payroll"})

	absenceRepo := payroll.NewMockAbsenceRepository()
	h.absenceService = payroll.NewAbsenceService(absenceRepo, &payrollImportHandlerIDGenerator{prefix: "leave"})

	return h, repo, absenceRepo
}

func payrollImportRequest(method, path string, body any) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, createTestClaims("user-1", "user@example.com", "tenant-1", "owner"))
	return withURLParams(req, map[string]string{"tenantID": "tenant-1"})
}

func payrollHandlerRequest(method, path string, body any, params map[string]string) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, createTestClaims("user-1", "user@example.com", "tenant-1", "owner"))
	return withURLParams(req, params)
}

func invokePayrollImportJSON[T any](t *testing.T, wantStatus int, handler func(http.ResponseWriter, *http.Request), req *http.Request) T {
	t.Helper()

	rec := httptest.NewRecorder()
	handler(rec, req)
	require.Equal(t, wantStatus, rec.Code, rec.Body.String())

	var result T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	return result
}

func payrollImportHandlerByName(h *Handlers, name string) func(http.ResponseWriter, *http.Request) {
	switch name {
	case "employees":
		return h.ImportEmployees
	case "payroll history":
		return h.ImportPayrollHistory
	case "TSD history":
		return h.ImportTSDHistory
	case "leave balances":
		return h.ImportLeaveBalances
	default:
		panic("unknown payroll import handler")
	}
}

func payrollImportEmployee(id, number string) *payroll.Employee {
	return &payroll.Employee{
		ID:             id,
		TenantID:       "tenant-1",
		EmployeeNumber: number,
		FirstName:      "Mari",
		LastName:       "Maasikas",
		PersonalCode:   "49001010001",
		Email:          number + "@example.com",
		StartDate:      time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		IsActive:       true,
	}
}

type payrollImportHandlerIDGenerator struct {
	prefix  string
	counter int
}

func (g *payrollImportHandlerIDGenerator) New() string {
	g.counter++
	return fmt.Sprintf("%s-%d", g.prefix, g.counter)
}

type payrollImportHandlerRepository struct {
	employees        map[string]*payroll.Employee
	salaryComponents []payroll.SalaryComponent
	payrollRuns      map[string]*payroll.PayrollRun
	payslips         []payroll.Payslip
	tsdDeclarations  map[string]*payroll.TSDDeclaration
	tsdRows          []payroll.TSDRow
}

func newPayrollImportHandlerRepository() *payrollImportHandlerRepository {
	return &payrollImportHandlerRepository{
		employees:       map[string]*payroll.Employee{},
		payrollRuns:     map[string]*payroll.PayrollRun{},
		tsdDeclarations: map[string]*payroll.TSDDeclaration{},
	}
}

func (r *payrollImportHandlerRepository) seedEmployee(employee *payroll.Employee) {
	r.employees[employee.ID] = employee
}

func (r *payrollImportHandlerRepository) CreateEmployee(ctx context.Context, schemaName string, emp *payroll.Employee) error {
	r.employees[emp.ID] = emp
	return nil
}

func (r *payrollImportHandlerRepository) GetEmployee(ctx context.Context, schemaName, tenantID, employeeID string) (*payroll.Employee, error) {
	emp, ok := r.employees[employeeID]
	if !ok {
		return nil, payroll.ErrEmployeeNotFound
	}
	return emp, nil
}

func (r *payrollImportHandlerRepository) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]payroll.Employee, error) {
	employees := make([]payroll.Employee, 0, len(r.employees))
	for _, emp := range r.employees {
		if emp.TenantID == tenantID && (!activeOnly || emp.IsActive) {
			employees = append(employees, *emp)
		}
	}
	return employees, nil
}

func (r *payrollImportHandlerRepository) UpdateEmployee(ctx context.Context, schemaName string, emp *payroll.Employee) error {
	r.employees[emp.ID] = emp
	return nil
}

func (r *payrollImportHandlerRepository) EndCurrentBaseSalary(ctx context.Context, schemaName, tenantID, employeeID string, effectiveTo time.Time) error {
	return nil
}

func (r *payrollImportHandlerRepository) CreateSalaryComponent(ctx context.Context, schemaName string, comp *payroll.SalaryComponent) error {
	r.salaryComponents = append(r.salaryComponents, *comp)
	return nil
}

func (r *payrollImportHandlerRepository) ListSalaryComponents(ctx context.Context, schemaName, tenantID, employeeID string, activeOn *time.Time) ([]payroll.SalaryComponent, error) {
	components := make([]payroll.SalaryComponent, 0)
	for _, comp := range r.salaryComponents {
		if comp.TenantID == tenantID && comp.EmployeeID == employeeID {
			components = append(components, comp)
		}
	}
	return components, nil
}

func (r *payrollImportHandlerRepository) GetCurrentSalary(ctx context.Context, schemaName, tenantID, employeeID string) (decimal.Decimal, error) {
	total := decimal.Zero
	for _, comp := range r.salaryComponents {
		if comp.TenantID == tenantID && comp.EmployeeID == employeeID {
			total = total.Add(comp.Amount)
		}
	}
	return total, nil
}

func (r *payrollImportHandlerRepository) CreatePayrollRun(ctx context.Context, schemaName string, run *payroll.PayrollRun) error {
	r.payrollRuns[run.ID] = run
	return nil
}

func (r *payrollImportHandlerRepository) GetPayrollRun(ctx context.Context, schemaName, tenantID, runID string) (*payroll.PayrollRun, error) {
	run, ok := r.payrollRuns[runID]
	if !ok {
		return nil, payroll.ErrPayrollRunNotFound
	}
	return run, nil
}

func (r *payrollImportHandlerRepository) ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]payroll.PayrollRun, error) {
	runs := make([]payroll.PayrollRun, 0, len(r.payrollRuns))
	for _, run := range r.payrollRuns {
		if run.TenantID == tenantID && (year == 0 || run.PeriodYear == year) {
			runs = append(runs, *run)
		}
	}
	return runs, nil
}

func (r *payrollImportHandlerRepository) UpdatePayrollRun(ctx context.Context, schemaName string, run *payroll.PayrollRun) error {
	r.payrollRuns[run.ID] = run
	return nil
}

func (r *payrollImportHandlerRepository) ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error {
	return nil
}

func (r *payrollImportHandlerRepository) DeletePayslipsByRunID(ctx context.Context, schemaName, runID string) error {
	payslips := r.payslips[:0]
	for _, payslip := range r.payslips {
		if payslip.PayrollRunID != runID {
			payslips = append(payslips, payslip)
		}
	}
	r.payslips = payslips
	return nil
}

func (r *payrollImportHandlerRepository) CreatePayslip(ctx context.Context, schemaName string, payslip *payroll.Payslip) error {
	r.payslips = append(r.payslips, *payslip)
	return nil
}

func (r *payrollImportHandlerRepository) GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]payroll.Payslip, error) {
	payslips := make([]payroll.Payslip, 0)
	for _, payslip := range r.payslips {
		if payslip.TenantID == tenantID && payslip.PayrollRunID == payrollRunID {
			if payslip.Employee == nil {
				payslip.Employee = r.employees[payslip.EmployeeID]
			}
			payslips = append(payslips, payslip)
		}
	}
	return payslips, nil
}

func (r *payrollImportHandlerRepository) DeleteTSDByPeriod(ctx context.Context, schemaName, tenantID string, year, month int) error {
	for id, declaration := range r.tsdDeclarations {
		if declaration.TenantID == tenantID && declaration.PeriodYear == year && declaration.PeriodMonth == month {
			delete(r.tsdDeclarations, id)
		}
	}
	return nil
}

func (r *payrollImportHandlerRepository) CreateTSDDeclaration(ctx context.Context, schemaName string, declaration *payroll.TSDDeclaration) error {
	r.tsdDeclarations[declaration.ID] = declaration
	return nil
}

func (r *payrollImportHandlerRepository) CreateTSDRows(ctx context.Context, schemaName string, rows []payroll.TSDRow) error {
	r.tsdRows = append(r.tsdRows, rows...)
	return nil
}

func (r *payrollImportHandlerRepository) GetTSD(ctx context.Context, schemaName, tenantID string, year, month int) (*payroll.TSDDeclaration, error) {
	for _, declaration := range r.tsdDeclarations {
		if declaration.TenantID == tenantID && declaration.PeriodYear == year && declaration.PeriodMonth == month {
			return declaration, nil
		}
	}
	return nil, payroll.ErrTSDDeclarationNotFound
}

func (r *payrollImportHandlerRepository) GetTSDRows(ctx context.Context, schemaName, tenantID, declarationID string) ([]payroll.TSDRow, error) {
	rows := make([]payroll.TSDRow, 0)
	for _, row := range r.tsdRows {
		if row.TenantID == tenantID && row.DeclarationID == declarationID {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func (r *payrollImportHandlerRepository) ListTSD(ctx context.Context, schemaName, tenantID string, filter payroll.TSDListFilter) ([]payroll.TSDDeclaration, error) {
	declarations := make([]payroll.TSDDeclaration, 0, len(r.tsdDeclarations))
	for _, declaration := range r.tsdDeclarations {
		if declaration.TenantID == tenantID && (filter.Year == 0 || declaration.PeriodYear == filter.Year) && (filter.Month == 0 || declaration.PeriodMonth == filter.Month) {
			declarations = append(declarations, *declaration)
		}
	}
	return declarations, nil
}

func (r *payrollImportHandlerRepository) MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string, submittedAt time.Time) error {
	if declaration, ok := r.tsdDeclarations[declarationID]; ok {
		declaration.Status = payroll.TSDSubmitted
		declaration.EMTAReference = emtaReference
		declaration.SubmittedAt = &submittedAt
	}
	return nil
}

func (r *payrollImportHandlerRepository) UpdateTSDStatus(ctx context.Context, schemaName, tenantID, declarationID string, status payroll.TSDStatus, updatedAt time.Time) error {
	if declaration, ok := r.tsdDeclarations[declarationID]; ok {
		declaration.Status = status
	}
	return nil
}

func (r *payrollImportHandlerRepository) WithTransaction(ctx context.Context, fn func(txRepo payroll.Repository) error) error {
	return fn(r)
}
