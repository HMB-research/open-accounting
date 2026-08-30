package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/importsession"
)

const maxImportSessionRequestBytes = 2 << 20

// ValidateImportSessionPackage validates a synthetic SmartAccounts canonical
// package without persisting it or creating accounting records.
// @Summary Validate a canonical import package
// @Description Perform read-only validation of a synthetic SmartAccounts canonical package. This endpoint never writes accounting data or package payloads.
// @Tags Import Sessions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body importsession.PackageRequest true "Canonical package"
// @Success 200 {object} importsession.ValidationReport
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Router /tenants/{tenantID}/import-sessions/validate [post]
func (h *Handlers) ValidateImportSessionPackage(w http.ResponseWriter, r *http.Request) {
	if h.importSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Import session service is not configured")
		return
	}
	var req importsession.PackageRequest
	if !decodeImportSessionRequest(w, r, &req) {
		return
	}
	respondJSON(w, http.StatusOK, h.importSessionService.ValidatePackage(req.Package))
}

// CreateImportSession validates and persists a metadata-only receipt for a
// canonical package. It intentionally cannot import financial transactions.
// @Summary Receive a canonical import package
// @Description Validate and persist an auditable receipt for a synthetic SmartAccounts canonical package. No package payloads, credentials, or accounting entries are persisted by this endpoint.
// @Tags Import Sessions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body importsession.PackageRequest true "Canonical package"
// @Success 201 {object} importsession.Receipt
// @Success 200 {object} importsession.Receipt
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 422 {object} importsession.ValidationReport
// @Router /tenants/{tenantID}/import-sessions [post]
func (h *Handlers) CreateImportSession(w http.ResponseWriter, r *http.Request) {
	if h.importSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Import session service is not configured")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	var req importsession.PackageRequest
	if !decodeImportSessionRequest(w, r, &req) {
		return
	}

	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	receipt, report, err := h.importSessionService.Receive(
		r.Context(),
		h.getSchemaName(r.Context(), tenantID),
		tenantID,
		claims.UserID,
		req.Package,
	)
	if errors.Is(err, importsession.ErrPackageValidationFailed) {
		respondJSON(w, http.StatusUnprocessableEntity, report)
		return
	}
	if errors.Is(err, importsession.ErrSourceCompanyBoundToOtherTenant) {
		respondError(w, http.StatusConflict, "Source company is already bound to another tenant")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to save import session receipt")
		return
	}
	if receipt.Created {
		respondJSON(w, http.StatusCreated, receipt)
		return
	}
	respondJSON(w, http.StatusOK, receipt)
}

// GetImportSession returns a tenant-scoped import-session receipt. It never
// exposes source package payloads.
// @Summary Get an import-session receipt
// @Description Return metadata and read-only validation status for one tenant-scoped import session.
// @Tags Import Sessions
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param sessionID path string true "Import session ID"
// @Success 200 {object} importsession.Receipt
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Router /tenants/{tenantID}/import-sessions/{sessionID} [get]
func (h *Handlers) GetImportSession(w http.ResponseWriter, r *http.Request) {
	if h.importSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Import session service is not configured")
		return
	}
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "sessionID is required")
		return
	}
	receipt, err := h.importSessionService.Get(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, sessionID)
	if errors.Is(err, importsession.ErrImportSessionNotFound) {
		respondError(w, http.StatusNotFound, "Import session not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to load import session receipt")
		return
	}
	respondJSON(w, http.StatusOK, receipt)
}

// PlanImportSession produces a deterministic, read-only journal dry run from
// a previously staged receipt. It never creates, posts, or changes journals.
// @Summary Dry-run a staged ledger receipt
// @Description Map a verified SmartAccounts-authoritative staged ledger receipt to existing tenant accounts and return planned journals plus reconciliation expectations. This endpoint never writes financial records.
// @Tags Import Sessions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param sessionID path string true "Import session ID"
// @Param request body importsession.ImportPlanRequest true "Explicit account mappings"
// @Success 200 {object} importsession.ImportPlanResult
// @Failure 400 {object} object{error=string}
// @Failure 401 {object} object{error=string}
// @Failure 403 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} importsession.ImportPlanResult
// @Failure 422 {object} importsession.ImportPlanResult
// @Router /tenants/{tenantID}/import-sessions/{sessionID}/plan [post]
func (h *Handlers) PlanImportSession(w http.ResponseWriter, r *http.Request) {
	if h.importSessionService == nil {
		respondError(w, http.StatusInternalServerError, "Import session service is not configured")
		return
	}
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantID"))
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
	if tenantID == "" {
		respondError(w, http.StatusBadRequest, "tenantID is required")
		return
	}
	if sessionID == "" {
		respondError(w, http.StatusBadRequest, "sessionID is required")
		return
	}
	var req importsession.ImportPlanRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxImportSessionRequestBytes)
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	plan, err := h.importSessionService.Plan(r.Context(), h.getSchemaName(r.Context(), tenantID), tenantID, sessionID, req)
	if errors.Is(err, importsession.ErrImportSessionNotFound) {
		respondError(w, http.StatusNotFound, "Import session not found")
		return
	}
	if errors.Is(err, importsession.ErrImportPlanReviewRequired) || errors.Is(err, importsession.ErrLedgerPlanInputUnavailable) {
		respondJSON(w, http.StatusConflict, plan)
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to plan import session")
		return
	}
	if !plan.Ready {
		respondJSON(w, http.StatusUnprocessableEntity, plan)
		return
	}
	respondJSON(w, http.StatusOK, plan)
}

func decodeImportSessionRequest(w http.ResponseWriter, r *http.Request, target *importsession.PackageRequest) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportSessionRequestBytes)
	if err := decodeJSON(r, target); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return false
	}
	return true
}
