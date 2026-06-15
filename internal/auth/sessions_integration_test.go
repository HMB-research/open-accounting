//go:build integration

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestRefreshSessionServiceLifecycle(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	userID := testutil.CreateTestUser(t, pool, "refresh-session@example.com")

	now := time.Now().UTC().Truncate(time.Second)
	service := NewRefreshSessionService(pool)
	service.now = func() time.Time { return now }

	ctx := context.Background()
	oldTokenID := uuid.NewString()
	oldToken := "old-refresh-token"
	oldHash := HashRefreshToken(oldToken)
	expiresAt := now.Add(24 * time.Hour)

	err := service.CreateRefreshSession(ctx, userID, oldTokenID, oldHash, expiresAt)
	require.NoError(t, err)

	newTokenID := uuid.NewString()
	newToken := "new-refresh-token"
	newHash := HashRefreshToken(newToken)
	err = service.RotateRefreshSession(ctx, userID, oldTokenID, oldHash, newTokenID, newHash, expiresAt.Add(time.Hour))
	require.NoError(t, err)

	activeSessions, err := service.ListRefreshSessions(ctx, userID, false)
	require.NoError(t, err)
	require.Len(t, activeSessions, 1)
	assert.Equal(t, newTokenID, activeSessions[0].ID)

	allSessions, err := service.ListRefreshSessions(ctx, userID, true)
	require.NoError(t, err)
	require.Len(t, allSessions, 2)

	err = service.RevokeAllRefreshSessions(ctx, userID)
	require.NoError(t, err)

	activeSessions, err = service.ListRefreshSessions(ctx, userID, false)
	require.NoError(t, err)
	require.Empty(t, activeSessions)

	// Create one more active session so the single-session revoke path remains covered.
	newTokenID = uuid.NewString()
	newHash = HashRefreshToken("new-refresh-token-after-all-revoke")
	err = service.CreateRefreshSession(ctx, userID, newTokenID, newHash, expiresAt.Add(2*time.Hour))
	require.NoError(t, err)

	err = service.RotateRefreshSession(ctx, userID, oldTokenID, oldHash, uuid.NewString(), HashRefreshToken("reuse"), expiresAt)
	assert.ErrorIs(t, err, ErrRefreshSessionInvalid)

	err = service.RevokeRefreshSessionByID(ctx, userID, newTokenID)
	require.NoError(t, err)

	err = service.RevokeRefreshSession(ctx, userID, newTokenID, newHash)
	assert.ErrorIs(t, err, ErrRefreshSessionInvalid)
}

func TestRefreshSessionServiceRejectsExpiredSession(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	userID := testutil.CreateTestUser(t, pool, "refresh-session-expired@example.com")

	now := time.Now().UTC().Truncate(time.Second)
	service := NewRefreshSessionService(pool)
	service.now = func() time.Time { return now }

	ctx := context.Background()
	tokenID := uuid.NewString()
	tokenHash := HashRefreshToken("expired-refresh-token")

	err := service.CreateRefreshSession(ctx, userID, tokenID, tokenHash, now.Add(-time.Minute))
	require.NoError(t, err)

	err = service.RevokeRefreshSession(ctx, userID, tokenID, tokenHash)
	assert.ErrorIs(t, err, ErrRefreshSessionInvalid)
}
