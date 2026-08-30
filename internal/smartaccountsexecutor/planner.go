package smartaccountsexecutor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrPackageNotReady       = errors.New("SmartAccounts archive package is not ready for review")
	ErrPreviewReviewRequired = errors.New("SmartAccounts executor preview requires review")
)

// ArchiveReader is the server-only archive capability required by the
// executor. It has no browser response method and no accounting write method.
type ArchiveReader interface {
	GetStatus(ctx context.Context, schemaName, tenantID, packageID string) (importdelivery.Status, error)
	GetManifest(ctx context.Context, schemaName, tenantID, packageID string) (importdelivery.Manifest, error)
	IterateRecords(ctx context.Context, schemaName, tenantID, packageID string, visit func(json.RawMessage) error) error
}

type PostingLookup interface {
	GetPostedIdentity(ctx context.Context, schemaName, tenantID, provider, sourceCompanyID, resource, externalID string) (*PostedIdentity, error)
}

type AccountResolver interface {
	ResolveAccount(ctx context.Context, schemaName, tenantID, accountID string) error
}

type AccountResolverFunc func(context.Context, string, string, string) error

func (f AccountResolverFunc) ResolveAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	return f(ctx, schemaName, tenantID, accountID)
}

type ChartAccount struct{ ID, Code, Name string }
type AccountCatalog interface {
	ListChartAccounts(context.Context, string, string) ([]ChartAccount, error)
}
type AccountCatalogFunc func(context.Context, string, string) ([]ChartAccount, error)

func (f AccountCatalogFunc) ListChartAccounts(ctx context.Context, schemaName, tenantID string) ([]ChartAccount, error) {
	return f(ctx, schemaName, tenantID)
}

// CaptureCoverageReader returns only safe, already-staged capture metadata.
// It is deliberately separate from ArchiveReader: it cannot return source
// rows, credentials, paths, queries, or cursors.
type CaptureCoverageReader interface {
	AssessCaptureCoverage(context.Context, string, string, string) (CaptureCoverage, error)
}

type Planner struct {
	archive  ArchiveReader
	postings PostingLookup
	accounts AccountResolver
	catalog  AccountCatalog
	coverage CaptureCoverageReader
}

func NewPlanner(archive ArchiveReader, postings PostingLookup, accounts AccountResolver) *Planner {
	return &Planner{archive: archive, postings: postings, accounts: accounts}
}
func (p *Planner) SetAccountCatalog(catalog AccountCatalog) *Planner { p.catalog = catalog; return p }
func (p *Planner) SetCaptureCoverageReader(reader CaptureCoverageReader) *Planner {
	p.coverage = reader
	return p
}

type bridgeRecord struct {
	EntityType string `json:"entity_type"`
	// Resource and SourceSchema are mandatory for a browser-captured
	// authoritative journal. They bind financial planning to the exact reviewed
	// General Ledger CSV adapter rather than a similarly named summary grid.
	Resource        string          `json:"resource"`
	SourceSchema    string          `json:"source_schema"`
	ExternalID      string          `json:"external_id"`
	Revision        string          `json:"revision"`
	Operation       string          `json:"operation"`
	Payload         json.RawMessage `json:"payload"`
	PayloadSHA256   string          `json:"payload_sha256"`
	SourceCompanyID string          `json:"source_company_id"`
	GLPostingMode   string          `json:"gl_posting_mode"`
	ReviewReason    string          `json:"review_reason"`
	Journal         *bridgeJournal  `json:"journal"`
}

type bridgeJournal struct {
	PostingDate       string              `json:"posting_date"`
	Currency          string              `json:"currency"`
	ExchangeRate      decimal.Decimal     `json:"exchange_rate"`
	DocumentReference string              `json:"document_reference"`
	InternalNumber    string              `json:"internal_number"`
	Rows              []bridgeJournalLine `json:"rows"`
}
type bridgeJournalLine struct {
	SourceAccountExternalID string          `json:"source_account_external_id"`
	SourceAccountCode       string          `json:"source_account_code"`
	SourceAccountName       string          `json:"source_account_name"`
	Debit                   decimal.Decimal `json:"debit"`
	Credit                  decimal.Decimal `json:"credit"`
	DebitOriginalCurrency   decimal.Decimal `json:"debit_original_currency"`
	CreditOriginalCurrency  decimal.Decimal `json:"credit_original_currency"`
	ObjectID                string          `json:"object_id"`
	Description             string          `json:"description"`
}

// Preview parses only finalized authoritative GL records. Every other record
// is counted as non-posting evidence and cannot become a journal action.
func (p *Planner) Preview(ctx context.Context, schemaName, tenantID, packageID string, req PreviewRequest) (*Preview, error) {
	if p == nil || p.archive == nil {
		return nil, errors.New("SmartAccounts archive reader is not configured")
	}
	status, err := p.archive.GetStatus(ctx, schemaName, tenantID, packageID)
	if err != nil {
		return nil, err
	}
	manifest, err := p.archive.GetManifest(ctx, schemaName, tenantID, packageID)
	if err != nil {
		return nil, err
	}
	scopeSHA, scopeErr := canonicalScopeSHA256(manifest.Scope)
	preview := &Preview{ID: uuid.NewString(), TenantID: tenantID, PackageID: packageID, SourceCompanyID: status.SourceCompanyID, ScopeSHA256: scopeSHA, Status: PlanStatusReviewRequired}
	if scopeErr != nil {
		issues := func(code, externalID, message string) {
			preview.Issues = append(preview.Issues, Issue{Code: code, Resource: ResourceGeneralLedger, ExternalID: externalID, Message: message})
		}
		issues("package_scope_invalid", "", "package scope cannot be canonically bound")
		preview.PreviewSHA256 = previewDigest(*preview)
		return preview, ErrPackageNotReady
	}
	issues := func(code, externalID, message string) {
		preview.Issues = append(preview.Issues, Issue{Code: code, Resource: ResourceGeneralLedger, ExternalID: externalID, Message: message})
	}
	if status.Status != importdelivery.StatusStagedReview || manifest.Provider != Provider || manifest.SourceCompanyID != status.SourceCompanyID || manifest.Authority.GeneralLedgerAuthority != Provider || !manifest.Authority.SmartAccountsGLAuthoritative {
		issues("package_not_authoritative", "", "package must be a completed SmartAccounts-authoritative staged archive")
		preview.PreviewSHA256 = previewDigest(*preview)
		return preview, ErrPackageNotReady
	}
	if p.coverage == nil {
		issues("full_capture_coverage_unavailable", "", "GL preview requires durable full-capture coverage evidence")
		preview.PreviewSHA256 = previewDigest(*preview)
		return preview, ErrPreviewReviewRequired
	}
	coverage, coverageErr := p.coverage.AssessCaptureCoverage(ctx, tenantID, manifest.SourceCompanyID, packageID)
	if coverageErr != nil {
		issues("full_capture_coverage_unavailable", "", "GL preview requires durable full-capture coverage evidence")
		preview.PreviewSHA256 = previewDigest(*preview)
		return preview, ErrPreviewReviewRequired
	}
	if !coverage.Complete {
		for _, gap := range coverage.Gaps {
			issues(gap.Code, gap.ResourceID, gap.Message)
		}
		preview.PreviewSHA256 = previewDigest(*preview)
		return preview, ErrPreviewReviewRequired
	}
	autoImports := []AccountImport(nil)
	if req.UseSourceChart {
		autoImports = p.sourceChartImports(ctx, schemaName, tenantID, packageID, manifest.SourceCompanyID, issues)
	}
	mappings, imports := validateDecisions(req, autoImports, tenantID, status.SourceCompanyID, issues)
	importTargets := make(map[string]bool, len(imports))
	for _, imp := range imports {
		importTargets[uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenantID+"\x00"+status.SourceCompanyID+"\x00"+imp.SourceAccountExternalID)).String()] = true
	}
	for _, targetID := range sortedMappingTargets(mappings) {
		if importTargets[targetID] {
			continue
		}
		if p.accounts == nil || p.accounts.ResolveAccount(ctx, schemaName, tenantID, targetID) != nil {
			issues("target_account_unavailable", "", "mapped Open Accounting account is unavailable")
		}
	}
	accountTotals := map[string]*AccountReconciliation{}
	err = p.archive.IterateRecords(ctx, schemaName, tenantID, packageID, func(raw json.RawMessage) error {
		var record bridgeRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			issues("invalid_archive_record", "", "archive record cannot be decoded")
			return nil
		}
		if record.EntityType != ResourceGeneralLedger {
			preview.NonPostingRecordCount++
			return nil
		}
		if manifest.Scope.Mode == "partial_browser_capture" && (record.Resource != browserGeneralLedgerResourceID || record.SourceSchema != browserGeneralLedgerSourceSchema) {
			issues("browser_gl_source_contract_invalid", record.ExternalID, "browser GL planning requires the reviewed general_ledger_csv_v1 source; journal_entries is archive-only summary evidence")
			return nil
		}
		if strings.TrimSpace(record.SourceCompanyID) != manifest.SourceCompanyID {
			issues("source_binding_mismatch", record.ExternalID, "journal record source does not match package binding")
			return nil
		}
		if record.Operation == OperationTombstone {
			issues("source_tombstone_review_required", record.ExternalID, "source journal deletion requires reviewed correction; it cannot overwrite posted history")
			return nil
		}
		if record.Operation != OperationUpsert || record.GLPostingMode != PostingModeAuthoritativeOnce || record.Journal == nil {
			issues("journal_not_authoritative", record.ExternalID, "only authoritative_once upsert journal records may be planned")
			return nil
		}
		if !validDigest(record.Revision) || record.Revision != record.PayloadSHA256 || !validDigest(record.PayloadSHA256) || !payloadMatches(record.Payload, record.PayloadSHA256) {
			issues("journal_revision_invalid", record.ExternalID, "journal revision and canonical payload digest must match")
			return nil
		}
		journal, journalIssues := normalizeJournal(record)
		for _, issue := range journalIssues {
			issues(issue.Code, record.ExternalID, issue.Message)
		}
		if len(journalIssues) > 0 {
			return nil
		}
		if p.postings != nil {
			existing, err := p.postings.GetPostedIdentity(ctx, schemaName, tenantID, Provider, manifest.SourceCompanyID, ResourceGeneralLedger, journal.ExternalID)
			if err != nil {
				return err
			}
			if existing != nil {
				if existing.Revision == journal.Revision && existing.Status == PlanStatusApplied {
					preview.Journals = append(preview.Journals, PlannedJournal{Journal: journal, Action: "ALREADY_APPLIED"})
					return nil
				}
				issues("source_revision_correction_required", journal.ExternalID, "source revision conflicts with durable posting history; create a reviewed correction instead")
				return nil
			}
		}
		planned := PlannedJournal{Journal: journal, Action: "CREATE_AND_POST"}
		for _, line := range journal.Lines {
			targetID, ok := mappings[line.SourceAccountExternalID]
			if !ok {
				issues("account_mapping_required", journal.ExternalID, "each journal account needs an explicit existing-account mapping or chart import decision")
				continue
			}
			planned.MappedLines = append(planned.MappedLines, MappedLine{SourceAccountExternalID: line.SourceAccountExternalID, TargetAccountID: targetID, Debit: line.Debit, Credit: line.Credit})
			planned.DebitTotal = planned.DebitTotal.Add(line.Debit)
			planned.CreditTotal = planned.CreditTotal.Add(line.Credit)
			total := accountTotals[line.SourceAccountExternalID]
			if total == nil {
				total = &AccountReconciliation{SourceAccountExternalID: line.SourceAccountExternalID, TargetAccountID: targetID, Currency: journal.Currency}
				accountTotals[line.SourceAccountExternalID] = total
			}
			total.DebitTotal = total.DebitTotal.Add(line.Debit)
			total.CreditTotal = total.CreditTotal.Add(line.Credit)
		}
		preview.Journals = append(preview.Journals, planned)
		return nil
	})
	if err != nil {
		return nil, err
	}
	preview.AccountImports = imports
	for _, total := range accountTotals {
		preview.AccountReconciliation = append(preview.AccountReconciliation, *total)
	}
	sort.Slice(preview.AccountReconciliation, func(i, j int) bool {
		return preview.AccountReconciliation[i].SourceAccountExternalID < preview.AccountReconciliation[j].SourceAccountExternalID
	})
	if len(preview.Issues) == 0 {
		preview.Status = PlanStatusPreviewReady
		for _, journal := range preview.Journals {
			if journal.Action == "CREATE_AND_POST" {
				preview.FinancialWritesPlanned = true
			}
		}
	}
	preview.PreviewSHA256 = previewDigest(*preview)
	if preview.Status != PlanStatusPreviewReady {
		return preview, ErrPreviewReviewRequired
	}
	return preview, nil
}

func validateDecisions(req PreviewRequest, autoImports []AccountImport, tenantID, sourceID string, issue func(string, string, string)) (map[string]string, []AccountImport) {
	mappings := map[string]string{}
	imports := make([]AccountImport, 0, len(req.AccountImports))
	for _, mapping := range req.AccountMappings {
		source := strings.TrimSpace(mapping.SourceAccountExternalID)
		target := strings.TrimSpace(mapping.TargetAccountID)
		if source == "" || target == "" || mappings[source] != "" {
			issue("invalid_account_mapping", source, "source account mappings must be nonempty and unique")
			continue
		}
		mappings[source] = target
	}
	for _, imp := range req.AccountImports {
		source := strings.TrimSpace(imp.SourceAccountExternalID)
		if source == "" || mappings[source] != "" || strings.TrimSpace(imp.Code) == "" || strings.TrimSpace(imp.Name) == "" || !validAccountType(imp.AccountType) {
			issue("invalid_account_import", source, "chart import requires unique source account, code, name, and valid type")
			continue
		}
		target := uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenantID+"\x00"+sourceID+"\x00"+source)).String()
		mappings[source] = target
		imports = append(imports, imp)
	}
	for _, imp := range autoImports {
		source := strings.TrimSpace(imp.SourceAccountExternalID)
		if mappings[source] != "" {
			continue
		} // an explicit reviewer selection wins
		mappings[source] = uuid.NewSHA1(uuid.NameSpaceURL, []byte(tenantID+"\x00"+sourceID+"\x00"+source)).String()
		imports = append(imports, imp)
	}
	return mappings, imports
}

type sourceChartPayload struct {
	ID            string `json:"id"`
	Code          string `json:"code"`
	Type          string `json:"type"`
	DescriptionET string `json:"descriptionEt"`
	DescriptionEN string `json:"descriptionEn"`
}

func (p *Planner) sourceChartImports(ctx context.Context, schemaName, tenantID, packageID, sourceID string, issue func(string, string, string)) []AccountImport {
	if p.catalog == nil {
		issue("source_chart_catalog_unavailable", "", "source chart proposals require a local chart catalog")
		return nil
	}
	existing, err := p.catalog.ListChartAccounts(ctx, schemaName, tenantID)
	if err != nil {
		issue("source_chart_catalog_unavailable", "", "local chart cannot be inspected for collisions")
		return nil
	}
	codes, names := map[string]bool{}, map[string]bool{}
	for _, account := range existing {
		codes[strings.ToUpper(strings.TrimSpace(account.Code))] = true
		names[strings.ToUpper(strings.TrimSpace(account.Name))] = true
	}
	proposals := []AccountImport{}
	seenIDs, seenCodes, seenNames := map[string]bool{}, map[string]bool{}, map[string]bool{}
	err = p.archive.IterateRecords(ctx, schemaName, tenantID, packageID, func(raw json.RawMessage) error {
		var record bridgeRecord
		if json.Unmarshal(raw, &record) != nil || record.EntityType != "account" {
			return nil
		}
		if strings.TrimSpace(record.SourceCompanyID) != sourceID {
			issue("source_chart_binding_mismatch", record.ExternalID, "source account does not match package binding")
			return nil
		}
		var source sourceChartPayload
		if json.Unmarshal(record.Payload, &source) != nil {
			issue("source_chart_record_invalid", record.ExternalID, "source account payload cannot be decoded")
			return nil
		}
		id, code := strings.TrimSpace(source.ID), strings.TrimSpace(source.Code)
		name := strings.TrimSpace(source.DescriptionET)
		if name == "" {
			name = strings.TrimSpace(source.DescriptionEN)
		}
		accountType, ok := mapSourceAccountType(strings.TrimSpace(source.Type))
		if id == "" || id != strings.TrimSpace(record.ExternalID) || code == "" || name == "" || !ok {
			issue("source_chart_account_review_required", record.ExternalID, "source account requires id, code, known type, and Estonian or English description")
			return nil
		}
		codeKey, nameKey := strings.ToUpper(code), strings.ToUpper(name)
		if seenIDs[id] || seenCodes[codeKey] || seenNames[nameKey] || codes[codeKey] || names[nameKey] {
			issue("source_chart_name_collision", id, "source account code or name collides with the staged or Open Accounting chart")
			return nil
		}
		seenIDs[id], seenCodes[codeKey], seenNames[nameKey] = true, true, true
		proposals = append(proposals, AccountImport{SourceAccountExternalID: id, Code: code, Name: name, AccountType: accountType})
		return nil
	})
	if err != nil {
		issue("source_chart_archive_unavailable", "", "source chart records could not be read")
	}
	sort.Slice(proposals, func(i, j int) bool {
		return proposals[i].SourceAccountExternalID < proposals[j].SourceAccountExternalID
	})
	return proposals
}
func mapSourceAccountType(value string) (string, bool) {
	switch strings.ToUpper(value) {
	case "ASSET":
		return "ASSET", true
	case "LIABILITY":
		return "LIABILITY", true
	case "INCOME":
		return "REVENUE", true
	case "EXPENSE":
		return "EXPENSE", true
	}
	return "", false
}
func normalizeJournal(record bridgeRecord) (Journal, []Issue) {
	j := Journal{ExternalID: strings.TrimSpace(record.ExternalID), Revision: record.Revision, PostingDate: record.Journal.PostingDate, Currency: record.Journal.Currency, ExchangeRate: record.Journal.ExchangeRate, DocumentReference: record.Journal.DocumentReference, InternalNumber: record.Journal.InternalNumber}
	var issues []Issue
	add := func(c, m string) { issues = append(issues, Issue{Code: c, Message: m}) }
	if j.ExternalID == "" {
		add("journal_external_id_required", "journal external ID is required")
	}
	if _, err := time.Parse("2006-01-02", j.PostingDate); err != nil {
		add("journal_posting_date_invalid", "journal posting_date must be YYYY-MM-DD")
	}
	if len(j.Currency) != 3 || strings.ToUpper(j.Currency) != j.Currency {
		add("journal_currency_invalid", "journal currency must be ISO-3 uppercase")
	}
	if j.Currency != "EUR" && j.ExchangeRate.LessThanOrEqual(decimal.Zero) {
		add("journal_exchange_rate_required", "non-EUR journal requires a positive exchange rate")
	}
	if len(record.Journal.Rows) < 2 {
		add("journal_rows_invalid", "journal requires at least two rows")
	}
	for _, row := range record.Journal.Rows {
		line := JournalLine{SourceAccountExternalID: strings.TrimSpace(row.SourceAccountExternalID), SourceAccountCode: row.SourceAccountCode, SourceAccountName: row.SourceAccountName, Debit: row.Debit, Credit: row.Credit, DebitOriginalCurrency: row.DebitOriginalCurrency, CreditOriginalCurrency: row.CreditOriginalCurrency, ObjectID: row.ObjectID, Description: row.Description}
		if line.SourceAccountExternalID == "" {
			add("journal_account_required", "journal row source account is required")
		}
		if line.Debit.IsNegative() || line.Credit.IsNegative() || line.Debit.IsPositive() == line.Credit.IsPositive() {
			add("journal_row_invalid", "each journal row must have exactly one positive debit or credit")
		}
		if j.Currency != "EUR" && (line.DebitOriginalCurrency.IsNegative() || line.CreditOriginalCurrency.IsNegative() || line.DebitOriginalCurrency.IsPositive() == line.CreditOriginalCurrency.IsPositive()) {
			add("journal_original_currency_required", "non-EUR journal row requires exactly one original-currency debit or credit")
		}
		j.Lines = append(j.Lines, line)
	}
	debit, credit := decimal.Zero, decimal.Zero
	for _, line := range j.Lines {
		debit = debit.Add(line.Debit)
		credit = credit.Add(line.Credit)
	}
	if !debit.Equal(credit) {
		add("journal_unbalanced", "journal base debit and credit must balance")
	}
	return j, issues
}
func payloadMatches(payload json.RawMessage, digest string) bool {
	var value interface{}
	if json.Unmarshal(payload, &value) != nil {
		return false
	}
	canonical, err := json.Marshal(value)
	return err == nil && sha256Hex(canonical) == digest
}
func validDigest(v string) bool {
	if len(v) != 64 || strings.ToLower(v) != v {
		return false
	}
	_, err := hex.DecodeString(v)
	return err == nil
}
func sha256Hex(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }
func validAccountType(v string) bool {
	switch v {
	case "ASSET", "LIABILITY", "EQUITY", "REVENUE", "EXPENSE":
		return true
	}
	return false
}
func sortedMappingTargets(m map[string]string) []string {
	values := make([]string, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	sort.Strings(values)
	return values
}

func canonicalScopeSHA256(scope importdelivery.Scope) (string, error) {
	resourceIDs := append([]string(nil), scope.ResourceIDs...)
	sort.Strings(resourceIDs)
	canonical := struct {
		Mode           string   `json:"mode"`
		DateFrom       string   `json:"date_from"`
		DateTo         string   `json:"date_to"`
		ResourceIDs    []string `json:"resource_ids"`
		SourceAsOfDate string   `json:"source_as_of_date"`
		CutoffAt       string   `json:"cutoff_at"`
	}{scope.Mode, scope.DateFrom, scope.DateTo, resourceIDs, scope.SourceAsOfDate, scope.CutoffAt}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(canonical); err != nil {
		return "", err
	}
	return sha256Hex(bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})), nil
}
func previewDigest(preview Preview) string {
	preview.ID = ""
	preview.PreviewSHA256 = ""
	encoded, _ := json.Marshal(preview)
	return sha256Hex(encoded)
}

var _ = fmt.Sprintf
