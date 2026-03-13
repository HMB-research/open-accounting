package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type mockDocumentRepository struct {
	entityExists bool
	docs         map[string]*documents.Document
}

func newMockDocumentRepository() *mockDocumentRepository {
	return &mockDocumentRepository{
		entityExists: true,
		docs:         make(map[string]*documents.Document),
	}
}

func (m *mockDocumentRepository) EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error) {
	return m.entityExists, nil
}

func (m *mockDocumentRepository) CreateDocument(ctx context.Context, schemaName string, doc *documents.Document) error {
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockDocumentRepository) ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]documents.Document, error) {
	result := make([]documents.Document, 0, len(m.docs))
	for _, doc := range m.docs {
		if doc.TenantID == tenantID && doc.EntityType == entityType && doc.EntityID == entityID {
			result = append(result, *doc)
		}
	}
	return result, nil
}

func (m *mockDocumentRepository) ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]documents.ReviewSummary, error) {
	result := make(map[string]documents.ReviewSummary, len(entityIDs))
	for _, entityID := range entityIDs {
		total := 0
		pending := 0
		reviewed := 0
		for _, doc := range m.docs {
			if doc.TenantID != tenantID || doc.EntityType != entityType || doc.EntityID != entityID {
				continue
			}
			total++
			switch doc.ReviewStatus {
			case documents.ReviewStatusReviewed:
				reviewed++
			default:
				pending++
			}
		}
		result[entityID] = documents.ReviewSummary{
			EntityType:         entityType,
			EntityID:           entityID,
			TotalCount:         total,
			PendingReviewCount: pending,
			ReviewedCount:      reviewed,
			MissingEvidence:    total == 0,
			HasPendingReview:   pending > 0,
		}
	}
	return result, nil
}

func (m *mockDocumentRepository) GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*documents.Document, error) {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return nil, io.EOF
	}
	return doc, nil
}

func (m *mockDocumentRepository) MarkDocumentReviewed(ctx context.Context, schemaName, tenantID, documentID, reviewedBy string, reviewedAt time.Time) error {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return io.EOF
	}
	doc.ReviewStatus = documents.ReviewStatusReviewed
	doc.ReviewedBy = &reviewedBy
	doc.ReviewedAt = &reviewedAt
	return nil
}

func (m *mockDocumentRepository) DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error {
	delete(m.docs, documentID)
	return nil
}

func setupDocumentHandlers(t *testing.T) (*Handlers, *mockDocumentRepository) {
	t.Helper()

	store, err := documents.NewLocalStore(t.TempDir())
	require.NoError(t, err)

	tenantRepo := newMockTenantRepository()
	repo := newMockDocumentRepository()

	return &Handlers{
		tenantService:    tenant.NewServiceWithRepository(tenantRepo),
		documentsService: documents.NewService(repo, store),
		tokenService:     auth.NewTokenService("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour),
	}, repo
}

func TestUploadListDownloadAndDeleteDocument(t *testing.T) {
	h, repo := setupDocumentHandlers(t)
	claims := createTestClaims("user-1", "user@example.com", "tenant-1", "admin")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("entity_type", documents.EntityTypeBankTxn))
	require.NoError(t, writer.WriteField("entity_id", "txn-1"))
	require.NoError(t, writer.WriteField("document_type", documents.DocumentTypeReconciliation))
	require.NoError(t, writer.WriteField("notes", "Matched against statement export"))
	require.NoError(t, writer.WriteField("retention_until", "2027-03-31"))

	part, err := writer.CreateFormFile("file", "invoice.pdf")
	require.NoError(t, err)
	_, err = io.Copy(part, bytes.NewBufferString("invoice pdf"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	uploadReq := httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/documents", &body)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq = withURLParams(uploadReq, map[string]string{"tenantID": "tenant-1"})
	uploadReq = uploadReq.WithContext(contextWithClaims(uploadReq.Context(), claims))

	uploadResp := httptest.NewRecorder()
	h.UploadDocument(uploadResp, uploadReq)
	require.Equal(t, http.StatusCreated, uploadResp.Code)

	var uploaded documents.Document
	require.NoError(t, json.NewDecoder(uploadResp.Body).Decode(&uploaded))
	require.NotEmpty(t, uploaded.ID)
	require.Equal(t, documents.DocumentTypeReconciliation, uploaded.DocumentType)
	require.Equal(t, "Matched against statement export", uploaded.Notes)
	require.Equal(t, documents.ReviewStatusPending, uploaded.ReviewStatus)
	require.NotNil(t, uploaded.RetentionUntil)
	require.Len(t, repo.docs, 1)

	listReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents?entity_type=bank_transaction&entity_id=txn-1", nil, claims)
	listReq = withURLParams(listReq, map[string]string{"tenantID": "tenant-1"})
	listResp := httptest.NewRecorder()
	h.ListDocuments(listResp, listReq)
	require.Equal(t, http.StatusOK, listResp.Code)

	var listed []documents.Document
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	require.Len(t, listed, 1)

	summaryReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/review-summary", map[string]any{
		"entity_type": "bank_transaction",
		"entity_ids":  []string{"txn-1", "txn-2"},
	}, claims)
	summaryReq = withURLParams(summaryReq, map[string]string{"tenantID": "tenant-1"})
	summaryResp := httptest.NewRecorder()
	h.ListDocumentReviewSummaries(summaryResp, summaryReq)
	require.Equal(t, http.StatusOK, summaryResp.Code)

	var summaries []documents.ReviewSummary
	require.NoError(t, json.NewDecoder(summaryResp.Body).Decode(&summaries))
	require.Len(t, summaries, 2)
	require.Equal(t, "txn-1", summaries[0].EntityID)
	require.False(t, summaries[0].MissingEvidence)
	require.Equal(t, "txn-2", summaries[1].EntityID)
	require.True(t, summaries[1].MissingEvidence)

	reviewReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/"+uploaded.ID+"/mark-reviewed", nil, claims)
	reviewReq = withURLParams(reviewReq, map[string]string{"tenantID": "tenant-1", "documentID": uploaded.ID})
	reviewResp := httptest.NewRecorder()
	h.MarkDocumentReviewed(reviewResp, reviewReq)
	require.Equal(t, http.StatusOK, reviewResp.Code)

	var reviewed documents.Document
	require.NoError(t, json.NewDecoder(reviewResp.Body).Decode(&reviewed))
	require.Equal(t, documents.ReviewStatusReviewed, reviewed.ReviewStatus)
	require.NotNil(t, reviewed.ReviewedBy)
	require.Equal(t, "user-1", *reviewed.ReviewedBy)

	downloadReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/"+uploaded.ID+"/download", nil, claims)
	downloadReq = withURLParams(downloadReq, map[string]string{"tenantID": "tenant-1", "documentID": uploaded.ID})
	downloadResp := httptest.NewRecorder()
	h.DownloadDocument(downloadResp, downloadReq)
	require.Equal(t, http.StatusOK, downloadResp.Code)
	require.Equal(t, "invoice pdf", downloadResp.Body.String())

	deleteReq := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/documents/"+uploaded.ID, nil, claims)
	deleteReq = withURLParams(deleteReq, map[string]string{"tenantID": "tenant-1", "documentID": uploaded.ID})
	deleteResp := httptest.NewRecorder()
	h.DeleteDocument(deleteResp, deleteReq)
	require.Equal(t, http.StatusOK, deleteResp.Code)
	require.Empty(t, repo.docs)
}
