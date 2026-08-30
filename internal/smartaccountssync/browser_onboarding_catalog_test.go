package smartaccountssync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type browserOnboardingCatalogMemoryStore struct {
	mu       sync.Mutex
	receipts map[string]BrowserOnboardingCatalogReceipt
}

func (s *browserOnboardingCatalogMemoryStore) CreateBrowserOnboardingCatalogReceipt(_ context.Context, receipt BrowserOnboardingCatalogReceipt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.receipts == nil {
		s.receipts = map[string]BrowserOnboardingCatalogReceipt{}
	}
	if _, exists := s.receipts[receipt.ID]; exists {
		return ErrBrowserOnboardingCatalogConflict
	}
	s.receipts[receipt.ID] = cloneBrowserOnboardingCatalogReceipt(receipt)
	return nil
}

func (s *browserOnboardingCatalogMemoryStore) GetBrowserOnboardingCatalogReceipt(_ context.Context, ownerID, catalogID string) (*BrowserOnboardingCatalogReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receipt, found := s.receipts[catalogID]
	if !found || (ownerID != "" && receipt.OwnerID != ownerID) {
		return nil, ErrBrowserOnboardingCatalogUnauthorized
	}
	copy := cloneBrowserOnboardingCatalogReceipt(receipt)
	return &copy, nil
}

func (s *browserOnboardingCatalogMemoryStore) AcceptBrowserOnboardingCatalogReceipt(_ context.Context, receipt BrowserOnboardingCatalogReceipt) (*BrowserOnboardingCatalogReceipt, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, found := s.receipts[receipt.ID]
	if !found || current.OwnerID != receipt.OwnerID {
		return nil, false, ErrBrowserOnboardingCatalogUnauthorized
	}
	if current.Status != BrowserOnboardingCatalogStatusIssued {
		copy := cloneBrowserOnboardingCatalogReceipt(current)
		return &copy, false, nil
	}
	s.receipts[receipt.ID] = cloneBrowserOnboardingCatalogReceipt(receipt)
	copy := cloneBrowserOnboardingCatalogReceipt(receipt)
	return &copy, true, nil
}

func cloneBrowserOnboardingCatalogReceipt(receipt BrowserOnboardingCatalogReceipt) BrowserOnboardingCatalogReceipt {
	receipt.Sources = append([]BrowserOnboardingSource(nil), receipt.Sources...)
	return receipt
}

func newBrowserOnboardingCatalogTestService() (*BrowserOnboardingCatalogService, *browserOnboardingCatalogMemoryStore) {
	store := &browserOnboardingCatalogMemoryStore{}
	service := NewBrowserOnboardingCatalogService(store)
	service.now = func() time.Time { return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC) }
	ids := []string{"90000000-0000-4000-8000-000000000001", "90000000-0000-4000-8000-000000000002"}
	service.newID = func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}
	tokens := []string{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq", "0123456789_abcdefghijklmnopqrstuvwxyzABCDEF"}
	service.newToken = func() (string, error) {
		token := tokens[0]
		tokens = tokens[1:]
		return token, nil
	}
	return service, store
}

func testBrowserOnboardingCatalogConsent() BrowserOnboardingCatalogIssueRequest {
	return BrowserOnboardingCatalogIssueRequest{CatalogConsent: BrowserOnboardingCatalogConsent{Version: 1, Confirmed: true, ConfirmedAt: time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC), Scope: "visible_company_catalog"}}
}

func catalogHandoffForIssue(t *testing.T, issue *BrowserOnboardingCatalogIssue, companies []BrowserOnboardingCatalogCompany) BrowserOnboardingCatalogHandoff {
	t.Helper()
	canonical, ok := canonicalBrowserOnboardingCatalogCompanies(companies)
	require.True(t, ok)
	encoded, err := jsonMarshalBrowserOnboardingCatalogDigest(BrowserOnboardingCatalogSchemaVersion, canonical)
	require.NoError(t, err)
	digest := sha256.Sum256(encoded)
	return BrowserOnboardingCatalogHandoff{SchemaVersion: BrowserOnboardingCatalogHandoffSchemaVersion, CatalogID: issue.CatalogID, WorkflowID: issue.WorkflowID, Nonce: issue.Nonce, CatalogCount: len(canonical), CatalogSHA256: hex.EncodeToString(digest[:]), Companies: canonical}
}

func TestBrowserOnboardingCatalogHandoffGoldenDigestAndIdempotentReceipt(t *testing.T) {
	companies := []BrowserOnboardingCatalogCompany{
		{SourceCompanyID: "sa-browser-v1-12", DisplayName: "A & B OÜ"},
		{SourceCompanyID: "sa-browser-v1-34", DisplayName: "<"},
		{SourceCompanyID: "sa-browser-v1-56", DisplayName: ">"},
		{SourceCompanyID: "sa-browser-v1-78", DisplayName: "ÄäöÖüÜ Eesti"},
		{SourceCompanyID: "sa-browser-v1-90", DisplayName: "Quote \" and slash \\"},
	}
	encoded, err := jsonMarshalBrowserOnboardingCatalogDigest(BrowserOnboardingCatalogSchemaVersion, companies)
	require.NoError(t, err)
	assert.Equal(t, `{"schema_version":"smartaccounts-browser-source-catalog-v1","source_id_version":"sa-browser-v1","companies":[{"source_company_id":"sa-browser-v1-12","display_name":"A & B OÜ"},{"source_company_id":"sa-browser-v1-34","display_name":"<"},{"source_company_id":"sa-browser-v1-56","display_name":">"},{"source_company_id":"sa-browser-v1-78","display_name":"ÄäöÖüÜ Eesti"},{"source_company_id":"sa-browser-v1-90","display_name":"Quote \" and slash \\"}]}`, string(encoded))
	sum := sha256.Sum256(encoded)
	assert.Equal(t, "cbe750250d900f83d49ba63ba10038a37f324c5024d11795b0dc8fb8f6108c1b", hex.EncodeToString(sum[:]))

	service, store := newBrowserOnboardingCatalogTestService()
	issue, err := service.Issue(context.Background(), "owner-1", testBrowserOnboardingCatalogConsent())
	require.NoError(t, err)
	assert.NotContains(t, mustJSON(t, store.receipts[issue.CatalogID]), issue.CatalogToken)
	handoff := catalogHandoffForIssue(t, issue, companies)
	accepted, err := service.Handoff(context.Background(), issue.CatalogID, issue.CatalogToken, handoff)
	require.NoError(t, err)
	assert.Equal(t, "accepted", accepted.Status)
	assert.Equal(t, handoff.CatalogSHA256, accepted.CatalogSHA256)
	replayed, err := service.Handoff(context.Background(), issue.CatalogID, issue.CatalogToken, handoff)
	require.NoError(t, err)
	assert.Equal(t, "already_accepted", replayed.Status)

	receipt, err := service.GetBrowserOnboardingCatalogReceipt(context.Background(), "owner-1", issue.CatalogID)
	require.NoError(t, err)
	assert.Equal(t, handoff.CatalogSHA256, receipt.CatalogSHA256)
	assert.Equal(t, companies, sourcesToBrowserOnboardingCatalogCompanies(receipt.Sources))
	_, err = service.GetBrowserOnboardingCatalogReceipt(context.Background(), "owner-2", issue.CatalogID)
	assert.ErrorIs(t, err, ErrBrowserOnboardingCatalogUnauthorized)
}

func TestBrowserOnboardingCatalogRejectsEmptyNoncanonicalAndBadDisplayNames(t *testing.T) {
	for _, companies := range [][]BrowserOnboardingCatalogCompany{
		nil,
		{{SourceCompanyID: batchSourceOne, DisplayName: " Leading"}},
		{{SourceCompanyID: batchSourceOne, DisplayName: "Two  spaces"}},
		{{SourceCompanyID: batchSourceOne, DisplayName: "Line\nfeed"}},
		{{SourceCompanyID: batchSourceOne, DisplayName: string(make([]byte, 121))}},
	} {
		_, ok := canonicalBrowserOnboardingCatalogCompanies(companies)
		assert.False(t, ok)
	}
	acceptedName := strings.Repeat("ü", 60) // 120 UTF-8 bytes.
	_, ok := canonicalBrowserOnboardingCatalogCompanies([]BrowserOnboardingCatalogCompany{{SourceCompanyID: batchSourceOne, DisplayName: acceptedName}})
	assert.True(t, ok)
	_, ok = canonicalBrowserOnboardingCatalogCompanies([]BrowserOnboardingCatalogCompany{{SourceCompanyID: batchSourceOne, DisplayName: strings.Repeat("ü", 61)}})
	assert.False(t, ok)
}

func TestBrowserOnboardingCatalogRawCapabilityExpiresBeforeAcceptedReceiptUse(t *testing.T) {
	service, _ := newBrowserOnboardingCatalogTestService()
	issue, err := service.Issue(context.Background(), "owner-1", testBrowserOnboardingCatalogConsent())
	require.NoError(t, err)
	handoff := catalogHandoffForIssue(t, issue, []BrowserOnboardingCatalogCompany{{SourceCompanyID: batchSourceOne, DisplayName: "Hold My Beer OÜ"}})
	_, err = service.Handoff(context.Background(), issue.CatalogID, issue.CatalogToken, handoff)
	require.NoError(t, err)

	service.now = func() time.Time { return time.Date(2026, 8, 28, 12, 3, 0, 0, time.UTC) }
	_, err = service.Handoff(context.Background(), issue.CatalogID, issue.CatalogToken, handoff)
	assert.ErrorIs(t, err, ErrBrowserOnboardingCatalogUnauthorized)
	_, err = service.GetBrowserOnboardingCatalogReceipt(context.Background(), "owner-1", issue.CatalogID)
	require.NoError(t, err)

	service.now = func() time.Time { return time.Date(2026, 8, 28, 12, 11, 0, 0, time.UTC) }
	_, err = service.GetBrowserOnboardingCatalogReceipt(context.Background(), "owner-1", issue.CatalogID)
	assert.ErrorIs(t, err, ErrBrowserOnboardingCatalogUnauthorized)
}
