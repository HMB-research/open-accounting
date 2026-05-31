package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultPasswordResetTokenTTL     = time.Hour
	defaultPasswordResetCooldown     = 5 * time.Minute
	defaultPasswordResetTokenEntropy = 32
)

// ErrPasswordResetTokenInvalid is returned for missing, expired, or already-used reset tokens.
var ErrPasswordResetTokenInvalid = errors.New("password reset token invalid")

// PasswordResetRequestResult describes the outcome of a password reset request.
type PasswordResetRequestResult struct {
	Email     string     `json:"email,omitempty"`
	UserID    string     `json:"-"`
	Issued    bool       `json:"issued"`
	Throttled bool       `json:"throttled,omitempty"`
	Token     string     `json:"reset_token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// PasswordResetService manages one-time password reset tokens.
type PasswordResetService struct {
	db               *gorm.DB
	now              func() time.Time
	tokenTTL         time.Duration
	requestCooldown  time.Duration
	generateRawToken func() (string, error)
}

// NewPasswordResetService creates a PostgreSQL-backed password reset service.
func NewPasswordResetService(pool *pgxpool.Pool) *PasswordResetService {
	gormDB, err := database.NewGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create password reset GORM repository: %w", err))
	}
	return &PasswordResetService{
		db:               gormDB,
		now:              time.Now,
		tokenTTL:         defaultPasswordResetTokenTTL,
		requestCooldown:  defaultPasswordResetCooldown,
		generateRawToken: generatePasswordResetToken,
	}
}

// RequestPasswordReset creates a new one-time reset token for an active user.
func (s *PasswordResetService) RequestPasswordReset(ctx context.Context, email, requestIP, userAgent string) (*PasswordResetRequestResult, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return nil, fmt.Errorf("email is required")
	}

	result := &PasswordResetRequestResult{Email: normalizedEmail}

	var issuedToken string
	var expiresAt time.Time

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("LOWER(email) = ?", normalizedEmail).
			First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find user for password reset: %w", err)
		}
		if !user.IsActive {
			return nil
		}
		result.UserID = user.ID

		now := s.now().UTC()
		var recentRequestCount int64
		err = tx.Model(&models.PasswordResetToken{}).
			Where("user_id = ? AND used_at IS NULL AND created_at > ?", user.ID, now.Add(-s.requestCooldown)).
			Count(&recentRequestCount).Error
		if err != nil {
			return fmt.Errorf("check password reset cooldown: %w", err)
		}
		if recentRequestCount > 0 {
			result.Throttled = true
			return nil
		}

		token, err := s.generateRawToken()
		if err != nil {
			return fmt.Errorf("generate password reset token: %w", err)
		}

		expiresAt = now.Add(s.tokenTTL)
		err = tx.Model(&models.PasswordResetToken{}).
			Where("user_id = ? AND used_at IS NULL", user.ID).
			Update("used_at", now).Error
		if err != nil {
			return fmt.Errorf("expire previous password reset tokens: %w", err)
		}

		err = tx.Create(&models.PasswordResetToken{
			ID:             uuid.NewString(),
			UserID:         user.ID,
			TokenHash:      HashRefreshToken(token),
			RequestedEmail: normalizedEmail,
			RequestIP:      truncateAuditField(requestIP),
			UserAgent:      truncateAuditField(userAgent),
			CreatedAt:      now,
			ExpiresAt:      expiresAt,
		}).Error
		if err != nil {
			return fmt.Errorf("create password reset token: %w", err)
		}

		issuedToken = token
		return nil
	})
	if err != nil {
		return nil, err
	}
	if issuedToken == "" {
		return result, nil
	}

	result.Issued = true
	result.Token = issuedToken
	result.ExpiresAt = &expiresAt
	return result, nil
}

// ResetPassword consumes a valid reset token and stores a new password hash.
func (s *PasswordResetService) ResetPassword(ctx context.Context, resetToken, newPassword string) (string, error) {
	resetToken = strings.TrimSpace(resetToken)
	if resetToken == "" {
		return "", ErrPasswordResetTokenInvalid
	}
	if newPassword == "" {
		return "", fmt.Errorf("new password is required")
	}
	if len(newPassword) < 8 {
		return "", fmt.Errorf("new password must be at least 8 characters")
	}

	now := s.now().UTC()
	var userID string
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var resetTokenModel models.PasswordResetToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("User").
			Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", HashRefreshToken(resetToken), now).
			First(&resetTokenModel).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPasswordResetTokenInvalid
		}
		if err != nil {
			return fmt.Errorf("get password reset token: %w", err)
		}
		if resetTokenModel.User == nil {
			return ErrPasswordResetTokenInvalid
		}
		user := resetTokenModel.User
		userID = user.ID

		if !user.IsActive {
			return fmt.Errorf("account is disabled")
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)) == nil {
			return fmt.Errorf("new password must be different from current password")
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		err = tx.Model(&models.User{}).
			Where("id = ?", user.ID).
			Updates(map[string]interface{}{
				"password_hash": string(newHash),
				"updated_at":    now,
			}).Error
		if err != nil {
			return fmt.Errorf("update user password: %w", err)
		}

		err = tx.Model(&models.PasswordResetToken{}).
			Where("user_id = ? AND used_at IS NULL", user.ID).
			Update("used_at", now).Error
		if err != nil {
			return fmt.Errorf("consume password reset tokens: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return userID, nil
}

func generatePasswordResetToken() (string, error) {
	raw := make([]byte, defaultPasswordResetTokenEntropy)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func truncateAuditField(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 512 {
		return value
	}
	return value[:512]
}
