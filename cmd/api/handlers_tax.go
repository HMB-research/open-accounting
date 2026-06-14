package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/tax"
)

var (
	errApprovedKMDSubmissionEvidenceRequired = errors.New("approved KMD submission evidence is required")
	errApprovedKMDAcceptanceEvidenceRequired = errors.New("approved KMD acceptance evidence is required")
)

// HandleGenerateKMD generates a KMD declaration for a period
// @Summary Generate KMD declaration
// @Description Generate an Estonian VAT declaration (KMD) for a specific period
// @Tags Tax
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tax.CreateKMDRequest true "Period to generate"
// @Success 200 {object} tax.KMDDeclaration
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd [post]
func (h *Handlers) HandleGenerateKMD(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	var req tax.CreateKMDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	decl, err := h.taxService.GenerateKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, decl)
}

// HandleImportKMDHistory imports historical KMD declarations from CSV data.
// @Summary Import historical KMD declarations
// @Description Import historical Estonian VAT declarations (KMD) from CSV data
// @Tags Tax
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body tax.ImportKMDHistoryRequest true "CSV import payload"
// @Success 200 {object} tax.ImportKMDHistoryResult
// @Failure 400 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd/import-history [post]
func (h *Handlers) HandleImportKMDHistory(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	var req tax.ImportKMDHistoryRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.CSVContent) == "" {
		respondError(w, http.StatusBadRequest, "csv_content is required")
		return
	}

	if req.FileName == "" {
		req.FileName = "kmd-history.csv"
	}

	result, err := h.taxService.ImportKMDHistoryCSV(r.Context(), tenantCtx.schemaName, tenantCtx.tenantID, &req)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// HandleListKMD lists all KMD declarations for a tenant
// @Summary List KMD declarations
// @Description Get all KMD declarations for a tenant
// @Tags Tax
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Success 200 {array} tax.KMDDeclaration
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd [get]
func (h *Handlers) HandleListKMD(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	declarations, err := h.taxService.ListKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, declarations)
}

// HandleGenerateKMDINF generates a KMD INF appendix report for a period.
// @Summary Generate KMD INF appendix report
// @Description Generate KMD INF A/B invoice appendix rows for an Estonian VAT period
// @Tags Tax
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path string true "Year"
// @Param month path string true "Month"
// @Param threshold query string false "Partner-period threshold excluding VAT"
// @Success 200 {object} tax.KMDINFReport
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd/{year}/{month}/inf [get]
func (h *Handlers) HandleGenerateKMDINF(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}
	month, err := strconv.Atoi(chi.URLParam(r, "month"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid month")
		return
	}

	threshold := tax.KMDINFDefaultThreshold
	if raw := strings.TrimSpace(r.URL.Query().Get("threshold")); raw != "" {
		threshold, err = decimal.NewFromString(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid threshold")
			return
		}
	}
	if threshold.LessThanOrEqual(decimal.Zero) {
		respondError(w, http.StatusBadRequest, "threshold must be positive")
		return
	}

	report, err := h.taxService.GenerateKMDINF(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, &tax.KMDINFReportRequest{
		Year:      year,
		Month:     month,
		Threshold: threshold,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// HandleGenerateEUVATOSS generates a quarterly EU VAT OSS report.
// @Summary Generate EU VAT OSS report
// @Description Generate quarterly EU VAT OSS destination-country totals from non-Estonian EU sales invoices
// @Tags Tax
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year query int true "Year"
// @Param quarter query int true "Quarter"
// @Param include_b2b query bool false "Include contacts with VAT numbers"
// @Success 200 {object} tax.EUVATOSSReport
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/eu-vat/oss [get]
func (h *Handlers) HandleGenerateEUVATOSS(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)

	year, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("year")))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}
	if year < 2020 || year > 2100 {
		respondError(w, http.StatusBadRequest, "Invalid year")
		return
	}
	quarter, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("quarter")))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid quarter")
		return
	}
	if quarter < 1 || quarter > 4 {
		respondError(w, http.StatusBadRequest, "Invalid quarter")
		return
	}
	includeB2B := false
	if raw := strings.TrimSpace(r.URL.Query().Get("include_b2b")); raw != "" {
		includeB2B, err = strconv.ParseBool(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid include_b2b")
			return
		}
	}

	report, err := h.taxService.GenerateEUVATOSS(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, &tax.EUVATOSSReportRequest{
		Year:       year,
		Quarter:    quarter,
		IncludeB2B: includeB2B,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// HandleMarkKMDSubmitted marks a KMD declaration as submitted.
// @Summary Mark KMD as submitted
// @Description Mark a KMD declaration as submitted to e-MTA after approved tax/support evidence is attached, and record the submission timestamp.
// @Tags Tax
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path string true "Year"
// @Param month path string true "Month"
// @Success 200 {object} object{status=string}
// @Failure 409 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd/{year}/{month}/submit [post]
func (h *Handlers) HandleMarkKMDSubmitted(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	year := chi.URLParam(r, "year")
	month := chi.URLParam(r, "month")

	declaration, err := h.taxService.GetKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, year, month)
	if err != nil || declaration == nil {
		respondError(w, http.StatusNotFound, "Declaration not found")
		return
	}

	if err := h.requireApprovedKMDSubmissionEvidence(r.Context(), tenantCtx.schemaName, tenantCtx.tenantID, declaration.ID); err != nil {
		if errors.Is(err, errApprovedKMDSubmissionEvidenceRequired) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to verify KMD submission evidence")
		return
	}

	if err := h.taxService.MarkKMDSubmitted(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, year, month); err != nil {
		if errors.Is(err, tax.ErrKMDDeclarationNotFound) {
			respondError(w, http.StatusNotFound, "Declaration not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "submitted"})
}

// HandleMarkKMDAccepted marks a KMD declaration as accepted.
// @Summary Mark KMD as accepted
// @Description Mark a KMD declaration as accepted by e-MTA after approved tax/support evidence is attached.
// @Tags Tax
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path string true "Year"
// @Param month path string true "Month"
// @Success 200 {object} object{status=string}
// @Failure 409 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd/{year}/{month}/accept [post]
func (h *Handlers) HandleMarkKMDAccepted(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	year := chi.URLParam(r, "year")
	month := chi.URLParam(r, "month")

	declaration, err := h.taxService.GetKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, year, month)
	if err != nil || declaration == nil {
		respondError(w, http.StatusNotFound, "Declaration not found")
		return
	}

	if err := h.requireApprovedKMDAcceptanceEvidence(r.Context(), tenantCtx.schemaName, tenantCtx.tenantID, declaration.ID); err != nil {
		if errors.Is(err, errApprovedKMDAcceptanceEvidenceRequired) {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to verify KMD acceptance evidence")
		return
	}

	if err := h.taxService.MarkKMDAccepted(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, year, month); err != nil {
		if errors.Is(err, tax.ErrKMDDeclarationNotFound) {
			respondError(w, http.StatusNotFound, "Declaration not found")
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

func (h *Handlers) requireApprovedKMDSubmissionEvidence(ctx context.Context, schemaName, tenantID, declarationID string) error {
	return h.requireApprovedKMDEvidence(ctx, schemaName, tenantID, declarationID, "submission", "submitted", errApprovedKMDSubmissionEvidenceRequired)
}

func (h *Handlers) requireApprovedKMDAcceptanceEvidence(ctx context.Context, schemaName, tenantID, declarationID string) error {
	return h.requireApprovedKMDEvidence(ctx, schemaName, tenantID, declarationID, "acceptance", "accepted", errApprovedKMDAcceptanceEvidenceRequired)
}

func (h *Handlers) requireApprovedKMDEvidence(ctx context.Context, schemaName, tenantID, declarationID, evidenceStage, status string, requiredErr error) error {
	if h.documentsService == nil {
		return nil
	}

	results, err := h.documentsService.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeKMD,
		EntityIDs:  []string{declarationID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeTaxSupport,
				documents.DocumentTypeSupportingDocument,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate KMD %s evidence: %w", evidenceStage, err)
	}
	if len(results) == 0 || !results[0].Compliant {
		return fmt.Errorf("%w before marking KMD declaration %s %s", requiredErr, declarationID, status)
	}
	return nil
}

// HandleExportKMD exports a KMD declaration to XML
// @Summary Export KMD to XML
// @Description Export a KMD declaration to Estonian e-MTA XML format
// @Tags Tax
// @Produce application/xml
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param year path string true "Year"
// @Param month path string true "Month"
// @Success 200 {file} file "XML file"
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/tax/kmd/{year}/{month}/xml [get]
func (h *Handlers) HandleExportKMD(w http.ResponseWriter, r *http.Request) {
	tenantCtx := h.tenantContextFromRequest(r)
	year := chi.URLParam(r, "year")
	month := chi.URLParam(r, "month")

	currentTenant, err := h.tenantService.GetTenant(r.Context(), tenantCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	decl, err := h.taxService.GetKMD(r.Context(), tenantCtx.tenantID, tenantCtx.schemaName, year, month)
	if err != nil {
		respondError(w, http.StatusNotFound, "Declaration not found")
		return
	}

	xmlBytes, err := tax.ExportKMDToXML(decl, currentTenant.Settings.RegCode)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=KMD_%s_%s.xml", year, month))
	_, _ = w.Write(xmlBytes)
}
