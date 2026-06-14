package documents

import (
	"testing"
	"time"
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
