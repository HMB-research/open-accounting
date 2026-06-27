package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type wave5PayrollRepository struct {
	*payrollImportHandlerRepository

	listEmployeesErr         error
	createEmployeeErr        error
	updateEmployeeErr        error
	createSalaryComponentErr error
	listSalaryComponentsErr  error
	createPayrollRunErr      error
	listPayrollRunsErr       error
	updatePayrollRunErr      error
	approvePayrollRunErr     error
	getPayslipsErr           error
	listTSDErr               error
	markSubmittedErr         error
	updateTSDStatusErr       error
	createTSDRowsErr         error
}

func (r *wave5PayrollRepository) ListEmployees(ctx context.Context, schemaName, tenantID string, activeOnly bool) ([]payroll.Employee, error) {
	if r.listEmployeesErr != nil {
		return nil, r.listEmployeesErr
	}
	return r.payrollImportHandlerRepository.ListEmployees(ctx, schemaName, tenantID, activeOnly)
}

func (r *wave5PayrollRepository) CreateEmployee(ctx context.Context, schemaName string, emp *payroll.Employee) error {
	if r.createEmployeeErr != nil {
		return r.createEmployeeErr
	}
	return r.payrollImportHandlerRepository.CreateEmployee(ctx, schemaName, emp)
}

func (r *wave5PayrollRepository) UpdateEmployee(ctx context.Context, schemaName string, emp *payroll.Employee) error {
	if r.updateEmployeeErr != nil {
		return r.updateEmployeeErr
	}
	return r.payrollImportHandlerRepository.UpdateEmployee(ctx, schemaName, emp)
}

func (r *wave5PayrollRepository) CreateSalaryComponent(ctx context.Context, schemaName string, comp *payroll.SalaryComponent) error {
	if r.createSalaryComponentErr != nil {
		return r.createSalaryComponentErr
	}
	return r.payrollImportHandlerRepository.CreateSalaryComponent(ctx, schemaName, comp)
}

func (r *wave5PayrollRepository) ListSalaryComponents(ctx context.Context, schemaName, tenantID, employeeID string, activeOn *time.Time) ([]payroll.SalaryComponent, error) {
	if r.listSalaryComponentsErr != nil {
		return nil, r.listSalaryComponentsErr
	}
	return r.payrollImportHandlerRepository.ListSalaryComponents(ctx, schemaName, tenantID, employeeID, activeOn)
}

func (r *wave5PayrollRepository) CreatePayrollRun(ctx context.Context, schemaName string, run *payroll.PayrollRun) error {
	if r.createPayrollRunErr != nil {
		return r.createPayrollRunErr
	}
	return r.payrollImportHandlerRepository.CreatePayrollRun(ctx, schemaName, run)
}

func (r *wave5PayrollRepository) ListPayrollRuns(ctx context.Context, schemaName, tenantID string, year int) ([]payroll.PayrollRun, error) {
	if r.listPayrollRunsErr != nil {
		return nil, r.listPayrollRunsErr
	}
	return r.payrollImportHandlerRepository.ListPayrollRuns(ctx, schemaName, tenantID, year)
}

func (r *wave5PayrollRepository) UpdatePayrollRun(ctx context.Context, schemaName string, run *payroll.PayrollRun) error {
	if r.updatePayrollRunErr != nil {
		return r.updatePayrollRunErr
	}
	return r.payrollImportHandlerRepository.UpdatePayrollRun(ctx, schemaName, run)
}

func (r *wave5PayrollRepository) ApprovePayrollRun(ctx context.Context, schemaName, tenantID, runID, approverID string) error {
	if r.approvePayrollRunErr != nil {
		return r.approvePayrollRunErr
	}
	return r.payrollImportHandlerRepository.ApprovePayrollRun(ctx, schemaName, tenantID, runID, approverID)
}

func (r *wave5PayrollRepository) GetPayslipsWithEmployees(ctx context.Context, schemaName, tenantID, payrollRunID string) ([]payroll.Payslip, error) {
	if r.getPayslipsErr != nil {
		return nil, r.getPayslipsErr
	}
	return r.payrollImportHandlerRepository.GetPayslipsWithEmployees(ctx, schemaName, tenantID, payrollRunID)
}

func (r *wave5PayrollRepository) CreateTSDRows(ctx context.Context, schemaName string, rows []payroll.TSDRow) error {
	if r.createTSDRowsErr != nil {
		return r.createTSDRowsErr
	}
	return r.payrollImportHandlerRepository.CreateTSDRows(ctx, schemaName, rows)
}

func (r *wave5PayrollRepository) ListTSD(ctx context.Context, schemaName, tenantID string, filter payroll.TSDListFilter) ([]payroll.TSDDeclaration, error) {
	if r.listTSDErr != nil {
		return nil, r.listTSDErr
	}
	return r.payrollImportHandlerRepository.ListTSD(ctx, schemaName, tenantID, filter)
}

func (r *wave5PayrollRepository) MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string, submittedAt time.Time) error {
	if r.markSubmittedErr != nil {
		return r.markSubmittedErr
	}
	return r.payrollImportHandlerRepository.MarkTSDSubmitted(ctx, schemaName, tenantID, declarationID, emtaReference, submittedAt)
}

func (r *wave5PayrollRepository) UpdateTSDStatus(ctx context.Context, schemaName, tenantID, declarationID string, status payroll.TSDStatus, updatedAt time.Time) error {
	if r.updateTSDStatusErr != nil {
		return r.updateTSDStatusErr
	}
	return r.payrollImportHandlerRepository.UpdateTSDStatus(ctx, schemaName, tenantID, declarationID, status, updatedAt)
}

func (r *wave5PayrollRepository) WithTransaction(ctx context.Context, fn func(txRepo payroll.Repository) error) error {
	return fn(r)
}

func setupWave5PayrollHandlers(t *testing.T) (*Handlers, *wave5PayrollRepository, *payroll.MockAbsenceRepository) {
	t.Helper()

	h, baseRepo, absenceRepo := setupPayrollImportHandlerTest(t)
	repo := &wave5PayrollRepository{payrollImportHandlerRepository: baseRepo}
	h.payrollService = payroll.NewServiceWithRepository(repo, &payrollImportHandlerIDGenerator{prefix: "wave5-payroll"})
	return h, repo, absenceRepo
}

func wave5PayrollInventoryRawRequest(method, path, body string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParams(req, params)
	return req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "user@example.com", "tenant-1", "owner")))
}

func wave5InventoryUnauthenticatedRequest(method, path, body string, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return withURLParams(req, params)
}

func TestWave5PayrollEmployeeSalaryRunBranches(t *testing.T) {
	t.Run("list employees repository error", func(t *testing.T) {
		h, repo, _ := setupWave5PayrollHandlers(t)
		repo.listEmployeesErr = errors.New("employee store unavailable")

		rec := invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListEmployees, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/employees?active_only=true",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list employees")
	})

	t.Run("create and update employee request errors", func(t *testing.T) {
		h, _, _ := setupWave5PayrollHandlers(t)

		rec := invokePayrollImportRaw(t, http.StatusBadRequest, h.CreateEmployee, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CreateEmployee, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees",
			payroll.CreateEmployeeRequest{FirstName: "Mari"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "first name and last name are required")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetEmployee, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/employees/missing",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Employee not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdateEmployee, wave5PayrollInventoryRawRequest(
			http.MethodPut,
			"/tenants/tenant-1/employees/missing",
			"{",
			map[string]string{"tenantID": "tenant-1", "employeeID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdateEmployee, payrollHandlerRequest(
			http.MethodPut,
			"/tenants/tenant-1/employees/missing",
			payroll.UpdateEmployeeRequest{FirstName: "Updated"},
			map[string]string{"tenantID": "tenant-1", "employeeID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "employee not found")
	})

	t.Run("salary handlers validation and service errors", func(t *testing.T) {
		h, repo, _ := setupWave5PayrollHandlers(t)
		employee := payrollImportEmployee("emp-wave5", "EMP-W5")
		repo.seedEmployee(employee)

		rec := invokePayrollImportRaw(t, http.StatusBadRequest, h.SetBaseSalary, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/emp-wave5/salary",
			"{",
			map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.SetBaseSalary, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/emp-wave5/salary",
			map[string]string{"amount": "0"},
			map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID},
		))
		assert.Contains(t, rec.Body.String(), "Amount is required")

		rec = invokePayrollImportRaw(t, http.StatusOK, h.SetBaseSalary, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/emp-wave5/salary",
			map[string]string{"amount": "2500.00"},
			map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID},
		))
		assert.Contains(t, rec.Body.String(), "salary updated")

		repo.createSalaryComponentErr = errors.New("salary component insert failed")
		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.SetBaseSalary, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/emp-wave5/salary",
			map[string]string{"amount": "2600.00"},
			map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID},
		))
		assert.Contains(t, rec.Body.String(), "salary component insert failed")
		repo.createSalaryComponentErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ListSalaryComponents, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/employees/emp-wave5/salary-components?active_on=31-03-2026",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID},
		))
		assert.Contains(t, rec.Body.String(), "Invalid active_on date")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ListSalaryComponents, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/employees/missing/salary-components",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "employee not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.AddSalaryComponent, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/emp-wave5/salary-components",
			"{",
			map[string]string{"tenantID": "tenant-1", "employeeID": employee.ID},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.AddSalaryComponent, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/missing/salary-components",
			payroll.CreateSalaryComponentRequest{
				ComponentType: payroll.SalaryComponentBonus,
				Amount:        decimal.NewFromInt(100),
				EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			map[string]string{"tenantID": "tenant-1", "employeeID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "employee not found")
	})

	t.Run("payroll run handlers validation and service errors", func(t *testing.T) {
		h, repo, _ := setupWave5PayrollHandlers(t)

		repo.listPayrollRunsErr = errors.New("run list unavailable")
		rec := invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListPayrollRuns, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/payroll-runs?year=bad",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list payroll runs")
		repo.listPayrollRunsErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CreatePayrollRun, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CreatePayrollRun, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs",
			payroll.CreatePayrollRunRequest{PeriodYear: 2026, PeriodMonth: 13},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "invalid period month")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetPayrollRun, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/payroll-runs/missing",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Payroll run not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdatePayrollRunPaymentDate, wave5PayrollInventoryRawRequest(
			http.MethodPatch,
			"/tenants/tenant-1/payroll-runs/missing/payment-date",
			"{",
			map[string]string{"tenantID": "tenant-1", "runID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdatePayrollRunPaymentDate, payrollHandlerRequest(
			http.MethodPatch,
			"/tenants/tenant-1/payroll-runs/missing/payment-date",
			payroll.UpdatePayrollRunPaymentDateRequest{PaymentDate: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)},
			map[string]string{"tenantID": "tenant-1", "runID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "payroll run not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CalculatePayroll, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs/missing/calculate",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "payroll run not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ProcessPayrollRun, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs/missing/process",
			"{",
			map[string]string{"tenantID": "tenant-1", "runID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ProcessPayrollRun, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs/missing/process",
			payroll.ProcessPayrollRunRequest{},
			map[string]string{"tenantID": "tenant-1", "runID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "payroll run not found")

		repo.approvePayrollRunErr = errors.New("approval store unavailable")
		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ApprovePayroll, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs/run-1/approve",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
		))
		assert.Contains(t, rec.Body.String(), "approval store unavailable")
		repo.approvePayrollRunErr = nil

		repo.getPayslipsErr = errors.New("payslip list unavailable")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetPayslips, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/payroll-runs/run-1/payslips",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "run-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get payslips")
		repo.getPayslipsErr = nil
	})

	t.Run("payslip pdf and tax preview branch errors", func(t *testing.T) {
		h, repo, _ := setupWave5PayrollHandlers(t)
		repo.payrollRuns["run-1"] = &payroll.PayrollRun{
			ID:          "run-1",
			TenantID:    "tenant-1",
			PeriodYear:  2026,
			PeriodMonth: 6,
		}

		rec := invokePayrollImportRaw(t, http.StatusNotFound, h.GetPayslipPDF, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/payroll-runs/missing/payslips/payslip-1/pdf",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "missing", "payslipID": "payslip-1"},
		))
		assert.Contains(t, rec.Body.String(), "Payroll run not found")

		repo.getPayslipsErr = errors.New("payslip load failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetPayslipPDF, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/payroll-runs/run-1/payslips/payslip-1/pdf",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "run-1", "payslipID": "payslip-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get payslips")
		repo.getPayslipsErr = nil

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetPayslipPDF, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/payroll-runs/run-1/payslips/missing/pdf",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "run-1", "payslipID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Payslip not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CalculateTaxPreview, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll/tax-preview",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CalculateTaxPreview, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll/tax-preview",
			map[string]string{"gross_salary": "-1"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Gross salary must be positive")

		rec = invokePayrollImportRaw(t, http.StatusOK, h.CalculateTaxPreview, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll/tax-preview",
			map[string]any{
				"gross_salary":          "2000.00",
				"apply_basic_exemption": true,
			},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "gross_salary")

		rec = invokePayrollImportRaw(t, http.StatusOK, h.CalculateTaxPreview, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll/tax-preview",
			map[string]any{
				"gross_salary":        "2000.00",
				"basic_exemption":     "500.00",
				"funded_pension_rate": "0.02",
			},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "income_tax")
	})
}

func TestWave5TSDStatusAndExportBranches(t *testing.T) {
	t.Run("generate and lookup validation errors", func(t *testing.T) {
		h, repo, _ := setupWave5PayrollHandlers(t)

		rec := invokePayrollImportRaw(t, http.StatusBadRequest, h.GenerateTSD, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/payroll-runs/missing/tsd",
			nil,
			map[string]string{"tenantID": "tenant-1", "runID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "payroll run not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetTSD, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/tsd/year/3",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "year", "month": "3"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid year")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetTSD, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/tsd/2026/month",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "month"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid month")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetTSD, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/tsd/2026/3",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
		))
		assert.Contains(t, rec.Body.String(), "TSD declaration not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ListTSD, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/tsd?year=bad",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "invalid year")

		repo.listTSDErr = errors.New("tsd list unavailable")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListTSD, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/tsd",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list TSD declarations")
	})

	t.Run("export validation and service errors", func(t *testing.T) {
		h, _, _ := setupWave5PayrollHandlers(t)

		exportCases := []struct {
			name    string
			handler func(http.ResponseWriter, *http.Request)
			path    string
			params  map[string]string
			want    int
			text    string
		}{
			{
				name:    "xml invalid year",
				handler: h.ExportTSDXML,
				path:    "/tenants/tenant-1/tsd/year/3/xml",
				params:  map[string]string{"tenantID": "tenant-1", "year": "year", "month": "3"},
				want:    http.StatusBadRequest,
				text:    "Invalid year",
			},
			{
				name:    "xml invalid month",
				handler: h.ExportTSDXML,
				path:    "/tenants/tenant-1/tsd/2026/month/xml",
				params:  map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "month"},
				want:    http.StatusBadRequest,
				text:    "Invalid month",
			},
			{
				name:    "xml tenant missing",
				handler: h.ExportTSDXML,
				path:    "/tenants/missing/tsd/2026/3/xml",
				params:  map[string]string{"tenantID": "missing", "year": "2026", "month": "3"},
				want:    http.StatusNotFound,
				text:    "Tenant not found",
			},
			{
				name:    "xml declaration missing",
				handler: h.ExportTSDXML,
				path:    "/tenants/tenant-1/tsd/2026/3/xml",
				params:  map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
				want:    http.StatusInternalServerError,
				text:    "TSD declaration not found",
			},
			{
				name:    "csv invalid year",
				handler: h.ExportTSDCSV,
				path:    "/tenants/tenant-1/tsd/year/3/csv",
				params:  map[string]string{"tenantID": "tenant-1", "year": "year", "month": "3"},
				want:    http.StatusBadRequest,
				text:    "Invalid year",
			},
			{
				name:    "csv invalid month",
				handler: h.ExportTSDCSV,
				path:    "/tenants/tenant-1/tsd/2026/month/csv",
				params:  map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "month"},
				want:    http.StatusBadRequest,
				text:    "Invalid month",
			},
			{
				name:    "csv tenant missing",
				handler: h.ExportTSDCSV,
				path:    "/tenants/missing/tsd/2026/3/csv",
				params:  map[string]string{"tenantID": "missing", "year": "2026", "month": "3"},
				want:    http.StatusNotFound,
				text:    "Tenant not found",
			},
			{
				name:    "csv declaration missing",
				handler: h.ExportTSDCSV,
				path:    "/tenants/tenant-1/tsd/2026/3/csv",
				params:  map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "3"},
				want:    http.StatusInternalServerError,
				text:    "TSD declaration not found",
			},
		}

		for _, tc := range exportCases {
			t.Run(tc.name, func(t *testing.T) {
				rec := invokePayrollImportRaw(t, tc.want, tc.handler, payrollHandlerRequest(
					http.MethodGet,
					tc.path,
					nil,
					tc.params,
				))
				assert.Contains(t, rec.Body.String(), tc.text)
			})
		}
	})

	t.Run("status validation and evidence errors", func(t *testing.T) {
		h, repo, _ := setupWave5PayrollHandlers(t)
		repo.tsdDeclarations["tsd-wave5"] = &payroll.TSDDeclaration{
			ID:            "tsd-wave5",
			TenantID:      "tenant-1",
			PeriodYear:    2026,
			PeriodMonth:   6,
			Status:        payroll.TSDDraft,
			TotalPayments: decimal.NewFromInt(1000),
		}

		rec := invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDSubmitted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/year/6/submit",
			map[string]string{"emta_reference": "EMTA-1"},
			map[string]string{"tenantID": "tenant-1", "year": "year", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid year")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDSubmitted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/month/submit",
			map[string]string{"emta_reference": "EMTA-1"},
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "month"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid month")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDSubmitted, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/6/submit",
			"{",
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.MarkTSDSubmitted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/7/submit",
			map[string]string{"emta_reference": "EMTA-1"},
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "7"},
		))
		assert.Contains(t, rec.Body.String(), "TSD declaration not found")

		docRepo := newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("documents unavailable")
		h.documentsService = documents.NewService(docRepo, nil)
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.MarkTSDSubmitted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/6/submit",
			map[string]string{"emta_reference": "EMTA-1"},
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to verify TSD submission evidence")

		installApprovedEvidenceDocuments(t, h, documents.Document{
			EntityType:   documents.EntityTypeTSD,
			EntityID:     "tsd-wave5",
			DocumentType: documents.DocumentTypeTaxSupport,
		})
		repo.markSubmittedErr = errors.New("submit status write failed")
		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDSubmitted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/6/submit",
			map[string]string{"emta_reference": "EMTA-1"},
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "submit status write failed")
		repo.markSubmittedErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDAccepted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/year/6/accept",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "year", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid year")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDAccepted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/month/accept",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "month"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid month")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.MarkTSDAccepted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/7/accept",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "7"},
		))
		assert.Contains(t, rec.Body.String(), "TSD declaration not found")

		docRepo = newMockDocumentRepository()
		docRepo.listDocumentsErr = errors.New("documents unavailable")
		h.documentsService = documents.NewService(docRepo, nil)
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.MarkTSDAccepted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/6/accept",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to verify TSD acceptance evidence")

		installApprovedEvidenceDocuments(t, h, documents.Document{
			EntityType:   documents.EntityTypeTSD,
			EntityID:     "tsd-wave5",
			DocumentType: documents.DocumentTypeTaxSupport,
		})
		repo.updateTSDStatusErr = errors.New("status write failed")
		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDAccepted, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/6/accept",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "status write failed")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.MarkTSDRejected, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/6/reject",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "status write failed")
		repo.updateTSDStatusErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, func(w http.ResponseWriter, r *http.Request) {
			h.markTSDStatusByPeriod(w, r, payroll.TSDStatus("ARCHIVED"), "archived")
		}, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/tsd/2026/6/archive",
			nil,
			map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "6"},
		))
		assert.Contains(t, rec.Body.String(), "Unsupported TSD status")
	})
}

func TestWave5InventoryMasterDataBranches(t *testing.T) {
	t.Run("product category error branches", func(t *testing.T) {
		h, repo, tenantRepo := setupInventoryTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

		repo.listCategoriesErr = errors.New("category list failed")
		rec := invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListProductCategories, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/product-categories",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list categories")
		repo.listCategoriesErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CreateProductCategory, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/product-categories",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		repo.createCategoryErr = errors.New("category insert failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.CreateProductCategory, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/product-categories",
			inventory.CreateCategoryRequest{Name: "Retail"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to create category")
		repo.createCategoryErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportProductCategories, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/product-categories/import",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportProductCategories, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/product-categories/import",
			inventory.ImportProductCategoriesRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "csv_content is required")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportProductCategories, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/product-categories/import",
			inventory.ImportProductCategoriesRequest{CSVContent: "code\nA"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "missing required columns: name")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetProductCategory, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/product-categories/missing",
			nil,
			map[string]string{"tenantID": "tenant-1", "categoryID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Category not found")

		repo.deleteCategoryErr = errors.New("delete failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.DeleteProductCategory, newInventoryJSONRequest(t,
			http.MethodDelete,
			"/tenants/tenant-1/product-categories/cat-1",
			nil,
			map[string]string{"tenantID": "tenant-1", "categoryID": "cat-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to delete category")
	})

	t.Run("product error branches", func(t *testing.T) {
		h, repo, tenantRepo := setupInventoryTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
		repo.products["prod-1"] = &inventory.Product{
			ID:          "prod-1",
			TenantID:    "tenant-1",
			Code:        "SKU-001",
			Name:        "Widget",
			ProductType: inventory.ProductTypeGoods,
			SalesPrice:  decimal.NewFromInt(10),
			IsActive:    true,
		}

		repo.listProductsErr = errors.New("product list failed")
		rec := invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListProducts, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/products?search=widget&low_stock=true",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list products")
		repo.listProductsErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportProducts, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/products/import",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportProducts, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/products/import",
			inventory.ImportProductsRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "csv_content is required")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportProducts, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/products/import",
			inventory.ImportProductsRequest{CSVContent: "code\nSKU-1"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "missing required columns")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdateProduct, wave5PayrollInventoryRawRequest(
			http.MethodPut,
			"/tenants/tenant-1/products/prod-1",
			"{",
			map[string]string{"tenantID": "tenant-1", "productID": "prod-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		repo.updateProductErr = errors.New("product update failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.UpdateProduct, newInventoryJSONRequest(t,
			http.MethodPut,
			"/tenants/tenant-1/products/prod-1",
			inventory.UpdateProductRequest{Name: "Updated", SalesPrice: "11.00", IsActive: true},
			map[string]string{"tenantID": "tenant-1", "productID": "prod-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to update product")
		repo.updateProductErr = nil

		repo.deleteProductErr = errors.New("product delete failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.DeleteProduct, newInventoryJSONRequest(t,
			http.MethodDelete,
			"/tenants/tenant-1/products/prod-1",
			nil,
			map[string]string{"tenantID": "tenant-1", "productID": "prod-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to delete product")
		repo.deleteProductErr = nil

		repo.getStockErr = errors.New("stock read failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetStockLevels, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/products/prod-1/stock-levels",
			nil,
			map[string]string{"tenantID": "tenant-1", "productID": "prod-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get stock levels")
		repo.getStockErr = nil

		repo.listMovementsErr = errors.New("movement read failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetInventoryMovements, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/products/prod-1/movements",
			nil,
			map[string]string{"tenantID": "tenant-1", "productID": "prod-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get movements")
	})

	t.Run("warehouse error branches", func(t *testing.T) {
		h, repo, tenantRepo := setupInventoryTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
		repo.warehouses["wh-1"] = &inventory.Warehouse{ID: "wh-1", TenantID: "tenant-1", Code: "MAIN", Name: "Main", IsActive: true}

		repo.listWarehousesErr = errors.New("warehouse list failed")
		rec := invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListWarehouses, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/warehouses?active_only=true",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list warehouses")
		repo.listWarehousesErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CreateWarehouse, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/warehouses",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		repo.createWarehouseErr = errors.New("warehouse insert failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.CreateWarehouse, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/warehouses",
			inventory.CreateWarehouseRequest{Code: "BACK", Name: "Back room"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to create warehouse")
		repo.createWarehouseErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportWarehouses, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/warehouses/import",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportWarehouses, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/warehouses/import",
			inventory.ImportWarehousesRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "csv_content is required")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportWarehouses, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/warehouses/import",
			inventory.ImportWarehousesRequest{CSVContent: "code\nMAIN"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "missing required columns")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetWarehouse, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/warehouses/missing",
			nil,
			map[string]string{"tenantID": "tenant-1", "warehouseID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Warehouse not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdateWarehouse, wave5PayrollInventoryRawRequest(
			http.MethodPut,
			"/tenants/tenant-1/warehouses/wh-1",
			"{",
			map[string]string{"tenantID": "tenant-1", "warehouseID": "wh-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		repo.updateWarehouseErr = errors.New("warehouse update failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.UpdateWarehouse, newInventoryJSONRequest(t,
			http.MethodPut,
			"/tenants/tenant-1/warehouses/wh-1",
			inventory.UpdateWarehouseRequest{Name: "Updated", IsActive: true},
			map[string]string{"tenantID": "tenant-1", "warehouseID": "wh-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to update warehouse")
		repo.updateWarehouseErr = nil

		repo.deleteWarehouseErr = errors.New("warehouse delete failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.DeleteWarehouse, newInventoryJSONRequest(t,
			http.MethodDelete,
			"/tenants/tenant-1/warehouses/wh-1",
			nil,
			map[string]string{"tenantID": "tenant-1", "warehouseID": "wh-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to delete warehouse")
	})
}

func TestWave5InventoryReportBranches(t *testing.T) {
	t.Run("valuation error branches", func(t *testing.T) {
		h, repo, tenantRepo := setupInventoryTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

		rec := invokePayrollImportRaw(t, http.StatusNotFound, h.GetInventoryValuation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/missing/inventory/valuation",
			nil,
			map[string]string{"tenantID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Tenant not found")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetInventoryValuation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/valuation?method=unsupported",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "invalid valuation method")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetInventoryValuation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/valuation?warehouse_id=missing",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "warehouse")

		repo.listProductsErr = errors.New("product list failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetInventoryValuation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/valuation",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get inventory valuation")
	})

	t.Run("subledger reconciliation error branches", func(t *testing.T) {
		h, repo, tenantRepo := setupInventoryTestHandlers()
		h.inventoryService = inventory.NewServiceWithRepositoryAndAccounting(repo, mockInventoryAccountingBalancer{
			balances: map[string]decimal.Decimal{},
		})
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

		rec := invokePayrollImportRaw(t, http.StatusNotFound, h.GetInventorySubledgerReconciliation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/missing/inventory/subledger-reconciliation",
			nil,
			map[string]string{"tenantID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Tenant not found")

		rec = invokePayrollImportRaw(t, http.StatusOK, h.GetInventorySubledgerReconciliation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/subledger-reconciliation",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "generated_at")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetInventorySubledgerReconciliation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/subledger-reconciliation?method=unsupported",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "invalid valuation method")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetInventorySubledgerReconciliation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/subledger-reconciliation?warehouse_id=missing",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "warehouse")

		repo.listProductsErr = errors.New("product list failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetInventorySubledgerReconciliation, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/subledger-reconciliation",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get inventory subledger reconciliation")
	})

	t.Run("lot report error branches", func(t *testing.T) {
		h, repo, tenantRepo := setupInventoryTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}
		repo.products["prod-1"] = &inventory.Product{
			ID:             "prod-1",
			TenantID:       "tenant-1",
			Code:           "SKU-001",
			Name:           "Widget",
			ProductType:    inventory.ProductTypeGoods,
			PurchasePrice:  decimal.NewFromInt(10),
			TrackInventory: true,
		}

		rec := invokePayrollImportRaw(t, http.StatusBadRequest, h.GetInventoryLotReport, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/lots?warehouse_id=missing",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "warehouse")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetInventoryLotReport, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/lots?product_id=missing",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "get product")

		repo.listProductsErr = errors.New("product list failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetInventoryLotReport, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/lots",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get inventory lot report")
		repo.listProductsErr = nil

		repo.listMovementsErr = errors.New("movement list failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetInventoryLotReport, newInventoryJSONRequest(t,
			http.MethodGet,
			"/tenants/tenant-1/inventory/lots?product_id=prod-1",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to get inventory lot report")
	})
}

func TestWave5StockOperationBranches(t *testing.T) {
	t.Run("stock adjustment import and adjust request branches", func(t *testing.T) {
		h, _, tenantRepo := setupInventoryTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

		rec := invokePayrollImportRaw(t, http.StatusUnauthorized, h.ImportStockAdjustments, wave5InventoryUnauthenticatedRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/stock-import",
			`{"csv_content":"product_code,warehouse_code,quantity\nSKU,MAIN,1"}`,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid or missing authentication")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportStockAdjustments, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/stock-import",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportStockAdjustments, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/inventory/stock-import",
			inventory.ImportStockAdjustmentsRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "csv_content is required")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ImportStockAdjustments, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/inventory/stock-import",
			inventory.ImportStockAdjustmentsRequest{CSVContent: "product_code\nSKU-001"},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "missing required columns")

		rec = invokePayrollImportRaw(t, http.StatusUnauthorized, h.AdjustStock, wave5InventoryUnauthenticatedRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/adjust",
			`{"product_id":"prod","warehouse_id":"wh","quantity":"1"}`,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid or missing authentication")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.AdjustStock, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/adjust",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")
	})

	t.Run("issue transfer reserve release request branches", func(t *testing.T) {
		h, _, tenantRepo := setupInventoryTestHandlers()
		tenantRepo.tenants["tenant-1"] = &tenant.Tenant{ID: "tenant-1", SchemaName: "tenant_test"}

		rec := invokePayrollImportRaw(t, http.StatusNotFound, h.IssueStock, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/missing/inventory/issue",
			inventory.IssueStockRequest{},
			map[string]string{"tenantID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Tenant not found")

		rec = invokePayrollImportRaw(t, http.StatusUnauthorized, h.IssueStock, wave5InventoryUnauthenticatedRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/issue",
			`{"product_id":"prod","warehouse_id":"wh","quantity":"1"}`,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid or missing authentication")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.IssueStock, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/issue",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.IssueStock, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/inventory/issue",
			inventory.IssueStockRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to issue stock")

		rec = invokePayrollImportRaw(t, http.StatusUnauthorized, h.TransferStock, wave5InventoryUnauthenticatedRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/transfer",
			`{"product_id":"prod","from_warehouse_id":"a","to_warehouse_id":"b","quantity":"1"}`,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid or missing authentication")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.TransferStock, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/transfer",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.TransferStock, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/inventory/transfer",
			inventory.TransferStockRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to transfer stock")

		rec = invokePayrollImportRaw(t, http.StatusUnauthorized, h.ReserveStock, wave5InventoryUnauthenticatedRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/reserve",
			`{"product_id":"prod","warehouse_id":"wh","quantity":"1"}`,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid or missing authentication")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ReserveStock, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/reserve",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ReserveStock, newInventoryJSONRequest(t,
			http.MethodPost,
			"/tenants/tenant-1/inventory/reserve",
			inventory.StockReservationRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to reserve stock")

		rec = invokePayrollImportRaw(t, http.StatusUnauthorized, h.ReleaseStock, wave5InventoryUnauthenticatedRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/release",
			`{"product_id":"prod","warehouse_id":"wh","quantity":"1"}`,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid or missing authentication")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ReleaseStock, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/inventory/release",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")
	})
}

func TestWave5LeaveHandlerErrorBranches(t *testing.T) {
	t.Run("absence type and balance errors", func(t *testing.T) {
		h, _, absenceRepo := setupWave5PayrollHandlers(t)
		absenceRepo.AbsenceTypes["annual"] = &payroll.AbsenceType{
			ID:                 "annual",
			TenantID:           "tenant-1",
			Code:               "ANNUAL",
			Name:               "Annual leave",
			DefaultDaysPerYear: decimal.NewFromInt(28),
			IsActive:           true,
		}

		absenceRepo.ListAbsenceTypesErr = errors.New("absence type list failed")
		rec := invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListAbsenceTypes, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/absence-types?active_only=true",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list absence types")
		absenceRepo.ListAbsenceTypesErr = nil

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetAbsenceType, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/absence-types/missing",
			nil,
			map[string]string{"tenantID": "tenant-1", "typeID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Absence type not found")

		absenceRepo.ListLeaveBalancesErr = errors.New("leave balance list failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListLeaveBalances, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/employees/emp-1/leave-balances?year=bad",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list leave balances")
		absenceRepo.ListLeaveBalancesErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.GetLeaveBalancesByYear, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/employees/emp-1/leave-balances/year",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1", "year": "year"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid year")

		absenceRepo.ListLeaveBalancesErr = errors.New("leave balance list failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.GetLeaveBalancesByYear, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/employees/emp-1/leave-balances/2026",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1", "year": "2026"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list leave balances")
		absenceRepo.ListLeaveBalancesErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdateLeaveBalance, payrollHandlerRequest(
			http.MethodPut,
			"/tenants/tenant-1/employees/emp-1/leave-balances/year/annual",
			payroll.UpdateLeaveBalanceRequest{},
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1", "year": "year", "typeID": "annual"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid year")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdateLeaveBalance, wave5PayrollInventoryRawRequest(
			http.MethodPut,
			"/tenants/tenant-1/employees/emp-1/leave-balances/2026/annual",
			"{",
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1", "year": "2026", "typeID": "annual"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.UpdateLeaveBalance, payrollHandlerRequest(
			http.MethodPut,
			"/tenants/tenant-1/employees/emp-1/leave-balances/2026/annual",
			payroll.UpdateLeaveBalanceRequest{},
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1", "year": "2026", "typeID": "annual"},
		))
		assert.Contains(t, rec.Body.String(), "get leave balance")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.InitializeLeaveBalances, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/emp-1/leave-balances/year/initialize",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1", "year": "year"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid year")

		absenceRepo.CreateLeaveBalanceErr = errors.New("leave balance insert failed")
		rec = invokePayrollImportRaw(t, http.StatusInternalServerError, h.InitializeLeaveBalances, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/employees/emp-1/leave-balances/2026/initialize",
			nil,
			map[string]string{"tenantID": "tenant-1", "employeeID": "emp-1", "year": "2026"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to initialize leave balances")
	})

	t.Run("leave record errors", func(t *testing.T) {
		h, _, absenceRepo := setupWave5PayrollHandlers(t)
		absenceRepo.AbsenceTypes["requires-doc"] = &payroll.AbsenceType{
			ID:               "requires-doc",
			TenantID:         "tenant-1",
			Code:             "SICK",
			Name:             "Sick leave",
			RequiresDocument: true,
			IsActive:         true,
		}
		absenceRepo.LeaveRecords["leave-pending"] = &payroll.LeaveRecord{
			ID:            "leave-pending",
			TenantID:      "tenant-1",
			EmployeeID:    "emp-1",
			AbsenceTypeID: "requires-doc",
			StartDate:     time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			EndDate:       time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			WorkingDays:   decimal.NewFromInt(1),
			Status:        payroll.LeavePending,
		}

		absenceRepo.ListLeaveRecordsErr = errors.New("leave record list failed")
		rec := invokePayrollImportRaw(t, http.StatusInternalServerError, h.ListLeaveRecords, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/leave-records?employee_id=emp-1&year=bad",
			nil,
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Failed to list leave records")
		absenceRepo.ListLeaveRecordsErr = nil

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CreateLeaveRecord, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-records",
			"{",
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CreateLeaveRecord, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-records",
			payroll.CreateLeaveRecordRequest{},
			map[string]string{"tenantID": "tenant-1"},
		))
		assert.Contains(t, rec.Body.String(), "employee ID is required")

		rec = invokePayrollImportRaw(t, http.StatusNotFound, h.GetLeaveRecord, payrollHandlerRequest(
			http.MethodGet,
			"/tenants/tenant-1/leave-records/missing",
			nil,
			map[string]string{"tenantID": "tenant-1", "recordID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "Leave record not found")

		rec = invokePayrollImportRaw(t, http.StatusConflict, h.ApproveLeaveRecord, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-records/leave-pending/approve",
			nil,
			map[string]string{"tenantID": "tenant-1", "recordID": "leave-pending"},
		))
		assert.Contains(t, rec.Body.String(), "approved leave document is required")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.ApproveLeaveRecord, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-records/missing/approve",
			nil,
			map[string]string{"tenantID": "tenant-1", "recordID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "get leave record")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.RejectLeaveRecord, wave5PayrollInventoryRawRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-records/leave-pending/reject",
			"{",
			map[string]string{"tenantID": "tenant-1", "recordID": "leave-pending"},
		))
		assert.Contains(t, rec.Body.String(), "Invalid request body")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.RejectLeaveRecord, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-records/missing/reject",
			payroll.RejectLeaveRequest{Reason: "No coverage"},
			map[string]string{"tenantID": "tenant-1", "recordID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "get leave record")

		rec = invokePayrollImportRaw(t, http.StatusBadRequest, h.CancelLeaveRecord, payrollHandlerRequest(
			http.MethodPost,
			"/tenants/tenant-1/leave-records/missing/cancel",
			nil,
			map[string]string{"tenantID": "tenant-1", "recordID": "missing"},
		))
		assert.Contains(t, rec.Body.String(), "get leave record")
	})
}

func TestWave5PayrollRepositoryErrorInjection(t *testing.T) {
	h, repo, _ := setupWave5PayrollHandlers(t)
	repo.createEmployeeErr = errors.New("employee insert failed")

	rec := invokePayrollImportRaw(t, http.StatusBadRequest, h.CreateEmployee, payrollHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/employees",
		payroll.CreateEmployeeRequest{
			FirstName:      "Mari",
			LastName:       "Maasikas",
			StartDate:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			EmploymentType: payroll.EmploymentFullTime,
		},
		map[string]string{"tenantID": "tenant-1"},
	))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "employee insert failed")
}
