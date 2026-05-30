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
		apiTokenService:      service,
		securityAuditService: &mockSecurityAuditService{},
	}, repo
}

func setupTenantUserAPITokenHandlers() (*Handlers, *mockTenantRepository, *mockAPITokenRepository) {
	tenantRepo := newMockTenantRepository()
	apiTokenRepo := newMockAPITokenRepository()

	return &Handlers{
		tenantService:        tenant.NewServiceWithRepository(tenantRepo),
		apiTokenService:      apitoken.NewServiceWithRepository(apiTokenRepo),
		securityAuditService: &mockSecurityAuditService{},
	}, tenantRepo, apiTokenRepo
}

func TestCreateAPIToken(t *testing.T) {
	h, _ := setupAPITokenHandlers()
	claims := &auth.Claims{
		UserID:   "user-1",
		Email:    "user@example.com",
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

	auditEvents := h.securityAuditService.(*mockSecurityAuditService).events
	require.NotEmpty(t, auditEvents)
	assert.Equal(t, auth.SecurityAuditActionAPITokenCreated, auditEvents[0].Action)
	assert.Equal(t, "user-1", auditEvents[0].TargetUserID)
	assert.Equal(t, resp.APIToken.ID, auditEvents[0].Metadata["token_id"])
}

func TestListAPITokens(t *testing.T) {
	h, repo := setupAPITokenHandlers()
	claims := &auth.Claims{
		UserID:   "user-1",
		Email:    "user@example.com",
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

	auditEvents := h.securityAuditService.(*mockSecurityAuditService).events
	require.NotEmpty(t, auditEvents)
	assert.Equal(t, auth.SecurityAuditActionAPITokenRevoked, auditEvents[0].Action)
	assert.Equal(t, "user-1", auditEvents[0].TargetUserID)
	assert.Equal(t, "token-1", auditEvents[0].Metadata["token_id"])
}

func TestListTenantUserAPITokens(t *testing.T) {
	h, tenantRepo, apiTokenRepo := setupTenantUserAPITokenHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	tenantRepo.addTestUser("user-2", "target@example.com", "Target", "password", true)
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "admin-1", Role: tenant.RoleAdmin, IsActive: true, CreatedAt: time.Now()},
		{TenantID: "tenant-1", UserID: "user-2", Role: tenant.RoleViewer, IsActive: true, CreatedAt: time.Now()},
	}
	apiTokenRepo.tokens["token-1"] = &apitoken.APIToken{
		ID:          "token-1",
		UserID:      "user-2",
		TenantID:    "tenant-1",
		Name:        "CLI token",
		TokenPrefix: "oa_123456",
		CreatedAt:   time.Now(),
	}
	apiTokenRepo.tokens["other-token"] = &apitoken.APIToken{
		ID:          "other-token",
		UserID:      "admin-1",
		TenantID:    "tenant-1",
		Name:        "Admin token",
		TokenPrefix: "oa_admin",
		CreatedAt:   time.Now(),
	}
	claims := &auth.Claims{UserID: "admin-1", Email: "admin@example.com", TenantID: "tenant-1", Role: tenant.RoleAdmin}
	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/users/user-2/api-tokens", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2"})
	w := httptest.NewRecorder()

	h.ListTenantUserAPITokens(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp []apitoken.APIToken
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "token-1", resp[0].ID)
}

func TestRevokeTenantUserAPITokenAuditsAction(t *testing.T) {
	h, tenantRepo, apiTokenRepo := setupTenantUserAPITokenHandlers()
	tenantRepo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	tenantRepo.addTestUser("user-2", "target@example.com", "Target", "password", true)
	tenantRepo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "admin-1", Role: tenant.RoleAdmin, IsActive: true, CreatedAt: time.Now()},
		{TenantID: "tenant-1", UserID: "user-2", Role: tenant.RoleViewer, IsActive: true, CreatedAt: time.Now()},
	}
	apiTokenRepo.tokens["token-1"] = &apitoken.APIToken{
		ID:          "token-1",
		UserID:      "user-2",
		TenantID:    "tenant-1",
		Name:        "CLI token",
		TokenPrefix: "oa_123456",
		CreatedAt:   time.Now(),
	}
	claims := &auth.Claims{UserID: "admin-1", Email: "admin@example.com", TenantID: "tenant-1", Role: tenant.RoleAdmin}
	req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/tenant-1/users/user-2/api-tokens/token-1", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "userID": "user-2", "tokenID": "token-1"})
	w := httptest.NewRecorder()

	h.RevokeTenantUserAPIToken(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	require.NotNil(t, apiTokenRepo.tokens["token-1"].RevokedAt)

	securityEvents := h.securityAuditService.(*mockSecurityAuditService).events
	require.NotEmpty(t, securityEvents)
	assert.Equal(t, auth.SecurityAuditActionAPITokenRevoked, securityEvents[0].Action)
	assert.Equal(t, "target@example.com", securityEvents[0].TargetEmail)
	assert.Equal(t, "token-1", securityEvents[0].Metadata["token_id"])

	require.NotEmpty(t, tenantRepo.auditEvents["tenant-1"])
	assert.Equal(t, tenant.AuditActionUserAPITokenRevoked, tenantRepo.auditEvents["tenant-1"][0].Action)
	assert.Equal(t, "token-1", tenantRepo.auditEvents["tenant-1"][0].Metadata["token_id"])
}
