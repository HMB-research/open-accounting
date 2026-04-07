package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://example.com", normalizeBaseURL("example.com/"))
	assert.Equal(t, "http://localhost:8080", normalizeBaseURL(" http://localhost:8080/ "))
	assert.Equal(t, "http://localhost:8080", normalizeBaseURL(""))
}

func TestLoadRuntimeConfigDefaultsWithoutStoredConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OA_BASE_URL", "")
	t.Setenv("OA_API_TOKEN", "")
	t.Setenv("OA_TENANT_ID", "")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
	assert.Empty(t, cfg.APIToken)
	assert.Empty(t, cfg.TenantID)
}

func TestLoadRuntimeConfigAppliesStoredConfigAndEnvOverrides(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://stored.example.com/",
		TenantID:   "tenant-stored",
		TenantName: "Stored Tenant",
		TenantSlug: "stored-tenant",
		APIToken:   "stored-token",
	}))

	t.Setenv("OA_BASE_URL", "api.example.com/")
	t.Setenv("OA_API_TOKEN", "env-token")
	t.Setenv("OA_TENANT_ID", "tenant-env")

	cfg, err := loadRuntimeConfig()
	require.NoError(t, err)

	assert.Equal(t, "https://api.example.com", cfg.BaseURL)
	assert.Equal(t, "env-token", cfg.APIToken)
	assert.Equal(t, "tenant-env", cfg.TenantID)
	assert.Equal(t, "Stored Tenant", cfg.TenantName)
	assert.Equal(t, "stored-tenant", cfg.TenantSlug)
}
