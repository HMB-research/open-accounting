package documents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
)

func TestEntityTableName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		entityType string
		wantTable  string
		wantErr    bool
	}{
		{name: "invoice", entityType: EntityTypeInvoice, wantTable: "invoices"},
		{name: "journal entry", entityType: EntityTypeJournalEntry, wantTable: "journal_entries"},
		{name: "payment", entityType: EntityTypePayment, wantTable: "payments"},
		{name: "bank transaction", entityType: EntityTypeBankTxn, wantTable: "bank_transactions"},
		{name: "asset", entityType: EntityTypeAsset, wantTable: "fixed_assets"},
		{name: "expense", entityType: EntityTypeExpense, wantTable: "expenses"},
		{name: "quote", entityType: EntityTypeQuote, wantTable: "quotes"},
		{name: "order", entityType: EntityTypeOrder, wantTable: "orders"},
		{name: "year-end close", entityType: EntityTypeYearEndClose},
		{name: "leave record", entityType: EntityTypeLeaveRecord, wantTable: "leave_records"},
		{name: "TSD declaration", entityType: EntityTypeTSD, wantTable: "tsd_declarations"},
		{name: "KMD declaration", entityType: EntityTypeKMD, wantTable: "kmd_declarations"},
		{name: "unsupported", entityType: "contact", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := entityTableName(tt.entityType)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.entityType)
				}
				return
			}
			if err != nil {
				t.Fatalf("entityTableName returned error: %v", err)
			}
			if got != tt.wantTable {
				t.Fatalf("expected table %q, got %q", tt.wantTable, got)
			}
		})
	}
}

func TestDocumentLifecycleModelMapping(t *testing.T) {
	t.Parallel()

	lifecycleAt := time.Date(2026, 3, 15, 12, 30, 0, 0, time.UTC)
	reviewerID := "reviewer-1"
	replacementID := "replacement-1"

	model := documentToModel(&Document{
		ID:              "doc-1",
		TenantID:        "tenant-1",
		EntityType:      EntityTypeExpense,
		EntityID:        "expense-1",
		DocumentType:    DocumentTypeReceipt,
		FileName:        "receipt.pdf",
		ContentType:     "application/pdf",
		FileSize:        128,
		StorageKey:      "documents/doc-1.pdf",
		ReviewStatus:    ReviewStatusApproved,
		LifecycleStatus: "",
		LifecycleNote:   "Superseded by corrected receipt",
		SupersededBy:    &replacementID,
		LifecycleBy:     &reviewerID,
		LifecycleAt:     &lifecycleAt,
		UploadedBy:      "user-1",
		CreatedAt:       lifecycleAt.Add(-time.Hour),
	})
	if model.LifecycleStatus != LifecycleStatusActive {
		t.Fatalf("expected blank lifecycle status to map to ACTIVE, got %q", model.LifecycleStatus)
	}
	if model.LifecycleNote == nil || *model.LifecycleNote != "Superseded by corrected receipt" {
		t.Fatalf("expected lifecycle note pointer to be preserved, got %#v", model.LifecycleNote)
	}

	doc := modelToDocument(model)
	if doc.LifecycleStatus != LifecycleStatusActive {
		t.Fatalf("expected ACTIVE lifecycle status, got %q", doc.LifecycleStatus)
	}
	if doc.LifecycleNote != "Superseded by corrected receipt" {
		t.Fatalf("expected lifecycle note to round-trip, got %q", doc.LifecycleNote)
	}
	if doc.SupersededBy == nil || *doc.SupersededBy != replacementID {
		t.Fatalf("expected superseded document link to round-trip, got %#v", doc.SupersededBy)
	}
	if doc.LifecycleBy == nil || *doc.LifecycleBy != reviewerID || doc.LifecycleAt == nil || !doc.LifecycleAt.Equal(lifecycleAt) {
		t.Fatalf("expected lifecycle audit metadata to round-trip, got %#v", doc)
	}
}

func TestGORMRepositoryNilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	entityID := "entity-1"
	documentID := "document-1"
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	replacementID := "replacement-1"

	if repo == nil {
		t.Fatal("expected repository")
	}
	if repo.db != nil {
		t.Fatal("expected nil database")
	}

	constructed := NewRepository(nil)
	if constructed == nil || constructed.db != nil {
		t.Fatalf("expected nil pgx constructor to return repository without database, got %#v", constructed)
	}

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "documents")
				if table != nil {
					t.Fatalf("expected nil table, got %#v", table)
				}
				return err
			},
		},
		{
			name: "documentsTable",
			run: func(t *testing.T) error {
				table, err := repo.documentsTable(ctx, schemaName)
				if table != nil {
					t.Fatalf("expected nil documents table, got %#v", table)
				}
				return err
			},
		},
		{
			name: "EntityExists",
			run: func(t *testing.T) error {
				exists, err := repo.EntityExists(ctx, schemaName, tenantID, EntityTypeInvoice, entityID)
				if exists {
					t.Fatal("expected entity to not exist")
				}
				return err
			},
		},
		{
			name: "CreateDocument",
			run: func(t *testing.T) error {
				return repo.CreateDocument(ctx, schemaName, &Document{TenantID: tenantID})
			},
		},
		{
			name: "ListDocuments",
			run: func(t *testing.T) error {
				docs, err := repo.ListDocuments(ctx, schemaName, tenantID, EntityTypeExpense, entityID)
				if docs != nil {
					t.Fatalf("expected nil documents, got %#v", docs)
				}
				return err
			},
		},
		{
			name: "ListReviewQueueDocuments",
			run: func(t *testing.T) error {
				docs, err := repo.ListReviewQueueDocuments(ctx, schemaName, tenantID, ReviewQueueFilter{
					EntityType:   EntityTypeExpense,
					DocumentType: DocumentTypeReceipt,
					ReviewStatus: ReviewStatusPending,
					Limit:        10,
				})
				if docs != nil {
					t.Fatalf("expected nil documents, got %#v", docs)
				}
				return err
			},
		},
		{
			name: "ListRetentionReviewDocuments",
			run: func(t *testing.T) error {
				docs, err := repo.ListRetentionReviewDocuments(ctx, schemaName, tenantID, now, true)
				if docs != nil {
					t.Fatalf("expected nil documents, got %#v", docs)
				}
				return err
			},
		},
		{
			name: "ListReviewSummaries",
			run: func(t *testing.T) error {
				summaries, err := repo.ListReviewSummaries(ctx, schemaName, tenantID, EntityTypeExpense, []string{entityID})
				if summaries != nil {
					t.Fatalf("expected nil summaries, got %#v", summaries)
				}
				return err
			},
		},
		{
			name: "GetDocumentByID",
			run: func(t *testing.T) error {
				doc, err := repo.GetDocumentByID(ctx, schemaName, tenantID, documentID)
				if doc != nil {
					t.Fatalf("expected nil document, got %#v", doc)
				}
				return err
			},
		},
		{
			name: "DocumentHasSupersededDependents",
			run: func(t *testing.T) error {
				hasDependents, err := repo.DocumentHasSupersededDependents(ctx, schemaName, tenantID, documentID)
				if hasDependents {
					t.Fatal("expected no superseded dependents")
				}
				return err
			},
		},
		{
			name: "UpdateDocumentRetention",
			run: func(t *testing.T) error {
				return repo.UpdateDocumentRetention(ctx, schemaName, tenantID, documentID, &now)
			},
		},
		{
			name: "UpdateDocumentLifecycle",
			run: func(t *testing.T) error {
				return repo.UpdateDocumentLifecycle(ctx, schemaName, tenantID, documentID, LifecycleStatusSuperseded, "replaced", "user-1", now, &replacementID)
			},
		},
		{
			name: "UpdateDocumentLegalHold",
			run: func(t *testing.T) error {
				return repo.UpdateDocumentLegalHold(ctx, schemaName, tenantID, documentID, true, "litigation", "user-1", now)
			},
		},
		{
			name: "ReviewDocument",
			run: func(t *testing.T) error {
				return repo.ReviewDocument(ctx, schemaName, tenantID, documentID, ReviewStatusReviewed, "ready", "user-1", now)
			},
		},
		{
			name: "DeleteDocument",
			run: func(t *testing.T) error {
				return repo.DeleteDocument(ctx, schemaName, tenantID, documentID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "documents repository database is not configured") {
				t.Fatalf("expected nil database guard error, got %q", err.Error())
			}
		})
	}
}

func TestEntityExistsYearEndCloseDoesNotRequireDatabase(t *testing.T) {
	t.Parallel()

	repo := NewGORMRepository(nil)
	ctx := context.Background()
	validCloseID := "11111111-1111-1111-1111-111111111111"

	exists, err := repo.EntityExists(ctx, "tenant_schema", "tenant-1", EntityTypeYearEndClose, validCloseID)
	if err != nil {
		t.Fatalf("EntityExists returned error: %v", err)
	}
	if !exists {
		t.Fatal("expected valid year-end close UUID to exist without database lookup")
	}

	exists, err = repo.EntityExists(ctx, "tenant_schema", "tenant-1", EntityTypeYearEndClose, "not-a-uuid")
	if err != nil {
		t.Fatalf("EntityExists returned error: %v", err)
	}
	if exists {
		t.Fatal("expected invalid year-end close UUID to not exist")
	}
}

func TestEntityExistsUnsupportedTypeDoesNotRequireDatabase(t *testing.T) {
	t.Parallel()

	repo := NewGORMRepository(nil)

	exists, err := repo.EntityExists(context.Background(), "tenant_schema", "tenant-1", "unsupported", "entity-1")
	if err == nil {
		t.Fatal("expected unsupported entity type error")
	}
	if exists {
		t.Fatal("expected unsupported entity to not exist")
	}
	if !strings.Contains(err.Error(), "unsupported document entity type") {
		t.Fatalf("expected unsupported entity type error, got %q", err.Error())
	}
}

func TestListReviewSummariesEmptyEntityIDsDoesNotRequireDatabase(t *testing.T) {
	t.Parallel()

	repo := NewGORMRepository(nil)

	summaries, err := repo.ListReviewSummaries(context.Background(), "tenant_schema", "tenant-1", EntityTypeExpense, nil)
	if err != nil {
		t.Fatalf("ListReviewSummaries returned error: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected empty summaries, got %#v", summaries)
	}
}

func TestModelsToDocumentsMapsSliceAndDefaults(t *testing.T) {
	t.Parallel()

	retentionUntil := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	reviewedBy := "reviewer-1"
	reviewedAt := time.Date(2026, 6, 20, 8, 30, 0, 0, time.UTC)
	legalHoldBy := "legal-1"
	legalHoldAt := time.Date(2026, 6, 21, 9, 45, 0, 0, time.UTC)
	createdAt := time.Date(2026, 6, 10, 11, 15, 0, 0, time.UTC)

	docs := modelsToDocuments([]models.Document{
		{
			ID:              "doc-1",
			TenantID:        "tenant-1",
			EntityType:      EntityTypeExpense,
			EntityID:        "expense-1",
			DocumentType:    DocumentTypeReceipt,
			FileName:        "receipt.pdf",
			ContentType:     "application/pdf",
			FileSize:        128,
			StorageKey:      "documents/doc-1.pdf",
			Notes:           "Travel receipt",
			RetentionUntil:  &retentionUntil,
			ReviewStatus:    ReviewStatusReviewed,
			ReviewedBy:      &reviewedBy,
			ReviewedAt:      &reviewedAt,
			LifecycleStatus: "",
			LegalHold:       true,
			LegalHoldBy:     &legalHoldBy,
			LegalHoldAt:     &legalHoldAt,
			UploadedBy:      "user-1",
			CreatedAt:       createdAt,
		},
	})

	if len(docs) != 1 {
		t.Fatalf("expected one document, got %d", len(docs))
	}
	doc := docs[0]
	if doc.ID != "doc-1" || doc.TenantID != "tenant-1" || doc.StorageKey != "documents/doc-1.pdf" {
		t.Fatalf("unexpected document identifiers: %#v", doc)
	}
	if doc.ReviewNote != "" || doc.LifecycleNote != "" || doc.LegalHoldNote != "" {
		t.Fatalf("expected nil notes to map to empty strings, got %#v", doc)
	}
	if doc.LifecycleStatus != LifecycleStatusActive {
		t.Fatalf("expected blank lifecycle status to default to ACTIVE, got %q", doc.LifecycleStatus)
	}
	if doc.RetentionUntil == nil || !doc.RetentionUntil.Equal(retentionUntil) {
		t.Fatalf("expected retention date to map, got %#v", doc.RetentionUntil)
	}
	if doc.ReviewedBy == nil || *doc.ReviewedBy != reviewedBy || doc.ReviewedAt == nil || !doc.ReviewedAt.Equal(reviewedAt) {
		t.Fatalf("expected review audit fields to map, got %#v", doc)
	}
	if !doc.LegalHold || doc.LegalHoldBy == nil || *doc.LegalHoldBy != legalHoldBy || doc.LegalHoldAt == nil || !doc.LegalHoldAt.Equal(legalHoldAt) {
		t.Fatalf("expected legal hold fields to map, got %#v", doc)
	}
}
