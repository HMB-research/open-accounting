package invoicing

import (
	"context"
	"testing"
	"time"
)

type mockReminderRuleRepo struct {
	rules            []ReminderRule
	invoices         []InvoiceForReminder
	sentReminders    map[string]bool
	recordedReminder *PaymentReminder
}

func (m *mockReminderRuleRepo) ListRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
	return m.rules, nil
}

func (m *mockReminderRuleRepo) ListActiveRules(ctx context.Context, schemaName, tenantID string) ([]ReminderRule, error) {
	var active []ReminderRule
	for _, r := range m.rules {
		if r.IsActive {
			active = append(active, r)
		}
	}
	return active, nil
}

func (m *mockReminderRuleRepo) GetRule(ctx context.Context, schemaName, tenantID, ruleID string) (*ReminderRule, error) {
	for _, r := range m.rules {
		if r.ID == ruleID {
			return &r, nil
		}
	}
	return nil, ErrRuleNotFound
}

func (m *mockReminderRuleRepo) CreateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
	m.rules = append(m.rules, *rule)
	return nil
}

func (m *mockReminderRuleRepo) UpdateRule(ctx context.Context, schemaName string, rule *ReminderRule) error {
	for i, r := range m.rules {
		if r.ID == rule.ID {
			m.rules[i] = *rule
			return nil
		}
	}
	return ErrRuleNotFound
}

func (m *mockReminderRuleRepo) DeleteRule(ctx context.Context, schemaName, tenantID, ruleID string) error {
	for i, r := range m.rules {
		if r.ID == ruleID {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			return nil
		}
	}
	return ErrRuleNotFound
}

func (m *mockReminderRuleRepo) GetInvoicesForRule(ctx context.Context, schemaName, tenantID string, rule *ReminderRule, asOfDate time.Time) ([]InvoiceForReminder, error) {
	return m.invoices, nil
}

func (m *mockReminderRuleRepo) HasReminderBeenSent(ctx context.Context, schemaName, tenantID, invoiceID, ruleID string) (bool, error) {
	key := invoiceID + ":" + ruleID
	return m.sentReminders[key], nil
}

func (m *mockReminderRuleRepo) RecordReminderSent(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	m.recordedReminder = reminder
	if m.sentReminders == nil {
		m.sentReminders = make(map[string]bool)
	}
	if reminder.RuleID != nil {
		m.sentReminders[reminder.InvoiceID+":"+*reminder.RuleID] = true
	}
	return nil
}

func TestCreateReminderRule(t *testing.T) {
	repo := &mockReminderRuleRepo{}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	req := &CreateReminderRuleRequest{
		Name:        "7 Days Overdue",
		TriggerType: TriggerAfterDue,
		DaysOffset:  7,
		IsActive:    true,
	}

	rule, err := service.CreateRule(context.Background(), "tenant-1", "tenant_abc", req)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}

	if rule.Name != "7 Days Overdue" {
		t.Errorf("Expected name '7 Days Overdue', got '%s'", rule.Name)
	}
	if rule.TriggerType != TriggerAfterDue {
		t.Errorf("Expected trigger type AFTER_DUE, got '%s'", rule.TriggerType)
	}
	if rule.EmailTemplateType != "OVERDUE_REMINDER" {
		t.Errorf("Expected template OVERDUE_REMINDER, got '%s'", rule.EmailTemplateType)
	}
}

func TestCreateReminderRuleValidation(t *testing.T) {
	repo := &mockReminderRuleRepo{}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	tests := []struct {
		name    string
		req     *CreateReminderRuleRequest
		wantErr bool
	}{
		{
			name: "missing name",
			req: &CreateReminderRuleRequest{
				TriggerType: TriggerAfterDue,
				DaysOffset:  7,
			},
			wantErr: true,
		},
		{
			name: "missing trigger type",
			req: &CreateReminderRuleRequest{
				Name:       "Test",
				DaysOffset: 7,
			},
			wantErr: true,
		},
		{
			name: "negative days offset",
			req: &CreateReminderRuleRequest{
				Name:        "Test",
				TriggerType: TriggerAfterDue,
				DaysOffset:  -1,
			},
			wantErr: true,
		},
		{
			name: "valid request",
			req: &CreateReminderRuleRequest{
				Name:        "Test",
				TriggerType: TriggerAfterDue,
				DaysOffset:  7,
				IsActive:    true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.CreateRule(context.Background(), "tenant-1", "tenant_abc", tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListRules(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "1", Name: "Rule 1", IsActive: true},
			{ID: "2", Name: "Rule 2", IsActive: false},
		},
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	rules, err := service.ListRules(context.Background(), "tenant-1", "tenant_abc")
	if err != nil {
		t.Fatalf("ListRules failed: %v", err)
	}

	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}
}

func TestDefaultTemplateAssignment(t *testing.T) {
	repo := &mockReminderRuleRepo{}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	tests := []struct {
		triggerType      TriggerType
		expectedTemplate string
	}{
		{TriggerBeforeDue, "PAYMENT_DUE_SOON"},
		{TriggerOnDue, "PAYMENT_DUE_TODAY"},
		{TriggerAfterDue, "OVERDUE_REMINDER"},
	}

	for _, tt := range tests {
		t.Run(string(tt.triggerType), func(t *testing.T) {
			req := &CreateReminderRuleRequest{
				Name:        "Test",
				TriggerType: tt.triggerType,
				DaysOffset:  7,
				IsActive:    true,
			}

			rule, err := service.CreateRule(context.Background(), "tenant-1", "tenant_abc", req)
			if err != nil {
				t.Fatalf("CreateRule failed: %v", err)
			}

			if rule.EmailTemplateType != tt.expectedTemplate {
				t.Errorf("Expected template '%s', got '%s'", tt.expectedTemplate, rule.EmailTemplateType)
			}
		})
	}
}

func TestGetRule(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "rule-1", TenantID: "tenant-1", Name: "Test Rule"},
		},
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	rule, err := service.GetRule(context.Background(), "tenant-1", "tenant_abc", "rule-1")
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if rule.Name != "Test Rule" {
		t.Errorf("Expected name 'Test Rule', got '%s'", rule.Name)
	}

	_, err = service.GetRule(context.Background(), "tenant-1", "tenant_abc", "nonexistent")
	if err != ErrRuleNotFound {
		t.Errorf("Expected ErrRuleNotFound, got %v", err)
	}
}

func TestUpdateRule(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "rule-1", TenantID: "tenant-1", Name: "Original Name", IsActive: true},
		},
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	newName := "Updated Name"
	newActive := false
	updated, err := service.UpdateRule(context.Background(), "tenant-1", "tenant_abc", "rule-1", &UpdateReminderRuleRequest{
		Name:     &newName,
		IsActive: &newActive,
	})
	if err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("Expected name 'Updated Name', got '%s'", updated.Name)
	}
	if updated.IsActive {
		t.Errorf("Expected IsActive false, got true")
	}
}

func TestDeleteRule(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "rule-1", TenantID: "tenant-1", Name: "To Delete"},
		},
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	err := service.DeleteRule(context.Background(), "tenant-1", "tenant_abc", "rule-1")
	if err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	if len(repo.rules) != 0 {
		t.Errorf("Expected 0 rules after deletion, got %d", len(repo.rules))
	}

	err = service.DeleteRule(context.Background(), "tenant-1", "tenant_abc", "nonexistent")
	if err != ErrRuleNotFound {
		t.Errorf("Expected ErrRuleNotFound, got %v", err)
	}
}

func TestProcessRemindersForTenant_NoActiveRules(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "rule-1", Name: "Inactive Rule", IsActive: false},
		},
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	results, err := service.ProcessRemindersForTenant(context.Background(), "tenant-1", "tenant_abc", "Test Company")
	if err != nil {
		t.Fatalf("ProcessRemindersForTenant failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results for inactive rules, got %d", len(results))
	}
}

func TestProcessRemindersForTenant_NoInvoices(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "rule-1", Name: "Active Rule", IsActive: true, TriggerType: TriggerAfterDue, DaysOffset: 7},
		},
		invoices: []InvoiceForReminder{}, // No invoices
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	results, err := service.ProcessRemindersForTenant(context.Background(), "tenant-1", "tenant_abc", "Test Company")
	if err != nil {
		t.Fatalf("ProcessRemindersForTenant failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].InvoicesFound != 0 {
		t.Errorf("Expected 0 invoices found, got %d", results[0].InvoicesFound)
	}
	if results[0].RemindersSent != 0 {
		t.Errorf("Expected 0 reminders sent, got %d", results[0].RemindersSent)
	}
}

func TestProcessRemindersForTenant_SkipsNoEmail(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "rule-1", Name: "Active Rule", IsActive: true, TriggerType: TriggerAfterDue, DaysOffset: 7},
		},
		invoices: []InvoiceForReminder{
			{ID: "inv-1", InvoiceNumber: "INV-001", ContactEmail: ""}, // No email
		},
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	results, err := service.ProcessRemindersForTenant(context.Background(), "tenant-1", "tenant_abc", "Test Company")
	if err != nil {
		t.Fatalf("ProcessRemindersForTenant failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].InvoicesFound != 1 {
		t.Errorf("Expected 1 invoice found, got %d", results[0].InvoicesFound)
	}
	if results[0].Skipped != 1 {
		t.Errorf("Expected 1 skipped (no email), got %d", results[0].Skipped)
	}
}

func TestProcessRemindersForTenant_SkipsAlreadySent(t *testing.T) {
	repo := &mockReminderRuleRepo{
		rules: []ReminderRule{
			{ID: "rule-1", Name: "Active Rule", IsActive: true, TriggerType: TriggerAfterDue, DaysOffset: 7},
		},
		invoices: []InvoiceForReminder{
			{ID: "inv-1", InvoiceNumber: "INV-001", ContactEmail: "test@example.com"},
		},
		sentReminders: map[string]bool{
			"inv-1:rule-1": true, // Already sent
		},
	}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	results, err := service.ProcessRemindersForTenant(context.Background(), "tenant-1", "tenant_abc", "Test Company")
	if err != nil {
		t.Fatalf("ProcessRemindersForTenant failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Skipped != 1 {
		t.Errorf("Expected 1 skipped (already sent), got %d", results[0].Skipped)
	}
}

func TestCustomTemplateAssignment(t *testing.T) {
	repo := &mockReminderRuleRepo{}
	service := NewAutomatedReminderServiceWithRepository(repo, nil)

	req := &CreateReminderRuleRequest{
		Name:              "Custom Template Rule",
		TriggerType:       TriggerAfterDue,
		DaysOffset:        7,
		EmailTemplateType: "CUSTOM_TEMPLATE",
		IsActive:          true,
	}

	rule, err := service.CreateRule(context.Background(), "tenant-1", "tenant_abc", req)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}

	if rule.EmailTemplateType != "CUSTOM_TEMPLATE" {
		t.Errorf("Expected custom template, got '%s'", rule.EmailTemplateType)
	}
}
