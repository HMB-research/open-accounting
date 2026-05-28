package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func newTestCLIApp() (*cliApp, *strings.Builder, *strings.Builder) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	return &cliApp{stdout: stdout, stderr: stderr}, stdout, stderr
}

func configureCLIEnv(t *testing.T) {
	t.Helper()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", tempHome)
	t.Setenv("OA_BASE_URL", "")
	t.Setenv("OA_API_TOKEN", "")
	t.Setenv("OA_TENANT_ID", "")
}

func writeTempCSV(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestCLIAppRunHelpAndUnknownCommand(t *testing.T) {
	t.Parallel()

	app, stdout, _ := newTestCLIApp()

	require.NoError(t, app.run(context.Background(), nil))
	assert.Contains(t, stdout.String(), "Open Accounting CLI")

	stdout.Reset()
	require.NoError(t, app.run(context.Background(), []string{"help"}))
	assert.Contains(t, stdout.String(), "Commands:")

	err := app.run(context.Background(), []string{"nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown command "nope"`)
}

func TestCLIOperationalCommands(t *testing.T) {
	configureCLIEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			_, _ = w.Write([]byte("OK"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/demo/status":
			w.Header().Set("Content-Type", "application/json")
			require.Equal(t, "demo-secret", r.Header.Get("X-Demo-Secret"))
			assert.Equal(t, "1", r.URL.Query().Get("user"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user":     1,
				"accounts": map[string]any{"count": 3, "keys": []string{"Cash"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/demo/reset":
			w.Header().Set("Content-Type", "application/json")
			require.Equal(t, "demo-secret", r.Header.Get("X-Demo-Secret"))
			assert.Equal(t, "2", r.URL.Query().Get("user"))
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "reset"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"health", "--base-url", server.URL})
	require.NoError(t, err)
	assert.Equal(t, "OK\n", stdout.String())

	stdout.Reset()
	err = app.run(context.Background(), []string{"demo", "status", "--base-url", server.URL, "--secret", "demo-secret", "--user", "1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "\"user\": 1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"demo", "reset", "--base-url", server.URL, "--secret", "demo-secret", "--user", "2"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "\"message\": \"reset\"")
}

func TestCLIAuthInitStatusAndLogoutFlow(t *testing.T) {
	configureCLIEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt-123"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/me/tenants":
			assert.Contains(t, []string{"Bearer jwt-123", "Bearer oa_raw_token_123456789"}, r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode([]tenant.TenantMembership{
				{
					Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"},
					Role:   tenant.RoleAdmin,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens":
			require.Equal(t, "Bearer jwt-123", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "oa_raw_token_123456789",
				"api_token": map[string]any{
					"id":           "token-1",
					"tenant_id":    "tenant-1",
					"user_id":      "user-1",
					"name":         "CLI Token",
					"token_prefix": "oa_raw_token_",
					"created_at":   "2026-03-12T00:00:00Z",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/me":
			require.Equal(t, "Bearer oa_raw_token_123456789", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":    "user-1",
				"name":  "CLI User",
				"email": "cli@example.com",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/sessions":
			require.Equal(t, "Bearer oa_raw_token_123456789", r.Header.Get("Authorization"))
			assert.Equal(t, "true", r.URL.Query().Get("include_inactive"))
			_ = json.NewEncoder(w).Encode([]map[string]string{
				{
					"id":         "session-1",
					"user_id":    "user-1",
					"created_at": "2026-05-28T12:00:00Z",
					"expires_at": "2026-06-04T12:00:00Z",
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/auth/sessions/session-1":
			require.Equal(t, "Bearer oa_raw_token_123456789", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"auth",
		"init",
		"--base-url", server.URL,
		"--email", "cli@example.com",
		"--password", "secret",
		"--tenant", "alpha",
		"--token-name", "CLI Token",
		"--expires-in-days", "30",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Stored API token for tenant Alpha (tenant-1)")
	assert.Contains(t, stdout.String(), "Token preview")

	cfg, err := loadStoredConfig()
	require.NoError(t, err)
	assert.Equal(t, server.URL, cfg.BaseURL)
	assert.Equal(t, "tenant-1", cfg.TenantID)
	assert.Equal(t, "oa_raw_token_123456789", cfg.APIToken)

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "tenants"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Alpha")
	assert.Contains(t, stdout.String(), "admin")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "status"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "CLI User <cli@example.com>")
	assert.Contains(t, stdout.String(), "Tenant: Alpha (tenant-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "sessions", "--include-inactive"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "session-1")
	assert.Contains(t, stdout.String(), "active")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "revoke-session", "--id", "session-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Revoked refresh session")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "logout"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Removed local CLI config")

	_, err = loadStoredConfig()
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestCLIAuthPublicCommands(t *testing.T) {
	configureCLIEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/register":
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "new@example.com", req["email"])
			assert.Equal(t, "New User", req["name"])
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":    "user-2",
				"email": "new@example.com",
				"name":  "New User",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "new@example.com", req["email"])
			assert.Equal(t, "secret123", req["password"])
			assert.Equal(t, "tenant-1", req["tenant_id"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "jwt-login",
				"refresh_token": "refresh-login",
				"token_type":    "Bearer",
				"expires_in":    900,
				"user": map[string]string{
					"id":    "user-2",
					"email": "new@example.com",
					"name":  "New User",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/refresh":
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "refresh-123", req["refresh_token"])
			assert.Equal(t, "tenant-1", req["tenant_id"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "jwt-refreshed",
				"token_type":   "Bearer",
				"expires_in":   900,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/logout":
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "refresh-123", req["refresh_token"])
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"auth", "register", "--base-url", server.URL, "--email", "new@example.com", "--password", "secret123", "--name", "New User"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Registered New User")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "login", "--base-url", server.URL, "--email", "new@example.com", "--password", "secret123", "--tenant-id", "tenant-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Access token: jwt-login")
	assert.Contains(t, stdout.String(), "Refresh token: refresh-login")
	assert.Contains(t, stdout.String(), "User: New User <new@example.com> (user-2)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "refresh", "--base-url", server.URL, "--refresh-token", "refresh-123", "--tenant-id", "tenant-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "jwt-refreshed")
	assert.Contains(t, stdout.String(), "900 seconds")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "logout", "--base-url", server.URL, "--refresh-token", "refresh-123"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Revoked refresh session")
	assert.Contains(t, stdout.String(), "Removed local CLI config")
}

func TestCLITokenCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":           "token-1",
				"name":         "CLI",
				"token_prefix": "oa_token_123",
				"created_at":   "2026-03-12T00:00:00Z",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "oa_created_token",
				"api_token": map[string]any{
					"id":           "token-2",
					"name":         "Nightly",
					"token_prefix": "oa_created_to",
					"created_at":   "2026-03-12T00:00:00Z",
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens/token-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"tokens", "list", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "CLI"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"tokens", "create", "--name", "Nightly"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created token Nightly (token-2)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tokens", "revoke", "--id", "token-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Revoked token token-1")
}

func TestCLITenantCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	tenantResponse := func(id, name, slug, email string, onboardingComplete bool) map[string]any {
		return map[string]any{
			"id":                   id,
			"name":                 name,
			"slug":                 slug,
			"schema_name":          "tenant_" + id,
			"is_active":            true,
			"onboarding_completed": onboardingComplete,
			"settings": map[string]any{
				"default_currency": "EUR",
				"country_code":     "EE",
				"timezone":         "Europe/Tallinn",
				"email":            email,
			},
			"created_at": "2026-03-12T00:00:00Z",
			"updated_at": "2026-03-12T00:00:00Z",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1":
			_ = json.NewEncoder(w).Encode(tenantResponse("tenant-1", "Alpha", "alpha", "finance@example.com", false))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants":
			var req tenant.CreateTenantRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Beta", req.Name)
			assert.Equal(t, "beta", req.Slug)
			require.NotNil(t, req.Settings)
			assert.Equal(t, "EUR", req.Settings.DefaultCurrency)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(tenantResponse("tenant-2", "Beta", "beta", "", false))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1":
			var req tenant.UpdateTenantRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.NotNil(t, req.Name)
			assert.Equal(t, "Alpha Finance", *req.Name)
			require.NotNil(t, req.Settings)
			assert.Equal(t, "finance@example.com", req.Settings.Email)
			_ = json.NewEncoder(w).Encode(tenantResponse("tenant-1", "Alpha Finance", "alpha", "finance@example.com", false))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/complete-onboarding":
			_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"tenant", "get"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Tenant Alpha")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tenant", "create", "--name", "Beta", "--slug", "beta", "--settings-json", `{"default_currency":"EUR","country_code":"EE","timezone":"Europe/Tallinn"}`})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Tenant Beta")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tenant", "update", "--name", "Alpha Finance", "--settings-json", `{"email":"finance@example.com","timezone":"Europe/Tallinn"}`})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Tenant Alpha Finance")
	assert.Contains(t, stdout.String(), "finance@example.com")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tenant", "complete-onboarding"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Marked tenant tenant-1 onboarding complete")
}

func TestCLIUsersCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/users":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"tenant_id":  "tenant-1",
				"user_id":    "user-2",
				"role":       "viewer",
				"is_default": false,
				"created_at": "2026-03-12T00:00:00Z",
			}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/users/user-2/role":
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "accountant", req["role"])
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/users/user-2":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"users", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "user-2")
	assert.Contains(t, stdout.String(), "viewer")

	stdout.Reset()
	err = app.run(context.Background(), []string{"users", "update-role", "--id", "user-2", "--role", "accountant"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Updated user user-2 role to accountant")

	stdout.Reset()
	err = app.run(context.Background(), []string{"users", "remove", "--id", "user-2"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Removed user user-2")
}

func TestCLIInvitationCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	invitationResponse := map[string]any{
		"id":          "inv-1",
		"tenant_id":   "tenant-1",
		"tenant_name": "Alpha",
		"email":       "new@example.com",
		"role":        "accountant",
		"invited_by":  "user-1",
		"expires_at":  "2026-03-19T00:00:00Z",
		"created_at":  "2026-03-12T00:00:00Z",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/v1/tenants/") {
			require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))
		} else {
			assert.Empty(t, r.Header.Get("Authorization"))
		}

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invitations":
			_ = json.NewEncoder(w).Encode([]map[string]any{invitationResponse})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invitations":
			var req tenant.CreateInvitationRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "new@example.com", req.Email)
			assert.Equal(t, tenant.RoleAccountant, req.Role)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(invitationResponse)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/invitations/inv-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "revoked"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/invitations/public-token":
			_ = json.NewEncoder(w).Encode(invitationResponse)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/invitations/accept":
			var req tenant.AcceptInvitationRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "public-token", req.Token)
			assert.Equal(t, "secret", req.Password)
			assert.Equal(t, "New User", req.Name)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant": map[string]any{
					"id":   "tenant-1",
					"name": "Alpha",
					"slug": "alpha",
					"settings": map[string]any{
						"default_currency": "EUR",
						"country_code":     "EE",
						"timezone":         "Europe/Tallinn",
					},
					"is_active":  true,
					"created_at": "2026-03-12T00:00:00Z",
					"updated_at": "2026-03-12T00:00:00Z",
				},
				"role":       "accountant",
				"is_default": false,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"invitations", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "new@example.com")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invitations", "create", "--email", "new@example.com", "--role", "accountant"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Invited new@example.com as accountant")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invitations", "revoke", "--id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Revoked invitation inv-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invitations", "get", "--token", "public-token", "--base-url", server.URL})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "new@example.com")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invitations", "accept", "--token", "public-token", "--name", "New User", "--password", "secret", "--base-url", server.URL})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Joined tenant Alpha")
}

func TestCLIPluginCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	const pluginID = "11111111-1111-1111-1111-111111111111"
	const tenantUUID = "22222222-2222-2222-2222-222222222222"
	const tenantPluginID = "33333333-3333-3333-3333-333333333333"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/plugins":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":         tenantPluginID,
				"tenant_id":  tenantUUID,
				"plugin_id":  pluginID,
				"is_enabled": true,
				"settings":   map[string]any{"threshold": 5},
				"created_at": "2026-03-12T00:00:00Z",
				"updated_at": "2026-03-12T00:00:00Z",
				"plugin": map[string]any{
					"id":                  pluginID,
					"name":                "vat-tools",
					"display_name":        "VAT Tools",
					"version":             "1.0.0",
					"repository_url":      "https://github.com/example/vat-tools",
					"repository_type":     "github",
					"state":               "enabled",
					"granted_permissions": []string{"invoices:read"},
					"manifest":            map[string]any{},
					"installed_at":        "2026-03-12T00:00:00Z",
					"updated_at":          "2026-03-12T00:00:00Z",
				},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/plugins/"+pluginID+"/enable":
			var req plugin.TenantPluginSettingsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.JSONEq(t, `{"threshold":5}`, string(req.Settings))
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/plugins/"+pluginID+"/disable":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/plugins/"+pluginID+"/settings":
			_, _ = w.Write([]byte(`{"threshold":5}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/plugins/"+pluginID+"/settings":
			var settings map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&settings))
			assert.Equal(t, float64(8), settings["threshold"])
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"plugins", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "VAT Tools")

	stdout.Reset()
	err = app.run(context.Background(), []string{"plugins", "enable", "--id", pluginID, "--settings-json", `{"threshold":5}`})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Enabled tenant plugin")

	stdout.Reset()
	err = app.run(context.Background(), []string{"plugins", "settings", "get", "--id", pluginID})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"threshold": 5`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"plugins", "settings", "update", "--id", pluginID, "--settings-json", `{"threshold":8}`})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Updated tenant plugin")

	stdout.Reset()
	err = app.run(context.Background(), []string{"plugins", "disable", "--id", pluginID})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Disabled tenant plugin")
}

func TestCLIAdminPluginCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:  "https://placeholder.example.com",
		APIToken: "oa_saved_token",
	}))

	const pluginID = "11111111-1111-1111-1111-111111111111"
	const registryID = "44444444-4444-4444-4444-444444444444"

	pluginResponse := map[string]any{
		"id":                  pluginID,
		"name":                "vat-tools",
		"display_name":        "VAT Tools",
		"version":             "1.0.0",
		"repository_url":      "https://github.com/example/vat-tools",
		"repository_type":     "github",
		"state":               "enabled",
		"granted_permissions": []string{"invoices:read"},
		"manifest":            map[string]any{},
		"installed_at":        "2026-03-12T00:00:00Z",
		"updated_at":          "2026-03-12T00:00:00Z",
	}
	registryResponse := map[string]any{
		"id":          registryID,
		"name":        "Official",
		"url":         "https://plugins.example.com",
		"description": "Official plugins",
		"is_official": false,
		"is_active":   true,
		"created_at":  "2026-03-12T00:00:00Z",
		"updated_at":  "2026-03-12T00:00:00Z",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/plugin-registries":
			_ = json.NewEncoder(w).Encode([]map[string]any{registryResponse})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/plugin-registries":
			var req plugin.CreateRegistryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Official", req.Name)
			assert.Equal(t, "https://plugins.example.com", req.URL)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(registryResponse)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/plugin-registries/"+registryID+"/sync":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "synced"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/admin/plugin-registries/"+registryID:
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/plugins":
			_ = json.NewEncoder(w).Encode([]map[string]any{pluginResponse})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/plugins/search":
			assert.Equal(t, "vat", r.URL.Query().Get("q"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"plugin": map[string]any{
					"name":         "vat-tools",
					"display_name": "VAT Tools",
					"repository":   "https://github.com/example/vat-tools",
					"version":      "1.0.0",
				},
				"registry": "Official",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/plugins/permissions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"invoices:read": map[string]any{
					"name":        "invoices:read",
					"category":    "data",
					"risk":        "low",
					"description": "Read invoices",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/plugins/install":
			var req plugin.InstallPluginRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "https://github.com/example/vat-tools", req.RepositoryURL)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(pluginResponse)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/admin/plugins/"+pluginID:
			_ = json.NewEncoder(w).Encode(pluginResponse)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/plugins/"+pluginID+"/enable":
			var req plugin.EnablePluginRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, []string{"invoices:read"}, req.GrantedPermissions)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "enabled"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/plugins/"+pluginID+"/disable":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "disabled"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/admin/plugins/"+pluginID:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"admin", "registries", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Official")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "registries", "create", "--name", "Official", "--url", "https://plugins.example.com"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Official")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "registries", "sync", "--id", registryID})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Synced plugin registry")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "registries", "delete", "--id", registryID})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Removed plugin registry")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "VAT Tools")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "search", "--q", "vat"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "VAT Tools")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "permissions"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "invoices:read")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "install", "--repository-url", "https://github.com/example/vat-tools"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "VAT Tools")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "get", "--id", pluginID})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "VAT Tools")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "enable", "--id", pluginID, "--permission", "invoices:read"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Enabled plugin")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "disable", "--id", pluginID})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Disabled plugin")

	stdout.Reset()
	err = app.run(context.Background(), []string{"admin", "plugins", "uninstall", "--id", pluginID})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Uninstalled plugin")
}

func TestCLIAccountsCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	importFile := writeTempCSV(t, "accounts.csv", "code,name,type\n1000,Cash,ASSET\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/accounts":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":           "acc-1",
				"code":         "1000",
				"name":         "Cash",
				"account_type": "ASSET",
				"is_active":    true,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/accounts":
			var req accounting.CreateAccountRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, accounting.AccountTypeAsset, req.AccountType)
			assert.Equal(t, "1000", req.Code)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "acc-1",
				"code":         req.Code,
				"name":         req.Name,
				"account_type": req.AccountType,
				"is_active":    true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/accounts/acc-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "acc-1",
				"tenant_id":    "tenant-1",
				"code":         "1000",
				"name":         "Cash",
				"account_type": "ASSET",
				"is_active":    true,
				"is_system":    false,
				"description":  "Main cash account",
				"created_at":   "2026-03-12T00:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/accounts/import":
			var req accounting.ImportAccountsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "accounts.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "Cash")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":   1,
				"accounts_created": 1,
				"rows_skipped":     0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"accounts", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"code": "1000"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"accounts",
		"create",
		"--code", "1000",
		"--name", "Cash",
		"--type", "asset",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created account 1000 (acc-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"accounts", "get", "--id", "acc-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Account 1000 Cash")
	assert.Contains(t, stdout.String(), "Main cash account")

	stdout.Reset()
	err = app.run(context.Background(), []string{"accounts", "import", "--file", importFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 accounts, skipped 0 rows")
}

func TestCLIInvoiceLifecycleCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	invoicePayload := func(status string) map[string]any {
		return map[string]any{
			"id":             "inv-1",
			"tenant_id":      "tenant-1",
			"invoice_number": "INV-00001",
			"invoice_type":   "SALES",
			"contact_id":     "contact-1",
			"contact": map[string]any{
				"id":           "contact-1",
				"name":         "Acme",
				"contact_type": "CUSTOMER",
				"is_active":    true,
			},
			"issue_date":      "2026-03-15T00:00:00Z",
			"due_date":        "2026-03-29T00:00:00Z",
			"currency":        "EUR",
			"exchange_rate":   "1.00",
			"subtotal":        "180.00",
			"vat_amount":      "39.60",
			"total":           "219.60",
			"base_subtotal":   "180.00",
			"base_vat_amount": "39.60",
			"base_total":      "219.60",
			"amount_paid":     "0.00",
			"status":          status,
			"reference":       "REF-1",
			"notes":           "March services",
			"created_at":      "2026-03-15T12:00:00Z",
			"created_by":      "user-1",
			"updated_at":      "2026-03-15T12:00:00Z",
			"lines": []map[string]any{{
				"id":               "line-1",
				"tenant_id":        "tenant-1",
				"invoice_id":       "inv-1",
				"line_number":      1,
				"description":      "Consulting",
				"quantity":         "2.00",
				"unit":             "hour",
				"unit_price":       "100.00",
				"discount_percent": "10.00",
				"vat_rate":         "22.00",
				"line_subtotal":    "180.00",
				"line_vat":         "39.60",
				"line_total":       "219.60",
				"account_id":       "acc-1",
				"product_id":       "prod-1",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices":
			w.Header().Set("Content-Type", "application/json")
			require.Equal(t, "SALES", r.URL.Query().Get("type"))
			require.Equal(t, "DRAFT", r.URL.Query().Get("status"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			require.Equal(t, "INV", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{invoicePayload("DRAFT")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices":
			w.Header().Set("Content-Type", "application/json")
			var req invoicing.CreateInvoiceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, invoicing.InvoiceTypeSales, req.InvoiceType)
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-15", req.IssueDate.Format("2006-01-02"))
			assert.Equal(t, "2026-03-29", req.DueDate.Format("2006-01-02"))
			assert.Equal(t, "EUR", req.Currency)
			require.Len(t, req.Lines, 1)
			line := req.Lines[0]
			assert.Equal(t, "Consulting", line.Description)
			assert.True(t, line.Quantity.Equal(decimal.RequireFromString("2.00")))
			assert.True(t, line.UnitPrice.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, line.DiscountPercent.Equal(decimal.RequireFromString("10.00")))
			assert.True(t, line.VATRate.Equal(decimal.RequireFromString("22.00")))
			require.NotNil(t, line.AccountID)
			require.NotNil(t, line.ProductID)
			assert.Equal(t, "acc-1", *line.AccountID)
			assert.Equal(t, "prod-1", *line.ProductID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(invoicePayload("DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(invoicePayload("DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4 invoice"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/send":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/void":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "voided"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"invoices", "list",
		"--type", "sales",
		"--status", "draft",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--search", "INV",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"invoice_number": "INV-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"invoices", "create",
		"--type", "sales",
		"--contact-id", "contact-1",
		"--issue-date", "2026-03-15",
		"--due-date", "2026-03-29",
		"--currency", "eur",
		"--reference", "REF-1",
		"--notes", "March services",
		"--line", "description=Consulting,quantity=2,unit=hour,unit_price=100.00,discount_percent=10.00,vat_rate=22.00,account_id=acc-1,product_id=prod-1",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created invoice INV-00001 (inv-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "get", "--id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Invoice INV-00001")
	assert.Contains(t, stdout.String(), "Consulting")

	stdout.Reset()
	outputPath := filepath.Join(t.TempDir(), "invoice.pdf")
	err = app.run(context.Background(), []string{"invoices", "pdf", "--id", "inv-1", "--output", outputPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote Invoice PDF")
	pdf, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.4 invoice", string(pdf))

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "send", "--id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Sent invoice inv-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "void", "--id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Voided invoice inv-1")
}

func TestCLIQuoteCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	quotePayload := func(id, number, status string) map[string]any {
		return map[string]any{
			"id":           id,
			"tenant_id":    "tenant-1",
			"quote_number": number,
			"contact_id":   "contact-1",
			"contact": map[string]any{
				"id":           "contact-1",
				"name":         "Acme",
				"contact_type": "CUSTOMER",
				"is_active":    true,
			},
			"quote_date":    "2026-03-15T00:00:00Z",
			"valid_until":   "2026-04-15T00:00:00Z",
			"status":        status,
			"currency":      "EUR",
			"exchange_rate": "1.00",
			"subtotal":      "180.00",
			"vat_amount":    "39.60",
			"total":         "219.60",
			"notes":         "March offer",
			"created_at":    "2026-03-15T12:00:00Z",
			"created_by":    "user-1",
			"updated_at":    "2026-03-15T12:00:00Z",
			"lines": []map[string]any{{
				"id":               "line-1",
				"tenant_id":        "tenant-1",
				"quote_id":         id,
				"line_number":      1,
				"description":      "Consulting",
				"quantity":         "2.00",
				"unit":             "hour",
				"unit_price":       "100.00",
				"discount_percent": "10.00",
				"vat_rate":         "22.00",
				"line_subtotal":    "180.00",
				"line_vat":         "39.60",
				"line_total":       "219.60",
				"product_id":       "prod-1",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/quotes":
			require.Equal(t, "DRAFT", r.URL.Query().Get("status"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			require.Equal(t, "QUO", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{quotePayload("quote-1", "QUO-00001", "DRAFT")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes":
			var req quotes.CreateQuoteRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-15", req.QuoteDate.Format("2006-01-02"))
			require.NotNil(t, req.ValidUntil)
			assert.Equal(t, "2026-04-15", req.ValidUntil.Format("2006-01-02"))
			assert.Equal(t, "EUR", req.Currency)
			assert.Equal(t, "March offer", req.Notes)
			require.Len(t, req.Lines, 1)
			line := req.Lines[0]
			assert.Equal(t, "Consulting", line.Description)
			assert.True(t, line.Quantity.Equal(decimal.RequireFromString("2.00")))
			assert.True(t, line.UnitPrice.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, line.DiscountPercent.Equal(decimal.RequireFromString("10.00")))
			assert.True(t, line.VATRate.Equal(decimal.RequireFromString("22.00")))
			require.NotNil(t, line.ProductID)
			assert.Equal(t, "prod-1", *line.ProductID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(quotePayload("quote-1", "QUO-00001", "DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1":
			_ = json.NewEncoder(w).Encode(quotePayload("quote-1", "QUO-00001", "DRAFT"))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1":
			var req quotes.UpdateQuoteRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-16", req.QuoteDate.Format("2006-01-02"))
			assert.Equal(t, "Updated offer", req.Notes)
			require.Len(t, req.Lines, 1)
			assert.Equal(t, "Updated consulting", req.Lines[0].Description)
			_ = json.NewEncoder(w).Encode(quotePayload("quote-1", "QUO-00002", "DRAFT"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1/send":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1/accept":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1/reject":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"quotes", "list",
		"--status", "draft",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--search", "QUO",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"quote_number": "QUO-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"quotes", "create",
		"--contact-id", "contact-1",
		"--quote-date", "2026-03-15",
		"--valid-until", "2026-04-15",
		"--currency", "eur",
		"--notes", "March offer",
		"--line", "description=Consulting,quantity=2,unit=hour,unit_price=100.00,discount_percent=10.00,vat_rate=22.00,product_id=prod-1",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created quote QUO-00001 (quote-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "get", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Quote QUO-00001")
	assert.Contains(t, stdout.String(), "Consulting")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"quotes", "update",
		"--id", "quote-1",
		"--contact-id", "contact-1",
		"--quote-date", "2026-03-16",
		"--currency", "eur",
		"--notes", "Updated offer",
		"--line", "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Quote QUO-00002")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "send", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Sent quote quote-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "accept", "--id", "quote-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "accepted"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "reject", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Rejected quote quote-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "delete", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted quote quote-1")
}

func TestCLIOrderCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	orderPayload := func(id, number, status string) map[string]any {
		return map[string]any{
			"id":           id,
			"tenant_id":    "tenant-1",
			"order_number": number,
			"contact_id":   "contact-1",
			"contact": map[string]any{
				"id":           "contact-1",
				"name":         "Acme",
				"contact_type": "CUSTOMER",
				"is_active":    true,
			},
			"order_date":        "2026-03-15T00:00:00Z",
			"expected_delivery": "2026-03-22T00:00:00Z",
			"status":            status,
			"currency":          "EUR",
			"exchange_rate":     "1.00",
			"subtotal":          "180.00",
			"vat_amount":        "39.60",
			"total":             "219.60",
			"notes":             "March order",
			"quote_id":          "quote-1",
			"created_at":        "2026-03-15T12:00:00Z",
			"created_by":        "user-1",
			"updated_at":        "2026-03-15T12:00:00Z",
			"lines": []map[string]any{{
				"id":               "line-1",
				"tenant_id":        "tenant-1",
				"order_id":         id,
				"line_number":      1,
				"description":      "Consulting",
				"quantity":         "2.00",
				"unit":             "hour",
				"unit_price":       "100.00",
				"discount_percent": "10.00",
				"vat_rate":         "22.00",
				"line_subtotal":    "180.00",
				"line_vat":         "39.60",
				"line_total":       "219.60",
				"product_id":       "prod-1",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/orders":
			require.Equal(t, "CONFIRMED", r.URL.Query().Get("status"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			require.Equal(t, "ORD", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{orderPayload("order-1", "ORD-00001", "CONFIRMED")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders":
			var req orders.CreateOrderRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-15", req.OrderDate.Format("2006-01-02"))
			require.NotNil(t, req.ExpectedDelivery)
			assert.Equal(t, "2026-03-22", req.ExpectedDelivery.Format("2006-01-02"))
			assert.Equal(t, "EUR", req.Currency)
			assert.Equal(t, "March order", req.Notes)
			require.NotNil(t, req.QuoteID)
			assert.Equal(t, "quote-1", *req.QuoteID)
			require.Len(t, req.Lines, 1)
			line := req.Lines[0]
			assert.Equal(t, "Consulting", line.Description)
			assert.True(t, line.Quantity.Equal(decimal.RequireFromString("2.00")))
			assert.True(t, line.UnitPrice.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, line.DiscountPercent.Equal(decimal.RequireFromString("10.00")))
			assert.True(t, line.VATRate.Equal(decimal.RequireFromString("22.00")))
			require.NotNil(t, line.ProductID)
			assert.Equal(t, "prod-1", *line.ProductID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(orderPayload("order-1", "ORD-00001", "PENDING"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1":
			_ = json.NewEncoder(w).Encode(orderPayload("order-1", "ORD-00001", "CONFIRMED"))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1":
			var req orders.UpdateOrderRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-16", req.OrderDate.Format("2006-01-02"))
			assert.Equal(t, "Updated order", req.Notes)
			require.Len(t, req.Lines, 1)
			assert.Equal(t, "Updated consulting", req.Lines[0].Description)
			_ = json.NewEncoder(w).Encode(orderPayload("order-1", "ORD-00002", "CONFIRMED"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/confirm":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "confirmed"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/process":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/ship":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "shipped"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/deliver":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "delivered"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/cancel":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "canceled"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"orders", "list",
		"--status", "confirmed",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--search", "ORD",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"order_number": "ORD-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"orders", "create",
		"--contact-id", "contact-1",
		"--order-date", "2026-03-15",
		"--expected-delivery", "2026-03-22",
		"--currency", "eur",
		"--notes", "March order",
		"--quote-id", "quote-1",
		"--line", "description=Consulting,quantity=2,unit=hour,unit_price=100.00,discount_percent=10.00,vat_rate=22.00,product_id=prod-1",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created order ORD-00001 (order-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "get", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Order ORD-00001")
	assert.Contains(t, stdout.String(), "Consulting")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"orders", "update",
		"--id", "order-1",
		"--contact-id", "contact-1",
		"--order-date", "2026-03-16",
		"--currency", "eur",
		"--notes", "Updated order",
		"--line", "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Order ORD-00002")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "confirm", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Confirmed order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "process", "--id", "order-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "processing"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "ship", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Shipped order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "deliver", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Delivered order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "cancel", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Canceled order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "delete", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted order order-1")
}

func TestCLIRecurringInvoiceCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	recurringPayload := func(id, name string, active bool) map[string]any {
		return map[string]any{
			"id":                       id,
			"tenant_id":                "tenant-1",
			"name":                     name,
			"contact_id":               "contact-1",
			"contact_name":             "Acme",
			"invoice_type":             "SALES",
			"currency":                 "EUR",
			"frequency":                "MONTHLY",
			"start_date":               "2026-03-15T00:00:00Z",
			"end_date":                 "2026-12-31T00:00:00Z",
			"next_generation_date":     "2026-04-15T00:00:00Z",
			"payment_terms_days":       21,
			"reference":                "RET-1",
			"notes":                    "Monthly services",
			"is_active":                active,
			"generated_count":          2,
			"created_at":               "2026-03-15T12:00:00Z",
			"created_by":               "user-1",
			"updated_at":               "2026-03-15T12:00:00Z",
			"send_email_on_generation": true,
			"email_template_type":      "INVOICE_SEND",
			"recipient_email_override": "billing@example.com",
			"attach_pdf_to_email":      true,
			"email_subject_override":   "Monthly invoice",
			"email_message":            "Please see attached invoice.",
			"lines": []map[string]any{{
				"id":                   "line-1",
				"recurring_invoice_id": id,
				"line_number":          1,
				"description":          "Consulting",
				"quantity":             "2.00",
				"unit":                 "hour",
				"unit_price":           "100.00",
				"discount_percent":     "10.00",
				"vat_rate":             "22.00",
				"account_id":           "acc-1",
				"product_id":           "prod-1",
			}},
		}
	}
	generationPayload := map[string]any{
		"recurring_invoice_id":     "rec-1",
		"generated_invoice_id":     "inv-1",
		"generated_invoice_number": "INV-00001",
		"email_sent":               true,
		"email_status":             "SENT",
		"email_log_id":             "email-1",
		"email_error":              "",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{recurringPayload("rec-1", "Monthly retainer", true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices":
			var req recurring.CreateRecurringInvoiceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Monthly retainer", req.Name)
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "SALES", req.InvoiceType)
			assert.Equal(t, "EUR", req.Currency)
			assert.Equal(t, recurring.FrequencyMonthly, req.Frequency)
			assert.Equal(t, "2026-03-15", req.StartDate.Format("2006-01-02"))
			require.NotNil(t, req.EndDate)
			assert.Equal(t, "2026-12-31", req.EndDate.Format("2006-01-02"))
			assert.Equal(t, 21, req.PaymentTermsDays)
			assert.True(t, req.SendEmailOnGeneration)
			assert.Equal(t, "billing@example.com", req.RecipientEmailOverride)
			require.NotNil(t, req.AttachPDFToEmail)
			assert.True(t, *req.AttachPDFToEmail)
			require.Len(t, req.Lines, 1)
			assert.Equal(t, "Consulting", req.Lines[0].Description)
			assert.True(t, req.Lines[0].Quantity.Equal(decimal.RequireFromString("2.00")))
			assert.True(t, req.Lines[0].DiscountPercent.Equal(decimal.RequireFromString("10.00")))
			require.NotNil(t, req.Lines[0].AccountID)
			require.NotNil(t, req.Lines[0].ProductID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(recurringPayload("rec-1", "Monthly retainer", true))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/from-invoice/inv-template":
			var req recurring.CreateFromInvoiceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "From invoice", req.Name)
			assert.Equal(t, recurring.FrequencyBiweekly, req.Frequency)
			assert.Equal(t, "2026-04-01", req.StartDate.Format("2006-01-02"))
			assert.Equal(t, 14, req.PaymentTermsDays)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(recurringPayload("rec-2", "From invoice", true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/rec-1":
			_ = json.NewEncoder(w).Encode(recurringPayload("rec-1", "Monthly retainer", true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/rec-1":
			var req recurring.UpdateRecurringInvoiceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.NotNil(t, req.Name)
			assert.Equal(t, "Quarterly retainer", *req.Name)
			require.NotNil(t, req.Frequency)
			assert.Equal(t, recurring.FrequencyQuarterly, *req.Frequency)
			require.NotNil(t, req.PaymentTermsDays)
			assert.Equal(t, 30, *req.PaymentTermsDays)
			require.NotNil(t, req.SendEmailOnGeneration)
			assert.False(t, *req.SendEmailOnGeneration)
			require.NotNil(t, req.AttachPDFToEmail)
			assert.False(t, *req.AttachPDFToEmail)
			require.Len(t, req.Lines, 1)
			assert.Equal(t, "Updated consulting", req.Lines[0].Description)
			payload := recurringPayload("rec-1", "Quarterly retainer", true)
			payload["frequency"] = "QUARTERLY"
			payload["payment_terms_days"] = 30
			payload["send_email_on_generation"] = false
			payload["attach_pdf_to_email"] = false
			payload["lines"] = []map[string]any{{
				"id":                   "line-2",
				"recurring_invoice_id": "rec-1",
				"line_number":          1,
				"description":          "Updated consulting",
				"quantity":             "3.00",
				"unit":                 "hour",
				"unit_price":           "100.00",
				"vat_rate":             "22.00",
			}}
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/rec-1/pause":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/rec-1/resume":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/rec-1/generate":
			_ = json.NewEncoder(w).Encode(generationPayload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/generate-due":
			_ = json.NewEncoder(w).Encode([]map[string]any{generationPayload})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/recurring-invoices/rec-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"recurring-invoices", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Monthly retainer"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"recurring-invoices", "create",
		"--name", "Monthly retainer",
		"--contact-id", "contact-1",
		"--type", "sales",
		"--currency", "eur",
		"--frequency", "monthly",
		"--start-date", "2026-03-15",
		"--end-date", "2026-12-31",
		"--payment-terms-days", "21",
		"--reference", "RET-1",
		"--notes", "Monthly services",
		"--send-email",
		"--recipient-email", "billing@example.com",
		"--attach-pdf",
		"--email-subject", "Monthly invoice",
		"--email-message", "Please see attached invoice.",
		"--line", "description=Consulting,quantity=2,unit=hour,unit_price=100.00,discount_percent=10.00,vat_rate=22.00,account_id=acc-1,product_id=prod-1",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created recurring invoice Monthly retainer (rec-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"recurring-invoices", "from-invoice",
		"--invoice-id", "inv-template",
		"--name", "From invoice",
		"--frequency", "biweekly",
		"--start-date", "2026-04-01",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created recurring invoice From invoice (rec-2) from invoice inv-template")

	stdout.Reset()
	err = app.run(context.Background(), []string{"recurring-invoices", "get", "--id", "rec-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Recurring invoice Monthly retainer")
	assert.Contains(t, stdout.String(), "Consulting")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"recurring-invoices", "update",
		"--id", "rec-1",
		"--name", "Quarterly retainer",
		"--frequency", "quarterly",
		"--payment-terms-days", "30",
		"--send-email", "false",
		"--attach-pdf", "false",
		"--line", "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Recurring invoice Quarterly retainer")
	assert.Contains(t, stdout.String(), "Attach PDF: false")

	stdout.Reset()
	err = app.run(context.Background(), []string{"recurring-invoices", "pause", "--id", "rec-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Paused recurring invoice rec-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"recurring-invoices", "resume", "--id", "rec-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "resumed"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"recurring-invoices", "generate", "--id", "rec-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Generated invoice INV-00001 (inv-1) from recurring invoice rec-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"recurring-invoices", "generate-due"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "INV-00001")

	stdout.Reset()
	err = app.run(context.Background(), []string{"recurring-invoices", "delete", "--id", "rec-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted recurring invoice rec-1")
}

func TestCLIAssetCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	categoryPayload := map[string]any{
		"id":                                  "cat-1",
		"tenant_id":                           "tenant-1",
		"name":                                "Equipment",
		"description":                         "Office equipment",
		"depreciation_method":                 "STRAIGHT_LINE",
		"default_useful_life_months":          60,
		"default_residual_value_percent":      "10.00",
		"asset_account_id":                    "acc-asset",
		"depreciation_expense_account_id":     "acc-expense",
		"accumulated_depreciation_account_id": "acc-accum",
		"created_at":                          "2026-03-15T12:00:00Z",
		"updated_at":                          "2026-03-15T12:00:00Z",
	}
	assetPayload := func(status string) map[string]any {
		return map[string]any{
			"id":                                  "asset-1",
			"tenant_id":                           "tenant-1",
			"asset_number":                        "FA-00001",
			"name":                                "Laptop",
			"description":                         "Developer laptop",
			"category_id":                         "cat-1",
			"status":                              status,
			"purchase_date":                       "2026-03-15T00:00:00Z",
			"purchase_cost":                       "1200.00",
			"supplier_id":                         "supplier-1",
			"serial_number":                       "SN-1",
			"location":                            "Tallinn",
			"depreciation_method":                 "STRAIGHT_LINE",
			"useful_life_months":                  36,
			"residual_value":                      "100.00",
			"depreciation_start_date":             "2026-04-01T00:00:00Z",
			"accumulated_depreciation":            "50.00",
			"book_value":                          "1150.00",
			"last_depreciation_date":              "2026-04-30T00:00:00Z",
			"asset_account_id":                    "acc-asset",
			"depreciation_expense_account_id":     "acc-expense",
			"accumulated_depreciation_account_id": "acc-accum",
			"created_at":                          "2026-03-15T12:00:00Z",
			"created_by":                          "user-1",
			"updated_at":                          "2026-03-15T12:00:00Z",
		}
	}
	depreciationPayload := map[string]any{
		"id":                  "dep-1",
		"tenant_id":           "tenant-1",
		"asset_id":            "asset-1",
		"depreciation_date":   "2026-04-30T00:00:00Z",
		"period_start":        "2026-04-01T00:00:00Z",
		"period_end":          "2026-04-30T00:00:00Z",
		"depreciation_amount": "25.00",
		"accumulated_total":   "75.00",
		"book_value_after":    "1125.00",
		"created_at":          "2026-04-30T12:00:00Z",
		"created_by":          "user-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories":
			_ = json.NewEncoder(w).Encode([]map[string]any{categoryPayload})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories":
			var req assets.CreateCategoryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Equipment", req.Name)
			assert.Equal(t, assets.DepreciationStraightLine, req.DepreciationMethod)
			assert.Equal(t, 60, req.DefaultUsefulLifeMonths)
			assert.True(t, req.DefaultResidualValuePercent.Equal(decimal.RequireFromString("10.00")))
			require.NotNil(t, req.AssetAccountID)
			assert.Equal(t, "acc-asset", *req.AssetAccountID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(categoryPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories/cat-1":
			_ = json.NewEncoder(w).Encode(categoryPayload)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories/cat-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/assets":
			require.Equal(t, "ACTIVE", r.URL.Query().Get("status"))
			require.Equal(t, "cat-1", r.URL.Query().Get("category_id"))
			require.Equal(t, "Laptop", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{assetPayload("ACTIVE")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets":
			var req assets.CreateAssetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Laptop", req.Name)
			assert.Equal(t, "2026-03-15", req.PurchaseDate.Format("2006-01-02"))
			assert.True(t, req.PurchaseCost.Equal(decimal.RequireFromString("1200.00")))
			assert.Equal(t, assets.DepreciationStraightLine, req.DepreciationMethod)
			assert.Equal(t, 36, req.UsefulLifeMonths)
			assert.True(t, req.ResidualValue.Equal(decimal.RequireFromString("100.00")))
			require.NotNil(t, req.DepreciationStartDate)
			assert.Equal(t, "2026-04-01", req.DepreciationStartDate.Format("2006-01-02"))
			require.NotNil(t, req.CategoryID)
			require.NotNil(t, req.SupplierID)
			assert.Equal(t, "cat-1", *req.CategoryID)
			assert.Equal(t, "supplier-1", *req.SupplierID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(assetPayload("DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1":
			_ = json.NewEncoder(w).Encode(assetPayload("ACTIVE"))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1":
			var req assets.UpdateAssetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Updated laptop", req.Name)
			assert.Equal(t, "Tartu", req.Location)
			assert.Equal(t, 48, req.UsefulLifeMonths)
			assert.True(t, req.ResidualValue.Equal(decimal.RequireFromString("150.00")))
			_ = json.NewEncoder(w).Encode(assetPayload("ACTIVE"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/activate":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "active"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/dispose":
			var req assets.DisposeAssetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2026-05-01", req.DisposalDate.Format("2006-01-02"))
			assert.Equal(t, assets.DisposalSold, req.DisposalMethod)
			assert.True(t, req.DisposalProceeds.Equal(decimal.RequireFromString("900.00")))
			assert.Equal(t, "Sold to employee", req.DisposalNotes)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "disposed"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/depreciation":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(depreciationPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/depreciation":
			_ = json.NewEncoder(w).Encode([]map[string]any{depreciationPayload})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"assets", "categories", "list", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Equipment"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "categories", "create",
		"--name", "Equipment",
		"--description", "Office equipment",
		"--depreciation-method", "straight_line",
		"--useful-life-months", "60",
		"--residual-percent", "10.00",
		"--asset-account-id", "acc-asset",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created asset category Equipment (cat-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "categories", "get", "--id", "cat-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Asset category Equipment")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "categories", "delete", "--id", "cat-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted asset category cat-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "list", "--status", "active", "--category-id", "cat-1", "--search", "Laptop", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"asset_number": "FA-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "create",
		"--name", "Laptop",
		"--description", "Developer laptop",
		"--category-id", "cat-1",
		"--purchase-date", "2026-03-15",
		"--purchase-cost", "1200.00",
		"--supplier-id", "supplier-1",
		"--serial-number", "SN-1",
		"--location", "Tallinn",
		"--depreciation-method", "straight_line",
		"--useful-life-months", "36",
		"--residual-value", "100.00",
		"--depreciation-start-date", "2026-04-01",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created asset FA-00001 (asset-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "get", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Asset FA-00001 Laptop")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "update",
		"--id", "asset-1",
		"--name", "Updated laptop",
		"--location", "Tartu",
		"--useful-life-months", "48",
		"--residual-value", "150.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Asset FA-00001")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "activate", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Activated asset asset-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "dispose",
		"--id", "asset-1",
		"--disposal-date", "2026-05-01",
		"--method", "sold",
		"--proceeds", "900.00",
		"--notes", "Sold to employee",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Disposed asset asset-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "depreciate", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Recorded depreciation 25")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "depreciation", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "dep-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "delete", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted asset asset-1")
}

func TestCLIInventoryCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	categoryPayload := map[string]any{
		"id":          "cat-1",
		"tenant_id":   "tenant-1",
		"name":        "Parts",
		"description": "Spare parts",
		"parent_id":   "parent-1",
		"created_at":  "2026-03-15T12:00:00Z",
		"updated_at":  "2026-03-15T12:00:00Z",
	}
	productPayload := func(name string, active bool) map[string]any {
		return map[string]any{
			"id":                   "prod-1",
			"tenant_id":            "tenant-1",
			"code":                 "PRD-001",
			"name":                 name,
			"description":          "Inventory item",
			"product_type":         "GOODS",
			"category_id":          "cat-1",
			"unit":                 "pcs",
			"purchase_price":       "10.50",
			"sales_price":          "15.00",
			"vat_rate":             "22.00",
			"min_stock_level":      "5.00",
			"current_stock":        "12.00",
			"reorder_point":        "7.00",
			"sale_account_id":      "acc-sale",
			"purchase_account_id":  "acc-purchase",
			"inventory_account_id": "acc-inventory",
			"track_inventory":      true,
			"is_active":            active,
			"barcode":              "123456",
			"supplier_id":          "supplier-1",
			"lead_time_days":       4,
			"created_at":           "2026-03-15T12:00:00Z",
			"updated_at":           "2026-03-15T12:00:00Z",
		}
	}
	warehousePayload := func(name string) map[string]any {
		return map[string]any{
			"id":         "wh-1",
			"tenant_id":  "tenant-1",
			"code":       "MAIN",
			"name":       name,
			"address":    "Tallinn",
			"is_default": true,
			"is_active":  true,
			"created_at": "2026-03-15T12:00:00Z",
			"updated_at": "2026-03-15T12:00:00Z",
		}
	}
	stockLevelPayload := map[string]any{
		"id":            "stock-1",
		"tenant_id":     "tenant-1",
		"product_id":    "prod-1",
		"warehouse_id":  "wh-1",
		"quantity":      "12.00",
		"reserved_qty":  "2.00",
		"available_qty": "10.00",
		"last_updated":  "2026-03-15T12:00:00Z",
	}
	movementPayload := map[string]any{
		"id":            "mov-1",
		"tenant_id":     "tenant-1",
		"product_id":    "prod-1",
		"warehouse_id":  "wh-1",
		"movement_type": "ADJUSTMENT",
		"quantity":      "2.00",
		"unit_cost":     "10.50",
		"total_cost":    "21.00",
		"reference":     "ADJ-1",
		"notes":         "Cycle count",
		"movement_date": "2026-03-15T00:00:00Z",
		"created_at":    "2026-03-15T12:00:00Z",
		"created_by":    "user-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/product-categories":
			_ = json.NewEncoder(w).Encode([]map[string]any{categoryPayload})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/product-categories":
			var req inventory.CreateCategoryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Parts", req.Name)
			assert.Equal(t, "Spare parts", req.Description)
			assert.Equal(t, "parent-1", req.ParentID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(categoryPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/product-categories/cat-1":
			_ = json.NewEncoder(w).Encode(categoryPayload)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/product-categories/cat-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/products":
			require.Equal(t, "GOODS", r.URL.Query().Get("product_type"))
			require.Equal(t, "ACTIVE", r.URL.Query().Get("status"))
			require.Equal(t, "cat-1", r.URL.Query().Get("category_id"))
			require.Equal(t, "Widget", r.URL.Query().Get("search"))
			require.Equal(t, "true", r.URL.Query().Get("low_stock"))
			_ = json.NewEncoder(w).Encode([]map[string]any{productPayload("Widget", true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/products":
			var req inventory.CreateProductRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "PRD-001", req.Code)
			assert.Equal(t, "Widget", req.Name)
			assert.Equal(t, "GOODS", req.ProductType)
			assert.Equal(t, "cat-1", req.CategoryID)
			assert.Equal(t, "pcs", req.Unit)
			assert.Equal(t, "10.5", req.PurchasePrice)
			assert.Equal(t, "15", req.SalesPrice)
			assert.Equal(t, "22", req.VATRate)
			assert.Equal(t, "5", req.MinStockLevel)
			assert.Equal(t, "7", req.ReorderPoint)
			assert.True(t, req.TrackInventory)
			assert.Equal(t, "123456", req.Barcode)
			assert.Equal(t, "supplier-1", req.SupplierID)
			assert.Equal(t, 4, req.LeadTimeDays)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(productPayload("Widget", true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/products/prod-1":
			_ = json.NewEncoder(w).Encode(productPayload("Widget", true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/products/prod-1":
			var req inventory.UpdateProductRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Updated widget", req.Name)
			assert.Equal(t, "16", req.SalesPrice)
			assert.False(t, req.IsActive)
			assert.True(t, req.TrackInventory)
			_ = json.NewEncoder(w).Encode(productPayload("Updated widget", false))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/products/prod-1/stock-levels":
			_ = json.NewEncoder(w).Encode([]map[string]any{stockLevelPayload})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/products/prod-1/movements":
			_ = json.NewEncoder(w).Encode([]map[string]any{movementPayload})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/products/prod-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/warehouses":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{warehousePayload("Main warehouse")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/warehouses":
			var req inventory.CreateWarehouseRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "MAIN", req.Code)
			assert.Equal(t, "Main warehouse", req.Name)
			assert.Equal(t, "Tallinn", req.Address)
			assert.True(t, req.IsDefault)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(warehousePayload("Main warehouse"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/warehouses/wh-1":
			_ = json.NewEncoder(w).Encode(warehousePayload("Main warehouse"))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/warehouses/wh-1":
			var req inventory.UpdateWarehouseRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Updated warehouse", req.Name)
			assert.Equal(t, "Tartu", req.Address)
			assert.False(t, req.IsDefault)
			assert.True(t, req.IsActive)
			_ = json.NewEncoder(w).Encode(warehousePayload("Updated warehouse"))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/warehouses/wh-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/inventory/adjust":
			var req inventory.AdjustStockRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "prod-1", req.ProductID)
			assert.Equal(t, "wh-1", req.WarehouseID)
			assert.Equal(t, "-2", req.Quantity)
			assert.Equal(t, "10.5", req.UnitCost)
			assert.Equal(t, "Cycle count", req.Reason)
			_ = json.NewEncoder(w).Encode(movementPayload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/inventory/transfer":
			var req inventory.TransferStockRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "prod-1", req.ProductID)
			assert.Equal(t, "wh-1", req.FromWarehouseID)
			assert.Equal(t, "wh-2", req.ToWarehouseID)
			assert.Equal(t, "3", req.Quantity)
			assert.Equal(t, "Move to branch", req.Notes)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "transferred"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"inventory", "categories", "list", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Parts"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "categories", "create", "--name", "Parts", "--description", "Spare parts", "--parent-id", "parent-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created product category Parts (cat-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "categories", "get", "--id", "cat-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Product category Parts")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "categories", "delete", "--id", "cat-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted product category cat-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"inventory", "products", "list",
		"--type", "goods",
		"--status", "active",
		"--category-id", "cat-1",
		"--search", "Widget",
		"--low-stock",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"code": "PRD-001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"inventory", "products", "create",
		"--code", "PRD-001",
		"--name", "Widget",
		"--description", "Inventory item",
		"--type", "goods",
		"--category-id", "cat-1",
		"--unit", "pcs",
		"--purchase-price", "10.50",
		"--sales-price", "15.00",
		"--vat-rate", "22.00",
		"--min-stock-level", "5.00",
		"--reorder-point", "7.00",
		"--sale-account-id", "acc-sale",
		"--purchase-account-id", "acc-purchase",
		"--inventory-account-id", "acc-inventory",
		"--barcode", "123456",
		"--supplier-id", "supplier-1",
		"--lead-time-days", "4",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created product PRD-001 Widget (prod-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "products", "get", "--id", "prod-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Product PRD-001 Widget")
	assert.Contains(t, stdout.String(), "Current stock: 12")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"inventory", "products", "update",
		"--id", "prod-1",
		"--name", "Updated widget",
		"--purchase-price", "10.50",
		"--sales-price", "16.00",
		"--active=false",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Product PRD-001 Updated widget")
	assert.Contains(t, stdout.String(), "Active: false")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "products", "stock-levels", "--id", "prod-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "AVAILABLE")
	assert.Contains(t, stdout.String(), "10")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "products", "movements", "--id", "prod-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "ADJUSTMENT")
	assert.Contains(t, stdout.String(), "Cycle count")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "products", "delete", "--id", "prod-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted product prod-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "warehouses", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"code": "MAIN"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "warehouses", "create", "--code", "MAIN", "--name", "Main warehouse", "--address", "Tallinn", "--default"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created warehouse MAIN Main warehouse (wh-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "warehouses", "get", "--id", "wh-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Warehouse MAIN Main warehouse")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "warehouses", "update", "--id", "wh-1", "--name", "Updated warehouse", "--address", "Tartu"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Warehouse MAIN Updated warehouse")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "warehouses", "delete", "--id", "wh-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted warehouse wh-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "adjust", "--product-id", "prod-1", "--warehouse-id", "wh-1", "--quantity", "-2.00", "--unit-cost", "10.50", "--reason", "Cycle count"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Adjusted stock for product prod-1 by -2 in warehouse wh-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"inventory", "transfer", "--product-id", "prod-1", "--from-warehouse-id", "wh-1", "--to-warehouse-id", "wh-2", "--quantity", "3.00", "--notes", "Move to branch", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "transferred"`)
}

func TestCLICostCenterCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	costCenterPayload := map[string]any{
		"id":            "cc-1",
		"tenant_id":     "tenant-1",
		"code":          "CC001",
		"name":          "Sales",
		"description":   "Sales team",
		"is_active":     true,
		"budget_amount": "1000.00",
		"budget_period": "MONTHLY",
		"created_at":    "2026-03-15T12:00:00Z",
		"updated_at":    "2026-03-15T12:00:00Z",
	}
	reportPayload := map[string]any{
		"tenant_id":      "tenant-1",
		"period_start":   "2026-03-01T00:00:00Z",
		"period_end":     "2026-03-31T00:00:00Z",
		"generated_at":   "2026-03-31T12:00:00Z",
		"total_expenses": "250.00",
		"total_budget":   "1000.00",
		"cost_centers": []map[string]any{{
			"cost_center":            costCenterPayload,
			"total_expenses":         "250.00",
			"budget_amount":          "1000.00",
			"budget_used_percentage": "25.00",
			"is_over_budget":         false,
			"period_start":           "2026-03-01T00:00:00Z",
			"period_end":             "2026-03-31T00:00:00Z",
		}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{costCenterPayload})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers":
			var req accounting.CreateCostCenterRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "CC001", req.Code)
			assert.Equal(t, "Sales", req.Name)
			assert.True(t, req.IsActive)
			require.NotNil(t, req.BudgetAmount)
			assert.True(t, req.BudgetAmount.Equal(decimal.RequireFromString("1000.00")))
			assert.Equal(t, accounting.BudgetPeriodMonthly, req.BudgetPeriod)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(costCenterPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/cc-1":
			_ = json.NewEncoder(w).Encode(costCenterPayload)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/cc-1":
			var req accounting.UpdateCostCenterRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "CC002", req.Code)
			assert.Equal(t, "Sales updated", req.Name)
			require.NotNil(t, req.BudgetAmount)
			assert.True(t, req.BudgetAmount.Equal(decimal.RequireFromString("1200.00")))
			payload := map[string]any{}
			for key, value := range costCenterPayload {
				payload[key] = value
			}
			payload["code"] = "CC002"
			payload["name"] = "Sales updated"
			payload["budget_amount"] = "1200.00"
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/report":
			require.Equal(t, "2026-03-01", r.URL.Query().Get("start_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("end_date"))
			_ = json.NewEncoder(w).Encode(reportPayload)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/cc-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"cost-centers", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"code": "CC001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"cost-centers", "create",
		"--code", "CC001",
		"--name", "Sales",
		"--description", "Sales team",
		"--budget-amount", "1000.00",
		"--budget-period", "monthly",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created cost center CC001 Sales (cc-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"cost-centers", "get", "--id", "cc-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Cost center CC001 Sales")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"cost-centers", "update",
		"--id", "cc-1",
		"--code", "CC002",
		"--name", "Sales updated",
		"--budget-amount", "1200.00",
		"--budget-period", "monthly",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Cost center CC002 Sales updated")

	stdout.Reset()
	err = app.run(context.Background(), []string{"cost-centers", "report", "--start", "2026-03-01", "--end", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Total expenses: 250")
	assert.Contains(t, stdout.String(), "Sales")

	stdout.Reset()
	err = app.run(context.Background(), []string{"cost-centers", "delete", "--id", "cc-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted cost center cc-1")
}

func TestCLIJournalEntryCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	journalPayload := func(id, number string, status accounting.JournalEntryStatus) map[string]any {
		return map[string]any{
			"id":           id,
			"tenant_id":    "tenant-1",
			"entry_number": number,
			"entry_date":   "2026-03-31T00:00:00Z",
			"description":  "Manual accrual",
			"reference":    "ACC-1",
			"source_type":  "MANUAL",
			"status":       status,
			"created_at":   "2026-03-31T12:00:00Z",
			"created_by":   "user-1",
			"lines": []map[string]any{
				{
					"id":               "line-1",
					"tenant_id":        "tenant-1",
					"journal_entry_id": id,
					"account_id":       "acc-1",
					"description":      "Expense",
					"debit_amount":     "100.00",
					"credit_amount":    "0.00",
					"currency":         "EUR",
					"exchange_rate":    "1.00",
					"base_debit":       "100.00",
					"base_credit":      "0.00",
				},
				{
					"id":               "line-2",
					"tenant_id":        "tenant-1",
					"journal_entry_id": id,
					"account_id":       "acc-2",
					"description":      "Accrual",
					"debit_amount":     "0.00",
					"credit_amount":    "100.00",
					"currency":         "EUR",
					"exchange_rate":    "1.00",
					"base_debit":       "0.00",
					"base_credit":      "100.00",
				},
			},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries":
			require.Equal(t, "25", r.URL.Query().Get("limit"))
			_ = json.NewEncoder(w).Encode([]map[string]any{journalPayload("je-1", "JE-2026-001", accounting.StatusDraft)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries":
			var req accounting.CreateJournalEntryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2026-03-31", req.EntryDate.Format("2006-01-02"))
			assert.Equal(t, "Manual accrual", req.Description)
			assert.Equal(t, "ACC-1", req.Reference)
			require.Len(t, req.Lines, 2)
			assert.Equal(t, "acc-1", req.Lines[0].AccountID)
			assert.True(t, req.Lines[0].DebitAmount.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, req.Lines[1].CreditAmount.Equal(decimal.RequireFromString("100.00")))
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(journalPayload("je-1", "JE-2026-001", accounting.StatusDraft))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/je-1":
			_ = json.NewEncoder(w).Encode(journalPayload("je-1", "JE-2026-001", accounting.StatusDraft))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/je-1/post":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "posted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/je-1/void":
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Duplicate entry", req["reason"])
			payload := journalPayload("je-rev-1", "JE-2026-002", accounting.StatusPosted)
			payload["void_reason"] = "Duplicate entry"
			_ = json.NewEncoder(w).Encode(payload)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"journal", "list", "--limit", "25", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"entry_number": "JE-2026-001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"journal", "create",
		"--entry-date", "2026-03-31",
		"--description", "Manual accrual",
		"--reference", "ACC-1",
		"--source-type", "MANUAL",
		"--line", "account_id=acc-1,description=Expense,debit=100.00",
		"--line", "account_id=acc-2,description=Accrual,credit=100.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created journal entry JE-2026-001 (je-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"journal", "get", "--id", "je-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Journal entry JE-2026-001")
	assert.Contains(t, stdout.String(), "Balanced: true")

	stdout.Reset()
	err = app.run(context.Background(), []string{"journal", "post", "--id", "je-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Posted journal entry je-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"journal", "void", "--id", "je-1", "--reason", "Duplicate entry"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Voided journal entry je-1 with reversal JE-2026-002")
}

func TestCLIContactsInvoicesAndJournalCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	contactsFile := writeTempCSV(t, "contacts.csv", "name,email\nAcme,hello@example.com\n")
	invoicesFile := writeTempCSV(t, "invoices.csv", "invoice_number,contact_name,total\nINV-1,Acme,100\n")
	openingBalancesFile := writeTempCSV(t, "opening-balances.csv", "account_code,debit,credit\n1000,500,0\n")
	contactPayload := func(name string, active bool) map[string]any {
		return map[string]any{
			"id":                 "contact-1",
			"tenant_id":          "tenant-1",
			"code":               "CUST-1",
			"name":               name,
			"contact_type":       "CUSTOMER",
			"reg_code":           "12345678",
			"vat_number":         "EE123456789",
			"email":              "hello@example.com",
			"phone":              "+372 555 1234",
			"address_line1":      "123 Main St",
			"city":               "Tallinn",
			"postal_code":        "10111",
			"country_code":       "EE",
			"payment_terms_days": 14,
			"credit_limit":       "1500.00",
			"is_active":          active,
			"notes":              "Key customer",
			"created_at":         "2026-03-12T00:00:00Z",
			"updated_at":         "2026-03-12T00:00:00Z",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/contacts":
			require.Equal(t, "CUSTOMER", r.URL.Query().Get("type"))
			require.Equal(t, "acme", r.URL.Query().Get("search"))
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{contactPayload("Acme", true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/contacts":
			var req contacts.CreateContactRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Acme", req.Name)
			assert.True(t, req.CreditLimit.Equal(decimal.RequireFromString("1500")))
			_ = json.NewEncoder(w).Encode(contactPayload(req.Name, true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/contact-1":
			_ = json.NewEncoder(w).Encode(contactPayload("Acme", true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/contact-1":
			var req contacts.UpdateContactRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.NotNil(t, req.Name)
			assert.Equal(t, "Acme Updated", *req.Name)
			require.NotNil(t, req.PaymentTermsDays)
			assert.Equal(t, 30, *req.PaymentTermsDays)
			require.NotNil(t, req.CreditLimit)
			assert.True(t, req.CreditLimit.Equal(decimal.RequireFromString("2500.00")))
			require.NotNil(t, req.IsActive)
			assert.False(t, *req.IsActive)
			_ = json.NewEncoder(w).Encode(contactPayload("Acme Updated", false))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/contact-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/import":
			var req contacts.ImportContactsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contacts.csv", req.FileName)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":   1,
				"contacts_created": 1,
				"rows_skipped":     0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/import":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":   1,
				"invoices_created": 1,
				"lines_imported":   1,
				"rows_skipped":     0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/import-opening-balances":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"journal_entry": map[string]any{
					"id":           "je-1",
					"entry_number": "JE-2026-001",
				},
				"lines_imported": 1,
				"total_debit":    "500.00",
				"total_credit":   "500.00",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"contacts",
		"list",
		"--type", "customer",
		"--search", "acme",
		"--active-only",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Acme"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"contacts",
		"create",
		"--name", "Acme",
		"--email", "hello@example.com",
		"--credit-limit", "1500",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created contact Acme (contact-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"contacts", "get", "--id", "contact-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Contact Acme")
	assert.Contains(t, stdout.String(), "Credit limit: 1500")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"contacts",
		"update",
		"--id", "contact-1",
		"--name", "Acme Updated",
		"--payment-terms-days", "30",
		"--credit-limit", "2500.00",
		"--active", "false",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Contact Acme Updated")
	assert.Contains(t, stdout.String(), "Active: false")

	stdout.Reset()
	err = app.run(context.Background(), []string{"contacts", "delete", "--id", "contact-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted contact contact-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"contacts", "import", "--file", contactsFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 contacts, skipped 0 rows")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "import", "--file", invoicesFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 invoices, imported 1 lines, skipped 0 rows")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"journal",
		"import-opening-balances",
		"--file", openingBalancesFile,
		"--entry-date", "2026-01-01",
		"--description", "Opening balances",
		"--reference", "OB-2026",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created posted journal entry JE-2026-001")
	assert.Contains(t, stdout.String(), "debit 500")
}

func TestCLIPaymentCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	paymentPayload := func(id, number string) map[string]any {
		return map[string]any{
			"id":             id,
			"tenant_id":      "tenant-1",
			"payment_number": number,
			"payment_type":   "RECEIVED",
			"contact_id":     "contact-1",
			"payment_date":   "2026-03-15T00:00:00Z",
			"amount":         "100.00",
			"currency":       "EUR",
			"exchange_rate":  "1.00",
			"base_amount":    "100.00",
			"payment_method": "BANK_TRANSFER",
			"bank_account":   "EE471000001020145685",
			"reference":      "REF-1",
			"notes":          "March receipt",
			"created_at":     "2026-03-15T12:00:00Z",
			"created_by":     "user-1",
			"allocations": []map[string]any{{
				"id":         "alloc-1",
				"tenant_id":  "tenant-1",
				"payment_id": id,
				"invoice_id": "inv-1",
				"amount":     "60.00",
				"created_at": "2026-03-15T12:05:00Z",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payments":
			require.Equal(t, "RECEIVED", r.URL.Query().Get("type"))
			require.Equal(t, "BANK_TRANSFER", r.URL.Query().Get("method"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			_ = json.NewEncoder(w).Encode([]map[string]any{paymentPayload("pay-1", "PMT-00001")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payments":
			var req payments.CreatePaymentRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, payments.PaymentTypeReceived, req.PaymentType)
			require.NotNil(t, req.ContactID)
			assert.Equal(t, "contact-1", *req.ContactID)
			assert.Equal(t, "2026-03-15", req.PaymentDate.Format("2006-01-02"))
			assert.True(t, req.Amount.Equal(decimal.RequireFromString("100.00")))
			assert.Equal(t, "EUR", req.Currency)
			assert.Equal(t, "BANK_TRANSFER", req.PaymentMethod)
			assert.Equal(t, "REF-1", req.Reference)
			require.Len(t, req.Allocations, 1)
			assert.Equal(t, "inv-1", req.Allocations[0].InvoiceID)
			assert.True(t, req.Allocations[0].Amount.Equal(decimal.RequireFromString("60.00")))
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(paymentPayload("pay-1", "PMT-00001"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payments/pay-1":
			_ = json.NewEncoder(w).Encode(paymentPayload("pay-1", "PMT-00001"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payments/pay-1/allocate":
			var req payments.AllocationRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "inv-2", req.InvoiceID)
			assert.True(t, req.Amount.Equal(decimal.RequireFromString("40.00")))
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "allocated"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payments/unallocated":
			require.Equal(t, "RECEIVED", r.URL.Query().Get("type"))
			payload := paymentPayload("pay-2", "PMT-00002")
			payload["allocations"] = []map[string]any{}
			_ = json.NewEncoder(w).Encode([]map[string]any{payload})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"payments", "list",
		"--type", "received",
		"--method", "BANK_TRANSFER",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"payment_number": "PMT-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"payments", "create",
		"--type", "received",
		"--amount", "100.00",
		"--date", "2026-03-15",
		"--currency", "eur",
		"--method", "BANK_TRANSFER",
		"--contact-id", "contact-1",
		"--bank-account", "EE471000001020145685",
		"--reference", "REF-1",
		"--notes", "March receipt",
		"--allocate", "inv-1:60.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created payment PMT-00001 (pay-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payments", "get", "--id", "pay-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Payment PMT-00001")
	assert.Contains(t, stdout.String(), "inv-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payments", "allocate", "--id", "pay-1", "--invoice-id", "inv-2", "--amount", "40.00"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Allocated 40 to invoice inv-2 for payment pay-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payments", "unallocated", "--type", "received"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "PMT-00002")
}

func TestCLIReminderCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	now := "2026-03-15T12:00:00Z"
	overduePayload := map[string]any{
		"total_overdue":        "500.00",
		"invoice_count":        1,
		"contact_count":        1,
		"average_days_overdue": 14,
		"generated_at":         now,
		"invoices": []map[string]any{{
			"id":                 "inv-1",
			"invoice_number":     "INV-00001",
			"contact_id":         "contact-1",
			"contact_name":       "Acme",
			"contact_email":      "billing@example.com",
			"issue_date":         "2026-02-01",
			"due_date":           "2026-03-01",
			"total":              "600.00",
			"amount_paid":        "100.00",
			"outstanding_amount": "500.00",
			"currency":           "EUR",
			"days_overdue":       14,
			"reminder_count":     1,
			"last_reminder_at":   now,
		}},
	}
	reminderResult := map[string]any{
		"invoice_id":     "inv-1",
		"invoice_number": "INV-00001",
		"success":        true,
		"message":        "Reminder #2 sent successfully",
		"reminder_id":    "rem-2",
	}
	bulkResult := map[string]any{
		"total_requested": 2,
		"successful":      2,
		"failed":          0,
		"results": []map[string]any{
			reminderResult,
			{
				"invoice_id":     "inv-2",
				"invoice_number": "INV-00002",
				"success":        true,
				"message":        "Reminder #1 sent successfully",
				"reminder_id":    "rem-3",
			},
		},
	}
	historyPayload := []map[string]any{{
		"id":              "rem-1",
		"tenant_id":       "tenant-1",
		"invoice_id":      "inv-1",
		"invoice_number":  "INV-00001",
		"contact_id":      "contact-1",
		"contact_name":    "Acme",
		"contact_email":   "billing@example.com",
		"reminder_number": 1,
		"status":          "SENT",
		"sent_at":         now,
		"created_at":      now,
		"updated_at":      now,
	}}
	rulePayload := func(name string, active bool) map[string]any {
		return map[string]any{
			"id":                  "rule-1",
			"tenant_id":           "tenant-1",
			"name":                name,
			"trigger_type":        "AFTER_DUE",
			"days_offset":         7,
			"email_template_type": "OVERDUE_REMINDER",
			"is_active":           active,
			"created_at":          now,
			"updated_at":          now,
		}
	}
	triggerPayload := []map[string]any{{
		"tenant_id":      "tenant-1",
		"rule_id":        "rule-1",
		"rule_name":      "Seven days overdue",
		"invoices_found": 2,
		"reminders_sent": 1,
		"skipped":        1,
		"failed":         0,
		"run_at":         now,
	}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/overdue":
			_ = json.NewEncoder(w).Encode(overduePayload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/reminders":
			var req invoicing.SendReminderRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "inv-1", req.InvoiceID)
			assert.Equal(t, "Please pay", req.Message)
			_ = json.NewEncoder(w).Encode(reminderResult)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/reminders/bulk":
			var req invoicing.SendBulkRemindersRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, []string{"inv-1", "inv-2"}, req.InvoiceIDs)
			assert.Equal(t, "Please pay", req.Message)
			_ = json.NewEncoder(w).Encode(bulkResult)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/reminders":
			_ = json.NewEncoder(w).Encode(historyPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reminder-rules":
			_ = json.NewEncoder(w).Encode([]map[string]any{rulePayload("Seven days overdue", true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/reminder-rules":
			var req invoicing.CreateReminderRuleRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Seven days overdue", req.Name)
			assert.Equal(t, invoicing.TriggerAfterDue, req.TriggerType)
			assert.Equal(t, 7, req.DaysOffset)
			assert.Equal(t, "OVERDUE_REMINDER", req.EmailTemplateType)
			assert.True(t, req.IsActive)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(rulePayload("Seven days overdue", true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reminder-rules/rule-1":
			_ = json.NewEncoder(w).Encode(rulePayload("Seven days overdue", true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/reminder-rules/rule-1":
			var req invoicing.UpdateReminderRuleRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.NotNil(t, req.Name)
			assert.Equal(t, "Updated reminder", *req.Name)
			require.NotNil(t, req.EmailTemplateType)
			assert.Equal(t, "CUSTOM_TEMPLATE", *req.EmailTemplateType)
			require.NotNil(t, req.IsActive)
			assert.False(t, *req.IsActive)
			_ = json.NewEncoder(w).Encode(rulePayload("Updated reminder", false))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/reminder-rules/rule-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/reminder-rules/trigger":
			_ = json.NewEncoder(w).Encode(triggerPayload)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"reminders", "overdue", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"invoice_number": "INV-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "send", "--invoice-id", "inv-1", "--message", "Please pay"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Reminder ID: rem-2")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "send-bulk", "--invoice-id", "inv-1", "--invoice-id", "inv-2", "--message", "Please pay"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Successful: 2")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "history", "--invoice-id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "rem-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "rules", "list", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Seven days overdue"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"reminders", "rules", "create",
		"--name", "Seven days overdue",
		"--trigger-type", "after_due",
		"--days-offset", "7",
		"--template-type", "OVERDUE_REMINDER",
		"--active", "true",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created reminder rule Seven days overdue (rule-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "rules", "get", "--id", "rule-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Reminder rule Seven days overdue")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "rules", "update", "--id", "rule-1", "--name", "Updated reminder", "--template-type", "CUSTOM_TEMPLATE", "--active", "false"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Active: false")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "rules", "delete", "--id", "rule-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted reminder rule rule-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reminders", "rules", "trigger"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Seven days overdue")
}

func TestCLIEmailCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	now := "2026-03-15T12:00:00Z"
	templatePayload := func(templateType, subject string, active bool) map[string]any {
		return map[string]any{
			"id":            "tmpl-1",
			"tenant_id":     "tenant-1",
			"template_type": templateType,
			"subject":       subject,
			"body_html":     "<p>Reminder</p>",
			"body_text":     "Reminder",
			"is_active":     active,
			"created_at":    now,
			"updated_at":    now,
		}
	}
	emailSentPayload := map[string]any{
		"success": true,
		"log_id":  "email-2",
		"message": "Email sent successfully",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/settings/smtp":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"smtp_host":       "smtp.example.com",
				"smtp_port":       587,
				"smtp_username":   "robot",
				"smtp_from_email": "billing@example.com",
				"smtp_from_name":  "Billing",
				"smtp_use_tls":    true,
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/settings/smtp":
			var req email.UpdateSMTPConfigRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "smtp2.example.com", req.Host)
			assert.Equal(t, 2525, req.Port)
			assert.Equal(t, "robot", req.Username)
			assert.Equal(t, "secret", req.Password)
			assert.Equal(t, "billing@example.com", req.FromEmail)
			assert.Equal(t, "Billing", req.FromName)
			assert.False(t, req.UseTLS)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/settings/smtp/test":
			var req email.TestSMTPRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "test@example.com", req.RecipientEmail)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Test email sent successfully"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/email-templates":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				templatePayload("INVOICE_SEND", "Invoice", true),
				templatePayload("OVERDUE_REMINDER", "Reminder", true),
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/email-templates/OVERDUE_REMINDER":
			var req email.UpdateTemplateRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Reminder", req.Subject)
			assert.Equal(t, "<p>Reminder</p>", req.BodyHTML)
			assert.Equal(t, "Reminder", req.BodyText)
			assert.False(t, req.IsActive)
			_ = json.NewEncoder(w).Encode(templatePayload("OVERDUE_REMINDER", "Reminder", false))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/email-log":
			require.Equal(t, "25", r.URL.Query().Get("limit"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":              "email-1",
				"tenant_id":       "tenant-1",
				"email_type":      "INVOICE_SEND",
				"recipient_email": "billing@example.com",
				"recipient_name":  "Acme",
				"subject":         "Invoice INV-00001",
				"status":          "SENT",
				"sent_at":         now,
				"related_id":      "inv-1",
				"created_at":      now,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/email":
			var req email.SendInvoiceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "billing@example.com", req.RecipientEmail)
			assert.Equal(t, "Acme", req.RecipientName)
			assert.Equal(t, "Invoice INV-00001", req.Subject)
			assert.Equal(t, "See attached", req.Message)
			assert.True(t, req.AttachPDF)
			_ = json.NewEncoder(w).Encode(emailSentPayload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payments/pay-1/email-receipt":
			var req email.SendPaymentReceiptRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "billing@example.com", req.RecipientEmail)
			assert.Equal(t, "Acme", req.RecipientName)
			assert.Equal(t, "Receipt", req.Subject)
			assert.Equal(t, "Thanks", req.Message)
			_ = json.NewEncoder(w).Encode(emailSentPayload)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"email", "smtp", "get", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"smtp_host": "smtp.example.com"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"email", "smtp", "update", "--host", "smtp2.example.com", "--port", "2525", "--username", "robot", "--password", "secret", "--from-email", "billing@example.com", "--from-name", "Billing", "--use-tls=false"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Updated SMTP configuration")

	stdout.Reset()
	err = app.run(context.Background(), []string{"email", "smtp", "test", "--recipient-email", "test@example.com"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Success: true")

	stdout.Reset()
	err = app.run(context.Background(), []string{"email", "templates", "list", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"template_type": "INVOICE_SEND"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"email", "templates", "update", "--type", "overdue_reminder", "--subject", "Reminder", "--body-html", "<p>Reminder</p>", "--body-text", "Reminder", "--active", "false"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Email template OVERDUE_REMINDER")
	assert.Contains(t, stdout.String(), "Active: false")

	stdout.Reset()
	err = app.run(context.Background(), []string{"email", "log", "--limit", "25"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "email-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"email", "invoice", "--invoice-id", "inv-1", "--recipient-email", "billing@example.com", "--recipient-name", "Acme", "--subject", "Invoice INV-00001", "--message", "See attached", "--attach-pdf"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Log ID: email-2")

	stdout.Reset()
	err = app.run(context.Background(), []string{"email", "payment-receipt", "--payment-id", "pay-1", "--recipient-email", "billing@example.com", "--recipient-name", "Acme", "--subject", "Receipt", "--message", "Thanks"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Email sent")
}

func TestCLIInterestCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	now := "2026-03-15T12:00:00Z"
	calculationPayload := map[string]any{
		"invoice_id":          "inv-1",
		"invoice_number":      "INV-00001",
		"due_date":            "2026-03-01T00:00:00Z",
		"days_overdue":        14,
		"outstanding_amount":  "500.00",
		"interest_rate":       "0.0005",
		"daily_interest":      "0.25",
		"total_interest":      "3.50",
		"total_with_interest": "503.50",
		"calculated_at":       now,
		"currency":            "EUR",
	}
	historyPayload := []map[string]any{{
		"id":                  "interest-1",
		"invoice_id":          "inv-1",
		"calculated_at":       now,
		"days_overdue":        14,
		"principal_amount":    "500.00",
		"interest_rate":       "0.0005",
		"interest_amount":     "3.50",
		"total_with_interest": "503.50",
		"created_at":          now,
	}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/settings/interest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rate":        0.0005,
				"annual_rate": 0.1825,
				"description": "0.050% daily (18.2% annually)",
				"is_enabled":  true,
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/settings/interest":
			var req invoicing.UpdateInterestSettingsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.InDelta(t, 0.0005, req.Rate, 0.0000001)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rate":        req.Rate,
				"annual_rate": req.Rate * 365,
				"description": "0.050% daily (18.2% annually)",
				"is_enabled":  true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/overdue-with-interest":
			_ = json.NewEncoder(w).Encode([]map[string]any{calculationPayload})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/interest":
			_ = json.NewEncoder(w).Encode(calculationPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/interest/history":
			_ = json.NewEncoder(w).Encode(historyPayload)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"interest", "settings", "get", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"is_enabled": true`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"interest", "settings", "update", "--annual-rate", "0.1825"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Daily rate: 0.000500")

	stdout.Reset()
	err = app.run(context.Background(), []string{"interest", "overdue", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"invoice_number": "INV-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"interest", "invoice", "--invoice-id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Interest for invoice INV-00001")
	assert.Contains(t, stdout.String(), "Total interest: 3.5")

	stdout.Reset()
	err = app.run(context.Background(), []string{"interest", "history", "--invoice-id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "interest-1")
}

func TestCLICloseCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	now := "2026-03-31T12:00:00Z"
	lockAfter := "2026-03-31"
	tenantPayload := map[string]any{
		"id":          "tenant-1",
		"name":        "Alpha",
		"slug":        "alpha",
		"schema_name": "tenant_alpha",
		"is_active":   true,
		"settings": map[string]any{
			"default_currency":        "EUR",
			"country_code":            "EE",
			"timezone":                "Europe/Tallinn",
			"date_format":             "DD.MM.YYYY",
			"decimal_sep":             ",",
			"thousands_sep":           " ",
			"fiscal_year_start_month": 1,
			"period_lock_date":        lockAfter,
		},
		"created_at": now,
		"updated_at": now,
	}
	closeEventPayload := func(action, note string) map[string]any {
		return map[string]any{
			"id":              "close-1",
			"tenant_id":       "tenant-1",
			"action":          action,
			"close_kind":      "month_end",
			"period_end_date": "2026-03-31",
			"lock_date_after": lockAfter,
			"note":            note,
			"performed_by":    "user-1",
			"created_at":      now,
		}
	}
	statusPayload := map[string]any{
		"period_end_date":               "2025-12-31",
		"fiscal_year_label":             "2025",
		"fiscal_year_start_date":        "2025-01-01",
		"fiscal_year_end_date":          "2025-12-31",
		"carry_forward_date":            "2026-01-01",
		"locked_through_date":           "2025-12-31",
		"is_fiscal_year_end":            true,
		"period_closed":                 true,
		"has_profit_and_loss_activity":  true,
		"carry_forward_needed":          true,
		"carry_forward_ready":           true,
		"has_retained_earnings_account": true,
		"retained_earnings_account":     map[string]any{"id": "acc-retained", "code": "2999", "name": "Retained earnings"},
		"net_income":                    "1200.00",
	}
	carryForwardPayload := map[string]any{
		"journal_entry": map[string]any{
			"id":           "je-1",
			"tenant_id":    "tenant-1",
			"entry_number": "JE-2026-001",
			"entry_date":   "2026-01-01T00:00:00Z",
			"description":  "Year-end carry-forward",
			"source_type":  "YEAR_END_CARRY_FORWARD",
			"status":       "POSTED",
			"created_at":   now,
			"created_by":   "user-1",
			"lines":        []map[string]any{},
		},
		"status": statusPayload,
	}
	reversalPayload := map[string]any{
		"reversal_journal_entry": map[string]any{
			"id":           "je-2",
			"tenant_id":    "tenant-1",
			"entry_number": "JE-2026-002",
			"entry_date":   "2026-01-01T00:00:00Z",
			"description":  "Reversal of year-end carry-forward",
			"source_type":  "YEAR_END_CARRY_FORWARD_REVERSAL",
			"status":       "POSTED",
			"created_at":   now,
			"created_by":   "user-1",
			"lines":        []map[string]any{},
		},
		"status": statusPayload,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/period-close-events":
			require.Equal(t, "10", r.URL.Query().Get("limit"))
			_ = json.NewEncoder(w).Encode([]map[string]any{closeEventPayload("close", "March close")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/period-close":
			var req tenant.ClosePeriodRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2026-03-31", req.PeriodEndDate)
			assert.Equal(t, "March close", req.Note)
			_ = json.NewEncoder(w).Encode(map[string]any{"tenant": tenantPayload, "event": closeEventPayload("close", "March close")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/period-reopen":
			var req tenant.ReopenPeriodRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2026-03-31", req.PeriodEndDate)
			assert.Equal(t, "Adjustments", req.Note)
			_ = json.NewEncoder(w).Encode(map[string]any{"tenant": tenantPayload, "event": closeEventPayload("reopen", "Adjustments")})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/year-end-close-status":
			require.Equal(t, "2025-12-31", r.URL.Query().Get("period_end_date"))
			_ = json.NewEncoder(w).Encode(statusPayload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/year-end-carry-forward":
			var req accounting.CreateYearEndCarryForwardRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2025-12-31", req.PeriodEndDate)
			_ = json.NewEncoder(w).Encode(carryForwardPayload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/year-end-carry-forward/reverse":
			var req accounting.ReverseYearEndCarryForwardRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2025-12-31", req.PeriodEndDate)
			assert.Equal(t, "Late supplier accrual", req.Reason)
			_ = json.NewEncoder(w).Encode(reversalPayload)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"close", "events", "--limit", "10", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"action": "close"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"close", "period", "--period-end", "2026-03-31", "--note", "March close"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Closed period")
	assert.Contains(t, stdout.String(), "Period end: 2026-03-31")

	stdout.Reset()
	err = app.run(context.Background(), []string{"close", "reopen", "--period-end", "2026-03-31", "--note", "Adjustments"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Reopened period")

	stdout.Reset()
	err = app.run(context.Background(), []string{"close", "year-end-status", "--period-end", "2025-12-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Carry-forward ready: true")

	stdout.Reset()
	err = app.run(context.Background(), []string{"close", "carry-forward", "--period-end", "2025-12-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created year-end carry-forward JE-2026-001")

	stdout.Reset()
	err = app.run(context.Background(), []string{"close", "reverse-carry-forward", "--period-end", "2025-12-31", "--reason", "Late supplier accrual"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Reversed year-end carry-forward JE-2026-002")
}

func TestCLIBankingCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	importFile := writeTempCSV(t, "bank.csv", "date;amount;description;reference;counterparty_name;external_id\n2026-03-15;100.00;Client payment;REF-1;Acme;ext-1\n")
	glAccountID := "acc-bank"
	paymentID := "pay-1"
	reconciliationID := "rec-1"
	accountPayload := func(active bool) map[string]any {
		return map[string]any{
			"id":             "bank-1",
			"tenant_id":      "tenant-1",
			"name":           "Main bank",
			"account_number": "EE471000001020145685",
			"bank_name":      "LHV",
			"swift_code":     "LHVBEE22",
			"currency":       "EUR",
			"gl_account_id":  glAccountID,
			"is_default":     true,
			"is_active":      active,
			"created_at":     "2026-03-15T12:00:00Z",
			"balance":        "100.00",
		}
	}
	transactionPayload := func(followUp string) map[string]any {
		return map[string]any{
			"id":                   "tx-1",
			"tenant_id":            "tenant-1",
			"bank_account_id":      "bank-1",
			"transaction_date":     "2026-03-15T00:00:00Z",
			"value_date":           "2026-03-16T00:00:00Z",
			"amount":               "100.00",
			"currency":             "EUR",
			"description":          "Client payment",
			"reference":            "REF-1",
			"counterparty_name":    "Acme",
			"counterparty_account": "EE111",
			"status":               "UNMATCHED",
			"follow_up_status":     followUp,
			"review_note":          "Need receipt",
			"matched_payment_id":   paymentID,
			"reconciliation_id":    reconciliationID,
			"imported_at":          "2026-03-15T12:00:00Z",
			"external_id":          "ext-1",
		}
	}
	importPayload := map[string]any{
		"import_id":             "import-1",
		"transactions_imported": 1,
		"transactions_matched":  0,
		"duplicates_skipped":    0,
	}
	importHistoryPayload := map[string]any{
		"id":                    "import-1",
		"tenant_id":             "tenant-1",
		"bank_account_id":       "bank-1",
		"file_name":             "bank.csv",
		"transactions_imported": 1,
		"transactions_matched":  0,
		"duplicates_skipped":    0,
		"created_at":            "2026-03-15T12:00:00Z",
	}
	suggestionPayload := map[string]any{
		"payment_id":     "pay-1",
		"payment_number": "PMT-00001",
		"payment_date":   "2026-03-15T00:00:00Z",
		"amount":         "100.00",
		"contact_name":   "Acme",
		"reference":      "REF-1",
		"confidence":     0.95,
		"match_reason":   "Amount and reference match",
	}
	reconciliationPayload := map[string]any{
		"id":              "rec-1",
		"tenant_id":       "tenant-1",
		"bank_account_id": "bank-1",
		"statement_date":  "2026-03-31T00:00:00Z",
		"opening_balance": "0.00",
		"closing_balance": "100.00",
		"status":          "IN_PROGRESS",
		"created_at":      "2026-03-31T12:00:00Z",
		"created_by":      "user-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{accountPayload(true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts":
			var req banking.CreateBankAccountRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Main bank", req.Name)
			assert.Equal(t, "EE471000001020145685", req.AccountNumber)
			assert.Equal(t, "LHV", req.BankName)
			assert.Equal(t, "LHVBEE22", req.SwiftCode)
			assert.Equal(t, "EUR", req.Currency)
			require.NotNil(t, req.GLAccountID)
			assert.Equal(t, "acc-bank", *req.GLAccountID)
			assert.True(t, req.IsDefault)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(accountPayload(true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1":
			_ = json.NewEncoder(w).Encode(accountPayload(true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1":
			var req banking.UpdateBankAccountRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Updated bank", req.Name)
			assert.Equal(t, "SEB", req.BankName)
			require.NotNil(t, req.IsActive)
			assert.False(t, *req.IsActive)
			require.NotNil(t, req.IsDefault)
			assert.True(t, *req.IsDefault)
			payload := accountPayload(false)
			payload["name"] = "Updated bank"
			payload["bank_name"] = "SEB"
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1/transactions":
			require.Equal(t, "UNMATCHED", r.URL.Query().Get("status"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			_ = json.NewEncoder(w).Encode([]map[string]any{transactionPayload("EVIDENCE_REQUIRED")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1/import":
			var req banking.ImportCSVRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "bank.csv", req.FileName)
			require.Len(t, req.Transactions, 1)
			assert.Equal(t, "2026-03-15", req.Transactions[0].Date)
			assert.Equal(t, "100.00", req.Transactions[0].Amount)
			assert.Equal(t, "Client payment", req.Transactions[0].Description)
			assert.True(t, req.SkipDuplicates)
			_ = json.NewEncoder(w).Encode(importPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1/import-history":
			_ = json.NewEncoder(w).Encode([]map[string]any{importHistoryPayload})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/bank-transactions/tx-1":
			_ = json.NewEncoder(w).Encode(transactionPayload("EVIDENCE_REQUIRED"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/bank-transactions/tx-1/suggestions":
			_ = json.NewEncoder(w).Encode([]map[string]any{suggestionPayload})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-transactions/tx-1/match":
			var req banking.MatchTransactionRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "pay-1", req.PaymentID)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "matched"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-transactions/tx-1/unmatch":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "unmatched"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-transactions/tx-1/review":
			var req banking.UpdateTransactionReviewRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.NotNil(t, req.FollowUpStatus)
			assert.Equal(t, banking.FollowUpReadyToMatch, *req.FollowUpStatus)
			require.NotNil(t, req.ReviewNote)
			assert.Equal(t, "Ready after receipt", *req.ReviewNote)
			_ = json.NewEncoder(w).Encode(transactionPayload("READY_TO_MATCH"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-transactions/tx-1/create-payment":
			_ = json.NewEncoder(w).Encode(map[string]string{"payment_id": "pay-created"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1/auto-match":
			require.Equal(t, "0.8", r.URL.Query().Get("min_confidence"))
			_ = json.NewEncoder(w).Encode(map[string]int{"matched": 1})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1/reconciliations":
			_ = json.NewEncoder(w).Encode([]map[string]any{reconciliationPayload})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/bank-accounts/bank-1/reconciliation":
			var req banking.CreateReconciliationRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2026-03-31", req.StatementDate)
			assert.True(t, req.OpeningBalance.Equal(decimal.Zero))
			assert.True(t, req.ClosingBalance.Equal(decimal.RequireFromString("100.00")))
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(reconciliationPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reconciliations/rec-1":
			_ = json.NewEncoder(w).Encode(reconciliationPayload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/reconciliations/rec-1/complete":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"banking", "accounts", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Main bank"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "accounts", "create", "--name", "Main bank", "--account-number", "EE471000001020145685", "--bank-name", "LHV", "--swift-code", "LHVBEE22", "--currency", "eur", "--gl-account-id", "acc-bank", "--default"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created bank account Main bank (bank-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "accounts", "get", "--id", "bank-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bank account Main bank")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "accounts", "update", "--id", "bank-1", "--name", "Updated bank", "--bank-name", "SEB", "--active", "false", "--default", "true"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bank account Updated bank")
	assert.Contains(t, stdout.String(), "Active: false")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "accounts", "delete", "--id", "bank-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted bank account bank-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "list", "--account-id", "bank-1", "--status", "unmatched", "--from", "2026-03-01", "--to", "2026-03-31", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"description": "Client payment"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "import", "--account-id", "bank-1", "--file", importFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Imported: 1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "import-history", "--account-id", "bank-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "bank.csv")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "get", "--id", "tx-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bank transaction tx-1")
	assert.Contains(t, stdout.String(), "Need receipt")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "suggestions", "--id", "tx-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "PMT-00001")
	assert.Contains(t, stdout.String(), "0.95")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "match", "--id", "tx-1", "--payment-id", "pay-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Matched bank transaction tx-1 to payment pay-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "unmatch", "--id", "tx-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "unmatched"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "review", "--id", "tx-1", "--follow-up-status", "ready_to_match", "--review-note", "Ready after receipt"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Follow-up: READY_TO_MATCH")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "create-payment", "--id", "tx-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created payment pay-created from bank transaction tx-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "transactions", "auto-match", "--account-id", "bank-1", "--min-confidence", "0.8"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Matched 1 bank transactions")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "reconciliations", "list", "--account-id", "bank-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "IN_PROGRESS"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "reconciliations", "create", "--account-id", "bank-1", "--statement-date", "2026-03-31", "--opening-balance", "0.00", "--closing-balance", "100.00"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created bank reconciliation rec-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "reconciliations", "get", "--id", "rec-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Bank reconciliation rec-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"banking", "reconciliations", "complete", "--id", "rec-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Completed bank reconciliation rec-1")
}

func TestCLIAnalyticsCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/analytics/dashboard":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"total_revenue":       "1200.00",
				"total_expenses":      "700.00",
				"net_income":          "500.00",
				"revenue_change":      "10.00",
				"expenses_change":     "5.00",
				"total_receivables":   "900.00",
				"total_payables":      "300.00",
				"overdue_receivables": "100.00",
				"overdue_payables":    "50.00",
				"draft_invoices":      1,
				"pending_invoices":    2,
				"overdue_invoices":    3,
				"period_start":        "2026-03-01T00:00:00Z",
				"period_end":          "2026-03-31T00:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/analytics/revenue-expense":
			require.Equal(t, "3", r.URL.Query().Get("months"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"labels":   []string{"2026-03"},
				"revenue":  []string{"1200.00"},
				"expenses": []string{"700.00"},
				"profit":   []string{"500.00"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/analytics/cash-flow":
			require.Equal(t, "6", r.URL.Query().Get("months"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"labels":   []string{"2026-03"},
				"inflows":  []string{"1500.00"},
				"outflows": []string{"800.00"},
				"net":      []string{"700.00"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/analytics/activity":
			require.Equal(t, "5", r.URL.Query().Get("limit"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":          "act-1",
				"type":        "INVOICE",
				"action":      "created",
				"description": "Invoice INV-1",
				"created_at":  "2026-03-31T12:00:00Z",
				"amount":      "219.00",
			}})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"analytics", "dashboard"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Net income: 500")

	stdout.Reset()
	err = app.run(context.Background(), []string{"analytics", "revenue-expense", "--months", "3"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "2026-03")
	assert.Contains(t, stdout.String(), "500")

	stdout.Reset()
	err = app.run(context.Background(), []string{"analytics", "cash-flow", "--months", "6"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "INFLOWS")
	assert.Contains(t, stdout.String(), "700")

	stdout.Reset()
	err = app.run(context.Background(), []string{"analytics", "activity", "--limit", "5", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"description": "Invoice INV-1"`)
}

func TestCLIReportsCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/trial-balance":
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			if r.URL.Query().Get("format") == "csv" {
				w.Header().Set("Content-Type", "text/csv")
				_, _ = w.Write([]byte("account_code,account_name,account_type,debit_balance,credit_balance,net_balance\n1000,Cash,ASSET,500.00,0.00,500.00\n"))
				return
			}
			if r.URL.Query().Get("format") == "xlsx" {
				w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
				_, _ = w.Write([]byte("xlsx-trial-balance"))
				return
			}
			if r.URL.Query().Get("format") == "pdf" {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte("%PDF trial balance"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":     "tenant-1",
				"as_of_date":    "2026-03-31T00:00:00Z",
				"generated_at":  "2026-03-31T12:00:00Z",
				"total_debits":  "500.00",
				"total_credits": "500.00",
				"is_balanced":   true,
				"accounts": []map[string]any{{
					"account_id":     "acc-1",
					"account_code":   "1000",
					"account_name":   "Cash",
					"account_type":   "ASSET",
					"debit_balance":  "500.00",
					"credit_balance": "0.00",
					"net_balance":    "500.00",
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/account-balance/acc-1":
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"account_id": "acc-1",
				"as_of_date": "2026-03-31",
				"balance":    "500.00",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/balance-sheet":
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of"))
			if r.URL.Query().Get("format") == "pdf" {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte("%PDF balance sheet"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":         "tenant-1",
				"as_of_date":        "2026-03-31T00:00:00Z",
				"generated_at":      "2026-03-31T12:00:00Z",
				"assets":            []map[string]any{},
				"liabilities":       []map[string]any{},
				"equity":            []map[string]any{},
				"total_assets":      "500.00",
				"total_liabilities": "200.00",
				"total_equity":      "300.00",
				"retained_earnings": "100.00",
				"is_balanced":       true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/income-statement":
			require.Equal(t, "2026-01-01", r.URL.Query().Get("start"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("end"))
			if r.URL.Query().Get("format") == "pdf" {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte("%PDF income statement"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":      "tenant-1",
				"start_date":     "2026-01-01T00:00:00Z",
				"end_date":       "2026-03-31T00:00:00Z",
				"generated_at":   "2026-03-31T12:00:00Z",
				"revenue":        []map[string]any{},
				"expenses":       []map[string]any{},
				"total_revenue":  "1200.00",
				"total_expenses": "700.00",
				"net_income":     "500.00",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/cash-flow":
			require.Equal(t, "2026-01-01", r.URL.Query().Get("start_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("end_date"))
			if r.URL.Query().Get("format") == "csv" {
				w.Header().Set("Content-Type", "text/csv")
				_, _ = w.Write([]byte("section,code,description,description_et,amount,is_subtotal\nsummary,closing_cash,Closing cash,,500.00,true\n"))
				return
			}
			if r.URL.Query().Get("format") == "pdf" {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte("%PDF cash flow"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":            "tenant-1",
				"start_date":           "2026-01-01",
				"end_date":             "2026-03-31",
				"operating_activities": []map[string]any{{"code": "CF_OPER_TOTAL", "description": "Operating total", "amount": "500.00"}},
				"investing_activities": []map[string]any{},
				"financing_activities": []map[string]any{},
				"total_operating":      "500.00",
				"total_investing":      "0.00",
				"total_financing":      "0.00",
				"net_cash_change":      "500.00",
				"opening_cash":         "0.00",
				"closing_cash":         "500.00",
				"generated_at":         "2026-03-31T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/aging/receivables":
			if r.URL.Query().Get("format") == "csv" {
				w.Header().Set("Content-Type", "text/csv")
				_, _ = w.Write([]byte("row_type,report_type,contact_name,total\ncontact,receivables,Acme,900.00\n"))
				return
			}
			if r.URL.Query().Get("format") == "pdf" {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte("%PDF receivables aging"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"report_type": "receivables",
				"as_of_date":  "2026-03-31T00:00:00Z",
				"total":       "900.00",
				"buckets": []map[string]any{{
					"label":  "Current",
					"amount": "700.00",
					"count":  3,
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/balance-confirmations":
			require.Equal(t, "RECEIVABLE", r.URL.Query().Get("type"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			if r.URL.Query().Get("format") == "xlsx" {
				w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
				_, _ = w.Write([]byte("xlsx-balance-confirmations"))
				return
			}
			if r.URL.Query().Get("format") == "pdf" {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte("%PDF balance confirmations"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":          "RECEIVABLE",
				"as_of_date":    "2026-03-31",
				"total_balance": "900.00",
				"contact_count": 1,
				"invoice_count": 2,
				"contacts": []map[string]any{{
					"contact_id":    "contact-1",
					"contact_name":  "Acme",
					"contact_code":  "CUST-1",
					"contact_email": "billing@example.com",
					"balance":       "900.00",
					"invoice_count": 2,
				}},
				"generated_at": "2026-03-31T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/balance-confirmations/contact-1":
			require.Equal(t, "RECEIVABLE", r.URL.Query().Get("type"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			if r.URL.Query().Get("format") == "csv" {
				w.Header().Set("Content-Type", "text/csv")
				_, _ = w.Write([]byte("invoice_id,invoice_number,outstanding_amount\ninvoice-1,INV-1,900.00\n"))
				return
			}
			if r.URL.Query().Get("format") == "pdf" {
				w.Header().Set("Content-Type", "application/pdf")
				_, _ = w.Write([]byte("%PDF balance confirmation"))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "confirmation-1",
				"tenant_id":     "tenant-1",
				"contact_id":    "contact-1",
				"contact_name":  "Acme",
				"contact_code":  "CUST-1",
				"type":          "RECEIVABLE",
				"as_of_date":    "2026-03-31",
				"total_balance": "900.00",
				"invoices": []map[string]any{{
					"invoice_id":         "invoice-1",
					"invoice_number":     "INV-1",
					"invoice_date":       "2026-03-01",
					"due_date":           "2026-03-15",
					"total_amount":       "1000.00",
					"amount_paid":        "100.00",
					"outstanding_amount": "900.00",
					"currency":           "EUR",
					"days_overdue":       16,
				}},
				"generated_at": "2026-03-31T12:00:00Z",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"reports", "trial-balance", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Trial balance as of 2026-03-31")
	assert.Contains(t, stdout.String(), "1000")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "trial-balance", "--as-of", "2026-03-31", "--csv"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "account_code,account_name")

	stdout.Reset()
	trialBalanceXLSXPath := filepath.Join(t.TempDir(), "trial-balance.xlsx")
	err = app.run(context.Background(), []string{"reports", "trial-balance", "--as-of", "2026-03-31", "--xlsx", "--output", trialBalanceXLSXPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote trial balance XLSX")
	trialBalanceXLSX, err := os.ReadFile(trialBalanceXLSXPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("xlsx-trial-balance"), trialBalanceXLSX)

	stdout.Reset()
	trialBalancePDFPath := filepath.Join(t.TempDir(), "trial-balance.pdf")
	err = app.run(context.Background(), []string{"reports", "trial-balance", "--as-of", "2026-03-31", "--pdf", "--output", trialBalancePDFPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote trial balance PDF")
	trialBalancePDF, err := os.ReadFile(trialBalancePDFPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF trial balance"), trialBalancePDF)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "account-balance", "--account-id", "acc-1", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "500.00")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "balance-sheet", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Balance sheet as of 2026-03-31")

	stdout.Reset()
	balanceSheetPDFPath := filepath.Join(t.TempDir(), "balance-sheet.pdf")
	err = app.run(context.Background(), []string{"reports", "balance-sheet", "--as-of", "2026-03-31", "--pdf", "--output", balanceSheetPDFPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote balance sheet PDF")
	balanceSheetPDF, err := os.ReadFile(balanceSheetPDFPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF balance sheet"), balanceSheetPDF)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "income-statement", "--start", "2026-01-01", "--end", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Net income: 500")

	stdout.Reset()
	incomeStatementPDFPath := filepath.Join(t.TempDir(), "income-statement.pdf")
	err = app.run(context.Background(), []string{"reports", "income-statement", "--start", "2026-01-01", "--end", "2026-03-31", "--pdf", "--output", incomeStatementPDFPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote income statement PDF")
	incomeStatementPDF, err := os.ReadFile(incomeStatementPDFPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF income statement"), incomeStatementPDF)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "cash-flow", "--start", "2026-01-01", "--end", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Closing cash: 500")

	stdout.Reset()
	cashFlowCSVPath := filepath.Join(t.TempDir(), "cash-flow.csv")
	err = app.run(context.Background(), []string{"reports", "cash-flow", "--start", "2026-01-01", "--end", "2026-03-31", "--csv", "--output", cashFlowCSVPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote cash flow CSV")
	cashFlowCSV, err := os.ReadFile(cashFlowCSVPath)
	require.NoError(t, err)
	assert.Contains(t, string(cashFlowCSV), "closing_cash")

	stdout.Reset()
	cashFlowPDFPath := filepath.Join(t.TempDir(), "cash-flow.pdf")
	err = app.run(context.Background(), []string{"reports", "cash-flow", "--start", "2026-01-01", "--end", "2026-03-31", "--pdf", "--output", cashFlowPDFPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote cash flow PDF")
	cashFlowPDF, err := os.ReadFile(cashFlowPDFPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF cash flow"), cashFlowPDF)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "aging", "--type", "receivables", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"report_type": "receivables"`)

	stdout.Reset()
	agingCSVPath := filepath.Join(t.TempDir(), "aging.csv")
	err = app.run(context.Background(), []string{"reports", "aging", "--type", "receivables", "--csv", "--output", agingCSVPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote receivables aging CSV")
	agingCSV, err := os.ReadFile(agingCSVPath)
	require.NoError(t, err)
	assert.Contains(t, string(agingCSV), "Acme")

	stdout.Reset()
	agingPDFPath := filepath.Join(t.TempDir(), "aging.pdf")
	err = app.run(context.Background(), []string{"reports", "aging", "--type", "receivables", "--pdf", "--output", agingPDFPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote receivables aging PDF")
	agingPDF, err := os.ReadFile(agingPDFPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF receivables aging"), agingPDF)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "balance-confirmations", "--type", "receivable", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Total balance: 900")

	stdout.Reset()
	confirmationSummaryXLSXPath := filepath.Join(t.TempDir(), "balance-confirmations.xlsx")
	err = app.run(context.Background(), []string{"reports", "balance-confirmations", "--type", "receivable", "--as-of", "2026-03-31", "--xlsx", "--output", confirmationSummaryXLSXPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote balance confirmations XLSX")
	confirmationSummaryXLSX, err := os.ReadFile(confirmationSummaryXLSXPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("xlsx-balance-confirmations"), confirmationSummaryXLSX)

	stdout.Reset()
	confirmationSummaryPDFPath := filepath.Join(t.TempDir(), "balance-confirmations.pdf")
	err = app.run(context.Background(), []string{"reports", "balance-confirmations", "--type", "receivable", "--as-of", "2026-03-31", "--pdf", "--output", confirmationSummaryPDFPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote balance confirmations PDF")
	confirmationSummaryPDF, err := os.ReadFile(confirmationSummaryPDFPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF balance confirmations"), confirmationSummaryPDF)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "balance-confirmation", "--contact-id", "contact-1", "--type", "RECEIVABLE", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "INV-1")

	stdout.Reset()
	confirmationCSVPath := filepath.Join(t.TempDir(), "balance-confirmation.csv")
	err = app.run(context.Background(), []string{"reports", "balance-confirmation", "--contact-id", "contact-1", "--type", "RECEIVABLE", "--as-of", "2026-03-31", "--csv", "--output", confirmationCSVPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote balance confirmation CSV")
	confirmationCSV, err := os.ReadFile(confirmationCSVPath)
	require.NoError(t, err)
	assert.Contains(t, string(confirmationCSV), "INV-1")

	stdout.Reset()
	confirmationPDFPath := filepath.Join(t.TempDir(), "balance-confirmation.pdf")
	err = app.run(context.Background(), []string{"reports", "balance-confirmation", "--contact-id", "contact-1", "--type", "RECEIVABLE", "--as-of", "2026-03-31", "--pdf", "--output", confirmationPDFPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote balance confirmation PDF")
	confirmationPDF, err := os.ReadFile(confirmationPDFPath)
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF balance confirmation"), confirmationPDF)
}

func TestCLIEmployeesCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	employeesFile := writeTempCSV(t, "employees.csv", "employee_number,first_name,last_name,start_date,base_salary\nEMP-001,Mari,Maasikas,2026-01-15,3200\n")
	employeePayload := func(firstName, lastName string, active bool) map[string]any {
		return map[string]any{
			"id":                     "emp-1",
			"tenant_id":              "tenant-1",
			"employee_number":        "EMP-001",
			"first_name":             firstName,
			"last_name":              lastName,
			"personal_code":          "49001010001",
			"email":                  "mari@example.com",
			"phone":                  "+372 555 1234",
			"address":                "Tallinn",
			"bank_account":           "EE471000001020145685",
			"start_date":             "2026-01-15T00:00:00Z",
			"position":               "Accountant",
			"department":             "Finance",
			"employment_type":        "FULL_TIME",
			"tax_residency":          "EE",
			"apply_basic_exemption":  true,
			"basic_exemption_amount": "700.00",
			"funded_pension_rate":    "0.02",
			"is_active":              active,
			"created_at":             "2026-01-15T00:00:00Z",
			"updated_at":             "2026-01-15T00:00:00Z",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/employees":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{employeePayload("Mari", "Maasikas", true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees":
			var req payroll.CreateEmployeeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Mari", req.FirstName)
			assert.Equal(t, "Maasikas", req.LastName)
			assert.Equal(t, payroll.EmploymentFullTime, req.EmploymentType)
			_ = json.NewEncoder(w).Encode(employeePayload(req.FirstName, req.LastName, true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1":
			_ = json.NewEncoder(w).Encode(employeePayload("Mari", "Maasikas", true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1":
			var req payroll.UpdateEmployeeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Maria", req.FirstName)
			assert.Equal(t, "Finance", req.Department)
			require.NotNil(t, req.ApplyBasicExemption)
			assert.False(t, *req.ApplyBasicExemption)
			require.NotNil(t, req.IsActive)
			assert.True(t, *req.IsActive)
			_ = json.NewEncoder(w).Encode(employeePayload("Maria", "Maasikas", true))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1/salary":
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Contains(t, req, "amount")
			assert.Contains(t, req, "effective_from")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "salary updated"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees/import":
			var req payroll.ImportEmployeesRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "employees.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "EMP-001")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":    1,
				"employees_created": 1,
				"salaries_created":  1,
				"rows_skipped":      0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"employees", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"employee_number": "EMP-001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"employees",
		"create",
		"--employee-number", "EMP-001",
		"--first-name", "Mari",
		"--last-name", "Maasikas",
		"--start-date", "2026-01-15",
		"--employment-type", "FULL_TIME",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created employee Mari Maasikas (emp-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"employees", "get", "--id", "emp-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Employee Mari Maasikas")
	assert.Contains(t, stdout.String(), "Position: Accountant")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"employees", "update",
		"--id", "emp-1",
		"--first-name", "Maria",
		"--department", "Finance",
		"--apply-basic-exemption", "false",
		"--active", "true",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Employee Maria Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"employees", "set-salary", "--id", "emp-1", "--amount", "3200.00", "--effective-from", "2026-03-01"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Set base salary for employee emp-1 to 3200")

	stdout.Reset()
	err = app.run(context.Background(), []string{"employees", "import", "--file", employeesFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 employees, set 1 salaries, skipped 0 rows")
}

func TestCLIPayrollRunCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	payrollRunPayload := func(id, status string, year, month int) map[string]any {
		return map[string]any{
			"id":                  id,
			"tenant_id":           "tenant-1",
			"period_year":         year,
			"period_month":        month,
			"status":              status,
			"payment_date":        "2026-03-31T00:00:00Z",
			"total_gross":         "3200.00",
			"total_net":           "2534.80",
			"total_employer_cost": "4281.60",
			"notes":               "March payroll",
			"created_at":          "2026-03-20T12:00:00Z",
			"updated_at":          "2026-03-20T12:00:00Z",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs":
			require.Equal(t, "2026", r.URL.Query().Get("year"))
			_ = json.NewEncoder(w).Encode([]map[string]any{payrollRunPayload("run-1", "DRAFT", 2026, 3)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs":
			var req payroll.CreatePayrollRunRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, 2026, req.PeriodYear)
			assert.Equal(t, 3, req.PeriodMonth)
			require.NotNil(t, req.PaymentDate)
			assert.Equal(t, "March payroll", req.Notes)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(payrollRunPayload("run-1", "DRAFT", 2026, 3))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1":
			payload := payrollRunPayload("run-1", "DRAFT", 2026, 3)
			payload["payslips"] = []map[string]any{{
				"id":                              "payslip-1",
				"tenant_id":                       "tenant-1",
				"payroll_run_id":                  "run-1",
				"employee_id":                     "emp-1",
				"gross_salary":                    "3200.00",
				"taxable_income":                  "2500.00",
				"income_tax":                      "550.00",
				"unemployment_insurance_employee": "51.20",
				"funded_pension":                  "64.00",
				"net_salary":                      "2534.80",
				"social_tax":                      "1056.00",
				"unemployment_insurance_employer": "25.60",
				"total_employer_cost":             "4281.60",
				"basic_exemption_applied":         "700.00",
				"payment_status":                  "PENDING",
				"created_at":                      "2026-03-20T12:00:00Z",
				"employee": map[string]any{
					"id":              "emp-1",
					"tenant_id":       "tenant-1",
					"employee_number": "EMP-001",
					"first_name":      "Mari",
					"last_name":       "Maasikas",
					"start_date":      "2026-01-01T00:00:00Z",
					"employment_type": "FULL_TIME",
					"is_active":       true,
					"created_at":      "2026-01-01T00:00:00Z",
					"updated_at":      "2026-01-01T00:00:00Z",
				},
			}}
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/calculate":
			_ = json.NewEncoder(w).Encode(payrollRunPayload("run-1", "CALCULATED", 2026, 3))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/approve":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/payslips":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":                              "payslip-1",
				"tenant_id":                       "tenant-1",
				"payroll_run_id":                  "run-1",
				"employee_id":                     "emp-1",
				"gross_salary":                    "3200.00",
				"taxable_income":                  "2500.00",
				"income_tax":                      "550.00",
				"unemployment_insurance_employee": "51.20",
				"funded_pension":                  "64.00",
				"net_salary":                      "2534.80",
				"social_tax":                      "1056.00",
				"unemployment_insurance_employer": "25.60",
				"total_employer_cost":             "4281.60",
				"basic_exemption_applied":         "700.00",
				"payment_status":                  "PENDING",
				"created_at":                      "2026-03-20T12:00:00Z",
				"employee": map[string]any{
					"id":              "emp-1",
					"tenant_id":       "tenant-1",
					"employee_number": "EMP-001",
					"first_name":      "Mari",
					"last_name":       "Maasikas",
					"start_date":      "2026-01-01T00:00:00Z",
					"employment_type": "FULL_TIME",
					"is_active":       true,
					"created_at":      "2026-01-01T00:00:00Z",
					"updated_at":      "2026-01-01T00:00:00Z",
				},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll/tax-preview":
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Contains(t, req, "gross_salary")
			assert.Equal(t, true, req["apply_basic_exemption"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"gross_salary":          "3200.00",
				"basic_exemption":       "700.00",
				"taxable_income":        "2500.00",
				"income_tax":            "550.00",
				"unemployment_employee": "51.20",
				"funded_pension":        "64.00",
				"total_deductions":      "665.20",
				"net_salary":            "2534.80",
				"social_tax":            "1056.00",
				"unemployment_employer": "25.60",
				"total_employer_cost":   "4281.60",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"payroll", "runs", "list", "--year", "2026"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"payroll", "runs", "create",
		"--year", "2026",
		"--month", "3",
		"--payment-date", "2026-03-31",
		"--notes", "March payroll",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created payroll run 2026-03 (run-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "get", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Payroll run 2026-03")
	assert.Contains(t, stdout.String(), "Mari Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "calculate", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "CALCULATED")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "approve", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Approved payroll run run-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "payslips", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Mari Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "tax-preview", "--gross-salary", "3200.00"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Net salary: 2534.8")
}

func TestCLIPayrollImportHistoryCommand(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	payrollFile := writeTempCSV(t, "payroll-history.csv", "period_year,period_month,employee_number,gross_salary\n2025,12,EMP-100,3200.00\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/import-history":
			var req payroll.ImportPayrollHistoryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "payroll-history.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "EMP-100")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":       1,
				"payroll_runs_created": 1,
				"payslips_created":     1,
				"rows_skipped":         0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"payroll", "import-history", "--file", payrollFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 payroll runs, created 1 payslips, skipped 0 rows")
}

func TestCLIPayrollImportLeaveBalancesCommand(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	leaveFile := writeTempCSV(t, "leave-balances.csv", "year,employee_number,absence_type_code,entitled_days\n2025,EMP-100,ANNUAL_LEAVE,28\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/leave-balances/import":
			var req payroll.ImportLeaveBalancesRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "leave-balances.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "ANNUAL_LEAVE")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":         1,
				"leave_balances_created": 1,
				"leave_balances_updated": 0,
				"rows_skipped":           0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"payroll", "import-leave-balances", "--file", leaveFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 leave balances, updated 0 leave balances, skipped 0 rows")
}

func TestCLILeaveCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	leaveFile := writeTempCSV(t, "leave-balances.csv", "year,employee_number,absence_type_code,entitled_days\n2026,EMP-100,ANNUAL_LEAVE,28\n")
	absenceTypePayload := map[string]any{
		"id":                    "type-1",
		"tenant_id":             "tenant-1",
		"code":                  "ANNUAL_LEAVE",
		"name":                  "Annual leave",
		"name_et":               "Pohipuhkus",
		"description":           "Paid annual leave",
		"is_paid":               true,
		"affects_salary":        false,
		"requires_document":     false,
		"default_days_per_year": "28.00",
		"max_carryover_days":    "5.00",
		"is_system":             true,
		"is_active":             true,
		"sort_order":            1,
		"created_at":            "2026-01-01T00:00:00Z",
		"updated_at":            "2026-01-01T00:00:00Z",
	}
	balancePayload := func(entitled string) map[string]any {
		return map[string]any{
			"id":              "balance-1",
			"tenant_id":       "tenant-1",
			"employee_id":     "emp-1",
			"absence_type_id": "type-1",
			"year":            2026,
			"entitled_days":   entitled,
			"carryover_days":  "2.00",
			"used_days":       "5.00",
			"pending_days":    "1.00",
			"remaining_days":  "24.00",
			"notes":           "Imported",
			"created_at":      "2026-01-01T00:00:00Z",
			"updated_at":      "2026-01-01T00:00:00Z",
			"absence_type":    absenceTypePayload,
		}
	}
	leaveRecordPayload := func(status string) map[string]any {
		payload := map[string]any{
			"id":               "leave-1",
			"tenant_id":        "tenant-1",
			"employee_id":      "emp-1",
			"absence_type_id":  "type-1",
			"start_date":       "2026-03-15T00:00:00Z",
			"end_date":         "2026-03-19T00:00:00Z",
			"total_days":       "5.00",
			"working_days":     "3.00",
			"status":           status,
			"document_number":  "DOC-1",
			"document_date":    "2026-03-14T00:00:00Z",
			"requested_at":     "2026-03-01T12:00:00Z",
			"requested_by":     "user-1",
			"notes":            "Spring break",
			"created_at":       "2026-03-01T12:00:00Z",
			"updated_at":       "2026-03-01T12:00:00Z",
			"absence_type":     absenceTypePayload,
			"employee":         map[string]any{"id": "emp-1", "first_name": "Mari", "last_name": "Maasikas"},
			"rejection_reason": "",
		}
		if status == string(payroll.LeaveRejected) {
			payload["rejection_reason"] = "Staffing shortage"
		}
		return payload
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/absence-types":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{absenceTypePayload})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/absence-types/type-1":
			_ = json.NewEncoder(w).Encode(absenceTypePayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1/leave-balances":
			require.Equal(t, "2026", r.URL.Query().Get("year"))
			_ = json.NewEncoder(w).Encode([]map[string]any{balancePayload("28.00")})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1/leave-balances/2026":
			_ = json.NewEncoder(w).Encode([]map[string]any{balancePayload("28.00")})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1/leave-balances/2026/type-1":
			var req payroll.UpdateLeaveBalanceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.NotNil(t, req.EntitledDays)
			assert.True(t, req.EntitledDays.Equal(decimal.RequireFromString("30.00")))
			require.NotNil(t, req.CarryoverDays)
			assert.True(t, req.CarryoverDays.Equal(decimal.RequireFromString("3.00")))
			assert.Equal(t, "Manual correction", req.Notes)
			_ = json.NewEncoder(w).Encode(balancePayload("30.00"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1/leave-balances/2026/initialize":
			_ = json.NewEncoder(w).Encode([]map[string]any{balancePayload("28.00")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/leave-balances/import":
			var req payroll.ImportLeaveBalancesRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "leave-balances.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "ANNUAL_LEAVE")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":         1,
				"leave_balances_created": 1,
				"leave_balances_updated": 0,
				"rows_skipped":           0,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/leave-records":
			require.Equal(t, "emp-1", r.URL.Query().Get("employee_id"))
			require.Equal(t, "2026", r.URL.Query().Get("year"))
			_ = json.NewEncoder(w).Encode([]map[string]any{leaveRecordPayload(string(payroll.LeavePending))})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/leave-records":
			var req payroll.CreateLeaveRecordRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "emp-1", req.EmployeeID)
			assert.Equal(t, "type-1", req.AbsenceTypeID)
			assert.Equal(t, "2026-03-15", req.StartDate.Format("2006-01-02"))
			assert.True(t, req.TotalDays.Equal(decimal.RequireFromString("5.00")))
			assert.True(t, req.WorkingDays.Equal(decimal.RequireFromString("3.00")))
			assert.Equal(t, "DOC-1", req.DocumentNumber)
			require.NotNil(t, req.DocumentDate)
			assert.Equal(t, "2026-03-14", req.DocumentDate.Format("2006-01-02"))
			_ = json.NewEncoder(w).Encode(leaveRecordPayload(string(payroll.LeavePending)))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/leave-records/leave-1":
			_ = json.NewEncoder(w).Encode(leaveRecordPayload(string(payroll.LeavePending)))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/leave-records/leave-1/approve":
			_ = json.NewEncoder(w).Encode(leaveRecordPayload(string(payroll.LeaveApproved)))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/leave-records/leave-1/reject":
			var req payroll.RejectLeaveRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Staffing shortage", req.Reason)
			_ = json.NewEncoder(w).Encode(leaveRecordPayload(string(payroll.LeaveRejected)))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/leave-records/leave-1/cancel":
			_ = json.NewEncoder(w).Encode(leaveRecordPayload(string(payroll.LeaveCanceled)))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"leave", "absence-types", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"code": "ANNUAL_LEAVE"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "absence-types", "get", "--id", "type-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Absence type ANNUAL_LEAVE Annual leave")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "balances", "list", "--employee-id", "emp-1", "--year", "2026"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "ANNUAL_LEAVE")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "balances", "by-year", "--employee-id", "emp-1", "--year", "2026", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"remaining_days": "24"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "balances", "update", "--employee-id", "emp-1", "--absence-type-id", "type-1", "--year", "2026", "--entitled-days", "30.00", "--carryover-days", "3.00", "--notes", "Manual correction"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "30")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "balances", "initialize", "--employee-id", "emp-1", "--year", "2026"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "ANNUAL_LEAVE")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "balances", "import", "--file", leaveFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 leave balances")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "records", "list", "--employee-id", "emp-1", "--year", "2026"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Mari Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "records", "create", "--employee-id", "emp-1", "--absence-type-id", "type-1", "--start-date", "2026-03-15", "--end-date", "2026-03-19", "--total-days", "5.00", "--working-days", "3.00", "--document-number", "DOC-1", "--document-date", "2026-03-14", "--notes", "Spring break"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created leave record leave-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "records", "get", "--id", "leave-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Leave record leave-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "records", "approve", "--id", "leave-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "APPROVED")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "records", "reject", "--id", "leave-1", "--reason", "Staffing shortage"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Staffing shortage")

	stdout.Reset()
	err = app.run(context.Background(), []string{"leave", "records", "cancel", "--id", "leave-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "CANCELLED"`) //nolint:misspell // API status value uses existing database spelling.
}

func TestCLITaxAndTSDCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	kmdFile := writeTempCSV(t, "kmd-history.csv", "year,month,row_code,tax_base,tax_amount\n2025,12,1,1000.00,220.00\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":                          "tsd-1",
				"tenant_id":                   "tenant-1",
				"period_year":                 2026,
				"period_month":                3,
				"total_payments":              "3200.00",
				"total_income_tax":            "500.00",
				"total_social_tax":            "1056.00",
				"total_unemployment_employer": "25.60",
				"total_unemployment_employee": "51.20",
				"total_funded_pension":        "64.00",
				"status":                      "DRAFT",
				"created_at":                  "2026-03-31T12:00:00Z",
				"updated_at":                  "2026-03-31T12:00:00Z",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                          "tsd-1",
				"tenant_id":                   "tenant-1",
				"period_year":                 2026,
				"period_month":                3,
				"total_payments":              "3200.00",
				"total_income_tax":            "500.00",
				"total_social_tax":            "1056.00",
				"total_unemployment_employer": "25.60",
				"total_unemployment_employee": "51.20",
				"total_funded_pension":        "64.00",
				"status":                      "DRAFT",
				"created_at":                  "2026-03-31T12:00:00Z",
				"updated_at":                  "2026-03-31T12:00:00Z",
				"rows": []map[string]any{{
					"id":                              "row-1",
					"tenant_id":                       "tenant-1",
					"declaration_id":                  "tsd-1",
					"employee_id":                     "emp-1",
					"personal_code":                   "49001010001",
					"first_name":                      "Mari",
					"last_name":                       "Maasikas",
					"payment_type":                    "10",
					"gross_payment":                   "3200.00",
					"basic_exemption":                 "700.00",
					"taxable_amount":                  "2500.00",
					"income_tax":                      "500.00",
					"social_tax":                      "1056.00",
					"unemployment_insurance_employer": "25.60",
					"unemployment_insurance_employee": "51.20",
					"funded_pension":                  "64.00",
					"created_at":                      "2026-03-31T12:00:00Z",
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/tsd":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":               "tsd-2",
				"tenant_id":        "tenant-1",
				"period_year":      2026,
				"period_month":     4,
				"total_payments":   "4000.00",
				"total_income_tax": "650.00",
				"total_social_tax": "1320.00",
				"status":           "DRAFT",
				"created_at":       "2026-04-30T12:00:00Z",
				"updated_at":       "2026-04-30T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3/xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte("<TSD>ok</TSD>"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3/csv":
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("period,total\n2026-03,3200.00\n"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3/submit":
			w.Header().Set("Content-Type", "application/json")
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "EMTA-123", req["emta_reference"])
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tax/kmd":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":               "kmd-1",
				"tenant_id":        "tenant-1",
				"year":             2026,
				"month":            3,
				"status":           "DRAFT",
				"total_output_vat": "220.00",
				"total_input_vat":  "80.00",
				"rows":             []map[string]any{},
				"created_at":       "2026-03-31T12:00:00Z",
				"updated_at":       "2026-03-31T12:00:00Z",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/tax/kmd":
			w.Header().Set("Content-Type", "application/json")
			var req map[string]int
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, 2026, req["year"])
			assert.Equal(t, 3, req["month"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":               "kmd-1",
				"tenant_id":        "tenant-1",
				"year":             2026,
				"month":            3,
				"status":           "DRAFT",
				"total_output_vat": "220.00",
				"total_input_vat":  "80.00",
				"rows": []map[string]any{{
					"code":        "1",
					"description": "Taxable sales",
					"tax_base":    "1000.00",
					"tax_amount":  "220.00",
				}},
				"created_at": "2026-03-31T12:00:00Z",
				"updated_at": "2026-03-31T12:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/tax/kmd/import-history":
			w.Header().Set("Content-Type", "application/json")
			var req tax.ImportKMDHistoryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "kmd-history.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "1000.00")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":       1,
				"declarations_created": 1,
				"rows_imported":        1,
				"rows_skipped":         0,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tax/kmd/2026/3/xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte("<KMD>ok</KMD>"))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"tsd", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "get", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Mari Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "generate", "--run-id", "run-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"id": "tsd-2"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "export-xml", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Equal(t, "<TSD>ok</TSD>", stdout.String())

	stdout.Reset()
	outputPath := filepath.Join(t.TempDir(), "tsd.csv")
	err = app.run(context.Background(), []string{"tsd", "export-csv", "--year", "2026", "--month", "3", "--output", outputPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote TSD CSV")
	exported, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(exported), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "mark-submitted", "--year", "2026", "--month", "3", "--emta-reference", "EMTA-123"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Marked TSD 2026-03 as submitted")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tax", "kmd", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tax", "kmd", "generate", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "KMD 2026-03")
	assert.Contains(t, stdout.String(), "Payable: 140")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tax", "kmd", "import-history", "--file", kmdFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 KMD declarations, imported 1 rows, skipped 0 rows")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tax", "kmd", "export-xml", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Equal(t, "<KMD>ok</KMD>", stdout.String())
}

func TestCLIDocumentCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	uploadPath := writeTempCSV(t, "evidence.txt", "statement line")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/documents":
			assert.Equal(t, "payment", r.URL.Query().Get("entity_type"))
			assert.Equal(t, "pay-1", r.URL.Query().Get("entity_id"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":            "doc-1",
				"tenant_id":     "tenant-1",
				"entity_type":   "payment",
				"entity_id":     "pay-1",
				"document_type": "receipt",
				"file_name":     "receipt.pdf",
				"content_type":  "application/pdf",
				"file_size":     1024,
				"review_status": "PENDING",
				"uploaded_by":   "user-1",
				"created_at":    "2026-03-12T00:00:00Z",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/documents/review-summary":
			var req documentReviewSummaryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, documents.EntityTypePayment, req.EntityType)
			assert.Equal(t, []string{"pay-1", "pay-2"}, req.EntityIDs)
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"entity_type":          "payment",
				"entity_id":            "pay-1",
				"total_count":          2,
				"pending_review_count": 1,
				"reviewed_count":       1,
				"approved_count":       0,
				"rejected_count":       0,
				"missing_evidence":     false,
				"has_pending_review":   true,
				"has_rejected":         false,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/documents/retention":
			assert.Equal(t, "2027-03-01", r.URL.Query().Get("as_of"))
			assert.Equal(t, "45", r.URL.Query().Get("horizon_days"))
			assert.Equal(t, "true", r.URL.Query().Get("include_missing"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"as_of_date":              "2027-03-01",
				"cutoff_date":             "2027-04-15",
				"total_count":             1,
				"expired_count":           0,
				"due_soon_count":          1,
				"missing_retention_count": 0,
				"pending_review_count":    1,
				"rejected_count":          0,
				"documents": []map[string]any{{
					"id":              "doc-1",
					"tenant_id":       "tenant-1",
					"entity_type":     "payment",
					"entity_id":       "pay-1",
					"document_type":   "receipt",
					"file_name":       "receipt.pdf",
					"content_type":    "application/pdf",
					"file_size":       1024,
					"retention_until": "2027-03-31T00:00:00Z",
					"review_status":   "PENDING",
					"uploaded_by":     "user-1",
					"created_at":      "2026-03-12T00:00:00Z",
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/documents":
			require.NoError(t, r.ParseMultipartForm(2<<20))
			assert.Equal(t, "bank_transaction", r.FormValue("entity_type"))
			assert.Equal(t, "txn-1", r.FormValue("entity_id"))
			assert.Equal(t, documents.DocumentTypeReconciliation, r.FormValue("document_type"))
			assert.Equal(t, "Statement evidence", r.FormValue("notes"))
			assert.Equal(t, "2027-03-31", r.FormValue("retention_until"))
			file, header, err := r.FormFile("file")
			require.NoError(t, err)
			defer func() { _ = file.Close() }()
			payload, err := io.ReadAll(file)
			require.NoError(t, err)
			assert.Equal(t, "evidence.txt", header.Filename)
			assert.Equal(t, "statement line", string(payload))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "doc-2",
				"tenant_id":     "tenant-1",
				"entity_type":   "bank_transaction",
				"entity_id":     "txn-1",
				"document_type": "reconciliation_evidence",
				"file_name":     "evidence.txt",
				"content_type":  "text/plain",
				"file_size":     len(payload),
				"review_status": "PENDING",
				"uploaded_by":   "user-1",
				"created_at":    "2026-03-12T00:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/documents/doc-2/download":
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Disposition", `attachment; filename="evidence.txt"`)
			_, _ = w.Write([]byte("statement line"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/documents/doc-2/mark-reviewed":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "doc-2",
				"tenant_id":     "tenant-1",
				"entity_type":   "bank_transaction",
				"entity_id":     "txn-1",
				"document_type": "reconciliation_evidence",
				"file_name":     "evidence.txt",
				"content_type":  "text/plain",
				"file_size":     14,
				"review_status": "REVIEWED",
				"uploaded_by":   "user-1",
				"created_at":    "2026-03-12T00:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/documents/doc-2/review":
			var req documents.ReviewDocumentRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, documents.ReviewStatusApproved, req.ReviewStatus)
			assert.Equal(t, "Evidence accepted", req.ReviewNote)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "doc-2",
				"tenant_id":     "tenant-1",
				"entity_type":   "bank_transaction",
				"entity_id":     "txn-1",
				"document_type": "reconciliation_evidence",
				"file_name":     "evidence.txt",
				"content_type":  "text/plain",
				"file_size":     14,
				"review_status": "APPROVED",
				"review_note":   "Evidence accepted",
				"uploaded_by":   "user-1",
				"created_at":    "2026-03-12T00:00:00Z",
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/documents/doc-2":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"documents", "list", "--entity-type", "payment", "--entity-id", "pay-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"file_name": "receipt.pdf"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"documents", "review-summary", "--entity-type", "payment", "--entity-id", "pay-1", "--entity-id", "pay-2"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "pay-1")
	assert.Contains(t, stdout.String(), "true")

	stdout.Reset()
	err = app.run(context.Background(), []string{"documents", "retention", "--as-of", "2027-03-01", "--horizon-days", "45", "--include-missing"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Document retention review as of 2027-03-01")
	assert.Contains(t, stdout.String(), "receipt.pdf")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"documents",
		"upload",
		"--entity-type", "bank_transaction",
		"--entity-id", "txn-1",
		"--file", uploadPath,
		"--document-type", "reconciliation_evidence",
		"--notes", "Statement evidence",
		"--retention-until", "2027-03-31",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Uploaded evidence.txt (doc-2)")

	stdout.Reset()
	downloadPath := filepath.Join(t.TempDir(), "downloaded-evidence.txt")
	err = app.run(context.Background(), []string{"documents", "download", "--id", "doc-2", "--output", downloadPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Downloaded doc-2")
	downloaded, err := os.ReadFile(downloadPath)
	require.NoError(t, err)
	assert.Equal(t, "statement line", string(downloaded))

	stdout.Reset()
	err = app.run(context.Background(), []string{"documents", "mark-reviewed", "--id", "doc-2"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Marked document doc-2 as reviewed")

	stdout.Reset()
	err = app.run(context.Background(), []string{"documents", "review", "--id", "doc-2", "--status", "approved", "--note", "Evidence accepted"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Reviewed document doc-2 as APPROVED")

	stdout.Reset()
	err = app.run(context.Background(), []string{"documents", "delete", "--id", "doc-2"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted document doc-2")
}

func TestCLIHelperFunctionsAndErrors(t *testing.T) {
	configureCLIEnv(t)

	app, _, _ := newTestCLIApp()

	err := app.runAuth(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth subcommand required")

	err = app.runTenant(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant subcommand required")

	err = app.runUsers(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "users subcommand required")

	err = app.runInvitations(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invitations subcommand required")

	err = app.runPlugins(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugins subcommand required")

	err = app.runAdmin(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "admin subcommand required")

	err = app.runTokens(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokens subcommand required")

	err = app.runAccounts(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accounts subcommand required")

	err = app.runContacts(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contacts subcommand required")

	err = app.runEmployees(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "employees subcommand required")

	err = app.runLeave(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leave subcommand required")

	err = app.runLeaveAbsenceTypes(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leave absence-types subcommand required")

	err = app.runLeaveBalances(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leave balances subcommand required")

	err = app.runLeaveRecords(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "leave records subcommand required")

	err = app.runInvoices(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoices subcommand required")

	err = app.runPayments(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payments subcommand required")

	err = app.runReminders(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reminders subcommand required")

	err = app.runReminderRules(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reminders rules subcommand required")

	err = app.runEmail(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email subcommand required")

	err = app.runEmailSMTP(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email smtp subcommand required")

	err = app.runEmailTemplates(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email templates subcommand required")

	err = app.runInterest(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interest subcommand required")

	err = app.runInterestSettings(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interest settings subcommand required")

	err = app.runClose(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close subcommand required")

	err = app.runBanking(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "banking subcommand required")

	err = app.runBankAccounts(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "banking accounts subcommand required")

	err = app.runBankTransactions(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "banking transactions subcommand required")

	err = app.runBankReconciliations(context.Background(), &cliConfig{}, &apiClient{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "banking reconciliations subcommand required")

	err = app.runTSD(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tsd subcommand required")

	err = app.runTax(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tax subcommand required")

	err = app.runReports(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reports subcommand required")

	err = app.runDocuments(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "documents subcommand required")

	err = app.runJournal(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "journal subcommand required")

	password, err := resolvePassword("secret", false)
	require.NoError(t, err)
	assert.Equal(t, "secret", password)

	_, err = resolvePassword("", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password is required")

	originalStdin := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString("stdin-secret\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = originalStdin
	})

	password, err = resolvePassword("", true)
	require.NoError(t, err)
	assert.Equal(t, "stdin-secret", password)

	csvPath := writeTempCSV(t, "rows.csv", "code,name\n1000,Cash\n")
	content, fileName, err := readCSVInput(csvPath)
	require.NoError(t, err)
	assert.Equal(t, "rows.csv", fileName)
	assert.Contains(t, content, "Cash")

	originalStdin = os.Stdin
	r, w, err = os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString("from,stdin\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	content, fileName, err = readCSVInput("-")
	require.NoError(t, err)
	assert.Equal(t, "stdin.csv", fileName)
	assert.Equal(t, "from,stdin\n", content)

	originalStdin = os.Stdin
	r, w, err = os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString("binary-stdin")
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	data, fileName, err := readFileInput("-", "stdin.bin")
	require.NoError(t, err)
	assert.Equal(t, "stdin.bin", fileName)
	assert.Equal(t, []byte("binary-stdin"), data)

	assert.True(t, isValidAccountType(accounting.AccountTypeRevenue))
	assert.False(t, isValidAccountType("INVALID"))

	year, month, err := parseYearMonthFlags("2026", "3")
	require.NoError(t, err)
	assert.Equal(t, 2026, year)
	assert.Equal(t, 3, month)

	_, _, err = parseYearMonthFlags("2026", "13")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "month must be between 1 and 12")

	bounded, err := parseRequiredBoundedInt("limit", "20", 1, 100)
	require.NoError(t, err)
	assert.Equal(t, 20, bounded)

	_, err = parseRequiredBoundedInt("limit", "101", 1, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "limit must be between 1 and 100")

	invoiceType, err := parseRequiredInvoiceType("credit_note")
	require.NoError(t, err)
	assert.Equal(t, invoicing.InvoiceTypeCreditNote, invoiceType)

	_, err = parseRequiredInvoiceType("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid invoice type")

	invoiceStatus, err := parseOptionalInvoiceStatus("partially_paid")
	require.NoError(t, err)
	assert.Equal(t, invoicing.StatusPartiallyPaid, invoiceStatus)

	quoteStatus, err := parseOptionalQuoteStatus("converted")
	require.NoError(t, err)
	assert.Equal(t, quotes.QuoteStatusConverted, quoteStatus)

	_, err = parseOptionalQuoteStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid quote status")

	orderStatus, err := parseOptionalOrderStatus("processing")
	require.NoError(t, err)
	assert.Equal(t, orders.OrderStatusProcessing, orderStatus)

	_, err = parseOptionalOrderStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order status")

	assetStatus, err := parseOptionalAssetStatus("disposed")
	require.NoError(t, err)
	assert.Equal(t, assets.AssetStatusDisposed, assetStatus)

	_, err = parseOptionalAssetStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid asset status")

	productType, err := parseRequiredProductType("service")
	require.NoError(t, err)
	assert.Equal(t, inventory.ProductTypeService, productType)

	_, err = parseRequiredProductType("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type is required")

	productStatus, err := parseOptionalProductStatus("inactive")
	require.NoError(t, err)
	assert.Equal(t, inventory.ProductStatusInactive, productStatus)

	_, err = parseOptionalProductStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid product status")

	quantity, err := parseRequiredDecimal("quantity", "-2.50")
	require.NoError(t, err)
	assert.True(t, quantity.Equal(decimal.RequireFromString("-2.50")))

	frequency, err := parseRequiredRecurringFrequency("yearly")
	require.NoError(t, err)
	assert.Equal(t, recurring.FrequencyYearly, frequency)

	_, err = parseRequiredRecurringFrequency("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid recurring invoice frequency")

	boolValue, err := parseOptionalBoolPtr("send-email", "true")
	require.NoError(t, err)
	require.NotNil(t, boolValue)
	assert.True(t, *boolValue)

	boolValue, err = parseOptionalBoolPtr("send-email", "")
	require.NoError(t, err)
	assert.Nil(t, boolValue)

	depreciationMethod, err := parseOptionalDepreciationMethod("units_of_production")
	require.NoError(t, err)
	assert.Equal(t, assets.DepreciationUnitsOfProd, depreciationMethod)

	_, err = parseOptionalDepreciationMethod("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid depreciation method")

	disposalMethod, err := parseRequiredDisposalMethod("scrapped")
	require.NoError(t, err)
	assert.Equal(t, assets.DisposalScrapped, disposalMethod)

	_, err = parseRequiredDisposalMethod("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method is required")

	budgetPeriod, err := parseOptionalBudgetPeriod("quarterly")
	require.NoError(t, err)
	assert.Equal(t, accounting.BudgetPeriodQuarterly, budgetPeriod)

	_, err = parseOptionalBudgetPeriod("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid budget period")

	optionalAmount, err := parseOptionalNonNegativeDecimalPtr("budget-amount", "12.50")
	require.NoError(t, err)
	require.NotNil(t, optionalAmount)
	assert.True(t, optionalAmount.Equal(decimal.RequireFromString("12.50")))

	optionalAmount, err = parseOptionalNonNegativeDecimalPtr("budget-amount", "")
	require.NoError(t, err)
	assert.Nil(t, optionalAmount)

	var invoiceLines invoiceLineFlags
	require.NoError(t, invoiceLines.Set("description=Service,quantity=1,unit_price=100,vat_rate=22"))
	assert.Equal(t, "Service", invoiceLines.String())

	err = invoiceLines.Set("description=Missing price,quantity=1,vat_rate=22")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unit_price is required")

	var quoteLines quoteLineFlags
	require.NoError(t, quoteLines.Set("description=Offer,qty=2,price=50,vat=22,product=prod-1"))
	assert.Equal(t, "Offer", quoteLines.String())
	require.NotNil(t, quoteLines[0].ProductID)
	assert.Equal(t, "prod-1", *quoteLines[0].ProductID)

	err = quoteLines.Set("description=Missing vat,quantity=1,unit_price=100")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vat_rate is required")

	var orderLines orderLineFlags
	require.NoError(t, orderLines.Set("description=Order line,qty=2,price=50,vat=22,product=prod-2"))
	assert.Equal(t, "Order line", orderLines.String())
	require.NotNil(t, orderLines[0].ProductID)
	assert.Equal(t, "prod-2", *orderLines[0].ProductID)

	err = orderLines.Set("description=Missing quantity,unit_price=100,vat_rate=22")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quantity is required")

	var recurringLines recurringLineFlags
	require.NoError(t, recurringLines.Set("description=Recurring line,qty=2,price=50,vat=22,account=acc-1,product=prod-1"))
	assert.Equal(t, "Recurring line", recurringLines.String())
	require.NotNil(t, recurringLines[0].AccountID)
	require.NotNil(t, recurringLines[0].ProductID)

	err = recurringLines.Set("description=Missing vat,quantity=1,unit_price=100")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vat_rate is required")

	var journalLines journalLineFlags
	require.NoError(t, journalLines.Set("account_id=acc-1,debit=100,description=Debit line"))
	require.NoError(t, journalLines.Set("account_id=acc-2,credit=100,description=Credit line"))
	assert.Equal(t, "acc-1,acc-2", journalLines.String())

	err = journalLines.Set("account_id=acc-3,debit=10,credit=10")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one")

	paymentType, err := parseRequiredPaymentType("made")
	require.NoError(t, err)
	assert.Equal(t, payments.PaymentTypeMade, paymentType)

	_, err = parseRequiredPaymentType("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment type")

	reminderTriggerType, err := parseRequiredReminderTriggerType("after_due")
	require.NoError(t, err)
	assert.Equal(t, invoicing.TriggerAfterDue, reminderTriggerType)

	_, err = parseRequiredReminderTriggerType("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "trigger-type is required")

	_, err = parseRequiredReminderTriggerType("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid reminder trigger type")

	emailTemplateType, err := parseRequiredEmailTemplateType("payment_receipt")
	require.NoError(t, err)
	assert.Equal(t, email.TemplatePaymentReceipt, emailTemplateType)

	_, err = parseRequiredEmailTemplateType("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type is required")

	_, err = parseRequiredEmailTemplateType("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email template type")

	interestRate, err := parseInterestRateFlags("0.0005", "")
	require.NoError(t, err)
	assert.Equal(t, 0.0005, interestRate)

	interestRate, err = parseInterestRateFlags("", "0.1825")
	require.NoError(t, err)
	assert.InDelta(t, 0.0005, interestRate, 0.0000001)

	_, err = parseInterestRateFlags("0.0005", "0.1825")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate and annual-rate cannot both be set")

	_, err = parseInterestRateFlags("", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate or annual-rate is required")

	bankStatus, err := parseOptionalBankTransactionStatus("matched")
	require.NoError(t, err)
	assert.Equal(t, banking.StatusMatched, bankStatus)

	_, err = parseOptionalBankTransactionStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bank transaction status")

	followUpStatus, err := parseRequiredBankFollowUpStatus("evidence_required")
	require.NoError(t, err)
	assert.Equal(t, banking.FollowUpEvidenceRequired, followUpStatus)

	_, err = parseRequiredBankFollowUpStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bank follow-up status")

	bankRows, err := parseBankTransactionCSVRows("date;amount;description\n2026-03-15;10.00;Payment\n")
	require.NoError(t, err)
	require.Len(t, bankRows, 1)
	assert.Equal(t, "2026-03-15", bankRows[0].Date)
	assert.Equal(t, "10.00", bankRows[0].Amount)
	assert.Equal(t, "Payment", bankRows[0].Description)

	_, err = parseBankTransactionCSVRows("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bank transaction CSV is empty")

	var allocations allocationFlags
	require.NoError(t, allocations.Set("inv-1:12.50"))
	assert.Equal(t, "inv-1:12.5", allocations.String())

	err = allocations.Set("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoice-id:amount")

	var invoiceIDs stringListFlags
	require.NoError(t, invoiceIDs.Set(" inv-1 "))
	require.NoError(t, invoiceIDs.Set("inv-2"))
	assert.Equal(t, "inv-1,inv-2", invoiceIDs.String())

	err = invoiceIDs.Set(" ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "value cannot be empty")

	textPath := writeTempCSV(t, "body.html", "<p>From file</p>")
	resolvedText, err := resolveTextFlag("body-html", "", textPath)
	require.NoError(t, err)
	assert.Equal(t, "<p>From file</p>", resolvedText)

	_, err = resolveTextFlag("body-html", "inline", textPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "body-html and body-html-file cannot both be set")

	var exportBuf strings.Builder
	err = writeExportOutput(&exportBuf, "", []byte("raw export"), "Raw")
	require.NoError(t, err)
	assert.Equal(t, "raw export", exportBuf.String())

	require.NoError(t, saveConfig(&cliConfig{BaseURL: "https://api.example.com"}))
	_, _, err = app.loadAuthenticatedClient()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no API token configured")
}

func TestCLIHelperEdgeCases(t *testing.T) {
	_, err := resolveTenantMembership(nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant memberships found")

	memberships := []tenant.TenantMembership{
		{Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"}},
	}
	_, err = resolveTenantMembership(memberships, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tenant "missing" not found`)

	tempDir := t.TempDir()
	badPath := filepath.Join(tempDir, "missing.csv")
	_, _, err = readCSVInput(badPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read file")

	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL: "https://api.example.com",
	}))
	require.NoError(t, deleteConfig())
	require.NoError(t, deleteConfig())
}

func TestLoadStoredConfigRejectsInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	path, err := configPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0o600))

	_, err = loadStoredConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode config")
}

func TestParseDaysToExpiryAndOptionalIntEdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, parseDaysToExpiry(-1))

	_, err := parseOptionalInt(" 42 ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse integer")

	value, err := parseOptionalInt("42")
	require.NoError(t, err)
	assert.Equal(t, 42, value)

	expiresAt := parseDaysToExpiry(1)
	require.NotNil(t, expiresAt)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), *expiresAt, 2*time.Second)
}
