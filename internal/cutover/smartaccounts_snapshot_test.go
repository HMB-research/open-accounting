package cutover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPrepareSmartAccountsSnapshotCanonicalizesAndValidatesBundle(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "prepared")
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "chart_of_accounts.csv"), []byte("account_no;account_title;classification\n1000;Cash;ASSET\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no;client_name;registration_no;email_address\nC-1;Example OU;12345678;info@example.test\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "customers.csv"), []byte("client_no;client_name\nC-2;Second OU\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".ignored.csv"), []byte("client_no,client_name\nC-ignored,Ignored OU\n"), 0o644))
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
	require.Len(t, report.PreparedFiles, 3)
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
	require.Contains(t, string(contactsContent), "C-2")

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

func TestPrepareSmartAccountsSnapshotInputAndWalkErrors(t *testing.T) {
	_, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{})
	require.ErrorContains(t, err, "source dir is required")

	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: t.TempDir()})
	require.ErrorContains(t, err, "output dir is required")

	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: filepath.Join(t.TempDir(), "missing"), OutputDir: filepath.Join(t.TempDir(), "out")})
	require.ErrorContains(t, err, "stat source dir")

	sourceFile := filepath.Join(t.TempDir(), "source.csv")
	require.NoError(t, os.WriteFile(sourceFile, []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: sourceFile, OutputDir: filepath.Join(t.TempDir(), "out")})
	require.ErrorContains(t, err, "source dir must be a directory")

	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))
	outputFile := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(outputFile, []byte("file"), 0o644))
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: sourceDir, OutputDir: filepath.Join(outputFile, "out")})
	require.ErrorContains(t, err, "create output dir")

	hiddenOnlyDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(hiddenOnlyDir, ".hidden"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenOnlyDir, ".hidden", "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: hiddenOnlyDir, OutputDir: filepath.Join(t.TempDir(), "out")})
	require.ErrorContains(t, err, "no supported SmartAccounts CSV or XML files")

	brokenLinkDir := t.TempDir()
	require.NoError(t, os.Symlink(filepath.Join(brokenLinkDir, "missing.csv"), filepath.Join(brokenLinkDir, "clients.csv")))
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: brokenLinkDir, OutputDir: filepath.Join(t.TempDir(), "out")})
	require.ErrorContains(t, err, "read clients.csv")

	badCSVDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(badCSVDir, "clients.csv"), []byte("\"unterminated"), 0o644))
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: badCSVDir, OutputDir: filepath.Join(t.TempDir(), "out")})
	require.ErrorContains(t, err, "parse clients.csv")

	unreadableDir := t.TempDir()
	blockedDir := filepath.Join(unreadableDir, "blocked")
	require.NoError(t, os.Mkdir(blockedDir, 0o755))
	require.NoError(t, os.Chmod(blockedDir, 0))
	defer func() { _ = os.Chmod(blockedDir, 0o755) }()
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: unreadableDir, OutputDir: filepath.Join(t.TempDir(), "out")})
	require.Error(t, err)
}

func TestPrepareSmartAccountsSnapshotHandlesXMLAndUnsupportedXML(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "prepared")
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "bad.xml"), []byte("<not_invoice/>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "empty.xml"), []byte("  "), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "supplier-a.xml"), []byte("<E_Invoice></E_Invoice>"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "supplier-b.xml"), []byte("<Invoice></Invoice>"), 0o644))

	report, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir:   sourceDir,
		OutputDir:   outputDir,
		GeneratedAt: time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Len(t, report.UnsupportedFiles, 2)
	require.Contains(t, report.Warnings, "multiple e-invoice XML files were prepared; pass additional XML files through the API bundle request or validate them one at a time with the CLI")
	require.FileExists(t, filepath.Join(outputDir, "bundle", "e_invoices.xml"))
	require.FileExists(t, filepath.Join(outputDir, "bundle", "e_invoices-2.xml"))
	require.Contains(t, report.SnapshotHash, "")
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

func TestSmartAccountsSnapshotHelperEdges(t *testing.T) {
	require.Nil(t, (*SmartAccountsSnapshotReport)(nil).BundleFiles())
	require.Nil(t, (&SmartAccountsSnapshotReport{}).BundleFiles())

	_, _, err := readSmartAccountsCSV("")
	require.ErrorContains(t, err, "csv content is empty")

	_, rows, err := readSmartAccountsCSV("a,b\n\n1,2\n\"unterminated")
	require.ErrorContains(t, err, "read row")
	require.Nil(t, rows)

	headers, rows, err := readSmartAccountsCSV("\ufeffclient_no,client_name\n,\nC-1,Example OU\n")
	require.NoError(t, err)
	require.Equal(t, []string{"client_no", "client_name"}, headers)
	require.Len(t, rows, 1)

	kind, score, required := scoreSmartAccountsCSVHeaders([]string{"unmapped"}, KindContacts, true)
	require.Equal(t, KindContacts, kind)
	require.Greater(t, score, 0)
	require.Zero(t, required)

	require.True(t, valueInAliasMap(map[string]string{"source": "canonical"}, "canonical"))
	require.False(t, valueInAliasMap(map[string]string{"source": "canonical"}, "missing"))

	kind, ok := smartAccountsFilenameKind("recurring_invoice_export.csv")
	require.True(t, ok)
	require.Equal(t, KindRecurringInvoices, kind)
	_, ok = smartAccountsFilenameKind("unmapped-file.csv")
	require.False(t, ok)

	require.Equal(t, "", smartAccountsValidationCommand("/tmp/out", nil))
	command := smartAccountsValidationCommand("/tmp/path with spaces", []SmartAccountsSnapshotPreparedFile{
		{Kind: KindContacts, OutputPath: "bundle/contacts.csv"},
		{Kind: KindContacts, OutputPath: "bundle/contacts-2.csv"},
		{Kind: FileKind("custom"), OutputPath: "bundle/custom.csv"},
	})
	require.Equal(t, 1, strings.Count(command, "--contacts "))
	require.Contains(t, command, "'/tmp/path with spaces/bundle/")
	require.NotContains(t, command, "custom.csv")

	require.Equal(t, "''", shellQuote(""))
	require.Equal(t, "/tmp/plain-path.csv", shellQuote("/tmp/plain-path.csv"))
	require.Equal(t, "'/tmp/has spaces/it'\\''s.csv'", shellQuote("/tmp/has spaces/it's.csv"))
	require.True(t, isBlankCSVRecord([]string{" ", ""}))
	require.False(t, isBlankCSVRecord([]string{"value"}))

	root, ok := nearestGitWorktreeRoot("")
	require.False(t, ok)
	require.Empty(t, root)
	root, ok = nearestGitWorktreeRoot(t.TempDir())
	require.False(t, ok)
	require.Empty(t, root)
	repoDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(repoDir, ".git"), 0o755))
	nestedFile := filepath.Join(repoDir, "nested", "file.csv")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedFile), 0o755))
	require.NoError(t, os.WriteFile(nestedFile, []byte("x"), 0o644))
	root, ok = nearestGitWorktreeRoot(nestedFile)
	require.True(t, ok)
	require.Equal(t, repoDir, root)
	warnings := smartAccountsGitWorktreeWarnings(filepath.Join(repoDir, "a"), filepath.Join(repoDir, "b"))
	require.Len(t, warnings, 1)
}

func TestSmartAccountsSnapshotDirectWriteErrors(t *testing.T) {
	report := &SmartAccountsSnapshotReport{}
	err := writeSmartAccountsCSVBundles(t.TempDir(), []smartAccountsCSVSource{{
		kind:          KindContacts,
		relSourcePath: "contacts.csv",
		sourceHash:    "hash",
		headers:       []string{"code", "name"},
		rows:          [][]string{{"C-1", "Example OU"}},
	}}, report)
	require.ErrorContains(t, err, "write")

	report = &SmartAccountsSnapshotReport{}
	err = writeSmartAccountsXMLBundles(t.TempDir(), []smartAccountsXMLSource{{
		kind:          KindEInvoices,
		relSourcePath: "invoice.xml",
		sourceHash:    "hash",
		content:       "<E_Invoice></E_Invoice>",
	}}, report)
	require.ErrorContains(t, err, "write")

	err = writeSmartAccountsManifest(filepath.Join(t.TempDir(), "missing", "manifest.json"), &SmartAccountsSnapshotReport{})
	require.ErrorContains(t, err, "write manifest")
}

func TestPrepareSmartAccountsSnapshotWriteErrors(t *testing.T) {
	sourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))
	outputDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(outputDir, "bundle"), 0o755))
	require.NoError(t, os.Chmod(filepath.Join(outputDir, "bundle"), 0o500))
	defer func() { _ = os.Chmod(filepath.Join(outputDir, "bundle"), 0o755) }()
	_, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: sourceDir, OutputDir: outputDir})
	require.ErrorContains(t, err, "write")

	xmlSourceDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(xmlSourceDir, "invoice.xml"), []byte("<Invoice></Invoice>"), 0o644))
	xmlOutputDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(xmlOutputDir, "bundle"), 0o755))
	require.NoError(t, os.Chmod(filepath.Join(xmlOutputDir, "bundle"), 0o500))
	defer func() { _ = os.Chmod(filepath.Join(xmlOutputDir, "bundle"), 0o755) }()
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: xmlSourceDir, OutputDir: xmlOutputDir})
	require.ErrorContains(t, err, "write")

	manifestSourceDir := t.TempDir()
	manifestOutputDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(manifestSourceDir, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(manifestOutputDir, "bundle"), 0o755))
	require.NoError(t, os.Chmod(manifestOutputDir, 0o500))
	defer func() { _ = os.Chmod(manifestOutputDir, 0o755) }()
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: manifestSourceDir, OutputDir: manifestOutputDir})
	require.ErrorContains(t, err, "write manifest")
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

func TestClassifySmartAccountsXML(t *testing.T) {
	_, reason := classifySmartAccountsXML("empty.xml", "empty.xml", "hash", "")
	require.Equal(t, "xml file is empty", reason)

	_, reason = classifySmartAccountsXML("other.xml", "other.xml", "hash", "<other/>")
	require.Equal(t, "xml does not look like an Estonian e-invoice payload", reason)

	source, reason := classifySmartAccountsXML("invoice.xml", "invoice.xml", "hash", "<Invoice></Invoice>")
	require.Empty(t, reason)
	require.Equal(t, KindEInvoices, source.kind)
}

func TestClassifySmartAccountsCSVEdges(t *testing.T) {
	_, reason, err := classifySmartAccountsCSV("unknown.csv", "unknown.csv", "hash", "only_one_column\nvalue\n")
	require.NoError(t, err)
	require.Equal(t, "could not classify CSV headers as a supported SmartAccounts migration file", reason)

	source, reason, err := classifySmartAccountsCSV("accounts.csv", "accounts.csv", "hash", "account_no,account_title,classification\n1000,Cash,ASSET\n")
	require.NoError(t, err)
	require.Empty(t, reason)
	require.Equal(t, KindAccounts, source.kind)
	require.Equal(t, "filename", source.classification)

	_, _, err = classifySmartAccountsCSV("clients.csv", "clients.csv", "hash", "\"unterminated")
	require.ErrorContains(t, err, "parse clients.csv")
}

func TestWriteCSVContent(t *testing.T) {
	content := writeCSVContent([]string{"a", "b"}, [][]string{{"1", "2"}})
	require.True(t, strings.HasPrefix(content, "a,b\n1,2\n"))
}
