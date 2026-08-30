// Package importsession owns the safe, receive-only boundary for external
// accounting import packages. It deliberately does not perform financial
// writes; later import phases must consume an approved session explicitly.
package importsession

import (
	"encoding/json"
	"time"
)

const (
	// ProviderSmartAccounts identifies packages prepared from SmartAccounts.
	ProviderSmartAccounts = "smartaccounts"
	// CanonicalSchemaVersionV1 is the only package schema accepted by this
	// receiver vertical slice.
	CanonicalSchemaVersionV1 = "v1"
	// SessionStatusReceivedValidated confirms metadata and canonical records
	// passed read-only validation. It does not mean that data was imported.
	SessionStatusReceivedValidated = "RECEIVED_VALIDATED"
	// SessionStatusReceivedReviewRequired confirms receipt validation completed
	// but declared source variance or staleness requires accountant review.
	SessionStatusReceivedReviewRequired = "RECEIVED_REVIEW_REQUIRED"
	// LedgerVerificationStatusVerified means source variance and staleness were
	// both declared clear; it never means Open Accounting posted the ledger.
	LedgerVerificationStatusVerified = "VERIFIED"
	// LedgerVerificationStatusReviewRequired means receipt staging is allowed
	// but an accountant must review source variance or staleness before any
	// future approved import phase.
	LedgerVerificationStatusReviewRequired = "REVIEW_REQUIRED"
	// ScopeModeFull declares a complete source-company package. It does not
	// apply a period or resource-subset boundary.
	ScopeModeFull = "full"
	// ScopeModePartial declares an intentionally bounded journal-entry slice.
	ScopeModePartial = "partial"
	// ScopeResourceAll is the only resource declaration for a full package.
	ScopeResourceAll = "all"
	// ScopeResourceJournalEntry is the sole supported partial-scope resource in
	// v1. Other subsets must use a later, explicitly designed contract.
	ScopeResourceJournalEntry = "journal_entry"
)

// LedgerAuthorityDeclaration states which system is authoritative for the
// general ledger represented by the canonical package. v1 only accepts the
// package provider (SmartAccounts); Open Accounting is never authoritative for
// an external receipt.
type LedgerAuthorityDeclaration struct {
	GeneralLedgerAuthority       string `json:"general_ledger_authority"`
	SmartAccountsGLAuthoritative *bool  `json:"smartaccounts_gl_authoritative"`
	SourceAsOfDate               string `json:"source_as_of_date"`
	VarianceCount                int    `json:"variance_count"`
	Stale                        bool   `json:"stale"`
}

// ImportScope makes the source selection explicit. A full scope declares all
// resources without a date range. A partial scope is restricted to complete
// journal groups wholly inside its inclusive period range.
type ImportScope struct {
	Mode          string   `json:"mode"`
	ResourceTypes []string `json:"resource_types"`
	PeriodStart   string   `json:"period_start,omitempty"`
	PeriodEnd     string   `json:"period_end,omitempty"`
}

// LedgerVerification is safe receipt metadata for the declared ledger
// authority, source scope, and journal-group balance checks. It intentionally
// contains neither source line details nor financial amounts.
type LedgerVerification struct {
	GeneralLedgerAuthority      string   `json:"general_ledger_authority,omitempty"`
	SourceAsOfDate              string   `json:"source_as_of_date,omitempty"`
	VarianceCount               int      `json:"variance_count"`
	Stale                       bool     `json:"stale"`
	ReviewRequired              bool     `json:"review_required"`
	VerificationStatus          string   `json:"verification_status"`
	JournalStagingAllowed       bool     `json:"journal_staging_allowed"`
	FinancialPostingPlanAllowed bool     `json:"financial_posting_plan_allowed"`
	ScopeMode                   string   `json:"scope_mode,omitempty"`
	ResourceTypes               []string `json:"resource_types,omitempty"`
	PeriodStart                 string   `json:"period_start,omitempty"`
	PeriodEnd                   string   `json:"period_end,omitempty"`
	JournalGroupCount           int      `json:"journal_group_count"`
	BalancedJournalGroupCount   int      `json:"balanced_journal_group_count"`
}

// StagedLedgerJournal is the minimal normalized ledger metadata retained for
// a later dry-run plan. It is not a raw SmartAccounts payload and cannot be
// used to write a journal; all financial writes remain outside Import Session
// v1.
type StagedLedgerJournal struct {
	SourceJournalExternalID string             `json:"source_journal_external_id"`
	SourceRevision          string             `json:"source_revision"`
	JournalGroupID          string             `json:"journal_group_id"`
	PeriodStart             string             `json:"period_start"`
	PeriodEnd               string             `json:"period_end"`
	Currency                string             `json:"currency"`
	Lines                   []StagedLedgerLine `json:"lines"`
}

// StagedLedgerLine is one normalized source ledger line. Amounts are stored
// as canonical decimal strings so repeated dry runs produce the same plan.
type StagedLedgerLine struct {
	AccountExternalID string `json:"account_external_id"`
	Debit             string `json:"debit"`
	Credit            string `json:"credit"`
}

// CanonicalPackage is a deliberately small, synthetic package contract for
// the initial receiver. The package digest is over source identity and record
// metadata/payload digests, never a substitute for artifact storage.
type CanonicalPackage struct {
	SchemaVersion   string                      `json:"schema_version"`
	Provider        string                      `json:"provider"`
	SourceCompanyID string                      `json:"source_company_id"`
	LedgerAuthority *LedgerAuthorityDeclaration `json:"ledger_authority"`
	Scope           *ImportScope                `json:"scope"`
	PackageSHA256   string                      `json:"package_sha256"`
	Records         []CanonicalRecord           `json:"records"`
}

// CanonicalRecord identifies one source record and carries a canonical JSON
// payload used exclusively for read-only validation in v1.
type CanonicalRecord struct {
	EntityType    string          `json:"entity_type"`
	ExternalID    string          `json:"external_id"`
	Revision      string          `json:"revision"`
	Operation     string          `json:"operation"`
	Payload       json.RawMessage `json:"payload,omitempty" swaggertype:"object"`
	PayloadSHA256 string          `json:"payload_sha256,omitempty"`
}

// PackageRequest wraps the package so the API can add request metadata later
// without changing the canonical package itself.
type PackageRequest struct {
	Package CanonicalPackage `json:"package"`
}

// ValidationIssue reports a package validation problem without echoing raw
// source payloads or external record values into API responses.
type ValidationIssue struct {
	Code        string `json:"code"`
	RecordIndex int    `json:"record_index,omitempty"`
	Field       string `json:"field,omitempty"`
	Message     string `json:"message"`
}

// ValidationReport is a safe-to-store/read-only validation result. It records
// summaries and errors, never source payload JSON.
type ValidationReport struct {
	Ready              bool                `json:"ready"`
	RecordCount        int                 `json:"record_count"`
	EntityCounts       map[string]int      `json:"entity_counts,omitempty"`
	LedgerVerification *LedgerVerification `json:"ledger_verification,omitempty"`
	Issues             []ValidationIssue   `json:"issues,omitempty"`
}

// Receipt is the auditable status returned for a persisted package receipt.
// It intentionally excludes the raw package and canonical record payloads.
type Receipt struct {
	ID                 string             `json:"id"`
	TenantID           string             `json:"tenant_id"`
	Provider           string             `json:"provider"`
	SourceCompanyID    string             `json:"source_company_id"`
	SchemaVersion      string             `json:"schema_version"`
	PackageSHA256      string             `json:"package_sha256"`
	Status             string             `json:"status"`
	RecordCount        int                `json:"record_count"`
	EntityCounts       map[string]int     `json:"entity_counts"`
	LedgerVerification LedgerVerification `json:"ledger_verification"`
	// LedgerPlanInput is stored in the receipt table for deterministic planning
	// but is intentionally never returned by receipt endpoints.
	LedgerPlanInput []StagedLedgerJournal `json:"-"`
	Validation      ValidationReport      `json:"validation"`
	CreatedBy       string                `json:"created_by,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	Created         bool                  `json:"created"`
}

// AccountMapping maps one SmartAccounts source account external ID to an
// existing Open Accounting account UUID for a dry-run plan.
type AccountMapping struct {
	SourceAccountExternalID string `json:"source_account_external_id"`
	TargetAccountID         string `json:"target_account_id"`
}

// ImportPlanRequest contains only explicit account mappings. It never accepts
// a journal payload: the plan is always derived from a staged receipt.
type ImportPlanRequest struct {
	AccountMappings []AccountMapping `json:"account_mappings"`
}

// ImportPlanIssue describes a non-mutating dry-run blocker.
type ImportPlanIssue struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// PlannedJournalLine identifies the source account and its verified target
// account without creating an accounting journal line.
type PlannedJournalLine struct {
	SourceAccountExternalID string `json:"source_account_external_id"`
	TargetAccountID         string `json:"target_account_id"`
	Debit                   string `json:"debit"`
	Credit                  string `json:"credit"`
}

// PlannedJournalAction is a deterministic, dry-run-only journal action.
type PlannedJournalAction struct {
	SourceJournalExternalID string               `json:"source_journal_external_id"`
	SourceRevision          string               `json:"source_revision"`
	JournalGroupID          string               `json:"journal_group_id"`
	PeriodStart             string               `json:"period_start"`
	PeriodEnd               string               `json:"period_end"`
	Currency                string               `json:"currency"`
	Lines                   []PlannedJournalLine `json:"lines"`
	DebitTotal              string               `json:"debit_total"`
	CreditTotal             string               `json:"credit_total"`
}

// JournalReconciliationExpectation states the per-journal totals that a
// later approved import must reconcile. It performs no comparison to live GL
// balances in v1.
type JournalReconciliationExpectation struct {
	SourceJournalExternalID string `json:"source_journal_external_id"`
	SourceRevision          string `json:"source_revision"`
	JournalGroupID          string `json:"journal_group_id"`
	Currency                string `json:"currency"`
	DebitTotal              string `json:"debit_total"`
	CreditTotal             string `json:"credit_total"`
	Balanced                bool   `json:"balanced"`
}

// AccountReconciliationExpectation aggregates planned debit and credit by
// mapped account and currency for a later accountant reconciliation.
type AccountReconciliationExpectation struct {
	SourceAccountExternalID string `json:"source_account_external_id"`
	TargetAccountID         string `json:"target_account_id"`
	Currency                string `json:"currency"`
	DebitTotal              string `json:"debit_total"`
	CreditTotal             string `json:"credit_total"`
}

// ImportPlanResult is the dry-run result. FinancialWritesPlanned remains
// false by contract; this type cannot authorize or execute a journal post.
type ImportPlanResult struct {
	Ready                  bool                               `json:"ready"`
	FinancialWritesPlanned bool                               `json:"financial_writes_planned"`
	ImportSessionID        string                             `json:"import_session_id,omitempty"`
	PackageSHA256          string                             `json:"package_sha256,omitempty"`
	Scope                  ImportScope                        `json:"scope"`
	JournalActions         []PlannedJournalAction             `json:"journal_actions,omitempty"`
	JournalReconciliations []JournalReconciliationExpectation `json:"journal_reconciliation_expectations,omitempty"`
	AccountReconciliations []AccountReconciliationExpectation `json:"account_reconciliation_expectations,omitempty"`
	PlanSHA256             string                             `json:"plan_sha256,omitempty"`
	Issues                 []ImportPlanIssue                  `json:"issues,omitempty"`
}
