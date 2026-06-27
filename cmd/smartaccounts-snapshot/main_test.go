package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/stretchr/testify/require"
)

func TestRunOutputsJSONManifest(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "prepared")
	writeSnapshotTestFile(t, filepath.Join(sourceDir, "clients.csv"), "client_no,client_name\nC-1,Example OU\n")

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"--source-dir", sourceDir,
		"--out-dir", outputDir,
		"--company-id", "12345678",
		"--company-name", "Example Export OU",
		"--cutover-date", "2026-01-01",
		"--json",
	}, &stdout, &stderr)
	require.NoError(t, err)
	require.Empty(t, stderr.String())

	var report cutover.SmartAccountsSnapshotReport
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &report))
	require.Equal(t, cutover.MigrationProviderPresetSmartAccounts, report.Provider)
	require.Len(t, report.PreparedFiles, 1)
	require.NotEmpty(t, report.SnapshotHash)
}

func TestRunOutputsHumanSummaryWithWarningsAndUnsupportedFiles(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755))
	sourceDir := filepath.Join(repoDir, "export")
	outputDir := filepath.Join(repoDir, "prepared")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	writeSnapshotTestFile(t, filepath.Join(sourceDir, "clients.csv"), "client_no,client_name\nC-1,Example OU\n")
	writeSnapshotTestFile(t, filepath.Join(sourceDir, "notes.txt"), "manual note\n")

	var stdout, stderr bytes.Buffer
	err := run([]string{"--source-dir", sourceDir, "--out-dir", outputDir}, &stdout, &stderr)
	require.NoError(t, err)
	require.Empty(t, stderr.String())
	require.Contains(t, stdout.String(), "SmartAccounts snapshot prepared")
	require.Contains(t, stdout.String(), "Prepared files: 1")
	require.Contains(t, stdout.String(), "Unsupported files: 1")
	require.Contains(t, stdout.String(), "Warnings: 1")
	require.Contains(t, stdout.String(), "Validate bundle:")
}

func TestRunReportsFlagAndInputErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	require.Error(t, run([]string{"--bogus"}, &stdout, &stderr))

	stdout.Reset()
	stderr.Reset()
	require.ErrorContains(t, run(nil, &stdout, &stderr), "source-dir and out-dir are required")

	stdout.Reset()
	stderr.Reset()
	require.ErrorContains(t, run([]string{"--source-dir", filepath.Join(t.TempDir(), "missing"), "--out-dir", filepath.Join(t.TempDir(), "out")}, &stdout, &stderr), "stat source dir")
}

func TestMainUsesInjectedIOAndExit(t *testing.T) {
	oldArgs, oldStdout, oldStderr, oldExit := commandArgs, commandStdout, commandStderr, exitProcess
	defer func() {
		commandArgs, commandStdout, commandStderr, exitProcess = oldArgs, oldStdout, oldStderr, oldExit
	}()
	require.NotNil(t, oldArgs())

	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "prepared")
	writeSnapshotTestFile(t, filepath.Join(sourceDir, "clients.csv"), "client_no,client_name\nC-1,Example OU\n")
	var stdout, stderr bytes.Buffer
	commandArgs = func() []string { return []string{"--source-dir", sourceDir, "--out-dir", outputDir} }
	commandStdout = &stdout
	commandStderr = &stderr
	exitProcess = func(code int) { t.Fatalf("unexpected exit %d", code) }
	main()
	require.Contains(t, stdout.String(), "SmartAccounts snapshot prepared")
	require.Empty(t, stderr.String())

	stdout.Reset()
	stderr.Reset()
	exitCode := 0
	commandArgs = func() []string { return nil }
	exitProcess = func(code int) { exitCode = code }
	main()
	require.Equal(t, 1, exitCode)
	require.Contains(t, stderr.String(), "source-dir and out-dir are required")
}

func writeSnapshotTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
