package smartaccountssync

import (
	"context"
	"errors"
	"testing"

	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	browserOnboardingOwnerID   = "owner-1"
	browserOnboardingTenantID  = "b436c224-5df5-4b4d-a772-1897f9147400"
	browserOnboardingSourceOne = "sa-browser-v1-123456"
	browserOnboardingSourceTwo = "sa-browser-v1-654321"
)

type browserOnboardingMemoryStore struct {
	browserPairingMemoryStore
	bindings map[string]BrowserOnboardingBinding
	targets  map[string][]BrowserOnboardingBinding
}

func (s *browserOnboardingMemoryStore) GetBrowserOnboarding(_ context.Context, sourceCompanyID string) (*BrowserOnboardingBinding, error) {
	binding, exists := s.bindings[sourceCompanyID]
	if !exists {
		return nil, ErrBrowserOnboardingNotFound
	}
	return &binding, nil
}

func (s *browserOnboardingMemoryStore) CreateBrowserOnboarding(_ context.Context, binding BrowserOnboardingBinding) (*BrowserOnboardingBinding, bool, error) {
	if s.bindings == nil {
		s.bindings = map[string]BrowserOnboardingBinding{}
	}
	if _, exists := s.bindings[binding.SourceCompanyID]; exists {
		return nil, false, nil
	}
	s.bindings[binding.SourceCompanyID] = binding
	return &binding, true, nil
}

func (s *browserOnboardingMemoryStore) SetBrowserOnboardingTarget(_ context.Context, sourceCompanyID, tenantID, tenantName string) (*BrowserOnboardingBinding, error) {
	binding, exists := s.bindings[sourceCompanyID]
	if !exists {
		return nil, ErrBrowserOnboardingNotFound
	}
	if binding.TenantID == "" {
		binding.TenantID = tenantID
		binding.TenantName = tenantName
		binding.Status = BrowserOnboardingTargetReady
		s.bindings[sourceCompanyID] = binding
	}
	return &binding, nil
}

func (s *browserOnboardingMemoryStore) SetBrowserOnboardingPairing(_ context.Context, sourceCompanyID, pairingID string) (*BrowserOnboardingBinding, error) {
	binding, exists := s.bindings[sourceCompanyID]
	if !exists || binding.TenantID == "" {
		return nil, ErrBrowserOnboardingNotFound
	}
	binding.PairingID = pairingID
	binding.Status = BrowserOnboardingPairingIssued
	s.bindings[sourceCompanyID] = binding
	return &binding, nil
}

func (s *browserOnboardingMemoryStore) FindBrowserOnboardingTargets(_ context.Context, sourceCompanyID string) ([]BrowserOnboardingBinding, error) {
	return append([]BrowserOnboardingBinding(nil), s.targets[sourceCompanyID]...), nil
}

type browserOnboardingTenantManager struct {
	memberships []tenant.TenantMembership
	created     []*tenant.Tenant
	createErr   map[string]error
}

func (m *browserOnboardingTenantManager) ListUserTenants(_ context.Context, _ string) ([]tenant.TenantMembership, error) {
	return append([]tenant.TenantMembership(nil), m.memberships...), nil
}

func (m *browserOnboardingTenantManager) CreateTenant(_ context.Context, request *tenant.CreateTenantRequest) (*tenant.Tenant, error) {
	if err := m.createErr[request.Name]; err != nil {
		return nil, err
	}
	created := &tenant.Tenant{ID: uuid.NewString(), Name: request.Name, Slug: request.Slug}
	m.created = append(m.created, created)
	m.memberships = append(m.memberships, tenant.TenantMembership{Tenant: *created, Role: tenant.RoleOwner})
	return created, nil
}

func newBrowserOnboardingServiceForTest(store *browserOnboardingMemoryStore, tenants *browserOnboardingTenantManager) *BrowserOnboardingService {
	syncService := NewService(store, UnavailableBridgeCatalog{})
	pairings := NewBrowserPairingService(store, syncService)
	pairings.newID = uuid.NewString
	pairings.newToken = func() (string, error) { return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq", nil }
	return NewBrowserOnboardingService(store, tenants, pairings)
}

func TestBrowserOnboardingCreatesMissingTargetThenExpectedSourcePairingAndReusesItOnRetry(t *testing.T) {
	store := &browserOnboardingMemoryStore{}
	tenants := &browserOnboardingTenantManager{createErr: map[string]error{}}
	service := newBrowserOnboardingServiceForTest(store, tenants)

	first, err := service.Start(context.Background(), browserOnboardingOwnerID, BrowserOnboardingRequest{
		Sources:                       []BrowserOnboardingSource{{SourceCompanyID: browserOnboardingSourceOne, SourceCompanyName: "Hold My Beer OÜ"}},
		CreateMissingTenantsConfirmed: true,
	})

	require.NoError(t, err)
	require.Len(t, first.Bindings, 1)
	created := first.Bindings[0]
	assert.True(t, created.TenantCreated)
	assert.False(t, created.TenantReused)
	assert.Equal(t, BrowserOnboardingPairingIssued, created.Status)
	require.NotNil(t, created.Pairing)
	assert.Equal(t, browserOnboardingSourceOne, store.pairings[created.Pairing.PairingID].ExpectedSourceCompanyID)
	assert.Len(t, tenants.created, 1)

	second, err := service.Start(context.Background(), browserOnboardingOwnerID, BrowserOnboardingRequest{
		Sources:                       []BrowserOnboardingSource{{SourceCompanyID: browserOnboardingSourceOne, SourceCompanyName: "Hold My Beer OÜ"}},
		CreateMissingTenantsConfirmed: true,
	})
	require.NoError(t, err)
	require.Len(t, second.Bindings, 1)
	assert.True(t, second.Bindings[0].TenantReused == false) // A durable binding is stronger than a name reuse.
	assert.Equal(t, created.TenantID, second.Bindings[0].TenantID)
	assert.Len(t, tenants.created, 1)
	require.NotNil(t, second.Bindings[0].Pairing)
	assert.NotEqual(t, created.Pairing.PairingID, second.Bindings[0].Pairing.PairingID)
}

func TestBrowserOnboardingReusesOneOwnerTenantAndDoesNotRevealOtherOwnerBinding(t *testing.T) {
	ownerTenant := tenant.Tenant{ID: browserOnboardingTenantID, Name: "Existing Company", Slug: "existing-company-123456"}
	store := &browserOnboardingMemoryStore{bindings: map[string]BrowserOnboardingBinding{
		browserOnboardingSourceTwo: {SourceCompanyID: browserOnboardingSourceTwo, SourceCompanyName: "Other Company", TenantID: "other-tenant", TenantName: "Other Company", Status: BrowserOnboardingPaired},
	}}
	tenants := &browserOnboardingTenantManager{memberships: []tenant.TenantMembership{{Tenant: ownerTenant, Role: tenant.RoleOwner}}, createErr: map[string]error{}}
	service := newBrowserOnboardingServiceForTest(store, tenants)

	response, err := service.Start(context.Background(), browserOnboardingOwnerID, BrowserOnboardingRequest{
		Sources: []BrowserOnboardingSource{
			{SourceCompanyID: browserOnboardingSourceOne, SourceCompanyName: "Existing Company"},
			{SourceCompanyID: browserOnboardingSourceTwo, SourceCompanyName: "Other Company"},
		},
		CreateMissingTenantsConfirmed: true,
	})
	require.NoError(t, err)
	require.Len(t, response.Bindings, 2)
	assert.Equal(t, browserOnboardingTenantID, response.Bindings[0].TenantID)
	assert.True(t, response.Bindings[0].TenantReused)
	assert.Equal(t, BrowserOnboardingReview, response.Bindings[1].Status)
	assert.Equal(t, "source_already_bound", response.Bindings[1].ReasonCode)
	assert.Empty(t, response.Bindings[1].TenantID)
	assert.Empty(t, response.Bindings[1].TenantName)
	assert.Empty(t, tenants.created)
}

func TestBrowserOnboardingDoesNotShareOneNameMatchedTenantAcrossSelectedSources(t *testing.T) {
	existing := tenant.Tenant{ID: browserOnboardingTenantID, Name: "Repeated Company", Slug: "repeated-company"}
	store := &browserOnboardingMemoryStore{}
	tenants := &browserOnboardingTenantManager{
		memberships: []tenant.TenantMembership{{Tenant: existing, Role: tenant.RoleOwner}},
		createErr:   map[string]error{},
	}
	service := newBrowserOnboardingServiceForTest(store, tenants)

	response, err := service.Start(context.Background(), browserOnboardingOwnerID, BrowserOnboardingRequest{
		Sources: []BrowserOnboardingSource{
			{SourceCompanyID: browserOnboardingSourceOne, SourceCompanyName: "Repeated Company"},
			{SourceCompanyID: browserOnboardingSourceTwo, SourceCompanyName: "Repeated Company"},
		},
		CreateMissingTenantsConfirmed: true,
	})

	require.NoError(t, err)
	require.Len(t, response.Bindings, 2)
	assert.Equal(t, browserOnboardingTenantID, response.Bindings[0].TenantID)
	assert.True(t, response.Bindings[0].TenantReused)
	assert.True(t, response.Bindings[1].TenantCreated)
	assert.NotEmpty(t, response.Bindings[1].TenantID)
	assert.NotEqual(t, response.Bindings[0].TenantID, response.Bindings[1].TenantID)
	assert.Len(t, tenants.created, 1)
}

func TestBrowserOnboardingIsolatesCreateFailureAndRequiresExplicitOwnerAuthorization(t *testing.T) {
	store := &browserOnboardingMemoryStore{}
	tenants := &browserOnboardingTenantManager{createErr: map[string]error{"Broken Company": errors.New("test create failure")}}
	service := newBrowserOnboardingServiceForTest(store, tenants)

	_, err := service.Start(context.Background(), browserOnboardingOwnerID, BrowserOnboardingRequest{Sources: []BrowserOnboardingSource{{SourceCompanyID: browserOnboardingSourceOne, SourceCompanyName: "Good Company"}}})
	assert.ErrorIs(t, err, ErrBrowserOnboardingInvalid)

	response, err := service.Start(context.Background(), browserOnboardingOwnerID, BrowserOnboardingRequest{
		Sources: []BrowserOnboardingSource{
			{SourceCompanyID: browserOnboardingSourceOne, SourceCompanyName: "Good Company"},
			{SourceCompanyID: browserOnboardingSourceTwo, SourceCompanyName: "Broken Company"},
		},
		CreateMissingTenantsConfirmed: true,
	})
	require.NoError(t, err)
	require.Len(t, response.Bindings, 2)
	assert.Equal(t, BrowserOnboardingPairingIssued, response.Bindings[0].Status)
	assert.Equal(t, BrowserOnboardingFailed, response.Bindings[1].Status)
	assert.Equal(t, "tenant_create_failed", response.Bindings[1].ReasonCode)
	assert.Len(t, tenants.created, 1)
}
