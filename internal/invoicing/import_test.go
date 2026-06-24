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
	"github.com/HMB-research/open-accounting/internal/importrefs"
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

	t.Run("imports invoice contact by VAT number", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "contact-vat",
				TenantID:         tenantID,
				Name:             "VAT Customer",
				VATNumber:        "EE123456789",
				ContactType:      contacts.ContactTypeCustomer,
				CountryCode:      "EE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, nil, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_vat_number,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-VAT-001,SALES,EE123456789,2026-02-01,2026-02-15,Implementation work,1,100.00,22\n",
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 1, result.InvoicesCreated)
		assert.Zero(t, result.RowsSkipped)
		assert.Empty(t, result.Errors)

		require.Len(t, repo.invoices, 1)
		for _, invoice := range repo.invoices {
			assert.Equal(t, "contact-vat", invoice.ContactID)
		}
	})

	t.Run("imports invoice contact by vat_number alias", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "contact-vat-alias",
				TenantID:         tenantID,
				Name:             "VAT Alias Customer",
				VATNumber:        "EE987654321",
				ContactType:      contacts.ContactTypeCustomer,
				CountryCode:      "EE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, nil, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,vat_number,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-VAT-ALIAS-001,SALES,EE987654321,2026-02-01,2026-02-15,Implementation work,1,100.00,22\n",
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 1, result.InvoicesCreated)
		assert.Zero(t, result.RowsSkipped)
		assert.Empty(t, result.Errors)

		require.Len(t, repo.invoices, 1)
		for _, invoice := range repo.invoices {
			assert.Equal(t, "contact-vat-alias", invoice.ContactID)
		}
	})

	t.Run("does not resolve registry code through VAT number", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "contact-vat",
				TenantID:         tenantID,
				Name:             "VAT Customer",
				VATNumber:        "EE123456789",
				ContactType:      contacts.ContactTypeCustomer,
				CountryCode:      "EE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, nil, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_reg_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-REG-001,SALES,EE123456789,2026-02-01,2026-02-15,Implementation work,1,100.00,22\n",
		}, nil)

		require.NoError(t, err)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Zero(t, result.InvoicesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, `contact_reg_code "EE123456789" was not found`)
		assert.Empty(t, repo.invoices)
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

	status, amount, err = deriveInvoiceImportStatus(StatusPaid, decimal.Zero, false, total, now, now)
	require.NoError(t, err)
	assert.Equal(t, StatusPaid, status)
	assert.True(t, amount.Equal(total))

	status, amount, err = deriveInvoiceImportStatus(StatusPartiallyPaid, decimal.RequireFromString("60.00"), true, total, now, now)
	require.NoError(t, err)
	assert.Equal(t, StatusPartiallyPaid, status)
	assert.True(t, amount.Equal(decimal.RequireFromString("60.00")))

	status, amount, err = deriveInvoiceImportStatus("", total, true, total, now, now)
	require.NoError(t, err)
	assert.Equal(t, StatusPaid, status)
	assert.True(t, amount.Equal(total))

	status, amount, err = deriveInvoiceImportStatus("", decimal.Zero, false, total, time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC), now)
	require.NoError(t, err)
	assert.Equal(t, StatusSent, status)
	assert.True(t, amount.IsZero())

	_, _, err = deriveInvoiceImportStatus("", decimal.RequireFromString("200.00"), true, total, now, now)
	require.ErrorContains(t, err, "amount_paid cannot exceed")

	_, _, err = deriveInvoiceImportStatus(StatusDraft, decimal.RequireFromString("1.00"), true, total, now, now)
	require.ErrorContains(t, err, "amount_paid must be zero")

	_, _, err = deriveInvoiceImportStatus(StatusPartiallyPaid, total, true, total, now, now)
	require.ErrorContains(t, err, "greater than zero and less than total")

	_, _, err = deriveInvoiceImportStatus(InvoiceStatus("ARCHIVED"), decimal.Zero, false, total, now, now)
	require.ErrorContains(t, err, `invalid status "ARCHIVED"`)
}

func TestInvoiceImportServiceEdges(t *testing.T) {
	ctx := context.Background()
	tenantID := "tenant-1"
	schemaName := "tenant_test"

	t.Run("rejects files with only headers", func(t *testing.T) {
		service := NewServiceWithRepository(NewMockRepository(), nil)

		_, err := service.ImportCSV(ctx, tenantID, schemaName, nil, nil, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n",
		}, nil)

		require.ErrorContains(t, err, "no invoices found in CSV")
	})

	t.Run("wraps repository list errors", func(t *testing.T) {
		repo := NewMockRepository()
		repo.ListFn = func(context.Context, string, string, *InvoiceFilter) ([]Invoice, error) {
			return nil, fmt.Errorf("database unavailable")
		}
		service := NewServiceWithRepository(repo, nil)

		_, err := service.ImportCSV(ctx, tenantID, schemaName, nil, nil, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-1,SALES,CUST-1,2026-03-01,2026-03-15,Consulting,1,100,22\n",
		}, nil)

		require.ErrorContains(t, err, "list existing invoices")
	})

	t.Run("skips duplicate imported IDs", func(t *testing.T) {
		existingID := "11111111-1111-4111-8111-111111111111"
		repo := NewMockRepository()
		repo.invoices[existingID] = &Invoice{
			ID:            existingID,
			TenantID:      tenantID,
			InvoiceNumber: "INV-EXISTING-ID",
			InvoiceType:   InvoiceTypeSales,
		}
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportInvoicesRequest{
			CSVContent: "id,invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				existingID + ",INV-NEW,SALES,CUST-1,2026-03-01,2026-03-15,Consulting,1,100,22\n",
		}, nil)

		require.NoError(t, err)
		assert.Zero(t, result.InvoicesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "already exists")
	})

	t.Run("records repository create errors as skipped rows", func(t *testing.T) {
		repo := NewMockRepository()
		repo.CreateFn = func(context.Context, string, *Invoice) error {
			return fmt.Errorf("write failed")
		}
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportCSV(ctx, tenantID, schemaName, []contacts.Contact{{
			ID:   "contact-1",
			Code: "CUST-1",
		}}, nil, &ImportInvoicesRequest{
			CSVContent: "invoice_number,invoice_type,contact_code,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
				"INV-1,SALES,CUST-1,2026-03-01,2026-03-15,Consulting,1,100,22\n",
		}, nil)

		require.NoError(t, err)
		assert.Zero(t, result.InvoicesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "write failed")
	})
}

func TestParseInvoiceImportDataRowEdges(t *testing.T) {
	validValues := map[string]string{
		"invoice_number":   "INV-1",
		"invoice_type":     "SALES",
		"contact_code":     "CUST-1",
		"issue_date":       "2026-03-01",
		"due_date":         "2026-03-15",
		"line_description": "Consulting",
		"quantity":         "1",
		"unit_price":       "100",
		"vat_rate":         "22",
	}

	tests := []struct {
		name        string
		mutate      func(map[string]string)
		wantMessage string
	}{
		{
			name:        "invoice number required",
			mutate:      func(values map[string]string) { values["invoice_number"] = "" },
			wantMessage: "invoice_number is required",
		},
		{
			name:        "contact identifier required",
			mutate:      func(values map[string]string) { values["contact_code"] = "" },
			wantMessage: "a contact identifier is required",
		},
		{
			name:        "invalid due date",
			mutate:      func(values map[string]string) { values["due_date"] = "2026/03/15" },
			wantMessage: "due_date must use YYYY-MM-DD",
		},
		{
			name:        "invalid exchange rate",
			mutate:      func(values map[string]string) { values["exchange_rate"] = "bad" },
			wantMessage: "invalid exchange_rate",
		},
		{
			name:        "zero exchange rate",
			mutate:      func(values map[string]string) { values["exchange_rate"] = "0" },
			wantMessage: "exchange_rate must be greater than zero",
		},
		{
			name:        "invalid amount paid",
			mutate:      func(values map[string]string) { values["amount_paid"] = "bad" },
			wantMessage: "invalid amount_paid",
		},
		{
			name:        "negative amount paid",
			mutate:      func(values map[string]string) { values["amount_paid"] = "-1" },
			wantMessage: "amount_paid cannot be negative",
		},
		{
			name:        "line description required",
			mutate:      func(values map[string]string) { values["line_description"] = "" },
			wantMessage: "line_description is required",
		},
		{
			name:        "negative unit price",
			mutate:      func(values map[string]string) { values["unit_price"] = "-1" },
			wantMessage: "unit_price cannot be negative",
		},
		{
			name:        "invalid discount",
			mutate:      func(values map[string]string) { values["discount_percent"] = "bad" },
			wantMessage: "invalid discount_percent",
		},
		{
			name:        "discount over limit",
			mutate:      func(values map[string]string) { values["discount_percent"] = "101" },
			wantMessage: "discount_percent must be between 0 and 100",
		},
		{
			name:        "negative VAT rate",
			mutate:      func(values map[string]string) { values["vat_rate"] = "-1" },
			wantMessage: "vat_rate cannot be negative",
		},
		{
			name: "reverse charge requires positive VAT rate",
			mutate: func(values map[string]string) {
				values["vat_rate"] = "0"
				values["vat_treatment"] = "reverse_charge"
			},
			wantMessage: "reverse charge VAT rate must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := make(map[string]string, len(validValues))
			for key, value := range validValues {
				values[key] = value
			}
			tt.mutate(values)

			_, err := parseInvoiceImportDataRow(invoiceImportRow{rowNumber: 2, values: values}, importrefs.NewProductLookup(nil))

			require.ErrorContains(t, err, tt.wantMessage)
		})
	}
}

func TestInvoiceImportParserHelpers(t *testing.T) {
	invoiceType, err := parseInvoiceImportType("SALES")
	require.NoError(t, err)
	assert.Equal(t, InvoiceTypeSales, invoiceType)

	invoiceType, err = parseInvoiceImportType("credit note")
	require.NoError(t, err)
	assert.Equal(t, InvoiceTypeCreditNote, invoiceType)

	_, err = parseInvoiceImportType("")
	require.ErrorContains(t, err, "invoice_type is required")

	_, err = parseInvoiceImportType("unsupported")
	require.ErrorContains(t, err, "invalid invoice_type")

	status, err := parseInvoiceImportStatus("")
	require.NoError(t, err)
	assert.Empty(t, status)

	status, err = parseInvoiceImportStatus("PARTIALLY_PAID")
	require.NoError(t, err)
	assert.Equal(t, StatusPartiallyPaid, status)

	status, err = parseInvoiceImportStatus("void")
	require.NoError(t, err)
	assert.Equal(t, StatusVoided, status)

	_, err = parseInvoiceImportStatus("waiting")
	require.ErrorContains(t, err, "invalid status")

	treatment, err := parseInvoiceImportVATTreatment("REVERSE-CHARGE", "")
	require.NoError(t, err)
	assert.Equal(t, VATTreatmentReverseCharge, treatment)

	_, err = parseInvoiceImportVATTreatment("outside-scope", "")
	require.ErrorContains(t, err, "invalid vat_treatment")

	_, err = parseInvoiceImportDate("2026/03/01", "issue_date")
	require.ErrorContains(t, err, "issue_date must use YYYY-MM-DD")

	_, err = parseInvoiceImportDecimal("not-a-number", "quantity")
	require.ErrorContains(t, err, "invalid quantity")

	assert.Equal(t, "invoice_number", canonicalInvoiceImportHeader(" invoice_no. "))
	assert.Empty(t, canonicalInvoiceImportHeader("legacy_only"))
	assert.Equal(t, '\t', detectInvoiceImportDelimiter("a\tb\n1\t2"))
	assert.Equal(t, ';', detectInvoiceImportDelimiter("a;b\n1;2"))
	assert.Equal(t, ',', detectInvoiceImportDelimiter("a,b\n1,2"))
}

func TestMergeInvoiceImportGroupEdges(t *testing.T) {
	issueDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	baseHeader := invoiceImportHeader{
		invoiceNumber: "INV-1",
		invoiceType:   InvoiceTypeSales,
		contactRef:    invoiceImportContactRef{code: "CUST-1"},
		issueDate:     issueDate,
		dueDate:       dueDate,
		currency:      "EUR",
		exchangeRate:  decimal.NewFromInt(1),
	}

	t.Run("fills optional header values", func(t *testing.T) {
		group := &invoiceImportGroup{header: baseHeader}
		next := baseHeader
		next.id = "11111111-1111-1111-1111-111111111111"
		next.reference = "PO-1"
		next.notes = "Imported note"
		next.explicitStatus = StatusPaid
		next.amountPaidSpecified = true
		next.amountPaid = decimal.RequireFromString("122.00")

		conflict := mergeInvoiceImportGroup(group, next, 7)

		assert.Empty(t, conflict)
		assert.Equal(t, next.id, group.header.id)
		assert.Equal(t, "PO-1", group.header.reference)
		assert.Equal(t, "Imported note", group.header.notes)
		assert.Equal(t, StatusPaid, group.header.explicitStatus)
		assert.True(t, group.header.amountPaid.Equal(decimal.RequireFromString("122.00")))
	})

	tests := []struct {
		name         string
		mutateGroup  func(*invoiceImportHeader)
		mutateNext   func(*invoiceImportHeader)
		wantConflict string
	}{
		{
			name:         "id mismatch",
			mutateGroup:  func(header *invoiceImportHeader) { header.id = "legacy-1" },
			mutateNext:   func(header *invoiceImportHeader) { header.id = "legacy-2" },
			wantConflict: "id must be consistent",
		},
		{
			name:         "invoice type mismatch",
			mutateNext:   func(header *invoiceImportHeader) { header.invoiceType = InvoiceTypePurchase },
			wantConflict: "invoice_type must be consistent",
		},
		{
			name:         "issue date mismatch",
			mutateNext:   func(header *invoiceImportHeader) { header.issueDate = issueDate.AddDate(0, 0, 1) },
			wantConflict: "issue_date must be consistent",
		},
		{
			name:         "due date mismatch",
			mutateNext:   func(header *invoiceImportHeader) { header.dueDate = dueDate.AddDate(0, 0, 1) },
			wantConflict: "due_date must be consistent",
		},
		{
			name:         "currency mismatch",
			mutateNext:   func(header *invoiceImportHeader) { header.currency = "USD" },
			wantConflict: "currency must be consistent",
		},
		{
			name:         "exchange rate mismatch",
			mutateNext:   func(header *invoiceImportHeader) { header.exchangeRate = decimal.RequireFromString("1.1") },
			wantConflict: "exchange_rate must be consistent",
		},
		{
			name:         "contact code mismatch",
			mutateNext:   func(header *invoiceImportHeader) { header.contactRef.code = "CUST-2" },
			wantConflict: "contact_code must be consistent",
		},
		{
			name:         "reference mismatch",
			mutateGroup:  func(header *invoiceImportHeader) { header.reference = "PO-1" },
			mutateNext:   func(header *invoiceImportHeader) { header.reference = "PO-2" },
			wantConflict: "reference must be consistent",
		},
		{
			name:         "notes mismatch",
			mutateGroup:  func(header *invoiceImportHeader) { header.notes = "note 1" },
			mutateNext:   func(header *invoiceImportHeader) { header.notes = "note 2" },
			wantConflict: "notes must be consistent",
		},
		{
			name:         "status mismatch",
			mutateGroup:  func(header *invoiceImportHeader) { header.explicitStatus = StatusSent },
			mutateNext:   func(header *invoiceImportHeader) { header.explicitStatus = StatusPaid },
			wantConflict: "status must be consistent",
		},
		{
			name: "amount paid mismatch",
			mutateGroup: func(header *invoiceImportHeader) {
				header.amountPaidSpecified = true
				header.amountPaid = decimal.RequireFromString("10.00")
			},
			mutateNext: func(header *invoiceImportHeader) {
				header.amountPaidSpecified = true
				header.amountPaid = decimal.RequireFromString("11.00")
			},
			wantConflict: "amount_paid must be consistent for each invoice_number (row 7)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupHeader := baseHeader
			next := baseHeader
			if tt.mutateGroup != nil {
				tt.mutateGroup(&groupHeader)
			}
			if tt.mutateNext != nil {
				tt.mutateNext(&next)
			}
			group := &invoiceImportGroup{header: groupHeader}

			conflict := mergeInvoiceImportGroup(group, next, 7)

			assert.Contains(t, conflict, tt.wantConflict)
		})
	}
}

func TestInvoiceImportContactLookupEdges(t *testing.T) {
	lookup := buildInvoiceImportContactLookup([]contacts.Contact{
		{
			ID:        "by-code",
			Code:      "CUST-1",
			Name:      "Code Customer",
			RegCode:   "100",
			VATNumber: "EE100",
			Email:     "code@example.com",
		},
		{
			ID:    "by-email",
			Email: "ops@example.com",
		},
		{
			ID:   "by-name",
			Name: "Acme OU",
		},
	})

	contact, err := lookup.find(invoiceImportContactRef{email: " OPS@example.com "})
	require.NoError(t, err)
	assert.Equal(t, "by-email", contact.ID)

	contact, err = lookup.find(invoiceImportContactRef{name: " acme ou "})
	require.NoError(t, err)
	assert.Equal(t, "by-name", contact.ID)

	_, err = lookup.find(invoiceImportContactRef{code: "missing"})
	require.ErrorContains(t, err, `contact_code "missing" was not found`)

	_, err = lookup.find(invoiceImportContactRef{vatNumber: "EE404"})
	require.ErrorContains(t, err, `contact_vat_number "EE404" was not found`)

	_, err = lookup.find(invoiceImportContactRef{email: "missing@example.com"})
	require.ErrorContains(t, err, `contact_email "missing@example.com" was not found`)

	_, err = lookup.find(invoiceImportContactRef{})
	require.ErrorContains(t, err, "a contact identifier is required")
}

func TestMergeInvoiceImportContactRefConflicts(t *testing.T) {
	tests := []struct {
		name         string
		target       invoiceImportContactRef
		next         invoiceImportContactRef
		wantConflict string
	}{
		{
			name:         "registry code",
			target:       invoiceImportContactRef{regCode: "100"},
			next:         invoiceImportContactRef{regCode: "200"},
			wantConflict: "contact_reg_code must be consistent",
		},
		{
			name:         "VAT number",
			target:       invoiceImportContactRef{vatNumber: "EE100"},
			next:         invoiceImportContactRef{vatNumber: "EE200"},
			wantConflict: "contact_vat_number must be consistent",
		},
		{
			name:         "email",
			target:       invoiceImportContactRef{email: "a@example.com"},
			next:         invoiceImportContactRef{email: "b@example.com"},
			wantConflict: "contact_email must be consistent",
		},
		{
			name:         "name",
			target:       invoiceImportContactRef{name: "Alpha"},
			next:         invoiceImportContactRef{name: "Beta"},
			wantConflict: "contact_name must be consistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflict := mergeInvoiceImportContactRef(&tt.target, tt.next)

			assert.Contains(t, conflict, tt.wantConflict)
		})
	}
}
