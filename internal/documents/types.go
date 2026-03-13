package documents

import "time"

const (
	EntityTypeInvoice      = "invoice"
	EntityTypeJournalEntry = "journal_entry"
	EntityTypePayment      = "payment"
	EntityTypeBankTxn      = "bank_transaction"
	EntityTypeAsset        = "asset"

	DocumentTypeSupportingDocument = "supporting_document"
	DocumentTypeReceipt            = "receipt"
	DocumentTypeReconciliation     = "reconciliation_evidence"
	DocumentTypeContract           = "contract"
	DocumentTypeAssetRecord        = "asset_record"
	DocumentTypeTaxSupport         = "tax_support"
	DocumentTypeOther              = "other"

	ReviewStatusPending  = "PENDING"
	ReviewStatusReviewed = "REVIEWED"

	MaxDocumentSizeBytes = 10 << 20
)

type Document struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	EntityType     string     `json:"entity_type"`
	EntityID       string     `json:"entity_id"`
	DocumentType   string     `json:"document_type"`
	FileName       string     `json:"file_name"`
	ContentType    string     `json:"content_type"`
	FileSize       int64      `json:"file_size"`
	StorageKey     string     `json:"-"`
	Notes          string     `json:"notes,omitempty"`
	RetentionUntil *time.Time `json:"retention_until,omitempty"`
	ReviewStatus   string     `json:"review_status"`
	ReviewedBy     *string    `json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	UploadedBy     string     `json:"uploaded_by"`
	CreatedAt      time.Time  `json:"created_at"`
}

type UploadDocumentRequest struct {
	EntityType     string
	EntityID       string
	DocumentType   string
	FileName       string
	ContentType    string
	FileSize       int64
	Notes          string
	RetentionUntil *time.Time
	UploadedBy     string
}

type ReviewSummary struct {
	EntityType         string `json:"entity_type"`
	EntityID           string `json:"entity_id"`
	TotalCount         int    `json:"total_count"`
	PendingReviewCount int    `json:"pending_review_count"`
	ReviewedCount      int    `json:"reviewed_count"`
	MissingEvidence    bool   `json:"missing_evidence"`
	HasPendingReview   bool   `json:"has_pending_review"`
}
