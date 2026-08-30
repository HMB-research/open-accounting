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

const maxSmartAccountsBrowserCSVSchemaReviewBytes = 1024

// ReviewSmartAccountsBrowserCSVSchema records an owner-confirmed, reviewed
// CSV schema binding for one already-authorized browser discovery resource.
// It does not start capture, relay a source request, upload CSV data, or
// create/apply financial records. OA resolves the opaque source binding only
// from its existing discovery authorization before contacting the private
// bridge.
// @Summary Register a reviewed SmartAccounts browser CSV schema
// @Description Tenant-owner-only. Accepts only explicit review confirmation for a previously authorized discovery resource. Persists and returns aggregate binding/status/digest metadata only; it never accepts or exposes source IDs, header names, source data, cookies, credentials, or bridge tokens.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param discoveryID path string true "Issued discovery UUID"
// @Param resourceID path string true "Reviewed CSV resource ID"
// @Param schemaID path string true "Reviewed schema ID"
// @Param request body smartaccountssync.BrowserCSVSchemaApprovalRequest true "Owner confirmation only"
// @Success 200 {object} smartaccountssync.BrowserCSVSchemaApprovalResponse
// @Success 201 {object} smartaccountssync.BrowserCSVSchemaApprovalResponse
// @Failure 400 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-discoveries/{discoveryID}/resources/{resourceID}/schemas/{schemaID}/review [post]
func (h *Handlers) ReviewSmartAccountsBrowserCSVSchema(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserCSVSchemaOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser CSV schema review")
		return
	}
	var request smartaccountssync.BrowserCSVSchemaApprovalRequest
	if !decodeSmartAccountsBrowserCSVSchemaReview(w, r, &request) {
		return
	}
	response, created, err := h.smartAccountsBrowserCSVSchemaApprovalService.Review(
		r.Context(),
		strings.TrimSpace(chi.URLParam(r, "tenantID")),
		strings.TrimSpace(claims.UserID),
		strings.TrimSpace(chi.URLParam(r, "discoveryID")),
		strings.TrimSpace(chi.URLParam(r, "resourceID")),
		strings.TrimSpace(chi.URLParam(r, "schemaID")),
		request,
	)
	successStatus := http.StatusOK
	if created {
		successStatus = http.StatusCreated
	}
	h.respondSmartAccountsBrowserCSVSchemaResult(w, err, response, successStatus)
}

// GetSmartAccountsBrowserCSVSchemaReview returns only the status and immutable
// approval digest of a previously registered schema. It never returns the
// private source selector or any browser/source data.
// @Summary Get a reviewed SmartAccounts browser CSV schema status
// @Description Tenant-owner-only. Returns aggregate resource/schema registration status and approval digest for an existing discovery binding only.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param discoveryID path string true "Issued discovery UUID"
// @Param resourceID path string true "Reviewed CSV resource ID"
// @Param schemaID path string true "Reviewed schema ID"
// @Success 200 {object} smartaccountssync.BrowserCSVSchemaApprovalResponse
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-discoveries/{discoveryID}/resources/{resourceID}/schemas/{schemaID}/review [get]
func (h *Handlers) GetSmartAccountsBrowserCSVSchemaReview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireSmartAccountsBrowserCSVSchemaOwner(w, r); !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser CSV schema review status")
		return
	}
	response, err := h.smartAccountsBrowserCSVSchemaApprovalService.Status(
		r.Context(),
		strings.TrimSpace(chi.URLParam(r, "tenantID")),
		strings.TrimSpace(chi.URLParam(r, "discoveryID")),
		strings.TrimSpace(chi.URLParam(r, "resourceID")),
		strings.TrimSpace(chi.URLParam(r, "schemaID")),
	)
	h.respondSmartAccountsBrowserCSVSchemaResult(w, err, response, http.StatusOK)
}

func (h *Handlers) requireSmartAccountsBrowserCSVSchemaOwner(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	if h.smartAccountsBrowserCSVSchemaApprovalService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser CSV schema review is not configured")
		return nil, false
	}
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

func (h *Handlers) respondSmartAccountsBrowserCSVSchemaResult(w http.ResponseWriter, err error, response smartaccountssync.BrowserCSVSchemaApprovalResponse, successStatus int) {
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalUnauthorized), errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalNotFound):
		respondError(w, http.StatusNotFound, "SmartAccounts browser CSV schema review was not found for this tenant")
	case errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalInvalid):
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser CSV schema review")
	case errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalConflict):
		respondError(w, http.StatusConflict, "SmartAccounts browser CSV schema review conflicts with the existing review")
	case errors.Is(err, smartaccountssync.ErrBridgeClientUnavailable):
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser CSV schema review bridge is unavailable")
	case err != nil:
		respondError(w, http.StatusBadGateway, "SmartAccounts browser CSV schema review relay failed")
	default:
		w.Header().Set("Cache-Control", "no-store")
		respondJSON(w, successStatus, response)
	}
}

func decodeSmartAccountsBrowserCSVSchemaReview(w http.ResponseWriter, r *http.Request, target *smartaccountssync.BrowserCSVSchemaApprovalRequest) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsBrowserCSVSchemaReviewBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser CSV schema review")
		return false
	}
	return true
}
