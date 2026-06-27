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

func TestParseFallbacks(t *testing.T) {
	invoices, err := Parse("\ufeff" + fallbackEInvoiceXML())
	require.NoError(t, err)
	require.Len(t, invoices, 1)

	invoice := invoices[0]
	assert.Equal(t, "INV-FALLBACK", invoice.ID)
	assert.Equal(t, "INV-FALLBACK", invoice.Number)
	assert.Equal(t, "CRE", invoice.Type)
	assert.Equal(t, "ORIG-1", invoice.SourceInvoice)
	assert.Equal(t, "Fallback Seller", invoice.Seller.Name)
	assert.Equal(t, "11223344", invoice.Seller.RegNumber)
	assert.Equal(t, "Fallback Buyer", invoice.Buyer.Name)
	assert.Equal(t, "99887766", invoice.Buyer.RegNumber)
	assert.Equal(t, "buyer@example.com", invoice.Buyer.Email)
	assert.Equal(t, "USD", invoice.Currency)
	assert.Equal(t, "RF-FALLBACK", invoice.Reference)
	assert.Equal(t, "Memo: First note\nSecond note", invoice.Notes)
	assert.Equal(t, "2026-04-15", invoice.IssueDate.Format("2006-01-02"))
	assert.Equal(t, "2026-04-29", invoice.DueDate.Format("2006-01-02"))
	require.Len(t, invoice.Lines, 1)
	assert.Equal(t, "Derived price line", invoice.Lines[0].Description)
	assert.True(t, invoice.Lines[0].Quantity.Equal(decimal.NewFromInt(2)))
	assert.True(t, invoice.Lines[0].UnitPrice.Round(2).Equal(decimal.NewFromInt(100)))
	assert.True(t, invoice.Lines[0].VATRate.Equal(decimal.NewFromInt(22)))
}

func TestParseRejectsInvalidRoot(t *testing.T) {
	_, err := Parse("<Invoice></Invoice>")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "root element must be E_Invoice")
}

func TestParseRejectsInvalidDocuments(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty",
			content: "\ufeff   ",
			want:    "xml_content is required",
		},
		{
			name:    "malformed XML",
			content: "<E_Invoice>",
			want:    "parse e-invoice XML",
		},
		{
			name:    "no invoices",
			content: "<E_Invoice></E_Invoice>",
			want:    "no invoices found in XML",
		},
		{
			name:    "missing number",
			content: minimalEInvoice("", "<InvoiceDate>2026-03-15</InvoiceDate>", validInvoiceItemXML(), ""),
			want:    "InvoiceNumber or invoiceId is required",
		},
		{
			name:    "invalid invoice date",
			content: minimalEInvoice("INV-BAD-DATE", "<InvoiceDate>2026/03/15</InvoiceDate>", validInvoiceItemXML(), ""),
			want:    "InvoiceDate must use YYYY-MM-DD",
		},
		{
			name:    "missing invoice date",
			content: minimalEInvoice("INV-MISSING-DATE", "", validInvoiceItemXML(), ""),
			want:    "InvoiceDate must use YYYY-MM-DD",
		},
		{
			name:    "invalid due date",
			content: minimalEInvoice("INV-BAD-DUE", "<InvoiceDate>2026-03-15</InvoiceDate><DueDate>bad</DueDate>", validInvoiceItemXML(), ""),
			want:    "DueDate must use YYYY-MM-DD",
		},
		{
			name:    "invalid item entry",
			content: minimalEInvoice("INV-BAD-LINE", "<InvoiceDate>2026-03-15</InvoiceDate>", "<InvoiceItem><InvoiceItemGroup><ItemEntry></ItemEntry></InvoiceItemGroup></InvoiceItem>", ""),
			want:    "Description is required",
		},
		{
			name:    "invalid payment due date",
			content: minimalEInvoice("INV-BAD-PAY-DUE", "<InvoiceDate>2026-03-15</InvoiceDate>", validInvoiceItemXML(), "<PaymentInfo><PayDueDate>bad</PayDueDate></PaymentInfo>"),
			want:    "PayDueDate must use YYYY-MM-DD",
		},
		{
			name:    "missing item entries",
			content: minimalEInvoice("INV-NO-LINES", "<InvoiceDate>2026-03-15</InvoiceDate>", "", ""),
			want:    "at least one ItemEntry is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.content)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestNormalizeLineValidation(t *testing.T) {
	tests := []struct {
		name  string
		entry itemEntryXML
		want  string
	}{
		{
			name:  "missing description",
			entry: itemEntryXML{},
			want:  "Description is required",
		},
		{
			name: "invalid amount",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "not-number"}},
			},
			want: "invalid ItemAmount",
		},
		{
			name: "invalid price",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "1", Price: "not-number"}},
			},
			want: "invalid ItemPrice",
		},
		{
			name: "invalid item sum",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "1"}},
				ItemSum:     "not-number",
			},
			want: "invalid ItemSum",
		},
		{
			name: "invalid item total",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "2"}},
				ItemTotal:   "not-number",
			},
			want: "invalid ItemTotal",
		},
		{
			name: "zero quantity",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "0", Price: "10"}},
			},
			want: "ItemAmount must be greater than zero",
		},
		{
			name: "negative price",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "1", Price: "-10"}},
			},
			want: "ItemPrice cannot be negative",
		},
		{
			name: "invalid VAT rate",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "1", Price: "10"}},
				VAT:         vatXML{Rate: "not-number"},
			},
			want: "invalid VATRate",
		},
		{
			name: "invalid discount",
			entry: itemEntryXML{
				Description: "Line",
				Details:     []detailInfoXML{{Amount: "1", Price: "10"}},
				Additions:   []additionXML{{Code: "DSC", Rate: "not-number"}},
			},
			want: "invalid AddRate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeLine(tt.entry, "INV-ERR")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestNormalizeLineDerivesPriceAndDiscount(t *testing.T) {
	line, err := normalizeLine(itemEntryXML{
		Description: "Derived",
		Details:     []detailInfoXML{{Unit: "pcs", Amount: "2"}},
		ItemSum:     "199,98",
		Additions:   []additionXML{{Code: "dsc", Rate: "10"}},
		VAT:         vatXML{Rate: "20"},
	}, "INV-DERIVED")

	require.NoError(t, err)
	assert.Equal(t, "pcs", line.Unit)
	assert.True(t, line.UnitPrice.Equal(decimal.RequireFromString("99.99")))
	assert.True(t, line.DiscountPercent.Equal(decimal.NewFromInt(10)))
	assert.True(t, line.VATRate.Equal(decimal.NewFromInt(20)))
}

func TestNormalizeLineDerivesNetPriceFromItemTotal(t *testing.T) {
	line, err := normalizeLine(itemEntryXML{
		Description: "Total only",
		Details:     []detailInfoXML{{Amount: "2"}},
		ItemTotal:   "244.00",
		VAT:         vatXML{Rate: "22"},
	}, "INV-TOTAL")

	require.NoError(t, err)
	assert.True(t, line.UnitPrice.Equal(decimal.NewFromInt(100)))
	assert.True(t, line.Quantity.Equal(decimal.NewFromInt(2)))
}

func TestAdditionalInfoNotesKeepsNamelessContent(t *testing.T) {
	notes := additionalInfoNotes([]extensionXML{
		{Name: " Memo ", Content: " First "},
		{Content: " Second "},
		{Name: "Ignored"},
	})

	assert.Equal(t, "Memo: First\nSecond", notes)
}

func TestNormalizeCurrencyFallbacks(t *testing.T) {
	assert.Equal(t, "GBP", normalizeCurrency(invoiceXML{
		SumGroups: []sumGroupXML{{VAT: []vatXML{{Currency: "gbp"}}}},
	}))
	assert.Equal(t, "EUR", normalizeCurrency(invoiceXML{}))
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

func fallbackEInvoiceXML() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<E_Invoice>
  <Invoice invoiceId=" INV-FALLBACK " regNumber="99887766" sellerRegnumber="11223344">
    <InvoiceParties>
      <SellerParty>
        <Name>Fallback Seller</Name>
      </SellerParty>
      <BuyerParty>
        <Name>Fallback Buyer</Name>
        <ContactData>
          <EmailAddress>buyer@example.com</EmailAddress>
        </ContactData>
      </BuyerParty>
    </InvoiceParties>
    <InvoiceInformation>
      <Type>CRE</Type>
      <SourceInvoice> ORIG-1 </SourceInvoice>
      <InvoiceDate>2026-04-15</InvoiceDate>
    </InvoiceInformation>
    <InvoiceItem>
      <InvoiceTotalGroup>
        <ItemEntry>
          <Description>Derived price line</Description>
          <ItemDetailInfo>
            <ItemUnit>pcs</ItemUnit>
            <ItemAmount>2</ItemAmount>
          </ItemDetailInfo>
          <VAT>
            <VATRate>22</VATRate>
          </VAT>
          <ItemTotal>244.00</ItemTotal>
        </ItemEntry>
      </InvoiceTotalGroup>
    </InvoiceItem>
    <PaymentInfo>
      <Currency>usd</Currency>
      <PaymentRefId>RF-FALLBACK</PaymentRefId>
    </PaymentInfo>
    <AdditionalInformation>
      <InformationName>Memo</InformationName>
      <InformationContent> First note </InformationContent>
    </AdditionalInformation>
    <AdditionalInformation>
      <InformationContent>Second note</InformationContent>
    </AdditionalInformation>
  </Invoice>
</E_Invoice>`
}

func minimalEInvoice(invoiceID, information, items, payment string) string {
	return `<E_Invoice><Invoice invoiceId="` + invoiceID + `"><InvoiceInformation>` + information + `</InvoiceInformation>` + items + payment + `</Invoice></E_Invoice>`
}

func validInvoiceItemXML() string {
	return `<InvoiceItem><InvoiceItemGroup><ItemEntry><Description>Line</Description><ItemDetailInfo><ItemAmount>1</ItemAmount><ItemPrice>10</ItemPrice></ItemDetailInfo></ItemEntry></InvoiceItemGroup></InvoiceItem>`
}
