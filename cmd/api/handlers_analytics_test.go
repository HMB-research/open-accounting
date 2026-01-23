package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// mockAnalyticsRepository implements analytics.Repository for testing
type mockAnalyticsRepository struct {
	revenue           decimal.Decimal
	expenses          decimal.Decimal
	receivablesTotal  decimal.Decimal
	receivablesOverdue decimal.Decimal
	payablesTotal     decimal.Decimal
	payablesOverdue   decimal.Decimal
	draftCount        int
	pendingCount      int
	overdueCount      int
	monthlyData       []analytics.MonthlyData
	cashFlowData      []analytics.MonthlyCashFlowData
	agingData         []analytics.ContactAging
	topCustomers      []analytics.TopItem
	activityItems     []analytics.ActivityItem

	revenueErr     error
	receivablesErr error
	payablesErr    error
	countsErr      error
	monthlyErr     error
	cashFlowErr    error
	agingErr       error
	topErr         error
	activityErr    error
}

func newMockAnalyticsRepository() *mockAnalyticsRepository {
	return &mockAnalyticsRepository{
		revenue:            decimal.NewFromInt(10000),
		expenses:           decimal.NewFromInt(5000),
		receivablesTotal:   decimal.NewFromInt(3000),
		receivablesOverdue: decimal.NewFromInt(500),
		payablesTotal:      decimal.NewFromInt(2000),
		payablesOverdue:    decimal.NewFromInt(300),
		draftCount:         5,
		pendingCount:       10,
		overdueCount:       2,
	}
}

func (m *mockAnalyticsRepository) GetRevenueExpenses(ctx context.Context, schemaName string, start, end time.Time) (revenue, expenses decimal.Decimal, err error) {
	if m.revenueErr != nil {
		return decimal.Zero, decimal.Zero, m.revenueErr
	}
	return m.revenue, m.expenses, nil
}

func (m *mockAnalyticsRepository) GetReceivablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error) {
	if m.receivablesErr != nil {
		return decimal.Zero, decimal.Zero, m.receivablesErr
	}
	return m.receivablesTotal, m.receivablesOverdue, nil
}

func (m *mockAnalyticsRepository) GetPayablesSummary(ctx context.Context, schemaName string) (total, overdue decimal.Decimal, err error) {
	if m.payablesErr != nil {
		return decimal.Zero, decimal.Zero, m.payablesErr
	}
	return m.payablesTotal, m.payablesOverdue, nil
}

func (m *mockAnalyticsRepository) GetInvoiceCounts(ctx context.Context, schemaName string) (draft, pending, overdue int, err error) {
	if m.countsErr != nil {
		return 0, 0, 0, m.countsErr
	}
	return m.draftCount, m.pendingCount, m.overdueCount, nil
}

func (m *mockAnalyticsRepository) GetMonthlyRevenueExpenses(ctx context.Context, schemaName string, months int) ([]analytics.MonthlyData, error) {
	if m.monthlyErr != nil {
		return nil, m.monthlyErr
	}
	if m.monthlyData == nil {
		return []analytics.MonthlyData{
			{Label: "Jan 2026", Revenue: decimal.NewFromInt(5000), Expenses: decimal.NewFromInt(2500)},
			{Label: "Feb 2026", Revenue: decimal.NewFromInt(6000), Expenses: decimal.NewFromInt(3000)},
		}, nil
	}
	return m.monthlyData, nil
}

func (m *mockAnalyticsRepository) GetMonthlyCashFlow(ctx context.Context, schemaName string, months int) ([]analytics.MonthlyCashFlowData, error) {
	if m.cashFlowErr != nil {
		return nil, m.cashFlowErr
	}
	if m.cashFlowData == nil {
		return []analytics.MonthlyCashFlowData{
			{Label: "Jan 2026", Inflows: decimal.NewFromInt(8000), Outflows: decimal.NewFromInt(4000)},
			{Label: "Feb 2026", Inflows: decimal.NewFromInt(9000), Outflows: decimal.NewFromInt(5000)},
		}, nil
	}
	return m.cashFlowData, nil
}

func (m *mockAnalyticsRepository) GetAgingByContact(ctx context.Context, schemaName, invoiceType string) ([]analytics.ContactAging, error) {
	if m.agingErr != nil {
		return nil, m.agingErr
	}
	if m.agingData == nil {
		return []analytics.ContactAging{
			{ContactID: "c1", ContactName: "Customer A", Current: decimal.NewFromInt(1000), Total: decimal.NewFromInt(1000)},
		}, nil
	}
	return m.agingData, nil
}

func (m *mockAnalyticsRepository) GetTopCustomers(ctx context.Context, schemaName string, limit int) ([]analytics.TopItem, error) {
	if m.topErr != nil {
		return nil, m.topErr
	}
	if m.topCustomers == nil {
		return []analytics.TopItem{
			{ID: "c1", Name: "Customer A", Amount: decimal.NewFromInt(5000), Count: 10},
		}, nil
	}
	return m.topCustomers, nil
}

func (m *mockAnalyticsRepository) GetRecentActivity(ctx context.Context, schemaName string, limit int) ([]analytics.ActivityItem, error) {
	if m.activityErr != nil {
		return nil, m.activityErr
	}
	if m.activityItems == nil {
		amount := decimal.NewFromInt(100)
		return []analytics.ActivityItem{
			{ID: "inv-1", Type: "INVOICE", Action: "created", Description: "Invoice INV-001", CreatedAt: time.Now(), Amount: &amount},
		}, nil
	}
	return m.activityItems, nil
}

func setupAnalyticsTestHandlers() (*Handlers, *mockAnalyticsRepository, *mockTenantRepository) {
	analyticsRepo := newMockAnalyticsRepository()
	analyticsSvc := analytics.NewServiceWithRepository(analyticsRepo)

	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)

	h := &Handlers{
		analyticsService: analyticsSvc,
		tenantService:    tenantSvc,
	}
	return h, analyticsRepo, tenantRepo
}

func TestGetDashboardSummary(t *testing.T) {
	h, _, tenantRepo := setupAnalyticsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/analytics/dashboard", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetDashboardSummary(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result analytics.DashboardSummary
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.True(t, result.TotalRevenue.GreaterThan(decimal.Zero))
	assert.Equal(t, 5, result.DraftInvoices)
	assert.Equal(t, 10, result.PendingInvoices)
}

func TestGetRevenueExpenseChart(t *testing.T) {
	h, _, tenantRepo := setupAnalyticsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "default period",
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "12 months",
			query:      "?months=12",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/analytics/revenue-expenses"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetRevenueExpenseChart(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result analytics.RevenueExpenseChart
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result.Labels)
			}
		})
	}
}

func TestGetCashFlowChart(t *testing.T) {
	h, _, tenantRepo := setupAnalyticsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/analytics/cash-flow", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetCashFlowChart(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result analytics.CashFlowChart
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Labels)
}

func TestGetReceivablesAging(t *testing.T) {
	h, _, tenantRepo := setupAnalyticsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/analytics/aging/receivables", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetReceivablesAging(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result analytics.AgingReport
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "receivables", result.ReportType)
}

func TestGetPayablesAging(t *testing.T) {
	h, _, tenantRepo := setupAnalyticsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/analytics/aging/payables", nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

	rr := httptest.NewRecorder()
	h.GetPayablesAging(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result analytics.AgingReport
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "payables", result.ReportType)
}

func TestGetRecentActivity(t *testing.T) {
	h, _, tenantRepo := setupAnalyticsTestHandlers()

	tenantRepo.tenants["tenant-1"] = &tenant.Tenant{
		ID:         "tenant-1",
		SchemaName: "tenant_test",
	}

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "default limit",
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "custom limit",
			query:      "?limit=5",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/analytics/activity"+tt.query, nil)
			req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
			req = req.WithContext(contextWithClaims(req.Context(), createTestClaims("user-1", "test@example.com", "tenant-1", "owner")))

			rr := httptest.NewRecorder()
			h.GetRecentActivity(rr, req)

			assert.Equal(t, tt.wantStatus, rr.Code)

			if tt.wantStatus == http.StatusOK {
				var result []analytics.ActivityItem
				err := json.Unmarshal(rr.Body.Bytes(), &result)
				require.NoError(t, err)
				assert.NotEmpty(t, result)
			}
		})
	}
}
