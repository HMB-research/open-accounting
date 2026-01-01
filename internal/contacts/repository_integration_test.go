//go:build integration

package contacts

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestNewPostgresRepository(t *testing.T) {
	pool := testutil.SetupTestDB(t)

	repo := NewPostgresRepository(pool)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
	if repo.db != pool {
		t.Error("expected repository to have the correct pool")
	}
}

func TestRepository_Create(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "C001-pgx",
		Name:             "Test Customer",
		ContactType:      ContactTypeCustomer,
		RegCode:          "12345678",
		VATNumber:        "EE123456789",
		Email:            "test@example.com",
		Phone:            "+372 555 1234",
		AddressLine1:     "Test Street 1",
		City:             "Tallinn",
		PostalCode:       "10111",
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.NewFromFloat(1000.00),
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := repo.Create(ctx, tenant.SchemaName, contact)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify the contact was created
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != contact.Name {
		t.Errorf("expected name %s, got %s", contact.Name, retrieved.Name)
	}
	if retrieved.ContactType != contact.ContactType {
		t.Errorf("expected contact type %s, got %s", contact.ContactType, retrieved.ContactType)
	}
	if !retrieved.CreditLimit.Equal(contact.CreditLimit) {
		t.Errorf("expected credit limit %s, got %s", contact.CreditLimit, retrieved.CreditLimit)
	}
}

func TestRepository_Create_AllFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "C002-all",
		Name:             "Full Contact",
		ContactType:      ContactTypeSupplier,
		RegCode:          "REG123",
		VATNumber:        "VAT456",
		Email:            "full@example.com",
		Phone:            "+1234567890",
		AddressLine1:     "123 Main St",
		AddressLine2:     "Suite 100",
		City:             "New York",
		PostalCode:       "10001",
		CountryCode:      "US",
		PaymentTermsDays: 30,
		CreditLimit:      decimal.NewFromFloat(50000.00),
		DefaultAccountID: nil, // FK constraint requires valid account - keep nil
		IsActive:         true,
		Notes:            "This is a test note",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := repo.Create(ctx, tenant.SchemaName, contact)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Verify all fields
	if retrieved.Code != contact.Code {
		t.Errorf("expected code %s, got %s", contact.Code, retrieved.Code)
	}
	if retrieved.RegCode != contact.RegCode {
		t.Errorf("expected reg_code %s, got %s", contact.RegCode, retrieved.RegCode)
	}
	if retrieved.VATNumber != contact.VATNumber {
		t.Errorf("expected vat_number %s, got %s", contact.VATNumber, retrieved.VATNumber)
	}
	if retrieved.Email != contact.Email {
		t.Errorf("expected email %s, got %s", contact.Email, retrieved.Email)
	}
	if retrieved.Phone != contact.Phone {
		t.Errorf("expected phone %s, got %s", contact.Phone, retrieved.Phone)
	}
	if retrieved.AddressLine1 != contact.AddressLine1 {
		t.Errorf("expected address_line1 %s, got %s", contact.AddressLine1, retrieved.AddressLine1)
	}
	if retrieved.AddressLine2 != contact.AddressLine2 {
		t.Errorf("expected address_line2 %s, got %s", contact.AddressLine2, retrieved.AddressLine2)
	}
	if retrieved.City != contact.City {
		t.Errorf("expected city %s, got %s", contact.City, retrieved.City)
	}
	if retrieved.PostalCode != contact.PostalCode {
		t.Errorf("expected postal_code %s, got %s", contact.PostalCode, retrieved.PostalCode)
	}
	if retrieved.CountryCode != contact.CountryCode {
		t.Errorf("expected country_code %s, got %s", contact.CountryCode, retrieved.CountryCode)
	}
	if retrieved.PaymentTermsDays != contact.PaymentTermsDays {
		t.Errorf("expected payment_terms_days %d, got %d", contact.PaymentTermsDays, retrieved.PaymentTermsDays)
	}
	if retrieved.DefaultAccountID != nil {
		t.Errorf("expected nil default_account_id, got %v", retrieved.DefaultAccountID)
	}
	if retrieved.Notes != contact.Notes {
		t.Errorf("expected notes %s, got %s", contact.Notes, retrieved.Notes)
	}
}

func TestRepository_Create_DuplicateID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contactID := uuid.New().String()
	contact := &Contact{
		ID:               contactID,
		TenantID:         tenant.ID,
		Code:             "C-DUP-1",
		Name:             "First Contact",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err := repo.Create(ctx, tenant.SchemaName, contact)
	if err != nil {
		t.Fatalf("First create failed: %v", err)
	}

	// Try to create another contact with same ID
	contact2 := &Contact{
		ID:               contactID, // Same ID
		TenantID:         tenant.ID,
		Code:             "C-DUP-2",
		Name:             "Second Contact",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	err = repo.Create(ctx, tenant.SchemaName, contact2)
	if err == nil {
		t.Error("expected error when creating duplicate ID, got nil")
	}
}

func TestRepository_GetByID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "GET-001",
		Name:             "Get Test",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ID != contact.ID {
		t.Errorf("expected ID %s, got %s", contact.ID, retrieved.ID)
	}
	if retrieved.TenantID != contact.TenantID {
		t.Errorf("expected TenantID %s, got %s", contact.TenantID, retrieved.TenantID)
	}
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrContactNotFound {
		t.Errorf("expected ErrContactNotFound, got %v", err)
	}
}

func TestRepository_GetByID_WrongTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "WRONG-T",
		Name:             "Wrong Tenant Test",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Try to get with a different tenant ID
	wrongTenantID := uuid.New().String()
	_, err := repo.GetByID(ctx, tenant.SchemaName, wrongTenantID, contact.ID)
	if err != ErrContactNotFound {
		t.Errorf("expected ErrContactNotFound for wrong tenant, got %v", err)
	}
}

func TestRepository_List(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test contacts
	for i := 1; i <= 3; i++ {
		contact := &Contact{
			ID:               uuid.New().String(),
			TenantID:         tenant.ID,
			Code:             "C00" + string(rune('0'+i)) + "-pgx",
			Name:             "Customer " + string(rune('A'+i-1)),
			ContactType:      ContactTypeCustomer,
			CountryCode:      "EE",
			PaymentTermsDays: 14,
			CreditLimit:      decimal.Zero,
			IsActive:         true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
			t.Fatalf("Create contact %d failed: %v", i, err)
		}
	}

	// List all contacts
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(contacts) < 3 {
		t.Errorf("expected at least 3 contacts, got %d", len(contacts))
	}

	// Test with filter
	filter := &ContactFilter{
		ContactType: ContactTypeCustomer,
		ActiveOnly:  true,
	}
	filteredContacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}

	if len(filteredContacts) < 3 {
		t.Errorf("expected at least 3 filtered contacts, got %d", len(filteredContacts))
	}
}

func TestRepository_List_EmptyResult(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// List with no contacts created
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if contacts != nil && len(contacts) > 0 {
		t.Errorf("expected empty list, got %d contacts", len(contacts))
	}
}

func TestRepository_List_OrderByName(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contacts in reverse alphabetical order
	names := []string{"Zeta Company", "Alpha Corp", "Beta Inc"}
	for i, name := range names {
		contact := &Contact{
			ID:               uuid.New().String(),
			TenantID:         tenant.ID,
			Code:             "ORD-" + string(rune('0'+i)),
			Name:             name,
			ContactType:      ContactTypeCustomer,
			CountryCode:      "EE",
			PaymentTermsDays: 14,
			CreditLimit:      decimal.Zero,
			IsActive:         true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Verify order: Alpha Corp, Beta Inc, Zeta Company
	expectedOrder := []string{"Alpha Corp", "Beta Inc", "Zeta Company"}
	for i, expected := range expectedOrder {
		if i >= len(contacts) {
			t.Errorf("expected at least %d contacts", i+1)
			break
		}
		if contacts[i].Name != expected {
			t.Errorf("expected contact %d to be %s, got %s", i, expected, contacts[i].Name)
		}
	}
}

func TestRepository_Update(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "C001-upd",
		Name:             "Original Name",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update the contact
	contact.Name = "Updated Name"
	contact.CreditLimit = decimal.NewFromFloat(5000.00)

	if err := repo.Update(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify the update
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", retrieved.Name)
	}
	if !retrieved.CreditLimit.Equal(decimal.NewFromFloat(5000.00)) {
		t.Errorf("expected credit limit 5000.00, got %s", retrieved.CreditLimit)
	}
}

func TestRepository_Update_AllFields(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "UPD-ALL-1",
		Name:             "Original",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update all updatable fields (except DefaultAccountID which has FK constraint)
	contact.Name = "Fully Updated"
	contact.RegCode = "NEW-REG"
	contact.VATNumber = "NEW-VAT"
	contact.Email = "new@example.com"
	contact.Phone = "+999"
	contact.AddressLine1 = "New Address 1"
	contact.AddressLine2 = "New Address 2"
	contact.City = "New City"
	contact.PostalCode = "99999"
	contact.CountryCode = "US"
	contact.PaymentTermsDays = 60
	contact.CreditLimit = decimal.NewFromFloat(100000.00)
	contact.DefaultAccountID = nil // FK constraint requires valid account - keep nil
	contact.IsActive = false
	contact.Notes = "Updated notes"
	contact.UpdatedAt = time.Now()

	if err := repo.Update(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != "Fully Updated" {
		t.Errorf("expected name 'Fully Updated', got '%s'", retrieved.Name)
	}
	if retrieved.RegCode != "NEW-REG" {
		t.Errorf("expected reg_code 'NEW-REG', got '%s'", retrieved.RegCode)
	}
	if retrieved.VATNumber != "NEW-VAT" {
		t.Errorf("expected vat_number 'NEW-VAT', got '%s'", retrieved.VATNumber)
	}
	if retrieved.Email != "new@example.com" {
		t.Errorf("expected email 'new@example.com', got '%s'", retrieved.Email)
	}
	if retrieved.Phone != "+999" {
		t.Errorf("expected phone '+999', got '%s'", retrieved.Phone)
	}
	if retrieved.City != "New City" {
		t.Errorf("expected city 'New City', got '%s'", retrieved.City)
	}
	if retrieved.PaymentTermsDays != 60 {
		t.Errorf("expected payment_terms_days 60, got %d", retrieved.PaymentTermsDays)
	}
	if !retrieved.CreditLimit.Equal(decimal.NewFromFloat(100000.00)) {
		t.Errorf("expected credit_limit 100000.00, got %s", retrieved.CreditLimit)
	}
	if retrieved.IsActive {
		t.Error("expected is_active to be false")
	}
	if retrieved.Notes != "Updated notes" {
		t.Errorf("expected notes 'Updated notes', got '%s'", retrieved.Notes)
	}
}

func TestRepository_Update_NonExistent(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Name:             "Non-existent",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		UpdatedAt:        time.Now(),
	}

	// Update should not return an error for non-existent contact
	// It just affects 0 rows
	err := repo.Update(ctx, tenant.SchemaName, contact)
	if err != nil {
		t.Errorf("Update of non-existent contact should not return error, got: %v", err)
	}
}

func TestRepository_Delete(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "C001-del",
		Name:             "To Delete",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete (soft-delete) the contact
	if err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, contact.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify the contact is soft-deleted (is_active = false)
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.IsActive {
		t.Error("expected contact to be inactive after delete")
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, uuid.New().String())
	if err != ErrContactNotFound {
		t.Errorf("expected ErrContactNotFound, got %v", err)
	}
}

func TestRepository_Delete_WrongTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "DEL-WRONG",
		Name:             "Delete Wrong Tenant",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Try to delete with a different tenant ID
	wrongTenantID := uuid.New().String()
	err := repo.Delete(ctx, tenant.SchemaName, wrongTenantID, contact.ID)
	if err != ErrContactNotFound {
		t.Errorf("expected ErrContactNotFound for wrong tenant, got %v", err)
	}

	// Verify the contact is still active
	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if !retrieved.IsActive {
		t.Error("contact should still be active after failed delete with wrong tenant")
	}
}

func TestRepository_Delete_AlreadyInactive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "DEL-INACTIVE",
		Name:             "Already Inactive",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         false, // Already inactive
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete an already inactive contact - should still succeed
	err := repo.Delete(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Errorf("Delete of already inactive contact should succeed, got: %v", err)
	}
}

func TestRepository_List_AllFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create customer contacts
	for i := 1; i <= 2; i++ {
		contact := &Contact{
			ID:               uuid.New().String(),
			TenantID:         tenant.ID,
			Code:             "CUST-" + string(rune('0'+i)) + "-flt",
			Name:             "Customer " + string(rune('A'+i-1)),
			ContactType:      ContactTypeCustomer,
			CountryCode:      "EE",
			PaymentTermsDays: 14,
			CreditLimit:      decimal.Zero,
			IsActive:         true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
			t.Fatalf("Create customer failed: %v", err)
		}
	}

	// Create supplier contacts
	for i := 1; i <= 2; i++ {
		contact := &Contact{
			ID:               uuid.New().String(),
			TenantID:         tenant.ID,
			Code:             "SUPP-" + string(rune('0'+i)) + "-flt",
			Name:             "Supplier " + string(rune('A'+i-1)),
			ContactType:      ContactTypeSupplier,
			CountryCode:      "EE",
			PaymentTermsDays: 30,
			CreditLimit:      decimal.Zero,
			IsActive:         i == 1, // First is active, second is inactive
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
			t.Fatalf("Create supplier failed: %v", err)
		}
	}

	// Test filter by supplier type
	t.Run("FilterBySupplierType", func(t *testing.T) {
		filter := &ContactFilter{ContactType: ContactTypeSupplier}
		contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(contacts) < 2 {
			t.Errorf("expected at least 2 suppliers, got %d", len(contacts))
		}
		for _, c := range contacts {
			if c.ContactType != ContactTypeSupplier {
				t.Errorf("expected supplier type, got %s", c.ContactType)
			}
		}
	})

	// Test filter by search term
	t.Run("FilterBySearchTerm", func(t *testing.T) {
		filter := &ContactFilter{Search: "Customer A"}
		contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(contacts) < 1 {
			t.Error("expected at least 1 contact matching search term")
		}
	})

	// Test filter by active only
	t.Run("FilterActiveOnly", func(t *testing.T) {
		filter := &ContactFilter{ActiveOnly: true}
		contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		for _, c := range contacts {
			if !c.IsActive {
				t.Error("expected all contacts to be active")
			}
		}
	})

	// Test combined filters
	t.Run("CombinedFilters", func(t *testing.T) {
		filter := &ContactFilter{
			ContactType: ContactTypeSupplier,
			ActiveOnly:  true,
		}
		contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		for _, c := range contacts {
			if c.ContactType != ContactTypeSupplier {
				t.Errorf("expected supplier type, got %s", c.ContactType)
			}
			if !c.IsActive {
				t.Error("expected contact to be active")
			}
		}
	})
}

func TestRepository_List_SearchByCode(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "UNIQUE-CODE-123",
		Name:             "Search By Code",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	filter := &ContactFilter{Search: "UNIQUE-CODE"}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, c := range contacts {
		if c.ID == contact.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find contact by code search")
	}
}

func TestRepository_List_SearchByEmail(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "EMAIL-SEARCH",
		Name:             "Search By Email",
		Email:            "unique-email@searchtest.com",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	filter := &ContactFilter{Search: "unique-email@searchtest"}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, c := range contacts {
		if c.ID == contact.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find contact by email search")
	}
}

func TestRepository_List_SearchCaseInsensitive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "CASE-TEST",
		Name:             "UPPERCASE NAME",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Search with lowercase
	filter := &ContactFilter{Search: "uppercase name"}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, c := range contacts {
		if c.ID == contact.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected case-insensitive search to find contact")
	}
}

func TestRepository_List_AllFiltersCombined(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a specific contact that matches all filters
	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "ALL-FILTERS-MATCH",
		Name:             "Matching Contact",
		ContactType:      ContactTypeBoth,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create a contact that doesn't match
	notMatch := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "NOT-MATCH",
		Name:             "Not Matching",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, notMatch); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	filter := &ContactFilter{
		ContactType: ContactTypeBoth,
		ActiveOnly:  true,
		Search:      "Matching",
	}

	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(contacts) != 1 {
		t.Errorf("expected exactly 1 contact, got %d", len(contacts))
	}

	if len(contacts) > 0 && contacts[0].ID != contact.ID {
		t.Errorf("expected contact ID %s, got %s", contact.ID, contacts[0].ID)
	}
}

func TestRepository_List_NoFilterResults(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create a contact
	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "NO-MATCH",
		Name:             "No Match Contact",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Search for something that doesn't exist
	filter := &ContactFilter{Search: "ZZZZZZZ-NON-EXISTENT"}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(contacts) != 0 {
		t.Errorf("expected 0 contacts for non-matching filter, got %d", len(contacts))
	}
}

func TestRepository_ContactTypeBoth(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "BOTH-TYPE",
		Name:             "Both Type Contact",
		ContactType:      ContactTypeBoth,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.ContactType != ContactTypeBoth {
		t.Errorf("expected contact type BOTH, got %s", retrieved.ContactType)
	}

	// Filter by BOTH type
	filter := &ContactFilter{ContactType: ContactTypeBoth}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, c := range contacts {
		if c.ID == contact.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find BOTH type contact")
	}
}

func TestRepository_NilDefaultAccountID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "NIL-ACC",
		Name:             "Nil Account ID",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		DefaultAccountID: nil, // Explicitly nil
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.DefaultAccountID != nil {
		t.Errorf("expected nil DefaultAccountID, got %v", retrieved.DefaultAccountID)
	}
}

func TestRepository_ZeroValues(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "", // Empty code
		Name:             "Zero Values Test",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 0, // Zero payment terms
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Code != "" {
		t.Errorf("expected empty code, got %s", retrieved.Code)
	}
	if retrieved.PaymentTermsDays != 0 {
		t.Errorf("expected 0 payment terms, got %d", retrieved.PaymentTermsDays)
	}
}

func TestRepository_LargeCreditLimit(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	largeCreditLimit := decimal.NewFromFloat(999999999999.99)
	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "LARGE-CREDIT",
		Name:             "Large Credit Limit",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      largeCreditLimit,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if !retrieved.CreditLimit.Equal(largeCreditLimit) {
		t.Errorf("expected credit limit %s, got %s", largeCreditLimit, retrieved.CreditLimit)
	}
}

func TestRepository_MultipleTenantsIsolation(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant1 := testutil.CreateTestTenant(t, pool)
	tenant2 := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contact for tenant1
	contact1 := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant1.ID,
		Code:             "T1-CONTACT",
		Name:             "Tenant 1 Contact",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant1.SchemaName, contact1); err != nil {
		t.Fatalf("Create for tenant1 failed: %v", err)
	}

	// Create contact for tenant2
	contact2 := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant2.ID,
		Code:             "T2-CONTACT",
		Name:             "Tenant 2 Contact",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant2.SchemaName, contact2); err != nil {
		t.Fatalf("Create for tenant2 failed: %v", err)
	}

	// Tenant1 should only see their contact
	contacts1, err := repo.List(ctx, tenant1.SchemaName, tenant1.ID, nil)
	if err != nil {
		t.Fatalf("List for tenant1 failed: %v", err)
	}

	for _, c := range contacts1 {
		if c.TenantID != tenant1.ID {
			t.Errorf("tenant1 list should only contain tenant1 contacts, found tenant %s", c.TenantID)
		}
	}

	// Tenant2 should only see their contact
	contacts2, err := repo.List(ctx, tenant2.SchemaName, tenant2.ID, nil)
	if err != nil {
		t.Fatalf("List for tenant2 failed: %v", err)
	}

	for _, c := range contacts2 {
		if c.TenantID != tenant2.ID {
			t.Errorf("tenant2 list should only contain tenant2 contacts, found tenant %s", c.TenantID)
		}
	}

	// Cross-tenant access should not work
	_, err = repo.GetByID(ctx, tenant1.SchemaName, tenant2.ID, contact1.ID)
	if err != ErrContactNotFound {
		t.Error("cross-tenant GetByID should return ErrContactNotFound")
	}
}

func TestRepository_Timestamps(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	now := time.Now()
	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "TS-TEST",
		Name:             "Timestamp Test",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	// Timestamps should be preserved (within reasonable precision)
	if retrieved.CreatedAt.Sub(now).Abs() > time.Second {
		t.Errorf("created_at timestamp differs too much: expected %v, got %v", now, retrieved.CreatedAt)
	}
	if retrieved.UpdatedAt.Sub(now).Abs() > time.Second {
		t.Errorf("updated_at timestamp differs too much: expected %v, got %v", now, retrieved.UpdatedAt)
	}
}

func TestRepository_SpecialCharacters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "SPEC-CHAR",
		Name:             "O'Reilly & Associates - Test \"Quotes\" <brackets>",
		Email:            "test+special@example.com",
		AddressLine1:     "123 Main St #456",
		Notes:            "Notes with 'quotes' and \"double quotes\" and special chars: <>&",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, tenant.SchemaName, tenant.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if retrieved.Name != contact.Name {
		t.Errorf("expected name %s, got %s", contact.Name, retrieved.Name)
	}
	if retrieved.Notes != contact.Notes {
		t.Errorf("expected notes %s, got %s", contact.Notes, retrieved.Notes)
	}
}

func TestRepository_List_OnlyTypeFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create customers
	for i := 0; i < 3; i++ {
		contact := &Contact{
			ID:               uuid.New().String(),
			TenantID:         tenant.ID,
			Code:             "ONLY-TYPE-CUST-" + string(rune('0'+i)),
			Name:             "Only Type Customer " + string(rune('A'+i)),
			ContactType:      ContactTypeCustomer,
			CountryCode:      "EE",
			PaymentTermsDays: 14,
			CreditLimit:      decimal.Zero,
			IsActive:         true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Create suppliers
	for i := 0; i < 2; i++ {
		contact := &Contact{
			ID:               uuid.New().String(),
			TenantID:         tenant.ID,
			Code:             "ONLY-TYPE-SUPP-" + string(rune('0'+i)),
			Name:             "Only Type Supplier " + string(rune('A'+i)),
			ContactType:      ContactTypeSupplier,
			CountryCode:      "EE",
			PaymentTermsDays: 30,
			CreditLimit:      decimal.Zero,
			IsActive:         true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	// Filter only by type (not ActiveOnly, not Search)
	filter := &ContactFilter{ContactType: ContactTypeCustomer}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(contacts) < 3 {
		t.Errorf("expected at least 3 customers, got %d", len(contacts))
	}

	for _, c := range contacts {
		if c.ContactType != ContactTypeCustomer {
			t.Errorf("expected customer type, got %s", c.ContactType)
		}
	}
}

func TestRepository_List_OnlyActiveFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create active contact
	active := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "ACTIVE-ONLY",
		Name:             "Active Contact",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, active); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create inactive contact
	inactive := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "INACTIVE-ONLY",
		Name:             "Inactive Contact",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, inactive); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Filter only by ActiveOnly (not ContactType, not Search)
	filter := &ContactFilter{ActiveOnly: true}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, c := range contacts {
		if !c.IsActive {
			t.Errorf("expected only active contacts, got inactive: %s", c.Name)
		}
	}

	// Check that inactive contact is not included
	for _, c := range contacts {
		if c.ID == inactive.ID {
			t.Error("inactive contact should not be in results")
		}
	}
}

func TestRepository_List_OnlySearchFilter(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create contacts with specific names
	contact := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "SEARCH-ONLY",
		Name:             "Searchable Unique Name",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, contact); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Filter only by Search (not ContactType, not ActiveOnly)
	filter := &ContactFilter{Search: "Searchable Unique"}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, c := range contacts {
		if c.ID == contact.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find contact with search-only filter")
	}
}

func TestRepository_List_TypeAndActiveFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create active supplier
	activeSupplier := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "ACT-SUPP",
		Name:             "Active Supplier",
		ContactType:      ContactTypeSupplier,
		CountryCode:      "EE",
		PaymentTermsDays: 30,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, activeSupplier); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create inactive supplier
	inactiveSupplier := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "INACT-SUPP",
		Name:             "Inactive Supplier",
		ContactType:      ContactTypeSupplier,
		CountryCode:      "EE",
		PaymentTermsDays: 30,
		CreditLimit:      decimal.Zero,
		IsActive:         false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, inactiveSupplier); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create active customer
	activeCustomer := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "ACT-CUST",
		Name:             "Active Customer",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, activeCustomer); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Filter by ContactType AND ActiveOnly (not Search)
	filter := &ContactFilter{
		ContactType: ContactTypeSupplier,
		ActiveOnly:  true,
	}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, c := range contacts {
		if c.ContactType != ContactTypeSupplier {
			t.Errorf("expected supplier type, got %s", c.ContactType)
		}
		if !c.IsActive {
			t.Error("expected only active contacts")
		}
	}

	// Should find active supplier but not inactive supplier or active customer
	foundActive := false
	foundInactive := false
	foundCustomer := false
	for _, c := range contacts {
		if c.ID == activeSupplier.ID {
			foundActive = true
		}
		if c.ID == inactiveSupplier.ID {
			foundInactive = true
		}
		if c.ID == activeCustomer.ID {
			foundCustomer = true
		}
	}

	if !foundActive {
		t.Error("expected to find active supplier")
	}
	if foundInactive {
		t.Error("should not find inactive supplier")
	}
	if foundCustomer {
		t.Error("should not find customer when filtering by supplier type")
	}
}

func TestRepository_List_TypeAndSearchFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create supplier with specific name
	supplier := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "TYPE-SEARCH-SUPP",
		Name:             "Findable Supplier",
		ContactType:      ContactTypeSupplier,
		CountryCode:      "EE",
		PaymentTermsDays: 30,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, supplier); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create customer with similar name
	customer := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "TYPE-SEARCH-CUST",
		Name:             "Findable Customer",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, customer); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Filter by ContactType AND Search (not ActiveOnly)
	filter := &ContactFilter{
		ContactType: ContactTypeSupplier,
		Search:      "Findable",
	}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	foundSupplier := false
	foundCustomer := false
	for _, c := range contacts {
		if c.ID == supplier.ID {
			foundSupplier = true
		}
		if c.ID == customer.ID {
			foundCustomer = true
		}
	}

	if !foundSupplier {
		t.Error("expected to find supplier with combined filters")
	}
	if foundCustomer {
		t.Error("should not find customer when filtering by supplier type")
	}
}

func TestRepository_List_ActiveAndSearchFilters(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create active contact with specific name
	active := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "ACT-SEARCH",
		Name:             "Locatable Active",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, active); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create inactive contact with similar name
	inactive := &Contact{
		ID:               uuid.New().String(),
		TenantID:         tenant.ID,
		Code:             "INACT-SEARCH",
		Name:             "Locatable Inactive",
		ContactType:      ContactTypeCustomer,
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.Zero,
		IsActive:         false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := repo.Create(ctx, tenant.SchemaName, inactive); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Filter by ActiveOnly AND Search (not ContactType)
	filter := &ContactFilter{
		ActiveOnly: true,
		Search:     "Locatable",
	}
	contacts, err := repo.List(ctx, tenant.SchemaName, tenant.ID, filter)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	foundActive := false
	foundInactive := false
	for _, c := range contacts {
		if c.ID == active.ID {
			foundActive = true
		}
		if c.ID == inactive.ID {
			foundInactive = true
		}
	}

	if !foundActive {
		t.Error("expected to find active contact")
	}
	if foundInactive {
		t.Error("should not find inactive contact when filtering by active only")
	}
}
