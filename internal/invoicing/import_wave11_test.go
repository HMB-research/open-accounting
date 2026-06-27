package invoicing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

func TestInvoiceImportWave11RawEnumFallbacks(t *testing.T) {
	previousType, hadType := invoiceImportTypeAliases["sales"]
	delete(invoiceImportTypeAliases, "sales")
	t.Cleanup(func() {
		if hadType {
			invoiceImportTypeAliases["sales"] = previousType
		}
	})

	invoiceType, err := parseInvoiceImportType("SALES")
	require.NoError(t, err)
	require.Equal(t, InvoiceTypeSales, invoiceType)

	previousStatus, hadStatus := invoiceImportStatusAliases["paid"]
	delete(invoiceImportStatusAliases, "paid")
	t.Cleanup(func() {
		if hadStatus {
			invoiceImportStatusAliases["paid"] = previousStatus
		}
	})

	status, err := parseInvoiceImportStatus("PAID")
	require.NoError(t, err)
	require.Equal(t, StatusPaid, status)
}

func TestInvoiceImportWave11RecordsEInvoiceBuildErrors(t *testing.T) {
	previous := buildEInvoiceImportedInvoice
	buildEInvoiceImportedInvoice = func(string, string, string, *invoiceImportGroup, time.Time) (*Invoice, error) {
		return nil, errors.New("mapped invoice invalid")
	}
	t.Cleanup(func() {
		buildEInvoiceImportedInvoice = previous
	})

	repo := NewMockRepository()
	service := NewServiceWithRepository(repo, nil)

	result, err := service.ImportEInvoiceXML(
		context.Background(),
		"tenant-1",
		"tenant_test",
		[]contacts.Contact{{
			ID:          "supplier-1",
			TenantID:    "tenant-1",
			Name:        "Supplier OU",
			RegCode:     "12345678",
			ContactType: contacts.ContactTypeSupplier,
			IsActive:    true,
		}},
		&ImportEInvoiceRequest{
			FileName:   "supplier.xml",
			UserID:     "user-1",
			XMLContent: sampleEInvoiceXML(),
		},
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, "supplier.xml", result.FileName)
	assert.Equal(t, 1, result.RowsProcessed)
	assert.Zero(t, result.InvoicesCreated)
	assert.Equal(t, 1, result.RowsSkipped)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "BILL-2026-001", result.Errors[0].InvoiceNumber)
	assert.Contains(t, result.Errors[0].Message, "mapped invoice invalid")
	assert.Empty(t, repo.invoices)
}

func TestReminderRuleWave11RejectsUnknownTriggerType(t *testing.T) {
	err := (&CreateReminderRuleRequest{
		Name:        "Unknown trigger",
		TriggerType: TriggerType("AFTER_PAYMENT"),
		DaysOffset:  1,
	}).Validate()

	require.ErrorIs(t, err, ErrInvalidTriggerType)
}
