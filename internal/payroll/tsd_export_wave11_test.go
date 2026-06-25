package payroll

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type payrollWave11FailingXMLEncoder struct{}

func (payrollWave11FailingXMLEncoder) Indent(string, string) {}

func (payrollWave11FailingXMLEncoder) Encode(any) error {
	return errors.New("encode unavailable")
}

func TestExportTSDToXMLWave11WrapsEncodeErrors(t *testing.T) {
	previous := newTSDXMLEncoder
	newTSDXMLEncoder = func(*bytes.Buffer) tsdXMLEncoder {
		return payrollWave11FailingXMLEncoder{}
	}
	t.Cleanup(func() {
		newTSDXMLEncoder = previous
	})

	repo := NewMockRepository()
	seedTSDForExport(repo)
	service := NewServiceWithRepository(repo, &MockUUIDGenerator{prefix: "tsd"})

	_, err := service.ExportTSDToXML(context.Background(), "tenant_schema", "tenant-1", 2025, 1, TSDCompanyInfo{
		RegistryCode: "12345678",
		Name:         "Example OU",
	})

	require.ErrorContains(t, err, "encode XML: encode unavailable")
}
