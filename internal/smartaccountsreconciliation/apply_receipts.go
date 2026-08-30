package smartaccountsreconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RecordFirstGLApply implements smartaccountsexecutor.ApplyReceiptRecorder.
// It saves a closed ID-only mapping and posting-identity snapshot with the
// first successful explicit apply. Existing receipts are never overwritten.
func (r *Repository) RecordFirstGLApply(ctx context.Context, input smartaccountsexecutor.ApplyReceiptInput) error {
	db, err := r.requireDB(ctx)
	if err != nil {
		return err
	}
	mappings, mappingSHA, err := canonicalMappings(input.Mappings)
	if err != nil {
		return ErrInvalid
	}
	identities, identitySHA, err := canonicalIdentities(input.Identities)
	if err != nil {
		return ErrInvalid
	}
	if !safeID(input.TenantID) || !safeSource(input.SourceCompanyID) || !safeID(input.PackageID) || !safeID(input.PreviewID) || !safeDigest(input.PreviewSHA256) || !safeDigest(input.TolerancePolicySHA256) || !safeActor(input.ActorID) {
		return ErrInvalid
	}
	now := time.Now().UTC()
	receipt := models.SmartAccountsGLApplyReceiptRecord{ID: uuid.NewString(), TenantID: input.TenantID, SourceCompanyID: input.SourceCompanyID, PackageID: input.PackageID, PreviewID: input.PreviewID, PreviewSHA256: input.PreviewSHA256, MappingSnapshotSHA256: mappingSHA, AppliedIdentitySHA256: identitySHA, TolerancePolicySHA256: input.TolerancePolicySHA256, MappingCount: len(mappings), AppliedIdentityCount: len(identities), FirstAppliedBy: input.ActorID, FirstAppliedAt: now}
	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "source_company_id"}, {Name: "package_id"}, {Name: "preview_sha256"}}, DoNothing: true}).Create(&receipt)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var existing models.SmartAccountsGLApplyReceiptRecord
			if err := tx.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND preview_sha256 = ?", input.TenantID, input.SourceCompanyID, input.PackageID, input.PreviewSHA256).First(&existing).Error; err != nil {
				return err
			}
			if existing.PreviewID != input.PreviewID || existing.MappingSnapshotSHA256 != mappingSHA || existing.AppliedIdentitySHA256 != identitySHA || existing.TolerancePolicySHA256 != input.TolerancePolicySHA256 {
				return ErrConflict
			}
			return nil
		}
		for _, mapping := range mappings {
			if err := tx.Create(&models.SmartAccountsGLApplyReceiptMappingRecord{ReceiptID: receipt.ID, SourceAccountExternalID: mapping.SourceAccountExternalID, TargetAccountID: mapping.TargetAccountID}).Error; err != nil {
				return err
			}
		}
		for _, identity := range identities {
			if err := tx.Create(&models.SmartAccountsGLApplyReceiptIdentityRecord{ReceiptID: receipt.ID, ExternalID: identity.ExternalID, Revision: identity.Revision, ReservationID: identity.ReservationID, JournalID: identity.JournalID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordExactGLReplay records the separate exact-replay attestation. It
// refuses to create a receipt for legacy applied previews, preserving that
// gap as reconciliation evidence rather than silently backfilling history.
func (r *Repository) RecordExactGLReplay(ctx context.Context, tenantID, sourceID, packageID, previewSHA, actorID string) error {
	db, err := r.requireDB(ctx)
	if err != nil {
		return err
	}
	if !safeID(tenantID) || !safeSource(sourceID) || !safeID(packageID) || !safeDigest(previewSHA) || !safeActor(actorID) {
		return ErrInvalid
	}
	var receipt models.SmartAccountsGLApplyReceiptRecord
	if err := db.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND preview_sha256 = ?", tenantID, sourceID, packageID, previewSHA).First(&receipt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if receipt.ExactReplayAt != nil {
		// The first exact-replay attestation is immutable evidence. A later
		// qualified actor repeating the same already-confirmed preview is a
		// financial no-op and must remain idempotent; it must not overwrite the
		// original attestation or turn an exact retry into a conflict.
		return nil
	}
	now := time.Now().UTC()
	result := db.Model(&models.SmartAccountsGLApplyReceiptRecord{}).Where("id = ? AND exact_replay_at IS NULL", receipt.ID).Updates(map[string]any{"exact_replay_by": actorID, "exact_replay_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		// Another exact replay may have won the compare-and-swap between our
		// initial read and update. Re-read the immutable tuple and treat an
		// established replay as the same idempotent operation, irrespective of
		// which qualified actor issued it.
		var current models.SmartAccountsGLApplyReceiptRecord
		if err := db.Where("id = ? AND tenant_id = ? AND source_company_id = ? AND package_id = ? AND preview_sha256 = ?", receipt.ID, tenantID, sourceID, packageID, previewSHA).First(&current).Error; err != nil {
			return ErrConflict
		}
		if current.ExactReplayAt != nil {
			return nil
		}
		return ErrConflict
	}
	return nil
}

func (r *Repository) GetGLApplyReceipt(ctx context.Context, tenantID, sourceID, packageID, previewSHA string) (*GLApplyReceipt, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsGLApplyReceiptRecord
	if err := db.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND preview_sha256 = ?", tenantID, sourceID, packageID, previewSHA).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return receiptFromRecord(record), nil
}

// ListGLApplyReceiptMappings returns only the closed ID-only mapping snapshot
// for a receipt. It is server-only reconciliation plumbing: no names, source
// rows, journal lines, or monetary values are selected.
func (r *Repository) ListGLApplyReceiptMappings(ctx context.Context, tenantID, sourceID, packageID, previewSHA string) ([]smartaccountsexecutor.AppliedMapping, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	var receipt models.SmartAccountsGLApplyReceiptRecord
	if err := db.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND preview_sha256 = ?", tenantID, sourceID, packageID, previewSHA).First(&receipt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var rows []models.SmartAccountsGLApplyReceiptMappingRecord
	if err := db.Where("receipt_id = ?", receipt.ID).Order("source_account_external_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]smartaccountsexecutor.AppliedMapping, 0, len(rows))
	for _, row := range rows {
		result = append(result, smartaccountsexecutor.AppliedMapping{SourceAccountExternalID: row.SourceAccountExternalID, TargetAccountID: row.TargetAccountID})
	}
	return result, nil
}

// ListGLApplyReceiptIdentities returns the append-only ID/revision/journal
// snapshot that was actually marked posted. It deliberately cannot return a
// journal body or any amounts.
func (r *Repository) ListGLApplyReceiptIdentities(ctx context.Context, tenantID, sourceID, packageID, previewSHA string) ([]smartaccountsexecutor.AppliedIdentity, error) {
	db, err := r.requireDB(ctx)
	if err != nil {
		return nil, err
	}
	var receipt models.SmartAccountsGLApplyReceiptRecord
	if err := db.Where("tenant_id = ? AND source_company_id = ? AND package_id = ? AND preview_sha256 = ?", tenantID, sourceID, packageID, previewSHA).First(&receipt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var rows []models.SmartAccountsGLApplyReceiptIdentityRecord
	if err := db.Where("receipt_id = ?", receipt.ID).Order("external_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]smartaccountsexecutor.AppliedIdentity, 0, len(rows))
	for _, row := range rows {
		result = append(result, smartaccountsexecutor.AppliedIdentity{ExternalID: row.ExternalID, Revision: row.Revision, ReservationID: row.ReservationID, JournalID: row.JournalID})
	}
	return result, nil
}

func receiptFromRecord(r models.SmartAccountsGLApplyReceiptRecord) *GLApplyReceipt {
	value := &GLApplyReceipt{TenantID: r.TenantID, SourceCompanyID: r.SourceCompanyID, PackageID: r.PackageID, PreviewID: r.PreviewID, PreviewSHA256: r.PreviewSHA256, FirstAppliedAt: r.FirstAppliedAt.UTC(), FirstAppliedBy: r.FirstAppliedBy, MappingSnapshotSHA256: r.MappingSnapshotSHA256, AppliedIdentitySHA256: r.AppliedIdentitySHA256, TolerancePolicySHA256: r.TolerancePolicySHA256, MappingCount: r.MappingCount, AppliedIdentityCount: r.AppliedIdentityCount}
	if r.ExactReplayAt != nil {
		at := r.ExactReplayAt.UTC()
		value.ExactReplayAt = &at
	}
	if r.ExactReplayBy != nil {
		value.ExactReplayBy = *r.ExactReplayBy
	}
	return value
}

func canonicalMappings(input []smartaccountsexecutor.AppliedMapping) ([]smartaccountsexecutor.AppliedMapping, string, error) {
	values := append([]smartaccountsexecutor.AppliedMapping(nil), input...)
	sort.Slice(values, func(i, j int) bool { return values[i].SourceAccountExternalID < values[j].SourceAccountExternalID })
	if len(values) == 0 {
		return nil, "", ErrInvalid
	}
	for index, value := range values {
		if !safeID(value.SourceAccountExternalID) || !safeID(value.TargetAccountID) || (index > 0 && value.SourceAccountExternalID == values[index-1].SourceAccountExternalID) {
			return nil, "", ErrInvalid
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, "", err
	}
	h := sha256.Sum256(encoded)
	return values, hex.EncodeToString(h[:]), nil
}

func canonicalIdentities(input []smartaccountsexecutor.AppliedIdentity) ([]smartaccountsexecutor.AppliedIdentity, string, error) {
	values := append([]smartaccountsexecutor.AppliedIdentity(nil), input...)
	sort.Slice(values, func(i, j int) bool { return values[i].ExternalID < values[j].ExternalID })
	if len(values) == 0 {
		return nil, "", ErrInvalid
	}
	for index, value := range values {
		if !safeID(value.ExternalID) || !safeDigest(value.Revision) || !safeUUID(value.ReservationID) || !safeUUID(value.JournalID) || (index > 0 && value.ExternalID == values[index-1].ExternalID) {
			return nil, "", ErrInvalid
		}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, "", err
	}
	h := sha256.Sum256(encoded)
	return values, hex.EncodeToString(h[:]), nil
}

var _ smartaccountsexecutor.ApplyReceiptRecorder = (*Repository)(nil)
