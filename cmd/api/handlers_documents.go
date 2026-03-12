package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"

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
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "File is required")
		return
	}
	defer func() {
		_ = file.Close()
	}()

	doc, err := h.documentsService.UploadDocument(r.Context(), schemaName, tenantID, &documents.UploadDocumentRequest{
		EntityType:  entityType,
		EntityID:    entityID,
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		FileSize:    header.Size,
		UploadedBy:  claims.UserID,
	}, file)
	if err != nil {
		respondDocumentError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, doc)
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
		strings.Contains(message, "limit"):
		respondError(w, http.StatusBadRequest, message)
	default:
		respondError(w, http.StatusInternalServerError, message)
	}
}
