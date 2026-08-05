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

func TestSetupRouterRegistersRepresentativeRouteGroups(t *testing.T) {
	t.Setenv("CORS_DEBUG", "")
	t.Setenv("DEMO_MODE", "true")

	router := setupRouter(
		&Config{AllowedOrigins: []string{"http://localhost:5173"}},
		&Handlers{},
		auth.NewTokenService("secret", time.Minute, time.Hour),
	)

	routes := make(map[string]struct{})
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = struct{}{}
		return nil
	})
	require.NoError(t, err)

	for _, route := range []string{
		"POST /api/v1/auth/login",
		"GET /api/v1/invitations/{token}",
		"GET /api/v1/me",
		"GET /api/v1/admin/plugins",
		"GET /api/v1/tenants/{tenantID}/accounts",
	} {
		assert.Contains(t, routes, route)
	}

	for _, method := range []string{http.MethodGet, http.MethodPut} {
		req := httptest.NewRequest(method, "/api/v1/tenants/tenant-1", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "%s exact tenant route should be registered", method)
	}
}
