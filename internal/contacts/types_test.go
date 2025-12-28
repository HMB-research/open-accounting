package contacts

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestContactType_Values(t *testing.T) {
	tests := []struct {
		name     string
		ct       ContactType
		expected string
	}{
		{"customer", ContactTypeCustomer, "CUSTOMER"},
		{"supplier", ContactTypeSupplier, "SUPPLIER"},
		{"both", ContactTypeBoth, "BOTH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.ct))
		})
	}
}

func TestContact_Defaults(t *testing.T) {
	contact := Contact{
		ID:          "contact-123",
		TenantID:    "tenant-456",
		Name:        "Test Company",
		ContactType: ContactTypeCustomer,
		CountryCode: "EE",
		IsActive:    true,
	}

	assert.Equal(t, "contact-123", contact.ID)
	assert.Equal(t, "tenant-456", contact.TenantID)
	assert.Equal(t, "Test Company", contact.Name)
	assert.Equal(t, ContactTypeCustomer, contact.ContactType)
	assert.Equal(t, "EE", contact.CountryCode)
	assert.True(t, contact.IsActive)
	assert.Equal(t, 0, contact.PaymentTermsDays)
	assert.True(t, contact.CreditLimit.IsZero())
}

func TestContact_WithCreditLimit(t *testing.T) {
	contact := Contact{
		ID:          "contact-123",
		Name:        "Big Customer",
		ContactType: ContactTypeCustomer,
		CreditLimit: decimal.NewFromFloat(50000),
	}

	assert.True(t, contact.CreditLimit.Equal(decimal.NewFromFloat(50000)))
}

func TestContact_PaymentTerms(t *testing.T) {
	contact := Contact{
		ID:               "contact-123",
		Name:             "Net-30 Customer",
		ContactType:      ContactTypeCustomer,
		PaymentTermsDays: 30,
	}

	assert.Equal(t, 30, contact.PaymentTermsDays)
}

func TestCreateContactRequest_Minimal(t *testing.T) {
	req := CreateContactRequest{
		Name:        "New Customer",
		ContactType: ContactTypeCustomer,
	}

	assert.Equal(t, "New Customer", req.Name)
	assert.Equal(t, ContactTypeCustomer, req.ContactType)
	assert.Equal(t, "", req.Email)
	assert.Equal(t, "", req.VATNumber)
}

func TestCreateContactRequest_Full(t *testing.T) {
	accountID := "acc-123"
	req := CreateContactRequest{
		Code:             "CUST-001",
		Name:             "Full Customer",
		ContactType:      ContactTypeBoth,
		RegCode:          "12345678",
		VATNumber:        "EE123456789",
		Email:            "test@example.com",
		Phone:            "+372 5551234",
		AddressLine1:     "Main Street 1",
		AddressLine2:     "Suite 100",
		City:             "Tallinn",
		PostalCode:       "10111",
		CountryCode:      "EE",
		PaymentTermsDays: 14,
		CreditLimit:      decimal.NewFromFloat(10000),
		DefaultAccountID: &accountID,
		Notes:            "Important customer",
	}

	assert.Equal(t, "CUST-001", req.Code)
	assert.Equal(t, "Full Customer", req.Name)
	assert.Equal(t, ContactTypeBoth, req.ContactType)
	assert.Equal(t, "EE123456789", req.VATNumber)
	assert.Equal(t, "test@example.com", req.Email)
	assert.Equal(t, 14, req.PaymentTermsDays)
	assert.True(t, req.CreditLimit.Equal(decimal.NewFromFloat(10000)))
	assert.Equal(t, "acc-123", *req.DefaultAccountID)
}

func TestUpdateContactRequest_PartialUpdate(t *testing.T) {
	name := "Updated Name"
	email := "new@example.com"
	active := false

	req := UpdateContactRequest{
		Name:     &name,
		Email:    &email,
		IsActive: &active,
	}

	assert.Equal(t, "Updated Name", *req.Name)
	assert.Equal(t, "new@example.com", *req.Email)
	assert.False(t, *req.IsActive)
	assert.Nil(t, req.Phone)
	assert.Nil(t, req.VATNumber)
}

func TestContactFilter_ActiveOnly(t *testing.T) {
	filter := ContactFilter{
		ActiveOnly: true,
	}

	assert.True(t, filter.ActiveOnly)
	assert.Equal(t, ContactType(""), filter.ContactType)
}

func TestContactFilter_ByType(t *testing.T) {
	filter := ContactFilter{
		ContactType: ContactTypeSupplier,
	}

	assert.Equal(t, ContactTypeSupplier, filter.ContactType)
}

func TestContactFilter_WithSearch(t *testing.T) {
	filter := ContactFilter{
		Search:     "acme",
		ActiveOnly: true,
	}

	assert.Equal(t, "acme", filter.Search)
	assert.True(t, filter.ActiveOnly)
}

func TestContact_SupplierType(t *testing.T) {
	contact := Contact{
		ID:          "supp-123",
		Name:        "Supplier Inc",
		ContactType: ContactTypeSupplier,
		VATNumber:   "DE123456789",
		CountryCode: "DE",
	}

	assert.Equal(t, ContactTypeSupplier, contact.ContactType)
	assert.Equal(t, "DE123456789", contact.VATNumber)
	assert.Equal(t, "DE", contact.CountryCode)
}
