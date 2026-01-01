//go:build integration && gorm

package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func getTestDatabaseURL(t *testing.T) string {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	return dbURL
}

func TestGormDB_New(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	assert.NotNil(t, gormDB)
	assert.NotNil(t, gormDB.DB)

	err = gormDB.Close()
	assert.NoError(t, err)
}

func TestGormDB_New_InvalidConnection(t *testing.T) {
	ctx := context.Background()
	_, err := NewGormDB(ctx, "postgres://invalid:invalid@localhost:9999/nonexistent")
	assert.Error(t, err)
}

func TestGormDB_Close(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)

	err = gormDB.Close()
	assert.NoError(t, err)
}

func TestGormDB_WithContext(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	defer gormDB.Close()

	// Test WithContext returns a valid DB
	db := gormDB.WithContext(ctx)
	assert.NotNil(t, db)

	// Verify it works by running a simple query
	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, 1, result)
}

func TestGormDB_Transaction(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	defer gormDB.Close()

	// Test successful transaction
	err = gormDB.Transaction(ctx, func(tx *gorm.DB) error {
		var result int
		return tx.Raw("SELECT 1").Scan(&result).Error
	})
	assert.NoError(t, err)
}

func TestGormDB_Transaction_Rollback(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	defer gormDB.Close()

	// Test transaction rollback on error
	err = gormDB.Transaction(ctx, func(tx *gorm.DB) error {
		return assert.AnError // Return error to trigger rollback
	})
	assert.Error(t, err)
}

// Test SchemaScope function
func TestSchemaScope(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	defer gormDB.Close()

	t.Run("with tenant schema", func(t *testing.T) {
		scope := SchemaScope("test_tenant_schema")
		assert.NotNil(t, scope)

		// Apply scope to DB
		db := scope(gormDB.DB)
		assert.NotNil(t, db)
	})

	t.Run("with public schema", func(t *testing.T) {
		scope := SchemaScope("public")
		assert.NotNil(t, scope)

		// Should return DB unchanged
		db := scope(gormDB.DB)
		assert.NotNil(t, db)
	})

	t.Run("with empty schema", func(t *testing.T) {
		scope := SchemaScope("")
		assert.NotNil(t, scope)

		// Should return DB unchanged
		db := scope(gormDB.DB)
		assert.NotNil(t, db)
	})
}

// Test TenantDB function
func TestTenantDB(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	defer gormDB.Close()

	t.Run("with tenant schema", func(t *testing.T) {
		db := TenantDB(gormDB.DB, "test_tenant_schema")
		assert.NotNil(t, db)
	})

	t.Run("with public schema", func(t *testing.T) {
		db := TenantDB(gormDB.DB, "public")
		assert.NotNil(t, db)
		// Should return same DB
		assert.Equal(t, gormDB.DB, db)
	})

	t.Run("with empty schema", func(t *testing.T) {
		db := TenantDB(gormDB.DB, "")
		assert.NotNil(t, db)
		// Should return same DB
		assert.Equal(t, gormDB.DB, db)
	})
}

// Test TenantDBCache with real GORM DB
func TestTenantDBCache_Integration(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	defer gormDB.Close()

	cache := NewTenantDBCache(gormDB.DB)

	t.Run("get creates new scoped DB", func(t *testing.T) {
		db := cache.Get("tenant_test_1")
		assert.NotNil(t, db)
	})

	t.Run("get returns cached DB", func(t *testing.T) {
		db1 := cache.Get("tenant_test_2")
		db2 := cache.Get("tenant_test_2")
		// Should return the same cached instance
		assert.Equal(t, db1, db2)
	})

	t.Run("remove clears specific schema", func(t *testing.T) {
		db1 := cache.Get("tenant_test_3")
		assert.NotNil(t, db1)

		cache.Remove("tenant_test_3")

		// Get again should create a new one
		db2 := cache.Get("tenant_test_3")
		assert.NotNil(t, db2)
	})

	t.Run("clear removes all cached DBs", func(t *testing.T) {
		cache.Get("tenant_test_4")
		cache.Get("tenant_test_5")

		cache.Clear()

		// Get should create new instances
		db := cache.Get("tenant_test_4")
		assert.NotNil(t, db)
	})
}
