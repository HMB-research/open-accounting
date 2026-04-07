package apitoken

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTokenNotFound = fmt.Errorf("api token not found")
)

// Repository defines API token storage operations.
type Repository interface {
	CreateToken(ctx context.Context, token *APIToken, tokenHash string) error
	ListTokens(ctx context.Context, userID, tenantID string) ([]APIToken, error)
	RevokeToken(ctx context.Context, userID, tenantID, tokenID string, revokedAt time.Time) error
	GetValidationRecord(ctx context.Context, tokenHash string, now time.Time) (*ValidationRecord, error)
	TouchToken(ctx context.Context, tokenID string, lastUsedAt time.Time) error
}

// PostgresRepository stores API tokens in PostgreSQL.
type PostgresRepository struct {
	db *pgxpool.Pool
}

// NewPostgresRepository creates a PostgreSQL-backed API token repository.
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateToken(ctx context.Context, token *APIToken, tokenHash string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO api_tokens (
			id, tenant_id, user_id, name, token_hash, token_prefix, last_used_at, expires_at, revoked_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		token.ID,
		token.TenantID,
		token.UserID,
		token.Name,
		tokenHash,
		token.TokenPrefix,
		token.LastUsedAt,
		token.ExpiresAt,
		token.RevokedAt,
		token.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create api token: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListTokens(ctx context.Context, userID, tenantID string) ([]APIToken, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, tenant_id, user_id, name, token_prefix, last_used_at, expires_at, revoked_at, created_at
		FROM api_tokens
		WHERE user_id = $1 AND tenant_id = $2 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list api tokens: %w", err)
	}
	defer rows.Close()

	tokens := make([]APIToken, 0)
	for rows.Next() {
		var token APIToken
		if err := rows.Scan(
			&token.ID,
			&token.TenantID,
			&token.UserID,
			&token.Name,
			&token.TokenPrefix,
			&token.LastUsedAt,
			&token.ExpiresAt,
			&token.RevokedAt,
			&token.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api token: %w", err)
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate api tokens: %w", err)
	}

	return tokens, nil
}

func (r *PostgresRepository) RevokeToken(ctx context.Context, userID, tenantID, tokenID string, revokedAt time.Time) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE api_tokens
		SET revoked_at = $1
		WHERE id = $2 AND user_id = $3 AND tenant_id = $4 AND revoked_at IS NULL
	`, revokedAt, tokenID, userID, tenantID)
	if err != nil {
		return fmt.Errorf("revoke api token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTokenNotFound
	}
	return nil
}

func (r *PostgresRepository) GetValidationRecord(ctx context.Context, tokenHash string, now time.Time) (*ValidationRecord, error) {
	var record ValidationRecord
	err := r.db.QueryRow(ctx, `
		SELECT
			at.id,
			at.tenant_id,
			at.user_id,
			at.name,
			at.token_prefix,
			at.last_used_at,
			at.expires_at,
			at.revoked_at,
			at.created_at,
			u.email,
			tu.role
		FROM api_tokens at
		JOIN users u ON u.id = at.user_id AND u.is_active = true
		JOIN tenants t ON t.id = at.tenant_id AND t.is_active = true
		JOIN tenant_users tu ON tu.tenant_id = at.tenant_id AND tu.user_id = at.user_id
		WHERE at.token_hash = $1
			AND at.revoked_at IS NULL
			AND (at.expires_at IS NULL OR at.expires_at > $2)
	`, tokenHash, now).Scan(
		&record.ID,
		&record.TenantID,
		&record.UserID,
		&record.Name,
		&record.TokenPrefix,
		&record.LastUsedAt,
		&record.ExpiresAt,
		&record.RevokedAt,
		&record.CreatedAt,
		&record.Email,
		&record.Role,
	)
	if err == pgx.ErrNoRows {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api token validation record: %w", err)
	}
	return &record, nil
}

func (r *PostgresRepository) TouchToken(ctx context.Context, tokenID string, lastUsedAt time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE api_tokens
		SET last_used_at = $1
		WHERE id = $2
	`, lastUsedAt, tokenID)
	if err != nil {
		return fmt.Errorf("touch api token: %w", err)
	}
	return nil
}
