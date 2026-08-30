package smartaccountsreferences

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
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
		panic(fmt.Errorf("create SmartAccounts reference repository: %w", err))
	}
	return &Repository{db: db}
}
func (r *Repository) table(ctx context.Context, schema, name string) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts reference database is not configured")
	}
	return database.TenantTable(r.db.WithContext(ctx), schema, name)
}

func (r *Repository) SavePreview(ctx context.Context, schema string, stored *StoredPreview, user string) error {
	if stored == nil {
		return errors.New("reference preview is required")
	}
	table, err := r.table(ctx, schema, "smartaccounts_reference_previews")
	if err != nil {
		return err
	}
	plan, err := json.Marshal(stored.Actions)
	if err != nil {
		return err
	}
	reconciliation, err := json.Marshal(stored.Preview.Reconciliation)
	if err != nil {
		return err
	}
	issues, err := json.Marshal(stored.Preview.Issues)
	if err != nil {
		return err
	}
	row := models.SmartAccountsReferencePreviewRecord{ID: stored.Preview.ID, TenantID: stored.Preview.TenantID, PackageID: stored.Preview.PackageID, SourceCompanyID: stored.Preview.SourceCompanyID, PreviewSHA256: stored.Preview.PreviewSHA256, Status: stored.Preview.Status, Plan: plan, Reconciliation: reconciliation, Issues: issues, CreatedBy: user, CreatedAt: time.Now().UTC()}
	result := table.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "package_id"}, {Name: "preview_sha256"}}, DoNothing: true}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var existing models.SmartAccountsReferencePreviewRecord
		if err := table.Where("tenant_id=? AND package_id=? AND preview_sha256=?", row.TenantID, row.PackageID, row.PreviewSHA256).First(&existing).Error; err != nil {
			return err
		}
		stored.Preview.ID, stored.Preview.Status, stored.Preview.AppliedAt = existing.ID, existing.Status, existing.AppliedAt
	}
	return nil
}
func (r *Repository) GetPreview(ctx context.Context, schema, tenant, id string) (*StoredPreview, error) {
	table, err := r.table(ctx, schema, "smartaccounts_reference_previews")
	if err != nil {
		return nil, err
	}
	var row models.SmartAccountsReferencePreviewRecord
	if err = table.Where("tenant_id=? AND id=?", tenant, id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPreviewNotFound
		}
		return nil, err
	}
	result := &StoredPreview{Preview: Preview{ID: row.ID, TenantID: row.TenantID, PackageID: row.PackageID, SourceCompanyID: row.SourceCompanyID, Status: row.Status, PreviewSHA256: row.PreviewSHA256, AppliedAt: row.AppliedAt}}
	_ = json.Unmarshal(row.Plan, &result.Actions)
	_ = json.Unmarshal(row.Reconciliation, &result.Preview.Reconciliation)
	_ = json.Unmarshal(row.Issues, &result.Preview.Issues)
	for _, a := range result.Actions {
		result.Preview.Actions = append(result.Preview.Actions, a.Action)
	}
	return result, nil
}

func (r *Repository) GetLatestPreviewForPackage(ctx context.Context, schema, tenant, packageID string) (*StoredPreview, error) {
	table, err := r.table(ctx, schema, "smartaccounts_reference_previews")
	if err != nil {
		return nil, err
	}
	var row models.SmartAccountsReferencePreviewRecord
	if err = table.Where("tenant_id=? AND package_id=?", tenant, packageID).Order("created_at DESC, id DESC").First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPreviewNotFound
		}
		return nil, err
	}
	return r.GetPreview(ctx, schema, tenant, row.ID)
}
func (r *Repository) GetIdentity(ctx context.Context, schema, tenant, provider, source, entity, external string) (*Identity, error) {
	table, err := r.table(ctx, schema, "smartaccounts_reference_identities")
	if err != nil {
		return nil, err
	}
	var row models.SmartAccountsReferenceIdentityRecord
	if err = table.Where("tenant_id=? AND provider=? AND source_company_id=? AND entity_type=? AND external_id=?", tenant, provider, source, entity, external).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &Identity{EntityType: row.EntityType, ExternalID: row.ExternalID, Revision: row.Revision, TargetID: row.TargetID, Status: row.Status}, nil
}
func (r *Repository) ReserveIdentity(ctx context.Context, schema, tenant, provider, source, entity, external, revision, target string) (*Identity, bool, error) {
	table, err := r.table(ctx, schema, "smartaccounts_reference_identities")
	if err != nil {
		return nil, false, err
	}
	now := time.Now().UTC()
	row := models.SmartAccountsReferenceIdentityRecord{ID: uuid.NewString(), TenantID: tenant, Provider: provider, SourceCompanyID: source, EntityType: entity, ExternalID: external, Revision: revision, TargetID: target, Status: IdentityPending, CreatedAt: now, UpdatedAt: now}
	result := table.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "provider"}, {Name: "source_company_id"}, {Name: "entity_type"}, {Name: "external_id"}}, DoNothing: true}).Create(&row)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		return &Identity{EntityType: entity, ExternalID: external, Revision: revision, TargetID: target, Status: IdentityPending}, true, nil
	}
	existing, err := r.GetIdentity(ctx, schema, tenant, provider, source, entity, external)
	return existing, false, err
}
func (r *Repository) MarkIdentityApplied(ctx context.Context, schema, tenant, provider, source, entity, external string) error {
	table, err := r.table(ctx, schema, "smartaccounts_reference_identities")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return table.Where("tenant_id=? AND provider=? AND source_company_id=? AND entity_type=? AND external_id=?", tenant, provider, source, entity, external).Updates(map[string]any{"status": IdentityApplied, "applied_at": now, "updated_at": now}).Error
}
func (r *Repository) MarkPreviewApplied(ctx context.Context, schema, tenant, id string) error {
	table, err := r.table(ctx, schema, "smartaccounts_reference_previews")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return table.Where("tenant_id=? AND id=?", tenant, id).Updates(map[string]any{"status": StatusApplied, "applied_at": now}).Error
}
