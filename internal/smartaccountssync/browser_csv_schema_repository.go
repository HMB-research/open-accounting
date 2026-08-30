package smartaccountssync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GORMRepository) browserCSVSchemaApprovalsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser CSV schema approval database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_csv_schema_approvals"), nil
}

func (r *GORMRepository) FindOrCreateBrowserCSVSchemaApproval(ctx context.Context, approval BrowserCSVSchemaApproval) (*BrowserCSVSchemaApproval, bool, error) {
	if !validBrowserCSVSchemaApproval(approval) {
		return nil, false, ErrBrowserCSVSchemaApprovalInvalid
	}
	table, err := r.browserCSVSchemaApprovalsTable(ctx)
	if err != nil {
		return nil, false, err
	}
	record := browserCSVSchemaApprovalToRecord(approval)
	result := table.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "discovery_id"}, {Name: "resource_id"}},
		DoNothing: true,
	}).Create(&record)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create SmartAccounts browser CSV schema approval: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		created, err := browserCSVSchemaApprovalFromRecord(&record)
		return created, true, err
	}
	existing, err := r.GetBrowserCSVSchemaApproval(ctx, approval.TenantID, approval.DiscoveryID, approval.ResourceID)
	if err != nil {
		return nil, false, err
	}
	return existing, false, nil
}

func (r *GORMRepository) GetBrowserCSVSchemaApproval(ctx context.Context, tenantID, discoveryID, resourceID string) (*BrowserCSVSchemaApproval, error) {
	table, err := r.browserCSVSchemaApprovalsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserCSVSchemaApprovalRecord
	err = table.Where("tenant_id = ? AND discovery_id = ? AND resource_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(discoveryID), strings.TrimSpace(resourceID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBrowserCSVSchemaApprovalNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load SmartAccounts browser CSV schema approval: %w", err)
	}
	return browserCSVSchemaApprovalFromRecord(&record)
}

func (r *GORMRepository) MarkBrowserCSVSchemaApprovalRegistered(ctx context.Context, approval BrowserCSVSchemaApproval, observedAt time.Time) (*BrowserCSVSchemaApproval, error) {
	if !validBrowserCSVSchemaApproval(approval) || approval.Status != BrowserCSVSchemaStatusRegistered {
		return nil, ErrBrowserCSVSchemaApprovalInvalid
	}
	table, err := r.browserCSVSchemaApprovalsTable(ctx)
	if err != nil {
		return nil, err
	}
	approvalDigest := strings.TrimSpace(approval.ApprovalSHA256)
	result := table.Where("tenant_id = ? AND discovery_id = ? AND resource_id = ? AND schema_id = ? AND review_audit_id = ?", strings.TrimSpace(approval.TenantID), strings.TrimSpace(approval.DiscoveryID), strings.TrimSpace(approval.ResourceID), strings.TrimSpace(approval.SchemaID), strings.TrimSpace(approval.Review.AuditID)).Updates(map[string]interface{}{
		"status":          BrowserCSVSchemaStatusRegistered,
		"approval_sha256": approvalDigest,
		"updated_at":      observedAt.UTC(),
	})
	if result.Error != nil {
		return nil, fmt.Errorf("record SmartAccounts browser CSV schema approval: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, ErrBrowserCSVSchemaApprovalConflict
	}
	return r.GetBrowserCSVSchemaApproval(ctx, approval.TenantID, approval.DiscoveryID, approval.ResourceID)
}

func browserCSVSchemaApprovalToRecord(approval BrowserCSVSchemaApproval) models.SmartAccountsBrowserCSVSchemaApprovalRecord {
	var digest *string
	if value := strings.TrimSpace(approval.ApprovalSHA256); value != "" {
		digest = &value
	}
	return models.SmartAccountsBrowserCSVSchemaApprovalRecord{
		TenantID: strings.TrimSpace(approval.TenantID), DiscoveryID: strings.TrimSpace(approval.DiscoveryID),
		ResourceID: strings.TrimSpace(approval.ResourceID), SchemaID: strings.TrimSpace(approval.SchemaID),
		ReviewVersion: approval.Review.Version, Confirmed: approval.Review.Confirmed,
		ReviewedAt: approval.Review.ReviewedAt.UTC(), ReviewAuditID: approval.Review.AuditID,
		ReviewedBy: strings.TrimSpace(approval.ReviewedBy), Status: approval.Status, ApprovalSHA256: digest,
		CreatedAt: approval.CreatedAt.UTC(), UpdatedAt: approval.UpdatedAt.UTC(),
	}
}

func browserCSVSchemaApprovalFromRecord(record *models.SmartAccountsBrowserCSVSchemaApprovalRecord) (*BrowserCSVSchemaApproval, error) {
	if record == nil {
		return nil, ErrBrowserCSVSchemaApprovalNotFound
	}
	approval := &BrowserCSVSchemaApproval{
		TenantID: record.TenantID, DiscoveryID: record.DiscoveryID, ResourceID: record.ResourceID, SchemaID: record.SchemaID,
		Review:     BrowserCSVSchemaReview{Version: record.ReviewVersion, Confirmed: record.Confirmed, ReviewedAt: record.ReviewedAt.UTC(), AuditID: record.ReviewAuditID},
		ReviewedBy: record.ReviewedBy, Status: record.Status, ApprovalSHA256: stringValue(record.ApprovalSHA256),
		CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC(),
	}
	if !validBrowserCSVSchemaApproval(*approval) {
		return nil, ErrBrowserCSVSchemaApprovalUnavailable
	}
	return approval, nil
}

var _ BrowserCSVSchemaApprovalStore = (*GORMRepository)(nil)
