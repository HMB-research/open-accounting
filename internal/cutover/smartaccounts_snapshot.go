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
	"sort"
	"strings"
	"time"
)

const SmartAccountsSnapshotAdapterVersion = "smartaccounts-snapshot-v1"

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
	Transformations   []string `json:"transformations,omitempty"`
	Classification    string   `json:"classification"`
	ValidationCLIFlag string   `json:"validation_cli_flag,omitempty"`
}

type SmartAccountsSnapshotUnsupported struct {
	SourcePath string `json:"source_path"`
	Reason     string `json:"reason"`
}

type smartAccountsCSVSource struct {
	kind           FileKind
	relSourcePath  string
	sourceHash     string
	headers        []string
	rows           [][]string
	classification string
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
	if opts.GeneratedAt.IsZero() {
		opts.GeneratedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "bundle"), 0o755); err != nil {
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
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.Base(path), ".") {
			return nil
		}
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
				report.UnsupportedFiles = append(report.UnsupportedFiles, SmartAccountsSnapshotUnsupported{SourcePath: relPath, Reason: reason})
				return nil
			}
			csvSources = append(csvSources, source)
		case ".xml":
			source, reason := classifySmartAccountsXML(path, relPath, sourceHash, string(content))
			if reason != "" {
				report.UnsupportedFiles = append(report.UnsupportedFiles, SmartAccountsSnapshotUnsupported{SourcePath: relPath, Reason: reason})
				return nil
			}
			xmlSources = append(xmlSources, source)
		default:
			report.UnsupportedFiles = append(report.UnsupportedFiles, SmartAccountsSnapshotUnsupported{
				SourcePath: relPath,
				Reason:     "unsupported file extension; expected SmartAccounts CSV export or Estonian e-invoice XML",
			})
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
	headers, _, err := readSmartAccountsCSV(content)
	if err != nil {
		return smartAccountsCSVSource{}, "", fmt.Errorf("parse %s: %w", relPath, err)
	}
	if len(headers) == 0 {
		return smartAccountsCSVSource{}, "csv header is empty", nil
	}

	hintedKind, hasHint := smartAccountsFilenameKind(path)
	bestKind, bestScore, matchedRequired := scoreSmartAccountsCSVHeaders(headers, hintedKind, hasHint)
	if hasHint {
		bestKind = hintedKind
	}
	if !hasHint && (bestScore < 100 || matchedRequired == 0) {
		return smartAccountsCSVSource{}, "could not classify CSV headers as a supported SmartAccounts migration file", nil
	}

	canonicalContent, err := canonicalizeCSVHeaders(content, fileSpecForProviderPreset(bestKind, MigrationProviderPresetSmartAccounts))
	if err != nil {
		return smartAccountsCSVSource{}, "", fmt.Errorf("canonicalize %s: %w", relPath, err)
	}
	canonicalHeaders, canonicalRows, err := readSmartAccountsCSV(canonicalContent)
	if err != nil {
		return smartAccountsCSVSource{}, "", fmt.Errorf("parse canonical %s: %w", relPath, err)
	}
	classification := "headers"
	if hasHint {
		classification = "filename"
	}
	return smartAccountsCSVSource{
		kind:           bestKind,
		relSourcePath:  relPath,
		sourceHash:     sourceHash,
		headers:        canonicalHeaders,
		rows:           canonicalRows,
		classification: classification,
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

func readSmartAccountsCSV(content string) ([]string, [][]string, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(content), "\ufeff")
	if trimmed == "" {
		return nil, nil, fmt.Errorf("csv content is empty")
	}
	reader := csv.NewReader(strings.NewReader(trimmed))
	reader.Comma = detectDelimiter(trimmed)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	headers, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}
	for i := range headers {
		headers[i] = strings.TrimPrefix(strings.TrimSpace(headers[i]), "\ufeff")
	}
	rows := make([][]string, 0)
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, fmt.Errorf("read row: %w", err)
		}
		if isBlankCSVRecord(record) {
			continue
		}
		rows = append(rows, record)
	}
	return headers, rows, nil
}

func scoreSmartAccountsCSVHeaders(headers []string, hintedKind FileKind, hasHint bool) (FileKind, int, int) {
	bestKind := KindContacts
	bestScore := -1
	bestRequired := 0
	for _, kind := range migrationPresetCatalogFileKinds() {
		if kind == KindEInvoices {
			continue
		}
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
		{[]string{"bank_account", "bank_accounts", "bank"}, KindBankAccounts},
		{[]string{"payment", "payments", "laekumine", "tasumine"}, KindPayments},
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
		{[]string{"product", "products", "item", "items"}, KindProducts},
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
		content, err := writeCSVContent(mergedHeaders, mergedRows)
		if err != nil {
			return fmt.Errorf("write %s bundle: %w", kind, err)
		}
		outputPath := filepath.Join(outputDir, "bundle", string(kind)+".csv")
		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", outputPath, err)
		}
		outputHash := sha256Hex([]byte(content))
		report.bundleFiles = append(report.bundleFiles, BundleFile{
			Kind:       kind,
			FileName:   filepath.Base(outputPath),
			CSVContent: content,
		})
		relOutputPath, _ := filepath.Rel(outputDir, outputPath)
		for _, source := range byKind[kind] {
			report.PreparedFiles = append(report.PreparedFiles, SmartAccountsSnapshotPreparedFile{
				Kind:              kind,
				SourcePath:        source.relSourcePath,
				OutputPath:        relOutputPath,
				SourceSHA256:      source.sourceHash,
				OutputSHA256:      outputHash,
				Rows:              len(source.rows),
				Transformations:   []string{"canonicalized SmartAccounts CSV headers", "merged by cutover file kind"},
				Classification:    source.classification,
				ValidationCLIFlag: migrationKindCLIFlag(kind),
			})
		}
	}
	return nil
}

func writeSmartAccountsXMLBundles(outputDir string, sources []smartAccountsXMLSource, report *SmartAccountsSnapshotReport) error {
	for i, source := range sources {
		fileName := string(KindEInvoices) + ".xml"
		if i > 0 {
			fileName = fmt.Sprintf("%s-%d.xml", KindEInvoices, i+1)
			report.Warnings = append(report.Warnings, "multiple e-invoice XML files were prepared; pass additional XML files through the API bundle request or validate them one at a time with the CLI")
		}
		outputPath := filepath.Join(outputDir, "bundle", fileName)
		if err := os.WriteFile(outputPath, []byte(source.content), 0o644); err != nil {
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

func writeCSVContent(headers []string, rows [][]string) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(headers); err != nil {
		return "", err
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func writeSmartAccountsManifest(path string, report *SmartAccountsSnapshotReport) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
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
	sort.Strings(lines[5:])
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func smartAccountsValidationCommand(outputDir string, files []SmartAccountsSnapshotPreparedFile) string {
	if len(files) == 0 {
		return ""
	}
	seen := map[FileKind]bool{}
	parts := []string{"go run ./cmd/oa migration validate", "--provider-preset smartaccounts"}
	sorted := append([]SmartAccountsSnapshotPreparedFile(nil), files...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].OutputPath < sorted[j].OutputPath
	})
	for _, file := range sorted {
		if seen[file.Kind] {
			continue
		}
		flag := migrationKindCLIFlag(file.Kind)
		if flag == "" {
			continue
		}
		seen[file.Kind] = true
		parts = append(parts, "--"+flag, shellQuote(filepath.Join(outputDir, file.OutputPath)))
	}
	parts = append(parts, "--json")
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !(r == '/' || r == '.' || r == '_' || r == '-' || r == ':' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'))
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
