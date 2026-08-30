package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreconciliation"
	"github.com/HMB-research/open-accounting/internal/smartaccountsreferences"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// smartAccountsReconciliationResolver joins only safe control/store state. It
// deliberately has no route that returns archive records, proof bytes or
// monetary values. A nil proof computer leaves the evaluation
// EVIDENCE_PENDING instead of claiming reconciliation success.
type smartAccountsReconciliationResolver struct {
	batches    *smartaccountssync.BrowserOnboardingBatchService
	workflows  *smartaccountssync.BrowserBatchWorkflowActionsService
	delivery   *importdelivery.Service
	executor   *smartaccountsexecutor.Service
	references *smartaccountsreferences.Service
	receipts   *smartaccountsreconciliation.Repository
	tenants    *tenant.Service
	proofs     smartaccountsreconciliation.StreamingProofComputer
}

func (r smartAccountsReconciliationResolver) Resolve(ctx context.Context, ownerID, batchID, sourceID string) (smartaccountsreconciliation.EvaluationInput, error) {
	input := smartaccountsreconciliation.EvaluationInput{BatchID: batchID, SourceCompanyID: sourceID}
	if r.batches == nil || r.workflows == nil || r.tenants == nil {
		return input, smartaccountsreconciliation.ErrNotFound
	}
	batch, err := r.batches.Status(ctx, ownerID, batchID)
	if err != nil || batch == nil || batch.Batch.ID != batchID || !batchSelectsSource(batch.Batch, sourceID) {
		return input, smartaccountsreconciliation.ErrNotFound
	}
	outcome, found := batchOutcome(batch.Outcomes, sourceID)
	if !found || strings.TrimSpace(outcome.TenantID) == "" {
		return input, smartaccountsreconciliation.ErrNotFound
	}
	input.TenantID = outcome.TenantID
	input.BindingComplete = outcome.Status == smartaccountssync.BrowserOnboardingPaired
	workflow, err := r.workflows.Status(ctx, ownerID, batchID)
	if err != nil || workflow == nil {
		return input, nil
	}
	source, found := batchWorkflowSource(workflow.Sources, sourceID, outcome.TenantID)
	if !found {
		return input, nil
	}
	input.BindingComplete = input.BindingComplete && source.TenantID == outcome.TenantID
	if strings.TrimSpace(source.PackageID) == "" || strings.TrimSpace(source.PreviewID) == "" || r.delivery == nil || r.executor == nil {
		return input, nil
	}
	tenantRecord, err := r.tenants.GetTenant(ctx, outcome.TenantID)
	if err != nil {
		return input, nil
	}
	status, err := r.delivery.GetStatus(ctx, tenantRecord.SchemaName, outcome.TenantID, source.PackageID)
	if err != nil || status.Status != importdelivery.StatusStagedReview || status.SourceCompanyID != sourceID || status.PackageSHA256 != source.PackageSHA256 {
		return input, nil
	}
	manifest, err := r.delivery.GetManifest(ctx, tenantRecord.SchemaName, outcome.TenantID, source.PackageID)
	if err != nil || manifest.SourceCompanyID != sourceID || manifest.PackageSHA256 != source.PackageSHA256 {
		return input, nil
	}
	scopeSHA, err := digestReconciliationScope(manifest.Scope)
	if err != nil {
		return input, nil
	}
	input.PackageID, input.ManifestSHA256, input.RecordsSHA256, input.ScopeSHA256 = manifest.PackageID, manifest.ManifestSHA256, manifest.RecordsSHA256, scopeSHA
	input.SourceAsOfDate, input.CutoffAt = manifest.Scope.SourceAsOfDate, manifest.Scope.CutoffAt
	if manifest.Scope.Mode == "full" || manifest.Scope.Mode == "full_history" {
		input.ExpectedCoverageState = "full"
	} else {
		// Browser-compiled partial scopes are valuable evidence but cannot
		// qualify an all/selected batch for technical reconciliation.
		input.ExpectedCoverageState = "partial"
	}
	preview, err := r.executor.GetPreview(ctx, tenantRecord.SchemaName, outcome.TenantID, source.PreviewID)
	if err == nil && preview != nil && preview.PackageID == source.PackageID && preview.SourceCompanyID == sourceID && preview.PreviewSHA256 == source.PreviewSHA256 {
		input.GLPreviewID, input.GLPreviewSHA256, input.GLPreviewApplied = preview.ID, preview.PreviewSHA256, preview.Status == smartaccountsexecutor.PlanStatusApplied
		for _, issue := range preview.Issues {
			switch issue.Code {
			case "source_revision_correction_required", "journal_revision_invalid":
				input.GLRevisionUnresolved++
			case "source_tombstone_review_required":
				input.GLTombstoneUnresolved++
			}
		}
		if r.receipts != nil {
			if receipt, receiptErr := r.receipts.GetGLApplyReceipt(ctx, outcome.TenantID, sourceID, source.PackageID, preview.PreviewSHA256); receiptErr == nil {
				input.GLApplyReceipt = receipt
				input.GLMappingSnapshotSHA256 = receipt.MappingSnapshotSHA256
				input.GLAppliedIdentitySHA256 = receipt.AppliedIdentitySHA256
			}
		}
	}
	if r.references != nil {
		if reference, refErr := r.references.GetPackageEvidence(ctx, tenantRecord.SchemaName, outcome.TenantID, source.PackageID); refErr == nil {
			input.ReferenceApplicable, input.ReferencePreviewID, input.ReferencePreviewSHA256, input.ReferencePreviewApplied = reference.Applicable, reference.PreviewID, reference.PreviewSHA256, reference.Applied
			input.ReferenceRevisionUnresolved, input.ReferenceTombstoneUnresolved = reference.RevisionUnresolved, reference.TombstoneUnresolved
		}
	}
	if r.proofs != nil && input.GLApplyReceipt != nil {
		if proof, proofErr := r.proofs.ComputeProof(ctx, smartaccountsreconciliation.ProofMaterial{SchemaName: tenantRecord.SchemaName, TenantID: input.TenantID, SourceCompanyID: input.SourceCompanyID, PackageID: input.PackageID, ManifestSHA256: input.ManifestSHA256, RecordsSHA256: input.RecordsSHA256, ScopeSHA256: input.ScopeSHA256, MappingSnapshotSHA256: input.GLMappingSnapshotSHA256, AppliedIdentitySHA256: input.GLAppliedIdentitySHA256, ToleranceSHA256: input.GLApplyReceipt.TolerancePolicySHA256, PreviewID: input.GLPreviewID, PreviewSHA256: input.GLPreviewSHA256, ExpectedCoverageState: input.ExpectedCoverageState}); proofErr == nil {
			// Scope expectation comes from the staged manifest, never the proof
			// computer. A proof can attest only a claim kind within that already
			// immutable server-derived scope.
			if proof.ExpectedCoverageState == "" || proof.ExpectedCoverageState == input.ExpectedCoverageState {
				input.ProofID, input.ProofSHA256, input.ClaimSHA256, input.CoverageSHA256, input.ClaimKind, input.ToleranceSHA256, input.VarianceWithinPolicy = proof.ProofID, proof.ProofSHA256, proof.ClaimSHA256, proof.CoverageSHA256, proof.ClaimKind, proof.ToleranceSHA256, proof.VarianceWithinPolicy
			}
		}
	}
	return input, nil
}

func (r smartAccountsReconciliationResolver) ListBindings(ctx context.Context, ownerID, batchID string) ([]smartaccountsreconciliation.SourceBinding, error) {
	if r.batches == nil {
		return nil, smartaccountsreconciliation.ErrNotFound
	}
	batch, err := r.batches.Status(ctx, ownerID, batchID)
	if err != nil || batch == nil || batch.Batch.ID != batchID {
		return nil, smartaccountsreconciliation.ErrNotFound
	}
	result := make([]smartaccountsreconciliation.SourceBinding, 0, len(batch.Batch.SelectedSources))
	for _, selected := range batch.Batch.SelectedSources {
		outcome, found := batchOutcome(batch.Outcomes, selected.SourceCompanyID)
		result = append(result, smartaccountsreconciliation.SourceBinding{BatchID: batchID, SourceCompanyID: selected.SourceCompanyID, TenantID: outcome.TenantID, Paired: found && outcome.Status == smartaccountssync.BrowserOnboardingPaired})
	}
	return result, nil
}

// ResolveTolerancePolicyBinding derives both immutable scope and the exact
// persisted preview SHA. Neither is accepted from an owner/API request, so a
// policy cannot be reused for altered mappings within one package.
func (r smartAccountsReconciliationResolver) ResolveTolerancePolicyBinding(ctx context.Context, tenantID, sourceID, packageID, previewID string) (smartaccountsreconciliation.TolerancePolicyBinding, error) {
	if r.tenants == nil || r.delivery == nil || r.executor == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(sourceID) == "" || strings.TrimSpace(packageID) == "" || strings.TrimSpace(previewID) == "" {
		return smartaccountsreconciliation.TolerancePolicyBinding{}, smartaccountsreconciliation.ErrNotFound
	}
	tenantRecord, err := r.tenants.GetTenant(ctx, strings.TrimSpace(tenantID))
	if err != nil {
		return smartaccountsreconciliation.TolerancePolicyBinding{}, err
	}
	status, err := r.delivery.GetStatus(ctx, tenantRecord.SchemaName, strings.TrimSpace(tenantID), strings.TrimSpace(packageID))
	if err != nil || status.Status != importdelivery.StatusStagedReview || status.SourceCompanyID != strings.TrimSpace(sourceID) {
		return smartaccountsreconciliation.TolerancePolicyBinding{}, smartaccountsreconciliation.ErrNotFound
	}
	manifest, err := r.delivery.GetManifest(ctx, tenantRecord.SchemaName, strings.TrimSpace(tenantID), strings.TrimSpace(packageID))
	if err != nil || manifest.SourceCompanyID != strings.TrimSpace(sourceID) || manifest.PackageID != strings.TrimSpace(packageID) {
		return smartaccountsreconciliation.TolerancePolicyBinding{}, smartaccountsreconciliation.ErrNotFound
	}
	scopeSHA, err := digestReconciliationScope(manifest.Scope)
	if err != nil {
		return smartaccountsreconciliation.TolerancePolicyBinding{}, err
	}
	preview, err := r.executor.GetPreview(ctx, tenantRecord.SchemaName, strings.TrimSpace(tenantID), strings.TrimSpace(previewID))
	if err != nil || preview == nil || (preview.Status != smartaccountsexecutor.PlanStatusPreviewReady && preview.Status != smartaccountsexecutor.PlanStatusApplied) || preview.PackageID != strings.TrimSpace(packageID) || preview.SourceCompanyID != strings.TrimSpace(sourceID) || preview.ScopeSHA256 != scopeSHA {
		return smartaccountsreconciliation.TolerancePolicyBinding{}, smartaccountsreconciliation.ErrNotFound
	}
	currencySetSHA, err := digestReconciliationCurrencySet(preview.Journals)
	if err != nil {
		return smartaccountsreconciliation.TolerancePolicyBinding{}, smartaccountsreconciliation.ErrNotFound
	}
	return smartaccountsreconciliation.TolerancePolicyBinding{ScopeSHA256: scopeSHA, PreviewSHA256: preview.PreviewSHA256, CurrencySetSHA256: currencySetSHA, PreviewStatus: preview.Status}, nil
}

func batchSelectsSource(batch smartaccountssync.BrowserOnboardingBatch, sourceID string) bool {
	for _, selected := range batch.SelectedSources {
		if selected.SourceCompanyID == sourceID {
			return true
		}
	}
	return false
}
func batchOutcome(outcomes []smartaccountssync.BrowserOnboardingBatchOutcome, sourceID string) (smartaccountssync.BrowserOnboardingBatchOutcome, bool) {
	for _, outcome := range outcomes {
		if outcome.SourceCompanyID == sourceID {
			return outcome, true
		}
	}
	return smartaccountssync.BrowserOnboardingBatchOutcome{}, false
}
func batchWorkflowSource(sources []smartaccountssync.BrowserBatchSourceWorkflow, sourceID, tenantID string) (smartaccountssync.BrowserBatchSourceWorkflow, bool) {
	for _, source := range sources {
		if source.SourceCompanyID == sourceID && source.TenantID == tenantID {
			return source, true
		}
	}
	return smartaccountssync.BrowserBatchSourceWorkflow{}, false
}

func digestReconciliationScope(scope importdelivery.Scope) (string, error) {
	ids := append([]string(nil), scope.ResourceIDs...)
	sort.Strings(ids)
	canonical := struct {
		Mode           string   `json:"mode"`
		DateFrom       string   `json:"date_from"`
		DateTo         string   `json:"date_to"`
		ResourceIDs    []string `json:"resource_ids"`
		SourceAsOfDate string   `json:"source_as_of_date"`
		CutoffAt       string   `json:"cutoff_at"`
	}{scope.Mode, scope.DateFrom, scope.DateTo, ids, scope.SourceAsOfDate, scope.CutoffAt}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(canonical); err != nil {
		return "", err
	}
	encoded := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// digestReconciliationCurrencySet binds the conservative exact-match policy
// to the current preview's currency metadata without returning a currency list
// or any amounts to the policy API. A changed preview/currency set therefore
// produces a different server-derived candidate digest.
func digestReconciliationCurrencySet(journals []smartaccountsexecutor.PlannedJournal) (string, error) {
	set := make(map[string]struct{}, len(journals))
	for _, journal := range journals {
		currency := strings.TrimSpace(journal.Currency)
		if len(currency) != 3 || currency != strings.ToUpper(currency) || strings.Trim(currency, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
			return "", smartaccountsreconciliation.ErrInvalid
		}
		set[currency] = struct{}{}
	}
	if len(set) == 0 {
		return "", smartaccountsreconciliation.ErrInvalid
	}
	currencies := make([]string, 0, len(set))
	for currency := range set {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	canonical := struct {
		Version    string   `json:"version"`
		Currencies []string `json:"currencies"`
	}{"smartaccounts-reconciliation-currency-set-v1", currencies}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(canonical); err != nil {
		return "", err
	}
	encoded := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

var _ smartaccountsreconciliation.EvidenceResolver = (*smartAccountsReconciliationResolver)(nil)
var _ smartaccountsreconciliation.TolerancePolicyScopeResolver = (*smartAccountsReconciliationResolver)(nil)
