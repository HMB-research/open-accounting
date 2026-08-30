package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreferences"
	"github.com/go-chi/chi/v5"
)

// PreviewSmartAccountsReferenceMasters creates a stored, non-financial plan
// from an already staged package. Source payloads remain archive-only.
// @Summary Preview staged SmartAccounts reference masters
// @Description Tenant settings permission required. Plans only accounts, customers, vendors, and items selected from reviewed canonical records. It never posts a journal, invoice, or payment.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param packageID path string true "Staged bridge package ID"
// @Param request body smartaccountsreferences.PreviewRequest true "Optional supported reference entity subset"
// @Success 200 {object} smartaccountsreferences.Preview
// @Failure 409 {object} smartaccountsreferences.Preview
// @Router /tenants/{tenantID}/smartaccounts-sync/packages/{packageID}/reference-preview [post]
func (h *Handlers) PreviewSmartAccountsReferenceMasters(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsReferenceService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reference import is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req smartaccountsreferences.PreviewRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsExecutorRequestBytes)
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	tenantID, packageID := strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "packageID"))
	if tenantID == "" || packageID == "" {
		respondError(w, http.StatusBadRequest, "tenantID and packageID are required")
		return
	}
	preview, err := h.smartAccountsReferenceService.Preview(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, packageID, claims.UserID, req)
	if errors.Is(err, smartaccountsreferences.ErrPackageNotReady) || errors.Is(err, smartaccountsreferences.ErrPreviewReviewRequired) {
		respondJSON(w, http.StatusConflict, preview)
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to preview SmartAccounts reference masters")
		return
	}
	respondJSON(w, http.StatusOK, preview)
}

// ApplySmartAccountsReferenceMasters creates only exact confirmed, projected
// reference masters. Changed revisions and tombstones are review-only.
// @Summary Apply confirmed SmartAccounts reference-master preview
// @Description Creates no financial posting. Requires an exact stored preview digest and explicit confirmation.
// @Tags SmartAccounts Sync
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body smartaccountsreferences.ConfirmRequest true "Exact reference preview confirmation"
// @Success 200 {object} smartaccountsreferences.Preview
// @Failure 409 {object} smartaccountsreferences.Preview
// @Router /tenants/{tenantID}/smartaccounts-sync/reference-masters/apply [post]
func (h *Handlers) ApplySmartAccountsReferenceMasters(w http.ResponseWriter, r *http.Request) {
	if h.smartAccountsReferenceService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts reference import is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req smartaccountsreferences.ConfirmRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxSmartAccountsExecutorRequestBytes)
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	preview, err := h.smartAccountsReferenceService.Apply(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, claims.UserID, req)
	if errors.Is(err, smartaccountsreferences.ErrPreviewNotFound) {
		respondError(w, http.StatusNotFound, "SmartAccounts reference preview not found")
		return
	}
	if errors.Is(err, smartaccountsreferences.ErrConfirmationRequired) {
		respondJSON(w, http.StatusConflict, preview)
		return
	}
	if err != nil {
		respondError(w, http.StatusConflict, "SmartAccounts reference apply requires review")
		return
	}
	respondJSON(w, http.StatusOK, preview)
}
