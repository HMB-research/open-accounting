package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/HMB-research/open-accounting/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGORMRepository_NilDatabase(t *testing.T) {
	repo := NewGORMRepository(nil)
	ctx := context.Background()
	schemaName := "tenant_schema"
	startDate := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC)

	require.NotNil(t, repo)
	assert.Nil(t, repo.db)

	tests := []struct {
		name string
		run  func(t *testing.T) error
	}{
		{
			name: "tenantTable",
			run: func(t *testing.T) error {
				db, err := repo.tenantTable(ctx, schemaName, "invoices", "i")
				assert.Nil(t, db)
				return err
			},
		},
		{
			name: "GetRevenueExpenses",
			run: func(t *testing.T) error {
				revenue, expenses, err := repo.GetRevenueExpenses(ctx, schemaName, startDate, endDate)
				assert.True(t, revenue.IsZero())
				assert.True(t, expenses.IsZero())
				return err
			},
		},
		{
			name: "GetReceivablesSummary",
			run: func(t *testing.T) error {
				total, overdue, err := repo.GetReceivablesSummary(ctx, schemaName)
				assert.True(t, total.IsZero())
				assert.True(t, overdue.IsZero())
				return err
			},
		},
		{
			name: "GetPayablesSummary",
			run: func(t *testing.T) error {
				total, overdue, err := repo.GetPayablesSummary(ctx, schemaName)
				assert.True(t, total.IsZero())
				assert.True(t, overdue.IsZero())
				return err
			},
		},
		{
			name: "GetInvoiceCounts",
			run: func(t *testing.T) error {
				draft, pending, overdue, err := repo.GetInvoiceCounts(ctx, schemaName)
				assert.Zero(t, draft)
				assert.Zero(t, pending)
				assert.Zero(t, overdue)
				return err
			},
		},
		{
			name: "GetMonthlyRevenueExpenses",
			run: func(t *testing.T) error {
				data, err := repo.GetMonthlyRevenueExpenses(ctx, schemaName, 3)
				assert.Nil(t, data)
				return err
			},
		},
		{
			name: "GetMonthlyCashFlow",
			run: func(t *testing.T) error {
				data, err := repo.GetMonthlyCashFlow(ctx, schemaName, 3)
				assert.Nil(t, data)
				return err
			},
		},
		{
			name: "GetAgingByContact",
			run: func(t *testing.T) error {
				data, err := repo.GetAgingByContact(ctx, schemaName, string(models.InvoiceTypeSales))
				assert.Nil(t, data)
				return err
			},
		},
		{
			name: "GetTopCustomers",
			run: func(t *testing.T) error {
				items, err := repo.GetTopCustomers(ctx, schemaName, 5)
				assert.Nil(t, items)
				return err
			},
		},
		{
			name: "GetRecentActivity",
			run: func(t *testing.T) error {
				items, err := repo.GetRecentActivity(ctx, schemaName, 5)
				assert.Nil(t, items)
				return err
			},
		},
		{
			name: "recentInvoiceActivity",
			run: func(t *testing.T) error {
				items, err := repo.recentInvoiceActivity(ctx, schemaName, 5)
				assert.Nil(t, items)
				return err
			},
		},
		{
			name: "recentPaymentActivity",
			run: func(t *testing.T) error {
				items, err := repo.recentPaymentActivity(ctx, schemaName, 5)
				assert.Nil(t, items)
				return err
			},
		},
		{
			name: "recentJournalEntryActivity",
			run: func(t *testing.T) error {
				items, err := repo.recentJournalEntryActivity(ctx, schemaName, 5)
				assert.Nil(t, items)
				return err
			},
		},
		{
			name: "recentContactActivity",
			run: func(t *testing.T) error {
				items, err := repo.recentContactActivity(ctx, schemaName, 5)
				assert.Nil(t, items)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "analytics repository database is not configured")
		})
	}
}

func TestGORMRepository_LimitGuardsDoNotRequireDatabase(t *testing.T) {
	repo := NewRepository(nil)
	ctx := context.Background()

	revenueExpenses, err := repo.GetMonthlyRevenueExpenses(ctx, "tenant_schema", 0)
	require.NoError(t, err)
	assert.Empty(t, revenueExpenses)

	cashFlow, err := repo.GetMonthlyCashFlow(ctx, "tenant_schema", 0)
	require.NoError(t, err)
	assert.Empty(t, cashFlow)

	topCustomers, err := repo.GetTopCustomers(ctx, "tenant_schema", 0)
	require.NoError(t, err)
	assert.Empty(t, topCustomers)

	activity, err := repo.GetRecentActivity(ctx, "tenant_schema", 0)
	require.NoError(t, err)
	assert.Empty(t, activity)
}

func TestInvoiceActivityActionUnitCases(t *testing.T) {
	tests := []struct {
		name   string
		status models.InvoiceStatus
		want   string
	}{
		{name: "draft", status: models.InvoiceStatusDraft, want: "created"},
		{name: "sent", status: models.InvoiceStatusSent, want: "sent"},
		{name: "paid", status: models.InvoiceStatusPaid, want: "paid"},
		{name: "voided", status: models.InvoiceStatusVoided, want: "voided"},
		{name: "other status", status: models.InvoiceStatusPartiallyPaid, want: "updated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, invoiceActivityAction(tt.status))
		})
	}
}

func TestRecentMonthStarts(t *testing.T) {
	assert.Empty(t, recentMonthStarts(0))
	assert.Empty(t, recentMonthStarts(-1))

	monthStarts := recentMonthStarts(4)
	require.Len(t, monthStarts, 4)
	for i, month := range monthStarts {
		assert.Equal(t, 1, month.Day())
		assert.Equal(t, 0, month.Hour())
		assert.Equal(t, 0, month.Minute())
		assert.Equal(t, 0, month.Second())
		assert.Equal(t, 0, month.Nanosecond())
		if i > 0 {
			assert.Equal(t, monthStarts[i-1].AddDate(0, 1, 0), month)
		}
	}
}

func TestMonthFormattingHelpers(t *testing.T) {
	month := time.Date(2026, time.March, 15, 14, 30, 0, 0, time.UTC)

	assert.Equal(t, "2026-03", monthKey(month))
	assert.Equal(t, "Mar 2026", monthLabel(month))
}

func TestQualifiedTenantTable(t *testing.T) {
	qualified := qualifiedTenantTable("tenant_schema", "invoices")
	assert.Equal(t, `"tenant_schema"."invoices"`, qualified)
}
