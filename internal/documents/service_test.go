package documents

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type mockRepository struct {
	entityExists bool
	docs         map[string]*Document
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		entityExists: true,
		docs:         make(map[string]*Document),
	}
}

func (m *mockRepository) EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error) {
	return m.entityExists, nil
}

func (m *mockRepository) CreateDocument(ctx context.Context, schemaName string, doc *Document) error {
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockRepository) ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error) {
	result := make([]Document, 0, len(m.docs))
	for _, doc := range m.docs {
		if doc.TenantID == tenantID && doc.EntityType == entityType && doc.EntityID == entityID {
			result = append(result, *doc)
		}
	}
	return result, nil
}

func (m *mockRepository) GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error) {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return nil, os.ErrNotExist
	}
	return doc, nil
}

func (m *mockRepository) DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error {
	delete(m.docs, documentID)
	return nil
}

func TestService_UploadOpenListAndDeleteDocument(t *testing.T) {
	t.Parallel()

	rootDir := t.TempDir()
	store, err := NewLocalStore(rootDir)
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}

	repo := newMockRepository()
	svc := NewService(repo, store)

	doc, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", &UploadDocumentRequest{
		EntityType:  EntityTypeInvoice,
		EntityID:    "inv-1",
		FileName:    "invoice 001.pdf",
		ContentType: "application/pdf",
		FileSize:    int64(len("hello world")),
		UploadedBy:  "user-1",
	}, bytes.NewBufferString("hello world"))
	if err != nil {
		t.Fatalf("UploadDocument failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, doc.StorageKey)); err != nil {
		t.Fatalf("expected stored file to exist: %v", err)
	}

	listed, err := svc.ListDocuments(context.Background(), "tenant_demo", "tenant-1", EntityTypeInvoice, "inv-1")
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed document, got %d", len(listed))
	}

	openedDoc, reader, err := svc.OpenDocument(context.Background(), "tenant_demo", "tenant-1", doc.ID)
	if err != nil {
		t.Fatalf("OpenDocument failed: %v", err)
	}
	defer reader.Close()

	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(payload) != "hello world" {
		t.Fatalf("unexpected payload %q", string(payload))
	}
	if openedDoc.FileName != "invoice_001.pdf" {
		t.Fatalf("unexpected sanitized file name %q", openedDoc.FileName)
	}

	if err := svc.DeleteDocument(context.Background(), "tenant_demo", "tenant-1", doc.ID); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, doc.StorageKey)); !os.IsNotExist(err) {
		t.Fatalf("expected stored file to be deleted, got err=%v", err)
	}
}
