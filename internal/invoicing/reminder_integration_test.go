package invoicing

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/testutil"
	"github.com/google/uuid"
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

func TestReminderRepositoriesIntegration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	tenant := testutil.CreateTestTenant(t, pool)
	userID := testutil.CreateTestUser(t, pool, "reminders-integration@example.com")
	reminderRepo := NewReminderPostgresRepository(pool)
	ruleRepo := NewReminderRulePostgresRepository(pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `DELETE FROM `+tenant.SchemaName+`.reminder_rules WHERE tenant_id = $1`, tenant.ID); err != nil {
		t.Fatalf("failed to clear default reminder rules: %v", err)
	}

	contactID := uuid.New().String()
	if _, err := pool.Exec(ctx, `
		INSERT INTO `+tenant.SchemaName+`.contacts (id, tenant_id, contact_type, name, email, created_at, updated_at)
		VALUES ($1, $2, 'CUSTOMER', 'Reminder Contact', 'reminder@example.com', NOW(), NOW())
	`, contactID, tenant.ID); err != nil {
		t.Fatalf("failed to create contact: %v", err)
	}

	beforeDueInvoiceID := uuid.New().String()
	onDueInvoiceID := uuid.New().String()
	afterDueInvoiceID := uuid.New().String()
	paidInvoiceID := uuid.New().String()
	for _, invoice := range []struct {
		id         string
		number     string
		dueDate    string
		status     string
		amountPaid string
	}{
		{beforeDueInvoiceID, "INV-BEFORE", "2025-02-08", "SENT", "0"},
		{onDueInvoiceID, "INV-ON", "2025-02-05", "PARTIALLY_PAID", "10"},
		{afterDueInvoiceID, "INV-AFTER", "2025-02-02", "OVERDUE", "0"},
		{paidInvoiceID, "INV-PAID", "2025-02-02", "PAID", "120"},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO `+tenant.SchemaName+`.invoices
				(id, tenant_id, invoice_number, invoice_type, contact_id, issue_date, due_date, currency,
				 subtotal, vat_amount, total, amount_paid, status, created_by, created_at, updated_at)
			VALUES
				($1, $2, $3, 'SALES', $4, DATE '2025-01-20', $5, 'EUR', 100, 20, 120, $6, $7, $8, NOW(), NOW())
		`, invoice.id, tenant.ID, invoice.number, contactID, invoice.dueDate, invoice.amountPaid, invoice.status, userID); err != nil {
			t.Fatalf("failed to create invoice %s: %v", invoice.number, err)
		}
	}

	if overdueInvoices, err := reminderRepo.GetOverdueInvoices(ctx, tenant.SchemaName, tenant.ID, time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("GetOverdueInvoices failed: %v", err)
	} else if len(overdueInvoices) != 1 || overdueInvoices[0].ID != afterDueInvoiceID {
		t.Fatalf("expected only overdue invoice %s, got %+v", afterDueInvoiceID, overdueInvoices)
	}

	reminder := &PaymentReminder{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		InvoiceID:      afterDueInvoiceID,
		InvoiceNumber:  "INV-AFTER",
		ContactID:      contactID,
		ContactName:    "Reminder Contact",
		ContactEmail:   "reminder@example.com",
		TriggerType:    string(TriggerAfterDue),
		DaysOffset:     0,
		ReminderNumber: 1,
		Status:         ReminderStatusPending,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := reminderRepo.CreateReminder(ctx, tenant.SchemaName, reminder); err != nil {
		t.Fatalf("CreateReminder failed: %v", err)
	}

	if count, lastSentAt, err := reminderRepo.GetReminderCount(ctx, tenant.SchemaName, tenant.ID, afterDueInvoiceID); err != nil {
		t.Fatalf("GetReminderCount failed: %v", err)
	} else if count != 0 || lastSentAt != nil {
		t.Fatalf("expected no sent reminders yet, got count=%d lastSentAt=%v", count, lastSentAt)
	}

	sentAt := time.Date(2025, 2, 5, 15, 0, 0, 0, time.UTC)
	if err := reminderRepo.UpdateReminderStatus(ctx, tenant.SchemaName, reminder.ID, ReminderStatusSent, &sentAt, ""); err != nil {
		t.Fatalf("UpdateReminderStatus failed: %v", err)
	}

	if count, lastSentAt, err := reminderRepo.GetReminderCount(ctx, tenant.SchemaName, tenant.ID, afterDueInvoiceID); err != nil {
		t.Fatalf("GetReminderCount after send failed: %v", err)
	} else if count != 1 || lastSentAt == nil || !lastSentAt.Equal(sentAt) {
		t.Fatalf("expected one sent reminder at %v, got count=%d lastSentAt=%v", sentAt, count, lastSentAt)
	}

	reminders, err := reminderRepo.GetRemindersByInvoice(ctx, tenant.SchemaName, tenant.ID, afterDueInvoiceID)
	if err != nil {
		t.Fatalf("GetRemindersByInvoice failed: %v", err)
	}
	if len(reminders) != 1 || reminders[0].Status != ReminderStatusSent {
		t.Fatalf("expected one sent reminder, got %+v", reminders)
	}

	beforeRule := &ReminderRule{
		ID:                uuid.New().String(),
		TenantID:          tenant.ID,
		Name:              "Before due",
		TriggerType:       TriggerBeforeDue,
		DaysOffset:        3,
		EmailTemplateType: "OVERDUE_REMINDER",
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	onRule := &ReminderRule{
		ID:                uuid.New().String(),
		TenantID:          tenant.ID,
		Name:              "On due",
		TriggerType:       TriggerOnDue,
		DaysOffset:        0,
		EmailTemplateType: "OVERDUE_REMINDER",
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	afterRule := &ReminderRule{
		ID:                uuid.New().String(),
		TenantID:          tenant.ID,
		Name:              "After due",
		TriggerType:       TriggerAfterDue,
		DaysOffset:        3,
		EmailTemplateType: "OVERDUE_REMINDER",
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	for _, rule := range []*ReminderRule{beforeRule, onRule, afterRule} {
		if err := ruleRepo.CreateRule(ctx, tenant.SchemaName, rule); err != nil {
			t.Fatalf("CreateRule %s failed: %v", rule.Name, err)
		}
	}

	asOfDate := time.Date(2025, 2, 5, 0, 0, 0, 0, time.UTC)
	if invoices, err := ruleRepo.GetInvoicesForRule(ctx, tenant.SchemaName, tenant.ID, beforeRule, asOfDate); err != nil {
		t.Fatalf("GetInvoicesForRule before due failed: %v", err)
	} else if len(invoices) != 1 || invoices[0].ID != beforeDueInvoiceID || invoices[0].DaysUntilDue != 3 {
		t.Fatalf("unexpected before-due invoices: %+v", invoices)
	}

	if invoices, err := ruleRepo.GetInvoicesForRule(ctx, tenant.SchemaName, tenant.ID, onRule, asOfDate); err != nil {
		t.Fatalf("GetInvoicesForRule on due failed: %v", err)
	} else if len(invoices) != 1 || invoices[0].ID != onDueInvoiceID || invoices[0].DaysUntilDue != 0 {
		t.Fatalf("unexpected on-due invoices: %+v", invoices)
	}

	if invoices, err := ruleRepo.GetInvoicesForRule(ctx, tenant.SchemaName, tenant.ID, afterRule, asOfDate); err != nil {
		t.Fatalf("GetInvoicesForRule after due failed: %v", err)
	} else if len(invoices) != 1 || invoices[0].ID != afterDueInvoiceID || invoices[0].DaysOverdue != 3 {
		t.Fatalf("unexpected after-due invoices: %+v", invoices)
	}

	sentReminder := &PaymentReminder{
		ID:             uuid.New().String(),
		TenantID:       tenant.ID,
		InvoiceID:      afterDueInvoiceID,
		InvoiceNumber:  "INV-AFTER",
		ContactID:      contactID,
		ContactName:    "Reminder Contact",
		ContactEmail:   "reminder@example.com",
		RuleID:         &afterRule.ID,
		TriggerType:    string(afterRule.TriggerType),
		DaysOffset:     afterRule.DaysOffset,
		ReminderNumber: 2,
		Status:         ReminderStatusSent,
		SentAt:         &sentAt,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := ruleRepo.RecordReminderSent(ctx, tenant.SchemaName, sentReminder); err != nil {
		t.Fatalf("RecordReminderSent failed: %v", err)
	}

	alreadySent, err := ruleRepo.HasReminderBeenSent(ctx, tenant.SchemaName, tenant.ID, afterDueInvoiceID, afterRule.ID)
	if err != nil {
		t.Fatalf("HasReminderBeenSent failed: %v", err)
	}
	if !alreadySent {
		t.Fatal("expected reminder sent status to be tracked")
	}

	notSent, err := ruleRepo.HasReminderBeenSent(ctx, tenant.SchemaName, tenant.ID, beforeDueInvoiceID, beforeRule.ID)
	if err != nil {
		t.Fatalf("HasReminderBeenSent non-match failed: %v", err)
	}
	if notSent {
		t.Fatal("expected unmatched invoice/rule pair to be false")
	}

	history, err := reminderRepo.GetRemindersByInvoice(ctx, tenant.SchemaName, tenant.ID, afterDueInvoiceID)
	if err != nil {
		t.Fatalf("GetRemindersByInvoice history failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected both direct and rule-based reminders in history, got %d", len(history))
	}

	if history[0].Status != ReminderStatusSent && history[1].Status != ReminderStatusSent {
		t.Fatalf("expected sent reminders in history, got %+v", history)
	}
}
