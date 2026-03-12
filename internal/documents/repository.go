package documents

import (
	"context"
	"fmt"
	"strings"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	EntityExists(ctx context.Context, schemaName, tenantID, entityType, entityID string) (bool, error)
	CreateDocument(ctx context.Context, schemaName string, doc *Document) error
	ListDocuments(ctx context.Context, schemaName, tenantID, entityType, entityID string) ([]Document, error)
	GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error)
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
			id, tenant_id, entity_type, entity_id, file_name, content_type, file_size, storage_key, uploaded_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, table), doc.ID, doc.TenantID, doc.EntityType, doc.EntityID, doc.FileName, doc.ContentType, doc.FileSize, doc.StorageKey, doc.UploadedBy, doc.CreatedAt)
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
		SELECT id, tenant_id, entity_type, entity_id, file_name, content_type, file_size, storage_key, uploaded_by, created_at
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
			&doc.ID, &doc.TenantID, &doc.EntityType, &doc.EntityID, &doc.FileName,
			&doc.ContentType, &doc.FileSize, &doc.StorageKey, &doc.UploadedBy, &doc.CreatedAt,
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

func (r *PostgresRepository) GetDocumentByID(ctx context.Context, schemaName, tenantID, documentID string) (*Document, error) {
	table, err := database.QualifiedTable(schemaName, "documents")
	if err != nil {
		return nil, fmt.Errorf("qualify documents table: %w", err)
	}

	var doc Document
	err = r.db.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, entity_type, entity_id, file_name, content_type, file_size, storage_key, uploaded_by, created_at
		FROM %s
		WHERE tenant_id = $1 AND id = $2
	`, table), tenantID, documentID).Scan(
		&doc.ID, &doc.TenantID, &doc.EntityType, &doc.EntityID, &doc.FileName,
		&doc.ContentType, &doc.FileSize, &doc.StorageKey, &doc.UploadedBy, &doc.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("document not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}

	return &doc, nil
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
	default:
		return "", fmt.Errorf("unsupported document entity type")
	}
}
