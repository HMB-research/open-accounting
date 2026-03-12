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

func (m *mockDocumentRepository) GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*documents.Document, error) {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return nil, io.EOF
	}
	return doc, nil
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
	require.NoError(t, writer.WriteField("entity_type", documents.EntityTypeInvoice))
	require.NoError(t, writer.WriteField("entity_id", "inv-1"))

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
	require.Len(t, repo.docs, 1)

	listReq := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/documents?entity_type=invoice&entity_id=inv-1", nil, claims)
	listReq = withURLParams(listReq, map[string]string{"tenantID": "tenant-1"})
	listResp := httptest.NewRecorder()
	h.ListDocuments(listResp, listReq)
	require.Equal(t, http.StatusOK, listResp.Code)

	var listed []documents.Document
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listed))
	require.Len(t, listed, 1)

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
