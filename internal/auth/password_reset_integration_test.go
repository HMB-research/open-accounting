//go:build integration

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/HMB-research/open-accounting/internal/database"
	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

func TestPasswordResetServiceLifecycleAndCooldown(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := newPasswordResetTestDB(t, pool)
	userID := testutil.CreateTestUser(t, pool, "password-reset@example.com")

	oldHash, err := bcrypt.GenerateFromPassword([]byte("oldpassword123"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, db.WithContext(context.Background()).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("password_hash", string(oldHash)).Error)

	now := time.Now().UTC().Truncate(time.Second)
	service := NewPasswordResetService(pool, WithPasswordResetHashCost(bcrypt.MinCost))
	service.now = func() time.Time { return now }
	service.generateRawToken = func() (string, error) { return "reset-token-1", nil }

	ctx := context.Background()
	result, err := service.RequestPasswordReset(ctx, " Password-Reset@Example.com ", "127.0.0.1", "test-agent")
	require.NoError(t, err)
	require.True(t, result.Issued)
	assert.False(t, result.Throttled)
	assert.Equal(t, "reset-token-1", result.Token)
	require.NotNil(t, result.ExpiresAt)
	assert.Equal(t, now.Add(time.Hour), *result.ExpiresAt)

	secondResult, err := service.RequestPasswordReset(ctx, "password-reset@example.com", "127.0.0.1", "test-agent")
	require.NoError(t, err)
	assert.False(t, secondResult.Issued)
	assert.True(t, secondResult.Throttled)
	assert.Empty(t, secondResult.Token)

	resetUserID, err := service.ResetPassword(ctx, result.Token, "newpassword123")
	require.NoError(t, err)
	assert.Equal(t, userID, resetUserID)

	var updatedUser models.User
	err = db.WithContext(ctx).
		Select("password_hash").
		Where("id = ?", userID).
		Take(&updatedUser).Error
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(updatedUser.PasswordHash), []byte("newpassword123")))

	_, err = service.ResetPassword(ctx, result.Token, "anotherpassword123")
	assert.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
}

func TestPasswordResetServiceRejectsExpiredAndSamePassword(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := newPasswordResetTestDB(t, pool)
	userID := testutil.CreateTestUser(t, pool, "password-reset-expired@example.com")

	oldHash, err := bcrypt.GenerateFromPassword([]byte("oldpassword123"), bcrypt.MinCost)
	require.NoError(t, err)
	require.NoError(t, db.WithContext(context.Background()).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("password_hash", string(oldHash)).Error)

	now := time.Now().UTC().Truncate(time.Second)
	service := NewPasswordResetService(pool, WithPasswordResetHashCost(bcrypt.MinCost))
	service.now = func() time.Time { return now }
	service.generateRawToken = func() (string, error) { return "reset-token-expired", nil }

	ctx := context.Background()
	result, err := service.RequestPasswordReset(ctx, "password-reset-expired@example.com", "", "")
	require.NoError(t, err)
	require.True(t, result.Issued)

	_, err = service.ResetPassword(ctx, result.Token, "oldpassword123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "new password must be different")

	service.now = func() time.Time { return now.Add(2 * time.Hour) }
	_, err = service.ResetPassword(ctx, result.Token, "newpassword123")
	assert.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
}

func TestPasswordResetServiceDoesNotIssueForUnknownOrDisabledUsers(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := newPasswordResetTestDB(t, pool)
	userID := testutil.CreateTestUser(t, pool, "password-reset-disabled@example.com")
	require.NoError(t, db.WithContext(context.Background()).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("is_active", false).Error)

	service := NewPasswordResetService(pool)
	service.generateRawToken = func() (string, error) {
		return "", errors.New("token should not be generated")
	}

	result, err := service.RequestPasswordReset(context.Background(), "missing@example.com", "", "")
	require.NoError(t, err)
	assert.False(t, result.Issued)
	assert.False(t, result.Throttled)

	result, err = service.RequestPasswordReset(context.Background(), "password-reset-disabled@example.com", "", "")
	require.NoError(t, err)
	assert.False(t, result.Issued)
	assert.False(t, result.Throttled)
}

func newPasswordResetTestDB(t *testing.T, pool *pgxpool.Pool) *gorm.DB {
	t.Helper()

	db, err := database.NewGormDBFromPool(context.Background(), pool)
	require.NoError(t, err)
	return db
}
