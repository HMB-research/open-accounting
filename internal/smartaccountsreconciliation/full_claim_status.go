package smartaccountsreconciliation

import (
	"context"
	"sort"
	"strings"

	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
)

const (
	fullClaimBlockerCurrentPass      = "selected_sources_not_current_pass"
	fullClaimBlockerTombstones       = "unresolved_tombstone_gap"
	fullClaimBlockerSourceCoverage   = "selected_source_coverage_gap"
	fullClaimBlockerDomainEvidence   = "selected_domain_evidence_gap"
	fullClaimBlockerMatrixFilter     = "matrix_filter_contract_gap"
	fullClaimBlockerMatrixPageOnly   = "matrix_page_only_gap"
	fullClaimBlockerMatrixReview     = "matrix_review_required"
	fullClaimBlockerMatrixUnconsumed = "matrix_unconsumed"
	fullClaimBlockerMatrixMissingAPI = "matrix_missing_api_endpoint"
	fullClaimBlockerMatrixExport     = "matrix_export_contract_gap"
	fullClaimBlockerMatrixUnknown    = "matrix_unknown_contract_gap"
	fullClaimBlockerMatrixSchema     = "matrix_schema_gap"
	fullClaimBlockerMatrixCoverage   = "matrix_coverage_gap"
)

// FullClaimStatus reads the original immutable selected/all batch members and
// their current reconciliation state, then combines that result with the
// immutable per-domain source-selection plan. It never creates an evaluation,
// reconciliation proof, approval, posting, or import.
//
// The plan retains every route as an auditable alternative, but selects one
// primary route per business domain. An unreviewed Brave fallback therefore
// cannot block an API-primary domain. The inverse is also deliberately true:
// a current reconciliation PASS cannot make the product report FULL until the
// exact selected route for every domain has durable tenant/source/package/
// scope-bound source, schema, completeness, reconciliation, tombstone, and
// accountant evidence. The returned result is count/code only and cannot
// identify a company or expose source content.
func (s *Service) FullClaimStatus(ctx context.Context, ownerID, batchID string) (*FullClaimStatus, error) {
	if s == nil || s.store == nil || s.resolver == nil || !safeActor(ownerID) || !safeID(batchID) {
		return nil, ErrInvalid
	}
	batchID = strings.TrimSpace(batchID)
	ownerID = strings.TrimSpace(ownerID)
	bindings, err := s.resolver.ListBindings(ctx, ownerID, batchID)
	if err != nil || len(bindings) == 0 {
		return nil, ErrNotFound
	}

	plan := s.currentFullClaimPlan()
	status := &FullClaimStatus{Status: FullClaimStatusNotEligible, SelectedCount: len(bindings)}
	seenSources := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		if binding.BatchID != batchID || !safeSource(binding.SourceCompanyID) {
			return nil, ErrInvalid
		}
		if _, duplicate := seenSources[binding.SourceCompanyID]; duplicate {
			return nil, ErrInvalid
		}
		seenSources[binding.SourceCompanyID] = struct{}{}
		if !binding.Paired || !safeID(binding.TenantID) {
			status.CurrentPassGapCount++
			continue
		}

		evaluation, getErr := s.GetForOwner(ctx, ownerID, batchID, binding.SourceCompanyID)
		if getErr != nil || !fullClaimCurrentPass(evaluation, binding) {
			status.CurrentPassGapCount++
			if evaluation != nil {
				if evaluation.GLTombstoneUnresolved > 0 || evaluation.ReferenceTombstoneUnresolved > 0 {
					status.TombstoneGapSourceCount++
				}
				if evaluation.ClaimKind != "full" || evaluation.ExpectedCoverageState != "full" {
					status.SourceCoverageGapCount++
				}
			}
			continue
		}
		status.CurrentPassCount++
		evidenceBinding := FullClaimEvidenceBinding{BatchID: binding.BatchID, TenantID: binding.TenantID, SourceCompanyID: binding.SourceCompanyID, PackageID: evaluation.PackageID, ScopeSHA256: evaluation.ScopeSHA256, ReconciliationEvidenceSHA256: evaluation.EvidenceSHA256}
		evidence, evidenceErr := s.store.ListFullClaimDomainEvidence(ctx, evidenceBinding, smartaccountssync.FullClaimCoveragePlanVersion)
		domainEligibility := EvaluateFullClaimDomainEvidence(plan, evidenceBinding, evidence, evaluation.GLTombstoneUnresolved+evaluation.ReferenceTombstoneUnresolved)
		if evidenceErr != nil || !domainEligibility.FullClaimEligible {
			status.DomainEvidenceGapSourceCount++
		}
	}

	matrix := summarizeFullClaimDomainPlan(plan)
	matrix.review += status.DomainEvidenceGapSourceCount
	matrix.blockers = matrix.filter + matrix.pageOnly + matrix.review + matrix.unconsumed + matrix.missingEndpoint + matrix.exportContract + matrix.unknown
	status.MatrixBlockerCount = matrix.blockers
	status.MatrixFilterContractGapCount = matrix.filter
	status.MatrixPageOnlyGapCount = matrix.pageOnly
	status.MatrixReviewRequiredCount = matrix.review
	status.MatrixUnconsumedCount = matrix.unconsumed
	status.MatrixMissingEndpointCount = matrix.missingEndpoint
	status.MatrixSchemaGapCount = matrix.filter
	status.MatrixCoverageGapCount = matrix.filter + matrix.pageOnly + matrix.exportContract
	status.BlockingCodes = fullClaimBlockingCodes(*status, matrix.unknown)
	status.FullClaimEligible = len(status.BlockingCodes) == 0 && status.CurrentPassCount == status.SelectedCount
	if status.FullClaimEligible {
		status.Status = FullClaimStatusEligible
	}
	return status, nil
}

type fullClaimMatrixSummary struct {
	blockers        int
	filter          int
	pageOnly        int
	review          int
	unconsumed      int
	missingEndpoint int
	exportContract  int
	unknown         int
}

// summarizeFullClaimDomainPlan projects only selected routes into the existing
// count-only status contract. Dynamic selected routes are counted from actual
// scope-bound evidence per selected source by FullClaimStatus; a static
// capability or synthetic fixture is never treated as live proof. Browser/API
// alternatives are excluded from this count by design.
func summarizeFullClaimDomainPlan(plan []smartaccountssync.FullClaimDomainPlanEntry) fullClaimMatrixSummary {
	var summary fullClaimMatrixSummary
	for _, entry := range plan {
		switch entry.Selected.Disposition {
		case smartaccountssync.FullClaimDispositionFilterRequired:
			summary.filter++
		case smartaccountssync.FullClaimDispositionPageOnlyRequired:
			summary.pageOnly++
		case smartaccountssync.FullClaimDispositionReviewRequired:
			summary.review++
		case smartaccountssync.FullClaimDispositionUnconsumed:
			summary.unconsumed++
		case smartaccountssync.FullClaimDispositionMissingAPIEndpoint:
			summary.missingEndpoint++
		case smartaccountssync.FullClaimDispositionExportContractRequired:
			summary.exportContract++
		case smartaccountssync.FullClaimDispositionGLApplyGated,
			smartaccountssync.FullClaimDispositionReferenceApplyGated,
			smartaccountssync.FullClaimDispositionArchiveOnly,
			smartaccountssync.FullClaimDispositionResolved:
			// Dynamic evidence is added by FullClaimStatus per source.
		default:
			summary.unknown++
		}
	}
	summary.blockers = summary.filter + summary.pageOnly + summary.review + summary.unconsumed + summary.missingEndpoint + summary.exportContract + summary.unknown
	return summary
}

func fullClaimBlockingCodes(status FullClaimStatus, unknownMatrixRows int) []string {
	codes := make([]string, 0, 12)
	if status.CurrentPassGapCount > 0 {
		codes = append(codes, fullClaimBlockerCurrentPass)
	}
	if status.TombstoneGapSourceCount > 0 {
		codes = append(codes, fullClaimBlockerTombstones)
	}
	if status.SourceCoverageGapCount > 0 {
		codes = append(codes, fullClaimBlockerSourceCoverage)
	}
	if status.DomainEvidenceGapSourceCount > 0 {
		codes = append(codes, fullClaimBlockerDomainEvidence)
	}
	if status.MatrixFilterContractGapCount > 0 {
		codes = append(codes, fullClaimBlockerMatrixFilter, fullClaimBlockerMatrixSchema)
	}
	if status.MatrixPageOnlyGapCount > 0 {
		codes = append(codes, fullClaimBlockerMatrixPageOnly)
	}
	if status.MatrixReviewRequiredCount > 0 {
		codes = append(codes, fullClaimBlockerMatrixReview)
	}
	if status.MatrixUnconsumedCount > 0 {
		codes = append(codes, fullClaimBlockerMatrixUnconsumed)
	}
	if status.MatrixMissingEndpointCount > 0 {
		codes = append(codes, fullClaimBlockerMatrixMissingAPI)
	}
	if status.MatrixCoverageGapCount > status.MatrixFilterContractGapCount+status.MatrixPageOnlyGapCount {
		codes = append(codes, fullClaimBlockerMatrixExport)
	}
	if unknownMatrixRows > 0 {
		codes = append(codes, fullClaimBlockerMatrixUnknown)
	}
	if status.MatrixCoverageGapCount > 0 {
		codes = append(codes, fullClaimBlockerMatrixCoverage)
	}
	sort.Strings(codes)
	return codes
}

func fullClaimCurrentPass(evaluation *Evaluation, binding SourceBinding) bool {
	return evaluation != nil && evaluation.BatchID == binding.BatchID && evaluation.SourceCompanyID == binding.SourceCompanyID && evaluation.TenantID == binding.TenantID && evaluation.Status == StatusPass && evaluation.ClaimKind == "full" && evaluation.ExpectedCoverageState == "full" && evaluation.GLRevisionUnresolved == 0 && evaluation.GLTombstoneUnresolved == 0 && evaluation.ReferenceRevisionUnresolved == 0 && evaluation.ReferenceTombstoneUnresolved == 0 && len(evaluation.Blockers) == 0
}
