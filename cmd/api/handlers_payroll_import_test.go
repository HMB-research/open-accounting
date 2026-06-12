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
