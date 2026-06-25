package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

// ErrRefreshSessionInvalid is returned when a refresh token session is missing, expired, or revoked.
var ErrRefreshSessionInvalid = errors.New("refresh session invalid")

// RefreshSessionService stores and revokes refresh-token sessions.
type RefreshSessionService struct {
	db  *gorm.DB
	now func() time.Time
}

// RefreshSession describes a stored refresh-token session without exposing token material.
type RefreshSession struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// NewRefreshSessionService creates a refresh session service backed by PostgreSQL.
func NewRefreshSessionService(pool *pgxpool.Pool) *RefreshSessionService {
	gormDB, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create refresh session GORM repository: %w", err))
	}
	return &RefreshSessionService{
		db:  gormDB,
		now: time.Now,
	}
}

// CreateRefreshSession stores a newly issued refresh token session.
func (s *RefreshSessionService) CreateRefreshSession(ctx context.Context, userID, tokenID, tokenHash string, expiresAt time.Time) error {
	now := s.now().UTC()
	return s.db.WithContext(ctx).Create(&models.RefreshSession{
		ID:        tokenID,
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: now,
		ExpiresAt: expiresAt.UTC(),
	}).Error
}

// RotateRefreshSession revokes an active refresh session and stores its replacement.
func (s *RefreshSessionService) RotateRefreshSession(ctx context.Context, userID, oldTokenID, oldTokenHash, newTokenID, newTokenHash string, newExpiresAt time.Time) error {
	now := s.now().UTC()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.RefreshSession{}).
			Where("id = ? AND user_id = ? AND token_hash = ?", oldTokenID, userID, oldTokenHash).
			Where("revoked_at IS NULL AND expires_at > ?", now).
			Updates(map[string]interface{}{
				"revoked_at":   now,
				"last_used_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRefreshSessionInvalid
		}

		return tx.Create(&models.RefreshSession{
			ID:        newTokenID,
			UserID:    userID,
			TokenHash: newTokenHash,
			CreatedAt: now,
			ExpiresAt: newExpiresAt.UTC(),
		}).Error
	})
}

// RevokeRefreshSession revokes a single active refresh session.
func (s *RefreshSessionService) RevokeRefreshSession(ctx context.Context, userID, tokenID, tokenHash string) error {
	now := s.now().UTC()
	result := s.db.WithContext(ctx).Model(&models.RefreshSession{}).
		Where("id = ? AND user_id = ? AND token_hash = ?", tokenID, userID, tokenHash).
		Where("revoked_at IS NULL AND expires_at > ?", now).
		Updates(map[string]interface{}{
			"revoked_at":   now,
			"last_used_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRefreshSessionInvalid
	}
	return nil
}

// ListRefreshSessions returns refresh sessions for a user.
func (s *RefreshSessionService) ListRefreshSessions(ctx context.Context, userID string, includeInactive bool) ([]RefreshSession, error) {
	now := s.now().UTC()

	query := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if !includeInactive {
		query = query.Where("revoked_at IS NULL AND expires_at > ?", now)
	}

	var sessionModels []models.RefreshSession
	if err := query.Order("created_at DESC").Find(&sessionModels).Error; err != nil {
		return nil, err
	}

	sessions := make([]RefreshSession, len(sessionModels))
	for i, sessionModel := range sessionModels {
		sessions[i] = modelToRefreshSession(&sessionModel)
	}
	return sessions, nil
}

// RevokeRefreshSessionByID revokes an active refresh session by id for a user.
func (s *RefreshSessionService) RevokeRefreshSessionByID(ctx context.Context, userID, tokenID string) error {
	now := s.now().UTC()
	result := s.db.WithContext(ctx).Model(&models.RefreshSession{}).
		Where("id = ? AND user_id = ?", tokenID, userID).
		Where("revoked_at IS NULL AND expires_at > ?", now).
		Update("revoked_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRefreshSessionInvalid
	}
	return nil
}

// RevokeAllRefreshSessions revokes all active refresh sessions for a user.
func (s *RefreshSessionService) RevokeAllRefreshSessions(ctx context.Context, userID string) error {
	now := s.now().UTC()
	return s.db.WithContext(ctx).Model(&models.RefreshSession{}).
		Where("user_id = ?", userID).
		Where("revoked_at IS NULL AND expires_at > ?", now).
		Update("revoked_at", now).Error
}

func modelToRefreshSession(session *models.RefreshSession) RefreshSession {
	return RefreshSession{
		ID:         session.ID,
		UserID:     session.UserID,
		CreatedAt:  session.CreatedAt,
		LastUsedAt: session.LastUsedAt,
		ExpiresAt:  session.ExpiresAt,
		RevokedAt:  session.RevokedAt,
	}
}
