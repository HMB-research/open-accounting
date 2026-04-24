package main

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
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
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.ImportEmployeesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "employees_import.csv"
	}

	result, err := h.payrollService.ImportEmployeesCSV(r.Context(), schemaName, tenantID, &req)
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
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.ImportPayrollHistoryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "payroll-history.csv"
	}

	result, err := h.payrollService.ImportPayrollHistoryCSV(r.Context(), schemaName, tenantID, claims.UserID, &req)
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
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req payroll.ImportLeaveBalancesRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "leave-balances.csv"
	}

	result, err := h.absenceService.ImportLeaveBalancesCSV(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}
