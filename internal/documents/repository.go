package documents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

type Repository interface {
	EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error)
	CreateDocument(ctx context.Context, schemaName string, doc *Document) error
	ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error)
	ListReviewQueueDocuments(ctx context.Context, schemaName, tenantID string, filter ReviewQueueFilter) ([]Document, error)
	ListRetentionReviewDocuments(ctx context.Context, schemaName, tenantID string, cutoff time.Time, includeMissing bool) ([]Document, error)
	ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]ReviewSummary, error)
	GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error)
	DocumentHasSupersededDependents(ctx context.Context, schemaName, tenantID, documentID string) (bool, error)
	UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) error
	UpdateDocumentLifecycle(ctx context.Context, schemaName, tenantID, documentID, lifecycleStatus, lifecycleNote, lifecycleBy string, lifecycleAt time.Time, supersededBy *string) error
	UpdateDocumentLegalHold(ctx context.Context, schemaName, tenantID, documentID string, legalHold bool, note, actionedBy string, actionedAt time.Time) error
	ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewStatus, reviewNote, reviewedBy string, reviewedAt time.Time) error
	DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error
}

type GORMRepository struct {
	db *gorm.DB
}

var newGormDBFromPool = database.NewGormDBFromPool

func NewRepository(db *pgxpool.Pool) *GORMRepository {
	if db == nil {
		return &GORMRepository{}
	}
	gormDB, err := newGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create documents GORM repository: %w", err))
	}
	return NewGORMRepository(gormDB)
}

func NewGORMRepository(db *gorm.DB) *GORMRepository {
	return &GORMRepository{db: db}
}

func (r *GORMRepository) tenantTable(ctx context.Context, schemaName, tableName string) (*gorm.DB, error) {
	if r.db == nil {
		return nil, fmt.Errorf("documents repository database is not configured")
	}
	return database.TenantTable(r.db.WithContext(ctx), schemaName, tableName)
}

func (r *GORMRepository) documentsTable(ctx context.Context, schemaName string) (*gorm.DB, error) {
	return r.tenantTable(ctx, schemaName, "documents")
}

func (r *GORMRepository) EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error) {
	if strings.TrimSpace(strings.ToLower(entityType)) == EntityTypeYearEndClose {
		if _, err := uuid.Parse(strings.TrimSpace(entityID)); err != nil {
			return false, nil
		}
		return true, nil
	}

	tableName, err := entityTableName(entityType)
	if err != nil {
		return false, err
	}

	db, err := r.tenantTable(ctx, schemaName, tableName)
	if err != nil {
		return false, fmt.Errorf("qualify entity table: %w", err)
	}

	var count int64
	if err := db.Where("id = ? AND tenant_id = ?", entityID, tenantID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check entity exists: %w", err)
	}
	return count > 0, nil
}

func (r *GORMRepository) CreateDocument(ctx context.Context, schemaName string, doc *Document) error {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	if err := db.Create(documentToModel(doc)).Error; err != nil {
		return fmt.Errorf("create document: %w", err)
	}
	return nil
}

func (r *GORMRepository) ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error) {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	var docModels []models.Document
	if err := db.
		Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenantID, entityType, entityID).
		Order("created_at DESC, file_name ASC").
		Find(&docModels).Error; err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	return modelsToDocuments(docModels), nil
}

func (r *GORMRepository) ListReviewQueueDocuments(ctx context.Context, schemaName, tenantID string, filter ReviewQueueFilter) ([]Document, error) {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	query := db.Where("tenant_id = ?", tenantID)
	if strings.TrimSpace(filter.EntityType) != "" {
		query = query.Where("entity_type = ?", strings.TrimSpace(filter.EntityType))
	}
	if strings.TrimSpace(filter.DocumentType) != "" {
		query = query.Where("document_type = ?", strings.TrimSpace(filter.DocumentType))
	}
	if strings.TrimSpace(filter.ReviewStatus) != "" {
		query = query.Where("review_status = ?", strings.TrimSpace(filter.ReviewStatus))
	}

	var docModels []models.Document
	if err := query.
		Order("CASE review_status WHEN 'PENDING' THEN 0 WHEN 'REJECTED' THEN 1 WHEN 'REVIEWED' THEN 2 WHEN 'APPROVED' THEN 3 ELSE 4 END").
		Order("created_at ASC").
		Order("file_name ASC").
		Limit(filter.Limit).
		Find(&docModels).Error; err != nil {
		return nil, fmt.Errorf("list document review queue: %w", err)
	}
	return modelsToDocuments(docModels), nil
}

func (r *GORMRepository) ListRetentionReviewDocuments(ctx context.Context, schemaName, tenantID string, cutoff time.Time, includeMissing bool) ([]Document, error) {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	query := db.Where("tenant_id = ?", tenantID)
	if includeMissing {
		query = query.Where("retention_until <= ? OR retention_until IS NULL", cutoff)
	} else {
		query = query.Where("retention_until <= ?", cutoff)
	}

	var docModels []models.Document
	if err := query.
		Order("retention_until ASC NULLS FIRST").
		Order("created_at DESC").
		Order("file_name ASC").
		Find(&docModels).Error; err != nil {
		return nil, fmt.Errorf("list retention review documents: %w", err)
	}
	return modelsToDocuments(docModels), nil
}

func (r *GORMRepository) ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]ReviewSummary, error) {
	summaries := make(map[string]ReviewSummary, len(entityIDs))
	if len(entityIDs) == 0 {
		return summaries, nil
	}

	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	var rows []struct {
		EntityID           string
		TotalCount         int
		PendingReviewCount int
		ReviewedCount      int
		ApprovedCount      int
		RejectedCount      int
	}
	if err := db.
		Select(`
			entity_id::text AS entity_id,
			COUNT(*)::int AS total_count,
			COUNT(*) FILTER (WHERE review_status = ?)::int AS pending_review_count,
			COUNT(*) FILTER (WHERE review_status IN (?, ?, ?))::int AS reviewed_count,
			COUNT(*) FILTER (WHERE review_status = ?)::int AS approved_count,
			COUNT(*) FILTER (WHERE review_status = ?)::int AS rejected_count
		`, ReviewStatusPending, ReviewStatusReviewed, ReviewStatusApproved, ReviewStatusRejected, ReviewStatusApproved, ReviewStatusRejected).
		Where("tenant_id = ? AND entity_type = ? AND entity_id IN ?", tenantID, entityType, entityIDs).
		Group("entity_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list review summaries: %w", err)
	}

	for _, row := range rows {
		summary := ReviewSummary{
			EntityType:         entityType,
			EntityID:           row.EntityID,
			TotalCount:         row.TotalCount,
			PendingReviewCount: row.PendingReviewCount,
			ReviewedCount:      row.ReviewedCount,
			ApprovedCount:      row.ApprovedCount,
			RejectedCount:      row.RejectedCount,
		}
		summary.MissingEvidence = summary.TotalCount == 0
		summary.HasPendingReview = summary.PendingReviewCount > 0
		summary.HasRejected = summary.RejectedCount > 0
		summaries[summary.EntityID] = summary
	}

	return summaries, nil
}

func (r *GORMRepository) GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error) {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	var docModel models.Document
	err = db.Where("tenant_id = ? AND id = ?", tenantID, documentID).First(&docModel).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	return modelToDocument(&docModel), nil
}

func (r *GORMRepository) DocumentHasSupersededDependents(ctx context.Context, schemaName, tenantID, documentID string) (bool, error) {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return false, fmt.Errorf("qualify documents table: %w", err)
	}

	var count int64
	if err := db.
		Where("tenant_id = ? AND superseded_by_document_id = ?", tenantID, documentID).
		Limit(1).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("check document supersession dependents: %w", err)
	}
	return count > 0, nil
}

func (r *GORMRepository) UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) error {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result := db.Where("tenant_id = ? AND id = ?", tenantID, documentID).
		Update("retention_until", retentionUntil)
	if result.Error != nil {
		return fmt.Errorf("update document retention: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func (r *GORMRepository) UpdateDocumentLifecycle(ctx context.Context, schemaName, tenantID, documentID, lifecycleStatus, lifecycleNote, lifecycleBy string, lifecycleAt time.Time, supersededBy *string) error {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result := db.Where("tenant_id = ? AND id = ?", tenantID, documentID).
		Updates(map[string]interface{}{
			"lifecycle_status":          lifecycleStatus,
			"lifecycle_note":            nilIfEmpty(lifecycleNote),
			"superseded_by_document_id": supersededBy,
			"lifecycle_actioned_by":     nilIfEmpty(lifecycleBy),
			"lifecycle_actioned_at":     lifecycleAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update document lifecycle: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func (r *GORMRepository) UpdateDocumentLegalHold(ctx context.Context, schemaName, tenantID, documentID string, legalHold bool, note, actionedBy string, actionedAt time.Time) error {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result := db.Where("tenant_id = ? AND id = ?", tenantID, documentID).
		Updates(map[string]interface{}{
			"legal_hold":             legalHold,
			"legal_hold_note":        nilIfEmpty(note),
			"legal_hold_actioned_by": nilIfEmpty(actionedBy),
			"legal_hold_actioned_at": actionedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update document legal hold: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func (r *GORMRepository) ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewStatus, reviewNote, reviewedBy string, reviewedAt time.Time) error {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result := db.Where("tenant_id = ? AND id = ?", tenantID, documentID).
		Updates(map[string]interface{}{
			"review_status": reviewStatus,
			"review_note":   gorm.Expr("NULLIF(?, '')", reviewNote),
			"reviewed_by":   reviewedBy,
			"reviewed_at":   reviewedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("review document: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func (r *GORMRepository) DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error {
	db, err := r.documentsTable(ctx, schemaName)
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result := db.Where("tenant_id = ? AND id = ?", tenantID, documentID).Delete(&models.Document{})
	if result.Error != nil {
		return fmt.Errorf("delete document: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("document not found")
	}
	return nil
}

func documentToModel(doc *Document) *models.Document {
	return &models.Document{
		ID:              doc.ID,
		TenantID:        doc.TenantID,
		EntityType:      doc.EntityType,
		EntityID:        doc.EntityID,
		DocumentType:    doc.DocumentType,
		FileName:        doc.FileName,
		ContentType:     doc.ContentType,
		FileSize:        doc.FileSize,
		StorageKey:      doc.StorageKey,
		Notes:           doc.Notes,
		RetentionUntil:  doc.RetentionUntil,
		ReviewStatus:    doc.ReviewStatus,
		ReviewNote:      nilIfEmpty(doc.ReviewNote),
		ReviewedBy:      doc.ReviewedBy,
		ReviewedAt:      doc.ReviewedAt,
		LifecycleStatus: normalizeStoredLifecycleStatus(doc.LifecycleStatus),
		LifecycleNote:   nilIfEmpty(doc.LifecycleNote),
		SupersededBy:    doc.SupersededBy,
		LifecycleBy:     doc.LifecycleBy,
		LifecycleAt:     doc.LifecycleAt,
		LegalHold:       doc.LegalHold,
		LegalHoldNote:   nilIfEmpty(doc.LegalHoldNote),
		LegalHoldBy:     doc.LegalHoldBy,
		LegalHoldAt:     doc.LegalHoldAt,
		UploadedBy:      doc.UploadedBy,
		CreatedAt:       doc.CreatedAt,
	}
}

func modelToDocument(doc *models.Document) *Document {
	return &Document{
		ID:              doc.ID,
		TenantID:        doc.TenantID,
		EntityType:      doc.EntityType,
		EntityID:        doc.EntityID,
		DocumentType:    doc.DocumentType,
		FileName:        doc.FileName,
		ContentType:     doc.ContentType,
		FileSize:        doc.FileSize,
		StorageKey:      doc.StorageKey,
		Notes:           doc.Notes,
		RetentionUntil:  doc.RetentionUntil,
		ReviewStatus:    doc.ReviewStatus,
		ReviewNote:      valueOrEmpty(doc.ReviewNote),
		ReviewedBy:      doc.ReviewedBy,
		ReviewedAt:      doc.ReviewedAt,
		LifecycleStatus: normalizeStoredLifecycleStatus(doc.LifecycleStatus),
		LifecycleNote:   valueOrEmpty(doc.LifecycleNote),
		SupersededBy:    doc.SupersededBy,
		LifecycleBy:     doc.LifecycleBy,
		LifecycleAt:     doc.LifecycleAt,
		LegalHold:       doc.LegalHold,
		LegalHoldNote:   valueOrEmpty(doc.LegalHoldNote),
		LegalHoldBy:     doc.LegalHoldBy,
		LegalHoldAt:     doc.LegalHoldAt,
		UploadedBy:      doc.UploadedBy,
		CreatedAt:       doc.CreatedAt,
	}
}

func modelsToDocuments(docModels []models.Document) []Document {
	docs := make([]Document, len(docModels))
	for i := range docModels {
		docs[i] = *modelToDocument(&docModels[i])
	}
	return docs
}

func nilIfEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func normalizeStoredLifecycleStatus(value string) string {
	trimmed := strings.TrimSpace(strings.ToUpper(value))
	if trimmed == "" {
		return LifecycleStatusActive
	}
	return trimmed
}

func entityTableName(entityType string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(entityType)) {
	case EntityTypeInvoice:
		return "invoices", nil
	case EntityTypeJournalEntry:
		return "journal_entries", nil
	case EntityTypePayment:
		return "payments", nil
	case EntityTypeBankTxn:
		return "bank_transactions", nil
	case EntityTypeAsset:
		return "fixed_assets", nil
	case EntityTypeExpense:
		return "expenses", nil
	case EntityTypeQuote:
		return "quotes", nil
	case EntityTypeOrder:
		return "orders", nil
	case EntityTypeYearEndClose:
		return "", nil
	case EntityTypeLeaveRecord:
		return "leave_records", nil
	case EntityTypeTSD:
		return "tsd_declarations", nil
	case EntityTypeKMD:
		return "kmd_declarations", nil
	default:
		return "", fmt.Errorf("unsupported document entity type")
	}
}
