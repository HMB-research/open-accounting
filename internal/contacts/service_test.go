package contacts

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	contacts map[string]*Contact
	CreateFn func(ctx context.Context, schemaName string, contact *Contact) error
	GetFn    func(ctx context.Context, schemaName, tenantID, contactID string) (*Contact, error)
	ListFn   func(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error)
	UpdateFn func(ctx context.Context, schemaName string, contact *Contact) error
	DeleteFn func(ctx context.Context, schemaName, tenantID, contactID string) error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		contacts: make(map[string]*Contact),
	}
}

func (m *MockRepository) Create(ctx context.Context, schemaName string, contact *Contact) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, schemaName, contact)
	}
	m.contacts[contact.ID] = contact
	return nil
}

func (m *MockRepository) GetByID(ctx context.Context, schemaName, tenantID, contactID string) (*Contact, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, schemaName, tenantID, contactID)
	}
	if c, ok := m.contacts[contactID]; ok && c.TenantID == tenantID {
		return c, nil
	}
	return nil, ErrContactNotFound
}

func (m *MockRepository) List(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, schemaName, tenantID, filter)
	}
	var result []Contact
	for _, c := range m.contacts {
		if c.TenantID == tenantID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *MockRepository) Update(ctx context.Context, schemaName string, contact *Contact) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, schemaName, contact)
	}
	m.contacts[contact.ID] = contact
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, schemaName, tenantID, contactID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, schemaName, tenantID, contactID)
	}
	if c, ok := m.contacts[contactID]; ok && c.TenantID == tenantID {
		c.IsActive = false
		return nil
	}
	return ErrContactNotFound
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)
	if service == nil {
		t.Error("NewServiceWithRepository should return a non-nil service")
	}
}

func TestService_Create(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	tests := []struct {
		name     string
		tenantID string
		req      *CreateContactRequest
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "Valid customer",
			tenantID: "tenant-1",
			req: &CreateContactRequest{
				Name:        "Test Customer",
				ContactType: ContactTypeCustomer,
				Email:       "test@example.com",
			},
			wantErr: false,
		},
		{
			name:     "Valid supplier",
			tenantID: "tenant-1",
			req: &CreateContactRequest{
				Name:        "Test Supplier",
				ContactType: ContactTypeSupplier,
			},
			wantErr: false,
		},
		{
			name:     "Missing name",
			tenantID: "tenant-1",
			req: &CreateContactRequest{
				ContactType: ContactTypeCustomer,
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name:     "Missing contact type",
			tenantID: "tenant-1",
			req: &CreateContactRequest{
				Name: "Test",
			},
			wantErr: true,
			errMsg:  "contact type is required",
		},
		{
			name:     "Invalid contact type",
			tenantID: "tenant-1",
			req: &CreateContactRequest{
				Name:        "Test",
				ContactType: ContactType("INVALID"),
			},
			wantErr: true,
			errMsg:  "invalid contact type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contact, err := service.Create(ctx, tt.tenantID, "public", tt.req)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if contact.ID == "" {
				t.Error("Contact ID should not be empty")
			}
			if contact.TenantID != tt.tenantID {
				t.Errorf("TenantID = %q, want %q", contact.TenantID, tt.tenantID)
			}
			if contact.Name != tt.req.Name {
				t.Errorf("Name = %q, want %q", contact.Name, tt.req.Name)
			}
		})
	}
}

func TestService_Create_DefaultValues(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	req := &CreateContactRequest{
		Name:        "Test Contact",
		ContactType: ContactTypeCustomer,
	}

	contact, err := service.Create(ctx, "tenant-1", "public", req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check defaults
	if contact.CountryCode != "EE" {
		t.Errorf("Default CountryCode = %q, want %q", contact.CountryCode, "EE")
	}
	if contact.PaymentTermsDays != 14 {
		t.Errorf("Default PaymentTermsDays = %d, want %d", contact.PaymentTermsDays, 14)
	}
	if !contact.IsActive {
		t.Error("IsActive should be true by default")
	}
}

func TestService_Create_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.CreateFn = func(ctx context.Context, schemaName string, contact *Contact) error {
		return errors.New("database error")
	}
	service := NewServiceWithRepository(repo)

	req := &CreateContactRequest{
		Name:        "Test",
		ContactType: ContactTypeCustomer,
	}

	_, err := service.Create(ctx, "tenant-1", "public", req)
	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestService_GetByID(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	// Create a contact first
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateContactRequest{
		Name:        "Test Contact",
		ContactType: ContactTypeCustomer,
	})

	tests := []struct {
		name      string
		tenantID  string
		contactID string
		wantErr   bool
	}{
		{
			name:      "Existing contact",
			tenantID:  "tenant-1",
			contactID: created.ID,
			wantErr:   false,
		},
		{
			name:      "Non-existing contact",
			tenantID:  "tenant-1",
			contactID: "non-existent",
			wantErr:   true,
		},
		{
			name:      "Wrong tenant",
			tenantID:  "other-tenant",
			contactID: created.ID,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contact, err := service.GetByID(ctx, tt.tenantID, "public", tt.contactID)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if contact.ID != tt.contactID {
				t.Errorf("Contact ID = %q, want %q", contact.ID, tt.contactID)
			}
		})
	}
}

func TestService_List(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	// Create some contacts
	_, _ = service.Create(ctx, "tenant-1", "public", &CreateContactRequest{
		Name:        "Customer 1",
		ContactType: ContactTypeCustomer,
	})
	_, _ = service.Create(ctx, "tenant-1", "public", &CreateContactRequest{
		Name:        "Supplier 1",
		ContactType: ContactTypeSupplier,
	})
	_, _ = service.Create(ctx, "tenant-2", "public", &CreateContactRequest{
		Name:        "Other Tenant",
		ContactType: ContactTypeCustomer,
	})

	contacts, err := service.List(ctx, "tenant-1", "public", nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(contacts) != 2 {
		t.Errorf("Expected 2 contacts, got %d", len(contacts))
	}
}

func TestService_List_WithFilter(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.ListFn = func(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error) {
		contacts := []Contact{
			{ID: "1", TenantID: tenantID, Name: "Customer", ContactType: ContactTypeCustomer, IsActive: true},
			{ID: "2", TenantID: tenantID, Name: "Supplier", ContactType: ContactTypeSupplier, IsActive: true},
		}

		if filter != nil && filter.ContactType != "" {
			var filtered []Contact
			for _, c := range contacts {
				if c.ContactType == filter.ContactType {
					filtered = append(filtered, c)
				}
			}
			return filtered, nil
		}
		return contacts, nil
	}
	service := NewServiceWithRepository(repo)

	// Filter by type
	contacts, err := service.List(ctx, "tenant-1", "public", &ContactFilter{ContactType: ContactTypeCustomer})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(contacts) != 1 {
		t.Errorf("Expected 1 customer, got %d", len(contacts))
	}
}

func TestService_List_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.ListFn = func(ctx context.Context, schemaName, tenantID string, filter *ContactFilter) ([]Contact, error) {
		return nil, errors.New("database error")
	}
	service := NewServiceWithRepository(repo)

	_, err := service.List(ctx, "tenant-1", "public", nil)
	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestService_Update(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	// Create a contact first
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateContactRequest{
		Name:        "Original Name",
		ContactType: ContactTypeCustomer,
		Email:       "original@example.com",
	})

	// Update the contact
	newName := "Updated Name"
	newEmail := "updated@example.com"
	newPhone := "+372 5551234"
	newPaymentTerms := 30
	newCreditLimit := decimal.NewFromFloat(10000.00)

	updated, err := service.Update(ctx, "tenant-1", "public", created.ID, &UpdateContactRequest{
		Name:             &newName,
		Email:            &newEmail,
		Phone:            &newPhone,
		PaymentTermsDays: &newPaymentTerms,
		CreditLimit:      &newCreditLimit,
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if updated.Name != newName {
		t.Errorf("Name = %q, want %q", updated.Name, newName)
	}
	if updated.Email != newEmail {
		t.Errorf("Email = %q, want %q", updated.Email, newEmail)
	}
	if updated.Phone != newPhone {
		t.Errorf("Phone = %q, want %q", updated.Phone, newPhone)
	}
	if updated.PaymentTermsDays != newPaymentTerms {
		t.Errorf("PaymentTermsDays = %d, want %d", updated.PaymentTermsDays, newPaymentTerms)
	}
	if !updated.CreditLimit.Equal(newCreditLimit) {
		t.Errorf("CreditLimit = %s, want %s", updated.CreditLimit, newCreditLimit)
	}
}

func TestService_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	newName := "Updated Name"
	_, err := service.Update(ctx, "tenant-1", "public", "non-existent", &UpdateContactRequest{
		Name: &newName,
	})

	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestService_Update_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	// Create a contact first
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateContactRequest{
		Name:        "Test Contact",
		ContactType: ContactTypeCustomer,
	})

	// Set the update function to return an error
	repo.UpdateFn = func(ctx context.Context, schemaName string, contact *Contact) error {
		return errors.New("database error")
	}

	newName := "Updated Name"
	_, err := service.Update(ctx, "tenant-1", "public", created.ID, &UpdateContactRequest{
		Name: &newName,
	})

	if err == nil {
		t.Error("Expected error but got nil")
	}
	if !contains(err.Error(), "update contact") {
		t.Errorf("Error should contain 'update contact', got %q", err.Error())
	}
}

func TestService_Update_AllFields(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	created, _ := service.Create(ctx, "tenant-1", "public", &CreateContactRequest{
		Name:        "Original",
		ContactType: ContactTypeCustomer,
	})

	// Update all fields
	name := "New Name"
	regCode := "12345678"
	vatNumber := "EE123456789"
	email := "new@example.com"
	phone := "+372 5551234"
	address1 := "Street 1"
	address2 := "Apt 2"
	city := "Tallinn"
	postalCode := "10111"
	countryCode := "EE"
	paymentTerms := 45
	creditLimit := decimal.NewFromFloat(5000.00)
	accountID := "acc-123"
	notes := "Some notes"
	isActive := false

	updated, err := service.Update(ctx, "tenant-1", "public", created.ID, &UpdateContactRequest{
		Name:             &name,
		RegCode:          &regCode,
		VATNumber:        &vatNumber,
		Email:            &email,
		Phone:            &phone,
		AddressLine1:     &address1,
		AddressLine2:     &address2,
		City:             &city,
		PostalCode:       &postalCode,
		CountryCode:      &countryCode,
		PaymentTermsDays: &paymentTerms,
		CreditLimit:      &creditLimit,
		DefaultAccountID: &accountID,
		Notes:            &notes,
		IsActive:         &isActive,
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if updated.RegCode != regCode {
		t.Errorf("RegCode = %q, want %q", updated.RegCode, regCode)
	}
	if updated.VATNumber != vatNumber {
		t.Errorf("VATNumber = %q, want %q", updated.VATNumber, vatNumber)
	}
	if updated.AddressLine1 != address1 {
		t.Errorf("AddressLine1 = %q, want %q", updated.AddressLine1, address1)
	}
	if updated.AddressLine2 != address2 {
		t.Errorf("AddressLine2 = %q, want %q", updated.AddressLine2, address2)
	}
	if updated.City != city {
		t.Errorf("City = %q, want %q", updated.City, city)
	}
	if updated.PostalCode != postalCode {
		t.Errorf("PostalCode = %q, want %q", updated.PostalCode, postalCode)
	}
	if *updated.DefaultAccountID != accountID {
		t.Errorf("DefaultAccountID = %q, want %q", *updated.DefaultAccountID, accountID)
	}
	if updated.Notes != notes {
		t.Errorf("Notes = %q, want %q", updated.Notes, notes)
	}
	if updated.IsActive != isActive {
		t.Errorf("IsActive = %v, want %v", updated.IsActive, isActive)
	}
}

func TestService_Delete(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	// Create a contact first
	created, _ := service.Create(ctx, "tenant-1", "public", &CreateContactRequest{
		Name:        "To Delete",
		ContactType: ContactTypeCustomer,
	})

	// Delete the contact
	err := service.Delete(ctx, "tenant-1", "public", created.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify it's deactivated
	contact, _ := service.GetByID(ctx, "tenant-1", "public", created.ID)
	if contact.IsActive {
		t.Error("Contact should be inactive after delete")
	}
}

func TestService_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	service := NewServiceWithRepository(repo)

	err := service.Delete(ctx, "tenant-1", "public", "non-existent")
	if err == nil {
		t.Error("Expected error but got nil")
	}
}

func TestIsValidContactType(t *testing.T) {
	tests := []struct {
		ct       ContactType
		expected bool
	}{
		{ContactTypeCustomer, true},
		{ContactTypeSupplier, true},
		{ContactTypeBoth, true},
		{ContactType("INVALID"), false},
		{ContactType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.ct), func(t *testing.T) {
			result := isValidContactType(tt.ct)
			if result != tt.expected {
				t.Errorf("isValidContactType(%q) = %v, want %v", tt.ct, result, tt.expected)
			}
		})
	}
}

func TestValidateCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *CreateContactRequest
		wantErr bool
	}{
		{
			name: "Valid request",
			req: &CreateContactRequest{
				Name:        "Test",
				ContactType: ContactTypeCustomer,
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			req: &CreateContactRequest{
				ContactType: ContactTypeCustomer,
			},
			wantErr: true,
		},
		{
			name: "Missing contact type",
			req: &CreateContactRequest{
				Name: "Test",
			},
			wantErr: true,
		},
		{
			name: "Invalid contact type",
			req: &CreateContactRequest{
				Name:        "Test",
				ContactType: ContactType("WRONG"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyUpdates(t *testing.T) {
	contact := &Contact{
		Name:             "Original",
		Email:            "original@test.com",
		PaymentTermsDays: 14,
		IsActive:         true,
	}

	newName := "Updated"
	newEmail := "updated@test.com"
	newTerms := 30
	inactive := false

	applyUpdates(contact, &UpdateContactRequest{
		Name:             &newName,
		Email:            &newEmail,
		PaymentTermsDays: &newTerms,
		IsActive:         &inactive,
	})

	if contact.Name != newName {
		t.Errorf("Name = %q, want %q", contact.Name, newName)
	}
	if contact.Email != newEmail {
		t.Errorf("Email = %q, want %q", contact.Email, newEmail)
	}
	if contact.PaymentTermsDays != newTerms {
		t.Errorf("PaymentTermsDays = %d, want %d", contact.PaymentTermsDays, newTerms)
	}
	if contact.IsActive != inactive {
		t.Errorf("IsActive = %v, want %v", contact.IsActive, inactive)
	}
}

func TestApplyUpdates_NilFields(t *testing.T) {
	original := Contact{
		Name:             "Original",
		Email:            "original@test.com",
		PaymentTermsDays: 14,
	}
	contact := original

	// Apply empty update request - nothing should change
	applyUpdates(&contact, &UpdateContactRequest{})

	if contact.Name != original.Name {
		t.Errorf("Name changed unexpectedly: %q -> %q", original.Name, contact.Name)
	}
	if contact.Email != original.Email {
		t.Errorf("Email changed unexpectedly: %q -> %q", original.Email, contact.Email)
	}
	if contact.PaymentTermsDays != original.PaymentTermsDays {
		t.Errorf("PaymentTermsDays changed unexpectedly: %d -> %d", original.PaymentTermsDays, contact.PaymentTermsDays)
	}
}

func TestContactTypeConstants(t *testing.T) {
	if ContactTypeCustomer != "CUSTOMER" {
		t.Errorf("ContactTypeCustomer = %q, want CUSTOMER", ContactTypeCustomer)
	}
	if ContactTypeSupplier != "SUPPLIER" {
		t.Errorf("ContactTypeSupplier = %q, want SUPPLIER", ContactTypeSupplier)
	}
	if ContactTypeBoth != "BOTH" {
		t.Errorf("ContactTypeBoth = %q, want BOTH", ContactTypeBoth)
	}
}

func TestErrContactNotFound(t *testing.T) {
	if ErrContactNotFound.Error() != "contact not found" {
		t.Errorf("ErrContactNotFound = %q, want 'contact not found'", ErrContactNotFound.Error())
	}
}

func TestContact_Fields(t *testing.T) {
	now := time.Now()
	accountID := "acc-123"

	contact := Contact{
		ID:               "contact-1",
		TenantID:         "tenant-1",
		Code:             "C001",
		Name:             "Test Contact",
		ContactType:      ContactTypeCustomer,
		RegCode:          "12345678",
		VATNumber:        "EE123456789",
		Email:            "test@example.com",
		Phone:            "+372 5551234",
		AddressLine1:     "Street 1",
		AddressLine2:     "Apt 2",
		City:             "Tallinn",
		PostalCode:       "10111",
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.NewFromFloat(10000.00),
		DefaultAccountID: &accountID,
		IsActive:         true,
		Notes:            "Test notes",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if contact.ID != "contact-1" {
		t.Errorf("ID = %q, want contact-1", contact.ID)
	}
	if contact.Name != "Test Contact" {
		t.Errorf("Name = %q, want Test Contact", contact.Name)
	}
	if contact.ContactType != ContactTypeCustomer {
		t.Errorf("ContactType = %q, want CUSTOMER", contact.ContactType)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
