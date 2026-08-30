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

const maxSmartAccountsBrowserOnboardingRequestBytes = 8 << 10

// StartSmartAccountsBrowserOnboarding creates or reuses only the owner’s
// explicitly selected tenants, then returns one expected-source pairing per
// recoverable source. It performs no capture, package staging, journal write,
// invoice write, payment write, or financial apply.
// @Summary Start selected SmartAccounts Brave company onboarding
// @Description Owner-authorized, non-financial selected-company onboarding. It reserves each opaque source identifier, reuses an existing verified binding or exactly one owner tenant name, creates only missing tenants, and returns one expected-source relay pairing per independently recoverable company.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body smartaccountssync.BrowserOnboardingRequest true "Selected browser company metadata and owner authorization"
// @Success 200 {object} smartaccountssync.BrowserOnboardingResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding [post]
func (h *Handlers) StartSmartAccountsBrowserOnboarding(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserOnboardingService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if claims.TokenKind == auth.TokenKindAPIToken {
		respondError(w, http.StatusForbidden, "API tokens cannot create SmartAccounts onboarding tenants")
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser onboarding request")
		return
	}
	var request smartaccountssync.BrowserOnboardingRequest
	if !decodeSmartAccountsBrowserOnboardingRequest(w, r, &request) {
		return
	}
	response, err := h.smartAccountsBrowserOnboardingService.Start(r.Context(), strings.TrimSpace(claims.UserID), request)
	if errors.Is(err, smartaccountssync.ErrBrowserOnboardingInvalid) {
		respondError(w, http.StatusBadRequest, "Select one or more distinct SmartAccounts companies and confirm tenant creation")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, response)
}

// GetSmartAccountsBrowserOnboarding returns owner-scoped progress for one
// selected source and never exposes a raw pairing token or source data.
// @Summary Get selected SmartAccounts Brave company onboarding progress
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param sourceCompanyID path string true "Opaque browser source selector"
// @Success 200 {object} smartaccountssync.BrowserOnboardingResult
// @Failure 401 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/{sourceCompanyID} [get]
func (h *Handlers) GetSmartAccountsBrowserOnboarding(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserOnboardingService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser onboarding status request")
		return
	}
	result, err := h.smartAccountsBrowserOnboardingService.Status(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "sourceCompanyID")))
	if errors.Is(err, smartaccountssync.ErrBrowserOnboardingNotFound) {
		respondError(w, http.StatusNotFound, "SmartAccounts browser onboarding was not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser onboarding is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, result)
}

func decodeSmartAccountsBrowserOnboardingRequest(w http.ResponseWriter, r *http.Request, target *smartaccountssync.BrowserOnboardingRequest) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsBrowserOnboardingRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser onboarding request")
		return false
	}
	return true
}
