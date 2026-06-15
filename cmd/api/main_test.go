package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
)

func TestLoadConfigUsesDefaultsAndEnvOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://db")
	t.Setenv("PORT", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")
	t.Setenv("APP_ENV", "")
	t.Setenv("ENV", "")
	t.Setenv("GO_ENV", "")
	t.Setenv("RAILWAY_ENVIRONMENT", "")
	t.Setenv("PASSWORD_RESET_BASE_URL", "")
	t.Setenv("PASSWORD_RESET_EXPOSE_TOKEN", "")
	t.Setenv("PASSWORD_RESET_SMTP_HOST", "")
	t.Setenv("PASSWORD_RESET_SMTP_FROM_EMAIL", "")

	cfg := loadConfig()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "postgres://db", cfg.DatabaseURL)
	assert.Equal(t, developmentJWTSecret, cfg.JWTSecret)
	assert.Equal(t, 15*time.Minute, cfg.AccessExpiry)
	assert.Equal(t, 7*24*time.Hour, cfg.RefreshExpiry)
	assert.Contains(t, cfg.AllowedOrigins, "http://localhost:5173")
	assert.Contains(t, cfg.AllowedOrigins, "https://app.example.com")
	assert.Contains(t, cfg.AllowedOrigins, "https://admin.example.com")
	assert.False(t, cfg.PasswordReset.ExposeToken)
	assert.Nil(t, cfg.PasswordReset.SMTPConfig)

	t.Setenv("PORT", "9090")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("PASSWORD_RESET_BASE_URL", "https://app.example.com/reset-password")
	t.Setenv("PASSWORD_RESET_EXPOSE_TOKEN", "true")
	t.Setenv("PASSWORD_RESET_SMTP_HOST", "smtp.example.com")
	t.Setenv("PASSWORD_RESET_SMTP_PORT", "2525")
	t.Setenv("PASSWORD_RESET_SMTP_USERNAME", "mailer")
	t.Setenv("PASSWORD_RESET_SMTP_PASSWORD", "smtp-secret")
	t.Setenv("PASSWORD_RESET_SMTP_FROM_EMAIL", "no-reply@example.com")
	t.Setenv("PASSWORD_RESET_SMTP_FROM_NAME", "Open Accounting")
	t.Setenv("PASSWORD_RESET_SMTP_USE_TLS", "true")

	cfg = loadConfig()
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "secret", cfg.JWTSecret)
	assert.Equal(t, []string{"http://localhost:5173", "http://localhost:3000"}, cfg.AllowedOrigins)
	assert.Equal(t, "https://app.example.com/reset-password", cfg.PasswordReset.BaseURL)
	assert.True(t, cfg.PasswordReset.ExposeToken)
	require.NotNil(t, cfg.PasswordReset.SMTPConfig)
	assert.Equal(t, "smtp.example.com", cfg.PasswordReset.SMTPConfig.Host)
	assert.Equal(t, 2525, cfg.PasswordReset.SMTPConfig.Port)
	assert.Equal(t, "mailer", cfg.PasswordReset.SMTPConfig.Username)
	assert.Equal(t, "smtp-secret", cfg.PasswordReset.SMTPConfig.Password)
	assert.Equal(t, "no-reply@example.com", cfg.PasswordReset.SMTPConfig.FromEmail)
	assert.Equal(t, "Open Accounting", cfg.PasswordReset.SMTPConfig.FromName)
	assert.True(t, cfg.PasswordReset.SMTPConfig.UseTLS)
}

func TestLoadDocumentRetentionReminderPolicy(t *testing.T) {
	t.Setenv("DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS", "5")
	t.Setenv("DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS", "4")

	policy := loadDocumentRetentionReminderPolicy()
	assert.Equal(t, 5, policy.MaxAttempts)
	assert.Equal(t, 4, policy.EscalateAfterAttempts)

	t.Setenv("DOCUMENT_RETENTION_REMINDER_MAX_ATTEMPTS", "0")
	t.Setenv("DOCUMENT_RETENTION_REMINDER_ESCALATE_AFTER_ATTEMPTS", "bad")

	policy = loadDocumentRetentionReminderPolicy()
	assert.Zero(t, policy.MaxAttempts)
	assert.Zero(t, policy.EscalateAfterAttempts)
}

func TestProductionConfigValidation(t *testing.T) {
	secret, err := resolveJWTSecret("", true)
	require.Error(t, err)
	assert.Empty(t, secret)

	secret, err = resolveJWTSecret("short", true)
	require.Error(t, err)
	assert.Empty(t, secret)

	secret, err = resolveJWTSecret("01234567890123456789012345678901", true)
	require.NoError(t, err)
	assert.Equal(t, "01234567890123456789012345678901", secret)

	origins, err := resolveAllowedOrigins("", true)
	require.Error(t, err)
	assert.Empty(t, origins)

	origins, err = resolveAllowedOrigins("https://app.example.com, https://admin.example.com", true)
	require.NoError(t, err)
	assert.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, origins)
	assert.NotContains(t, origins, "http://localhost:5173")
}

func TestSetupRouterRegistersCoreRoutes(t *testing.T) {
	cfg := &Config{
		AllowedOrigins: []string{"http://localhost:5173"},
	}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)

	t.Setenv("CORS_DEBUG", "true")
	t.Setenv("DEMO_MODE", "false")

	router := setupRouter(cfg, &Handlers{}, tokenService)
	require.NotNil(t, router)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "OK", rr.Body.String())

	routes := make(map[string]string)
	err := chi.Walk(router, func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = route
		return nil
	})
	require.NoError(t, err)

	assert.Contains(t, routes, "GET /health")
	assert.Contains(t, routes, "POST /api/v1/auth/login")
	assert.Contains(t, routes, "POST /api/v1/auth/logout")
	assert.Contains(t, routes, "POST /api/v1/auth/password-reset/request")
	assert.Contains(t, routes, "POST /api/v1/auth/password-reset/confirm")
	assert.Contains(t, routes, "GET /api/v1/auth/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/auth/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/auth/sessions/{sessionID}")
	assert.Contains(t, routes, "GET /api/v1/auth/security-events")
	assert.Contains(t, routes, "GET /api/v1/me")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/complete-onboarding")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/users/{userID}/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/users/{userID}/sessions")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/users/{userID}/sessions/{sessionID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/users/{userID}/api-tokens")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/users/{userID}/api-tokens/{tokenID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/users/{userID}/security-events")
	assert.Contains(t, routes, "PUT /api/v1/tenants/{tenantID}/users/{userID}/status")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/period-close-events")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/period-close")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/period-reopen")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/year-end-close-status")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/year-end-close-pack")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/year-end-carry-forward")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/year-end-carry-forward/reverse")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/journal-entries")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/quotes/import")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/import")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/orders/{orderID}/stock-check")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/orders/{orderID}/stock-reservations")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/orders/{orderID}/pick-list")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/{orderID}/reserve-stock")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/{orderID}/release-stock")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/orders/{orderID}/convert-to-invoice")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/recurring-invoices/import")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/review-summary")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/evidence-policy")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents/retention")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/purge")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents/{documentID}/download")
	assert.Contains(t, routes, "PATCH /api/v1/tenants/{tenantID}/documents/{documentID}/retention")
	assert.Contains(t, routes, "PATCH /api/v1/tenants/{tenantID}/documents/{documentID}/lifecycle")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/{documentID}/review")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/{documentID}/mark-reviewed")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/documents/{documentID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/provider-presets")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/migration/validate")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/migration/execution-plan")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/migration/execute")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/execution-runs")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/execution-runs/{runID}")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/migration/execution-runs/{runID}/events")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/bank-transactions/{transactionID}/review")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/webhooks/events")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/webhooks/{webhookID}/test")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/expenses")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/expenses/import")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/expenses/{expenseID}/post")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/plugins/{pluginID}/runtime/*")
	assert.Contains(t, routes, "GET /api/v1/admin/plugins")
	assert.Contains(t, routes, "GET /swagger/*")

	for _, tt := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/tenants/tenant-1"},
		{method: http.MethodPut, path: "/api/v1/tenants/tenant-1"},
	} {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "%s %s should be protected, not missing", tt.method, tt.path)
	}
}

func TestSetupRouterDisablesRateLimitInDemoMode(t *testing.T) {
	cfg := &Config{AllowedOrigins: []string{"http://localhost:5173"}}
	tokenService := auth.NewTokenService("secret", time.Minute, time.Hour)

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("CORS_DEBUG", "")

	router := setupRouter(cfg, &Handlers{}, tokenService)
	require.NotNil(t, router)

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "http://localhost:5173", rr.Header().Get("Access-Control-Allow-Origin"))
}
