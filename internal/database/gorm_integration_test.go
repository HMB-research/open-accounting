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

	t.Run("with invalid schema", func(t *testing.T) {
		scope := SchemaScope("tenant-demo")
		db := scope(gormDB.DB)
		assert.Error(t, db.Error)
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

	t.Run("with nil db", func(t *testing.T) {
		assert.Nil(t, TenantDB(nil, "public"))
	})
}

func TestTenantTable(t *testing.T) {
	dbURL := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	gormDB, err := NewGormDB(ctx, dbURL)
	require.NoError(t, err)
	defer gormDB.Close()

	db, err := TenantTable(gormDB.DB, "public", "tenants")
	require.NoError(t, err)
	assert.NotNil(t, db)

	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error
	assert.NoError(t, err)
	assert.Equal(t, 1, result)

	_, err = TenantTable(gormDB.DB, "tenant-demo", "contacts")
	assert.Error(t, err)
}
