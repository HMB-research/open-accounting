package orders

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrderImportWave11RawStatusFallback(t *testing.T) {
	previous, hadPrevious := orderImportStatusAliases["pending"]
	delete(orderImportStatusAliases, "pending")
	t.Cleanup(func() {
		if hadPrevious {
			orderImportStatusAliases["pending"] = previous
		}
	})

	status, err := parseOrderImportStatus("PENDING")

	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, status)
}
