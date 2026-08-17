package main

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

var errApprovedClosePackEvidenceRequired = errors.New("approved close-pack evidence is required")

type periodCloseResponse struct {
	Tenant *tenant.Tenant           `json:"tenant"`
	Event  *tenant.PeriodCloseEvent `json:"event"`
}

// ListPeriodCloseEvents lists recent period close and reopen events.
// @Summary List period close events
// @Description List recent period close and reopen events for a tenant
// @Tags Period Close
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param limit query int false "Maximum events to return, from 1 to 100"
// @Success 200 {array} tenant.PeriodCloseEvent
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/period-close-events [get]
func (h *Handlers) ListPeriodCloseEvents(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	limit := 20

	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsedLimit, err := strconv.Atoi(rawLimit)
		if err != nil || parsedLimit <= 0 || parsedLimit > 100 {
			respondError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = parsedLimit
	}

	events, err := h.tenantService.ListPeriodCloseEvents(r.Context(), tenantID, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load period close history")
		return
	}

	respondJSON(w, http.StatusOK, events)
}

// ClosePeriod closes a month-end or year-end period.
// @Summary Close period
// @Description Close a tenant accounting period after validating permissions, fiscal-year inventory costing readiness using the explicit method or tenant valuation policy, and required review evidence
// @Tags Period Close
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tenant.ClosePeriodRequest true "Period close request"
// @Success 200 {object} periodCloseResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/period-close [post]
func (h *Handlers) ClosePeriod(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req tenant.ClosePeriodRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	req.InventoryValuationMethod = tenantInventoryValuationMethod(tenantRecord, req.InventoryValuationMethod)
	requireCloseEvidence := req.ReviewerSignOff
	if !requireCloseEvidence {
		requireCloseEvidence, err = h.requiresHighRiskEvidence(r.Context(), tenantID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to load tenant evidence policy")
			return
		}
	}
	if requireCloseEvidence {
		if err := h.requireApprovedYearEndClosePackEvidence(r.Context(), tenantRecord, req.PeriodEndDate); err != nil {
			h.recordHighRiskEvidenceBlock(r.Context(), tenantID, evidencePolicyActorID(r.Context()), "period_close")
			respondPeriodCloseError(w, err)
			return
		}
		if err := h.requireYearEndInventoryCostingReady(r.Context(), tenantRecord.SchemaName, tenantID, tenantRecord.Settings.FiscalYearStart, req.PeriodEndDate, req.InventoryValuationMethod); err != nil {
			respondPeriodCloseError(w, err)
			return
		}
	}

	updatedTenant, event, err := h.tenantService.ClosePeriod(r.Context(), tenantID, userID, &req)
	if err != nil {
		respondPeriodCloseError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, periodCloseResponse{
		Tenant: updatedTenant,
		Event:  event,
	})
}

// ReopenPeriod reopens a previously closed period.
// @Summary Reopen period
// @Description Reopen a tenant accounting period when no blocking carry-forward exists
// @Tags Period Close
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tenant.ReopenPeriodRequest true "Period reopen request"
// @Success 200 {object} periodCloseResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/period-reopen [post]
func (h *Handlers) ReopenPeriod(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req tenant.ReopenPeriodRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	yearEndCarryForwardExists, err := h.yearEndCarryForwardExists(r, tenantRecord, req.PeriodEndDate)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	if yearEndCarryForwardExists {
		respondError(w, http.StatusConflict, "cannot reopen a fiscal year after carry-forward has been posted")
		return
	}

	updatedTenant, event, err := h.tenantService.ReopenPeriod(r.Context(), tenantID, userID, &req)
	if err != nil {
		respondPeriodCloseError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, periodCloseResponse{
		Tenant: updatedTenant,
		Event:  event,
	})
}

func (h *Handlers) authorizePeriodCloseMutation(w http.ResponseWriter, r *http.Request) (tenantID string, userID string, ok bool) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return "", "", false
	}

	tenantID = chi.URLParam(r, "tenantID")
	role, err := h.tenantService.GetUserRole(r.Context(), tenantID, claims.UserID)
	if err != nil {
		respondError(w, http.StatusForbidden, "Access denied")
		return "", "", false
	}

	perms := tenant.GetRolePermissions(role)
	if !perms.CanManageClose {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return "", "", false
	}

	return tenantID, claims.UserID, true
}

func respondPeriodCloseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errApprovedClosePackEvidenceRequired):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "tenant not found"):
		respondError(w, http.StatusNotFound, "Tenant not found")
	case strings.Contains(err.Error(), "period end date"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "note is required"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "reviewer sign-off is required"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "invalid valuation method"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "period already closed through"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "no closed period to reopen"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "is not currently closed"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "has not been closed yet"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "carry-forward has been posted"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "inventory costing review"):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "Failed to update period close state")
	}
}
