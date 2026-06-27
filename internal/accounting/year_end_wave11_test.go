package accounting

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestYearEndWave11StatusDateAndBalanceGuards(t *testing.T) {
	parsed, err := parseYearEndStatusDate("fiscal year start", "2026-01-01")
	require.NoError(t, err)
	require.Equal(t, 2026, parsed.Year())

	_, err = parseYearEndStatusDate("fiscal year end", "not-a-date")
	require.ErrorContains(t, err, "parse fiscal year end date")

	require.NoError(t, ensureYearEndCarryForwardBalances(decimal.NewFromInt(10), decimal.NewFromInt(10)))
	require.ErrorContains(t,
		ensureYearEndCarryForwardBalances(decimal.NewFromInt(10), decimal.NewFromInt(9)),
		"carry-forward journal entry does not balance",
	)
}
