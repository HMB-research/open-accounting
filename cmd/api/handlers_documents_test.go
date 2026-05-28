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

func (m *mockDocumentRepository) ListRetentionReviewDocuments(ctx context.Context, schemaName, tenantID string, cutoff time.Time, includeMissing bool) ([]documents.Document, error) {
	result := make([]documents.Document, 0, len(m.docs))
	for _, doc := range m.docs {
		if doc.TenantID != tenantID {
			continue
		}
		if doc.RetentionUntil == nil {
			if includeMissing {
				result = append(result, *doc)
			}
			continue
		}
		if !doc.RetentionUntil.After(cutoff) {
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
		approved := 0
		rejected := 0
		for _, doc := range m.docs {
			if doc.TenantID != tenantID || doc.EntityType != entityType || doc.EntityID != entityID {
				continue
			}
			total++
			switch doc.ReviewStatus {
			case documents.ReviewStatusReviewed, documents.ReviewStatusApproved, documents.ReviewStatusRejected:
				reviewed++
				if doc.ReviewStatus == documents.ReviewStatusApproved {
					approved++
				}
				if doc.ReviewStatus == documents.ReviewStatusRejected {
					rejected++
				}
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
			ApprovedCount:      approved,
			RejectedCount:      rejected,
			MissingEvidence:    total == 0,
			HasPendingReview:   pending > 0,
			HasRejected:        rejected > 0,
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

func (m *mockDocumentRepository) ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewStatus, reviewNote, reviewedBy string, reviewedAt time.Time) error {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return io.EOF
	}
	doc.ReviewStatus = reviewStatus
	doc.ReviewNote = reviewNote
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

	retentionReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents/retention?as_of=2027-03-01&horizon_days=45", nil, claims)
	retentionReq = withURLParams(retentionReq, map[string]string{"tenantID": "tenant-1"})
	retentionResp := httptest.NewRecorder()
	h.GetDocumentRetentionReview(retentionResp, retentionReq)
	require.Equal(t, http.StatusOK, retentionResp.Code)

	var retentionReview documents.RetentionReview
	require.NoError(t, json.NewDecoder(retentionResp.Body).Decode(&retentionReview))
	require.Equal(t, 1, retentionReview.TotalCount)
	require.Equal(t, 1, retentionReview.DueSoonCount)
	require.Len(t, retentionReview.Documents, 1)

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

	approveReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/"+uploaded.ID+"/review", map[string]any{
		"review_status": "APPROVED",
		"review_note":   "Evidence matches bank statement",
	}, claims)
	approveReq = withURLParams(approveReq, map[string]string{"tenantID": "tenant-1", "documentID": uploaded.ID})
	approveResp := httptest.NewRecorder()
	h.ReviewDocument(approveResp, approveReq)
	require.Equal(t, http.StatusOK, approveResp.Code)

	var approved documents.Document
	require.NoError(t, json.NewDecoder(approveResp.Body).Decode(&approved))
	require.Equal(t, documents.ReviewStatusApproved, approved.ReviewStatus)
	require.Equal(t, "Evidence matches bank statement", approved.ReviewNote)

	policyReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/evidence-policy", map[string]any{
		"entity_type": "bank_transaction",
		"entity_ids":  []string{"txn-1", "txn-2"},
		"rules": []map[string]any{{
			"document_types":   []string{"reconciliation_evidence"},
			"min_count":        1,
			"require_approved": true,
		}},
	}, claims)
	policyReq = withURLParams(policyReq, map[string]string{"tenantID": "tenant-1"})
	policyResp := httptest.NewRecorder()
	h.EvaluateDocumentEvidencePolicy(policyResp, policyReq)
	require.Equal(t, http.StatusOK, policyResp.Code)

	var policyResults []documents.EvidencePolicyResult
	require.NoError(t, json.NewDecoder(policyResp.Body).Decode(&policyResults))
	require.Len(t, policyResults, 2)
	require.Equal(t, "txn-1", policyResults[0].EntityID)
	require.True(t, policyResults[0].Compliant)
	require.Equal(t, 1, policyResults[0].ApprovedDocumentTypeCounts[documents.DocumentTypeReconciliation])
	require.Equal(t, "txn-2", policyResults[1].EntityID)
	require.False(t, policyResults[1].Compliant)
	require.True(t, policyResults[1].MissingEvidence)

	rejectReq := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/documents/"+uploaded.ID+"/review", map[string]any{
		"review_status": "REJECTED",
	}, claims)
	rejectReq = withURLParams(rejectReq, map[string]string{"tenantID": "tenant-1", "documentID": uploaded.ID})
	rejectResp := httptest.NewRecorder()
	h.ReviewDocument(rejectResp, rejectReq)
	require.Equal(t, http.StatusBadRequest, rejectResp.Code)

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
