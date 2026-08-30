package smartaccountssync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	BrowserOnboardingCatalogHandoffSchemaVersion = "smartaccounts-browser-source-catalog-handoff-v1"
	BrowserOnboardingCatalogSchemaVersion        = "smartaccounts-browser-source-catalog-v1"
	BrowserOnboardingCatalogIntentVersion        = "smartaccounts-browser-source-catalog-intent-v1"
	BrowserOnboardingCatalogSourceIDVersion      = "sa-browser-v1"
	BrowserOnboardingCatalogDigestAlgorithm      = "sha256"
	BrowserOnboardingCatalogStatusIssued         = "ISSUED"
	BrowserOnboardingCatalogStatusAccepted       = "ACCEPTED"

	browserOnboardingCatalogLifetime        = 2 * time.Minute
	browserOnboardingCatalogReceiptLifetime = 10 * time.Minute
)

var (
	ErrBrowserOnboardingCatalogUnauthorized = errors.New("SmartAccounts browser source catalog capability is invalid, expired, or not scoped to this request")
	ErrBrowserOnboardingCatalogInvalid      = errors.New("SmartAccounts browser source catalog handoff is invalid")
	ErrBrowserOnboardingCatalogConflict     = errors.New("SmartAccounts browser source catalog handoff conflicts with its immutable receipt")
	ErrBrowserOnboardingCatalogUnavailable  = errors.New("SmartAccounts browser source catalog is unavailable")
)

// BrowserOnboardingCatalogConsent is action-time owner consent for reading
// only the visible picker metadata. It cannot authorize record transfer,
// browser-cookie access, financial posting, or package application.
type BrowserOnboardingCatalogConsent struct {
	Version     int       `json:"version"`
	Confirmed   bool      `json:"confirmed"`
	ConfirmedAt time.Time `json:"confirmed_at"`
	Scope       string    `json:"scope"`
}

type BrowserOnboardingCatalogIssueRequest struct {
	CatalogConsent BrowserOnboardingCatalogConsent `json:"catalog_consent"`
}

// BrowserOnboardingCatalogDigestIntent is a fixed relay protocol declaration.
// No request can choose its schema, identity version, or digest algorithm.
type BrowserOnboardingCatalogDigestIntent struct {
	Version              string `json:"version"`
	CatalogSchemaVersion string `json:"catalog_schema_version"`
	SourceIDVersion      string `json:"source_id_version"`
	DigestAlgorithm      string `json:"digest_algorithm"`
}

// BrowserOnboardingCatalogIssue returns its raw catalog token once to the
// authenticated owner page. The UI sends the envelope immediately by
// postMessage to the extension and keeps no token in persistent state.
type BrowserOnboardingCatalogIssue struct {
	CatalogID           string                               `json:"catalog_id"`
	WorkflowID          string                               `json:"workflow_id"`
	CatalogToken        string                               `json:"catalog_token"`
	Nonce               string                               `json:"nonce"`
	IssuedAt            time.Time                            `json:"issued_at"`
	ExpiresAt           time.Time                            `json:"expires_at"`
	CatalogDigestIntent BrowserOnboardingCatalogDigestIntent `json:"catalog_digest_intent"`
	CatalogConsent      BrowserOnboardingCatalogConsent      `json:"catalog_consent"`
}

// BrowserOnboardingCatalogCompany is relay-visible picker metadata only. Its
// display name is used solely for target-tenant creation/review; no source row
// is accepted at this boundary.
type BrowserOnboardingCatalogCompany struct {
	SourceCompanyID string `json:"source_company_id"`
	DisplayName     string `json:"display_name"`
}

// BrowserOnboardingCatalogHandoff is the strict extension-worker request.
// The raw authorization is only its Bearer token; HTTP cookies are not read.
type BrowserOnboardingCatalogHandoff struct {
	SchemaVersion string                            `json:"schema_version"`
	CatalogID     string                            `json:"catalog_id"`
	WorkflowID    string                            `json:"workflow_id"`
	Nonce         string                            `json:"nonce"`
	CatalogCount  int                               `json:"catalog_count"`
	CatalogSHA256 string                            `json:"catalog_sha256"`
	Companies     []BrowserOnboardingCatalogCompany `json:"companies"`
}

type BrowserOnboardingCatalogHandoffStatus struct {
	Status        string `json:"status"`
	CatalogID     string `json:"catalog_id"`
	CatalogSHA256 string `json:"catalog_sha256"`
	CatalogCount  int    `json:"catalog_count"`
}

// BrowserOnboardingCatalogStatus is the authenticated owner's safe view of an
// accepted relay receipt. The companies are picker metadata required for an
// explicit selected/all decision; raw capabilities and their digests are
// deliberately never returned.
type BrowserOnboardingCatalogStatus struct {
	CatalogID     string                            `json:"catalog_id"`
	WorkflowID    string                            `json:"workflow_id"`
	Status        string                            `json:"status"`
	CatalogSHA256 string                            `json:"catalog_sha256,omitempty"`
	CatalogCount  int                               `json:"catalog_count,omitempty"`
	ObservedAt    time.Time                         `json:"observed_at,omitempty"`
	ExpiresAt     time.Time                         `json:"expires_at"`
	Companies     []BrowserOnboardingCatalogCompany `json:"companies,omitempty"`
}

// BrowserOnboardingCatalogStore retains token/nonce hashes and accepted
// picker metadata. It never receives a raw token, cookie, SmartAccounts API
// credential, source row, or financial instruction.
type BrowserOnboardingCatalogStore interface {
	CreateBrowserOnboardingCatalogReceipt(context.Context, BrowserOnboardingCatalogReceipt) error
	GetBrowserOnboardingCatalogReceipt(context.Context, string, string) (*BrowserOnboardingCatalogReceipt, error)
	AcceptBrowserOnboardingCatalogReceipt(context.Context, BrowserOnboardingCatalogReceipt) (*BrowserOnboardingCatalogReceipt, bool, error)
}

// BrowserOnboardingCatalogService owns the owner issue → extension handoff →
// batch-receipt boundary. Its accepted receipt implements the batch's source
// catalog reader, which prevents a batch endpoint from accepting a weaker
// caller-supplied list of source companies.
type BrowserOnboardingCatalogService struct {
	store    BrowserOnboardingCatalogStore
	now      func() time.Time
	newID    func() string
	newToken func() (string, error)
}

func NewBrowserOnboardingCatalogService(store BrowserOnboardingCatalogStore) *BrowserOnboardingCatalogService {
	return &BrowserOnboardingCatalogService{store: store, now: time.Now, newID: uuid.NewString, newToken: newBrowserPairingToken}
}

func (s *BrowserOnboardingCatalogService) Issue(ctx context.Context, ownerID string, request BrowserOnboardingCatalogIssueRequest) (*BrowserOnboardingCatalogIssue, error) {
	if s == nil || s.store == nil || s.newID == nil || s.newToken == nil || strings.TrimSpace(ownerID) == "" {
		return nil, ErrBrowserOnboardingCatalogUnavailable
	}
	now := s.currentTime()
	if !validBrowserOnboardingCatalogConsent(request.CatalogConsent, now) {
		return nil, ErrBrowserOnboardingCatalogInvalid
	}
	catalogID, workflowID := s.newID(), s.newID()
	if !validBrowserPairingID(catalogID) || !validBrowserPairingID(workflowID) {
		return nil, ErrBrowserOnboardingCatalogUnavailable
	}
	token, err := s.newToken()
	if err != nil || !validBrowserPairingToken(token) {
		return nil, ErrBrowserOnboardingCatalogUnavailable
	}
	nonce, err := s.newToken()
	if err != nil || !validBrowserPairingToken(nonce) {
		return nil, ErrBrowserOnboardingCatalogUnavailable
	}
	receipt := BrowserOnboardingCatalogReceipt{ID: catalogID, WorkflowID: workflowID, OwnerID: strings.TrimSpace(ownerID), TokenSHA256: browserPairingTokenSHA256(token), NonceSHA256: browserPairingTokenSHA256(nonce), SchemaVersion: BrowserOnboardingCatalogSchemaVersion, IntentVersion: BrowserOnboardingCatalogIntentVersion, SourceIDVersion: BrowserOnboardingCatalogSourceIDVersion, DigestAlgorithm: BrowserOnboardingCatalogDigestAlgorithm, Status: BrowserOnboardingCatalogStatusIssued, ExpiresAt: now.Add(browserOnboardingCatalogLifetime), CreatedAt: now, UpdatedAt: now}
	if err := s.store.CreateBrowserOnboardingCatalogReceipt(ctx, receipt); err != nil {
		return nil, ErrBrowserOnboardingCatalogUnavailable
	}
	return &BrowserOnboardingCatalogIssue{CatalogID: catalogID, WorkflowID: workflowID, CatalogToken: token, Nonce: nonce, IssuedAt: now, ExpiresAt: receipt.ExpiresAt, CatalogDigestIntent: browserOnboardingCatalogDigestIntent(), CatalogConsent: request.CatalogConsent}, nil
}

// Handoff accepts only a strict extension-worker snapshot while the issued
// capability remains current. An identical retry is harmless; any changed
// catalog for the same receipt is a conflict and cannot broaden an all batch.
func (s *BrowserOnboardingCatalogService) Handoff(ctx context.Context, catalogID, token string, handoff BrowserOnboardingCatalogHandoff) (*BrowserOnboardingCatalogHandoffStatus, error) {
	if s == nil || s.store == nil || !validBrowserPairingID(strings.TrimSpace(catalogID)) || !validBrowserPairingToken(token) {
		return nil, ErrBrowserOnboardingCatalogUnauthorized
	}
	receipt, err := s.store.GetBrowserOnboardingCatalogReceipt(ctx, "", strings.TrimSpace(catalogID))
	if err != nil || receipt == nil {
		return nil, ErrBrowserOnboardingCatalogUnauthorized
	}
	now := s.currentTime()
	if !validBrowserOnboardingCatalogCapability(*receipt, token, now) {
		return nil, ErrBrowserOnboardingCatalogUnauthorized
	}
	companies, digest, err := canonicalBrowserOnboardingCatalogHandoff(handoff)
	if err != nil || handoff.CatalogID != receipt.ID || handoff.WorkflowID != receipt.WorkflowID || browserPairingTokenSHA256(handoff.Nonce) != receipt.NonceSHA256 {
		return nil, ErrBrowserOnboardingCatalogInvalid
	}
	if receipt.Status == BrowserOnboardingCatalogStatusAccepted {
		if receipt.CatalogSHA256 != digest || !sameBrowserOnboardingCatalogCompanies(companies, sourcesToBrowserOnboardingCatalogCompanies(receipt.Sources)) {
			return nil, ErrBrowserOnboardingCatalogConflict
		}
		return &BrowserOnboardingCatalogHandoffStatus{Status: "already_accepted", CatalogID: receipt.ID, CatalogSHA256: receipt.CatalogSHA256, CatalogCount: receipt.CatalogCount}, nil
	}
	receipt.Status = BrowserOnboardingCatalogStatusAccepted
	receipt.CatalogSHA256 = digest
	receipt.CatalogCount = len(companies)
	receipt.Sources = browserOnboardingCatalogCompaniesToSources(companies)
	receipt.ObservedAt, receipt.AcceptedAt, receipt.UpdatedAt = now, now, now
	receipt.ReceiptExpiresAt = now.Add(browserOnboardingCatalogReceiptLifetime)
	persisted, accepted, err := s.store.AcceptBrowserOnboardingCatalogReceipt(ctx, *receipt)
	if err != nil || persisted == nil {
		return nil, ErrBrowserOnboardingCatalogUnavailable
	}
	if !accepted {
		if persisted.Status != BrowserOnboardingCatalogStatusAccepted || persisted.CatalogSHA256 != digest || !sameBrowserOnboardingCatalogCompanies(companies, sourcesToBrowserOnboardingCatalogCompanies(persisted.Sources)) {
			return nil, ErrBrowserOnboardingCatalogConflict
		}
		return &BrowserOnboardingCatalogHandoffStatus{Status: "already_accepted", CatalogID: persisted.ID, CatalogSHA256: persisted.CatalogSHA256, CatalogCount: persisted.CatalogCount}, nil
	}
	return &BrowserOnboardingCatalogHandoffStatus{Status: "accepted", CatalogID: persisted.ID, CatalogSHA256: persisted.CatalogSHA256, CatalogCount: persisted.CatalogCount}, nil
}

// GetBrowserOnboardingCatalogReceipt implements BrowserOnboardingSourceCatalog
// for the batch service. The owner check and current expiry gate happen before
// any source list reaches an immutable selected/all manifest.
func (s *BrowserOnboardingCatalogService) GetBrowserOnboardingCatalogReceipt(ctx context.Context, ownerID, catalogID string) (*BrowserOnboardingCatalogReceipt, error) {
	if s == nil || s.store == nil || strings.TrimSpace(ownerID) == "" || !validBrowserPairingID(strings.TrimSpace(catalogID)) {
		return nil, ErrBrowserOnboardingCatalogUnauthorized
	}
	receipt, err := s.store.GetBrowserOnboardingCatalogReceipt(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(catalogID))
	if err != nil || receipt == nil || !validBrowserOnboardingCatalogReceipt(receipt, ownerID, s.currentTime()) || receipt.Status != BrowserOnboardingCatalogStatusAccepted || !validSHA256(receipt.CatalogSHA256) || receipt.CatalogCount != len(receipt.Sources) {
		return nil, ErrBrowserOnboardingCatalogUnauthorized
	}
	return receipt, nil
}

// Status returns accepted picker metadata only to the owner who issued the
// receipt. Relay postMessage status continues to contain only count/digest.
func (s *BrowserOnboardingCatalogService) Status(ctx context.Context, ownerID, catalogID string) (*BrowserOnboardingCatalogStatus, error) {
	receipt, err := s.GetBrowserOnboardingCatalogReceipt(ctx, ownerID, catalogID)
	if err != nil || receipt == nil {
		return nil, ErrBrowserOnboardingCatalogUnauthorized
	}
	return &BrowserOnboardingCatalogStatus{
		CatalogID:     receipt.ID,
		WorkflowID:    receipt.WorkflowID,
		Status:        receipt.Status,
		CatalogSHA256: receipt.CatalogSHA256,
		CatalogCount:  receipt.CatalogCount,
		ObservedAt:    receipt.ObservedAt,
		ExpiresAt:     receipt.ReceiptExpiresAt,
		Companies:     sourcesToBrowserOnboardingCatalogCompanies(receipt.Sources),
	}, nil
}

func browserOnboardingCatalogDigestIntent() BrowserOnboardingCatalogDigestIntent {
	return BrowserOnboardingCatalogDigestIntent{Version: BrowserOnboardingCatalogIntentVersion, CatalogSchemaVersion: BrowserOnboardingCatalogSchemaVersion, SourceIDVersion: BrowserOnboardingCatalogSourceIDVersion, DigestAlgorithm: BrowserOnboardingCatalogDigestAlgorithm}
}

func canonicalBrowserOnboardingCatalogHandoff(handoff BrowserOnboardingCatalogHandoff) ([]BrowserOnboardingCatalogCompany, string, error) {
	if handoff.SchemaVersion != BrowserOnboardingCatalogHandoffSchemaVersion || !validBrowserPairingID(handoff.CatalogID) || !validBrowserPairingID(handoff.WorkflowID) || !validBrowserPairingToken(handoff.Nonce) || handoff.CatalogCount != len(handoff.Companies) || handoff.CatalogCount < 1 || handoff.CatalogCount > BrowserOnboardingMaxSources || !validSHA256(handoff.CatalogSHA256) {
		return nil, "", ErrBrowserOnboardingCatalogInvalid
	}
	companies, ok := canonicalBrowserOnboardingCatalogCompanies(handoff.Companies)
	if !ok || !sameBrowserOnboardingCatalogCompanies(companies, handoff.Companies) {
		return nil, "", ErrBrowserOnboardingCatalogInvalid
	}
	encoded, err := jsonMarshalBrowserOnboardingCatalogDigest(BrowserOnboardingCatalogSchemaVersion, companies)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(encoded)
	digest := hex.EncodeToString(sum[:])
	if digest != handoff.CatalogSHA256 {
		return nil, "", ErrBrowserOnboardingCatalogInvalid
	}
	return companies, digest, nil
}

func jsonMarshalBrowserOnboardingCatalogDigest(schemaVersion string, companies []BrowserOnboardingCatalogCompany) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(struct {
		SchemaVersion   string                            `json:"schema_version"`
		SourceIDVersion string                            `json:"source_id_version"`
		Companies       []BrowserOnboardingCatalogCompany `json:"companies"`
	}{SchemaVersion: schemaVersion, SourceIDVersion: BrowserOnboardingCatalogSourceIDVersion, Companies: companies}); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

func canonicalBrowserOnboardingCatalogCompanies(input []BrowserOnboardingCatalogCompany) ([]BrowserOnboardingCatalogCompany, bool) {
	if len(input) == 0 || len(input) > BrowserOnboardingMaxSources {
		return nil, false
	}
	output := make([]BrowserOnboardingCatalogCompany, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, company := range input {
		id := strings.TrimSpace(company.SourceCompanyID)
		name := company.DisplayName
		if !validBrowserSourceCompanyID(id) || !validBrowserOnboardingCatalogDisplayName(name) {
			return nil, false
		}
		if _, duplicate := seen[id]; duplicate {
			return nil, false
		}
		seen[id] = struct{}{}
		output = append(output, BrowserOnboardingCatalogCompany{SourceCompanyID: id, DisplayName: name})
	}
	sort.Slice(output, func(i, j int) bool { return output[i].SourceCompanyID < output[j].SourceCompanyID })
	return output, true
}

func validBrowserOnboardingCatalogDisplayName(value string) bool {
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 120 || strings.Join(strings.Fields(value), " ") != value {
		return false
	}
	for _, character := range value {
		if character <= 0x1f || character == 0x7f {
			return false
		}
	}
	return true
}

func sameBrowserOnboardingCatalogCompanies(left, right []BrowserOnboardingCatalogCompany) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func browserOnboardingCatalogCompaniesToSources(companies []BrowserOnboardingCatalogCompany) []BrowserOnboardingSource {
	sources := make([]BrowserOnboardingSource, 0, len(companies))
	for _, company := range companies {
		sources = append(sources, BrowserOnboardingSource{SourceCompanyID: company.SourceCompanyID, SourceCompanyName: company.DisplayName})
	}
	return sources
}

func sourcesToBrowserOnboardingCatalogCompanies(sources []BrowserOnboardingSource) []BrowserOnboardingCatalogCompany {
	companies := make([]BrowserOnboardingCatalogCompany, 0, len(sources))
	for _, source := range sources {
		companies = append(companies, BrowserOnboardingCatalogCompany{SourceCompanyID: source.SourceCompanyID, DisplayName: source.SourceCompanyName})
	}
	return companies
}

func validBrowserOnboardingCatalogConsent(consent BrowserOnboardingCatalogConsent, now time.Time) bool {
	return consent.Version == 1 && consent.Confirmed && consent.Scope == "visible_company_catalog" && !consent.ConfirmedAt.IsZero() && !consent.ConfirmedAt.Before(now.Add(-2*time.Minute)) && !consent.ConfirmedAt.After(now.Add(30*time.Second))
}

func validBrowserOnboardingCatalogCapability(receipt BrowserOnboardingCatalogReceipt, token string, now time.Time) bool {
	return (receipt.Status == BrowserOnboardingCatalogStatusIssued || receipt.Status == BrowserOnboardingCatalogStatusAccepted) && validBrowserPairingID(receipt.ID) && validBrowserPairingID(receipt.WorkflowID) && validSHA256(receipt.TokenSHA256) && validSHA256(receipt.NonceSHA256) && receipt.ExpiresAt.After(now) && receipt.SchemaVersion == BrowserOnboardingCatalogSchemaVersion && receipt.IntentVersion == BrowserOnboardingCatalogIntentVersion && receipt.SourceIDVersion == BrowserOnboardingCatalogSourceIDVersion && receipt.DigestAlgorithm == BrowserOnboardingCatalogDigestAlgorithm && browserPairingTokenSHA256(token) == receipt.TokenSHA256
}

func (s *BrowserOnboardingCatalogService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}
