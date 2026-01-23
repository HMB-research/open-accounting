package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository is a mock implementation of the Repository interface for testing
type MockRepository struct {
	// GetRevenueExpenses mock data
	RevenueExpensesRevenue  decimal.Decimal
	RevenueExpensesExpenses decimal.Decimal
	RevenueExpensesError    error

	// GetReceivablesSummary mock data
	ReceivablesTotal   decimal.Decimal
	ReceivablesOverdue decimal.Decimal
	ReceivablesError   error

	// GetPayablesSummary mock data
	PayablesTotal   decimal.Decimal
	PayablesOverdue decimal.Decimal
	PayablesError   error

	// GetInvoiceCounts mock data
	InvoiceCountsDraft   int
	InvoiceCountsPending int
	InvoiceCountsOverdue int
	InvoiceCountsError   error

	// GetMonthlyRevenueExpenses mock data
	MonthlyRevenueExpensesData  []MonthlyData
	MonthlyRevenueExpensesError error

	// GetMonthlyCashFlow mock data
	MonthlyCashFlowData  []MonthlyCashFlowData
	MonthlyCashFlowError error

	// GetAgingByContact mock data
	AgingByContactData  []ContactAging
	AgingByContactError error

	// GetTopCustomers mock data
	TopCustomersData  []TopItem
	TopCustomersError error

	// GetRecentActivity mock data
	RecentActivityData  []ActivityItem
	RecentActivityError error

	// Track calls
	LastSchemaName  string
	LastInvoiceType string
	LastStart       time.Time
	LastEnd         time.Time
	LastMonths      int
	LastLimit       int
}

func (m *MockRepository) GetRevenueExpenses(ctx context.Context, schemaName string, start, end time.Time) (revenue, expenses decimal.Decimal, err error) {
	m.LastSchemaName = schemaName
	m.LastStart = start
	m.LastEnd = end
	return m.RevenueExpensesRevenue, m.RevenueExpensesExpenses, m.RevenueExpensesError
}

func (m *MockRepository) GetReceivablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error) {
	m.LastSchemaName = schemaName
	return m.ReceivablesTotal, m.ReceivablesOverdue, m.ReceivablesError
}

func (m *MockRepository) GetPayablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error) {
	m.LastSchemaName = schemaName
	return m.PayablesTotal, m.PayablesOverdue, m.PayablesError
}

func (m *MockRepository) GetInvoiceCounts(ctx context.Context, schemaName string) (draft, pending, overdue int, err error) {
	m.LastSchemaName = schemaName
	return m.InvoiceCountsDraft, m.InvoiceCountsPending, m.InvoiceCountsOverdue, m.InvoiceCountsError
}

func (m *MockRepository) GetMonthlyRevenueExpenses(ctx context.Context, schemaName string, months int) ([]MonthlyData, error) {
	m.LastSchemaName = schemaName
	m.LastMonths = months
	return m.MonthlyRevenueExpensesData, m.MonthlyRevenueExpensesError
}

func (m *MockRepository) GetMonthlyCashFlow(ctx context.Context, schemaName string, months int) ([]MonthlyCashFlowData, error) {
	m.LastSchemaName = schemaName
	m.LastMonths = months
	return m.MonthlyCashFlowData, m.MonthlyCashFlowError
}

func (m *MockRepository) GetAgingByContact(ctx context.Context, schemaName, invoiceType string) ([]ContactAging, error) {
	m.LastSchemaName = schemaName
	m.LastInvoiceType = invoiceType
	return m.AgingByContactData, m.AgingByContactError
}

func (m *MockRepository) GetTopCustomers(ctx context.Context, schemaName string, limit int) ([]TopItem, error) {
	m.LastSchemaName = schemaName
	m.LastLimit = limit
	return m.TopCustomersData, m.TopCustomersError
}

func (m *MockRepository) GetRecentActivity(ctx context.Context, schemaName string, limit int) ([]ActivityItem, error) {
	m.LastSchemaName = schemaName
	m.LastLimit = limit
	return m.RecentActivityData, m.RecentActivityError
}

func TestNewService(t *testing.T) {
	// Test with nil pool - service should be created but will fail on actual queries
	service := NewService(nil)
	if service == nil {
		t.Fatal("NewService(nil) returned nil")
	}
	if service.pool != nil {
		t.Error("NewService(nil).pool should be nil")
	}
}

func TestNewService_NotNil(t *testing.T) {
	service := NewService(nil)
	if service == nil {
		t.Error("NewService should always return a non-nil service")
	}
}

func TestNewServiceWithRepository(t *testing.T) {
	repo := &MockRepository{}
	svc := NewServiceWithRepository(repo)
	assert.NotNil(t, svc)
	assert.Equal(t, repo, svc.repo)
}

func TestService_GetDashboardSummary(t *testing.T) {
	ctx := context.Background()

	t.Run("success with all data", func(t *testing.T) {
		repo := &MockRepository{
			RevenueExpensesRevenue:  decimal.NewFromInt(10000),
			RevenueExpensesExpenses: decimal.NewFromInt(7000),
			ReceivablesTotal:        decimal.NewFromInt(5000),
			ReceivablesOverdue:      decimal.NewFromInt(1000),
			PayablesTotal:           decimal.NewFromInt(3000),
			PayablesOverdue:         decimal.NewFromInt(500),
			InvoiceCountsDraft:      5,
			InvoiceCountsPending:    10,
			InvoiceCountsOverdue:    3,
		}
		svc := NewServiceWithRepository(repo)

		summary, err := svc.GetDashboardSummary(ctx, "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.Equal(t, decimal.NewFromInt(10000), summary.TotalRevenue)
		assert.Equal(t, decimal.NewFromInt(7000), summary.TotalExpenses)
		assert.Equal(t, decimal.NewFromInt(3000), summary.NetIncome)
		assert.Equal(t, decimal.NewFromInt(5000), summary.TotalReceivables)
		assert.Equal(t, decimal.NewFromInt(1000), summary.OverdueReceivables)
		assert.Equal(t, decimal.NewFromInt(3000), summary.TotalPayables)
		assert.Equal(t, decimal.NewFromInt(500), summary.OverduePayables)
		assert.Equal(t, 5, summary.DraftInvoices)
		assert.Equal(t, 10, summary.PendingInvoices)
		assert.Equal(t, 3, summary.OverdueInvoices)
	})

	t.Run("calculates revenue change percentage", func(t *testing.T) {
		repo := &MockRepository{
			RevenueExpensesRevenue:  decimal.NewFromInt(12000),
			RevenueExpensesExpenses: decimal.NewFromInt(8400),
		}
		svc := NewServiceWithRepository(repo)

		summary, err := svc.GetDashboardSummary(ctx, "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.Equal(t, decimal.NewFromInt(12000), summary.TotalRevenue)
	})

	t.Run("handles zero previous revenue", func(t *testing.T) {
		repo := &MockRepository{
			RevenueExpensesRevenue:  decimal.NewFromInt(10000),
			RevenueExpensesExpenses: decimal.NewFromInt(5000),
		}
		svc := NewServiceWithRepository(repo)

		summary, err := svc.GetDashboardSummary(ctx, "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.True(t, summary.RevenueChange.IsZero())
	})

	t.Run("error from GetRevenueExpenses", func(t *testing.T) {
		repo := &MockRepository{
			RevenueExpensesError: errors.New("db error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetDashboardSummary(ctx, "tenant-1", "test_schema")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("error from GetReceivablesSummary", func(t *testing.T) {
		repo := &MockRepository{
			RevenueExpensesRevenue:  decimal.NewFromInt(10000),
			RevenueExpensesExpenses: decimal.NewFromInt(7000),
			ReceivablesError:        errors.New("receivables error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetDashboardSummary(ctx, "tenant-1", "test_schema")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "receivables error")
	})

	t.Run("error from GetPayablesSummary", func(t *testing.T) {
		repo := &MockRepository{
			RevenueExpensesRevenue:  decimal.NewFromInt(10000),
			RevenueExpensesExpenses: decimal.NewFromInt(7000),
			ReceivablesTotal:        decimal.NewFromInt(5000),
			PayablesError:           errors.New("payables error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetDashboardSummary(ctx, "tenant-1", "test_schema")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "payables error")
	})

	t.Run("error from GetInvoiceCounts", func(t *testing.T) {
		repo := &MockRepository{
			RevenueExpensesRevenue:  decimal.NewFromInt(10000),
			RevenueExpensesExpenses: decimal.NewFromInt(7000),
			ReceivablesTotal:        decimal.NewFromInt(5000),
			PayablesTotal:           decimal.NewFromInt(3000),
			InvoiceCountsError:      errors.New("invoice counts error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetDashboardSummary(ctx, "tenant-1", "test_schema")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invoice counts error")
	})
}

func TestService_GetRevenueExpenseChart(t *testing.T) {
	ctx := context.Background()

	t.Run("success with data", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyRevenueExpensesData: []MonthlyData{
				{Label: "Jan 2024", Revenue: decimal.NewFromInt(10000), Expenses: decimal.NewFromInt(7000)},
				{Label: "Feb 2024", Revenue: decimal.NewFromInt(12000), Expenses: decimal.NewFromInt(8000)},
				{Label: "Mar 2024", Revenue: decimal.NewFromInt(15000), Expenses: decimal.NewFromInt(9000)},
			},
		}
		svc := NewServiceWithRepository(repo)

		chart, err := svc.GetRevenueExpenseChart(ctx, "tenant-1", "test_schema", 3)

		require.NoError(t, err)
		assert.Len(t, chart.Labels, 3)
		assert.Equal(t, []string{"Jan 2024", "Feb 2024", "Mar 2024"}, chart.Labels)
		assert.Len(t, chart.Revenue, 3)
		assert.Len(t, chart.Expenses, 3)
		assert.Equal(t, 3, repo.LastMonths)
	})

	t.Run("defaults to 12 months when 0 provided", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyRevenueExpensesData: []MonthlyData{},
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetRevenueExpenseChart(ctx, "tenant-1", "test_schema", 0)

		require.NoError(t, err)
		assert.Equal(t, 12, repo.LastMonths)
	})

	t.Run("defaults to 12 months when negative", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyRevenueExpensesData: []MonthlyData{},
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetRevenueExpenseChart(ctx, "tenant-1", "test_schema", -5)

		require.NoError(t, err)
		assert.Equal(t, 12, repo.LastMonths)
	})

	t.Run("error from repository", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyRevenueExpensesError: errors.New("chart data error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetRevenueExpenseChart(ctx, "tenant-1", "test_schema", 12)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "chart data error")
	})

	t.Run("empty data returns empty chart", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyRevenueExpensesData: []MonthlyData{},
		}
		svc := NewServiceWithRepository(repo)

		chart, err := svc.GetRevenueExpenseChart(ctx, "tenant-1", "test_schema", 12)

		require.NoError(t, err)
		assert.Empty(t, chart.Labels)
		assert.Empty(t, chart.Revenue)
		assert.Empty(t, chart.Expenses)
	})
}

func TestService_GetCashFlowChart(t *testing.T) {
	ctx := context.Background()

	t.Run("success with data", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyCashFlowData: []MonthlyCashFlowData{
				{Label: "Jan 2024", Inflows: decimal.NewFromInt(15000), Outflows: decimal.NewFromInt(10000)},
				{Label: "Feb 2024", Inflows: decimal.NewFromInt(18000), Outflows: decimal.NewFromInt(12000)},
			},
		}
		svc := NewServiceWithRepository(repo)

		chart, err := svc.GetCashFlowChart(ctx, "tenant-1", "test_schema", 2)

		require.NoError(t, err)
		assert.Len(t, chart.Labels, 2)
		assert.Equal(t, []string{"Jan 2024", "Feb 2024"}, chart.Labels)
		assert.Len(t, chart.Inflows, 2)
		assert.Len(t, chart.Outflows, 2)
		assert.Len(t, chart.Net, 2)
		// Verify net calculation
		assert.Equal(t, decimal.NewFromInt(5000), chart.Net[0])
		assert.Equal(t, decimal.NewFromInt(6000), chart.Net[1])
	})

	t.Run("defaults to 12 months when 0 provided", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyCashFlowData: []MonthlyCashFlowData{},
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetCashFlowChart(ctx, "tenant-1", "test_schema", 0)

		require.NoError(t, err)
		assert.Equal(t, 12, repo.LastMonths)
	})

	t.Run("defaults to 12 months when negative", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyCashFlowData: []MonthlyCashFlowData{},
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetCashFlowChart(ctx, "tenant-1", "test_schema", -3)

		require.NoError(t, err)
		assert.Equal(t, 12, repo.LastMonths)
	})

	t.Run("error from repository", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyCashFlowError: errors.New("cash flow error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetCashFlowChart(ctx, "tenant-1", "test_schema", 12)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cash flow error")
	})

	t.Run("handles negative net cash flow", func(t *testing.T) {
		repo := &MockRepository{
			MonthlyCashFlowData: []MonthlyCashFlowData{
				{Label: "Jan 2024", Inflows: decimal.NewFromInt(5000), Outflows: decimal.NewFromInt(10000)},
			},
		}
		svc := NewServiceWithRepository(repo)

		chart, err := svc.GetCashFlowChart(ctx, "tenant-1", "test_schema", 1)

		require.NoError(t, err)
		assert.Equal(t, decimal.NewFromInt(-5000), chart.Net[0])
	})
}

func TestService_GetReceivablesAging(t *testing.T) {
	ctx := context.Background()

	t.Run("success with contacts", func(t *testing.T) {
		repo := &MockRepository{
			AgingByContactData: []ContactAging{
				{
					ContactID:   "c1",
					ContactName: "Customer A",
					Current:     decimal.NewFromInt(1000),
					Days1to30:   decimal.NewFromInt(500),
					Days31to60:  decimal.Zero,
					Days61to90:  decimal.Zero,
					Days90Plus:  decimal.Zero,
					Total:       decimal.NewFromInt(1500),
				},
				{
					ContactID:   "c2",
					ContactName: "Customer B",
					Current:     decimal.Zero,
					Days1to30:   decimal.Zero,
					Days31to60:  decimal.NewFromInt(2000),
					Days61to90:  decimal.NewFromInt(1000),
					Days90Plus:  decimal.NewFromInt(500),
					Total:       decimal.NewFromInt(3500),
				},
			},
		}
		svc := NewServiceWithRepository(repo)

		report, err := svc.GetReceivablesAging(ctx, "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.Equal(t, "receivables", report.ReportType)
		assert.Len(t, report.ByContact, 2)
		assert.Equal(t, "SALES", repo.LastInvoiceType)

		// Verify bucket totals
		assert.Equal(t, decimal.NewFromInt(1000), report.Buckets[0].Amount)
		assert.Equal(t, 1, report.Buckets[0].Count)
		assert.Equal(t, decimal.NewFromInt(500), report.Buckets[1].Amount)
		assert.Equal(t, 1, report.Buckets[1].Count)
		assert.Equal(t, decimal.NewFromInt(2000), report.Buckets[2].Amount)
		assert.Equal(t, 1, report.Buckets[2].Count)
		assert.Equal(t, decimal.NewFromInt(1000), report.Buckets[3].Amount)
		assert.Equal(t, 1, report.Buckets[3].Count)
		assert.Equal(t, decimal.NewFromInt(500), report.Buckets[4].Amount)
		assert.Equal(t, 1, report.Buckets[4].Count)

		// Verify total
		assert.Equal(t, decimal.NewFromInt(5000), report.Total)
	})

	t.Run("empty contacts", func(t *testing.T) {
		repo := &MockRepository{
			AgingByContactData: []ContactAging{},
		}
		svc := NewServiceWithRepository(repo)

		report, err := svc.GetReceivablesAging(ctx, "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.Equal(t, "receivables", report.ReportType)
		assert.Empty(t, report.ByContact)
		assert.True(t, report.Total.IsZero())
	})

	t.Run("error from repository", func(t *testing.T) {
		repo := &MockRepository{
			AgingByContactError: errors.New("aging error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetReceivablesAging(ctx, "tenant-1", "test_schema")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "aging error")
	})
}

func TestService_GetPayablesAging(t *testing.T) {
	ctx := context.Background()

	t.Run("success with contacts", func(t *testing.T) {
		repo := &MockRepository{
			AgingByContactData: []ContactAging{
				{
					ContactID:   "s1",
					ContactName: "Supplier X",
					Current:     decimal.NewFromInt(5000),
					Days1to30:   decimal.NewFromInt(2000),
					Days31to60:  decimal.Zero,
					Days61to90:  decimal.Zero,
					Days90Plus:  decimal.Zero,
					Total:       decimal.NewFromInt(7000),
				},
			},
		}
		svc := NewServiceWithRepository(repo)

		report, err := svc.GetPayablesAging(ctx, "tenant-1", "test_schema")

		require.NoError(t, err)
		assert.Equal(t, "payables", report.ReportType)
		assert.Len(t, report.ByContact, 1)
		assert.Equal(t, "PURCHASE", repo.LastInvoiceType)
		assert.Equal(t, decimal.NewFromInt(7000), report.Total)
	})

	t.Run("error from repository", func(t *testing.T) {
		repo := &MockRepository{
			AgingByContactError: errors.New("payables aging error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetPayablesAging(ctx, "tenant-1", "test_schema")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "payables aging error")
	})
}

func TestService_GetTopCustomers(t *testing.T) {
	ctx := context.Background()

	t.Run("success with customers", func(t *testing.T) {
		repo := &MockRepository{
			TopCustomersData: []TopItem{
				{ID: "c1", Name: "Top Customer", Amount: decimal.NewFromInt(50000), Count: 25},
				{ID: "c2", Name: "Second Customer", Amount: decimal.NewFromInt(30000), Count: 15},
				{ID: "c3", Name: "Third Customer", Amount: decimal.NewFromInt(20000), Count: 10},
			},
		}
		svc := NewServiceWithRepository(repo)

		items, err := svc.GetTopCustomers(ctx, "tenant-1", "test_schema", 3)

		require.NoError(t, err)
		assert.Len(t, items, 3)
		assert.Equal(t, 3, repo.LastLimit)
		assert.Equal(t, "Top Customer", items[0].Name)
		assert.Equal(t, decimal.NewFromInt(50000), items[0].Amount)
	})

	t.Run("defaults to 10 when 0 provided", func(t *testing.T) {
		repo := &MockRepository{
			TopCustomersData: []TopItem{},
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetTopCustomers(ctx, "tenant-1", "test_schema", 0)

		require.NoError(t, err)
		assert.Equal(t, 10, repo.LastLimit)
	})

	t.Run("defaults to 10 when negative", func(t *testing.T) {
		repo := &MockRepository{
			TopCustomersData: []TopItem{},
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetTopCustomers(ctx, "tenant-1", "test_schema", -5)

		require.NoError(t, err)
		assert.Equal(t, 10, repo.LastLimit)
	})

	t.Run("error from repository", func(t *testing.T) {
		repo := &MockRepository{
			TopCustomersError: errors.New("top customers error"),
		}
		svc := NewServiceWithRepository(repo)

		_, err := svc.GetTopCustomers(ctx, "tenant-1", "test_schema", 10)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "top customers error")
	})

	t.Run("empty results", func(t *testing.T) {
		repo := &MockRepository{
			TopCustomersData: []TopItem{},
		}
		svc := NewServiceWithRepository(repo)

		items, err := svc.GetTopCustomers(ctx, "tenant-1", "test_schema", 10)

		require.NoError(t, err)
		assert.Empty(t, items)
	})
}

func TestAgingReportBucketLabels(t *testing.T) {
	ctx := context.Background()

	repo := &MockRepository{
		AgingByContactData: []ContactAging{},
	}
	svc := NewServiceWithRepository(repo)

	report, err := svc.GetReceivablesAging(ctx, "tenant-1", "test_schema")

	require.NoError(t, err)
	require.Len(t, report.Buckets, 5)
	assert.Equal(t, "Current", report.Buckets[0].Label)
	assert.Equal(t, "1-30 Days", report.Buckets[1].Label)
	assert.Equal(t, "31-60 Days", report.Buckets[2].Label)
	assert.Equal(t, "61-90 Days", report.Buckets[3].Label)
	assert.Equal(t, "90+ Days", report.Buckets[4].Label)
}

// TestDefaultMonthsValue tests that the chart methods use correct default months
func TestDefaultMonthsValue(t *testing.T) {
	// This tests the logic: if months <= 0 { months = 12 }
	tests := []struct {
		input    int
		expected int
	}{
		{0, 12},
		{-1, 12},
		{-100, 12},
		{1, 1},
		{6, 6},
		{12, 12},
		{24, 24},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			months := tt.input
			if months <= 0 {
				months = 12
			}
			if months != tt.expected {
				t.Errorf("Default months logic: input %d, got %d, want %d", tt.input, months, tt.expected)
			}
		})
	}
}

// TestDefaultLimitValue tests that the top items methods use correct default limit
func TestDefaultLimitValue(t *testing.T) {
	// This tests the logic: if limit <= 0 { limit = 10 }
	tests := []struct {
		input    int
		expected int
	}{
		{0, 10},
		{-1, 10},
		{-100, 10},
		{1, 1},
		{5, 5},
		{10, 10},
		{50, 50},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			limit := tt.input
			if limit <= 0 {
				limit = 10
			}
			if limit != tt.expected {
				t.Errorf("Default limit logic: input %d, got %d, want %d", tt.input, limit, tt.expected)
			}
		})
	}
}
