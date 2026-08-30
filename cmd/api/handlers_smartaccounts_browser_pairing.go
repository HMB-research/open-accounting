package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
)

const maxSmartAccountsBrowserPairingClaimBytes = 1024

var braveExtensionOriginPattern = regexp.MustCompile(`^chrome-extension://[a-p]{32}$`)

type smartAccountsBrowserPairingClaimRequest struct {
	SourceCompanyID string `json:"source_company_id"`
}

type smartAccountsBrowserPairingClaimResponse struct {
	Status string `json:"status"`
}

// IssueSmartAccountsBrowserPairing creates a ten-minute, one-time pairing
// token for a locally installed Brave relay. The raw token is returned once to
// the authenticated OA page and is neither persisted nor logged.
// @Summary Issue SmartAccounts Brave pairing
// @Description Create a short-lived, one-time Brave browser pairing. It authorizes only an opaque source-company selector claim; it cannot transfer SmartAccounts records or create accounting transactions.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 201 {object} smartaccountssync.BrowserPairingIssue
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-pairings [post]
func (h *Handlers) IssueSmartAccountsBrowserPairing(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserPairingService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave pairing is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	issue, err := h.smartAccountsBrowserPairingService.Issue(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(claims.UserID))
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave pairing is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, issue)
}

// GetSmartAccountsBrowserPairing returns an authenticated owner's safe pairing
// state. It deliberately excludes its raw token and token hash.
// @Summary Get SmartAccounts Brave pairing status
// @Description Return a tenant-scoped Brave pairing state without browser credentials, tokens, source records, or secret references.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param pairingID path string true "Pairing ID"
// @Success 200 {object} smartaccountssync.BrowserPairingStatus
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/smartaccounts-sync/browser-pairings/{pairingID} [get]
func (h *Handlers) GetSmartAccountsBrowserPairing(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsBrowserPairingService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave pairing is not configured")
		return
	}
	status, err := h.smartAccountsBrowserPairingService.Status(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "pairingID")))
	if errors.Is(err, smartaccountssync.ErrBrowserPairingNotClaimable) {
		respondError(w, http.StatusNotFound, "SmartAccounts Brave pairing was not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave pairing is unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// OptionsSmartAccountsBrowserPairingClaim explicitly supports the Brave
// extension preflight. Browser origins and ordinary web origins are rejected.
func (h *Handlers) OptionsSmartAccountsBrowserPairingClaim(w http.ResponseWriter, r *http.Request) {
	if !allowBraveExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ClaimSmartAccountsBrowserPairing consumes a high-entropy one-time token and
// records only the opaque source UI selector under its intended OA tenant. The
// response reveals neither the source ID nor any SmartAccounts browser state.
// @Summary Claim SmartAccounts Brave pairing
// @Description Brave-extension-only, one-time pairing claim. A pairing token can bind one opaque SmartAccounts UI company selector to its issuing tenant. It does not capture source data or create accounting records.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Param pairingID path string true "Pairing ID"
// @Param request body smartAccountsBrowserPairingClaimRequest true "Opaque source-company selector"
// @Success 200 {object} smartAccountsBrowserPairingClaimResponse
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /smartaccounts-browser-pairings/{pairingID}/claim [post]
func (h *Handlers) ClaimSmartAccountsBrowserPairing(w http.ResponseWriter, r *http.Request) {
	if !allowBraveExtensionOrigin(w, r) {
		respondError(w, http.StatusForbidden, "Brave extension origin required")
		return
	}
	if h.smartAccountsBrowserPairingService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts Brave pairing is unavailable")
		return
	}
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave pairing claim")
		return
	}
	pairingToken, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave pairing claim failed")
		return
	}
	var request smartAccountsBrowserPairingClaimRequest
	if !decodeSmartAccountsBrowserPairingClaim(w, r, &request) {
		return
	}
	_, err := h.smartAccountsBrowserPairingService.Claim(r.Context(), strings.TrimSpace(chi.URLParam(r, "pairingID")), pairingToken, strings.TrimSpace(request.SourceCompanyID))
	if err != nil {
		// Preserve one-time-token confidentiality and pairing enumeration safety.
		respondError(w, http.StatusUnauthorized, "SmartAccounts Brave pairing claim failed")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, smartAccountsBrowserPairingClaimResponse{Status: smartaccountssync.BrowserPairingStatusClaimed})
}

func allowBraveExtensionOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if !braveExtensionOriginPattern.MatchString(origin) {
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", http.MethodPost+", "+http.MethodOptions)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Max-Age", "600")
	w.Header().Set("Vary", "Origin")
	return true
}

func bearerToken(value string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(value, prefix)
	if token == "" || strings.TrimSpace(token) != token || strings.ContainsAny(token, "\r\n\t ") {
		return "", false
	}
	return token, true
}

func decodeSmartAccountsBrowserPairingClaim(w http.ResponseWriter, r *http.Request, target *smartAccountsBrowserPairingClaimRequest) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsBrowserPairingClaimBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts Brave pairing claim")
		return false
	}
	return true
}

func isJSONContentType(value string) bool {
	mediaType := strings.TrimSpace(strings.Split(value, ";")[0])
	return mediaType == "application/json"
}
