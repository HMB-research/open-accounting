package smartaccountsreconciliation

import (
	"context"
	"sort"
	"strings"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

// SaveFullClaimDomainEvidence and ListFullClaimDomainEvidence are deliberately
// separate from the general reconciliation methods so a caller cannot turn a
// reconciliation PASS into a full-sync claim without the complete selected
// domain ledger.
type FullClaimEvidenceStore interface {
	SaveFullClaimDomainEvidence(context.Context, FullClaimDomainEvidence) (*FullClaimDomainEvidence, bool, error)
	ListFullClaimDomainEvidence(context.Context, FullClaimEvidenceBinding, string) ([]FullClaimDomainEvidence, error)
}

// RecordFullClaimDomainEvidence persists one server-derived, immutable domain
// receipt. There is intentionally no HTTP handler for this method: trusted
// server-side coverage workers must derive the receipt from source/schema/
// completeness/reconciliation/tombstone/accountant controls already held in
// Open Accounting. A caller assertion alone cannot create a usable record.
func (s *Service) RecordFullClaimDomainEvidence(ctx context.Context, evidence FullClaimDomainEvidence) (*FullClaimDomainEvidence, bool, error) {
	if s == nil || s.store == nil {
		return nil, false, ErrInvalid
	}
	if strings.TrimSpace(evidence.ID) == "" {
		evidence.ID = uuid.NewString()
	}
	if evidence.RecordedAt.IsZero() {
		evidence.RecordedAt = s.currentTime()
	}
	if !validFullClaimDomainEvidence(evidence) || !matchesCurrentFullClaimPlan(s.currentFullClaimPlan(), evidence) {
		return nil, false, ErrInvalid
	}
	return s.store.SaveFullClaimDomainEvidence(ctx, evidence)
}

// EvaluateFullClaimDomainEvidence applies the immutable selected-domain plan
// to evidence for one exact tenant/source/package/scope. It fails closed for
// a missing, duplicate, cross-boundary, stale-plan, or incomplete receipt.
// It returns only fixed route blocker codes, never the binding or receipt.
func EvaluateFullClaimDomainEvidence(plan []smartaccountssync.FullClaimDomainPlanEntry, binding FullClaimEvidenceBinding, evidence []FullClaimDomainEvidence, unresolvedTombstones int) smartaccountssync.FullClaimEligibility {
	blockers := make([]string, 0, 2)
	if !validFullClaimEvidenceBinding(binding) {
		blockers = append(blockers, "evidence:invalid_binding")
	}

	domainEvidence := make([]smartaccountssync.FullClaimDomainEvidence, 0, len(evidence))
	for _, item := range evidence {
		if !validFullClaimDomainEvidence(item) || !sameFullClaimEvidenceBinding(item.FullClaimEvidenceBinding, binding) {
			blockers = append(blockers, "evidence:invalid_binding")
			continue
		}
		domainEvidence = append(domainEvidence, smartaccountssync.FullClaimDomainEvidence{
			PlanVersion:             item.PlanVersion,
			DomainID:                item.DomainID,
			Source:                  item.Source,
			ResourceID:              item.ResourceID,
			ContractVersion:         item.ContractVersion,
			LiveSourceValidated:     item.LiveSourceValidated,
			SchemaValidated:         item.SchemaValidated,
			CompletenessValidated:   item.CompletenessValidated,
			ReconciliationValidated: item.ReconciliationValidated,
			TombstonesResolved:      item.TombstonesResolved,
			AccountantAttested:      item.AccountantAttested,
		})
	}

	result := smartaccountssync.AssessFullClaimDomainPlanEligibility(plan, domainEvidence, unresolvedTombstones)
	blockers = append(blockers, result.BlockingResources...)
	sort.Strings(blockers)
	blockers = compactFullClaimEvidenceBlockers(blockers)
	return smartaccountssync.FullClaimEligibility{FullClaimEligible: len(blockers) == 0, BlockingResources: blockers}
}

func (s *Service) currentFullClaimPlan() []smartaccountssync.FullClaimDomainPlanEntry {
	if s != nil && s.fullClaimPlan != nil {
		return s.fullClaimPlan()
	}
	return smartaccountssync.CurrentFullClaimDomainPlan()
}

func validFullClaimEvidenceBinding(binding FullClaimEvidenceBinding) bool {
	return safeID(binding.BatchID) && safeID(binding.TenantID) && safeSource(binding.SourceCompanyID) && safeID(binding.PackageID) && safeDigest(binding.ScopeSHA256) && safeDigest(binding.ReconciliationEvidenceSHA256)
}

func sameFullClaimEvidenceBinding(left, right FullClaimEvidenceBinding) bool {
	return left.BatchID == right.BatchID && left.TenantID == right.TenantID && left.SourceCompanyID == right.SourceCompanyID && left.PackageID == right.PackageID && left.ScopeSHA256 == right.ScopeSHA256 && left.ReconciliationEvidenceSHA256 == right.ReconciliationEvidenceSHA256
}

func validFullClaimDomainEvidence(item FullClaimDomainEvidence) bool {
	return safeUUID(item.ID) && validFullClaimEvidenceBinding(item.FullClaimEvidenceBinding) && item.PlanVersion == smartaccountssync.FullClaimCoveragePlanVersion && safeID(item.DomainID) && safeID(item.Source) && safeID(item.ResourceID) && safeID(item.ContractVersion) && safeDigest(item.EvidenceSHA256) && !item.RecordedAt.IsZero() && item.LiveSourceValidated && item.SchemaValidated && item.CompletenessValidated && item.ReconciliationValidated && item.TombstonesResolved && item.AccountantAttested
}

func matchesCurrentFullClaimPlan(plan []smartaccountssync.FullClaimDomainPlanEntry, evidence FullClaimDomainEvidence) bool {
	for _, entry := range plan {
		if entry.PlanVersion == evidence.PlanVersion && entry.DomainID == evidence.DomainID && entry.Selected.Source == evidence.Source && entry.Selected.ResourceID == evidence.ResourceID && entry.Selected.ContractVersion == evidence.ContractVersion {
			return true
		}
	}
	return false
}

func compactFullClaimEvidenceBlockers(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func fullClaimDomainEvidenceToRecord(value FullClaimDomainEvidence) (models.SmartAccountsFullClaimDomainEvidenceRecord, error) {
	if !validFullClaimDomainEvidence(value) {
		return models.SmartAccountsFullClaimDomainEvidenceRecord{}, ErrInvalid
	}
	return models.SmartAccountsFullClaimDomainEvidenceRecord{
		ID: value.ID, BatchID: value.BatchID, TenantID: value.TenantID, SourceCompanyID: value.SourceCompanyID, PackageID: value.PackageID, ScopeSHA256: value.ScopeSHA256, ReconciliationEvidenceSHA256: value.ReconciliationEvidenceSHA256,
		PlanVersion: value.PlanVersion, DomainID: value.DomainID, Source: value.Source, ResourceID: value.ResourceID, ContractVersion: value.ContractVersion,
		LiveSourceValidated: value.LiveSourceValidated, SchemaValidated: value.SchemaValidated, CompletenessValidated: value.CompletenessValidated, ReconciliationValidated: value.ReconciliationValidated, TombstonesResolved: value.TombstonesResolved, AccountantAttested: value.AccountantAttested,
		EvidenceSHA256: value.EvidenceSHA256, RecordedAt: value.RecordedAt.UTC(),
	}, nil
}

func fullClaimDomainEvidenceFromRecord(record models.SmartAccountsFullClaimDomainEvidenceRecord) (*FullClaimDomainEvidence, error) {
	value := &FullClaimDomainEvidence{
		ID:                       record.ID,
		FullClaimEvidenceBinding: FullClaimEvidenceBinding{BatchID: record.BatchID, TenantID: record.TenantID, SourceCompanyID: record.SourceCompanyID, PackageID: record.PackageID, ScopeSHA256: record.ScopeSHA256, ReconciliationEvidenceSHA256: record.ReconciliationEvidenceSHA256},
		PlanVersion:              record.PlanVersion, DomainID: record.DomainID, Source: record.Source, ResourceID: record.ResourceID, ContractVersion: record.ContractVersion,
		LiveSourceValidated: record.LiveSourceValidated, SchemaValidated: record.SchemaValidated, CompletenessValidated: record.CompletenessValidated, ReconciliationValidated: record.ReconciliationValidated, TombstonesResolved: record.TombstonesResolved, AccountantAttested: record.AccountantAttested,
		EvidenceSHA256: record.EvidenceSHA256, RecordedAt: record.RecordedAt.UTC(),
	}
	if !validFullClaimDomainEvidence(*value) {
		return nil, ErrInvalid
	}
	return value, nil
}

func sameFullClaimDomainEvidence(left, right FullClaimDomainEvidence) bool {
	return sameFullClaimEvidenceBinding(left.FullClaimEvidenceBinding, right.FullClaimEvidenceBinding) && left.PlanVersion == right.PlanVersion && left.DomainID == right.DomainID && left.Source == right.Source && left.ResourceID == right.ResourceID && left.ContractVersion == right.ContractVersion && left.LiveSourceValidated == right.LiveSourceValidated && left.SchemaValidated == right.SchemaValidated && left.CompletenessValidated == right.CompletenessValidated && left.ReconciliationValidated == right.ReconciliationValidated && left.TombstonesResolved == right.TombstonesResolved && left.AccountantAttested == right.AccountantAttested && left.EvidenceSHA256 == right.EvidenceSHA256
}

func (r *Repository) SaveFullClaimDomainEvidence(ctx context.Context, evidence FullClaimDomainEvidence) (*FullClaimDomainEvidence, bool, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, false, err
	}
	record, err := fullClaimDomainEvidenceToRecord(evidence)
	if err != nil {
		return nil, false, err
	}
	result := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "batch_id"}, {Name: "tenant_id"}, {Name: "source_company_id"}, {Name: "package_id"}, {Name: "scope_sha256"}, {Name: "reconciliation_evidence_sha256"}, {Name: "plan_version"}, {Name: "domain_id"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		value, convertErr := fullClaimDomainEvidenceFromRecord(record)
		return value, true, convertErr
	}

	// A unique binding/domain conflict is idempotent only when every immutable
	// proof flag and the opaque evidence digest match. A changed receipt must
	// use a new package/scope (or future plan version), never overwrite history.
	var existing models.SmartAccountsFullClaimDomainEvidenceRecord
	lookup := db.Where("batch_id = ? AND tenant_id = ? AND source_company_id = ? AND package_id = ? AND scope_sha256 = ? AND reconciliation_evidence_sha256 = ? AND plan_version = ? AND domain_id = ?", evidence.BatchID, evidence.TenantID, evidence.SourceCompanyID, evidence.PackageID, evidence.ScopeSHA256, evidence.ReconciliationEvidenceSHA256, evidence.PlanVersion, evidence.DomainID).First(&existing)
	if lookup.Error != nil {
		return nil, false, lookup.Error
	}
	value, convertErr := fullClaimDomainEvidenceFromRecord(existing)
	if convertErr != nil {
		return nil, false, convertErr
	}
	if !sameFullClaimDomainEvidence(*value, evidence) {
		return nil, false, ErrConflict
	}
	return value, false, nil
}

func (r *Repository) ListFullClaimDomainEvidence(ctx context.Context, binding FullClaimEvidenceBinding, planVersion string) ([]FullClaimDomainEvidence, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	if !validFullClaimEvidenceBinding(binding) || planVersion != smartaccountssync.FullClaimCoveragePlanVersion {
		return nil, ErrInvalid
	}
	var records []models.SmartAccountsFullClaimDomainEvidenceRecord
	if err := db.Where("batch_id = ? AND tenant_id = ? AND source_company_id = ? AND package_id = ? AND scope_sha256 = ? AND reconciliation_evidence_sha256 = ? AND plan_version = ?", binding.BatchID, binding.TenantID, binding.SourceCompanyID, binding.PackageID, binding.ScopeSHA256, binding.ReconciliationEvidenceSHA256, planVersion).Order("domain_id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	values := make([]FullClaimDomainEvidence, 0, len(records))
	for _, record := range records {
		value, convertErr := fullClaimDomainEvidenceFromRecord(record)
		if convertErr != nil || !sameFullClaimEvidenceBinding(value.FullClaimEvidenceBinding, binding) {
			return nil, ErrInvalid
		}
		values = append(values, *value)
	}
	return values, nil
}

var _ FullClaimEvidenceStore = (*Repository)(nil)
