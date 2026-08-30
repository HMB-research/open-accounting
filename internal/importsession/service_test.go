package importsession

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	receipts map[string]Receipt
	bindings map[string]string
	creates  int
}

type recordingAccountResolver struct {
	calls []string
	errs  map[string]error
}

func (r *recordingAccountResolver) ResolveAccount(_ context.Context, _, tenantID, accountID string) error {
	r.calls = append(r.calls, tenantID+"\x00"+accountID)
	return r.errs[accountID]
}

// createRaceStore models the repository path where another request inserts an
// identical receipt after FindByPackage reports not found but before Create.
type createRaceStore struct {
	memoryStore
}

func (s *createRaceStore) FindByPackage(_ context.Context, _, _, _, _, _ string) (Receipt, error) {
	return Receipt{}, ErrImportSessionNotFound
}

func (s *createRaceStore) Create(_ context.Context, _, _ string, receipt Receipt) (Receipt, error) {
	s.creates++
	receipt.ID = "concurrent-session"
	receipt.Created = false
	return receipt, nil
}

func smartAccountsGLAuthority() *LedgerAuthorityDeclaration {
	authoritative := true
	return &LedgerAuthorityDeclaration{
		GeneralLedgerAuthority:       ProviderSmartAccounts,
		SmartAccountsGLAuthoritative: &authoritative,
		SourceAsOfDate:               "2026-08-31",
	}
}

func (s *memoryStore) EnsureSourceCompanyBinding(_ context.Context, tenantID, provider, sourceCompanyID string) error {
	if s.bindings == nil {
		s.bindings = map[string]string{}
	}
	key := provider + "\x00" + sourceCompanyID
	if boundTenantID, exists := s.bindings[key]; exists && boundTenantID != tenantID {
		return ErrSourceCompanyBoundToOtherTenant
	}
	s.bindings[key] = tenantID
	return nil
}

func (s *memoryStore) Create(_ context.Context, _, _ string, receipt Receipt) (Receipt, error) {
	if s.receipts == nil {
		s.receipts = map[string]Receipt{}
	}
	s.creates++
	receipt.ID = "session-1"
	s.receipts[receipt.PackageSHA256] = receipt
	return receipt, nil
}

func (s *memoryStore) FindByPackage(_ context.Context, _, _, _, _, packageSHA256 string) (Receipt, error) {
	receipt, ok := s.receipts[packageSHA256]
	if !ok {
		return Receipt{}, ErrImportSessionNotFound
	}
	return receipt, nil
}

func (s *memoryStore) Get(_ context.Context, _, _, sessionID string) (Receipt, error) {
	for _, receipt := range s.receipts {
		if receipt.ID == sessionID {
			return receipt, nil
		}
	}
	return Receipt{}, ErrImportSessionNotFound
}

func (s *memoryStore) ListLedgerPlanInputs(_ context.Context, _, tenantID, provider, sourceCompanyID, excludeSessionID string) ([]StagedLedgerJournal, error) {
	var journals []StagedLedgerJournal
	for _, receipt := range s.receipts {
		if receipt.ID == excludeSessionID || receipt.TenantID != tenantID || receipt.Provider != provider || receipt.SourceCompanyID != sourceCompanyID {
			continue
		}
		journals = append(journals, cloneStagedJournals(receipt.LedgerPlanInput)...)
	}
	return journals, nil
}

func validPackage(t *testing.T) CanonicalPackage {
	t.Helper()
	pkg := CanonicalPackage{
		SchemaVersion:   CanonicalSchemaVersionV1,
		Provider:        ProviderSmartAccounts,
		SourceCompanyID: "source-company-test-001",
		LedgerAuthority: smartAccountsGLAuthority(),
		Scope:           &ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}},
		Records: []CanonicalRecord{{
			EntityType: "account",
			ExternalID: "1000",
			Revision:   "2026-08-27T10:00:00Z",
			Operation:  "upsert",
			Payload:    []byte(`{"name":"Cash","code":"1000"}`),
		}},
	}
	canonicalPayload, err := canonicalPayloadJSON(pkg.Records[0].Payload)
	require.NoError(t, err)
	pkg.Records[0].PayloadSHA256 = sha256Hex(canonicalPayload)
	digest, err := PackageDigest(pkg)
	require.NoError(t, err)
	pkg.PackageSHA256 = digest
	return pkg
}

func validJournalPackage(t *testing.T, scope ImportScope) CanonicalPackage {
	t.Helper()
	pkg := CanonicalPackage{
		SchemaVersion:   CanonicalSchemaVersionV1,
		Provider:        ProviderSmartAccounts,
		SourceCompanyID: "source-company-test-001",
		LedgerAuthority: smartAccountsGLAuthority(),
		Scope:           &scope,
		Records: []CanonicalRecord{{
			EntityType: "journal_entry",
			ExternalID: "JE-100",
			Revision:   "2026-08-27T10:00:00Z",
			Operation:  "upsert",
			Payload: []byte(`{
"journal_group_id":"JE-100",
"period_start":"2026-08-01",
"period_end":"2026-08-31",
"currency":"EUR",
"lines":[
{"account_external_id":"1000","debit":"100.00","credit":"0.00"},
{"account_external_id":"3000","debit":"0.00","credit":"100.00"}
]}`),
		}},
	}
	refreshPackageDigest(t, &pkg)
	return pkg
}

func refreshPackageDigest(t *testing.T, pkg *CanonicalPackage) {
	t.Helper()
	for index := range pkg.Records {
		if pkg.Records[index].Operation != "upsert" {
			continue
		}
		payload, err := canonicalPayloadJSON(pkg.Records[index].Payload)
		require.NoError(t, err)
		pkg.Records[index].PayloadSHA256 = sha256Hex(payload)
	}
	digest, err := PackageDigest(*pkg)
	require.NoError(t, err)
	pkg.PackageSHA256 = digest
}

func hasValidationIssue(report ValidationReport, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestValidatePackageAcceptsCanonicalSmartAccountsV1(t *testing.T) {
	service := NewService(nil)
	report := service.ValidatePackage(validPackage(t))

	assert.True(t, report.Ready)
	assert.Equal(t, 1, report.RecordCount)
	assert.Equal(t, map[string]int{"account": 1}, report.EntityCounts)
	assert.Empty(t, report.Issues)
}

func TestValidatePackageRejectsUnsafeOrInvalidMetadata(t *testing.T) {
	pkg := validPackage(t)
	pkg.Records = append(pkg.Records, CanonicalRecord{
		EntityType: "account",
		ExternalID: "1000",
		Revision:   "next",
		Operation:  "delete",
	})
	pkg.PackageSHA256 = "not-a-digest"

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.Len(t, report.Issues, 2)
	assert.Equal(t, "duplicate_source_record", report.Issues[0].Code)
	assert.Equal(t, "invalid_sha256", report.Issues[1].Code)
}

func TestValidatePackageAllowsDeleteWithoutPayload(t *testing.T) {
	pkg := CanonicalPackage{
		SchemaVersion:   CanonicalSchemaVersionV1,
		Provider:        ProviderSmartAccounts,
		SourceCompanyID: "source-company-test-001",
		LedgerAuthority: smartAccountsGLAuthority(),
		Scope:           &ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}},
		Records: []CanonicalRecord{{
			EntityType: "contact",
			ExternalID: "42",
			Revision:   "deleted-2026-08-27",
			Operation:  "delete",
		}},
	}
	digest, err := PackageDigest(pkg)
	require.NoError(t, err)
	pkg.PackageSHA256 = digest

	report := NewService(nil).ValidatePackage(pkg)
	assert.True(t, report.Ready)
}

func TestValidatePackageVerifiesBalancedCanonicalJournalGroup(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{
		Mode:          ScopeModePartial,
		ResourceTypes: []string{ScopeResourceJournalEntry},
		PeriodStart:   "2026-08-01",
		PeriodEnd:     "2026-08-31",
	})

	report := NewService(nil).ValidatePackage(pkg)

	require.True(t, report.Ready)
	require.NotNil(t, report.LedgerVerification)
	assert.Equal(t, ProviderSmartAccounts, report.LedgerVerification.GeneralLedgerAuthority)
	assert.Equal(t, ScopeModePartial, report.LedgerVerification.ScopeMode)
	assert.Equal(t, 1, report.LedgerVerification.JournalGroupCount)
	assert.Equal(t, 1, report.LedgerVerification.BalancedJournalGroupCount)
	assert.True(t, report.LedgerVerification.JournalStagingAllowed)
	assert.False(t, report.LedgerVerification.FinancialPostingPlanAllowed)
	assert.False(t, report.LedgerVerification.ReviewRequired)
	assert.Equal(t, LedgerVerificationStatusVerified, report.LedgerVerification.VerificationStatus)
}

func TestValidatePackageRejectsUnbalancedCanonicalJournalGroup(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})
	pkg.Records[0].Payload = []byte(`{
"journal_group_id":"JE-100",
"period_start":"2026-08-01",
"period_end":"2026-08-31",
"currency":"EUR",
"lines":[
{"account_external_id":"1000","debit":"100.00","credit":"0.00"},
{"account_external_id":"3000","debit":"0.00","credit":"99.99"}
]}`)
	refreshPackageDigest(t, &pkg)

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.True(t, hasValidationIssue(report, "unbalanced_journal_group"))
	assert.Equal(t, 1, report.LedgerVerification.JournalGroupCount)
	assert.Zero(t, report.LedgerVerification.BalancedJournalGroupCount)
}

func TestValidatePackageRejectsPartialScopeBoundaryAndUnsupportedSubset(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{
		Mode:          ScopeModePartial,
		ResourceTypes: []string{ScopeResourceJournalEntry, "contact"},
		PeriodStart:   "2026-08-01",
		PeriodEnd:     "2026-08-31",
	})
	pkg.Records[0].Payload = []byte(`{
"journal_group_id":"JE-100",
"period_start":"2026-07-31",
"period_end":"2026-08-01",
"currency":"EUR",
"lines":[
{"account_external_id":"1000","debit":"100.00","credit":"0.00"},
{"account_external_id":"3000","debit":"0.00","credit":"100.00"}
]}`)
	pkg.Records = append(pkg.Records, CanonicalRecord{
		EntityType: "contact",
		ExternalID: "42",
		Revision:   "2026-08-27T10:00:00Z",
		Operation:  "upsert",
		Payload:    []byte(`{"name":"Customer"}`),
	})
	refreshPackageDigest(t, &pkg)

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.True(t, hasValidationIssue(report, "unsupported_resource_subset"))
	assert.True(t, hasValidationIssue(report, "journal_group_outside_scope"))
	assert.True(t, hasValidationIssue(report, "scope_resource_outside_subset"))
}

func TestValidatePackageAcceptsFullScopeJournalWithoutPeriodBoundary(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})

	report := NewService(nil).ValidatePackage(pkg)

	assert.True(t, report.Ready)
	assert.Equal(t, ScopeModeFull, report.LedgerVerification.ScopeMode)
}

func TestValidatePackageRejectsPeriodBoundaryForFullScope(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{
		Mode:          ScopeModeFull,
		ResourceTypes: []string{ScopeResourceAll},
		PeriodStart:   "2026-08-01",
		PeriodEnd:     "2026-08-31",
	})

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.True(t, hasValidationIssue(report, "scope_period_not_allowed"))
}

func TestValidatePackageRejectsJournalGroupAfterSourceCutoff(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})
	pkg.Records[0].Payload = []byte(`{
"journal_group_id":"JE-100",
"period_start":"2026-09-01",
"period_end":"2026-09-01",
"currency":"EUR",
"lines":[
{"account_external_id":"1000","debit":"100.00","credit":"0.00"},
{"account_external_id":"3000","debit":"0.00","credit":"100.00"}
]}`)
	refreshPackageDigest(t, &pkg)

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.True(t, hasValidationIssue(report, "journal_group_after_source_as_of"))
}

func TestValidatePackageRequiresSourceLedgerAuthorityDeclaration(t *testing.T) {
	pkg := validPackage(t)
	pkg.LedgerAuthority = nil
	refreshPackageDigest(t, &pkg)

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.True(t, hasValidationIssue(report, "required"))
}

func TestValidatePackageRequiresConfirmedGLAuthorityAndSourceAsOfDate(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})
	authoritative := false
	pkg.LedgerAuthority.SmartAccountsGLAuthoritative = &authoritative
	pkg.LedgerAuthority.SourceAsOfDate = ""
	refreshPackageDigest(t, &pkg)

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.True(t, hasValidationIssue(report, "ledger_authority_not_confirmed"))
	assert.True(t, hasValidationIssue(report, "invalid_source_as_of_date"))
	assert.False(t, report.LedgerVerification.JournalStagingAllowed)
	assert.Empty(t, report.LedgerVerification.VerificationStatus)
}

func TestValidatePackageBlocksDuplicateInvoiceAndPaymentPostingPlans(t *testing.T) {
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})
	pkg.Records = append(pkg.Records,
		CanonicalRecord{EntityType: "sales_invoice", ExternalID: "INV-100", Revision: "2026-08-27T10:00:00Z", Operation: "upsert", Payload: []byte(`{"number":"INV-100"}`)},
		CanonicalRecord{EntityType: "purchase_invoice", ExternalID: "BILL-100", Revision: "2026-08-27T10:00:00Z", Operation: "upsert", Payload: []byte(`{"number":"BILL-100"}`)},
		CanonicalRecord{EntityType: "payment", ExternalID: "PAY-100", Revision: "2026-08-27T10:00:00Z", Operation: "upsert", Payload: []byte(`{"reference":"PAY-100"}`)},
	)
	refreshPackageDigest(t, &pkg)

	report := NewService(nil).ValidatePackage(pkg)

	assert.False(t, report.Ready)
	assert.True(t, hasValidationIssue(report, "duplicate_financial_posting_plan"))
	assert.True(t, report.LedgerVerification.JournalStagingAllowed)
	assert.False(t, report.LedgerVerification.FinancialPostingPlanAllowed)
}

func TestReceivePersistsReceiptOnlyAndIsIdempotent(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	fixedTime := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedTime }
	pkg := validPackage(t)

	first, report, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", pkg)
	require.NoError(t, err)
	assert.True(t, report.Ready)
	assert.True(t, first.Created)
	assert.Equal(t, SessionStatusReceivedValidated, first.Status)
	assert.Equal(t, "session-1", first.ID)
	assert.Equal(t, fixedTime, first.CreatedAt)
	assert.Equal(t, 1, store.creates)

	second, report, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-2", pkg)
	require.NoError(t, err)
	assert.True(t, report.Ready)
	assert.False(t, second.Created)
	assert.Equal(t, "user-1", second.CreatedBy)
	assert.Equal(t, 1, store.creates)

	got, err := service.Get(context.Background(), "tenant_import_test", "tenant-1", first.ID)
	require.NoError(t, err)
	assert.Equal(t, first.PackageSHA256, got.PackageSHA256)
}

func TestReceiveLedgerJournalPackageIsIdempotent(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})

	first, report, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", pkg)
	require.NoError(t, err)
	require.True(t, report.Ready)
	assert.True(t, first.Created)
	assert.Equal(t, 1, first.LedgerVerification.BalancedJournalGroupCount)
	require.Len(t, first.LedgerPlanInput, 1)
	assert.Equal(t, "JE-100", first.LedgerPlanInput[0].SourceJournalExternalID)

	second, report, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", pkg)
	require.NoError(t, err)
	assert.True(t, report.Ready)
	assert.False(t, second.Created)
	assert.Equal(t, 1, store.creates)
}

func TestReceivePreservesConcurrentIdempotentReplayReceipt(t *testing.T) {
	store := &createRaceStore{}
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})

	receipt, report, err := NewService(store).Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", pkg)

	require.NoError(t, err)
	assert.True(t, report.Ready)
	assert.False(t, receipt.Created)
	assert.Equal(t, "concurrent-session", receipt.ID)
	assert.Equal(t, 1, store.creates)
}

func TestReceiveStagesReviewRequiredLedgerReceiptWithoutFinancialWrite(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})
	pkg.LedgerAuthority.VarianceCount = 2
	pkg.LedgerAuthority.Stale = true
	refreshPackageDigest(t, &pkg)

	receipt, report, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", pkg)

	require.NoError(t, err)
	assert.True(t, report.Ready)
	assert.True(t, report.LedgerVerification.ReviewRequired)
	assert.Equal(t, LedgerVerificationStatusReviewRequired, report.LedgerVerification.VerificationStatus)
	assert.Equal(t, SessionStatusReceivedReviewRequired, receipt.Status)
	assert.Equal(t, 1, store.creates)
}

func TestReceiveNeverPersistsInvalidPackage(t *testing.T) {
	store := &memoryStore{}
	pkg := validPackage(t)
	pkg.Records[0].PayloadSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"

	receipt, report, err := NewService(store).Receive(context.Background(), "tenant_schema", "tenant-1", "user-1", pkg)

	assert.Nil(t, receipt)
	assert.False(t, report.Ready)
	assert.ErrorIs(t, err, ErrPackageValidationFailed)
	assert.Zero(t, store.creates)
}

func TestReceiveBindsSourceCompanyToOneTenant(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	pkg := validPackage(t)

	_, _, err := service.Receive(context.Background(), "tenant_one", "tenant-1", "user-1", pkg)
	require.NoError(t, err)

	_, _, err = service.Receive(context.Background(), "tenant_two", "tenant-2", "user-2", pkg)
	assert.ErrorIs(t, err, ErrSourceCompanyBoundToOtherTenant)
	assert.Equal(t, 1, store.creates)
}

func TestGetPassesThroughNotFound(t *testing.T) {
	service := NewService(&memoryStore{})
	_, err := service.Get(context.Background(), "tenant_schema", "tenant-1", "missing")
	assert.True(t, errors.Is(err, ErrImportSessionNotFound))
}

func TestPlanProducesFullScopeReceiptOnlyActionsWithoutFinancialWrites(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	resolver := &recordingAccountResolver{}
	service.SetAccountResolver(resolver)
	receipt, _, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", validJournalPackage(t, ImportScope{
		Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll},
	}))
	require.NoError(t, err)
	createsBeforePlan := store.creates

	plan, err := service.Plan(context.Background(), "tenant_import_test", "tenant-1", receipt.ID, ImportPlanRequest{AccountMappings: []AccountMapping{
		{SourceAccountExternalID: "1000", TargetAccountID: "00000000-0000-0000-0000-000000000001"},
		{SourceAccountExternalID: "3000", TargetAccountID: "00000000-0000-0000-0000-000000000002"},
	}})

	require.NoError(t, err)
	require.True(t, plan.Ready)
	assert.False(t, plan.FinancialWritesPlanned)
	assert.Len(t, plan.JournalActions, 1)
	assert.Len(t, plan.JournalActions[0].Lines, 2)
	assert.Equal(t, "100", plan.JournalActions[0].DebitTotal)
	assert.Equal(t, "100", plan.JournalActions[0].CreditTotal)
	assert.Len(t, plan.JournalReconciliations, 1)
	assert.True(t, plan.JournalReconciliations[0].Balanced)
	assert.Len(t, plan.AccountReconciliations, 2)
	assert.NotEmpty(t, plan.PlanSHA256)
	assert.Equal(t, createsBeforePlan, store.creates, "planning must not persist a financial or receipt write")
	assert.Len(t, resolver.calls, 2, "planning only performs read-only target-account resolution")
}

func TestPlanEnforcesPartialReceiptJournalBoundary(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	service.SetAccountResolver(&recordingAccountResolver{})
	receipt, _, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", validJournalPackage(t, ImportScope{
		Mode: ScopeModePartial, ResourceTypes: []string{ScopeResourceJournalEntry}, PeriodStart: "2026-08-01", PeriodEnd: "2026-08-31",
	}))
	require.NoError(t, err)
	mappings := []AccountMapping{
		{SourceAccountExternalID: "1000", TargetAccountID: "00000000-0000-0000-0000-000000000001"},
		{SourceAccountExternalID: "3000", TargetAccountID: "00000000-0000-0000-0000-000000000002"},
	}
	validPlan, err := service.Plan(context.Background(), "tenant_import_test", "tenant-1", receipt.ID, ImportPlanRequest{AccountMappings: mappings})
	require.NoError(t, err)
	require.True(t, validPlan.Ready)
	assert.Equal(t, ScopeModePartial, validPlan.Scope.Mode)
	stored := store.receipts[receipt.PackageSHA256]
	stored.LedgerPlanInput[0].PeriodStart = "2026-07-31"
	store.receipts[receipt.PackageSHA256] = stored

	plan, err := service.Plan(context.Background(), "tenant_import_test", "tenant-1", receipt.ID, ImportPlanRequest{AccountMappings: mappings})

	require.NoError(t, err)
	assert.False(t, plan.Ready)
	assert.True(t, hasPlanIssue(plan, "journal_group_outside_scope"))
}

func TestPlanBlocksDuplicateAndConflictingSourceRevisionsWithinSameBinding(t *testing.T) {
	for _, tc := range []struct {
		name     string
		revision string
		want     string
	}{
		{name: "replay", revision: "2026-08-27T10:00:00Z", want: "duplicate_source_revision"},
		{name: "revision conflict", revision: "2026-08-28T10:00:00Z", want: "source_revision_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &memoryStore{}
			service := NewService(store)
			service.SetAccountResolver(&recordingAccountResolver{})
			receipt, _, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", validJournalPackage(t, ImportScope{
				Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll},
			}))
			require.NoError(t, err)
			other := *receipt
			other.ID = "session-other"
			other.PackageSHA256 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
			other.LedgerPlanInput = cloneStagedJournals(receipt.LedgerPlanInput)
			other.LedgerPlanInput[0].SourceRevision = tc.revision
			store.receipts[other.PackageSHA256] = other

			plan, err := service.Plan(context.Background(), "tenant_import_test", "tenant-1", receipt.ID, ImportPlanRequest{AccountMappings: []AccountMapping{
				{SourceAccountExternalID: "1000", TargetAccountID: "00000000-0000-0000-0000-000000000001"},
				{SourceAccountExternalID: "3000", TargetAccountID: "00000000-0000-0000-0000-000000000002"},
			}})

			require.NoError(t, err)
			assert.False(t, plan.Ready)
			assert.True(t, hasPlanIssue(plan, tc.want))
		})
	}
}

func TestPlanKeepsSiblingTenantLedgerMetadataOutOfScope(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	service.SetAccountResolver(&recordingAccountResolver{})
	receipt, _, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", validJournalPackage(t, ImportScope{
		Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll},
	}))
	require.NoError(t, err)
	other := *receipt
	other.ID = "session-tenant-two"
	other.TenantID = "tenant-2"
	other.PackageSHA256 = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	store.receipts[other.PackageSHA256] = other

	plan, err := service.Plan(context.Background(), "tenant_import_test", "tenant-1", receipt.ID, ImportPlanRequest{AccountMappings: []AccountMapping{
		{SourceAccountExternalID: "1000", TargetAccountID: "00000000-0000-0000-0000-000000000001"},
		{SourceAccountExternalID: "3000", TargetAccountID: "00000000-0000-0000-0000-000000000002"},
	}})

	require.NoError(t, err)
	assert.True(t, plan.Ready)
}

func TestPlanBlocksReviewRequiredReceiptBeforeAccountResolution(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store)
	resolver := &recordingAccountResolver{}
	service.SetAccountResolver(resolver)
	pkg := validJournalPackage(t, ImportScope{Mode: ScopeModeFull, ResourceTypes: []string{ScopeResourceAll}})
	pkg.LedgerAuthority.VarianceCount = 1
	pkg.LedgerAuthority.Stale = true
	refreshPackageDigest(t, &pkg)
	receipt, _, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", pkg)
	require.NoError(t, err)

	plan, err := service.Plan(context.Background(), "tenant_import_test", "tenant-1", receipt.ID, ImportPlanRequest{})

	assert.ErrorIs(t, err, ErrImportPlanReviewRequired)
	assert.False(t, plan.Ready)
	assert.True(t, hasPlanIssue(plan, "review_required"))
	assert.Empty(t, resolver.calls)
}

func hasPlanIssue(plan *ImportPlanResult, code string) bool {
	if plan == nil {
		return false
	}
	for _, issue := range plan.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
