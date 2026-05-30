package invoicing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

func TestService_ImportEInvoiceXML(t *testing.T) {
	ctx := context.Background()
	schemaName := "tenant_test"
	tenantID := "tenant-1"

	t.Run("imports purchase e-invoice and matches supplier", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportEInvoiceXML(ctx, tenantID, schemaName, []contacts.Contact{
			{
				ID:               "supplier-1",
				TenantID:         tenantID,
				Name:             "Supplier OÜ",
				RegCode:          "12345678",
				VATNumber:        "EE12345678",
				ContactType:      contacts.ContactTypeSupplier,
				CountryCode:      "EE",
				PaymentTermsDays: 14,
				IsActive:         true,
			},
		}, &ImportEInvoiceRequest{
			FileName:   "supplier.xml",
			UserID:     "user-1",
			XMLContent: sampleEInvoiceXML(),
		}, nil)
		require.NoError(t, err)

		assert.Equal(t, "supplier.xml", result.FileName)
		assert.Equal(t, 1, result.RowsProcessed)
		assert.Equal(t, 1, result.InvoicesCreated)
		assert.Equal(t, 2, result.LinesImported)
		assert.Zero(t, result.RowsSkipped)
		assert.Empty(t, result.Errors)

		require.Len(t, repo.invoices, 1)
		for _, invoice := range repo.invoices {
			assert.Equal(t, "BILL-2026-001", invoice.InvoiceNumber)
			assert.Equal(t, InvoiceTypePurchase, invoice.InvoiceType)
			assert.Equal(t, "supplier-1", invoice.ContactID)
			assert.Equal(t, "RF18539007547034", invoice.Reference)
			assert.Equal(t, "Office supplies", invoice.Notes)
			assert.Equal(t, "EUR", invoice.Currency)
			assert.Equal(t, StatusSent, invoice.Status)
			require.Len(t, invoice.Lines, 2)
			assert.Equal(t, "Office chairs", invoice.Lines[0].Description)
		}
	})

	t.Run("skips duplicate invoices", func(t *testing.T) {
		repo := NewMockRepository()
		repo.invoices["existing"] = &Invoice{
			ID:            "existing",
			TenantID:      tenantID,
			InvoiceNumber: "BILL-2026-001",
			InvoiceType:   InvoiceTypePurchase,
		}
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportEInvoiceXML(ctx, tenantID, schemaName, []contacts.Contact{{
			ID:          "supplier-1",
			TenantID:    tenantID,
			Name:        "Supplier OÜ",
			RegCode:     "12345678",
			ContactType: contacts.ContactTypeSupplier,
			CountryCode: "EE",
			IsActive:    true,
		}}, &ImportEInvoiceRequest{XMLContent: sampleEInvoiceXML()}, nil)
		require.NoError(t, err)

		assert.Equal(t, 1, result.RowsProcessed)
		assert.Zero(t, result.InvoicesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "already exists")
	})

	t.Run("skips invoices blocked by period validation", func(t *testing.T) {
		repo := NewMockRepository()
		service := NewServiceWithRepository(repo, nil)

		result, err := service.ImportEInvoiceXML(ctx, tenantID, schemaName, []contacts.Contact{{
			ID:          "supplier-1",
			TenantID:    tenantID,
			Name:        "Supplier OÜ",
			RegCode:     "12345678",
			ContactType: contacts.ContactTypeSupplier,
			CountryCode: "EE",
			IsActive:    true,
		}}, &ImportEInvoiceRequest{XMLContent: sampleEInvoiceXML()}, func(issueDate time.Time) error {
			return fmt.Errorf("period locked through 2026-03-31; transaction date %s must be later", issueDate.Format("2006-01-02"))
		})
		require.NoError(t, err)

		assert.Equal(t, 1, result.RowsProcessed)
		assert.Zero(t, result.InvoicesCreated)
		assert.Equal(t, 1, result.RowsSkipped)
		require.Len(t, result.Errors, 1)
		assert.Contains(t, result.Errors[0].Message, "period locked through 2026-03-31")
		assert.Empty(t, repo.invoices)
	})
}

func sampleEInvoiceXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<E_Invoice>
  <Header>
    <Date>2026-03-15</Date>
    <FileId>file-1</FileId>
    <Version>1.2</Version>
  </Header>
  <Invoice invoiceId="BILL-2026-001" regNumber="87654321" sellerRegnumber="12345678">
    <InvoiceParties>
      <SellerParty>
        <Name>Supplier OÜ</Name>
        <RegNumber>12345678</RegNumber>
        <VATRegNumber>EE12345678</VATRegNumber>
      </SellerParty>
      <BuyerParty>
        <Name>Buyer OÜ</Name>
        <RegNumber>87654321</RegNumber>
      </BuyerParty>
    </InvoiceParties>
    <InvoiceInformation>
      <Type type="DEB"></Type>
      <DocumentName>Invoice</DocumentName>
      <InvoiceNumber>BILL-2026-001</InvoiceNumber>
      <InvoiceContentText>Office supplies</InvoiceContentText>
      <PaymentReferenceNumber>RF18539007547034</PaymentReferenceNumber>
      <InvoiceDate>2026-03-15</InvoiceDate>
      <DueDate>2026-03-29</DueDate>
    </InvoiceInformation>
    <InvoiceSumGroup>
      <VAT>
        <SumBeforeVAT>250.00</SumBeforeVAT>
        <VATRate>22</VATRate>
        <VATSum>55.00</VATSum>
        <Currency>EUR</Currency>
      </VAT>
      <TotalSum>305.00</TotalSum>
      <Currency>EUR</Currency>
    </InvoiceSumGroup>
    <InvoiceItem>
      <InvoiceItemGroup>
        <ItemEntry>
          <Description>Office chairs</Description>
          <ItemDetailInfo>
            <ItemUnit>pcs</ItemUnit>
            <ItemAmount>2</ItemAmount>
            <ItemPrice>100.00</ItemPrice>
          </ItemDetailInfo>
          <ItemSum>200.00</ItemSum>
          <VAT><VATRate>22</VATRate></VAT>
          <ItemTotal>244.00</ItemTotal>
        </ItemEntry>
        <ItemEntry>
          <Description>Setup</Description>
          <ItemDetailInfo>
            <ItemUnit>hour</ItemUnit>
            <ItemAmount>1</ItemAmount>
            <ItemPrice>50.00</ItemPrice>
          </ItemDetailInfo>
          <ItemSum>50.00</ItemSum>
          <VAT><VATRate>22</VATRate></VAT>
          <ItemTotal>61.00</ItemTotal>
        </ItemEntry>
      </InvoiceItemGroup>
    </InvoiceItem>
    <PaymentInfo>
      <Currency>EUR</Currency>
      <PayDueDate>2026-03-29</PayDueDate>
      <PaymentTotalSum>305.00</PaymentTotalSum>
      <PaymentId>RF18539007547034</PaymentId>
    </PaymentInfo>
  </Invoice>
</E_Invoice>`
}
