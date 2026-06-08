package main

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/tenant"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestResolveTenantMembership(t *testing.T) {
	memberships := []tenant.TenantMembership{
		{
			Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"},
		},
		{
			Tenant:    tenant.Tenant{ID: "tenant-2", Name: "Beta", Slug: "beta"},
			IsDefault: true,
		},
	}

	match, err := resolveTenantMembership(memberships, "")
	require.NoError(t, err)
	assert.Equal(t, "tenant-2", match.Tenant.ID)

	match, err = resolveTenantMembership(memberships, "alpha")
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", match.Tenant.ID)

	match, err = resolveTenantMembership(memberships, "tenant-2")
	require.NoError(t, err)
	assert.Equal(t, "Beta", match.Tenant.Name)
}

func TestResolveTenantMembershipRequiresSelectorWhenAmbiguous(t *testing.T) {
	memberships := []tenant.TenantMembership{
		{Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"}},
		{Tenant: tenant.Tenant{ID: "tenant-2", Name: "Beta", Slug: "beta"}},
	}

	_, err := resolveTenantMembership(memberships, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple tenants found")
}

func TestLoadPublicClient(t *testing.T) {
	app := &cliApp{}

	client, err := app.loadPublicClient("https://api.example.com/")
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com", client.baseURL)

	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{BaseURL: "https://stored.example.com/"}))
	client, err = app.loadPublicClient("")
	require.NoError(t, err)
	assert.Equal(t, "https://stored.example.com", client.baseURL)
}

func TestResolveOperatorScriptPathFromEnv(t *testing.T) {
	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "db-backup.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\n"), 0o700))
	t.Setenv("OA_SCRIPT_DIR", scriptDir)

	resolved, err := resolveOperatorScriptPath("db-backup.sh")

	require.NoError(t, err)
	expectedInfo, err := os.Stat(scriptPath)
	require.NoError(t, err)
	actualInfo, err := os.Stat(resolved)
	require.NoError(t, err)
	assert.True(t, os.SameFile(expectedInfo, actualInfo))
}

func TestResolveOperatorScriptPathSearchesParentScripts(t *testing.T) {
	rootDir := t.TempDir()
	scriptDir := filepath.Join(rootDir, "scripts")
	require.NoError(t, os.MkdirAll(scriptDir, 0o755))
	scriptPath := filepath.Join(scriptDir, "db-backup.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\n"), 0o700))
	nestedDir := filepath.Join(rootDir, "cmd", "oa")
	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	t.Setenv("OA_SCRIPT_DIR", "")

	oldCWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(nestedDir))
	t.Cleanup(func() {
		_ = os.Chdir(oldCWD)
	})

	resolved, err := resolveOperatorScriptPath("db-backup.sh")

	require.NoError(t, err)
	expectedInfo, err := os.Stat(scriptPath)
	require.NoError(t, err)
	actualInfo, err := os.Stat(resolved)
	require.NoError(t, err)
	assert.True(t, os.SameFile(expectedInfo, actualInfo))
}

func TestResolveOperatorScriptPathReturnsMissingScriptError(t *testing.T) {
	t.Setenv("OA_SCRIPT_DIR", t.TempDir())

	_, err := resolveOperatorScriptPath("missing.sh")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator script not found")
}

func TestResolveOperatorScriptPathFallsBackToRelativeMissingScript(t *testing.T) {
	rootDir := t.TempDir()
	t.Setenv("OA_SCRIPT_DIR", "")
	oldCWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(rootDir))
	t.Cleanup(func() {
		_ = os.Chdir(oldCWD)
	})

	_, err = resolveOperatorScriptPath("definitely-missing-oa-script.sh")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator script not found")
}

func TestMainPrintsUsageWithoutArgs(t *testing.T) {
	oldArgs := os.Args
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	os.Args = []string{"oa"}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
		_ = reader.Close()
	})

	main()
	require.NoError(t, writer.Close())
	output := make([]byte, 4096)
	n, err := reader.Read(output)
	require.NoError(t, err)
	assert.Contains(t, string(output[:n]), "Open Accounting CLI")
}

func TestLoadTokenClientReturnsConfigLoadError(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	path, err := configPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0o600))

	_, _, err = (&cliApp{}).loadTokenClient()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode config")
}

func TestResolvePasswordPair(t *testing.T) {
	currentPassword, newPassword, err := resolvePasswordPair("current", "new", false)
	require.NoError(t, err)
	assert.Equal(t, "current", currentPassword)
	assert.Equal(t, "new", newPassword)

	_, _, err = resolvePasswordPair("", "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current-password and new-password are required")

	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	_, err = writer.WriteString("current-from-stdin\nnew-from-stdin\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	})

	currentPassword, newPassword, err = resolvePasswordPair("", "", true)
	require.NoError(t, err)
	assert.Equal(t, "current-from-stdin", currentPassword)
	assert.Equal(t, "new-from-stdin", newPassword)
}

func TestResolvePasswordStdinReadErrors(t *testing.T) {
	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.NoError(t, writer.Close())
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = oldStdin
	})

	_, err = resolvePassword("", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read password from stdin")
}

func TestResolvePasswordPairPropagatesStdinReadErrors(t *testing.T) {
	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.NoError(t, writer.Close())
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = oldStdin
	})

	_, _, err = resolvePasswordPair("", "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read current password from stdin")

	reader, writer, err = os.Pipe()
	require.NoError(t, err)
	_, err = writer.WriteString("current\n")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	os.Stdin = reader
	t.Cleanup(func() {
		_ = reader.Close()
	})

	_, _, err = resolvePasswordPair("", "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "new password from stdin is empty")
}

func TestReadPasswordLine(t *testing.T) {
	value, err := readPasswordLine(bufio.NewReader(strings.NewReader("secret\r\n")), "password")
	require.NoError(t, err)
	assert.Equal(t, "secret", value)

	_, err = readPasswordLine(bufio.NewReader(strings.NewReader("\n")), "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password from stdin is empty")

	_, err = readPasswordLine(bufio.NewReader(errReader{}), "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read password from stdin")
}

func TestReadFileInputStdinReadError(t *testing.T) {
	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.NoError(t, writer.Close())
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = oldStdin
	})

	_, _, err = readFileInput("-", "stdin.bin")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read stdin")
}

func TestParseBankAccountCSVRowsHeaderError(t *testing.T) {
	_, err := parseBankAccountCSVRows(`"`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read bank account CSV header")
}
