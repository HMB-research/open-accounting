package tax

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestExportKMDToXMLWave11WrapsMarshalErrors(t *testing.T) {
	previous := marshalKMDXML
	marshalKMDXML = func(any, string, string) ([]byte, error) {
		return nil, errors.New("marshal unavailable")
	}
	t.Cleanup(func() {
		marshalKMDXML = previous
	})

	_, err := ExportKMDToXML(&KMDDeclaration{
		Year:           2026,
		Month:          6,
		TotalOutputVAT: decimal.NewFromInt(10),
		TotalInputVAT:  decimal.NewFromInt(5),
	}, "12345678")

	require.ErrorContains(t, err, "marshal XML: marshal unavailable")
}
