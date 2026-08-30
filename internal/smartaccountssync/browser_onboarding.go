package smartaccountssync

import (
	"context"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/HMB-research/open-accounting/internal/tenant"
)

const (
	// BrowserOnboardingMaxSources is shared by the direct selected-company
	// runner and the immutable selected/all batch manifest. Keeping one bound
	// prevents a persisted all-company batch from being rejected later by the
	// runner solely because it contains the 26th source.
	BrowserOnboardingMaxSources = 250

	BrowserOnboardingTargetReady   = "TARGET_READY"
	BrowserOnboardingPairingIssued = "PAIRING_ISSUED"
	BrowserOnboardingPaired        = "PAIRED"
	BrowserOnboardingReview        = "REVIEW_REQUIRED"
	BrowserOnboardingFailed        = "FAILED"
)

var (
	ErrBrowserOnboardingUnavailable = errors.New("SmartAccounts browser onboarding is unavailable")
	ErrBrowserOnboardingInvalid     = errors.New("SmartAccounts browser onboarding request is invalid")
	ErrBrowserOnboardingNotFound    = errors.New("SmartAccounts browser onboarding binding was not found")
)

// BrowserOnboardingSource is metadata selected from the locally installed
// relay's visible company picker. It is not a source record. The opaque ID is
// subsequently verified by an expected-source browser pairing before it can
// configure the tenant/source control.
type BrowserOnboardingSource struct {
	SourceCompanyID   string `json:"source_company_id"`
	SourceCompanyName string `json:"source_company_name"`
}

// BrowserOnboardingRequest makes tenant creation an explicit owner action. It
// never accepts a target tenant ID, credential, browser token, source record,
// or financial apply instruction.
type BrowserOnboardingRequest struct {
	Sources                       []BrowserOnboardingSource `json:"sources"`
	CreateMissingTenantsConfirmed bool                      `json:"create_missing_tenants_confirmed"`
}

// BrowserOnboardingBinding is durable, safe progress for exactly one opaque
// SmartAccounts source. Pairing is populated only on the action response and
// contains a raw token only long enough for the page to hand it to the relay.
type BrowserOnboardingBinding struct {
	SourceCompanyID   string `json:"source_company_id"`
	SourceCompanyName string `json:"source_company_name"`
	TenantID          string `json:"tenant_id,omitempty"`
	TenantName        string `json:"tenant_name,omitempty"`
	PairingID         string `json:"pairing_id,omitempty"`
	Status            string `json:"status"`
	CreatedBy         string `json:"-"`
}

type BrowserOnboardingResult struct {
	BrowserOnboardingBinding
	TenantCreated bool                 `json:"tenant_created"`
	TenantReused  bool                 `json:"tenant_reused"`
	ReasonCode    string               `json:"reason_code,omitempty"`
	Pairing       *BrowserPairingIssue `json:"pairing,omitempty"`
}

type BrowserOnboardingResponse struct {
	Bindings []BrowserOnboardingResult `json:"bindings"`
}

// BrowserOnboardingStore owns only non-financial source-to-tenant onboarding
// state. A source is globally reserved before a tenant can be created, which
// prevents a retry or a second owner from creating a second target for it.
type BrowserOnboardingStore interface {
	GetBrowserOnboarding(ctx context.Context, sourceCompanyID string) (*BrowserOnboardingBinding, error)
	CreateBrowserOnboarding(ctx context.Context, binding BrowserOnboardingBinding) (*BrowserOnboardingBinding, bool, error)
	SetBrowserOnboardingTarget(ctx context.Context, sourceCompanyID, tenantID, tenantName string) (*BrowserOnboardingBinding, error)
	SetBrowserOnboardingPairing(ctx context.Context, sourceCompanyID, pairingID string) (*BrowserOnboardingBinding, error)
	FindBrowserOnboardingTargets(ctx context.Context, sourceCompanyID string) ([]BrowserOnboardingBinding, error)
}

type browserOnboardingTenantService interface {
	ListUserTenants(ctx context.Context, userID string) ([]tenant.TenantMembership, error)
	CreateTenant(ctx context.Context, req *tenant.CreateTenantRequest) (*tenant.Tenant, error)
}

// BrowserOnboardingService owns automatic selected-company target creation.
// It has no accounting service and no package/executor dependency. A future
// financial action remains exclusively behind the existing reviewed apply.
type BrowserOnboardingService struct {
	store    BrowserOnboardingStore
	tenants  browserOnboardingTenantService
	pairings *BrowserPairingService
}

func NewBrowserOnboardingService(store BrowserOnboardingStore, tenants browserOnboardingTenantService, pairings *BrowserPairingService) *BrowserOnboardingService {
	return &BrowserOnboardingService{store: store, tenants: tenants, pairings: pairings}
}

func (s *BrowserOnboardingService) Start(ctx context.Context, actorID string, request BrowserOnboardingRequest) (*BrowserOnboardingResponse, error) {
	if s == nil || s.store == nil || s.tenants == nil || s.pairings == nil || !request.CreateMissingTenantsConfirmed {
		return nil, ErrBrowserOnboardingInvalid
	}
	sources, ok := canonicalBrowserOnboardingSources(request.Sources)
	if !ok {
		return nil, ErrBrowserOnboardingInvalid
	}
	memberships, err := s.tenants.ListUserTenants(ctx, strings.TrimSpace(actorID))
	if err != nil {
		return nil, ErrBrowserOnboardingUnavailable
	}
	owners := make(map[string]tenant.Tenant)
	for _, membership := range memberships {
		if membership.Role == tenant.RoleOwner && membership.Tenant.ID != "" {
			owners[membership.Tenant.ID] = membership.Tenant
		}
	}
	results := make([]BrowserOnboardingResult, 0, len(sources))
	// A selected source must not silently share an OA target with another
	// selected source just because their visible company names are equal.
	// Existing durable bindings are checked too, so a retry cannot reintroduce
	// a cross-source target while the operator is onboarding a batch.
	reservedTargetIDs := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		result := s.startOne(ctx, strings.TrimSpace(actorID), owners, reservedTargetIDs, source)
		if result.TenantID != "" && result.Status != BrowserOnboardingReview && result.Status != BrowserOnboardingFailed {
			reservedTargetIDs[result.TenantID] = struct{}{}
		}
		results = append(results, result)
	}
	return &BrowserOnboardingResponse{Bindings: results}, nil
}

func (s *BrowserOnboardingService) Status(ctx context.Context, actorID, sourceCompanyID string) (*BrowserOnboardingResult, error) {
	if s == nil || s.store == nil || s.tenants == nil || s.pairings == nil || !validBrowserSourceCompanyID(sourceCompanyID) {
		return nil, ErrBrowserOnboardingUnavailable
	}
	memberships, err := s.tenants.ListUserTenants(ctx, strings.TrimSpace(actorID))
	if err != nil {
		return nil, ErrBrowserOnboardingUnavailable
	}
	owners := make(map[string]tenant.Tenant)
	for _, membership := range memberships {
		if membership.Role == tenant.RoleOwner {
			owners[membership.Tenant.ID] = membership.Tenant
		}
	}
	binding, err := s.store.GetBrowserOnboarding(ctx, strings.TrimSpace(sourceCompanyID))
	if errors.Is(err, ErrBrowserOnboardingNotFound) {
		return nil, ErrBrowserOnboardingNotFound
	}
	if err != nil || binding == nil || owners[binding.TenantID].ID == "" {
		// Do not reveal a different owner's tenant or even its name.
		return nil, ErrBrowserOnboardingNotFound
	}
	result := BrowserOnboardingResult{BrowserOnboardingBinding: *binding}
	return s.refreshPairing(ctx, result), nil
}

func (s *BrowserOnboardingService) startOne(ctx context.Context, actorID string, owners map[string]tenant.Tenant, reservedTargetIDs map[string]struct{}, source BrowserOnboardingSource) BrowserOnboardingResult {
	binding, err := s.store.GetBrowserOnboarding(ctx, source.SourceCompanyID)
	if errors.Is(err, ErrBrowserOnboardingNotFound) {
		binding, err = s.createOrReuseControlTarget(ctx, actorID, source)
	}
	if err != nil || binding == nil {
		return BrowserOnboardingResult{BrowserOnboardingBinding: BrowserOnboardingBinding{SourceCompanyID: source.SourceCompanyID, SourceCompanyName: source.SourceCompanyName, Status: BrowserOnboardingFailed}, ReasonCode: "onboarding_persistence_unavailable"}
	}
	if binding.TenantID != "" && owners[binding.TenantID].ID == "" {
		return BrowserOnboardingResult{BrowserOnboardingBinding: BrowserOnboardingBinding{SourceCompanyID: source.SourceCompanyID, SourceCompanyName: source.SourceCompanyName, Status: BrowserOnboardingReview}, ReasonCode: "source_already_bound"}
	}
	if binding.TenantID != "" {
		if _, reserved := reservedTargetIDs[binding.TenantID]; reserved {
			return BrowserOnboardingResult{BrowserOnboardingBinding: BrowserOnboardingBinding{SourceCompanyID: source.SourceCompanyID, SourceCompanyName: source.SourceCompanyName, Status: BrowserOnboardingReview}, ReasonCode: "target_already_selected"}
		}
	}
	if binding.Status == BrowserOnboardingReview {
		// The source is already associated with more than one browser control.
		// Do not use a name match to turn that ambiguity into a new target.
		return BrowserOnboardingResult{BrowserOnboardingBinding: *binding, ReasonCode: "ambiguous_source_binding"}
	}
	result := BrowserOnboardingResult{BrowserOnboardingBinding: *binding}
	if result.TenantID == "" {
		return s.chooseTarget(ctx, actorID, owners, reservedTargetIDs, source, result)
	}
	return s.issueOrRefreshPairing(ctx, actorID, result)
}

func (s *BrowserOnboardingService) createOrReuseControlTarget(ctx context.Context, actorID string, source BrowserOnboardingSource) (*BrowserOnboardingBinding, error) {
	// A manually paired existing browser control is already a verified
	// source→tenant binding. Reuse it rather than creating a name-matched
	// tenant or asking the relay to pair a second time.
	targets, err := s.store.FindBrowserOnboardingTargets(ctx, source.SourceCompanyID)
	if err != nil {
		return nil, err
	}
	binding := BrowserOnboardingBinding{SourceCompanyID: source.SourceCompanyID, SourceCompanyName: source.SourceCompanyName, CreatedBy: actorID, Status: BrowserOnboardingTargetReady}
	if len(targets) == 1 {
		binding.TenantID = targets[0].TenantID
		binding.TenantName = targets[0].TenantName
		binding.Status = BrowserOnboardingPaired
	}
	if len(targets) > 1 {
		binding.Status = BrowserOnboardingReview
	}
	created, wasCreated, err := s.store.CreateBrowserOnboarding(ctx, binding)
	if err != nil {
		return nil, err
	}
	if !wasCreated {
		return s.store.GetBrowserOnboarding(ctx, source.SourceCompanyID)
	}
	return created, nil
}

func (s *BrowserOnboardingService) chooseTarget(ctx context.Context, actorID string, owners map[string]tenant.Tenant, reservedTargetIDs map[string]struct{}, source BrowserOnboardingSource, result BrowserOnboardingResult) BrowserOnboardingResult {
	var exactMatches []tenant.Tenant
	wantName := normalizedBrowserTenantName(source.SourceCompanyName)
	for _, candidate := range owners {
		if _, reserved := reservedTargetIDs[candidate.ID]; reserved {
			continue
		}
		if normalizedBrowserTenantName(candidate.Name) == wantName {
			exactMatches = append(exactMatches, candidate)
		}
	}
	if len(exactMatches) > 1 {
		result.Status = BrowserOnboardingReview
		result.ReasonCode = "ambiguous_existing_tenant_name"
		return result
	}
	var target *tenant.Tenant
	if len(exactMatches) == 1 {
		copy := exactMatches[0]
		target = &copy
		result.TenantReused = true
	} else {
		slug := browserOnboardingTenantSlug(source)
		for _, candidate := range owners {
			if _, reserved := reservedTargetIDs[candidate.ID]; reserved {
				continue
			}
			if candidate.Slug == slug {
				copy := candidate
				target = &copy
				result.TenantReused = true
				break
			}
		}
		if target == nil {
			created, err := s.tenants.CreateTenant(ctx, &tenant.CreateTenantRequest{Name: source.SourceCompanyName, Slug: slug, OwnerID: actorID})
			if err != nil || created == nil {
				result.Status = BrowserOnboardingFailed
				result.ReasonCode = "tenant_create_failed"
				return result
			}
			target = created
			result.TenantCreated = true
		}
	}
	updated, err := s.store.SetBrowserOnboardingTarget(ctx, source.SourceCompanyID, target.ID, target.Name)
	if err != nil || updated == nil {
		// The tenant may exist after this retriable persistence failure. Its
		// deterministic source-derived slug lets a later request reuse it.
		result.Status = BrowserOnboardingFailed
		result.ReasonCode = "target_binding_persistence_failed"
		return result
	}
	result.BrowserOnboardingBinding = *updated
	return s.issueOrRefreshPairing(ctx, actorID, result)
}

func (s *BrowserOnboardingService) issueOrRefreshPairing(ctx context.Context, actorID string, result BrowserOnboardingResult) BrowserOnboardingResult {
	if result.Status == BrowserOnboardingPaired {
		return result
	}
	if result.PairingID != "" {
		if status, err := s.pairings.Status(ctx, result.TenantID, result.PairingID); err == nil && status != nil && status.Status == BrowserPairingStatusClaimed {
			result.Status = BrowserOnboardingPaired
			return result
		}
	}
	issue, err := s.pairings.IssueForExpectedSource(ctx, result.TenantID, actorID, result.SourceCompanyID)
	if err != nil || issue == nil {
		result.Status = BrowserOnboardingFailed
		result.ReasonCode = "pairing_issue_failed"
		return result
	}
	updated, err := s.store.SetBrowserOnboardingPairing(ctx, result.SourceCompanyID, issue.PairingID)
	if err != nil || updated == nil {
		result.Status = BrowserOnboardingFailed
		result.ReasonCode = "pairing_persistence_failed"
		return result
	}
	result.BrowserOnboardingBinding = *updated
	result.Pairing = issue
	return result
}

func (s *BrowserOnboardingService) refreshPairing(ctx context.Context, result BrowserOnboardingResult) *BrowserOnboardingResult {
	if result.PairingID == "" || result.TenantID == "" || result.Status == BrowserOnboardingPaired {
		return &result
	}
	if status, err := s.pairings.Status(ctx, result.TenantID, result.PairingID); err == nil && status != nil && status.Status == BrowserPairingStatusClaimed {
		result.Status = BrowserOnboardingPaired
	}
	return &result
}

func canonicalBrowserOnboardingSources(input []BrowserOnboardingSource) ([]BrowserOnboardingSource, bool) {
	if len(input) == 0 || len(input) > BrowserOnboardingMaxSources {
		return nil, false
	}
	seen := make(map[string]struct{}, len(input))
	output := make([]BrowserOnboardingSource, 0, len(input))
	for _, source := range input {
		id := strings.TrimSpace(source.SourceCompanyID)
		name := strings.Join(strings.Fields(strings.TrimSpace(source.SourceCompanyName)), " ")
		if !validBrowserSourceCompanyID(id) || len(name) == 0 || len(name) > 120 {
			return nil, false
		}
		if _, exists := seen[id]; exists {
			return nil, false
		}
		seen[id] = struct{}{}
		output = append(output, BrowserOnboardingSource{SourceCompanyID: id, SourceCompanyName: name})
	}
	sort.Slice(output, func(i, j int) bool { return output[i].SourceCompanyID < output[j].SourceCompanyID })
	return output, true
}

func normalizedBrowserTenantName(value string) string {
	var out strings.Builder
	space := false
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			out.WriteRune(character)
			space = false
		} else if !space {
			out.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(out.String())
}

func browserOnboardingTenantSlug(source BrowserOnboardingSource) string {
	base := normalizedBrowserTenantName(source.SourceCompanyName)
	var slug strings.Builder
	for _, character := range base {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			slug.WriteRune(character)
		} else if slug.Len() > 0 && !strings.HasSuffix(slug.String(), "-") {
			slug.WriteByte('-')
		}
	}
	name := strings.Trim(slug.String(), "-")
	if name == "" {
		name = "smartaccounts-company"
	}
	suffix := strings.TrimPrefix(source.SourceCompanyID, "sa-browser-v1-")
	maxBase := 50 - len(suffix) - 1
	if maxBase < 3 {
		maxBase = 3
	}
	if len(name) > maxBase {
		name = strings.Trim(name[:maxBase], "-")
	}
	return name + "-" + suffix
}
