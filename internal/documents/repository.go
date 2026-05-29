package documents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error)
	CreateDocument(ctx context.Context, schemaName string, doc *Document) error
	ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error)
	ListReviewQueueDocuments(ctx context.Context, schemaName, tenantID string, filter ReviewQueueFilter) ([]Document, error)
	ListRetentionReviewDocuments(ctx context.Context, schemaName, tenantID string, cutoff time.Time, includeMissing bool) ([]Document, error)
	ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]ReviewSummary, error)
	GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error)
	UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) error
	ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewStatus, reviewNote, reviewedBy string, reviewedAt time.Time) error
	DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error) {
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

	table, err := database.QualifiedTable(schemaName, tableName)
	if err != nil {
		return false, fmt.Errorf("qualify entity table: %w", err)
	}

	var exists bool
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1
			FROM %s
			WHERE id = $1 AND tenant_id = $2
		)
	`, table), entityID, tenantID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check entity exists: %w", err)
	}

	return exists, nil
}

func (r *PostgresRepository) CreateDocument(ctx context.Context, schemaName string, doc *Document) error {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	_, err = r.db.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			id, tenant_id, entity_type, entity_id, document_type, file_name, content_type, file_size, storage_key,
			notes, retention_until, review_status, review_note, reviewed_by, reviewed_at, uploaded_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULLIF($13, ''), $14, $15, $16, $17)
	`, table), doc.ID, doc.TenantID, doc.EntityType, doc.EntityID, doc.DocumentType, doc.FileName, doc.ContentType, doc.FileSize, doc.StorageKey, doc.Notes, doc.RetentionUntil, doc.ReviewStatus, doc.ReviewNote, doc.ReviewedBy, doc.ReviewedAt, doc.UploadedBy, doc.CreatedAt)
	if err != nil {
		return fmt.Errorf("create document: %w", err)
	}

	return nil
}

func (r *PostgresRepository) ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error) {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, entity_type, entity_id, document_type, file_name, content_type, file_size, storage_key,
		       COALESCE(notes, ''), retention_until, review_status, COALESCE(review_note, ''), reviewed_by, reviewed_at, uploaded_by, created_at
		FROM %s
		WHERE tenant_id = $1 AND entity_type = $2 AND entity_id = $3
		ORDER BY created_at DESC, file_name ASC
	`, table), tenantID, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(
			&doc.ID, &doc.TenantID, &doc.EntityType, &doc.EntityID, &doc.DocumentType, &doc.FileName,
			&doc.ContentType, &doc.FileSize, &doc.StorageKey, &doc.Notes, &doc.RetentionUntil,
			&doc.ReviewStatus, &doc.ReviewNote, &doc.ReviewedBy, &doc.ReviewedAt, &doc.UploadedBy, &doc.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		docs = append(docs, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}

	return docs, nil
}

func (r *PostgresRepository) ListReviewQueueDocuments(ctx context.Context, schemaName, tenantID string, filter ReviewQueueFilter) ([]Document, error) {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	conditions := []string{"tenant_id = $1"}
	queryArgs := []any{tenantID}
	if strings.TrimSpace(filter.EntityType) != "" {
		queryArgs = append(queryArgs, strings.TrimSpace(filter.EntityType))
		conditions = append(conditions, fmt.Sprintf("entity_type = $%d", len(queryArgs)))
	}
	if strings.TrimSpace(filter.DocumentType) != "" {
		queryArgs = append(queryArgs, strings.TrimSpace(filter.DocumentType))
		conditions = append(conditions, fmt.Sprintf("document_type = $%d", len(queryArgs)))
	}
	if strings.TrimSpace(filter.ReviewStatus) != "" {
		queryArgs = append(queryArgs, strings.TrimSpace(filter.ReviewStatus))
		conditions = append(conditions, fmt.Sprintf("review_status = $%d", len(queryArgs)))
	}
	queryArgs = append(queryArgs, ReviewStatusPending, ReviewStatusRejected, ReviewStatusReviewed, ReviewStatusApproved, filter.Limit)
	pendingStatusPlaceholder := len(queryArgs) - 4
	rejectedStatusPlaceholder := len(queryArgs) - 3
	reviewedStatusPlaceholder := len(queryArgs) - 2
	approvedStatusPlaceholder := len(queryArgs) - 1
	limitPlaceholder := len(queryArgs)

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, entity_type, entity_id, document_type, file_name, content_type, file_size, storage_key,
		       COALESCE(notes, ''), retention_until, review_status, COALESCE(review_note, ''), reviewed_by, reviewed_at, uploaded_by, created_at
		FROM %s
		WHERE %s
		ORDER BY
			CASE review_status
				WHEN $%d THEN 0
				WHEN $%d THEN 1
				WHEN $%d THEN 2
				WHEN $%d THEN 3
				ELSE 4
			END,
			created_at ASC,
			file_name ASC
		LIMIT $%d
	`, table, strings.Join(conditions, " AND "), pendingStatusPlaceholder, rejectedStatusPlaceholder, reviewedStatusPlaceholder, approvedStatusPlaceholder, limitPlaceholder), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list document review queue: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocumentRows(rows, "review queue document")
	if err != nil {
		return nil, err
	}

	return docs, nil
}

func (r *PostgresRepository) ListRetentionReviewDocuments(ctx context.Context, schemaName, tenantID string, cutoff time.Time, includeMissing bool) ([]Document, error) {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, entity_type, entity_id, document_type, file_name, content_type, file_size, storage_key,
		       COALESCE(notes, ''), retention_until, review_status, COALESCE(review_note, ''), reviewed_by, reviewed_at, uploaded_by, created_at
		FROM %s
		WHERE tenant_id = $1
		  AND (retention_until <= $2 OR ($3::boolean AND retention_until IS NULL))
		ORDER BY retention_until ASC NULLS FIRST, created_at DESC, file_name ASC
	`, table), tenantID, cutoff, includeMissing)
	if err != nil {
		return nil, fmt.Errorf("list retention review documents: %w", err)
	}
	defer rows.Close()

	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(
			&doc.ID, &doc.TenantID, &doc.EntityType, &doc.EntityID, &doc.DocumentType, &doc.FileName,
			&doc.ContentType, &doc.FileSize, &doc.StorageKey, &doc.Notes, &doc.RetentionUntil,
			&doc.ReviewStatus, &doc.ReviewNote, &doc.ReviewedBy, &doc.ReviewedAt, &doc.UploadedBy, &doc.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan retention review document: %w", err)
		}
		docs = append(docs, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate retention review documents: %w", err)
	}

	return docs, nil
}

func scanDocumentRows(rows pgx.Rows, label string) ([]Document, error) {
	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(
			&doc.ID, &doc.TenantID, &doc.EntityType, &doc.EntityID, &doc.DocumentType, &doc.FileName,
			&doc.ContentType, &doc.FileSize, &doc.StorageKey, &doc.Notes, &doc.RetentionUntil,
			&doc.ReviewStatus, &doc.ReviewNote, &doc.ReviewedBy, &doc.ReviewedAt, &doc.UploadedBy, &doc.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan %s: %w", label, err)
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", label, err)
	}
	return docs, nil
}

func (r *PostgresRepository) ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]ReviewSummary, error) {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	summaries := make(map[string]ReviewSummary, len(entityIDs))
	if len(entityIDs) == 0 {
		return summaries, nil
	}

	placeholders := make([]string, 0, len(entityIDs))
	queryArgs := make([]any, 0, len(entityIDs)+6)
	queryArgs = append(queryArgs, ReviewStatusPending, ReviewStatusReviewed, ReviewStatusApproved, ReviewStatusRejected, tenantID, entityType)
	for idx, entityID := range entityIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx+7))
		queryArgs = append(queryArgs, entityID)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT entity_id::text,
		       COUNT(*)::int,
		       COUNT(*) FILTER (WHERE review_status = $1)::int,
		       COUNT(*) FILTER (WHERE review_status IN ($2, $3, $4))::int,
		       COUNT(*) FILTER (WHERE review_status = $3)::int,
		       COUNT(*) FILTER (WHERE review_status = $4)::int
		FROM %s
		WHERE tenant_id = $5 AND entity_type = $6 AND entity_id IN (%s)
		GROUP BY entity_id
	`, table, strings.Join(placeholders, ", ")), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list review summaries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var summary ReviewSummary
		if err := rows.Scan(&summary.EntityID, &summary.TotalCount, &summary.PendingReviewCount, &summary.ReviewedCount, &summary.ApprovedCount, &summary.RejectedCount); err != nil {
			return nil, fmt.Errorf("scan review summary: %w", err)
		}
		summary.EntityType = entityType
		summary.MissingEvidence = summary.TotalCount == 0
		summary.HasPendingReview = summary.PendingReviewCount > 0
		summary.HasRejected = summary.RejectedCount > 0
		summaries[summary.EntityID] = summary
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review summaries: %w", err)
	}

	return summaries, nil
}

func (r *PostgresRepository) GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error) {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	var doc Document
	err = r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, entity_type, entity_id, document_type, file_name, content_type, file_size, storage_key,
		       COALESCE(notes, ''), retention_until, review_status, COALESCE(review_note, ''), reviewed_by, reviewed_at, uploaded_by, created_at
		FROM %s
		WHERE tenant_id = $1 AND id = $2
	`, table), tenantID, documentID).Scan(
		&doc.ID, &doc.TenantID, &doc.EntityType, &doc.EntityID, &doc.DocumentType, &doc.FileName,
		&doc.ContentType, &doc.FileSize, &doc.StorageKey, &doc.Notes, &doc.RetentionUntil,
		&doc.ReviewStatus, &doc.ReviewNote, &doc.ReviewedBy, &doc.ReviewedAt, &doc.UploadedBy, &doc.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	return &doc, nil
}

func (r *PostgresRepository) UpdateDocumentRetention(ctx context.Context, schemaName, tenantID, documentID string, retentionUntil *time.Time) error {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET retention_until = $1
		WHERE tenant_id = $2 AND id = $3
	`, table), retentionUntil, tenantID, documentID)
	if err != nil {
		return fmt.Errorf("update document retention: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("document not found")
	}

	return nil
}

func (r *PostgresRepository) ReviewDocument(ctx context.Context, schemaName, tenantID, documentID, reviewStatus, reviewNote, reviewedBy string, reviewedAt time.Time) error {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET review_status = $1, review_note = NULLIF($2, ''), reviewed_by = $3, reviewed_at = $4
		WHERE tenant_id = $5 AND id = $6
	`, table), reviewStatus, reviewNote, reviewedBy, reviewedAt, tenantID, documentID)
	if err != nil {
		return fmt.Errorf("review document: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("document not found")
	}

	return nil
}

func (r *PostgresRepository) DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s
		WHERE tenant_id = $1 AND id = $2
	`, table), tenantID, documentID)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("document not found")
	}

	return nil
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
	case EntityTypeYearEndClose:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported document entity type")
	}
}
