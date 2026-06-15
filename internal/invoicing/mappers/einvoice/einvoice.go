package einvoice

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Invoice is a normalized Estonian e-invoice payload ready for service import.
type Invoice struct {
	ID            string
	Number        string
	Type          string
	SourceInvoice string
	Seller        Party
	Buyer         Party
	IssueDate     time.Time
	DueDate       time.Time
	Currency      string
	Reference     string
	Notes         string
	Lines         []Line
}

// Party contains contact-matching fields from an e-invoice party block.
type Party struct {
	Name         string
	RegNumber    string
	VATRegNumber string
	Email        string
}

// Line is a normalized invoice row.
type Line struct {
	Description     string
	Quantity        decimal.Decimal
	Unit            string
	UnitPrice       decimal.Decimal
	DiscountPercent decimal.Decimal
	VATRate         decimal.Decimal
}

// Parse parses Estonian e-invoice XML.
func Parse(content string) ([]Invoice, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(content, "\ufeff"))
	if trimmed == "" {
		return nil, fmt.Errorf("xml_content is required")
	}

	var document documentXML
	if err := xml.Unmarshal([]byte(trimmed), &document); err != nil {
		return nil, fmt.Errorf("parse e-invoice XML: %w", err)
	}
	if document.XMLName.Local != "E_Invoice" {
		return nil, fmt.Errorf("root element must be E_Invoice")
	}
	if len(document.Invoices) == 0 {
		return nil, fmt.Errorf("no invoices found in XML")
	}

	invoices := make([]Invoice, 0, len(document.Invoices))
	for index, raw := range document.Invoices {
		invoice, err := normalizeInvoice(raw, index+1)
		if err != nil {
			return nil, err
		}
		invoices = append(invoices, invoice)
	}
	return invoices, nil
}

type documentXML struct {
	XMLName  xml.Name
	Invoices []invoiceXML `xml:"Invoice"`
}

type invoiceXML struct {
	ID              string            `xml:"invoiceId,attr"`
	ReceiverReg     string            `xml:"regNumber,attr"`
	SellerReg       string            `xml:"sellerRegnumber,attr"`
	Parties         invoicePartiesXML `xml:"InvoiceParties"`
	Information     informationXML    `xml:"InvoiceInformation"`
	SumGroups       []sumGroupXML     `xml:"InvoiceSumGroup"`
	Items           []invoiceItemXML  `xml:"InvoiceItem"`
	PaymentInfo     paymentInfoXML    `xml:"PaymentInfo"`
	AdditionalInfos []extensionXML    `xml:"AdditionalInformation"`
}

type invoicePartiesXML struct {
	Seller partyXML `xml:"SellerParty"`
	Buyer  partyXML `xml:"BuyerParty"`
}

type partyXML struct {
	Name         string         `xml:"Name"`
	RegNumber    string         `xml:"RegNumber"`
	VATRegNumber string         `xml:"VATRegNumber"`
	ContactData  contactDataXML `xml:"ContactData"`
}

type contactDataXML struct {
	Email    string `xml:"E-mailAddress"`
	EmailAlt string `xml:"EmailAddress"`
}

type informationXML struct {
	Type                   typeXML `xml:"Type"`
	SourceInvoice          string  `xml:"SourceInvoice"`
	DocumentName           string  `xml:"DocumentName"`
	Number                 string  `xml:"InvoiceNumber"`
	ContentText            string  `xml:"InvoiceContentText"`
	PaymentReferenceNumber string  `xml:"PaymentReferenceNumber"`
	InvoiceDate            string  `xml:"InvoiceDate"`
	DueDate                string  `xml:"DueDate"`
}

type typeXML struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type sumGroupXML struct {
	Currency string   `xml:"Currency"`
	VAT      []vatXML `xml:"VAT"`
}

type invoiceItemXML struct {
	TotalGroups []itemGroupXML `xml:"InvoiceTotalGroup"`
	Groups      []itemGroupXML `xml:"InvoiceItemGroup"`
}

type itemGroupXML struct {
	Entries []itemEntryXML `xml:"ItemEntry"`
}

type itemEntryXML struct {
	Description string          `xml:"Description"`
	Details     []detailInfoXML `xml:"ItemDetailInfo"`
	ItemSum     string          `xml:"ItemSum"`
	ItemTotal   string          `xml:"ItemTotal"`
	Additions   []additionXML   `xml:"Addition"`
	VAT         vatXML          `xml:"VAT"`
}

type detailInfoXML struct {
	Unit   string `xml:"ItemUnit"`
	Amount string `xml:"ItemAmount"`
	Price  string `xml:"ItemPrice"`
}

type additionXML struct {
	Code string `xml:"addCode,attr"`
	Rate string `xml:"AddRate"`
	Sum  string `xml:"AddSum"`
}

type vatXML struct {
	Rate     string `xml:"VATRate"`
	Currency string `xml:"Currency"`
}

type paymentInfoXML struct {
	Currency     string `xml:"Currency"`
	Description  string `xml:"PaymentDescription"`
	RefID        string `xml:"PaymentRefId"`
	DueDate      string `xml:"PayDueDate"`
	PaymentID    string `xml:"PaymentId"`
	PaymentTotal string `xml:"PaymentTotalSum"`
	PayToName    string `xml:"PayToName"`
	PayerName    string `xml:"PayerName"`
	PayToAccount string `xml:"PayToAccount"`
	PayToBIC     string `xml:"PayToBIC"`
	Payable      string `xml:"Payable"`
}

type extensionXML struct {
	Name    string `xml:"InformationName"`
	Content string `xml:"InformationContent"`
}

func normalizeInvoice(raw invoiceXML, ordinal int) (Invoice, error) {
	number := firstNonEmpty(raw.Information.Number, raw.ID)
	if number == "" {
		return Invoice{}, fmt.Errorf("invoice %d: InvoiceNumber or invoiceId is required", ordinal)
	}

	issueDate, err := parseDate(raw.Information.InvoiceDate)
	if err != nil {
		return Invoice{}, fmt.Errorf("invoice %s: InvoiceDate must use YYYY-MM-DD", number)
	}
	dueDate, err := parseOptionalDate(raw.Information.DueDate)
	if err != nil {
		return Invoice{}, fmt.Errorf("invoice %s: DueDate must use YYYY-MM-DD", number)
	}
	if dueDate.IsZero() {
		dueDate, err = parseOptionalDate(raw.PaymentInfo.DueDate)
		if err != nil {
			return Invoice{}, fmt.Errorf("invoice %s: PayDueDate must use YYYY-MM-DD", number)
		}
	}
	if dueDate.IsZero() {
		dueDate = issueDate.AddDate(0, 0, 14)
	}

	lines, err := normalizeLines(raw.Items, number)
	if err != nil {
		return Invoice{}, err
	}

	seller := normalizeParty(raw.Parties.Seller)
	if seller.RegNumber == "" {
		seller.RegNumber = strings.TrimSpace(raw.SellerReg)
	}
	buyer := normalizeParty(raw.Parties.Buyer)
	if buyer.RegNumber == "" {
		buyer.RegNumber = strings.TrimSpace(raw.ReceiverReg)
	}

	return Invoice{
		ID:            strings.TrimSpace(raw.ID),
		Number:        number,
		Type:          firstNonEmpty(raw.Information.Type.Type, raw.Information.Type.Value),
		SourceInvoice: strings.TrimSpace(raw.Information.SourceInvoice),
		Seller:        seller,
		Buyer:         buyer,
		IssueDate:     issueDate,
		DueDate:       dueDate,
		Currency:      normalizeCurrency(raw),
		Reference:     firstNonEmpty(raw.Information.PaymentReferenceNumber, raw.PaymentInfo.RefID, raw.PaymentInfo.PaymentID),
		Notes:         firstNonEmpty(raw.Information.ContentText, raw.PaymentInfo.Description, additionalInfoNotes(raw.AdditionalInfos)),
		Lines:         lines,
	}, nil
}

func normalizeParty(raw partyXML) Party {
	email := firstNonEmpty(raw.ContactData.Email, raw.ContactData.EmailAlt)
	return Party{
		Name:         strings.TrimSpace(raw.Name),
		RegNumber:    strings.TrimSpace(raw.RegNumber),
		VATRegNumber: strings.TrimSpace(raw.VATRegNumber),
		Email:        strings.TrimSpace(email),
	}
}

func normalizeCurrency(raw invoiceXML) string {
	for _, group := range raw.SumGroups {
		if value := strings.ToUpper(strings.TrimSpace(group.Currency)); value != "" {
			return value
		}
		for _, vat := range group.VAT {
			if value := strings.ToUpper(strings.TrimSpace(vat.Currency)); value != "" {
				return value
			}
		}
	}
	if value := strings.ToUpper(strings.TrimSpace(raw.PaymentInfo.Currency)); value != "" {
		return value
	}
	return "EUR"
}

func normalizeLines(items []invoiceItemXML, invoiceNumber string) ([]Line, error) {
	lines := []Line{}
	for _, item := range items {
		for _, group := range append(item.TotalGroups, item.Groups...) {
			for _, entry := range group.Entries {
				line, err := normalizeLine(entry, invoiceNumber)
				if err != nil {
					return nil, err
				}
				lines = append(lines, line)
			}
		}
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("invoice %s: at least one ItemEntry is required", invoiceNumber)
	}
	return lines, nil
}

func normalizeLine(entry itemEntryXML, invoiceNumber string) (Line, error) {
	description := strings.TrimSpace(entry.Description)
	if description == "" {
		return Line{}, fmt.Errorf("invoice %s: ItemEntry Description is required", invoiceNumber)
	}

	quantity := decimal.NewFromInt(1)
	unit := ""
	unitPrice := decimal.Zero
	if len(entry.Details) > 0 {
		detail := entry.Details[0]
		unit = strings.TrimSpace(detail.Unit)
		if strings.TrimSpace(detail.Amount) != "" {
			parsed, err := parseDecimal(detail.Amount)
			if err != nil {
				return Line{}, fmt.Errorf("invoice %s: invalid ItemAmount", invoiceNumber)
			}
			quantity = parsed
		}
		if strings.TrimSpace(detail.Price) != "" {
			parsed, err := parseDecimal(detail.Price)
			if err != nil {
				return Line{}, fmt.Errorf("invoice %s: invalid ItemPrice", invoiceNumber)
			}
			unitPrice = parsed
		}
	}

	itemSum := decimal.Zero
	if strings.TrimSpace(entry.ItemSum) != "" {
		parsed, err := parseDecimal(entry.ItemSum)
		if err != nil {
			return Line{}, fmt.Errorf("invoice %s: invalid ItemSum", invoiceNumber)
		}
		itemSum = parsed
	}
	if unitPrice.IsZero() && !quantity.IsZero() && !itemSum.IsZero() {
		unitPrice = itemSum.Div(quantity)
	}
	if unitPrice.IsZero() && strings.TrimSpace(entry.ItemTotal) != "" {
		itemTotal, err := parseDecimal(entry.ItemTotal)
		if err != nil {
			return Line{}, fmt.Errorf("invoice %s: invalid ItemTotal", invoiceNumber)
		}
		vatRate, _ := parseOptionalDecimal(entry.VAT.Rate)
		divisor := decimal.NewFromInt(1).Add(vatRate.Div(decimal.NewFromInt(100)))
		if !divisor.IsZero() && !quantity.IsZero() {
			unitPrice = itemTotal.Div(divisor).Div(quantity)
		}
	}
	if quantity.LessThanOrEqual(decimal.Zero) {
		return Line{}, fmt.Errorf("invoice %s: ItemAmount must be greater than zero", invoiceNumber)
	}
	if unitPrice.IsNegative() {
		return Line{}, fmt.Errorf("invoice %s: ItemPrice cannot be negative", invoiceNumber)
	}

	vatRate, err := parseOptionalDecimal(entry.VAT.Rate)
	if err != nil {
		return Line{}, fmt.Errorf("invoice %s: invalid VATRate", invoiceNumber)
	}
	discountPercent, err := discountPercent(entry.Additions)
	if err != nil {
		return Line{}, fmt.Errorf("invoice %s: invalid AddRate", invoiceNumber)
	}

	return Line{
		Description:     description,
		Quantity:        quantity,
		Unit:            unit,
		UnitPrice:       unitPrice,
		DiscountPercent: discountPercent,
		VATRate:         vatRate,
	}, nil
}

func discountPercent(additions []additionXML) (decimal.Decimal, error) {
	for _, addition := range additions {
		if strings.EqualFold(strings.TrimSpace(addition.Code), "DSC") && strings.TrimSpace(addition.Rate) != "" {
			rate, err := parseDecimal(addition.Rate)
			if err != nil {
				return decimal.Zero, err
			}
			if rate.IsNegative() {
				rate = rate.Neg()
			}
			return rate, nil
		}
	}
	return decimal.Zero, nil
}

func parseDate(value string) (time.Time, error) {
	parsed, err := parseOptionalDate(value)
	if err != nil {
		return time.Time{}, err
	}
	if parsed.IsZero() {
		return time.Time{}, fmt.Errorf("missing date")
	}
	return parsed, nil
}

func parseOptionalDate(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), nil
}

func parseOptionalDecimal(value string) (decimal.Decimal, error) {
	if strings.TrimSpace(value) == "" {
		return decimal.Zero, nil
	}
	return parseDecimal(value)
}

func parseDecimal(value string) (decimal.Decimal, error) {
	normalized := strings.TrimSpace(value)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, ",", ".")
	return decimal.NewFromString(normalized)
}

func additionalInfoNotes(infos []extensionXML) string {
	parts := make([]string, 0, len(infos))
	for _, info := range infos {
		content := strings.TrimSpace(info.Content)
		if content == "" {
			continue
		}
		name := strings.TrimSpace(info.Name)
		if name == "" {
			parts = append(parts, content)
		} else {
			parts = append(parts, name+": "+content)
		}
	}
	return strings.Join(parts, "\n")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
