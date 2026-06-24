package invoicing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
)

func TestReminderGORMRepositoryNilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	invoiceID := "invoice-1"
	asOf := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)

	repo := NewReminderGORMRepository(nil)
	if repo == nil || repo.db != nil {
		t.Fatalf("NewReminderGORMRepository(nil) = %#v, want repository with nil database", repo)
	}

	constructed := NewReminderRepository(nil)
	if constructed == nil || constructed.db != nil {
		t.Fatalf("NewReminderRepository(nil) = %#v, want repository with nil database", constructed)
	}

	var nilRepo *ReminderGORMRepository
	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "nil receiver dbWithContext",
			run: func(t *testing.T) error {
				db, err := nilRepo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "dbWithContext",
			run: func(t *testing.T) error {
				db, err := repo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "payment_reminders")
				if table != nil {
					t.Fatalf("tenantTable() table = %#v, want nil", table)
				}
				return err
			},
		},
		{
			name: "GetOverdueInvoices",
			run: func(t *testing.T) error {
				invoices, err := repo.GetOverdueInvoices(ctx, schemaName, tenantID, asOf)
				if invoices != nil {
					t.Fatalf("GetOverdueInvoices() invoices = %#v, want nil", invoices)
				}
				return err
			},
		},
		{
			name: "GetReminderCount",
			run: func(t *testing.T) error {
				count, lastSentAt, err := repo.GetReminderCount(ctx, schemaName, tenantID, invoiceID)
				if count != 0 || lastSentAt != nil {
					t.Fatalf("GetReminderCount() = (%d, %#v), want zero values", count, lastSentAt)
				}
				return err
			},
		},
		{
			name: "CreateReminder",
			run: func(t *testing.T) error {
				return repo.CreateReminder(ctx, schemaName, &PaymentReminder{ID: "reminder-1", TenantID: tenantID, InvoiceID: invoiceID})
			},
		},
		{
			name: "UpdateReminderStatus",
			run: func(t *testing.T) error {
				return repo.UpdateReminderStatus(ctx, schemaName, "reminder-1", ReminderStatusSent, &asOf, "smtp error")
			},
		},
		{
			name: "GetRemindersByInvoice",
			run: func(t *testing.T) error {
				reminders, err := repo.GetRemindersByInvoice(ctx, schemaName, tenantID, invoiceID)
				if reminders != nil {
					t.Fatalf("GetRemindersByInvoice() reminders = %#v, want nil", reminders)
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrorContains(t, tt.run(t), "reminder repository database is not configured")
		})
	}
}

func TestPaymentReminderModelMappings(t *testing.T) {
	ruleID := "rule-1"
	sentAt := time.Date(2026, 6, 25, 9, 30, 0, 0, time.UTC)
	createdAt := sentAt.Add(-time.Hour)
	updatedAt := sentAt.Add(time.Minute)
	reminder := &PaymentReminder{
		ID:             "reminder-1",
		TenantID:       "tenant-1",
		InvoiceID:      "invoice-1",
		InvoiceNumber:  "INV-001",
		ContactID:      "contact-1",
		ContactName:    "Customer",
		ContactEmail:   "customer@example.com",
		RuleID:         &ruleID,
		TriggerType:    string(TriggerAfterDue),
		DaysOffset:     7,
		ReminderNumber: 2,
		Status:         ReminderStatusFailed,
		SentAt:         &sentAt,
		ErrorMessage:   "smtp error",
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}

	model := paymentReminderToModel(reminder)
	if model.ID != reminder.ID || model.TenantID != reminder.TenantID || model.InvoiceID != reminder.InvoiceID {
		t.Fatalf("paymentReminderToModel() identifiers = %#v, want reminder identifiers", model)
	}
	if model.InvoiceNumber != reminder.InvoiceNumber || model.ContactID != reminder.ContactID || model.ContactName != reminder.ContactName {
		t.Fatalf("paymentReminderToModel() invoice/contact fields = %#v, want reminder fields", model)
	}
	if model.ContactEmail == nil || *model.ContactEmail != reminder.ContactEmail {
		t.Fatalf("paymentReminderToModel() ContactEmail = %#v, want %q", model.ContactEmail, reminder.ContactEmail)
	}
	if model.RuleID == nil || *model.RuleID != ruleID {
		t.Fatalf("paymentReminderToModel() RuleID = %#v, want %q", model.RuleID, ruleID)
	}
	if model.TriggerType != reminder.TriggerType || model.DaysOffset != reminder.DaysOffset || model.ReminderNumber != reminder.ReminderNumber {
		t.Fatalf("paymentReminderToModel() trigger fields = %#v, want reminder fields", model)
	}
	if model.Status != string(reminder.Status) || model.SentAt != reminder.SentAt {
		t.Fatalf("paymentReminderToModel() status fields = %#v, want reminder fields", model)
	}
	if model.ErrorMessage == nil || *model.ErrorMessage != reminder.ErrorMessage {
		t.Fatalf("paymentReminderToModel() ErrorMessage = %#v, want %q", model.ErrorMessage, reminder.ErrorMessage)
	}
	if !model.CreatedAt.Equal(createdAt) || !model.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("paymentReminderToModel() timestamps = (%v, %v), want (%v, %v)", model.CreatedAt, model.UpdatedAt, createdAt, updatedAt)
	}

	roundTrip := paymentReminderFromModel(model)
	if roundTrip.ID != reminder.ID || roundTrip.TenantID != reminder.TenantID || roundTrip.InvoiceID != reminder.InvoiceID {
		t.Fatalf("paymentReminderFromModel() identifiers = %#v, want reminder identifiers", roundTrip)
	}
	if roundTrip.ContactEmail != reminder.ContactEmail || roundTrip.ErrorMessage != reminder.ErrorMessage {
		t.Fatalf("paymentReminderFromModel() optional strings = (%q, %q), want (%q, %q)", roundTrip.ContactEmail, roundTrip.ErrorMessage, reminder.ContactEmail, reminder.ErrorMessage)
	}
	if roundTrip.Status != reminder.Status || roundTrip.TriggerType != reminder.TriggerType || roundTrip.DaysOffset != reminder.DaysOffset {
		t.Fatalf("paymentReminderFromModel() status/trigger fields = %#v, want reminder fields", roundTrip)
	}

	emptyModel := paymentReminderToModel(&PaymentReminder{})
	if emptyModel.ContactEmail != nil || emptyModel.ErrorMessage != nil {
		t.Fatalf("paymentReminderToModel() empty optional strings = (%#v, %#v), want nils", emptyModel.ContactEmail, emptyModel.ErrorMessage)
	}

	if nilIfEmpty("") != nil {
		t.Fatal("nilIfEmpty(\"\") returned non-nil pointer")
	}
	value := nilIfEmpty("value")
	if value == nil || *value != "value" {
		t.Fatalf("nilIfEmpty(\"value\") = %#v, want pointer to value", value)
	}
	if got := valueOrEmpty(nil); got != "" {
		t.Fatalf("valueOrEmpty(nil) = %q, want empty string", got)
	}
	if got := valueOrEmpty(value); got != "value" {
		t.Fatalf("valueOrEmpty(value) = %q, want value", got)
	}
}

func TestReminderRuleGORMRepositoryNilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	ruleID := "rule-1"
	invoiceID := "invoice-1"
	asOf := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	rule := &ReminderRule{
		ID:                ruleID,
		TenantID:          tenantID,
		Name:              "After due",
		TriggerType:       TriggerAfterDue,
		DaysOffset:        7,
		EmailTemplateType: "OVERDUE_REMINDER",
		IsActive:          true,
	}

	repo := NewReminderRuleGORMRepository(nil)
	if repo == nil || repo.db != nil {
		t.Fatalf("NewReminderRuleGORMRepository(nil) = %#v, want repository with nil database", repo)
	}

	constructed := NewReminderRuleRepository(nil)
	if constructed == nil || constructed.db != nil {
		t.Fatalf("NewReminderRuleRepository(nil) = %#v, want repository with nil database", constructed)
	}

	var nilRepo *ReminderRuleGORMRepository
	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "nil receiver dbWithContext",
			run: func(t *testing.T) error {
				db, err := nilRepo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "dbWithContext",
			run: func(t *testing.T) error {
				db, err := repo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "reminder_rules")
				if table != nil {
					t.Fatalf("tenantTable() table = %#v, want nil", table)
				}
				return err
			},
		},
		{
			name: "ListRules",
			run: func(t *testing.T) error {
				rules, err := repo.ListRules(ctx, schemaName, tenantID)
				if rules != nil {
					t.Fatalf("ListRules() rules = %#v, want nil", rules)
				}
				return err
			},
		},
		{
			name: "ListActiveRules",
			run: func(t *testing.T) error {
				rules, err := repo.ListActiveRules(ctx, schemaName, tenantID)
				if rules != nil {
					t.Fatalf("ListActiveRules() rules = %#v, want nil", rules)
				}
				return err
			},
		},
		{
			name: "GetRule",
			run: func(t *testing.T) error {
				gotRule, err := repo.GetRule(ctx, schemaName, tenantID, ruleID)
				if gotRule != nil {
					t.Fatalf("GetRule() rule = %#v, want nil", gotRule)
				}
				return err
			},
		},
		{
			name: "CreateRule",
			run: func(t *testing.T) error {
				return repo.CreateRule(ctx, schemaName, rule)
			},
		},
		{
			name: "UpdateRule",
			run: func(t *testing.T) error {
				return repo.UpdateRule(ctx, schemaName, rule)
			},
		},
		{
			name: "DeleteRule",
			run: func(t *testing.T) error {
				return repo.DeleteRule(ctx, schemaName, tenantID, ruleID)
			},
		},
		{
			name: "GetInvoicesForRule",
			run: func(t *testing.T) error {
				invoices, err := repo.GetInvoicesForRule(ctx, schemaName, tenantID, rule, asOf)
				if invoices != nil {
					t.Fatalf("GetInvoicesForRule() invoices = %#v, want nil", invoices)
				}
				return err
			},
		},
		{
			name: "HasReminderBeenSent",
			run: func(t *testing.T) error {
				sent, err := repo.HasReminderBeenSent(ctx, schemaName, tenantID, invoiceID, ruleID)
				if sent {
					t.Fatal("HasReminderBeenSent() sent = true, want false")
				}
				return err
			},
		},
		{
			name: "RecordReminderSent",
			run: func(t *testing.T) error {
				return repo.RecordReminderSent(ctx, schemaName, &PaymentReminder{ID: "reminder-1", TenantID: tenantID, InvoiceID: invoiceID, RuleID: &ruleID})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrorContains(t, tt.run(t), "reminder rule repository database is not configured")
		})
	}
}

func TestReminderRuleGetInvoicesForRuleInvalidTriggerBeforeDatabase(t *testing.T) {
	var repo *ReminderRuleGORMRepository
	_, err := repo.GetInvoicesForRule(context.Background(), "tenant-schema", "tenant-1", &ReminderRule{
		TriggerType: TriggerType("NEVER"),
	}, time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrInvalidTriggerType) {
		t.Fatalf("GetInvoicesForRule() error = %v, want ErrInvalidTriggerType", err)
	}
	if strings.Contains(err.Error(), "database") {
		t.Fatalf("GetInvoicesForRule() touched database before trigger validation: %v", err)
	}
}

func TestReminderRuleModelMappings(t *testing.T) {
	createdAt := time.Date(2026, 6, 25, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	rule := &ReminderRule{
		ID:                "rule-1",
		TenantID:          "tenant-1",
		Name:              "Before due",
		TriggerType:       TriggerBeforeDue,
		DaysOffset:        3,
		EmailTemplateType: "PAYMENT_REMINDER",
		IsActive:          true,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}

	model := reminderRuleToModel(rule)
	if model.ID != rule.ID || model.TenantID != rule.TenantID || model.Name != rule.Name {
		t.Fatalf("reminderRuleToModel() identifiers = %#v, want rule fields", model)
	}
	if model.TriggerType != string(rule.TriggerType) || model.DaysOffset != rule.DaysOffset || model.EmailTemplateType != rule.EmailTemplateType {
		t.Fatalf("reminderRuleToModel() trigger/template fields = %#v, want rule fields", model)
	}
	if model.IsActive != rule.IsActive || !model.CreatedAt.Equal(createdAt) || !model.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("reminderRuleToModel() state fields = %#v, want rule fields", model)
	}

	roundTrip := reminderRuleFromModel(model)
	if roundTrip.ID != rule.ID || roundTrip.TenantID != rule.TenantID || roundTrip.Name != rule.Name {
		t.Fatalf("reminderRuleFromModel() identifiers = %#v, want rule fields", roundTrip)
	}
	if roundTrip.TriggerType != rule.TriggerType || roundTrip.DaysOffset != rule.DaysOffset || roundTrip.EmailTemplateType != rule.EmailTemplateType {
		t.Fatalf("reminderRuleFromModel() trigger/template fields = %#v, want rule fields", roundTrip)
	}
	if roundTrip.IsActive != rule.IsActive || !roundTrip.CreatedAt.Equal(createdAt) || !roundTrip.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("reminderRuleFromModel() state fields = %#v, want rule fields", roundTrip)
	}

	rules := reminderRulesFromModels([]models.ReminderRule{
		*model,
		{
			ID:                "rule-2",
			TenantID:          "tenant-1",
			Name:              "On due",
			TriggerType:       string(TriggerOnDue),
			DaysOffset:        0,
			EmailTemplateType: "DUE_REMINDER",
			IsActive:          false,
			CreatedAt:         createdAt.Add(time.Minute),
			UpdatedAt:         updatedAt.Add(time.Minute),
		},
	})
	if len(rules) != 2 {
		t.Fatalf("reminderRulesFromModels() len = %d, want 2", len(rules))
	}
	if rules[0].ID != "rule-1" || rules[0].TriggerType != TriggerBeforeDue || !rules[0].IsActive {
		t.Fatalf("reminderRulesFromModels()[0] = %#v, want first rule", rules[0])
	}
	if rules[1].ID != "rule-2" || rules[1].TriggerType != TriggerOnDue || rules[1].IsActive {
		t.Fatalf("reminderRulesFromModels()[1] = %#v, want second rule", rules[1])
	}
}

func TestInterestGORMRepositoryNilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_schema"
	tenantID := "tenant-1"
	invoiceID := "invoice-1"
	asOf := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)

	repo := NewInterestGORMRepository(nil)
	if repo == nil || repo.db != nil {
		t.Fatalf("NewInterestGORMRepository(nil) = %#v, want repository with nil database", repo)
	}

	constructed := NewInterestRepository(nil)
	if constructed == nil || constructed.db != nil {
		t.Fatalf("NewInterestRepository(nil) = %#v, want repository with nil database", constructed)
	}

	var nilRepo *InterestGORMRepository
	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "nil receiver dbWithContext",
			run: func(t *testing.T) error {
				db, err := nilRepo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "dbWithContext",
			run: func(t *testing.T) error {
				db, err := repo.dbWithContext(ctx)
				if db != nil {
					t.Fatalf("dbWithContext() db = %#v, want nil", db)
				}
				return err
			},
		},
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				table, err := repo.tenantTable(ctx, schemaName, "invoice_interest")
				if table != nil {
					t.Fatalf("tenantTable() table = %#v, want nil", table)
				}
				return err
			},
		},
		{
			name: "GetInvoiceForInterest",
			run: func(t *testing.T) error {
				invoice, err := repo.GetInvoiceForInterest(ctx, schemaName, tenantID, invoiceID)
				if invoice != nil {
					t.Fatalf("GetInvoiceForInterest() invoice = %#v, want nil", invoice)
				}
				return err
			},
		},
		{
			name: "CreateInterest",
			run: func(t *testing.T) error {
				return repo.CreateInterest(ctx, schemaName, &InvoiceInterest{ID: "interest-1", InvoiceID: invoiceID})
			},
		},
		{
			name: "GetLatestInterest",
			run: func(t *testing.T) error {
				interest, err := repo.GetLatestInterest(ctx, schemaName, invoiceID)
				if interest != nil {
					t.Fatalf("GetLatestInterest() interest = %#v, want nil", interest)
				}
				return err
			},
		},
		{
			name: "ListInterestHistory",
			run: func(t *testing.T) error {
				history, err := repo.ListInterestHistory(ctx, schemaName, invoiceID)
				if history != nil {
					t.Fatalf("ListInterestHistory() history = %#v, want nil", history)
				}
				return err
			},
		},
		{
			name: "ListOverdueInvoices",
			run: func(t *testing.T) error {
				invoices, err := repo.ListOverdueInvoices(ctx, schemaName, tenantID, asOf)
				if invoices != nil {
					t.Fatalf("ListOverdueInvoices() invoices = %#v, want nil", invoices)
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireErrorContains(t, tt.run(t), "interest repository database is not configured")
		})
	}
}

func requireErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err.Error(), want)
	}
}
