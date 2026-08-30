package smartaccountsreconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/shopspring/decimal"
)

// ArchiveRecordReader is intentionally server-only. The implementation visits
// bounded archive records one by one and never returns them over an API.
type ArchiveRecordReader interface {
	IterateRecords(context.Context, string, string, string, func(json.RawMessage) error) error
}

// MappingSnapshotReader and AppliedPostingReader expose only immutable IDs
// from a public apply receipt. They deliberately cannot expose source rows,
// account names, target journal lines, or amounts.
type MappingSnapshotReader interface {
	ListGLApplyReceiptMappings(context.Context, string, string, string, string) ([]smartaccountsexecutor.AppliedMapping, error)
}
type AppliedPostingReader interface {
	ListGLApplyReceiptIdentities(context.Context, string, string, string, string) ([]smartaccountsexecutor.AppliedIdentity, error)
}

// TargetJournalReader reads the exact OA journals named by receipt identities.
// Amounts are compared only in memory and are never returned or persisted by
// this reconciliation package.
type TargetJournalReader interface {
	GetJournalEntry(context.Context, string, string, string) (*accounting.JournalEntry, error)
}

// StreamingProofComputer uses receipt-bound mappings/identities, streams the
// canonical archive, and reads exact POSTED OA journals. It writes no data and
// returns only digest/count-derived proof handles.
type ZeroFileStreamingProofComputer struct {
	archive    ArchiveRecordReader
	mappings   MappingSnapshotReader
	identities AppliedPostingReader
	targets    TargetJournalReader
}

func NewZeroFileStreamingProofComputer(archive ArchiveRecordReader, mappings MappingSnapshotReader, identities AppliedPostingReader, targets TargetJournalReader) *ZeroFileStreamingProofComputer {
	return &ZeroFileStreamingProofComputer{archive: archive, mappings: mappings, identities: identities, targets: targets}
}

func (c *ZeroFileStreamingProofComputer) ComputeProof(ctx context.Context, material ProofMaterial) (ComputedProof, error) {
	if c == nil || c.archive == nil || c.mappings == nil || c.identities == nil || c.targets == nil || !validProofMaterial(material) {
		return ComputedProof{}, ErrInvalid
	}
	mappings, err := c.mappings.ListGLApplyReceiptMappings(ctx, material.TenantID, material.SourceCompanyID, material.PackageID, material.PreviewSHA256)
	if err != nil {
		return ComputedProof{}, err
	}
	identities, err := c.identities.ListGLApplyReceiptIdentities(ctx, material.TenantID, material.SourceCompanyID, material.PackageID, material.PreviewSHA256)
	if err != nil {
		return ComputedProof{}, err
	}
	if mappingDigest, ok := proofMappingsDigest(mappings); !ok || mappingDigest != material.MappingSnapshotSHA256 {
		return ComputedProof{}, ErrConflict
	}
	if identityDigest, ok := proofIdentitiesDigest(identities); !ok || identityDigest != material.AppliedIdentitySHA256 {
		return ComputedProof{}, ErrConflict
	}
	mappingBySource := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		mappingBySource[mapping.SourceAccountExternalID] = mapping.TargetAccountID
	}
	identityByExternal := make(map[string]smartaccountsexecutor.AppliedIdentity, len(identities))
	for _, identity := range identities {
		if _, duplicate := identityByExternal[identity.ExternalID]; duplicate {
			return ComputedProof{}, ErrConflict
		}
		identityByExternal[identity.ExternalID] = identity
	}
	if len(identityByExternal) == 0 {
		return ComputedProof{}, ErrConflict
	}
	seen := make(map[string]struct{}, len(identityByExternal))
	fingerprints := make([]proofJournalFingerprint, 0, len(identityByExternal))
	var sourceBaseDebit, sourceBaseCredit, targetBaseDebit, targetBaseCredit decimal.Decimal
	var sourceOriginalDebit, sourceOriginalCredit, targetOriginalDebit, targetOriginalCredit decimal.Decimal
	err = c.archive.IterateRecords(ctx, material.SchemaName, material.TenantID, material.PackageID, func(raw json.RawMessage) error {
		var record proofArchiveRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return ErrConflict
		}
		if record.EntityType != smartaccountsexecutor.ResourceGeneralLedger {
			return nil
		}
		identity, expected := identityByExternal[record.ExternalID]
		if !expected {
			return ErrConflict
		}
		if _, duplicate := seen[record.ExternalID]; duplicate || record.SourceCompanyID != material.SourceCompanyID || record.Operation != smartaccountsexecutor.OperationUpsert || record.GLPostingMode != smartaccountsexecutor.PostingModeAuthoritativeOnce || record.Revision != identity.Revision || record.Revision != record.PayloadSHA256 || !safeDigest(record.Revision) || record.Journal == nil {
			return ErrConflict
		}
		if !proofPayloadMatches(record.Payload, record.PayloadSHA256) {
			return ErrConflict
		}
		// Financial reconciliation accepts only the reviewed v2 General Ledger
		// CSV canonical form. Capture breadth changes the claim state, never the
		// source-schema gate; historical v1 summary evidence remains read-only.
		if record.Resource != "general_ledger" || record.SourceSchema != "smartaccounts-brave-ui-v2/general_ledger_csv_v1" {
			return ErrConflict
		}
		target, err := c.targets.GetJournalEntry(ctx, material.SchemaName, material.TenantID, identity.JournalID)
		if err != nil || !sameProofJournal(target, identity, record, mappingBySource) {
			return ErrConflict
		}
		sourceTotals, targetTotals, hash, err := proofJournalTotals(record, target, mappingBySource)
		if err != nil {
			return ErrConflict
		}
		sourceBaseDebit = sourceBaseDebit.Add(sourceTotals.baseDebit)
		sourceBaseCredit = sourceBaseCredit.Add(sourceTotals.baseCredit)
		targetBaseDebit = targetBaseDebit.Add(targetTotals.baseDebit)
		targetBaseCredit = targetBaseCredit.Add(targetTotals.baseCredit)
		sourceOriginalDebit = sourceOriginalDebit.Add(sourceTotals.originalDebit)
		sourceOriginalCredit = sourceOriginalCredit.Add(sourceTotals.originalCredit)
		targetOriginalDebit = targetOriginalDebit.Add(targetTotals.originalDebit)
		targetOriginalCredit = targetOriginalCredit.Add(targetTotals.originalCredit)
		seen[record.ExternalID] = struct{}{}
		fingerprints = append(fingerprints, proofJournalFingerprint{ExternalID: record.ExternalID, Revision: record.Revision, JournalID: identity.JournalID, ComparisonSHA256: hash})
		return nil
	})
	if err != nil || len(seen) != len(identityByExternal) || !sourceBaseDebit.Equal(targetBaseDebit) || !sourceBaseCredit.Equal(targetBaseCredit) || !sourceOriginalDebit.Equal(targetOriginalDebit) || !sourceOriginalCredit.Equal(targetOriginalCredit) {
		return ComputedProof{}, ErrConflict
	}
	sort.Slice(fingerprints, func(i, j int) bool { return fingerprints[i].ExternalID < fingerprints[j].ExternalID })
	claimKind := material.ExpectedCoverageState
	if claimKind != "full" && claimKind != "partial" {
		return ComputedProof{}, ErrInvalid
	}
	computedProofDigest := proofDigest("proof", material, fingerprints)
	claimDigest := proofDigest("claim", material, fingerprints)
	coverageDigest := proofDigest("coverage", material, fingerprints)
	return ComputedProof{ProofID: "proof-" + computedProofDigest[:40], ProofSHA256: computedProofDigest, ClaimSHA256: claimDigest, CoverageSHA256: coverageDigest, ClaimKind: claimKind, ExpectedCoverageState: material.ExpectedCoverageState, ToleranceSHA256: material.ToleranceSHA256, VarianceWithinPolicy: true}, nil
}

type proofArchiveRecord struct {
	EntityType      string          `json:"entity_type"`
	Resource        string          `json:"resource"`
	SourceSchema    string          `json:"source_schema"`
	ExternalID      string          `json:"external_id"`
	Revision        string          `json:"revision"`
	Operation       string          `json:"operation"`
	Payload         json.RawMessage `json:"payload"`
	PayloadSHA256   string          `json:"payload_sha256"`
	SourceCompanyID string          `json:"source_company_id"`
	GLPostingMode   string          `json:"gl_posting_mode"`
	Journal         *proofJournal   `json:"journal"`
}
type proofJournal struct {
	PostingDate       string          `json:"posting_date"`
	Currency          string          `json:"currency"`
	ExchangeRate      decimal.Decimal `json:"exchange_rate"`
	DocumentReference string          `json:"document_reference"`
	Rows              []proofRow      `json:"rows"`
}
type proofRow struct {
	SourceAccountExternalID string          `json:"source_account_external_id"`
	Debit                   decimal.Decimal `json:"debit"`
	Credit                  decimal.Decimal `json:"credit"`
	DebitOriginalCurrency   decimal.Decimal `json:"debit_original_currency"`
	CreditOriginalCurrency  decimal.Decimal `json:"credit_original_currency"`
	Description             string          `json:"description"`
}
type proofTotals struct{ baseDebit, baseCredit, originalDebit, originalCredit decimal.Decimal }
type proofJournalFingerprint struct {
	ExternalID       string `json:"external_id"`
	Revision         string `json:"revision"`
	JournalID        string `json:"journal_id"`
	ComparisonSHA256 string `json:"comparison_sha256"`
}

func validProofMaterial(m ProofMaterial) bool {
	return safeID(m.SchemaName) && safeID(m.TenantID) && safeSource(m.SourceCompanyID) && safeID(m.PackageID) && safeUUID(m.PreviewID) && safeDigest(m.PreviewSHA256) && safeDigest(m.ManifestSHA256) && safeDigest(m.RecordsSHA256) && safeDigest(m.ScopeSHA256) && safeDigest(m.MappingSnapshotSHA256) && safeDigest(m.AppliedIdentitySHA256) && safeDigest(m.ToleranceSHA256) && (m.ExpectedCoverageState == "full" || m.ExpectedCoverageState == "partial")
}

func proofMappingsDigest(values []smartaccountsexecutor.AppliedMapping) (string, bool) {
	copyValues := append([]smartaccountsexecutor.AppliedMapping(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool {
		return copyValues[i].SourceAccountExternalID < copyValues[j].SourceAccountExternalID
	})
	if len(copyValues) == 0 {
		return "", false
	}
	for i, value := range copyValues {
		if !safeID(value.SourceAccountExternalID) || !safeID(value.TargetAccountID) || i > 0 && value.SourceAccountExternalID == copyValues[i-1].SourceAccountExternalID {
			return "", false
		}
	}
	b, err := json.Marshal(copyValues)
	if err != nil {
		return "", false
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), true
}

// proofPayloadMatches mirrors the authoritative planner's canonical JSON
// validation. A record with a copied revision string but a different payload
// therefore cannot become reconciliation evidence.
func proofPayloadMatches(payload json.RawMessage, digest string) bool {
	var value any
	if len(payload) == 0 || json.Unmarshal(payload, &value) != nil {
		return false
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return false
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]) == digest
}

func proofIdentitiesDigest(values []smartaccountsexecutor.AppliedIdentity) (string, bool) {
	copyValues := append([]smartaccountsexecutor.AppliedIdentity(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i].ExternalID < copyValues[j].ExternalID })
	if len(copyValues) == 0 {
		return "", false
	}
	for i, value := range copyValues {
		if !safeID(value.ExternalID) || !safeDigest(value.Revision) || !safeUUID(value.ReservationID) || !safeUUID(value.JournalID) || i > 0 && value.ExternalID == copyValues[i-1].ExternalID {
			return "", false
		}
	}
	b, err := json.Marshal(copyValues)
	if err != nil {
		return "", false
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), true
}

func sameProofJournal(entry *accounting.JournalEntry, identity smartaccountsexecutor.AppliedIdentity, record proofArchiveRecord, mappingBySource map[string]string) bool {
	if entry == nil || record.Journal == nil || entry.ID != identity.JournalID || entry.Status != accounting.StatusPosted || entry.SourceType != smartaccountsexecutor.SmartAccountsGLSourceType || entry.SourceID == nil || *entry.SourceID != identity.ReservationID || entry.EntryDate.Format("2006-01-02") != record.Journal.PostingDate || entry.Description != "SmartAccounts GL journal "+record.ExternalID || entry.Reference != record.Journal.DocumentReference || len(entry.Lines) != len(record.Journal.Rows) {
		return false
	}
	for _, row := range record.Journal.Rows {
		if mappingBySource[row.SourceAccountExternalID] == "" {
			return false
		}
	}
	return true
}

func proofJournalTotals(record proofArchiveRecord, entry *accounting.JournalEntry, mappingBySource map[string]string) (proofTotals, proofTotals, string, error) {
	if record.Journal == nil || entry == nil {
		return proofTotals{}, proofTotals{}, "", errors.New("missing journal")
	}
	source := proofTotals{}
	target := proofTotals{}
	expected := make([]string, 0, len(record.Journal.Rows))
	for _, row := range record.Journal.Rows {
		accountID := mappingBySource[row.SourceAccountExternalID]
		if accountID == "" || row.Debit.IsNegative() || row.Credit.IsNegative() || row.Debit.IsPositive() == row.Credit.IsPositive() {
			return proofTotals{}, proofTotals{}, "", errors.New("invalid source row")
		}
		debit, credit := row.Debit, row.Credit
		if record.Journal.Currency != "EUR" {
			debit, credit = row.DebitOriginalCurrency, row.CreditOriginalCurrency
			if debit.IsNegative() || credit.IsNegative() || debit.IsPositive() == credit.IsPositive() || !record.Journal.ExchangeRate.IsPositive() {
				return proofTotals{}, proofTotals{}, "", errors.New("invalid source foreign currency row")
			}
		}
		source.baseDebit = source.baseDebit.Add(row.Debit)
		source.baseCredit = source.baseCredit.Add(row.Credit)
		source.originalDebit = source.originalDebit.Add(debit)
		source.originalCredit = source.originalCredit.Add(credit)
		expected = append(expected, proofLineFingerprint(accountID, row.Description, debit, credit, record.Journal.Currency, proofExchangeRate(record.Journal.Currency, record.Journal.ExchangeRate), debit.Mul(proofExchangeRate(record.Journal.Currency, record.Journal.ExchangeRate)), credit.Mul(proofExchangeRate(record.Journal.Currency, record.Journal.ExchangeRate)), decimal.Zero, false))
	}
	actual := make([]string, 0, len(entry.Lines))
	for _, line := range entry.Lines {
		if line.DebitAmount.IsNegative() || line.CreditAmount.IsNegative() || line.DebitAmount.IsPositive() == line.CreditAmount.IsPositive() || !line.VATRate.IsZero() || line.IsVATInclusive {
			return proofTotals{}, proofTotals{}, "", errors.New("invalid target row")
		}
		target.baseDebit = target.baseDebit.Add(line.BaseDebit)
		target.baseCredit = target.baseCredit.Add(line.BaseCredit)
		target.originalDebit = target.originalDebit.Add(line.DebitAmount)
		target.originalCredit = target.originalCredit.Add(line.CreditAmount)
		actual = append(actual, proofLineFingerprint(line.AccountID, line.Description, line.DebitAmount, line.CreditAmount, line.Currency, line.ExchangeRate, line.BaseDebit, line.BaseCredit, line.VATRate, line.IsVATInclusive))
	}
	sort.Strings(expected)
	sort.Strings(actual)
	if strings.Join(expected, "\x00") != strings.Join(actual, "\x00") || !source.baseDebit.Equal(source.baseCredit) || !target.baseDebit.Equal(target.baseCredit) || !source.originalDebit.Equal(source.originalCredit) || !target.originalDebit.Equal(target.originalCredit) {
		return proofTotals{}, proofTotals{}, "", errors.New("journal comparison mismatch")
	}
	encoded, err := json.Marshal(struct {
		ExternalID string   `json:"external_id"`
		Revision   string   `json:"revision"`
		Lines      []string `json:"lines"`
	}{record.ExternalID, record.Revision, expected})
	if err != nil {
		return proofTotals{}, proofTotals{}, "", err
	}
	hash := sha256.Sum256(encoded)
	return source, target, hex.EncodeToString(hash[:]), nil
}

func proofExchangeRate(currency string, rate decimal.Decimal) decimal.Decimal {
	if currency == "EUR" {
		return decimal.NewFromInt(1)
	}
	return rate
}

func proofLineFingerprint(accountID, description string, debit, credit decimal.Decimal, currency string, rate, baseDebit, baseCredit, vatRate decimal.Decimal, vatInclusive bool) string {
	return strings.Join([]string{accountID, description, debit.String(), credit.String(), currency, rate.String(), baseDebit.String(), baseCredit.String(), vatRate.String(), fmt.Sprintf("%t", vatInclusive)}, "\x1f")
}

func proofDigest(kind string, material ProofMaterial, fingerprints []proofJournalFingerprint) string {
	encoded, err := json.Marshal(struct {
		Kind        string                    `json:"kind"`
		ManifestSHA string                    `json:"manifest_sha256"`
		RecordsSHA  string                    `json:"records_sha256"`
		ScopeSHA    string                    `json:"scope_sha256"`
		MappingSHA  string                    `json:"mapping_snapshot_sha256"`
		IdentitySHA string                    `json:"applied_identity_sha256"`
		Coverage    string                    `json:"expected_coverage_state"`
		Journals    []proofJournalFingerprint `json:"journals"`
	}{kind, material.ManifestSHA256, material.RecordsSHA256, material.ScopeSHA256, material.MappingSnapshotSHA256, material.AppliedIdentitySHA256, material.ExpectedCoverageState, fingerprints})
	if err != nil {
		panic(fmt.Sprintf("canonical reconciliation proof digest: %v", err))
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

var _ StreamingProofComputer = (*ZeroFileStreamingProofComputer)(nil)
