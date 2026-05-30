package einvoice

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	invoices, err := Parse(testEInvoiceXML())
	require.NoError(t, err)
	require.Len(t, invoices, 1)

	invoice := invoices[0]
	assert.Equal(t, "BILL-2026-001", invoice.Number)
	assert.Equal(t, "DEB", invoice.Type)
	assert.Equal(t, "Supplier OÜ", invoice.Seller.Name)
	assert.Equal(t, "12345678", invoice.Seller.RegNumber)
	assert.Equal(t, "supplier@example.com", invoice.Seller.Email)
	assert.Equal(t, "Buyer OÜ", invoice.Buyer.Name)
	assert.Equal(t, "EUR", invoice.Currency)
	assert.Equal(t, "RF18539007547034", invoice.Reference)
	assert.Equal(t, "Office supplies", invoice.Notes)
	assert.Equal(t, "2026-03-15", invoice.IssueDate.Format("2006-01-02"))
	assert.Equal(t, "2026-03-29", invoice.DueDate.Format("2006-01-02"))
	require.Len(t, invoice.Lines, 2)
	assert.Equal(t, "Office chairs", invoice.Lines[0].Description)
	assert.True(t, invoice.Lines[0].Quantity.Equal(decimal.RequireFromString("2")))
	assert.True(t, invoice.Lines[0].UnitPrice.Equal(decimal.RequireFromString("100.00")))
	assert.True(t, invoice.Lines[0].VATRate.Equal(decimal.RequireFromString("22")))
	assert.Equal(t, "Setup", invoice.Lines[1].Description)
	assert.True(t, invoice.Lines[1].DiscountPercent.Equal(decimal.RequireFromString("10")))
}

func TestParseRejectsInvalidRoot(t *testing.T) {
	_, err := Parse("<Invoice></Invoice>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root element must be E_Invoice")
}

func testEInvoiceXML() string {
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
        <ContactData>
          <E-mailAddress>supplier@example.com</E-mailAddress>
        </ContactData>
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
          <VAT>
            <VATRate>22</VATRate>
          </VAT>
          <ItemTotal>244.00</ItemTotal>
        </ItemEntry>
        <ItemEntry>
          <Description>Setup</Description>
          <ItemDetailInfo>
            <ItemUnit>hour</ItemUnit>
            <ItemAmount>1</ItemAmount>
            <ItemPrice>55.56</ItemPrice>
          </ItemDetailInfo>
          <ItemSum>55.56</ItemSum>
          <Addition addCode="DSC">
            <AddRate>-10</AddRate>
          </Addition>
          <VAT>
            <VATRate>22</VATRate>
          </VAT>
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
