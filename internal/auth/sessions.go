package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrRefreshSessionInvalid is returned when a refresh token session is missing, expired, or revoked.
var ErrRefreshSessionInvalid = errors.New("refresh session invalid")

// RefreshSessionService stores and revokes refresh-token sessions.
type RefreshSessionService struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewRefreshSessionService creates a refresh session service backed by PostgreSQL.
func NewRefreshSessionService(pool *pgxpool.Pool) *RefreshSessionService {
	return &RefreshSessionService{
		pool: pool,
		now:  time.Now,
	}
}

// CreateRefreshSession stores a newly issued refresh token session.
func (s *RefreshSessionService) CreateRefreshSession(ctx context.Context, userID, tokenID, tokenHash string, expiresAt time.Time) error {
	now := s.now().UTC()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_sessions (id, user_id, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, tokenID, userID, tokenHash, now, expiresAt.UTC())
	return err
}

// RotateRefreshSession revokes an active refresh session and stores its replacement.
func (s *RefreshSessionService) RotateRefreshSession(ctx context.Context, userID, oldTokenID, oldTokenHash, newTokenID, newTokenHash string, newExpiresAt time.Time) error {
	now := s.now().UTC()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit path owns the successful transaction.

	tag, err := tx.Exec(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = $4, last_used_at = $4
		WHERE id = $1
			AND user_id = $2
			AND token_hash = $3
			AND revoked_at IS NULL
			AND expires_at > $4
	`, oldTokenID, userID, oldTokenHash, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrRefreshSessionInvalid
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_sessions (id, user_id, token_hash, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, newTokenID, userID, newTokenHash, now, newExpiresAt.UTC())
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// RevokeRefreshSession revokes a single active refresh session.
func (s *RefreshSessionService) RevokeRefreshSession(ctx context.Context, userID, tokenID, tokenHash string) error {
	now := s.now().UTC()
	tag, err := s.pool.Exec(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = $4, last_used_at = $4
		WHERE id = $1
			AND user_id = $2
			AND token_hash = $3
			AND revoked_at IS NULL
			AND expires_at > $4
	`, tokenID, userID, tokenHash, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrRefreshSessionInvalid
	}
	return nil
}
