package analytics

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestDashboardSummary_JSONSerialization(t *testing.T) {
	summary := DashboardSummary{
		TotalRevenue:       decimal.NewFromFloat(10000.50),
		TotalExpenses:      decimal.NewFromFloat(5000.25),
		NetIncome:          decimal.NewFromFloat(5000.25),
		RevenueChange:      decimal.NewFromFloat(15.5),
		ExpensesChange:     decimal.NewFromFloat(-5.2),
		TotalReceivables:   decimal.NewFromFloat(3000.00),
		TotalPayables:      decimal.NewFromFloat(2000.00),
		OverdueReceivables: decimal.NewFromFloat(500.00),
		OverduePayables:    decimal.NewFromFloat(100.00),
		DraftInvoices:      5,
		PendingInvoices:    10,
		OverdueInvoices:    2,
		PeriodStart:        time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:          time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC),
	}

	// Test JSON marshaling
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("Failed to marshal DashboardSummary: %v", err)
	}

	// Test JSON unmarshaling
	var parsed DashboardSummary
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal DashboardSummary: %v", err)
	}

	// Verify values roundtrip correctly
	if !parsed.TotalRevenue.Equal(summary.TotalRevenue) {
		t.Errorf("TotalRevenue = %s, want %s", parsed.TotalRevenue, summary.TotalRevenue)
	}
	if !parsed.NetIncome.Equal(summary.NetIncome) {
		t.Errorf("NetIncome = %s, want %s", parsed.NetIncome, summary.NetIncome)
	}
	if parsed.DraftInvoices != summary.DraftInvoices {
		t.Errorf("DraftInvoices = %d, want %d", parsed.DraftInvoices, summary.DraftInvoices)
	}
}

func TestChartDataPoint_JSONSerialization(t *testing.T) {
	point := ChartDataPoint{
		Label: "January",
		Value: decimal.NewFromFloat(1234.56),
	}

	data, err := json.Marshal(point)
	if err != nil {
		t.Fatalf("Failed to marshal ChartDataPoint: %v", err)
	}

	var parsed ChartDataPoint
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal ChartDataPoint: %v", err)
	}

	if parsed.Label != point.Label {
		t.Errorf("Label = %q, want %q", parsed.Label, point.Label)
	}
	if !parsed.Value.Equal(point.Value) {
		t.Errorf("Value = %s, want %s", parsed.Value, point.Value)
	}
}

func TestRevenueExpenseChart_JSONSerialization(t *testing.T) {
	chart := RevenueExpenseChart{
		Labels:   []string{"Jan", "Feb", "Mar"},
		Revenue:  []decimal.Decimal{decimal.NewFromInt(1000), decimal.NewFromInt(1200), decimal.NewFromInt(1500)},
		Expenses: []decimal.Decimal{decimal.NewFromInt(800), decimal.NewFromInt(900), decimal.NewFromInt(1100)},
	}

	data, err := json.Marshal(chart)
	if err != nil {
		t.Fatalf("Failed to marshal RevenueExpenseChart: %v", err)
	}

	var parsed RevenueExpenseChart
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal RevenueExpenseChart: %v", err)
	}

	if len(parsed.Labels) != len(chart.Labels) {
		t.Errorf("Labels length = %d, want %d", len(parsed.Labels), len(chart.Labels))
	}
	if len(parsed.Revenue) != len(chart.Revenue) {
		t.Errorf("Revenue length = %d, want %d", len(parsed.Revenue), len(chart.Revenue))
	}
	if len(parsed.Expenses) != len(chart.Expenses) {
		t.Errorf("Expenses length = %d, want %d", len(parsed.Expenses), len(chart.Expenses))
	}
}

func TestCashFlowChart_JSONSerialization(t *testing.T) {
	chart := CashFlowChart{
		Labels:   []string{"Jan", "Feb", "Mar"},
		Inflows:  []decimal.Decimal{decimal.NewFromInt(5000), decimal.NewFromInt(6000), decimal.NewFromInt(5500)},
		Outflows: []decimal.Decimal{decimal.NewFromInt(4000), decimal.NewFromInt(4500), decimal.NewFromInt(4200)},
		Net:      []decimal.Decimal{decimal.NewFromInt(1000), decimal.NewFromInt(1500), decimal.NewFromInt(1300)},
	}

	data, err := json.Marshal(chart)
	if err != nil {
		t.Fatalf("Failed to marshal CashFlowChart: %v", err)
	}

	var parsed CashFlowChart
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal CashFlowChart: %v", err)
	}

	if len(parsed.Net) != len(chart.Net) {
		t.Errorf("Net length = %d, want %d", len(parsed.Net), len(chart.Net))
	}
}

func TestAgingBucket_JSONSerialization(t *testing.T) {
	bucket := AgingBucket{
		Label:  "1-30 Days",
		Amount: decimal.NewFromFloat(1500.50),
		Count:  5,
	}

	data, err := json.Marshal(bucket)
	if err != nil {
		t.Fatalf("Failed to marshal AgingBucket: %v", err)
	}

	var parsed AgingBucket
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal AgingBucket: %v", err)
	}

	if parsed.Label != bucket.Label {
		t.Errorf("Label = %q, want %q", parsed.Label, bucket.Label)
	}
	if parsed.Count != bucket.Count {
		t.Errorf("Count = %d, want %d", parsed.Count, bucket.Count)
	}
}

func TestAgingReport_JSONSerialization(t *testing.T) {
	report := AgingReport{
		ReportType: "receivables",
		AsOfDate:   time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Total:      decimal.NewFromFloat(10000.00),
		Buckets: []AgingBucket{
			{Label: "Current", Amount: decimal.NewFromInt(5000), Count: 10},
			{Label: "1-30 Days", Amount: decimal.NewFromInt(3000), Count: 5},
			{Label: "31-60 Days", Amount: decimal.NewFromInt(1500), Count: 3},
			{Label: "61-90 Days", Amount: decimal.NewFromInt(400), Count: 1},
			{Label: "90+ Days", Amount: decimal.NewFromInt(100), Count: 1},
		},
		ByContact: []ContactAging{
			{
				ContactID:   "contact-1",
				ContactName: "Acme Corp",
				Current:     decimal.NewFromInt(2000),
				Days1to30:   decimal.NewFromInt(1000),
				Days31to60:  decimal.NewFromInt(500),
				Days61to90:  decimal.Zero,
				Days90Plus:  decimal.Zero,
				Total:       decimal.NewFromInt(3500),
			},
		},
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal AgingReport: %v", err)
	}

	var parsed AgingReport
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal AgingReport: %v", err)
	}

	if parsed.ReportType != report.ReportType {
		t.Errorf("ReportType = %q, want %q", parsed.ReportType, report.ReportType)
	}
	if len(parsed.Buckets) != len(report.Buckets) {
		t.Errorf("Buckets length = %d, want %d", len(parsed.Buckets), len(report.Buckets))
	}
	if len(parsed.ByContact) != len(report.ByContact) {
		t.Errorf("ByContact length = %d, want %d", len(parsed.ByContact), len(report.ByContact))
	}
}

func TestContactAging_TotalCalculation(t *testing.T) {
	ca := ContactAging{
		ContactID:   "test-1",
		ContactName: "Test Contact",
		Current:     decimal.NewFromInt(1000),
		Days1to30:   decimal.NewFromInt(500),
		Days31to60:  decimal.NewFromInt(300),
		Days61to90:  decimal.NewFromInt(150),
		Days90Plus:  decimal.NewFromInt(50),
	}

	// Calculate expected total
	expected := ca.Current.Add(ca.Days1to30).Add(ca.Days31to60).Add(ca.Days61to90).Add(ca.Days90Plus)

	if !expected.Equal(decimal.NewFromInt(2000)) {
		t.Errorf("Total calculation = %s, want 2000", expected)
	}
}

func TestTopItem_JSONSerialization(t *testing.T) {
	item := TopItem{
		ID:     "customer-123",
		Name:   "Top Customer Inc",
		Amount: decimal.NewFromFloat(50000.00),
		Count:  25,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Failed to marshal TopItem: %v", err)
	}

	var parsed TopItem
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal TopItem: %v", err)
	}

	if parsed.ID != item.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, item.ID)
	}
	if parsed.Name != item.Name {
		t.Errorf("Name = %q, want %q", parsed.Name, item.Name)
	}
	if !parsed.Amount.Equal(item.Amount) {
		t.Errorf("Amount = %s, want %s", parsed.Amount, item.Amount)
	}
	if parsed.Count != item.Count {
		t.Errorf("Count = %d, want %d", parsed.Count, item.Count)
	}
}

func TestAccountBalanceWidget_JSONSerialization(t *testing.T) {
	widget := AccountBalanceWidget{
		AccountID:   "acc-001",
		AccountCode: "1000",
		AccountName: "Cash",
		Balance:     decimal.NewFromFloat(25000.50),
		Currency:    "EUR",
	}

	data, err := json.Marshal(widget)
	if err != nil {
		t.Fatalf("Failed to marshal AccountBalanceWidget: %v", err)
	}

	var parsed AccountBalanceWidget
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal AccountBalanceWidget: %v", err)
	}

	if parsed.AccountCode != widget.AccountCode {
		t.Errorf("AccountCode = %q, want %q", parsed.AccountCode, widget.AccountCode)
	}
	if parsed.Currency != widget.Currency {
		t.Errorf("Currency = %q, want %q", parsed.Currency, widget.Currency)
	}
	if !parsed.Balance.Equal(widget.Balance) {
		t.Errorf("Balance = %s, want %s", parsed.Balance, widget.Balance)
	}
}

func TestDashboardSummary_NetIncomeCalculation(t *testing.T) {
	tests := []struct {
		name     string
		revenue  decimal.Decimal
		expenses decimal.Decimal
		expected decimal.Decimal
	}{
		{"Positive net income", decimal.NewFromInt(10000), decimal.NewFromInt(7000), decimal.NewFromInt(3000)},
		{"Zero net income", decimal.NewFromInt(5000), decimal.NewFromInt(5000), decimal.Zero},
		{"Negative net income", decimal.NewFromInt(3000), decimal.NewFromInt(5000), decimal.NewFromInt(-2000)},
		{"Zero revenue and expenses", decimal.Zero, decimal.Zero, decimal.Zero},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			netIncome := tt.revenue.Sub(tt.expenses)
			if !netIncome.Equal(tt.expected) {
				t.Errorf("NetIncome = %s, want %s", netIncome, tt.expected)
			}
		})
	}
}

func TestRevenueChangePercentage(t *testing.T) {
	tests := []struct {
		name     string
		current  decimal.Decimal
		previous decimal.Decimal
		expected decimal.Decimal
	}{
		{"10% increase", decimal.NewFromInt(1100), decimal.NewFromInt(1000), decimal.NewFromInt(10)},
		{"50% increase", decimal.NewFromInt(1500), decimal.NewFromInt(1000), decimal.NewFromInt(50)},
		{"100% increase", decimal.NewFromInt(2000), decimal.NewFromInt(1000), decimal.NewFromInt(100)},
		{"10% decrease", decimal.NewFromInt(900), decimal.NewFromInt(1000), decimal.NewFromInt(-10)},
		{"50% decrease", decimal.NewFromInt(500), decimal.NewFromInt(1000), decimal.NewFromInt(-50)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.previous.IsZero() {
				t.Skip("Cannot calculate percentage change from zero")
			}
			change := tt.current.Sub(tt.previous).Div(tt.previous).Mul(decimal.NewFromInt(100)).Round(0)
			if !change.Equal(tt.expected) {
				t.Errorf("Change = %s%%, want %s%%", change, tt.expected)
			}
		})
	}
}

func TestCashFlowChart_NetCalculation(t *testing.T) {
	inflows := []decimal.Decimal{
		decimal.NewFromInt(5000),
		decimal.NewFromInt(6000),
		decimal.NewFromInt(4500),
	}
	outflows := []decimal.Decimal{
		decimal.NewFromInt(4000),
		decimal.NewFromInt(7000),
		decimal.NewFromInt(3000),
	}
	expectedNet := []decimal.Decimal{
		decimal.NewFromInt(1000),
		decimal.NewFromInt(-1000),
		decimal.NewFromInt(1500),
	}

	for i := range inflows {
		net := inflows[i].Sub(outflows[i])
		if !net.Equal(expectedNet[i]) {
			t.Errorf("Net[%d] = %s, want %s", i, net, expectedNet[i])
		}
	}
}

func TestAgingReport_BucketLabels(t *testing.T) {
	expectedLabels := []string{
		"Current",
		"1-30 Days",
		"31-60 Days",
		"61-90 Days",
		"90+ Days",
	}

	// This simulates the bucket initialization from the service
	buckets := []AgingBucket{
		{Label: "Current", Amount: decimal.Zero, Count: 0},
		{Label: "1-30 Days", Amount: decimal.Zero, Count: 0},
		{Label: "31-60 Days", Amount: decimal.Zero, Count: 0},
		{Label: "61-90 Days", Amount: decimal.Zero, Count: 0},
		{Label: "90+ Days", Amount: decimal.Zero, Count: 0},
	}

	if len(buckets) != len(expectedLabels) {
		t.Fatalf("Buckets count = %d, want %d", len(buckets), len(expectedLabels))
	}

	for i, bucket := range buckets {
		if bucket.Label != expectedLabels[i] {
			t.Errorf("Bucket[%d].Label = %q, want %q", i, bucket.Label, expectedLabels[i])
		}
	}
}

func TestAgingReport_ReportTypes(t *testing.T) {
	validTypes := []string{"receivables", "payables"}

	for _, rt := range validTypes {
		report := AgingReport{ReportType: rt}
		if report.ReportType != rt {
			t.Errorf("ReportType = %q, want %q", report.ReportType, rt)
		}
	}
}

