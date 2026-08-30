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

const maxSmartAccountsBrowserCaptureRequestBytes = 16 << 10

// IssueSmartAccountsBrowserCapture issues a short-lived scoped capability for
// one existing tenant/source Brave binding, UUID run, exact manifest version,
// and immutable scope. It never receives browser cookies, SmartAccounts API
// keys, source rows, or financial-posting instructions.
// @Summary Authorize a scoped SmartAccounts Brave CSV capture
// @Description Tenant-owner-only. Returns a ten-minute scoped capture capability exactly once, bound to one existing Brave source, tenant, UUID run, manifest version, and exact scope. It never posts accounting data.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartaccountssync.BrowserCaptureStartRequest true "Immutable browser capture scope"
// @Success 201 {object} smartaccountssync.BrowserCaptureIssue
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-captures [post]
func (h *Handlers) IssueSmartAccountsBrowserCapture(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCaptureService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	// Browser-export transfer is a tenant-owner decision, not merely an
	// accounting write capability. TenantContext has refreshed this role.
	if claims.Role != tenant.RoleOwner {
		respondError(w, http.StatusForbidden, "Tenant owner permission required")
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture authorization")
		return
	}
	var request smartaccountssync.BrowserCaptureStartRequest
	if !decodeSmartAccountsBrowserCaptureRequest(w, r, &request) {
		return
	}
	issue, err := h.smartAccountsBrowserCaptureService.Issue(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), request)
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureInvalid) {
		respondError(w, http.StatusBadRequest, "Only the reviewed SmartAccounts General Ledger browser scope is supported")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureUnauthorized) {
		respondError(w, http.StatusForbidden, "SmartAccounts Brave capture source is not paired to this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, issue)
}

// ResumeSmartAccountsBrowserCapture rotates a lost/expired in-memory relay
// capability for the same persisted immutable run. It does not start a new
// bridge capture and requires the owner to explicitly reconfirm transfer.
// @Summary Resume a scoped SmartAccounts Brave CSV capture
// @Description Tenant-owner-only. Rotates the short-lived relay capability for the exact existing tenant/source/run/manifest/scope after renewed transfer consent; it never starts a new run or posts accounting data.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Capture run ID"
// @Param request body smartaccountssync.BrowserCaptureResumeRequest true "Renewed transfer consent"
// @Success 201 {object} smartaccountssync.BrowserCaptureIssue
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-captures/{runID}/resume [post]
func (h *Handlers) ResumeSmartAccountsBrowserCapture(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCaptureService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture is not configured")
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
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture resume")
		return
	}
	var request smartaccountssync.BrowserCaptureResumeRequest
	if !decodeSmartAccountsBrowserCaptureRequest(w, r, &request) {
		return
	}
	issue, err := h.smartAccountsBrowserCaptureService.Resume(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "runID")), request)
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureConsent) {
		respondError(w, http.StatusBadRequest, "Renewed owner transfer consent is required")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureUnauthorized) {
		respondError(w, http.StatusNotFound, "SmartAccounts Brave capture was not found for this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture resume is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, issue)
}

// GetSmartAccountsBrowserCaptureOwnerStatus returns safe progress for the
// persisted capture binding. It is owner-authenticated rather than capability
// authenticated so an operator can resume a run after the relay restarts; it
// never returns a capture token or its hash.
// @Summary Get owner-safe SmartAccounts Brave capture status
// @Description Tenant-owner-only. Returns safe immutable receipt/staging progress for an existing tenant/run binding, never a relay capability, source rows, credentials, or token hash.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Capture run ID"
// @Success 200 {object} smartaccountssync.BrowserCaptureStatus
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-captures/{runID} [get]
func (h *Handlers) GetSmartAccountsBrowserCaptureOwnerStatus(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCaptureService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture is not configured")
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
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture status")
		return
	}
	status, err := h.smartAccountsBrowserCaptureService.OwnerStatus(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")))
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureUnauthorized) {
		respondError(w, http.StatusNotFound, "SmartAccounts Brave capture was not found for this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave capture status is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// OptionsSmartAccountsBrowserCapture supports only a locally installed Brave
// extension. Ordinary browser origins cannot turn an owner-issued relay token
// into an upload capability.
func (h *Handlers) OptionsSmartAccountsBrowserCapture(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSmartAccountsBrowserCapture returns extension-origin safe capture status.
// @Summary Get SmartAccounts Brave capture status
// @Description Extension-only, scoped-token status with no source rows, credentials, or token hash.
// @Tags SmartAccounts Sync
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Capture run ID"
// @Success 200 {object} smartaccountssync.BrowserCaptureStatus
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID} [get]
func (h *Handlers) GetSmartAccountsBrowserCapture(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	status, ok := h.browserCaptureStatus(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// UploadSmartAccountsBrowserCaptureResource relays bounded raw CSV only when
// the immutable authorization contains the exact resource ID.
// @Summary Relay one scoped SmartAccounts Brave CSV resource
// @Description Extension-only raw CSV relay. It verifies the token, tenant/run/resource scope, size, and SHA-256 before passing bytes to the private bridge; no accounting write occurs.
// @Tags SmartAccounts Sync
// @Accept text/csv
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Capture run ID"
// @Param resourceID path string true "Approved relay resource ID"
// @Param X-SA-Browser-Resource-SHA256 header string true "Lowercase raw CSV SHA-256"
// @Success 200 {object} smartaccountssync.BrowserCaptureResourceStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Router /smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID}/resources/{resourceID} [put]
func (h *Handlers) UploadSmartAccountsBrowserCaptureResource(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserCaptureService == nil || r.URL.RawQuery != "" || strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]) != "text/csv" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture resource")
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave capture authorization failed")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, smartaccountssync.BrowserCaptureMaxResourceBytes))
	if err != nil || len(body) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture resource")
		return
	}
	resource, err := h.smartAccountsBrowserCaptureService.Upload(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), strings.TrimSpace(chi.URLParam(r, "resourceID")), token, strings.TrimSpace(r.Header.Get("X-SA-Browser-Resource-SHA256")), body)
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave capture authorization failed")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts Brave capture resource relay failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, resource)
}

// FinalizeSmartAccountsBrowserCapture records a safe coverage receipt only.
// @Summary Finalize a SmartAccounts Brave capture receipt
// @Description Extension-only scoped-token finalization. It can record partial/full-blocked coverage but never creates journals or applies a package.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Capture run ID"
// @Success 200 {object} smartaccountssync.BrowserCaptureStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Router /smartaccounts-browser-captures/tenants/{tenantID}/runs/{runID}/finalize [post]
func (h *Handlers) FinalizeSmartAccountsBrowserCapture(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserCaptureService == nil || r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture finalization")
		return
	}
	if body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2)); err != nil || strings.TrimSpace(string(body)) != "{}" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture finalization")
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave capture authorization failed")
		return
	}
	status, err := h.smartAccountsBrowserCaptureService.Finalize(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), token)
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave capture authorization failed")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts Brave capture finalization failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

func (h *Handlers) browserCaptureStatus(w http.ResponseWriter, r *http.Request) (smartaccountssync.BrowserCaptureStatus, bool) {
	if h.smartAccountsBrowserCaptureService == nil || r.URL.RawQuery != "" {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave capture authorization failed")
		return smartaccountssync.BrowserCaptureStatus{}, false
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave capture authorization failed")
		return smartaccountssync.BrowserCaptureStatus{}, false
	}
	status, err := h.smartAccountsBrowserCaptureService.Status(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), token)
	if errors.Is(err, smartaccountssync.ErrBrowserCaptureUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave capture authorization failed")
		return smartaccountssync.BrowserCaptureStatus{}, false
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts Brave capture status unavailable")
		return smartaccountssync.BrowserCaptureStatus{}, false
	}
	return status, true
}

func allowBraveCaptureExtensionOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if !braveExtensionOriginPattern.MatchString(origin) {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-SA-Browser-Resource-SHA256, X-SA-Browser-Commercial-SHA256")
	w.Header().Set("Access-Control-Max-Age", "600")
	w.Header().Set("Vary", "Origin")
	return true
}

func decodeSmartAccountsBrowserCaptureRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsBrowserCaptureRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave capture authorization")
		return false
	}
	return true
}
