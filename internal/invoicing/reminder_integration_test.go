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

	// First, create the reminder_rules table in the tenant schema (matching migration schema)
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS `+tenant.SchemaName+`.reminder_rules (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			name VARCHAR(255) NOT NULL,
			trigger_type VARCHAR(20) NOT NULL,
			days_offset INTEGER NOT NULL DEFAULT 0,
			email_template_type VARCHAR(50) NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create reminder_rules table: %v", err)
	}

	// Generate proper UUIDs for tests
	ruleID := uuid.New().String()
	nonexistentID := uuid.New().String()

	// Test 1: Create a rule
	t.Run("CreateRule", func(t *testing.T) {
		rule := &ReminderRule{
			ID:                ruleID,
			TenantID:          tenant.ID,
			Name:              "7 Days Overdue",
			TriggerType:       TriggerAfterDue,
			DaysOffset:        7,
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

		if rules[0].Name != "7 Days Overdue" {
			t.Errorf("Expected name '7 Days Overdue', got '%s'", rules[0].Name)
		}
	})

	// Test 3: Get single rule
	t.Run("GetRule", func(t *testing.T) {
		rule, err := repo.GetRule(ctx, tenant.SchemaName, tenant.ID, ruleID)
		if err != nil {
			t.Fatalf("GetRule failed: %v", err)
		}

		if rule.Name != "7 Days Overdue" {
			t.Errorf("Expected name '7 Days Overdue', got '%s'", rule.Name)
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
