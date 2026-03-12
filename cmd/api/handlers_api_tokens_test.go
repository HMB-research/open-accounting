package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type mockAPITokenRepository struct {
	tokens        map[string]*apitoken.APIToken
	hashToTokenID map[string]string
}

func newMockAPITokenRepository() *mockAPITokenRepository {
	return &mockAPITokenRepository{
		tokens:        make(map[string]*apitoken.APIToken),
		hashToTokenID: make(map[string]string),
	}
}

func (m *mockAPITokenRepository) CreateToken(ctx context.Context, token *apitoken.APIToken, tokenHash string) error {
	m.tokens[token.ID] = token
	m.hashToTokenID[tokenHash] = token.ID
	return nil
}

func (m *mockAPITokenRepository) ListTokens(ctx context.Context, userID, tenantID string) ([]apitoken.APIToken, error) {
	result := make([]apitoken.APIToken, 0)
	for _, token := range m.tokens {
		if token.UserID == userID && token.TenantID == tenantID && token.RevokedAt == nil {
			result = append(result, *token)
		}
	}
	return result, nil
}

func (m *mockAPITokenRepository) RevokeToken(ctx context.Context, userID, tenantID, tokenID string, revokedAt time.Time) error {
	token, ok := m.tokens[tokenID]
	if !ok || token.UserID != userID || token.TenantID != tenantID || token.RevokedAt != nil {
		return apitoken.ErrTokenNotFound
	}
	token.RevokedAt = &revokedAt
	return nil
}

func (m *mockAPITokenRepository) GetValidationRecord(ctx context.Context, tokenHash string, now time.Time) (*apitoken.ValidationRecord, error) {
	return nil, apitoken.ErrTokenNotFound
}

func (m *mockAPITokenRepository) TouchToken(ctx context.Context, tokenID string, lastUsedAt time.Time) error {
	return nil
}

func setupAPITokenHandlers() (*Handlers, *mockAPITokenRepository) {
	repo := newMockAPITokenRepository()
	service := apitoken.NewServiceWithRepository(repo)

	return &Handlers{
		apiTokenService: service,
	}, repo
}

func TestCreateAPIToken(t *testing.T) {
	h, _ := setupAPITokenHandlers()
	claims := &auth.Claims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     tenant.RoleOwner,
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/api-tokens", map[string]any{
		"name": "CLI token",
	}, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateAPIToken(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "response body: %s", w.Body.String())

	var resp apitoken.CreateResult
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.NotNil(t, resp.APIToken)
	assert.Equal(t, "CLI token", resp.APIToken.Name)
	assert.NotEmpty(t, resp.Token)
}

func TestListAPITokens(t *testing.T) {
	h, repo := setupAPITokenHandlers()
	claims := &auth.Claims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     tenant.RoleOwner,
	}
	repo.tokens["token-1"] = &apitoken.APIToken{
		ID:          "token-1",
		UserID:      "user-1",
		TenantID:    "tenant-1",
		Name:        "CLI token",
		TokenPrefix: "oa_123456",
		CreatedAt:   time.Now(),
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/api-tokens", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ListAPITokens(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())

	var resp []apitoken.APIToken
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, "CLI token", resp[0].Name)
}

func TestRevokeAPIToken(t *testing.T) {
	h, repo := setupAPITokenHandlers()
	claims := &auth.Claims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Role:     tenant.RoleOwner,
	}
	repo.tokens["token-1"] = &apitoken.APIToken{
		ID:          "token-1",
		UserID:      "user-1",
		TenantID:    "tenant-1",
		Name:        "CLI token",
		TokenPrefix: "oa_123456",
		CreatedAt:   time.Now(),
	}

	req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/api-tokens/token-1", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "tokenID": "token-1"})
	w := httptest.NewRecorder()

	h.RevokeAPIToken(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	require.NotNil(t, repo.tokens["token-1"].RevokedAt)
}
