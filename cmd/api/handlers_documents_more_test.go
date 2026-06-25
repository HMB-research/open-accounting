package main

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/documents"
)

func TestDocumentHandlersValidationAndServiceErrors(t *testing.T) {
	h, repo := setupDocumentHandlers(t)
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	tests := []struct {
		name       string
		handler    func(http.ResponseWriter, *http.Request)
		request    *http.Request
		wantStatus int
		wantBody   string
	}{
		{
			name:       "list requires entity filters",
			handler:    h.ListDocuments,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents", nil, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "entity_type and entity_id are required",
		},
		{
			name:       "review summary rejects invalid JSON",
			handler:    h.ListDocumentReviewSummaries,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents/review-summary", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "review summary requires entities",
			handler:    h.ListDocumentReviewSummaries,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/review-summary", map[string]any{"entity_type": "expense"}, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "entity_type and entity_ids are required",
		},
		{
			name:       "review queue rejects invalid limit",
			handler:    h.GetDocumentReviewQueue,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/review-queue?limit=-1", nil, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "limit must be zero or greater",
		},
		{
			name:       "retention review rejects invalid as_of",
			handler:    h.GetDocumentRetentionReview,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/retention?as_of=2026/03/15", nil, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid as_of date",
		},
		{
			name:       "retention review rejects invalid horizon",
			handler:    h.GetDocumentRetentionReview,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/retention?horizon_days=-1", nil, claims), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "horizon_days must be zero or greater",
		},
		{
			name:       "purge rejects invalid JSON",
			handler:    h.PurgeExpiredDocuments,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents/purge", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "retention update rejects invalid JSON",
			handler:    h.UpdateDocumentRetention,
			request:    withURLParams(httptest.NewRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/retention", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "retention update requires value or clear",
			handler:    h.UpdateDocumentRetention,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/retention", map[string]any{}, claims), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "retention_until is required",
		},
		{
			name:       "retention update rejects invalid date",
			handler:    h.UpdateDocumentRetention,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/retention", map[string]any{"retention_until": "2026/03/15"}, claims), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid retention_until date",
		},
		{
			name:       "evidence policy rejects invalid JSON",
			handler:    h.EvaluateDocumentEvidencePolicy,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents/evidence-policy", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "lifecycle rejects invalid JSON",
			handler:    h.UpdateDocumentLifecycle,
			request:    withURLParams(httptest.NewRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/lifecycle", strings.NewReader("{")).WithContext(contextWithClaims(httptest.NewRequest(http.MethodPatch, "/", nil).Context(), claims)), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "legal hold rejects invalid JSON",
			handler:    h.UpdateDocumentLegalHold,
			request:    withURLParams(httptest.NewRequest(http.MethodPatch, "/tenants/tenant-1/documents/doc-1/legal-hold", strings.NewReader("{")).WithContext(contextWithClaims(httptest.NewRequest(http.MethodPatch, "/", nil).Context(), claims)), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "review rejects invalid JSON",
			handler:    h.ReviewDocument,
			request:    withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents/doc-1/review", strings.NewReader("{")).WithContext(contextWithClaims(httptest.NewRequest(http.MethodPost, "/", nil).Context(), claims)), map[string]string{"tenantID": "tenant-1", "documentID": "doc-1"}),
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid JSON payload",
		},
		{
			name:       "download maps missing document",
			handler:    h.DownloadDocument,
			request:    withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/missing/download", nil, claims), map[string]string{"tenantID": "tenant-1", "documentID": "missing"}),
			wantStatus: http.StatusInternalServerError,
			wantBody:   "EOF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			tt.handler(resp, tt.request)
			require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), tt.wantBody)
		})
	}

	repo.listDocumentsErr = errors.New("database unavailable")
	listReq := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents?entity_type=expense&entity_id=expense-1", nil, claims), map[string]string{"tenantID": "tenant-1"})
	listResp := httptest.NewRecorder()
	h.ListDocuments(listResp, listReq)
	require.Equal(t, http.StatusInternalServerError, listResp.Code, listResp.Body.String())
	require.Contains(t, listResp.Body.String(), "database unavailable")
}

func TestUploadDocumentValidationBranches(t *testing.T) {
	h, _ := setupDocumentHandlers(t)
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	malformedReq := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents", strings.NewReader("--bad"))
	malformedReq.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	malformedReq = malformedReq.WithContext(contextWithClaims(malformedReq.Context(), claims))
	malformedReq = withURLParams(malformedReq, map[string]string{"tenantID": "tenant-1"})
	malformedResp := httptest.NewRecorder()
	h.UploadDocument(malformedResp, malformedReq)
	require.Equal(t, http.StatusBadRequest, malformedResp.Code, malformedResp.Body.String())
	require.Contains(t, malformedResp.Body.String(), "Invalid multipart form payload")

	tests := []struct {
		name       string
		fields     map[string]string
		withFile   bool
		wantStatus int
		wantBody   string
	}{
		{
			name: "retention_until rejects invalid date",
			fields: map[string]string{
				"entity_type":     documents.EntityTypeExpense,
				"entity_id":       "expense-1",
				"document_type":   documents.DocumentTypeReceipt,
				"retention_until": "2026/03/15",
			},
			withFile:   false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "Invalid retention_until date",
		},
		{
			name: "retention_years rejects invalid integer",
			fields: map[string]string{
				"entity_type":     documents.EntityTypeExpense,
				"entity_id":       "expense-1",
				"document_type":   documents.DocumentTypeReceipt,
				"retention_years": "-1",
			},
			withFile:   false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "retention_years must be zero or greater",
		},
		{
			name: "retention_years rejects excessive value",
			fields: map[string]string{
				"entity_type":     documents.EntityTypeExpense,
				"entity_id":       "expense-1",
				"document_type":   documents.DocumentTypeReceipt,
				"retention_years": "101",
			},
			withFile:   false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "retention_years cannot exceed",
		},
		{
			name: "file is required",
			fields: map[string]string{
				"entity_type":   documents.EntityTypeExpense,
				"entity_id":     "expense-1",
				"document_type": documents.DocumentTypeReceipt,
			},
			withFile:   false,
			wantStatus: http.StatusBadRequest,
			wantBody:   "File is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := multipartDocumentRequest(t, tt.fields, tt.withFile, claims)
			resp := httptest.NewRecorder()
			h.UploadDocument(resp, req)
			require.Equal(t, tt.wantStatus, resp.Code, resp.Body.String())
			require.Contains(t, resp.Body.String(), tt.wantBody)
		})
	}
}

func multipartDocumentRequest(t *testing.T, fields map[string]string, withFile bool, claims *auth.Claims) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	if withFile {
		part, err := writer.CreateFormFile("file", "receipt.pdf")
		require.NoError(t, err)
		_, err = io.Copy(part, bytes.NewBufferString("receipt"))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = req.WithContext(contextWithClaims(req.Context(), claims))
	return withURLParams(req, map[string]string{"tenantID": "tenant-1"})
}
