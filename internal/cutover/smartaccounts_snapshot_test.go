package cutover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestPrepareSmartAccountsSnapshotCanonicalizesAndValidatesBundle(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "prepared")
	copySmartAccountsSnapshotFixture(t, sourceDir, "chart_of_accounts.csv")
	copySmartAccountsSnapshotFixture(t, sourceDir, "clients.csv")
	copySmartAccountsSnapshotFixture(t, sourceDir, "customers.csv")
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, ".ignored.csv"), []byte("client_no,client_name\nC-ignored,Ignored OU\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "notes.txt"), []byte("not part of migration"), 0o644))

	report, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir:         sourceDir,
		OutputDir:         outputDir,
		SourceCompanyID:   "12345678",
		SourceCompanyName: "Example Export OU",
		CutoverDate:       "2026-01-01",
		GeneratedAt:       time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	require.Equal(t, MigrationProviderPresetSmartAccounts, report.Provider)
	require.Len(t, report.PreparedFiles, 3)
	require.Len(t, report.UnsupportedFiles, 1)
	require.Equal(t, int64(len("not part of migration")), report.UnsupportedFiles[0].SizeBytes)
	require.NotEmpty(t, report.UnsupportedFiles[0].SourceSHA256)
	require.NotEmpty(t, report.SnapshotHash)
	require.FileExists(t, filepath.Join(outputDir, "manifest.json"))
	require.FileExists(t, filepath.Join(outputDir, "bundle", "accounts.csv"))
	require.FileExists(t, filepath.Join(outputDir, "bundle", "contacts.csv"))
	require.Contains(t, report.ValidationCommand, "--provider-preset smartaccounts")
	require.Contains(t, report.ValidationCommand, "--manifest")
	require.Contains(t, report.ValidationCommand, filepath.Join(outputDir, "manifest.json"))

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
	require.Equal(t, 1, manifest.PreparedFiles[1].OutputRowStart)
	require.GreaterOrEqual(t, manifest.PreparedFiles[1].OutputRowEnd, manifest.PreparedFiles[1].OutputRowStart)
}

func TestSmartAccountsSnapshotSkipsGridPreambleAndCanonicalizesEstonianHeaders(t *testing.T) {
	content := strings.Join([]string{
		"Kontoplaan: Example Export OU,,,,",
		",,,,",
		"Koostatud,27.06.2026,,,",
		"Kood,Kirjeldus,Tüüp,Bilansi või kasumiaruande rida,Rahavoogude aruande rida",
		"1000,Cash,Aktiva,,",
	}, "\n")

	headers, rows, err := readSmartAccountsCSVWithHint(content, KindAccounts, true)
	require.NoError(t, err)
	require.Equal(t, []string{"Kood", "Kirjeldus", "Tüüp", "Bilansi või kasumiaruande rida", "Rahavoogude aruande rida"}, headers)
	require.Len(t, rows, 1)

	source, reason, err := classifySmartAccountsCSV("accounts.csv", "accounts.csv", "hash", content)
	require.NoError(t, err)
	require.Empty(t, reason)
	require.Equal(t, KindAccounts, source.kind)
	require.Equal(t, []string{"code", "name", "account_type", "bilansi_või_kasumiaruande_rida", "rahavoogude_aruande_rida"}, source.headers)

	canonical, err := CanonicalizeBundleFileCSV(BundleFile{Kind: KindAccounts, FileName: "accounts.csv", CSVContent: content}, MigrationProviderPresetSmartAccounts)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(canonical.CSVContent, "code,name,account_type"))
	require.Contains(t, canonical.CSVContent, "1000,Cash,ASSET")
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

	sourceDirWithCutover := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sourceDirWithCutover, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))
	_, err = PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{SourceDir: sourceDirWithCutover, OutputDir: filepath.Join(t.TempDir(), "out"), CutoverDate: "2026/01/01"})
	require.ErrorContains(t, err, "cutover date must be YYYY-MM-DD")

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
	require.Contains(t, report.Warnings, "multiple e-invoice XML files were prepared as separate bundle files; keep the manifest-driven flow so every XML file is validated and planned together")
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
		SourceCompanyID: "12345678",
		GeneratedAt:     time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	second, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir:       sourceDir,
		OutputDir:       filepath.Join(t.TempDir(), "second"),
		SourceCompanyID: "12345678",
		GeneratedAt:     time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)

	require.Equal(t, first.SnapshotHash, second.SnapshotHash)
	require.NotEqual(t, first.ValidationCommand, second.ValidationCommand)
}

func TestPrepareSmartAccountsSnapshotWarnsWhenUsingGitWorktree(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(repoDir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
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

func TestPrepareSmartAccountsSnapshotRejectsPublicOpenAccountingWorktree(t *testing.T) {
	repoDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(repoDir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module github.com/HMB-research/open-accounting\n"), 0o644))
	sourceDir := filepath.Join(repoDir, "private", "smartaccounts-export")
	outputDir := filepath.Join(repoDir, "private", "smartaccounts-prepared")
	require.NoError(t, os.MkdirAll(sourceDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "clients.csv"), []byte("client_no,client_name\nC-1,Example OU\n"), 0o644))

	_, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir: sourceDir,
		OutputDir: outputDir,
	})
	require.ErrorContains(t, err, "must not be inside public Open Accounting Git worktree")
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
	kind, ok = smartAccountsFilenameKind("bankpayments.csv")
	require.True(t, ok)
	require.Equal(t, KindPayments, kind)
	kind, ok = smartAccountsFilenameKind("articles.csv")
	require.True(t, ok)
	require.Equal(t, KindProducts, kind)
	_, ok = smartAccountsFilenameKind("unmapped-file.csv")
	require.False(t, ok)

	require.Equal(t, "", smartAccountsValidationCommand("/tmp/out", nil))
	command := smartAccountsValidationCommand("/tmp/path with spaces", []SmartAccountsSnapshotPreparedFile{
		{Kind: KindContacts, OutputPath: "bundle/contacts.csv"},
		{Kind: KindContacts, OutputPath: "bundle/contacts-2.csv"},
		{Kind: FileKind("custom"), OutputPath: "bundle/custom.csv"},
	})
	require.NotContains(t, command, "--contacts ")
	require.Contains(t, command, "--manifest")
	require.Contains(t, command, "'/tmp/path with spaces/manifest.json'")
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
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	nestedFile := filepath.Join(repoDir, "nested", "file.csv")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedFile), 0o755))
	require.NoError(t, os.WriteFile(nestedFile, []byte("x"), 0o644))
	root, ok = nearestGitWorktreeRoot(nestedFile)
	require.True(t, ok)
	require.Equal(t, repoDir, root)
	linkedWorktree := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(linkedWorktree, ".git"), []byte("gitdir: /private/common.git/worktrees/pilot\n"), 0o644))
	root, ok = nearestGitWorktreeRoot(linkedWorktree)
	require.True(t, ok)
	require.Equal(t, linkedWorktree, root)
	warnings := smartAccountsGitWorktreeWarnings(filepath.Join(repoDir, "a"), filepath.Join(repoDir, "b"))
	require.Len(t, warnings, 1)

	configRepoDir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(configRepoDir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configRepoDir, ".git", "config"), []byte("[remote \"origin\"]\n\turl = https://github.com/HMB-research/open-accounting.git\n"), 0o644))
	require.True(t, isOpenAccountingPublicWorktree(configRepoDir))
	require.NoError(t, os.WriteFile(filepath.Join(configRepoDir, ".git", "config"), []byte("[remote \"origin\"]\n\turl = https://github.com/HMB-research/open-accounting-smartaccounts-migration-data.git\n"), 0o644))
	require.False(t, isOpenAccountingPublicWorktree(configRepoDir))
}

func TestSmartAccountsSnapshotRequiresCutoverDateForOpeningBalances(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := filepath.Join(t.TempDir(), "prepared")
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "opening-balances.csv"), []byte("account_code,debit,credit\n1000,100,0\n3000,0,100\n"), 0o644))

	report, err := PrepareSmartAccountsSnapshot(SmartAccountsSnapshotOptions{
		SourceDir:   sourceDir,
		OutputDir:   outputDir,
		GeneratedAt: time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Contains(t, report.Warnings, "cutover date is required before executing opening balance or historical journal imports")
}

func TestSmartAccountsSnapshotCSVHeaderAndCanonicalizeEdges(t *testing.T) {
	_, _, err := readSmartAccountsCSV(",,\n,,\n")
	require.ErrorContains(t, err, "read header")

	records := make([][]string, 21)
	for i := range records {
		records[i] = []string{""}
	}
	require.Equal(t, -1, smartAccountsCSVHeaderIndex(records, KindContacts, true))

	_, err = CanonicalizeBundleFileCSV(BundleFile{
		Kind:       KindAccounts,
		FileName:   "accounts.csv",
		CSVContent: `"`,
	}, MigrationProviderPresetSmartAccounts)
	require.ErrorContains(t, err, "read row")
}

func TestSmartAccountsSnapshotNormalizesLocalizedValuesAndDerivedTypes(t *testing.T) {
	headers, rows, transformations := normalizeSmartAccountsSourceRows(KindContacts, "hankijad.csv", []string{"code", "name"}, [][]string{{"S-1", "Supplier OU"}})
	require.Equal(t, []string{"code", "name", "contact_type"}, headers)
	require.Equal(t, "SUPPLIER", rows[0][2])
	require.Contains(t, transformations, "derived contact_type from SmartAccounts source export")

	headers, rows, transformations = normalizeSmartAccountsSourceRows(KindInvoices, "ostuarved.csv", []string{"invoice_number", "amount"}, [][]string{{"P-1", "1,00"}})
	require.Equal(t, []string{"invoice_number", "amount", "invoice_type"}, headers)
	require.Equal(t, []string{"P-1", "1.00", "PURCHASE"}, rows[0])
	require.Contains(t, transformations, "derived invoice_type from SmartAccounts source export")

	headers, rows, _ = normalizeSmartAccountsSourceRows(KindInvoices, "müügiarved.csv", []string{"invoice_number"}, [][]string{{"S-1"}})
	require.Equal(t, []string{"invoice_number", "invoice_type"}, headers)
	require.Equal(t, "SALES", rows[0][1])

	headers, rows, transformations = normalizeSmartAccountsSourceRows(KindPayments, "payments.csv", []string{"amount", "payment_date"}, [][]string{
		{"-1 234,50", "27.06.2026"},
		{"2,50", ""},
	})
	require.Equal(t, []string{"amount", "payment_date", "payment_type"}, headers)
	require.Equal(t, []string{"1234.50", "2026-06-27", "MADE"}, rows[0])
	require.Equal(t, []string{"2.50", "", "RECEIVED"}, rows[1])
	require.Contains(t, transformations, "derived payment_type from SmartAccounts amount sign")
	require.Contains(t, transformations, "normalized SmartAccounts localized dates, decimals, and enum values")

	shortRow := []string{"27.06.2026"}
	require.True(t, normalizeSmartAccountsRowValues(KindInvoices, []string{"issue_date", "due_date"}, shortRow))
	require.Equal(t, []string{"2026-06-27"}, shortRow)

	productRow := []string{"", "teenus"}
	require.True(t, normalizeSmartAccountsRowValues(KindProducts, []string{"sales_price", "product_type"}, productRow))
	require.Equal(t, []string{"0", "SERVICE"}, productRow)

	require.Equal(t, "", normalizeSmartAccountsDateValue(" "))
	require.Equal(t, "2026/01/02", normalizeSmartAccountsDateValue("2026/01/02"))
	require.Equal(t, "", normalizeSmartAccountsDecimalValue(" "))
	require.Equal(t, "0", normalizeSmartAccountsDecimalValue("KM vaba"))
	require.Equal(t, "", normalizeSmartAccountsDecimalValue("jah"))
	require.Equal(t, "1234.50", normalizeSmartAccountsDecimalValue("1 234,50%"))

	require.Equal(t, "LIABILITY", normalizeSmartAccountsAccountType("Kohustus", "2000"))
	require.Equal(t, "EQUITY", normalizeSmartAccountsAccountType("Omakapital", ""))
	require.Equal(t, "REVENUE", normalizeSmartAccountsAccountType("Tulu", ""))
	require.Equal(t, "EXPENSE", normalizeSmartAccountsAccountType("Kulu", ""))
	require.Equal(t, "REVENUE", normalizeSmartAccountsAccountType("", "4000"))
	require.Equal(t, "fallback", normalizeSmartAccountsAccountType("fallback", ""))
	require.Equal(t, "ASSET", inferSmartAccountsAccountTypeFromCode("1000", "fallback"))
	require.Equal(t, "LIABILITY", inferSmartAccountsAccountTypeFromCode("2000", "fallback"))
	require.Equal(t, "EQUITY", inferSmartAccountsAccountTypeFromCode("3000", "fallback"))
	require.Equal(t, "REVENUE", inferSmartAccountsAccountTypeFromCode("4000", "fallback"))
	require.Equal(t, "EXPENSE", inferSmartAccountsAccountTypeFromCode("5000", "fallback"))
	require.Equal(t, "fallback", inferSmartAccountsAccountTypeFromCode("0000", "fallback"))

	require.Equal(t, "GOODS", normalizeSmartAccountsProductType("kaup"))
	require.Equal(t, "custom", normalizeSmartAccountsProductType("custom"))
}

func TestSmartAccountsSnapshotContactDedupeBranches(t *testing.T) {
	headers := []string{"code", "name", "reg_code", "vat_number", "contact_type", "email"}
	rows := [][]string{
		{"C-0", "", "", "", "CUSTOMER", ""},
		{"C-1", "Example OU", "12345678", "", "CUSTOMER", ""},
		{"S-1", "Example OU", "", "", "SUPPLIER", "supplier@example.com"},
	}

	normalized, changed := dedupeSmartAccountsContactRows(headers, rows)
	require.True(t, changed)
	require.Len(t, normalized, 2)
	require.Equal(t, "BOTH", normalized[1][4])
	require.Equal(t, "supplier@example.com", normalized[1][5])

	mergedRows, transformations := normalizeSmartAccountsMergedRows(KindContacts, headers, rows)
	require.Len(t, mergedRows, 2)
	require.Contains(t, transformations, "merged duplicate SmartAccounts client/vendor contacts by registry, VAT, or name")

	mergeSmartAccountsContactRow([]string{"code", "name"}, []string{"C-2"}, []string{"", "Filled"})
	require.Equal(t, "", valueAtHeader([]string{"name"}, []string{"Example OU"}, "missing"))
	require.Equal(t, []string{"a"}, uniqueStrings([]string{"", "a", "a"}))
}

func TestSmartAccountsSnapshotNormalizesDistinctDuplicatePaymentNumbers(t *testing.T) {
	headers := []string{
		"payment_number",
		"payment_type",
		"payment_date",
		"amount",
		"currency",
		"reference",
		"invoice_number",
		"allocation_amount",
	}
	rows := [][]string{
		{"PAY-7", "RECEIVED", "2026-06-01", "100", "EUR", "REF-A", "INV-1", "100"},
		{"PAY-7", "RECEIVED", "2026-06-02", "50", "EUR", "REF-B", "INV-2", "50"},
	}

	normalized, changed := normalizeDistinctSmartAccountsPaymentNumbers(headers, rows)
	require.True(t, changed)
	require.Equal(t, "PAY-7~SA01", normalized[0][0])
	require.Equal(t, "PAY-7~SA02", normalized[1][0])

	merged, transformations := normalizeSmartAccountsMergedRows(KindPayments, headers, [][]string{
		{"PAY-7", "RECEIVED", "2026-06-01", "100", "EUR", "REF-A", "INV-1", "100"},
		{"PAY-7", "RECEIVED", "2026-06-02", "50", "EUR", "REF-B", "INV-2", "50"},
	})
	require.Equal(t, normalized, merged)
	require.Contains(t, transformations, "normalized distinct SmartAccounts duplicate payment_number values")
}

func TestSmartAccountsSnapshotKeepsGroupedPaymentDuplicateNumbersBlocked(t *testing.T) {
	headers := []string{
		"payment_number",
		"payment_type",
		"payment_date",
		"amount",
		"currency",
		"reference",
		"invoice_number",
		"allocation_amount",
	}
	rows := [][]string{
		{"PAY-7", "RECEIVED", "2026-06-01", "150", "EUR", "REF-A", "INV-1", "100"},
		{"PAY-7", "RECEIVED", "2026-06-01", "150", "EUR", "REF-A", "INV-2", "50"},
	}

	normalized, changed := normalizeDistinctSmartAccountsPaymentNumbers(headers, rows)
	require.False(t, changed)
	require.Equal(t, "PAY-7", normalized[0][0])
	require.Equal(t, "PAY-7", normalized[1][0])
}

func TestSmartAccountsSnapshotPaymentNumberNormalizationSkipsIncompleteRows(t *testing.T) {
	rows := [][]string{{"PAY-7"}, {"PAY-7"}}
	normalized, changed := normalizeDistinctSmartAccountsPaymentNumbers([]string{"reference"}, rows)
	require.False(t, changed)
	require.Equal(t, rows, normalized)

	headers := []string{"payment_number", "payment_date"}
	rows = [][]string{
		{"PAY-7", "2026-06-01"},
		{"", "2026-06-02"},
		{"   ", "2026-06-03"},
		{"PAY-8"},
		{},
	}
	normalized, changed = normalizeDistinctSmartAccountsPaymentNumbers(headers, rows)
	require.False(t, changed)
	require.Equal(t, rows, normalized)
}

func TestSmartAccountsPaymentNumberSuffixRespectsDatabaseLimit(t *testing.T) {
	number := strings.Repeat("A", 80)
	got := suffixedSmartAccountsPaymentNumber(number, 12, 2)
	require.Len(t, []rune(got), smartAccountsPaymentNumberMaxLength)
	require.True(t, strings.HasSuffix(got, "~SA12"))

	nonASCII := strings.Repeat("Õ", 80)
	got = suffixedSmartAccountsPaymentNumber(nonASCII, 1, 2)
	require.Len(t, []rune(got), smartAccountsPaymentNumberMaxLength)
	require.True(t, strings.HasSuffix(got, "~SA01"))
	require.True(t, utf8.ValidString(got))

	require.Equal(t, "", firstRunes("PAY-7", 0))
	require.Equal(t, "~SA"+strings.Repeat("0", smartAccountsPaymentNumberMaxLength-3), suffixedSmartAccountsPaymentNumber("PAY-7", 1, smartAccountsPaymentNumberMaxLength-2))
}

func copySmartAccountsSnapshotFixture(t *testing.T, destDir, name string) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "smartaccounts", name))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(destDir, name), content, 0o644))
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
