package smartaccountsreconciliation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

var newGormDBFromPool = database.NewGormDBFromPool

func NewRepository(pool *pgxpool.Pool) *Repository {
	if pool == nil {
		return &Repository{}
	}
	db, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create SmartAccounts reconciliation repository: %w", err))
	}
	return &Repository{db: db}
}

func NewGORMRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) requireDB(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts reconciliation database is not configured")
	}
	return r.db.WithContext(ctx), nil
}

func (r *Repository) SaveEvaluation(ctx context.Context, evaluation Evaluation) (*Evaluation, bool, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, false, err
	}
	record, err := evaluationToRecord(evaluation)
	if err != nil {
		return nil, false, err
	}
	result := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "batch_id"}, {Name: "source_company_id"}, {Name: "tenant_id"}, {Name: "evidence_sha256"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		return evaluationFromRecord(record)
	}
	var existing models.SmartAccountsReconciliationEvaluationRecord
	if err := db.Where("batch_id = ? AND source_company_id = ? AND tenant_id = ? AND evidence_sha256 = ?", evaluation.BatchID, evaluation.SourceCompanyID, evaluation.TenantID, evaluation.EvidenceSHA256).First(&existing).Error; err != nil {
		return nil, false, err
	}
	value, _, err := evaluationFromRecord(existing)
	return value, false, err
}

func (r *Repository) GetLatestEvaluation(ctx context.Context, batchID, sourceID, tenantID string) (*Evaluation, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsReconciliationEvaluationRecord
	if err := db.Where("batch_id = ? AND source_company_id = ? AND tenant_id = ?", batchID, sourceID, tenantID).Order("created_at DESC, id DESC").First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	value, _, err := evaluationFromRecord(record)
	return value, err
}

func (r *Repository) GetEvaluation(ctx context.Context, id string) (*Evaluation, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsReconciliationEvaluationRecord
	if err := db.Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	value, _, err := evaluationFromRecord(record)
	return value, err
}

func (r *Repository) CreateApproval(ctx context.Context, approval Approval) (*Approval, bool, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, false, err
	}
	record := models.SmartAccountsReconciliationApprovalRecord{ID: approval.ID, EvaluationID: approval.EvaluationID, EvidenceSHA256: approval.EvidenceSHA256, ToleranceSHA256: approval.ToleranceSHA256, ApprovedBy: approval.ApprovedBy, ApprovedAt: approval.ApprovedAt.UTC()}
	result := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "evaluation_id"}, {Name: "evidence_sha256"}, {Name: "tolerance_sha256"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		return approvalFromRecord(record), true, nil
	}
	var existing models.SmartAccountsReconciliationApprovalRecord
	if err := db.Where("evaluation_id = ? AND evidence_sha256 = ? AND tolerance_sha256 = ?", approval.EvaluationID, approval.EvidenceSHA256, approval.ToleranceSHA256).First(&existing).Error; err != nil {
		return nil, false, err
	}
	return approvalFromRecord(existing), false, nil
}

func (r *Repository) GetApproval(ctx context.Context, evaluationID string) (*Approval, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsReconciliationApprovalRecord
	if err := db.Where("evaluation_id = ?", evaluationID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return approvalFromRecord(record), nil
}

func (r *Repository) MarkEvaluationPassed(ctx context.Context, id, evidenceSHA, toleranceSHA string, at time.Time) (*Evaluation, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	result := db.Model(&models.SmartAccountsReconciliationEvaluationRecord{}).Where("id = ? AND evidence_sha256 = ? AND tolerance_sha256 = ? AND status = ?", id, evidenceSHA, toleranceSHA, StatusReadyForAccountant).Updates(map[string]any{"status": StatusPass, "accountant_approved_at": at.UTC(), "updated_at": at.UTC()})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		value, getErr := r.GetEvaluation(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		if value.Status == StatusPass && value.EvidenceSHA256 == evidenceSHA && value.ToleranceSHA256 == toleranceSHA {
			return value, nil
		}
		return nil, ErrConflict
	}
	return r.GetEvaluation(ctx, id)
}

func evaluationToRecord(e Evaluation) (models.SmartAccountsReconciliationEvaluationRecord, error) {
	if !validBlockers(e.Blockers) {
		return models.SmartAccountsReconciliationEvaluationRecord{}, ErrInvalid
	}
	blockers, err := json.Marshal(e.Blockers)
	if err != nil {
		return models.SmartAccountsReconciliationEvaluationRecord{}, err
	}
	var asOf, cutoff *time.Time
	if e.SourceAsOfDate != "" {
		v, parseErr := time.Parse(time.DateOnly, e.SourceAsOfDate)
		if parseErr != nil {
			return models.SmartAccountsReconciliationEvaluationRecord{}, parseErr
		}
		asOf = &v
	}
	if e.CutoffAt != "" {
		v, parseErr := time.Parse(time.RFC3339, e.CutoffAt)
		if parseErr != nil {
			return models.SmartAccountsReconciliationEvaluationRecord{}, parseErr
		}
		cutoff = &v
	}
	glID, refID, refSHA := optionalString(e.GLPreviewID), optionalString(e.ReferencePreviewID), optionalString(e.ReferencePreviewSHA256)
	return models.SmartAccountsReconciliationEvaluationRecord{ID: e.ID, BatchID: e.BatchID, SourceCompanyID: e.SourceCompanyID, TenantID: e.TenantID, PackageID: e.PackageID, ManifestSHA256: e.ManifestSHA256, RecordsSHA256: e.RecordsSHA256, ScopeSHA256: e.ScopeSHA256, SourceAsOfDate: asOf, CutoffAt: cutoff, GLPreviewID: glID, GLPreviewSHA256: e.GLPreviewSHA256, GLState: e.GLState, GLMappingSnapshotSHA256: e.GLMappingSnapshotSHA256, GLAppliedIdentitySHA256: e.GLAppliedIdentitySHA256, ReferencePreviewID: refID, ReferencePreviewSHA256: refSHA, ReferenceState: e.ReferenceState, ProofID: e.ProofID, ProofSHA256: e.ProofSHA256, ClaimSHA256: e.ClaimSHA256, CoverageSHA256: e.CoverageSHA256, ClaimKind: e.ClaimKind, ExpectedCoverageState: e.ExpectedCoverageState, ToleranceSHA256: e.ToleranceSHA256, VarianceWithinPolicy: e.VarianceWithinPolicy, GLRevisionUnresolved: e.GLRevisionUnresolved, GLTombstoneUnresolved: e.GLTombstoneUnresolved, ReferenceRevisionUnresolved: e.ReferenceRevisionUnresolved, ReferenceTombstoneUnresolved: e.ReferenceTombstoneUnresolved, Blockers: blockers, EvidenceSHA256: e.EvidenceSHA256, EvidenceSubmittedBy: e.EvidenceSubmittedBy, GLFirstAppliedBy: e.GLFirstAppliedBy, GLExactReplayBy: e.GLExactReplayBy, Status: e.Status, AccountantApprovedAt: e.AccountantApprovedAt, CreatedAt: e.CreatedAt.UTC(), UpdatedAt: e.UpdatedAt.UTC()}, nil
}

func evaluationFromRecord(r models.SmartAccountsReconciliationEvaluationRecord) (*Evaluation, bool, error) {
	var blockers []string
	if err := json.Unmarshal(r.Blockers, &blockers); err != nil {
		return nil, false, err
	}
	if !validBlockers(blockers) {
		return nil, false, ErrInvalid
	}
	e := &Evaluation{ID: r.ID, BatchID: r.BatchID, SourceCompanyID: r.SourceCompanyID, TenantID: r.TenantID, PackageID: r.PackageID, ManifestSHA256: r.ManifestSHA256, RecordsSHA256: r.RecordsSHA256, ScopeSHA256: r.ScopeSHA256, GLPreviewSHA256: r.GLPreviewSHA256, GLState: r.GLState, GLMappingSnapshotSHA256: r.GLMappingSnapshotSHA256, GLAppliedIdentitySHA256: r.GLAppliedIdentitySHA256, ReferenceState: r.ReferenceState, ProofID: r.ProofID, ProofSHA256: r.ProofSHA256, ClaimSHA256: r.ClaimSHA256, CoverageSHA256: r.CoverageSHA256, ClaimKind: r.ClaimKind, ExpectedCoverageState: r.ExpectedCoverageState, ToleranceSHA256: r.ToleranceSHA256, VarianceWithinPolicy: r.VarianceWithinPolicy, GLRevisionUnresolved: r.GLRevisionUnresolved, GLTombstoneUnresolved: r.GLTombstoneUnresolved, ReferenceRevisionUnresolved: r.ReferenceRevisionUnresolved, ReferenceTombstoneUnresolved: r.ReferenceTombstoneUnresolved, Blockers: blockers, EvidenceSHA256: r.EvidenceSHA256, EvidenceSubmittedBy: r.EvidenceSubmittedBy, GLFirstAppliedBy: r.GLFirstAppliedBy, GLExactReplayBy: r.GLExactReplayBy, Status: r.Status, AccountantApprovedAt: r.AccountantApprovedAt, CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC()}
	if r.SourceAsOfDate != nil {
		e.SourceAsOfDate = r.SourceAsOfDate.UTC().Format(time.DateOnly)
	}
	if r.CutoffAt != nil {
		e.CutoffAt = r.CutoffAt.UTC().Format(time.RFC3339)
	}
	if r.ReferencePreviewID != nil {
		e.ReferencePreviewID = *r.ReferencePreviewID
	}
	if r.GLPreviewID != nil {
		e.GLPreviewID = *r.GLPreviewID
	}
	if r.ReferencePreviewSHA256 != nil {
		e.ReferencePreviewSHA256 = *r.ReferencePreviewSHA256
	}
	return e, true, nil
}

func approvalFromRecord(r models.SmartAccountsReconciliationApprovalRecord) *Approval {
	return &Approval{ID: r.ID, EvaluationID: r.EvaluationID, EvidenceSHA256: r.EvidenceSHA256, ToleranceSHA256: r.ToleranceSHA256, ApprovedBy: r.ApprovedBy, ApprovedAt: r.ApprovedAt.UTC()}
}
func optionalString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}
