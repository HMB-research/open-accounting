package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Now()

func TestNewReminderServiceWithRepository(t *testing.T) {
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	assert.NotNil(t, svc)
	assert.Equal(t, repo, svc.repo)
}

func TestReminderService_GetOverdueInvoicesSummary(t *testing.T) {
	t.Run("returns empty summary when no overdue invoices", func(t *testing.T) {
		repo := NewMockReminderRepository()
		svc := NewReminderServiceWithRepository(repo, nil)

		summary, err := svc.GetOverdueInvoicesSummary(context.Background(), "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.NotNil(t, summary)
		assert.Empty(t, summary.Invoices)
		assert.True(t, summary.TotalOverdue.IsZero())
		assert.Equal(t, 0, summary.InvoiceCount)
	})

	t.Run("returns summary with overdue invoices", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.AddMockOverdueInvoice(
			"inv-1", "INV-001", "contact-1", "Test Contact", "test@example.com",
			"EUR", decimal.NewFromFloat(100.00), decimal.NewFromFloat(0), 5,
		)
		repo.AddMockOverdueInvoice(
			"inv-2", "INV-002", "contact-2", "Another Contact", "another@example.com",
			"EUR", decimal.NewFromFloat(200.00), decimal.NewFromFloat(50.00), 10,
		)

		svc := NewReminderServiceWithRepository(repo, nil)

		summary, err := svc.GetOverdueInvoicesSummary(context.Background(), "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.NotNil(t, summary)
		assert.Len(t, summary.Invoices, 2)
		assert.Equal(t, 2, summary.InvoiceCount)
		assert.Equal(t, 2, summary.ContactCount)
		// Total overdue = 100 + (200-50) = 250
		assert.True(t, summary.TotalOverdue.Equal(decimal.NewFromFloat(250.00)),
			"Expected TotalOverdue to be 250, got %s", summary.TotalOverdue.String())
	})

	t.Run("returns error when repository fails", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.GetOverdueErr = errors.New("database error")
		svc := NewReminderServiceWithRepository(repo, nil)

		_, err := svc.GetOverdueInvoicesSummary(context.Background(), "tenant-1", "test_schema")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
	})
}

func TestReminderService_GetReminderHistory(t *testing.T) {
	t.Run("returns empty history for invoice with no reminders", func(t *testing.T) {
		repo := NewMockReminderRepository()
		svc := NewReminderServiceWithRepository(repo, nil)

		history, err := svc.GetReminderHistory(context.Background(), "tenant-1", "test_schema", "inv-1")

		require.NoError(t, err)
		assert.Empty(t, history)
	})

	t.Run("returns reminder history for invoice", func(t *testing.T) {
		repo := NewMockReminderRepository()
		// Add some reminders
		repo.Reminders["inv-1"] = []PaymentReminder{
			{ID: "rem-1", InvoiceID: "inv-1", Status: ReminderStatusSent},
			{ID: "rem-2", InvoiceID: "inv-1", Status: ReminderStatusPending},
		}
		svc := NewReminderServiceWithRepository(repo, nil)

		history, err := svc.GetReminderHistory(context.Background(), "tenant-1", "test_schema", "inv-1")

		require.NoError(t, err)
		assert.Len(t, history, 2)
	})
}

func TestMockReminderRepository(t *testing.T) {
	t.Run("NewMockReminderRepository creates valid mock", func(t *testing.T) {
		repo := NewMockReminderRepository()
		assert.NotNil(t, repo)
		assert.Empty(t, repo.OverdueInvoices)
		assert.NotNil(t, repo.Reminders)
	})

	t.Run("GetOverdueInvoices returns empty slice by default", func(t *testing.T) {
		repo := NewMockReminderRepository()
		invoices, err := repo.GetOverdueInvoices(context.Background(), "schema", "tenant", now)

		require.NoError(t, err)
		assert.Empty(t, invoices)
	})

	t.Run("GetOverdueInvoices returns error when configured", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.GetOverdueErr = errors.New("test error")

		_, err := repo.GetOverdueInvoices(context.Background(), "schema", "tenant", now)

		require.Error(t, err)
		assert.Equal(t, "test error", err.Error())
	})

	t.Run("CreateReminder adds reminder to map", func(t *testing.T) {
		repo := NewMockReminderRepository()
		reminder := &PaymentReminder{ID: "rem-1", InvoiceID: "inv-1"}

		err := repo.CreateReminder(context.Background(), "schema", reminder)

		require.NoError(t, err)
		assert.Len(t, repo.Reminders["inv-1"], 1)
	})

	t.Run("UpdateReminderStatus updates existing reminder", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.Reminders["inv-1"] = []PaymentReminder{{ID: "rem-1", InvoiceID: "inv-1", Status: ReminderStatusPending}}

		err := repo.UpdateReminderStatus(context.Background(), "schema", "rem-1", ReminderStatusSent, nil, "")

		require.NoError(t, err)
		assert.Equal(t, ReminderStatusSent, repo.Reminders["inv-1"][0].Status)
	})

	t.Run("GetRemindersByInvoice returns reminders", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.Reminders["inv-1"] = []PaymentReminder{{ID: "rem-1"}, {ID: "rem-2"}}

		reminders, err := repo.GetRemindersByInvoice(context.Background(), "schema", "tenant", "inv-1")

		require.NoError(t, err)
		assert.Len(t, reminders, 2)
	})

	t.Run("GetReminderCount returns count of sent reminders", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.Reminders["inv-1"] = []PaymentReminder{
			{ID: "rem-1", Status: ReminderStatusSent},
			{ID: "rem-2", Status: ReminderStatusPending},
			{ID: "rem-3", Status: ReminderStatusSent},
		}

		count, _, err := repo.GetReminderCount(context.Background(), "schema", "tenant", "inv-1")

		require.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("AddMockOverdueInvoice adds invoice to list", func(t *testing.T) {
		repo := NewMockReminderRepository()
		repo.AddMockOverdueInvoice("inv-1", "INV-001", "contact-1", "Test", "test@example.com", "EUR",
			decimal.NewFromFloat(100), decimal.NewFromFloat(0), 5)

		assert.Len(t, repo.OverdueInvoices, 1)
		assert.Equal(t, "inv-1", repo.OverdueInvoices[0].ID)
		assert.Equal(t, 5, repo.OverdueInvoices[0].DaysOverdue)
	})
}
