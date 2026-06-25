package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type invoicingWave9TemplateRepo struct {
	reminderEmailRepo
	template *email.EmailTemplate
}

type invoicingWave9ReminderRepo struct {
	*MockReminderRepository
	createErr error
	countErr  error
}

func (r invoicingWave9ReminderRepo) GetReminderCount(ctx context.Context, schemaName, tenantID, invoiceID string) (int, *time.Time, error) {
	if r.countErr != nil {
		return 0, nil, r.countErr
	}
	return r.MockReminderRepository.GetReminderCount(ctx, schemaName, tenantID, invoiceID)
}

func (r invoicingWave9ReminderRepo) CreateReminder(ctx context.Context, schemaName string, reminder *PaymentReminder) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.MockReminderRepository.CreateReminder(ctx, schemaName, reminder)
}

func (r invoicingWave9TemplateRepo) GetTemplate(context.Context, string, string, email.TemplateType) (*email.EmailTemplate, error) {
	if r.template != nil {
		return r.template, nil
	}
	return r.reminderEmailRepo.GetTemplate(context.Background(), "", "", "")
}

func TestInvoicingWave9ServiceConstructorPanicsForGormPoolError(t *testing.T) {
	pool := stubNewGormDBFromPoolError(t, errors.New("pool unavailable"))

	require.PanicsWithError(t, "create invoicing GORM repository: pool unavailable", func() {
		_ = NewService(pool, nil)
	})
}

func TestInvoicingWave9ResolveInvoiceBlankNumber(t *testing.T) {
	_, err := NewServiceWithRepository(NewMockRepository(), nil).ResolveInvoiceIDByNumber(context.Background(), "tenant-1", "tenant_demo", " ")
	require.ErrorContains(t, err, "invoice_number is required")
}

func TestInvoicingWave9ReminderSendErrorBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("create reminder error", func(t *testing.T) {
		base := NewMockReminderRepository()
		base.AddMockOverdueInvoice("inv-1", "INV-1", "contact-1", "Acme", "billing@example.com", "EUR", decimal.NewFromInt(100), decimal.Zero, 10)
		repo := invoicingWave9ReminderRepo{MockReminderRepository: base, createErr: errors.New("create failed")}
		_, err := NewReminderServiceWithRepository(repo, nil).SendReminder(ctx, "tenant-1", "tenant_demo", &SendReminderRequest{InvoiceID: "inv-1"}, "Acme")
		require.ErrorContains(t, err, "create reminder")
	})

	t.Run("reminder count error", func(t *testing.T) {
		base := NewMockReminderRepository()
		base.AddMockOverdueInvoice("inv-1", "INV-1", "contact-1", "Acme", "billing@example.com", "EUR", decimal.NewFromInt(100), decimal.Zero, 10)
		repo := invoicingWave9ReminderRepo{MockReminderRepository: base, countErr: errors.New("count failed")}
		_, err := NewReminderServiceWithRepository(repo, nil).SendReminder(ctx, "tenant-1", "tenant_demo", &SendReminderRequest{InvoiceID: "inv-1"}, "Acme")
		require.ErrorContains(t, err, "get reminder count")
	})

	t.Run("template lookup failure returns failed result", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.AddMockOverdueInvoice("inv-1", "INV-1", "contact-1", "Acme", "billing@example.com", "EUR", decimal.NewFromInt(100), decimal.Zero, 10)
		emailSvc := email.NewServiceWithRepository(&reminderEmailRepo{getTemplateErr: errors.New("template unavailable")}, &reminderMailSender{})
		result, err := NewReminderServiceWithRepository(repo, emailSvc).SendReminder(ctx, "tenant-1", "tenant_demo", &SendReminderRequest{InvoiceID: "inv-1"}, "Acme")
		require.NoError(t, err)
		require.False(t, result.Success)
		assert.Contains(t, result.Message, "Failed to get email template")
	})

	t.Run("render failure returns failed result", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.AddMockOverdueInvoice("inv-1", "INV-1", "contact-1", "Acme", "billing@example.com", "EUR", decimal.NewFromInt(100), decimal.Zero, 10)
		emailSvc := email.NewServiceWithRepository(&invoicingWave9TemplateRepo{template: &email.EmailTemplate{
			TemplateType: email.TemplateOverdueReminder,
			Subject:      "{{",
			BodyHTML:     "body",
		}}, &reminderMailSender{})
		result, err := NewReminderServiceWithRepository(repo, emailSvc).SendReminder(ctx, "tenant-1", "tenant_demo", &SendReminderRequest{InvoiceID: "inv-1"}, "Acme")
		require.NoError(t, err)
		require.False(t, result.Success)
		assert.Contains(t, result.Message, "Failed to render email template")
	})
}

func TestInvoicingWave9AutomatedReminderBranches(t *testing.T) {
	ctx := context.Background()
	invoice := &InvoiceForReminder{
		ID:                "inv-1",
		InvoiceNumber:     "INV-1",
		ContactID:         "contact-1",
		ContactName:       "Acme",
		ContactEmail:      "billing@example.com",
		OutstandingAmount: "100",
		Currency:          "EUR",
		DueDate:           "2026-06-01",
		DaysOverdue:       5,
	}
	rule := &ReminderRule{ID: "rule-1", EmailTemplateType: "CUSTOM_TEMPLATE", TriggerType: TriggerAfterDue, DaysOffset: 5}

	t.Run("fallback template failure", func(t *testing.T) {
		service := NewAutomatedReminderServiceWithRepository(&mockReminderRuleRepo{}, email.NewServiceWithRepository(&reminderEmailRepo{getTemplateErr: errors.New("template db down")}, &reminderMailSender{}))
		err := service.sendReminder(ctx, "tenant-1", "tenant_demo", "Acme", rule, invoice)
		require.ErrorContains(t, err, "get template")
	})

	t.Run("render failure", func(t *testing.T) {
		service := NewAutomatedReminderServiceWithRepository(&mockReminderRuleRepo{}, email.NewServiceWithRepository(&invoicingWave9TemplateRepo{template: &email.EmailTemplate{
			TemplateType: email.TemplateOverdueReminder,
			Subject:      "{{",
			BodyHTML:     "body",
		}}, &reminderMailSender{}))
		err := service.sendReminder(ctx, "tenant-1", "tenant_demo", "Acme", rule, invoice)
		require.ErrorContains(t, err, "render template")
	})

	t.Run("record reminder sets sent timestamp and ignores record errors", func(t *testing.T) {
		repo := &mockReminderRuleRepo{recordErr: errors.New("record failed")}
		service := NewAutomatedReminderServiceWithRepository(repo, nil)
		service.recordReminder(ctx, "tenant_demo", "tenant-1", rule, invoice, ReminderStatusSent, "")
		// recordErr path logs and leaves recordedReminder nil.
		assert.Nil(t, repo.recordedReminder)

		repo.recordErr = nil
		service.recordReminder(ctx, "tenant_demo", "tenant-1", rule, invoice, ReminderStatusSent, "")
		require.NotNil(t, repo.recordedReminder)
		require.NotNil(t, repo.recordedReminder.SentAt)
	})

	t.Run("update rule propagates get error", func(t *testing.T) {
		_, err := NewAutomatedReminderServiceWithRepository(&mockReminderRuleRepo{}, nil).UpdateRule(ctx, "tenant-1", "tenant_demo", "missing", &UpdateReminderRuleRequest{})
		require.ErrorIs(t, err, ErrRuleNotFound)
	})
}
