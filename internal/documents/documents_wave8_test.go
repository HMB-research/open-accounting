package documents

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDocumentsWave8ReviewDocumentRepositoryErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("get error", func(t *testing.T) {
		repo := newMockRepository()
		repo.getErr = errors.New("lookup failed")
		service := NewService(repo, &mockStore{})

		_, err := service.ReviewDocument(ctx, "tenant_demo", "tenant-1", "doc-1", "reviewer-1", &ReviewDocumentRequest{
			ReviewStatus: ReviewStatusApproved,
		})
		if err == nil || !strings.Contains(err.Error(), "lookup failed") {
			t.Fatalf("ReviewDocument() get error = %v, want lookup failure", err)
		}
	})

	t.Run("review update error", func(t *testing.T) {
		repo := newMockRepository()
		repo.docs["doc-1"] = &Document{
			ID:           "doc-1",
			TenantID:     "tenant-1",
			EntityType:   EntityTypeExpense,
			EntityID:     "expense-1",
			DocumentType: DocumentTypeReceipt,
			ReviewStatus: ReviewStatusPending,
			UploadedBy:   "user-1",
			CreatedAt:    time.Now(),
		}
		repo.reviewErr = errors.New("review failed")
		service := NewService(repo, &mockStore{})

		_, err := service.ReviewDocument(ctx, "tenant_demo", "tenant-1", "doc-1", "reviewer-1", &ReviewDocumentRequest{
			ReviewStatus: ReviewStatusApproved,
		})
		if err == nil || !strings.Contains(err.Error(), "review failed") {
			t.Fatalf("ReviewDocument() review error = %v, want update failure", err)
		}
	})
}

func TestDocumentsWave8ReviewQueueHelpers(t *testing.T) {
	for _, status := range []string{ReviewStatusReviewed, ReviewStatusApproved, ReviewStatusRejected} {
		got, err := normalizeReviewQueueStatus(strings.ToLower(status))
		if err != nil || got != status {
			t.Fatalf("normalizeReviewQueueStatus(%q) = %q, %v", status, got, err)
		}
	}

	got, err := normalizeReviewQueueLimit(7)
	if err != nil || got != 7 {
		t.Fatalf("normalizeReviewQueueLimit(7) = %d, %v", got, err)
	}
}

func TestDocumentsWave8EvidencePolicyCountsRejectedDocuments(t *testing.T) {
	result := evaluateEvidencePolicyForDocuments(EntityTypeExpense, "expense-1", []Document{{
		ID:              "doc-1",
		TenantID:        "tenant-1",
		EntityType:      EntityTypeExpense,
		EntityID:        "expense-1",
		DocumentType:    DocumentTypeReceipt,
		ReviewStatus:    ReviewStatusRejected,
		LifecycleStatus: LifecycleStatusActive,
	}}, []EvidencePolicyRule{{
		DocumentTypes:   []string{DocumentTypeReceipt},
		MinCount:        1,
		RequireApproved: true,
	}})

	if result.ReviewedCount != 1 || result.RejectedCount != 1 || result.ApprovedCount != 0 {
		t.Fatalf("evaluateEvidencePolicyForDocuments() counts = %#v", result)
	}
	if result.Compliant {
		t.Fatalf("evaluateEvidencePolicyForDocuments() compliant = true, want violation for rejected receipt")
	}
}
