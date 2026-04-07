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

func (h *Handlers) UploadDocument(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.GetClaims(r.Context())
	tenantID := chi.URLParam(r, "tenantID")
	schemaName := h.getSchemaName(r.Context(), tenantID)

	r.Body = http.MaxBytesReader(w, r.Body, documents.MaxDocumentSizeBytes+(1<<20))
	if err := r.ParseMultipartForm(documents.MaxDocumentSizeBytes + (1 << 20)); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid multipart form payload")
		return
	}

	entityType := strings.TrimSpace(r.FormValue("entity_type"))
	entityID := strings.TrimSpace(r.FormValue("entity_id"))
	documentType := strings.TrimSpace(r.FormValue("document_type"))
	notes := strings.TrimSpace(r.FormValue("notes"))
	var retentionUntil *time.Time
	if rawRetention := strings.TrimSpace(r.FormValue("retention_until")); rawRetention != "" {
		parsed, err := time.Parse("2006-01-02", rawRetention)
		if err != nil {
			respondError(w, http.StatusBadRequest, "Invalid retention_until date, expected YYYY-MM-DD")
			return
		}
		normalized := parsed.UTC()
		retentionUntil = &normalized
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
		UploadedBy:     claims.UserID,
	}, file)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, doc)
}

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
		strings.Contains(message, "invalid"):
		respondError(w, http.StatusBadRequest, message)
	default:
		respondError(w, http.StatusInternalServerError, message)
	}
}
