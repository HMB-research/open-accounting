package cutover

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const SmartAccountsSnapshotAdapterVersion = "smartaccounts-snapshot-v2"

var smartAccountsGeneralLedgerSourceNamespace = uuid.MustParse("4274fdcb-50ca-56d9-80f3-4cb25bf2aa4a")

type SmartAccountsSnapshotOptions struct {
	SourceDir         string
	OutputDir         string
	SourceCompanyID   string
	SourceCompanyName string
	CutoverDate       string
	GeneratedAt       time.Time
}

type SmartAccountsSnapshotReport struct {
	Provider          MigrationProviderPreset             `json:"provider"`
	AdapterVersion    string                              `json:"adapter_version"`
	GeneratedAt       string                              `json:"generated_at"`
	SourceDir         string                              `json:"source_dir"`
	OutputDir         string                              `json:"output_dir"`
	SourceCompanyID   string                              `json:"source_company_id,omitempty"`
	SourceCompanyName string                              `json:"source_company_name,omitempty"`
	CutoverDate       string                              `json:"cutover_date,omitempty"`
	SnapshotHash      string                              `json:"snapshot_hash"`
	ManifestPath      string                              `json:"manifest_path"`
	ValidationCommand string                              `json:"validation_command,omitempty"`
	PreparedFiles     []SmartAccountsSnapshotPreparedFile `json:"prepared_files,omitempty"`
	UnsupportedFiles  []SmartAccountsSnapshotUnsupported  `json:"unsupported_files,omitempty"`
	Warnings          []string                            `json:"warnings,omitempty"`
	bundleFiles       []BundleFile                        `json:"-"`
}

type SmartAccountsSnapshotPreparedFile struct {
	Kind              FileKind `json:"kind"`
	SourcePath        string   `json:"source_path"`
	OutputPath        string   `json:"output_path"`
	SourceSHA256      string   `json:"source_sha256"`
	OutputSHA256      string   `json:"output_sha256"`
	Rows              int      `json:"rows"`
	OutputRowStart    int      `json:"output_row_start,omitempty"`
	OutputRowEnd      int      `json:"output_row_end,omitempty"`
	Transformations   []string `json:"transformations,omitempty"`
	Classification    string   `json:"classification"`
	ValidationCLIFlag string   `json:"validation_cli_flag,omitempty"`
}

type SmartAccountsSnapshotUnsupported struct {
	SourcePath   string `json:"source_path"`
	Reason       string `json:"reason"`
	SourceSHA256 string `json:"source_sha256"`
	SizeBytes    int64  `json:"size_bytes"`
}

type smartAccountsCSVSource struct {
	kind            FileKind
	relSourcePath   string
	sourceHash      string
	headers         []string
	rows            [][]string
	classification  string
	transformations []string
	derivedSources  []smartAccountsCSVSource
}

type smartAccountsXMLSource struct {
	kind           FileKind
	relSourcePath  string
	sourceHash     string
	content        string
	classification string
}

func PrepareSmartAccountsSnapshot(opts SmartAccountsSnapshotOptions) (*SmartAccountsSnapshotReport, error) {
	sourceDir := strings.TrimSpace(opts.SourceDir)
	if sourceDir == "" {
		return nil, fmt.Errorf("source dir is required")
	}
	outputDir := strings.TrimSpace(opts.OutputDir)
	if outputDir == "" {
		return nil, fmt.Errorf("output dir is required")
	}
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("stat source dir: %w", err)
	}
	if !sourceInfo.IsDir() {
		return nil, fmt.Errorf("source dir must be a directory")
	}
	if err := rejectSmartAccountsPublicWorktreePaths(sourceDir, outputDir); err != nil {
		return nil, err
	}
	if opts.GeneratedAt.IsZero() {
		opts.GeneratedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "bundle"), 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	report := &SmartAccountsSnapshotReport{
		Provider:          MigrationProviderPresetSmartAccounts,
		AdapterVersion:    SmartAccountsSnapshotAdapterVersion,
		GeneratedAt:       opts.GeneratedAt.UTC().Format(time.RFC3339),
		SourceDir:         sourceDir,
		OutputDir:         outputDir,
		SourceCompanyID:   strings.TrimSpace(opts.SourceCompanyID),
		SourceCompanyName: strings.TrimSpace(opts.SourceCompanyName),
		CutoverDate:       strings.TrimSpace(opts.CutoverDate),
	}
	if err := validateSmartAccountsCutoverDate(report.CutoverDate); err != nil {
		return nil, err
	}
	report.Warnings = append(report.Warnings, smartAccountsGitWorktreeWarnings(sourceDir, outputDir)...)

	csvSources := make([]smartAccountsCSVSource, 0)
	xmlSources := make([]smartAccountsXMLSource, 0)
	err = filepath.WalkDir(sourceDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != sourceDir && strings.HasPrefix(filepath.Base(path), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, _ := filepath.Rel(sourceDir, path)
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
		// #nosec G304,G122 -- operator-selected local export path; contents are hashed into the migration manifest before use.
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", relPath, err)
		}
		sourceHash := sha256Hex(content)

		switch strings.ToLower(filepath.Ext(path)) {
		case ".csv":
			source, reason, err := classifySmartAccountsCSV(path, relPath, sourceHash, string(content))
			if err != nil {
				return err
			}
			if reason != "" {
				report.UnsupportedFiles = append(report.UnsupportedFiles, smartAccountsUnsupportedFile(relPath, reason, sourceHash, content))
				return nil
			}
			csvSources = append(csvSources, source)
			csvSources = append(csvSources, source.derivedSources...)
		case ".xml":
			source, reason := classifySmartAccountsXML(path, relPath, sourceHash, string(content))
			if reason != "" {
				report.UnsupportedFiles = append(report.UnsupportedFiles, smartAccountsUnsupportedFile(relPath, reason, sourceHash, content))
				return nil
			}
			xmlSources = append(xmlSources, source)
		default:
			report.UnsupportedFiles = append(report.UnsupportedFiles, smartAccountsUnsupportedFile(relPath, "unsupported file extension; expected SmartAccounts CSV export or Estonian e-invoice XML", sourceHash, content))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(csvSources) == 0 && len(xmlSources) == 0 {
		return nil, fmt.Errorf("no supported SmartAccounts CSV or XML files found in %s", sourceDir)
	}

	sort.Slice(csvSources, func(i, j int) bool {
		if csvSources[i].kind != csvSources[j].kind {
			return csvSources[i].kind < csvSources[j].kind
		}
		return csvSources[i].relSourcePath < csvSources[j].relSourcePath
	})
	sort.Slice(xmlSources, func(i, j int) bool {
		return xmlSources[i].relSourcePath < xmlSources[j].relSourcePath
	})

	if err := writeSmartAccountsCSVBundles(outputDir, csvSources, report); err != nil {
		return nil, err
	}
	if err := writeSmartAccountsXMLBundles(outputDir, xmlSources, report); err != nil {
		return nil, err
	}
	if report.CutoverDate == "" && smartAccountsSnapshotNeedsCutoverDate(csvSources) {
		report.Warnings = append(report.Warnings, "cutover date is required before executing opening balance or historical journal imports")
	}

	report.SnapshotHash = smartAccountsSnapshotHash(report, csvSources, xmlSources)
	report.ValidationCommand = smartAccountsValidationCommand(report.OutputDir, report.PreparedFiles)
	manifestPath := filepath.Join(outputDir, "manifest.json")
	report.ManifestPath = manifestPath
	if err := writeSmartAccountsManifest(manifestPath, report); err != nil {
		return nil, err
	}
	return report, nil
}

func (r *SmartAccountsSnapshotReport) BundleFiles() []BundleFile {
	if r == nil || len(r.bundleFiles) == 0 {
		return nil
	}
	files := make([]BundleFile, len(r.bundleFiles))
	copy(files, r.bundleFiles)
	return files
}

func classifySmartAccountsCSV(path, relPath, sourceHash, content string) (smartAccountsCSVSource, string, error) {
	hintedKind, hasHint := smartAccountsFilenameKind(path)
	if !hasHint || hintedKind == KindJournalEntries {
		journalHeaders, journalRows, accountRows, ok, err := parseSmartAccountsGeneralLedgerGrid(content)
		if err != nil {
			return smartAccountsCSVSource{}, "", fmt.Errorf("parse %s: %w", relPath, err)
		}
		if ok {
			return smartAccountsCSVSource{
				kind:            KindJournalEntries,
				relSourcePath:   relPath,
				sourceHash:      sourceHash,
				headers:         journalHeaders,
				rows:            journalRows,
				classification:  "smartaccounts-general-ledger-grid",
				transformations: []string{"expanded grouped SmartAccounts general ledger CSV into canonical journal lines", "removed zero-value ledger display rows", "split combined debit and credit display rows"},
				derivedSources: []smartAccountsCSVSource{{
					kind:            KindAccounts,
					relSourcePath:   relPath,
					sourceHash:      sourceHash,
					headers:         []string{"code", "name", "account_type"},
					rows:            accountRows,
					classification:  "derived-from-smartaccounts-general-ledger-grid",
					transformations: []string{"derived chart of accounts from SmartAccounts general ledger account sections"},
				}},
			}, "", nil
		}
	}
	headers, rows, err := readSmartAccountsCSVWithHint(content, hintedKind, hasHint)
	if err != nil {
		return smartAccountsCSVSource{}, "", fmt.Errorf("parse %s: %w", relPath, err)
	}
	bestKind, bestScore, matchedRequired := scoreSmartAccountsCSVHeaders(headers, hintedKind, hasHint)
	if hasHint {
		bestKind = hintedKind
	}
	if !hasHint && (bestScore < 100 || matchedRequired == 0) {
		return smartAccountsCSVSource{}, "could not classify CSV headers as a supported SmartAccounts migration file", nil
	}

	spec := fileSpecForProviderPreset(bestKind, MigrationProviderPresetSmartAccounts)
	canonicalHeaders := make([]string, len(headers))
	for i, header := range headers {
		canonicalHeaders[i] = canonicalHeader(spec.aliases, header)
	}
	canonicalHeaders, rows, transformations := normalizeSmartAccountsSourceRows(bestKind, relPath, canonicalHeaders, rows)
	classification := "headers"
	if hasHint {
		classification = "filename"
	}
	return smartAccountsCSVSource{
		kind:            bestKind,
		relSourcePath:   relPath,
		sourceHash:      sourceHash,
		headers:         canonicalHeaders,
		rows:            rows,
		classification:  classification,
		transformations: transformations,
	}, "", nil
}

func classifySmartAccountsXML(path, relPath, sourceHash, content string) (smartAccountsXMLSource, string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return smartAccountsXMLSource{}, "xml file is empty"
	}
	lower := strings.ToLower(trimmed)
	if !strings.Contains(lower, "<e_invoice") && !strings.Contains(lower, "<invoice") {
		return smartAccountsXMLSource{}, "xml does not look like an Estonian e-invoice payload"
	}
	return smartAccountsXMLSource{
		kind:           KindEInvoices,
		relSourcePath:  relPath,
		sourceHash:     sourceHash,
		content:        content,
		classification: "xml-root",
	}, ""
}

func smartAccountsUnsupportedFile(relPath, reason, sourceHash string, content []byte) SmartAccountsSnapshotUnsupported {
	return SmartAccountsSnapshotUnsupported{
		SourcePath:   relPath,
		Reason:       reason,
		SourceSHA256: sourceHash,
		SizeBytes:    int64(len(content)),
	}
}

func readSmartAccountsCSV(content string) ([]string, [][]string, error) {
	return readSmartAccountsCSVWithHint(content, "", false)
}

func readSmartAccountsCSVWithHint(content string, hintedKind FileKind, hasHint bool) ([]string, [][]string, error) {
	records, err := readSmartAccountsCSVRecords(content)
	if err != nil {
		return nil, nil, err
	}
	headerIndex := smartAccountsCSVHeaderIndex(records, hintedKind, hasHint)
	if headerIndex < 0 {
		return nil, nil, fmt.Errorf("read header: EOF")
	}
	headers := records[headerIndex]
	rows := make([][]string, 0)
	for _, record := range records[headerIndex+1:] {
		if isBlankCSVRecord(record) {
			continue
		}
		rows = append(rows, record)
	}
	return headers, rows, nil
}

func readSmartAccountsCSVRecords(content string) ([][]string, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, fmt.Errorf("csv content is empty")
	}
	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records := make([][]string, 0)
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read row: %w", err)
		}
		records = append(records, trimSmartAccountsCSVRecord(record))
	}
	return records, nil
}

var smartAccountsLedgerAccountPattern = regexp.MustCompile(`^\s*([0-9]+)\s*-\s*(.+?)\s*$`)

type smartAccountsLedgerGridColumns struct {
	date        int
	reference   int
	description int
	document    int
	debit       int
	credit      int
}

func parseSmartAccountsGeneralLedgerGrid(content string) ([]string, [][]string, [][]string, bool, error) {
	records, err := readSmartAccountsCSVRecords(content)
	if err != nil {
		return nil, nil, nil, false, err
	}
	columns := smartAccountsLedgerGridColumns{date: -1, reference: -1, description: -1, document: -1, debit: -1, credit: -1}
	currentAccount := ""
	accountNames := make(map[string]string)
	accountOrder := make([]string, 0)
	journalRows := make([][]string, 0)
	foundGrid := false

	for _, record := range records {
		if next, ok := smartAccountsLedgerGridHeader(record); ok {
			columns = next
			foundGrid = true
			continue
		}
		if match := smartAccountsLedgerAccountPattern.FindStringSubmatch(strings.TrimSpace(record[0])); len(match) == 3 {
			currentAccount = strings.TrimSpace(match[1])
			if _, exists := accountNames[currentAccount]; !exists {
				accountOrder = append(accountOrder, currentAccount)
			}
			accountNames[currentAccount] = strings.TrimSpace(match[2])
			continue
		}
		if !foundGrid || currentAccount == "" || columns.date >= len(record) {
			continue
		}
		entryDate, err := time.Parse("02.01.2006", strings.TrimSpace(record[columns.date]))
		if err != nil {
			continue
		}
		reference := valueAtIndex(record, columns.reference)
		if reference == "" {
			reference = valueAtIndex(record, columns.document)
		}
		if reference == "" {
			reference = "ledger"
		}
		entryReference := entryDate.Format("2006-01-02") + " " + reference
		sourceID := uuid.NewSHA1(smartAccountsGeneralLedgerSourceNamespace, []byte(entryReference)).String()
		description := valueAtIndex(record, columns.description)
		if description == "" {
			description = valueAtIndex(record, columns.document)
		}
		debit, credit, err := smartAccountsLedgerDebitCredit(valueAtIndex(record, columns.debit), valueAtIndex(record, columns.credit))
		if err != nil {
			return nil, nil, nil, false, err
		}
		if debit.IsZero() && credit.IsZero() {
			continue
		}
		appendRow := func(debitValue, creditValue decimal.Decimal) {
			journalRows = append(journalRows, []string{
				entryReference,
				entryDate.Format("2006-01-02"),
				currentAccount,
				description,
				decimalCSVValue(debitValue),
				decimalCSVValue(creditValue),
				"SMARTACCOUNTS_GL",
				sourceID,
			})
		}
		if debit.IsPositive() && credit.IsPositive() {
			appendRow(debit, decimal.Zero)
			appendRow(decimal.Zero, credit)
			continue
		}
		appendRow(debit, credit)
	}

	if !foundGrid {
		return nil, nil, nil, false, nil
	}
	if len(journalRows) == 0 || len(accountOrder) == 0 {
		return nil, nil, nil, false, fmt.Errorf("SmartAccounts general ledger grid contains no journal rows or account sections")
	}
	accountRows := make([][]string, 0, len(accountOrder))
	for _, code := range accountOrder {
		accountRows = append(accountRows, []string{code, accountNames[code], inferSmartAccountsAccountTypeFromCode(code, "")})
	}
	return []string{"entry_reference", "entry_date", "account_code", "line_description", "debit", "credit", "source_type", "source_id"}, journalRows, accountRows, true, nil
}

func smartAccountsLedgerGridHeader(record []string) (smartAccountsLedgerGridColumns, bool) {
	columns := smartAccountsLedgerGridColumns{date: -1, reference: -1, description: -1, document: -1, debit: -1, credit: -1}
	for i, header := range record {
		switch normalizedHeader(header) {
		case "kuupaev", "kuupäev":
			columns.date = i
		case "alus":
			if columns.reference == -1 {
				columns.reference = i
			}
		case "kande_kirjeldus":
			columns.description = i
		case "alusdokument":
			columns.document = i
		case "deebet":
			columns.debit = i
		case "kreedit":
			columns.credit = i
		}
	}
	return columns, columns.date >= 0 && columns.reference >= 0 && columns.debit >= 0 && columns.credit >= 0
}

func smartAccountsLedgerDebitCredit(debitValue, creditValue string) (decimal.Decimal, decimal.Decimal, error) {
	parse := func(value string) (decimal.Decimal, error) {
		normalized := normalizeSmartAccountsDecimalValue(value)
		if normalized == "" {
			return decimal.Zero, nil
		}
		parsed, err := decimal.NewFromString(normalized)
		if err != nil {
			return decimal.Zero, fmt.Errorf("parse SmartAccounts ledger amount %q: %w", value, err)
		}
		return parsed, nil
	}
	debit, err := parse(debitValue)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	credit, err := parse(creditValue)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	if debit.IsNegative() {
		credit = credit.Add(debit.Abs())
		debit = decimal.Zero
	}
	if credit.IsNegative() {
		debit = debit.Add(credit.Abs())
		credit = decimal.Zero
	}
	return debit, credit, nil
}

func decimalCSVValue(value decimal.Decimal) string {
	if value.IsZero() {
		return ""
	}
	return value.String()
}

func valueAtIndex(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func trimSmartAccountsCSVRecord(record []string) []string {
	trimmed := make([]string, len(record))
	for i := range record {
		trimmed[i] = strings.TrimPrefix(strings.TrimSpace(record[i]), "\ufeff")
	}
	return trimmed
}

func smartAccountsCSVHeaderIndex(records [][]string, hintedKind FileKind, hasHint bool) int {
	bestIndex := -1
	bestScore := 0
	firstNonBlank := -1
	limit := len(records)
	if limit > 20 {
		limit = 20
	}
	for i := 0; i < limit; i++ {
		record := records[i]
		if isBlankCSVRecord(record) {
			continue
		}
		if firstNonBlank == -1 {
			firstNonBlank = i
		}
		score := smartAccountsCSVHeaderCandidateScore(record, hintedKind, hasHint)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex >= 0 {
		return bestIndex
	}
	return firstNonBlank
}

func smartAccountsCSVHeaderCandidateScore(headers []string, hintedKind FileKind, hasHint bool) int {
	kinds := migrationPresetCatalogFileKinds()
	if hasHint {
		kinds = []FileKind{hintedKind}
	}
	bestScore := 0
	for _, kind := range kinds {
		spec := fileSpecForProviderPreset(kind, MigrationProviderPresetSmartAccounts)
		canonicalSet := make(map[string]bool, len(headers))
		matchedKnown := 0
		for _, header := range headers {
			normalized := normalizedHeader(header)
			canonical := canonicalHeader(spec.aliases, header)
			canonicalSet[canonical] = true
			if canonical != normalized || valueInAliasMap(spec.aliases, canonical) {
				matchedKnown++
			}
		}
		score := matchedKnown + matchedRequiredGroups(spec.requiredGroups, canonicalSet)*100
		if score > bestScore {
			bestScore = score
		}
	}
	return bestScore
}

func scoreSmartAccountsCSVHeaders(headers []string, hintedKind FileKind, hasHint bool) (FileKind, int, int) {
	bestKind := KindContacts
	bestScore := -1
	bestRequired := 0
	for _, kind := range migrationPresetCatalogFileKinds() {
		spec := fileSpecForProviderPreset(kind, MigrationProviderPresetSmartAccounts)
		canonicalSet := make(map[string]bool, len(headers))
		matchedKnown := 0
		for _, header := range headers {
			normalized := normalizedHeader(header)
			canonical := canonicalHeader(spec.aliases, header)
			canonicalSet[canonical] = true
			if canonical != normalized || valueInAliasMap(spec.aliases, canonical) {
				matchedKnown++
			}
		}
		required := matchedRequiredGroups(spec.requiredGroups, canonicalSet)
		score := matchedKnown + required*100
		if hasHint && kind == hintedKind {
			score += 50
		}
		if score > bestScore || (score == bestScore && required > bestRequired) {
			bestKind = kind
			bestScore = score
			bestRequired = required
		}
	}
	return bestKind, bestScore, bestRequired
}

func matchedRequiredGroups(groups [][]string, canonicalSet map[string]bool) int {
	matched := 0
	for _, group := range groups {
		for _, field := range group {
			if canonicalSet[field] {
				matched++
				break
			}
		}
	}
	return matched
}

func valueInAliasMap(aliases map[string]string, value string) bool {
	for _, canonical := range aliases {
		if canonical == value {
			return true
		}
	}
	return false
}

func smartAccountsFilenameKind(path string) (FileKind, bool) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	key := normalizedHeader(base)
	patterns := []struct {
		needles []string
		kind    FileKind
	}{
		{[]string{"e_invoice", "einvoice", "e_arve"}, KindEInvoices},
		{[]string{"recurring_invoice", "recurring"}, KindRecurringInvoices},
		{[]string{"sales_invoice", "sale_invoice", "client_invoice", "customer_invoice", "invoice", "arve"}, KindInvoices},
		{[]string{"opening_balance", "opening_balances", "algsaldo"}, KindOpeningBalances},
		{[]string{"journal_entry", "journal_entries", "general_ledger", "ledger", "kanne", "kanded"}, KindJournalEntries},
		{[]string{"bank_transaction", "bank_transactions", "statement", "transactions"}, KindBankTransactions},
		{[]string{"payment", "payments", "laekumine", "tasumine"}, KindPayments},
		{[]string{"bank_account", "bank_accounts"}, KindBankAccounts},
		{[]string{"account", "accounts", "chart_of_accounts", "kontoplaan"}, KindAccounts},
		{[]string{"contact", "contacts", "client", "clients", "customer", "customers", "vendor", "vendors", "supplier", "suppliers"}, KindContacts},
		{[]string{"employee", "employees", "tootaja"}, KindEmployees},
		{[]string{"payroll_history", "payroll", "salary_history"}, KindPayrollHistory},
		{[]string{"leave_balance", "leave_balances", "vacation"}, KindLeaveBalances},
		{[]string{"tsd_history", "tsd"}, KindTSDHistory},
		{[]string{"kmd_history", "kmd", "vat_return"}, KindKMDHistory},
		{[]string{"quote", "quotes", "offer", "offers"}, KindQuotes},
		{[]string{"order", "orders"}, KindOrders},
		{[]string{"cost_allocation", "cost_allocations"}, KindCostAllocations},
		{[]string{"cost_center", "cost_centers", "project", "projects"}, KindCostCenters},
		{[]string{"product_category", "product_categories", "item_category"}, KindProductCategories},
		{[]string{"warehouse", "warehouses"}, KindWarehouses},
		{[]string{"stock_adjustment", "stock_adjustments", "stock"}, KindStockAdjustments},
		{[]string{"product", "products", "item", "items", "article", "articles", "artikkel", "artiklid"}, KindProducts},
		{[]string{"fixed_asset", "fixed_assets", "asset", "assets"}, KindFixedAssets},
		{[]string{"expense", "expenses"}, KindExpenses},
	}
	for _, pattern := range patterns {
		for _, needle := range pattern.needles {
			if key == needle || strings.Contains(key, needle) {
				return pattern.kind, true
			}
		}
	}
	return "", false
}

func writeSmartAccountsCSVBundles(outputDir string, sources []smartAccountsCSVSource, report *SmartAccountsSnapshotReport) error {
	byKind := make(map[FileKind][]smartAccountsCSVSource)
	for _, source := range sources {
		byKind[source.kind] = append(byKind[source.kind], source)
	}
	kinds := make([]FileKind, 0, len(byKind))
	for kind := range byKind {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })

	for _, kind := range kinds {
		mergedHeaders, mergedRows := mergeSmartAccountsCSVRows(byKind[kind])
		mergeTransformations := []string{"canonicalized SmartAccounts CSV headers", "merged by cutover file kind"}
		normalizedRows, extraTransformations := normalizeSmartAccountsMergedRows(kind, mergedHeaders, mergedRows)
		mergedRows = normalizedRows
		mergeTransformations = append(mergeTransformations, extraTransformations...)
		content := writeCSVContent(mergedHeaders, mergedRows)
		outputPath := filepath.Join(outputDir, "bundle", string(kind)+".csv")
		if err := os.WriteFile(outputPath, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", outputPath, err)
		}
		outputHash := sha256Hex([]byte(content))
		report.bundleFiles = append(report.bundleFiles, BundleFile{
			Kind:       kind,
			FileName:   filepath.Base(outputPath),
			CSVContent: content,
		})
		relOutputPath, _ := filepath.Rel(outputDir, outputPath)
		nextOutputRow := 1
		for _, source := range byKind[kind] {
			outputRowStart := 0
			outputRowEnd := 0
			if len(source.rows) > 0 {
				outputRowStart = nextOutputRow
				outputRowEnd = nextOutputRow + len(source.rows) - 1
			}
			report.PreparedFiles = append(report.PreparedFiles, SmartAccountsSnapshotPreparedFile{
				Kind:              kind,
				SourcePath:        source.relSourcePath,
				OutputPath:        relOutputPath,
				SourceSHA256:      source.sourceHash,
				OutputSHA256:      outputHash,
				Rows:              len(source.rows),
				OutputRowStart:    outputRowStart,
				OutputRowEnd:      outputRowEnd,
				Transformations:   append(append([]string{}, mergeTransformations...), source.transformations...),
				Classification:    source.classification,
				ValidationCLIFlag: migrationKindCLIFlag(kind),
			})
			nextOutputRow += len(source.rows)
		}
	}
	return nil
}

func writeSmartAccountsXMLBundles(outputDir string, sources []smartAccountsXMLSource, report *SmartAccountsSnapshotReport) error {
	for i, source := range sources {
		fileName := string(KindEInvoices) + ".xml"
		if i > 0 {
			fileName = fmt.Sprintf("%s-%d.xml", KindEInvoices, i+1)
			report.Warnings = append(report.Warnings, "multiple e-invoice XML files were prepared as separate bundle files; keep the manifest-driven flow so every XML file is validated and planned together")
		}
		outputPath := filepath.Join(outputDir, "bundle", fileName)
		if err := os.WriteFile(outputPath, []byte(source.content), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", outputPath, err)
		}
		outputHash := sha256Hex([]byte(source.content))
		report.bundleFiles = append(report.bundleFiles, BundleFile{
			Kind:       KindEInvoices,
			FileName:   fileName,
			XMLContent: source.content,
		})
		relOutputPath, _ := filepath.Rel(outputDir, outputPath)
		report.PreparedFiles = append(report.PreparedFiles, SmartAccountsSnapshotPreparedFile{
			Kind:              KindEInvoices,
			SourcePath:        source.relSourcePath,
			OutputPath:        relOutputPath,
			SourceSHA256:      source.sourceHash,
			OutputSHA256:      outputHash,
			Rows:              1,
			OutputRowStart:    1,
			OutputRowEnd:      1,
			Transformations:   []string{"copied Estonian e-invoice XML"},
			Classification:    source.classification,
			ValidationCLIFlag: migrationKindCLIFlag(KindEInvoices),
		})
	}
	return nil
}

func mergeSmartAccountsCSVRows(sources []smartAccountsCSVSource) ([]string, [][]string) {
	headerSeen := map[string]bool{}
	headers := make([]string, 0)
	for _, source := range sources {
		for _, header := range source.headers {
			if !headerSeen[header] {
				headerSeen[header] = true
				headers = append(headers, header)
			}
		}
	}

	rows := make([][]string, 0)
	for _, source := range sources {
		sourceIndex := make(map[string]int, len(source.headers))
		for i, header := range source.headers {
			sourceIndex[header] = i
		}
		for _, row := range source.rows {
			merged := make([]string, len(headers))
			for i, header := range headers {
				if idx, ok := sourceIndex[header]; ok && idx < len(row) {
					merged[i] = row[idx]
				}
			}
			rows = append(rows, merged)
		}
	}
	return headers, rows
}

func normalizeSmartAccountsSourceRows(kind FileKind, relPath string, headers []string, rows [][]string) ([]string, [][]string, []string) {
	transformations := make([]string, 0)
	if kind == KindContacts && !headerIndex(headers, "contact_type").ok {
		contactType := ""
		sourceKey := normalizedHeader(relPath)
		if strings.Contains(sourceKey, "vendor") || strings.Contains(sourceKey, "supplier") || strings.Contains(sourceKey, "hankija") {
			contactType = "SUPPLIER"
		} else if strings.Contains(sourceKey, "client") || strings.Contains(sourceKey, "customer") || strings.Contains(sourceKey, "kliendid") {
			contactType = "CUSTOMER"
		}
		if contactType != "" {
			headers = append(headers, "contact_type")
			for i := range rows {
				rows[i] = append(rows[i], contactType)
			}
			transformations = append(transformations, "derived contact_type from SmartAccounts source export")
		}
	}
	if kind == KindInvoices && !headerIndex(headers, "invoice_type").ok {
		invoiceType := ""
		sourceKey := normalizedHeader(relPath)
		if strings.Contains(sourceKey, "vendor") || strings.Contains(sourceKey, "supplier") || strings.Contains(sourceKey, "ostuar") {
			invoiceType = "PURCHASE"
		} else if strings.Contains(sourceKey, "client") || strings.Contains(sourceKey, "customer") || strings.Contains(sourceKey, "müügi") || strings.Contains(sourceKey, "muugi") {
			invoiceType = "SALES"
		}
		if invoiceType != "" {
			headers = append(headers, "invoice_type")
			for i := range rows {
				rows[i] = append(rows[i], invoiceType)
			}
			transformations = append(transformations, "derived invoice_type from SmartAccounts source export")
		}
	}
	if kind == KindPayments && !headerIndex(headers, "payment_type").ok {
		headers = append(headers, "payment_type")
		amountIndex := headerIndex(headers, "amount")
		for i := range rows {
			paymentType := "RECEIVED"
			if amountIndex.ok && amountIndex.index < len(rows[i]) && strings.HasPrefix(strings.TrimSpace(rows[i][amountIndex.index]), "-") {
				paymentType = "MADE"
			}
			rows[i] = append(rows[i], paymentType)
		}
		transformations = append(transformations, "derived payment_type from SmartAccounts amount sign")
	}
	for i := range rows {
		if normalizeSmartAccountsRowValues(kind, headers, rows[i]) {
			transformations = append(transformations, "normalized SmartAccounts localized dates, decimals, and enum values")
		}
	}
	return headers, rows, uniqueStrings(transformations)
}

func normalizeSmartAccountsMergedRows(kind FileKind, headers []string, rows [][]string) ([][]string, []string) {
	switch kind {
	case KindContacts:
		normalized, changed := dedupeSmartAccountsContactRows(headers, rows)
		if changed {
			return normalized, []string{"merged duplicate SmartAccounts client/vendor contacts by registry, VAT, or name"}
		}
	case KindPayments:
		normalized, changed := normalizeDistinctSmartAccountsPaymentNumbers(headers, rows)
		if changed {
			return normalized, []string{"normalized distinct SmartAccounts duplicate payment_number values"}
		}
	}
	return rows, nil
}

type smartAccountsHeaderLookup struct {
	index int
	ok    bool
}

func headerIndex(headers []string, header string) smartAccountsHeaderLookup {
	for i, candidate := range headers {
		if candidate == header {
			return smartAccountsHeaderLookup{index: i, ok: true}
		}
	}
	return smartAccountsHeaderLookup{}
}

func normalizeSmartAccountsRowValues(kind FileKind, headers []string, row []string) bool {
	changed := false
	for i, header := range headers {
		if i >= len(row) {
			continue
		}
		value := strings.TrimSpace(row[i])
		next := value
		switch header {
		case "issue_date", "due_date", "payment_date", "purchase_date", "depreciation_start_date", "depreciation_end_date", "entry_date", "date":
			next = normalizeSmartAccountsDateValue(value)
		case "amount", "total_amount", "balance_due", "purchase_cost", "sales_price", "purchase_price", "vat_rate", "debit", "credit", "depreciation_rate":
			next = normalizeSmartAccountsDecimalValue(value)
			if kind == KindPayments && header == "amount" && strings.HasPrefix(next, "-") {
				next = strings.TrimPrefix(next, "-")
			}
			if kind == KindProducts && header == "sales_price" && next == "" {
				next = "0"
			}
		case "account_type":
			next = normalizeSmartAccountsAccountType(value, valueAtHeader(headers, row, "code"))
		case "product_type":
			next = normalizeSmartAccountsProductType(value)
		}
		if next != value {
			row[i] = next
			changed = true
		}
	}
	return changed
}

func normalizeSmartAccountsDateValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if parsed, err := time.Parse("02.01.2006", trimmed); err == nil {
		return parsed.Format("2006-01-02")
	}
	return trimmed
}

func normalizeSmartAccountsDecimalValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.TrimSuffix(trimmed, "%")
	trimmed = strings.TrimSpace(trimmed)
	normalized := normalizedHeader(trimmed)
	switch normalized {
	case "km_vaba", "kaibemaksuvaba", "käibemaksuvaba":
		return "0"
	case "jah", "ei":
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, " ", "")
	trimmed = strings.ReplaceAll(trimmed, "\u00a0", "")
	trimmed = strings.ReplaceAll(trimmed, ",", ".")
	return trimmed
}

func normalizeSmartAccountsAccountType(value, code string) string {
	switch normalizedHeader(value) {
	case "aktiva", "asset", "assets", "vara":
		return "ASSET"
	case "passiva", "kohustus", "kohustis", "liability", "liabilities":
		return inferSmartAccountsAccountTypeFromCode(code, "LIABILITY")
	case "omakapital", "equity":
		return "EQUITY"
	case "inc", "tulu", "revenue", "income":
		return "REVENUE"
	case "kulu", "expense", "expenses":
		return "EXPENSE"
	}
	return inferSmartAccountsAccountTypeFromCode(code, value)
}

func inferSmartAccountsAccountTypeFromCode(code, fallback string) string {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return fallback
	}
	switch trimmed[0] {
	case '1':
		return "ASSET"
	case '2':
		return "LIABILITY"
	case '3':
		return "REVENUE"
	case '4', '5', '6', '7', '8', '9':
		return "EXPENSE"
	default:
		return fallback
	}
}

func normalizeSmartAccountsProductType(value string) string {
	switch normalizedHeader(value) {
	case "teenus", "service":
		return "SERVICE"
	case "kaup", "laoartikkel", "goods", "item", "stock_item":
		return "GOODS"
	default:
		return value
	}
}

func dedupeSmartAccountsContactRows(headers []string, rows [][]string) ([][]string, bool) {
	indexByHeader := make(map[string]int, len(headers))
	for i, header := range headers {
		indexByHeader[header] = i
	}
	keyFields := []string{"reg_code", "vat_number", "name"}
	rowByKey := map[string]int{}
	result := make([][]string, 0, len(rows))
	changed := false
	for _, row := range rows {
		keys := make([]string, 0, len(keyFields))
		for _, field := range keyFields {
			idx, ok := indexByHeader[field]
			if !ok || idx >= len(row) {
				continue
			}
			if value := normalizedHeader(row[idx]); value != "" {
				keys = append(keys, field+":"+value)
			}
		}
		if len(keys) == 0 {
			result = append(result, row)
			continue
		}
		existingIndex := -1
		for _, key := range keys {
			if candidate, ok := rowByKey[key]; ok {
				existingIndex = candidate
				break
			}
		}
		if existingIndex == -1 {
			existingIndex = len(result)
			result = append(result, row)
			for _, key := range keys {
				rowByKey[key] = existingIndex
			}
			continue
		}
		mergeSmartAccountsContactRow(headers, result[existingIndex], row)
		for _, key := range keys {
			rowByKey[key] = existingIndex
		}
		changed = true
	}
	return result, changed
}

func mergeSmartAccountsContactRow(headers []string, target, source []string) {
	for i := range headers {
		if i >= len(source) || strings.TrimSpace(source[i]) == "" {
			continue
		}
		for len(target) <= i {
			target = append(target, "")
		}
		if strings.TrimSpace(target[i]) == "" {
			target[i] = source[i]
			continue
		}
		if headers[i] == "contact_type" && !strings.EqualFold(strings.TrimSpace(target[i]), strings.TrimSpace(source[i])) {
			target[i] = "BOTH"
		}
	}
}

const smartAccountsPaymentNumberMaxLength = 50

var smartAccountsPaymentDistinctKeyFields = []string{
	"payment_type",
	"payment_date",
	"currency",
	"exchange_rate",
	"contact_id",
	"contact_code",
	"contact_reg_code",
	"contact_vat_number",
	"contact_email",
	"contact_name",
	"payment_method",
	"bank_account",
	"reference",
}

func normalizeDistinctSmartAccountsPaymentNumbers(headers []string, rows [][]string) ([][]string, bool) {
	paymentNumber := headerIndex(headers, "payment_number")
	if !paymentNumber.ok {
		return rows, false
	}

	rowsByNumber := map[string][]int{}
	for i, row := range rows {
		if paymentNumber.index >= len(row) {
			continue
		}
		number := strings.TrimSpace(row[paymentNumber.index])
		if number == "" {
			continue
		}
		rowsByNumber[normalizedValue(number)] = append(rowsByNumber[normalizedValue(number)], i)
	}

	changed := false
	for _, rowIndexes := range rowsByNumber {
		if len(rowIndexes) < 2 || smartAccountsPaymentDuplicateGroupHasSharedDistinctKey(headers, rows, rowIndexes) {
			continue
		}
		digits := len(strconv.Itoa(len(rowIndexes)))
		if digits < 2 {
			digits = 2
		}
		for ordinal, rowIndex := range rowIndexes {
			rows[rowIndex][paymentNumber.index] = suffixedSmartAccountsPaymentNumber(rows[rowIndex][paymentNumber.index], ordinal+1, digits)
			changed = true
		}
	}
	return rows, changed
}

func smartAccountsPaymentDuplicateGroupHasSharedDistinctKey(headers []string, rows [][]string, rowIndexes []int) bool {
	seen := map[string]bool{}
	for _, rowIndex := range rowIndexes {
		key := smartAccountsPaymentDistinctKey(headers, rows[rowIndex])
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}

func smartAccountsPaymentDistinctKey(headers []string, row []string) string {
	parts := make([]string, 0, len(smartAccountsPaymentDistinctKeyFields))
	for _, field := range smartAccountsPaymentDistinctKeyFields {
		parts = append(parts, field+"="+normalizedHeader(valueAtHeader(headers, row, field)))
	}
	return strings.Join(parts, "\x1f")
}

func suffixedSmartAccountsPaymentNumber(number string, ordinal, digits int) string {
	trimmed := strings.TrimSpace(number)
	suffix := "~SA" + leftPadInt(ordinal, digits)
	maxBaseLen := smartAccountsPaymentNumberMaxLength - len(suffix)
	if maxBaseLen < 1 {
		return suffix[:smartAccountsPaymentNumberMaxLength]
	}
	trimmed = firstRunes(trimmed, maxBaseLen)
	return trimmed + suffix
}

func firstRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func leftPadInt(value, width int) string {
	text := strconv.Itoa(value)
	if len(text) >= width {
		return text
	}
	return strings.Repeat("0", width-len(text)) + text
}

func valueAtHeader(headers []string, row []string, header string) string {
	idx := headerIndex(headers, header)
	if !idx.ok || idx.index >= len(row) {
		return ""
	}
	return row[idx.index]
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}

func writeCSVContent(headers []string, rows [][]string) string {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write(headers)
	for _, row := range rows {
		_ = writer.Write(row)
	}
	writer.Flush()
	return buf.String()
}

func writeSmartAccountsManifest(path string, report *SmartAccountsSnapshotReport) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(report)
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

func smartAccountsSnapshotHash(report *SmartAccountsSnapshotReport, csvSources []smartAccountsCSVSource, xmlSources []smartAccountsXMLSource) string {
	lines := []string{
		"provider=" + string(MigrationProviderPresetSmartAccounts),
		"adapter=" + SmartAccountsSnapshotAdapterVersion,
		"company_id=" + report.SourceCompanyID,
		"company_name=" + report.SourceCompanyName,
		"cutover_date=" + report.CutoverDate,
	}
	for _, source := range csvSources {
		lines = append(lines, fmt.Sprintf("csv:%s:%s:%s", source.kind, source.relSourcePath, source.sourceHash))
	}
	for _, source := range xmlSources {
		lines = append(lines, fmt.Sprintf("xml:%s:%s:%s", source.kind, source.relSourcePath, source.sourceHash))
	}
	for _, file := range report.UnsupportedFiles {
		lines = append(lines, fmt.Sprintf("unsupported:%s:%d:%s:%s", file.SourcePath, file.SizeBytes, file.SourceSHA256, file.Reason))
	}
	sort.Strings(lines[5:])
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func smartAccountsValidationCommand(outputDir string, files []SmartAccountsSnapshotPreparedFile) string {
	if len(files) == 0 {
		return ""
	}
	parts := []string{
		"go run ./cmd/oa migration validate",
		"--provider-preset smartaccounts",
		"--manifest",
		shellQuote(filepath.Join(outputDir, "manifest.json")),
	}
	parts = append(parts, "--json")
	return strings.Join(parts, " ")
}

func validateSmartAccountsCutoverDate(value string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("cutover date must be YYYY-MM-DD: %w", err)
	}
	return nil
}

func smartAccountsSnapshotNeedsCutoverDate(sources []smartAccountsCSVSource) bool {
	for _, source := range sources {
		if source.kind == KindOpeningBalances || source.kind == KindJournalEntries {
			return true
		}
	}
	return false
}

func smartAccountsGitWorktreeWarnings(paths ...string) []string {
	warnings := make([]string, 0)
	seen := map[string]bool{}
	for _, path := range paths {
		root, ok := nearestGitWorktreeRoot(path)
		if !ok || seen[root] {
			continue
		}
		seen[root] = true
		warnings = append(warnings, fmt.Sprintf("path %s is inside Git worktree %s; keep real SmartAccounts data outside public repositories or in a separate private repository", path, root))
	}
	return warnings
}

func rejectSmartAccountsPublicWorktreePaths(paths ...string) error {
	seen := map[string]bool{}
	for _, path := range paths {
		root, ok := nearestGitWorktreeRoot(path)
		if !ok || seen[root] {
			continue
		}
		seen[root] = true
		if isOpenAccountingPublicWorktree(root) {
			return fmt.Errorf("SmartAccounts snapshot paths must not be inside public Open Accounting Git worktree %s; use a private directory or a separate private repository", root)
		}
	}
	return nil
}

func isOpenAccountingPublicWorktree(root string) bool {
	moduleFile, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err == nil && strings.Contains(string(moduleFile), "module github.com/HMB-research/open-accounting") {
		return true
	}
	gitConfig, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		return false
	}
	config := strings.ToLower(string(gitConfig))
	return strings.Contains(config, "github.com/hmb-research/open-accounting") &&
		!strings.Contains(config, "open-accounting-smartaccounts-migration-data")
}

func nearestGitWorktreeRoot(path string) (string, bool) {
	if strings.TrimSpace(path) == "" {
		return "", false
	}
	absPath, _ := filepath.Abs(path)
	info, err := os.Stat(absPath)
	if err == nil && !info.IsDir() {
		absPath = filepath.Dir(absPath)
	}
	for {
		if hasGitWorktreeMarker(absPath) {
			return absPath, true
		}
		parent := filepath.Dir(absPath)
		if parent == absPath {
			return "", false
		}
		absPath = parent
	}
}

func hasGitWorktreeMarker(root string) bool {
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		// Linked worktrees use a .git file pointing to their common Git dir.
		return true
	}
	// A directory named .git alone is not a checkout. Require a standard Git
	// control file so a parent temporary directory cannot be mistaken for a
	// worktree and block safe private export paths.
	for _, name := range []string{"HEAD", "config"} {
		if _, err := os.Stat(filepath.Join(gitPath, name)); err == nil {
			return true
		}
	}
	return false
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return r != '/' && r != '.' && r != '_' && r != '-' && r != ':' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z')
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func isBlankCSVRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
