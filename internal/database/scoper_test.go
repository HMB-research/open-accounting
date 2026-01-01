//go:build gorm

package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithSchema(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_123"

	result := WithSchema(ctx, schemaName)

	// Verify the schema was added to context
	value := result.Value(schemaKey)
	assert.Equal(t, schemaName, value)
}

func TestGetSchema(t *testing.T) {
	t.Run("returns schema from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), schemaKey, "tenant_test")

		result := GetSchema(ctx)

		assert.Equal(t, "tenant_test", result)
	})

	t.Run("returns public when no schema in context", func(t *testing.T) {
		ctx := context.Background()

		result := GetSchema(ctx)

		assert.Equal(t, "public", result)
	})
}

func TestWithSchemaAndGetSchema_Integration(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_integration_test"

	// Set schema
	ctx = WithSchema(ctx, schemaName)

	// Get schema
	result := GetSchema(ctx)

	assert.Equal(t, schemaName, result)
}

func TestNewTenantDBCache(t *testing.T) {
	// Note: We can't test with a real GORM DB without a database connection
	// but we can test that the cache is initialized correctly
	cache := NewTenantDBCache(nil)
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.cache)
	assert.Empty(t, cache.cache)
}

func TestTenantDBCache_Clear(t *testing.T) {
	cache := NewTenantDBCache(nil)
	// Manually add something to cache
	cache.cache["test_schema"] = nil
	assert.Len(t, cache.cache, 1)

	// Clear should empty it
	cache.Clear()
	assert.Empty(t, cache.cache)
}

func TestTenantDBCache_Remove(t *testing.T) {
	cache := NewTenantDBCache(nil)
	// Manually add something to cache
	cache.cache["test_schema1"] = nil
	cache.cache["test_schema2"] = nil
	assert.Len(t, cache.cache, 2)

	// Remove one
	cache.Remove("test_schema1")
	assert.Len(t, cache.cache, 1)
	_, exists := cache.cache["test_schema1"]
	assert.False(t, exists)
	_, exists = cache.cache["test_schema2"]
	assert.True(t, exists)

	// Remove non-existent should not panic
	cache.Remove("non_existent")
	assert.Len(t, cache.cache, 1)
}

func TestTenantDBCache_Get_PublicSchema(t *testing.T) {
	cache := NewTenantDBCache(nil)

	// Empty schema should return base
	result := cache.Get("")
	assert.Nil(t, result) // Our base is nil

	// Public schema should return base
	result = cache.Get("public")
	assert.Nil(t, result) // Our base is nil
}
