package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreconciliation"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/go-chi/chi/v5"
)

const maxSmartAccountsReconciliationRequestBytes = 8 << 10

type smartAccountsReconciliationEvaluationResponse struct {
	Evaluation *smartaccountsreconciliation.Evaluation `json:"evaluation"`
	Reused     bool                                    `json:"reused"`
}

// EvaluateSmartAccountsReconciliation creates an immutable, digest-only
// technical snapshot for one original 081 selected source. The action starts
// no capture and applies no financial/reference data.
// @Summary Evaluate SmartAccounts selected source reconciliation
// @Description Owner-only. Derives a safe technical evidence snapshot from the current staged package, durable GL/reference state, and reconciliation proof seam. It never accepts source rows, proof data, amounts, notes, or financial confirmation.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable selected/all onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Success 200 {object} smartAccountsReconciliationEvaluationResponse
// @Success 201 {object} smartAccountsReconciliationEvaluationResponse
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/sources/{sourceCompanyID}/reconciliation [post]
func (h *Handlers) EvaluateSmartAccountsReconciliation(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsReconciliationOwner(w, r)
	if !ok {
		return
	}
	var empty struct{}
	if !decodeSmartAccountsReconciliationJSON(w, r, &empty) {
		return
	}
	evaluation, created, err := h.smartAccountsReconciliationService.Evaluate(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "batchID")), strings.TrimSpace(chi.URLParam(r, "sourceCompanyID")))
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	respondJSON(w, status, smartAccountsReconciliationEvaluationResponse{Evaluation: evaluation, Reused: !created})
}

// GetSmartAccountsReconciliation returns a current owner-scoped evaluation.
// It fail-closes to NOT_EVALUATED when package/preview/scope state changed
// after an earlier pass instead of displaying stale approval as current.
// @Summary Get SmartAccounts selected source reconciliation
// @Description Owner-only safe status. Returns no source records, proofs, monetary values, identities, or capabilities.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable selected/all onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Success 200 {object} smartaccountsreconciliation.Evaluation
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/sources/{sourceCompanyID}/reconciliation [get]
func (h *Handlers) GetSmartAccountsReconciliation(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsReconciliationOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts reconciliation status")
		return
	}
	evaluation, err := h.smartAccountsReconciliationService.GetForOwner(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "batchID")), strings.TrimSpace(chi.URLParam(r, "sourceCompanyID")))
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, evaluation)
}

// GetSmartAccountsReconciliationRollup summarizes exactly the original
// immutable selected/all batch members; it never uses a caller-provided list.
// @Summary Get SmartAccounts selected/all reconciliation rollup
// @Description Owner-only aggregate counts and status for original selected sources. PASS requires every original source to pass independent accountant attestation.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable selected/all onboarding batch ID"
// @Success 200 {object} smartaccountsreconciliation.Rollup
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/reconciliation [get]
func (h *Handlers) GetSmartAccountsReconciliationRollup(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsReconciliationOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts reconciliation rollup")
		return
	}
	rollup, err := h.smartAccountsReconciliationService.Rollup(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "batchID")))
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, rollup)
}

// GetSmartAccountsFullClaimEligibility reports whether the current immutable
// selected/all batch can truthfully be described as a full claim. It is a
// count-only coverage/readiness gate, distinct from a reconciliation roll-up
// and incapable of applying, approving, or importing anything.
// @Summary Get SmartAccounts selected/all full-claim eligibility
// @Description Owner-only read-only status. Requires every original selected source to have a current reconciliation PASS and reports only fixed matrix blocker codes and counts; it never returns source identities, rows, proof material, amounts, digests, or capabilities.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable selected/all onboarding batch ID"
// @Success 200 {object} smartaccountsreconciliation.FullClaimStatus
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/full-claim-eligibility [get]
func (h *Handlers) GetSmartAccountsFullClaimEligibility(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsReconciliationOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts full-claim eligibility status")
		return
	}
	status, err := h.smartAccountsReconciliationService.FullClaimStatus(r.Context(), strings.TrimSpace(claims.UserID), strings.TrimSpace(chi.URLParam(r, "batchID")))
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, status)
}

// GetSmartAccountsTolerancePolicyCandidate derives the only currently
// supported conservative tolerance candidate. It is a read-only interactive
// accountant action: no raw financial values, arbitrary tolerance digest, or
// policy rule can enter this boundary.
// @Summary Derive SmartAccounts exact-match tolerance candidate
// @Description Accountant-only. Derives the current exact-match zero-variance candidate from staged package, preview, scope, and currency metadata; it returns only algorithm version, label, and digest.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param sourceCompanyID path string true "Opaque paired source ID"
// @Param request body smartaccountsreconciliation.TolerancePolicyCandidateRequest true "Existing package and preview"
// @Success 200 {object} smartaccountsreconciliation.TolerancePolicyCandidate
// @Router /tenants/{tenantID}/smartaccounts-sync/sources/{sourceCompanyID}/tolerance-policy-candidates [post]
func (h *Handlers) GetSmartAccountsTolerancePolicyCandidate(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsTolerancePolicyService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation policy service is not configured")
		return
	}
	claims, ok := h.requireSmartAccountsInteractiveAccountant(w, r)
	if !ok {
		return
	}
	var request smartaccountsreconciliation.TolerancePolicyCandidateRequest
	if !decodeSmartAccountsReconciliationJSON(w, r, &request) {
		return
	}
	candidate, err := h.smartAccountsTolerancePolicyService.Candidate(r.Context(), claims.Role, strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "sourceCompanyID")), request)
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, candidate)
}

// ApproveSmartAccountsTolerancePolicy records an accountant-only immutable
// policy handle. Its package scope and exact-match candidate are re-derived
// from staged OA state; the request cannot supply a scope, rate, rule, or
// arbitrary tolerance digest.
// @Summary Approve SmartAccounts reconciliation tolerance policy
// @Description Accountant-only. Registers an opaque already-approved tolerance policy digest for one server-derived staged package scope; it does not apply journals.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param sourceCompanyID path string true "Opaque paired source ID"
// @Param request body smartaccountsreconciliation.TolerancePolicyApprovalRequest true "Confirmed current server-derived candidate"
// @Success 200 {object} smartaccountsreconciliation.TolerancePolicy
// @Success 201 {object} smartaccountsreconciliation.TolerancePolicy
// @Router /tenants/{tenantID}/smartaccounts-sync/sources/{sourceCompanyID}/tolerance-policies [post]
func (h *Handlers) ApproveSmartAccountsTolerancePolicy(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsTolerancePolicyService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation policy service is not configured")
		return
	}
	claims, ok := h.requireSmartAccountsInteractiveAccountant(w, r)
	if !ok {
		return
	}
	var request smartaccountsreconciliation.TolerancePolicyApprovalRequest
	if !decodeSmartAccountsReconciliationJSON(w, r, &request) {
		return
	}
	policy, created, err := h.smartAccountsTolerancePolicyService.Approve(r.Context(), strings.TrimSpace(claims.UserID), claims.Role, strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "sourceCompanyID")), request)
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	respondJSON(w, status, policy)
}

// ResolveSmartAccountsTolerancePolicy returns the currently approved, exact
// policy handle for a stored preview. This lets a different owner financial
// actor apply the preview without copying an arbitrary digest from an
// accountant screen.
// @Summary Resolve approved SmartAccounts exact-match policy
// @Description Owner- or accountant-only. Resolves the current immutable policy for an existing staged package and preview; returns only ID, algorithm, label, digest, and approval time.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param sourceCompanyID path string true "Opaque paired source ID"
// @Param request body smartaccountsreconciliation.TolerancePolicyCandidateRequest true "Existing package and preview"
// @Success 200 {object} smartaccountsreconciliation.ResolvedTolerancePolicy
// @Router /tenants/{tenantID}/smartaccounts-sync/sources/{sourceCompanyID}/tolerance-policy-resolutions [post]
func (h *Handlers) ResolveSmartAccountsTolerancePolicy(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsTolerancePolicyService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation policy service is not configured")
		return
	}
	if _, ok := h.requireSmartAccountsInteractiveOwnerOrAccountant(w, r); !ok {
		return
	}
	var request smartaccountsreconciliation.TolerancePolicyCandidateRequest
	if !decodeSmartAccountsReconciliationJSON(w, r, &request) {
		return
	}
	policy, err := h.smartAccountsTolerancePolicyService.Resolve(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "sourceCompanyID")), request)
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, policy)
}

// GetSmartAccountsTenantReconciliation lets the independent accountant read
// only the current safe evaluation handles needed for a later attestation.
// Tenant, batch, and opaque source are all path-bound; the service rejects a
// known evaluation in any other tenant and returns a safe stale state rather
// than exposing an earlier PASS after current evidence changes.
// @Summary Get current SmartAccounts reconciliation for accountant review
// @Description Accountant-only safe handoff. Returns status, fixed blockers, dates, counts, and digest/ID handles for one exact tenant, immutable batch, and opaque source. It never returns actors, source rows, proof payloads, monetary values, mappings, names, or capabilities.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param batchID path string true "Immutable selected/all onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Success 200 {object} smartaccountsreconciliation.Evaluation
// @Router /tenants/{tenantID}/smartaccounts-sync/reconciliation/batches/{batchID}/sources/{sourceCompanyID} [get]
func (h *Handlers) GetSmartAccountsTenantReconciliation(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsReconciliationService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation service is not configured")
		return
	}
	if _, ok := h.requireSmartAccountsInteractiveAccountant(w, r); !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts reconciliation status")
		return
	}
	evaluation, err := h.smartAccountsReconciliationService.GetForTenant(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "batchID")), strings.TrimSpace(chi.URLParam(r, "sourceCompanyID")))
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, evaluation)
}

// ApproveSmartAccountsReconciliation is a separate accountant attestation of
// an immutable current evidence digest. The handler passes the URL tenant to
// the service, which rejects a known cross-tenant evaluation ID.
// @Summary Attest SmartAccounts reconciliation evidence
// @Description Accountant-only and actor-separated. Confirms an exact current evidence and tolerance digest; it cannot create postings or accept raw proof/amount values.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param evaluationID path string true "Evaluation ID"
// @Param request body smartaccountsreconciliation.ApprovalRequest true "Exact digest confirmation"
// @Success 200 {object} smartaccountsreconciliation.Evaluation
// @Router /tenants/{tenantID}/smartaccounts-sync/reconciliation/evaluations/{evaluationID}/approval [post]
func (h *Handlers) ApproveSmartAccountsReconciliation(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsReconciliationService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation service is not configured")
		return
	}
	claims, ok := h.requireSmartAccountsInteractiveAccountant(w, r)
	if !ok {
		return
	}
	var request smartaccountsreconciliation.ApprovalRequest
	if !decodeSmartAccountsReconciliationJSON(w, r, &request) {
		return
	}
	evaluation, _, err := h.smartAccountsReconciliationService.Approve(r.Context(), strings.TrimSpace(claims.UserID), claims.Role, strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "evaluationID")), request)
	if h.respondSmartAccountsReconciliationError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, evaluation)
}

func (h *Handlers) requireSmartAccountsReconciliationOwner(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	if h.smartAccountsReconciliationService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation service is not configured")
		return nil, false
	}
	return h.requireSmartAccountsBrowserOnboardingOwner(w, r)
}

func (h *Handlers) requireSmartAccountsInteractiveAccountant(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return nil, false
	}
	if claims.TokenKind != auth.TokenKindAccessToken {
		respondError(w, http.StatusForbidden, "Interactive accountant session required")
		return nil, false
	}
	if claims.Role != tenant.RoleAccountant {
		respondError(w, http.StatusForbidden, "Accountant permission required")
		return nil, false
	}
	return claims, true
}

// requireSmartAccountsInteractiveOwnerOrAccountant is deliberately narrower
// than normal tenant settings access. A policy resolution is a human-session
// handoff between accountant and financial actor, never an API-token lookup.
func (h *Handlers) requireSmartAccountsInteractiveOwnerOrAccountant(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok || claims == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return nil, false
	}
	if claims.TokenKind != auth.TokenKindAccessToken {
		respondError(w, http.StatusForbidden, "Interactive owner or accountant session required")
		return nil, false
	}
	if claims.Role != tenant.RoleOwner && claims.Role != tenant.RoleAccountant {
		respondError(w, http.StatusForbidden, "Owner or accountant permission required")
		return nil, false
	}
	return claims, true
}

func decodeSmartAccountsReconciliationJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts reconciliation request")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsReconciliationRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts reconciliation request")
		return false
	}
	return true
}

func (h *Handlers) respondSmartAccountsReconciliationError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, smartaccountsreconciliation.ErrNotFound):
		respondError(w, http.StatusNotFound, "SmartAccounts reconciliation evidence was not found")
	case errors.Is(err, smartaccountsreconciliation.ErrInvalid):
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts reconciliation request")
	case errors.Is(err, smartaccountsreconciliation.ErrAccountantRequired), errors.Is(err, smartaccountsreconciliation.ErrActorSeparation):
		respondError(w, http.StatusForbidden, "Independent accountant approval is required")
	case errors.Is(err, smartaccountsreconciliation.ErrConflict), errors.Is(err, smartaccountsreconciliation.ErrNotReady):
		respondError(w, http.StatusConflict, "SmartAccounts reconciliation is not ready for this action")
	default:
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation is unavailable")
	}
	return true
}
