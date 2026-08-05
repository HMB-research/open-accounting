package main

import (
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/payroll"
)

// ImportEmployees imports employees and optional base salaries from CSV data.
// @Summary Import employees
// @Description Import employees from CSV data and optionally create recurring base salary components
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payroll.ImportEmployeesRequest true "CSV import payload"
// @Success 200 {object} payroll.ImportEmployeesResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/employees/import [post]
func (h *Handlers) ImportEmployees(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	var req payroll.ImportEmployeesRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "employees_import.csv"
	}

	result, err := h.payrollService.ImportEmployeesCSV(r.Context(), tenantCtx.schemaName, tenantCtx.tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ImportPayrollHistory imports finalized historical payroll runs and payslips from CSV data.
// @Summary Import historical payroll
// @Description Import finalized historical payroll runs and payslips from CSV data
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payroll.ImportPayrollHistoryRequest true "CSV import payload"
// @Success 200 {object} payroll.ImportPayrollHistoryResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/payroll-runs/import-history [post]
func (h *Handlers) ImportPayrollHistory(w http.ResponseWriter, r *http.Request) {
	claims := userClaimsFromRequest(r)
	tenantCtx := h.tenantContextFromRequest(r)

	var req payroll.ImportPayrollHistoryRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "payroll-history.csv"
	}

	result, err := h.payrollService.ImportPayrollHistoryCSV(r.Context(), tenantCtx.schemaName, tenantCtx.tenantID, claims.UserID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ImportTSDHistory imports historical TSD declarations and rows from CSV data.
// @Summary Import historical TSD declarations
// @Description Import historical TSD declarations and declaration rows from CSV data
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payroll.ImportTSDHistoryRequest true "CSV import payload"
// @Success 200 {object} payroll.ImportTSDHistoryResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/tsd/import-history [post]
func (h *Handlers) ImportTSDHistory(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	var req payroll.ImportTSDHistoryRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "tsd-history.csv"
	}

	result, err := h.payrollService.ImportTSDHistoryCSV(r.Context(), tenantCtx.schemaName, tenantCtx.tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ImportLeaveBalances imports or updates employee leave balances from CSV data.
// @Summary Import leave balances
// @Description Import or update employee leave balances from CSV data
// @Tags Payroll
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body payroll.ImportLeaveBalancesRequest true "CSV import payload"
// @Success 200 {object} payroll.ImportLeaveBalancesResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/leave-balances/import [post]
func (h *Handlers) ImportLeaveBalances(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	var req payroll.ImportLeaveBalancesRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "leave-balances.csv"
	}

	result, err := h.absenceService.ImportLeaveBalancesCSV(r.Context(), tenantCtx.schemaName, tenantCtx.tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}
