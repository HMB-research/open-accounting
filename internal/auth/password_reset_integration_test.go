package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestPasswordResetServiceLifecycleAndCooldown(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	userID := testutil.CreateTestUser(t, pool, "password-reset@example.com")

	oldHash, err := bcrypt.GenerateFromPassword([]byte("oldpassword123"), 12)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, string(oldHash))
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	service := NewPasswordResetService(pool)
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

	var storedHash string
	err = pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&storedHash)
	require.NoError(t, err)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("newpassword123")))

	_, err = service.ResetPassword(ctx, result.Token, "anotherpassword123")
	assert.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
}

func TestPasswordResetServiceRejectsExpiredAndSamePassword(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	userID := testutil.CreateTestUser(t, pool, "password-reset-expired@example.com")

	oldHash, err := bcrypt.GenerateFromPassword([]byte("oldpassword123"), 12)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, string(oldHash))
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	service := NewPasswordResetService(pool)
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
	userID := testutil.CreateTestUser(t, pool, "password-reset-disabled@example.com")
	_, err := pool.Exec(context.Background(), `UPDATE users SET is_active = false WHERE id = $1`, userID)
	require.NoError(t, err)

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
