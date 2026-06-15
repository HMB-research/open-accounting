package contactrefs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

func TestSupplierLookupResolveID(t *testing.T) {
	explicitID := "11111111-1111-4111-8111-111111111111"
	lookup := NewSupplierLookup([]contacts.Contact{
		{
			ID:        "supplier-1",
			Code:      "SUP-001",
			Name:      "Supplier One",
			RegCode:   "12345678",
			VATNumber: "EE12345678",
			Email:     "billing@supplier.example",
		},
		{
			ID:    "supplier-2",
			Code:  "SUP-002",
			Name:  "Supplier Two",
			Email: "accounts@supplier.example",
		},
	})

	t.Run("preserves explicit supplier id", func(t *testing.T) {
		id, err := lookup.ResolveID(" "+explicitID+" ", Reference{Field: "supplier_code", Value: "SUP-001"})

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, explicitID, *id)
	})

	t.Run("reports invalid explicit supplier id", func(t *testing.T) {
		id, err := lookup.ResolveID("legacy-supplier", Reference{Field: "supplier_code", Value: "SUP-001"})

		require.EqualError(t, err, "supplier_id must be a valid UUID")
		assert.Nil(t, id)
	})

	t.Run("resolves code case insensitively", func(t *testing.T) {
		id, err := lookup.ResolveID("", Reference{Field: "supplier_code", Value: " sup-001 "})

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "supplier-1", *id)
	})

	t.Run("resolves later identity fields", func(t *testing.T) {
		id, err := lookup.ResolveID("",
			Reference{Field: "supplier_code"},
			Reference{Field: "supplier_email", Value: "BILLING@SUPPLIER.EXAMPLE"},
		)

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "supplier-1", *id)
	})

	t.Run("returns nil when no supplier reference is present", func(t *testing.T) {
		id, err := lookup.ResolveID("", Reference{Field: "supplier_code"}, Reference{Field: "supplier_name"})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("reports missing supplier reference", func(t *testing.T) {
		id, err := lookup.ResolveID("", Reference{Field: "supplier_name", Value: "Missing Supplier"})

		require.EqualError(t, err, `supplier_name "Missing Supplier" was not found`)
		assert.Nil(t, id)
	})
}

func TestSupplierLookupResolveIDReportsDuplicateReferences(t *testing.T) {
	lookup := NewSupplierLookup([]contacts.Contact{
		{ID: "supplier-1", Name: "Supplier One"},
		{ID: "supplier-2", Name: " supplier one "},
	})

	id, err := lookup.ResolveID("", Reference{Field: "supplier_name", Value: "Supplier One"})

	require.EqualError(t, err, `supplier_name "Supplier One" matched multiple contacts`)
	assert.Nil(t, id)
}

func TestContactLookupResolveID(t *testing.T) {
	explicitID := "11111111-1111-4111-8111-111111111111"
	lookup := NewContactLookup([]contacts.Contact{
		{
			ID:        "contact-1",
			Code:      "CUST-001",
			Name:      "Customer One",
			RegCode:   "12345678",
			VATNumber: "EE12345678",
			Email:     "billing@customer.example",
		},
		{
			ID:    "contact-2",
			Code:  "CUST-002",
			Name:  "Customer Two",
			Email: "accounts@customer.example",
		},
	})

	t.Run("preserves explicit contact id", func(t *testing.T) {
		id, err := lookup.ResolveID(" "+explicitID+" ", Reference{Field: "contact_code", Value: "CUST-001"})

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, explicitID, *id)
	})

	t.Run("reports invalid explicit contact id", func(t *testing.T) {
		id, err := lookup.ResolveID("legacy-contact", Reference{Field: "contact_code", Value: "CUST-001"})

		require.EqualError(t, err, "contact_id must be a valid UUID")
		assert.Nil(t, id)
	})

	t.Run("resolves identity fields in priority order", func(t *testing.T) {
		id, err := lookup.ResolveID("",
			Reference{Field: "contact_code"},
			Reference{Field: "contact_email", Value: "BILLING@CUSTOMER.EXAMPLE"},
			Reference{Field: "contact_name", Value: "Customer Two"},
		)

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "contact-1", *id)
	})

	t.Run("returns nil when no contact reference is present", func(t *testing.T) {
		id, err := lookup.ResolveID("", Reference{Field: "contact_code"}, Reference{Field: "contact_name"})

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("reports missing contact reference", func(t *testing.T) {
		id, err := lookup.ResolveID("", Reference{Field: "contact_name", Value: "Missing Customer"})

		require.EqualError(t, err, `contact_name "Missing Customer" was not found`)
		assert.Nil(t, id)
	})
}

func TestContactLookupResolveIDReportsDuplicateReferences(t *testing.T) {
	lookup := NewContactLookup([]contacts.Contact{
		{ID: "contact-1", Name: "Customer One"},
		{ID: "contact-2", Name: " customer one "},
	})

	id, err := lookup.ResolveID("", Reference{Field: "contact_name", Value: "Customer One"})

	require.EqualError(t, err, `contact_name "Customer One" matched multiple contacts`)
	assert.Nil(t, id)
}
