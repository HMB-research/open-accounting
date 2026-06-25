package apitoken

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepository struct {
	tokens          map[string]*APIToken
	hashToTokenID   map[string]string
	createErr       error
	listErr         error
	revokeErr       error
	validationErr   error
	touchErr        error
	touchedTokenIDs []string
	validationRole  string
	validationEmail string
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		tokens:          make(map[string]*APIToken),
		hashToTokenID:   make(map[string]string),
		validationRole:  "accountant",
		validationEmail: "user@example.com",
	}
}

func (m *mockRepository) CreateToken(ctx context.Context, token *APIToken, tokenHash string) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.tokens[token.ID] = token
	m.hashToTokenID[tokenHash] = token.ID
	return nil
}

func (m *mockRepository) ListTokens(ctx context.Context, userID, tenantID string) ([]APIToken, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]APIToken, 0)
	for _, token := range m.tokens {
		if token.UserID == userID && token.TenantID == tenantID && token.RevokedAt == nil {
			result = append(result, *token)
		}
	}
	return result, nil
}

func (m *mockRepository) RevokeToken(ctx context.Context, userID, tenantID, tokenID string, revokedAt time.Time) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	token, ok := m.tokens[tokenID]
	if !ok || token.UserID != userID || token.TenantID != tenantID || token.RevokedAt != nil {
		return ErrTokenNotFound
	}
	token.RevokedAt = &revokedAt
	return nil
}

func (m *mockRepository) GetValidationRecord(ctx context.Context, tokenHash string, now time.Time) (*ValidationRecord, error) {
	if m.validationErr != nil {
		return nil, m.validationErr
	}
	tokenID, ok := m.hashToTokenID[tokenHash]
	if !ok {
		return nil, ErrTokenNotFound
	}
	token := m.tokens[tokenID]
	if token == nil || token.RevokedAt != nil {
		return nil, ErrTokenNotFound
	}
	if token.ExpiresAt != nil && !token.ExpiresAt.After(now) {
		return nil, ErrTokenNotFound
	}
	return &ValidationRecord{
		APIToken: *token,
		Email:    m.validationEmail,
		Role:     m.validationRole,
	}, nil
}

func (m *mockRepository) TouchToken(ctx context.Context, tokenID string, lastUsedAt time.Time) error {
	if m.touchErr != nil {
		return m.touchErr
	}
	token, ok := m.tokens[tokenID]
	if !ok {
		return ErrTokenNotFound
	}
	token.LastUsedAt = &lastUsedAt
	m.touchedTokenIDs = append(m.touchedTokenIDs, tokenID)
	return nil
}

func TestService_CreateToken(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	result, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{
		Name: "CLI token",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.Token)
	assert.NotNil(t, result.APIToken)
	assert.Equal(t, "CLI token", result.APIToken.Name)
	assert.Equal(t, "tenant-1", result.APIToken.TenantID)
	assert.Equal(t, "user-1", result.APIToken.UserID)
	assert.True(t, strings.HasPrefix(result.Token, tokenPrefix))
}

func TestNewServiceWithNilPoolLeavesRepositoryUnconfigured(t *testing.T) {
	service := NewService(nil)
	require.NotNil(t, service)
	assert.Nil(t, service.repo)
}

func TestNewServicePanicsWhenPoolCannotPing(t *testing.T) {
	pool := newUnreachableAPITokenPool(t)
	defer pool.Close()

	assert.Panics(t, func() {
		NewService(pool)
	})
}

func TestService_CreateTokenRejectsBadInput(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	_, err := service.CreateToken(context.Background(), "user-1", "tenant-1", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request is required")

	_, err = service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")

	expired := time.Now().Add(-time.Hour)
	_, err = service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{
		Name:      "Expired",
		ExpiresAt: &expired,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expires_at")
}

func TestService_CreateTokenReturnsRepositoryError(t *testing.T) {
	repo := newMockRepository()
	repo.createErr = assert.AnError
	service := NewServiceWithRepository(repo)

	_, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{
		Name: "CLI token",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestService_CreateTokenReturnsRandomGenerationError(t *testing.T) {
	randomErr := errors.New("random unavailable")
	originalRandomRead := tokenRandomRead
	tokenRandomRead = func([]byte) (int, error) {
		return 0, randomErr
	}
	t.Cleanup(func() {
		tokenRandomRead = originalRandomRead
	})

	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	result, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{
		Name: "CLI token",
	})
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, randomErr)
	assert.Contains(t, err.Error(), "generate api token")

	rawToken, prefix, tokenHash, err := generateTokenMaterial()
	require.Error(t, err)
	assert.Empty(t, rawToken)
	assert.Empty(t, prefix)
	assert.Empty(t, tokenHash)
	assert.ErrorIs(t, err, randomErr)
}

func TestService_ValidateAPIToken(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	result, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{
		Name: "CLI token",
	})
	require.NoError(t, err)

	claims, err := service.ValidateAPIToken(context.Background(), result.Token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
	assert.Equal(t, "tenant-1", claims.TenantID)
	assert.Equal(t, "accountant", claims.Role)
	assert.Len(t, repo.touchedTokenIDs, 1)
	assert.Equal(t, result.APIToken.ID, repo.touchedTokenIDs[0])
}

func TestService_RevokeToken(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	result, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{
		Name: "CLI token",
	})
	require.NoError(t, err)

	err = service.RevokeToken(context.Background(), "user-1", "tenant-1", result.APIToken.ID)
	require.NoError(t, err)
	require.NotNil(t, repo.tokens[result.APIToken.ID].RevokedAt)
}

func TestService_ListTokens(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	first, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{Name: "First"})
	require.NoError(t, err)
	second, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{Name: "Second"})
	require.NoError(t, err)

	tokens, err := service.ListTokens(context.Background(), "user-1", "tenant-1")
	require.NoError(t, err)
	assert.Len(t, tokens, 2)
	assert.ElementsMatch(t, []string{first.APIToken.ID, second.APIToken.ID}, []string{tokens[0].ID, tokens[1].ID})
}

func TestService_ListTokensReturnsRepositoryError(t *testing.T) {
	repo := newMockRepository()
	repo.listErr = assert.AnError
	service := NewServiceWithRepository(repo)

	tokens, err := service.ListTokens(context.Background(), "user-1", "tenant-1")
	require.Error(t, err)
	assert.Nil(t, tokens)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestService_RevokeTokenRejectsMissingIDAndNotFound(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	err := service.RevokeToken(context.Background(), "user-1", "tenant-1", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token id is required")

	err = service.RevokeToken(context.Background(), "user-1", "tenant-1", "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api token not found")
}

func TestService_RevokeTokenReturnsRepositoryError(t *testing.T) {
	repo := newMockRepository()
	repo.revokeErr = assert.AnError
	service := NewServiceWithRepository(repo)

	err := service.RevokeToken(context.Background(), "user-1", "tenant-1", "token-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestService_MethodsRequireConfiguredRepository(t *testing.T) {
	ctx := context.Background()
	services := []struct {
		name    string
		service *Service
	}{
		{name: "nil service"},
		{name: "nil repository", service: NewServiceWithRepository(nil)},
	}

	for _, tt := range services {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.service.CreateToken(ctx, "user-1", "tenant-1", &CreateRequest{Name: "CLI token"})
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "api token repository is not configured")

			tokens, err := tt.service.ListTokens(ctx, "user-1", "tenant-1")
			require.Error(t, err)
			assert.Nil(t, tokens)
			assert.Contains(t, err.Error(), "api token repository is not configured")

			err = tt.service.RevokeToken(ctx, "user-1", "tenant-1", "token-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "api token repository is not configured")

			claims, err := tt.service.ValidateAPIToken(ctx, "oa_anything")
			require.Error(t, err)
			assert.Nil(t, claims)
			assert.Contains(t, err.Error(), "api token repository is not configured")
		})
	}
}

func TestService_ValidateAPITokenReturnsErrors(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	_, err := service.ValidateAPIToken(context.Background(), "oa_missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api token not found")

	result, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{Name: "CLI token"})
	require.NoError(t, err)

	repo.touchErr = assert.AnError
	_, err = service.ValidateAPIToken(context.Background(), result.Token)
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestService_ValidateAPITokenReturnsValidationRepositoryError(t *testing.T) {
	repo := newMockRepository()
	repo.validationErr = assert.AnError
	service := NewServiceWithRepository(repo)

	_, err := service.ValidateAPIToken(context.Background(), "oa_anything")
	require.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

func newUnreachableAPITokenPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	return pool
}
