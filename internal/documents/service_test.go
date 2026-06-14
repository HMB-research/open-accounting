package documents

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type mockRepository struct {
	entityExists          bool
	entityErr             error
	createErr             error
	listErr               error
	reviewQueueErr        error
	retentionReviewErr    error
	reviewSummaryErr      error
	getErr                error
	updateRetentionErr    error
	reviewErr             error
	deleteErr             error
	reviewCount           int
	lastReviewQueueFilter ReviewQueueFilter
	docs                  map[string]*Document
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		entityExists: true,
		docs:         make(map[string]*Document),
	}
}

func (m *mockRepository) EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error) {
	if m.entityErr != nil {
		return false, m.entityErr
	}
	return m.entityExists, nil
}

func (m *mockRepository) CreateDocument(ctx context.Context, schemaName string, doc *Document) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.docs[doc.ID] = doc
	return nil
}

func (m *mockRepository) ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]Document, 0, len(m.docs))
	for _, doc := range m.docs {
		if doc.TenantID == tenantID && doc.EntityType == entityType && doc.EntityID == entityID {
			result = append(result, *doc)
		}
	}
	return result, nil
}

func (m *mockRepository) ListReviewQueueDocuments(ctx context.Context, schemaName, tenantID string, filter ReviewQueueFilter) ([]Document, error) {
	if m.reviewQueueErr != nil {
		return nil, m.reviewQueueErr
	}
	m.lastReviewQueueFilter = filter
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
	if m.retentionReviewErr != nil {
		return nil, m.retentionReviewErr
	}
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
	if m.reviewSummaryErr != nil {
		return nil, m.reviewSummaryErr
	}
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
	if m.getErr != nil {
		return nil, m.getErr
	}
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return nil, os.ErrNotExist
	}
	return doc, nil
}

func (m *mockRepository) UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) error {
	if m.updateRetentionErr != nil {
		return m.updateRetentionErr
	}
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return os.ErrNotExist
	}
	doc.RetentionUntil = retentionUntil
	return nil
}

func (m *mockRepository) ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewStatus, reviewNote, reviewedBy string, reviewedAt time.Time) error {
	if m.reviewErr != nil {
		return m.reviewErr
	}
	doc, ok := m.docs[documentID]
	if !ok || doc.TenantID != tenantID {
		return os.ErrNotExist
	}
	m.reviewCount++
	doc.ReviewStatus = reviewStatus
	doc.ReviewNote = reviewNote
	doc.ReviewedBy = &reviewedBy
	doc.ReviewedAt = &reviewedAt
	return nil
}

func (m *mockRepository) DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.docs, documentID)
	return nil
}

type mockStore struct {
	saveErr   error
	openErr   error
	deleteErr error
	savedKey  string
	deleted   []string
	content   []byte
}

func (m *mockStore) Save(_ context.Context, key string, content io.Reader) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	payload, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	m.savedKey = key
	m.content = payload
	return nil
}

func (m *mockStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	if m.openErr != nil {
		return nil, m.openErr
	}
	if key != m.savedKey {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(m.content)), nil
}

func (m *mockStore) Delete(_ context.Context, key string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleted = append(m.deleted, key)
	return nil
}

func TestService_UploadDocumentValidationAndCleanup(t *testing.T) {
	t.Parallel()

	validRequest := func(mutators ...func(*UploadDocumentRequest)) *UploadDocumentRequest {
		req := &UploadDocumentRequest{
			EntityType:   EntityTypePayment,
			EntityID:     "payment-1",
			DocumentType: DocumentTypeReceipt,
			FileName:     " receipt 001.txt ",
			FileSize:     int64(len("receipt")),
			UploadedBy:   " user-1 ",
		}
		for _, mutate := range mutators {
			mutate(req)
		}
		return req
	}

	t.Run("normalizes defaults", func(t *testing.T) {
		repo := newMockRepository()
		store := &mockStore{}
		svc := NewService(repo, store)

		doc, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", validRequest(func(req *UploadDocumentRequest) {
			req.DocumentType = ""
			req.ContentType = ""
			req.Notes = "  Receipt attached  "
		}), bytes.NewBufferString("receipt"))

		if err != nil {
			t.Fatalf("UploadDocument failed: %v", err)
		}
		if doc.DocumentType != DocumentTypeSupportingDocument {
			t.Fatalf("expected default document type %q, got %q", DocumentTypeSupportingDocument, doc.DocumentType)
		}
		if doc.ContentType != "text/plain; charset=utf-8" {
			t.Fatalf("expected inferred content type, got %q", doc.ContentType)
		}
		if doc.FileName != "receipt_001.txt" {
			t.Fatalf("expected sanitized file name, got %q", doc.FileName)
		}
		if doc.Notes != "Receipt attached" {
			t.Fatalf("expected trimmed notes, got %q", doc.Notes)
		}
		if !strings.Contains(store.savedKey, "receipt_001.txt") {
			t.Fatalf("expected storage key to include sanitized file name, got %q", store.savedKey)
		}
	})

	tests := []struct {
		name    string
		req     *UploadDocumentRequest
		content io.Reader
		want    string
	}{
		{name: "unsupported entity type", req: validRequest(func(req *UploadDocumentRequest) { req.EntityType = "unknown" }), content: bytes.NewBufferString("receipt"), want: "unsupported document entity type"},
		{name: "unsupported document type", req: validRequest(func(req *UploadDocumentRequest) { req.DocumentType = "unknown" }), content: bytes.NewBufferString("receipt"), want: "unsupported document type"},
		{name: "blank entity id", req: validRequest(func(req *UploadDocumentRequest) { req.EntityID = " " }), content: bytes.NewBufferString("receipt"), want: "entity ID is required"},
		{name: "blank uploaded by", req: validRequest(func(req *UploadDocumentRequest) { req.UploadedBy = " " }), content: bytes.NewBufferString("receipt"), want: "uploaded by user is required"},
		{name: "blank file name", req: validRequest(func(req *UploadDocumentRequest) { req.FileName = "!!!" }), content: bytes.NewBufferString("receipt"), want: "file name is required"},
		{name: "empty file", req: validRequest(func(req *UploadDocumentRequest) { req.FileSize = 0 }), content: bytes.NewBufferString(""), want: "document file is empty"},
		{name: "oversized file", req: validRequest(func(req *UploadDocumentRequest) { req.FileSize = MaxDocumentSizeBytes + 1 }), content: bytes.NewBufferString("receipt"), want: "document exceeds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(newMockRepository(), &mockStore{})

			_, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", tt.req, tt.content)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	t.Run("entity lookup error", func(t *testing.T) {
		repo := newMockRepository()
		repo.entityErr = errors.New("lookup failed")
		svc := NewService(repo, &mockStore{})

		_, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", validRequest(), bytes.NewBufferString("receipt"))

		if err == nil || !strings.Contains(err.Error(), "lookup failed") {
			t.Fatalf("expected lookup error, got %v", err)
		}
	})

	t.Run("target not found", func(t *testing.T) {
		repo := newMockRepository()
		repo.entityExists = false
		svc := NewService(repo, &mockStore{})

		_, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", validRequest(), bytes.NewBufferString("receipt"))

		if err == nil || !strings.Contains(err.Error(), "target record not found") {
			t.Fatalf("expected target record error, got %v", err)
		}
	})

	t.Run("store save error", func(t *testing.T) {
		svc := NewService(newMockRepository(), &mockStore{saveErr: errors.New("save failed")})

		_, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", validRequest(), bytes.NewBufferString("receipt"))

		if err == nil || !strings.Contains(err.Error(), "save failed") {
			t.Fatalf("expected save error, got %v", err)
		}
	})

	t.Run("repository create error deletes stored content", func(t *testing.T) {
		repo := newMockRepository()
		repo.createErr = errors.New("create failed")
		store := &mockStore{}
		svc := NewService(repo, store)

		_, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", validRequest(), bytes.NewBufferString("receipt"))

		if err == nil || !strings.Contains(err.Error(), "create failed") {
			t.Fatalf("expected create error, got %v", err)
		}
		if len(store.deleted) != 1 || store.deleted[0] != store.savedKey {
			t.Fatalf("expected saved content cleanup, saved=%q deleted=%#v", store.savedKey, store.deleted)
		}
	})
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

func TestService_GetReviewQueueValidationAndLimit(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	svc := NewService(repo, &mockStore{})

	tests := []struct {
		name   string
		filter ReviewQueueFilter
		want   string
	}{
		{name: "invalid entity type", filter: ReviewQueueFilter{EntityType: "unsupported"}, want: "unsupported document entity type"},
		{name: "invalid document type", filter: ReviewQueueFilter{DocumentType: "unsupported"}, want: "unsupported document type"},
		{name: "invalid review status", filter: ReviewQueueFilter{ReviewStatus: "unknown"}, want: "review_status must be PENDING"},
		{name: "negative limit", filter: ReviewQueueFilter{Limit: -1}, want: "limit must be zero or greater"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GetReviewQueue(context.Background(), "tenant_demo", "tenant-1", tt.filter)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	queue, err := svc.GetReviewQueue(context.Background(), "tenant_demo", "tenant-1", ReviewQueueFilter{
		ReviewStatus: "approved",
		Limit:        maxReviewQueueLimit + 1,
	})
	if err != nil {
		t.Fatalf("GetReviewQueue failed: %v", err)
	}
	if queue.Limit != maxReviewQueueLimit {
		t.Fatalf("expected capped limit %d, got %d", maxReviewQueueLimit, queue.Limit)
	}
	if repo.lastReviewQueueFilter.ReviewStatus != ReviewStatusApproved {
		t.Fatalf("expected normalized approved status, got %#v", repo.lastReviewQueueFilter)
	}

	repo.reviewQueueErr = errors.New("queue failed")
	_, err = svc.GetReviewQueue(context.Background(), "tenant_demo", "tenant-1", ReviewQueueFilter{})
	if err == nil || !strings.Contains(err.Error(), "queue failed") {
		t.Fatalf("expected queue error, got %v", err)
	}
}

func TestService_UploadAcceptsWorkflowDocuments(t *testing.T) {
	t.Parallel()

	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	repo := newMockRepository()
	svc := NewService(repo, store)

	tests := []struct {
		name       string
		entityType string
		entityID   string
	}{
		{name: "quote", entityType: EntityTypeQuote, entityID: "quote-1"},
		{name: "order", entityType: EntityTypeOrder, entityID: "order-1"},
		{name: "leave", entityType: EntityTypeLeaveRecord, entityID: "leave-1"},
		{name: "TSD", entityType: EntityTypeTSD, entityID: "tsd-1"},
		{name: "KMD", entityType: EntityTypeKMD, entityID: "kmd-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := svc.UploadDocument(context.Background(), "tenant_demo", "tenant-1", &UploadDocumentRequest{
				EntityType:   tt.entityType,
				EntityID:     tt.entityID,
				DocumentType: DocumentTypeContract,
				FileName:     tt.name + "-contract.pdf",
				ContentType:  "application/pdf",
				FileSize:     int64(len("contract")),
				UploadedBy:   "user-1",
			}, bytes.NewBufferString("contract"))
			if err != nil {
				t.Fatalf("UploadDocument failed: %v", err)
			}
			if doc.EntityType != tt.entityType || doc.EntityID != tt.entityID || doc.DocumentType != DocumentTypeContract {
				t.Fatalf("unexpected workflow document: %#v", doc)
			}
		})
	}
}

func TestService_ReviewDocumentValidationAndIdempotency(t *testing.T) {
	t.Parallel()

	repo := newMockRepository()
	svc := NewService(repo, &mockStore{})
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	repo.docs["doc-1"] = &Document{
		ID:           "doc-1",
		TenantID:     "tenant-1",
		EntityType:   EntityTypePayment,
		EntityID:     "pay-1",
		DocumentType: DocumentTypeReceipt,
		FileName:     "receipt.pdf",
		ReviewStatus: ReviewStatusApproved,
		ReviewNote:   "looks good",
		UploadedBy:   "user-1",
		CreatedAt:    now,
	}

	tests := []struct {
		name       string
		reviewedBy string
		req        *ReviewDocumentRequest
		want       string
	}{
		{name: "blank reviewer", reviewedBy: " ", req: &ReviewDocumentRequest{ReviewStatus: ReviewStatusReviewed}, want: "reviewed by user is required"},
		{name: "nil request", reviewedBy: "reviewer-1", req: nil, want: "review request is required"},
		{name: "invalid status", reviewedBy: "reviewer-1", req: &ReviewDocumentRequest{ReviewStatus: "PENDING"}, want: "review_status must be REVIEWED"},
		{name: "long note", reviewedBy: "reviewer-1", req: &ReviewDocumentRequest{ReviewStatus: ReviewStatusReviewed, ReviewNote: strings.Repeat("x", 2001)}, want: "review note must be 2000 characters or less"},
		{name: "rejection requires note", reviewedBy: "reviewer-1", req: &ReviewDocumentRequest{ReviewStatus: ReviewStatusRejected}, want: "review note is required when rejecting a document"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ReviewDocument(context.Background(), "tenant_demo", "tenant-1", "doc-1", tt.reviewedBy, tt.req)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	doc, err := svc.ReviewDocument(context.Background(), "tenant_demo", "tenant-1", "doc-1", "reviewer-1", &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusApproved,
		ReviewNote:   " looks good ",
	})
	if err != nil {
		t.Fatalf("ReviewDocument idempotent call failed: %v", err)
	}
	if doc.ID != "doc-1" || repo.reviewCount != 0 {
		t.Fatalf("expected idempotent review without repository update, doc=%#v reviewCount=%d", doc, repo.reviewCount)
	}

	reviewed, err := svc.ReviewDocument(context.Background(), "tenant_demo", "tenant-1", "doc-1", " reviewer-2 ", &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusRejected,
		ReviewNote:   "Does not match payment",
	})
	if err != nil {
		t.Fatalf("ReviewDocument rejected failed: %v", err)
	}
	if reviewed.ReviewStatus != ReviewStatusRejected || reviewed.ReviewedBy == nil || *reviewed.ReviewedBy != "reviewer-2" {
		t.Fatalf("unexpected rejected document: %#v", reviewed)
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
	dueSoonDate := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	dueSoonDoc := *doc
	dueSoonDoc.ID = "doc-due-soon"
	dueSoonDoc.EntityID = "txn-due-soon"
	dueSoonDoc.RetentionUntil = &dueSoonDate
	dueSoonDoc.ReviewStatus = ReviewStatusApproved
	repo.docs[dueSoonDoc.ID] = &dueSoonDoc
	repo.docs[doc.ID].RetentionUntil = &expiredDate
	retentionReview, err := svc.GetRetentionReview(context.Background(), "tenant_demo", "tenant-1", time.Date(2026, 3, 15, 8, 0, 0, 0, time.UTC), 30, true)
	if err != nil {
		t.Fatalf("GetRetentionReview failed: %v", err)
	}
	if retentionReview.TotalCount != 3 || retentionReview.ExpiredCount != 1 || retentionReview.DueSoonCount != 1 || retentionReview.MissingRetentionCount != 1 {
		t.Fatalf("unexpected retention review: %#v", retentionReview)
	}
	if retentionReview.PendingReviewCount != 1 || retentionReview.RejectedCount != 1 {
		t.Fatalf("unexpected retention review status counts: %#v", retentionReview)
	}
	if len(retentionReview.ReminderActions) != 5 {
		t.Fatalf("expected 5 retention reminder actions, got %#v", retentionReview.ReminderActions)
	}
	if len(retentionReview.RemediationActions) != 5 {
		t.Fatalf("expected 5 retention remediation actions, got %#v", retentionReview.RemediationActions)
	}
	actionCounts := map[string]int{}
	for _, action := range retentionReview.ReminderActions {
		actionCounts[action.Action]++
		if action.DocumentID == "doc-due-soon" && action.Action == RetentionReminderDueSoon {
			if action.DaysUntilRetention == nil || *action.DaysUntilRetention != 15 {
				t.Fatalf("expected due-soon reminder to be 15 days out, got %#v", action.DaysUntilRetention)
			}
			if action.RetentionUntil == nil || action.RetentionUntil.Format("2006-01-02") != "2026-03-30" {
				t.Fatalf("unexpected reminder retention date: %#v", action.RetentionUntil)
			}
		}
	}
	for _, action := range []string{
		RetentionReminderExpired,
		RetentionReminderDueSoon,
		RetentionReminderMissingRetention,
		RetentionReminderPendingReview,
		RetentionReminderRejected,
	} {
		if actionCounts[action] == 0 {
			t.Fatalf("missing retention reminder action %q in %#v", action, retentionReview.ReminderActions)
		}
	}
	remediationCodes := documentRemediationCodes(retentionReview.RemediationActions)
	for _, code := range []string{
		"document_retention_expired",
		"document_retention_due_soon",
		"document_retention_missing",
		"document_review_pending",
		"document_review_rejected",
	} {
		if remediationCodes[code] == 0 {
			t.Fatalf("missing retention remediation code %q in %#v", code, retentionReview.RemediationActions)
		}
	}
	if retentionReview.RemediationActions[0].Scope != "documents" || retentionReview.RemediationActions[0].OwnerRole != "accountant" {
		t.Fatalf("unexpected retention remediation ownership: %#v", retentionReview.RemediationActions[0])
	}
	if retentionReview.RemediationActions[0].WorkspaceQueue != "document_review" || retentionReview.RemediationActions[0].AssignmentKey == "" || retentionReview.RemediationActions[0].Priority == "" {
		t.Fatalf("expected retention remediation assignment metadata: %#v", retentionReview.RemediationActions[0])
	}
	if _, err := svc.GetRetentionReview(context.Background(), "tenant_demo", "tenant-1", time.Now(), -1, false); err == nil {
		t.Fatal("expected negative retention horizon to fail")
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
	if len(policyResults[0].RemediationActions) != 0 {
		t.Fatalf("expected compliant policy to have no remediation actions: %#v", policyResults[0].RemediationActions)
	}
	if policyResults[1].Compliant || len(policyResults[1].Violations) != 1 || policyResults[1].RuleResults[0].AcceptedCount != 0 {
		t.Fatalf("expected pay-2 policy to fail without an approved receipt: %#v", policyResults[1])
	}
	if len(policyResults[1].RemediationActions) != 1 || policyResults[1].RemediationActions[0].Code != "document_evidence_unapproved" {
		t.Fatalf("expected pay-2 unapproved-evidence remediation action: %#v", policyResults[1].RemediationActions)
	}
	if policyResults[1].RemediationActions[0].WorkspaceQueue != "document_review" || policyResults[1].RemediationActions[0].Priority != "high" || policyResults[1].RemediationActions[0].DueInDays != 1 {
		t.Fatalf("expected evidence policy assignment metadata: %#v", policyResults[1].RemediationActions[0])
	}
	if policyResults[1].RemediationActions[0].DocumentID != "doc-pending-receipt" || policyResults[1].RemediationActions[0].FileName != "receipt-draft.pdf" {
		t.Fatalf("expected unapproved evidence action to target pending document: %#v", policyResults[1].RemediationActions[0])
	}
	if policyResults[1].RemediationActions[0].CLICommand != "oa documents review --id doc-pending-receipt --status approved" {
		t.Fatalf("expected direct review command, got %q", policyResults[1].RemediationActions[0].CLICommand)
	}
	if policyResults[2].Compliant || !policyResults[2].MissingEvidence {
		t.Fatalf("expected pay-3 policy to fail as missing evidence: %#v", policyResults[2])
	}
	if len(policyResults[2].RemediationActions) != 1 || policyResults[2].RemediationActions[0].Code != "document_evidence_missing" {
		t.Fatalf("expected pay-3 missing-evidence remediation action: %#v", policyResults[2].RemediationActions)
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
	if len(closePackResults[0].RemediationActions) != 0 {
		t.Fatalf("expected compliant close-pack policy to have no remediation actions: %#v", closePackResults[0].RemediationActions)
	}

	if err := svc.DeleteDocument(context.Background(), "tenant_demo", "tenant-1", doc.ID); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rootDir, doc.StorageKey)); !os.IsNotExist(err) {
		t.Fatalf("expected stored file to be deleted, got err=%v", err)
	}
}

func TestNormalizeUploadRetentionValidation(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 3, 15, 8, 30, 0, 0, time.FixedZone("EET", 2*60*60))
	explicitRetention := time.Date(2029, 3, 31, 15, 45, 0, 0, time.FixedZone("EET", 2*60*60))

	tests := []struct {
		name           string
		retentionUntil *time.Time
		retentionYears int
		wantDate       string
		wantErr        string
	}{
		{name: "no retention", wantDate: ""},
		{name: "negative years", retentionYears: -1, wantErr: "retention years must be zero or greater"},
		{name: "too many years", retentionYears: MaxRetentionYears + 1, wantErr: "retention years cannot exceed"},
		{name: "explicit date", retentionUntil: &explicitRetention, wantDate: "2029-03-31"},
		{name: "years from upload date", retentionYears: 7, wantDate: "2033-03-15"},
		{name: "conflicting retention inputs", retentionUntil: &explicitRetention, retentionYears: 7, wantErr: "retention_until and retention_years cannot be combined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retentionUntil, err := normalizeUploadRetention(tt.retentionUntil, tt.retentionYears, createdAt)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("normalizeUploadRetention failed: %v", err)
			}
			if tt.wantDate == "" {
				if retentionUntil != nil {
					t.Fatalf("expected nil retention date, got %#v", retentionUntil)
				}
				return
			}
			if retentionUntil == nil || retentionUntil.Format("2006-01-02") != tt.wantDate {
				t.Fatalf("expected retention date %s, got %#v", tt.wantDate, retentionUntil)
			}
		})
	}
}

func TestService_EvaluateEvidencePolicyValidation(t *testing.T) {
	t.Parallel()

	svc := NewService(newMockRepository(), &mockStore{})

	tests := []struct {
		name string
		req  *EvidencePolicyRequest
		want string
	}{
		{name: "nil request", req: nil, want: "evidence policy request is required"},
		{name: "invalid entity type", req: &EvidencePolicyRequest{EntityType: "bad", EntityIDs: []string{"id-1"}, Rules: []EvidencePolicyRule{{MinCount: 1}}}, want: "unsupported document entity type"},
		{name: "empty entity IDs", req: &EvidencePolicyRequest{EntityType: EntityTypePayment, EntityIDs: []string{" ", ""}, Rules: []EvidencePolicyRule{{MinCount: 1}}}, want: "at least one entity ID is required"},
		{name: "missing rules", req: &EvidencePolicyRequest{EntityType: EntityTypePayment, EntityIDs: []string{"id-1"}}, want: "at least one evidence policy rule is required"},
		{name: "negative min count", req: &EvidencePolicyRequest{EntityType: EntityTypePayment, EntityIDs: []string{"id-1"}, Rules: []EvidencePolicyRule{{MinCount: -1}}}, want: "min_count must be one or greater"},
		{name: "invalid rule document type", req: &EvidencePolicyRequest{EntityType: EntityTypePayment, EntityIDs: []string{"id-1"}, Rules: []EvidencePolicyRule{{DocumentTypes: []string{"bad"}, MinCount: 1}}}, want: "unsupported document type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.EvaluateEvidencePolicy(context.Background(), "tenant_demo", "tenant-1", tt.req)

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	repo := newMockRepository()
	repo.listErr = errors.New("list failed")
	svc = NewService(repo, &mockStore{})
	_, err := svc.EvaluateEvidencePolicy(context.Background(), "tenant_demo", "tenant-1", &EvidencePolicyRequest{
		EntityType: EntityTypePayment,
		EntityIDs:  []string{"pay-1"},
		Rules:      []EvidencePolicyRule{{MinCount: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "list failed") {
		t.Fatalf("expected list error, got %v", err)
	}
}

func documentRemediationCodes(actions []DocumentRemediationAction) map[string]int {
	codes := make(map[string]int, len(actions))
	for _, action := range actions {
		codes[action.Code]++
	}
	return codes
}
