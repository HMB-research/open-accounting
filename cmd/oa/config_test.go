package main

import (
	"os"
	"path/filepath"
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

func TestDefaultBaseURLUsesEnv(t *testing.T) {
	t.Setenv("OA_BASE_URL", " api.example.com/ ")

	assert.Equal(t, "https://api.example.com", defaultBaseURL())
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

func TestConfigFilesystemErrorBranches(t *testing.T) {
	tempDir := t.TempDir()
	configRootFile := filepath.Join(tempDir, "config-root")
	require.NoError(t, os.WriteFile(configRootFile, []byte("not a directory"), 0o600))
	t.Setenv("HOME", configRootFile)
	t.Setenv("XDG_CONFIG_HOME", configRootFile)

	err := saveConfig(&cliConfig{BaseURL: "https://api.example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create config dir")

	tempDir = t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	path, err := configPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(path, 0o700))

	err = saveConfig(&cliConfig{BaseURL: "https://api.example.com"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write config")

	childPath := filepath.Join(path, "child")
	require.NoError(t, os.WriteFile(childPath, []byte("x"), 0o600))
	err = deleteConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove config")
}
