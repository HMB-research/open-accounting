package main

import (
	"context"
	"fmt"
	"log"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// requiresHighRiskEvidence reads the persisted tenant policy so request fields
// cannot relax pilot accounting controls.
func (h *Handlers) requiresHighRiskEvidence(ctx context.Context, tenantID string) (bool, error) {
	if h == nil || h.tenantService == nil {
		return false, nil
	}
	record, err := h.tenantService.GetTenant(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return record.Settings.BlocksHighRiskEvidence(), nil
}

// requireApprovedExpensePostingEvidence applies the tenant-wide pilot control
// even if an expense was created without the older optional receipt flag.
func (h *Handlers) requireApprovedExpensePostingEvidence(ctx context.Context, schemaName, tenantID, expenseID string) error {
	required, err := h.requiresHighRiskEvidence(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("load tenant evidence policy: %w", err)
	}
	if !required {
		return nil
	}
	if h.documentsService == nil {
		return fmt.Errorf("approved expense ledger-posting evidence is required before posting expense %s", expenseID)
	}

	results, err := h.documentsService.EvaluateEvidencePolicy(ctx, schemaName, tenantID, &documents.EvidencePolicyRequest{
		EntityType: documents.EntityTypeExpense,
		EntityIDs:  []string{expenseID},
		Rules: []documents.EvidencePolicyRule{{
			DocumentTypes: []string{
				documents.DocumentTypeReceipt,
				documents.DocumentTypeSupportingDocument,
			},
			MinCount:        1,
			RequireApproved: true,
		}},
	})
	if err != nil {
		return fmt.Errorf("evaluate expense ledger-posting evidence: %w", err)
	}
	for _, result := range results {
		if !result.Compliant {
			return &evidencePolicyConflictError{
				Err:     fmt.Errorf("approved expense ledger-posting evidence is required before posting expense %s", expenseID),
				Results: results,
			}
		}
	}
	return nil
}

// recordHighRiskEvidenceBlock keeps an audit trail without hiding the original
// policy conflict when audit storage is temporarily unavailable.
func (h *Handlers) recordHighRiskEvidenceBlock(ctx context.Context, tenantID, userID, operation string) {
	required, err := h.requiresHighRiskEvidence(ctx, tenantID)
	if err != nil || !required || h == nil || h.tenantService == nil {
		return
	}
	if err := h.tenantService.RecordTenantAuditEvent(ctx, &tenant.TenantAuditEvent{
		TenantID:    tenantID,
		ActorUserID: userID,
		Action:      tenant.AuditActionEvidencePolicyBlocked,
		TargetType:  tenant.AuditTargetTenant,
		TargetID:    tenantID,
		Metadata: map[string]string{
			"operation":   operation,
			"policy_mode": tenant.EvidencePolicyModeBlockHighRisk,
		},
	}); err != nil {
		log.Printf("failed to record pilot evidence-policy block for tenant %s operation %s: %v", tenantID, operation, err)
	}
}

func evidencePolicyActorID(ctx context.Context) string {
	claims, ok := auth.GetClaims(ctx)
	if !ok || claims == nil {
		return ""
	}
	return claims.UserID
}
