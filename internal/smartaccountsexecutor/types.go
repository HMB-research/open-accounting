// Package smartaccountsexecutor plans and applies only verified,
// SmartAccounts-authoritative general-ledger journals. It deliberately treats
// every non-GL source record as archive/evidence and never maps invoices or
// payments to financial postings.
package smartaccountsexecutor

import (
	"context"

	"github.com/shopspring/decimal"
)

const (
	Provider                  = "smartaccounts"
	ResourceGeneralLedger     = "general_ledger_journal"
	SmartAccountsGLSourceType = "SMARTACCOUNTS_GL"
	// Browser-captured GL records may reach the financial planner only through
	// the reviewed v2 General Ledger CSV adapter. journal_entries is a summary
	// surface retained as non-posting archive evidence.
	browserGeneralLedgerResourceID   = "general_ledger"
	browserGeneralLedgerSourceSchema = "smartaccounts-brave-ui-v2/general_ledger_csv_v1"
	PostingModeAuthoritativeOnce     = "authoritative_once"
	OperationUpsert                  = "upsert"
	OperationTombstone               = "tombstone"
	PlanStatusPreviewReady           = "PREVIEW_READY"
	PlanStatusReviewRequired         = "REVIEW_REQUIRED"
	PlanStatusApplied                = "APPLIED"
)

type AccountMapping struct {
	SourceAccountExternalID string `json:"source_account_external_id"`
	TargetAccountID         string `json:"target_account_id"`
}

// AccountImport is an explicit chart decision. Source account data is never
// guessed into an OA account type; the reviewer supplies a valid target type.
type AccountImport struct {
	SourceAccountExternalID string `json:"source_account_external_id"`
	Code                    string `json:"code"`
	Name                    string `json:"name"`
	AccountType             string `json:"account_type"`
}

type PreviewRequest struct {
	AccountMappings []AccountMapping `json:"account_mappings"`
	AccountImports  []AccountImport  `json:"account_imports"`
	UseSourceChart  bool             `json:"use_source_chart,omitempty"`
}

type ConfirmRequest struct {
	Confirm       bool   `json:"confirm"`
	PreviewID     string `json:"preview_id"`
	PreviewSHA256 string `json:"preview_sha256"`
	// TolerancePolicySHA256 is a pre-approved accountant policy handle. It is
	// selected at explicit GL apply and persisted only as a digest so a later
	// reconciliation can reproduce its policy binding without an amount rule.
	TolerancePolicySHA256 string `json:"tolerance_policy_sha256"`
}

type Issue struct {
	Code       string `json:"code"`
	Resource   string `json:"resource,omitempty"`
	ExternalID string `json:"external_id,omitempty"`
	Message    string `json:"message"`
}

type JournalLine struct {
	SourceAccountExternalID string          `json:"source_account_external_id"`
	SourceAccountCode       string          `json:"source_account_code,omitempty"`
	SourceAccountName       string          `json:"source_account_name,omitempty"`
	Debit                   decimal.Decimal `json:"debit"`
	Credit                  decimal.Decimal `json:"credit"`
	DebitOriginalCurrency   decimal.Decimal `json:"debit_original_currency,omitempty"`
	CreditOriginalCurrency  decimal.Decimal `json:"credit_original_currency,omitempty"`
	ObjectID                string          `json:"object_id,omitempty"`
	Description             string          `json:"description,omitempty"`
}

type Journal struct {
	ExternalID        string          `json:"external_id"`
	Revision          string          `json:"revision"`
	PostingDate       string          `json:"posting_date"`
	Currency          string          `json:"currency"`
	ExchangeRate      decimal.Decimal `json:"exchange_rate,omitempty"`
	DocumentReference string          `json:"document_reference,omitempty"`
	InternalNumber    string          `json:"internal_number,omitempty"`
	Lines             []JournalLine   `json:"lines"`
}

type PlannedJournal struct {
	Journal
	MappedLines []MappedLine    `json:"mapped_lines"`
	DebitTotal  decimal.Decimal `json:"debit_total"`
	CreditTotal decimal.Decimal `json:"credit_total"`
	Action      string          `json:"action"`
}

type MappedLine struct {
	SourceAccountExternalID string          `json:"source_account_external_id"`
	TargetAccountID         string          `json:"target_account_id"`
	Debit                   decimal.Decimal `json:"debit"`
	Credit                  decimal.Decimal `json:"credit"`
}

type AccountReconciliation struct {
	SourceAccountExternalID string          `json:"source_account_external_id"`
	TargetAccountID         string          `json:"target_account_id,omitempty"`
	Currency                string          `json:"currency"`
	DebitTotal              decimal.Decimal `json:"debit_total"`
	CreditTotal             decimal.Decimal `json:"credit_total"`
}

type Preview struct {
	ID                     string                  `json:"id"`
	TenantID               string                  `json:"tenant_id"`
	PackageID              string                  `json:"package_id"`
	SourceCompanyID        string                  `json:"source_company_id"`
	ScopeSHA256            string                  `json:"scope_sha256,omitempty"`
	Status                 string                  `json:"status"`
	PreviewSHA256          string                  `json:"preview_sha256"`
	FinancialWritesPlanned bool                    `json:"financial_writes_planned"`
	FinancialWritesApplied bool                    `json:"financial_writes_applied"`
	Journals               []PlannedJournal        `json:"journals,omitempty"`
	AccountImports         []AccountImport         `json:"account_imports,omitempty"`
	AccountReconciliation  []AccountReconciliation `json:"account_reconciliation,omitempty"`
	NonPostingRecordCount  int                     `json:"non_posting_record_count"`
	Issues                 []Issue                 `json:"issues,omitempty"`
}

type PostedIdentity struct {
	ExternalID    string
	Revision      string
	ReservationID string
	JournalID     string
	Status        string
	PackageID     string
	PreviewID     string
	ReservedBy    string
	AppliedBy     string
}

// AppliedMapping and AppliedIdentity are the ID-only snapshot persisted with
// a GL apply receipt. They never contain account names, journal lines or
// monetary values.
type AppliedMapping struct {
	SourceAccountExternalID string
	TargetAccountID         string
}

type AppliedIdentity struct {
	ExternalID    string
	Revision      string
	ReservationID string
	JournalID     string
	// AppliedBy is retained only while the executor establishes actor
	// separation. It is deliberately excluded from the immutable receipt
	// identity digest: actor evidence is recorded separately on the receipt.
	AppliedBy string `json:"-"`
}

type ApplyReceiptInput struct {
	TenantID              string
	SourceCompanyID       string
	PackageID             string
	PreviewID             string
	PreviewSHA256         string
	TolerancePolicySHA256 string
	Mappings              []AppliedMapping
	Identities            []AppliedIdentity
	ActorID               string
}

// TolerancePolicyBinding carries the immutable source/package boundary at
// which an accountant-approved reconciliation tolerance policy is valid. The
// executor deliberately receives no policy rules or monetary thresholds.
type TolerancePolicyBinding struct {
	TenantID              string
	SourceCompanyID       string
	PackageID             string
	ScopeSHA256           string
	PreviewSHA256         string
	TolerancePolicySHA256 string
	ActorID               string
}

// TolerancePolicyVerifier is a server-owned policy registry seam. A supplied
// digest is never treated as an approval by itself: it must resolve to an
// immutable accountant-approved policy bound to this tenant/source/package
// scope before any financial posting is attempted.
type TolerancePolicyVerifier interface {
	VerifyTolerancePolicy(context.Context, TolerancePolicyBinding) error
}

// ApplyReceiptRecorder lives outside the executor's tenant schema so it can
// later support selected/all roll-ups. It is still invoked only after this
// service has explicitly applied an exact confirmed preview.
type ApplyReceiptRecorder interface {
	RecordFirstGLApply(context.Context, ApplyReceiptInput) error
	RecordExactGLReplay(context.Context, string, string, string, string, string) error
}
