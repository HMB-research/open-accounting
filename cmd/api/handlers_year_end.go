package main

import (
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// GetYearEndCloseStatus returns fiscal year-end close readiness.
// @Summary Get year-end close status
// @Description Get fiscal year close readiness, retained-earnings mapping, net income, period-lock status, and existing carry-forward state
// @Tags Period Close
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param period_end_date query string true "Fiscal year-end date (YYYY-MM-DD)"
// @Success 200 {object} accounting.YearEndCloseStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-close-status [get]
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

// CreateYearEndCarryForward creates and posts a fiscal year-end carry-forward journal.
// @Summary Create year-end carry-forward
// @Description Create and post retained-earnings carry-forward journal entries after the fiscal year has been closed
// @Tags Period Close
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.CreateYearEndCarryForwardRequest true "Carry-forward request"
// @Success 200 {object} accounting.YearEndCarryForwardResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-carry-forward [post]
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

// ReverseYearEndCarryForward voids a posted carry-forward and creates a reversal journal.
// @Summary Reverse year-end carry-forward
// @Description Void an existing posted fiscal year-end carry-forward and create a posted reversal journal for controlled corrections
// @Tags Period Close
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.ReverseYearEndCarryForwardRequest true "Carry-forward reversal request"
// @Success 200 {object} accounting.YearEndCarryForwardReversalResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-carry-forward/reverse [post]
func (h *Handlers) ReverseYearEndCarryForward(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req accounting.ReverseYearEndCarryForwardRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	req.UserID = userID

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	result, err := h.accountingService.ReverseYearEndCarryForward(
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
	case strings.Contains(err.Error(), "reason is required"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "fiscal year must be closed"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "carry-forward already exists"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "carry-forward does not exist"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "current status"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "not in posted status"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "retained earnings account is required"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "no revenue or expense activity found"):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "Failed to process year-end close workflow")
	}
}
