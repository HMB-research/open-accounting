package accounting

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestYearEndCarryForwardSourceID(t *testing.T) {
	periodEnd := time.Date(2025, time.December, 31, 15, 30, 0, 0, time.UTC)

	sourceID := YearEndCarryForwardSourceID("tenant-1", periodEnd)

	assert.Equal(t, sourceID, YearEndCarryForwardSourceID("tenant-1", periodEnd))
	assert.NotEqual(t, sourceID, YearEndCarryForwardSourceID("tenant-2", periodEnd))
	assert.NotEqual(t, sourceID, YearEndCarryForwardSourceID("tenant-1", periodEnd.AddDate(1, 0, 0)))
	_, err := uuid.Parse(sourceID)
	require.NoError(t, err)
}

func TestYearEndCloseEvidenceEntityID(t *testing.T) {
	entityID, err := YearEndCloseEvidenceEntityID("tenant-1", "2025-12-31")

	require.NoError(t, err)
	assert.Equal(t, entityID, yearEndCloseEvidenceEntityID("tenant-1", time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)))
	_, err = uuid.Parse(entityID)
	require.NoError(t, err)
}

func TestYearEndCloseEvidenceEntityIDRejectsInvalidDate(t *testing.T) {
	entityID, err := YearEndCloseEvidenceEntityID("tenant-1", "2025-12-30")

	assert.Empty(t, entityID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "period end date must be the last day of a month")
}

func TestPeriodCloseEvidenceEntityID(t *testing.T) {
	monthEnd, err := PeriodCloseEvidenceEntityID("tenant-1", "2025-11-30")
	require.NoError(t, err)
	yearEnd, err := PeriodCloseEvidenceEntityID("tenant-1", "2025-12-31")
	require.NoError(t, err)
	assert.NotEmpty(t, monthEnd)
	assert.NotEqual(t, monthEnd, yearEnd)

	_, err = PeriodCloseEvidenceEntityID("tenant-1", "not-a-date")
	require.Error(t, err)
	_, err = PeriodCloseEvidenceEntityID("", "2025-11-30")
	require.Error(t, err)
}

func TestIsFiscalYearEndPeriod(t *testing.T) {
	tests := []struct {
		name                 string
		periodEndDate        string
		fiscalYearStartMonth int
		expected             bool
	}{
		{
			name:                 "calendar fiscal year end",
			periodEndDate:        "2025-12-31",
			fiscalYearStartMonth: 1,
			expected:             true,
		},
		{
			name:                 "non-calendar fiscal year end",
			periodEndDate:        "2025-06-30",
			fiscalYearStartMonth: 7,
			expected:             true,
		},
		{
			name:                 "valid month end but not fiscal year end",
			periodEndDate:        "2025-12-31",
			fiscalYearStartMonth: 7,
			expected:             false,
		},
		{
			name:                 "invalid fiscal year start defaults to calendar year",
			periodEndDate:        "2025-12-31",
			fiscalYearStartMonth: 0,
			expected:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isYearEnd, err := IsFiscalYearEndPeriod(tt.periodEndDate, tt.fiscalYearStartMonth)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, isYearEnd)
		})
	}
}

func TestIsFiscalYearEndPeriodRejectsInvalidDate(t *testing.T) {
	isYearEnd, err := IsFiscalYearEndPeriod("not-a-date", 1)

	assert.False(t, isYearEnd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "period end date must use YYYY-MM-DD")
}

func TestIsFiscalYearEndPeriodRequiresDate(t *testing.T) {
	isYearEnd, err := IsFiscalYearEndPeriod("  ", 1)

	assert.False(t, isYearEnd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "period end date is required")
}

func TestYearEndCarryForwardHelpers(t *testing.T) {
	retained := &AccountSummary{ID: "retained", Code: "3200", Name: "Retained Earnings"}

	t.Run("builds balanced lines without retained earnings when P&L totals offset", func(t *testing.T) {
		lines, err := buildYearEndCarryForwardLines([]AccountBalance{
			{AccountID: "revenue", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromInt(100)},
			{AccountID: "expense", AccountType: AccountTypeExpense, NetBalance: decimal.NewFromInt(100)},
		}, nil)

		require.NoError(t, err)
		require.Len(t, lines, 2)
		assert.True(t, lines[0].DebitAmount.Equal(decimal.NewFromInt(100)))
		assert.True(t, lines[1].CreditAmount.Equal(decimal.NewFromInt(100)))
	})

	t.Run("debits retained earnings for net loss carry-forward", func(t *testing.T) {
		lines, err := buildYearEndCarryForwardLines([]AccountBalance{
			{AccountID: "revenue", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromInt(-100)},
			{AccountID: "expense", AccountType: AccountTypeExpense, NetBalance: decimal.NewFromInt(-40)},
			{AccountID: "asset", AccountType: AccountTypeAsset, NetBalance: decimal.NewFromInt(999)},
		}, retained)

		require.NoError(t, err)
		require.Len(t, lines, 3)
		assert.True(t, lines[0].CreditAmount.Equal(decimal.NewFromInt(100)))
		assert.True(t, lines[1].DebitAmount.Equal(decimal.NewFromInt(40)))
		assert.Equal(t, "retained", lines[2].AccountID)
		assert.True(t, lines[2].DebitAmount.Equal(decimal.NewFromInt(60)))
	})

	t.Run("credits retained earnings for net profit carry-forward", func(t *testing.T) {
		lines, err := buildYearEndCarryForwardLines([]AccountBalance{
			{AccountID: "revenue", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromInt(250)},
			{AccountID: "expense", AccountType: AccountTypeExpense, NetBalance: decimal.NewFromInt(90)},
		}, retained)

		require.NoError(t, err)
		require.Len(t, lines, 3)
		assert.Equal(t, "retained", lines[2].AccountID)
		assert.True(t, lines[2].CreditAmount.Equal(decimal.NewFromInt(160)))
	})

	t.Run("requires retained earnings account when carry-forward is imbalanced", func(t *testing.T) {
		lines, err := buildYearEndCarryForwardLines([]AccountBalance{
			{AccountID: "revenue", AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromInt(100)},
		}, nil)

		require.Error(t, err)
		assert.Nil(t, lines)
		assert.Contains(t, err.Error(), "retained earnings account is required")
	})

	t.Run("rejects balances without revenue or expense activity", func(t *testing.T) {
		lines, err := buildYearEndCarryForwardLines([]AccountBalance{
			{AccountID: "zero-revenue", AccountType: AccountTypeRevenue, NetBalance: decimal.Zero},
			{AccountID: "asset", AccountType: AccountTypeAsset, NetBalance: decimal.NewFromInt(100)},
		}, retained)

		require.Error(t, err)
		assert.Nil(t, lines)
		assert.Contains(t, err.Error(), "no revenue or expense activity")
	})

	t.Run("computes absolute carry-forward difference across signs", func(t *testing.T) {
		diff := carryForwardDiff([]AccountBalance{
			{AccountType: AccountTypeRevenue, NetBalance: decimal.NewFromInt(-100)},
			{AccountType: AccountTypeExpense, NetBalance: decimal.NewFromInt(-40)},
			{AccountType: AccountTypeAsset, NetBalance: decimal.NewFromInt(999)},
			{AccountType: AccountTypeRevenue, NetBalance: decimal.Zero},
		})

		assert.True(t, diff.Equal(decimal.NewFromInt(60)))
	})
}

func TestYearEndRetainedEarningsSelection(t *testing.T) {
	accounts := []Account{
		{ID: "ignored", Code: "3200", Name: "Retained Earnings", AccountType: AccountTypeLiability},
		{ID: "contains", Code: "3100", Name: "Prior retained earnings", AccountType: AccountTypeEquity},
		{ID: "exact", Code: "3200", Name: "Retained Earnings", AccountType: AccountTypeEquity},
	}

	selected := findRetainedEarningsAccount(accounts)

	require.NotNil(t, selected)
	assert.Equal(t, "exact", selected.ID)

	assert.Nil(t, findRetainedEarningsAccount([]Account{
		{ID: "equity", Code: "3000", Name: "Share Capital", AccountType: AccountTypeEquity},
	}))
}

func TestYearEndDateHelpers(t *testing.T) {
	t.Run("normalizes nil and blank lock dates", func(t *testing.T) {
		target := time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)

		closed, normalized, err := periodClosedThrough(nil, target)
		require.NoError(t, err)
		assert.False(t, closed)
		assert.Nil(t, normalized)

		blank := "  "
		closed, normalized, err = periodClosedThrough(&blank, target)
		require.NoError(t, err)
		assert.False(t, closed)
		assert.Nil(t, normalized)
	})

	t.Run("rejects invalid tenant lock date", func(t *testing.T) {
		target := time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
		raw := "not-a-date"

		closed, normalized, err := periodClosedThrough(&raw, target)

		require.Error(t, err)
		assert.False(t, closed)
		assert.Nil(t, normalized)
		assert.Contains(t, err.Error(), "invalid tenant period lock date")
	})

	t.Run("formats cross-year fiscal labels", func(t *testing.T) {
		start := time.Date(2024, time.July, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2025, time.June, 30, 0, 0, 0, 0, time.UTC)

		assert.Equal(t, "2024/2025", fiscalYearLabel(start, end))
	})
}
