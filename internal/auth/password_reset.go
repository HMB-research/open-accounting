package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
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
	Issued    bool       `json:"issued"`
	Throttled bool       `json:"throttled,omitempty"`
	Token     string     `json:"reset_token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// PasswordResetService manages one-time password reset tokens.
type PasswordResetService struct {
	pool             *pgxpool.Pool
	now              func() time.Time
	tokenTTL         time.Duration
	requestCooldown  time.Duration
	generateRawToken func() (string, error)
}

// NewPasswordResetService creates a PostgreSQL-backed password reset service.
func NewPasswordResetService(pool *pgxpool.Pool) *PasswordResetService {
	return &PasswordResetService{
		pool:             pool,
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

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin password reset request: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit path owns the successful transaction.

	var userID string
	var isActive bool
	err = tx.QueryRow(ctx, `
		SELECT id::text, is_active
		FROM users
		WHERE LOWER(email) = $1
		FOR UPDATE
	`, normalizedEmail).Scan(&userID, &isActive)
	if err == pgx.ErrNoRows || !isActive {
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find user for password reset: %w", err)
	}

	now := s.now().UTC()
	var recentRequestExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM password_reset_tokens
			WHERE user_id = $1
				AND used_at IS NULL
				AND created_at > $2
		)
	`, userID, now.Add(-s.requestCooldown)).Scan(&recentRequestExists); err != nil {
		return nil, fmt.Errorf("check password reset cooldown: %w", err)
	}
	if recentRequestExists {
		result.Throttled = true
		return result, nil
	}

	token, err := s.generateRawToken()
	if err != nil {
		return nil, fmt.Errorf("generate password reset token: %w", err)
	}

	expiresAt := now.Add(s.tokenTTL)
	_, err = tx.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = $2
		WHERE user_id = $1
			AND used_at IS NULL
	`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("expire previous password reset tokens: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO password_reset_tokens (
			user_id, token_hash, requested_email, request_ip, user_agent, created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, userID, HashRefreshToken(token), normalizedEmail, truncateAuditField(requestIP), truncateAuditField(userAgent), now, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("create password reset token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit password reset request: %w", err)
	}

	result.Issued = true
	result.Token = token
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin password reset: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit path owns the successful transaction.

	var userID string
	var passwordHash string
	var isActive bool
	err = tx.QueryRow(ctx, `
		SELECT u.id::text, u.password_hash, u.is_active
		FROM password_reset_tokens prt
		JOIN users u ON u.id = prt.user_id
		WHERE prt.token_hash = $1
			AND prt.used_at IS NULL
			AND prt.expires_at > $2
		FOR UPDATE OF prt
	`, HashRefreshToken(resetToken), now).Scan(&userID, &passwordHash, &isActive)
	if err == pgx.ErrNoRows {
		return "", ErrPasswordResetTokenInvalid
	}
	if err != nil {
		return "", fmt.Errorf("get password reset token: %w", err)
	}
	if !isActive {
		return "", fmt.Errorf("account is disabled")
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(newPassword)) == nil {
		return "", fmt.Errorf("new password must be different from current password")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET password_hash = $2, updated_at = $3
		WHERE id = $1
	`, userID, string(newHash), now); err != nil {
		return "", fmt.Errorf("update user password: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE password_reset_tokens
		SET used_at = $2
		WHERE user_id = $1
			AND used_at IS NULL
	`, userID, now); err != nil {
		return "", fmt.Errorf("consume password reset tokens: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit password reset: %w", err)
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
