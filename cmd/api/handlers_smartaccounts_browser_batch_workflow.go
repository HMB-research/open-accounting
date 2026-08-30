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

const maxSmartAccountsBrowserBatchWorkflowRequestBytes = 1 << 20

// PrepareSmartAccountsBrowserBatchWorkflow creates the non-financial 082
// phase record after the existing immutable selected/all batch has completed
// tenant creation and expected-source pairing.
// @Summary Prepare a SmartAccounts browser batch workflow
// @Description Owner-only. Records an immutable history boundary and discovery/schema consent for an already paired selected/all browser batch. It cannot capture source data or apply financial changes.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param request body smartaccountssync.BrowserBatchPreparationRequest true "Owner-confirmed workflow preparation"
// @Success 200 {object} smartaccountssync.BrowserBatchWorkflowStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow [post]
func (h *Handlers) PrepareSmartAccountsBrowserBatchWorkflow(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchPreparationRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	status, err := h.smartAccountsBrowserBatchWorkflowActions.Prepare(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), request)
	h.respondSmartAccountsBrowserBatchWorkflow(w, status, err, http.StatusOK)
}

// GetSmartAccountsBrowserBatchWorkflow returns only durable owner-safe phase
// state. It never contains a browser capability, token hash, source row,
// cookie, header, or credential.
// @Summary Get SmartAccounts browser batch workflow status
// @Description Owner-only safe aggregate and per-source phase status. GET never returns relay capabilities or financial apply instructions.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Success 200 {object} smartaccountssync.BrowserBatchWorkflowStatus
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow [get]
func (h *Handlers) GetSmartAccountsBrowserBatchWorkflow(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	if r.URL.RawQuery != "" {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser batch workflow status")
		return
	}
	status, err := h.smartAccountsBrowserBatchWorkflowActions.Status(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r))
	h.respondSmartAccountsBrowserBatchWorkflow(w, status, err, http.StatusOK)
}

// ResumeSmartAccountsBrowserBatchWorkflow releases only expired serial
// control leases. It cannot issue a relay capability or alter source/scope.
// @Summary Recover expired SmartAccounts browser batch work leases
// @Description Owner-only. Makes an elapsed exact workflow lease retryable without changing selection, target, scope, package, or financial state.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Success 200 {object} smartaccountssync.BrowserBatchWorkflowStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/resume [post]
func (h *Handlers) ResumeSmartAccountsBrowserBatchWorkflow(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var empty struct{}
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &empty) {
		return
	}
	status, err := h.smartAccountsBrowserBatchWorkflowActions.Resume(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r))
	h.respondSmartAccountsBrowserBatchWorkflow(w, status, err, http.StatusOK)
}

// AdvanceSmartAccountsBrowserBatchWorkflowSafe recovers one idempotent
// non-sensitive phase transition. It cannot issue a capability, confirm
// source transfer, create a preview, apply financial data, or approve an
// accountant decision.
// @Summary Advance one safe SmartAccounts browser batch phase
// @Description Owner-only. Advances only completed discovery into schema-review-required or every-approved schema set into transfer-confirmation-required. It requires no source transfer or financial consent and never returns a relay capability.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Success 200 {object} smartaccountssync.BrowserBatchWorkflowStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/advance-safe [post]
func (h *Handlers) AdvanceSmartAccountsBrowserBatchWorkflowSafe(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var empty struct{}
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &empty) {
		return
	}
	status, err := h.smartAccountsBrowserBatchWorkflowActions.AdvanceSafe(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r))
	h.respondSmartAccountsBrowserBatchWorkflow(w, status, err, http.StatusOK)
}

// AcquireSmartAccountsBrowserBatchDiscovery claims exactly one serialized
// source and returns a no-token discovery event only in this action response.
// @Summary Acquire next browser discovery action
// @Description Owner-only. Claims one source discovery lease and returns a same-window metadata-only discovery issue. No GET/status response contains this action issue.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param request body smartaccountssync.BrowserBatchDiscoveryAcquireRequest true "Fresh discovery consent"
// @Success 201 {object} smartaccountssync.BrowserBatchDiscoveryAction
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/discovery/acquire [post]
func (h *Handlers) AcquireSmartAccountsBrowserBatchDiscovery(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchDiscoveryAcquireRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	action, err := h.smartAccountsBrowserBatchWorkflowActions.AcquireDiscovery(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), strings.TrimSpace(claims.UserID), request)
	h.respondSmartAccountsBrowserBatchAction(w, action, err, http.StatusCreated)
}

// ReissueSmartAccountsBrowserBatchDiscovery rotates a lost relay/page action
// without waiting for its prior lease to expire. It preserves the immutable
// batch/source binding and returns the new issue only in this owner action.
// @Summary Reissue one running browser discovery action
// @Description Owner-only. Fresh consent rotates the exact running source lease and returns a new same-window discovery issue. Old lease/generation completions are rejected.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Param request body smartaccountssync.BrowserBatchDiscoveryAcquireRequest true "Fresh discovery consent"
// @Success 201 {object} smartaccountssync.BrowserBatchDiscoveryAction
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/discovery/reissue [post]
func (h *Handlers) ReissueSmartAccountsBrowserBatchDiscovery(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchDiscoveryAcquireRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	action, err := h.smartAccountsBrowserBatchWorkflowActions.ReissueDiscovery(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), smartAccountsBrowserBatchSourceID(r), strings.TrimSpace(claims.UserID), request)
	h.respondSmartAccountsBrowserBatchAction(w, action, err, http.StatusCreated)
}

// CompleteSmartAccountsBrowserBatchDiscovery persists only the existing
// redacted discovery receipt digest after the relay event passes 078 checks.
// @Summary Complete one browser batch discovery action
// @Description Owner-only. Relays an exact redacted discovery event for the claimed source/lease and stores receipt digests only. Partial discovery cannot advance this selected/all batch.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Param request body smartaccountssync.BrowserBatchDiscoveryCompleteRequest true "Claim-bound redacted relay result"
// @Success 200 {object} smartaccountssync.BrowserBatchSourceWorkflow
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/discovery/complete [post]
func (h *Handlers) CompleteSmartAccountsBrowserBatchDiscovery(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchDiscoveryCompleteRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	source, err := h.smartAccountsBrowserBatchWorkflowActions.CompleteDiscovery(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), smartAccountsBrowserBatchSourceID(r), request)
	h.respondSmartAccountsBrowserBatchAction(w, source, err, http.StatusOK)
}

// RequireSmartAccountsBrowserBatchSchemaReview remains an idempotent
// compatibility/recovery operation. Completed discovery now enters this
// state server-side, while the later owner schema approval remains separate.
// @Summary Require reviewed browser CSV schema approval
// @Description Owner-only compatibility/recovery operation. A completed full discovery normally enters the separate reviewed-schema phase server-side; this endpoint cannot select a schema or transfer source data.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Param request body smartaccountssync.BrowserBatchSchemaPhaseRequest true "Exact discovery phase generation"
// @Success 200 {object} smartaccountssync.BrowserBatchSourceWorkflow
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/schema/require [post]
func (h *Handlers) RequireSmartAccountsBrowserBatchSchemaReview(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchSchemaPhaseRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	source, err := h.smartAccountsBrowserBatchWorkflowActions.RequireSchemaReview(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), smartAccountsBrowserBatchSourceID(r), request)
	h.respondSmartAccountsBrowserBatchAction(w, source, err, http.StatusOK)
}

// RefreshSmartAccountsBrowserBatchSchemaReadiness recovers a lost response
// from an existing reviewed schema registration without creating a new review.
// @Summary Refresh browser CSV schema readiness
// @Description Owner-only. Reads an existing registered reviewed-schema receipt and advances the exact source generation only when its immutable digest matches.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Param request body smartaccountssync.BrowserBatchSchemaPhaseRequest true "Exact schema-review phase generation"
// @Success 200 {object} smartaccountssync.BrowserBatchSourceWorkflow
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/schema/refresh [post]
func (h *Handlers) RefreshSmartAccountsBrowserBatchSchemaReadiness(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchSchemaPhaseRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	source, err := h.smartAccountsBrowserBatchWorkflowActions.RefreshSchemaReadiness(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), smartAccountsBrowserBatchSourceID(r), request)
	h.respondSmartAccountsBrowserBatchAction(w, source, err, http.StatusOK)
}

// ConfirmSmartAccountsBrowserBatchSchema delegates only the explicit owner
// confirmation to the existing 080 schema registry for the reviewed General
// Ledger CSV adapter.
// @Summary Confirm reviewed browser CSV schema
// @Description Owner-only. Registers the fixed reviewed general_ledger_csv_v1 adapter for the exact completed discovery; the journal_entries summary grid is archive-only and never accepts an authoritative adapter. This endpoint never accepts headers or source data.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Param request body smartaccountssync.BrowserBatchSchemaConfirmRequest true "Explicit schema review confirmation"
// @Success 200 {object} smartaccountssync.BrowserBatchSourceWorkflow
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 502 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/schema/confirm [post]
func (h *Handlers) ConfirmSmartAccountsBrowserBatchSchema(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchSchemaConfirmRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	source, err := h.smartAccountsBrowserBatchWorkflowActions.ConfirmSchema(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), smartAccountsBrowserBatchSourceID(r), strings.TrimSpace(claims.UserID), request)
	h.respondSmartAccountsBrowserBatchAction(w, source, err, http.StatusOK)
}

// OpenSmartAccountsBrowserBatchTransferConfirmation gathers all approved
// selected sources into one immutable second confirmation boundary.
// @Summary Open SmartAccounts browser transfer confirmation
// @Description Owner-only. Opens the single source-transfer confirmation only when every source in the immutable selected/all batch has a reviewed schema digest.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Success 200 {object} smartaccountssync.BrowserBatchWorkflowStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/transfer/open [post]
func (h *Handlers) OpenSmartAccountsBrowserBatchTransferConfirmation(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var empty struct{}
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &empty) {
		return
	}
	status, err := h.smartAccountsBrowserBatchWorkflowActions.OpenTransferConfirmation(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r))
	h.respondSmartAccountsBrowserBatchWorkflow(w, status, err, http.StatusOK)
}

// ConfirmSmartAccountsBrowserBatchTransfer freezes the exact reviewed schema
// set plus server-derived date/cutoff. It does not issue a capture token.
// @Summary Confirm SmartAccounts browser source transfer scope
// @Description Owner-only. Freezes the exact partial journal scope and reviewed-schema digest for every selected source; a later capture action still needs explicit transfer consent before returning a relay capability.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param request body smartaccountssync.BrowserBatchTransferConfirmationRequest true "Matching reviewed-schema confirmation"
// @Success 200 {object} smartaccountssync.BrowserBatchWorkflowStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/transfer/confirm [post]
func (h *Handlers) ConfirmSmartAccountsBrowserBatchTransfer(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchTransferConfirmationRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	status, err := h.smartAccountsBrowserBatchWorkflowActions.ConfirmTransfer(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), request)
	h.respondSmartAccountsBrowserBatchWorkflow(w, status, err, http.StatusOK)
}

// AcquireSmartAccountsBrowserBatchCapture returns a transient capability only
// after both frozen scope and fresh owner transfer consent. The same durable
// run ID is reused after an extension restart.
// @Summary Acquire next SmartAccounts browser batch capture
// @Description Owner-only. Returns a short-lived capability only in this response for one serial immutable source/run/scope. It never posts journals or applies a preview.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param request body smartaccountssync.BrowserBatchCaptureAcquireRequest true "Fresh explicit transfer consent"
// @Success 201 {object} smartaccountssync.BrowserBatchCaptureAction
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/capture/acquire [post]
func (h *Handlers) AcquireSmartAccountsBrowserBatchCapture(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchCaptureAcquireRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	action, err := h.smartAccountsBrowserBatchWorkflowActions.AcquireCapture(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), strings.TrimSpace(claims.UserID), request)
	h.respondSmartAccountsBrowserBatchAction(w, action, err, http.StatusCreated)
}

// CompleteSmartAccountsBrowserBatchCapture checks only bridge-safe staging
// progress. Compiling is kept in CAPTURE_RUNNING; only a finalized package
// receipt can advance the source to STAGED.
// @Summary Record SmartAccounts browser batch capture staging progress
// @Description Owner-only. Reads safe bridge capture progress for the exact lease/run. A compiling package remains pollable; only a finalized package digest becomes staged. No source rows or accounting writes are accepted.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Param request body smartaccountssync.BrowserBatchCaptureCompleteRequest true "Exact capture lease generation"
// @Success 200 {object} smartaccountssync.BrowserBatchCaptureCompletion
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/capture/complete [post]
func (h *Handlers) CompleteSmartAccountsBrowserBatchCapture(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchCaptureCompleteRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	completion, err := h.smartAccountsBrowserBatchWorkflowActions.CompleteCapture(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), smartAccountsBrowserBatchSourceID(r), request)
	h.respondSmartAccountsBrowserBatchAction(w, completion, err, http.StatusOK)
}

// PreviewSmartAccountsBrowserBatchPackage requests only a staged-package
// preview. It cannot call the executor apply path; review-required previews
// remain review-required checkpoints.
// @Summary Preview a staged SmartAccounts browser batch package
// @Description Owner-only. Runs the existing non-financial staged-package preview for one exact batch source and records only preview ID/digest/status. Financial apply remains a separate explicit tenant action.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param batchID path string true "Immutable browser onboarding batch ID"
// @Param sourceCompanyID path string true "Opaque selected source ID"
// @Param request body smartaccountssync.BrowserBatchPreviewRequest true "Exact staged phase generation and chart option"
// @Success 200 {object} smartaccountssync.BrowserBatchSourceWorkflow
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 503 {object} object{error=string}
// @Router /smartaccounts-sync/browser-onboarding/batches/{batchID}/workflow/sources/{sourceCompanyID}/preview [post]
func (h *Handlers) PreviewSmartAccountsBrowserBatchPackage(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.requireSmartAccountsBrowserBatchWorkflowOwner(w, r)
	if !ok {
		return
	}
	var request smartaccountssync.BrowserBatchPreviewRequest
	if !decodeSmartAccountsBrowserBatchWorkflowRequest(w, r, &request) {
		return
	}
	source, err := h.smartAccountsBrowserBatchWorkflowActions.Preview(r.Context(), strings.TrimSpace(claims.UserID), smartAccountsBrowserBatchID(r), smartAccountsBrowserBatchSourceID(r), strings.TrimSpace(claims.UserID), request)
	h.respondSmartAccountsBrowserBatchAction(w, source, err, http.StatusOK)
}

func (h *Handlers) requireSmartAccountsBrowserBatchWorkflowOwner(w http.ResponseWriter, r *http.Request) (*auth.Claims, bool) {
	if h.smartAccountsBrowserBatchWorkflowActions == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser batch workflow is not configured")
		return nil, false
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return nil, false
	}
	if claims.TokenKind == auth.TokenKindAPIToken || strings.TrimSpace(claims.UserID) == "" {
		respondError(w, http.StatusForbidden, "User owner authentication required")
		return nil, false
	}
	return claims, true
}

func (h *Handlers) respondSmartAccountsBrowserBatchWorkflow(w http.ResponseWriter, status *smartaccountssync.BrowserBatchWorkflowStatus, err error, success int) {
	if h.respondSmartAccountsBrowserBatchWorkflowError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, success, status)
}

func (h *Handlers) respondSmartAccountsBrowserBatchAction(w http.ResponseWriter, value interface{}, err error, success int) {
	if h.respondSmartAccountsBrowserBatchWorkflowError(w, err) {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, success, value)
}

func (h *Handlers) respondSmartAccountsBrowserBatchWorkflowError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, smartaccountssync.ErrBrowserBatchWorkflowNotFound), errors.Is(err, smartaccountssync.ErrBrowserDiscoveryUnauthorized), errors.Is(err, smartaccountssync.ErrBrowserDiscoveryNotFound), errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalUnauthorized), errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalNotFound):
		respondError(w, http.StatusNotFound, "SmartAccounts browser batch workflow was not found")
	case errors.Is(err, smartaccountssync.ErrBrowserBatchWorkflowInvalid), errors.Is(err, smartaccountssync.ErrBrowserDiscoveryInvalid), errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalInvalid):
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser batch workflow request")
	case errors.Is(err, smartaccountssync.ErrBrowserBatchWorkflowConflict), errors.Is(err, smartaccountssync.ErrBrowserBatchWorkflowNotReady), errors.Is(err, smartaccountssync.ErrBrowserDiscoveryConflict), errors.Is(err, smartaccountssync.ErrBrowserCSVSchemaApprovalConflict):
		respondError(w, http.StatusConflict, "SmartAccounts browser batch workflow is not ready for this action")
	default:
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts browser batch workflow is unavailable")
	}
	return true
}

func decodeSmartAccountsBrowserBatchWorkflowRequest(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	if r.URL.RawQuery != "" || !isJSONContentType(r.Header.Get("Content-Type")) {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser batch workflow request")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsBrowserBatchWorkflowRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		respondError(w, http.StatusBadRequest, "Invalid SmartAccounts browser batch workflow request")
		return false
	}
	return true
}

func smartAccountsBrowserBatchID(r *http.Request) string {
	return strings.TrimSpace(chi.URLParam(r, "batchID"))
}

func smartAccountsBrowserBatchSourceID(r *http.Request) string {
	return strings.TrimSpace(chi.URLParam(r, "sourceCompanyID"))
}
