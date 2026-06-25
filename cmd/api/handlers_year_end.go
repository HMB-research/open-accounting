package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/shopspring/decimal"
)

type yearEndCloseAuditArchiveWriter interface {
	Create(string) (io.Writer, error)
	Close() error
}

var (
	evaluateDocumentsEvidencePolicy = func(service *documents.Service, ctx context.Context, schemaName, tenantID string, req *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error) {
		return service.EvaluateEvidencePolicy(ctx, schemaName, tenantID, req)
	}
	marshalYearEndCloseAuditManifest  = json.MarshalIndent
	newYearEndCloseAuditArchiveWriter = func(writer io.Writer) yearEndCloseAuditArchiveWriter {
		return zip.NewWriter(writer)
	}
)

// GetYearEndCloseStatus returns fiscal year-end close readiness.
// @Summary Get year-end close status
// @Description Get fiscal year close readiness, retained-earnings mapping, net income, period-lock status, inventory costing review using the explicit method or tenant valuation policy, and existing carry-forward state
// @Tags Period Close
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param period_end_date query string true "Fiscal year-end date (YYYY-MM-DD)"
// @Param inventory_valuation_method query string false "Inventory valuation method override for close review: standard-cost, weighted-average, or fifo"
// @Success 200 {object} accounting.YearEndCloseStatus
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-close-status [get]
func (h *Handlers) GetYearEndCloseStatus(w http.ResponseWriter, r *http.Request) {
	routeCtx := h.tenantContextFromRequest(r)
	periodEndDate := strings.TrimSpace(r.URL.Query().Get("period_end_date"))
	if periodEndDate == "" {
		respondError(w, http.StatusBadRequest, "period end date is required")
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), routeCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	inventoryValuationMethod := tenantInventoryValuationMethod(tenantRecord, yearEndInventoryValuationMethod(r))

	status, err := h.accountingService.GetYearEndCloseStatus(
		r.Context(),
		routeCtx.schemaName,
		routeCtx.tenantID,
		tenantRecord.Settings.FiscalYearStart,
		periodEndDate,
		tenantRecord.Settings.PeriodLockDate,
	)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	if err := h.attachYearEndCloseEvidenceStatus(r.Context(), routeCtx.schemaName, routeCtx.tenantID, status); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to evaluate close-pack evidence")
		return
	}
	if err := h.attachYearEndInventoryCostingReview(r.Context(), routeCtx.schemaName, routeCtx.tenantID, inventoryValuationMethod, status); err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	attachYearEndCloseRemediationActions(status)

	respondJSON(w, http.StatusOK, status)
}

// GetYearEndClosePack returns close readiness with year-end financial reports.
// @Summary Get year-end close pack
// @Description Get year-end close readiness plus inventory costing review using the explicit method or tenant valuation policy, trial balance, balance sheet, and income statement for the fiscal year
// @Tags Period Close
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param period_end_date query string true "Fiscal year-end date (YYYY-MM-DD)"
// @Param inventory_valuation_method query string false "Inventory valuation method override for close review: standard-cost, weighted-average, or fifo"
// @Success 200 {object} accounting.YearEndClosePack
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-close-pack [get]
func (h *Handlers) GetYearEndClosePack(w http.ResponseWriter, r *http.Request) {
	routeCtx := h.tenantContextFromRequest(r)
	periodEndDate := strings.TrimSpace(r.URL.Query().Get("period_end_date"))
	if periodEndDate == "" {
		respondError(w, http.StatusBadRequest, "period end date is required")
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), routeCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	inventoryValuationMethod := tenantInventoryValuationMethod(tenantRecord, yearEndInventoryValuationMethod(r))

	pack, err := h.accountingService.GetYearEndClosePack(
		r.Context(),
		routeCtx.schemaName,
		routeCtx.tenantID,
		tenantRecord.Settings.FiscalYearStart,
		periodEndDate,
		tenantRecord.Settings.PeriodLockDate,
	)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	if pack.Status != nil {
		if err := h.attachYearEndCloseEvidenceStatus(r.Context(), routeCtx.schemaName, routeCtx.tenantID, pack.Status); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to evaluate close-pack evidence")
			return
		}
		if err := h.attachYearEndInventoryCostingReview(r.Context(), routeCtx.schemaName, routeCtx.tenantID, inventoryValuationMethod, pack.Status); err != nil {
			respondYearEndCloseError(w, err)
			return
		}
		attachYearEndCloseRemediationActions(pack.Status)
	}

	respondJSON(w, http.StatusOK, pack)
}

// GetYearEndCloseAuditEvidence returns the year-end close pack plus close-pack reviewer evidence metadata.
// @Summary Get year-end close audit evidence
// @Description Get year-end close readiness, inventory costing review using the explicit method or tenant valuation policy, core reports, close-pack evidence policy, and attached close-pack document metadata
// @Tags Period Close
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param period_end_date query string true "Fiscal year-end date (YYYY-MM-DD)"
// @Param inventory_valuation_method query string false "Inventory valuation method override for close review: standard-cost, weighted-average, or fifo"
// @Success 200 {object} accounting.YearEndCloseAuditEvidence
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-close-audit-evidence [get]
func (h *Handlers) GetYearEndCloseAuditEvidence(w http.ResponseWriter, r *http.Request) {
	routeCtx := h.tenantContextFromRequest(r)
	periodEndDate := strings.TrimSpace(r.URL.Query().Get("period_end_date"))
	if periodEndDate == "" {
		respondError(w, http.StatusBadRequest, "period end date is required")
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), routeCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	inventoryValuationMethod := tenantInventoryValuationMethod(tenantRecord, yearEndInventoryValuationMethod(r))

	audit, err := h.buildYearEndCloseAuditEvidence(r.Context(), tenantRecord, periodEndDate, inventoryValuationMethod)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, audit)
}

// DownloadYearEndCloseAuditArchive returns a ZIP archive with close-pack audit manifest and attached evidence files.
// @Summary Download year-end close audit archive
// @Description Download a ZIP archive containing year-end close pack metadata, inventory costing review using the explicit method or tenant valuation policy, evidence-policy results, and close-pack documents
// @Tags Period Close
// @Produce application/zip
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param period_end_date query string true "Fiscal year-end date (YYYY-MM-DD)"
// @Param inventory_valuation_method query string false "Inventory valuation method override for close review: standard-cost, weighted-average, or fifo"
// @Success 200 {file} binary
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-close-audit-archive [get]
func (h *Handlers) DownloadYearEndCloseAuditArchive(w http.ResponseWriter, r *http.Request) {
	routeCtx := h.tenantContextFromRequest(r)
	periodEndDate := strings.TrimSpace(r.URL.Query().Get("period_end_date"))
	if periodEndDate == "" {
		respondError(w, http.StatusBadRequest, "period end date is required")
		return
	}
	if h.documentsService == nil {
		respondError(w, http.StatusInternalServerError, "Document storage is not configured")
		return
	}

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), routeCtx.tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	inventoryValuationMethod := tenantInventoryValuationMethod(tenantRecord, yearEndInventoryValuationMethod(r))

	audit, err := h.buildYearEndCloseAuditEvidence(r.Context(), tenantRecord, periodEndDate, inventoryValuationMethod)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	archive, err := h.buildYearEndCloseAuditArchive(r.Context(), tenantRecord, audit)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	fileName := fmt.Sprintf("year-end-close-audit-%s.zip", safeArchiveFileName(periodEndDate))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archive)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(archive)
}

// CreateYearEndCarryForward creates and posts a fiscal year-end carry-forward journal.
// @Summary Create year-end carry-forward
// @Description Create and post retained-earnings carry-forward journal entries after the fiscal year has been closed and inventory costing review has no blocking exceptions
// @Tags Period Close
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.CreateYearEndCarryForwardRequest true "Carry-forward request"
// @Success 200 {object} accounting.YearEndCarryForwardResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-carry-forward [post]
func (h *Handlers) CreateYearEndCarryForward(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req accounting.CreateYearEndCarryForwardRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	req.UserID = userID

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}
	req.InventoryValuationMethod = tenantInventoryValuationMethod(tenantRecord, req.InventoryValuationMethod)
	if err := h.requireApprovedYearEndClosePackEvidence(r.Context(), tenantRecord, req.PeriodEndDate); err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	if err := h.requireYearEndInventoryCostingReady(r.Context(), tenantRecord.SchemaName, tenantID, tenantRecord.Settings.FiscalYearStart, req.PeriodEndDate, req.InventoryValuationMethod); err != nil {
		respondYearEndCloseError(w, err)
		return
	}

	result, err := h.accountingService.CreateYearEndCarryForward(
		r.Context(),
		tenantRecord.SchemaName,
		tenantID,
		tenantRecord.Settings.FiscalYearStart,
		tenantRecord.Settings.PeriodLockDate,
		&req,
	)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	if result.Status != nil {
		if err := h.attachYearEndCloseEvidenceStatus(r.Context(), tenantRecord.SchemaName, tenantID, result.Status); err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to evaluate close-pack evidence")
			return
		}
		if err := h.attachYearEndInventoryCostingReview(r.Context(), tenantRecord.SchemaName, tenantID, req.InventoryValuationMethod, result.Status); err != nil {
			respondYearEndCloseError(w, err)
			return
		}
		attachYearEndCloseRemediationActions(result.Status)
	}

	respondJSON(w, http.StatusOK, result)
}

// ReverseYearEndCarryForward voids a posted carry-forward and creates a reversal journal.
// @Summary Reverse year-end carry-forward
// @Description Void an existing posted fiscal year-end carry-forward and create a posted reversal journal for controlled corrections
// @Tags Period Close
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body accounting.ReverseYearEndCarryForwardRequest true "Carry-forward reversal request"
// @Success 200 {object} accounting.YearEndCarryForwardReversalResult
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 409 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/year-end-carry-forward/reverse [post]
func (h *Handlers) ReverseYearEndCarryForward(w http.ResponseWriter, r *http.Request) {
	tenantID, userID, ok := h.authorizePeriodCloseMutation(w, r)
	if !ok {
		return
	}

	var req accounting.ReverseYearEndCarryForwardRequest
	if !decodeJSONRequest(w, r, &req) {
		return
	}
	req.UserID = userID

	tenantRecord, err := h.tenantService.GetTenant(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	result, err := h.accountingService.ReverseYearEndCarryForward(
		r.Context(),
		tenantRecord.SchemaName,
		tenantID,
		tenantRecord.Settings.FiscalYearStart,
		tenantRecord.Settings.PeriodLockDate,
		&req,
	)
	if err != nil {
		respondYearEndCloseError(w, err)
		return
	}
	attachYearEndCloseRemediationActions(result.Status)

	respondJSON(w, http.StatusOK, result)
}

func (h *Handlers) yearEndCarryForwardExists(r *http.Request, tenantRecord *tenant.Tenant, rawPeriodEndDate string) (bool, error) {
	if h.accountingService == nil {
		return false, nil
	}

	status, err := h.accountingService.GetYearEndCloseStatus(
		r.Context(),
		tenantRecord.SchemaName,
		tenantRecord.ID,
		tenantRecord.Settings.FiscalYearStart,
		rawPeriodEndDate,
		tenantRecord.Settings.PeriodLockDate,
	)
	if err != nil {
		return false, err
	}

	return status.IsFiscalYearEnd && status.ExistingCarryForward != nil, nil
}

func (h *Handlers) buildYearEndCloseAuditEvidence(ctx context.Context, tenantRecord *tenant.Tenant, periodEndDate, inventoryValuationMethod string) (*accounting.YearEndCloseAuditEvidence, error) {
	pack, err := h.accountingService.GetYearEndClosePack(
		ctx,
		tenantRecord.SchemaName,
		tenantRecord.ID,
		tenantRecord.Settings.FiscalYearStart,
		periodEndDate,
		tenantRecord.Settings.PeriodLockDate,
	)
	if err != nil {
		return nil, err
	}

	var attachedDocuments []documents.Document
	var evidencePolicy *documents.EvidencePolicyResult
	if pack.Status != nil {
		if err := h.attachYearEndCloseEvidenceStatus(ctx, tenantRecord.SchemaName, tenantRecord.ID, pack.Status); err != nil {
			return nil, fmt.Errorf("evaluate close-pack evidence: %w", err)
		}
		if err := h.attachYearEndInventoryCostingReview(ctx, tenantRecord.SchemaName, tenantRecord.ID, inventoryValuationMethod, pack.Status); err != nil {
			return nil, err
		}
		attachYearEndCloseRemediationActions(pack.Status)
		evidencePolicy = pack.Status.ClosePackEvidence
		if h.documentsService != nil && strings.TrimSpace(pack.Status.ClosePackEvidenceEntityID) != "" {
			attachedDocuments, err = h.documentsService.ListDocuments(
				ctx,
				tenantRecord.SchemaName,
				tenantRecord.ID,
				documents.EntityTypeYearEndClose,
				pack.Status.ClosePackEvidenceEntityID,
			)
			if err != nil {
				return nil, err
			}
		}
	}

	return &accounting.YearEndCloseAuditEvidence{
		Pack:           pack,
		EvidencePolicy: evidencePolicy,
		Documents:      attachedDocuments,
		GeneratedAt:    time.Now().UTC(),
	}, nil
}

func (h *Handlers) buildYearEndCloseAuditArchive(ctx context.Context, tenantRecord *tenant.Tenant, audit *accounting.YearEndCloseAuditEvidence) ([]byte, error) {
	var buffer bytes.Buffer
	writer := newYearEndCloseAuditArchiveWriter(&buffer)

	manifest, err := marshalYearEndCloseAuditManifest(audit, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode audit manifest: %w", err)
	}
	manifestFile, err := writer.Create("manifest.json")
	if err != nil {
		return nil, fmt.Errorf("create audit manifest: %w", err)
	}
	if _, err := manifestFile.Write(manifest); err != nil {
		return nil, fmt.Errorf("write audit manifest: %w", err)
	}

	for _, doc := range audit.Documents {
		docInfo, reader, err := h.documentsService.OpenDocument(ctx, tenantRecord.SchemaName, tenantRecord.ID, doc.ID)
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		fileName := fmt.Sprintf("documents/%s-%s", safeArchiveFileName(docInfo.ID), safeArchiveFileName(docInfo.FileName))
		docFile, err := writer.Create(fileName)
		if err != nil {
			_ = reader.Close()
			_ = writer.Close()
			return nil, fmt.Errorf("create archive document entry: %w", err)
		}
		if _, err := io.Copy(docFile, reader); err != nil {
			_ = reader.Close()
			_ = writer.Close()
			return nil, fmt.Errorf("write archive document entry: %w", err)
		}
		if err := reader.Close(); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("close document reader: %w", err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close audit archive: %w", err)
	}
	return buffer.Bytes(), nil
}

func safeArchiveFileName(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	result := strings.Trim(b.String(), "._-")
	if result == "" {
		return "file"
	}
	return result
}

func (h *Handlers) requireApprovedYearEndClosePackEvidence(ctx context.Context, tenantRecord *tenant.Tenant, rawPeriodEndDate string) error {
	isYearEnd, err := accounting.IsFiscalYearEndPeriod(rawPeriodEndDate, tenantRecord.Settings.FiscalYearStart)
	if err != nil {
		return err
	}
	if !isYearEnd {
		return nil
	}

	entityID, _ := accounting.YearEndCloseEvidenceEntityID(tenantRecord.ID, rawPeriodEndDate)
	if h.documentsService == nil {
		return fmt.Errorf("%w before completing fiscal-year close workflow for %s (entity_id: %s)", errApprovedClosePackEvidenceRequired, rawPeriodEndDate, entityID)
	}
	results, err := h.yearEndClosePackEvidence(ctx, tenantRecord.SchemaName, tenantRecord.ID, entityID)
	if err != nil {
		return err
	}
	if len(results) == 0 || !results[0].Compliant {
		return fmt.Errorf("%w before completing fiscal-year close workflow for %s (entity_id: %s)", errApprovedClosePackEvidenceRequired, rawPeriodEndDate, entityID)
	}

	return nil
}

func (h *Handlers) attachYearEndCloseEvidenceStatus(ctx context.Context, schemaName, tenantID string, status *accounting.YearEndCloseStatus) error {
	if h.documentsService == nil || status == nil || strings.TrimSpace(status.ClosePackEvidenceEntityID) == "" {
		return nil
	}

	results, err := h.yearEndClosePackEvidence(ctx, schemaName, tenantID, status.ClosePackEvidenceEntityID)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return nil
	}

	status.ClosePackEvidence = &results[0]
	status.CarryForwardReady = status.CarryForwardReady && results[0].Compliant
	return nil
}

func yearEndInventoryValuationMethod(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("inventory_valuation_method"))
}

func (h *Handlers) requireYearEndInventoryCostingReady(ctx context.Context, schemaName, tenantID string, fiscalYearStartMonth int, rawPeriodEndDate, method string) error {
	if h.inventoryService == nil {
		return nil
	}
	isYearEnd, err := accounting.IsFiscalYearEndPeriod(rawPeriodEndDate, fiscalYearStartMonth)
	if err != nil {
		return err
	}
	if !isYearEnd {
		return nil
	}
	review, err := h.yearEndInventoryCostingReview(ctx, schemaName, tenantID, method)
	if err != nil {
		return err
	}
	if review != nil && !review.Ready {
		return fmt.Errorf("inventory costing review has %d blocking exception lines", review.BlockingExceptionLineCount)
	}
	return nil
}

func (h *Handlers) attachYearEndInventoryCostingReview(ctx context.Context, schemaName, tenantID, method string, status *accounting.YearEndCloseStatus) error {
	if h.inventoryService == nil || status == nil || !status.IsFiscalYearEnd {
		return nil
	}
	review, err := h.yearEndInventoryCostingReview(ctx, schemaName, tenantID, method)
	if err != nil {
		return err
	}
	status.InventoryCostingReview = review
	if review != nil {
		status.CarryForwardReady = status.CarryForwardReady && review.Ready
	}
	return nil
}

func attachYearEndCloseRemediationActions(status *accounting.YearEndCloseStatus) {
	if status != nil {
		status.RemediationActions = accounting.BuildYearEndCloseRemediationActions(status)
	}
}

func (h *Handlers) yearEndInventoryCostingReview(ctx context.Context, schemaName, tenantID, method string) (*accounting.YearEndInventoryCostingReview, error) {
	if h.inventoryService == nil {
		return nil, nil
	}
	report, err := h.inventoryService.GetInventoryValuation(ctx, tenantID, schemaName, "", method)
	if err != nil {
		return nil, err
	}
	review := &accounting.YearEndInventoryCostingReview{
		ValuationMethod: report.ValuationMethod,
		LineCount:       len(report.Lines),
		TotalQuantity:   report.TotalQuantity,
		TotalReserved:   report.TotalReserved,
		TotalAvailable:  report.TotalAvailable,
		TotalValue:      report.TotalValue,
		GeneratedAt:     report.GeneratedAt,
	}
	for _, line := range report.Lines {
		blocking := false
		if line.Quantity.LessThan(decimal.Zero) {
			review.NegativeQuantityLineCount++
			blocking = true
		}
		if line.AvailableQty.LessThan(decimal.Zero) {
			review.NegativeAvailableLineCount++
			blocking = true
		}
		if line.InventoryValue.LessThan(decimal.Zero) {
			review.NegativeValueLineCount++
			blocking = true
		}
		if line.Quantity.GreaterThan(decimal.Zero) && !line.UnitCost.GreaterThan(decimal.Zero) {
			review.MissingCostLineCount++
			blocking = true
		}
		if blocking {
			review.BlockingExceptionLineCount++
		}
	}
	review.Ready = review.BlockingExceptionLineCount == 0
	return review, nil
}

func (h *Handlers) yearEndClosePackEvidence(ctx context.Context, schemaName, tenantID, entityID string) ([]documents.EvidencePolicyResult, error) {
	return evaluateDocumentsEvidencePolicy(h.documentsService, ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeYearEndClose,
		EntityIDs:  []string{entityID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes:   []string{documents.DocumentTypeClosePack},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
}

func respondYearEndCloseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errApprovedClosePackEvidenceRequired):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "period end date"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "invalid valuation method"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "must match the fiscal year end"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "user_id is required"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "reason is required"):
		respondError(w, http.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), "fiscal year must be closed"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "carry-forward already exists"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "carry-forward does not exist"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "current status"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "not in posted status"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "retained earnings account is required"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "no revenue or expense activity found"):
		respondError(w, http.StatusConflict, err.Error())
	case strings.Contains(err.Error(), "inventory costing review"):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "Failed to process year-end close workflow")
	}
}
