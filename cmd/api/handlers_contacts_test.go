package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// =============================================================================
// Mock Contacts Repository
// =============================================================================

type mockContactsRepository struct {
	contacts map[string]*contacts.Contact // key: contactID

	// Error injection
	createErr error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error
}

func newMockContactsRepository() *mockContactsRepository {
	return &mockContactsRepository{
		contacts: make(map[string]*contacts.Contact),
	}
}

func (m *mockContactsRepository) Create(ctx context.Context, schemaName string, contact *contacts.Contact) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.contacts[contact.ID] = contact
	return nil
}

func (m *mockContactsRepository) GetByID(ctx context.Context, schemaName, tenantID, contactID string) (*contacts.Contact, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	c, ok := m.contacts[contactID]
	if !ok {
		return nil, contacts.ErrContactNotFound
	}
	if c.TenantID != tenantID {
		return nil, contacts.ErrContactNotFound
	}
	return c, nil
}

func (m *mockContactsRepository) List(ctx context.Context, schemaName, tenantID string, filter *contacts.ContactFilter) ([]contacts.Contact, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []contacts.Contact
	for _, c := range m.contacts {
		if c.TenantID != tenantID {
			continue
		}
		if filter != nil {
			if filter.ContactType != "" && c.ContactType != filter.ContactType {
				continue
			}
			if filter.ActiveOnly && !c.IsActive {
				continue
			}
		}
		result = append(result, *c)
	}
	return result, nil
}

func (m *mockContactsRepository) Update(ctx context.Context, schemaName string, contact *contacts.Contact) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.contacts[contact.ID]; !ok {
		return contacts.ErrContactNotFound
	}
	m.contacts[contact.ID] = contact
	return nil
}

func (m *mockContactsRepository) Delete(ctx context.Context, schemaName, tenantID, contactID string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	c, ok := m.contacts[contactID]
	if !ok {
		return contacts.ErrContactNotFound
	}
	if c.TenantID != tenantID {
		return contacts.ErrContactNotFound
	}
	c.IsActive = false
	return nil
}

// Helper to add a test contact
func (m *mockContactsRepository) addTestContact(id, tenantID, name string, contactType contacts.ContactType, isActive bool) *contacts.Contact {
	c := &contacts.Contact{
		ID:               id,
		TenantID:         tenantID,
		Name:             name,
		ContactType:      contactType,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		IsActive:         isActive,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	m.contacts[id] = c
	return c
}

// =============================================================================
// Test Setup Helpers
// =============================================================================

func setupContactsTestHandlers() (*Handlers, *mockTenantRepository, *mockContactsRepository) {
	tenantRepo := newMockTenantRepository()
	contactsRepo := newMockContactsRepository()

	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)
	contactsSvc := contacts.NewServiceWithRepository(contactsRepo)
	tokenSvc := auth.NewTokenService("test-secret-key-for-testing-only", 15*time.Minute, 7*24*time.Hour)

	h := &Handlers{
		tenantService:   tenantSvc,
		contactsService: contactsSvc,
		tokenService:    tokenSvc,
	}

	return h, tenantRepo, contactsRepo
}

// =============================================================================
// ListContacts Handler Tests
// =============================================================================

func TestListContacts(t *testing.T) {
	tests := []struct {
		name          string
		tenantID      string
		queryParams   map[string]string
		claims        *auth.Claims
		setupMock     func(*mockTenantRepository, *mockContactsRepository)
		wantStatus    int
		checkResponse func(*testing.T, []map[string]interface{})
	}{
		{
			name:     "list all contacts",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "Customer One", contacts.ContactTypeCustomer, true)
				cr.addTestContact("contact-2", "tenant-1", "Supplier One", contacts.ContactTypeSupplier, true)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 2)
			},
		},
		{
			name:     "filter by type - customers only",
			tenantID: "tenant-1",
			queryParams: map[string]string{
				"type": "CUSTOMER",
			},
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "Customer One", contacts.ContactTypeCustomer, true)
				cr.addTestContact("contact-2", "tenant-1", "Supplier One", contacts.ContactTypeSupplier, true)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 1)
				assert.Equal(t, "Customer One", resp[0]["name"])
			},
		},
		{
			name:     "filter active only",
			tenantID: "tenant-1",
			queryParams: map[string]string{
				"active_only": "true",
			},
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "Active", contacts.ContactTypeCustomer, true)
				cr.addTestContact("contact-2", "tenant-1", "Inactive", contacts.ContactTypeCustomer, false)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Len(t, resp, 1)
				assert.Equal(t, "Active", resp[0]["name"])
			},
		},
		{
			name:     "empty list",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp []map[string]interface{}) {
				assert.Empty(t, resp)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, contactsRepo := setupContactsTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, contactsRepo)
			}

			path := "/tenants/" + tt.tenantID + "/contacts"
			if len(tt.queryParams) > 0 {
				path += "?"
				for k, v := range tt.queryParams {
					path += k + "=" + v + "&"
				}
			}

			req := makeAuthenticatedRequest(http.MethodGet, path, nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.ListContacts(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.checkResponse != nil {
				var resp []map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

// =============================================================================
// CreateContact Handler Tests
// =============================================================================

func TestCreateContact(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository, *mockContactsRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:     "create customer",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"name":         "New Customer",
				"contact_type": "CUSTOMER",
				"email":        "customer@example.com",
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.NotEmpty(t, resp["id"])
				assert.Equal(t, "New Customer", resp["name"])
				assert.Equal(t, "CUSTOMER", resp["contact_type"])
			},
		},
		{
			name:     "create supplier",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"name":         "New Supplier",
				"contact_type": "SUPPLIER",
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "SUPPLIER", resp["contact_type"])
			},
		},
		{
			name:     "default to customer type",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"name": "Customer Without Type",
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "CUSTOMER", resp["contact_type"])
			},
		},
		{
			name:     "missing name",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"contact_type": "CUSTOMER",
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "Name is required",
		},
		{
			name:     "with full details",
			tenantID: "tenant-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"name":               "Full Contact",
				"contact_type":       "CUSTOMER",
				"email":              "full@example.com",
				"phone":              "+1234567890",
				"address_line_1":     "123 Main St",
				"city":               "Test City",
				"postal_code":        "12345",
				"country_code":       "US",
				"payment_terms_days": 30,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus: http.StatusCreated,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "Full Contact", resp["name"])
				assert.Equal(t, "full@example.com", resp["email"])
				assert.Equal(t, "US", resp["country_code"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, contactsRepo := setupContactsTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, contactsRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPost, "/tenants/"+tt.tenantID+"/contacts", tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID})
			w := httptest.NewRecorder()

			h.CreateContact(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

func TestCreateContactInvalidJSON(t *testing.T) {
	h, tenantRepo, _ := setupContactsTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}
	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/contacts", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.CreateContact(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// GetContact Handler Tests
// =============================================================================

func TestGetContact(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		contactID      string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository, *mockContactsRepository)
		wantStatus     int
		wantErrContain string
		checkResponse  func(*testing.T, map[string]interface{})
	}{
		{
			name:      "get existing contact",
			tenantID:  "tenant-1",
			contactID: "contact-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "Test Customer", contacts.ContactTypeCustomer, true)
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, "contact-1", resp["id"])
				assert.Equal(t, "Test Customer", resp["name"])
			},
		},
		{
			name:      "contact not found",
			tenantID:  "tenant-1",
			contactID: "nonexistent",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusNotFound,
			wantErrContain: "not found",
		},
		{
			name:      "contact from different tenant",
			tenantID:  "tenant-1",
			contactID: "contact-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				tr.addTestTenant("tenant-2", "Other Tenant", "other-tenant")
				cr.addTestContact("contact-1", "tenant-2", "Other Contact", contacts.ContactTypeCustomer, true)
			},
			wantStatus:     http.StatusNotFound,
			wantErrContain: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, contactsRepo := setupContactsTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, contactsRepo)
			}

			req := makeAuthenticatedRequest(http.MethodGet, "/tenants/"+tt.tenantID+"/contacts/"+tt.contactID, nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID, "contactID": tt.contactID})
			w := httptest.NewRecorder()

			h.GetContact(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}

			if tt.checkResponse != nil {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.checkResponse(t, resp)
			}
		})
	}
}

// =============================================================================
// UpdateContact Handler Tests
// =============================================================================

func TestUpdateContact(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		contactID      string
		claims         *auth.Claims
		body           map[string]interface{}
		setupMock      func(*mockTenantRepository, *mockContactsRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:      "update name",
			tenantID:  "tenant-1",
			contactID: "contact-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"name": "Updated Name",
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "Original Name", contacts.ContactTypeCustomer, true)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "update multiple fields",
			tenantID:  "tenant-1",
			contactID: "contact-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"name":               "Updated Name",
				"email":              "updated@example.com",
				"payment_terms_days": 45,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "Original Name", contacts.ContactTypeCustomer, true)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "contact not found",
			tenantID:  "tenant-1",
			contactID: "nonexistent",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			body: map[string]interface{}{
				"name": "New Name",
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, contactsRepo := setupContactsTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, contactsRepo)
			}

			req := makeAuthenticatedRequest(http.MethodPut, "/tenants/"+tt.tenantID+"/contacts/"+tt.contactID, tt.body, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID, "contactID": tt.contactID})
			w := httptest.NewRecorder()

			h.UpdateContact(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}
		})
	}
}

func TestUpdateContactInvalidJSON(t *testing.T) {
	h, tenantRepo, contactsRepo := setupContactsTestHandlers()
	tenantRepo.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
	contactsRepo.addTestContact("contact-1", "tenant-1", "Test", contacts.ContactTypeCustomer, true)

	claims := &auth.Claims{UserID: "user-1", TenantID: "tenant-1", Role: tenant.RoleOwner}
	req := makeAuthenticatedRequest(http.MethodPut, "/tenants/tenant-1/contacts/contact-1", nil, claims)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1", "contactID": "contact-1"})
	req.Body = http.NoBody

	w := httptest.NewRecorder()
	h.UpdateContact(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// =============================================================================
// DeleteContact Handler Tests
// =============================================================================

func TestDeleteContact(t *testing.T) {
	tests := []struct {
		name           string
		tenantID       string
		contactID      string
		claims         *auth.Claims
		setupMock      func(*mockTenantRepository, *mockContactsRepository)
		wantStatus     int
		wantErrContain string
	}{
		{
			name:      "delete existing contact",
			tenantID:  "tenant-1",
			contactID: "contact-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "To Delete", contacts.ContactTypeCustomer, true)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:      "delete nonexistent contact",
			tenantID:  "tenant-1",
			contactID: "nonexistent",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
			},
			wantStatus:     http.StatusBadRequest,
			wantErrContain: "not found",
		},
		{
			name:      "delete already deleted contact",
			tenantID:  "tenant-1",
			contactID: "contact-1",
			claims: &auth.Claims{
				UserID:   "user-1",
				TenantID: "tenant-1",
				Role:     tenant.RoleOwner,
			},
			setupMock: func(tr *mockTenantRepository, cr *mockContactsRepository) {
				tr.addTestTenant("tenant-1", "Test Tenant", "test-tenant")
				cr.addTestContact("contact-1", "tenant-1", "Already Deleted", contacts.ContactTypeCustomer, false)
			},
			wantStatus: http.StatusOK, // Soft delete is idempotent
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, tenantRepo, contactsRepo := setupContactsTestHandlers()

			if tt.setupMock != nil {
				tt.setupMock(tenantRepo, contactsRepo)
			}

			req := makeAuthenticatedRequest(http.MethodDelete, "/tenants/"+tt.tenantID+"/contacts/"+tt.contactID, nil, tt.claims)
			req = withURLParams(req, map[string]string{"tenantID": tt.tenantID, "contactID": tt.contactID})
			w := httptest.NewRecorder()

			h.DeleteContact(w, req)

			assert.Equal(t, tt.wantStatus, w.Code, "response body: %s", w.Body.String())

			if tt.wantErrContain != "" {
				var resp map[string]string
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp["error"], tt.wantErrContain)
			}
		})
	}
}
