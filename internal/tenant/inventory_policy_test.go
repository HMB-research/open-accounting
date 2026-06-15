package tenant

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInventoryPolicyNormalization(t *testing.T) {
	tests := []struct {
		name           string
		issueInput     string
		wantIssue      string
		valuationInput string
		wantValuation  string
	}{
		{
			name:          "defaults",
			wantIssue:     InventoryIssueCostingMethodLot,
			wantValuation: InventoryValuationMethodStandardCost,
		},
		{
			name:           "friendly aliases",
			issueInput:     "weighted-average",
			wantIssue:      InventoryIssueCostingMethodWeightedAverage,
			valuationInput: "fifo",
			wantValuation:  InventoryValuationMethodFIFO,
		},
		{
			name:           "canonical values",
			issueInput:     "STANDARD_COST",
			wantIssue:      InventoryIssueCostingMethodStandardCost,
			valuationInput: "WEIGHTED_AVERAGE",
			wantValuation:  InventoryValuationMethodWeightedAverage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issueMethod, err := NormalizeInventoryIssueCostingMethod(tt.issueInput)
			require.NoError(t, err)
			assert.Equal(t, tt.wantIssue, issueMethod)

			valuationMethod, err := NormalizeInventoryValuationMethod(tt.valuationInput)
			require.NoError(t, err)
			assert.Equal(t, tt.wantValuation, valuationMethod)
		})
	}
}

func TestInventoryPolicyNormalizationRejectsInvalidValues(t *testing.T) {
	_, err := NormalizeInventoryIssueCostingMethod("lifo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid inventory issue costing method")

	_, err = NormalizeInventoryValuationMethod("replacement-cost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid inventory valuation method")
}

func TestDefaultSettingsIncludesInventoryPolicy(t *testing.T) {
	settings := DefaultSettings()

	assert.Equal(t, InventoryIssueCostingMethodLot, settings.InventoryIssueCostingMethod)
	assert.Equal(t, InventoryValuationMethodStandardCost, settings.InventoryValuationMethod)
}
