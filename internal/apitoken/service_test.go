package apitoken

import (
	"context"
	"strings"
	"testing"
	"time"

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

func TestService_CreateTokenRejectsBadInput(t *testing.T) {
	repo := newMockRepository()
	service := NewServiceWithRepository(repo)

	_, err := service.CreateToken(context.Background(), "user-1", "tenant-1", &CreateRequest{})
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
