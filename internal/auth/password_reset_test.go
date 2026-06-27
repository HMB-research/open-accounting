package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type fakePasswordResetRepository struct {
	users              map[string]*models.User
	recentRequestCount int64
	requestErr         error
	requestEmail       string
	requestNow         time.Time
	requestCooldown    time.Duration
	createdTokens      []models.PasswordResetToken

	resetUser           *models.User
	resetErr            error
	resetTokenHash      string
	resetNow            time.Time
	updatedPasswordHash string
	consumedUserID      string
}

func (f *fakePasswordResetRepository) RequestPasswordReset(
	ctx context.Context,
	email string,
	now time.Time,
	cooldown time.Duration,
	buildToken func(userID string) (*models.PasswordResetToken, error),
) (*passwordResetRequestOutcome, error) {
	f.requestEmail = email
	f.requestNow = now
	f.requestCooldown = cooldown

	if f.requestErr != nil {
		return nil, f.requestErr
	}

	outcome := &passwordResetRequestOutcome{}
	user := f.users[email]
	if user == nil || !user.IsActive {
		return outcome, nil
	}

	outcome.UserID = user.ID
	if f.recentRequestCount > 0 {
		outcome.Throttled = true
		return outcome, nil
	}

	token, err := buildToken(user.ID)
	if err != nil {
		return nil, err
	}
	f.createdTokens = append(f.createdTokens, *token)
	outcome.Issued = true
	return outcome, nil
}

func (f *fakePasswordResetRepository) ResetPassword(
	ctx context.Context,
	tokenHash string,
	now time.Time,
	buildPasswordHash func(user *models.User) (string, error),
) (string, error) {
	f.resetTokenHash = tokenHash
	f.resetNow = now

	if f.resetErr != nil {
		return "", f.resetErr
	}
	if f.resetUser == nil {
		return "", ErrPasswordResetTokenInvalid
	}

	newHash, err := buildPasswordHash(f.resetUser)
	if err != nil {
		return "", err
	}
	f.updatedPasswordHash = newHash
	f.consumedUserID = f.resetUser.ID
	return f.resetUser.ID, nil
}

func TestPasswordResetServiceRequestPasswordResetUnit(t *testing.T) {
	now := time.Date(2026, 6, 12, 15, 4, 5, 0, time.FixedZone("EET", 2*60*60))
	repo := &fakePasswordResetRepository{
		users: map[string]*models.User{
			"user@example.com": {
				ID:       "user-1",
				Email:    "user@example.com",
				IsActive: true,
			},
		},
	}
	service := newPasswordResetServiceWithRepository(repo)
	service.now = func() time.Time { return now }
	service.generateRawToken = func() (string, error) { return "reset-token", nil }

	longAuditValue := " " + strings.Repeat("x", 600) + " "
	result, err := service.RequestPasswordReset(context.Background(), " User@Example.COM ", longAuditValue, longAuditValue)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.ExpiresAt)

	assert.Equal(t, "user@example.com", result.Email)
	assert.Equal(t, "user-1", result.UserID)
	assert.True(t, result.Issued)
	assert.False(t, result.Throttled)
	assert.Equal(t, "reset-token", result.Token)
	assert.Equal(t, now.UTC().Add(time.Hour), *result.ExpiresAt)
	assert.Equal(t, "user@example.com", repo.requestEmail)
	assert.Equal(t, now.UTC(), repo.requestNow)
	assert.Equal(t, 5*time.Minute, repo.requestCooldown)

	require.Len(t, repo.createdTokens, 1)
	createdToken := repo.createdTokens[0]
	assert.NotEmpty(t, createdToken.ID)
	assert.Equal(t, "user-1", createdToken.UserID)
	assert.Equal(t, HashRefreshToken("reset-token"), createdToken.TokenHash)
	assert.Equal(t, "user@example.com", createdToken.RequestedEmail)
	assert.Len(t, createdToken.RequestIP, 512)
	assert.Len(t, createdToken.UserAgent, 512)
	assert.Equal(t, now.UTC(), createdToken.CreatedAt)
	assert.Equal(t, now.UTC().Add(time.Hour), createdToken.ExpiresAt)
}

func TestPasswordResetServiceRequestPasswordResetNonIssuingBranchesUnit(t *testing.T) {
	t.Run("unknown and disabled users do not generate tokens", func(t *testing.T) {
		repo := &fakePasswordResetRepository{
			users: map[string]*models.User{
				"disabled@example.com": {
					ID:       "disabled-user",
					Email:    "disabled@example.com",
					IsActive: false,
				},
			},
		}
		service := newPasswordResetServiceWithRepository(repo)
		service.generateRawToken = func() (string, error) {
			return "", errors.New("token should not be generated")
		}

		result, err := service.RequestPasswordReset(context.Background(), "missing@example.com", "", "")
		require.NoError(t, err)
		assert.Equal(t, "missing@example.com", result.Email)
		assert.False(t, result.Issued)
		assert.False(t, result.Throttled)
		assert.Empty(t, result.UserID)

		result, err = service.RequestPasswordReset(context.Background(), "disabled@example.com", "", "")
		require.NoError(t, err)
		assert.False(t, result.Issued)
		assert.False(t, result.Throttled)
		assert.Empty(t, result.UserID)
		assert.Empty(t, repo.createdTokens)
	})

	t.Run("recent active request throttles token generation", func(t *testing.T) {
		repo := &fakePasswordResetRepository{
			users: map[string]*models.User{
				"user@example.com": {
					ID:       "user-1",
					Email:    "user@example.com",
					IsActive: true,
				},
			},
			recentRequestCount: 1,
		}
		service := newPasswordResetServiceWithRepository(repo)
		service.generateRawToken = func() (string, error) {
			return "", errors.New("token should not be generated")
		}

		result, err := service.RequestPasswordReset(context.Background(), "user@example.com", "", "")
		require.NoError(t, err)
		assert.Equal(t, "user-1", result.UserID)
		assert.False(t, result.Issued)
		assert.True(t, result.Throttled)
		assert.Empty(t, result.Token)
		assert.Empty(t, repo.createdTokens)
	})

	t.Run("token generation failure is surfaced", func(t *testing.T) {
		repo := &fakePasswordResetRepository{
			users: map[string]*models.User{
				"user@example.com": {
					ID:       "user-1",
					Email:    "user@example.com",
					IsActive: true,
				},
			},
		}
		service := newPasswordResetServiceWithRepository(repo)
		service.generateRawToken = func() (string, error) {
			return "", errors.New("rng down")
		}

		result, err := service.RequestPasswordReset(context.Background(), "user@example.com", "", "")
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "generate password reset token")
		assert.Empty(t, repo.createdTokens)
	})
}

func TestPasswordResetServiceResetPasswordUnit(t *testing.T) {
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	require.NoError(t, err)

	now := time.Date(2026, 6, 12, 15, 4, 5, 0, time.FixedZone("EET", 2*60*60))
	repo := &fakePasswordResetRepository{
		resetUser: &models.User{
			ID:           "user-1",
			Email:        "user@example.com",
			PasswordHash: string(oldHash),
			IsActive:     true,
		},
	}
	service := newPasswordResetServiceWithRepository(repo, WithPasswordResetHashCost(bcrypt.MinCost))
	service.now = func() time.Time { return now }

	userID, err := service.ResetPassword(context.Background(), " reset-token ", "new-password")
	require.NoError(t, err)

	assert.Equal(t, "user-1", userID)
	assert.Equal(t, HashRefreshToken("reset-token"), repo.resetTokenHash)
	assert.Equal(t, now.UTC(), repo.resetNow)
	assert.Equal(t, "user-1", repo.consumedUserID)
	require.NotEmpty(t, repo.updatedPasswordHash)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(repo.updatedPasswordHash), []byte("new-password")))
	assert.NotEqual(t, string(oldHash), repo.updatedPasswordHash)
	cost, err := bcrypt.Cost([]byte(repo.updatedPasswordHash))
	require.NoError(t, err)
	assert.Equal(t, bcrypt.MinCost, cost)
}

func TestPasswordResetServiceResetPasswordRejectsInvalidInputsUnit(t *testing.T) {
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	require.NoError(t, err)

	t.Run("empty token", func(t *testing.T) {
		repo := &fakePasswordResetRepository{}
		service := newPasswordResetServiceWithRepository(repo)

		userID, err := service.ResetPassword(context.Background(), " ", "new-password")
		assert.Empty(t, userID)
		assert.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
		assert.Empty(t, repo.resetTokenHash)
	})

	t.Run("empty password", func(t *testing.T) {
		repo := &fakePasswordResetRepository{}
		service := newPasswordResetServiceWithRepository(repo)

		userID, err := service.ResetPassword(context.Background(), "reset-token", "")
		assert.Empty(t, userID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "new password is required")
		assert.Empty(t, repo.resetTokenHash)
	})

	t.Run("short password", func(t *testing.T) {
		repo := &fakePasswordResetRepository{}
		service := newPasswordResetServiceWithRepository(repo)

		userID, err := service.ResetPassword(context.Background(), "reset-token", "short")
		assert.Empty(t, userID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least 8 characters")
		assert.Empty(t, repo.resetTokenHash)
	})

	t.Run("repository invalid token", func(t *testing.T) {
		repo := &fakePasswordResetRepository{resetErr: ErrPasswordResetTokenInvalid}
		service := newPasswordResetServiceWithRepository(repo)

		userID, err := service.ResetPassword(context.Background(), "reset-token", "new-password")
		assert.Empty(t, userID)
		assert.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
		assert.Equal(t, HashRefreshToken("reset-token"), repo.resetTokenHash)
	})

	t.Run("disabled account", func(t *testing.T) {
		repo := &fakePasswordResetRepository{
			resetUser: &models.User{
				ID:           "user-1",
				PasswordHash: string(oldHash),
				IsActive:     false,
			},
		}
		service := newPasswordResetServiceWithRepository(repo)

		userID, err := service.ResetPassword(context.Background(), "reset-token", "new-password")
		assert.Empty(t, userID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "account is disabled")
		assert.Empty(t, repo.updatedPasswordHash)
	})

	t.Run("same password", func(t *testing.T) {
		repo := &fakePasswordResetRepository{
			resetUser: &models.User{
				ID:           "user-1",
				PasswordHash: string(oldHash),
				IsActive:     true,
			},
		}
		service := newPasswordResetServiceWithRepository(repo)

		userID, err := service.ResetPassword(context.Background(), "reset-token", "old-password")
		assert.Empty(t, userID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "different from current password")
		assert.Empty(t, repo.updatedPasswordHash)
	})
}

func TestPasswordResetHelpersUnit(t *testing.T) {
	token, err := generatePasswordResetToken()
	require.NoError(t, err)
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	assert.Len(t, decoded, defaultPasswordResetTokenEntropy)

	randomErr := errors.New("random unavailable")
	originalRandomRead := passwordResetRandomRead
	passwordResetRandomRead = func([]byte) (int, error) {
		return 0, randomErr
	}
	t.Cleanup(func() {
		passwordResetRandomRead = originalRandomRead
	})
	token, err = generatePasswordResetToken()
	assert.Empty(t, token)
	assert.ErrorIs(t, err, randomErr)

	assert.Equal(t, "audit", truncateAuditField(" audit "))
	assert.Len(t, truncateAuditField(strings.Repeat("a", 600)), 512)

	lowCostService := newPasswordResetServiceWithRepository(&fakePasswordResetRepository{}, WithPasswordResetHashCost(bcrypt.MinCost-1))
	assert.Equal(t, bcrypt.MinCost, lowCostService.passwordHashCost)

	highCostService := newPasswordResetServiceWithRepository(&fakePasswordResetRepository{}, WithPasswordResetHashCost(bcrypt.MaxCost+1))
	assert.Equal(t, bcrypt.MaxCost, highCostService.passwordHashCost)
}
