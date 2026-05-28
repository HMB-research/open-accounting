package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
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
	if err := h.attachYearEndCloseEvidenceStatus(r.Context(), routeCtx.schemaName, routeCtx.tenantID, status); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to evaluate close-pack evidence")
		return
	}

	respondJSON(w, http.StatusOK, status)
}

// GetYearEndClosePack returns close readiness with year-end financial reports.
// @Summary Get year-end close pack
// @Description Get year-end close readiness plus trial balance, balance sheet, and income statement for the fiscal year
// @Tags Period Close
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param period_end_date query string true "Fiscal year-end date (YYYY-MM-DD)"
// @Success 200 {object} accounting.YearEndClosePack
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-close-pack [get]
func (h *Handlers) GetYearEndClosePack(w http.ResponseWriter, r *http.Request) {
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

	pack, err := h.accountingService.GetYearEndClosePack(
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
	if pack.Status != nil {
		if err := h.attachYearEndCloseEvidenceStatus(r.Context(), routeCtx.schemaName, routeCtx.tenantID, pack.Status); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to evaluate close-pack evidence")
			return
		}
	}

	respondJSON(w, http.StatusOK, pack)
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
	if err := h.requireApprovedYearEndClosePackEvidence(r.Context(), tenantRecord, req.PeriodEndDate); err != nil {
		respondYearEndCloseError(w, err)
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
	if result.Status != nil {
		if err := h.attachYearEndCloseEvidenceStatus(r.Context(), tenantRecord.SchemaName, tenantID, result.Status); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to evaluate close-pack evidence")
			return
		}
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

func (h *Handlers) requireApprovedYearEndClosePackEvidence(ctx context.Context, tenantRecord *tenant.Tenant, rawPeriodEndDate string) error {
	if h.documentsService == nil {
		return nil
	}
	isYearEnd, err := accounting.IsFiscalYearEndPeriod(rawPeriodEndDate, tenantRecord.Settings.FiscalYearStart)
	if err != nil {
		return err
	}
	if !isYearEnd {
		return nil
	}

	entityID, err := accounting.YearEndCloseEvidenceEntityID(tenantRecord.ID, rawPeriodEndDate)
	if err != nil {
		return err
	}
	results, err := h.yearEndClosePackEvidence(ctx, tenantRecord.SchemaName, tenantRecord.ID, entityID)
	if err != nil {
		return err
	}
	if len(results) == 0 || !results[0].Compliant {
		return fmt.Errorf("%w before completing fiscal-year close workflow for %s (entity_id: %s)", errApprovedClosePackEvidenceRequired, rawPeriodEndDate, entityID)
	}

	return nil
}

func (h *Handlers) attachYearEndCloseEvidenceStatus(ctx context.Context, schemaName, tenantID string, status *accounting.YearEndCloseStatus) error {
	if h.documentsService == nil || status == nil || strings.TrimSpace(status.ClosePackEvidenceEntityID) == "" {
		return nil
	}

	results, err := h.yearEndClosePackEvidence(ctx, schemaName, tenantID, status.ClosePackEvidenceEntityID)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}

	status.ClosePackEvidence = &results[0]
	status.CarryForwardReady = status.CarryForwardReady && results[0].Compliant
	return nil
}

func (h *Handlers) yearEndClosePackEvidence(ctx context.Context, schemaName, tenantID, entityID string) ([]documents.EvidencePolicyResult, error) {
	return h.documentsService.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeYearEndClose,
		EntityIDs:  []string{entityID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes:   []string{documents.DocumentTypeClosePack},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
}

func respondYearEndCloseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errApprovedClosePackEvidenceRequired):
		respondError(w, http.StatusConflict, err.Error())
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
