//go:build integration && gorm

package database

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func getTestDatabaseURL(t *testing.T) string {
	t.Helper()
	return testutil.SetupTestDB(t).Config().ConnString()
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
