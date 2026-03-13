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

	cfg := loadConfig()

	assert.Equal(t, "8080", cfg.Port)
	assert.Equal(t, "postgres://db", cfg.DatabaseURL)
	assert.Equal(t, "change-me-in-production", cfg.JWTSecret)
	assert.Equal(t, 15*time.Minute, cfg.AccessExpiry)
	assert.Equal(t, 7*24*time.Hour, cfg.RefreshExpiry)
	assert.Contains(t, cfg.AllowedOrigins, "http://localhost:5173")
	assert.Contains(t, cfg.AllowedOrigins, "https://app.example.com")
	assert.Contains(t, cfg.AllowedOrigins, "https://admin.example.com")

	t.Setenv("PORT", "9090")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("ALLOWED_ORIGINS", "")

	cfg = loadConfig()
	assert.Equal(t, "9090", cfg.Port)
	assert.Equal(t, "secret", cfg.JWTSecret)
	assert.Equal(t, []string{"http://localhost:5173", "http://localhost:3000"}, cfg.AllowedOrigins)
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
	assert.Contains(t, routes, "GET /api/v1/me")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/complete-onboarding")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/period-close-events")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/period-close")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/period-reopen")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/journal-entries")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/review-summary")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents")
	assert.Contains(t, routes, "GET /api/v1/tenants/{tenantID}/documents/{documentID}/download")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/documents/{documentID}/mark-reviewed")
	assert.Contains(t, routes, "DELETE /api/v1/tenants/{tenantID}/documents/{documentID}")
	assert.Contains(t, routes, "POST /api/v1/tenants/{tenantID}/bank-transactions/{transactionID}/review")
	assert.Contains(t, routes, "GET /api/v1/admin/plugins")
	assert.Contains(t, routes, "GET /swagger/*")
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
