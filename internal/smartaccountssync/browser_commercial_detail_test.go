package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type browserCommercialDetailMemoryStore struct {
	byRun map[string]BrowserCommercialDetailAuthorization
	byKey map[string]string
}

func (s *browserCommercialDetailMemoryStore) FindOrCreateBrowserCommercialDetailAuthorization(_ context.Context, value BrowserCommercialDetailAuthorization) (*BrowserCommercialDetailAuthorization, bool, error) {
	if s.byRun == nil {
		s.byRun, s.byKey = map[string]BrowserCommercialDetailAuthorization{}, map[string]string{}
	}
	key := value.TenantID + "\x00" + value.BatchID + "\x00" + value.SourceCompanyID + "\x00" + value.ResourceID
	if id, ok := s.byKey[key]; ok {
		result := s.byRun[id]
		return &result, false, nil
	}
	s.byRun[value.RunID] = value
	s.byKey[key] = value.RunID
	return &value, true, nil
}
func (s *browserCommercialDetailMemoryStore) GetBrowserCommercialDetailAuthorization(_ context.Context, runID, tenantID string) (*BrowserCommercialDetailAuthorization, error) {
	value, ok := s.byRun[runID]
	if !ok || value.TenantID != tenantID {
		return nil, ErrBrowserCommercialDetailUnauthorized
	}
	return &value, nil
}
func (s *browserCommercialDetailMemoryStore) RotateBrowserCommercialDetailAuthorization(_ context.Context, value BrowserCommercialDetailAuthorization) error {
	if _, ok := s.byRun[value.RunID]; !ok {
		return ErrBrowserCommercialDetailUnauthorized
	}
	s.byRun[value.RunID] = value
	return nil
}
func (s *browserCommercialDetailMemoryStore) SaveBrowserCommercialDetailStatus(_ context.Context, value BrowserCommercialDetailAuthorization) error {
	if _, ok := s.byRun[value.RunID]; !ok {
		return ErrBrowserCommercialDetailUnauthorized
	}
	s.byRun[value.RunID] = value
	return nil
}

type browserCommercialDetailBatchStub struct {
	workflow BrowserBatchWorkflow
	sources  []BrowserBatchSourceWorkflow
}

func (s browserCommercialDetailBatchStub) GetBrowserBatchWorkflow(_ context.Context, owner, batch string) (*BrowserBatchWorkflow, error) {
	if owner != s.workflow.OwnerID || batch != s.workflow.BatchID {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	result := cloneBrowserBatchWorkflow(s.workflow)
	return &result, nil
}
func (s browserCommercialDetailBatchStub) ListBrowserBatchSourceWorkflows(_ context.Context, owner, batch string) ([]BrowserBatchSourceWorkflow, error) {
	if owner != s.workflow.OwnerID || batch != s.workflow.BatchID {
		return nil, ErrBrowserBatchWorkflowNotFound
	}
	return append([]BrowserBatchSourceWorkflow(nil), s.sources...), nil
}

type browserCommercialDetailBridgeStub struct {
	starts      []BrowserCommercialDetailStartRequest
	runRequests map[string]BrowserCommercialDetailStartRequest
}

func (s *browserCommercialDetailBridgeStub) StartBrowserCommercialDetail(_ context.Context, _ string, runID string, request BrowserCommercialDetailStartRequest) (BrowserCommercialDetailStatus, error) {
	if s.runRequests == nil {
		s.runRequests = map[string]BrowserCommercialDetailStartRequest{}
	}
	s.starts = append(s.starts, request)
	s.runRequests[runID] = request
	_, schema, sourceSchema, routeSHA, _ := browserCommercialDetailContractFor(request.Contract.Resource, request.Contract.Review)
	contractSHA, _ := browserCommercialDetailSHA256(request.Contract)
	consentSHA, _ := browserCommercialDetailConsentSHA256(request.Consent)
	return BrowserCommercialDetailStatus{RunID: runID, Status: "open", ManifestVersion: request.ManifestVersion, ResourceID: request.Contract.Resource, SchemaID: schema, SourceSchema: sourceSchema, RouteSHA256: routeSHA, ContractSHA256: contractSHA, ConsentSHA256: consentSHA}, nil
}
func (s *browserCommercialDetailBridgeStub) GetBrowserCommercialDetail(_ context.Context, _ string, runID string) (BrowserCommercialDetailStatus, error) {
	request, ok := s.runRequests[runID]
	if !ok {
		return BrowserCommercialDetailStatus{}, errors.New("missing bridge run")
	}
	_, schema, sourceSchema, routeSHA, _ := browserCommercialDetailContractFor(request.Contract.Resource, request.Contract.Review)
	contractSHA, _ := browserCommercialDetailSHA256(request.Contract)
	consentSHA, _ := browserCommercialDetailConsentSHA256(request.Consent)
	return BrowserCommercialDetailStatus{RunID: runID, Status: "open", ManifestVersion: request.ManifestVersion, ResourceID: request.Contract.Resource, SchemaID: schema, SourceSchema: sourceSchema, RouteSHA256: routeSHA, ContractSHA256: contractSHA, ConsentSHA256: consentSHA}, nil
}

func newBrowserCommercialDetailTestService(now time.Time) (*BrowserCommercialDetailService, *browserCommercialDetailMemoryStore, *browserCommercialDetailBridgeStub) {
	confirmed := now.Add(-time.Minute)
	workflow := BrowserBatchWorkflow{BatchID: batchWorkflowID, OwnerID: "owner-1", SchemaVersion: BrowserBatchWorkflowSchemaVersion, HistoryFrom: "2026-01-01", PreparatoryManifestSHA256: fmt.Sprintf("%064d", 1), PreparatoryConsentedAt: now, TransferManifestSHA256: fmt.Sprintf("%064d", 2), TransferScope: BrowserBatchTransferScope{Mode: "partial", FromInclusive: "2026-01-01", ToInclusive: "2026-08-28", CutoffAt: "2026-08-28T10:00:00Z", ResourceIDs: []string{BrowserGeneralLedgerResourceID}}, TransferConfirmedAt: &confirmed, CreatedAt: now, UpdatedAt: now}
	batch := browserCommercialDetailBatchStub{workflow: workflow, sources: []BrowserBatchSourceWorkflow{{BatchID: batchWorkflowID, SourceCompanyID: browserSourceID, TenantID: browserPairingTenantID, Ordinal: 0, Phase: BrowserBatchPhasePaired, CreatedAt: now, UpdatedAt: now}}}
	store, bridge := &browserCommercialDetailMemoryStore{}, &browserCommercialDetailBridgeStub{}
	service := NewBrowserCommercialDetailService(store, batch, bridge)
	service.now = func() time.Time { return now }
	serial := 0
	service.newToken = func() (string, error) { serial++; return fmt.Sprintf("%043d", serial), nil }
	return service, store, bridge
}

func TestBrowserCommercialDetailIssuesOnlyFixedOrderedBlockedRelayAndResumes(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	service, store, bridge := newBrowserCommercialDetailTestService(now)
	issues, err := service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserCommercialDetailAuthorizeRequest{BatchID: batchWorkflowID, SourceCompanyID: browserSourceID, TransferConsentConfirmed: true})
	require.NoError(t, err)
	require.Len(t, issues.Issues, 2)
	assert.Equal(t, []string{BrowserCommercialClientInvoicesResource, BrowserCommercialBankPaymentsResource}, []string{issues.Issues[0].ResourceID, issues.Issues[1].ResourceID})
	assert.Equal(t, []int{1, 2}, []int{issues.Issues[0].Workflow.Sequence, issues.Issues[1].Workflow.Sequence})
	assert.Equal(t, BrowserCommercialDetailListSelector, issues.Issues[0].ListSelectorStatus)
	// The private run is deliberately status-first: issuing a capability must
	// not create a bridge run before the extension's GET has received 404.
	assert.Empty(t, bridge.starts)
	persistedControl := mustJSON(t, store.byRun)
	assert.NotContains(t, persistedControl, issues.Issues[0].CaptureToken)
	assert.NotContains(t, persistedControl, "invoiceNumber")
	assert.NotContains(t, persistedControl, "paymentDocument")
	_, err = service.Status(context.Background(), browserPairingTenantID, issues.Issues[0].RunID, issues.Issues[0].CaptureToken)
	assert.ErrorIs(t, err, ErrBrowserCommercialDetailRunNotFound)
	first := store.byRun[issues.Issues[0].RunID]
	start, err := browserCommercialDetailStartRequest(&first)
	require.NoError(t, err)
	status, err := service.Start(context.Background(), browserPairingTenantID, issues.Issues[0].RunID, issues.Issues[0].CaptureToken, start)
	require.NoError(t, err)
	assert.Len(t, bridge.starts, 1)
	assert.Equal(t, BrowserCommercialClientInvoicesResource, bridge.starts[0].Contract.Resource)
	assert.Equal(t, BrowserCommercialDetailListSelector, status.Status)
	assert.Empty(t, status.PackageID)
	second := store.byRun[issues.Issues[1].RunID]
	secondStart, err := browserCommercialDetailStartRequest(&second)
	require.NoError(t, err)
	_, err = service.Start(context.Background(), browserPairingTenantID, issues.Issues[1].RunID, issues.Issues[1].CaptureToken, secondStart)
	assert.ErrorIs(t, err, ErrBrowserCommercialDetailBlocked)
	assert.Len(t, bridge.starts, 1)
	secondStatus, err := service.Status(context.Background(), browserPairingTenantID, issues.Issues[1].RunID, issues.Issues[1].CaptureToken)
	require.NoError(t, err)
	assert.Equal(t, BrowserCommercialDetailListSelector, secondStatus.Status)
	assert.ErrorIs(t, service.Upload(context.Background(), browserPairingTenantID, issues.Issues[0].RunID, issues.Issues[0].CaptureToken), ErrBrowserCommercialDetailBlocked)
	assert.ErrorIs(t, service.Finalize(context.Background(), browserPairingTenantID, issues.Issues[0].RunID, issues.Issues[0].CaptureToken), ErrBrowserCommercialDetailBlocked)
	service.now = func() time.Time { return now.Add(time.Minute) }
	resume, err := service.Resume(context.Background(), browserPairingTenantID, "owner-1", issues.Issues[0].RunID, BrowserCommercialDetailResumeRequest{TransferConsentConfirmed: true})
	require.NoError(t, err)
	assert.Equal(t, issues.Issues[0].RunID, resume.RunID)
	assert.NotEqual(t, issues.Issues[0].CaptureToken, resume.CaptureToken)
}

func TestBrowserCommercialDetailRejectsOtherTenantAndUnboundSource(t *testing.T) {
	now := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	service, _, _ := newBrowserCommercialDetailTestService(now)
	_, err := service.Authorize(context.Background(), "f7f93b37-51a1-4870-8a0d-72c8db198cd9", "owner-1", BrowserCommercialDetailAuthorizeRequest{BatchID: batchWorkflowID, SourceCompanyID: browserSourceID, TransferConsentConfirmed: true})
	assert.ErrorIs(t, err, ErrBrowserCommercialDetailUnauthorized)
	_, err = service.Authorize(context.Background(), browserPairingTenantID, "owner-1", BrowserCommercialDetailAuthorizeRequest{BatchID: batchWorkflowID, SourceCompanyID: "sa-browser-v1-999999", TransferConsentConfirmed: true})
	assert.ErrorIs(t, err, ErrBrowserCommercialDetailUnauthorized)
}

func TestBrowserCommercialDetailReviewedContractsMatchPrivateRelayCatalog(t *testing.T) {
	review := BrowserCommercialDetailReview{Version: BrowserCommercialDetailReviewVersion, Confirmed: true, ReviewedAt: time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC), AuditID: "4d8a3f84-5749-42cb-92a4-3fd2c56a702f"}
	tests := []struct {
		resource, schema, listPath, detailPrefix, detailForm, stableField string
		fields                                                            []BrowserCommercialDetailFieldRule
	}{
		{
			resource: BrowserCommercialClientInvoicesResource, schema: "client_invoices_detail_v1", listPath: "/et/clientinvoices", detailPrefix: "/et/clientinvoices.change/", detailForm: "/et/clientinvoices.clientinvoiceaddeditcomp.addeditform", stableField: "invoiceNumber",
			fields: []BrowserCommercialDetailFieldRule{{"clients", "string", false, nil}, {"contactName", "string", false, nil}, {"invoiceDate", "date", false, nil}, {"invoiceNumber", "string", true, nil}, {"invoiceDueDate", "date", false, nil}, {"invoiceInterest", "string", false, nil}, {"invoiceEntryDate", "date", false, nil}, {"invReferenceNumber", "string", false, nil}, {"invoiceCurrency", "string", false, nil}, {"branches2", "string", false, nil}, {"articles", "string", false, nil}, {"invoiceEntryDescription", "string", false, nil}, {"invoiceEntryQuantity", "string", false, nil}, {"invoiceEntryPrice", "string", false, nil}, {"invoiceEntryDiscountPc", "string", false, nil}, {"invoiceRoundAmount", "string", false, nil}, {"invoicePaymentMethod", "string", false, nil}, {"invoicePaymentAmountD", "string", false, nil}, {"selectedTemplate", "string", false, nil}, {"invoiceNote", "string", false, nil}, {"internalNote", "string", false, nil}},
		},
		{
			resource: BrowserCommercialBankPaymentsResource, schema: "bank_payments_detail_v1", listPath: "/et/payments/bank", detailPrefix: "/et/payments/bank.paymentslistcomp.change/", detailForm: "/et/payments/bank.paymentslistcomp.paymentaddeditcomp.addeditform", stableField: "paymentDocument",
			fields: []BrowserCommercialDetailFieldRule{{"vendors", "string", false, nil}, {"paymentBankAccount", "string", false, nil}, {"paymentDate", "date", false, nil}, {"paymentCurrency", "string", false, nil}, {"paymentDocument", "string", true, nil}, {"paymentExtraDescription", "string", false, nil}, {"paymentExtraQuantity", "string", false, nil}, {"paymentExtraPrice", "string", false, nil}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.resource, func(t *testing.T) {
			contract, schema, sourceSchema, routeSHA, found := browserCommercialDetailContractFor(tt.resource, review)
			require.True(t, found)
			assert.Equal(t, tt.schema, schema)
			assert.Equal(t, BrowserCommercialDetailManifestVersion+"/"+tt.schema, sourceSchema)
			assert.Equal(t, BrowserCommercialDetailContract{Version: BrowserCommercialDetailManifestVersion, Resource: tt.resource, Origin: "https://sa.smartaccounts.eu", ListPagePath: tt.listPath, DetailPathPrefix: tt.detailPrefix, StableIDField: tt.stableField, Fields: tt.fields, Review: review}, contract)
			route, err := json.Marshal(struct{ ListPagePath, DetailPathPrefix, DetailFormPath string }{tt.listPath, tt.detailPrefix, tt.detailForm})
			require.NoError(t, err)
			digest := sha256.Sum256(route)
			assert.Equal(t, hex.EncodeToString(digest[:]), routeSHA)
		})
	}
}
