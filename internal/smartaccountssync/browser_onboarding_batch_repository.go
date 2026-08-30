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

// WithBrowserOnboardingBatchStartLock serializes an exact owner/catalog retry
// across API processes. It row-locks the already-issued immutable catalog
// receipt through first runner progress persistence; it stores no capability,
// token, source row, or selection data.
func (r *GORMRepository) WithBrowserOnboardingBatchStartLock(ctx context.Context, ownerID, catalogReceiptID string, callback func() error) error {
	if r == nil || r.db == nil || callback == nil || strings.TrimSpace(ownerID) == "" || !validBrowserPairingID(strings.TrimSpace(catalogReceiptID)) {
		return ErrBrowserOnboardingBatchUnavailable
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt models.SmartAccountsBrowserOnboardingCatalogReceiptRecord
		// NO KEY UPDATE serializes other onboarding-start locks while remaining
		// compatible with the KEY SHARE foreign-key check made when the callback
		// inserts its child batch row on the repository's normal connection.
		if err := tx.Table("public.smartaccounts_browser_onboarding_catalog_receipts").Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).Where("owner_id = ? AND id = ?", strings.TrimSpace(ownerID), strings.TrimSpace(catalogReceiptID)).First(&receipt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrBrowserOnboardingBatchNotFound
			}
			return fmt.Errorf("lock SmartAccounts browser onboarding catalog receipt: %w", err)
		}
		return callback()
	})
}

func (r *GORMRepository) browserOnboardingBatchesTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser onboarding batch database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_onboarding_batches"), nil
}

func (r *GORMRepository) browserOnboardingBatchOutcomesTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser onboarding batch outcome database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_onboarding_batch_outcomes"), nil
}

func (r *GORMRepository) FindBrowserOnboardingBatchByCatalogReceipt(ctx context.Context, ownerID, catalogReceiptID string) (*BrowserOnboardingBatch, error) {
	table, err := r.browserOnboardingBatchesTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserOnboardingBatchRecord
	err = table.Where("owner_id = ? AND catalog_receipt_id = ?", strings.TrimSpace(ownerID), strings.TrimSpace(catalogReceiptID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load SmartAccounts browser onboarding batch by catalog receipt: %w", err)
	}
	return browserOnboardingBatchFromRecord(record)
}

func (r *GORMRepository) GetBrowserOnboardingBatch(ctx context.Context, ownerID, batchID string) (*BrowserOnboardingBatch, error) {
	table, err := r.browserOnboardingBatchesTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserOnboardingBatchRecord
	err = table.Where("owner_id = ? AND id = ?", strings.TrimSpace(ownerID), strings.TrimSpace(batchID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBrowserOnboardingBatchNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load SmartAccounts browser onboarding batch: %w", err)
	}
	return browserOnboardingBatchFromRecord(record)
}

func (r *GORMRepository) CreateBrowserOnboardingBatch(ctx context.Context, batch BrowserOnboardingBatch) (*BrowserOnboardingBatch, bool, error) {
	table, err := r.browserOnboardingBatchesTable(ctx)
	if err != nil {
		return nil, false, err
	}
	record, err := browserOnboardingBatchToRecord(batch)
	if err != nil {
		return nil, false, err
	}
	result := table.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "owner_id"}, {Name: "catalog_receipt_id"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create SmartAccounts browser onboarding batch: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	persisted, err := browserOnboardingBatchFromRecord(record)
	if err != nil {
		return nil, false, err
	}
	return persisted, true, nil
}

func (r *GORMRepository) SaveBrowserOnboardingBatchProgress(ctx context.Context, ownerID, batchID, status string, outcomes []BrowserOnboardingBatchOutcome, updatedAt time.Time) (*BrowserOnboardingBatch, error) {
	batch, err := r.GetBrowserOnboardingBatch(ctx, ownerID, batchID)
	if err != nil || batch == nil {
		return nil, err
	}
	if !validBrowserOnboardingBatchOutcomes(*batch, outcomes) || browserOnboardingBatchStatus(*batch, outcomes) != strings.TrimSpace(status) {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	updatedAt = updatedAt.UTC()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("public.smartaccounts_browser_onboarding_batches").Where("id = ? AND owner_id = ?", strings.TrimSpace(batchID), strings.TrimSpace(ownerID)).Updates(map[string]interface{}{
			"status": strings.TrimSpace(status), "updated_at": updatedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrBrowserOnboardingBatchNotFound
		}
		for _, outcome := range canonicalBrowserOnboardingBatchOutcomes(outcomes) {
			record := browserOnboardingBatchOutcomeToRecord(batchID, outcome)
			result := tx.Table("public.smartaccounts_browser_onboarding_batch_outcomes").Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "batch_id"}, {Name: "source_company_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"source_company_name": record.SourceCompanyName,
					"tenant_id":           record.TenantID,
					"tenant_name":         record.TenantName,
					"pairing_id":          record.PairingID,
					"status":              record.Status,
					"tenant_created":      record.TenantCreated,
					"tenant_reused":       record.TenantReused,
					"reason_code":         record.ReasonCode,
					"updated_at":          updatedAt,
				}),
			}).Create(&record)
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("save SmartAccounts browser onboarding batch progress: %w", err)
	}
	return r.GetBrowserOnboardingBatch(ctx, ownerID, batchID)
}

func (r *GORMRepository) ListBrowserOnboardingBatchOutcomes(ctx context.Context, ownerID, batchID string) ([]BrowserOnboardingBatchOutcome, error) {
	if _, err := r.GetBrowserOnboardingBatch(ctx, ownerID, batchID); err != nil {
		return nil, err
	}
	table, err := r.browserOnboardingBatchOutcomesTable(ctx)
	if err != nil {
		return nil, err
	}
	var records []models.SmartAccountsBrowserOnboardingBatchOutcomeRecord
	if err := table.Where("batch_id = ?", strings.TrimSpace(batchID)).Order("source_company_id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list SmartAccounts browser onboarding batch outcomes: %w", err)
	}
	outcomes := make([]BrowserOnboardingBatchOutcome, 0, len(records))
	for _, record := range records {
		outcome := browserOnboardingBatchOutcomeFromRecord(record)
		if outcome == nil {
			return nil, ErrBrowserOnboardingBatchUnavailable
		}
		outcomes = append(outcomes, *outcome)
	}
	return outcomes, nil
}

func browserOnboardingBatchToRecord(batch BrowserOnboardingBatch) (models.SmartAccountsBrowserOnboardingBatchRecord, error) {
	if !validBrowserOnboardingBatch(batch) {
		return models.SmartAccountsBrowserOnboardingBatchRecord{}, ErrBrowserOnboardingBatchInvalid
	}
	selectedSources, err := json.Marshal(batch.SelectedSources)
	if err != nil {
		return models.SmartAccountsBrowserOnboardingBatchRecord{}, err
	}
	observedIDs, err := json.Marshal(batch.ObservedSourceIDs)
	if err != nil {
		return models.SmartAccountsBrowserOnboardingBatchRecord{}, err
	}
	return models.SmartAccountsBrowserOnboardingBatchRecord{
		ID: batch.ID, OwnerID: batch.OwnerID, CatalogReceiptID: batch.CatalogReceiptID, RelayObservedAt: batch.RelayObservedAt.UTC(), Mode: batch.Mode, SelectedSources: selectedSources, ObservedSourceIDs: observedIDs,
		ObservedSourcesSHA256: batch.ObservedSourcesSHA256, ManifestSHA256: batch.ManifestSHA256, Status: batch.Status,
		CreatedAt: batch.CreatedAt.UTC(), UpdatedAt: batch.UpdatedAt.UTC(),
	}, nil
}

func browserOnboardingBatchFromRecord(record models.SmartAccountsBrowserOnboardingBatchRecord) (*BrowserOnboardingBatch, error) {
	var selected []BrowserOnboardingSource
	var observed []string
	if err := json.Unmarshal(record.SelectedSources, &selected); err != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	if err := json.Unmarshal(record.ObservedSourceIDs, &observed); err != nil {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	batch := BrowserOnboardingBatch{ID: record.ID, OwnerID: record.OwnerID, CatalogReceiptID: record.CatalogReceiptID, RelayObservedAt: record.RelayObservedAt.UTC(), Mode: record.Mode, SelectedSources: selected, ObservedSourceIDs: observed, ObservedSourcesSHA256: record.ObservedSourcesSHA256, ManifestSHA256: record.ManifestSHA256, Status: record.Status, CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
	if !validBrowserOnboardingBatch(batch) {
		return nil, ErrBrowserOnboardingBatchUnavailable
	}
	return &batch, nil
}

func browserOnboardingBatchOutcomeToRecord(batchID string, outcome BrowserOnboardingBatchOutcome) models.SmartAccountsBrowserOnboardingBatchOutcomeRecord {
	var tenantID, pairingID *string
	if outcome.TenantID != "" {
		value := outcome.TenantID
		tenantID = &value
	}
	if outcome.PairingID != "" {
		value := outcome.PairingID
		pairingID = &value
	}
	return models.SmartAccountsBrowserOnboardingBatchOutcomeRecord{BatchID: strings.TrimSpace(batchID), SourceCompanyID: outcome.SourceCompanyID, SourceCompanyName: outcome.SourceCompanyName, TenantID: tenantID, TenantName: outcome.TenantName, PairingID: pairingID, Status: outcome.Status, TenantCreated: outcome.TenantCreated, TenantReused: outcome.TenantReused, ReasonCode: outcome.ReasonCode, CreatedAt: outcome.CreatedAt.UTC(), UpdatedAt: outcome.UpdatedAt.UTC()}
}

func browserOnboardingBatchOutcomeFromRecord(record models.SmartAccountsBrowserOnboardingBatchOutcomeRecord) *BrowserOnboardingBatchOutcome {
	outcome := BrowserOnboardingBatchOutcome{SourceCompanyID: record.SourceCompanyID, SourceCompanyName: record.SourceCompanyName, TenantID: stringValue(record.TenantID), TenantName: record.TenantName, PairingID: stringValue(record.PairingID), Status: record.Status, TenantCreated: record.TenantCreated, TenantReused: record.TenantReused, ReasonCode: record.ReasonCode, CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
	return &outcome
}

func validBrowserOnboardingBatch(batch BrowserOnboardingBatch) bool {
	if !validBrowserPairingID(batch.ID) || strings.TrimSpace(batch.OwnerID) == "" || !validBrowserPairingID(batch.CatalogReceiptID) || batch.RelayObservedAt.IsZero() || !validSHA256(batch.ObservedSourcesSHA256) || !validSHA256(batch.ManifestSHA256) || batch.CreatedAt.IsZero() || batch.UpdatedAt.IsZero() || !validBrowserOnboardingBatchStatus(batch.Status) {
		return false
	}
	selected, selectedOK := canonicalBrowserOnboardingBatchSources(batch.SelectedSources)
	observed, observedOK := canonicalBrowserOnboardingBatchSourceIDs(batch.ObservedSourceIDs)
	if !selectedOK || !observedOK || !sameBrowserOnboardingSources(selected, batch.SelectedSources) || !sameBrowserOnboardingSourceIDs(observed, batch.ObservedSourceIDs) {
		return false
	}
	selectedIDs := make([]string, 0, len(selected))
	for _, source := range selected {
		selectedIDs = append(selectedIDs, source.SourceCompanyID)
	}
	if (batch.Mode == BrowserOnboardingBatchModeAll && !sameBrowserOnboardingSourceIDs(selectedIDs, observed)) || (batch.Mode == BrowserOnboardingBatchModeSelected && len(selectedIDs) >= len(observed)) || (batch.Mode != BrowserOnboardingBatchModeSelected && batch.Mode != BrowserOnboardingBatchModeAll) {
		return false
	}
	manifestDigest, err := browserOnboardingBatchDigest(struct {
		Version           string                    `json:"version"`
		CatalogReceiptID  string                    `json:"catalog_receipt_id"`
		CatalogSHA256     string                    `json:"catalog_sha256"`
		RelayObservedAt   time.Time                 `json:"relay_observed_at"`
		Mode              string                    `json:"mode"`
		SelectedSources   []BrowserOnboardingSource `json:"selected_sources"`
		ObservedSourceIDs []string                  `json:"observed_source_ids"`
	}{Version: browserOnboardingBatchManifestVersion, CatalogReceiptID: batch.CatalogReceiptID, CatalogSHA256: batch.ObservedSourcesSHA256, RelayObservedAt: batch.RelayObservedAt.UTC(), Mode: batch.Mode, SelectedSources: selected, ObservedSourceIDs: observed})
	return err == nil && manifestDigest == batch.ManifestSHA256
}

func validBrowserOnboardingBatchStatus(status string) bool {
	switch status {
	case BrowserOnboardingBatchPending, BrowserOnboardingBatchReview, BrowserOnboardingBatchReady, BrowserOnboardingBatchComplete:
		return true
	default:
		return false
	}
}
