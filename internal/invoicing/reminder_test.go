package invoicing

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMockReminderRepository(t *testing.T) {
	repo := NewMockReminderRepository()
	require.NotNil(t, repo)
	assert.Empty(t, repo.OverdueInvoices)
	assert.Empty(t, repo.Reminders)
}

func TestMockReminderRepository_GetOverdueInvoices(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	// Test empty result
	invoices, err := repo.GetOverdueInvoices(ctx, "test_schema", "tenant-1", time.Now())
	require.NoError(t, err)
	assert.Empty(t, invoices)

	// Add mock invoices
	repo.AddMockOverdueInvoice(
		"inv-1",
		"INV-2024-001",
		"contact-1",
		"Test Customer",
		"test@example.com",
		"EUR",
		decimal.NewFromInt(1000),
		decimal.NewFromInt(0),
		30,
	)

	invoices, err = repo.GetOverdueInvoices(ctx, "test_schema", "tenant-1", time.Now())
	require.NoError(t, err)
	assert.Len(t, invoices, 1)
	assert.Equal(t, "inv-1", invoices[0].ID)
	assert.Equal(t, "INV-2024-001", invoices[0].InvoiceNumber)
	assert.Equal(t, "Test Customer", invoices[0].ContactName)
}

func TestMockReminderRepository_GetOverdueInvoicesWithError(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.GetOverdueErr = assert.AnError

	invoices, err := repo.GetOverdueInvoices(ctx, "test_schema", "tenant-1", time.Now())
	require.Error(t, err)
	assert.Nil(t, invoices)
}

func TestMockReminderRepository_CreateReminder(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	reminder := &PaymentReminder{
		ID:            "reminder-1",
		TenantID:      "tenant-1",
		InvoiceID:     "inv-1",
		InvoiceNumber: "INV-2024-001",
		ContactID:     "contact-1",
		ContactName:   "Test Customer",
		ContactEmail:  "test@example.com",
		ReminderNumber: 1,
		Status:        ReminderStatusPending,
	}

	err := repo.CreateReminder(ctx, "test_schema", reminder)
	require.NoError(t, err)

	// Verify it was stored
	reminders := repo.Reminders["inv-1"]
	assert.Len(t, reminders, 1)
	assert.Equal(t, "reminder-1", reminders[0].ID)
}

func TestMockReminderRepository_GetReminderCount(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	// Test with no reminders
	count, lastSent, err := repo.GetReminderCount(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Nil(t, lastSent)

	// Add some reminders
	now := time.Now()
	earlier := now.Add(-24 * time.Hour)

	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", Status: ReminderStatusSent, SentAt: &earlier},
		{ID: "r2", InvoiceID: "inv-1", Status: ReminderStatusSent, SentAt: &now},
		{ID: "r3", InvoiceID: "inv-1", Status: ReminderStatusPending},
		{ID: "r4", InvoiceID: "inv-1", Status: ReminderStatusFailed},
	}

	count, lastSent, err = repo.GetReminderCount(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Equal(t, 2, count) // Only SENT status
	require.NotNil(t, lastSent)
	assert.Equal(t, now.Unix(), lastSent.Unix())
}

func TestMockReminderRepository_UpdateReminderStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	now := time.Now()
	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", Status: ReminderStatusPending},
	}

	err := repo.UpdateReminderStatus(ctx, "test_schema", "r1", ReminderStatusSent, &now, "")
	require.NoError(t, err)

	// Verify update
	assert.Equal(t, ReminderStatusSent, repo.Reminders["inv-1"][0].Status)
	assert.NotNil(t, repo.Reminders["inv-1"][0].SentAt)

	// Test with error message
	err = repo.UpdateReminderStatus(ctx, "test_schema", "r1", ReminderStatusFailed, nil, "Email failed")
	require.NoError(t, err)
	assert.Equal(t, ReminderStatusFailed, repo.Reminders["inv-1"][0].Status)
	assert.Equal(t, "Email failed", repo.Reminders["inv-1"][0].ErrorMessage)

	// Test updating non-existent reminder (should not error)
	err = repo.UpdateReminderStatus(ctx, "test_schema", "nonexistent", ReminderStatusSent, &now, "")
	require.NoError(t, err)
}

func TestMockReminderRepository_GetRemindersByInvoice(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()

	// Test empty
	reminders, err := repo.GetRemindersByInvoice(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Empty(t, reminders)

	// Add reminders
	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", ReminderNumber: 1},
		{ID: "r2", InvoiceID: "inv-1", ReminderNumber: 2},
	}
	repo.Reminders["inv-2"] = []PaymentReminder{
		{ID: "r3", InvoiceID: "inv-2", ReminderNumber: 1},
	}

	reminders, err = repo.GetRemindersByInvoice(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Len(t, reminders, 2)

	// Check inv-2
	reminders, err = repo.GetRemindersByInvoice(ctx, "test_schema", "tenant-1", "inv-2")
	require.NoError(t, err)
	assert.Len(t, reminders, 1)
}

func TestMockReminderRepository_AddMockOverdueInvoice(t *testing.T) {
	repo := NewMockReminderRepository()

	repo.AddMockOverdueInvoice(
		"inv-1",
		"INV-2024-001",
		"contact-1",
		"Test Customer",
		"test@example.com",
		"EUR",
		decimal.NewFromInt(1000),
		decimal.NewFromInt(500),
		30,
	)

	require.Len(t, repo.OverdueInvoices, 1)
	inv := repo.OverdueInvoices[0]

	assert.Equal(t, "inv-1", inv.ID)
	assert.Equal(t, "INV-2024-001", inv.InvoiceNumber)
	assert.Equal(t, "contact-1", inv.ContactID)
	assert.Equal(t, "Test Customer", inv.ContactName)
	assert.Equal(t, "test@example.com", inv.ContactEmail)
	assert.Equal(t, "EUR", inv.Currency)
	assert.True(t, inv.Total.Equal(decimal.NewFromInt(1000)))
	assert.True(t, inv.AmountPaid.Equal(decimal.NewFromInt(500)))
	assert.True(t, inv.OutstandingAmount.Equal(decimal.NewFromInt(500)))
	assert.Equal(t, 30, inv.DaysOverdue)
}

func TestReminderStatusConstants(t *testing.T) {
	assert.Equal(t, ReminderStatus("PENDING"), ReminderStatusPending)
	assert.Equal(t, ReminderStatus("SENT"), ReminderStatusSent)
	assert.Equal(t, ReminderStatus("FAILED"), ReminderStatusFailed)
	assert.Equal(t, ReminderStatus("CANCELED"), ReminderStatusCanceled)
}

// Test ReminderService with mock repository
func TestNewReminderServiceWithRepository(t *testing.T) {
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)
	require.NotNil(t, svc)
}

func TestReminderService_GetOverdueInvoicesSummary(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	// Add mock overdue invoices
	repo.AddMockOverdueInvoice("inv-1", "INV-001", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(1000), decimal.Zero, 30)
	repo.AddMockOverdueInvoice("inv-2", "INV-002", "c1", "Customer 1", "c1@test.com", "EUR",
		decimal.NewFromInt(500), decimal.Zero, 15)
	repo.AddMockOverdueInvoice("inv-3", "INV-003", "c2", "Customer 2", "c2@test.com", "EUR",
		decimal.NewFromInt(2000), decimal.NewFromInt(500), 45)

	summary, err := svc.GetOverdueInvoicesSummary(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)

	assert.Equal(t, 3, summary.InvoiceCount)
	assert.Equal(t, 2, summary.ContactCount) // 2 unique contacts
	assert.True(t, summary.TotalOverdue.Equal(decimal.NewFromInt(3000)))
	assert.Equal(t, 30, summary.AverageDaysOverdue) // (30+15+45)/3 = 30
	assert.Len(t, summary.Invoices, 3)
}

func TestReminderService_GetOverdueInvoicesSummary_Empty(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	summary, err := svc.GetOverdueInvoicesSummary(ctx, "test_schema", "tenant-1")
	require.NoError(t, err)

	assert.Equal(t, 0, summary.InvoiceCount)
	assert.Equal(t, 0, summary.ContactCount)
	assert.True(t, summary.TotalOverdue.IsZero())
	assert.Equal(t, 0, summary.AverageDaysOverdue)
}

func TestReminderService_GetOverdueInvoicesSummary_Error(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	repo.GetOverdueErr = assert.AnError
	svc := NewReminderServiceWithRepository(repo, nil)

	_, err := svc.GetOverdueInvoicesSummary(ctx, "test_schema", "tenant-1")
	require.Error(t, err)
}

func TestReminderService_GetReminderHistory(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	svc := NewReminderServiceWithRepository(repo, nil)

	// Add reminders
	now := time.Now()
	repo.Reminders["inv-1"] = []PaymentReminder{
		{ID: "r1", InvoiceID: "inv-1", ReminderNumber: 1, Status: ReminderStatusSent, SentAt: &now},
		{ID: "r2", InvoiceID: "inv-1", ReminderNumber: 2, Status: ReminderStatusPending},
	}

	reminders, err := svc.GetReminderHistory(ctx, "test_schema", "tenant-1", "inv-1")
	require.NoError(t, err)
	assert.Len(t, reminders, 2)
}
