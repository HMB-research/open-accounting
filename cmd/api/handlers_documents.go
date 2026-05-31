package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/documents"
)

// ListDocuments lists documents attached to one entity.
// @Summary List documents
// @Description List documents attached to an entity by entity type and entity ID
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param entity_type query string true "Entity type"
// @Param entity_id query string true "Entity ID"
// @Success 200 {array} documents.Document
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents [get]
func (h *Handlers) ListDocuments(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	entityType := strings.TrimSpace(r.URL.Query().Get("entity_type"))
	entityID := strings.TrimSpace(r.URL.Query().Get("entity_id"))
	if entityType == "" || entityID == "" {
		respondError(w, http.StatusBadRequest, "entity_type and entity_id are required")
		return
	}

	result, err := h.documentsService.ListDocuments(r.Context(), schemaName, tenantID, entityType, entityID)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// ListDocumentReviewSummaries returns review summary rows for multiple entities.
// @Summary List document review summaries
// @Description Return document review counts and missing evidence flags for multiple entity IDs
// @Tags Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body object true "Review summary request"
// @Success 200 {array} documents.ReviewSummary
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/review-summary [post]
func (h *Handlers) ListDocumentReviewSummaries(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req struct {
		EntityType string   `json:"entity_type"`
		EntityIDs  []string `json:"entity_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	if strings.TrimSpace(req.EntityType) == "" || len(req.EntityIDs) == 0 {
		respondError(w, http.StatusBadRequest, "entity_type and entity_ids are required")
		return
	}

	result, err := h.documentsService.ListReviewSummaries(r.Context(), schemaName, tenantID, req.EntityType, req.EntityIDs)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetDocumentReviewQueue returns tenant-wide documents waiting for review action.
// @Summary Get document review queue
// @Description List documents by review status with optional entity and document type filters
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param entity_type query string false "Entity type filter"
// @Param document_type query string false "Document type filter"
// @Param review_status query string false "Review status: PENDING, REVIEWED, APPROVED, REJECTED, or ALL"
// @Param limit query int false "Maximum documents to return"
// @Success 200 {object} documents.ReviewQueue
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/review-queue [get]
func (h *Handlers) GetDocumentReviewQueue(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	limit := 0
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			respondError(w, http.StatusBadRequest, "limit must be zero or greater")
			return
		}
		limit = parsed
	}

	reviewStatus := strings.TrimSpace(r.URL.Query().Get("review_status"))
	if reviewStatus == "" {
		reviewStatus = strings.TrimSpace(r.URL.Query().Get("status"))
	}
	result, err := h.documentsService.GetReviewQueue(r.Context(), schemaName, tenantID, documents.ReviewQueueFilter{
		EntityType:   strings.TrimSpace(r.URL.Query().Get("entity_type")),
		DocumentType: strings.TrimSpace(r.URL.Query().Get("document_type")),
		ReviewStatus: reviewStatus,
		Limit:        limit,
	})
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// EvaluateDocumentEvidencePolicy evaluates evidence requirements for entities.
// @Summary Evaluate document evidence policy
// @Description Evaluate configured document evidence rules for multiple entity IDs
// @Tags Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param request body documents.EvidencePolicyRequest true "Evidence policy request"
// @Success 200 {array} documents.EvidencePolicyResult
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/evidence-policy [post]
func (h *Handlers) EvaluateDocumentEvidencePolicy(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req documents.EvidencePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	result, err := h.documentsService.EvaluateEvidencePolicy(r.Context(), schemaName, tenantID, &req)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// GetDocumentRetentionReview returns documents due for retention review.
// @Summary Get document retention review
// @Description List documents with expired, due soon, or missing retention metadata
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param as_of query string false "Review date (YYYY-MM-DD)"
// @Param horizon_days query int false "Days ahead for due soon documents"
// @Param include_missing query bool false "Include documents missing retention metadata"
// @Success 200 {object} documents.RetentionReview
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/retention [get]
func (h *Handlers) GetDocumentRetentionReview(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	asOfDate := time.Now().UTC()
	if rawAsOf := strings.TrimSpace(r.URL.Query().Get("as_of")); rawAsOf != "" {
		parsed, err := time.Parse("2006-01-02", rawAsOf)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid as_of date, expected YYYY-MM-DD")
			return
		}
		asOfDate = parsed
	}
	horizonDays := 30
	if rawHorizon := strings.TrimSpace(r.URL.Query().Get("horizon_days")); rawHorizon != "" {
		parsed, err := strconv.Atoi(rawHorizon)
		if err != nil || parsed < 0 {
			respondError(w, http.StatusBadRequest, "horizon_days must be zero or greater")
			return
		}
		horizonDays = parsed
	}
	includeMissing := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_missing")), "true")

	result, err := h.documentsService.GetRetentionReview(r.Context(), schemaName, tenantID, asOfDate, horizonDays, includeMissing)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, result)
}

// UpdateDocumentRetention updates document retention metadata.
// @Summary Update document retention
// @Description Set or clear the retention date for a document
// @Tags Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param documentID path string true "Document ID"
// @Param request body object true "Retention update"
// @Success 200 {object} documents.Document
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/{documentID}/retention [patch]
func (h *Handlers) UpdateDocumentRetention(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	documentID := chi.URLParam(r, "documentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req struct {
		RetentionUntil *string `json:"retention_until"`
		ClearRetention bool    `json:"clear_retention"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	rawRetention := ""
	if req.RetentionUntil != nil {
		rawRetention = strings.TrimSpace(*req.RetentionUntil)
	}
	if req.ClearRetention && rawRetention != "" {
		respondError(w, http.StatusBadRequest, "retention_until cannot be set when clear_retention is true")
		return
	}
	if !req.ClearRetention && rawRetention == "" {
		respondError(w, http.StatusBadRequest, "retention_until is required unless clear_retention is true")
		return
	}

	var retentionUntil *time.Time
	if !req.ClearRetention {
		parsed, err := time.Parse("2006-01-02", rawRetention)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid retention_until date, expected YYYY-MM-DD")
			return
		}
		normalized := parsed.UTC()
		retentionUntil = &normalized
	}

	doc, err := h.documentsService.UpdateDocumentRetention(r.Context(), schemaName, tenantID, documentID, retentionUntil)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, doc)
}

// UploadDocument uploads a document attachment.
// @Summary Upload document
// @Description Upload a multipart document and attach it to an entity
// @Tags Documents
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param entity_type formData string true "Entity type"
// @Param entity_id formData string true "Entity ID"
// @Param document_type formData string true "Document type"
// @Param notes formData string false "Notes"
// @Param retention_until formData string false "Retention date (YYYY-MM-DD)"
// @Param retention_years formData int false "Retention years"
// @Param file formData file true "Document file"
// @Success 201 {object} documents.Document
// @Failure 400 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents [post]
func (h *Handlers) UploadDocument(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	const maxDocumentUploadPayloadBytes = documents.MaxDocumentSizeBytes + (1 << 20)
	r.Body = http.MaxBytesReader(w, r.Body, maxDocumentUploadPayloadBytes)
	// #nosec G120 -- MaxBytesReader caps the request body at this same bounded payload limit.
	if err := r.ParseMultipartForm(maxDocumentUploadPayloadBytes); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid multipart form payload")
		return
	}

	entityType := strings.TrimSpace(r.FormValue("entity_type"))
	entityID := strings.TrimSpace(r.FormValue("entity_id"))
	documentType := strings.TrimSpace(r.FormValue("document_type"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	rawRetention := strings.TrimSpace(r.FormValue("retention_until"))
	rawRetentionYears := strings.TrimSpace(r.FormValue("retention_years"))
	if rawRetention != "" && rawRetentionYears != "" {
		respondError(w, http.StatusBadRequest, "retention_until and retention_years cannot be combined")
		return
	}
	var retentionUntil *time.Time
	if rawRetention != "" {
		parsed, err := time.Parse("2006-01-02", rawRetention)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid retention_until date, expected YYYY-MM-DD")
			return
		}
		normalized := parsed.UTC()
		retentionUntil = &normalized
	}
	retentionYears := 0
	if rawRetentionYears != "" {
		parsed, err := strconv.Atoi(rawRetentionYears)
		if err != nil || parsed < 0 {
			respondError(w, http.StatusBadRequest, "retention_years must be zero or greater")
			return
		}
		if parsed > documents.MaxRetentionYears {
			respondError(w, http.StatusBadRequest, "retention_years cannot exceed "+strconv.Itoa(documents.MaxRetentionYears))
			return
		}
		retentionYears = parsed
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "File is required")
		return
	}
	defer func() {
		_ = file.Close()
	}()

	doc, err := h.documentsService.UploadDocument(r.Context(), schemaName, tenantID, &documents.UploadDocumentRequest{
		EntityType:     entityType,
		EntityID:       entityID,
		DocumentType:   documentType,
		FileName:       header.Filename,
		ContentType:    header.Header.Get("Content-Type"),
		FileSize:       header.Size,
		Notes:          notes,
		RetentionUntil: retentionUntil,
		RetentionYears: retentionYears,
		UploadedBy:     claims.UserID,
	}, file)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, doc)
}

// MarkDocumentReviewed marks a document as reviewed.
// @Summary Mark document reviewed
// @Description Mark a document reviewed by the current user
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param documentID path string true "Document ID"
// @Success 200 {object} documents.Document
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/{documentID}/mark-reviewed [post]
func (h *Handlers) MarkDocumentReviewed(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	documentID := chi.URLParam(r, "documentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	doc, err := h.documentsService.MarkDocumentReviewed(r.Context(), schemaName, tenantID, documentID, claims.UserID)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, doc)
}

// ReviewDocument records an explicit document review decision.
// @Summary Review document
// @Description Set a document review status and optional review note
// @Tags Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param documentID path string true "Document ID"
// @Param request body documents.ReviewDocumentRequest true "Review decision"
// @Success 200 {object} documents.Document
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/{documentID}/review [post]
func (h *Handlers) ReviewDocument(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	documentID := chi.URLParam(r, "documentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	var req documents.ReviewDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	doc, err := h.documentsService.ReviewDocument(r.Context(), schemaName, tenantID, documentID, claims.UserID, &req)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, doc)
}

// DownloadDocument downloads the stored document file.
// @Summary Download document
// @Description Stream a stored document file by document ID
// @Tags Documents
// @Produce application/octet-stream
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param documentID path string true "Document ID"
// @Success 200 {file} file
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/{documentID}/download [get]
func (h *Handlers) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	documentID := chi.URLParam(r, "documentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	doc, reader, err := h.documentsService.OpenDocument(r.Context(), schemaName, tenantID, documentID)
	if err != nil {
		respondDocumentError(w, err)
		return
	}
	defer func() {
		_ = reader.Close()
	}()

	w.Header().Set("Content-Type", doc.ContentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+doc.FileName+`"`)
	if doc.FileSize > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(doc.FileSize, 10))
	}

	if _, err := io.Copy(w, reader); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
}

// DeleteDocument deletes a stored document.
// @Summary Delete document
// @Description Delete a document and its stored file
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param tenantID path string true "Tenant ID"
// @Param documentID path string true "Document ID"
// @Success 200 {object} object{status=string}
// @Failure 400 {object} object{error=string}
// @Failure 404 {object} object{error=string}
// @Failure 500 {object} object{error=string}
// @Router /tenants/{tenantID}/documents/{documentID} [delete]
func (h *Handlers) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	documentID := chi.URLParam(r, "documentID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	if err := h.documentsService.DeleteDocument(r.Context(), schemaName, tenantID, documentID); err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func respondDocumentError(w http.ResponseWriter, err error) {
	message := err.Error()
	switch {
	case strings.Contains(message, "not found"):
		respondError(w, http.StatusNotFound, message)
	case strings.Contains(message, "unsupported"),
		strings.Contains(message, "required"),
		strings.Contains(message, "empty"),
		strings.Contains(message, "limit"),
		strings.Contains(message, "cannot"),
		strings.Contains(message, "invalid"),
		strings.Contains(message, "must"):
		respondError(w, http.StatusBadRequest, message)
	default:
		respondError(w, http.StatusInternalServerError, message)
	}
}
