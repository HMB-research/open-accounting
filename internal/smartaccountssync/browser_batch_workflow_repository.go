package smartaccountssync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GORMRepository) browserBatchWorkflowsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser batch workflow database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_batch_workflows"), nil
}

func (r *GORMRepository) browserBatchSourceWorkflowsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser batch source workflow database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_batch_source_workflows"), nil
}

func (r *GORMRepository) CreateBrowserBatchWorkflow(ctx context.Context, workflow BrowserBatchWorkflow, sources []BrowserBatchSourceWorkflow) (*BrowserBatchWorkflow, bool, error) {
	if !validBrowserBatchWorkflow(workflow) || !validBrowserBatchSourceSet(sources, workflow.BatchID) {
		return nil, false, ErrBrowserBatchWorkflowInvalid
	}
	record, err := browserBatchWorkflowToRecord(workflow)
	if err != nil {
		return nil, false, err
	}
	sourceRecords := make([]models.SmartAccountsBrowserBatchSourceWorkflowRecord, 0, len(sources))
	for _, source := range sources {
		converted, convertErr := browserBatchSourceWorkflowToRecord(source)
		if convertErr != nil {
			return nil, false, convertErr
		}
		sourceRecords = append(sourceRecords, converted)
	}
	created := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("public.smartaccounts_browser_batch_workflows").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "batch_id"}}, DoNothing: true}).Create(&record)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if result := tx.Table("public.smartaccounts_browser_batch_source_workflows").Create(&sourceRecords); result.Error != nil {
			return result.Error
		}
		created = true
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("create SmartAccounts browser batch workflow: %w", err)
	}
	if !created {
		existing, getErr := r.GetBrowserBatchWorkflow(ctx, workflow.OwnerID, workflow.BatchID)
		return existing, false, getErr
	}
	persisted, err := browserBatchWorkflowFromRecord(record)
	return persisted, true, err
}

func (r *GORMRepository) GetBrowserBatchWorkflow(ctx context.Context, ownerID, batchID string) (*BrowserBatchWorkflow, error) {
	table, err := r.browserBatchWorkflowsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserBatchWorkflowRecord
	err = table.Where("owner_id = ? AND batch_id = ?", strings.TrimSpace(ownerID), strings.TrimSpace(batchID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load SmartAccounts browser batch workflow: %w", err)
	}
	return browserBatchWorkflowFromRecord(record)
}

func (r *GORMRepository) ListBrowserBatchSourceWorkflows(ctx context.Context, ownerID, batchID string) ([]BrowserBatchSourceWorkflow, error) {
	if _, err := r.GetBrowserBatchWorkflow(ctx, ownerID, batchID); err != nil {
		return nil, err
	}
	table, err := r.browserBatchSourceWorkflowsTable(ctx)
	if err != nil {
		return nil, err
	}
	var records []models.SmartAccountsBrowserBatchSourceWorkflowRecord
	if err := table.Where("batch_id = ?", strings.TrimSpace(batchID)).Order("ordinal ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list SmartAccounts browser batch source workflows: %w", err)
	}
	result := make([]BrowserBatchSourceWorkflow, 0, len(records))
	for _, record := range records {
		source, err := browserBatchSourceWorkflowFromRecord(record)
		if err != nil {
			return nil, err
		}
		result = append(result, *source)
	}
	return result, nil
}

func (r *GORMRepository) CompareAndSwapBrowserBatchSource(ctx context.Context, ownerID, batchID, sourceID, expectedPhase string, expectedGeneration int64, expectedLeaseID string, next BrowserBatchSourceWorkflow) (*BrowserBatchSourceWorkflow, bool, error) {
	if _, err := r.GetBrowserBatchWorkflow(ctx, ownerID, batchID); err != nil {
		return nil, false, err
	}
	if next.BatchID != strings.TrimSpace(batchID) || next.SourceCompanyID != strings.TrimSpace(sourceID) || !validBrowserBatchSourceWorkflow(next) || next.PhaseGeneration != expectedGeneration+1 {
		return nil, false, ErrBrowserBatchWorkflowInvalid
	}
	record, err := browserBatchSourceWorkflowToRecord(next)
	if err != nil {
		return nil, false, err
	}
	table, err := r.browserBatchSourceWorkflowsTable(ctx)
	if err != nil {
		return nil, false, err
	}
	query := table.Where("batch_id = ? AND source_company_id = ? AND phase = ? AND phase_generation = ?", strings.TrimSpace(batchID), strings.TrimSpace(sourceID), expectedPhase, expectedGeneration)
	if expectedLeaseID == "" {
		query = query.Where("lease_id IS NULL")
	} else {
		query = query.Where("lease_id = ?", expectedLeaseID)
	}
	result := query.Updates(browserBatchSourceWorkflowUpdateMap(record))
	if result.Error != nil {
		return nil, false, fmt.Errorf("compare-and-swap SmartAccounts browser batch source workflow: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	persisted, err := browserBatchSourceWorkflowFromRecord(record)
	return persisted, true, err
}

// AcquireNextBrowserBatchLease serializes one source phase across the entire
// batch. It locks the batch workflow and all source checkpoints so two callers
// cannot claim different sources concurrently.
func (r *GORMRepository) AcquireNextBrowserBatchLease(ctx context.Context, ownerID, batchID, requiredPhase, leaseID string, now, expiresAt time.Time) (*BrowserBatchSourceWorkflow, bool, error) {
	if !validBrowserBatchPhase(requiredPhase) || !validBrowserPairingID(leaseID) || !expiresAt.After(now) {
		return nil, false, ErrBrowserBatchWorkflowInvalid
	}
	var acquired *BrowserBatchSourceWorkflow
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var workflowRecord models.SmartAccountsBrowserBatchWorkflowRecord
		if err := tx.Table("public.smartaccounts_browser_batch_workflows").Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND batch_id = ?", strings.TrimSpace(ownerID), strings.TrimSpace(batchID)).First(&workflowRecord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBrowserBatchWorkflowNotFound
			}
			return err
		}
		workflow, err := browserBatchWorkflowFromRecord(workflowRecord)
		if err != nil {
			return err
		}
		if requiredPhase == BrowserBatchPhaseTransferConfirmationRequired && workflow.TransferManifestSHA256 == "" {
			return ErrBrowserBatchWorkflowNotReady
		}
		var records []models.SmartAccountsBrowserBatchSourceWorkflowRecord
		if err := tx.Table("public.smartaccounts_browser_batch_source_workflows").Clauses(clause.Locking{Strength: "UPDATE"}).Where("batch_id = ?", strings.TrimSpace(batchID)).Order("ordinal ASC").Find(&records).Error; err != nil {
			return err
		}
		for _, record := range records {
			source, err := browserBatchSourceWorkflowFromRecord(record)
			if err != nil {
				return err
			}
			if (source.Phase == BrowserBatchPhaseDiscoveryRunning || source.Phase == BrowserBatchPhaseCaptureRunning) && source.LeaseExpiresAt != nil && source.LeaseExpiresAt.After(now) {
				return nil
			}
		}
		for _, record := range records {
			source, err := browserBatchSourceWorkflowFromRecord(record)
			if err != nil {
				return err
			}
			if source.Phase != requiredPhase || source.LeaseID != "" {
				continue
			}
			next := *source
			if requiredPhase == BrowserBatchPhaseDiscoveryRequired {
				next.Phase = BrowserBatchPhaseDiscoveryRunning
			} else if requiredPhase == BrowserBatchPhaseTransferConfirmationRequired {
				next.Phase = BrowserBatchPhaseCaptureRunning
				if next.CaptureRunID == "" {
					next.CaptureRunID = leaseID
				}
			} else {
				return ErrBrowserBatchWorkflowInvalid
			}
			next.PhaseGeneration++
			next.AttemptCount++
			next.LeaseID = leaseID
			expires := expiresAt.UTC()
			next.LeaseExpiresAt = &expires
			next.UpdatedAt = now.UTC()
			nextRecord, err := browserBatchSourceWorkflowToRecord(next)
			if err != nil {
				return err
			}
			result := tx.Table("public.smartaccounts_browser_batch_source_workflows").Where("batch_id = ? AND source_company_id = ? AND phase = ? AND phase_generation = ? AND lease_id IS NULL", next.BatchID, next.SourceCompanyID, requiredPhase, source.PhaseGeneration).Updates(browserBatchSourceWorkflowUpdateMap(nextRecord))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrBrowserBatchWorkflowConflict
			}
			acquired = &next
			return nil
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return acquired, acquired != nil, nil
}

// RecoverExpiredBrowserBatchLeases is deliberately a control-plane recovery:
// it clears only an elapsed concurrency lease and returns the source to its
// exact prior work requirement. It never creates a new relay authorization.
func (r *GORMRepository) RecoverExpiredBrowserBatchLeases(ctx context.Context, ownerID, batchID string, now time.Time) (int, error) {
	recovered := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.lockBrowserBatchWorkflow(tx, ownerID, batchID); err != nil {
			return err
		}
		sources, err := r.lockBrowserBatchSources(tx, batchID)
		if err != nil {
			return err
		}
		for _, source := range sources {
			if (source.Phase != BrowserBatchPhaseDiscoveryRunning && source.Phase != BrowserBatchPhaseCaptureRunning) || source.LeaseID == "" || source.LeaseExpiresAt == nil || source.LeaseExpiresAt.After(now) {
				continue
			}
			next := source
			if source.Phase == BrowserBatchPhaseDiscoveryRunning {
				next.Phase = BrowserBatchPhaseDiscoveryRequired
			} else {
				next.Phase = BrowserBatchPhaseTransferConfirmationRequired
			}
			next.PhaseGeneration++
			next.LeaseID, next.LeaseExpiresAt = "", nil
			next.ReasonCode = "lease_expired"
			next.UpdatedAt = now.UTC()
			nextRecord, err := browserBatchSourceWorkflowToRecord(next)
			if err != nil {
				return err
			}
			result := tx.Table("public.smartaccounts_browser_batch_source_workflows").Where("batch_id = ? AND source_company_id = ? AND phase = ? AND phase_generation = ? AND lease_id = ? AND lease_expires_at <= ?", source.BatchID, source.SourceCompanyID, source.Phase, source.PhaseGeneration, source.LeaseID, now.UTC()).Updates(browserBatchSourceWorkflowUpdateMap(nextRecord))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrBrowserBatchWorkflowConflict
			}
			recovered++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return recovered, nil
}

func (r *GORMRepository) OpenBrowserBatchTransferConfirmation(ctx context.Context, ownerID, batchID, expectedSchemaSHA256 string, updatedAt time.Time) (*BrowserBatchWorkflow, bool, error) {
	if !validSHA256(expectedSchemaSHA256) {
		return nil, false, ErrBrowserBatchWorkflowInvalid
	}
	var output *BrowserBatchWorkflow
	changed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		workflow, err := r.lockBrowserBatchWorkflow(tx, ownerID, batchID)
		if err != nil {
			return err
		}
		if workflow.TransferManifestSHA256 != "" {
			output = workflow
			return nil
		}
		sources, err := r.lockBrowserBatchSources(tx, batchID)
		if err != nil {
			return err
		}
		actual, ok := browserBatchSchemaReadinessSHA256(sources)
		if !ok || actual != expectedSchemaSHA256 {
			return ErrBrowserBatchWorkflowNotReady
		}
		alreadyOpen := true
		for _, source := range sources {
			if source.Phase != BrowserBatchPhaseTransferConfirmationRequired || source.LeaseID != "" {
				alreadyOpen = false
				break
			}
		}
		if alreadyOpen {
			output = workflow
			return nil
		}
		for _, source := range sources {
			next := source
			next.Phase = BrowserBatchPhaseTransferConfirmationRequired
			next.PhaseGeneration++
			next.UpdatedAt = updatedAt.UTC()
			nextRecord, err := browserBatchSourceWorkflowToRecord(next)
			if err != nil {
				return err
			}
			result := tx.Table("public.smartaccounts_browser_batch_source_workflows").Where("batch_id = ? AND source_company_id = ? AND phase = ? AND phase_generation = ? AND lease_id IS NULL", source.BatchID, source.SourceCompanyID, BrowserBatchPhaseSchemaApproved, source.PhaseGeneration).Updates(browserBatchSourceWorkflowUpdateMap(nextRecord))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrBrowserBatchWorkflowConflict
			}
		}
		output, changed = workflow, true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return output, changed, nil
}

func (r *GORMRepository) ConfirmBrowserBatchTransfer(ctx context.Context, ownerID, batchID, manifestSHA256 string, scope BrowserBatchTransferScope, confirmedAt time.Time) (*BrowserBatchWorkflow, bool, error) {
	if !validSHA256(manifestSHA256) || !validBrowserBatchTransferScope(scope) {
		return nil, false, ErrBrowserBatchWorkflowInvalid
	}
	encodedScope, err := json.Marshal(scope)
	if err != nil {
		return nil, false, err
	}
	var output *BrowserBatchWorkflow
	changed := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		workflow, lockErr := r.lockBrowserBatchWorkflow(tx, ownerID, batchID)
		if lockErr != nil {
			return lockErr
		}
		if workflow.TransferManifestSHA256 != "" {
			if workflow.TransferManifestSHA256 != manifestSHA256 {
				return ErrBrowserBatchWorkflowConflict
			}
			output = workflow
			return nil
		}
		sources, lockErr := r.lockBrowserBatchSources(tx, batchID)
		if lockErr != nil {
			return lockErr
		}
		for _, source := range sources {
			if source.Phase != BrowserBatchPhaseTransferConfirmationRequired || source.LeaseID != "" {
				return ErrBrowserBatchWorkflowNotReady
			}
		}
		at := confirmedAt.UTC()
		result := tx.Table("public.smartaccounts_browser_batch_workflows").Where("owner_id = ? AND batch_id = ? AND transfer_manifest_sha256 IS NULL", strings.TrimSpace(ownerID), strings.TrimSpace(batchID)).Updates(map[string]interface{}{"transfer_manifest_sha256": manifestSHA256, "transfer_scope": encodedScope, "transfer_confirmed_at": at, "updated_at": at})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrBrowserBatchWorkflowConflict
		}
		workflow.TransferManifestSHA256, workflow.TransferScope, workflow.TransferConfirmedAt, workflow.UpdatedAt = manifestSHA256, scope, &at, at
		output, changed = workflow, true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return output, changed, nil
}

func (r *GORMRepository) lockBrowserBatchWorkflow(tx *gorm.DB, ownerID, batchID string) (*BrowserBatchWorkflow, error) {
	var record models.SmartAccountsBrowserBatchWorkflowRecord
	err := tx.Table("public.smartaccounts_browser_batch_workflows").Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_id = ? AND batch_id = ?", strings.TrimSpace(ownerID), strings.TrimSpace(batchID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}
	return browserBatchWorkflowFromRecord(record)
}

func (r *GORMRepository) lockBrowserBatchSources(tx *gorm.DB, batchID string) ([]BrowserBatchSourceWorkflow, error) {
	var records []models.SmartAccountsBrowserBatchSourceWorkflowRecord
	if err := tx.Table("public.smartaccounts_browser_batch_source_workflows").Clauses(clause.Locking{Strength: "UPDATE"}).Where("batch_id = ?", strings.TrimSpace(batchID)).Order("ordinal ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	sources := make([]BrowserBatchSourceWorkflow, 0, len(records))
	for _, record := range records {
		source, err := browserBatchSourceWorkflowFromRecord(record)
		if err != nil {
			return nil, err
		}
		sources = append(sources, *source)
	}
	if !validBrowserBatchSourceSet(sources, strings.TrimSpace(batchID)) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return sources, nil
}

func browserBatchWorkflowToRecord(workflow BrowserBatchWorkflow) (models.SmartAccountsBrowserBatchWorkflowRecord, error) {
	if !validBrowserBatchWorkflow(workflow) {
		return models.SmartAccountsBrowserBatchWorkflowRecord{}, ErrBrowserBatchWorkflowInvalid
	}
	var scope json.RawMessage
	if workflow.TransferManifestSHA256 != "" {
		encoded, err := json.Marshal(workflow.TransferScope)
		if err != nil {
			return models.SmartAccountsBrowserBatchWorkflowRecord{}, err
		}
		scope = encoded
	}
	return models.SmartAccountsBrowserBatchWorkflowRecord{BatchID: workflow.BatchID, OwnerID: workflow.OwnerID, SchemaVersion: workflow.SchemaVersion, HistoryFrom: workflow.HistoryFrom, HeaderProbeConsentConfirmed: workflow.HeaderProbeConsentConfirmed, PreparatoryManifestSHA256: workflow.PreparatoryManifestSHA256, PreparatoryConsentedAt: workflow.PreparatoryConsentedAt.UTC(), TransferManifestSHA256: workflow.TransferManifestSHA256, TransferScope: scope, TransferConfirmedAt: copyTime(workflow.TransferConfirmedAt), CreatedAt: workflow.CreatedAt.UTC(), UpdatedAt: workflow.UpdatedAt.UTC()}, nil
}

func browserBatchWorkflowFromRecord(record models.SmartAccountsBrowserBatchWorkflowRecord) (*BrowserBatchWorkflow, error) {
	workflow := BrowserBatchWorkflow{BatchID: record.BatchID, OwnerID: record.OwnerID, SchemaVersion: record.SchemaVersion, HistoryFrom: record.HistoryFrom, HeaderProbeConsentConfirmed: record.HeaderProbeConsentConfirmed, PreparatoryManifestSHA256: record.PreparatoryManifestSHA256, PreparatoryConsentedAt: record.PreparatoryConsentedAt.UTC(), TransferManifestSHA256: record.TransferManifestSHA256, TransferConfirmedAt: copyTime(record.TransferConfirmedAt), CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
	if len(record.TransferScope) != 0 {
		if err := json.Unmarshal(record.TransferScope, &workflow.TransferScope); err != nil {
			return nil, ErrBrowserBatchWorkflowUnavailable
		}
	}
	if !validBrowserBatchWorkflow(workflow) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return &workflow, nil
}

func browserBatchSourceWorkflowToRecord(source BrowserBatchSourceWorkflow) (models.SmartAccountsBrowserBatchSourceWorkflowRecord, error) {
	if !validBrowserBatchSourceWorkflow(source) {
		return models.SmartAccountsBrowserBatchSourceWorkflowRecord{}, ErrBrowserBatchWorkflowInvalid
	}
	return models.SmartAccountsBrowserBatchSourceWorkflowRecord{BatchID: source.BatchID, SourceCompanyID: source.SourceCompanyID, TenantID: source.TenantID, Ordinal: source.Ordinal, Phase: source.Phase, PhaseGeneration: source.PhaseGeneration, AttemptCount: source.AttemptCount, LeaseID: stringPointer(source.LeaseID), LeaseExpiresAt: copyTime(source.LeaseExpiresAt), CaptureRunID: stringPointer(source.CaptureRunID), DiscoveryID: stringPointer(source.DiscoveryID), DiscoveryContractSHA256: source.DiscoveryContractSHA256, DiscoveryReceiptSHA256: source.DiscoveryReceiptSHA256, SchemaID: source.SchemaID, SchemaApprovalSHA256: source.SchemaApprovalSHA256, PackageID: source.PackageID, PackageSHA256: source.PackageSHA256, PreviewID: stringPointer(source.PreviewID), PreviewSHA256: source.PreviewSHA256, ReasonCode: source.ReasonCode, CreatedAt: source.CreatedAt.UTC(), UpdatedAt: source.UpdatedAt.UTC()}, nil
}

func browserBatchSourceWorkflowFromRecord(record models.SmartAccountsBrowserBatchSourceWorkflowRecord) (*BrowserBatchSourceWorkflow, error) {
	source := BrowserBatchSourceWorkflow{BatchID: record.BatchID, SourceCompanyID: record.SourceCompanyID, TenantID: record.TenantID, Ordinal: record.Ordinal, Phase: record.Phase, PhaseGeneration: record.PhaseGeneration, AttemptCount: record.AttemptCount, LeaseID: browserBatchStringValue(record.LeaseID), LeaseExpiresAt: copyTime(record.LeaseExpiresAt), CaptureRunID: browserBatchStringValue(record.CaptureRunID), DiscoveryID: browserBatchStringValue(record.DiscoveryID), DiscoveryContractSHA256: record.DiscoveryContractSHA256, DiscoveryReceiptSHA256: record.DiscoveryReceiptSHA256, SchemaID: record.SchemaID, SchemaApprovalSHA256: record.SchemaApprovalSHA256, PackageID: record.PackageID, PackageSHA256: record.PackageSHA256, PreviewID: browserBatchStringValue(record.PreviewID), PreviewSHA256: record.PreviewSHA256, ReasonCode: record.ReasonCode, CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
	if !validBrowserBatchSourceWorkflow(source) {
		return nil, ErrBrowserBatchWorkflowUnavailable
	}
	return &source, nil
}

func browserBatchSourceWorkflowUpdateMap(record models.SmartAccountsBrowserBatchSourceWorkflowRecord) map[string]interface{} {
	return map[string]interface{}{"tenant_id": record.TenantID, "ordinal": record.Ordinal, "phase": record.Phase, "phase_generation": record.PhaseGeneration, "attempt_count": record.AttemptCount, "lease_id": record.LeaseID, "lease_expires_at": record.LeaseExpiresAt, "capture_run_id": record.CaptureRunID, "discovery_id": record.DiscoveryID, "discovery_contract_sha256": record.DiscoveryContractSHA256, "discovery_receipt_sha256": record.DiscoveryReceiptSHA256, "schema_id": record.SchemaID, "schema_approval_sha256": record.SchemaApprovalSHA256, "package_id": record.PackageID, "package_sha256": record.PackageSHA256, "preview_id": record.PreviewID, "preview_sha256": record.PreviewSHA256, "reason_code": record.ReasonCode, "updated_at": record.UpdatedAt}
}

func copyTime(input *time.Time) *time.Time {
	if input == nil {
		return nil
	}
	value := input.UTC()
	return &value
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func browserBatchStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
