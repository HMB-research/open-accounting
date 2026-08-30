package smartaccountsexecutor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store holds only reviewed plan metadata and posting identities. Source rows
// remain in importdelivery's server-only archive.
type Store interface {
	SavePreview(context.Context, string, *Preview, string) error
	GetPreview(context.Context, string, string, string) (*Preview, error)
	GetPostedIdentity(context.Context, string, string, string, string, string, string) (*PostedIdentity, error)
	ReservePosting(context.Context, string, string, string, string, string, string, string, string, string, string) (*PostedIdentity, bool, error)
	MarkPostingApplied(context.Context, string, string, string, string, string) error
	MarkPreviewApplied(context.Context, string, string, string) error
	SaveMapping(context.Context, string, string, string, string, string, AccountImport) error
	ListAppliedPostings(context.Context, string, string, string, string, string) ([]AppliedIdentity, error)
}

type GORMRepository struct{ db *gorm.DB }

var newExecutorGormDBFromPool = database.NewGormDBFromPool

func NewRepository(pool *pgxpool.Pool) *GORMRepository {
	if pool == nil {
		return &GORMRepository{}
	}
	db, err := newExecutorGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create SmartAccounts executor repository: %w", err))
	}
	return &GORMRepository{db: db}
}
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }
func (r *GORMRepository) table(ctx context.Context, schema, name string) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("SmartAccounts executor database is not configured")
	}
	return database.TenantTable(r.db.WithContext(ctx), schema, name)
}

type previewRow struct {
	ID              string `gorm:"primaryKey"`
	TenantID        string
	PackageID       string
	SourceCompanyID string
	PreviewSHA256   string
	Status          string
	Plan            json.RawMessage
	Reconciliation  json.RawMessage
	Issues          json.RawMessage
	CreatedBy       string
	CreatedAt       time.Time
	AppliedAt       *time.Time
}

func (previewRow) TableName() string { return "smartaccounts_executor_previews" }

type postingRow struct {
	ID              string `gorm:"primaryKey"`
	TenantID        string
	Provider        string
	SourceCompanyID string
	Resource        string
	ExternalID      string
	Revision        string
	Status          string
	JournalEntryID  string
	PackageID       string
	PreviewID       string
	CreatedAt       time.Time
	ReservedBy      string
	AppliedAt       *time.Time
	AppliedBy       string
}

func (postingRow) TableName() string { return "smartaccounts_financial_postings" }

type mappingRow struct {
	TenantID                string
	Provider                string
	SourceCompanyID         string
	SourceAccountExternalID string
	TargetAccountID         string
	Decision                json.RawMessage
	CreatedAt               time.Time
}

func (mappingRow) TableName() string { return "smartaccounts_source_account_mappings" }

type persistedPlan struct {
	Journals       []PlannedJournal `json:"journals"`
	AccountImports []AccountImport  `json:"account_imports"`
	ScopeSHA256    string           `json:"scope_sha256"`
}

func (r *GORMRepository) SavePreview(ctx context.Context, schema string, p *Preview, userID string) error {
	t, err := r.table(ctx, schema, "smartaccounts_executor_previews")
	if err != nil {
		return err
	}
	plan, err := json.Marshal(persistedPlan{Journals: p.Journals, AccountImports: p.AccountImports, ScopeSHA256: p.ScopeSHA256})
	if err != nil {
		return err
	}
	rec, err := json.Marshal(p.AccountReconciliation)
	if err != nil {
		return err
	}
	issues, err := json.Marshal(p.Issues)
	if err != nil {
		return err
	}
	row := previewRow{ID: p.ID, TenantID: p.TenantID, PackageID: p.PackageID, SourceCompanyID: p.SourceCompanyID, PreviewSHA256: p.PreviewSHA256, Status: p.Status, Plan: plan, Reconciliation: rec, Issues: issues, CreatedBy: userID}
	result := t.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "package_id"}, {Name: "preview_sha256"}}, DoNothing: true}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var existing previewRow
		if err := t.Where("tenant_id = ? AND package_id = ? AND preview_sha256 = ?", p.TenantID, p.PackageID, p.PreviewSHA256).First(&existing).Error; err != nil {
			return err
		}
		p.ID, p.Status, p.FinancialWritesApplied = existing.ID, existing.Status, existing.AppliedAt != nil
	}
	return nil
}
func (r *GORMRepository) GetPreview(ctx context.Context, schema, tenantID, id string) (*Preview, error) {
	t, err := r.table(ctx, schema, "smartaccounts_executor_previews")
	if err != nil {
		return nil, err
	}
	var row previewRow
	if err = t.Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPreviewNotFound
		}
		return nil, err
	}
	p := &Preview{ID: row.ID, TenantID: row.TenantID, PackageID: row.PackageID, SourceCompanyID: row.SourceCompanyID, Status: row.Status, PreviewSHA256: row.PreviewSHA256, FinancialWritesPlanned: row.Status == PlanStatusPreviewReady, FinancialWritesApplied: row.AppliedAt != nil}
	var plan persistedPlan
	_ = json.Unmarshal(row.Plan, &plan)
	p.Journals = plan.Journals
	p.AccountImports = plan.AccountImports
	p.ScopeSHA256 = plan.ScopeSHA256
	_ = json.Unmarshal(row.Reconciliation, &p.AccountReconciliation)
	_ = json.Unmarshal(row.Issues, &p.Issues)
	return p, nil
}
func (r *GORMRepository) GetPostedIdentity(ctx context.Context, schema, tenant, provider, source, resource, external string) (*PostedIdentity, error) {
	t, err := r.table(ctx, schema, "smartaccounts_financial_postings")
	if err != nil {
		return nil, err
	}
	var row postingRow
	if err = t.Where("tenant_id=? AND provider=? AND source_company_id=? AND resource=? AND external_id=?", tenant, provider, source, resource, external).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &PostedIdentity{ExternalID: row.ExternalID, Revision: row.Revision, ReservationID: row.ID, JournalID: row.JournalEntryID, Status: row.Status, PackageID: row.PackageID, PreviewID: row.PreviewID, ReservedBy: row.ReservedBy, AppliedBy: row.AppliedBy}, nil
}
func (r *GORMRepository) ReservePosting(ctx context.Context, schema, tenant, provider, source, resource, external, revision, packageID, previewID, reservedBy string) (*PostedIdentity, bool, error) {
	t, err := r.table(ctx, schema, "smartaccounts_financial_postings")
	if err != nil {
		return nil, false, err
	}
	row := postingRow{ID: uuid.NewString(), TenantID: tenant, Provider: provider, SourceCompanyID: source, Resource: resource, ExternalID: external, Revision: revision, Status: "RESERVED", PackageID: packageID, PreviewID: previewID, ReservedBy: reservedBy}
	result := t.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "provider"}, {Name: "source_company_id"}, {Name: "resource"}, {Name: "external_id"}}, DoNothing: true}).Create(&row)
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected > 0 {
		return &PostedIdentity{ExternalID: external, Revision: revision, ReservationID: row.ID, Status: row.Status, PackageID: packageID, PreviewID: previewID, ReservedBy: reservedBy}, true, nil
	}
	existing, err := r.GetPostedIdentity(ctx, schema, tenant, provider, source, resource, external)
	return existing, false, err
}
func (r *GORMRepository) MarkPostingApplied(ctx context.Context, schema, tenant, external, journalID, actorID string) error {
	t, err := r.table(ctx, schema, "smartaccounts_financial_postings")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	result := t.Where("tenant_id=? AND id=? AND status=? AND reserved_by=?", tenant, external, "RESERVED", actorID).Updates(map[string]any{"status": PlanStatusApplied, "journal_entry_id": journalID, "applied_at": now, "applied_by": actorID})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	// A same-actor retry can race after the target journal was posted. Do not
	// overwrite it: accept only the exact immutable applied result.
	var existing postingRow
	if err := t.Where("tenant_id=? AND id=?", tenant, external).First(&existing).Error; err != nil {
		return err
	}
	if existing.Status == PlanStatusApplied && existing.JournalEntryID == journalID && existing.AppliedBy == actorID {
		return nil
	}
	return errors.New("SmartAccounts posting reservation changed and requires review")
}
func (r *GORMRepository) MarkPreviewApplied(ctx context.Context, schema, tenant, id string) error {
	t, err := r.table(ctx, schema, "smartaccounts_executor_previews")
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return t.Where("tenant_id=? AND id=?", tenant, id).Updates(map[string]any{"status": PlanStatusApplied, "applied_at": now}).Error
}
func (r *GORMRepository) SaveMapping(ctx context.Context, schema, tenant, source, sourceAccount, target string, decision AccountImport) error {
	t, err := r.table(ctx, schema, "smartaccounts_source_account_mappings")
	if err != nil {
		return err
	}
	payload, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	row := mappingRow{TenantID: tenant, Provider: Provider, SourceCompanyID: source, SourceAccountExternalID: sourceAccount, TargetAccountID: target, Decision: payload}
	return t.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}, {Name: "provider"}, {Name: "source_company_id"}, {Name: "source_account_external_id"}}, DoUpdates: clause.AssignmentColumns([]string{"target_account_id", "decision"})}).Create(&row).Error
}

// ListAppliedPostings reads exact posted identities from durable executor
// state; it never reconstructs them from a preview plan or financial amounts.
func (r *GORMRepository) ListAppliedPostings(ctx context.Context, schema, tenant, source, packageID, previewID string) ([]AppliedIdentity, error) {
	t, err := r.table(ctx, schema, "smartaccounts_financial_postings")
	if err != nil {
		return nil, err
	}
	var rows []postingRow
	q := t.Where("tenant_id = ? AND provider = ? AND source_company_id = ? AND package_id = ? AND status = ?", tenant, Provider, source, packageID, PlanStatusApplied)
	if previewID != "" {
		q = q.Where("preview_id = ?", previewID)
	}
	if err := q.Order("external_id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	identities := make([]AppliedIdentity, 0, len(rows))
	for _, row := range rows {
		if row.JournalEntryID == "" {
			return nil, errors.New("SmartAccounts applied posting identity is incomplete")
		}
		if row.AppliedBy == "" {
			return nil, errors.New("SmartAccounts applied posting actor is incomplete")
		}
		identities = append(identities, AppliedIdentity{ExternalID: row.ExternalID, Revision: row.Revision, ReservationID: row.ID, JournalID: row.JournalEntryID, AppliedBy: row.AppliedBy})
	}
	return identities, nil
}
