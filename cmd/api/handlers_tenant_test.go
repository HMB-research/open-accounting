package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// =============================================================================
// Test Setup Helpers (reuse mock from handlers_auth_test.go)
// =============================================================================

// setupTenantTestHandlers creates handlers with mock services for tenant testing
func setupTenantTestHandlers() (*Handlers, *mockTenantRepository) {
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
// ListMyTenants Handler Tests
// =============================================================================

func TestListMyTenants(t *testing.T) {
	tests := []struct {
		name           string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, []map[string]interface{})
	}{
		{
			name: "user with multiple tenants",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Tenant One", "tenant-one")
				m.addTestTenant("tenant-2", "Tenant Two", "tenant-two")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsDefault: true},
				}
				m.tenantUsers["tenant-2"] = []tenant.TenantUser{
					{TenantID: "tenant-2", UserID: "user-1", Role: tenant.RoleAdmin},
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 2)
			},
		},
		{
			name: "user with no tenants",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Empty(t, resp)
			},
		},
		{
			name:           "unauthenticated request",
			claims:         nil,
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Not authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/tenants", nil, tt.claims)
			w := httptest.NewRecorder()

			h.ListMyTenants(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp []map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestTenantContextRejectsAPITokenForDifferentTenant(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Tenant One", "tenant-one")
	repo.addTestTenant("tenant-2", "Tenant Two", "tenant-two")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner, IsDefault: true},
	}
	repo.tenantUsers["tenant-2"] = []tenant.TenantUser{
		{TenantID: "tenant-2", UserID: "user-1", Role: tenant.RoleOwner},
	}

	claims := &auth.Claims{
		UserID:    "user-1",
		Email:     "user@example.com",
		TenantID:  "tenant-1",
		Role:      tenant.RoleOwner,
		TokenKind: auth.TokenKindAPIToken,
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-2/accounts", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-2"})
	w := httptest.NewRecorder()

	handler := h.TenantContext(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "scoped to a different tenant")
}

func TestCompleteOnboarding(t *testing.T) {
	tests := []struct {
		name           string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, *mockTenantRepository, map[string]interface{})
	}{
		{
			name:   "completes onboarding for tenant member",
			claims: &auth.Claims{UserID: "user-1", Email: "user@example.com"},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Tenant One", "tenant-one")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{{
					TenantID: "tenant-1",
					UserID:   "user-1",
					Role:     tenant.RoleOwner,
				}}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, m *mockTenantRepository, resp map[string]interface{}) {
				assert.Equal(t, true, resp["success"])
				assert.Equal(t, []string{"tenant-1"}, m.completedOnboardings)
				assert.True(t, m.tenants["tenant-1"].OnboardingCompleted)
			},
		},
		{
			name:           "requires authentication",
			claims:         nil,
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Not authenticated",
		},
		{
			name:   "rejects users without tenant access",
			claims: &auth.Claims{UserID: "user-2", Email: "user2@example.com"},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Tenant One", "tenant-one")
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Access denied",
		},
		{
			name:   "returns internal error when completion fails",
			claims: &auth.Claims{UserID: "user-1", Email: "user@example.com"},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Tenant One", "tenant-one")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{{
					TenantID: "tenant-1",
					UserID:   "user-1",
					Role:     tenant.RoleOwner,
				}}
				m.completeOnboardingErr = errors.New("onboarding storage failure")
			},
			wantStatus:     http.StatusInternalServerError,
			wantErrContain: "onboarding storage failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()
			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/complete-onboarding", nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			w := httptest.NewRecorder()

			h.CompleteOnboarding(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
				return
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, repo, resp)
			}
		})
	}
}

// =============================================================================
// CreateTenant Handler Tests
// =============================================================================

func TestCreateTenant(t *testing.T) {
	tests := []struct {
		name           string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name: "valid tenant creation",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "My Company",
				"slug": "my-company",
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotEmpty(t, resp["id"])
				assert.Equal(t, "My Company", resp["name"])
				assert.Equal(t, "my-company", resp["slug"])
			},
		},
		{
			name: "missing name",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"slug": "my-company",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "required",
		},
		{
			name: "missing slug",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "My Company",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "required",
		},
		{
			name: "invalid slug format",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "My Company",
				"slug": "My Company!", // Invalid characters
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "slug",
		},
		{
			name: "slug too short",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "My Company",
				"slug": "ab", // Too short
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "slug",
		},
		{
			name:           "unauthenticated request",
			claims:         nil,
			body:           map[string]interface{}{"name": "Test", "slug": "test"},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Not authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants", tt.body, tt.claims)
			w := httptest.NewRecorder()

			h.CreateTenant(w, req)

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

func TestCreateTenantInvalidJSON(t *testing.T) {
	h, _ := setupTenantTestHandlers()

	claims := &auth.Claims{UserID: "user-1", Email: "user@example.com"}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants", nil, claims)
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.CreateTenant(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// GetTenant Handler Tests
// =============================================================================

func TestGetTenant(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:     "owner can get tenant",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "tenant-1", resp["id"])
				assert.Equal(t, "Test Tenant", resp["name"])
			},
		},
		{
			name:     "member can get tenant",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleViewer},
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "non-member denied",
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
			wantErrContain: "denied",
		},
		{
			name:     "tenant not found",
			tenantID: "nonexistent",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "denied",
		},
		{
			name:           "unauthenticated request",
			tenantID:       "tenant-1",
			claims:         nil,
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Not authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tt.tenantID, nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.GetTenant(w, req)

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
// UpdateTenant Handler Tests
// =============================================================================

func TestUpdateTenant(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:     "owner can update",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "Updated Name",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Old Name", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "admin can update",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "Updated Name",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Old Name", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleAdmin},
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "viewer cannot update",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "Updated Name",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Old Name", "test-tenant")
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleViewer},
				}
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Insufficient permissions",
		},
		{
			name:     "non-member denied",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			},
			body: map[string]interface{}{
				"name": "Updated Name",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Old Name", "test-tenant")
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "denied",
		},
		{
			name:           "unauthenticated request",
			tenantID:       "tenant-1",
			claims:         nil,
			body:           map[string]interface{}{"name": "Test"},
			wantStatus:     http.StatusUnauthorized,
			wantErrContain: "Not authenticated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tt.tenantID, tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.UpdateTenant(w, req)

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

func TestUpdateTenantRejectsPeriodLockMutation(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Old Name", "test-tenant")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}

	req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1", map[string]interface{}{
		"settings": map[string]interface{}{
			"period_lock_date": "2026-01-31",
		},
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})

	w := httptest.NewRecorder()
	h.UpdateTenant(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "response body: %s", w.Body.String())

	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "close or reopen actions")
}

func TestListPeriodCloseEvents(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Tenant", "tenant")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleViewer},
	}
	repo.periodCloseEvents["tenant-1"] = []tenant.PeriodCloseEvent{
		{
			ID:            "evt-1",
			TenantID:      "tenant-1",
			Action:        tenant.PeriodCloseActionClose,
			CloseKind:     tenant.PeriodCloseKindMonthEnd,
			PeriodEndDate: "2026-02-28",
			PerformedBy:   "user-1",
			CreatedAt:     time.Now(),
		},
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/period-close-events?limit=10", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ListPeriodCloseEvents(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var events []tenant.PeriodCloseEvent
	err := json.NewDecoder(w.Body).Decode(&events)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "evt-1", events[0].ID)
}

func TestListPeriodCloseEventsRejectsInvalidLimit(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Tenant", "tenant")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleViewer},
	}

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/period-close-events?limit=0", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ListPeriodCloseEvents(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "response body: %s", w.Body.String())
	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "limit must be between 1 and 100")
}

func TestListPeriodCloseEventsHandlesRepositoryError(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Tenant", "tenant")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleViewer},
	}
	repo.listPeriodCloseEventsErr = errors.New("history failure")

	req := makeAuthenticatedRequest(http.MethodGet, "/tenants/tenant-1/period-close-events", nil, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ListPeriodCloseEvents(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, "response body: %s", w.Body.String())
}

func TestClosePeriod(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		body           map[string]interface{}
		wantStatus     int
		wantErrContain string
	}{
		{
			name:       "accountant can close period",
			role:       tenant.RoleAccountant,
			body:       map[string]interface{}{"period_end_date": "2026-01-31", "note": "Month-end close"},
			wantStatus: http.StatusOK,
		},
		{
			name:           "viewer cannot close period",
			role:           tenant.RoleViewer,
			body:           map[string]interface{}{"period_end_date": "2026-01-31"},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Insufficient permissions",
		},
		{
			name:           "invalid month-end is rejected",
			role:           tenant.RoleOwner,
			body:           map[string]interface{}{"period_end_date": "2026-01-30"},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "last day of a month",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()
			repo.addTestTenant("tenant-1", "Tenant", "tenant")
			repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
				{TenantID: "tenant-1", UserID: "user-1", Role: tt.role},
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-close", tt.body, &auth.Claims{
				UserID: "user-1",
				Email:  "user@example.com",
			})
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			w := httptest.NewRecorder()

			h.ClosePeriod(w, req)

			require.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())
			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
				return
			}

			var resp struct {
				Tenant tenant.Tenant           `json:"tenant"`
				Event  tenant.PeriodCloseEvent `json:"event"`
			}
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.NotNil(t, resp.Tenant.Settings.PeriodLockDate)
			assert.Equal(t, "2026-01-31", *resp.Tenant.Settings.PeriodLockDate)
			assert.Equal(t, tenant.PeriodCloseActionClose, resp.Event.Action)
		})
	}
}

func TestClosePeriodRequiresAuthentication(t *testing.T) {
	h, _ := setupTenantTestHandlers()

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-close", map[string]interface{}{
		"period_end_date": "2026-01-31",
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ClosePeriod(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code, "response body: %s", w.Body.String())
}

func TestClosePeriodHandlesPersistenceError(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	repo.addTestTenant("tenant-1", "Tenant", "tenant")
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}
	repo.updateTenantWithEventErr = errors.New("storage unavailable")

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-close", map[string]interface{}{
		"period_end_date": "2026-01-31",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ClosePeriod(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, "response body: %s", w.Body.String())
}

func TestReopenPeriod(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	initialSettings := tenant.DefaultSettings()
	initialSettings.PeriodLockDate = stringPtr("2026-02-28")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:        "tenant-1",
		Name:      "Tenant",
		Slug:      "tenant",
		Settings:  initialSettings,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}
	repo.periodCloseEvents["tenant-1"] = []tenant.PeriodCloseEvent{
		{
			ID:             "close-2",
			TenantID:       "tenant-1",
			Action:         tenant.PeriodCloseActionClose,
			CloseKind:      tenant.PeriodCloseKindMonthEnd,
			PeriodEndDate:  "2026-02-28",
			LockDateBefore: stringPtr("2026-01-31"),
			LockDateAfter:  stringPtr("2026-02-28"),
			PerformedBy:    "user-1",
			CreatedAt:      time.Now(),
		},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", map[string]interface{}{
		"period_end_date": "2026-02-28",
		"note":            "Need to reopen",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ReopenPeriod(w, req)

	require.Equal(t, http.StatusOK, w.Code, "response body: %s", w.Body.String())
	var resp struct {
		Tenant tenant.Tenant           `json:"tenant"`
		Event  tenant.PeriodCloseEvent `json:"event"`
	}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Tenant.Settings.PeriodLockDate)
	assert.Equal(t, "2026-01-31", *resp.Tenant.Settings.PeriodLockDate)
	assert.Equal(t, tenant.PeriodCloseActionReopen, resp.Event.Action)
}

func TestReopenPeriodRequiresNote(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	initialSettings := tenant.DefaultSettings()
	initialSettings.PeriodLockDate = stringPtr("2026-01-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:        "tenant-1",
		Name:      "Tenant",
		Slug:      "tenant",
		Settings:  initialSettings,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", map[string]interface{}{
		"period_end_date": "2026-01-31",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ReopenPeriod(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "response body: %s", w.Body.String())
	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "note is required")
}

func TestReopenPeriodRejectsUnknownClosedPeriod(t *testing.T) {
	h, repo := setupTenantTestHandlers()
	initialSettings := tenant.DefaultSettings()
	initialSettings.PeriodLockDate = stringPtr("2026-01-31")
	repo.tenants["tenant-1"] = &tenant.Tenant{
		ID:        "tenant-1",
		Name:      "Tenant",
		Slug:      "tenant",
		Settings:  initialSettings,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	repo.tenantUsers["tenant-1"] = []tenant.TenantUser{
		{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
	}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/period-reopen", map[string]interface{}{
		"period_end_date": "2026-01-31",
		"note":            "Need to fix it",
	}, &auth.Claims{
		UserID: "user-1",
		Email:  "user@example.com",
	})
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ReopenPeriod(w, req)

	require.Equal(t, http.StatusConflict, w.Code, "response body: %s", w.Body.String())
	var resp map[string]string
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "has not been closed yet")
}

// =============================================================================
// ListTenantUsers Handler Tests
// =============================================================================

func TestListTenantUsers(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, []map[string]interface{})
	}{
		{
			name:     "list tenant users",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				Email:    "user@example.com",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.addTestUser("user-1", "owner@example.com", "Owner", "password", true)
				m.addTestUser("user-2", "admin@example.com", "Admin", "password", true)
				m.tenantUsers["tenant-1"] = []tenant.TenantUser{
					{TenantID: "tenant-1", UserID: "user-1", Role: tenant.RoleOwner},
					{TenantID: "tenant-1", UserID: "user-2", Role: tenant.RoleAdmin},
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 2)
			},
		},
		{
			name:     "viewer cannot list users",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				Email:    "user@example.com",
				TenantID: "tenant-1",
				Role:     tenant.RoleViewer,
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tt.tenantID+"/users", nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.ListTenantUsers(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp []map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

// =============================================================================
// CreateInvitation Handler Tests
// =============================================================================

func TestCreateInvitation(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		body           map[string]string
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:     "owner can invite",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				Email:    "owner@example.com",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]string{
				"email": "newuser@example.com",
				"role":  tenant.RoleAdmin,
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.addTestUser("user-1", "owner@example.com", "Owner", "password", true)
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:     "admin can invite",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				Email:    "admin@example.com",
				TenantID: "tenant-1",
				Role:     tenant.RoleAdmin,
			},
			body: map[string]string{
				"email": "newuser@example.com",
				"role":  tenant.RoleAccountant,
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:     "viewer cannot invite",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				Email:    "viewer@example.com",
				TenantID: "tenant-1",
				Role:     tenant.RoleViewer,
			},
			body: map[string]string{
				"email": "newuser@example.com",
				"role":  tenant.RoleViewer,
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Permission denied",
		},
		{
			name:     "missing email",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]string{
				"role": tenant.RoleAdmin,
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "required",
		},
		{
			name:     "missing role",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]string{
				"email": "newuser@example.com",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/invitations", tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.CreateInvitation(w, req)

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

// =============================================================================
// AcceptInvitation Handler Tests
// =============================================================================

func TestAcceptInvitation(t *testing.T) {
	tests := []struct {
		name           string
		body           map[string]string
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name: "valid token acceptance",
			body: map[string]string{
				"token":    "valid-token",
				"password": "newpassword123",
				"name":     "New User",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.invitations["valid-token"] = &tenant.UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "newuser@example.com",
					Role:      tenant.RoleAdmin,
					Token:     "valid-token",
					ExpiresAt: time.Now().Add(24 * time.Hour),
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "missing token",
			body: map[string]string{
				"password": "newpassword123",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "Token is required",
		},
		{
			name: "invalid token",
			body: map[string]string{
				"token": "invalid-token",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "not found",
		},
		{
			name: "expired token",
			body: map[string]string{
				"token":    "expired-token",
				"password": "newpassword123",
				"name":     "New User",
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.invitations["expired-token"] = &tenant.UserInvitation{
					ID:        "inv-1",
					TenantID:  "tenant-1",
					Email:     "newuser@example.com",
					Role:      tenant.RoleAdmin,
					Token:     "expired-token",
					ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired
				}
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/invitations/accept", tt.body, nil)
			w := httptest.NewRecorder()

			h.AcceptInvitation(w, req)

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

func TestAcceptInvitationInvalidJSON(t *testing.T) {
	h, _ := setupTenantTestHandlers()

	req := makeAuthenticatedRequest(http.MethodPost, "/invitations/accept", nil, nil)
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.AcceptInvitation(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// GetInvitationByToken Handler Tests
// =============================================================================

func TestGetInvitationByToken(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:  "valid token",
			token: "valid-token",
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.invitations["valid-token"] = &tenant.UserInvitation{
					ID:         "inv-1",
					TenantID:   "tenant-1",
					TenantName: "Test Tenant",
					Email:      "newuser@example.com",
					Role:       tenant.RoleAdmin,
					Token:      "valid-token",
					ExpiresAt:  time.Now().Add(24 * time.Hour),
				}
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "newuser@example.com", resp["email"])
				assert.Equal(t, "Test Tenant", resp["tenant_name"])
			},
		},
		{
			name:           "invalid token",
			token:          "invalid-token",
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/invitations/"+tt.token, nil, nil)
			req = withURLParams(req, map[string]string{"token": tt.token})
			w := httptest.NewRecorder()

			h.GetInvitationByToken(w, req)

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
// ListInvitations Handler Tests
// =============================================================================

func TestListInvitations(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:     "owner can list invitations",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "admin can list invitations",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleAdmin,
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "viewer cannot list invitations",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleViewer,
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tt.tenantID+"/invitations", nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.ListInvitations(w, req)

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

// =============================================================================
// RevokeInvitation Handler Tests
// =============================================================================

func TestRevokeInvitation(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		invitationID   string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:         "owner can revoke",
			tenantID:     "tenant-1",
			invitationID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(m *mockTenantRepository) {
				m.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				m.invitations["token-1"] = &tenant.UserInvitation{
					ID:       "inv-1",
					TenantID: "tenant-1",
					Token:    "token-1",
				}
			},
			wantStatus: http.StatusOK,
		},
		{
			name:         "viewer cannot revoke",
			tenantID:     "tenant-1",
			invitationID: "inv-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleViewer,
			},
			wantStatus:     http.StatusForbidden,
			wantErrContain: "Permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, repo := setupTenantTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(repo)
			}

			req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/"+tt.tenantID+"/invitations/"+tt.invitationID, nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID, "invitationID": tt.invitationID})
			w := httptest.NewRecorder()

			h.RevokeInvitation(w, req)

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

func stringPtr(value string) *string {
	return &value
}
