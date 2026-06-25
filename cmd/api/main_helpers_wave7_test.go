package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/scheduler"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWave7DefaultAPIMainDepsFactories(t *testing.T) {
	deps := defaultAPIMainDeps()
	require.NotNil(t, deps.getenv)
	require.NotNil(t, deps.loadConfig)
	require.NotNil(t, deps.newPool)
	require.NotNil(t, deps.buildApplication)
	require.NotNil(t, deps.newServer)
	require.NotNil(t, deps.signalNotify)

	pool, err := deps.newPool(context.Background(), "postgres://127.0.0.1:1/open_accounting_test?connect_timeout=1")
	require.NoError(t, err)
	require.NotNil(t, pool)
	pool.Close()

	app, err := deps.buildApplication(context.Background(), &Config{DocumentsDir: t.TempDir()}, &fakeAPIPool{}, scheduler.DefaultConfig())
	require.Error(t, err)
	assert.Nil(t, app)
	assert.Contains(t, err.Error(), "pgx pool")

	server := deps.newServer(&Config{Port: "9099", AllowedOrigins: []string{"http://localhost:5173"}}, &apiApplication{
		handlers:     &Handlers{},
		tokenService: auth.NewTokenService("secret", time.Minute, time.Hour),
	})
	httpServer, ok := server.(*http.Server)
	require.True(t, ok)
	assert.Equal(t, ":9099", httpServer.Addr)
	assert.Equal(t, 15*time.Second, httpServer.ReadTimeout)
	assert.Equal(t, 15*time.Second, httpServer.WriteTimeout)
	assert.Equal(t, 60*time.Second, httpServer.IdleTimeout)
}

func TestWave7MainHelperEdges(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/unit", nil)
	require.NoError(t, err)
	assert.Empty(t, userIDFromRequest(req))

	assert.Empty(t, tenantInventoryValuationMethod(nil, ""))
	assert.Empty(t, tenantInventoryIssueCostingMethod(nil, ""))
	assert.Equal(t, "FIFO", tenantInventoryValuationMethod(nil, " FIFO "))
	assert.Equal(t, "BATCH", tenantInventoryIssueCostingMethod(nil, " BATCH "))

	tenantRecord := &tenant.Tenant{}
	assert.Equal(t, tenant.InventoryValuationMethodStandardCost, tenantInventoryValuationMethod(tenantRecord, ""))
	assert.Equal(t, tenant.InventoryIssueCostingMethodLot, tenantInventoryIssueCostingMethod(tenantRecord, ""))
}

func TestWave7EnvironmentHelperBranches(t *testing.T) {
	for _, key := range []string{"APP_ENV", "ENV", "GO_ENV"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("APP_ENV", "")
			t.Setenv("ENV", "")
			t.Setenv("GO_ENV", "")
			t.Setenv("RAILWAY_ENVIRONMENT", "")
			t.Setenv(key, " production ")
			assert.True(t, isProductionEnvironment())
		})
	}

	t.Setenv("APP_ENV", "")
	t.Setenv("ENV", "")
	t.Setenv("GO_ENV", "")
	t.Setenv("RAILWAY_ENVIRONMENT", "production")
	assert.True(t, isProductionEnvironment())

	t.Setenv("PASSWORD_RESET_SMTP_HOST", "smtp.example.com")
	t.Setenv("PASSWORD_RESET_SMTP_FROM_EMAIL", "no-reply@example.com")
	t.Setenv("PASSWORD_RESET_SMTP_PORT", "bad")
	cfg := loadPasswordResetSMTPConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, 587, cfg.Port)
}
