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
	defaultPasswordResetHashCost     = 12
)

var (
	newGormDBFromPool       = database.NewGormDBFromPool
	passwordResetRandomRead = rand.Read
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

// PasswordResetServiceOption configures PasswordResetService.
type PasswordResetServiceOption func(*PasswordResetService)

// WithPasswordResetHashCost overrides the bcrypt cost used for new reset-password hashes.
func WithPasswordResetHashCost(cost int) PasswordResetServiceOption {
	return func(s *PasswordResetService) {
		if cost < bcrypt.MinCost {
			cost = bcrypt.MinCost
		}
		if cost > bcrypt.MaxCost {
			cost = bcrypt.MaxCost
		}
		s.passwordHashCost = cost
	}
}

// PasswordResetService manages one-time password reset tokens.
type PasswordResetService struct {
	repo             passwordResetRepository
	now              func() time.Time
	tokenTTL         time.Duration
	requestCooldown  time.Duration
	generateRawToken func() (string, error)
	passwordHashCost int
}

type passwordResetRepository interface {
	RequestPasswordReset(
		ctx context.Context,
		email string,
		now time.Time,
		cooldown time.Duration,
		buildToken func(userID string) (*models.PasswordResetToken, error),
	) (*passwordResetRequestOutcome, error)
	ResetPassword(
		ctx context.Context,
		tokenHash string,
		now time.Time,
		buildPasswordHash func(user *models.User) (string, error),
	) (string, error)
}

type passwordResetRequestOutcome struct {
	UserID    string
	Issued    bool
	Throttled bool
}

type passwordResetGORMRepository struct {
	db *gorm.DB
}

// NewPasswordResetService creates a PostgreSQL-backed password reset service.
func NewPasswordResetService(pool *pgxpool.Pool, opts ...PasswordResetServiceOption) *PasswordResetService {
	gormDB, err := newGormDBFromPool(context.Background(), pool)
	if err != nil {
		panic(fmt.Errorf("create password reset GORM repository: %w", err))
	}
	return newPasswordResetServiceWithRepository(&passwordResetGORMRepository{db: gormDB}, opts...)
}

func newPasswordResetServiceWithRepository(repo passwordResetRepository, opts ...PasswordResetServiceOption) *PasswordResetService {
	service := &PasswordResetService{
		repo:             repo,
		now:              time.Now,
		tokenTTL:         defaultPasswordResetTokenTTL,
		requestCooldown:  defaultPasswordResetCooldown,
		generateRawToken: generatePasswordResetToken,
		passwordHashCost: defaultPasswordResetHashCost,
	}
	for _, opt := range opts {
		opt(service)
	}
	return service
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

	now := s.now().UTC()
	outcome, err := s.repo.RequestPasswordReset(ctx, normalizedEmail, now, s.requestCooldown, func(userID string) (*models.PasswordResetToken, error) {
		token, err := s.generateRawToken()
		if err != nil {
			return nil, fmt.Errorf("generate password reset token: %w", err)
		}

		expiresAt = now.Add(s.tokenTTL)
		issuedToken = token
		return &models.PasswordResetToken{
			ID:             uuid.NewString(),
			UserID:         userID,
			TokenHash:      HashRefreshToken(token),
			RequestedEmail: normalizedEmail,
			RequestIP:      truncateAuditField(requestIP),
			UserAgent:      truncateAuditField(userAgent),
			CreatedAt:      now,
			ExpiresAt:      expiresAt,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	if outcome == nil || !outcome.Issued {
		if outcome != nil {
			result.UserID = outcome.UserID
			result.Throttled = outcome.Throttled
		}
		return result, nil
	}

	result.UserID = outcome.UserID
	result.Throttled = outcome.Throttled
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
	userID, err := s.repo.ResetPassword(ctx, HashRefreshToken(resetToken), now, func(user *models.User) (string, error) {
		if !user.IsActive {
			return "", fmt.Errorf("account is disabled")
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(newPassword)) == nil {
			return "", fmt.Errorf("new password must be different from current password")
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.passwordHashCost)
		if err != nil {
			return "", fmt.Errorf("hash password: %w", err)
		}
		return string(newHash), nil
	})
	if err != nil {
		return "", err
	}
	return userID, nil
}

func (r *passwordResetGORMRepository) RequestPasswordReset(
	ctx context.Context,
	email string,
	now time.Time,
	cooldown time.Duration,
	buildToken func(userID string) (*models.PasswordResetToken, error),
) (*passwordResetRequestOutcome, error) {
	outcome := &passwordResetRequestOutcome{}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("LOWER(email) = ?", email).
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
		outcome.UserID = user.ID

		var recentRequestCount int64
		err = tx.Model(&models.PasswordResetToken{}).
			Where("user_id = ? AND used_at IS NULL AND created_at > ?", user.ID, now.Add(-cooldown)).
			Count(&recentRequestCount).Error
		if err != nil {
			return fmt.Errorf("check password reset cooldown: %w", err)
		}
		if recentRequestCount > 0 {
			outcome.Throttled = true
			return nil
		}

		token, err := buildToken(user.ID)
		if err != nil {
			return err
		}

		err = tx.Model(&models.PasswordResetToken{}).
			Where("user_id = ? AND used_at IS NULL", user.ID).
			Update("used_at", now).Error
		if err != nil {
			return fmt.Errorf("expire previous password reset tokens: %w", err)
		}

		if err := tx.Create(token).Error; err != nil {
			return fmt.Errorf("create password reset token: %w", err)
		}

		outcome.Issued = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return outcome, nil
}

func (r *passwordResetGORMRepository) ResetPassword(
	ctx context.Context,
	tokenHash string,
	now time.Time,
	buildPasswordHash func(user *models.User) (string, error),
) (string, error) {
	var userID string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var resetTokenModel models.PasswordResetToken
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Preload("User").
			Where("token_hash = ? AND used_at IS NULL AND expires_at > ?", tokenHash, now).
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

		newHash, err := buildPasswordHash(user)
		if err != nil {
			return err
		}

		err = tx.Model(&models.User{}).
			Where("id = ?", user.ID).
			Updates(map[string]interface{}{
				"password_hash": newHash,
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
	if _, err := passwordResetRandomRead(raw); err != nil {
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
