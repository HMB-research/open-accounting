package documents

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDocumentsWave7ListReviewSummariesDedupesAndDefaults(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, &mockStore{})
	repo.docs["doc-1"] = &Document{
		ID:           "doc-1",
		TenantID:     "tenant-1",
		EntityType:   EntityTypeInvoice,
		EntityID:     "invoice-1",
		DocumentType: DocumentTypeSupportingDocument,
		ReviewStatus: ReviewStatusApproved,
		UploadedBy:   "user-1",
		CreatedAt:    time.Now(),
	}
	repo.docs["doc-2"] = &Document{
		ID:           "doc-2",
		TenantID:     "tenant-1",
		EntityType:   EntityTypeInvoice,
		EntityID:     "invoice-1",
		DocumentType: DocumentTypeReceipt,
		ReviewStatus: ReviewStatusRejected,
		UploadedBy:   "user-1",
		CreatedAt:    time.Now(),
	}

	empty, err := service.ListReviewSummaries(context.Background(), "tenant_demo", "tenant-1", EntityTypeInvoice, []string{" ", "\t"})
	if err != nil {
		t.Fatalf("ListReviewSummaries(empty) error = %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ListReviewSummaries(empty) = %#v, want empty slice", empty)
	}

	summaries, err := service.ListReviewSummaries(context.Background(), "tenant_demo", "tenant-1", " invoice ", []string{" invoice-1 ", "invoice-1", "invoice-2"})
	if err != nil {
		t.Fatalf("ListReviewSummaries() error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("ListReviewSummaries() len = %d, want deduped two summaries: %#v", len(summaries), summaries)
	}
	if summaries[0].EntityID != "invoice-1" || summaries[0].ReviewedCount != 2 || !summaries[0].HasRejected {
		t.Fatalf("first summary = %#v, want reviewed rejected summary for invoice-1", summaries[0])
	}
	if summaries[1].EntityID != "invoice-2" || !summaries[1].MissingEvidence {
		t.Fatalf("second summary = %#v, want missing evidence default for invoice-2", summaries[1])
	}

	repo.reviewSummaryErr = errors.New("summary failed")
	_, err = service.ListReviewSummaries(context.Background(), "tenant_demo", "tenant-1", EntityTypeInvoice, []string{"invoice-1"})
	if err == nil || !strings.Contains(err.Error(), "summary failed") {
		t.Fatalf("ListReviewSummaries() repo error = %v", err)
	}
}

func TestDocumentsWave7ReviewQueueCountsAllStatuses(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo, &mockStore{})
	now := time.Now()
	for _, doc := range []Document{
		{ID: "pending", TenantID: "tenant-1", EntityType: EntityTypeExpense, EntityID: "expense-1", DocumentType: DocumentTypeReceipt, ReviewStatus: ReviewStatusPending, UploadedBy: "user-1", CreatedAt: now},
		{ID: "reviewed", TenantID: "tenant-1", EntityType: EntityTypeExpense, EntityID: "expense-2", DocumentType: DocumentTypeReceipt, ReviewStatus: ReviewStatusReviewed, UploadedBy: "user-1", CreatedAt: now},
		{ID: "approved", TenantID: "tenant-1", EntityType: EntityTypeExpense, EntityID: "expense-3", DocumentType: DocumentTypeReceipt, ReviewStatus: ReviewStatusApproved, UploadedBy: "user-1", CreatedAt: now},
		{ID: "rejected", TenantID: "tenant-1", EntityType: EntityTypeExpense, EntityID: "expense-4", DocumentType: DocumentTypeReceipt, ReviewStatus: ReviewStatusRejected, UploadedBy: "user-1", CreatedAt: now},
	} {
		copyDoc := doc
		repo.docs[doc.ID] = &copyDoc
	}

	queue, err := service.GetReviewQueue(context.Background(), "tenant_demo", "tenant-1", ReviewQueueFilter{ReviewStatus: "all"})
	if err != nil {
		t.Fatalf("GetReviewQueue() error = %v", err)
	}
	if queue.TotalCount != 4 || queue.PendingReviewCount != 1 || queue.ReviewedCount != 3 || queue.ApprovedCount != 1 || queue.RejectedCount != 1 {
		t.Fatalf("GetReviewQueue() counts = %#v", queue)
	}
}

func TestDocumentsWave7PurgeExpiredDocumentsLimitAndErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)

	t.Run("repository error", func(t *testing.T) {
		repo := newMockRepository()
		repo.retentionReviewErr = errors.New("retention failed")
		_, err := NewService(repo, &mockStore{}).PurgeExpiredDocuments(ctx, "tenant_demo", "tenant-1", DocumentPurgeRequest{AsOfDate: now})
		if err == nil || !strings.Contains(err.Error(), "retention failed") {
			t.Fatalf("PurgeExpiredDocuments() error = %v", err)
		}
	})

	t.Run("limit reached marks remaining eligible candidates", func(t *testing.T) {
		repo := newMockRepository()
		expired := now.AddDate(0, 0, -1)
		repo.docs["doc-1"] = &Document{ID: "doc-1", TenantID: "tenant-1", EntityType: EntityTypeExpense, EntityID: "expense-1", DocumentType: DocumentTypeReceipt, RetentionUntil: &expired, LifecycleStatus: LifecycleStatusDisposed}
		repo.docs["doc-2"] = &Document{ID: "doc-2", TenantID: "tenant-1", EntityType: EntityTypeExpense, EntityID: "expense-2", DocumentType: DocumentTypeReceipt, RetentionUntil: &expired, LifecycleStatus: LifecycleStatusDisposed}
		result, err := NewService(repo, &mockStore{}).PurgeExpiredDocuments(ctx, "tenant_demo", "tenant-1", DocumentPurgeRequest{AsOfDate: now, Limit: 1, DryRun: true})
		if err != nil {
			t.Fatalf("PurgeExpiredDocuments() error = %v", err)
		}
		if result.EligibleCount != 1 || result.SkippedCount != 1 {
			t.Fatalf("PurgeExpiredDocuments() result = %#v, want one eligible and one skipped", result)
		}
		foundLimitSkip := false
		for _, candidate := range result.Candidates {
			if candidate.SkipReason == "limit_reached" {
				foundLimitSkip = true
			}
		}
		if !foundLimitSkip {
			t.Fatalf("PurgeExpiredDocuments() candidates = %#v, want limit_reached skip", result.Candidates)
		}
	})

	t.Run("delete error returns partial result", func(t *testing.T) {
		repo := newMockRepository()
		expired := now.AddDate(0, 0, -1)
		repo.docs["doc-1"] = &Document{ID: "doc-1", TenantID: "tenant-1", EntityType: EntityTypeExpense, EntityID: "expense-1", DocumentType: DocumentTypeReceipt, RetentionUntil: &expired, LifecycleStatus: LifecycleStatusDisposed}
		repo.deleteErr = errors.New("delete failed")
		result, err := NewService(repo, &mockStore{}).PurgeExpiredDocuments(ctx, "tenant_demo", "tenant-1", DocumentPurgeRequest{AsOfDate: now})
		if err == nil || !strings.Contains(err.Error(), "purge document doc-1") {
			t.Fatalf("PurgeExpiredDocuments() delete error = %v", err)
		}
		if result == nil || result.EligibleCount != 1 {
			t.Fatalf("PurgeExpiredDocuments() partial result = %#v", result)
		}
	})
}

func TestDocumentsWave7RetentionAndReviewNoopBranches(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepository()
	service := NewService(repo, &mockStore{})
	repo.docs["doc-1"] = &Document{
		ID:              "doc-1",
		TenantID:        "tenant-1",
		EntityType:      EntityTypeExpense,
		EntityID:        "expense-1",
		DocumentType:    DocumentTypeReceipt,
		ReviewStatus:    ReviewStatusApproved,
		ReviewNote:      "looks good",
		LifecycleStatus: LifecycleStatusActive,
		UploadedBy:      "user-1",
		CreatedAt:       time.Now(),
	}

	repo.getErr = os.ErrNotExist
	_, err := service.UpdateDocumentRetention(ctx, "tenant_demo", "tenant-1", "doc-1", nil)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("UpdateDocumentRetention() error = %v, want os.ErrNotExist", err)
	}
	repo.getErr = nil

	reviewed, err := service.ReviewDocument(ctx, "tenant_demo", "tenant-1", "doc-1", "reviewer-1", &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusApproved,
		ReviewNote:   " looks good ",
	})
	if err != nil {
		t.Fatalf("ReviewDocument() error = %v", err)
	}
	if reviewed.ID != "doc-1" || repo.reviewCount != 0 {
		t.Fatalf("ReviewDocument() = %#v, reviewCount=%d, want no-op", reviewed, repo.reviewCount)
	}

	_, err = service.ReviewDocument(ctx, "tenant_demo", "tenant-1", "doc-1", "reviewer-1", &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusRejected,
		ReviewNote:   strings.Repeat("x", 2001),
	})
	if err == nil || !strings.Contains(err.Error(), "review note must be 2000") {
		t.Fatalf("ReviewDocument() long note error = %v", err)
	}
	_, err = service.ReviewDocument(ctx, "tenant_demo", "tenant-1", "doc-1", "reviewer-1", &ReviewDocumentRequest{
		ReviewStatus: ReviewStatusRejected,
	})
	if err == nil || !strings.Contains(err.Error(), "review note is required") {
		t.Fatalf("ReviewDocument() rejected without note error = %v", err)
	}
}

func TestDocumentsWave7PurgeLimitHelper(t *testing.T) {
	if _, err := normalizePurgeLimit(-1); err == nil || !strings.Contains(err.Error(), "limit must be zero or greater") {
		t.Fatalf("normalizePurgeLimit(-1) error = %v", err)
	}
	if got, err := normalizePurgeLimit(0); err != nil || got != defaultPurgeLimit {
		t.Fatalf("normalizePurgeLimit(0) = %d, %v", got, err)
	}
	if got, err := normalizePurgeLimit(maxPurgeLimit + 1); err == nil || got != 0 || !strings.Contains(err.Error(), "limit cannot exceed") {
		t.Fatalf("normalizePurgeLimit(max+1) = %d, %v, want limit error", got, err)
	}
}
