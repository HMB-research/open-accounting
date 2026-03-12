//go:build integration

// Package testutil provides test utilities for integration tests.
package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// schemaLifecycleMutex serializes tenant/schema DDL within a test process.
// Cross-process coordination is handled by the PostgreSQL advisory lock below.
var schemaLifecycleMutex sync.Mutex

// Advisory lock key for database-level tenant/schema lifecycle serialization.
// Setup and cleanup use the same lock to avoid DDL deadlocks across test packages.
const schemaLifecycleAdvisoryLockKey = 12345678

// TestTenant contains the test tenant information
type TestTenant struct {
	ID         string
	SchemaName string
	Name       string
	Slug       string
}

// SetupTestDB connects to the test database.
// If DATABASE_URL is set, it uses that database.
// Otherwise, it uses testcontainers to start a PostgreSQL container.
// Returns the pool.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	// Use GetTestContainer which handles both DATABASE_URL and testcontainers
	return GetTestContainer(t)
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

	schemaLifecycleMutex.Lock()
	defer schemaLifecycleMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := acquireSchemaLifecycleConn(ctx, pool)
	if err != nil {
		t.Fatalf("failed to acquire lifecycle lock for test schema setup: %v", err)
	}
	defer releaseSchemaLifecycleConn(ctx, conn)

	// Create the schema using the existing create_tenant_schema function
	_, err = conn.Exec(ctx, "SELECT create_tenant_schema($1)", schemaName)
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

	schemaLifecycleMutex.Lock()
	defer schemaLifecycleMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := acquireSchemaLifecycleConn(ctx, pool)
	if err != nil {
		t.Fatalf("failed to acquire lifecycle lock for tenant setup: %v", err)
	}
	defer releaseSchemaLifecycleConn(ctx, conn)

	// Reset search_path to ensure we're using public schema
	_, err = conn.Exec(ctx, "SET search_path TO public")
	if err != nil {
		t.Fatalf("failed to reset search_path: %v", err)
	}

	// Insert tenant record (explicitly use public schema)
	_, err = conn.Exec(ctx, `
		INSERT INTO public.tenants (id, name, slug, schema_name, settings, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, $6, $7)
	`, tenantID, name, slug, schemaName, settings, now, now)
	if err != nil {
		t.Fatalf("failed to create test tenant: %v", err)
	}

	// Create tenant schema with all tables
	_, err = conn.Exec(ctx, "SELECT create_tenant_schema($1)", schemaName)
	if err != nil {
		t.Fatalf("failed to create tenant schema: %v", err)
	}

	// Add quotes and orders tables (from migration 014)
	_, err = conn.Exec(ctx, "SELECT add_quotes_and_orders_tables($1)", schemaName)
	if err != nil {
		t.Fatalf("failed to add quotes and orders tables: %v", err)
	}

	// Add fixed assets tables (from migration 015)
	_, err = conn.Exec(ctx, "SELECT add_fixed_assets_tables($1)", schemaName)
	if err != nil {
		t.Fatalf("failed to add fixed assets tables: %v", err)
	}

	// Add inventory tables (from migration 016)
	_, err = conn.Exec(ctx, "SELECT create_inventory_tables($1)", schemaName)
	if err != nil {
		t.Fatalf("failed to add inventory tables: %v", err)
	}

	// Add leave management tables (from migration 017)
	_, err = conn.Exec(ctx, "SELECT add_leave_management_tables($1)", schemaName)
	if err != nil {
		t.Fatalf("failed to add leave management tables: %v", err)
	}

	// Add VAT columns to journal_entry_lines (from migration 020)
	_, err = conn.Exec(ctx, "SELECT add_vat_columns_to_journal_lines($1)", schemaName)
	if err != nil {
		// This migration may not exist in all environments, log but don't fail
		t.Logf("warning: VAT columns not added (migration may not exist): %v", err)
	}

	// Create default chart of accounts
	_, err = conn.Exec(ctx, "SELECT create_default_chart_of_accounts($1, $2)", schemaName, tenantID)
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

	// Serialize cleanup to prevent deadlocks between parallel test cleanups.
	schemaLifecycleMutex.Lock()
	defer schemaLifecycleMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := acquireSchemaLifecycleConn(ctx, pool)
	if err != nil {
		t.Logf("warning: failed to acquire lifecycle lock for schema cleanup: %v", err)
		return
	}
	defer releaseSchemaLifecycleConn(ctx, conn)

	// Use CASCADE to drop all objects in the schema
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
	if err != nil {
		t.Logf("warning: failed to drop test schema %s: %v", schemaName, err)
	}
}

// cleanupTestTenant removes the test tenant and its schema
func cleanupTestTenant(t *testing.T, pool *pgxpool.Pool, tenant *TestTenant) {
	t.Helper()

	// Serialize cleanup to prevent deadlocks between DROP SCHEMA and DELETE FROM tenants.
	schemaLifecycleMutex.Lock()
	defer schemaLifecycleMutex.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := acquireSchemaLifecycleConn(ctx, pool)
	if err != nil {
		t.Logf("warning: failed to acquire lifecycle lock for tenant cleanup: %v", err)
		return
	}
	defer releaseSchemaLifecycleConn(ctx, conn)

	// Reset search_path to ensure we're using public schema
	_, _ = conn.Exec(ctx, "SET search_path TO public")

	// Drop tenant schema first (this is the heavyweight operation)
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", tenant.SchemaName))
	if err != nil {
		t.Logf("warning: failed to drop tenant schema %s: %v", tenant.SchemaName, err)
	}

	// Delete tenant record (only after schema is dropped) - explicitly use public schema
	_, err = conn.Exec(ctx, "DELETE FROM public.tenants WHERE id = $1", tenant.ID)
	if err != nil {
		t.Logf("warning: failed to delete test tenant %s: %v", tenant.ID, err)
	}
}

func acquireSchemaLifecycleConn(ctx context.Context, pool *pgxpool.Pool) (*pgxpool.Conn, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", schemaLifecycleAdvisoryLockKey); err != nil {
		conn.Release()
		return nil, err
	}

	return conn, nil
}

func releaseSchemaLifecycleConn(ctx context.Context, conn *pgxpool.Conn) {
	_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", schemaLifecycleAdvisoryLockKey)
	conn.Release()
}

// CreateTestUser creates a test user for integration tests.
// Returns the user ID. The user is automatically cleaned up after the test.
func CreateTestUser(t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()

	ctx := context.Background()

	userID := uuid.New().String()
	now := time.Now()

	_, err := pool.Exec(ctx, `
		INSERT INTO public.users (id, email, password_hash, name, is_active, created_at, updated_at)
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
	_, _ = pool.Exec(ctx, "DELETE FROM public.tenant_users WHERE user_id = $1", userID)

	// Delete user
	_, err := pool.Exec(ctx, "DELETE FROM public.users WHERE id = $1", userID)
	if err != nil {
		t.Logf("warning: failed to delete test user %s: %v", userID, err)
	}
}

// AddUserToTenant adds a user to a tenant for testing
func AddUserToTenant(t *testing.T, pool *pgxpool.Pool, tenantID, userID, role string) {
	t.Helper()

	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO public.tenant_users (tenant_id, user_id, role, is_default, created_at)
		VALUES ($1, $2, $3, false, NOW())
	`, tenantID, userID, role)
	if err != nil {
		t.Fatalf("failed to add user to tenant: %v", err)
	}
}

// SetupGormDB creates a GORM database connection for testing.
// If DATABASE_URL is set, it uses that database.
// Otherwise, it uses testcontainers to start a PostgreSQL container.
// Returns the GORM DB instance.
func SetupGormDB(t *testing.T) *gorm.DB {
	t.Helper()

	// Get database URL - either from environment or from testcontainer
	var dbURL string
	if envURL := os.Getenv("DATABASE_URL"); envURL != "" {
		dbURL = envURL
	} else {
		// Use testcontainer - get the pool first to ensure container is started
		pool := GetTestContainer(t)
		// Get the connection string from the container
		if containerInstance != nil {
			dbURL = containerInstance.ConnStr
		} else {
			// Fallback: construct from pool config
			config := pool.Config().ConnConfig
			dbURL = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
				config.User, config.Password, config.Host, config.Port, config.Database)
		}
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
