package documents

import "time"

const (
	EntityTypeInvoice      = "invoice"
	EntityTypeJournalEntry = "journal_entry"
	EntityTypePayment      = "payment"

	MaxDocumentSizeBytes = 10 << 20
)

type Document struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	EntityType  string    `json:"entity_type"`
	EntityID    string    `json:"entity_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	FileSize    int64     `json:"file_size"`
	StorageKey  string    `json:"-"`
	UploadedBy  string    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type UploadDocumentRequest struct {
	EntityType  string
	EntityID    string
	FileName    string
	ContentType string
	FileSize    int64
	UploadedBy  string
}
