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

const maxSmartAccountsBrowserMasterDetailRequestBytes = 16 << 10

// IssueSmartAccountsBrowserMasterDetails authorizes exactly the fixed
// clients/vendors/articles current-snapshot relay workflow. The request cannot
// choose browser routes, schemas, dates, source rows, or financial actions.
// @Summary Authorize SmartAccounts Brave master-detail snapshots
// @Description Tenant-owner-only. One explicit transfer consent creates or resumes three fixed, serial current-snapshot relay runs for clients, vendors, and articles. It returns short-lived capability material once and never posts financial data.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartaccountssync.BrowserMasterDetailAuthorizeRequest true "Paired source and owner transfer consent"
// @Success 201 {object} smartaccountssync.BrowserMasterDetailIssueSet
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-master-details [post]
func (h *Handlers) IssueSmartAccountsBrowserMasterDetails(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserMasterDetailService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts master-detail relay is not configured")
		return
	}
	claims, ok := masterDetailOwnerClaims(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail authorization")
		return
	}
	var request smartaccountssync.BrowserMasterDetailAuthorizeRequest
	if !decodeMasterDetailRequest(w, r, &request) {
		return
	}
	issues, err := h.smartAccountsBrowserMasterDetailService.Authorize(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), request)
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailUnauthorized) {
		respondError(w, http.StatusForbidden, "SmartAccounts master-detail source is not paired to this tenant")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailConsent) {
		respondError(w, http.StatusBadRequest, "Owner transfer consent is required")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts master-detail relay is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, issues)
}

// ResumeSmartAccountsBrowserMasterDetail rotates a short-lived capability for
// exactly one existing immutable resource run. It does not broaden a snapshot
// or start a new generation.
// @Summary Resume one SmartAccounts master-detail relay run
// @Description Tenant-owner-only. Requires renewed transfer consent and rotates only a lost or expired in-memory capability for the exact persisted tenant/source/run/resource snapshot.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Master-detail run ID"
// @Param request body smartaccountssync.BrowserMasterDetailResumeRequest true "Renewed transfer consent"
// @Success 201 {object} smartaccountssync.BrowserMasterDetailIssue
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-master-details/{runID}/resume [post]
func (h *Handlers) ResumeSmartAccountsBrowserMasterDetail(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserMasterDetailService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts master-detail relay is not configured")
		return
	}
	claims, ok := masterDetailOwnerClaims(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail resume")
		return
	}
	var request smartaccountssync.BrowserMasterDetailResumeRequest
	if !decodeMasterDetailRequest(w, r, &request) {
		return
	}
	issue, err := h.smartAccountsBrowserMasterDetailService.Resume(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "runID")), request)
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailConsent) {
		respondError(w, http.StatusBadRequest, "Renewed owner transfer consent is required")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailUnauthorized) {
		respondError(w, http.StatusNotFound, "SmartAccounts master-detail run was not found for this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts master-detail resume is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, issue)
}

// GetSmartAccountsBrowserMasterDetailOwnerStatus returns only safe metadata
// for an existing run. It never returns the raw capability, source records,
// browser evidence, cookies, or bridge credentials.
// @Summary Get owner-safe SmartAccounts master-detail status
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Master-detail run ID"
// @Success 200 {object} smartaccountssync.BrowserMasterDetailStatus
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-master-details/{runID} [get]
func (h *Handlers) GetSmartAccountsBrowserMasterDetailOwnerStatus(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserMasterDetailService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts master-detail relay is not configured")
		return
	}
	if _, ok := masterDetailOwnerClaims(w, r); !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail status")
		return
	}
	status, err := h.smartAccountsBrowserMasterDetailService.OwnerStatus(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")))
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailUnauthorized) {
		respondError(w, http.StatusNotFound, "SmartAccounts master-detail run was not found for this tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts master-detail status is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// OptionsSmartAccountsBrowserMasterDetail permits only the locally installed
// Brave extension to use its memory-only scoped relay capability.
func (h *Handlers) OptionsSmartAccountsBrowserMasterDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSmartAccountsBrowserMasterDetail is capability-scoped relay status.
// @Summary Get SmartAccounts master-detail relay status
// @Tags SmartAccounts Sync
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Master-detail run ID"
// @Success 200 {object} smartaccountssync.BrowserMasterDetailStatus
// @Router /smartaccounts-browser-master-detail-captures/tenants/{tenantID}/runs/{runID} [get]
func (h *Handlers) GetSmartAccountsBrowserMasterDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	status, ok := h.masterDetailRelayStatus(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// UploadSmartAccountsBrowserMasterDetail relays bounded protected NDJSON only.
// It neither parses source rows nor writes target contacts, products, journals,
// invoices, or payments.
// @Summary Relay SmartAccounts master-detail protected NDJSON
// @Tags SmartAccounts Sync
// @Accept application/x-ndjson
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Master-detail run ID"
// @Param X-SA-Browser-Resource-SHA256 header string true "Lowercase raw NDJSON SHA-256"
// @Success 200 {object} smartaccountssync.BrowserMasterDetailUploadResult
// @Router /smartaccounts-browser-master-detail-captures/tenants/{tenantID}/runs/{runID}/resource [put]
func (h *Handlers) UploadSmartAccountsBrowserMasterDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserMasterDetailService == nil || r.URL.RawQuery != "" || strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]) != "application/x-ndjson" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail resource")
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts master-detail authorization failed")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
	if err != nil || len(body) == 0 {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail resource")
		return
	}
	result, err := h.smartAccountsBrowserMasterDetailService.Upload(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), token, strings.TrimSpace(r.Header.Get("X-SA-Browser-Resource-SHA256")), "application/x-ndjson", body)
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts master-detail authorization failed")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts master-detail relay failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, result)
}

// FinalizeSmartAccountsBrowserMasterDetail accepts exactly `{}` from the
// extension, then invokes the private bridge's empty-body finalizer. It never
// applies the compiled reference package.
// @Summary Finalize a SmartAccounts master-detail relay run
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Param tenantID path string true "Tenant ID"
// @Param runID path string true "Master-detail run ID"
// @Success 200 {object} smartaccountssync.BrowserMasterDetailStatus
// @Router /smartaccounts-browser-master-detail-captures/tenants/{tenantID}/runs/{runID}/finalize [post]
func (h *Handlers) FinalizeSmartAccountsBrowserMasterDetail(w http.ResponseWriter, r *http.Request) {
	if !allowBraveCaptureExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserMasterDetailService == nil || r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail finalization")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2))
	if err != nil || strings.TrimSpace(string(body)) != "{}" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail finalization")
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts master-detail authorization failed")
		return
	}
	status, err := h.smartAccountsBrowserMasterDetailService.Finalize(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), token)
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts master-detail authorization failed")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts master-detail finalization failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

func masterDetailOwnerClaims(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
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

func decodeMasterDetailRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsBrowserMasterDetailRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts master-detail request")
		return false
	}
	return true
}

func (h *Handlers) masterDetailRelayStatus(w http.ResponseWriter, r *http.Request) (smartaccountssync.BrowserMasterDetailStatus, bool) {
	if h.smartAccountsBrowserMasterDetailService == nil || r.URL.RawQuery != "" {
		respondError(w, http.StatusUnauthorized, "SmartAccounts master-detail authorization failed")
		return smartaccountssync.BrowserMasterDetailStatus{}, false
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts master-detail authorization failed")
		return smartaccountssync.BrowserMasterDetailStatus{}, false
	}
	status, err := h.smartAccountsBrowserMasterDetailService.Status(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "runID")), token)
	if errors.Is(err, smartaccountssync.ErrBrowserMasterDetailUnauthorized) {
		respondError(w, http.StatusUnauthorized, "SmartAccounts master-detail authorization failed")
		return smartaccountssync.BrowserMasterDetailStatus{}, false
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts master-detail status unavailable")
		return smartaccountssync.BrowserMasterDetailStatus{}, false
	}
	return status, true
}
