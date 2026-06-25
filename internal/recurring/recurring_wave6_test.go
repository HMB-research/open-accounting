package recurring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecurringWave6ParseRecurringImportRowsAdditionalEdges(t *testing.T) {
	_, err := parseRecurringImportRows(" \ufeff ")
	require.ErrorContains(t, err, "csv_content is required")

	_, err = parseRecurringImportRows("name,contact_code,frequency,start_date,line_description,quantity,unit_price,vat_rate\n\"unterminated\n")
	require.ErrorContains(t, err, "parse csv row 2")
}
