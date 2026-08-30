package smartaccountsexecutor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/HMB-research/open-accounting/internal/importdelivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeArchive struct {
	status   importdelivery.Status
	manifest importdelivery.Manifest
	records  []json.RawMessage
}

func (f fakeArchive) GetStatus(context.Context, string, string, string) (importdelivery.Status, error) {
	return f.status, nil
}
func (f fakeArchive) GetManifest(context.Context, string, string, string) (importdelivery.Manifest, error) {
	return f.manifest, nil
}
func (f fakeArchive) IterateRecords(_ context.Context, _ string, _ string, _ string, visit func(json.RawMessage) error) error {
	for _, record := range f.records {
		if err := visit(record); err != nil {
			return err
		}
	}
	return nil
}

type fakeAccounts map[string]bool

func (a fakeAccounts) ResolveAccount(_ context.Context, _, _, id string) error {
	if a[id] {
		return nil
	}
	return ErrPreviewReviewRequired
}

type fakePostings map[string]*PostedIdentity

func (p fakePostings) GetPostedIdentity(_ context.Context, _, _, _, _, _, id string) (*PostedIdentity, error) {
	return p[id], nil
}

type staticCaptureCoverage struct {
	coverage CaptureCoverage
	err      error
}

func (s staticCaptureCoverage) AssessCaptureCoverage(context.Context, string, string, string) (CaptureCoverage, error) {
	return s.coverage, s.err
}

func canonicalDigest(t *testing.T, value any) (json.RawMessage, string) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(b)
	return b, hex.EncodeToString(s[:])
}

func authoritativeRecord(t *testing.T, source string) json.RawMessage {
	t.Helper()
	payload, digest := canonicalDigest(t, map[string]any{"id": "journal-1", "rows": []any{"source"}})
	record := map[string]any{
		"entity_type": ResourceGeneralLedger, "external_id": "journal-1", "revision": digest,
		"operation": OperationUpsert, "payload": json.RawMessage(payload), "payload_sha256": digest,
		"source_company_id": source, "gl_posting_mode": PostingModeAuthoritativeOnce,
		"journal": map[string]any{"posting_date": "2026-01-31", "currency": "EUR", "rows": []any{
			map[string]any{"source_account_external_id": "1000", "debit": "10", "credit": "0"},
			map[string]any{"source_account_external_id": "3000", "debit": "0", "credit": "10"},
		}},
	}
	b, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func browserGeneralLedgerRecord(t *testing.T, source, resource, sourceSchema string) json.RawMessage {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(authoritativeRecord(t, source), &record); err != nil {
		t.Fatal(err)
	}
	record["resource"] = resource
	record["source_schema"] = sourceSchema
	b, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func testPlanner(t *testing.T, record json.RawMessage, postings fakePostings) *Planner {
	t.Helper()
	source := "sa-key-v1-test"
	return NewPlanner(fakeArchive{
		status:   importdelivery.Status{Status: importdelivery.StatusStagedReview, SourceCompanyID: source},
		manifest: importdelivery.Manifest{Provider: Provider, SourceCompanyID: source, Authority: importdelivery.Authority{GeneralLedgerAuthority: Provider, SmartAccountsGLAuthoritative: true}},
		records:  []json.RawMessage{record},
	}, postings, fakeAccounts{"oa-1000": true, "oa-3000": true}).SetCaptureCoverageReader(staticCaptureCoverage{coverage: CaptureCoverage{Complete: true}})
}

func testBrowserPlanner(t *testing.T, record json.RawMessage) *Planner {
	t.Helper()
	source := "sa-key-v1-test"
	return NewPlanner(fakeArchive{
		status: importdelivery.Status{Status: importdelivery.StatusStagedReview, SourceCompanyID: source},
		manifest: importdelivery.Manifest{
			Provider: Provider, SourceCompanyID: source,
			Authority: importdelivery.Authority{GeneralLedgerAuthority: Provider, SmartAccountsGLAuthoritative: true},
			Scope:     importdelivery.Scope{Mode: "partial_browser_capture"},
		},
		records: []json.RawMessage{record},
	}, nil, fakeAccounts{"oa-1000": true, "oa-3000": true}).SetCaptureCoverageReader(staticCaptureCoverage{coverage: CaptureCoverage{Complete: true}})
}

func TestPreviewPlansOnlyBalancedAuthoritativeJournals(t *testing.T) {
	plan, err := testPlanner(t, authoritativeRecord(t, "sa-key-v1-test"), nil).Preview(context.Background(), "tenant_a", "tenant-id", "package-id", PreviewRequest{AccountMappings: []AccountMapping{{"1000", "oa-1000"}, {"3000", "oa-3000"}}})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if plan.Status != PlanStatusPreviewReady || !plan.FinancialWritesPlanned || len(plan.Journals) != 1 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if plan.Journals[0].Action != "CREATE_AND_POST" {
		t.Fatalf("action = %s", plan.Journals[0].Action)
	}
}

func TestPreviewBrowserGLRequiresReviewedGeneralLedgerCSVSource(t *testing.T) {
	t.Run("reviewed general ledger CSV may plan", func(t *testing.T) {
		plan, err := testBrowserPlanner(t, browserGeneralLedgerRecord(t, "sa-key-v1-test", browserGeneralLedgerResourceID, browserGeneralLedgerSourceSchema)).Preview(context.Background(), "tenant", "tenant-id", "package-id", PreviewRequest{AccountMappings: []AccountMapping{{"1000", "oa-1000"}, {"3000", "oa-3000"}}})
		require.NoError(t, err)
		assert.Equal(t, PlanStatusPreviewReady, plan.Status)
		assert.True(t, plan.FinancialWritesPlanned)
		assert.Len(t, plan.Journals, 1)
	})
	for _, test := range []struct {
		name, resource, schema string
	}{
		{name: "journal summary is archive only", resource: "journal_entries", schema: browserGeneralLedgerSourceSchema},
		{name: "wrong source schema", resource: browserGeneralLedgerResourceID, schema: "smartaccounts-brave-ui-v1/journal_entries_csv_v1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := testBrowserPlanner(t, browserGeneralLedgerRecord(t, "sa-key-v1-test", test.resource, test.schema)).Preview(context.Background(), "tenant", "tenant-id", "package-id", PreviewRequest{AccountMappings: []AccountMapping{{"1000", "oa-1000"}, {"3000", "oa-3000"}}})
			require.ErrorIs(t, err, ErrPreviewReviewRequired)
			assert.Equal(t, PlanStatusReviewRequired, plan.Status)
			assert.False(t, plan.FinancialWritesPlanned)
			assert.Empty(t, plan.Journals)
			require.Len(t, plan.Issues, 1)
			assert.Equal(t, "browser_gl_source_contract_invalid", plan.Issues[0].Code)
		})
	}
}

func TestPreviewRejectsSourceBindingAndUnbalancedJournal(t *testing.T) {
	record := authoritativeRecord(t, "different-source")
	plan, err := testPlanner(t, record, nil).Preview(context.Background(), "tenant_a", "tenant-id", "package-id", PreviewRequest{})
	if err == nil || plan.Status != PlanStatusReviewRequired {
		t.Fatalf("expected review gate, got %#v / %v", plan, err)
	}
	if len(plan.Issues) == 0 || plan.Issues[0].Code != "source_binding_mismatch" {
		t.Fatalf("issues = %#v", plan.Issues)
	}
}

func TestPreviewTreatsReplayAndTombstoneAsNonPosting(t *testing.T) {
	record := authoritativeRecord(t, "sa-key-v1-test")
	var decoded map[string]any
	if err := json.Unmarshal(record, &decoded); err != nil {
		t.Fatal(err)
	}
	digest := decoded["revision"].(string)
	plan, err := testPlanner(t, record, fakePostings{"journal-1": {ExternalID: "journal-1", Revision: digest, Status: PlanStatusApplied}}).Preview(context.Background(), "tenant_a", "tenant-id", "package-id", PreviewRequest{})
	if err != nil {
		t.Fatalf("replay should be safe: %v", err)
	}
	if len(plan.Journals) != 1 || plan.Journals[0].Action != "ALREADY_APPLIED" || plan.FinancialWritesPlanned {
		t.Fatalf("replay plan = %#v", plan)
	}
	decoded["operation"] = OperationTombstone
	tombstone, _ := json.Marshal(decoded)
	plan, err = testPlanner(t, tombstone, nil).Preview(context.Background(), "tenant_a", "tenant-id", "package-id", PreviewRequest{})
	if err == nil || len(plan.Journals) != 0 {
		t.Fatalf("tombstone must be review-only: %#v / %v", plan, err)
	}
}

func sourceAccountRecord(t *testing.T, id, code, kind, et string) json.RawMessage {
	t.Helper()
	payload, _ := json.Marshal(map[string]string{"id": id, "code": code, "type": kind, "descriptionEt": et})
	record, _ := json.Marshal(map[string]any{"entity_type": "account", "external_id": id, "source_company_id": "sa-key-v1-test", "payload": json.RawMessage(payload)})
	return record
}

func TestPreviewUseSourceChartProposesMappedImportsOnlyAfterReview(t *testing.T) {
	source := "sa-key-v1-test"
	planner := NewPlanner(fakeArchive{status: importdelivery.Status{Status: importdelivery.StatusStagedReview, SourceCompanyID: source}, manifest: importdelivery.Manifest{Provider: Provider, SourceCompanyID: source, Authority: importdelivery.Authority{GeneralLedgerAuthority: Provider, SmartAccountsGLAuthoritative: true}}, records: []json.RawMessage{sourceAccountRecord(t, "1000", "1000", "ASSET", "Pank"), sourceAccountRecord(t, "3000", "3000", "INCOME", "Müük"), authoritativeRecord(t, source)}}, nil, fakeAccounts{}).SetCaptureCoverageReader(staticCaptureCoverage{coverage: CaptureCoverage{Complete: true}}).SetAccountCatalog(AccountCatalogFunc(func(context.Context, string, string) ([]ChartAccount, error) { return nil, nil }))
	plan, err := planner.Preview(context.Background(), "tenant", "tenant-id", "package", PreviewRequest{UseSourceChart: true})
	if err != nil || plan.Status != PlanStatusPreviewReady || len(plan.AccountImports) != 2 || len(plan.Journals) != 1 {
		t.Fatalf("source chart plan: %#v / %v", plan, err)
	}
	if plan.AccountImports[1].AccountType != "REVENUE" {
		t.Fatalf("INCOME was not mapped to REVENUE: %#v", plan.AccountImports)
	}
}

func TestPreviewUseSourceChartRejectsUnknownTypeAndCollision(t *testing.T) {
	source := "sa-key-v1-test"
	planner := NewPlanner(fakeArchive{status: importdelivery.Status{Status: importdelivery.StatusStagedReview, SourceCompanyID: source}, manifest: importdelivery.Manifest{Provider: Provider, SourceCompanyID: source, Authority: importdelivery.Authority{GeneralLedgerAuthority: Provider, SmartAccountsGLAuthoritative: true}}, records: []json.RawMessage{sourceAccountRecord(t, "1000", "1000", "UNKNOWN", "Pank")}}, nil, fakeAccounts{}).SetCaptureCoverageReader(staticCaptureCoverage{coverage: CaptureCoverage{Complete: true}}).SetAccountCatalog(AccountCatalogFunc(func(context.Context, string, string) ([]ChartAccount, error) {
		return []ChartAccount{{Code: "1000", Name: "Pank"}}, nil
	}))
	plan, err := planner.Preview(context.Background(), "tenant", "tenant-id", "package", PreviewRequest{UseSourceChart: true})
	if err == nil || plan.Status != PlanStatusReviewRequired || len(plan.Issues) == 0 {
		t.Fatalf("invalid source chart must be review-gated: %#v / %v", plan, err)
	}
}
