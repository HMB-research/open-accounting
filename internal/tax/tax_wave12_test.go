package tax

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTaxWave12KMDINFRemediationUsesDefaultThreshold(t *testing.T) {
	actions := BuildKMDINFRemediationActions(&KMDINFReport{
		Year:      2026,
		Month:     6,
		Threshold: decimal.Zero,
	})

	require.Len(t, actions, 1)
	require.Contains(t, actions[0].Message, KMDINFDefaultThreshold.String())
	require.True(t, strings.Contains(actions[0].CLICommand, "--threshold "+KMDINFDefaultThreshold.String()))
}
