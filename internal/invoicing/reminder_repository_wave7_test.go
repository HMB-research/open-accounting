package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReminderRepositoryWave7PanicsOnUnreachablePool(t *testing.T) {
	config, err := pgxpool.ParseConfig("postgres://open_accounting:open_accounting@127.0.0.1:1/open_accounting?sslmode=disable")
	require.NoError(t, err)
	config.ConnConfig.ConnectTimeout = 10 * time.Millisecond
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	defer pool.Close()

	require.Panics(t, func() {
		_ = NewReminderRepository(pool)
	})
}

func TestReminderGORMRepositoryWave7NilDatabaseGuards(t *testing.T) {
	ctx := context.Background()
	repo := NewReminderRepository(nil)
	asOfDate := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)

	invoices, err := repo.GetOverdueInvoices(ctx, "tenant_invoicing", "tenant-1", asOfDate)
	assert.Nil(t, invoices)
	require.ErrorContains(t, err, "query overdue invoices")
	assert.ErrorContains(t, err, "reminder repository database is not configured")

	count, lastSent, err := repo.GetReminderCount(ctx, "tenant_invoicing", "tenant-1", "invoice-1")
	assert.Zero(t, count)
	assert.Nil(t, lastSent)
	require.ErrorContains(t, err, "qualify payment reminders table")
	assert.ErrorContains(t, err, "reminder repository database is not configured")
}

func TestMockReminderRepositoryWave7CountsOnlySentReminders(t *testing.T) {
	ctx := context.Background()
	repo := NewMockReminderRepository()
	now := time.Date(2026, time.June, 25, 9, 0, 0, 0, time.UTC)
	older := now.Add(-time.Hour)
	repo.Reminders["invoice-1"] = []PaymentReminder{
		{ID: "pending", Status: ReminderStatusPending},
		{ID: "sent-old", Status: ReminderStatusSent, SentAt: &older},
		{ID: "sent-new", Status: ReminderStatusSent, SentAt: &now},
		{ID: "failed", Status: ReminderStatusFailed, SentAt: &now},
	}

	count, lastSent, err := repo.GetReminderCount(ctx, "tenant_invoicing", "tenant-1", "invoice-1")

	require.NoError(t, err)
	assert.Equal(t, 2, count)
	require.NotNil(t, lastSent)
	assert.Equal(t, now, *lastSent)

	expectedErr := errors.New("overdue failed")
	repo.GetOverdueErr = expectedErr
	invoices, err := repo.GetOverdueInvoices(ctx, "tenant_invoicing", "tenant-1", now)
	assert.Nil(t, invoices)
	assert.ErrorIs(t, err, expectedErr)

	repo.AddMockOverdueInvoice("invoice-2", "INV-2", "contact-1", "Acme", "billing@example.com", "EUR", decimal.NewFromInt(122), decimal.NewFromInt(22), 5)
	require.Len(t, repo.OverdueInvoices, 1)
	assert.True(t, repo.OverdueInvoices[0].OutstandingAmount.Equal(decimal.NewFromInt(100)))
}
