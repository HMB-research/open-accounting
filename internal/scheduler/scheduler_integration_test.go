//go:build integration

package scheduler

import (
	"context"
	"testing"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestPostgresRepository_ListActiveTenants(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create test tenants
	tenant1 := testutil.CreateTestTenant(t, pool)
	tenant2 := testutil.CreateTestTenant(t, pool)

	// List active tenants
	tenants, err := repo.ListActiveTenants(ctx)
	if err != nil {
		t.Fatalf("ListActiveTenants failed: %v", err)
	}

	// Should include our created tenants
	found1, found2 := false, false
	for _, tenant := range tenants {
		if tenant.ID == tenant1.ID {
			found1 = true
			if tenant.SchemaName != tenant1.SchemaName {
				t.Errorf("expected schema %s, got %s", tenant1.SchemaName, tenant.SchemaName)
			}
		}
		if tenant.ID == tenant2.ID {
			found2 = true
		}
	}

	if !found1 {
		t.Error("tenant1 not found in active tenants list")
	}
	if !found2 {
		t.Error("tenant2 not found in active tenants list")
	}
}

func TestPostgresRepository_ListActiveTenants_ExcludesInactive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	ctx := context.Background()

	// Create an active tenant
	activeTenant := testutil.CreateTestTenant(t, pool)

	// Deactivate it
	_, err := pool.Exec(ctx, "UPDATE tenants SET is_active = false WHERE id = $1", activeTenant.ID)
	if err != nil {
		t.Fatalf("failed to deactivate tenant: %v", err)
	}

	// List active tenants
	tenants, err := repo.ListActiveTenants(ctx)
	if err != nil {
		t.Fatalf("ListActiveTenants failed: %v", err)
	}

	// Should not include our deactivated tenant
	for _, tenant := range tenants {
		if tenant.ID == activeTenant.ID {
			t.Error("inactive tenant should not be in active tenants list")
		}
	}
}

func TestScheduler_WithRealRepository(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	repo := NewPostgresRepository(pool)
	config := DefaultConfig()

	// Create scheduler with real repository
	scheduler := NewSchedulerWithRepository(repo, nil, config)

	// Should not be running initially
	if scheduler.IsRunning() {
		t.Error("scheduler should not be running initially")
	}

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("scheduler should be running after Start")
	}

	// Stop scheduler
	ctx := scheduler.Stop()
	if ctx == nil {
		t.Error("Stop returned nil context")
	}

	if scheduler.IsRunning() {
		t.Error("scheduler should not be running after Stop")
	}
}
