package smartaccountsreconciliation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveResolvedTolerancePolicy is repository-only plumbing. Callers must use
// TolerancePolicyService, which derives ScopeSHA256 from a staged manifest;
// it must never be populated from an HTTP request.
func (r *Repository) SaveResolvedTolerancePolicy(ctx context.Context, policy TolerancePolicy) (*TolerancePolicy, bool, error) {
	if policy.AlgorithmVersion != ExactMatchTolerancePolicyVersion || !safeActor(policy.ApprovedBy) || !safeID(policy.TenantID) || !safeSource(policy.SourceCompanyID) || !safeID(policy.PackageID) || !safeDigest(policy.ScopeSHA256) || !safeDigest(policy.PreviewSHA256) || !safeDigest(policy.TolerancePolicySHA256) {
		return nil, false, ErrInvalid
	}
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, false, err
	}
	record := models.SmartAccountsGLTolerancePolicyRecord{
		ID: uuid.NewString(), AlgorithmVersion: policy.AlgorithmVersion, TenantID: strings.TrimSpace(policy.TenantID), SourceCompanyID: strings.TrimSpace(policy.SourceCompanyID),
		PackageID: strings.TrimSpace(policy.PackageID), ScopeSHA256: strings.TrimSpace(policy.ScopeSHA256), PreviewSHA256: strings.TrimSpace(policy.PreviewSHA256),
		TolerancePolicySHA256: strings.TrimSpace(policy.TolerancePolicySHA256), ApprovedBy: strings.TrimSpace(policy.ApprovedBy), ApprovedAt: policy.ApprovedAt.UTC(),
	}
	if record.ApprovedAt.IsZero() {
		record.ApprovedAt = time.Now().UTC()
	}
	result := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "source_company_id"}, {Name: "package_id"}, {Name: "scope_sha256"}, {Name: "preview_sha256"}, {Name: "algorithm_version"}, {Name: "tolerance_policy_sha256"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		return tolerancePolicyFromRecord(record), true, nil
	}
	var existing models.SmartAccountsGLTolerancePolicyRecord
	if err := db.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND scope_sha256 = ? AND preview_sha256 = ? AND algorithm_version = ? AND tolerance_policy_sha256 = ?", record.TenantID, record.SourceCompanyID, record.PackageID, record.ScopeSHA256, record.PreviewSHA256, record.AlgorithmVersion, record.TolerancePolicySHA256).First(&existing).Error; err != nil {
		return nil, false, err
	}
	// A different actor cannot mutate an immutable policy approval, but the
	// exact policy remains available for application. Return it as an
	// idempotent lookup instead of creating competing approvals.
	return tolerancePolicyFromRecord(existing), false, nil
}

// GetResolvedTolerancePolicy obtains an existing immutable policy using only
// its server-derived tenant/source/package/scope/preview/candidate binding.
// It cannot select a policy by an unbound browser-supplied ID.
func (r *Repository) GetResolvedTolerancePolicy(ctx context.Context, policy TolerancePolicy) (*TolerancePolicy, error) {
	if policy.AlgorithmVersion != ExactMatchTolerancePolicyVersion || !safeID(policy.TenantID) || !safeSource(policy.SourceCompanyID) || !safeID(policy.PackageID) || !safeDigest(policy.ScopeSHA256) || !safeDigest(policy.PreviewSHA256) || !safeDigest(policy.TolerancePolicySHA256) {
		return nil, ErrInvalid
	}
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsGLTolerancePolicyRecord
	err = db.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND scope_sha256 = ? AND preview_sha256 = ? AND algorithm_version = ? AND tolerance_policy_sha256 = ?", policy.TenantID, policy.SourceCompanyID, policy.PackageID, policy.ScopeSHA256, policy.PreviewSHA256, policy.AlgorithmVersion, policy.TolerancePolicySHA256).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return tolerancePolicyFromRecord(record), nil
}

// VerifyTolerancePolicy implements smartaccountsexecutor.TolerancePolicyVerifier.
// It requires the server-owned, accountant-approved registry record; a
// 64-character digest supplied by a browser/API caller is never sufficient.
func (r *Repository) VerifyTolerancePolicy(ctx context.Context, binding smartaccountsexecutor.TolerancePolicyBinding) error {
	if !safeID(binding.TenantID) || !safeSource(binding.SourceCompanyID) || !safeID(binding.PackageID) || !safeDigest(binding.ScopeSHA256) || !safeDigest(binding.PreviewSHA256) || !safeDigest(binding.TolerancePolicySHA256) || !safeActor(binding.ActorID) {
		return ErrInvalid
	}
	db, err := r.requireDB(ctx)
	if err != nil {
		return err
	}
	var record models.SmartAccountsGLTolerancePolicyRecord
	err = db.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND scope_sha256 = ? AND preview_sha256 = ? AND algorithm_version = ? AND tolerance_policy_sha256 = ?", binding.TenantID, binding.SourceCompanyID, binding.PackageID, binding.ScopeSHA256, binding.PreviewSHA256, ExactMatchTolerancePolicyVersion, binding.TolerancePolicySHA256).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil || record.AlgorithmVersion != ExactMatchTolerancePolicyVersion || !safeDigest(record.ScopeSHA256) || !safeDigest(record.PreviewSHA256) || !safeActor(record.ApprovedBy) || record.ApprovedBy == binding.ActorID {
		return ErrInvalid
	}
	return nil
}

func tolerancePolicyFromRecord(record models.SmartAccountsGLTolerancePolicyRecord) *TolerancePolicy {
	return &TolerancePolicy{ID: record.ID, AlgorithmVersion: record.AlgorithmVersion, TenantID: record.TenantID, SourceCompanyID: record.SourceCompanyID, PackageID: record.PackageID, ScopeSHA256: record.ScopeSHA256, PreviewSHA256: record.PreviewSHA256, TolerancePolicySHA256: record.TolerancePolicySHA256, ApprovedBy: record.ApprovedBy, ApprovedAt: record.ApprovedAt.UTC()}
}

var _ smartaccountsexecutor.TolerancePolicyVerifier = (*Repository)(nil)
