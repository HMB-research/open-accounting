package documents

import "testing"

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
	if action.CLICommand != "oa documents upload --entity-type payment --entity-id pay-3 --document-type receipt --file <replacement-file>" {
		t.Fatalf("expected replacement upload command, got %q", action.CLICommand)
	}
	if action.UIPath != "/documents?document_id=doc-rejected-receipt&entity_id=pay-3&entity_type=payment" {
		t.Fatalf("expected document-scoped UI path, got %q", action.UIPath)
	}
}
