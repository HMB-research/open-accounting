package documents

import (
	"testing"
	"time"
)

func TestBuildRetentionReviewRemediationActionsRejectedDocumentIncludesReplacementMetadata(t *testing.T) {
	actions := BuildRetentionReviewRemediationActions(&RetentionReview{
		ReminderActions: []RetentionReminderAction{
			{
				DocumentID:   "doc-rejected-receipt",
				EntityType:   EntityTypeExpense,
				EntityID:     "exp-1",
				DocumentType: DocumentTypeReceipt,
				FileName:     "wrong-receipt.pdf",
				Action:       RetentionReminderRejected,
			},
		},
	})

	if len(actions) != 1 {
		t.Fatalf("expected one remediation action, got %#v", actions)
	}
	action := actions[0]
	if action.Code != "document_review_rejected" {
		t.Fatalf("expected rejected document code, got %q", action.Code)
	}
	expectedCommand := "oa documents upload --entity-type expense --entity-id exp-1 --document-type receipt --file <replacement-file> --replaces-document-id doc-rejected-receipt --replacement-note \"Corrected evidence uploaded from remediation action\""
	if action.CLICommand != expectedCommand {
		t.Fatalf("expected replacement upload command with supersession metadata, got %q", action.CLICommand)
	}
}

func TestBuildRetentionReviewRemediationActionsCoversRetentionVariants(t *testing.T) {
	if actions := BuildRetentionReviewRemediationActions(nil); actions != nil {
		t.Fatalf("expected nil review to return nil actions, got %#v", actions)
	}

	daysUntilRetention := 7
	retentionUntil := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	actions := BuildRetentionReviewRemediationActions(&RetentionReview{
		ReminderActions: []RetentionReminderAction{
			{
				DocumentID:   "doc-missing",
				EntityType:   EntityTypeInvoice,
				EntityID:     "inv-1",
				DocumentType: DocumentTypeSupportingDocument,
				FileName:     "missing.pdf",
				Action:       RetentionReminderMissingRetention,
			},
			{
				DocumentID:         "doc-due-soon",
				EntityType:         EntityTypePayment,
				EntityID:           "pay-1",
				DocumentType:       DocumentTypeReceipt,
				FileName:           "due-soon.pdf",
				Action:             RetentionReminderDueSoon,
				Message:            "Retention is due on 2026-04-15",
				DaysUntilRetention: &daysUntilRetention,
				RetentionUntil:     &retentionUntil,
			},
			{
				DocumentID:   "doc-expired",
				EntityType:   EntityTypeExpense,
				EntityID:     "exp-1",
				DocumentType: DocumentTypeReceipt,
				FileName:     "expired.pdf",
				Action:       RetentionReminderExpired,
				Message:      "Retention expired on 2026-03-01",
			},
			{
				DocumentID:   "doc-pending",
				EntityType:   EntityTypeBankTxn,
				EntityID:     "txn-1",
				DocumentType: DocumentTypeReconciliation,
				FileName:     "pending.pdf",
				Action:       RetentionReminderPendingReview,
			},
			{
				DocumentID:   "doc-custom",
				EntityType:   EntityTypeAsset,
				EntityID:     "asset-1",
				DocumentType: DocumentTypeAssetRecord,
				FileName:     "custom.pdf",
				Action:       "unknown_retention_action",
				Message:      "Custom retention follow-up",
			},
		},
	})

	if len(actions) != 5 {
		t.Fatalf("expected five remediation actions, got %#v", actions)
	}

	expected := []struct {
		code     string
		severity string
		command  string
	}{
		{"document_retention_missing", "WARNING", "oa documents retention-set --id doc-missing --retention-until <YYYY-MM-DD>"},
		{"document_retention_due_soon", "INFO", "oa documents retention-set --id doc-due-soon --retention-until <YYYY-MM-DD>"},
		{"document_retention_expired", "ACTION", "oa documents retention-set --id doc-expired --retention-until <YYYY-MM-DD>"},
		{"document_review_pending", "ACTION", "oa documents review --id doc-pending --status approved"},
		{"document_retention_review", "WARNING", "oa documents retention --include-missing"},
	}
	for i, want := range expected {
		if actions[i].Code != want.code || actions[i].Severity != want.severity || actions[i].CLICommand != want.command {
			t.Fatalf("unexpected action %d: %#v", i, actions[i])
		}
		if actions[i].Scope != "documents" || actions[i].OwnerRole != "accountant" || actions[i].AssignmentKey == "" {
			t.Fatalf("expected assignment metadata on action %d, got %#v", i, actions[i])
		}
	}
	if actions[1].DueDate != "2026-04-15" || actions[1].DaysUntilRetention == nil || *actions[1].DaysUntilRetention != daysUntilRetention {
		t.Fatalf("expected due-soon action to preserve retention date metadata, got %#v", actions[1])
	}
}

func TestBuildEvidencePolicyRemediationActionsTargetsUnapprovedEvidenceDocument(t *testing.T) {
	result := &EvidencePolicyResult{
		EntityType: EntityTypePayment,
		EntityID:   "pay-2",
		Compliant:  false,
		Violations: []EvidencePolicyRuleResult{{
			DocumentTypes:         []string{DocumentTypeReceipt},
			RequiredCount:         1,
			MatchingCount:         1,
			ApprovedMatchingCount: 0,
			RequireApproved:       true,
		}},
	}
	docs := []Document{
		{
			ID:           "doc-other",
			EntityType:   EntityTypePayment,
			EntityID:     "pay-2",
			DocumentType: DocumentTypeSupportingDocument,
			FileName:     "support.pdf",
			ReviewStatus: ReviewStatusPending,
		},
		{
			ID:           "doc-pending-receipt",
			EntityType:   EntityTypePayment,
			EntityID:     "pay-2",
			DocumentType: DocumentTypeReceipt,
			FileName:     "receipt-draft.pdf",
			ReviewStatus: ReviewStatusPending,
		},
	}

	actions := BuildEvidencePolicyRemediationActions(result, docs...)

	if len(actions) != 1 {
		t.Fatalf("expected one remediation action, got %#v", actions)
	}
	action := actions[0]
	if action.Code != "document_evidence_unapproved" {
		t.Fatalf("expected unapproved evidence code, got %q", action.Code)
	}
	if action.DocumentID != "doc-pending-receipt" || action.FileName != "receipt-draft.pdf" {
		t.Fatalf("expected action to target the matching pending receipt, got %#v", action)
	}
	if action.CLICommand != "oa documents review --id doc-pending-receipt --status approved" {
		t.Fatalf("expected direct approval command, got %q", action.CLICommand)
	}
	if action.UIPath != "/documents?document_id=doc-pending-receipt&entity_id=pay-2&entity_type=payment" {
		t.Fatalf("expected document-scoped UI path, got %q", action.UIPath)
	}
	if action.WorkspaceQueue != "document_review" || action.AssignmentKey == "" {
		t.Fatalf("expected workspace assignment metadata, got %#v", action)
	}
}

func TestBuildEvidencePolicyRemediationActionsCoversMissingAndGenericViolations(t *testing.T) {
	if actions := BuildEvidencePolicyRemediationActions(nil); actions != nil {
		t.Fatalf("expected nil result to return nil actions, got %#v", actions)
	}
	if actions := BuildEvidencePolicyRemediationActions(&EvidencePolicyResult{Compliant: true}); actions != nil {
		t.Fatalf("expected compliant result to return nil actions, got %#v", actions)
	}

	missingResult := &EvidencePolicyResult{
		EntityType:         EntityTypeJournalEntry,
		EntityID:           "je-1",
		Compliant:          false,
		MissingEvidence:    true,
		TotalCount:         2,
		ApprovedCount:      1,
		RejectedCount:      0,
		PendingReviewCount: 1,
		Violations: []EvidencePolicyRuleResult{
			{
				DocumentTypes: []string{" ", DocumentTypeSupportingDocument},
				RequiredCount: 1,
				MatchingCount: 0,
			},
		},
	}

	actions := BuildEvidencePolicyRemediationActions(missingResult)

	if len(actions) != 1 {
		t.Fatalf("expected one missing-evidence remediation action, got %#v", actions)
	}
	if actions[0].Code != "document_evidence_missing" {
		t.Fatalf("expected missing evidence action, got %#v", actions[0])
	}
	expectedUpload := "oa documents upload --entity-type journal_entry --entity-id je-1 --document-type supporting_document --file <file>"
	if actions[0].CLICommand != expectedUpload {
		t.Fatalf("expected missing evidence upload command %q, got %q", expectedUpload, actions[0].CLICommand)
	}

	genericResult := &EvidencePolicyResult{
		EntityType: EntityTypeJournalEntry,
		EntityID:   "je-1",
		Compliant:  false,
		Violations: []EvidencePolicyRuleResult{
			{
				DocumentTypes:         []string{DocumentTypeReceipt},
				RequiredCount:         2,
				MatchingCount:         1,
				ApprovedMatchingCount: 1,
				Message:               "Receipt evidence does not satisfy policy.",
			},
			{
				DocumentTypes:         []string{},
				RequiredCount:         0,
				MatchingCount:         1,
				ApprovedMatchingCount: 1,
				RequireApproved:       true,
			},
		},
	}

	actions = BuildEvidencePolicyRemediationActions(genericResult)

	if len(actions) != 2 {
		t.Fatalf("expected two generic remediation actions, got %#v", actions)
	}

	if actions[0].Code != "document_evidence_policy_violation" || actions[0].Message != "Receipt evidence does not satisfy policy." {
		t.Fatalf("expected generic policy violation with supplied message, got %#v", actions[0])
	}
	expectedPolicy := "oa documents evidence-policy --entity-type journal_entry --entity-id je-1 --required-document-type receipt --min-count 2"
	if actions[0].CLICommand != expectedPolicy {
		t.Fatalf("expected policy command %q, got %q", expectedPolicy, actions[0].CLICommand)
	}

	if actions[1].DocumentType != DocumentTypeSupportingDocument {
		t.Fatalf("expected default supporting document type, got %#v", actions[1])
	}
	if actions[1].Message != "journal_entry je-1 does not satisfy the configured evidence policy." {
		t.Fatalf("expected default policy violation message, got %q", actions[1].Message)
	}
	expectedApprovedPolicy := "oa documents evidence-policy --entity-type journal_entry --entity-id je-1 --required-document-type supporting_document --min-count 1 --require-approved"
	if actions[1].CLICommand != expectedApprovedPolicy {
		t.Fatalf("expected require-approved policy command %q, got %q", expectedApprovedPolicy, actions[1].CLICommand)
	}
}

func TestBuildEvidencePolicyRemediationActionsUnapprovedEvidenceFallbacks(t *testing.T) {
	result := &EvidencePolicyResult{
		EntityType: EntityTypePayment,
		EntityID:   "pay-4",
		Compliant:  false,
		Violations: []EvidencePolicyRuleResult{{
			DocumentTypes:         []string{DocumentTypeReceipt},
			RequiredCount:         1,
			MatchingCount:         1,
			ApprovedMatchingCount: 0,
			RequireApproved:       true,
		}},
	}

	actions := BuildEvidencePolicyRemediationActions(result, Document{
		ID:           "doc-reviewed",
		EntityType:   EntityTypePayment,
		EntityID:     "pay-4",
		DocumentType: DocumentTypeReceipt,
		FileName:     "reviewed.pdf",
		ReviewStatus: ReviewStatusReviewed,
	})
	if len(actions) != 1 || actions[0].DocumentID != "doc-reviewed" {
		t.Fatalf("expected reviewed evidence document to be selected, got %#v", actions)
	}

	actions = BuildEvidencePolicyRemediationActions(result, Document{
		ID:           "doc-custom-status",
		EntityType:   EntityTypePayment,
		EntityID:     "pay-4",
		DocumentType: DocumentTypeReceipt,
		FileName:     "custom.pdf",
		ReviewStatus: "NEEDS_APPROVAL",
	})
	if len(actions) != 1 || actions[0].DocumentID != "doc-custom-status" {
		t.Fatalf("expected non-approved fallback evidence document to be selected, got %#v", actions)
	}

	actions = BuildEvidencePolicyRemediationActions(result, Document{
		ID:           "doc-approved",
		EntityType:   EntityTypePayment,
		EntityID:     "pay-4",
		DocumentType: DocumentTypeReceipt,
		FileName:     "approved.pdf",
		ReviewStatus: ReviewStatusApproved,
	})
	if len(actions) != 1 || actions[0].DocumentID != "" {
		t.Fatalf("expected review queue fallback with no document target, got %#v", actions)
	}
	expectedCommand := "oa documents review-queue --entity-type payment --status PENDING --document-type receipt"
	if actions[0].CLICommand != expectedCommand {
		t.Fatalf("expected review queue fallback command %q, got %q", expectedCommand, actions[0].CLICommand)
	}
}

func TestBuildEvidencePolicyRemediationActionsTargetsRejectedEvidenceReplacement(t *testing.T) {
	result := &EvidencePolicyResult{
		EntityType: EntityTypePayment,
		EntityID:   "pay-3",
		Compliant:  false,
		Violations: []EvidencePolicyRuleResult{{
			DocumentTypes:         []string{DocumentTypeReceipt},
			RequiredCount:         1,
			MatchingCount:         1,
			ApprovedMatchingCount: 0,
			RequireApproved:       true,
		}},
	}
	docs := []Document{
		{
			ID:           "doc-rejected-receipt",
			EntityType:   EntityTypePayment,
			EntityID:     "pay-3",
			DocumentType: DocumentTypeReceipt,
			FileName:     "rejected-receipt.pdf",
			ReviewStatus: ReviewStatusRejected,
		},
	}

	actions := BuildEvidencePolicyRemediationActions(result, docs...)

	if len(actions) != 1 {
		t.Fatalf("expected one remediation action, got %#v", actions)
	}
	action := actions[0]
	if action.Code != "document_review_rejected" {
		t.Fatalf("expected rejected document code, got %q", action.Code)
	}
	if action.DocumentID != "doc-rejected-receipt" || action.FileName != "rejected-receipt.pdf" {
		t.Fatalf("expected action to target the rejected receipt, got %#v", action)
	}
	expectedCommand := "oa documents upload --entity-type payment --entity-id pay-3 --document-type receipt --file <replacement-file> --replaces-document-id doc-rejected-receipt --replacement-note \"Corrected evidence uploaded from remediation action\""
	if action.CLICommand != expectedCommand {
		t.Fatalf("expected replacement upload command, got %q", action.CLICommand)
	}
	if action.UIPath != "/documents?document_id=doc-rejected-receipt&entity_id=pay-3&entity_type=payment" {
		t.Fatalf("expected document-scoped UI path, got %q", action.UIPath)
	}
}

func TestDocumentRemediationHelperFallbacks(t *testing.T) {
	if path := documentRemediationUIPath(" ", "", "\t"); path != "/documents" {
		t.Fatalf("expected blank document remediation path, got %q", path)
	}
	if !evidencePolicyDocumentTypeMatches(nil, DocumentTypeReceipt) {
		t.Fatal("expected empty policy document types to match any document type")
	}
	if evidencePolicyDocumentTypeMatches([]string{DocumentTypeTaxSupport}, DocumentTypeReceipt) {
		t.Fatal("expected unrelated policy document type not to match receipt")
	}
}
