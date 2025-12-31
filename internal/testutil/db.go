//go:build integration

package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestTenant contains the test tenant information
type TestTenant struct {
	ID         string
	SchemaName string
	Name       string
	Slug       string
}

// SetupTestDB connects to the test database. It skips the test if DATABASE_URL is not set.
// Returns the pool and a cleanup function.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// SetupTestSchema creates an isolated schema for the test.
// Returns the schema name and a cleanup function.
func SetupTestSchema(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()

	// Generate unique schema name based on test name and timestamp
	testName := strings.ToLower(t.Name())
	testName = strings.ReplaceAll(testName, "/", "_")
	testName = strings.ReplaceAll(testName, " ", "_")
	// Limit length and add uniqueness
	if len(testName) > 30 {
		testName = testName[:30]
	}
	schemaName := fmt.Sprintf("test_%s_%d", testName, time.Now().UnixNano()%100000)

	ctx := context.Background()

	// Create the schema using the existing create_tenant_schema function
	_, err := pool.Exec(ctx, "SELECT create_tenant_schema($1)", schemaName)
	if err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}

	t.Cleanup(func() {
		TeardownTestSchema(t, pool, schemaName)
	})

	return schemaName
}

// CreateTestTenant creates a tenant for integration tests.
// Returns the tenant info. The tenant is automatically cleaned up after the test.
func CreateTestTenant(t *testing.T, pool *pgxpool.Pool) *TestTenant {
	t.Helper()

	ctx := context.Background()

	tenantID := uuid.New().String()
	testName := strings.ToLower(t.Name())
	testName = strings.ReplaceAll(testName, "/", "_")
	testName = strings.ReplaceAll(testName, " ", "_")
	if len(testName) > 30 {
		testName = testName[:30]
	}

	slug := fmt.Sprintf("test_%s_%d", testName, time.Now().UnixNano()%100000)
	schemaName := fmt.Sprintf("tenant_%s", strings.ReplaceAll(slug, "-", "_"))
	name := fmt.Sprintf("Test Tenant %s", testName)

	now := time.Now()
	settings := []byte(`{}`)

	// Insert tenant record
	_, err := pool.Exec(ctx, `
		INSERT INTO tenants (id, name, slug, schema_name, settings, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, $6, $7)
	`, tenantID, name, slug, schemaName, settings, now, now)
	if err != nil {
		t.Fatalf("failed to create test tenant: %v", err)
	}

	// Create tenant schema with all tables
	_, err = pool.Exec(ctx, "SELECT create_tenant_schema($1)", schemaName)
	if err != nil {
		t.Fatalf("failed to create tenant schema: %v", err)
	}

	// Create default chart of accounts
	_, err = pool.Exec(ctx, "SELECT create_default_chart_of_accounts($1, $2)", schemaName, tenantID)
	if err != nil {
		t.Fatalf("failed to create default chart of accounts: %v", err)
	}

	tenant := &TestTenant{
		ID:         tenantID,
		SchemaName: schemaName,
		Name:       name,
		Slug:       slug,
	}

	t.Cleanup(func() {
		cleanupTestTenant(t, pool, tenant)
	})

	return tenant
}

// TeardownTestSchema drops a test schema
func TeardownTestSchema(t *testing.T, pool *pgxpool.Pool, schemaName string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use CASCADE to drop all objects in the schema
	_, err := pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
	if err != nil {
		t.Logf("warning: failed to drop test schema %s: %v", schemaName, err)
	}
}

// cleanupTestTenant removes the test tenant and its schema
func cleanupTestTenant(t *testing.T, pool *pgxpool.Pool, tenant *TestTenant) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Drop tenant schema first
	_, err := pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", tenant.SchemaName))
	if err != nil {
		t.Logf("warning: failed to drop tenant schema %s: %v", tenant.SchemaName, err)
	}

	// Delete tenant record
	_, err = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", tenant.ID)
	if err != nil {
		t.Logf("warning: failed to delete test tenant %s: %v", tenant.ID, err)
	}
}

// CreateTestUser creates a test user for integration tests.
// Returns the user ID. The user is automatically cleaned up after the test.
func CreateTestUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()

	ctx := context.Background()

	userID := uuid.New().String()
	now := time.Now()

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, $6)
	`, userID, email, "hashed_password", "Test User", now, now)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	t.Cleanup(func() {
		cleanupTestUser(t, pool, userID)
	})

	return userID
}

// cleanupTestUser removes the test user
func cleanupTestUser(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Remove from tenant_users first (foreign key)
	_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE user_id = $1", userID)

	// Delete user
	_, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		t.Logf("warning: failed to delete test user %s: %v", userID, err)
	}
}

// AddUserToTenant adds a user to a tenant for testing
func AddUserToTenant(t *testing.T, pool *pgxpool.Pool, tenantID, userID, role string) {
	t.Helper()

	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO tenant_users (tenant_id, user_id, role, is_default, created_at)
		VALUES ($1, $2, $3, false, NOW())
	`, tenantID, userID, role)
	if err != nil {
		t.Fatalf("failed to add user to tenant: %v", err)
	}
}

// SetupGormDB creates a GORM database connection for testing.
// It skips the test if DATABASE_URL is not set.
// Returns the GORM DB instance.
func SetupGormDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("failed to connect to database with GORM: %v", err)
	}

	// Verify connection
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Logf("warning: failed to close GORM connection: %v", err)
		}
	})

	return db
}
