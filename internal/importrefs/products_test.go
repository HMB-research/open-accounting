package importrefs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/inventory"
)

func TestProductLookupResolveID(t *testing.T) {
	explicitID := "11111111-1111-4111-8111-111111111111"
	lookup := NewProductLookup([]inventory.Product{
		{ID: "empty-code", Code: "  "},
		{ID: "prod-1", Code: "SERV-001"},
		{ID: "prod-1", Code: " serv-001 "},
		{ID: "prod-2", Code: "WIDGET"},
	})

	t.Run("preserves explicit product id", func(t *testing.T) {
		id, err := lookup.ResolveID(" "+explicitID+" ", "SERV-001")

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, explicitID, *id)
	})

	t.Run("reports invalid explicit product id", func(t *testing.T) {
		id, err := lookup.ResolveID("legacy-product", "SERV-001")

		require.EqualError(t, err, "product_id must be a valid UUID")
		assert.Nil(t, id)
	})

	t.Run("resolves product code case insensitively", func(t *testing.T) {
		id, err := lookup.ResolveID("", " serv-001 ")

		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, "prod-1", *id)
	})

	t.Run("returns nil when no product reference is present", func(t *testing.T) {
		id, err := lookup.ResolveID("", "")

		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("reports missing product code", func(t *testing.T) {
		id, err := lookup.ResolveID("", "MISSING")

		require.EqualError(t, err, `product_code "MISSING" was not found`)
		assert.Nil(t, id)
	})
}

func TestProductLookupResolveIDReportsDuplicateCodes(t *testing.T) {
	lookup := NewProductLookup([]inventory.Product{
		{ID: "prod-1", Code: "SERV-001"},
		{ID: "prod-2", Code: "serv-001"},
	})

	id, err := lookup.ResolveID("", "SERV-001")

	require.EqualError(t, err, `product_code "SERV-001" matched multiple products`)
	assert.Nil(t, id)
}
