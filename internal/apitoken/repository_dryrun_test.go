package apitoken

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type apiTokenDryRunConnPool struct{}

func (apiTokenDryRunConnPool) PrepareContext(context.Context, string) (*sql.Stmt, error) {
	return nil, errors.New("dry-run tests should not prepare statements")
}

func (apiTokenDryRunConnPool) ExecContext(context.Context, string, ...interface{}) (sql.Result, error) {
	return nil, errors.New("dry-run tests should not execute statements")
}

func (apiTokenDryRunConnPool) QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error) {
	return nil, errors.New("dry-run tests should not query rows")
}

func (apiTokenDryRunConnPool) QueryRowContext(context.Context, string, ...interface{}) *sql.Row {
	return nil
}

func (apiTokenDryRunConnPool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	return &apiTokenDryRunTx{}, nil
}

type apiTokenDryRunTx struct {
	apiTokenDryRunConnPool
}

func (*apiTokenDryRunTx) Commit() error {
	return nil
}

func (*apiTokenDryRunTx) Rollback() error {
	return nil
}

type apiTokenDryRunDBOption func(t *testing.T, db *gorm.DB)

type apiTokenDryRunValidationRecord struct {
	token models.APIToken
	email string
	role  string
}

func newAPITokenDryRunDB(t *testing.T, opts ...apiTokenDryRunDBOption) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: apiTokenDryRunConnPool{}}), &gorm.Config{
		DisableAutomaticPing:   true,
		DryRun:                 true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	for _, opt := range opts {
		opt(t, db)
	}
	return db
}

func withAPITokenDryRunQueryFixtures(listTokens []models.APIToken, validation *apiTokenDryRunValidationRecord) apiTokenDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Query().After("gorm:query").Register(apiTokenCallbackName(t, "query_fixtures"), func(tx *gorm.DB) {
			switch dest := tx.Statement.Dest.(type) {
			case *[]models.APIToken:
				*dest = append([]models.APIToken(nil), listTokens...)
				tx.RowsAffected = int64(len(listTokens))
			default:
				if validation != nil && populateAPITokenValidationDest(dest, *validation) {
					tx.RowsAffected = 1
				}
			}
		})
		require.NoError(t, err)
	}
}

func withAPITokenDryRunCreateError(expectedErr error) apiTokenDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Create().Before("gorm:create").Register(apiTokenCallbackName(t, "create_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAPITokenDryRunQueryError(expectedErr error) apiTokenDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Query().Before("gorm:query").Register(apiTokenCallbackName(t, "query_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func withAPITokenDryRunUpdateRows(rows int64) apiTokenDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Update().After("gorm:update").Register(apiTokenCallbackName(t, "update_rows"), func(tx *gorm.DB) {
			tx.RowsAffected = rows
		})
		require.NoError(t, err)
	}
}

func withAPITokenDryRunUpdateError(expectedErr error) apiTokenDryRunDBOption {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		err := db.Callback().Update().Before("gorm:update").Register(apiTokenCallbackName(t, "update_error"), func(tx *gorm.DB) {
			tx.AddError(expectedErr)
		})
		require.NoError(t, err)
	}
}

func apiTokenCallbackName(t *testing.T, suffix string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return "apitoken_test:" + replacer.Replace(t.Name()) + ":" + suffix
}

func populateAPITokenValidationDest(dest any, record apiTokenDryRunValidationRecord) bool {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Pointer || destValue.IsNil() {
		return false
	}
	recordValue := destValue.Elem()
	if recordValue.Kind() != reflect.Struct {
		return false
	}

	tokenField := recordValue.FieldByName("APIToken")
	if !tokenField.IsValid() || !tokenField.CanSet() || tokenField.Type() != reflect.TypeOf(models.APIToken{}) {
		return false
	}
	tokenField.Set(reflect.ValueOf(record.token))
	setStringField(recordValue, "Email", record.email)
	setStringField(recordValue, "Role", record.role)
	return true
}

func setStringField(recordValue reflect.Value, name string, value string) {
	field := recordValue.FieldByName(name)
	if field.IsValid() && field.CanSet() && field.Kind() == reflect.String {
		field.SetString(value)
	}
}

func TestGORMRepositoryDryRunOperations(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 24, 10, 15, 0, 0, time.UTC)
	lastUsedAt := now.Add(-time.Hour)
	expiresAt := now.Add(time.Hour)
	tokenModel := models.APIToken{
		ID:          "token-1",
		TenantID:    "tenant-1",
		UserID:      "user-1",
		Name:        "Automation token",
		TokenHash:   "token-hash",
		TokenPrefix: "oa_abcdef12345",
		LastUsedAt:  &lastUsedAt,
		ExpiresAt:   &expiresAt,
		CreatedAt:   now.Add(-2 * time.Hour),
	}
	repo := NewGORMRepository(newAPITokenDryRunDB(t,
		withAPITokenDryRunQueryFixtures([]models.APIToken{tokenModel}, &apiTokenDryRunValidationRecord{
			token: tokenModel,
			email: "user@example.com",
			role:  "owner",
		}),
		withAPITokenDryRunUpdateRows(1),
	))
	token := modelToAPIToken(&tokenModel)

	require.NoError(t, repo.CreateToken(ctx, token, tokenModel.TokenHash))

	tokens, err := repo.ListTokens(ctx, token.UserID, token.TenantID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Equal(t, token, &tokens[0])

	record, err := repo.GetValidationRecord(ctx, tokenModel.TokenHash, now)
	require.NoError(t, err)
	assert.Equal(t, tokenModel.ID, record.ID)
	assert.Equal(t, "user@example.com", record.Email)
	assert.Equal(t, "owner", record.Role)

	require.NoError(t, repo.RevokeToken(ctx, token.UserID, token.TenantID, token.ID, now))
	require.NoError(t, repo.TouchToken(ctx, token.ID, now))
}

func TestGORMRepositoryDryRunErrors(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.June, 24, 11, 0, 0, 0, time.UTC)
	token := &APIToken{
		ID:          "token-1",
		TenantID:    "tenant-1",
		UserID:      "user-1",
		Name:        "Automation token",
		TokenPrefix: "oa_abcdef12345",
		CreatedAt:   now,
	}

	t.Run("CreateToken wraps create errors", func(t *testing.T) {
		repo := NewGORMRepository(newAPITokenDryRunDB(t, withAPITokenDryRunCreateError(assert.AnError)))
		err := repo.CreateToken(ctx, token, "token-hash")
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "create api token")
	})

	t.Run("ListTokens wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newAPITokenDryRunDB(t, withAPITokenDryRunQueryError(assert.AnError)))
		tokens, err := repo.ListTokens(ctx, token.UserID, token.TenantID)
		require.Error(t, err)
		assert.Nil(t, tokens)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "list api tokens")
	})

	t.Run("RevokeToken wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newAPITokenDryRunDB(t, withAPITokenDryRunUpdateError(assert.AnError)))
		err := repo.RevokeToken(ctx, token.UserID, token.TenantID, token.ID, now)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "revoke api token")
	})

	t.Run("RevokeToken returns not found when no rows update", func(t *testing.T) {
		repo := NewGORMRepository(newAPITokenDryRunDB(t, withAPITokenDryRunUpdateRows(0)))
		err := repo.RevokeToken(ctx, token.UserID, token.TenantID, token.ID, now)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrTokenNotFound)
	})

	t.Run("GetValidationRecord maps record not found", func(t *testing.T) {
		repo := NewGORMRepository(newAPITokenDryRunDB(t, withAPITokenDryRunQueryError(gorm.ErrRecordNotFound)))
		record, err := repo.GetValidationRecord(ctx, "token-hash", now)
		require.Error(t, err)
		assert.Nil(t, record)
		assert.ErrorIs(t, err, ErrTokenNotFound)
	})

	t.Run("GetValidationRecord wraps query errors", func(t *testing.T) {
		repo := NewGORMRepository(newAPITokenDryRunDB(t, withAPITokenDryRunQueryError(assert.AnError)))
		record, err := repo.GetValidationRecord(ctx, "token-hash", now)
		require.Error(t, err)
		assert.Nil(t, record)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "get api token validation record")
	})

	t.Run("TouchToken wraps update errors", func(t *testing.T) {
		repo := NewGORMRepository(newAPITokenDryRunDB(t, withAPITokenDryRunUpdateError(assert.AnError)))
		err := repo.TouchToken(ctx, token.ID, now)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), "touch api token")
	})
}
