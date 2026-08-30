package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// IssueSmartAccountsBrowserDiscovery creates a same-window, metadata-only
// discovery authorization for an already paired Brave source. It returns no
// relay capability and does not contact the bridge, capture source data, or
// create any accounting record.
// @Summary Authorize redacted SmartAccounts Brave discovery
// @Description Tenant-owner-only. Issues a ten-minute metadata-only browser discovery binding for the currently paired source; owner consent is recorded at action time. Optional bounded CSV-header probing requires separate explicit consent. It never returns credentials, cookies, source rows, or accounting data.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartaccountssync.BrowserDiscoveryStartRequest true "Paired source and explicit discovery consent"
// @Success 201 {object} smartaccountssync.BrowserDiscoveryIssue
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-discoveries [post]
func (h *Handlers) IssueSmartAccountsBrowserDiscovery(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserDiscoveryService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave discovery is not configured")
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
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave discovery authorization")
		return
	}
	var request smartaccountssync.BrowserDiscoveryStartRequest
	if !decodeSmartAccountsBrowserDiscoveryRequest(w, r, &request) {
		return
	}
	issue, err := h.smartAccountsBrowserDiscoveryService.Issue(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), request)
	if errors.Is(err, smartaccountssync.ErrBrowserDiscoveryConsent) {
		respondError(w, http.StatusBadRequest, "Explicit metadata-only discovery consent is required")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBrowserDiscoveryUnauthorized) {
		respondError(w, http.StatusForbidden, "SmartAccounts Brave discovery source is not paired to this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave discovery is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, issue)
}

// ReceiveSmartAccountsBrowserDiscoveryReceipt proxies a strictly redacted,
// same-window relay result to the private bridge. The browser provides no
// source selector to this endpoint; OA resolves it from the issued tenant
// authorization and persists only the bridge-safe digest/count receipt.
// @Summary Persist a redacted SmartAccounts Brave discovery receipt
// @Description Tenant-owner-only. Verifies the exact issued discovery ID, manifest, selected resources, consent scope, and strict relay event before proxying the redacted contract to the private bridge. It stores and returns only aggregate receipt fields; no source selector, control ID, header name/value, cookie, source row, credential, or token is retained or returned.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param discoveryID path string true "Issued discovery UUID"
// @Param request body smartaccountssync.BrowserDiscoveryRelayResult true "Strict redacted relay result"
// @Success 200 {object} smartaccountssync.BrowserDiscoveryReceipt
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-discoveries/{discoveryID}/receipt [post]
func (h *Handlers) ReceiveSmartAccountsBrowserDiscoveryReceipt(w http.ResponseWriter, r *http.Request) {
	if !h.requireSmartAccountsBrowserDiscoveryOwner(w, r) {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave discovery receipt")
		return
	}
	var result smartaccountssync.BrowserDiscoveryRelayResult
	if !decodeSmartAccountsBrowserDiscoveryRequest(w, r, &result) {
		return
	}
	receipt, err := h.smartAccountsBrowserDiscoveryService.Receive(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "discoveryID")), result)
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserDiscoveryUnauthorized):
		respondError(w, http.StatusNotFound, "SmartAccounts Brave discovery was not found for this tenant")
	case errors.Is(err, smartaccountssync.ErrBrowserDiscoveryInvalid):
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave discovery receipt")
	case errors.Is(err, smartaccountssync.ErrBrowserDiscoveryConflict):
		respondError(w, http.StatusConflict, "SmartAccounts Brave discovery receipt conflicts with the existing receipt")
	case errors.Is(err, smartaccountssync.ErrBrowserDiscoveryNotFound):
		respondError(w, http.StatusNotFound, "SmartAccounts Brave discovery receipt was not found")
	case err != nil:
		respondError(w, http.StatusBadGateway, "SmartAccounts Brave discovery receipt relay failed")
	default:
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusOK, receipt)
	}
}

// GetSmartAccountsBrowserDiscoveryReceipt returns only a durable aggregate
// receipt to the current tenant owner. It never returns the issued source ID,
// resource contract, consent audit, or relay/browser secret.
// @Summary Get redacted SmartAccounts Brave discovery receipt
// @Description Tenant-owner-only. Returns only the private bridge's digest and aggregate discovery status for an existing tenant/discovery binding.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param discoveryID path string true "Issued discovery UUID"
// @Success 200 {object} smartaccountssync.BrowserDiscoveryReceipt
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-discoveries/{discoveryID} [get]
func (h *Handlers) GetSmartAccountsBrowserDiscoveryReceipt(w http.ResponseWriter, r *http.Request) {
	if !h.requireSmartAccountsBrowserDiscoveryOwner(w, r) {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave discovery receipt status")
		return
	}
	receipt, err := h.smartAccountsBrowserDiscoveryService.Status(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "discoveryID")))
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserDiscoveryUnauthorized), errors.Is(err, smartaccountssync.ErrBrowserDiscoveryNotFound):
		respondError(w, http.StatusNotFound, "SmartAccounts Brave discovery receipt was not found for this tenant")
	case errors.Is(err, smartaccountssync.ErrBrowserDiscoveryConflict):
		respondError(w, http.StatusConflict, "SmartAccounts Brave discovery receipt conflicts with the existing receipt")
	case err != nil:
		respondError(w, http.StatusBadGateway, "SmartAccounts Brave discovery receipt status is unavailable")
	default:
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusOK, receipt)
	}
}

func (h *Handlers) requireSmartAccountsBrowserDiscoveryOwner(w http.ResponseWriter, r *http.Request) bool {
	if h.smartAccountsBrowserDiscoveryService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave discovery is not configured")
		return false
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return false
	}
	if claims.Role != tenant.RoleOwner {
		respondError(w, http.StatusForbidden, "Tenant owner permission required")
		return false
	}
	return true
}

func decodeSmartAccountsBrowserDiscoveryRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, smartaccountssync.BrowserDiscoveryMaxReceiptBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave discovery request")
		return false
	}
	return true
}
