package tax

import (
	"encoding/xml"
	"fmt"

	"github.com/shopspring/decimal"
)

// KMDXML represents the Estonian e-MTA KMD XML format
type KMDXML struct {
	XMLName    xml.Name `xml:"KMD"`
	RegNr      string   `xml:"maksukohustuslane>regNr"`
	Period     string   `xml:"periood"`
	Row1Base   string   `xml:"rida1,omitempty"`
	Row1Tax    string   `xml:"rida1Km,omitempty"`
	Row2Base   string   `xml:"rida2,omitempty"`
	Row2Tax    string   `xml:"rida2Km,omitempty"`
	Row21Base  string   `xml:"rida21,omitempty"`
	Row21Tax   string   `xml:"rida21Km,omitempty"`
	Row3       string   `xml:"rida3,omitempty"`
	Row31      string   `xml:"rida31,omitempty"`
	Row4       string   `xml:"rida4,omitempty"`
	Row5       string   `xml:"rida5,omitempty"`
	Row6       string   `xml:"rida6,omitempty"`
	Row7       string   `xml:"rida7,omitempty"`
	Row8       string   `xml:"rida8,omitempty"`  // Total output VAT
	Row9       string   `xml:"rida9,omitempty"`  // Total input VAT
	Row10      string   `xml:"rida10,omitempty"` // VAT payable
	Row11      string   `xml:"rida11,omitempty"` // VAT refundable
}

// ExportKMDToXML exports a KMD declaration to Estonian e-MTA XML format
func ExportKMDToXML(decl *KMDDeclaration, companyRegNr string) ([]byte, error) {
	kmdXML := &KMDXML{
		RegNr:  companyRegNr,
		Period: decl.Period(),
	}

	// Map rows to XML fields
	for _, row := range decl.Rows {
		taxBase := formatDecimal(row.TaxBase)
		taxAmount := formatDecimal(row.TaxAmount)

		switch row.Code {
		case KMDRow1:
			kmdXML.Row1Base = taxBase
			kmdXML.Row1Tax = taxAmount
		case KMDRow2:
			kmdXML.Row2Base = taxBase
			kmdXML.Row2Tax = taxAmount
		case KMDRow21:
			kmdXML.Row21Base = taxBase
			kmdXML.Row21Tax = taxAmount
		case KMDRow3:
			kmdXML.Row3 = taxBase
		case KMDRow31:
			kmdXML.Row31 = taxBase
		case KMDRow4:
			kmdXML.Row4 = taxAmount
		case KMDRow5:
			kmdXML.Row5 = taxAmount
		case KMDRow6:
			kmdXML.Row6 = taxAmount
		case KMDRow7:
			kmdXML.Row7 = taxAmount
		}
	}

	// Calculate totals
	kmdXML.Row8 = formatDecimal(decl.TotalOutputVAT)
	kmdXML.Row9 = formatDecimal(decl.TotalInputVAT)

	payable := decl.CalculatePayable()
	if payable.IsPositive() {
		kmdXML.Row10 = formatDecimal(payable)
	} else if payable.IsNegative() {
		kmdXML.Row11 = formatDecimal(payable.Abs())
	}

	output, err := xml.MarshalIndent(kmdXML, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal XML: %w", err)
	}

	// Add XML declaration
	return append([]byte(xml.Header), output...), nil
}

// formatDecimal formats a decimal for XML output (rounded to 2 decimal places)
func formatDecimal(d decimal.Decimal) string {
	return d.Round(2).String()
}
