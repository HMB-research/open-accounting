package documents

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func (m *mockRepository) ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]ReviewSummary, error) {
	result := make(map[string]ReviewSummary, len(entityIDs))
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
			case ReviewStatusReviewed:
				reviewed++
			default:
				pending++
			}
		}
		if total == 0 {
			continue
		}
		result[entityID] = ReviewSummary{
			EntityType:         entityType,
			EntityID:           entityID,
			TotalCount:         total,
			PendingReviewCount: pending,
			ReviewedCount:      reviewed,
			MissingEvidence:    false,
			HasPendingReview:   pending > 0,
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

func (m *mockRepository) MarkDocumentReviewed(ctx context.Context, schemaName, tenantID, documentID, reviewedBy string, reviewedAt time.Time) error {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return os.ErrNotExist
	}
	doc.ReviewStatus = ReviewStatusReviewed
	doc.ReviewedBy = &reviewedBy
	doc.ReviewedAt = &reviewedAt
	return nil
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
		EntityType:   EntityTypeBankTxn,
		EntityID:     "txn-1",
		DocumentType: DocumentTypeReconciliation,
		FileName:     "invoice 001.pdf",
		ContentType:  "application/pdf",
		FileSize:     int64(len("hello world")),
		Notes:        "Matched to March bank statement",
		UploadedBy:   "user-1",
	}, bytes.NewBufferString("hello world"))
	if err != nil {
		t.Fatalf("UploadDocument failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, doc.StorageKey)); err != nil {
		t.Fatalf("expected stored file to exist: %v", err)
	}

	listed, err := svc.ListDocuments(context.Background(), "tenant_demo", "tenant-1", EntityTypeBankTxn, "txn-1")
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed document, got %d", len(listed))
	}
	if listed[0].DocumentType != DocumentTypeReconciliation {
		t.Fatalf("expected document type %q, got %q", DocumentTypeReconciliation, listed[0].DocumentType)
	}
	if listed[0].Notes != "Matched to March bank statement" {
		t.Fatalf("unexpected notes %q", listed[0].Notes)
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
	if openedDoc.ReviewStatus != ReviewStatusPending {
		t.Fatalf("expected pending review status, got %q", openedDoc.ReviewStatus)
	}

	reviewedDoc, err := svc.MarkDocumentReviewed(context.Background(), "tenant_demo", "tenant-1", doc.ID, "reviewer-1")
	if err != nil {
		t.Fatalf("MarkDocumentReviewed failed: %v", err)
	}
	if reviewedDoc.ReviewStatus != ReviewStatusReviewed {
		t.Fatalf("expected reviewed status, got %q", reviewedDoc.ReviewStatus)
	}
	if reviewedDoc.ReviewedBy == nil || *reviewedDoc.ReviewedBy != "reviewer-1" {
		t.Fatalf("expected reviewer-1, got %#v", reviewedDoc.ReviewedBy)
	}

	summaries, err := svc.ListReviewSummaries(context.Background(), "tenant_demo", "tenant-1", EntityTypeBankTxn, []string{"txn-1", "txn-2"})
	if err != nil {
		t.Fatalf("ListReviewSummaries failed: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].EntityID != "txn-1" || summaries[0].ReviewedCount != 1 {
		t.Fatalf("unexpected first summary: %#v", summaries[0])
	}
	if summaries[1].EntityID != "txn-2" || !summaries[1].MissingEvidence {
		t.Fatalf("unexpected missing-evidence summary: %#v", summaries[1])
	}

	if err := svc.DeleteDocument(context.Background(), "tenant_demo", "tenant-1", doc.ID); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, doc.StorageKey)); !os.IsNotExist(err) {
		t.Fatalf("expected stored file to be deleted, got err=%v", err)
	}
}
