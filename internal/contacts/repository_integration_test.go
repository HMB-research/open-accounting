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
