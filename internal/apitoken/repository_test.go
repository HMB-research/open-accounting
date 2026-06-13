package apitoken

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepository_NilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	require.NotNil(t, repo)

	ctx := context.Background()
	now := time.Date(2026, time.June, 13, 12, 0, 0, 0, time.UTC)
	token := &APIToken{
		ID:          "token-1",
		TenantID:    "tenant-1",
		UserID:      "user-1",
		Name:        "CLI",
		TokenPrefix: "oa_123",
		CreatedAt:   now,
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "dbWithContext",
			run: func() error {
				db, err := repo.dbWithContext(ctx)
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "CreateToken",
			run: func() error {
				return repo.CreateToken(ctx, token, "hash")
			},
		},
		{
			name: "ListTokens",
			run: func() error {
				tokens, err := repo.ListTokens(ctx, "user-1", "tenant-1")
				assert.Nil(t, tokens)
				return err
			},
		},
		{
			name: "RevokeToken",
			run: func() error {
				return repo.RevokeToken(ctx, "user-1", "tenant-1", "token-1", now)
			},
		},
		{
			name: "GetValidationRecord",
			run: func() error {
				record, err := repo.GetValidationRecord(ctx, "hash", now)
				assert.Nil(t, record)
				return err
			},
		},
		{
			name: "TouchToken",
			run: func() error {
				return repo.TouchToken(ctx, "token-1", now)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "api token repository database is not configured")
		})
	}
}

func TestAPITokenModelMappingRoundTrip(t *testing.T) {
	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	lastUsedAt := createdAt.Add(time.Hour)
	expiresAt := createdAt.Add(24 * time.Hour)
	revokedAt := createdAt.Add(48 * time.Hour)

	token := &APIToken{
		ID:          "token-1",
		TenantID:    "tenant-1",
		UserID:      "user-1",
		Name:        "Automation token",
		TokenPrefix: "oa_abcdef12345",
		LastUsedAt:  &lastUsedAt,
		ExpiresAt:   &expiresAt,
		RevokedAt:   &revokedAt,
		CreatedAt:   createdAt,
	}

	model := apiTokenToModel(token, "token-hash")
	require.NotNil(t, model)
	assert.Equal(t, "token-1", model.ID)
	assert.Equal(t, "tenant-1", model.TenantID)
	assert.Equal(t, "user-1", model.UserID)
	assert.Equal(t, "Automation token", model.Name)
	assert.Equal(t, "token-hash", model.TokenHash)
	assert.Equal(t, "oa_abcdef12345", model.TokenPrefix)
	assert.Equal(t, &lastUsedAt, model.LastUsedAt)
	assert.Equal(t, &expiresAt, model.ExpiresAt)
	assert.Equal(t, &revokedAt, model.RevokedAt)
	assert.Equal(t, createdAt, model.CreatedAt)

	roundTrip := modelToAPIToken(model)
	require.NotNil(t, roundTrip)
	assert.Equal(t, token, roundTrip)
}

func TestModelToAPITokenPreservesNilOptionalTimes(t *testing.T) {
	model := &models.APIToken{
		ID:          "token-2",
		TenantID:    "tenant-2",
		UserID:      "user-2",
		Name:        "No expiry",
		TokenHash:   "hash-is-not-exposed",
		TokenPrefix: "oa_prefix",
		CreatedAt:   time.Date(2026, time.February, 3, 4, 5, 6, 0, time.UTC),
	}

	token := modelToAPIToken(model)
	require.NotNil(t, token)
	assert.Equal(t, "token-2", token.ID)
	assert.Equal(t, "tenant-2", token.TenantID)
	assert.Equal(t, "user-2", token.UserID)
	assert.Equal(t, "No expiry", token.Name)
	assert.Equal(t, "oa_prefix", token.TokenPrefix)
	assert.Nil(t, token.LastUsedAt)
	assert.Nil(t, token.ExpiresAt)
	assert.Nil(t, token.RevokedAt)
	assert.Equal(t, model.CreatedAt, token.CreatedAt)
}

func TestNewServiceWithoutDatabase(t *testing.T) {
	service := NewService(nil)
	require.NotNil(t, service)
	assert.Nil(t, service.repo)
}
