package documents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error)
	CreateDocument(ctx context.Context, schemaName string, doc *Document) error
	ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error)
	ListReviewSummaries(ctx context.Context, schemaName, tenantID, entityType string, entityIDs []string) (map[string]ReviewSummary, error)
	GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error)
	MarkDocumentReviewed(ctx context.Context, schemaName, tenantID, documentID, reviewedBy string, reviewedAt time.Time) error
	DeleteDocument(ctx context.Context, schemaName, tenantID, documentID string) error
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error) {
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
			notes, retention_until, review_status, reviewed_by, reviewed_at, uploaded_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, table), doc.ID, doc.TenantID, doc.EntityType, doc.EntityID, doc.DocumentType, doc.FileName, doc.ContentType, doc.FileSize, doc.StorageKey, doc.Notes, doc.RetentionUntil, doc.ReviewStatus, doc.ReviewedBy, doc.ReviewedAt, doc.UploadedBy, doc.CreatedAt)
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
		       COALESCE(notes, ''), retention_until, review_status, reviewed_by, reviewed_at, uploaded_by, created_at
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
			&doc.ReviewStatus, &doc.ReviewedBy, &doc.ReviewedAt, &doc.UploadedBy, &doc.CreatedAt,
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
	queryArgs := make([]any, 0, len(entityIDs)+4)
	queryArgs = append(queryArgs, ReviewStatusPending, ReviewStatusReviewed, tenantID, entityType)
	for idx, entityID := range entityIDs {
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx+5))
		queryArgs = append(queryArgs, entityID)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT entity_id::text,
		       COUNT(*)::int,
		       COUNT(*) FILTER (WHERE review_status = $1)::int,
		       COUNT(*) FILTER (WHERE review_status = $2)::int
		FROM %s
		WHERE tenant_id = $3 AND entity_type = $4 AND entity_id IN (%s)
		GROUP BY entity_id
	`, table, strings.Join(placeholders, ", ")), queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("list review summaries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var summary ReviewSummary
		if err := rows.Scan(&summary.EntityID, &summary.TotalCount, &summary.PendingReviewCount, &summary.ReviewedCount); err != nil {
			return nil, fmt.Errorf("scan review summary: %w", err)
		}
		summary.EntityType = entityType
		summary.MissingEvidence = summary.TotalCount == 0
		summary.HasPendingReview = summary.PendingReviewCount > 0
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
		       COALESCE(notes, ''), retention_until, review_status, reviewed_by, reviewed_at, uploaded_by, created_at
		FROM %s
		WHERE tenant_id = $1 AND id = $2
	`, table), tenantID, documentID).Scan(
		&doc.ID, &doc.TenantID, &doc.EntityType, &doc.EntityID, &doc.DocumentType, &doc.FileName,
		&doc.ContentType, &doc.FileSize, &doc.StorageKey, &doc.Notes, &doc.RetentionUntil,
		&doc.ReviewStatus, &doc.ReviewedBy, &doc.ReviewedAt, &doc.UploadedBy, &doc.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	return &doc, nil
}

func (r *PostgresRepository) MarkDocumentReviewed(ctx context.Context, schemaName, tenantID, documentID, reviewedBy string, reviewedAt time.Time) error {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return fmt.Errorf("qualify documents table: %w", err)
	}

	result, err := r.db.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET review_status = $1, reviewed_by = $2, reviewed_at = $3
		WHERE tenant_id = $4 AND id = $5
	`, table), ReviewStatusReviewed, reviewedBy, reviewedAt, tenantID, documentID)
	if err != nil {
		return fmt.Errorf("mark document reviewed: %w", err)
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
	default:
		return "", fmt.Errorf("unsupported document entity type")
	}
}
