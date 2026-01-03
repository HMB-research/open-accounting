//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestDemoSeedSQL_ValidSchema(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()

	// Test with demo user 1 (the UUIDs get modified by generateDemoSeedForUser)
	// The replacement changes "a0000000-0000-0000-0000-" to "a0000000-0000-0000-0001-" for user 1
	// So original "a0000000-0000-0000-0000-000000000001" becomes "a0000000-0000-0000-0001-000000000001"
	demoUserID := "a0000000-0000-0000-0001-000000000001"
	demoTenantID := "b0000000-0000-0000-0001-000000000001"
	schemaName := "tenant_demo1"

	// Drop tenant schema if exists
	_, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	if err != nil {
		t.Fatalf("Failed to drop tenant schema: %v", err)
	}

	// Delete demo data from public tables
	_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id = $1", demoTenantID)
	_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", demoTenantID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", demoUserID)

	// Execute the seed SQL for user 1
	seedSQL := generateDemoSeedForUser(getDemoSeedTemplate(), 1)
	_, err = pool.Exec(ctx, seedSQL)
	if err != nil {
		t.Fatalf("Demo seed SQL failed: %v", err)
	}

	// Verify user was created
	var userCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE id = $1", demoUserID).Scan(&userCount)
	if err != nil {
		t.Fatalf("Failed to query users: %v", err)
	}
	if userCount != 1 {
		t.Errorf("Expected 1 demo user, got %d", userCount)
	}

	// Verify tenant was created
	var tenantCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenants WHERE id = $1", demoTenantID).Scan(&tenantCount)
	if err != nil {
		t.Fatalf("Failed to query tenants: %v", err)
	}
	if tenantCount != 1 {
		t.Errorf("Expected 1 demo tenant, got %d", tenantCount)
	}

	// Verify tenant_users link
	var linkCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM tenant_users WHERE tenant_id = $1 AND user_id = $2",
		demoTenantID, demoUserID).Scan(&linkCount)
	if err != nil {
		t.Fatalf("Failed to query tenant_users: %v", err)
	}
	if linkCount != 1 {
		t.Errorf("Expected 1 tenant_user link, got %d", linkCount)
	}

	// Verify tenant schema was created
	var schemaExists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", schemaName).Scan(&schemaExists)
	if err != nil {
		t.Fatalf("Failed to check schema: %v", err)
	}
	if !schemaExists {
		t.Errorf("Expected %s schema to exist", schemaName)
	}

	// Verify accounts were created
	var accountCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+schemaName+".accounts").Scan(&accountCount)
	if err != nil {
		t.Fatalf("Failed to query accounts: %v", err)
	}
	if accountCount < 10 {
		t.Errorf("Expected at least 10 accounts, got %d", accountCount)
	}

	// Verify contacts were created
	var contactCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+schemaName+".contacts").Scan(&contactCount)
	if err != nil {
		t.Fatalf("Failed to query contacts: %v", err)
	}
	if contactCount < 3 {
		t.Errorf("Expected at least 3 contacts, got %d", contactCount)
	}

	// Verify invoices were created
	var invoiceCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+schemaName+".invoices").Scan(&invoiceCount)
	if err != nil {
		t.Fatalf("Failed to query invoices: %v", err)
	}
	if invoiceCount < 2 {
		t.Errorf("Expected at least 2 invoices, got %d", invoiceCount)
	}

	// Verify bank accounts were created
	var bankAccountCount int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+schemaName+".bank_accounts").Scan(&bankAccountCount)
	if err != nil {
		t.Fatalf("Failed to query bank_accounts: %v", err)
	}
	if bankAccountCount < 1 {
		t.Errorf("Expected at least 1 bank account, got %d", bankAccountCount)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
	_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id = $1", demoTenantID)
	_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE id = $1", demoTenantID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", demoUserID)

	t.Log("Demo seed SQL validation passed")
}
