package smartaccountssync

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

// GORMRepository persists the non-financial, public-schema bridge control.
type GORMRepository struct {
	db *gorm.DB
}

var newGormDBFromPool = database.NewGormDBFromPool

func NewRepository(pool *pgxpool.Pool) *GORMRepository {
	if pool == nil {
		return &GORMRepository{}
	}
	db, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create SmartAccounts sync repository: %w", err))
	}
	return NewGORMRepository(db)
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) controlsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts sync database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_sync_controls"), nil
}

func (r *GORMRepository) browserPairingsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser pairing database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_pairings"), nil
}

func (r *GORMRepository) browserOnboardingTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser onboarding database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_onboarding_bindings"), nil
}
func (r *GORMRepository) browserCaptureAuthorizationsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser capture database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_capture_authorizations"), nil
}

func (r *GORMRepository) browserMasterDetailAuthorizationsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser master-detail database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_master_detail_authorizations"), nil
}

func (r *GORMRepository) browserCommercialDetailAuthorizationsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser commercial-detail database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_commercial_detail_authorizations"), nil
}

func (r *GORMRepository) browserCaptureWorkflowsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser capture workflow database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_capture_workflows"), nil
}

func (r *GORMRepository) browserDiscoveryAuthorizationsTable(ctx context.Context) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts browser discovery database is not configured")
	}
	return r.db.WithContext(ctx).Table("public.smartaccounts_browser_discovery_authorizations"), nil
}

func (r *GORMRepository) CreateBrowserDiscoveryAuthorization(ctx context.Context, authorization BrowserDiscoveryAuthorization) error {
	table, err := r.browserDiscoveryAuthorizationsTable(ctx)
	if err != nil {
		return err
	}
	if !validBrowserDiscoveryAuthorization(authorization) {
		return ErrBrowserDiscoveryUnauthorized
	}
	resourceIDs, err := json.Marshal(canonicalBrowserDiscoveryIdentifiers(authorization.ResourceIDs))
	if err != nil {
		return ErrBrowserDiscoveryUnavailable
	}
	record := models.SmartAccountsBrowserDiscoveryAuthorizationRecord{
		DiscoveryID:                  strings.TrimSpace(authorization.DiscoveryID),
		TenantID:                     strings.TrimSpace(authorization.TenantID),
		SourceCompanyID:              strings.TrimSpace(authorization.SourceCompanyID),
		ManifestVersion:              authorization.ManifestVersion,
		ContractVersion:              authorization.ContractVersion,
		ResourceIDs:                  resourceIDs,
		MetadataOnlyConsentConfirmed: authorization.MetadataOnlyConsentConfirmed,
		HeaderProbeConsentConfirmed:  authorization.HeaderProbeConsentConfirmed,
		ConsentedAt:                  authorization.ConsentedAt.UTC(),
		CreatedBy:                    strings.TrimSpace(authorization.CreatedBy),
		ExpiresAt:                    authorization.ExpiresAt.UTC(),
		CreatedAt:                    authorization.CreatedAt.UTC(),
		UpdatedAt:                    authorization.CreatedAt.UTC(),
	}
	if err := table.Create(&record).Error; err != nil {
		return fmt.Errorf("create SmartAccounts browser discovery authorization: %w", err)
	}
	return nil
}

func (r *GORMRepository) GetBrowserDiscoveryAuthorization(ctx context.Context, tenantID, discoveryID string) (*BrowserDiscoveryAuthorization, error) {
	table, err := r.browserDiscoveryAuthorizationsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserDiscoveryAuthorizationRecord
	if err := table.Where("tenant_id = ? AND discovery_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(discoveryID)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrowserDiscoveryUnauthorized
		}
		return nil, fmt.Errorf("load SmartAccounts browser discovery authorization: %w", err)
	}
	var resourceIDs []string
	if err := json.Unmarshal(record.ResourceIDs, &resourceIDs); err != nil {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	authorization := &BrowserDiscoveryAuthorization{
		DiscoveryID:                  record.DiscoveryID,
		TenantID:                     record.TenantID,
		SourceCompanyID:              record.SourceCompanyID,
		ManifestVersion:              record.ManifestVersion,
		ContractVersion:              record.ContractVersion,
		ResourceIDs:                  canonicalBrowserDiscoveryIdentifiers(resourceIDs),
		MetadataOnlyConsentConfirmed: record.MetadataOnlyConsentConfirmed,
		HeaderProbeConsentConfirmed:  record.HeaderProbeConsentConfirmed,
		ConsentedAt:                  record.ConsentedAt,
		CreatedBy:                    record.CreatedBy,
		ExpiresAt:                    record.ExpiresAt,
		CreatedAt:                    record.CreatedAt,
	}
	if !validBrowserDiscoveryAuthorization(*authorization) {
		return nil, ErrBrowserDiscoveryUnauthorized
	}
	return authorization, nil
}

func (r *GORMRepository) SaveBrowserDiscoveryReceipt(ctx context.Context, tenantID, discoveryID string, receipt BrowserDiscoveryReceipt, recordedAt time.Time) error {
	table, err := r.browserDiscoveryAuthorizationsTable(ctx)
	if err != nil {
		return err
	}
	status, digest := strings.TrimSpace(receipt.Status), strings.TrimSpace(receipt.ContractSHA256)
	resourceCount, captureReady := receipt.ResourceCount, receipt.CaptureReadyCount
	filterRequired, pageOnly := receipt.FilterRequiredCount, receipt.PageOnlyRequiredCount
	privateEndpoint, bindingBlocked := receipt.PrivateEndpointCount, receipt.BindingBlockedCount
	recordedAt = recordedAt.UTC()
	result := table.Where("tenant_id = ? AND discovery_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(discoveryID)).Updates(map[string]interface{}{
		"receipt_status":                    status,
		"contract_sha256":                   digest,
		"resource_count":                    resourceCount,
		"capture_ready_count":               captureReady,
		"filter_contract_required_count":    filterRequired,
		"page_only_contract_required_count": pageOnly,
		"private_endpoint_required_count":   privateEndpoint,
		"binding_blocked_count":             bindingBlocked,
		"receipt_recorded_at":               recordedAt,
		"updated_at":                        recordedAt,
	})
	if result.Error != nil {
		return fmt.Errorf("save SmartAccounts browser discovery receipt: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrBrowserDiscoveryUnauthorized
	}
	return nil
}
func (r *GORMRepository) CreateBrowserCaptureAuthorization(ctx context.Context, auth BrowserCaptureAuthorization) error {
	table, err := r.browserCaptureAuthorizationsTable(ctx)
	if err != nil {
		return err
	}
	scope, err := json.Marshal(canonicalBrowserCaptureScope(auth.Scope))
	if err != nil {
		return err
	}
	record := models.SmartAccountsBrowserCaptureAuthorizationRecord{RunID: auth.RunID, TenantID: auth.TenantID, SourceCompanyID: auth.SourceCompanyID, ManifestVersion: auth.ManifestVersion, Scope: scope, TokenSHA256: auth.TokenSHA256, CreatedBy: auth.CreatedBy, ExpiresAt: auth.ExpiresAt.UTC(), CreatedAt: auth.CreatedAt.UTC()}
	if err := table.Create(&record).Error; err != nil {
		return fmt.Errorf("create SmartAccounts browser capture authorization: %w", err)
	}
	return nil
}
func (r *GORMRepository) GetBrowserCaptureAuthorization(ctx context.Context, runID, tenantID string) (*BrowserCaptureAuthorization, error) {
	table, err := r.browserCaptureAuthorizationsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserCaptureAuthorizationRecord
	if err := table.Where("run_id = ? AND tenant_id = ?", strings.TrimSpace(runID), strings.TrimSpace(tenantID)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrowserCaptureUnauthorized
		}
		return nil, err
	}
	var scope BrowserCaptureScope
	if err := json.Unmarshal(record.Scope, &scope); err != nil {
		return nil, ErrBrowserCaptureUnauthorized
	}
	return &BrowserCaptureAuthorization{RunID: record.RunID, TenantID: record.TenantID, SourceCompanyID: record.SourceCompanyID, ManifestVersion: record.ManifestVersion, Scope: canonicalBrowserCaptureScope(scope), TokenSHA256: record.TokenSHA256, CreatedBy: record.CreatedBy, ExpiresAt: record.ExpiresAt, CreatedAt: record.CreatedAt}, nil
}

// RotateBrowserCaptureAuthorization atomically replaces only the capability
// digest and expiry for an already persisted tenant/run scope. Source, scope,
// manifest, and run are immutable and are deliberately absent from updates.
func (r *GORMRepository) RotateBrowserCaptureAuthorization(ctx context.Context, authorization BrowserCaptureAuthorization) error {
	table, err := r.browserCaptureAuthorizationsTable(ctx)
	if err != nil {
		return err
	}
	result := table.Where("run_id = ? AND tenant_id = ?", strings.TrimSpace(authorization.RunID), strings.TrimSpace(authorization.TenantID)).Updates(map[string]interface{}{
		"token_sha256": strings.TrimSpace(authorization.TokenSHA256),
		"created_by":   strings.TrimSpace(authorization.CreatedBy),
		"expires_at":   authorization.ExpiresAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("rotate SmartAccounts browser capture authorization: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrBrowserCaptureUnauthorized
	}
	return nil
}

func (r *GORMRepository) FindOrCreateBrowserMasterDetailAuthorization(ctx context.Context, authorization BrowserMasterDetailAuthorization) (*BrowserMasterDetailAuthorization, bool, error) {
	if !validBrowserMasterDetailAuthorization(authorization) {
		return nil, false, ErrBrowserMasterDetailUnauthorized
	}
	table, err := r.browserMasterDetailAuthorizationsTable(ctx)
	if err != nil {
		return nil, false, err
	}
	record, err := browserMasterDetailAuthorizationToRecord(authorization)
	if err != nil {
		return nil, false, err
	}
	result := table.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "batch_id"}, {Name: "resource_id"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create SmartAccounts browser master-detail authorization: %w", result.Error)
	}
	var persisted models.SmartAccountsBrowserMasterDetailAuthorizationRecord
	if err := table.Where("tenant_id = ? AND batch_id = ? AND resource_id = ?", record.TenantID, record.BatchID, record.ResourceID).First(&persisted).Error; err != nil {
		return nil, false, fmt.Errorf("load SmartAccounts browser master-detail authorization: %w", err)
	}
	resultAuthorization, err := browserMasterDetailAuthorizationFromRecord(&persisted)
	if err != nil {
		return nil, false, err
	}
	return resultAuthorization, result.RowsAffected == 1, nil
}

func (r *GORMRepository) GetBrowserMasterDetailAuthorization(ctx context.Context, runID, tenantID string) (*BrowserMasterDetailAuthorization, error) {
	table, err := r.browserMasterDetailAuthorizationsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserMasterDetailAuthorizationRecord
	if err := table.Where("run_id = ? AND tenant_id = ?", strings.TrimSpace(runID), strings.TrimSpace(tenantID)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrowserMasterDetailUnauthorized
		}
		return nil, fmt.Errorf("load SmartAccounts browser master-detail authorization: %w", err)
	}
	return browserMasterDetailAuthorizationFromRecord(&record)
}

func (r *GORMRepository) RotateBrowserMasterDetailAuthorization(ctx context.Context, authorization BrowserMasterDetailAuthorization) error {
	if !validBrowserMasterDetailAuthorization(authorization) {
		return ErrBrowserMasterDetailUnauthorized
	}
	table, err := r.browserMasterDetailAuthorizationsTable(ctx)
	if err != nil {
		return err
	}
	result := table.Where("run_id = ? AND tenant_id = ?", authorization.RunID, authorization.TenantID).Updates(map[string]interface{}{
		"token_sha256": authorization.TokenSHA256,
		"created_by":   authorization.CreatedBy,
		"expires_at":   authorization.ExpiresAt.UTC(),
	})
	if result.Error != nil {
		return fmt.Errorf("rotate SmartAccounts browser master-detail authorization: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrBrowserMasterDetailUnauthorized
	}
	return nil
}

func browserMasterDetailAuthorizationToRecord(value BrowserMasterDetailAuthorization) (models.SmartAccountsBrowserMasterDetailAuthorizationRecord, error) {
	contract, err := json.Marshal(value.Contract)
	if err != nil {
		return models.SmartAccountsBrowserMasterDetailAuthorizationRecord{}, err
	}
	scope, err := json.Marshal(value.Scope)
	if err != nil {
		return models.SmartAccountsBrowserMasterDetailAuthorizationRecord{}, err
	}
	return models.SmartAccountsBrowserMasterDetailAuthorizationRecord{
		RunID: value.RunID, TenantID: value.TenantID, BatchID: value.BatchID, SourceCompanyID: value.SourceCompanyID, SnapshotDate: value.SnapshotDate,
		ManifestVersion: value.ManifestVersion, ResourceID: value.ResourceID, SchemaID: value.SchemaID, SourceSchema: value.SourceSchema,
		Contract: contract, ContractSHA256: value.ContractSHA256, ApprovalSHA256: value.ApprovalSHA256, Scope: scope,
		TokenSHA256: value.TokenSHA256, CreatedBy: value.CreatedBy, ExpiresAt: value.ExpiresAt.UTC(), CreatedAt: value.CreatedAt.UTC(),
	}, nil
}

func browserMasterDetailAuthorizationFromRecord(record *models.SmartAccountsBrowserMasterDetailAuthorizationRecord) (*BrowserMasterDetailAuthorization, error) {
	if record == nil {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	var contract BrowserMasterDetailContract
	var scope BrowserMasterDetailScope
	if json.Unmarshal(record.Contract, &contract) != nil || json.Unmarshal(record.Scope, &scope) != nil {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	result := &BrowserMasterDetailAuthorization{
		RunID: record.RunID, TenantID: record.TenantID, BatchID: record.BatchID, SourceCompanyID: record.SourceCompanyID, SnapshotDate: record.SnapshotDate,
		ManifestVersion: record.ManifestVersion, ResourceID: record.ResourceID, SchemaID: record.SchemaID, SourceSchema: record.SourceSchema,
		Contract: contract, ContractSHA256: record.ContractSHA256, ApprovalSHA256: record.ApprovalSHA256, Scope: scope,
		TokenSHA256: record.TokenSHA256, CreatedBy: record.CreatedBy, ExpiresAt: record.ExpiresAt, CreatedAt: record.CreatedAt,
	}
	if !validBrowserMasterDetailAuthorization(*result) {
		return nil, ErrBrowserMasterDetailUnauthorized
	}
	return result, nil
}

func (r *GORMRepository) FindOrCreateBrowserCommercialDetailAuthorization(ctx context.Context, authorization BrowserCommercialDetailAuthorization) (*BrowserCommercialDetailAuthorization, bool, error) {
	if !validBrowserCommercialDetailAuthorization(authorization) {
		return nil, false, ErrBrowserCommercialDetailUnauthorized
	}
	table, err := r.browserCommercialDetailAuthorizationsTable(ctx)
	if err != nil {
		return nil, false, err
	}
	record := browserCommercialDetailAuthorizationToRecord(authorization)
	result := table.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "batch_id"}, {Name: "source_company_id"}, {Name: "resource_id"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create SmartAccounts browser commercial-detail authorization: %w", result.Error)
	}
	var persisted models.SmartAccountsBrowserCommercialDetailAuthorizationRecord
	if err := table.Where("tenant_id = ? AND batch_id = ? AND source_company_id = ? AND resource_id = ?", record.TenantID, record.BatchID, record.SourceCompanyID, record.ResourceID).First(&persisted).Error; err != nil {
		return nil, false, fmt.Errorf("load SmartAccounts browser commercial-detail authorization: %w", err)
	}
	value, err := browserCommercialDetailAuthorizationFromRecord(&persisted)
	return value, result.RowsAffected == 1, err
}

func (r *GORMRepository) GetBrowserCommercialDetailAuthorization(ctx context.Context, runID, tenantID string) (*BrowserCommercialDetailAuthorization, error) {
	table, err := r.browserCommercialDetailAuthorizationsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserCommercialDetailAuthorizationRecord
	if err := table.Where("run_id = ? AND tenant_id = ?", strings.TrimSpace(runID), strings.TrimSpace(tenantID)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrowserCommercialDetailUnauthorized
		}
		return nil, fmt.Errorf("load SmartAccounts browser commercial-detail authorization: %w", err)
	}
	return browserCommercialDetailAuthorizationFromRecord(&record)
}

func (r *GORMRepository) RotateBrowserCommercialDetailAuthorization(ctx context.Context, authorization BrowserCommercialDetailAuthorization) error {
	if !validBrowserCommercialDetailAuthorization(authorization) {
		return ErrBrowserCommercialDetailUnauthorized
	}
	table, err := r.browserCommercialDetailAuthorizationsTable(ctx)
	if err != nil {
		return err
	}
	result := table.Where("run_id = ? AND tenant_id = ?", authorization.RunID, authorization.TenantID).Updates(map[string]interface{}{"token_sha256": authorization.TokenSHA256, "created_by": authorization.CreatedBy, "expires_at": authorization.ExpiresAt.UTC(), "updated_at": authorization.UpdatedAt.UTC()})
	if result.Error != nil {
		return fmt.Errorf("rotate SmartAccounts browser commercial-detail authorization: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrBrowserCommercialDetailUnauthorized
	}
	return nil
}

func (r *GORMRepository) SaveBrowserCommercialDetailStatus(ctx context.Context, authorization BrowserCommercialDetailAuthorization) error {
	if !validBrowserCommercialDetailAuthorization(authorization) {
		return ErrBrowserCommercialDetailUnauthorized
	}
	table, err := r.browserCommercialDetailAuthorizationsTable(ctx)
	if err != nil {
		return err
	}
	result := table.Where("run_id = ? AND tenant_id = ?", authorization.RunID, authorization.TenantID).Updates(map[string]interface{}{"status": authorization.Status, "ndjson_sha256": authorization.NDJSONSHA256, "record_count": authorization.RecordCount, "review_required": authorization.ReviewRequired, "package_id": authorization.PackageID, "package_sha256": authorization.PackageSHA256, "bridge_started_at": authorization.BridgeStartedAt, "updated_at": authorization.UpdatedAt.UTC()})
	if result.Error != nil {
		return fmt.Errorf("save SmartAccounts browser commercial-detail status: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrBrowserCommercialDetailUnauthorized
	}
	return nil
}

func browserCommercialDetailAuthorizationToRecord(value BrowserCommercialDetailAuthorization) models.SmartAccountsBrowserCommercialDetailAuthorizationRecord {
	return models.SmartAccountsBrowserCommercialDetailAuthorizationRecord{RunID: value.RunID, TenantID: value.TenantID, BatchID: value.BatchID, WorkflowID: value.WorkflowID, SourceCompanyID: value.SourceCompanyID, ManifestVersion: value.ManifestVersion, ResourceID: value.ResourceID, Sequence: value.Sequence, SchemaID: value.SchemaID, SourceSchema: value.SourceSchema, ReviewAuditID: value.ReviewAuditID, ReviewedAt: value.ReviewedAt.UTC(), ContractSHA256: value.ContractSHA256, RouteSHA256: value.RouteSHA256, ConsentSHA256: value.ConsentSHA256, FromInclusive: value.Scope.FromInclusive, ToInclusive: value.Scope.ToInclusive, CutoffAt: mustParseBrowserCommercialCutoff(value.Scope.CutoffAt), TokenSHA256: value.TokenSHA256, Status: value.Status, NDJSONSHA256: value.NDJSONSHA256, RecordCount: value.RecordCount, ReviewRequired: value.ReviewRequired, PackageID: value.PackageID, PackageSHA256: value.PackageSHA256, BridgeStartedAt: value.BridgeStartedAt, CreatedBy: value.CreatedBy, ExpiresAt: value.ExpiresAt.UTC(), CreatedAt: value.CreatedAt.UTC(), UpdatedAt: value.UpdatedAt.UTC()}
}

func browserCommercialDetailAuthorizationFromRecord(record *models.SmartAccountsBrowserCommercialDetailAuthorizationRecord) (*BrowserCommercialDetailAuthorization, error) {
	if record == nil {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	value := &BrowserCommercialDetailAuthorization{RunID: record.RunID, TenantID: record.TenantID, BatchID: record.BatchID, WorkflowID: record.WorkflowID, SourceCompanyID: record.SourceCompanyID, ManifestVersion: record.ManifestVersion, ResourceID: record.ResourceID, Sequence: record.Sequence, SchemaID: record.SchemaID, SourceSchema: record.SourceSchema, ReviewAuditID: record.ReviewAuditID, ReviewedAt: record.ReviewedAt.UTC(), ContractSHA256: record.ContractSHA256, RouteSHA256: record.RouteSHA256, ConsentSHA256: record.ConsentSHA256, Scope: BrowserCommercialDetailScope{FromInclusive: record.FromInclusive, ToInclusive: record.ToInclusive, CutoffAt: record.CutoffAt.UTC().Format(time.RFC3339)}, TokenSHA256: record.TokenSHA256, Status: record.Status, NDJSONSHA256: record.NDJSONSHA256, RecordCount: record.RecordCount, ReviewRequired: record.ReviewRequired, PackageID: record.PackageID, PackageSHA256: record.PackageSHA256, BridgeStartedAt: record.BridgeStartedAt, CreatedBy: record.CreatedBy, ExpiresAt: record.ExpiresAt.UTC(), CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC()}
	if !validBrowserCommercialDetailAuthorization(*value) {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	return value, nil
}

func mustParseBrowserCommercialCutoff(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed.UTC()
}

// FindOrCreateBrowserCaptureWorkflow implements a narrowly idempotent policy:
// the owner-selected historical start date, tenant, paired opaque source, and
// server-derived plan day identify the workflow. A later day intentionally
// creates a new generation instead of extending an old immutable relay scope.
func (r *GORMRepository) FindOrCreateBrowserCaptureWorkflow(ctx context.Context, workflow BrowserCaptureWorkflow) (*BrowserCaptureWorkflow, bool, error) {
	table, err := r.browserCaptureWorkflowsTable(ctx)
	if err != nil {
		return nil, false, err
	}
	record, err := browserCaptureWorkflowToRecord(workflow)
	if err != nil {
		return nil, false, err
	}
	result := table.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "source_company_id"}, {Name: "from_inclusive"}, {Name: "to_inclusive"}},
		DoNothing: true,
	}).Create(&record)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create SmartAccounts browser capture workflow: %w", result.Error)
	}
	var persisted models.SmartAccountsBrowserCaptureWorkflowRecord
	if err := table.Where("tenant_id = ? AND source_company_id = ? AND from_inclusive = ? AND to_inclusive = ?", record.TenantID, record.SourceCompanyID, record.FromInclusive, record.ToInclusive).First(&persisted).Error; err != nil {
		return nil, false, fmt.Errorf("load SmartAccounts browser capture workflow: %w", err)
	}
	workflowResult, err := browserCaptureWorkflowFromRecord(&persisted)
	if err != nil {
		return nil, false, err
	}
	return workflowResult, result.RowsAffected == 1, nil
}

func (r *GORMRepository) GetBrowserCaptureWorkflow(ctx context.Context, workflowID, tenantID string) (*BrowserCaptureWorkflow, error) {
	table, err := r.browserCaptureWorkflowsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserCaptureWorkflowRecord
	if err := table.Where("id = ? AND tenant_id = ?", strings.TrimSpace(workflowID), strings.TrimSpace(tenantID)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrowserCaptureWorkflowNotFound
		}
		return nil, fmt.Errorf("load SmartAccounts browser capture workflow: %w", err)
	}
	return browserCaptureWorkflowFromRecord(&record)
}

func (r *GORMRepository) SetBrowserCaptureWorkflowRun(ctx context.Context, tenantID, workflowID, runID string, updatedAt time.Time) (*BrowserCaptureWorkflow, error) {
	table, err := r.browserCaptureWorkflowsTable(ctx)
	if err != nil {
		return nil, err
	}
	result := table.Where("id = ? AND tenant_id = ? AND (capture_run_id IS NULL OR capture_run_id = ?)", strings.TrimSpace(workflowID), strings.TrimSpace(tenantID), strings.TrimSpace(runID)).Updates(map[string]interface{}{
		"capture_run_id": strings.TrimSpace(runID),
		"status":         BrowserCaptureWorkflowIssued,
		"updated_at":     updatedAt.UTC(),
	})
	if result.Error != nil {
		return nil, fmt.Errorf("set SmartAccounts browser capture workflow run: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, ErrBrowserCaptureWorkflowNotFound
	}
	return r.GetBrowserCaptureWorkflow(ctx, workflowID, tenantID)
}

func (r *GORMRepository) CreateBrowserPairing(ctx context.Context, pairing BrowserPairing) error {
	table, err := r.browserPairingsTable(ctx)
	if err != nil {
		return err
	}
	record := browserPairingToRecord(pairing)
	if err := table.Create(&record).Error; err != nil {
		return fmt.Errorf("create SmartAccounts browser pairing: %w", err)
	}
	return nil
}

func browserPairingToRecord(pairing BrowserPairing) models.SmartAccountsBrowserPairingRecord {
	var expectedSourceCompanyID *string
	if sourceID := strings.TrimSpace(pairing.ExpectedSourceCompanyID); sourceID != "" {
		expectedSourceCompanyID = &sourceID
	}
	var sourceCompanyID *string
	if sourceID := strings.TrimSpace(pairing.SourceCompanyID); sourceID != "" {
		sourceCompanyID = &sourceID
	}
	return models.SmartAccountsBrowserPairingRecord{
		ID:                      strings.TrimSpace(pairing.ID),
		TenantID:                strings.TrimSpace(pairing.TenantID),
		TokenSHA256:             strings.TrimSpace(pairing.TokenSHA256),
		ExpectedSourceCompanyID: expectedSourceCompanyID,
		SourceCompanyID:         sourceCompanyID,
		CreatedBy:               strings.TrimSpace(pairing.CreatedBy),
		Status:                  strings.TrimSpace(pairing.Status),
		ExpiresAt:               pairing.ExpiresAt.UTC(),
		CreatedAt:               pairing.CreatedAt.UTC(),
	}
}

func (r *GORMRepository) GetBrowserPairing(ctx context.Context, pairingID, tenantID string) (*BrowserPairing, error) {
	table, err := r.browserPairingsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserPairingRecord
	err = table.Where("id = ? AND tenant_id = ?", strings.TrimSpace(pairingID), strings.TrimSpace(tenantID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrBrowserPairingNotClaimable
	}
	if err != nil {
		return nil, fmt.Errorf("load SmartAccounts browser pairing: %w", err)
	}
	return browserPairingFromRecord(&record), nil
}

func (r *GORMRepository) ClaimBrowserPairing(ctx context.Context, pairingID, tokenSHA256, sourceCompanyID string, claimedAt time.Time) (*BrowserPairing, error) {
	table, err := r.browserPairingsTable(ctx)
	if err != nil {
		return nil, err
	}
	claimedAt = claimedAt.UTC()
	result := table.Where("id = ? AND token_sha256 = ? AND status = ? AND expires_at > ? AND (expected_source_company_id IS NULL OR expected_source_company_id = ?)", strings.TrimSpace(pairingID), strings.TrimSpace(tokenSHA256), BrowserPairingStatusIssued, claimedAt, strings.TrimSpace(sourceCompanyID)).Updates(map[string]interface{}{
		"status":            BrowserPairingStatusClaimed,
		"source_company_id": strings.TrimSpace(sourceCompanyID),
		"claimed_at":        claimedAt,
	})
	if result.Error != nil {
		return nil, fmt.Errorf("claim SmartAccounts browser pairing: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return nil, ErrBrowserPairingNotClaimable
	}
	var record models.SmartAccountsBrowserPairingRecord
	if err := table.Where("id = ?", strings.TrimSpace(pairingID)).First(&record).Error; err != nil {
		return nil, fmt.Errorf("load claimed SmartAccounts browser pairing: %w", err)
	}
	return browserPairingFromRecord(&record), nil
}

func (r *GORMRepository) GetBrowserOnboarding(ctx context.Context, sourceCompanyID string) (*BrowserOnboardingBinding, error) {
	table, err := r.browserOnboardingTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsBrowserOnboardingBindingRecord
	if err := table.Where("source_company_id = ?", strings.TrimSpace(sourceCompanyID)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBrowserOnboardingNotFound
		}
		return nil, fmt.Errorf("load SmartAccounts browser onboarding binding: %w", err)
	}
	return browserOnboardingFromRecord(&record), nil
}

func (r *GORMRepository) CreateBrowserOnboarding(ctx context.Context, binding BrowserOnboardingBinding) (*BrowserOnboardingBinding, bool, error) {
	table, err := r.browserOnboardingTable(ctx)
	if err != nil {
		return nil, false, err
	}
	record := browserOnboardingToRecord(binding)
	result := table.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "source_company_id"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create SmartAccounts browser onboarding binding: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return browserOnboardingFromRecord(&record), true, nil
}

func (r *GORMRepository) SetBrowserOnboardingTarget(ctx context.Context, sourceCompanyID, tenantID, tenantName string) (*BrowserOnboardingBinding, error) {
	table, err := r.browserOnboardingTable(ctx)
	if err != nil {
		return nil, err
	}
	result := table.Where("source_company_id = ? AND tenant_id IS NULL", strings.TrimSpace(sourceCompanyID)).Updates(map[string]interface{}{
		"tenant_id":   strings.TrimSpace(tenantID),
		"tenant_name": strings.TrimSpace(tenantName),
		"status":      BrowserOnboardingTargetReady,
		"updated_at":  time.Now().UTC(),
	})
	if result.Error != nil {
		return nil, fmt.Errorf("set SmartAccounts browser onboarding target: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return r.GetBrowserOnboarding(ctx, sourceCompanyID)
	}
	return r.GetBrowserOnboarding(ctx, sourceCompanyID)
}

func (r *GORMRepository) SetBrowserOnboardingPairing(ctx context.Context, sourceCompanyID, pairingID string) (*BrowserOnboardingBinding, error) {
	table, err := r.browserOnboardingTable(ctx)
	if err != nil {
		return nil, err
	}
	result := table.Where("source_company_id = ? AND tenant_id IS NOT NULL", strings.TrimSpace(sourceCompanyID)).Updates(map[string]interface{}{
		"pairing_id": strings.TrimSpace(pairingID),
		"status":     BrowserOnboardingPairingIssued,
		"updated_at": time.Now().UTC(),
	})
	if result.Error != nil {
		return nil, fmt.Errorf("set SmartAccounts browser onboarding pairing: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrBrowserOnboardingNotFound
	}
	return r.GetBrowserOnboarding(ctx, sourceCompanyID)
}

func (r *GORMRepository) FindBrowserOnboardingTargets(ctx context.Context, sourceCompanyID string) ([]BrowserOnboardingBinding, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts sync database is not configured")
	}
	var records []struct {
		TenantID   string `gorm:"column:tenant_id"`
		TenantName string `gorm:"column:tenant_name"`
	}
	err := r.db.WithContext(ctx).
		Table("public.smartaccounts_sync_controls AS control").
		Select("control.tenant_id, tenants.name AS tenant_name").
		Joins("JOIN public.tenants AS tenants ON tenants.id = control.tenant_id").
		Where("control.source_company_id = ? AND control.secret_reference LIKE ?", strings.TrimSpace(sourceCompanyID), "brave-session://%").
		Order("control.tenant_id ASC").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("find SmartAccounts browser onboarding target: %w", err)
	}
	results := make([]BrowserOnboardingBinding, 0, len(records))
	for _, record := range records {
		results = append(results, BrowserOnboardingBinding{SourceCompanyID: strings.TrimSpace(sourceCompanyID), TenantID: record.TenantID, TenantName: record.TenantName, Status: BrowserOnboardingPaired})
	}
	return results, nil
}

func (r *GORMRepository) Get(ctx context.Context, tenantID, sourceCompanyID string) (*Control, error) {
	table, err := r.controlsTable(ctx)
	if err != nil {
		return nil, err
	}
	var record models.SmartAccountsSyncControlRecord
	err = table.Where("tenant_id = ? AND source_company_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrControlNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("load SmartAccounts sync control: %w", err)
	}
	return recordToControl(&record), nil
}

func (r *GORMRepository) Upsert(ctx context.Context, control Control) (*Control, error) {
	table, err := r.controlsTable(ctx)
	if err != nil {
		return nil, err
	}
	record := controlToRecord(control)
	if err := table.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "source_company_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"source_company_id":   record.SourceCompanyID,
			"source_company_name": record.SourceCompanyName,
			"secret_reference":    record.SecretReference,
			"created_by":          record.CreatedBy,
			"updated_at":          record.UpdatedAt,
		}),
	}).Create(record).Error; err != nil {
		return nil, fmt.Errorf("save SmartAccounts sync control: %w", err)
	}
	return r.Get(ctx, control.TenantID, control.SourceCompanyID)
}

func (r *GORMRepository) RecordCaptureRun(ctx context.Context, tenantID, sourceCompanyID, runID string, requestedAt time.Time) (*Control, error) {
	table, err := r.controlsTable(ctx)
	if err != nil {
		return nil, err
	}
	result := table.Where("tenant_id = ? AND source_company_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID)).Updates(map[string]interface{}{
		"capture_run_id":       strings.TrimSpace(runID),
		"dry_run_requested_at": requestedAt,
		"updated_at":           requestedAt,
	})
	if result.Error != nil {
		return nil, fmt.Errorf("record SmartAccounts capture run: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrControlNotConfigured
	}
	return r.Get(ctx, tenantID, sourceCompanyID)
}

// UpsertCaptureProgress preserves a safe snapshot for each explicit bridge
// run. It deliberately serializes only CaptureProgress, whose type excludes
// credentials, source records, queries, cursors, and private paths.
func (r *GORMRepository) UpsertCaptureProgress(ctx context.Context, tenantID, sourceCompanyID string, progress CaptureProgress, observedAt time.Time) error {
	if r == nil || r.db == nil || strings.TrimSpace(progress.RunID) == "" {
		return errors.New("SmartAccounts capture history storage is not configured")
	}
	encoded, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("encode SmartAccounts capture progress: %w", err)
	}
	record := models.SmartAccountsSyncCaptureRunRecord{
		TenantID:        strings.TrimSpace(tenantID),
		SourceCompanyID: strings.TrimSpace(sourceCompanyID),
		RunID:           strings.TrimSpace(progress.RunID),
		Progress:        encoded,
		UpdatedAt:       observedAt.UTC(),
	}
	if err := r.db.WithContext(ctx).Table("public.smartaccounts_sync_capture_run_history").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "source_company_id"}, {Name: "run_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"progress":   record.Progress,
			"updated_at": record.UpdatedAt,
		}),
	}).Create(&record).Error; err != nil {
		return fmt.Errorf("save SmartAccounts capture progress: %w", err)
	}
	return nil
}

func (r *GORMRepository) ListCaptureProgresses(ctx context.Context, tenantID, sourceCompanyID string) ([]CaptureProgress, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts capture history storage is not configured")
	}
	var records []models.SmartAccountsSyncCaptureRunRecord
	if err := r.db.WithContext(ctx).Table("public.smartaccounts_sync_capture_run_history").Where("tenant_id = ? AND source_company_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID)).Order("updated_at DESC, run_id DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list SmartAccounts capture progress: %w", err)
	}
	progresses := make([]CaptureProgress, 0, len(records))
	for _, record := range records {
		var progress CaptureProgress
		if err := json.Unmarshal(record.Progress, &progress); err != nil || strings.TrimSpace(progress.RunID) != record.RunID {
			return nil, errors.New("stored SmartAccounts capture progress is invalid")
		}
		progresses = append(progresses, progress)
	}
	return progresses, nil
}

func (r *GORMRepository) MarkDryRunRequested(ctx context.Context, tenantID, sourceCompanyID string, requestedAt time.Time) (*Control, error) {
	table, err := r.controlsTable(ctx)
	if err != nil {
		return nil, err
	}
	result := table.Where("tenant_id = ? AND source_company_id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(sourceCompanyID)).Updates(map[string]interface{}{
		"dry_run_requested_at": requestedAt,
		"updated_at":           requestedAt,
	})
	if result.Error != nil {
		return nil, fmt.Errorf("mark SmartAccounts dry run requested: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, ErrControlNotConfigured
	}
	return r.Get(ctx, tenantID, sourceCompanyID)
}

func controlToRecord(control Control) *models.SmartAccountsSyncControlRecord {
	now := control.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	createdAt := control.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	return &models.SmartAccountsSyncControlRecord{
		TenantID:          strings.TrimSpace(control.TenantID),
		SourceCompanyID:   strings.TrimSpace(control.SourceCompanyID),
		SourceCompanyName: strings.TrimSpace(control.SourceCompanyName),
		SecretReference:   strings.TrimSpace(control.SecretReference),
		CreatedBy:         strings.TrimSpace(control.CreatedBy),
		DryRunRequestedAt: control.DryRunRequestedAt,
		CaptureRunID:      strings.TrimSpace(control.CaptureRunID),
		CreatedAt:         createdAt,
		UpdatedAt:         now,
	}
}

func recordToControl(record *models.SmartAccountsSyncControlRecord) *Control {
	if record == nil {
		return nil
	}
	return &Control{
		TenantID:          record.TenantID,
		SourceCompanyID:   record.SourceCompanyID,
		SourceCompanyName: record.SourceCompanyName,
		SecretReference:   record.SecretReference,
		CreatedBy:         record.CreatedBy,
		DryRunRequestedAt: record.DryRunRequestedAt,
		CaptureRunID:      record.CaptureRunID,
		CreatedAt:         record.CreatedAt,
		UpdatedAt:         record.UpdatedAt,
	}
}

func browserPairingFromRecord(record *models.SmartAccountsBrowserPairingRecord) *BrowserPairing {
	if record == nil {
		return nil
	}
	return &BrowserPairing{
		ID:                      record.ID,
		TenantID:                record.TenantID,
		TokenSHA256:             record.TokenSHA256,
		ExpectedSourceCompanyID: stringValue(record.ExpectedSourceCompanyID),
		SourceCompanyID:         stringValue(record.SourceCompanyID),
		CreatedBy:               record.CreatedBy,
		Status:                  record.Status,
		ExpiresAt:               record.ExpiresAt,
		ClaimedAt:               record.ClaimedAt,
		CreatedAt:               record.CreatedAt,
	}
}

func browserOnboardingToRecord(binding BrowserOnboardingBinding) models.SmartAccountsBrowserOnboardingBindingRecord {
	var tenantID, pairingID *string
	if value := strings.TrimSpace(binding.TenantID); value != "" {
		tenantID = &value
	}
	if value := strings.TrimSpace(binding.PairingID); value != "" {
		pairingID = &value
	}
	now := time.Now().UTC()
	return models.SmartAccountsBrowserOnboardingBindingRecord{
		SourceCompanyID:   strings.TrimSpace(binding.SourceCompanyID),
		SourceCompanyName: strings.TrimSpace(binding.SourceCompanyName),
		TenantID:          tenantID,
		TenantName:        strings.TrimSpace(binding.TenantName),
		PairingID:         pairingID,
		Status:            strings.TrimSpace(binding.Status),
		CreatedBy:         strings.TrimSpace(binding.CreatedBy),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func browserOnboardingFromRecord(record *models.SmartAccountsBrowserOnboardingBindingRecord) *BrowserOnboardingBinding {
	if record == nil {
		return nil
	}
	return &BrowserOnboardingBinding{
		SourceCompanyID:   record.SourceCompanyID,
		SourceCompanyName: record.SourceCompanyName,
		TenantID:          stringValue(record.TenantID),
		TenantName:        record.TenantName,
		PairingID:         stringValue(record.PairingID),
		Status:            record.Status,
		CreatedBy:         record.CreatedBy,
	}
}

func browserCaptureWorkflowToRecord(workflow BrowserCaptureWorkflow) (models.SmartAccountsBrowserCaptureWorkflowRecord, error) {
	from, fromErr := time.Parse(time.DateOnly, strings.TrimSpace(workflow.FromInclusive))
	to, toErr := time.Parse(time.DateOnly, strings.TrimSpace(workflow.ToInclusive))
	if fromErr != nil || toErr != nil || !validBrowserCaptureWorkflow(workflow) {
		return models.SmartAccountsBrowserCaptureWorkflowRecord{}, ErrBrowserCaptureWorkflowUnavailable
	}
	var runID *string
	if value := strings.TrimSpace(workflow.CaptureRunID); value != "" {
		runID = &value
	}
	return models.SmartAccountsBrowserCaptureWorkflowRecord{
		ID: workflow.ID, TenantID: workflow.TenantID, SourceCompanyID: workflow.SourceCompanyID,
		FromInclusive: from.UTC(), ToInclusive: to.UTC(), CutoffAt: workflow.CutoffAt.UTC(), CaptureRunID: runID,
		Status: workflow.Status, CreatedBy: workflow.CreatedBy, CreatedAt: workflow.CreatedAt.UTC(), UpdatedAt: workflow.UpdatedAt.UTC(),
	}, nil
}

func browserCaptureWorkflowFromRecord(record *models.SmartAccountsBrowserCaptureWorkflowRecord) (*BrowserCaptureWorkflow, error) {
	if record == nil {
		return nil, ErrBrowserCaptureWorkflowNotFound
	}
	workflow := &BrowserCaptureWorkflow{
		ID: record.ID, TenantID: record.TenantID, SourceCompanyID: record.SourceCompanyID,
		FromInclusive: record.FromInclusive.UTC().Format(time.DateOnly), ToInclusive: record.ToInclusive.UTC().Format(time.DateOnly),
		CutoffAt: record.CutoffAt.UTC(), CaptureRunID: stringValue(record.CaptureRunID), Status: record.Status,
		CreatedBy: record.CreatedBy, CreatedAt: record.CreatedAt.UTC(), UpdatedAt: record.UpdatedAt.UTC(),
	}
	if !validBrowserCaptureWorkflow(*workflow) {
		return nil, ErrBrowserCaptureWorkflowUnavailable
	}
	return workflow, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
