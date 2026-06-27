package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type dryRunConnPool struct{}

func (dryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tests should not prepare statements")
}

func (dryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tests should not execute statements")
}

func (dryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tests should not query rows")
}

func (dryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (dryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &dryRunTx{}, nil
}

type dryRunTx struct {
	dryRunConnPool
}

func (*dryRunTx) Commit() error {
	return nil
}

func (*dryRunTx) Rollback() error {
	return nil
}

type authDryRunDBOption func(*gorm.DB)

type dryRunQueryFixtures struct {
	user            *models.User
	count           *int64
	resetToken      *models.PasswordResetToken
	refreshSessions []models.RefreshSession
	securityEvents  []models.SecurityAuditEvent
}

func newAuthDryRunDB(t *testing.T, opts ...authDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: dryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(db)
	}
	return db
}

func withDryRunRowsAffected(rows int64) authDryRunDBOption {
	return func(db *gorm.DB) {
		db.Callback().Update().After("gorm:update").Register("auth_test:update_rows_affected", func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
	}
}

func withDryRunQueryFixtures(fixtures dryRunQueryFixtures) authDryRunDBOption {
	return func(db *gorm.DB) {
		db.Callback().Query().After("gorm:query").Register("auth_test:query_fixtures", func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *models.User:
				if fixtures.user != nil {
					*dest = *fixtures.user
					tx.RowsAffected = 1
				}
			case *int64:
				if fixtures.count != nil {
					*dest = *fixtures.count
					tx.RowsAffected = 1
				}
			case *models.PasswordResetToken:
				if fixtures.resetToken != nil {
					*dest = *fixtures.resetToken
					tx.RowsAffected = 1
				}
			case *[]models.RefreshSession:
				if fixtures.refreshSessions != nil {
					*dest = append([]models.RefreshSession(nil), fixtures.refreshSessions...)
					tx.RowsAffected = int64(len(fixtures.refreshSessions))
				}
			case *[]models.SecurityAuditEvent:
				if fixtures.securityEvents != nil {
					*dest = append([]models.SecurityAuditEvent(nil), fixtures.securityEvents...)
					tx.RowsAffected = int64(len(fixtures.securityEvents))
				}
			}
		})
	}
}

func withDryRunCreateError(expectedErr error) authDryRunDBOption {
	return func(db *gorm.DB) {
		db.Callback().Create().Before("gorm:create").Register("auth_test:create_error", func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
	}
}

func withDryRunQueryErrorForDest(expectedErr error, matches func(interface{}) bool) authDryRunDBOption {
	return func(db *gorm.DB) {
		db.Callback().Query().Before("gorm:query").Register("auth_test:query_error_for_dest", func(tx *gorm.DB) {
			if matches(tx.Statement.Dest) {
				tx.AddError(expectedErr)
			}
		})
	}
}

func withDryRunUpdateErrorOnCall(callNumber int, expectedErr error) authDryRunDBOption {
	return func(db *gorm.DB) {
		calls := 0
		db.Callback().Update().Before("gorm:update").Register("auth_test:update_error_on_call", func(tx *gorm.DB) {
			calls++
			if calls == callNumber {
				tx.AddError(expectedErr)
			}
		})
	}
}

func TestRefreshSessionServiceConstructorsAndDryRunOperations(t *testing.T) {
	assert.Panics(t, func() {
		NewRefreshSessionService(nil)
	})

	now := time.Date(2026, 6, 24, 9, 15, 0, 0, time.FixedZone("EET", 3*60*60))
	service := &RefreshSessionService{
		db:  newAuthDryRunDB(t),
		now: func() time.Time { return now },
	}
	ctx := context.Background()

	err := service.CreateRefreshSession(ctx, "user-1", "token-1", "hash-1", now.Add(time.Hour))
	require.NoError(t, err)

	sessions, err := service.ListRefreshSessions(ctx, "user-1", false)
	require.NoError(t, err)
	assert.Empty(t, sessions)

	allSessions, err := service.ListRefreshSessions(ctx, "user-1", true)
	require.NoError(t, err)
	assert.Empty(t, allSessions)

	err = service.RotateRefreshSession(ctx, "user-1", "token-1", "hash-1", "token-2", "hash-2", now.Add(2*time.Hour))
	assert.ErrorIs(t, err, ErrRefreshSessionInvalid)

	err = service.RevokeRefreshSession(ctx, "user-1", "token-1", "hash-1")
	assert.ErrorIs(t, err, ErrRefreshSessionInvalid)

	err = service.RevokeRefreshSessionByID(ctx, "user-1", "token-1")
	assert.ErrorIs(t, err, ErrRefreshSessionInvalid)

	err = service.RevokeAllRefreshSessions(ctx, "user-1")
	require.NoError(t, err)
}

func TestRefreshSessionServiceDryRunSuccessBranches(t *testing.T) {
	now := time.Date(2026, 6, 24, 9, 45, 0, 0, time.UTC)
	lastUsedAt := now.Add(-10 * time.Minute)
	revokedAt := now.Add(-5 * time.Minute)
	service := &RefreshSessionService{
		db: newAuthDryRunDB(t,
			withDryRunRowsAffected(1),
			withDryRunQueryFixtures(dryRunQueryFixtures{
				refreshSessions: []models.RefreshSession{
					{
						ID:         "session-2",
						UserID:     "user-1",
						CreatedAt:  now.Add(-time.Hour),
						LastUsedAt: &lastUsedAt,
						ExpiresAt:  now.Add(time.Hour),
						RevokedAt:  &revokedAt,
					},
				},
			}),
		),
		now: func() time.Time { return now },
	}
	ctx := context.Background()

	err := service.RotateRefreshSession(ctx, "user-1", "token-1", "hash-1", "token-2", "hash-2", now.Add(2*time.Hour))
	require.NoError(t, err)

	err = service.RevokeRefreshSession(ctx, "user-1", "token-2", "hash-2")
	require.NoError(t, err)

	err = service.RevokeRefreshSessionByID(ctx, "user-1", "token-2")
	require.NoError(t, err)

	sessions, err := service.ListRefreshSessions(ctx, "user-1", false)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "session-2", sessions[0].ID)
	require.NotNil(t, sessions[0].LastUsedAt)
	assert.Equal(t, lastUsedAt, *sessions[0].LastUsedAt)
	require.NotNil(t, sessions[0].RevokedAt)
	assert.Equal(t, revokedAt, *sessions[0].RevokedAt)
}

func TestRefreshSessionServiceDryRunErrorBranches(t *testing.T) {
	now := time.Date(2026, 6, 24, 10, 0, 0, 0, time.UTC)
	ctx := context.Background()
	updateErr := errors.New("update failed")
	queryErr := errors.New("query failed")

	service := &RefreshSessionService{
		db:  newAuthDryRunDB(t, withDryRunUpdateErrorOnCall(1, updateErr)),
		now: func() time.Time { return now },
	}
	err := service.RotateRefreshSession(ctx, "user-1", "token-1", "hash-1", "token-2", "hash-2", now.Add(time.Hour))
	assert.ErrorIs(t, err, updateErr)

	service = &RefreshSessionService{
		db:  newAuthDryRunDB(t, withDryRunUpdateErrorOnCall(1, updateErr)),
		now: func() time.Time { return now },
	}
	err = service.RevokeRefreshSession(ctx, "user-1", "token-1", "hash-1")
	assert.ErrorIs(t, err, updateErr)

	service = &RefreshSessionService{
		db:  newAuthDryRunDB(t, withDryRunUpdateErrorOnCall(1, updateErr)),
		now: func() time.Time { return now },
	}
	err = service.RevokeRefreshSessionByID(ctx, "user-1", "token-1")
	assert.ErrorIs(t, err, updateErr)

	service = &RefreshSessionService{
		db: newAuthDryRunDB(t, withDryRunQueryErrorForDest(queryErr, func(dest interface{}) bool {
			_, ok := dest.(*[]models.RefreshSession)
			return ok
		})),
		now: func() time.Time { return now },
	}
	sessions, err := service.ListRefreshSessions(ctx, "user-1", false)
	assert.Nil(t, sessions)
	assert.ErrorIs(t, err, queryErr)
}

func TestSecurityAuditServiceConstructorsValidationAndDryRunOperations(t *testing.T) {
	assert.Panics(t, func() {
		NewSecurityAuditService(nil)
	})

	now := time.Date(2026, 6, 24, 10, 30, 0, 0, time.FixedZone("EET", 3*60*60))
	service := &SecurityAuditService{
		db:  newAuthDryRunDB(t),
		now: func() time.Time { return now },
	}
	ctx := context.Background()

	err := service.RecordEvent(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "event is required")

	err = service.RecordEvent(ctx, &SecurityAuditEvent{Action: " "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "action is required")

	event := &SecurityAuditEvent{
		ActorUserID:  " actor-1 ",
		ActorEmail:   " actor@example.com ",
		Action:       "  " + SecurityAuditActionLogin + "  ",
		TargetUserID: "target-1",
		TargetEmail:  "target@example.com",
		RequestIP:    "192.0.2.10",
		UserAgent:    "unit-test",
	}
	err = service.RecordEvent(ctx, event)
	require.NoError(t, err)
	assert.NotEmpty(t, event.ID)
	assert.Equal(t, SecurityAuditActionLogin, event.Action)
	assert.Equal(t, now.UTC(), event.CreatedAt)
	assert.Empty(t, event.Metadata)

	_, err = service.ListUserEvents(ctx, " ", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user ID is required")

	events, err := service.ListUserEvents(ctx, " user-1 ", 0)
	require.NoError(t, err)
	assert.Empty(t, events)

	events, err = service.ListUserEvents(ctx, "user-1", 500)
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestSecurityAuditServiceListUserEventsDryRunMappings(t *testing.T) {
	createdAt := time.Date(2026, 6, 24, 12, 15, 0, 0, time.UTC)
	actorUserID := "user-1"
	actorEmail := "user@example.com"
	targetUserID := "target-1"
	service := &SecurityAuditService{
		db: newAuthDryRunDB(t, withDryRunQueryFixtures(dryRunQueryFixtures{
			securityEvents: []models.SecurityAuditEvent{
				{
					ID:           "event-2",
					ActorUserID:  &actorUserID,
					ActorEmail:   &actorEmail,
					Action:       SecurityAuditActionPasswordChanged,
					TargetUserID: &targetUserID,
					Metadata:     json.RawMessage(`{"source":"settings"}`),
					CreatedAt:    createdAt,
				},
				{
					ID:        "event-1",
					Action:    SecurityAuditActionLogin,
					CreatedAt: createdAt.Add(-time.Minute),
				},
			},
		})),
		now: time.Now,
	}

	events, err := service.ListUserEvents(context.Background(), "user-1", 2)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "event-2", events[0].ID)
	assert.Equal(t, "settings", events[0].Metadata["source"])
	assert.Equal(t, actorUserID, events[0].ActorUserID)
	assert.Equal(t, actorEmail, events[0].ActorEmail)
	assert.Equal(t, targetUserID, events[0].TargetUserID)
	assert.Equal(t, createdAt, events[0].CreatedAt)
	assert.Empty(t, events[1].Metadata)

	badMetadataService := &SecurityAuditService{
		db: newAuthDryRunDB(t, withDryRunQueryFixtures(dryRunQueryFixtures{
			securityEvents: []models.SecurityAuditEvent{
				{
					ID:        "event-bad",
					Action:    SecurityAuditActionLoginFailed,
					Metadata:  json.RawMessage(`{`),
					CreatedAt: createdAt,
				},
			},
		})),
		now: time.Now,
	}

	events, err = badMetadataService.ListUserEvents(context.Background(), "user-1", 10)
	assert.Nil(t, events)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal security audit metadata")
}

func TestSecurityAuditServiceDryRunRepositoryErrors(t *testing.T) {
	createErr := errors.New("create failed")
	queryErr := errors.New("query failed")

	service := &SecurityAuditService{
		db:  newAuthDryRunDB(t, withDryRunCreateError(createErr)),
		now: time.Now,
	}
	err := service.RecordEvent(context.Background(), &SecurityAuditEvent{Action: SecurityAuditActionLogin})
	require.Error(t, err)
	assert.ErrorIs(t, err, createErr)
	assert.Contains(t, err.Error(), "record security audit event")

	service = &SecurityAuditService{
		db: newAuthDryRunDB(t, withDryRunQueryErrorForDest(queryErr, func(dest interface{}) bool {
			_, ok := dest.(*[]models.SecurityAuditEvent)
			return ok
		})),
		now: time.Now,
	}
	events, err := service.ListUserEvents(context.Background(), "user-1", 10)
	assert.Nil(t, events)
	require.Error(t, err)
	assert.ErrorIs(t, err, queryErr)
	assert.Contains(t, err.Error(), "list security audit events")
}

func TestPasswordResetConstructorAndGORMRepositoryDryRun(t *testing.T) {
	assert.Panics(t, func() {
		NewPasswordResetService(nil)
	})

	now := time.Date(2026, 6, 24, 11, 45, 0, 0, time.FixedZone("EET", 3*60*60))
	repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t)}
	ctx := context.Background()

	outcome, err := repo.RequestPasswordReset(ctx, "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
		return nil, errors.New("inactive dry-run user should not build a token")
	})
	require.NoError(t, err)
	require.NotNil(t, outcome)
	assert.False(t, outcome.Issued)
	assert.False(t, outcome.Throttled)

	userID, err := repo.ResetPassword(ctx, HashRefreshToken("reset-token"), now, func(user *models.User) (string, error) {
		return "", errors.New("missing dry-run user should not hash a password")
	})
	assert.Empty(t, userID)
	assert.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
}

func TestPasswordResetGORMRepositoryDryRunSuccessAndThrottleBranches(t *testing.T) {
	now := time.Date(2026, 6, 24, 13, 0, 0, 0, time.UTC)
	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	require.NoError(t, err)

	recentRequestCount := int64(0)
	repo := &passwordResetGORMRepository{
		db: newAuthDryRunDB(t,
			withDryRunRowsAffected(1),
			withDryRunQueryFixtures(dryRunQueryFixtures{
				user: &models.User{
					ID:       "user-1",
					Email:    "user@example.com",
					IsActive: true,
				},
				count: &recentRequestCount,
				resetToken: &models.PasswordResetToken{
					UserID: "user-1",
					User: &models.User{
						ID:           "user-1",
						PasswordHash: string(oldHash),
						IsActive:     true,
					},
				},
			}),
		),
	}

	outcome, err := repo.RequestPasswordReset(context.Background(), "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
		return &models.PasswordResetToken{
			ID:        "reset-token-1",
			UserID:    userID,
			TokenHash: HashRefreshToken("reset-token"),
			ExpiresAt: now.Add(time.Hour),
		}, nil
	})
	require.NoError(t, err)
	require.NotNil(t, outcome)
	assert.Equal(t, "user-1", outcome.UserID)
	assert.True(t, outcome.Issued)
	assert.False(t, outcome.Throttled)

	userID, err := repo.ResetPassword(context.Background(), HashRefreshToken("reset-token"), now, func(user *models.User) (string, error) {
		assert.Equal(t, "user-1", user.ID)
		return "new-password-hash", nil
	})
	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)

	throttleCount := int64(1)
	throttlingRepo := &passwordResetGORMRepository{
		db: newAuthDryRunDB(t, withDryRunQueryFixtures(dryRunQueryFixtures{
			user: &models.User{
				ID:       "user-1",
				Email:    "user@example.com",
				IsActive: true,
			},
			count: &throttleCount,
		})),
	}

	outcome, err = throttlingRepo.RequestPasswordReset(context.Background(), "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
		return nil, errors.New("throttled request should not build a token")
	})
	require.NoError(t, err)
	require.NotNil(t, outcome)
	assert.Equal(t, "user-1", outcome.UserID)
	assert.False(t, outcome.Issued)
	assert.True(t, outcome.Throttled)
}

func TestPasswordResetGORMRepositoryDryRunErrorBranches(t *testing.T) {
	now := time.Date(2026, 6, 24, 14, 0, 0, 0, time.UTC)
	ctx := context.Background()
	user := &models.User{
		ID:       "user-1",
		Email:    "user@example.com",
		IsActive: true,
	}
	zeroCount := int64(0)
	resetUser := &models.User{
		ID:           "user-1",
		PasswordHash: "$2a$04$2y7u8PvvPLuKHoKWFLaTh.VxzBFcyxfQdLq09AKiw0tu/yTKqXYOO",
		IsActive:     true,
	}

	t.Run("request find user error", func(t *testing.T) {
		findErr := errors.New("find failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t, withDryRunQueryErrorForDest(findErr, func(dest interface{}) bool {
			_, ok := dest.(*models.User)
			return ok
		}))}

		outcome, err := repo.RequestPasswordReset(ctx, "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
			return nil, errors.New("should not build token")
		})
		assert.Nil(t, outcome)
		require.Error(t, err)
		assert.ErrorIs(t, err, findErr)
		assert.Contains(t, err.Error(), "find user for password reset")
	})

	t.Run("request cooldown count error", func(t *testing.T) {
		countErr := errors.New("count failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t,
			withDryRunQueryFixtures(dryRunQueryFixtures{user: user}),
			withDryRunQueryErrorForDest(countErr, func(dest interface{}) bool {
				_, ok := dest.(*int64)
				return ok
			}),
		)}

		outcome, err := repo.RequestPasswordReset(ctx, "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
			return nil, errors.New("should not build token")
		})
		assert.Nil(t, outcome)
		require.Error(t, err)
		assert.ErrorIs(t, err, countErr)
		assert.Contains(t, err.Error(), "check password reset cooldown")
	})

	t.Run("request build token error", func(t *testing.T) {
		buildErr := errors.New("build failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t, withDryRunQueryFixtures(dryRunQueryFixtures{
			user:  user,
			count: &zeroCount,
		}))}

		outcome, err := repo.RequestPasswordReset(ctx, "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
			return nil, buildErr
		})
		assert.Nil(t, outcome)
		assert.ErrorIs(t, err, buildErr)
	})

	t.Run("request expire previous tokens error", func(t *testing.T) {
		updateErr := errors.New("update failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t,
			withDryRunQueryFixtures(dryRunQueryFixtures{user: user, count: &zeroCount}),
			withDryRunUpdateErrorOnCall(1, updateErr),
		)}

		outcome, err := repo.RequestPasswordReset(ctx, "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
			return &models.PasswordResetToken{ID: "reset-1", UserID: userID}, nil
		})
		assert.Nil(t, outcome)
		require.Error(t, err)
		assert.ErrorIs(t, err, updateErr)
		assert.Contains(t, err.Error(), "expire previous password reset tokens")
	})

	t.Run("request create token error", func(t *testing.T) {
		createErr := errors.New("create failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t,
			withDryRunQueryFixtures(dryRunQueryFixtures{user: user, count: &zeroCount}),
			withDryRunCreateError(createErr),
		)}

		outcome, err := repo.RequestPasswordReset(ctx, "user@example.com", now, time.Minute, func(userID string) (*models.PasswordResetToken, error) {
			return &models.PasswordResetToken{ID: "reset-1", UserID: userID}, nil
		})
		assert.Nil(t, outcome)
		require.Error(t, err)
		assert.ErrorIs(t, err, createErr)
		assert.Contains(t, err.Error(), "create password reset token")
	})

	t.Run("reset token query error", func(t *testing.T) {
		queryErr := errors.New("query failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t, withDryRunQueryErrorForDest(queryErr, func(dest interface{}) bool {
			_, ok := dest.(*models.PasswordResetToken)
			return ok
		}))}

		userID, err := repo.ResetPassword(ctx, HashRefreshToken("reset-token"), now, func(user *models.User) (string, error) {
			return "", errors.New("should not build password hash")
		})
		assert.Empty(t, userID)
		require.Error(t, err)
		assert.ErrorIs(t, err, queryErr)
		assert.Contains(t, err.Error(), "get password reset token")
	})

	t.Run("reset token missing user", func(t *testing.T) {
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t, withDryRunQueryFixtures(dryRunQueryFixtures{
			resetToken: &models.PasswordResetToken{ID: "reset-1", UserID: "user-1"},
		}))}

		userID, err := repo.ResetPassword(ctx, HashRefreshToken("reset-token"), now, func(user *models.User) (string, error) {
			return "", errors.New("should not build password hash")
		})
		assert.Empty(t, userID)
		assert.ErrorIs(t, err, ErrPasswordResetTokenInvalid)
	})

	t.Run("reset password hash builder error", func(t *testing.T) {
		buildErr := errors.New("build failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t, withDryRunQueryFixtures(dryRunQueryFixtures{
			resetToken: &models.PasswordResetToken{ID: "reset-1", UserID: "user-1", User: resetUser},
		}))}

		userID, err := repo.ResetPassword(ctx, HashRefreshToken("reset-token"), now, func(user *models.User) (string, error) {
			return "", buildErr
		})
		assert.Empty(t, userID)
		assert.ErrorIs(t, err, buildErr)
	})

	t.Run("reset update user password error", func(t *testing.T) {
		updateErr := errors.New("update failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t,
			withDryRunQueryFixtures(dryRunQueryFixtures{
				resetToken: &models.PasswordResetToken{ID: "reset-1", UserID: "user-1", User: resetUser},
			}),
			withDryRunUpdateErrorOnCall(1, updateErr),
		)}

		userID, err := repo.ResetPassword(ctx, HashRefreshToken("reset-token"), now, func(user *models.User) (string, error) {
			return "new-hash", nil
		})
		assert.Empty(t, userID)
		require.Error(t, err)
		assert.ErrorIs(t, err, updateErr)
		assert.Contains(t, err.Error(), "update user password")
	})

	t.Run("reset consume tokens error", func(t *testing.T) {
		updateErr := errors.New("update failed")
		repo := &passwordResetGORMRepository{db: newAuthDryRunDB(t,
			withDryRunQueryFixtures(dryRunQueryFixtures{
				resetToken: &models.PasswordResetToken{ID: "reset-1", UserID: "user-1", User: resetUser},
			}),
			withDryRunUpdateErrorOnCall(2, updateErr),
		)}

		userID, err := repo.ResetPassword(ctx, HashRefreshToken("reset-token"), now, func(user *models.User) (string, error) {
			return "new-hash", nil
		})
		assert.Empty(t, userID)
		require.Error(t, err)
		assert.ErrorIs(t, err, updateErr)
		assert.Contains(t, err.Error(), "consume password reset tokens")
	})
}

func TestPasswordResetServiceRequestValidationAndHashFailure(t *testing.T) {
	repo := &fakePasswordResetRepository{}
	service := newPasswordResetServiceWithRepository(repo)

	result, err := service.RequestPasswordReset(context.Background(), " ", "", "")
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")

	oldHash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	require.NoError(t, err)

	repo = &fakePasswordResetRepository{
		resetUser: &models.User{
			ID:           "user-1",
			PasswordHash: string(oldHash),
			IsActive:     true,
		},
	}
	service = newPasswordResetServiceWithRepository(repo, WithPasswordResetHashCost(bcrypt.MinCost))
	service.passwordHashCost = bcrypt.MaxCost + 1

	userID, err := service.ResetPassword(context.Background(), "reset-token", "new-password")
	assert.Empty(t, userID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash password")
	assert.Empty(t, repo.updatedPasswordHash)
}
