package smartaccountsreconciliation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/smartaccountsexecutor"
	"github.com/shopspring/decimal"
)

type proofArchive struct{ records []json.RawMessage }

func (a proofArchive) IterateRecords(_ context.Context, _, _, _ string, visit func(json.RawMessage) error) error {
	for _, record := range a.records {
		if err := visit(record); err != nil {
			return err
		}
	}
	return nil
}

type proofReceiptSnapshot struct {
	mappings   []smartaccountsexecutor.AppliedMapping
	identities []smartaccountsexecutor.AppliedIdentity
}

func (s proofReceiptSnapshot) ListGLApplyReceiptMappings(context.Context, string, string, string, string) ([]smartaccountsexecutor.AppliedMapping, error) {
	return append([]smartaccountsexecutor.AppliedMapping(nil), s.mappings...), nil
}
func (s proofReceiptSnapshot) ListGLApplyReceiptIdentities(context.Context, string, string, string, string) ([]smartaccountsexecutor.AppliedIdentity, error) {
	return append([]smartaccountsexecutor.AppliedIdentity(nil), s.identities...), nil
}

type proofTargets map[string]*accounting.JournalEntry

func (t proofTargets) GetJournalEntry(_ context.Context, _, _, id string) (*accounting.JournalEntry, error) {
	entry := t[id]
	if entry == nil {
		return nil, errors.New("target not found")
	}
	return entry, nil
}

func TestZeroFileStreamingProofComputesOnlyDigestBoundEvidence(t *testing.T) {
	material, archive, snapshot, targets := validProofFixture(t)
	proof, err := NewZeroFileStreamingProofComputer(archive, snapshot, snapshot, targets).ComputeProof(context.Background(), material)
	if err != nil || proof.ProofID == "" || proof.ClaimKind != "full" || !proof.VarianceWithinPolicy {
		t.Fatalf("proof = %#v / %v", proof, err)
	}
	encoded, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"account-1", "debit", "credit", "journal-one"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("proof exposed financial/source data %q in %s", forbidden, encoded)
		}
	}
}

func TestZeroFileStreamingProofFailsClosedForChangedTargetOrReceiptIdentity(t *testing.T) {
	material, archive, snapshot, targets := validProofFixture(t)
	targets["22222222-2222-2222-2222-222222222222"].Lines[0].DebitAmount = decimal.NewFromInt(9)
	if _, err := NewZeroFileStreamingProofComputer(archive, snapshot, snapshot, targets).ComputeProof(context.Background(), material); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed target proof = %v", err)
	}
	material, archive, snapshot, targets = validProofFixture(t)
	snapshot.identities[0].ReservationID = "33333333-3333-3333-3333-333333333333"
	if _, err := NewZeroFileStreamingProofComputer(archive, snapshot, snapshot, targets).ComputeProof(context.Background(), material); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed receipt identity proof = %v", err)
	}
	material, archive, snapshot, targets = validProofFixture(t)
	archive.records[0] = json.RawMessage(strings.Replace(string(archive.records[0]), "proof-fixture", "forged-payload", 1))
	if _, err := NewZeroFileStreamingProofComputer(archive, snapshot, snapshot, targets).ComputeProof(context.Background(), material); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed canonical payload proof = %v", err)
	}
	material, archive, snapshot, targets = validProofFixture(t)
	targets["22222222-2222-2222-2222-222222222222"].Lines[0].VATRate = decimal.NewFromInt(20)
	if _, err := NewZeroFileStreamingProofComputer(archive, snapshot, snapshot, targets).ComputeProof(context.Background(), material); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed target VAT proof = %v", err)
	}
}

func TestZeroFileStreamingProofCarriesPartialClaimWithoutPromotingIt(t *testing.T) {
	material, archive, snapshot, targets := validProofFixture(t)
	material.ExpectedCoverageState = "partial"
	proof, err := NewZeroFileStreamingProofComputer(archive, snapshot, snapshot, targets).ComputeProof(context.Background(), material)
	if err != nil || proof.ClaimKind != "partial" || proof.ExpectedCoverageState != "partial" {
		t.Fatalf("partial proof = %#v / %v", proof, err)
	}
}

func validProofFixture(t *testing.T) (ProofMaterial, proofArchive, proofReceiptSnapshot, proofTargets) {
	t.Helper()
	reservationID := "11111111-1111-1111-1111-111111111111"
	journalID := "22222222-2222-2222-2222-222222222222"
	mappings := []smartaccountsexecutor.AppliedMapping{{SourceAccountExternalID: "source-debit", TargetAccountID: "account-1"}, {SourceAccountExternalID: "source-credit", TargetAccountID: "account-2"}}
	payload, err := json.Marshal(map[string]any{"source": "proof-fixture"})
	if err != nil {
		t.Fatal(err)
	}
	payloadSum := sha256.Sum256(payload)
	revision := hex.EncodeToString(payloadSum[:])
	identities := []smartaccountsexecutor.AppliedIdentity{{ExternalID: "journal-one", Revision: revision, ReservationID: reservationID, JournalID: journalID}}
	mappingSHA, ok := proofMappingsDigest(mappings)
	if !ok {
		t.Fatal("mapping fixture invalid")
	}
	identitySHA, ok := proofIdentitiesDigest(identities)
	if !ok {
		t.Fatal("identity fixture invalid")
	}
	record, err := json.Marshal(map[string]any{"entity_type": "general_ledger_journal", "resource": "general_ledger", "source_schema": "smartaccounts-brave-ui-v2/general_ledger_csv_v1", "external_id": "journal-one", "revision": revision, "operation": "upsert", "payload": json.RawMessage(payload), "payload_sha256": revision, "source_company_id": "sa-browser-v1-1234", "gl_posting_mode": "authoritative_once", "journal": map[string]any{"posting_date": "2026-08-28", "currency": "EUR", "document_reference": "doc-1", "rows": []any{map[string]any{"source_account_external_id": "source-debit", "debit": "5", "credit": "0", "description": "debit"}, map[string]any{"source_account_external_id": "source-credit", "debit": "0", "credit": "5", "description": "credit"}}}})
	if err != nil {
		t.Fatal(err)
	}
	sourceID := reservationID
	target := &accounting.JournalEntry{ID: journalID, EntryDate: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), Description: "SmartAccounts GL journal journal-one", Reference: "doc-1", SourceType: smartaccountsexecutor.SmartAccountsGLSourceType, SourceID: &sourceID, Status: accounting.StatusPosted, Lines: []accounting.JournalEntryLine{
		{AccountID: "account-1", Description: "debit", DebitAmount: decimal.NewFromInt(5), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseDebit: decimal.NewFromInt(5)},
		{AccountID: "account-2", Description: "credit", CreditAmount: decimal.NewFromInt(5), Currency: "EUR", ExchangeRate: decimal.NewFromInt(1), BaseCredit: decimal.NewFromInt(5)},
	}}
	return ProofMaterial{SchemaName: "tenant_schema", TenantID: "tenant-1", SourceCompanyID: "sa-browser-v1-1234", PackageID: "package-1", ManifestSHA256: strings.Repeat("b", 64), RecordsSHA256: strings.Repeat("c", 64), ScopeSHA256: strings.Repeat("d", 64), MappingSnapshotSHA256: mappingSHA, AppliedIdentitySHA256: identitySHA, ToleranceSHA256: strings.Repeat("e", 64), PreviewID: "33333333-3333-3333-3333-333333333333", PreviewSHA256: strings.Repeat("f", 64), ExpectedCoverageState: "full"}, proofArchive{records: []json.RawMessage{record}}, proofReceiptSnapshot{mappings: mappings, identities: identities}, proofTargets{journalID: target}
}
