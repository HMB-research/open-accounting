package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func stubWave11GormDBFromPool(t *testing.T, fn func(context.Context, *pgxpool.Pool) (*gorm.DB, error)) {
	t.Helper()
	original := newGormDBFromPool
	newGormDBFromPool = fn
	t.Cleanup(func() {
		newGormDBFromPool = original
	})
}

func TestWave11AuthConstructorsUseInjectedGormDB(t *testing.T) {
	expectedDB := &gorm.DB{}
	pool := new(pgxpool.Pool)
	calls := 0
	stubWave11GormDBFromPool(t, func(ctx context.Context, got *pgxpool.Pool) (*gorm.DB, error) {
		require.NotNil(t, ctx)
		require.Same(t, pool, got)
		calls++
		return expectedDB, nil
	})

	passwordReset := NewPasswordResetService(pool)
	passwordResetRepo, ok := passwordReset.repo.(*passwordResetGORMRepository)
	require.True(t, ok)
	require.Same(t, expectedDB, passwordResetRepo.db)

	audit := NewSecurityAuditService(pool)
	require.Same(t, expectedDB, audit.db)

	sessions := NewRefreshSessionService(pool)
	require.Same(t, expectedDB, sessions.db)
	require.Equal(t, 3, calls)
}

func TestWave11AuthConstructorsPanicOnInjectedGormError(t *testing.T) {
	stubWave11GormDBFromPool(t, func(context.Context, *pgxpool.Pool) (*gorm.DB, error) {
		return nil, errors.New("pool unavailable")
	})

	pool := new(pgxpool.Pool)
	require.PanicsWithError(t, "create password reset GORM repository: pool unavailable", func() {
		_ = NewPasswordResetService(pool)
	})
	require.PanicsWithError(t, "create security audit GORM repository: pool unavailable", func() {
		_ = NewSecurityAuditService(pool)
	})
	require.PanicsWithError(t, "create refresh session GORM repository: pool unavailable", func() {
		_ = NewRefreshSessionService(pool)
	})
}

func TestWave11SecurityAuditMarshalError(t *testing.T) {
	expectedErr := errors.New("marshal failed")
	original := marshalSecurityAuditMetadata
	marshalSecurityAuditMetadata = func(any) ([]byte, error) {
		return nil, expectedErr
	}
	t.Cleanup(func() {
		marshalSecurityAuditMetadata = original
	})

	service := &SecurityAuditService{
		db:  newAuthDryRunDB(t),
		now: time.Now,
	}

	err := service.RecordEvent(context.Background(), &SecurityAuditEvent{Action: SecurityAuditActionLogin})
	require.ErrorIs(t, err, expectedErr)
	require.Contains(t, err.Error(), "marshal security audit metadata")
}

func TestWave11PasswordResetGORMRecordNotFoundBranches(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	requestRepo := &passwordResetGORMRepository{db: newAuthDryRunDB(t, withDryRunQueryErrorForDest(gorm.ErrRecordNotFound, func(dest interface{}) bool {
		_, ok := dest.(*models.User)
		return ok
	}))}
	outcome, err := requestRepo.RequestPasswordReset(context.Background(), "missing@example.com", now, time.Minute, func(string) (*models.PasswordResetToken, error) {
		return nil, errors.New("missing user should not build token")
	})
	require.NoError(t, err)
	require.NotNil(t, outcome)
	require.False(t, outcome.Issued)
	require.False(t, outcome.Throttled)

	resetRepo := &passwordResetGORMRepository{db: newAuthDryRunDB(t, withDryRunQueryErrorForDest(gorm.ErrRecordNotFound, func(dest interface{}) bool {
		_, ok := dest.(*models.PasswordResetToken)
		return ok
	}))}
	userID, err := resetRepo.ResetPassword(context.Background(), HashRefreshToken("reset-token"), now, func(*models.User) (string, error) {
		return "", errors.New("missing token should not build password hash")
	})
	require.Empty(t, userID)
	require.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
}

func TestWave11ParsedTokenValidationRejectsInvalidClaims(t *testing.T) {
	accessClaims, err := validateParsedAccessToken(&jwt.Token{Claims: jwt.MapClaims{}, Valid: true})
	require.Nil(t, accessClaims)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token claims")

	refreshClaims, err := validateParsedRefreshToken(&jwt.Token{Claims: &RefreshClaims{}, Valid: false})
	require.Nil(t, refreshClaims)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token claims")
}

func TestWave11RateLimitClampHelpers(t *testing.T) {
	require.Equal(t, 1, retryAfterSeconds(-time.Second))
	require.Equal(t, 1, retryAfterSeconds(500*time.Millisecond))
	require.Equal(t, 0, nonNegativeTokenCount(-1.25))
	require.Equal(t, 3, nonNegativeTokenCount(3.9))
}
