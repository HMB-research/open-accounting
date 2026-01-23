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
	"golang.org/x/crypto/bcrypt"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// =============================================================================
// Mock Tenant Repository for Handler Tests
// =============================================================================

// mockTenantRepository provides a controllable tenant repository for testing handlers
type mockTenantRepository struct {
	tenants     map[string]*tenant.Tenant
	users       map[string]*tenant.User
	tenantUsers map[string][]tenant.TenantUser
	invitations map[string]*tenant.UserInvitation

	// Error injection
	createTenantErr    error
	getTenantErr       error
	getUserByEmailErr  error
	getUserByIDErr     error
	createUserErr      error
	getUserRoleErr     error
	addUserToTenantErr error
}

func newMockTenantRepository() *mockTenantRepository {
	return &mockTenantRepository{
		tenants:     make(map[string]*tenant.Tenant),
		users:       make(map[string]*tenant.User),
		tenantUsers: make(map[string][]tenant.TenantUser),
		invitations: make(map[string]*tenant.UserInvitation),
	}
}

func (m *mockTenantRepository) CreateTenant(ctx context.Context, t *tenant.Tenant, settingsJSON []byte, ownerID string) error {
	if m.createTenantErr != nil {
		return m.createTenantErr
	}
	m.tenants[t.ID] = t
	if ownerID != "" {
		m.tenantUsers[t.ID] = append(m.tenantUsers[t.ID], tenant.TenantUser{
			TenantID:  t.ID,
			UserID:    ownerID,
			Role:      tenant.RoleOwner,
			IsDefault: true,
			CreatedAt: time.Now(),
		})
	}
	return nil
}

func (m *mockTenantRepository) GetTenant(ctx context.Context, tenantID string) (*tenant.Tenant, error) {
	if m.getTenantErr != nil {
		return nil, m.getTenantErr
	}
	t, ok := m.tenants[tenantID]
	if !ok {
		return nil, tenant.ErrTenantNotFound
	}
	return t, nil
}

func (m *mockTenantRepository) GetTenantBySlug(ctx context.Context, slug string) (*tenant.Tenant, error) {
	for _, t := range m.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return nil, tenant.ErrTenantNotFound
}

func (m *mockTenantRepository) UpdateTenant(ctx context.Context, tenantID, name string, settingsJSON []byte, updatedAt time.Time) error {
	return nil
}

func (m *mockTenantRepository) DeleteTenant(ctx context.Context, tenantID, schemaName string) error {
	delete(m.tenants, tenantID)
	return nil
}

func (m *mockTenantRepository) CompleteOnboarding(ctx context.Context, tenantID string) error {
	return nil
}

func (m *mockTenantRepository) AddUserToTenant(ctx context.Context, tenantID, userID, role string) error {
	if m.addUserToTenantErr != nil {
		return m.addUserToTenantErr
	}
	m.tenantUsers[tenantID] = append(m.tenantUsers[tenantID], tenant.TenantUser{
		TenantID:  tenantID,
		UserID:    userID,
		Role:      role,
		CreatedAt: time.Now(),
	})
	return nil
}

func (m *mockTenantRepository) RemoveUserFromTenant(ctx context.Context, tenantID, userID string) error {
	return nil
}

func (m *mockTenantRepository) GetUserRole(ctx context.Context, tenantID, userID string) (string, error) {
	if m.getUserRoleErr != nil {
		return "", m.getUserRoleErr
	}
	users := m.tenantUsers[tenantID]
	for _, u := range users {
		if u.UserID == userID {
			return u.Role, nil
		}
	}
	return "", tenant.ErrUserNotInTenant
}

func (m *mockTenantRepository) ListUserTenants(ctx context.Context, userID string) ([]tenant.TenantMembership, error) {
	var result []tenant.TenantMembership
	for tenantID, users := range m.tenantUsers {
		for _, u := range users {
			if u.UserID == userID {
				t := m.tenants[tenantID]
				if t != nil {
					result = append(result, tenant.TenantMembership{
						Tenant:    *t,
						Role:      u.Role,
						IsDefault: u.IsDefault,
					})
				}
			}
		}
	}
	return result, nil
}

func (m *mockTenantRepository) ListTenantUsers(ctx context.Context, tenantID string) ([]tenant.TenantUser, error) {
	return m.tenantUsers[tenantID], nil
}

func (m *mockTenantRepository) UpdateTenantUserRole(ctx context.Context, tenantID, userID, role string) error {
	return nil
}

func (m *mockTenantRepository) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	return nil
}

func (m *mockTenantRepository) CreateUser(ctx context.Context, user *tenant.User) error {
	if m.createUserErr != nil {
		return m.createUserErr
	}
	// Check for duplicate email
	for _, u := range m.users {
		if u.Email == user.Email {
			return tenant.ErrEmailExists
		}
	}
	m.users[user.ID] = user
	return nil
}

func (m *mockTenantRepository) GetUserByEmail(ctx context.Context, email string) (*tenant.User, error) {
	if m.getUserByEmailErr != nil {
		return nil, m.getUserByEmailErr
	}
	for _, u := range m.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, tenant.ErrUserNotFound
}

func (m *mockTenantRepository) GetUserByID(ctx context.Context, userID string) (*tenant.User, error) {
	if m.getUserByIDErr != nil {
		return nil, m.getUserByIDErr
	}
	u, ok := m.users[userID]
	if !ok {
		return nil, tenant.ErrUserNotFound
	}
	return u, nil
}

func (m *mockTenantRepository) CreateInvitation(ctx context.Context, inv *tenant.UserInvitation) error {
	m.invitations[inv.Token] = inv
	return nil
}

func (m *mockTenantRepository) GetInvitationByToken(ctx context.Context, token string) (*tenant.UserInvitation, error) {
	inv, ok := m.invitations[token]
	if !ok {
		return nil, tenant.ErrInvitationNotFound
	}
	return inv, nil
}

func (m *mockTenantRepository) AcceptInvitation(ctx context.Context, inv *tenant.UserInvitation, userID string, password string, name string, createUser bool) error {
	return nil
}

func (m *mockTenantRepository) ListInvitations(ctx context.Context, tenantID string) ([]tenant.UserInvitation, error) {
	return nil, nil
}

func (m *mockTenantRepository) RevokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	return nil
}

func (m *mockTenantRepository) CheckUserIsMember(ctx context.Context, tenantID, email string) (bool, error) {
	// Find user by email first
	var userID string
	for _, u := range m.users {
		if u.Email == email {
			userID = u.ID
			break
		}
	}
	if userID == "" {
		return false, nil
	}
	users := m.tenantUsers[tenantID]
	for _, u := range users {
		if u.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

// Helper to add a test user with a hashed password
func (m *mockTenantRepository) addTestUser(id, email, name, password string, isActive bool) *tenant.User {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := &tenant.User{
		ID:           id,
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		IsActive:     isActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.users[id] = user
	return user
}

// Helper to add a test tenant
func (m *mockTenantRepository) addTestTenant(id, name, slug string) *tenant.Tenant {
	t := &tenant.Tenant{
		ID:         id,
		Name:       name,
		Slug:       slug,
		SchemaName: "tenant_" + slug,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	m.tenants[id] = t
	return t
}

// =============================================================================
// Test Setup Helpers
// =============================================================================

// setupAuthTestHandlers creates handlers with mock services for auth testing
func setupAuthTestHandlers() (*Handlers, *mockTenantRepository) {
	repo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(repo)
	tokenSvc := auth.NewTokenService("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour)

	h := &Handlers{
		tenantService: tenantSvc,
		tokenService:  tokenSvc,
	}

	return h, repo
}

// =============================================================================
// Register Handler Tests
// =============================================================================

func TestRegister(t *testing.T) {
	tests := []struct {
		name           string
		body           map[string]string
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name: "valid registration",
			body: map[string]string{
				"email":    "newuser@example.com",
				"password": "securepassword123",
				"name":     "New User",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing email",
			body: map[string]string{
				"password": "securepassword123",
				"name":     "New User",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "required",
		},
		{
			name: "missing password",
			body: map[string]string{
				"email": "newuser@example.com",
				"name":  "New User",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "required",
		},
		{
			name: "missing name",
			body: map[string]string{
				"email":    "newuser@example.com",
				"password": "securepassword123",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "required",
		},
		{
			name: "weak password (too short)",
			body: map[string]string{
				"email":    "newuser@example.com",
				"password": "short",
				"name":     "New User",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "8 characters",
		},
		{
			name: "duplicate email",
			body: map[string]string{
				"email":    "existing@example.com",
				"password": "securepassword123",
				"name":     "New User",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestUser("user-1", "existing@example.com", "Existing User", "oldpassword123", true)
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupAuthTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/auth/register", tt.body, nil)
			w := httptest.NewRecorder()

			h.Register(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.wantStatus == http.StatusCreated {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.NotEmpty(t, resp["id"])
				assert.Equal(t, tt.body["email"], resp["email"])
				assert.Equal(t, tt.body["name"], resp["name"])
			}
		})
	}
}

func TestRegisterInvalidJSON(t *testing.T) {
	h, _ := setupAuthTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/auth/register", nil)
	req.Header.Set("Content-Type", "application/json")
	// Send invalid JSON by using the body directly
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.Register(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// Login Handler Tests
// =============================================================================

func TestLogin(t *testing.T) {
	tests := []struct {
		name           string
		body           map[string]string
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "valid login without tenant",
			body: map[string]string{
				"email":    "user@example.com",
				"password": "correctpassword123",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestUser("user-1", "user@example.com", "Test User", "correctpassword123", true)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotEmpty(t, resp["access_token"])
				assert.NotEmpty(t, resp["refresh_token"])
				assert.Equal(t, "Bearer", resp["token_type"])
				assert.Equal(t, float64(900), resp["expires_in"])

				user := resp["user"].(map[string]interface{})
				assert.Equal(t, "user-1", user["id"])
				assert.Equal(t, "user@example.com", user["email"])
			},
		},
		{
			name: "valid login with tenant",
			body: map[string]string{
				"email":     "user@example.com",
				"password":  "correctpassword123",
				"tenant_id": "tenant-1",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestUser("user-1", "user@example.com", "Test User", "correctpassword123", true)
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotEmpty(t, resp["access_token"])
			},
		},
		{
			name: "wrong password",
			body: map[string]string{
				"email":    "user@example.com",
				"password": "wrongpassword",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestUser("user-1", "user@example.com", "Test User", "correctpassword123", true)
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Invalid credentials",
		},
		{
			name: "non-existent user",
			body: map[string]string{
				"email":    "nonexistent@example.com",
				"password": "somepassword123",
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Invalid credentials",
		},
		{
			name: "disabled account",
			body: map[string]string{
				"email":    "disabled@example.com",
				"password": "correctpassword123",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestUser("user-1", "disabled@example.com", "Disabled User", "correctpassword123", false)
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "disabled",
		},
		{
			name: "login with tenant - no access",
			body: map[string]string{
				"email":     "user@example.com",
				"password":  "correctpassword123",
				"tenant_id": "tenant-1",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestUser("user-1", "user@example.com", "Test User", "correctpassword123", true)
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				// User is NOT a member of the tenant
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupAuthTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/auth/login", tt.body, nil)
			w := httptest.NewRecorder()

			h.Login(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestLoginInvalidJSON(t *testing.T) {
	h, _ := setupAuthTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.Login(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// RefreshToken Handler Tests
// =============================================================================

func TestRefreshToken(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mockTenantRepository, *auth.TokenService) map[string]string
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "valid refresh without tenant",
			setupMock: func(m *mockTenantRepository, ts *auth.TokenService) map[string]string {
				m.addTestUser("user-1", "user@example.com", "Test User", "password123", true)
				refreshToken, _ := ts.GenerateRefreshToken("user-1")
				return map[string]string{"refresh_token": refreshToken}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotEmpty(t, resp["access_token"])
				assert.Equal(t, "Bearer", resp["token_type"])
				assert.Equal(t, float64(900), resp["expires_in"])
			},
		},
		{
			name: "valid refresh with tenant",
			setupMock: func(m *mockTenantRepository, ts *auth.TokenService) map[string]string {
				m.addTestUser("user-1", "user@example.com", "Test User", "password123", true)
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleAdmin},
				}
				refreshToken, _ := ts.GenerateRefreshToken("user-1")
				return map[string]string{
					"refresh_token": refreshToken,
					"tenant_id":     "tenant-1",
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotEmpty(t, resp["access_token"])
			},
		},
		{
			name: "invalid refresh token",
			setupMock: func(m *mockTenantRepository, ts *auth.TokenService) map[string]string {
				return map[string]string{"refresh_token": "invalid-token"}
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Invalid refresh token",
		},
		{
			name: "user not found",
			setupMock: func(m *mockTenantRepository, ts *auth.TokenService) map[string]string {
				// Generate token for user that doesn't exist in repo
				refreshToken, _ := ts.GenerateRefreshToken("nonexistent-user")
				return map[string]string{"refresh_token": refreshToken}
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "not found",
		},
		{
			name: "refresh with tenant - no access",
			setupMock: func(m *mockTenantRepository, ts *auth.TokenService) map[string]string {
				m.addTestUser("user-1", "user@example.com", "Test User", "password123", true)
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				// User is NOT a member of the tenant
				refreshToken, _ := ts.GenerateRefreshToken("user-1")
				return map[string]string{
					"refresh_token": refreshToken,
					"tenant_id":     "tenant-1",
				}
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupAuthTestHandlers()

			var body map[string]string
			if tt.setupMock != nil {
				body = tt.setupMock(repo, h.tokenService)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/auth/refresh", body, nil)
			w := httptest.NewRecorder()

			h.RefreshToken(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestRefreshTokenInvalidJSON(t *testing.T) {
	h, _ := setupAuthTestHandlers()

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.RefreshToken(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// GetCurrentUser Handler Tests
// =============================================================================

func TestGetCurrentUser(t *testing.T) {
	tests := []struct {
		name           string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "authenticated user",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestUser("user-1", "user@example.com", "Test User", "password123", true)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "user-1", resp["id"])
				assert.Equal(t, "user@example.com", resp["email"])
				assert.Equal(t, "Test User", resp["name"])
				assert.NotEmpty(t, resp["created_at"])
			},
		},
		{
			name:           "unauthenticated request",
			claims:         nil,
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Not authenticated",
		},
		{
			name: "user not found",
			claims: &auth.Claims{
				UserID: "nonexistent-user",
				Email:  "user@example.com",
			},
			wantStatus:     http.StatusNotFound,
			wantErrContain: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupAuthTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/me", nil, tt.claims)
			w := httptest.NewRecorder()

			h.GetCurrentUser(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

// =============================================================================
// TenantContext Middleware Tests
// =============================================================================

func TestTenantContext(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:     "valid tenant access",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleAdmin},
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "no authentication",
			tenantID: "tenant-1",
			claims:   nil,
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Authentication required",
		},
		{
			name:     "user not a member",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				// User is NOT a member
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Access denied",
		},
		{
			name:     "missing tenant ID",
			tenantID: "",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "Tenant ID required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupAuthTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			// Create a test handler that returns 200 OK
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			// Create the middleware chain
			middleware := h.TenantContext(nextHandler)

			req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tt.tenantID+"/test", nil, tt.claims)
			if tt.tenantID != "" {
				req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			}
			w := httptest.NewRecorder()

			middleware.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}
		})
	}
}
