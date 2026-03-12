package invoicing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

func TestService_ImportCSV(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"

	t.Run("imports grouped invoice lines and preserves invoice number", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "contact-1",
				TenantID:         tenantID,
				Code:             "CUST-001",
				Name:             "Acme Corp",
				ContactType:      contacts.ContactTypeCustomer,
				CountryCode:      "EE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, &ImportInvoicesRequest{
			FileName: "invoices.csv",
			UserID:   "user-1",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,status,line_description,quantity,unit_price,vat_rate,amount_paid\n" +
				"INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,PAID,Implementation work,1,100.00,22,183.00\n" +
				"INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,PAID,Support retainer,1,50.00,22,183.00\n",
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, "invoices.csv", result.FileName)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.InvoicesCreated)
		assert.Equal(t, 2, result.LinesImported)
		assert.Zero(t, result.RowsSkipped)
		assert.Empty(t, result.Errors)

		require.Len(t, repo.invoices, 1)
		for _, invoice := range repo.invoices {
			assert.Equal(t, "INV-EXT-001", invoice.InvoiceNumber)
			assert.Equal(t, StatusPaid, invoice.Status)
			assert.True(t, invoice.AmountPaid.Equal(invoice.Total))
			assert.Len(t, invoice.Lines, 2)
		}
	})

	t.Run("skips rows when contact is missing or invoice number already exists", func(t *testing.T) {
		repo := NewMockRepository()
		repo.invoices["existing"] = &Invoice{
			ID:            "existing",
			TenantID:      tenantID,
			InvoiceNumber: "INV-EXT-001",
			InvoiceType:   InvoiceTypeSales,
		}
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "contact-1",
				TenantID:         tenantID,
				Name:             "Acme Corp",
				ContactType:      contacts.ContactTypeCustomer,
				CountryCode:      "EE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_name,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-EXT-001,SALES,Acme Corp,2026-02-01,2026-02-15,Implementation work,1,100.00,22\n" +
				"INV-EXT-002,SALES,Missing Contact,2026-02-01,2026-02-15,Support,1,50.00,22\n",
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, 2, result.RowsProcessed)
		assert.Zero(t, result.InvoicesCreated)
		assert.Zero(t, result.LinesImported)
		assert.Equal(t, 2, result.RowsSkipped)
		require.Len(t, result.Errors, 2)
		assert.Contains(t, result.Errors[0].Message, "already exists")
		assert.Contains(t, result.Errors[1].Message, "was not found")
	})

	t.Run("skips invoice groups blocked by period validation", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "contact-1",
				TenantID:         tenantID,
				Code:             "CUST-001",
				Name:             "Acme Corp",
				ContactType:      contacts.ContactTypeCustomer,
				CountryCode:      "EE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-LOCKED-001,SALES,CUST-001,2026-01-10,2026-01-24,Implementation work,1,100.00,22\n",
		}, func(issueDate time.Time) error {
			return fmt.Errorf("period locked through 2026-01-31; transaction date %s must be later", issueDate.Format("2006-01-02"))
		})
		require.NoError(t, err)

		assert.Equal(t, 1, result.RowsProcessed)
		assert.Zero(t, result.InvoicesCreated)
		assert.Zero(t, result.LinesImported)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "period locked through 2026-01-31")
	})
}

func TestDeriveInvoiceImportStatus(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	total := decimal.RequireFromString("122.00")

	status, amount, err := deriveInvoiceImportStatus("", decimal.Zero, false, total, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.Equal(t, StatusOverdue, status)
	assert.True(t, amount.IsZero())

	status, amount, err = deriveInvoiceImportStatus("", decimal.RequireFromString("60.00"), true, total, now, now)
	require.NoError(t, err)
	assert.Equal(t, StatusPartiallyPaid, status)
	assert.True(t, amount.Equal(decimal.RequireFromString("60.00")))

	_, _, err = deriveInvoiceImportStatus(StatusPaid, decimal.RequireFromString("60.00"), true, total, now, now)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must equal total")
}
