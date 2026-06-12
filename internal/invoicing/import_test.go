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
	"github.com/HMB-research/open-accounting/internal/inventory"
)

func TestService_ImportCSV(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"

	t.Run("imports grouped invoice lines and preserves invoice number", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)
		legacyInvoiceID := "11111111-1111-1111-1111-111111111111"

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
		}, []inventory.Product{
			{
				ID:       "prod-1",
				TenantID: tenantID,
				Code:     "SERV-001",
			},
		}, &ImportInvoicesRequest{
			FileName: "invoices.csv",
			UserID:   "user-1",
			CSVContent: "invoice_id,invoice_number,invoice_type,contact_code,issue_date,due_date,status,line_description,quantity,unit_price,vat_rate,product_code,amount_paid\n" +
				"11111111-1111-1111-1111-111111111111,INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,PAID,Implementation work,1,100.00,22,SERV-001,183.00\n" +
				"11111111-1111-1111-1111-111111111111,INV-EXT-001,SALES,CUST-001,2026-02-01,2026-02-15,PAID,Support retainer,1,50.00,22,,183.00\n",
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, "invoices.csv", result.FileName)
		assert.Equal(t, 2, result.RowsProcessed)
		assert.Equal(t, 1, result.InvoicesCreated)
		assert.Equal(t, 2, result.LinesImported)
		assert.Zero(t, result.RowsSkipped)
		assert.Empty(t, result.Errors)

		require.Len(t, repo.invoices, 1)
		invoice := repo.invoices[legacyInvoiceID]
		require.NotNil(t, invoice)
		assert.Equal(t, legacyInvoiceID, invoice.ID)
		assert.Equal(t, "INV-EXT-001", invoice.InvoiceNumber)
		assert.Equal(t, StatusPaid, invoice.Status)
		assert.True(t, invoice.AmountPaid.Equal(invoice.Total))
		require.Len(t, invoice.Lines, 2)
		assert.Equal(t, legacyInvoiceID, invoice.Lines[0].InvoiceID)
		require.NotNil(t, invoice.Lines[0].ProductID)
		assert.Equal(t, "prod-1", *invoice.Lines[0].ProductID)
	})

	t.Run("imports reverse charge purchase invoice lines", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "supplier-1",
				TenantID:         tenantID,
				Code:             "SUP-001",
				Name:             "EU Supplier",
				ContactType:      contacts.ContactTypeSupplier,
				CountryCode:      "DE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, nil, &ImportInvoicesRequest{
			FileName: "reverse-charge.csv",
			UserID:   "user-1",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,vat_treatment\n" +
				"BILL-RC-001,PURCHASE,SUP-001,2026-02-01,2026-02-15,EU service,1,100.00,22,reverse_charge\n",
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, 1, result.InvoicesCreated)
		require.Len(t, repo.invoices, 1)
		for _, invoice := range repo.invoices {
			assert.True(t, invoice.VATAmount.IsZero())
			assert.True(t, invoice.Total.Equal(decimal.RequireFromString("100")))
			require.Len(t, invoice.Lines, 1)
			assert.Equal(t, VATTreatmentReverseCharge, invoice.Lines[0].VATTreatment)
			assert.True(t, invoice.Lines[0].ReverseChargeVAT().Equal(decimal.RequireFromString("22")))
		}
	})

	t.Run("imports reverse charge boolean aliases", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "supplier-1",
				TenantID:         tenantID,
				Code:             "SUP-001",
				Name:             "EU Supplier",
				ContactType:      contacts.ContactTypeSupplier,
				CountryCode:      "DE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, nil, &ImportInvoicesRequest{
			FileName: "reverse-charge-bool.csv",
			UserID:   "user-1",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,reverse_charge\n" +
				"BILL-RC-TRUE,PURCHASE,SUP-001,2026-02-01,2026-02-15,EU service,1,100.00,22,yes\n" +
				"BILL-RC-FALSE,PURCHASE,SUP-001,2026-02-02,2026-02-16,Local service,1,100.00,22,0\n",
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, 2, result.InvoicesCreated)
		require.Len(t, repo.invoices, 2)

		var reverseChargeInvoice *Invoice
		var standardInvoice *Invoice
		for _, invoice := range repo.invoices {
			switch invoice.InvoiceNumber {
			case "BILL-RC-TRUE":
				reverseChargeInvoice = invoice
			case "BILL-RC-FALSE":
				standardInvoice = invoice
			}
		}

		require.NotNil(t, reverseChargeInvoice)
		require.Len(t, reverseChargeInvoice.Lines, 1)
		assert.Equal(t, VATTreatmentReverseCharge, reverseChargeInvoice.Lines[0].VATTreatment)
		assert.True(t, reverseChargeInvoice.VATAmount.IsZero())
		assert.True(t, reverseChargeInvoice.Total.Equal(decimal.RequireFromString("100")))

		require.NotNil(t, standardInvoice)
		require.Len(t, standardInvoice.Lines, 1)
		assert.Equal(t, VATTreatmentStandard, standardInvoice.Lines[0].VATTreatment)
		assert.True(t, standardInvoice.VATAmount.Equal(decimal.RequireFromString("22")))
		assert.True(t, standardInvoice.Total.Equal(decimal.RequireFromString("122")))
	})

	t.Run("skips invalid reverse charge boolean", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "supplier-1",
				TenantID:         tenantID,
				Code:             "SUP-001",
				Name:             "EU Supplier",
				ContactType:      contacts.ContactTypeSupplier,
				CountryCode:      "DE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, nil, &ImportInvoicesRequest{
			FileName: "reverse-charge-bad.csv",
			UserID:   "user-1",
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate,reverse_charge\n" +
				"BILL-RC-BAD,PURCHASE,SUP-001,2026-02-01,2026-02-15,EU service,1,100.00,22,maybe\n",
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, 1, result.RowsProcessed)
		assert.Zero(t, result.InvoicesCreated)
		assert.Zero(t, result.LinesImported)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 2, result.Errors[0].Row)
		assert.Equal(t, "BILL-RC-BAD", result.Errors[0].InvoiceNumber)
		assert.Contains(t, result.Errors[0].Message, "invalid reverse_charge")
		assert.Empty(t, repo.invoices)
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
		}, nil, &ImportInvoicesRequest{
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

	t.Run("skips invalid imported invoice id", func(t *testing.T) {
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
		}, nil, &ImportInvoicesRequest{
			CSVContent: "id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"legacy-id,INV-BAD-ID,SALES,CUST-001,2026-02-01,2026-02-15,Implementation work,1,100.00,22\n",
		}, nil)

		require.NoError(t, err)
		assert.Zero(t, result.InvoicesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Equal(t, 2, result.Errors[0].Row)
		assert.Contains(t, result.Errors[0].Message, "invalid id")
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
		}, nil, &ImportInvoicesRequest{
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
