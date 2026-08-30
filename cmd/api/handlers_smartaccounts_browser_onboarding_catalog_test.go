package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/smartaccountssync"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type onboardingCatalogHandlerStore struct {
	mu       sync.Mutex
	receipts map[string]smartaccountssync.BrowserOnboardingCatalogReceipt
}

func (s *onboardingCatalogHandlerStore) CreateBrowserOnboardingCatalogReceipt(_ context.Context, receipt smartaccountssync.BrowserOnboardingCatalogReceipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.receipts == nil {
		s.receipts = map[string]smartaccountssync.BrowserOnboardingCatalogReceipt{}
	}
	if _, found := s.receipts[receipt.ID]; found {
		return errors.New("duplicate receipt")
	}
	s.receipts[receipt.ID] = cloneOnboardingCatalogHandlerReceipt(receipt)
	return nil
}

func (s *onboardingCatalogHandlerStore) GetBrowserOnboardingCatalogReceipt(_ context.Context, ownerID, catalogID string) (*smartaccountssync.BrowserOnboardingCatalogReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, found := s.receipts[catalogID]
	if !found || (ownerID != "" && ownerID != receipt.OwnerID) {
		return nil, smartaccountssync.ErrBrowserOnboardingCatalogUnauthorized
	}
	copy := cloneOnboardingCatalogHandlerReceipt(receipt)
	return &copy, nil
}

func (s *onboardingCatalogHandlerStore) AcceptBrowserOnboardingCatalogReceipt(_ context.Context, receipt smartaccountssync.BrowserOnboardingCatalogReceipt) (*smartaccountssync.BrowserOnboardingCatalogReceipt, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, found := s.receipts[receipt.ID]
	if !found || current.OwnerID != receipt.OwnerID {
		return nil, false, smartaccountssync.ErrBrowserOnboardingCatalogUnauthorized
	}
	if current.Status != smartaccountssync.BrowserOnboardingCatalogStatusIssued {
		copy := cloneOnboardingCatalogHandlerReceipt(current)
		return &copy, false, nil
	}
	s.receipts[receipt.ID] = cloneOnboardingCatalogHandlerReceipt(receipt)
	copy := cloneOnboardingCatalogHandlerReceipt(receipt)
	return &copy, true, nil
}

func cloneOnboardingCatalogHandlerReceipt(receipt smartaccountssync.BrowserOnboardingCatalogReceipt) smartaccountssync.BrowserOnboardingCatalogReceipt {
	receipt.Sources = append([]smartaccountssync.BrowserOnboardingSource(nil), receipt.Sources...)
	return receipt
}

func newSmartAccountsBrowserOnboardingCatalogHandlers() (*Handlers, *onboardingCatalogHandlerStore) {
	store := &onboardingCatalogHandlerStore{}
	service := smartaccountssync.NewBrowserOnboardingCatalogService(store)
	return &Handlers{smartAccountsBrowserOnboardingCatalogService: service}, store
}

func TestSmartAccountsBrowserOnboardingCatalogIssueHandoffAndOwnerStatus(t *testing.T) {
	h, store := newSmartAccountsBrowserOnboardingCatalogHandlers()
	issueRequest := makeAuthenticatedRequest(http.MethodPost, "/", catalogConsentJSON(), createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner))
	issueRequest.Header.Set("Content-Type", "application/json")
	issueRecorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserOnboardingCatalog(issueRecorder, issueRequest)
	require.Equal(t, http.StatusCreated, issueRecorder.Code, issueRecorder.Body.String())
	require.Equal(t, "no-store", issueRecorder.Header().Get("Cache-Control"))
	var issue smartaccountssync.BrowserOnboardingCatalogIssue
	require.NoError(t, json.NewDecoder(issueRecorder.Body).Decode(&issue))
	require.Len(t, issue.CatalogToken, 43)
	stored := store.receipts[issue.CatalogID]
	assert.NotContains(t, mustMarshalJSON(t, stored), issue.CatalogToken)
	assert.NotContains(t, issueRecorder.Body.String(), stored.TokenSHA256)

	handoff := testCatalogHandoff(t, issue, []smartaccountssync.BrowserOnboardingCatalogCompany{{SourceCompanyID: testBrowserSourceID, DisplayName: "Hold My Beer OÜ"}})
	body, err := json.Marshal(handoff)
	require.NoError(t, err)
	relay := withURLParams(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)), map[string]string{"catalogID": issue.CatalogID})
	relay.Header.Set("Origin", testBraveExtensionOrigin)
	relay.Header.Set("Content-Type", "application/json")
	relay.Header.Set("Authorization", "Bearer "+issue.CatalogToken)
	relayRecorder := httptest.NewRecorder()
	h.HandoffSmartAccountsBrowserOnboardingCatalog(relayRecorder, relay)
	require.Equal(t, http.StatusCreated, relayRecorder.Code, relayRecorder.Body.String())
	assert.Equal(t, testBraveExtensionOrigin, relayRecorder.Header().Get("Access-Control-Allow-Origin"))
	assert.NotContains(t, relayRecorder.Body.String(), issue.CatalogToken)
	assert.NotContains(t, relayRecorder.Body.String(), stored.TokenSHA256)

	statusRequest := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/", nil, createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner)), map[string]string{"catalogID": issue.CatalogID})
	statusRecorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserOnboardingCatalog(statusRecorder, statusRequest)
	require.Equal(t, http.StatusOK, statusRecorder.Code, statusRecorder.Body.String())
	assert.Contains(t, statusRecorder.Body.String(), testBrowserSourceID)
	assert.NotContains(t, statusRecorder.Body.String(), issue.CatalogToken)
	assert.NotContains(t, statusRecorder.Body.String(), stored.TokenSHA256)

	wrongOwner := withURLParams(makeAuthenticatedRequest(http.MethodGet, "/", nil, createTestClaims("owner-2", "other@example.com", "", tenant.RoleOwner)), map[string]string{"catalogID": issue.CatalogID})
	wrongOwnerRecorder := httptest.NewRecorder()
	h.GetSmartAccountsBrowserOnboardingCatalog(wrongOwnerRecorder, wrongOwner)
	assert.Equal(t, http.StatusNotFound, wrongOwnerRecorder.Code)
}

func TestSmartAccountsBrowserOnboardingCatalogHandoffSupportsWorstCaseBoundedCatalog(t *testing.T) {
	h, _ := newSmartAccountsBrowserOnboardingCatalogHandlers()
	issue := issueBrowserOnboardingCatalogForHandler(t, h)
	companies := testCatalogCompanies(250, strings.Repeat("ü", 60))
	handoff := testCatalogHandoff(t, issue, companies)
	body, err := json.Marshal(handoff)
	require.NoError(t, err)
	// This deliberately exceeds the legacy 8 KiB onboarding body cap while
	// remaining far below this route's bounded 128 KiB cap.
	require.Greater(t, len(body), maxSmartAccountsBrowserOnboardingRequestBytes)
	require.Less(t, len(body), maxSmartAccountsBrowserOnboardingCatalogHandoffBytes)
	request := withURLParams(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)), map[string]string{"catalogID": issue.CatalogID})
	request.Header.Set("Origin", testBraveExtensionOrigin)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+issue.CatalogToken)
	recorder := httptest.NewRecorder()
	h.HandoffSmartAccountsBrowserOnboardingCatalog(recorder, request)
	assert.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())

	tooMany := testCatalogCompanies(251, "HMB")
	bad := testCatalogHandoffUnchecked(issue, tooMany)
	badBody, err := json.Marshal(bad)
	require.NoError(t, err)
	badRequest := withURLParams(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(badBody)), map[string]string{"catalogID": issue.CatalogID})
	badRequest.Header.Set("Origin", testBraveExtensionOrigin)
	badRequest.Header.Set("Content-Type", "application/json")
	badRequest.Header.Set("Authorization", "Bearer "+issue.CatalogToken)
	badRecorder := httptest.NewRecorder()
	h.HandoffSmartAccountsBrowserOnboardingCatalog(badRecorder, badRequest)
	assert.Equal(t, http.StatusBadRequest, badRecorder.Code)
}

func TestSmartAccountsBrowserOnboardingCatalogRejectsAPITokenAndNonExtensionRelay(t *testing.T) {
	h, _ := newSmartAccountsBrowserOnboardingCatalogHandlers()
	apiIssue := makeAuthenticatedRequest(http.MethodPost, "/", catalogConsentJSON(), &auth.Claims{UserID: "owner-1", TokenKind: auth.TokenKindAPIToken})
	apiIssue.Header.Set("Content-Type", "application/json")
	apiRecorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserOnboardingCatalog(apiRecorder, apiIssue)
	assert.Equal(t, http.StatusForbidden, apiRecorder.Code)

	issue := issueBrowserOnboardingCatalogForHandler(t, h)
	handoff := testCatalogHandoff(t, issue, []smartaccountssync.BrowserOnboardingCatalogCompany{{SourceCompanyID: testBrowserSourceID, DisplayName: "Hold My Beer OÜ"}})
	body, err := json.Marshal(handoff)
	require.NoError(t, err)
	request := withURLParams(httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)), map[string]string{"catalogID": issue.CatalogID})
	request.Header.Set("Origin", "https://sa.smartaccounts.eu")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+issue.CatalogToken)
	recorder := httptest.NewRecorder()
	h.HandoffSmartAccountsBrowserOnboardingCatalog(recorder, request)
	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestSmartAccountsBrowserOnboardingCatalogHandoffRouterCORS(t *testing.T) {
	h, _ := newSmartAccountsBrowserOnboardingCatalogHandlers()
	issue := issueBrowserOnboardingCatalogForHandler(t, h)
	router := setupRouter(&Config{}, h, nil)
	path := "/api/v1/smartaccounts-browser-onboarding/catalogs/" + issue.CatalogID + "/handoff"
	preflight := httptest.NewRequest(http.MethodOptions, path, nil)
	preflight.Header.Set("Origin", testBraveExtensionOrigin)
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflightRecorder := httptest.NewRecorder()
	router.ServeHTTP(preflightRecorder, preflight)
	require.Equal(t, http.StatusOK, preflightRecorder.Code)
	assert.Equal(t, testBraveExtensionOrigin, preflightRecorder.Header().Get("Access-Control-Allow-Origin"))
}

func issueBrowserOnboardingCatalogForHandler(t *testing.T, h *Handlers) smartaccountssync.BrowserOnboardingCatalogIssue {
	t.Helper()
	request := makeAuthenticatedRequest(http.MethodPost, "/", catalogConsentJSON(), createTestClaims("owner-1", "owner@example.com", "", tenant.RoleOwner))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	h.IssueSmartAccountsBrowserOnboardingCatalog(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	var issue smartaccountssync.BrowserOnboardingCatalogIssue
	require.NoError(t, json.NewDecoder(recorder.Body).Decode(&issue))
	return issue
}

func catalogConsentJSON() json.RawMessage {
	return json.RawMessage(`{"catalog_consent":{"version":1,"confirmed":true,"confirmed_at":"` + time.Now().UTC().Format(time.RFC3339) + `","scope":"visible_company_catalog"}}`)
}

func testCatalogCompanies(count int, name string) []smartaccountssync.BrowserOnboardingCatalogCompany {
	companies := make([]smartaccountssync.BrowserOnboardingCatalogCompany, 0, count)
	for index := 0; index < count; index++ {
		companies = append(companies, smartaccountssync.BrowserOnboardingCatalogCompany{SourceCompanyID: "sa-browser-v1-" + strconv.Itoa(index+1), DisplayName: name})
	}
	sort.Slice(companies, func(i, j int) bool { return companies[i].SourceCompanyID < companies[j].SourceCompanyID })
	return companies
}

func testCatalogHandoff(t *testing.T, issue smartaccountssync.BrowserOnboardingCatalogIssue, companies []smartaccountssync.BrowserOnboardingCatalogCompany) smartaccountssync.BrowserOnboardingCatalogHandoff {
	t.Helper()
	return testCatalogHandoffUnchecked(issue, companies)
}

func testCatalogHandoffUnchecked(issue smartaccountssync.BrowserOnboardingCatalogIssue, companies []smartaccountssync.BrowserOnboardingCatalogCompany) smartaccountssync.BrowserOnboardingCatalogHandoff {
	encoded, err := json.Marshal(struct {
		SchemaVersion   string                                              `json:"schema_version"`
		SourceIDVersion string                                              `json:"source_id_version"`
		Companies       []smartaccountssync.BrowserOnboardingCatalogCompany `json:"companies"`
	}{SchemaVersion: smartaccountssync.BrowserOnboardingCatalogSchemaVersion, SourceIDVersion: smartaccountssync.BrowserOnboardingCatalogSourceIDVersion, Companies: companies})
	if err != nil {
		panic(err)
	}
	// The test data has no HTML-sensitive names. The production golden-vector
	// test covers Encoder.SetEscapeHTML(false) compatibility with the worker.
	sum := sha256.Sum256(encoded)
	return smartaccountssync.BrowserOnboardingCatalogHandoff{SchemaVersion: smartaccountssync.BrowserOnboardingCatalogHandoffSchemaVersion, CatalogID: issue.CatalogID, WorkflowID: issue.WorkflowID, Nonce: issue.Nonce, CatalogCount: len(companies), CatalogSHA256: hex.EncodeToString(sum[:]), Companies: companies}
}
