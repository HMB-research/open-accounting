//go:build integration

package documents

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
)

func TestDocumentEntityConstraintAllowsWorkflowRecords(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "workflow-documents@example.com")

	table, err := database.QualifiedTable(tenant.SchemaName, "documents")
	if err != nil {
		t.Fatalf("QualifiedTable failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, entityType := range []string{EntityTypeExpense, EntityTypeQuote, EntityTypeOrder, EntityTypeLeaveRecord} {
		// #nosec G201 -- table is schema-qualified by database.QualifiedTable in test setup.
		_, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (
				id, tenant_id, entity_type, entity_id, document_type, file_name,
				content_type, file_size, storage_key, uploaded_by
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, table),
			uuid.New().String(),
			tenant.ID,
			entityType,
			uuid.New().String(),
			DocumentTypeContract,
			entityType+"-document.pdf",
			"application/pdf",
			int64(12),
			"test/"+entityType+"/"+uuid.New().String(),
			userID,
		)
		if err != nil {
			t.Fatalf("expected %s documents to satisfy entity type check: %v", entityType, err)
		}
	}
}

func TestRepositoryLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "documents-repository@example.com")
	repo := NewRepository(pool)
	ctx := context.Background()

	entityID := uuid.NewString()
	exists, err := repo.EntityExists(ctx, tenant.SchemaName, tenant.ID, EntityTypeYearEndClose, entityID)
	if err != nil {
		t.Fatalf("EntityExists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected year-end close entity IDs to be accepted")
	}

	exists, err = repo.EntityExists(ctx, tenant.SchemaName, tenant.ID, EntityTypeYearEndClose, "not-a-uuid")
	if err != nil {
		t.Fatalf("EntityExists with invalid UUID failed: %v", err)
	}
	if exists {
		t.Fatal("expected invalid year-end close entity ID to be rejected")
	}

	baseTime := time.Now().UTC().Truncate(time.Microsecond)
	expiredRetention := baseTime.AddDate(0, 0, -1)

	approvedDoc := repositoryTestDocument(tenant.ID, entityID, userID, "close-pack.pdf", ReviewStatusPending, baseTime, &expiredRetention)
	pendingDoc := repositoryTestDocument(tenant.ID, entityID, userID, "receipt.pdf", ReviewStatusPending, baseTime.Add(time.Minute), nil)
	rejectedDoc := repositoryTestDocument(tenant.ID, entityID, userID, "contract.pdf", ReviewStatusRejected, baseTime.Add(2*time.Minute), &expiredRetention)
	rejectedDoc.ReviewNote = "missing signature"
	rejectedDoc.ReviewedBy = &userID
	rejectedAt := baseTime.Add(3 * time.Minute)
	rejectedDoc.ReviewedAt = &rejectedAt

	for _, doc := range []*Document{approvedDoc, pendingDoc, rejectedDoc} {
		if err := repo.CreateDocument(ctx, tenant.SchemaName, doc); err != nil {
			t.Fatalf("CreateDocument(%s) failed: %v", doc.FileName, err)
		}
	}

	documents, err := repo.ListDocuments(ctx, tenant.SchemaName, tenant.ID, EntityTypeYearEndClose, entityID)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(documents) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(documents))
	}
	if documents[0].ID != rejectedDoc.ID {
		t.Fatalf("expected newest document first, got %#v", documents)
	}

	reviewedAt := baseTime.Add(4 * time.Minute)
	if err := repo.ReviewDocument(ctx, tenant.SchemaName, tenant.ID, approvedDoc.ID, ReviewStatusApproved, "ready", userID, reviewedAt); err != nil {
		t.Fatalf("ReviewDocument failed: %v", err)
	}

	approved, err := repo.GetDocumentByID(ctx, tenant.SchemaName, tenant.ID, approvedDoc.ID)
	if err != nil {
		t.Fatalf("GetDocumentByID after review failed: %v", err)
	}
	if approved.ReviewStatus != ReviewStatusApproved || approved.ReviewNote != "ready" {
		t.Fatalf("expected approved review metadata, got %#v", approved)
	}
	if approved.ReviewedBy == nil || *approved.ReviewedBy != userID || approved.ReviewedAt == nil {
		t.Fatalf("expected reviewer metadata to be persisted, got %#v", approved)
	}

	updatedRetention := baseTime.AddDate(1, 0, 0)
	if err := repo.UpdateDocumentRetention(ctx, tenant.SchemaName, tenant.ID, approvedDoc.ID, &updatedRetention); err != nil {
		t.Fatalf("UpdateDocumentRetention failed: %v", err)
	}
	approved, err = repo.GetDocumentByID(ctx, tenant.SchemaName, tenant.ID, approvedDoc.ID)
	if err != nil {
		t.Fatalf("GetDocumentByID after retention update failed: %v", err)
	}
	if approved.RetentionUntil == nil || !approved.RetentionUntil.Equal(updatedRetention) {
		t.Fatalf("expected updated retention %s, got %#v", updatedRetention, approved.RetentionUntil)
	}

	queue, err := repo.ListReviewQueueDocuments(ctx, tenant.SchemaName, tenant.ID, ReviewQueueFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListReviewQueueDocuments failed: %v", err)
	}
	if len(queue) != 3 {
		t.Fatalf("expected 3 review queue documents, got %d", len(queue))
	}
	if queue[0].ID != pendingDoc.ID || queue[1].ID != rejectedDoc.ID || queue[2].ID != approvedDoc.ID {
		t.Fatalf("expected queue ordering by review priority, got %#v", queue)
	}

	pendingQueue, err := repo.ListReviewQueueDocuments(ctx, tenant.SchemaName, tenant.ID, ReviewQueueFilter{ReviewStatus: ReviewStatusPending, Limit: 10})
	if err != nil {
		t.Fatalf("ListReviewQueueDocuments with status filter failed: %v", err)
	}
	if len(pendingQueue) != 1 || pendingQueue[0].ID != pendingDoc.ID {
		t.Fatalf("expected pending queue to contain the pending document, got %#v", pendingQueue)
	}

	summaries, err := repo.ListReviewSummaries(ctx, tenant.SchemaName, tenant.ID, EntityTypeYearEndClose, []string{entityID, uuid.NewString()})
	if err != nil {
		t.Fatalf("ListReviewSummaries failed: %v", err)
	}
	summary, ok := summaries[entityID]
	if !ok {
		t.Fatalf("expected summary for entity %s, got %#v", entityID, summaries)
	}
	if summary.TotalCount != 3 || summary.PendingReviewCount != 1 || summary.ReviewedCount != 2 || summary.ApprovedCount != 1 || summary.RejectedCount != 1 {
		t.Fatalf("unexpected summary counts: %#v", summary)
	}
	if summary.MissingEvidence || !summary.HasPendingReview || !summary.HasRejected {
		t.Fatalf("unexpected summary flags: %#v", summary)
	}

	retentionDocs, err := repo.ListRetentionReviewDocuments(ctx, tenant.SchemaName, tenant.ID, baseTime, true)
	if err != nil {
		t.Fatalf("ListRetentionReviewDocuments with missing retention failed: %v", err)
	}
	if len(retentionDocs) != 2 || retentionDocs[0].ID != pendingDoc.ID || retentionDocs[1].ID != rejectedDoc.ID {
		t.Fatalf("expected missing-retention and expired documents, got %#v", retentionDocs)
	}

	expiredDocs, err := repo.ListRetentionReviewDocuments(ctx, tenant.SchemaName, tenant.ID, baseTime, false)
	if err != nil {
		t.Fatalf("ListRetentionReviewDocuments without missing retention failed: %v", err)
	}
	if len(expiredDocs) != 1 || expiredDocs[0].ID != rejectedDoc.ID {
		t.Fatalf("expected only expired retention document, got %#v", expiredDocs)
	}

	replacementDoc := repositoryTestDocument(tenant.ID, entityID, userID, "close-pack-v2.pdf", ReviewStatusApproved, baseTime.Add(5*time.Minute), nil)
	if err := repo.CreateDocument(ctx, tenant.SchemaName, replacementDoc); err != nil {
		t.Fatalf("CreateDocument replacement failed: %v", err)
	}
	if err := repo.UpdateDocumentLifecycle(ctx, tenant.SchemaName, tenant.ID, approvedDoc.ID, LifecycleStatusSuperseded, "Corrected close pack", userID, baseTime.Add(6*time.Minute), &replacementDoc.ID); err != nil {
		t.Fatalf("UpdateDocumentLifecycle superseded failed: %v", err)
	}
	hasDependents, err := repo.DocumentHasSupersededDependents(ctx, tenant.SchemaName, tenant.ID, replacementDoc.ID)
	if err != nil {
		t.Fatalf("DocumentHasSupersededDependents failed: %v", err)
	}
	if !hasDependents {
		t.Fatalf("expected replacement document %s to have superseded dependents", replacementDoc.ID)
	}
	if err := repo.DeleteDocument(ctx, tenant.SchemaName, tenant.ID, replacementDoc.ID); err == nil {
		t.Fatalf("expected database FK to block deleting replacement evidence")
	}
	missingReplacementID := uuid.NewString()
	if err := repo.UpdateDocumentLifecycle(ctx, tenant.SchemaName, tenant.ID, rejectedDoc.ID, LifecycleStatusSuperseded, "Missing replacement should fail", userID, baseTime.Add(7*time.Minute), &missingReplacementID); err == nil {
		t.Fatalf("expected database FK to block missing replacement evidence link")
	}
	if err := repo.UpdateDocumentLifecycle(ctx, tenant.SchemaName, tenant.ID, rejectedDoc.ID, LifecycleStatusArchived, "Unknown actor should fail", uuid.NewString(), baseTime.Add(8*time.Minute), nil); err == nil {
		t.Fatalf("expected database FK to block unknown lifecycle actor")
	}
	if err := repo.UpdateDocumentLegalHold(ctx, tenant.SchemaName, tenant.ID, rejectedDoc.ID, true, "Unknown actor should fail", uuid.NewString(), baseTime.Add(9*time.Minute)); err == nil {
		t.Fatalf("expected database FK to block unknown legal-hold actor")
	}

	if err := repo.DeleteDocument(ctx, tenant.SchemaName, tenant.ID, pendingDoc.ID); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}
	if _, err := repo.GetDocumentByID(ctx, tenant.SchemaName, tenant.ID, pendingDoc.ID); err == nil {
		t.Fatal("expected deleted document lookup to fail")
	}
}

func repositoryTestDocument(tenantID, entityID, uploadedBy, fileName, reviewStatus string, createdAt time.Time, retentionUntil *time.Time) *Document {
	documentID := uuid.NewString()
	return &Document{
		ID:             documentID,
		TenantID:       tenantID,
		EntityType:     EntityTypeYearEndClose,
		EntityID:       entityID,
		DocumentType:   DocumentTypeClosePack,
		FileName:       fileName,
		ContentType:    "application/pdf",
		FileSize:       512,
		StorageKey:     fmt.Sprintf("test/documents/%s/%s", entityID, documentID),
		RetentionUntil: retentionUntil,
		ReviewStatus:   reviewStatus,
		UploadedBy:     uploadedBy,
		CreatedAt:      createdAt,
	}
}
