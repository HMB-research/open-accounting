//go:build integration

package invoicing

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/HMB-research/open-accounting/internal/testutil"
)

func TestReminderRuleIntegration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)

	repo := NewReminderRulePostgresRepository(pool)
	ctx := context.Background()

	// The reminder_rules table is created by the migration via create_tenant_schema.
	// Clear any existing rules to ensure test isolation (migration inserts default rules).
	_, err := pool.Exec(ctx, `DELETE FROM `+tenant.SchemaName+`.reminder_rules WHERE tenant_id = $1`, tenant.ID)
	if err != nil {
		t.Fatalf("Failed to clear existing reminder rules: %v", err)
	}

	// Generate proper UUIDs for tests
	ruleID := uuid.New().String()
	nonexistentID := uuid.New().String()

	// Use a unique days_offset (99) that won't conflict with any default rules
	testDaysOffset := 99

	// Test 1: Create a rule
	t.Run("CreateRule", func(t *testing.T) {
		rule := &ReminderRule{
			ID:                ruleID,
			TenantID:          tenant.ID,
			Name:              "Test Rule - 99 Days Overdue",
			TriggerType:       TriggerAfterDue,
			DaysOffset:        testDaysOffset,
			EmailTemplateType: "OVERDUE_REMINDER",
			IsActive:          true,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}

		err := repo.CreateRule(ctx, tenant.SchemaName, rule)
		if err != nil {
			t.Fatalf("CreateRule failed: %v", err)
		}
	})

	// Test 2: List rules
	t.Run("ListRules", func(t *testing.T) {
		rules, err := repo.ListRules(ctx, tenant.SchemaName, tenant.ID)
		if err != nil {
			t.Fatalf("ListRules failed: %v", err)
		}

		if len(rules) != 1 {
			t.Errorf("Expected 1 rule, got %d", len(rules))
		}

		if rules[0].Name != "Test Rule - 99 Days Overdue" {
			t.Errorf("Expected name 'Test Rule - 99 Days Overdue', got '%s'", rules[0].Name)
		}
	})

	// Test 3: Get single rule
	t.Run("GetRule", func(t *testing.T) {
		rule, err := repo.GetRule(ctx, tenant.SchemaName, tenant.ID, ruleID)
		if err != nil {
			t.Fatalf("GetRule failed: %v", err)
		}

		if rule.Name != "Test Rule - 99 Days Overdue" {
			t.Errorf("Expected name 'Test Rule - 99 Days Overdue', got '%s'", rule.Name)
		}
		if rule.TriggerType != TriggerAfterDue {
			t.Errorf("Expected trigger type AFTER_DUE, got '%s'", rule.TriggerType)
		}
	})

	// Test 4: Update rule
	t.Run("UpdateRule", func(t *testing.T) {
		rule := &ReminderRule{
			ID:                ruleID,
			TenantID:          tenant.ID,
			Name:              "Updated Name",
			EmailTemplateType: "CUSTOM_TEMPLATE",
			IsActive:          false,
		}

		err := repo.UpdateRule(ctx, tenant.SchemaName, rule)
		if err != nil {
			t.Fatalf("UpdateRule failed: %v", err)
		}

		// Verify update
		updated, err := repo.GetRule(ctx, tenant.SchemaName, tenant.ID, ruleID)
		if err != nil {
			t.Fatalf("GetRule failed: %v", err)
		}

		if updated.Name != "Updated Name" {
			t.Errorf("Expected name 'Updated Name', got '%s'", updated.Name)
		}
		if updated.IsActive {
			t.Error("Expected IsActive to be false")
		}
	})

	// Test 5: List active rules only
	t.Run("ListActiveRules", func(t *testing.T) {
		// Rule was deactivated in previous test
		rules, err := repo.ListActiveRules(ctx, tenant.SchemaName, tenant.ID)
		if err != nil {
			t.Fatalf("ListActiveRules failed: %v", err)
		}

		if len(rules) != 0 {
			t.Errorf("Expected 0 active rules, got %d", len(rules))
		}
	})

	// Test 6: Delete rule
	t.Run("DeleteRule", func(t *testing.T) {
		err := repo.DeleteRule(ctx, tenant.SchemaName, tenant.ID, ruleID)
		if err != nil {
			t.Fatalf("DeleteRule failed: %v", err)
		}

		// Verify deletion
		rules, _ := repo.ListRules(ctx, tenant.SchemaName, tenant.ID)
		if len(rules) != 0 {
			t.Errorf("Expected 0 rules after deletion, got %d", len(rules))
		}
	})

	// Test 7: Get non-existent rule
	t.Run("GetNonExistentRule", func(t *testing.T) {
		_, err := repo.GetRule(ctx, tenant.SchemaName, tenant.ID, nonexistentID)
		if err != ErrRuleNotFound {
			t.Errorf("Expected ErrRuleNotFound, got %v", err)
		}
	})

	// Test 8: Delete non-existent rule
	t.Run("DeleteNonExistentRule", func(t *testing.T) {
		err := repo.DeleteRule(ctx, tenant.SchemaName, tenant.ID, nonexistentID)
		if err != ErrRuleNotFound {
			t.Errorf("Expected ErrRuleNotFound, got %v", err)
		}
	})
}
