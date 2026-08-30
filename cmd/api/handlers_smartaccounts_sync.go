package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
)

const maxSmartAccountsSyncControlRequestBytes = 16 << 10

// DiscoverSmartAccountsSyncSources lists bridge-catalog source choices without
// contacting SmartAccounts or retrieving source data.
// @Summary Discover SmartAccounts bridge sources
// @Description Return configured bridge catalog metadata only. This endpoint never contacts SmartAccounts or reveals credentials.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {object} smartaccountssync.SourceDiscovery
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/sources [get]
func (h *Handlers) DiscoverSmartAccountsSyncSources(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsSyncService == nil {
		respondError(w, http.StatusInternalServerError, "SmartAccounts sync service is not configured")
		return
	}
	discovery, err := h.smartAccountsSyncService.DiscoverSources(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")))
	if errors.Is(err, smartaccountssync.ErrBridgeDiscoveryUnavailable) {
		respondError(w, http.StatusServiceUnavailable, "Verified SmartAccounts bridge discovery is not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to discover SmartAccounts bridge sources")
		return
	}
	respondJSON(w, http.StatusOK, discovery)
}

// GetSmartAccountsSyncStatus returns safe tenant-scoped control and dry-run
// status. Opaque secret references are deliberately excluded. The source ID is
// required to prevent an implicit all-source status lookup.
// @Summary Get SmartAccounts sync status
// @Description Return control, capture, plan, reconciliation, and financial-apply gate status without source data or secret references.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param source_company_id query string true "Bridge-verified source company ID"
// @Success 200 {object} smartaccountssync.SyncStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/status [get]
func (h *Handlers) GetSmartAccountsSyncStatus(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsSyncService == nil {
		respondError(w, http.StatusInternalServerError, "SmartAccounts sync service is not configured")
		return
	}
	sourceCompanyID := strings.TrimSpace(r.URL.Query().Get("source_company_id"))
	if sourceCompanyID == "" {
		respondError(w, http.StatusBadRequest, "source_company_id is required")
		return
	}
	status, err := h.smartAccountsSyncService.StatusWithCapture(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), sourceCompanyID, h.smartAccountsBridgeClient)
	if errors.Is(err, smartaccountssync.ErrBridgeClientUnavailable) {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts bridge is not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "Failed to load SmartAccounts sync status")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// ConfigureSmartAccountsSync sends transient SmartAccounts credentials only to
// the private bridge, validates one safe account request, and then stores only
// the returned opaque secret reference through the control service.
// @Summary Connect and validate SmartAccounts sync control
// @Description Send transient API credentials only to the private bridge for the selected source, validate a safe source account request, and persist only its opaque secret reference. Credentials are never echoed or stored in Open Accounting. GL authority must be SmartAccounts and invoice/payment records remain non-posting.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartaccountssync.ConnectRequest true "Transient bridge connection credentials"
// @Success 200 {object} smartaccountssync.SyncStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Failure 422 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/control [post]
func (h *Handlers) ConfigureSmartAccountsSync(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsSyncService == nil {
		respondError(w, http.StatusInternalServerError, "SmartAccounts sync service is not configured")
		return
	}
	if h.smartAccountsBridgeClient == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts bridge is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req smartaccountssync.ConnectRequest
	if !decodeSmartAccountsSyncControlRequest(w, r, &req) {
		return
	}
	// Ensure this handler does not retain credential fields after the request.
	defer func() {
		req.APIKey = ""
		req.APISecret = ""
	}()
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	if err := h.smartAccountsSyncService.ValidateConnectionRequest(r.Context(), tenantID, req); err != nil {
		if errors.Is(err, smartaccountssync.ErrBridgeDiscoveryUnavailable) {
			respondError(w, http.StatusServiceUnavailable, "Verified SmartAccounts bridge discovery is not configured")
			return
		}
		respondError(w, http.StatusUnprocessableEntity, "Invalid SmartAccounts sync control")
		return
	}
	connection, err := h.smartAccountsBridgeClient.ConnectAndValidate(r.Context(), tenantID, smartaccountssync.BridgeCredentials{
		APIKey:    req.APIKey,
		APISecret: req.APISecret,
	})
	// The service never needs raw credentials; clear their local copies before
	// validation of the returned opaque reference and persistence.
	req.APIKey = ""
	req.APISecret = ""
	if errors.Is(err, smartaccountssync.ErrBridgeClientUnavailable) {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts bridge is not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "SmartAccounts bridge connection or validation failed")
		return
	}
	status, err := h.smartAccountsSyncService.ConfigureBridgeConnection(r.Context(), tenantID, claims.UserID, req, connection)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Could not persist SmartAccounts sync control")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// RequestSmartAccountsSyncDryRun starts or resumes a private, read-only
// bridge capture. It receives safe progress only and never creates a plan,
// journal, invoice, payment, or any other accounting record.
// @Summary Request SmartAccounts dry-run capture
// @Description Start or resume a private bridge read-only capture for one configured binding and return safe progress only. No source records are exposed and no accounting data is written.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param source_company_id query string true "Bridge-derived source company ID"
// @Param request body smartaccountssync.CaptureRequest true "Explicit read-only capture scope (full_history or date window)"
// @Success 200 {object} smartaccountssync.SyncStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/dry-run [post]
func (h *Handlers) RequestSmartAccountsSyncDryRun(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsSyncService == nil {
		respondError(w, http.StatusInternalServerError, "SmartAccounts sync service is not configured")
		return
	}
	if h.smartAccountsBridgeClient == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts bridge is not configured")
		return
	}
	var captureRequest smartaccountssync.CaptureRequest
	if !decodeSmartAccountsSyncControlRequest(w, r, &captureRequest) {
		return
	}
	sourceCompanyID := strings.TrimSpace(r.URL.Query().Get("source_company_id"))
	if sourceCompanyID == "" {
		respondError(w, http.StatusBadRequest, "source_company_id is required")
		return
	}
	status, err := h.smartAccountsSyncService.StartCapture(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), sourceCompanyID, captureRequest, h.smartAccountsBridgeClient)
	if errors.Is(err, smartaccountssync.ErrControlNotConfigured) {
		respondError(w, http.StatusConflict, "Configure SmartAccounts sync control before requesting a dry run")
		return
	}
	if errors.Is(err, smartaccountssync.ErrBridgeClientUnavailable) {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts bridge is not configured")
		return
	}
	if err != nil {
		respondError(w, http.StatusBadGateway, "Failed to start SmartAccounts capture")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

// ConfirmSmartAccountsFinancialApply is intentionally the only future entry
// point for financial apply. v1 requires confirm=true and still returns a
// blocked status because no bridge executor or live capture exists.
// @Summary Confirm SmartAccounts financial apply
// @Description Require an explicit confirmation before any future financial apply. This v1 control endpoint never starts financial writes and remains blocked until live capture, planning, and reconciliation review exist.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param source_company_id query string true "Bridge-verified source company ID"
// @Param request body smartaccountssync.ConfirmApplyRequest true "Explicit confirmation"
// @Success 200 {object} smartaccountssync.SyncStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 409 {object} smartaccountssync.SyncStatus
// @Router /tenants/{tenantID}/smartaccounts-sync/apply [post]
func (h *Handlers) ConfirmSmartAccountsFinancialApply(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsSyncService == nil {
		respondError(w, http.StatusInternalServerError, "SmartAccounts sync service is not configured")
		return
	}
	var req smartaccountssync.ConfirmApplyRequest
	if !decodeSmartAccountsSyncControlRequest(w, r, &req) {
		return
	}
	sourceCompanyID := strings.TrimSpace(r.URL.Query().Get("source_company_id"))
	if sourceCompanyID == "" {
		respondError(w, http.StatusBadRequest, "source_company_id is required")
		return
	}
	status, err := h.smartAccountsSyncService.ConfirmFinancialApply(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), sourceCompanyID, req)
	if errors.Is(err, smartaccountssync.ErrExplicitConfirmation) {
		respondError(w, http.StatusBadRequest, "Explicit financial apply confirmation is required")
		return
	}
	if errors.Is(err, smartaccountssync.ErrFinancialApplyUnavailable) {
		respondJSON(w, http.StatusConflict, status)
		return
	}
	if errors.Is(err, smartaccountssync.ErrControlNotConfigured) {
		respondError(w, http.StatusConflict, "Configure SmartAccounts sync control before confirming financial apply")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to confirm SmartAccounts financial apply")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func decodeSmartAccountsSyncControlRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsSyncControlRequestBytes)
	if err := decodeJSON(r, target); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}
