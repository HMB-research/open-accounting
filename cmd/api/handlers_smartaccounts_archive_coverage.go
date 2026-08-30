package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/go-chi/chi/v5"
)

// GetSmartAccountsPackageArchiveCoverage returns count-only target coverage
// for a staged SmartAccounts package. It never returns a canonical record,
// payload field, attachment byte, source name, financial amount, or apply
// result. Financial GL and reference paths remain separately previewed and
// explicitly confirmed.
// @Summary Get staged SmartAccounts package coverage
// @Description Returns count-only apply-gated, archive-only, review-required, and unconsumed canonical-contract coverage. It never applies financial or source records.
// @Tags SmartAccounts Sync
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param packageID path string true "Staged bridge package ID"
// @Success 200 {object} importdelivery.CoverageReport
// @Failure 409 {object} map[string]string
// @Router /tenants/{tenantID}/smartaccounts-sync/packages/{packageID}/archive-coverage [get]
func (h *Handlers) GetSmartAccountsPackageArchiveCoverage(w http.ResponseWriter, r *http.Request) {
	if h.importDeliveryService == nil {
		respondError(w, http.StatusServiceUnavailable, "SmartAccounts package archive is not configured")
		return
	}
	tenantID, packageID := strings.TrimSpace(chi.URLParam(r, "tenantID")), strings.TrimSpace(chi.URLParam(r, "packageID"))
	if tenantID == "" || packageID == "" {
		respondError(w, http.StatusBadRequest, "tenantID and packageID are required")
		return
	}
	report, err := h.importDeliveryService.Coverage(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, packageID)
	if errors.Is(err, importdelivery.ErrCoverageNotReady) {
		respondError(w, http.StatusConflict, "SmartAccounts package coverage requires a staged review package")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Could not inspect SmartAccounts package coverage")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusOK, report)
}
