package cutover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPrepareSmartAccountsSnapshotCanonicalizesAndValidatesBundle(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "prepared")
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "chart_of_accounts.csv"), []byte("account_no;account_title;classification\n1000;Cash;ASSET\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no;client_name;registration_no;email_address\nC-1;Example OU;12345678;info@example.test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "notes.txt"), []byte("not part of migration"), 0o644))

	report, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir:         sourceDir,
		OutputDir:         outputDir,
		SourceCompanyID:   "14369460",
		SourceCompanyName: "Hold My Beer OU",
		CutoverDate:       "2026-01-01",
		GeneratedAt:       time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	require.Equal(t, MigrationProviderPresetSmartAccounts, report.Provider)
	require.Len(t, report.PreparedFiles, 2)
	require.Len(t, report.UnsupportedFiles, 1)
	require.NotEmpty(t, report.SnapshotHash)
	require.FileExists(t, filepath.Join(outputDir, "manifest.json"))
	require.FileExists(t, filepath.Join(outputDir, "bundle", "accounts.csv"))
	require.FileExists(t, filepath.Join(outputDir, "bundle", "contacts.csv"))
	require.Contains(t, report.ValidationCommand, "--provider-preset smartaccounts")
	require.Contains(t, report.ValidationCommand, filepath.Join(outputDir, "bundle", "accounts.csv"))

	accountsContent, err := os.ReadFile(filepath.Join(outputDir, "bundle", "accounts.csv"))
	require.NoError(t, err)
	require.Contains(t, string(accountsContent), "code,name,account_type")
	contactsContent, err := os.ReadFile(filepath.Join(outputDir, "bundle", "contacts.csv"))
	require.NoError(t, err)
	require.Contains(t, string(contactsContent), "code,name,reg_code,email")

	validation, err := ValidateBundle(&ValidateBundleRequest{
		Files:          report.BundleFiles(),
		ProviderPreset: MigrationProviderPresetSmartAccounts,
	})
	require.NoError(t, err)
	require.True(t, validation.Summary.Ready, "issues: %#v", validation.Issues)

	var manifest SmartAccountsSnapshotReport
	manifestContent, err := os.ReadFile(report.ManifestPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(manifestContent, &manifest))
	require.Equal(t, report.SnapshotHash, manifest.SnapshotHash)
}

func TestPrepareSmartAccountsSnapshotHashIgnoresGeneratedAtAndOutputDir(t *testing.T) {
	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))

	first, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir:       sourceDir,
		OutputDir:       filepath.Join(t.TempDir(), "first"),
		SourceCompanyID: "14369460",
		GeneratedAt:     time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	second, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir:       sourceDir,
		OutputDir:       filepath.Join(t.TempDir(), "second"),
		SourceCompanyID: "14369460",
		GeneratedAt:     time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	require.Equal(t, first.SnapshotHash, second.SnapshotHash)
	require.NotEqual(t, first.ValidationCommand, second.ValidationCommand)
}

func TestPrepareSmartAccountsSnapshotWarnsWhenUsingGitWorktree(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(repoDir, ".git"), 0o755))
	sourceDir := filepath.Join(repoDir, "private", "smartaccounts-export")
	outputDir := filepath.Join(repoDir, "private", "smartaccounts-prepared")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))

	report, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir: sourceDir,
		OutputDir: outputDir,
	})
	require.NoError(t, err)
	require.Len(t, report.Warnings, 1)
	require.Contains(t, report.Warnings[0], "inside Git worktree")
	require.Contains(t, report.Warnings[0], "separate private repository")
}

func TestPrepareSmartAccountsSnapshotRejectsUnclassifiableDirectory(t *testing.T) {
	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "unknown.csv"), []byte("only_one_column\nvalue\n"), 0o644))

	_, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir: sourceDir,
		OutputDir: filepath.Join(t.TempDir(), "prepared"),
	})
	require.ErrorContains(t, err, "no supported SmartAccounts CSV or XML files found")
}
