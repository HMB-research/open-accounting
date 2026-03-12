package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type periodCloseResponse struct {
	Tenant *tenant.Tenant           `json:"tenant"`
	Event  *tenant.PeriodCloseEvent `json:"event"`
}

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

func (h *Handlers) ClosePeriod(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req tenant.ClosePeriodRequest
	if !decodeJSONRequest(w, r, &req) {
		return
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

func (h *Handlers) ReopenPeriod(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req tenant.ReopenPeriodRequest
	if !decodeJSONRequest(w, r, &req) {
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
	case strings.Contains(err.Error(), "tenant not found"):
		respondError(w, http.StatusNotFound, "Tenant not found")
	case strings.Contains(err.Error(), "period end date"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "note is required"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "period already closed through"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "no closed period to reopen"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "is not currently closed"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "has not been closed yet"):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "Failed to update period close state")
	}
}
