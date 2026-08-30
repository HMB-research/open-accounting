package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/HMB-research/open-accounting/internal/importsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeImportSessionStore struct {
	receipts map[string]importsession.Receipt
	bindings map[string]string
	creates  int
}

func testSmartAccountsGLAuthority() *importsession.LedgerAuthorityDeclaration {
	authoritative := true
	return &importsession.LedgerAuthorityDeclaration{
		GeneralLedgerAuthority:       importsession.ProviderSmartAccounts,
		SmartAccountsGLAuthoritative: &authoritative,
		SourceAsOfDate:               "2026-08-31",
	}
}

func (s *fakeImportSessionStore) EnsureSourceCompanyBinding(_ context.Context, tenantID, provider, sourceCompanyID string) error {
	if s.bindings == nil {
		s.bindings = map[string]string{}
	}
	key := provider + "\x00" + sourceCompanyID
	if boundTenantID, exists := s.bindings[key]; exists && boundTenantID != tenantID {
		return importsession.ErrSourceCompanyBoundToOtherTenant
	}
	s.bindings[key] = tenantID
	return nil
}

func (s *fakeImportSessionStore) Create(_ context.Context, _, _ string, receipt importsession.Receipt) (importsession.Receipt, error) {
	if s.receipts == nil {
		s.receipts = map[string]importsession.Receipt{}
	}
	s.creates++
	receipt.ID = "import-session-1"
	s.receipts[receipt.PackageSHA256] = receipt
	return receipt, nil
}

func (s *fakeImportSessionStore) FindByPackage(_ context.Context, _, _, _, _, packageSHA256 string) (importsession.Receipt, error) {
	receipt, ok := s.receipts[packageSHA256]
	if !ok {
		return importsession.Receipt{}, importsession.ErrImportSessionNotFound
	}
	return receipt, nil
}

func (s *fakeImportSessionStore) Get(_ context.Context, _, _, sessionID string) (importsession.Receipt, error) {
	for _, receipt := range s.receipts {
		if receipt.ID == sessionID {
			return receipt, nil
		}
	}
	return importsession.Receipt{}, importsession.ErrImportSessionNotFound
}

func (s *fakeImportSessionStore) ListLedgerPlanInputs(_ context.Context, _, tenantID, provider, sourceCompanyID, excludeSessionID string) ([]importsession.StagedLedgerJournal, error) {
	var journals []importsession.StagedLedgerJournal
	for _, receipt := range s.receipts {
		if receipt.ID == excludeSessionID || receipt.TenantID != tenantID || receipt.Provider != provider || receipt.SourceCompanyID != sourceCompanyID {
			continue
		}
		journals = append(journals, receipt.LedgerPlanInput...)
	}
	return journals, nil
}

func validImportSessionPackage(t *testing.T) importsession.CanonicalPackage {
	t.Helper()
	payload := []byte(`{"code":"1000","name":"Cash"}`)
	digest := sha256.Sum256(payload)
	pkg := importsession.CanonicalPackage{
		SchemaVersion:   importsession.CanonicalSchemaVersionV1,
		Provider:        importsession.ProviderSmartAccounts,
		SourceCompanyID: "source-company-test-001",
		LedgerAuthority: testSmartAccountsGLAuthority(),
		Scope:           &importsession.ImportScope{Mode: importsession.ScopeModeFull, ResourceTypes: []string{importsession.ScopeResourceAll}},
		Records: []importsession.CanonicalRecord{{
			EntityType:    "account",
			ExternalID:    "1000",
			Revision:      "2026-08-27T10:00:00Z",
			Operation:     "upsert",
			Payload:       payload,
			PayloadSHA256: hex.EncodeToString(digest[:]),
		}},
	}
	packageDigest, err := importsession.PackageDigest(pkg)
	require.NoError(t, err)
	pkg.PackageSHA256 = packageDigest
	return pkg
}

func validLedgerImportSessionPackage(t *testing.T, secondCredit string) importsession.CanonicalPackage {
	t.Helper()
	pkg := validImportSessionPackage(t)
	pkg.Records[0] = importsession.CanonicalRecord{
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
{"account_external_id":"3000","debit":"0.00","credit":"` + secondCredit + `"}
]}`),
	}
	refreshImportSessionPackage(t, &pkg)
	return pkg
}

func refreshImportSessionPackage(t *testing.T, pkg *importsession.CanonicalPackage) {
	t.Helper()
	for index := range pkg.Records {
		if pkg.Records[index].Operation != "upsert" {
			continue
		}
		var payload map[string]any
		require.NoError(t, json.Unmarshal(pkg.Records[index].Payload, &payload))
		canonical, err := json.Marshal(payload)
		require.NoError(t, err)
		pkg.Records[index].Payload = canonical
		digest := sha256.Sum256(canonical)
		pkg.Records[index].PayloadSHA256 = hex.EncodeToString(digest[:])
	}
	digest, err := importsession.PackageDigest(*pkg)
	require.NoError(t, err)
	pkg.PackageSHA256 = digest
}

func hasImportSessionIssue(report importsession.ValidationReport, code string) bool {
	for _, issue := range report.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func TestValidateImportSessionPackageIsReadOnly(t *testing.T) {
	store := &fakeImportSessionStore{}
	h := &Handlers{importSessionService: importsession.NewService(store)}
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions/validate", importsession.PackageRequest{
		Package: validImportSessionPackage(t),
	}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ValidateImportSessionPackage(w, req)

	require.Equal(t, 200, w.Code)
	assert.Equal(t, 0, store.creates)
	var report importsession.ValidationReport
	require.NoError(t, decodeJSONResponse(w.Body, &report))
	assert.True(t, report.Ready)
}

func TestValidateImportSessionPackageReportsBalancedLedgerStagingOnly(t *testing.T) {
	store := &fakeImportSessionStore{}
	h := &Handlers{importSessionService: importsession.NewService(store)}
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions/validate", importsession.PackageRequest{
		Package: validLedgerImportSessionPackage(t, "100.00"),
	}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.ValidateImportSessionPackage(w, req)

	require.Equal(t, 200, w.Code)
	assert.Zero(t, store.creates)
	var report importsession.ValidationReport
	require.NoError(t, decodeJSONResponse(w.Body, &report))
	require.True(t, report.Ready)
	require.NotNil(t, report.LedgerVerification)
	assert.True(t, report.LedgerVerification.JournalStagingAllowed)
	assert.False(t, report.LedgerVerification.FinancialPostingPlanAllowed)
	assert.Equal(t, 1, report.LedgerVerification.BalancedJournalGroupCount)
}

func TestCreateImportSessionPersistsMetadataReceiptOnly(t *testing.T) {
	store := &fakeImportSessionStore{}
	h := &Handlers{importSessionService: importsession.NewService(store)}
	pkg := validImportSessionPackage(t)
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions", importsession.PackageRequest{Package: pkg}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateImportSession(w, req)

	require.Equal(t, 201, w.Code)
	assert.Equal(t, 1, store.creates)
	assert.NotContains(t, w.Body.String(), `"payload"`)
	assert.NotContains(t, w.Body.String(), `"ledger_plan_input"`)
	var receipt importsession.Receipt
	require.NoError(t, decodeJSONResponse(w.Body, &receipt))
	assert.Equal(t, importsession.SessionStatusReceivedValidated, receipt.Status)
	assert.True(t, receipt.Created)
	assert.Equal(t, pkg.PackageSHA256, receipt.PackageSHA256)
}

func TestCreateImportSessionRejectsInvalidPackageWithoutReceipt(t *testing.T) {
	store := &fakeImportSessionStore{}
	h := &Handlers{importSessionService: importsession.NewService(store)}
	pkg := validImportSessionPackage(t)
	pkg.Records[0].PayloadSHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions", importsession.PackageRequest{Package: pkg}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateImportSession(w, req)

	require.Equal(t, 422, w.Code)
	assert.Equal(t, 0, store.creates)
	var report importsession.ValidationReport
	require.NoError(t, decodeJSONResponse(w.Body, &report))
	assert.False(t, report.Ready)
}

func TestCreateImportSessionRejectsUnbalancedLedgerWithoutReceipt(t *testing.T) {
	store := &fakeImportSessionStore{}
	h := &Handlers{importSessionService: importsession.NewService(store)}
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions", importsession.PackageRequest{
		Package: validLedgerImportSessionPackage(t, "99.99"),
	}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateImportSession(w, req)

	require.Equal(t, 422, w.Code)
	assert.Zero(t, store.creates)
	var report importsession.ValidationReport
	require.NoError(t, decodeJSONResponse(w.Body, &report))
	assert.False(t, report.Ready)
	assert.True(t, hasImportSessionIssue(report, "unbalanced_journal_group"))
}

func TestCreateImportSessionLedgerReplayIsIdempotent(t *testing.T) {
	store := &fakeImportSessionStore{}
	h := &Handlers{importSessionService: importsession.NewService(store)}
	pkg := validLedgerImportSessionPackage(t, "100.00")
	makeRequest := func() *httptest.ResponseRecorder {
		req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions", importsession.PackageRequest{
			Package: pkg,
		}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{"tenantID": "tenant-1"})
		w := httptest.NewRecorder()
		h.CreateImportSession(w, req)
		return w
	}

	first := makeRequest()
	second := makeRequest()

	assert.Equal(t, 201, first.Code)
	assert.Equal(t, 200, second.Code)
	assert.Equal(t, 1, store.creates)
	assert.Contains(t, second.Body.String(), `"created":false`)
}

func TestGetImportSessionMapsNotFound(t *testing.T) {
	h := &Handlers{importSessionService: importsession.NewService(&fakeImportSessionStore{})}
	req := withURLParams(makeAuthenticatedRequest("GET", "/tenants/tenant-1/import-sessions/missing", nil, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{
		"tenantID":  "tenant-1",
		"sessionID": "missing",
	})
	w := httptest.NewRecorder()

	h.GetImportSession(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestCreateImportSessionRejectsSourceCompanyBoundToAnotherTenant(t *testing.T) {
	store := &fakeImportSessionStore{bindings: map[string]string{
		importsession.ProviderSmartAccounts + "\x00" + "source-company-test-001": "tenant-2",
	}}
	h := &Handlers{importSessionService: importsession.NewService(store)}
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions", importsession.PackageRequest{
		Package: validImportSessionPackage(t),
	}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{"tenantID": "tenant-1"})
	w := httptest.NewRecorder()

	h.CreateImportSession(w, req)

	assert.Equal(t, 409, w.Code)
	assert.Equal(t, 0, store.creates)
}

func TestPlanImportSessionReturnsReadOnlyJournalActions(t *testing.T) {
	store := &fakeImportSessionStore{}
	service := importsession.NewService(store)
	resolverCalls := 0
	service.SetAccountResolver(importsession.AccountResolverFunc(func(_ context.Context, _, _, _ string) error {
		resolverCalls++
		return nil
	}))
	receipt, _, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", validLedgerImportSessionPackage(t, "100.00"))
	require.NoError(t, err)
	createsBeforePlan := store.creates
	h := &Handlers{importSessionService: service}
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions/import-session-1/plan", importsession.ImportPlanRequest{AccountMappings: []importsession.AccountMapping{
		{SourceAccountExternalID: "1000", TargetAccountID: "00000000-0000-0000-0000-000000000001"},
		{SourceAccountExternalID: "3000", TargetAccountID: "00000000-0000-0000-0000-000000000002"},
	}}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{
		"tenantID": "tenant-1", "sessionID": receipt.ID,
	})
	w := httptest.NewRecorder()

	h.PlanImportSession(w, req)

	require.Equal(t, 200, w.Code)
	var plan importsession.ImportPlanResult
	require.NoError(t, decodeJSONResponse(w.Body, &plan))
	assert.True(t, plan.Ready)
	assert.False(t, plan.FinancialWritesPlanned)
	assert.Len(t, plan.JournalActions, 1)
	assert.Len(t, plan.AccountReconciliations, 2)
	assert.Equal(t, createsBeforePlan, store.creates)
	assert.Equal(t, 2, resolverCalls)
}

func TestPlanImportSessionBlocksReviewRequiredReceipt(t *testing.T) {
	store := &fakeImportSessionStore{}
	service := importsession.NewService(store)
	resolverCalls := 0
	service.SetAccountResolver(importsession.AccountResolverFunc(func(_ context.Context, _, _, _ string) error {
		resolverCalls++
		return nil
	}))
	pkg := validLedgerImportSessionPackage(t, "100.00")
	pkg.LedgerAuthority.VarianceCount = 1
	pkg.LedgerAuthority.Stale = true
	refreshImportSessionPackage(t, &pkg)
	receipt, _, err := service.Receive(context.Background(), "tenant_import_test", "tenant-1", "user-1", pkg)
	require.NoError(t, err)
	h := &Handlers{importSessionService: service}
	req := withURLParams(makeAuthenticatedRequest("POST", "/tenants/tenant-1/import-sessions/import-session-1/plan", importsession.ImportPlanRequest{}, createTestClaims("user-1", "user@example.com", "tenant-1", "accountant")), map[string]string{
		"tenantID": "tenant-1", "sessionID": receipt.ID,
	})
	w := httptest.NewRecorder()

	h.PlanImportSession(w, req)

	require.Equal(t, 409, w.Code)
	assert.Contains(t, w.Body.String(), `"review_required"`)
	assert.Zero(t, resolverCalls)
}
