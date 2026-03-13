package main

import (
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func (h *Handlers) GetYearEndCloseStatus(w http.ResponseWriter, r *http.Request) {
	routeCtx := h.tenantContextFromRequest(r)
	periodEndDate := strings.TrimSpace(r.URL.Query().Get("period_end_date"))
	if periodEndDate == "" {
		respondError(w, http.StatusBadRequest, "period end date is required")
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), routeCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	status, err := h.accountingService.GetYearEndCloseStatus(
		r.Context(),
		routeCtx.schemaName,
		routeCtx.tenantID,
		tenantRecord.Settings.FiscalYearStart,
		periodEndDate,
		tenantRecord.Settings.PeriodLockDate,
	)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (h *Handlers) CreateYearEndCarryForward(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req accounting.CreateYearEndCarryForwardRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	req.UserID = userID

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	result, err := h.accountingService.CreateYearEndCarryForward(
		r.Context(),
		tenantRecord.SchemaName,
		tenantID,
		tenantRecord.Settings.FiscalYearStart,
		tenantRecord.Settings.PeriodLockDate,
		&req,
	)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

func (h *Handlers) yearEndCarryForwardExists(r *http.Request, tenantRecord *tenant.Tenant, rawPeriodEndDate string) (bool, error) {
	if h.accountingService == nil {
		return false, nil
	}

	status, err := h.accountingService.GetYearEndCloseStatus(
		r.Context(),
		tenantRecord.SchemaName,
		tenantRecord.ID,
		tenantRecord.Settings.FiscalYearStart,
		rawPeriodEndDate,
		tenantRecord.Settings.PeriodLockDate,
	)
	if err != nil {
		return false, err
	}

	return status.IsFiscalYearEnd && status.ExistingCarryForward != nil, nil
}

func respondYearEndCloseError(w http.ResponseWriter, err error) {
	switch {
	case strings.Contains(err.Error(), "period end date"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "must match the fiscal year end"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "user_id is required"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "fiscal year must be closed"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "carry-forward already exists"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "retained earnings account is required"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "no revenue or expense activity found"):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "Failed to process year-end close workflow")
	}
}
