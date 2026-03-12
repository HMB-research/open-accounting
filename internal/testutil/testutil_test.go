package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestDatabaseSetupPaths(t *testing.T) {
	pool := SetupTestDB(t)
	if pool == nil {
		t.Fatal("expected pgx pool")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping shared test db: %v", err)
	}

	if containerInstance == nil {
		t.Fatal("expected shared container instance to be initialized")
	}

	t.Setenv("DATABASE_URL", containerInstance.ConnStr)
	externalPool := GetTestContainer(t)
	if externalPool == nil {
		t.Fatal("expected external database pool")
	}
	if err := externalPool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping external-path db: %v", err)
	}

	gormDB := SetupGormDB(t)
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("failed to get sql DB from gorm: %v", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping gorm db: %v", err)
	}
}

func TestSchemaTenantAndUserHelpers(t *testing.T) {
	pool := SetupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	schemaName := SetupTestSchema(t, pool)

	var schemaExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, schemaName).Scan(&schemaExists); err != nil {
		t.Fatalf("failed to check test schema existence: %v", err)
	}
	if !schemaExists {
		t.Fatalf("expected schema %s to exist", schemaName)
	}

	TeardownTestSchema(t, pool, schemaName)

	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)`, schemaName).Scan(&schemaExists); err != nil {
		t.Fatalf("failed to check schema after teardown: %v", err)
	}
	if schemaExists {
		t.Fatalf("expected schema %s to be dropped", schemaName)
	}

	tenant := CreateTestTenant(t, pool)
	if tenant == nil {
		t.Fatal("expected test tenant")
	}

	var tenantName string
	if err := pool.QueryRow(ctx, `SELECT name FROM public.tenants WHERE id = $1`, tenant.ID).Scan(&tenantName); err != nil {
		t.Fatalf("failed to query tenant: %v", err)
	}
	if tenantName == "" {
		t.Fatal("expected tenant name")
	}

	accounts := GetTestAccounts(t, pool, tenant.SchemaName)
	if accounts.AssetAccountID == "" || accounts.DepreciationExpenseAccountID == "" || accounts.AccumulatedDepreciationAcctID == "" {
		t.Fatal("expected default chart of accounts IDs")
	}

	userID := CreateTestUser(t, pool, fmt.Sprintf("helper-%d@example.com", time.Now().UnixNano()))
	AddUserToTenant(t, pool, tenant.ID, userID, "owner")

	var role string
	if err := pool.QueryRow(ctx, `SELECT role FROM public.tenant_users WHERE tenant_id = $1 AND user_id = $2`, tenant.ID, userID).Scan(&role); err != nil {
		t.Fatalf("failed to query tenant user role: %v", err)
	}
	if role != "owner" {
		t.Fatalf("expected owner role, got %q", role)
	}

	cleanupTestUser(t, pool, userID)

	var userExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.users WHERE id = $1)`, userID).Scan(&userExists); err != nil {
		t.Fatalf("failed to check cleaned up user: %v", err)
	}
	if userExists {
		t.Fatalf("expected user %s to be removed", userID)
	}

	cleanupTestTenant(t, pool, tenant)

	var tenantExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM public.tenants WHERE id = $1)`, tenant.ID).Scan(&tenantExists); err != nil {
		t.Fatalf("failed to check cleaned up tenant: %v", err)
	}
	if tenantExists {
		t.Fatalf("expected tenant %s to be removed", tenant.ID)
	}
}

func TestLifecycleConnectionHelpers(t *testing.T) {
	pool := SetupTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := acquireSchemaLifecycleConn(ctx, pool)
	if err != nil {
		t.Fatalf("failed to acquire schema lifecycle connection: %v", err)
	}

	var advisoryLockHeld bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1
		FROM pg_locks
		WHERE locktype = 'advisory'
		  AND objid = $1
	)`, schemaLifecycleAdvisoryLockKey).Scan(&advisoryLockHeld); err != nil {
		t.Fatalf("failed to inspect advisory lock: %v", err)
	}
	if !advisoryLockHeld {
		t.Fatal("expected advisory lock to be held")
	}

	releaseSchemaLifecycleConn(ctx, conn)
}
