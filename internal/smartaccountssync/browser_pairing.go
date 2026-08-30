package smartaccountssync

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BrowserPairingStatusIssued  = "ISSUED"
	BrowserPairingStatusClaimed = "CLAIMED"
	browserPairingLifetime      = 10 * time.Minute
	browserSessionReferenceURI  = "brave-session://"
)

var (
	ErrBrowserPairingUnavailable  = errors.New("SmartAccounts Brave pairing is unavailable")
	ErrBrowserPairingNotClaimable = errors.New("SmartAccounts Brave pairing is invalid, expired, or already claimed")
)

// BrowserPairing stores only a one-time token hash. Its TokenSHA256 is never
// included in an HTTP response. A raw token exists only between issuing it to
// the authenticated OA page and the browser relay's one-time claim request.
type BrowserPairing struct {
	ID          string
	TenantID    string
	TokenSHA256 string
	// ExpectedSourceCompanyID is set only for owner-authorized selected-company
	// onboarding. The relay claim must match it exactly, so it cannot bind a
	// different visible SmartAccounts company to the created/reused tenant.
	ExpectedSourceCompanyID string
	SourceCompanyID         string
	CreatedBy               string
	Status                  string
	ExpiresAt               time.Time
	ClaimedAt               *time.Time
	CreatedAt               time.Time
}

// BrowserPairingIssue is returned only to the authenticated tenant owner so a
// locally installed Brave relay can receive a short-lived pairing token. It
// must not be persisted, logged, or returned from status endpoints.
type BrowserPairingIssue struct {
	PairingID    string    `json:"pairing_id"`
	PairingToken string    `json:"pairing_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// BrowserPairingStatus is safe to return to either the tenant owner or the
// browser relay. It contains no raw pairing token, token hash, browser state,
// or source data.
type BrowserPairingStatus struct {
	PairingID       string     `json:"pairing_id"`
	Status          string     `json:"status"`
	ExpiresAt       time.Time  `json:"expires_at"`
	ClaimedAt       *time.Time `json:"claimed_at,omitempty"`
	SourceCompanyID string     `json:"source_company_id,omitempty"`
}

type BrowserPairingStore interface {
	CreateBrowserPairing(ctx context.Context, pairing BrowserPairing) error
	GetBrowserPairing(ctx context.Context, pairingID, tenantID string) (*BrowserPairing, error)
	ClaimBrowserPairing(ctx context.Context, pairingID, tokenSHA256, sourceCompanyID string, claimedAt time.Time) (*BrowserPairing, error)
}

// BrowserPairingService owns the non-financial one-time Brave pairing. It
// delegates the resulting tenant/source binding to Service but cannot fetch
// SmartAccounts data, receive browser state, build a package, or post a
// journal.
type BrowserPairingService struct {
	store       BrowserPairingStore
	syncService *Service
	now         func() time.Time
	newID       func() string
	newToken    func() (string, error)
}

func NewBrowserPairingService(store BrowserPairingStore, syncService *Service) *BrowserPairingService {
	return &BrowserPairingService{
		store:       store,
		syncService: syncService,
		now:         time.Now,
		newID:       uuid.NewString,
		newToken:    newBrowserPairingToken,
	}
}

func (s *BrowserPairingService) Issue(ctx context.Context, tenantID, actorID string) (*BrowserPairingIssue, error) {
	return s.issue(ctx, tenantID, actorID, "")
}

// IssueForExpectedSource creates a pairing for one owner-selected source
// selector. The selector is verified only when the Brave relay claims the
// token; no browser company data or credentials are accepted here.
func (s *BrowserPairingService) IssueForExpectedSource(ctx context.Context, tenantID, actorID, expectedSourceCompanyID string) (*BrowserPairingIssue, error) {
	if !validBrowserSourceCompanyID(expectedSourceCompanyID) {
		return nil, ErrBrowserPairingUnavailable
	}
	return s.issue(ctx, tenantID, actorID, strings.TrimSpace(expectedSourceCompanyID))
}

func (s *BrowserPairingService) issue(ctx context.Context, tenantID, actorID, expectedSourceCompanyID string) (*BrowserPairingIssue, error) {
	if s == nil || s.store == nil || s.newToken == nil || s.newID == nil {
		return nil, ErrBrowserPairingUnavailable
	}
	if !safeBridgeID(strings.TrimSpace(tenantID)) {
		return nil, ErrBrowserPairingUnavailable
	}
	token, err := s.newToken()
	if err != nil {
		return nil, ErrBrowserPairingUnavailable
	}
	now := s.currentTime()
	pairing := BrowserPairing{
		ID:                      s.newID(),
		TenantID:                strings.TrimSpace(tenantID),
		TokenSHA256:             browserPairingTokenSHA256(token),
		ExpectedSourceCompanyID: strings.TrimSpace(expectedSourceCompanyID),
		CreatedBy:               strings.TrimSpace(actorID),
		Status:                  BrowserPairingStatusIssued,
		ExpiresAt:               now.Add(browserPairingLifetime),
		CreatedAt:               now,
	}
	if !validBrowserPairing(pairing) {
		return nil, ErrBrowserPairingUnavailable
	}
	if err := s.store.CreateBrowserPairing(ctx, pairing); err != nil {
		return nil, ErrBrowserPairingUnavailable
	}
	return &BrowserPairingIssue{PairingID: pairing.ID, PairingToken: token, ExpiresAt: pairing.ExpiresAt}, nil
}

func (s *BrowserPairingService) Status(ctx context.Context, tenantID, pairingID string) (*BrowserPairingStatus, error) {
	if s == nil || s.store == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserPairingID(pairingID) {
		return nil, ErrBrowserPairingUnavailable
	}
	pairing, err := s.store.GetBrowserPairing(ctx, strings.TrimSpace(pairingID), strings.TrimSpace(tenantID))
	if err != nil {
		return nil, err
	}
	return statusForBrowserPairing(pairing, ""), nil
}

// Claim consumes the token before it creates the tenant/source binding, so a
// replay cannot substitute a different SmartAccounts company. A transient
// persistence failure fails closed: the owner issues a fresh pairing instead
// of reusing a possibly observed token.
func (s *BrowserPairingService) Claim(ctx context.Context, pairingID, pairingToken, sourceCompanyID string) (*BrowserPairingStatus, error) {
	if s == nil || s.store == nil || s.syncService == nil || !validBrowserPairingID(pairingID) || !validBrowserPairingToken(pairingToken) || !validBrowserSourceCompanyID(sourceCompanyID) {
		return nil, ErrBrowserPairingNotClaimable
	}
	pairing, err := s.store.ClaimBrowserPairing(ctx, strings.TrimSpace(pairingID), browserPairingTokenSHA256(pairingToken), strings.TrimSpace(sourceCompanyID), s.currentTime())
	if err != nil || pairing == nil || pairing.Status != BrowserPairingStatusClaimed || pairing.ClaimedAt == nil {
		return nil, ErrBrowserPairingNotClaimable
	}
	if _, err := s.syncService.ConfigureBrowserSession(ctx, pairing.TenantID, pairing.CreatedBy, pairing.ID, sourceCompanyID); err != nil {
		return nil, ErrBrowserPairingNotClaimable
	}
	return statusForBrowserPairing(pairing, pairing.SourceCompanyID), nil
}

func (s *BrowserPairingService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func newBrowserPairingToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func browserPairingTokenSHA256(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func statusForBrowserPairing(pairing *BrowserPairing, sourceCompanyID string) *BrowserPairingStatus {
	if pairing == nil {
		return nil
	}
	if strings.TrimSpace(sourceCompanyID) == "" {
		sourceCompanyID = pairing.SourceCompanyID
	}
	return &BrowserPairingStatus{PairingID: pairing.ID, Status: pairing.Status, ExpiresAt: pairing.ExpiresAt, ClaimedAt: pairing.ClaimedAt, SourceCompanyID: strings.TrimSpace(sourceCompanyID)}
}

func validBrowserPairing(pairing BrowserPairing) bool {
	return validBrowserPairingID(pairing.ID) && safeBridgeID(pairing.TenantID) && len(pairing.TokenSHA256) == 64 && pairing.Status == BrowserPairingStatusIssued && strings.TrimSpace(pairing.SourceCompanyID) == "" && (strings.TrimSpace(pairing.ExpectedSourceCompanyID) == "" || validBrowserSourceCompanyID(pairing.ExpectedSourceCompanyID)) && pairing.ExpiresAt.After(pairing.CreatedAt)
}

func validBrowserPairingID(value string) bool {
	_, err := uuid.Parse(strings.TrimSpace(value))
	return err == nil
}

func validBrowserPairingToken(value string) bool {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 43 {
		return false
	}
	for _, character := range trimmed {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func validBrowserSourceCompanyID(value string) bool {
	const prefix = "sa-browser-v1-"
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, prefix) || len(trimmed) <= len(prefix) || len(trimmed) > len(prefix)+20 {
		return false
	}
	for _, character := range trimmed[len(prefix):] {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func browserSessionReference(pairingID string) string {
	return browserSessionReferenceURI + strings.TrimSpace(pairingID)
}

func isBrowserSessionReference(reference string) bool {
	value := strings.TrimSpace(reference)
	return strings.HasPrefix(value, browserSessionReferenceURI) && validBrowserPairingID(strings.TrimPrefix(value, browserSessionReferenceURI))
}
