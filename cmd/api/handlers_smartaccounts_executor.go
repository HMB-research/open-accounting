package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreconciliation"
	"github.com/go-chi/chi/v5"
)

const maxSmartAccountsExecutorRequestBytes = 64 << 10

// smartAccountsPackageApplyRequest is the browser/API boundary for a
// financial apply. The caller supplies an opaque approved policy ID, never a
// copyable tolerance digest; the handler resolves its current exact binding
// before calling the internal executor.
type smartAccountsPackageApplyRequest struct {
	Confirm           bool   `json:"confirm"`
	PreviewID         string `json:"preview_id"`
	PreviewSHA256     string `json:"preview_sha256"`
	TolerancePolicyID string `json:"tolerance_policy_id"`
}

// PreviewSmartAccountsPackage creates and stores a tenant-scoped, safe
// financial preview from a completed private archive. Raw source records and
// attachments never appear in this response.
// @Summary Preview a staged SmartAccounts GL package
// @Description Persist a safe, review-gated GL-authoritative journal plan. This endpoint never posts journals.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param packageID path string true "Staged bridge package ID"
// @Param request body smartaccountsexecutor.PreviewRequest true "Explicit account mappings or imports"
// @Success 200 {object} smartaccountsexecutor.Preview
// @Failure 409 {object} smartaccountsexecutor.Preview
// @Router /tenants/{tenantID}/smartaccounts-sync/packages/{packageID}/preview [post]
func (h *Handlers) PreviewSmartAccountsPackage(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsExecutor == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts executor is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req smartaccountsexecutor.PreviewRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsExecutorRequestBytes)
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	packageID := strings.TrimSpace(chi.URLParam(r, "packageID"))
	if tenantID == "" || packageID == "" {
		respondError(w, http.StatusBadRequest, "tenantID and packageID are required")
		return
	}
	preview, err := h.smartAccountsExecutor.Preview(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, packageID, claims.UserID, req)
	if errors.Is(err, smartaccountsexecutor.ErrPackageNotReady) || errors.Is(err, smartaccountsexecutor.ErrPreviewReviewRequired) {
		respondJSON(w, http.StatusConflict, preview)
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to preview SmartAccounts package")
		return
	}
	respondJSON(w, http.StatusOK, preview)
}

// ApplySmartAccountsPackage executes only an exact, explicitly confirmed
// stored preview. It cannot accept source records, invoices, or payments.
// @Summary Apply a confirmed SmartAccounts GL preview
// @Description Create and post only the balanced GL journals from an exact persisted preview. Non-GL source records remain archive-only.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartAccountsPackageApplyRequest true "Exact preview confirmation and approved policy ID"
// @Success 200 {object} smartaccountsexecutor.Preview
// @Failure 409 {object} smartaccountsexecutor.Preview
// @Router /tenants/{tenantID}/smartaccounts-sync/packages/apply [post]
func (h *Handlers) ApplySmartAccountsPackage(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsExecutor == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts executor is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var request smartAccountsPackageApplyRequest
	if !decodeSmartAccountsReconciliationJSON(w, r, &request) {
		return
	}
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	request.PreviewID, request.PreviewSHA256, request.TolerancePolicyID = strings.TrimSpace(request.PreviewID), strings.TrimSpace(request.PreviewSHA256), strings.TrimSpace(request.TolerancePolicyID)
	req := smartaccountsexecutor.ConfirmRequest{Confirm: request.Confirm, PreviewID: request.PreviewID, PreviewSHA256: request.PreviewSHA256}
	if request.Confirm {
		if h.smartAccountsTolerancePolicyService == nil {
			respondError(w, http.StatusServiceUnavailable, "SmartAccounts reconciliation policy service is not configured")
			return
		}
		stored, storedErr := h.smartAccountsExecutor.GetPreview(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, request.PreviewID)
		if storedErr != nil || stored == nil || stored.PreviewSHA256 != request.PreviewSHA256 || request.TolerancePolicyID == "" {
			respondError(w, http.StatusConflict, "SmartAccounts apply requires review")
			return
		}
		policy, policyErr := h.smartAccountsTolerancePolicyService.Resolve(r.Context(), tenantID, stored.SourceCompanyID, smartaccountsreconciliation.TolerancePolicyCandidateRequest{PackageID: stored.PackageID, PreviewID: stored.ID})
		if h.respondSmartAccountsReconciliationError(w, policyErr) {
			return
		}
		if policy.PolicyID != request.TolerancePolicyID {
			respondError(w, http.StatusConflict, "SmartAccounts apply requires review")
			return
		}
		req.TolerancePolicySHA256 = policy.TolerancePolicySHA256
	}
	preview, err := h.smartAccountsExecutor.Apply(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, claims.UserID, req)
	if errors.Is(err, smartaccountsexecutor.ErrPreviewNotFound) {
		respondError(w, http.StatusNotFound, "SmartAccounts preview not found")
		return
	}
	if errors.Is(err, smartaccountsexecutor.ErrConfirmationRequired) {
		respondJSON(w, http.StatusConflict, preview)
		return
	}
	if err != nil {
		respondError(w, http.StatusConflict, "SmartAccounts apply requires review")
		return
	}
	respondJSON(w, http.StatusOK, preview)
}
