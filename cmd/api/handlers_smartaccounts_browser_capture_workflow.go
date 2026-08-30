package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// StartSmartAccountsBrowserCaptureWorkflow builds the only currently-proven
// browser capture plan (reviewed General Ledger CSV) for an already paired source. It
// derives its upper bound and cutoff server-side. A raw capture capability is
// returned only when the owner confirms transfer in this exact request.
// @Summary Build or issue a guided SmartAccounts Brave capture workflow
// @Description Tenant-owner-only. Reuses an exact tenant/source/history-start workflow, derives end/cutoff server-side, and returns a raw ten-minute relay capability only with action-time owner transfer consent. The v2 workflow captures the reviewed general_ledger_csv_v1 source only; journal_entries remains non-posting archive evidence. It never posts accounting data.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartaccountssync.BrowserCaptureWorkflowRequest true "Paired source, history start, and action-time consent"
// @Success 200 {object} smartaccountssync.BrowserCaptureWorkflowStatus
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-capture-workflows [post]
func (h *Handlers) StartSmartAccountsBrowserCaptureWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCaptureWorkflowService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture workflow is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if claims.Role != tenant.RoleOwner {
		respondError(w, http.StatusForbidden, "Tenant owner permission required")
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture workflow")
		return
	}
	var request smartaccountssync.BrowserCaptureWorkflowRequest
	if !decodeSmartAccountsBrowserCaptureRequest(w, r, &request) {
		return
	}
	workflow, err := h.smartAccountsBrowserCaptureWorkflowService.Start(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), request)
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureWorkflowInvalid) {
		respondError(w, http.StatusBadRequest, "A valid history start date no later than today is required")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureWorkflowNotPaired) {
		respondError(w, http.StatusForbidden, "SmartAccounts Brave capture source is not paired to this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture workflow is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, workflow)
}

// GetSmartAccountsBrowserCaptureWorkflowStatus lets an owner recover safe
// progress after an extension restart. It does not return a capture token,
// source bytes, credentials, cookies, or bridge implementation details.
// @Summary Get guided SmartAccounts Brave capture workflow status
// @Description Tenant-owner-only. Returns the immutable plan and safe progress for one tenant-scoped workflow, never a raw relay capability or source record.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param workflowID path string true "Workflow ID"
// @Success 200 {object} smartaccountssync.BrowserCaptureWorkflowStatus
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-capture-workflows/{workflowID} [get]
func (h *Handlers) GetSmartAccountsBrowserCaptureWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCaptureWorkflowService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture workflow is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if claims.Role != tenant.RoleOwner {
		respondError(w, http.StatusForbidden, "Tenant owner permission required")
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture workflow status")
		return
	}
	workflow, err := h.smartAccountsBrowserCaptureWorkflowService.Status(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "workflowID")))
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureWorkflowNotFound) {
		respondError(w, http.StatusNotFound, "SmartAccounts Brave capture workflow was not found for this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture workflow status is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, workflow)
}
