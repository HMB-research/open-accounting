package smartaccountssync

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	BrowserCSVSchemaReviewVersion    = "smartaccounts-browser-csv-schema-review-v1"
	BrowserCSVSchemaStatusPending    = "PENDING_BRIDGE_REGISTRATION"
	BrowserCSVSchemaStatusRegistered = "REGISTERED"
)

var (
	ErrBrowserCSVSchemaApprovalUnauthorized = errors.New("SmartAccounts browser CSV schema approval is not scoped to this tenant discovery")
	ErrBrowserCSVSchemaApprovalInvalid      = errors.New("SmartAccounts browser CSV schema approval is invalid")
	ErrBrowserCSVSchemaApprovalConflict     = errors.New("SmartAccounts browser CSV schema approval conflicts with its immutable review")
	ErrBrowserCSVSchemaApprovalNotFound     = errors.New("SmartAccounts browser CSV schema approval was not found")
	ErrBrowserCSVSchemaApprovalUnavailable  = errors.New("SmartAccounts browser CSV schema approval is unavailable")
)

// BrowserCSVSchemaApprovalRequest contains only action-time owner consent. OA
// derives review audit metadata itself; callers cannot select source bindings,
// submit headers, supply an audit ID, or carry browser/source data.
type BrowserCSVSchemaApprovalRequest struct {
	ReviewConfirmed bool `json:"review_confirmed"`
}

// BrowserCSVSchemaReview is retained only to retry the exact bridge assertion.
// AuditID is opaque and never returned to the browser-facing response.
type BrowserCSVSchemaReview struct {
	Version    string    `json:"version"`
	Confirmed  bool      `json:"confirmed"`
	ReviewedAt time.Time `json:"reviewed_at"`
	AuditID    string    `json:"audit_id"`
}

// BrowserCSVSchemaApproval is OA's durable, aggregate-only review record. It
// deliberately omits the private source selector, discovery header names,
// browser control IDs, cookies, credentials, bridge token, and raw CSV data.
type BrowserCSVSchemaApproval struct {
	TenantID       string
	DiscoveryID    string
	ResourceID     string
	SchemaID       string
	Review         BrowserCSVSchemaReview
	ReviewedBy     string
	Status         string
	ApprovalSHA256 string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BrowserCSVSchemaApprovalResponse is the only public representation of a
// reviewed schema. It is binding-safe metadata plus an immutable digest.
type BrowserCSVSchemaApprovalResponse struct {
	ResourceID     string `json:"resource_id"`
	SchemaID       string `json:"schema_id"`
	Status         string `json:"status"`
	ApprovalSHA256 string `json:"approval_sha256"`
}

// BrowserCSVSchemaApprovalBridgeRequest is the exact private bridge POST
// payload. It has no header names or source/browser data.
type BrowserCSVSchemaApprovalBridgeRequest struct {
	DiscoveryID string                 `json:"discovery_id"`
	SchemaID    string                 `json:"schema_id"`
	Review      BrowserCSVSchemaReview `json:"review"`
}

// BrowserCSVSchemaApprovalBridge is server-to-server only. sourceCompanyID is
// resolved from the existing OA discovery authorization and never comes from
// the public route or response.
type BrowserCSVSchemaApprovalBridge interface {
	RegisterBrowserCSVSchemaApproval(context.Context, string, string, string, string, BrowserCSVSchemaApprovalBridgeRequest) (BrowserCSVSchemaApprovalResponse, error)
	GetBrowserCSVSchemaApproval(context.Context, string, string, string, string) (BrowserCSVSchemaApprovalResponse, error)
}

// BrowserCSVSchemaApprovalStore persists only review/audit/digest metadata.
// Its unique tenant/discovery/resource key prevents two schema adapters being
// implicitly selected for one captured source resource.
type BrowserCSVSchemaApprovalStore interface {
	GetBrowserDiscoveryAuthorization(context.Context, string, string) (*BrowserDiscoveryAuthorization, error)
	FindOrCreateBrowserCSVSchemaApproval(context.Context, BrowserCSVSchemaApproval) (*BrowserCSVSchemaApproval, bool, error)
	GetBrowserCSVSchemaApproval(context.Context, string, string, string) (*BrowserCSVSchemaApproval, error)
	MarkBrowserCSVSchemaApprovalRegistered(context.Context, BrowserCSVSchemaApproval, time.Time) (*BrowserCSVSchemaApproval, error)
}

// BrowserCSVSchemaApprovalService binds an owner-confirmed reviewed schema to
// an already persisted tenant/discovery/source relationship. It cannot issue a
// discovery, start a capture, upload a resource, compile a package, or apply
// accounting records.
type BrowserCSVSchemaApprovalService struct {
	store    BrowserCSVSchemaApprovalStore
	controls BrowserDiscoveryControlReader
	bridge   BrowserCSVSchemaApprovalBridge
	now      func() time.Time
	newID    func() string
}

func NewBrowserCSVSchemaApprovalService(store BrowserCSVSchemaApprovalStore, controls BrowserDiscoveryControlReader, bridge BrowserCSVSchemaApprovalBridge) *BrowserCSVSchemaApprovalService {
	return &BrowserCSVSchemaApprovalService{store: store, controls: controls, bridge: bridge, now: time.Now, newID: uuid.NewString}
}

// Review returns created=false for an immutable replay or a retry of a
// previously persisted pending review. The public handler uses that only for
// HTTP 200/201 semantics; it never exposes audit or source metadata.
func (s *BrowserCSVSchemaApprovalService) Review(ctx context.Context, tenantID, actor, discoveryID, resourceID, schemaID string, request BrowserCSVSchemaApprovalRequest) (BrowserCSVSchemaApprovalResponse, bool, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || s.newID == nil || !request.ReviewConfirmed {
		return BrowserCSVSchemaApprovalResponse{}, false, ErrBrowserCSVSchemaApprovalInvalid
	}
	authorization, err := s.authorization(ctx, tenantID, discoveryID)
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, false, err
	}
	resourceID, schemaID = strings.TrimSpace(resourceID), strings.TrimSpace(schemaID)
	if !approvedBrowserGeneralLedgerSchema(resourceID, schemaID) {
		return BrowserCSVSchemaApprovalResponse{}, false, ErrBrowserCSVSchemaApprovalInvalid
	}
	// The private bridge hashes the RFC3339 review envelope. Keep it at whole
	// seconds so a database round-trip cannot change a retry's assertion.
	now := s.currentTime().Truncate(time.Second)
	candidate := BrowserCSVSchemaApproval{
		TenantID: authorization.TenantID, DiscoveryID: authorization.DiscoveryID,
		ResourceID: resourceID, SchemaID: schemaID,
		Review:     BrowserCSVSchemaReview{Version: BrowserCSVSchemaReviewVersion, Confirmed: true, ReviewedAt: now, AuditID: s.newID()},
		ReviewedBy: strings.TrimSpace(actor), Status: BrowserCSVSchemaStatusPending,
		CreatedAt: now, UpdatedAt: now,
	}
	if !validBrowserCSVSchemaApproval(candidate) {
		return BrowserCSVSchemaApprovalResponse{}, false, ErrBrowserCSVSchemaApprovalUnavailable
	}
	persisted, created, err := s.store.FindOrCreateBrowserCSVSchemaApproval(ctx, candidate)
	if err != nil || persisted == nil {
		return BrowserCSVSchemaApprovalResponse{}, false, ErrBrowserCSVSchemaApprovalUnavailable
	}
	if persisted.TenantID != authorization.TenantID || persisted.DiscoveryID != authorization.DiscoveryID || persisted.ResourceID != resourceID || persisted.SchemaID != schemaID {
		return BrowserCSVSchemaApprovalResponse{}, false, ErrBrowserCSVSchemaApprovalConflict
	}
	if !validBrowserCSVSchemaApproval(*persisted) {
		return BrowserCSVSchemaApprovalResponse{}, false, ErrBrowserCSVSchemaApprovalUnavailable
	}
	response, err := s.register(ctx, authorization, *persisted)
	return response, created, err
}

func (s *BrowserCSVSchemaApprovalService) Status(ctx context.Context, tenantID, discoveryID, resourceID, schemaID string) (BrowserCSVSchemaApprovalResponse, error) {
	authorization, err := s.authorization(ctx, tenantID, discoveryID)
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, err
	}
	resourceID, schemaID = strings.TrimSpace(resourceID), strings.TrimSpace(schemaID)
	if !approvedBrowserGeneralLedgerSchema(resourceID, schemaID) {
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalInvalid
	}
	persisted, err := s.store.GetBrowserCSVSchemaApproval(ctx, authorization.TenantID, authorization.DiscoveryID, resourceID)
	if err != nil || persisted == nil {
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalNotFound
	}
	if persisted.SchemaID != schemaID || !validBrowserCSVSchemaApproval(*persisted) {
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalConflict
	}
	response, err := s.bridge.GetBrowserCSVSchemaApproval(ctx, authorization.TenantID, authorization.SourceCompanyID, resourceID, schemaID)
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, err
	}
	return s.recordRegistered(ctx, *persisted, response)
}

func (s *BrowserCSVSchemaApprovalService) register(ctx context.Context, authorization *BrowserDiscoveryAuthorization, approval BrowserCSVSchemaApproval) (BrowserCSVSchemaApprovalResponse, error) {
	response, err := s.bridge.RegisterBrowserCSVSchemaApproval(ctx, authorization.TenantID, authorization.SourceCompanyID, approval.ResourceID, approval.SchemaID, BrowserCSVSchemaApprovalBridgeRequest{
		DiscoveryID: approval.DiscoveryID, SchemaID: approval.SchemaID, Review: approval.Review,
	})
	if err != nil {
		return BrowserCSVSchemaApprovalResponse{}, err
	}
	return s.recordRegistered(ctx, approval, response)
}

func (s *BrowserCSVSchemaApprovalService) recordRegistered(ctx context.Context, approval BrowserCSVSchemaApproval, response BrowserCSVSchemaApprovalResponse) (BrowserCSVSchemaApprovalResponse, error) {
	if response.ResourceID != approval.ResourceID || response.SchemaID != approval.SchemaID || response.Status != "registered" || !validSHA256(response.ApprovalSHA256) {
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalUnavailable
	}
	if approval.ApprovalSHA256 != "" && approval.ApprovalSHA256 != response.ApprovalSHA256 {
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalConflict
	}
	approval.Status, approval.ApprovalSHA256 = BrowserCSVSchemaStatusRegistered, response.ApprovalSHA256
	approval.UpdatedAt = s.currentTime()
	persisted, err := s.store.MarkBrowserCSVSchemaApprovalRegistered(ctx, approval, approval.UpdatedAt)
	if err != nil || persisted == nil || !validBrowserCSVSchemaApproval(*persisted) || persisted.ApprovalSHA256 != response.ApprovalSHA256 || persisted.Status != BrowserCSVSchemaStatusRegistered {
		return BrowserCSVSchemaApprovalResponse{}, ErrBrowserCSVSchemaApprovalUnavailable
	}
	return response, nil
}

func (s *BrowserCSVSchemaApprovalService) authorization(ctx context.Context, tenantID, discoveryID string) (*BrowserDiscoveryAuthorization, error) {
	if s == nil || s.store == nil || s.controls == nil || s.bridge == nil || !safeBridgeID(strings.TrimSpace(tenantID)) || !validBrowserDiscoveryID(strings.TrimSpace(discoveryID)) {
		return nil, ErrBrowserCSVSchemaApprovalUnauthorized
	}
	authorization, err := s.store.GetBrowserDiscoveryAuthorization(ctx, strings.TrimSpace(tenantID), strings.TrimSpace(discoveryID))
	if err != nil || authorization == nil || !validBrowserDiscoveryAuthorization(*authorization) {
		return nil, ErrBrowserCSVSchemaApprovalUnauthorized
	}
	control, err := s.controls.Get(ctx, authorization.TenantID, authorization.SourceCompanyID)
	if err != nil || control == nil || !isBrowserSessionReference(control.SecretReference) {
		return nil, ErrBrowserCSVSchemaApprovalUnauthorized
	}
	return authorization, nil
}

func (s *BrowserCSVSchemaApprovalService) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func validBrowserCSVSchemaApproval(approval BrowserCSVSchemaApproval) bool {
	if !safeBridgeID(approval.TenantID) || !validBrowserDiscoveryID(approval.DiscoveryID) || !validBrowserCSVSchemaResourceID(approval.ResourceID) || !validBrowserCSVSchemaID(approval.SchemaID) || approval.Review.Version != BrowserCSVSchemaReviewVersion || !approval.Review.Confirmed || !validBrowserDiscoveryID(approval.Review.AuditID) || approval.Review.ReviewedAt.IsZero() || strings.TrimSpace(approval.ReviewedBy) == "" || approval.CreatedAt.IsZero() || approval.UpdatedAt.IsZero() {
		return false
	}
	switch approval.Status {
	case BrowserCSVSchemaStatusPending:
		return approval.ApprovalSHA256 == ""
	case BrowserCSVSchemaStatusRegistered:
		return validSHA256(approval.ApprovalSHA256)
	default:
		return false
	}
}

func validBrowserCSVSchemaResourceID(resourceID string) bool {
	coverage, found := browserDiscoveryResourceCoverage(strings.TrimSpace(resourceID))
	return found && coverage == "export_csv"
}

func validBrowserCSVSchemaID(value string) bool {
	if len(value) < 1 || len(value) > 80 {
		return false
	}
	for _, character := range value {
		lowercaseOrDigit := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if !lowercaseOrDigit && character != '_' {
			return false
		}
	}
	return true
}
