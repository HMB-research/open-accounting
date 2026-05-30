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

func (m *mockRepository) ListReviewQueueDocuments(ctx context.Context, schemaName, tenantID string, filter ReviewQueueFilter) ([]Document, error) {
	result := make([]Document, 0, len(m.docs))
	for _, doc := range m.docs {
		if doc.TenantID != tenantID {
			continue
		}
		if filter.EntityType != "" && doc.EntityType != filter.EntityType {
			continue
		}
		if filter.DocumentType != "" && doc.DocumentType != filter.DocumentType {
			continue
		}
		if filter.ReviewStatus != "" && doc.ReviewStatus != filter.ReviewStatus {
			continue
		}
		result = append(result, *doc)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func (m *mockRepository) ListRetentionReviewDocuments(ctx context.Context, schemaName, tenantID string, cutoff time.Time, includeMissing bool) ([]Document, error) {
	result := make([]Document, 0, len(m.docs))
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

func (m *mockRepository) ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]ReviewSummary, error) {
	result := make(map[string]ReviewSummary, len(entityIDs))
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
			case ReviewStatusReviewed, ReviewStatusApproved, ReviewStatusRejected:
				reviewed++
				if doc.ReviewStatus == ReviewStatusApproved {
					approved++
				}
				if doc.ReviewStatus == ReviewStatusRejected {
					rejected++
				}
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
			ApprovedCount:      approved,
			RejectedCount:      rejected,
			MissingEvidence:    false,
			HasPendingReview:   pending > 0,
			HasRejected:        rejected > 0,
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

func (m *mockRepository) UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) error {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return os.ErrNotExist
	}
	doc.RetentionUntil = retentionUntil
	return nil
}

func (m *mockRepository) ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewStatus, reviewNote, reviewedBy string, reviewedAt time.Time) error {
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return os.ErrNotExist
	}
	doc.ReviewStatus = reviewStatus
	doc.ReviewNote = reviewNote
	doc.ReviewedBy = &reviewedBy
	doc.ReviewedAt = &reviewedAt
	return nil
}

func (m *mockRepository) DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error {
	delete(m.docs, documentID)
	return nil
}

func TestService_GetReviewQueueFiltersClosePackDocuments(t *testing.T) {
	t.Parallel()

	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	repo := newMockRepository()
	svc := NewService(repo, store)
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	repo.docs["doc-close-pack"] = &Document{
		ID:           "doc-close-pack",
		TenantID:     "tenant-1",
		EntityType:   EntityTypeYearEndClose,
		EntityID:     "year-end-close-2025",
		DocumentType: DocumentTypeClosePack,
		FileName:     "close-pack.pdf",
		ReviewStatus: ReviewStatusPending,
		UploadedBy:   "user-1",
		CreatedAt:    now,
	}
	repo.docs["doc-receipt"] = &Document{
		ID:           "doc-receipt",
		TenantID:     "tenant-1",
		EntityType:   EntityTypePayment,
		EntityID:     "pay-1",
		DocumentType: DocumentTypeReceipt,
		FileName:     "receipt.pdf",
		ReviewStatus: ReviewStatusPending,
		UploadedBy:   "user-1",
		CreatedAt:    now,
	}
	repo.docs["doc-approved"] = &Document{
		ID:           "doc-approved",
		TenantID:     "tenant-1",
		EntityType:   EntityTypeYearEndClose,
		EntityID:     "year-end-close-2024",
		DocumentType: DocumentTypeClosePack,
		FileName:     "approved-close-pack.pdf",
		ReviewStatus: ReviewStatusApproved,
		UploadedBy:   "user-1",
		CreatedAt:    now,
	}

	queue, err := svc.GetReviewQueue(context.Background(), "tenant_demo", "tenant-1", ReviewQueueFilter{
		EntityType:   EntityTypeYearEndClose,
		DocumentType: DocumentTypeClosePack,
	})
	if err != nil {
		t.Fatalf("GetReviewQueue failed: %v", err)
	}
	if queue.ReviewStatus != ReviewStatusPending || queue.Limit != defaultReviewQueueLimit {
		t.Fatalf("unexpected queue metadata: %#v", queue)
	}
	if queue.TotalCount != 1 || queue.PendingReviewCount != 1 || queue.Documents[0].ID != "doc-close-pack" {
		t.Fatalf("unexpected filtered queue: %#v", queue)
	}

	allQueue, err := svc.GetReviewQueue(context.Background(), "tenant_demo", "tenant-1", ReviewQueueFilter{
		EntityType:   EntityTypeYearEndClose,
		DocumentType: DocumentTypeClosePack,
		ReviewStatus: "all",
	})
	if err != nil {
		t.Fatalf("GetReviewQueue all failed: %v", err)
	}
	if allQueue.TotalCount != 2 || allQueue.ApprovedCount != 1 || allQueue.ReviewStatus != "ALL" {
		t.Fatalf("unexpected all-status queue: %#v", allQueue)
	}
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
		EntityType:     EntityTypeBankTxn,
		EntityID:       "txn-1",
		DocumentType:   DocumentTypeReconciliation,
		FileName:       "invoice 001.pdf",
		ContentType:    "application/pdf",
		FileSize:       int64(len("hello world")),
		Notes:          "Matched to March bank statement",
		RetentionYears: 7,
		UploadedBy:     "user-1",
	}, bytes.NewBufferString("hello world"))
	if err != nil {
		t.Fatalf("UploadDocument failed: %v", err)
	}
	expectedRetention := dateOnlyUTC(doc.CreatedAt.AddDate(7, 0, 0))
	if doc.RetentionUntil == nil || !doc.RetentionUntil.Equal(expectedRetention) {
		t.Fatalf("expected retention until %s, got %#v", expectedRetention.Format("2006-01-02"), doc.RetentionUntil)
	}

	if _, err := os.Stat(filepath.Join(rootDir, doc.StorageKey)); err != nil {
		t.Fatalf("expected stored file to exist: %v", err)
	}
	explicitRetention := time.Date(2029, 3, 31, 0, 0, 0, 0, time.UTC)
	if _, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", &UploadDocumentRequest{
		EntityType:     EntityTypeBankTxn,
		EntityID:       "txn-1",
		DocumentType:   DocumentTypeReconciliation,
		FileName:       "conflict.pdf",
		ContentType:    "application/pdf",
		FileSize:       int64(len("hello world")),
		RetentionUntil: &explicitRetention,
		RetentionYears: 7,
		UploadedBy:     "user-1",
	}, bytes.NewBufferString("hello world")); err == nil {
		t.Fatal("expected retention_until and retention_years to conflict")
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

	rejectedDoc, err := svc.ReviewDocument(context.Background(), "tenant_demo", "tenant-1", doc.ID, "reviewer-2", &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusRejected,
		ReviewNote:   "Receipt does not match bank statement",
	})
	if err != nil {
		t.Fatalf("ReviewDocument failed: %v", err)
	}
	if rejectedDoc.ReviewStatus != ReviewStatusRejected {
		t.Fatalf("expected rejected status, got %q", rejectedDoc.ReviewStatus)
	}
	if rejectedDoc.ReviewNote != "Receipt does not match bank statement" {
		t.Fatalf("unexpected review note %q", rejectedDoc.ReviewNote)
	}
	if _, err := svc.ReviewDocument(context.Background(), "tenant_demo", "tenant-1", doc.ID, "reviewer-2", &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusRejected,
	}); err == nil {
		t.Fatal("expected rejected documents to require a review note")
	}

	missingRetentionDoc := *doc
	missingRetentionDoc.ID = "doc-missing-retention"
	missingRetentionDoc.RetentionUntil = nil
	missingRetentionDoc.ReviewStatus = ReviewStatusPending
	repo.docs[missingRetentionDoc.ID] = &missingRetentionDoc

	expiredDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	repo.docs[doc.ID].RetentionUntil = &expiredDate
	retentionReview, err := svc.GetRetentionReview(context.Background(), "tenant_demo", "tenant-1", time.Date(2026, 3, 15, 8, 0, 0, 0, time.UTC), 30, true)
	if err != nil {
		t.Fatalf("GetRetentionReview failed: %v", err)
	}
	if retentionReview.TotalCount != 2 || retentionReview.ExpiredCount != 1 || retentionReview.MissingRetentionCount != 1 {
		t.Fatalf("unexpected retention review: %#v", retentionReview)
	}
	if retentionReview.PendingReviewCount != 1 || retentionReview.RejectedCount != 1 {
		t.Fatalf("unexpected retention review status counts: %#v", retentionReview)
	}

	correctedRetention := time.Date(2028, 3, 31, 15, 45, 0, 0, time.FixedZone("EET", 2*60*60))
	correctedDoc, err := svc.UpdateDocumentRetention(context.Background(), "tenant_demo", "tenant-1", doc.ID, &correctedRetention)
	if err != nil {
		t.Fatalf("UpdateDocumentRetention failed: %v", err)
	}
	if correctedDoc.RetentionUntil == nil || correctedDoc.RetentionUntil.Format("2006-01-02T15:04:05Z07:00") != "2028-03-31T00:00:00Z" {
		t.Fatalf("unexpected corrected retention: %#v", correctedDoc.RetentionUntil)
	}

	clearedDoc, err := svc.UpdateDocumentRetention(context.Background(), "tenant_demo", "tenant-1", doc.ID, nil)
	if err != nil {
		t.Fatalf("UpdateDocumentRetention clear failed: %v", err)
	}
	if clearedDoc.RetentionUntil != nil {
		t.Fatalf("expected cleared retention, got %#v", clearedDoc.RetentionUntil)
	}
	if _, err := svc.UpdateDocumentRetention(context.Background(), "tenant_demo", "tenant-1", "", nil); err == nil {
		t.Fatal("expected document ID to be required")
	}

	summaries, err := svc.ListReviewSummaries(context.Background(), "tenant_demo", "tenant-1", EntityTypeBankTxn, []string{"txn-1", "txn-2"})
	if err != nil {
		t.Fatalf("ListReviewSummaries failed: %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}
	if summaries[0].EntityID != "txn-1" || summaries[0].ReviewedCount != 1 || summaries[0].RejectedCount != 1 || !summaries[0].HasRejected {
		t.Fatalf("unexpected first summary: %#v", summaries[0])
	}
	if summaries[1].EntityID != "txn-2" || !summaries[1].MissingEvidence {
		t.Fatalf("unexpected missing-evidence summary: %#v", summaries[1])
	}

	repo.docs["doc-approved-receipt"] = &Document{
		ID:           "doc-approved-receipt",
		TenantID:     "tenant-1",
		EntityType:   EntityTypePayment,
		EntityID:     "pay-1",
		DocumentType: DocumentTypeReceipt,
		FileName:     "receipt.pdf",
		ReviewStatus: ReviewStatusApproved,
		UploadedBy:   "user-1",
		CreatedAt:    time.Now().UTC(),
	}
	repo.docs["doc-pending-receipt"] = &Document{
		ID:           "doc-pending-receipt",
		TenantID:     "tenant-1",
		EntityType:   EntityTypePayment,
		EntityID:     "pay-2",
		DocumentType: DocumentTypeReceipt,
		FileName:     "receipt-draft.pdf",
		ReviewStatus: ReviewStatusPending,
		UploadedBy:   "user-1",
		CreatedAt:    time.Now().UTC(),
	}
	policyResults, err := svc.EvaluateEvidencePolicy(context.Background(), "tenant_demo", "tenant-1", &EvidencePolicyRequest{
		EntityType: EntityTypePayment,
		EntityIDs:  []string{"pay-1", "pay-2", "pay-3", "pay-1"},
		Rules: []EvidencePolicyRule{{
			DocumentTypes:   []string{DocumentTypeReceipt},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		t.Fatalf("EvaluateEvidencePolicy failed: %v", err)
	}
	if len(policyResults) != 3 {
		t.Fatalf("expected deduped 3 policy results, got %d", len(policyResults))
	}
	if !policyResults[0].Compliant || policyResults[0].ApprovedDocumentTypeCounts[DocumentTypeReceipt] != 1 {
		t.Fatalf("expected pay-1 policy to pass with approved receipt: %#v", policyResults[0])
	}
	if policyResults[1].Compliant || len(policyResults[1].Violations) != 1 || policyResults[1].RuleResults[0].AcceptedCount != 0 {
		t.Fatalf("expected pay-2 policy to fail without an approved receipt: %#v", policyResults[1])
	}
	if policyResults[2].Compliant || !policyResults[2].MissingEvidence {
		t.Fatalf("expected pay-3 policy to fail as missing evidence: %#v", policyResults[2])
	}

	repo.docs["doc-close-pack"] = &Document{
		ID:           "doc-close-pack",
		TenantID:     "tenant-1",
		EntityType:   EntityTypeYearEndClose,
		EntityID:     "8a369f1a-f0c4-5a50-9b41-cb0fda4a09ee",
		DocumentType: DocumentTypeClosePack,
		FileName:     "close-pack.pdf",
		ReviewStatus: ReviewStatusApproved,
		UploadedBy:   "user-1",
		CreatedAt:    time.Now().UTC(),
	}
	closePackResults, err := svc.EvaluateEvidencePolicy(context.Background(), "tenant_demo", "tenant-1", &EvidencePolicyRequest{
		EntityType: EntityTypeYearEndClose,
		EntityIDs:  []string{"8a369f1a-f0c4-5a50-9b41-cb0fda4a09ee"},
		Rules: []EvidencePolicyRule{{
			DocumentTypes:   []string{DocumentTypeClosePack},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		t.Fatalf("EvaluateEvidencePolicy close pack failed: %v", err)
	}
	if len(closePackResults) != 1 || !closePackResults[0].Compliant || closePackResults[0].ApprovedDocumentTypeCounts[DocumentTypeClosePack] != 1 {
		t.Fatalf("expected close-pack policy to pass with approved close pack: %#v", closePackResults)
	}

	if err := svc.DeleteDocument(context.Background(), "tenant_demo", "tenant-1", doc.ID); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, doc.StorageKey)); !os.IsNotExist(err) {
		t.Fatalf("expected stored file to be deleted, got err=%v", err)
	}
}
