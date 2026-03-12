package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/payroll"
	tenantpkg "github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/testutil"
)

func setupPayrollIntegrationHandlers(t *testing.T) (*Handlers, *testutil.TestTenant, *auth.Claims, *pgxpool.Pool) {
	t.Helper()

	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, fmt.Sprintf("payroll-handler-%d@example.com", time.Now().UnixNano()))
	testutil.AddUserToTenant(t, pool, tenant.ID, userID, tenantpkg.RoleOwner)

	ctx := context.Background()
	if _, err := pool.Exec(ctx, "SELECT add_payroll_tables($1)", tenant.SchemaName); err != nil {
		t.Fatalf("failed to add payroll tables: %v", err)
	}

	h := &Handlers{
		pool:           pool,
		tenantService:  tenantpkg.NewService(pool),
		payrollService: payroll.NewService(pool),
		absenceService: payroll.NewAbsenceServiceWithPool(pool),
	}

	claims := createTestClaims(userID, fmt.Sprintf("payroll-handler-%d@example.com", time.Now().UnixNano()), tenant.ID, tenantpkg.RoleOwner)
	return h, tenant, claims, pool
}

func TestPayrollHandlersIntegration(t *testing.T) {
	h, tenant, claims, _ := setupPayrollIntegrationHandlers(t)

	employeeReq := payroll.CreateEmployeeRequest{
		EmployeeNumber:       "EMP-H-001",
		FirstName:            "Helmi",
		LastName:             "Handler",
		PersonalCode:         "48901234567",
		Email:                "helmi.handler@example.com",
		StartDate:            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EmploymentType:       payroll.EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: payroll.DefaultBasicExemption,
		FundedPensionRate:    payroll.FundedPensionRateDefault,
	}
	employee := invokeJSON[payroll.Employee](t, http.StatusCreated, func(w http.ResponseWriter, r *http.Request) {
		h.CreateEmployee(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/employees", employeeReq, claims), map[string]string{"tenantID": tenant.ID}))
	if employee.ID == "" {
		t.Fatal("expected created employee id")
	}

	listResp := invokeJSON[[]payroll.Employee](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListEmployees(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/employees?active_only=true", nil, claims), map[string]string{"tenantID": tenant.ID}))
	if len(listResp) != 1 {
		t.Fatalf("expected 1 employee from list, got %d", len(listResp))
	}

	gotEmployee := invokeJSON[payroll.Employee](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetEmployee(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/employees/"+employee.ID, nil, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID}))
	if gotEmployee.Email != employeeReq.Email {
		t.Fatalf("expected employee email %q, got %q", employeeReq.Email, gotEmployee.Email)
	}

	updateReq := map[string]any{
		"department": "Accounting",
		"position":   "Accountant",
	}
	updatedEmployee := invokeJSON[payroll.Employee](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.UpdateEmployee(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenant.ID+"/employees/"+employee.ID, updateReq, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID}))
	if updatedEmployee.Department != "Accounting" {
		t.Fatalf("expected updated department, got %q", updatedEmployee.Department)
	}

	invokeJSON[map[string]string](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.SetBaseSalary(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/employees/"+employee.ID+"/salary", map[string]any{
		"amount":         "3000.00",
		"effective_from": time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339),
	}, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID}))

	runReq := payroll.CreatePayrollRunRequest{
		PeriodYear:  2025,
		PeriodMonth: 2,
	}
	run := invokeJSON[payroll.PayrollRun](t, http.StatusCreated, func(w http.ResponseWriter, r *http.Request) {
		h.CreatePayrollRun(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/payroll-runs", runReq, claims), map[string]string{"tenantID": tenant.ID}))

	runs := invokeJSON[[]payroll.PayrollRun](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListPayrollRuns(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/payroll-runs?year=2025", nil, claims), map[string]string{"tenantID": tenant.ID}))
	if len(runs) != 1 {
		t.Fatalf("expected 1 payroll run, got %d", len(runs))
	}

	gotRun := invokeJSON[payroll.PayrollRun](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetPayrollRun(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/payroll-runs/"+run.ID, nil, claims), map[string]string{"tenantID": tenant.ID, "runID": run.ID}))
	if gotRun.ID != run.ID {
		t.Fatalf("expected payroll run %s, got %s", run.ID, gotRun.ID)
	}

	calculated := invokeJSON[payroll.PayrollRun](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.CalculatePayroll(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/payroll-runs/"+run.ID+"/calculate", nil, claims), map[string]string{"tenantID": tenant.ID, "runID": run.ID}))
	if calculated.Status != payroll.PayrollCalculated {
		t.Fatalf("expected calculated payroll status, got %s", calculated.Status)
	}

	payslips := invokeJSON[[]payroll.Payslip](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetPayslips(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/payroll-runs/"+run.ID+"/payslips", nil, claims), map[string]string{"tenantID": tenant.ID, "runID": run.ID}))
	if len(payslips) != 1 {
		t.Fatalf("expected 1 payslip, got %d", len(payslips))
	}

	invokeJSON[map[string]string](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ApprovePayroll(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/payroll-runs/"+run.ID+"/approve", nil, claims), map[string]string{"tenantID": tenant.ID, "runID": run.ID}))

	tsd := invokeJSON[payroll.TSDDeclaration](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GenerateTSD(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/payroll-runs/"+run.ID+"/tsd", nil, claims), map[string]string{"tenantID": tenant.ID, "runID": run.ID}))
	if tsd.ID == "" {
		t.Fatal("expected generated TSD declaration")
	}

	gotTSD := invokeJSON[payroll.TSDDeclaration](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetTSD(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/tsd/2025/2", nil, claims), map[string]string{"tenantID": tenant.ID, "year": "2025", "month": "2"}))
	if gotTSD.PeriodYear != 2025 || gotTSD.PeriodMonth != 2 {
		t.Fatalf("expected TSD period 2025-2, got %d-%d", gotTSD.PeriodYear, gotTSD.PeriodMonth)
	}

	tsdList := invokeJSON[[]payroll.TSDDeclaration](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListTSD(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/tsd", nil, claims), map[string]string{"tenantID": tenant.ID}))
	if len(tsdList) != 1 {
		t.Fatalf("expected 1 TSD declaration, got %d", len(tsdList))
	}

	xmlResp := invokeRaw(t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ExportTSDXML(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/tsd/2025/2/xml", nil, claims), map[string]string{"tenantID": tenant.ID, "year": "2025", "month": "2"}))
	if ct := xmlResp.Header().Get("Content-Type"); ct != "application/xml" {
		t.Fatalf("expected xml content type, got %q", ct)
	}

	csvResp := invokeRaw(t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ExportTSDCSV(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/tsd/2025/2/csv", nil, claims), map[string]string{"tenantID": tenant.ID, "year": "2025", "month": "2"}))
	if ct := csvResp.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("expected csv content type, got %q", ct)
	}

	invokeJSON[map[string]string](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.MarkTSDSubmitted(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/tsd/2025/2/submit", map[string]string{
		"emta_reference": "EMTA-REF-001",
	}, claims), map[string]string{"tenantID": tenant.ID, "year": "2025", "month": "2"}))
}

func TestLeaveHandlersIntegration(t *testing.T) {
	h, tenant, claims, pool := setupPayrollIntegrationHandlers(t)
	employeeReq := payroll.CreateEmployeeRequest{
		EmployeeNumber:       "EMP-L-001",
		FirstName:            "Leelo",
		LastName:             "Leave",
		PersonalCode:         "48801234567",
		Email:                "leelo.leave@example.com",
		StartDate:            time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EmploymentType:       payroll.EmploymentFullTime,
		ApplyBasicExemption:  true,
		BasicExemptionAmount: payroll.DefaultBasicExemption,
		FundedPensionRate:    payroll.FundedPensionRateDefault,
	}
	employee := invokeJSON[payroll.Employee](t, http.StatusCreated, func(w http.ResponseWriter, r *http.Request) {
		h.CreateEmployee(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/employees", employeeReq, claims), map[string]string{"tenantID": tenant.ID}))

	absenceType := insertTenantAbsenceType(t, pool, tenant)

	types := invokeJSON[[]payroll.AbsenceType](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListAbsenceTypes(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/absence-types?active_only=true", nil, claims), map[string]string{"tenantID": tenant.ID}))
	if len(types) != 1 {
		t.Fatalf("expected 1 tenant absence type, got %d", len(types))
	}

	gotType := invokeJSON[payroll.AbsenceType](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetAbsenceType(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/absence-types/"+absenceType.ID, nil, claims), map[string]string{"tenantID": tenant.ID, "typeID": absenceType.ID}))
	if gotType.Code != absenceType.Code {
		t.Fatalf("expected absence type %q, got %q", absenceType.Code, gotType.Code)
	}

	initialized := invokeJSON[[]payroll.LeaveBalance](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.InitializeLeaveBalances(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/employees/"+employee.ID+"/leave-balances/2025/initialize", nil, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID, "year": "2025"}))
	if len(initialized) != 1 {
		t.Fatalf("expected 1 initialized leave balance, got %d", len(initialized))
	}

	balances := invokeJSON[[]payroll.LeaveBalance](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListLeaveBalances(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/employees/"+employee.ID+"/leave-balances?year=2025", nil, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID}))
	if len(balances) != 1 {
		t.Fatalf("expected 1 leave balance, got %d", len(balances))
	}

	byYear := invokeJSON[[]payroll.LeaveBalance](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetLeaveBalancesByYear(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/employees/"+employee.ID+"/leave-balances/2025", nil, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID, "year": "2025"}))
	if len(byYear) != 1 {
		t.Fatalf("expected 1 leave balance by year, got %d", len(byYear))
	}

	entitledDays := decimal.NewFromInt(12)
	updatedBalance := invokeJSON[payroll.LeaveBalance](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.UpdateLeaveBalance(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tenant.ID+"/employees/"+employee.ID+"/leave-balances/2025/"+absenceType.ID, payroll.UpdateLeaveBalanceRequest{
		EntitledDays: &entitledDays,
		Notes:        "updated by handler",
	}, claims), map[string]string{"tenantID": tenant.ID, "employeeID": employee.ID, "year": "2025", "typeID": absenceType.ID}))
	if !updatedBalance.EntitledDays.Equal(entitledDays) {
		t.Fatalf("expected entitled days %s, got %s", entitledDays, updatedBalance.EntitledDays)
	}

	record1 := createLeaveRecordViaHandler(t, h, tenant.ID, employee.ID, absenceType.ID, claims, time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC))
	recordList := invokeJSON[[]payroll.LeaveRecord](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ListLeaveRecords(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/leave-records?employee_id="+employee.ID+"&year=2025", nil, claims), map[string]string{"tenantID": tenant.ID}))
	if len(recordList) != 1 {
		t.Fatalf("expected 1 leave record, got %d", len(recordList))
	}

	gotRecord := invokeJSON[payroll.LeaveRecord](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.GetLeaveRecord(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tenant.ID+"/leave-records/"+record1.ID, nil, claims), map[string]string{"tenantID": tenant.ID, "recordID": record1.ID}))
	if gotRecord.ID != record1.ID {
		t.Fatalf("expected leave record %s, got %s", record1.ID, gotRecord.ID)
	}

	approvedRecord := invokeJSON[payroll.LeaveRecord](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.ApproveLeaveRecord(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/leave-records/"+record1.ID+"/approve", nil, claims), map[string]string{"tenantID": tenant.ID, "recordID": record1.ID}))
	if approvedRecord.Status != payroll.LeaveApproved {
		t.Fatalf("expected approved leave record, got %s", approvedRecord.Status)
	}

	record2 := createLeaveRecordViaHandler(t, h, tenant.ID, employee.ID, absenceType.ID, claims, time.Date(2025, 8, 1, 0, 0, 0, 0, time.UTC))
	rejectedRecord := invokeJSON[payroll.LeaveRecord](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.RejectLeaveRecord(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/leave-records/"+record2.ID+"/reject", payroll.RejectLeaveRequest{
		Reason: "Too busy",
	}, claims), map[string]string{"tenantID": tenant.ID, "recordID": record2.ID}))
	if rejectedRecord.Status != payroll.LeaveRejected {
		t.Fatalf("expected rejected leave record, got %s", rejectedRecord.Status)
	}

	record3 := createLeaveRecordViaHandler(t, h, tenant.ID, employee.ID, absenceType.ID, claims, time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC))
	canceledRecord := invokeJSON[payroll.LeaveRecord](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.CancelLeaveRecord(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenant.ID+"/leave-records/"+record3.ID+"/cancel", nil, claims), map[string]string{"tenantID": tenant.ID, "recordID": record3.ID}))
	if canceledRecord.Status != payroll.LeaveCanceled {
		t.Fatalf("expected canceled leave record, got %s", canceledRecord.Status)
	}
}

func TestCalculateTaxPreviewHandler(t *testing.T) {
	h := &Handlers{}

	badReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payroll/tax-preview", map[string]any{
		"gross_salary": "0",
	}, nil)
	w := httptest.NewRecorder()
	h.CalculateTaxPreview(w, badReq)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for zero salary, got %d", w.Code)
	}

	result := invokeJSON[payroll.TaxCalculation](t, http.StatusOK, func(w http.ResponseWriter, r *http.Request) {
		h.CalculateTaxPreview(w, r)
	}, makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/payroll/tax-preview", map[string]any{
		"gross_salary":          "3000.00",
		"apply_basic_exemption": true,
		"funded_pension_rate":   "0.02",
	}, nil))
	if result.NetSalary.IsZero() {
		t.Fatal("expected non-zero tax preview result")
	}
}

func createLeaveRecordViaHandler(t *testing.T, h *Handlers, tenantID, employeeID, absenceTypeID string, claims *auth.Claims, startDate time.Time) payroll.LeaveRecord {
	t.Helper()
	record := invokeJSON[payroll.LeaveRecord](t, http.StatusCreated, func(w http.ResponseWriter, r *http.Request) {
		h.CreateLeaveRecord(w, r)
	}, withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tenantID+"/leave-records", payroll.CreateLeaveRecordRequest{
		EmployeeID:    employeeID,
		AbsenceTypeID: absenceTypeID,
		StartDate:     startDate,
		EndDate:       startDate.AddDate(0, 0, 2),
		TotalDays:     decimal.NewFromInt(3),
		WorkingDays:   decimal.NewFromInt(3),
		Notes:         "integration leave record",
	}, claims), map[string]string{"tenantID": tenantID}))
	return record
}

func insertTenantAbsenceType(t *testing.T, pool *pgxpool.Pool, tenant *testutil.TestTenant) payroll.AbsenceType {
	t.Helper()

	absenceType := payroll.AbsenceType{
		ID:                 uuid.New().String(),
		TenantID:           tenant.ID,
		Code:               "HANDLER_LEAVE",
		Name:               "Handler Leave",
		NameET:             "Kasutaja puhkus",
		IsPaid:             true,
		AffectsSalary:      false,
		RequiresDocument:   false,
		DefaultDaysPerYear: decimal.NewFromInt(10),
		MaxCarryoverDays:   decimal.NewFromInt(0),
		IsActive:           true,
		SortOrder:          1,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if _, err := pool.Exec(context.Background(), `
		INSERT INTO `+tenant.SchemaName+`.absence_types
			(id, tenant_id, code, name, name_et, is_paid, affects_salary, requires_document,
			 default_days_per_year, max_carryover_days, is_active, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`, absenceType.ID, absenceType.TenantID, absenceType.Code, absenceType.Name, absenceType.NameET,
		absenceType.IsPaid, absenceType.AffectsSalary, absenceType.RequiresDocument,
		absenceType.DefaultDaysPerYear, absenceType.MaxCarryoverDays, absenceType.IsActive, absenceType.SortOrder,
		absenceType.CreatedAt, absenceType.UpdatedAt); err != nil {
		t.Fatalf("failed to insert tenant absence type: %v", err)
	}

	return absenceType
}

func invokeRaw(t *testing.T, wantStatus int, handler func(http.ResponseWriter, *http.Request), req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != wantStatus {
		t.Fatalf("expected status %d, got %d, body=%s", wantStatus, w.Code, w.Body.String())
	}
	return w
}

func invokeJSON[T any](t *testing.T, wantStatus int, handler func(http.ResponseWriter, *http.Request), req *http.Request) T {
	t.Helper()
	w := invokeRaw(t, wantStatus, handler, req)
	var out T
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return out
}
