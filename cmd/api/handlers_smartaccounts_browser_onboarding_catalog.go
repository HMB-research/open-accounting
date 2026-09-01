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
)

const (
	maxSmartAccountsBrowserOnboardingCatalogIssueBytes   = 2048
	maxSmartAccountsBrowserOnboardingCatalogHandoffBytes = 128 << 10
	maxSmartAccountsBrowserOnboardingBatchBytes          = 32 << 10
)

// IssueSmartAccountsBrowserOnboardingCatalog creates a browser-only,
// short-lived capability for the relay to hand off a visible company picker.
// @Summary Issue SmartAccounts browser company catalog capability
// @Description Browser-only, owner-authenticated capability issuance. The returned raw token is one-time response material for the relay and is never stored, logged, accepted by the OA CLI, or usable for source-data transfer.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body smartaccountssync.BrowserOnboardingCatalogIssueRequest true "Fresh visible-company-catalog consent"
// @Success 201 {object} smartaccountssync.BrowserOnboardingCatalogIssue
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/catalogs [post]
func (h *Handlers) IssueSmartAccountsBrowserOnboardingCatalog(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserOnboardingCatalogService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser company catalog is unavailable")
		return
	}
	claims, ok := h.requireSmartAccountsBrowserOnboardingOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser company catalog request")
		return
	}
	var request smartaccountssync.BrowserOnboardingCatalogIssueRequest
	if !decodeSmartAccountsBrowserOnboardingJSON(w, r, maxSmartAccountsBrowserOnboardingCatalogIssueBytes, &request, "Invalid SmartAccounts browser company catalog request") {
		return
	}
	issue, err := h.smartAccountsBrowserOnboardingCatalogService.Issue(r.Context(), strings.TrimSpace(claims.UserID), request)
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserOnboardingCatalogInvalid):
		respondError(w, http.StatusBadRequest, "Fresh visible-company catalog consent is required")
	case err != nil:
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser company catalog is unavailable")
	default:
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, http.StatusCreated, issue)
	}
}

// GetSmartAccountsBrowserOnboardingCatalog exposes accepted picker metadata
// only to its issuing owner. The relay postMessage result deliberately stays
// digest/count-only; this authenticated route lets the owner select explicitly.
// @Summary Get an accepted SmartAccounts browser company catalog
// @Description Browser-only owner status. Returns relay-observed picker metadata and receipt expiry, but never a capability, nonce, cookie, credential, source row, or financial instruction.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param catalogID path string true "Catalog receipt ID"
// @Success 200 {object} smartaccountssync.BrowserOnboardingCatalogStatus
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/catalogs/{catalogID} [get]
func (h *Handlers) GetSmartAccountsBrowserOnboardingCatalog(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserOnboardingCatalogService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser company catalog is unavailable")
		return
	}
	claims, ok := h.requireSmartAccountsBrowserOnboardingOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser company catalog status request")
		return
	}
	status, err := h.smartAccountsBrowserOnboardingCatalogService.Status(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "catalogID")))
	if errors.Is(err, smartaccountssync.ErrBrowserOnboardingCatalogUnauthorized) {
		respondError(w, http.StatusNotFound, "SmartAccounts browser company catalog was not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser company catalog is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// OptionsSmartAccountsBrowserOnboardingCatalogHandoff supports the extension
// worker's direct, no-cookie catalog handoff.
func (h *Handlers) OptionsSmartAccountsBrowserOnboardingCatalogHandoff(w http.ResponseWriter, r *http.Request) {
	if !allowSmartAccountsMetadataRelayOrigin(w, r) {
		respondError(w, http.StatusForbidden, "SmartAccounts catalog relay origin required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandoffSmartAccountsBrowserOnboardingCatalog persists the exact relay
// snapshot bound to a short-lived browser capability. It accepts metadata
// only; no source records, cookies, SmartAccounts credentials, bridge token,
// or financial instruction can cross this route.
// @Summary Handoff SmartAccounts browser company catalog
// @Description Brave-extension-only, no-cookie relay handoff. The opaque Bearer capability is bound to one issued catalog/workflow/nonce and a strict visible-company metadata digest. This endpoint cannot be called by OA CLI clients.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Param catalogID path string true "Catalog receipt ID"
// @Param request body smartaccountssync.BrowserOnboardingCatalogHandoff true "Strict relay catalog metadata"
// @Success 200 {object} smartaccountssync.BrowserOnboardingCatalogHandoffStatus
// @Success 201 {object} smartaccountssync.BrowserOnboardingCatalogHandoffStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-browser-onboarding/catalogs/{catalogID}/handoff [post]
func (h *Handlers) HandoffSmartAccountsBrowserOnboardingCatalog(w http.ResponseWriter, r *http.Request) {
	if !allowSmartAccountsMetadataRelayOrigin(w, r) {
		respondError(w, http.StatusForbidden, "SmartAccounts catalog relay origin required")
		return
	}
	if h.smartAccountsBrowserOnboardingCatalogService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser company catalog is unavailable")
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser company catalog handoff")
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts browser company catalog handoff failed")
		return
	}
	var handoff smartaccountssync.BrowserOnboardingCatalogHandoff
	if !decodeSmartAccountsBrowserOnboardingJSON(w, r, maxSmartAccountsBrowserOnboardingCatalogHandoffBytes, &handoff, "Invalid SmartAccounts browser company catalog handoff") {
		return
	}
	status, err := h.smartAccountsBrowserOnboardingCatalogService.Handoff(r.Context(), strings.TrimSpace(chi.URLParam(r, "catalogID")), token, handoff)
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserOnboardingCatalogUnauthorized):
		respondError(w, http.StatusUnauthorized, "SmartAccounts browser company catalog handoff failed")
	case errors.Is(err, smartaccountssync.ErrBrowserOnboardingCatalogInvalid):
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser company catalog handoff")
	case errors.Is(err, smartaccountssync.ErrBrowserOnboardingCatalogConflict):
		respondError(w, http.StatusConflict, "SmartAccounts browser company catalog handoff conflicts with the issued receipt")
	case err != nil:
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser company catalog is unavailable")
	default:
		w.Header().Set("Cache-Control", "no-store")
		responseStatus := http.StatusCreated
		if status.Status == "already_accepted" {
			responseStatus = http.StatusOK
		}
		respondJSON(w, responseStatus, status)
	}
}

// StartSmartAccountsBrowserOnboardingBatch creates or retries a durable
// owner-confirmed selection from a trusted relay catalog receipt. It starts no
// capture and never performs financial writes.
// @Summary Start selected/all SmartAccounts browser onboarding batch
// @Description Browser-only owner action. Requires an accepted relay-observed catalog receipt and explicit selected/all intent. All must list exactly the observed set; selected must be a strict nonempty subset. It only creates/reuses targets and expected-source pairings.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body smartaccountssync.BrowserOnboardingBatchRequest true "Catalog receipt, explicit mode, and selected opaque IDs"
// @Success 200 {object} smartaccountssync.BrowserOnboardingBatchResponse
// @Success 201 {object} smartaccountssync.BrowserOnboardingBatchResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches [post]
func (h *Handlers) StartSmartAccountsBrowserOnboardingBatch(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserOnboardingBatchService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is not configured")
		return
	}
	claims, ok := h.requireSmartAccountsBrowserOnboardingOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser onboarding batch request")
		return
	}
	var request smartaccountssync.BrowserOnboardingBatchRequest
	if !decodeSmartAccountsBrowserOnboardingJSON(w, r, maxSmartAccountsBrowserOnboardingBatchBytes, &request, "Invalid SmartAccounts browser onboarding batch request") {
		return
	}
	response, err := h.smartAccountsBrowserOnboardingBatchService.Start(r.Context(), strings.TrimSpace(claims.UserID), request)
	h.respondSmartAccountsBrowserOnboardingBatch(w, response, err, true)
}

// GetSmartAccountsBrowserOnboardingBatch returns durable safe progress only.
// @Summary Get SmartAccounts browser onboarding batch progress
// @Description Browser-only owner status. Returns no raw pairing capability and never starts capture, discovery, schema review, package delivery, or financial application.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Batch ID"
// @Success 200 {object} smartaccountssync.BrowserOnboardingBatchResponse
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID} [get]
func (h *Handlers) GetSmartAccountsBrowserOnboardingBatch(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserOnboardingBatchService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is not configured")
		return
	}
	claims, ok := h.requireSmartAccountsBrowserOnboardingOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser onboarding batch status request")
		return
	}
	response, err := h.smartAccountsBrowserOnboardingBatchService.Status(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "batchID")))
	h.respondSmartAccountsBrowserOnboardingBatch(w, response, err, false)
}

// ResumeSmartAccountsBrowserOnboardingBatch reissues pairing capabilities only
// after another explicit owner action for the already immutable batch.
// @Summary Resume SmartAccounts browser onboarding batch
// @Description Browser-only owner action. It cannot change the selected sources or observed catalog and never starts capture or financial application. Any raw pairing capability is response-only.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Batch ID"
// @Param request body smartaccountssync.BrowserOnboardingBatchResumeRequest true "Fresh owner confirmation"
// @Success 200 {object} smartaccountssync.BrowserOnboardingBatchResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/resume [post]
func (h *Handlers) ResumeSmartAccountsBrowserOnboardingBatch(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserOnboardingBatchService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is not configured")
		return
	}
	claims, ok := h.requireSmartAccountsBrowserOnboardingOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser onboarding batch resume request")
		return
	}
	var request smartaccountssync.BrowserOnboardingBatchResumeRequest
	if !decodeSmartAccountsBrowserOnboardingJSON(w, r, maxSmartAccountsBrowserOnboardingBatchBytes, &request, "Invalid SmartAccounts browser onboarding batch resume request") {
		return
	}
	response, err := h.smartAccountsBrowserOnboardingBatchService.Resume(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "batchID")), request)
	h.respondSmartAccountsBrowserOnboardingBatch(w, response, err, false)
}

func (h *Handlers) requireSmartAccountsBrowserOnboardingOwner(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return nil, false
	}
	if claims.TokenKind == auth.TokenKindAPIToken {
		respondError(w, http.StatusForbidden, "API tokens cannot use SmartAccounts browser onboarding")
		return nil, false
	}
	return claims, true
}

func (h *Handlers) respondSmartAccountsBrowserOnboardingBatch(w http.ResponseWriter, response *smartaccountssync.BrowserOnboardingBatchResponse, err error, create bool) {
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserOnboardingBatchNotFound):
		respondError(w, http.StatusNotFound, "SmartAccounts browser onboarding batch was not found")
	case errors.Is(err, smartaccountssync.ErrBrowserOnboardingBatchInvalid):
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser onboarding batch")
	case errors.Is(err, smartaccountssync.ErrBrowserOnboardingBatchConflict):
		respondError(w, http.StatusConflict, "SmartAccounts browser onboarding batch conflicts with its immutable selection")
	case err != nil:
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is unavailable")
	default:
		status := http.StatusOK
		if create && !response.Reused {
			status = http.StatusCreated
		}
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, status, response)
	}
}

func decodeSmartAccountsBrowserOnboardingJSON(w http.ResponseWriter, r *http.Request, limit int64, target interface{}, message string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, message)
		return false
	}
	return true
}
