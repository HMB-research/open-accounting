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

const maxSmartAccountsBrowserCommercialDetailRequestBytes = 32 << 10

// IssueSmartAccountsBrowserCommercialDetails creates/reissues only the two
// fixed, source-bound commercial relay capabilities. It never accepts source
// values or applies data: the current private contract is deliberately held at
// list_selector_required before any browser/source access can occur.
// @Summary Authorize SmartAccounts commercial evidence relay
// @Description Tenant-owner-only. Issues fixed client-invoices then bank-payments review/archive-only relay capabilities from an existing selected onboarding batch. The current selector preflight blocks capture, staging promotion, preview, accounting apply, and reconciliation-full eligibility.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartaccountssync.BrowserCommercialDetailAuthorizeRequest true "Existing batch source and action-time transfer consent"
// @Success 201 {object} smartaccountssync.BrowserCommercialDetailIssueSet
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-commercial-details [post]
func (h *Handlers) IssueSmartAccountsBrowserCommercialDetails(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCommercialDetailService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts commercial relay is not configured")
		return
	}
	claims, ok := commercialDetailOwnerClaims(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial authorization")
		return
	}
	var request smartaccountssync.BrowserCommercialDetailAuthorizeRequest
	if !decodeCommercialDetailRequest(w, r, &request) {
		return
	}
	issues, err := h.smartAccountsBrowserCommercialDetailService.Authorize(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), request)
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailConsent):
		respondError(w, http.StatusBadRequest, "Owner transfer consent is required")
	case errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailUnauthorized):
		respondError(w, http.StatusForbidden, "SmartAccounts commercial source is not selected and paired for this tenant")
	case err != nil:
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts commercial relay is unavailable")
	default:
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusCreated, issues)
	}
}

// ResumeSmartAccountsBrowserCommercialDetail rotates exactly one lost same-run
// extension capability. It does not create a different source, route, scope,
// record payload, posting, or reconciliation result.
// @Summary Resume SmartAccounts commercial evidence relay
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Commercial relay run ID"
// @Param request body smartaccountssync.BrowserCommercialDetailResumeRequest true "Renewed owner transfer consent"
// @Success 201 {object} smartaccountssync.BrowserCommercialDetailIssue
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-commercial-details/{runID}/resume [post]
func (h *Handlers) ResumeSmartAccountsBrowserCommercialDetail(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCommercialDetailService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts commercial relay is not configured")
		return
	}
	claims, ok := commercialDetailOwnerClaims(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial resume")
		return
	}
	var request smartaccountssync.BrowserCommercialDetailResumeRequest
	if !decodeCommercialDetailRequest(w, r, &request) {
		return
	}
	issue, err := h.smartAccountsBrowserCommercialDetailService.Resume(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "runID")), request)
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailConsent):
		respondError(w, http.StatusBadRequest, "Renewed owner transfer consent is required")
	case errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailUnauthorized):
		respondError(w, http.StatusNotFound, "SmartAccounts commercial run was not found for this tenant")
	case err != nil:
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts commercial resume is unavailable")
	default:
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusCreated, issue)
	}
}

// GetSmartAccountsBrowserCommercialDetailOwnerStatus exposes count/digest-only
// state. It never returns the relay capability, source ID, routes, rows,
// names, amounts, browser state, private proof, preview, or apply result.
// @Summary Get owner-safe SmartAccounts commercial relay status
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Commercial relay run ID"
// @Success 200 {object} smartaccountssync.BrowserCommercialDetailStatus
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-commercial-details/{runID} [get]
func (h *Handlers) GetSmartAccountsBrowserCommercialDetailOwnerStatus(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserCommercialDetailService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts commercial relay is not configured")
		return
	}
	claims, ok := commercialDetailOwnerClaims(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial status")
		return
	}
	status, err := h.smartAccountsBrowserCommercialDetailService.OwnerStatus(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "runID")))
	if errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailUnauthorized) {
		respondError(w, http.StatusNotFound, "SmartAccounts commercial run was not found for this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts commercial status is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

func (h *Handlers) OptionsSmartAccountsBrowserCommercialDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCommercialDetailExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSmartAccountsBrowserCommercialDetail is an extension-only, bearer scoped
// relay status endpoint. It does not accept access-JWTs, cookies, queries, or
// any browser-origin other than the installed extension.
// @Summary Get SmartAccounts commercial relay status
// @Tags SmartAccounts Sync
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Commercial relay run ID"
// @Success 200 {object} smartaccountssync.BrowserCommercialDetailStatus
// @Router /smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID} [get]
func (h *Handlers) GetSmartAccountsBrowserCommercialDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCommercialDetailExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	status, ok := h.commercialDetailRelayStatus(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// StartSmartAccountsBrowserCommercialDetail relays an exact capability-bound
// start envelope. It does not collect source bytes and remains at the fixed
// list_selector_required gate.
// @Summary Start blocked SmartAccounts commercial relay
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Commercial relay run ID"
// @Param request body smartaccountssync.BrowserCommercialDetailStartRequest true "Exact issued commercial start envelope"
// @Success 200 {object} smartaccountssync.BrowserCommercialDetailStatus
// @Router /smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID} [post]
func (h *Handlers) StartSmartAccountsBrowserCommercialDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCommercialDetailExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserCommercialDetailService == nil || r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial start")
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts commercial authorization failed")
		return
	}
	var request smartaccountssync.BrowserCommercialDetailStartRequest
	if !decodeCommercialDetailRequest(w, r, &request) {
		return
	}
	status, err := h.smartAccountsBrowserCommercialDetailService.Start(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), token, request)
	if errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts commercial authorization failed")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailBlocked) {
		respondError(w, http.StatusConflict, "Commercial capture is blocked until a reviewed visible list selector and pager contract exists")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts commercial relay failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// UploadSmartAccountsBrowserCommercialDetail rejects data before reading its
// body. The e4b1524 relay has no reviewed visible list selector/pager, so an
// NDJSON upload would be unsafe even if syntactically well formed.
// @Summary Reject unreviewed SmartAccounts commercial resource upload
// @Description Extension-only. Always returns the fixed list-selector blocker before reading source bytes; no source data can be captured by this version.
// @Tags SmartAccounts Sync
// @Accept application/x-ndjson
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Commercial relay run ID"
// @Success 409 {object} object{error=string}
// @Router /smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID}/resource [put]
func (h *Handlers) UploadSmartAccountsBrowserCommercialDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCommercialDetailExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserCommercialDetailService == nil || r.URL.RawQuery != "" || strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]) != "application/x-ndjson" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial resource")
		return
	}
	if _, ok := bearerToken(r.Header.Get("Authorization")); !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts commercial authorization failed")
		return
	}
	respondError(w, http.StatusConflict, "Commercial capture is blocked until a reviewed visible list selector and pager contract exists")
}

// FinalizeSmartAccountsBrowserCommercialDetail accepts no browser data and is
// also held behind the same list-selector gate; a package cannot be claimed
// archive-complete until a reviewed selector can produce protected evidence.
// @Summary Reject blocked SmartAccounts commercial finalization
// @Description Extension-only. Literal `{}` is accepted syntactically but finalization remains blocked before bridge/package work until a reviewed list-selector/pager contract exists.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Commercial relay run ID"
// @Success 409 {object} object{error=string}
// @Router /smartaccounts-browser-commercial-captures/tenants/{tenantID}/runs/{runID}/finalize [post]
func (h *Handlers) FinalizeSmartAccountsBrowserCommercialDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCommercialDetailExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserCommercialDetailService == nil || r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial finalization")
		return
	}
	if _, ok := bearerToken(r.Header.Get("Authorization")); !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts commercial authorization failed")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2))
	if err != nil || strings.TrimSpace(string(body)) != "{}" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial finalization")
		return
	}
	respondError(w, http.StatusConflict, "Commercial capture is blocked until a reviewed visible list selector and pager contract exists")
}

func allowBraveCommercialDetailExtensionOrigin(w http.ResponseWriter, r *http.Request) bool {
	// Capability requests must not carry an OA web-session cookie. The private
	// relay contract is explicitly extension-origin + bearer only, and neither
	// the service nor bridge needs cookie credentials.
	if strings.TrimSpace(r.Header.Get("Cookie")) != "" {
		return false
	}
	return allowBraveCaptureExtensionOrigin(w, r)
}

func commercialDetailOwnerClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return nil, false
	}
	if claims.Role != tenant.RoleOwner {
		respondError(w, http.StatusForbidden, "Tenant owner permission required")
		return nil, false
	}
	return claims, true
}

func decodeCommercialDetailRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsBrowserCommercialDetailRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts commercial request")
		return false
	}
	return true
}

func (h *Handlers) commercialDetailRelayStatus(w http.ResponseWriter, r *http.Request) (smartaccountssync.BrowserCommercialDetailStatus, bool) {
	if h.smartAccountsBrowserCommercialDetailService == nil || r.URL.RawQuery != "" {
		respondError(w, http.StatusUnauthorized, "SmartAccounts commercial authorization failed")
		return smartaccountssync.BrowserCommercialDetailStatus{}, false
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts commercial authorization failed")
		return smartaccountssync.BrowserCommercialDetailStatus{}, false
	}
	status, err := h.smartAccountsBrowserCommercialDetailService.Status(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), token)
	if errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts commercial authorization failed")
		return smartaccountssync.BrowserCommercialDetailStatus{}, false
	}
	if errors.Is(err, smartaccountssync.ErrBrowserCommercialDetailRunNotFound) {
		respondError(w, http.StatusNotFound, "SmartAccounts commercial relay has not started")
		return smartaccountssync.BrowserCommercialDetailStatus{}, false
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts commercial status unavailable")
		return smartaccountssync.BrowserCommercialDetailStatus{}, false
	}
	return status, true
}
