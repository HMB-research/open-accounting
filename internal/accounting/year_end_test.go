package accounting

import (
	"testing"
	"time"

	"github.com/google/uuid"
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
