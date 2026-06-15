package documents

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/HMB-research/open-accounting/internal/workspace"
)

const (
	documentRemediationScope     = "documents"
	documentRemediationOwnerRole = "accountant"
)

// BuildRetentionReviewRemediationActions turns retention reminder rows into accountant follow-up actions.
func BuildRetentionReviewRemediationActions(review *RetentionReview) []DocumentRemediationAction {
	if review == nil {
		return nil
	}

	actions := make([]DocumentRemediationAction, 0, len(review.ReminderActions))
	for _, reminder := range review.ReminderActions {
		base := documentRemediationBase(reminder.EntityType, reminder.EntityID, reminder.DocumentID, reminder.DocumentType, reminder.FileName)
		base.DaysUntilRetention = reminder.DaysUntilRetention
		if reminder.RetentionUntil != nil {
			base.DueDate = reminder.RetentionUntil.Format("2006-01-02")
		}

		switch reminder.Action {
		case RetentionReminderMissingRetention:
			base.Code = "document_retention_missing"
			base.Severity = "WARNING"
			base.Message = fmt.Sprintf("Document %s is missing retention metadata.", reminder.FileName)
			base.Action = "Set a retention date or document why the record is exempt from retention policy."
			base.CLICommand = fmt.Sprintf("oa documents retention-set --id %s --retention-until <YYYY-MM-DD>", reminder.DocumentID)
		case RetentionReminderDueSoon:
			base.Code = "document_retention_due_soon"
			base.Severity = "INFO"
			base.Message = reminder.Message
			base.Action = "Review the document before the retention date and either extend retention or complete the disposal workflow."
			base.CLICommand = fmt.Sprintf("oa documents retention-set --id %s --retention-until <YYYY-MM-DD>", reminder.DocumentID)
		case RetentionReminderExpired:
			base.Code = "document_retention_expired"
			base.Severity = "ACTION"
			base.Message = reminder.Message
			base.Action = "Apply the retention policy now: archive, dispose, or extend retention with an audit note."
			base.CLICommand = fmt.Sprintf("oa documents retention-set --id %s --retention-until <YYYY-MM-DD>", reminder.DocumentID)
		case RetentionReminderPendingReview:
			base.Code = "document_review_pending"
			base.Severity = "ACTION"
			base.Message = fmt.Sprintf("Document %s is still pending review.", reminder.FileName)
			base.Action = "Review the attachment and approve, reject, or mark it reviewed."
			base.CLICommand = fmt.Sprintf("oa documents review --id %s --status approved", reminder.DocumentID)
		case RetentionReminderRejected:
			base.Code = "document_review_rejected"
			base.Severity = "ACTION"
			base.Message = fmt.Sprintf("Document %s was rejected and needs replacement or correction.", reminder.FileName)
			base.Action = "Upload corrected evidence or approve the existing document after the rejection has been resolved."
			base.CLICommand = replacementEvidenceCommand(reminder.EntityType, reminder.EntityID, reminder.DocumentType, reminder.DocumentID)
		default:
			base.Code = "document_retention_review"
			base.Severity = "WARNING"
			base.Message = reminder.Message
			base.Action = "Review the document retention follow-up item."
			base.CLICommand = "oa documents retention --include-missing"
		}

		assignDocumentRemediationAction(&base)
		actions = append(actions, base)
	}

	return actions
}

// BuildEvidencePolicyRemediationActions turns evidence-policy violations into accountant follow-up actions.
func BuildEvidencePolicyRemediationActions(result *EvidencePolicyResult, docs ...Document) []DocumentRemediationAction {
	if result == nil || result.Compliant {
		return nil
	}

	actions := make([]DocumentRemediationAction, 0, len(result.Violations))
	for _, violation := range result.Violations {
		documentType := firstDocumentType(violation.DocumentTypes)
		base := documentRemediationBase(result.EntityType, result.EntityID, "", documentType, "")

		if doc, ok := firstRejectedMatchingEvidenceDocument(docs, violation); ok {
			base.Code = "document_review_rejected"
			base.Severity = "ACTION"
			base.Message = fmt.Sprintf("Document %s was rejected and needs replacement or correction.", doc.FileName)
			base.Action = "Upload corrected evidence or approve the existing document after the rejection has been resolved."
			base.DocumentID = doc.ID
			base.FileName = doc.FileName
			base.UIPath = documentRemediationUIPath(result.EntityType, result.EntityID, doc.ID)
			base.CLICommand = replacementEvidenceCommand(result.EntityType, result.EntityID, documentType, doc.ID)
		} else if violation.RequireApproved && violation.MatchingCount > violation.ApprovedMatchingCount {
			base.Code = "document_evidence_unapproved"
			base.Severity = "ACTION"
			base.Message = fmt.Sprintf("%s %s has matching evidence, but not enough approved documents.", result.EntityType, result.EntityID)
			base.Action = "Review and approve enough matching evidence documents to satisfy the workflow policy."
			base.CLICommand = reviewQueueCommand(result.EntityType, documentType)
			if doc, ok := firstUnapprovedMatchingEvidenceDocument(docs, violation); ok {
				base.DocumentID = doc.ID
				base.FileName = doc.FileName
				base.UIPath = documentRemediationUIPath(result.EntityType, result.EntityID, doc.ID)
				base.CLICommand = fmt.Sprintf("oa documents review --id %s --status approved", doc.ID)
			}
		} else if result.MissingEvidence || violation.MatchingCount == 0 {
			base.Code = "document_evidence_missing"
			base.Severity = "ACTION"
			base.Message = fmt.Sprintf("%s %s is missing required evidence.", result.EntityType, result.EntityID)
			base.Action = "Upload the required evidence document before continuing the workflow."
			base.CLICommand = uploadEvidenceCommand(result.EntityType, result.EntityID, documentType)
		} else {
			base.Code = "document_evidence_policy_violation"
			base.Severity = "WARNING"
			base.Message = strings.TrimSpace(violation.Message)
			if base.Message == "" {
				base.Message = fmt.Sprintf("%s %s does not satisfy the configured evidence policy.", result.EntityType, result.EntityID)
			}
			base.Action = "Resolve the evidence-policy violation before continuing the protected workflow."
			base.CLICommand = evidencePolicyCommand(result.EntityType, result.EntityID, documentType, violation.RequiredCount, violation.RequireApproved)
		}

		assignDocumentRemediationAction(&base)
		actions = append(actions, base)
	}

	return actions
}

func firstRejectedMatchingEvidenceDocument(docs []Document, violation EvidencePolicyRuleResult) (Document, bool) {
	for _, doc := range docs {
		if doc.ReviewStatus != ReviewStatusRejected {
			continue
		}
		if evidencePolicyDocumentTypeMatches(violation.DocumentTypes, doc.DocumentType) {
			return doc, true
		}
	}
	return Document{}, false
}

func documentRemediationBase(entityType, entityID, documentID, documentType, fileName string) DocumentRemediationAction {
	return DocumentRemediationAction{
		Scope:        documentRemediationScope,
		OwnerRole:    documentRemediationOwnerRole,
		EntityType:   strings.TrimSpace(entityType),
		EntityID:     strings.TrimSpace(entityID),
		DocumentID:   strings.TrimSpace(documentID),
		DocumentType: strings.TrimSpace(documentType),
		FileName:     strings.TrimSpace(fileName),
		UIPath:       documentRemediationUIPath(entityType, entityID, documentID),
	}
}

func assignDocumentRemediationAction(action *DocumentRemediationAction) {
	meta := workspace.RemediationAssignment(
		"document_review",
		action.Code,
		action.Severity,
		action.EntityType,
		action.EntityID,
		action.DocumentType,
		action.DocumentID,
	)
	action.WorkspaceQueue = meta.WorkspaceQueue
	action.AssignmentKey = meta.AssignmentKey
	action.Priority = meta.Priority
	action.DueInDays = meta.DueInDays
}

func documentRemediationUIPath(entityType, entityID, documentID string) string {
	values := url.Values{}
	if strings.TrimSpace(entityType) != "" {
		values.Set("entity_type", strings.TrimSpace(entityType))
	}
	if strings.TrimSpace(entityID) != "" {
		values.Set("entity_id", strings.TrimSpace(entityID))
	}
	if strings.TrimSpace(documentID) != "" {
		values.Set("document_id", strings.TrimSpace(documentID))
	}
	if encoded := values.Encode(); encoded != "" {
		return "/documents?" + encoded
	}
	return "/documents"
}

func firstDocumentType(documentTypes []string) string {
	for _, documentType := range documentTypes {
		if trimmed := strings.TrimSpace(documentType); trimmed != "" {
			return trimmed
		}
	}
	return DocumentTypeSupportingDocument
}

func firstUnapprovedMatchingEvidenceDocument(docs []Document, violation EvidencePolicyRuleResult) (Document, bool) {
	for _, preferredStatus := range []string{ReviewStatusPending, ReviewStatusReviewed, ReviewStatusRejected} {
		for _, doc := range docs {
			if doc.ReviewStatus != preferredStatus {
				continue
			}
			if evidencePolicyDocumentTypeMatches(violation.DocumentTypes, doc.DocumentType) {
				return doc, true
			}
		}
	}

	for _, doc := range docs {
		if doc.ReviewStatus == ReviewStatusApproved {
			continue
		}
		if evidencePolicyDocumentTypeMatches(violation.DocumentTypes, doc.DocumentType) {
			return doc, true
		}
	}

	return Document{}, false
}

func evidencePolicyDocumentTypeMatches(documentTypes []string, documentType string) bool {
	if len(documentTypes) == 0 {
		return true
	}
	for _, candidate := range documentTypes {
		if candidate == documentType {
			return true
		}
	}
	return false
}

func uploadEvidenceCommand(entityType, entityID, documentType string) string {
	return fmt.Sprintf("oa documents upload --entity-type %s --entity-id %s --document-type %s --file <file>", entityType, entityID, firstDocumentType([]string{documentType}))
}

func replacementEvidenceCommand(entityType, entityID, documentType, documentID string) string {
	command := fmt.Sprintf("oa documents upload --entity-type %s --entity-id %s --document-type %s --file <replacement-file>", entityType, entityID, firstDocumentType([]string{documentType}))
	if trimmed := strings.TrimSpace(documentID); trimmed != "" {
		command += fmt.Sprintf(" --replaces-document-id %s --replacement-note \"Corrected evidence uploaded from remediation action\"", trimmed)
	}
	return command
}

func reviewQueueCommand(entityType, documentType string) string {
	command := fmt.Sprintf("oa documents review-queue --entity-type %s --status PENDING", entityType)
	if strings.TrimSpace(documentType) != "" {
		command += fmt.Sprintf(" --document-type %s", strings.TrimSpace(documentType))
	}
	return command
}

func evidencePolicyCommand(entityType, entityID, documentType string, minCount int, requireApproved bool) string {
	if minCount <= 0 {
		minCount = 1
	}
	command := fmt.Sprintf("oa documents evidence-policy --entity-type %s --entity-id %s --required-document-type %s --min-count %d", entityType, entityID, firstDocumentType([]string{documentType}), minCount)
	if requireApproved {
		command += " --require-approved"
	}
	return command
}
