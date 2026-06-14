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
