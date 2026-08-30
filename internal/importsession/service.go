package importsession

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	maxCanonicalRecords = 10000
	maxPayloadBytes     = 64 << 10
)

var (
	// ErrPackageValidationFailed means the request was parsed but must not be
	// persisted because its read-only validation report has blockers.
	ErrPackageValidationFailed = errors.New("canonical package validation failed")
	// ErrImportSessionNotFound is returned when a session is outside the tenant
	// or does not exist.
	ErrImportSessionNotFound = errors.New("import session not found")
	// ErrSourceCompanyBoundToOtherTenant prevents one SmartAccounts company
	// package from being replayed into a different Open Accounting tenant.
	ErrSourceCompanyBoundToOtherTenant = errors.New("source company is bound to another tenant")
	// ErrImportPlanReviewRequired means a receipt is structurally valid but its
	// source variance/staleness status requires accountant review before a
	// deterministic dry-run may be produced.
	ErrImportPlanReviewRequired = errors.New("import session requires review before planning")
	// ErrLedgerPlanInputUnavailable means the receipt predates the minimal
	// staged-ledger metadata added for deterministic dry-run planning.
	ErrLedgerPlanInputUnavailable = errors.New("staged ledger metadata is unavailable for import planning")
)

var supportedEntityTypes = map[string]struct{}{
	"account":            {},
	"attachment":         {},
	"bank_account":       {},
	"bank_transaction":   {},
	"contact":            {},
	"cost_center":        {},
	"employee":           {},
	"fixed_asset":        {},
	"inventory_movement": {},
	"journal_entry":      {},
	"order":              {},
	"payment":            {},
	"payroll_run":        {},
	"product":            {},
	"purchase_invoice":   {},
	"quote":              {},
	"recurring_invoice":  {},
	"sales_invoice":      {},
	"tax_declaration":    {},
	"vat_code":           {},
	"warehouse":          {},
}

// Service validates and persists package receipts. It does not depend on any
// accounting write service and cannot create business transactions.
type Service struct {
	store           Store
	accountResolver AccountResolver
	now             func() time.Time
}

// AccountResolver verifies that a mapped target account exists in the selected
// tenant. It is read-only by contract.
type AccountResolver interface {
	ResolveAccount(ctx context.Context, schemaName, tenantID, accountID string) error
}

// AccountResolverFunc adapts a read-only callback to AccountResolver.
type AccountResolverFunc func(ctx context.Context, schemaName, tenantID, accountID string) error

// ResolveAccount implements AccountResolver.
func (f AccountResolverFunc) ResolveAccount(ctx context.Context, schemaName, tenantID, accountID string) error {
	return f(ctx, schemaName, tenantID, accountID)
}

// NewService creates a receive-only import-session service.
func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// SetAccountResolver configures the read-only target-account verifier used by
// the import-plan phase. It cannot provide an accounting write capability.
func (s *Service) SetAccountResolver(resolver AccountResolver) {
	if s != nil {
		s.accountResolver = resolver
	}
}

// ValidatePackage performs all v1 package checks without writing any state.
func (s *Service) ValidatePackage(pkg CanonicalPackage) ValidationReport {
	report := ValidationReport{EntityCounts: make(map[string]int)}
	appendIssue := func(code string, recordIndex int, field, message string) {
		report.Issues = append(report.Issues, ValidationIssue{
			Code:        code,
			RecordIndex: recordIndex,
			Field:       field,
			Message:     message,
		})
	}

	if strings.TrimSpace(pkg.SchemaVersion) != CanonicalSchemaVersionV1 {
		appendIssue("unsupported_schema_version", 0, "schema_version", "schema_version must be v1")
	}
	if strings.TrimSpace(strings.ToLower(pkg.Provider)) != ProviderSmartAccounts {
		appendIssue("unsupported_provider", 0, "provider", "provider must be smartaccounts")
	}
	if strings.TrimSpace(pkg.SourceCompanyID) == "" {
		appendIssue("required", 0, "source_company_id", "source_company_id is required")
	}
	report.LedgerVerification = validateLedgerContract(pkg, appendIssue)
	if len(pkg.Records) == 0 {
		appendIssue("required", 0, "records", "at least one canonical record is required")
	}
	if len(pkg.Records) > maxCanonicalRecords {
		appendIssue("record_limit_exceeded", 0, "records", fmt.Sprintf("records must contain at most %d items", maxCanonicalRecords))
	}

	seenKeys := make(map[string]struct{}, len(pkg.Records))
	for index, record := range pkg.Records {
		recordIndex := index + 1
		entityType := strings.TrimSpace(strings.ToLower(record.EntityType))
		if _, ok := supportedEntityTypes[entityType]; !ok {
			appendIssue("unsupported_entity_type", recordIndex, "entity_type", "entity_type is not supported by import session v1")
		}
		if strings.TrimSpace(record.ExternalID) == "" {
			appendIssue("required", recordIndex, "external_id", "external_id is required")
		} else if len(record.ExternalID) > 255 {
			appendIssue("max_length_exceeded", recordIndex, "external_id", "external_id must be at most 255 bytes")
		}
		if strings.TrimSpace(record.Revision) == "" {
			appendIssue("required", recordIndex, "revision", "revision is required")
		} else if len(record.Revision) > 255 {
			appendIssue("max_length_exceeded", recordIndex, "revision", "revision must be at most 255 bytes")
		}

		operation := strings.TrimSpace(strings.ToLower(record.Operation))
		if operation != "upsert" && operation != "delete" {
			appendIssue("unsupported_operation", recordIndex, "operation", "operation must be upsert or delete")
		}

		if entityType != "" && strings.TrimSpace(record.ExternalID) != "" {
			key := entityType + "\x00" + strings.TrimSpace(record.ExternalID)
			if _, exists := seenKeys[key]; exists {
				appendIssue("duplicate_source_record", recordIndex, "external_id", "entity_type and external_id must be unique within a package")
			} else {
				seenKeys[key] = struct{}{}
			}
		}

		if operation == "upsert" {
			canonicalPayload, err := canonicalPayloadJSON(record.Payload)
			if err != nil {
				appendIssue("invalid_payload", recordIndex, "payload", "payload must be a JSON object")
				continue
			}
			if len(canonicalPayload) > maxPayloadBytes {
				appendIssue("payload_limit_exceeded", recordIndex, "payload", fmt.Sprintf("payload must be at most %d bytes", maxPayloadBytes))
				continue
			}
			if !isSHA256(record.PayloadSHA256) {
				appendIssue("invalid_sha256", recordIndex, "payload_sha256", "payload_sha256 must be a lowercase SHA-256 hex digest")
				continue
			}
			if record.PayloadSHA256 != sha256Hex(canonicalPayload) {
				appendIssue("payload_hash_mismatch", recordIndex, "payload_sha256", "payload_sha256 does not match payload")
			}
		}

		report.EntityCounts[entityType]++
	}
	report.RecordCount = len(pkg.Records)
	if len(report.EntityCounts) == 0 {
		report.EntityCounts = nil
	}

	if !isSHA256(pkg.PackageSHA256) {
		appendIssue("invalid_sha256", 0, "package_sha256", "package_sha256 must be a lowercase SHA-256 hex digest")
	} else if expected, err := PackageDigest(pkg); err != nil {
		appendIssue("invalid_package", 0, "package", "package cannot be digested")
	} else if pkg.PackageSHA256 != expected {
		appendIssue("package_hash_mismatch", 0, "package_sha256", "package_sha256 does not match canonical package metadata")
	}

	if len(report.Issues) == 0 {
		report.Issues = nil
		report.Ready = true
	}
	return report
}

// Receive validates a package first and persists only a metadata receipt for a
// valid package. It cannot write accounting entities.
func (s *Service) Receive(ctx context.Context, schemaName, tenantID, createdBy string, pkg CanonicalPackage) (*Receipt, ValidationReport, error) {
	report := s.ValidatePackage(pkg)
	if !report.Ready {
		return nil, report, ErrPackageValidationFailed
	}
	if s == nil || s.store == nil {
		return nil, report, errors.New("import session storage is not configured")
	}
	if err := s.store.EnsureSourceCompanyBinding(ctx, tenantID, ProviderSmartAccounts, strings.TrimSpace(pkg.SourceCompanyID)); err != nil {
		return nil, report, err
	}

	if existing, err := s.store.FindByPackage(ctx, schemaName, tenantID, ProviderSmartAccounts, strings.TrimSpace(pkg.SourceCompanyID), pkg.PackageSHA256); err == nil {
		receipt := existing
		receipt.Created = false
		return &receipt, report, nil
	} else if !errors.Is(err, ErrImportSessionNotFound) {
		return nil, report, fmt.Errorf("find import session receipt: %w", err)
	}
	ledgerPlanInput, err := stageLedgerJournals(pkg)
	if err != nil {
		return nil, report, fmt.Errorf("stage ledger metadata: %w", err)
	}

	receipt := Receipt{
		TenantID:           tenantID,
		Provider:           ProviderSmartAccounts,
		SourceCompanyID:    strings.TrimSpace(pkg.SourceCompanyID),
		SchemaVersion:      CanonicalSchemaVersionV1,
		PackageSHA256:      pkg.PackageSHA256,
		Status:             receiptStatusForReport(report),
		RecordCount:        report.RecordCount,
		EntityCounts:       cloneEntityCounts(report.EntityCounts),
		LedgerVerification: cloneLedgerVerification(report.LedgerVerification),
		LedgerPlanInput:    ledgerPlanInput,
		Validation:         report,
		CreatedBy:          strings.TrimSpace(createdBy),
		CreatedAt:          s.currentTime(),
		Created:            true,
	}
	created, err := s.store.Create(ctx, schemaName, tenantID, receipt)
	if err != nil {
		return nil, report, fmt.Errorf("save import session receipt: %w", err)
	}
	return &created, report, nil
}

// Get returns a tenant-scoped receipt without exposing raw source records.
func (s *Service) Get(ctx context.Context, schemaName, tenantID, sessionID string) (*Receipt, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("import session storage is not configured")
	}
	receipt, err := s.store.Get(ctx, schemaName, tenantID, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	return &receipt, nil
}

// PackageDigest calculates the declared digest of source identity, ledger
// authority, explicit scope, and every record identity, revision, operation,
// and payload digest. It is stable even when JSON object key order differs in
// raw payloads.
func PackageDigest(pkg CanonicalPackage) (string, error) {
	type digestRecord struct {
		EntityType    string `json:"entity_type"`
		ExternalID    string `json:"external_id"`
		Revision      string `json:"revision"`
		Operation     string `json:"operation"`
		PayloadSHA256 string `json:"payload_sha256,omitempty"`
	}
	type digestAuthority struct {
		GeneralLedgerAuthority       string `json:"general_ledger_authority"`
		SmartAccountsGLAuthoritative *bool  `json:"smartaccounts_gl_authoritative"`
		SourceAsOfDate               string `json:"source_as_of_date"`
		VarianceCount                int    `json:"variance_count"`
		Stale                        bool   `json:"stale"`
	}
	type digestScope struct {
		Mode          string   `json:"mode"`
		ResourceTypes []string `json:"resource_types"`
		PeriodStart   string   `json:"period_start,omitempty"`
		PeriodEnd     string   `json:"period_end,omitempty"`
	}
	type digestInput struct {
		SchemaVersion   string          `json:"schema_version"`
		Provider        string          `json:"provider"`
		SourceCompanyID string          `json:"source_company_id"`
		LedgerAuthority digestAuthority `json:"ledger_authority"`
		Scope           digestScope     `json:"scope"`
		Records         []digestRecord  `json:"records"`
	}

	records := make([]digestRecord, 0, len(pkg.Records))
	for _, record := range pkg.Records {
		digestRecord := digestRecord{
			EntityType: strings.TrimSpace(strings.ToLower(record.EntityType)),
			ExternalID: strings.TrimSpace(record.ExternalID),
			Revision:   strings.TrimSpace(record.Revision),
			Operation:  strings.TrimSpace(strings.ToLower(record.Operation)),
		}
		if digestRecord.Operation == "upsert" {
			payload, err := canonicalPayloadJSON(record.Payload)
			if err != nil {
				return "", err
			}
			digestRecord.PayloadSHA256 = sha256Hex(payload)
		}
		records = append(records, digestRecord)
	}
	sort.Slice(records, func(i, j int) bool {
		left := records[i].EntityType + "\x00" + records[i].ExternalID + "\x00" + records[i].Revision + "\x00" + records[i].Operation
		right := records[j].EntityType + "\x00" + records[j].ExternalID + "\x00" + records[j].Revision + "\x00" + records[j].Operation
		return left < right
	})
	authority := digestAuthority{}
	if pkg.LedgerAuthority != nil {
		authority.GeneralLedgerAuthority = strings.TrimSpace(strings.ToLower(pkg.LedgerAuthority.GeneralLedgerAuthority))
		if pkg.LedgerAuthority.SmartAccountsGLAuthoritative != nil {
			value := *pkg.LedgerAuthority.SmartAccountsGLAuthoritative
			authority.SmartAccountsGLAuthoritative = &value
		}
		authority.SourceAsOfDate = strings.TrimSpace(pkg.LedgerAuthority.SourceAsOfDate)
		authority.VarianceCount = pkg.LedgerAuthority.VarianceCount
		authority.Stale = pkg.LedgerAuthority.Stale
	}
	scope := digestScope{}
	if pkg.Scope != nil {
		scope = digestScope{
			Mode:          strings.TrimSpace(strings.ToLower(pkg.Scope.Mode)),
			ResourceTypes: normalizedResourceTypes(pkg.Scope.ResourceTypes),
			PeriodStart:   strings.TrimSpace(pkg.Scope.PeriodStart),
			PeriodEnd:     strings.TrimSpace(pkg.Scope.PeriodEnd),
		}
	}
	payload, err := json.Marshal(digestInput{
		SchemaVersion:   strings.TrimSpace(pkg.SchemaVersion),
		Provider:        strings.TrimSpace(strings.ToLower(pkg.Provider)),
		SourceCompanyID: strings.TrimSpace(pkg.SourceCompanyID),
		LedgerAuthority: authority,
		Scope:           scope,
		Records:         records,
	})
	if err != nil {
		return "", err
	}
	return sha256Hex(payload), nil
}

func canonicalPayloadJSON(raw json.RawMessage) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("payload is required")
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, errors.New("payload must be an object")
	}
	return json.Marshal(value)
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func sha256Hex(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func cloneEntityCounts(source map[string]int) map[string]int {
	if len(source) == 0 {
		return map[string]int{}
	}
	cloned := make(map[string]int, len(source))
	for entityType, count := range source {
		cloned[entityType] = count
	}
	return cloned
}

func cloneLedgerVerification(source *LedgerVerification) LedgerVerification {
	if source == nil {
		return LedgerVerification{}
	}
	cloned := *source
	cloned.ResourceTypes = append([]string(nil), source.ResourceTypes...)
	return cloned
}

func receiptStatusForReport(report ValidationReport) string {
	if report.LedgerVerification != nil && report.LedgerVerification.ReviewRequired {
		return SessionStatusReceivedReviewRequired
	}
	return SessionStatusReceivedValidated
}

func (s *Service) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
