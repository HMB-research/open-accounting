package smartaccountssync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	browserPairingTenantID = "b436c224-5df5-4b4d-a772-1897f9147400"
	browserPairingID       = "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3"
	browserSourceID        = "sa-browser-v1-123456"
)

type browserPairingMemoryStore struct {
	memoryStore
	pairings map[string]BrowserPairing
}

func (s *browserPairingMemoryStore) CreateBrowserPairing(_ context.Context, pairing BrowserPairing) error {
	if s.pairings == nil {
		s.pairings = map[string]BrowserPairing{}
	}
	if _, exists := s.pairings[pairing.ID]; exists {
		return errors.New("pairing already exists")
	}
	s.pairings[pairing.ID] = pairing
	return nil
}

func (s *browserPairingMemoryStore) GetBrowserPairing(_ context.Context, pairingID, tenantID string) (*BrowserPairing, error) {
	pairing, exists := s.pairings[pairingID]
	if !exists || pairing.TenantID != tenantID {
		return nil, ErrBrowserPairingNotClaimable
	}
	return &pairing, nil
}

func (s *browserPairingMemoryStore) ClaimBrowserPairing(_ context.Context, pairingID, tokenSHA256, sourceCompanyID string, claimedAt time.Time) (*BrowserPairing, error) {
	pairing, exists := s.pairings[pairingID]
	if !exists || pairing.Status != BrowserPairingStatusIssued || pairing.TokenSHA256 != tokenSHA256 || !pairing.ExpiresAt.After(claimedAt) || (pairing.ExpectedSourceCompanyID != "" && pairing.ExpectedSourceCompanyID != sourceCompanyID) {
		return nil, ErrBrowserPairingNotClaimable
	}
	pairing.Status = BrowserPairingStatusClaimed
	pairing.SourceCompanyID = sourceCompanyID
	pairing.ClaimedAt = &claimedAt
	s.pairings[pairingID] = pairing
	return &pairing, nil
}

func newBrowserPairingTestService(store *browserPairingMemoryStore) *BrowserPairingService {
	syncService := NewService(store, UnavailableBridgeCatalog{})
	service := NewBrowserPairingService(store, syncService)
	service.now = func() time.Time { return time.Date(2026, 8, 27, 15, 0, 0, 0, time.UTC) }
	service.newID = func() string { return browserPairingID }
	service.newToken = func() (string, error) { return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq", nil }
	return service
}

func TestBrowserPairingIssuesHashOnlyThenClaimsBrowserSourceBinding(t *testing.T) {
	store := &browserPairingMemoryStore{}
	service := newBrowserPairingTestService(store)

	issue, err := service.Issue(context.Background(), browserPairingTenantID, "owner-1")

	require.NoError(t, err)
	require.NotNil(t, issue)
	assert.Equal(t, browserPairingID, issue.PairingID)
	assert.NotEmpty(t, issue.PairingToken)
	stored := store.pairings[browserPairingID]
	assert.NotEqual(t, issue.PairingToken, stored.TokenSHA256)
	assert.Equal(t, browserPairingTokenSHA256(issue.PairingToken), stored.TokenSHA256)
	assert.Empty(t, stored.SourceCompanyID)
	assert.NotContains(t, mustJSON(t, stored), issue.PairingToken)

	claimed, err := service.Claim(context.Background(), issue.PairingID, issue.PairingToken, browserSourceID)

	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, BrowserPairingStatusClaimed, claimed.Status)
	assert.Equal(t, browserSourceID, claimed.SourceCompanyID)
	assert.NotNil(t, claimed.ClaimedAt)
	assert.NotContains(t, mustJSON(t, claimed), issue.PairingToken)
	control := store.controls[controlKey(browserPairingTenantID, browserSourceID)]
	assert.Equal(t, browserSessionReference(browserPairingID), control.SecretReference)
	assert.Equal(t, "SmartAccounts browser session", control.SourceCompanyName)
	assert.NotContains(t, mustJSON(t, control), issue.PairingToken)

	status, err := service.Status(context.Background(), browserPairingTenantID, issue.PairingID)
	require.NoError(t, err)
	assert.Equal(t, browserSourceID, status.SourceCompanyID)
	assert.Equal(t, BrowserPairingStatusClaimed, status.Status)
}

func TestBrowserPairingRejectsReplayWrongTokenInvalidSourceAndOtherTenant(t *testing.T) {
	store := &browserPairingMemoryStore{}
	service := newBrowserPairingTestService(store)
	issue, err := service.Issue(context.Background(), browserPairingTenantID, "owner-1")
	require.NoError(t, err)

	_, err = service.Claim(context.Background(), issue.PairingID, "wrong", browserSourceID)
	assert.ErrorIs(t, err, ErrBrowserPairingNotClaimable)
	_, err = service.Claim(context.Background(), issue.PairingID, issue.PairingToken, "untrusted-company-id")
	assert.ErrorIs(t, err, ErrBrowserPairingNotClaimable)
	_, err = service.Status(context.Background(), "60f7e37b-aa4b-4306-a205-0b1a8fdca0ae", issue.PairingID)
	assert.ErrorIs(t, err, ErrBrowserPairingNotClaimable)

	_, err = service.Claim(context.Background(), issue.PairingID, issue.PairingToken, browserSourceID)
	require.NoError(t, err)
	_, err = service.Claim(context.Background(), issue.PairingID, issue.PairingToken, browserSourceID)
	assert.ErrorIs(t, err, ErrBrowserPairingNotClaimable)
}

func TestBrowserPairingExpectedSourceRejectsSelectorSubstitution(t *testing.T) {
	store := &browserPairingMemoryStore{}
	service := newBrowserPairingTestService(store)
	issue, err := service.IssueForExpectedSource(context.Background(), browserPairingTenantID, "owner-1", browserSourceID)
	require.NoError(t, err)

	_, err = service.Claim(context.Background(), issue.PairingID, issue.PairingToken, "sa-browser-v1-654321")
	assert.ErrorIs(t, err, ErrBrowserPairingNotClaimable)
	claimed, err := service.Claim(context.Background(), issue.PairingID, issue.PairingToken, browserSourceID)
	require.NoError(t, err)
	assert.Equal(t, browserSourceID, claimed.SourceCompanyID)
}

func TestBrowserPairingBindingNeverCallsAPIKeyBridgeCapture(t *testing.T) {
	store := &browserPairingMemoryStore{}
	pairing := newBrowserPairingTestService(store)
	issue, err := pairing.Issue(context.Background(), browserPairingTenantID, "owner-1")
	require.NoError(t, err)
	_, err = pairing.Claim(context.Background(), issue.PairingID, issue.PairingToken, browserSourceID)
	require.NoError(t, err)

	syncService := pairing.syncService
	status, err := syncService.StartCapture(context.Background(), browserPairingTenantID, browserSourceID, CaptureRequest{ScopeMode: "full_history"}, nil)

	assert.ErrorIs(t, err, ErrBrowserCaptureRequired)
	require.NotNil(t, status)
	assert.Equal(t, "AWAITING_BRAVE_BROWSER_CAPTURE", status.CaptureStatus)
	assert.Empty(t, store.captureProgress)
}
