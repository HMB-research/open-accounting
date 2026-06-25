package payments

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBuildSEPAExportWave11WrapsMarshalErrors(t *testing.T) {
	previous := marshalSEPAXML
	marshalSEPAXML = func(any, string, string) ([]byte, error) {
		return nil, errors.New("encoder unavailable")
	}
	t.Cleanup(func() {
		marshalSEPAXML = previous
	})

	_, err := BuildSEPAExport(&SEPAExportRequest{
		MessageID:        "MSG-1",
		CreationDateTime: "2026-03-31T09:30:00Z",
		DebtorName:       "Example OU",
		DebtorIBAN:       "EE382200221020145685",
		ExecutionDate:    "2026-04-01",
		Lines: []SEPACreditTransferLine{{
			CreditorName: "Supplier AS",
			CreditorIBAN: "EE471000001020145685",
			Amount:       decimal.RequireFromString("125.50"),
		}},
	})

	require.ErrorContains(t, err, "marshal SEPA XML: encoder unavailable")
}
