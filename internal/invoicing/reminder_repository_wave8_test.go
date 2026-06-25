package invoicing

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReminderGORMRepositoryWave8OverdueAndCountScanSuccess(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_invoicing"
	tenantID := "tenant-1"
	issueDate := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC)
	asOf := time.Date(2026, time.June, 25, 0, 0, 0, 0, time.UTC)
	lastSent := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)
	repo := NewReminderGORMRepository(newInvoicingDryRunDB(t, withInvoicingWave6ScanRows(
		invoicingWave6RowSet{
			columns: []string{"id", "invoice_number", "contact_id", "contact_name", "contact_email", "issue_date", "due_date", "total", "amount_paid", "outstanding_amount", "currency", "days_overdue"},
			values: [][]driver.Value{{
				"invoice-1",
				"INV-001",
				"contact-1",
				"Acme OU",
				"billing@example.com",
				issueDate,
				dueDate,
				"125.00",
				"25.00",
				"100.00",
				"EUR",
				int64(25),
			}},
		},
		invoicingWave6RowSet{
			columns: []string{"count", "last_sent_at"},
			values:  [][]driver.Value{{int64(3), lastSent}},
		},
	)))

	invoices, err := repo.GetOverdueInvoices(ctx, schemaName, tenantID, asOf)
	require.NoError(t, err)
	require.Len(t, invoices, 1)
	assert.Equal(t, "invoice-1", invoices[0].ID)
	assert.Equal(t, "INV-001", invoices[0].InvoiceNumber)
	assert.Equal(t, "2026-05-01", invoices[0].IssueDate)
	assert.Equal(t, "2026-05-31", invoices[0].DueDate)
	assert.True(t, invoices[0].Total.Equal(decimal.RequireFromString("125.00")))
	assert.True(t, invoices[0].AmountPaid.Equal(decimal.RequireFromString("25.00")))
	assert.True(t, invoices[0].OutstandingAmount.Equal(decimal.RequireFromString("100.00")))
	assert.Equal(t, 25, invoices[0].DaysOverdue)

	count, gotLastSent, err := repo.GetReminderCount(ctx, schemaName, tenantID, "invoice-1")
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	require.NotNil(t, gotLastSent)
	assert.Equal(t, lastSent, *gotLastSent)
}
