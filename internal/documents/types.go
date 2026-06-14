package documents

import "time"

const (
	EntityTypeInvoice      = "invoice"
	EntityTypeJournalEntry = "journal_entry"
	EntityTypePayment      = "payment"
	EntityTypeBankTxn      = "bank_transaction"
	EntityTypeAsset        = "asset"
	EntityTypeExpense      = "expense"
	EntityTypeQuote        = "quote"
	EntityTypeOrder        = "order"
	EntityTypeYearEndClose = "year_end_close"
	EntityTypeLeaveRecord  = "leave_record"
	EntityTypeTSD          = "tsd_declaration"
	EntityTypeKMD          = "kmd_declaration"

	DocumentTypeSupportingDocument = "supporting_document"
	DocumentTypeReceipt            = "receipt"
	DocumentTypeReconciliation     = "reconciliation_evidence"
	DocumentTypeContract           = "contract"
	DocumentTypeAssetRecord        = "asset_record"
	DocumentTypeTaxSupport         = "tax_support"
	DocumentTypeClosePack          = "close_pack"
	DocumentTypeOther              = "other"

	ReviewStatusPending  = "PENDING"
	ReviewStatusReviewed = "REVIEWED"
	ReviewStatusApproved = "APPROVED"
	ReviewStatusRejected = "REJECTED"

	RetentionReminderExpired          = "expired_retention"
	RetentionReminderDueSoon          = "retention_due_soon"
	RetentionReminderMissingRetention = "missing_retention"
	RetentionReminderPendingReview    = "pending_review"
	RetentionReminderRejected         = "rejected_document"

	MaxDocumentSizeBytes = 10 << 20
	MaxRetentionYears    = 100
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
	ReviewNote     string     `json:"review_note,omitempty"`
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
	RetentionYears int
	UploadedBy     string
}

type ReviewDocumentRequest struct {
	ReviewStatus string `json:"review_status"`
	ReviewNote   string `json:"review_note,omitempty"`
}

type ReviewSummary struct {
	EntityType         string `json:"entity_type"`
	EntityID           string `json:"entity_id"`
	TotalCount         int    `json:"total_count"`
	PendingReviewCount int    `json:"pending_review_count"`
	ReviewedCount      int    `json:"reviewed_count"`
	ApprovedCount      int    `json:"approved_count"`
	RejectedCount      int    `json:"rejected_count"`
	MissingEvidence    bool   `json:"missing_evidence"`
	HasPendingReview   bool   `json:"has_pending_review"`
	HasRejected        bool   `json:"has_rejected"`
}

type ReviewQueueFilter struct {
	EntityType   string `json:"entity_type,omitempty"`
	DocumentType string `json:"document_type,omitempty"`
	ReviewStatus string `json:"review_status,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

type ReviewQueue struct {
	EntityType         string     `json:"entity_type,omitempty"`
	DocumentType       string     `json:"document_type,omitempty"`
	ReviewStatus       string     `json:"review_status"`
	Limit              int        `json:"limit"`
	TotalCount         int        `json:"total_count"`
	PendingReviewCount int        `json:"pending_review_count"`
	ReviewedCount      int        `json:"reviewed_count"`
	ApprovedCount      int        `json:"approved_count"`
	RejectedCount      int        `json:"rejected_count"`
	Documents          []Document `json:"documents"`
}

type EvidencePolicyRequest struct {
	EntityType string               `json:"entity_type"`
	EntityIDs  []string             `json:"entity_ids"`
	Rules      []EvidencePolicyRule `json:"rules"`
}

type EvidencePolicyRule struct {
	DocumentTypes   []string `json:"document_types,omitempty"`
	MinCount        int      `json:"min_count"`
	RequireApproved bool     `json:"require_approved"`
}

type EvidencePolicyRuleResult struct {
	RuleIndex             int      `json:"rule_index"`
	DocumentTypes         []string `json:"document_types,omitempty"`
	RequiredCount         int      `json:"required_count"`
	MatchingCount         int      `json:"matching_count"`
	ApprovedMatchingCount int      `json:"approved_matching_count"`
	AcceptedCount         int      `json:"accepted_count"`
	RequireApproved       bool     `json:"require_approved"`
	Compliant             bool     `json:"compliant"`
	Message               string   `json:"message,omitempty"`
}

type EvidencePolicyResult struct {
	EntityType                 string                      `json:"entity_type"`
	EntityID                   string                      `json:"entity_id"`
	Compliant                  bool                        `json:"compliant"`
	TotalCount                 int                         `json:"total_count"`
	PendingReviewCount         int                         `json:"pending_review_count"`
	ReviewedCount              int                         `json:"reviewed_count"`
	ApprovedCount              int                         `json:"approved_count"`
	RejectedCount              int                         `json:"rejected_count"`
	MissingEvidence            bool                        `json:"missing_evidence"`
	DocumentTypeCounts         map[string]int              `json:"document_type_counts"`
	ApprovedDocumentTypeCounts map[string]int              `json:"approved_document_type_counts"`
	RuleResults                []EvidencePolicyRuleResult  `json:"rule_results"`
	Violations                 []EvidencePolicyRuleResult  `json:"violations"`
	RemediationActions         []DocumentRemediationAction `json:"remediation_actions,omitempty"`
}

type RetentionReview struct {
	AsOfDate              string                      `json:"as_of_date"`
	CutoffDate            string                      `json:"cutoff_date"`
	TotalCount            int                         `json:"total_count"`
	ExpiredCount          int                         `json:"expired_count"`
	DueSoonCount          int                         `json:"due_soon_count"`
	MissingRetentionCount int                         `json:"missing_retention_count"`
	PendingReviewCount    int                         `json:"pending_review_count"`
	RejectedCount         int                         `json:"rejected_count"`
	ReminderActions       []RetentionReminderAction   `json:"reminder_actions"`
	RemediationActions    []DocumentRemediationAction `json:"remediation_actions,omitempty"`
	Documents             []Document                  `json:"documents"`
}

type RetentionReminderAction struct {
	DocumentID         string     `json:"document_id"`
	EntityType         string     `json:"entity_type"`
	EntityID           string     `json:"entity_id"`
	DocumentType       string     `json:"document_type"`
	FileName           string     `json:"file_name"`
	Action             string     `json:"action"`
	Message            string     `json:"message"`
	DaysUntilRetention *int       `json:"days_until_retention,omitempty"`
	RetentionUntil     *time.Time `json:"retention_until,omitempty"`
}

type DocumentRemediationAction struct {
	Code               string `json:"code"`
	Severity           string `json:"severity"`
	Scope              string `json:"scope"`
	OwnerRole          string `json:"owner_role"`
	WorkspaceQueue     string `json:"workspace_queue,omitempty"`
	AssignmentKey      string `json:"assignment_key,omitempty"`
	Priority           string `json:"priority,omitempty"`
	DueInDays          int    `json:"due_in_days,omitempty"`
	Message            string `json:"message"`
	Action             string `json:"action"`
	EntityType         string `json:"entity_type,omitempty"`
	EntityID           string `json:"entity_id,omitempty"`
	DocumentID         string `json:"document_id,omitempty"`
	DocumentType       string `json:"document_type,omitempty"`
	FileName           string `json:"file_name,omitempty"`
	DueDate            string `json:"due_date,omitempty"`
	DaysUntilRetention *int   `json:"days_until_retention,omitempty"`
	UIPath             string `json:"ui_path,omitempty"`
	CLICommand         string `json:"cli_command,omitempty"`
}
