package quotes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuoteImportWave11RawStatusFallback(t *testing.T) {
	previous, hadPrevious := quoteImportStatusAliases["converted"]
	delete(quoteImportStatusAliases, "converted")
	t.Cleanup(func() {
		if hadPrevious {
			quoteImportStatusAliases["converted"] = previous
		}
	})

	status, err := parseQuoteImportStatus("CONVERTED")

	require.NoError(t, err)
	require.Equal(t, QuoteStatusConverted, status)
}
