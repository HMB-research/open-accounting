package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
)

func (r *GORMRepository) browserOnboardingCatalogReceiptsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser onboarding catalog database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_onboarding_catalog_receipts"), nil
}

func (r *GORMRepository) CreateBrowserOnboardingCatalogReceipt(ctx context.Context, receipt BrowserOnboardingCatalogReceipt) error {
	table, err := r.browserOnboardingCatalogReceiptsTable(ctx)
	if err != nil {
		return err
	}
	record, err := browserOnboardingCatalogReceiptToRecord(receipt)
	if err != nil {
		return err
	}
	if err := table.Create(&record).Error; err != nil {
		return fmt.Errorf("create SmartAccounts browser onboarding catalog receipt: %w", err)
	}
	return nil
}

// GetBrowserOnboardingCatalogReceipt returns a server-side record. An empty
// owner is used only by the extension-token handoff path, which immediately
// validates the high-entropy capability digest and returns no record fields.
func (r *GORMRepository) GetBrowserOnboardingCatalogReceipt(ctx context.Context, ownerID, catalogID string) (*BrowserOnboardingCatalogReceipt, error) {
	table, err := r.browserOnboardingCatalogReceiptsTable(ctx)
	if err != nil {
		return nil, err
	}
	query := table.Where("id = ?", strings.TrimSpace(catalogID))
	if ownerID = strings.TrimSpace(ownerID); ownerID != "" {
		query = query.Where("owner_id = ?", ownerID)
	}
	var record models.SmartAccountsBrowserOnboardingCatalogReceiptRecord
	if err := query.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrowserOnboardingCatalogUnauthorized
		}
		return nil, fmt.Errorf("load SmartAccounts browser onboarding catalog receipt: %w", err)
	}
	return browserOnboardingCatalogReceiptFromRecord(record)
}

func (r *GORMRepository) AcceptBrowserOnboardingCatalogReceipt(ctx context.Context, receipt BrowserOnboardingCatalogReceipt) (*BrowserOnboardingCatalogReceipt, bool, error) {
	if !validBrowserOnboardingCatalogAcceptedReceipt(receipt) {
		return nil, false, ErrBrowserOnboardingCatalogInvalid
	}
	table, err := r.browserOnboardingCatalogReceiptsTable(ctx)
	if err != nil {
		return nil, false, err
	}
	companies, err := json.Marshal(sourcesToBrowserOnboardingCatalogCompanies(receipt.Sources))
	if err != nil {
		return nil, false, err
	}
	acceptedAt := receipt.AcceptedAt.UTC()
	observedAt := receipt.ObservedAt.UTC()
	result := table.Where("id = ? AND owner_id = ? AND status = ?", receipt.ID, receipt.OwnerID, BrowserOnboardingCatalogStatusIssued).Updates(map[string]interface{}{
		"status":             BrowserOnboardingCatalogStatusAccepted,
		"catalog_sha256":     receipt.CatalogSHA256,
		"catalog_count":      receipt.CatalogCount,
		"companies":          companies,
		"observed_at":        observedAt,
		"receipt_expires_at": receipt.ReceiptExpiresAt.UTC(),
		"accepted_at":        acceptedAt,
		"updated_at":         receipt.UpdatedAt.UTC(),
	})
	if result.Error != nil {
		return nil, false, fmt.Errorf("accept SmartAccounts browser onboarding catalog receipt: %w", result.Error)
	}
	persisted, err := r.GetBrowserOnboardingCatalogReceipt(ctx, receipt.OwnerID, receipt.ID)
	if err != nil {
		return nil, false, err
	}
	return persisted, result.RowsAffected == 1, nil
}

func browserOnboardingCatalogReceiptToRecord(receipt BrowserOnboardingCatalogReceipt) (models.SmartAccountsBrowserOnboardingCatalogReceiptRecord, error) {
	if !validBrowserOnboardingCatalogIssuedReceipt(receipt) {
		return models.SmartAccountsBrowserOnboardingCatalogReceiptRecord{}, ErrBrowserOnboardingCatalogInvalid
	}
	return models.SmartAccountsBrowserOnboardingCatalogReceiptRecord{ID: receipt.ID, WorkflowID: receipt.WorkflowID, OwnerID: receipt.OwnerID, TokenSHA256: receipt.TokenSHA256, NonceSHA256: receipt.NonceSHA256, SchemaVersion: receipt.SchemaVersion, IntentVersion: receipt.IntentVersion, SourceIDVersion: receipt.SourceIDVersion, DigestAlgorithm: receipt.DigestAlgorithm, Status: receipt.Status, ExpiresAt: receipt.ExpiresAt.UTC(), CreatedAt: receipt.CreatedAt.UTC(), UpdatedAt: receipt.UpdatedAt.UTC()}, nil
}

func browserOnboardingCatalogReceiptFromRecord(record models.SmartAccountsBrowserOnboardingCatalogReceiptRecord) (*BrowserOnboardingCatalogReceipt, error) {
	receipt := BrowserOnboardingCatalogReceipt{ID: record.ID, WorkflowID: record.WorkflowID, OwnerID: record.OwnerID, TokenSHA256: record.TokenSHA256, NonceSHA256: record.NonceSHA256, SchemaVersion: record.SchemaVersion, IntentVersion: record.IntentVersion, SourceIDVersion: record.SourceIDVersion, DigestAlgorithm: record.DigestAlgorithm, Status: record.Status, CatalogSHA256: stringValue(record.CatalogSHA256), ExpiresAt: record.ExpiresAt.UTC(), CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
	if record.CatalogCount != nil {
		receipt.CatalogCount = *record.CatalogCount
	}
	if record.ObservedAt != nil {
		receipt.ObservedAt = record.ObservedAt.UTC()
	}
	if record.ReceiptExpiresAt != nil {
		receipt.ReceiptExpiresAt = record.ReceiptExpiresAt.UTC()
	}
	if record.AcceptedAt != nil {
		receipt.AcceptedAt = record.AcceptedAt.UTC()
	}
	if len(record.Companies) > 0 {
		var companies []BrowserOnboardingCatalogCompany
		if err := json.Unmarshal(record.Companies, &companies); err != nil {
			return nil, ErrBrowserOnboardingCatalogUnavailable
		}
		canonical, ok := canonicalBrowserOnboardingCatalogCompanies(companies)
		if !ok || !sameBrowserOnboardingCatalogCompanies(companies, canonical) {
			return nil, ErrBrowserOnboardingCatalogUnavailable
		}
		receipt.Sources = browserOnboardingCatalogCompaniesToSources(companies)
	}
	if (receipt.Status == BrowserOnboardingCatalogStatusIssued && !validBrowserOnboardingCatalogIssuedReceipt(receipt)) || (receipt.Status == BrowserOnboardingCatalogStatusAccepted && !validBrowserOnboardingCatalogAcceptedReceipt(receipt)) {
		return nil, ErrBrowserOnboardingCatalogUnavailable
	}
	return &receipt, nil
}

func validBrowserOnboardingCatalogIssuedReceipt(receipt BrowserOnboardingCatalogReceipt) bool {
	return receipt.Status == BrowserOnboardingCatalogStatusIssued && validBrowserPairingID(receipt.ID) && validBrowserPairingID(receipt.WorkflowID) && strings.TrimSpace(receipt.OwnerID) != "" && validSHA256(receipt.TokenSHA256) && validSHA256(receipt.NonceSHA256) && receipt.SchemaVersion == BrowserOnboardingCatalogSchemaVersion && receipt.IntentVersion == BrowserOnboardingCatalogIntentVersion && receipt.SourceIDVersion == BrowserOnboardingCatalogSourceIDVersion && receipt.DigestAlgorithm == BrowserOnboardingCatalogDigestAlgorithm && receipt.ExpiresAt.After(receipt.CreatedAt) && receipt.CatalogSHA256 == "" && receipt.CatalogCount == 0 && len(receipt.Sources) == 0 && receipt.ObservedAt.IsZero() && receipt.ReceiptExpiresAt.IsZero() && receipt.AcceptedAt.IsZero()
}

func validBrowserOnboardingCatalogAcceptedReceipt(receipt BrowserOnboardingCatalogReceipt) bool {
	companies := sourcesToBrowserOnboardingCatalogCompanies(receipt.Sources)
	canonical, ok := canonicalBrowserOnboardingCatalogCompanies(companies)
	if !ok || !sameBrowserOnboardingCatalogCompanies(companies, canonical) || receipt.Status != BrowserOnboardingCatalogStatusAccepted || !validBrowserPairingID(receipt.ID) || !validBrowserPairingID(receipt.WorkflowID) || strings.TrimSpace(receipt.OwnerID) == "" || !validSHA256(receipt.TokenSHA256) || !validSHA256(receipt.NonceSHA256) || receipt.SchemaVersion != BrowserOnboardingCatalogSchemaVersion || receipt.IntentVersion != BrowserOnboardingCatalogIntentVersion || receipt.SourceIDVersion != BrowserOnboardingCatalogSourceIDVersion || receipt.DigestAlgorithm != BrowserOnboardingCatalogDigestAlgorithm || !validSHA256(receipt.CatalogSHA256) || receipt.CatalogCount != len(companies) || receipt.CatalogCount > BrowserOnboardingMaxSources || receipt.ObservedAt.IsZero() || receipt.ReceiptExpiresAt.IsZero() || !receipt.ReceiptExpiresAt.After(receipt.ObservedAt) || receipt.ReceiptExpiresAt.Sub(receipt.ObservedAt) > browserOnboardingCatalogReceiptLifetime || receipt.AcceptedAt.IsZero() || !receipt.ExpiresAt.After(receipt.CreatedAt) {
		return false
	}
	encoded, err := jsonMarshalBrowserOnboardingCatalogDigest(receipt.SchemaVersion, companies)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]) == receipt.CatalogSHA256
}
