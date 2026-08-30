package importsession

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store persists tenant-scoped import receipts. It never receives raw source
// records or credentials.
type Store interface {
	EnsureSourceCompanyBinding(ctx context.Context, tenantID, provider, sourceCompanyID string) error
	Create(ctx context.Context, schemaName, tenantID string, receipt Receipt) (Receipt, error)
	FindByPackage(ctx context.Context, schemaName, tenantID, provider, sourceCompanyID, packageSHA256 string) (Receipt, error)
	Get(ctx context.Context, schemaName, tenantID, sessionID string) (Receipt, error)
	ListLedgerPlanInputs(ctx context.Context, schemaName, tenantID, provider, sourceCompanyID, excludeSessionID string) ([]StagedLedgerJournal, error)
}

// GORMRepository implements Store in a tenant schema.
type GORMRepository struct {
	db *gorm.DB
}

var newGormDBFromPool = database.NewGormDBFromPool

// NewRepository builds an import-session repository from the shared database
// pool. A nil pool is allowed for defensive construction in tests.
func NewRepository(pool *pgxpool.Pool) *GORMRepository {
	if pool == nil {
		return &GORMRepository{}
	}
	db, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create import session repository: %w", err))
	}
	return NewGORMRepository(db)
}

// NewGORMRepository builds an import-session repository from a GORM database.
func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) sessionsTable(ctx context.Context, schemaName string) (*gorm.DB, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("import session database is not configured")
	}
	return database.TenantTable(r.db.WithContext(ctx), schemaName, "import_sessions")
}

// EnsureSourceCompanyBinding atomically establishes or verifies the provider
// company identity binding. The registry is outside tenant schemas so an
// otherwise valid package cannot cross a tenant boundary.
func (r *GORMRepository) EnsureSourceCompanyBinding(ctx context.Context, tenantID, provider, sourceCompanyID string) error {
	if r == nil || r.db == nil {
		return errors.New("import session database is not configured")
	}
	binding := &models.ImportSourceBinding{
		Provider:        strings.TrimSpace(provider),
		SourceCompanyID: strings.TrimSpace(sourceCompanyID),
		TenantID:        strings.TrimSpace(tenantID),
	}
	result := r.db.WithContext(ctx).Table("public.import_source_bindings").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider"}, {Name: "source_company_id"}},
		DoNothing: true,
	}).Create(binding)
	if result.Error != nil {
		return fmt.Errorf("bind source company: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}

	var existing models.ImportSourceBinding
	err := r.db.WithContext(ctx).Table("public.import_source_bindings").
		Where("provider = ? AND source_company_id = ?", binding.Provider, binding.SourceCompanyID).
		First(&existing).Error
	if err != nil {
		return fmt.Errorf("load source company binding: %w", err)
	}
	if existing.TenantID != binding.TenantID {
		return ErrSourceCompanyBoundToOtherTenant
	}
	return nil
}

// Create stores a receipt-only record. Package payloads are intentionally not
// represented by either Receipt or ImportSessionRecord.
func (r *GORMRepository) Create(ctx context.Context, schemaName, tenantID string, receipt Receipt) (Receipt, error) {
	table, err := r.sessionsTable(ctx, schemaName)
	if err != nil {
		return Receipt{}, err
	}
	record, err := receiptToRecord(tenantID, receipt)
	if err != nil {
		return Receipt{}, err
	}
	result := table.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "tenant_id"},
			{Name: "provider"},
			{Name: "source_company_id"},
			{Name: "package_sha256"},
		},
		DoNothing: true,
	}).Create(record)
	if result.Error != nil {
		return Receipt{}, fmt.Errorf("create import session: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		existing, err := r.FindByPackage(ctx, schemaName, tenantID, receipt.Provider, receipt.SourceCompanyID, receipt.PackageSHA256)
		if err != nil {
			return Receipt{}, fmt.Errorf("load idempotent import session receipt: %w", err)
		}
		existing.Created = false
		return existing, nil
	}
	created, err := recordToReceipt(record)
	if err != nil {
		return Receipt{}, err
	}
	created.Created = true
	return created, nil
}

// FindByPackage returns the existing receipt for an idempotent package retry.
func (r *GORMRepository) FindByPackage(ctx context.Context, schemaName, tenantID, provider, sourceCompanyID, packageSHA256 string) (Receipt, error) {
	table, err := r.sessionsTable(ctx, schemaName)
	if err != nil {
		return Receipt{}, err
	}
	var record models.ImportSessionRecord
	err = table.Where(
		"tenant_id = ? AND provider = ? AND source_company_id = ? AND package_sha256 = ?",
		strings.TrimSpace(tenantID), strings.TrimSpace(provider), strings.TrimSpace(sourceCompanyID), strings.TrimSpace(packageSHA256),
	).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Receipt{}, ErrImportSessionNotFound
	}
	if err != nil {
		return Receipt{}, fmt.Errorf("find import session: %w", err)
	}
	return recordToReceipt(&record)
}

// Get returns one tenant-scoped receipt by ID.
func (r *GORMRepository) Get(ctx context.Context, schemaName, tenantID, sessionID string) (Receipt, error) {
	table, err := r.sessionsTable(ctx, schemaName)
	if err != nil {
		return Receipt{}, err
	}
	var record models.ImportSessionRecord
	err = table.Where("tenant_id = ? AND id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(sessionID)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Receipt{}, ErrImportSessionNotFound
	}
	if err != nil {
		return Receipt{}, fmt.Errorf("get import session: %w", err)
	}
	return recordToReceipt(&record)
}

// ListLedgerPlanInputs returns normalized staged journal metadata from other
// receipts of the same tenant/source company. It is read-only and used solely
// to block duplicate or conflicting source revisions during dry-run planning.
func (r *GORMRepository) ListLedgerPlanInputs(ctx context.Context, schemaName, tenantID, provider, sourceCompanyID, excludeSessionID string) ([]StagedLedgerJournal, error) {
	table, err := r.sessionsTable(ctx, schemaName)
	if err != nil {
		return nil, err
	}
	var records []models.ImportSessionRecord
	query := table.Select("ledger_plan_input").Where(
		"tenant_id = ? AND provider = ? AND source_company_id = ? AND id <> ?",
		strings.TrimSpace(tenantID), strings.TrimSpace(provider), strings.TrimSpace(sourceCompanyID), strings.TrimSpace(excludeSessionID),
	)
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list staged ledger plan input: %w", err)
	}
	journals := make([]StagedLedgerJournal, 0)
	for _, record := range records {
		if len(record.LedgerPlanInput) == 0 {
			continue
		}
		var staged []StagedLedgerJournal
		if err := json.Unmarshal(record.LedgerPlanInput, &staged); err != nil {
			return nil, fmt.Errorf("decode staged ledger plan input: %w", err)
		}
		journals = append(journals, staged...)
	}
	return journals, nil
}

func receiptToRecord(tenantID string, receipt Receipt) (*models.ImportSessionRecord, error) {
	entityCounts, err := json.Marshal(cloneEntityCounts(receipt.EntityCounts))
	if err != nil {
		return nil, fmt.Errorf("marshal entity counts: %w", err)
	}
	ledgerVerification, err := json.Marshal(receipt.LedgerVerification)
	if err != nil {
		return nil, fmt.Errorf("marshal ledger verification: %w", err)
	}
	ledgerPlanInput, err := json.Marshal(receipt.LedgerPlanInput)
	if err != nil {
		return nil, fmt.Errorf("marshal ledger plan input: %w", err)
	}
	validation, err := json.Marshal(receipt.Validation)
	if err != nil {
		return nil, fmt.Errorf("marshal validation report: %w", err)
	}
	if strings.TrimSpace(receipt.ID) == "" {
		receipt.ID = uuid.NewString()
	}
	return &models.ImportSessionRecord{
		ID:                 receipt.ID,
		TenantID:           strings.TrimSpace(tenantID),
		Provider:           strings.TrimSpace(receipt.Provider),
		SourceCompanyID:    strings.TrimSpace(receipt.SourceCompanyID),
		SchemaVersion:      strings.TrimSpace(receipt.SchemaVersion),
		PackageSHA256:      strings.TrimSpace(receipt.PackageSHA256),
		Status:             strings.TrimSpace(receipt.Status),
		RecordCount:        receipt.RecordCount,
		EntityCounts:       entityCounts,
		LedgerVerification: ledgerVerification,
		LedgerPlanInput:    ledgerPlanInput,
		Validation:         validation,
		CreatedBy:          strings.TrimSpace(receipt.CreatedBy),
		CreatedAt:          receipt.CreatedAt,
	}, nil
}

func recordToReceipt(record *models.ImportSessionRecord) (Receipt, error) {
	if record == nil {
		return Receipt{}, ErrImportSessionNotFound
	}
	entityCounts := map[string]int{}
	if len(record.EntityCounts) > 0 {
		if err := json.Unmarshal(record.EntityCounts, &entityCounts); err != nil {
			return Receipt{}, fmt.Errorf("decode import session entity counts: %w", err)
		}
	}
	validation := ValidationReport{}
	if len(record.Validation) > 0 {
		if err := json.Unmarshal(record.Validation, &validation); err != nil {
			return Receipt{}, fmt.Errorf("decode import session validation report: %w", err)
		}
	}
	ledgerVerification := LedgerVerification{}
	if len(record.LedgerVerification) > 0 {
		if err := json.Unmarshal(record.LedgerVerification, &ledgerVerification); err != nil {
			return Receipt{}, fmt.Errorf("decode import session ledger verification: %w", err)
		}
	}
	ledgerPlanInput := []StagedLedgerJournal{}
	if len(record.LedgerPlanInput) > 0 {
		if err := json.Unmarshal(record.LedgerPlanInput, &ledgerPlanInput); err != nil {
			return Receipt{}, fmt.Errorf("decode import session ledger plan input: %w", err)
		}
	}
	return Receipt{
		ID:                 record.ID,
		TenantID:           record.TenantID,
		Provider:           record.Provider,
		SourceCompanyID:    record.SourceCompanyID,
		SchemaVersion:      record.SchemaVersion,
		PackageSHA256:      record.PackageSHA256,
		Status:             record.Status,
		RecordCount:        record.RecordCount,
		EntityCounts:       entityCounts,
		LedgerVerification: ledgerVerification,
		LedgerPlanInput:    ledgerPlanInput,
		Validation:         validation,
		CreatedBy:          record.CreatedBy,
		CreatedAt:          record.CreatedAt,
	}, nil
}
