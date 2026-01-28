package pdf

import (
	"fmt"
	"strings"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

// PDFSettings holds PDF-specific configuration for a tenant
type PDFSettings struct {
	PrimaryColor string `json:"primary_color"`
	FooterText   string `json:"footer_text"`
	BankDetails  string `json:"bank_details"`
	InvoiceTerms string `json:"invoice_terms"`
}

// DefaultPDFSettings returns default PDF settings
func DefaultPDFSettings() PDFSettings {
	return PDFSettings{
		PrimaryColor: "#1d4ed8",
		FooterText:   "Thank you for your business",
		BankDetails:  "",
		InvoiceTerms: "Payment due within 14 days of invoice date.",
	}
}

// PDFSettingsFromTenant extracts PDF settings from tenant settings
func (s *Service) PDFSettingsFromTenant(t *tenant.Tenant) PDFSettings {
	settings := DefaultPDFSettings()

	if t.Settings.PDFPrimaryColor != "" {
		settings.PrimaryColor = t.Settings.PDFPrimaryColor
	}
	if t.Settings.PDFFooterText != "" {
		settings.FooterText = t.Settings.PDFFooterText
	}
	if t.Settings.BankDetails != "" {
		settings.BankDetails = t.Settings.BankDetails
	}
	if t.Settings.InvoiceTerms != "" {
		settings.InvoiceTerms = t.Settings.InvoiceTerms
	}

	return settings
}

// Service handles PDF generation
type Service struct{}

// NewService creates a new PDF service
func NewService() *Service {
	return &Service{}
}

// GenerateInvoicePDF generates a PDF for the given invoice
func (s *Service) GenerateInvoicePDF(invoice *invoicing.Invoice, t *tenant.Tenant, pdfSettings PDFSettings) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageNumber(props.PageNumber{
			Pattern: "Page {current} of {total}",
			Place:   props.RightBottom,
			Size:    8,
		}).
		WithLeftMargin(15).
		WithTopMargin(15).
		WithRightMargin(15).
		Build()

	m := maroto.New(cfg)

	// Header with company info
	s.addHeader(m, t)

	// Invoice title and details
	s.addInvoiceTitle(m, invoice)

	// Bill to section
	s.addBillTo(m, invoice)

	// Line items table
	s.addLineItems(m, invoice, t)

	// Totals
	s.addTotals(m, invoice, t)

	// Payment details and notes
	s.addFooter(m, invoice, pdfSettings)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return doc.GetBytes(), nil
}

func (s *Service) addHeader(m core.Maroto, t *tenant.Tenant) {
	m.AddRow(20,
		col.New(8).Add(
			text.New(t.Name, props.Text{
				Size:  16,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		),
	)

	// Company details
	var companyDetails []string
	if t.Settings.Address != "" {
		companyDetails = append(companyDetails, t.Settings.Address)
	}
	if t.Settings.Email != "" {
		companyDetails = append(companyDetails, t.Settings.Email)
	}
	if t.Settings.Phone != "" {
		companyDetails = append(companyDetails, t.Settings.Phone)
	}
	if t.Settings.VATNumber != "" {
		companyDetails = append(companyDetails, fmt.Sprintf("VAT: %s", t.Settings.VATNumber))
	}
	if t.Settings.RegCode != "" {
		companyDetails = append(companyDetails, fmt.Sprintf("Reg: %s", t.Settings.RegCode))
	}

	for _, detail := range companyDetails {
		m.AddRow(5,
			col.New(8).Add(
				text.New(detail, props.Text{
					Size:  9,
					Align: align.Left,
				}),
			),
		)
	}

	// Separator line
	m.AddRow(5)
	m.AddRow(1,
		col.New(12).Add(
			line.New(props.Line{
				Thickness: 0.5,
			}),
		),
	)
	m.AddRow(5)
}

func (s *Service) addInvoiceTitle(m core.Maroto, invoice *invoicing.Invoice) {
	// Invoice title
	var title string
	switch invoice.InvoiceType {
	case invoicing.InvoiceTypeCreditNote:
		title = "CREDIT NOTE"
	case invoicing.InvoiceTypePurchase:
		title = "PURCHASE INVOICE"
	default:
		title = "INVOICE"
	}

	m.AddRow(12,
		col.New(6).Add(
			text.New(title, props.Text{
				Size:  20,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		),
		col.New(6).Add(
			text.New(fmt.Sprintf("No. %s", invoice.InvoiceNumber), props.Text{
				Size:  14,
				Style: fontstyle.Bold,
				Align: align.Right,
			}),
		),
	)

	// Dates
	m.AddRow(6,
		col.New(6).Add(
			text.New(fmt.Sprintf("Issue Date: %s", invoice.IssueDate.Format("02.01.2006")), props.Text{
				Size:  9,
				Align: align.Left,
			}),
		),
		col.New(6).Add(
			text.New(fmt.Sprintf("Status: %s", string(invoice.Status)), props.Text{
				Size:  9,
				Align: align.Right,
			}),
		),
	)

	m.AddRow(6,
		col.New(6).Add(
			text.New(fmt.Sprintf("Due Date: %s", invoice.DueDate.Format("02.01.2006")), props.Text{
				Size:  9,
				Align: align.Left,
			}),
		),
	)

	if invoice.Reference != "" {
		m.AddRow(6,
			col.New(6).Add(
				text.New(fmt.Sprintf("Reference: %s", invoice.Reference), props.Text{
					Size:  9,
					Align: align.Left,
				}),
			),
		)
	}

	m.AddRow(8)
}

func (s *Service) addBillTo(m core.Maroto, invoice *invoicing.Invoice) {
	m.AddRow(6,
		col.New(12).Add(
			text.New("Bill To:", props.Text{
				Size:  10,
				Style: fontstyle.Bold,
				Align: align.Left,
			}),
		),
	)

	if invoice.Contact != nil {
		c := invoice.Contact
		m.AddRow(5,
			col.New(12).Add(
				text.New(c.Name, props.Text{
					Size:  10,
					Style: fontstyle.Bold,
					Align: align.Left,
				}),
			),
		)

		if c.AddressLine1 != "" {
			m.AddRow(5,
				col.New(12).Add(
					text.New(c.AddressLine1, props.Text{
						Size:  9,
						Align: align.Left,
					}),
				),
			)
		}

		if c.AddressLine2 != "" {
			m.AddRow(5,
				col.New(12).Add(
					text.New(c.AddressLine2, props.Text{
						Size:  9,
						Align: align.Left,
					}),
				),
			)
		}

		cityLine := ""
		if c.City != "" {
			cityLine = c.City
		}
		if c.PostalCode != "" {
			if cityLine != "" {
				cityLine += ", "
			}
			cityLine += c.PostalCode
		}
		if c.CountryCode != "" {
			if cityLine != "" {
				cityLine += ", "
			}
			cityLine += c.CountryCode
		}
		if cityLine != "" {
			m.AddRow(5,
				col.New(12).Add(
					text.New(cityLine, props.Text{
						Size:  9,
						Align: align.Left,
					}),
				),
			)
		}

		if c.VATNumber != "" {
			m.AddRow(5,
				col.New(12).Add(
					text.New(fmt.Sprintf("VAT: %s", c.VATNumber), props.Text{
						Size:  9,
						Align: align.Left,
					}),
				),
			)
		}

		if c.Email != "" {
			m.AddRow(5,
				col.New(12).Add(
					text.New(c.Email, props.Text{
						Size:  9,
						Align: align.Left,
					}),
				),
			)
		}
	}

	m.AddRow(8)
}

func (s *Service) addLineItems(m core.Maroto, invoice *invoicing.Invoice, t *tenant.Tenant) {
	// Table header
	headerStyle := props.Text{
		Size:  9,
		Style: fontstyle.Bold,
		Align: align.Left,
	}
	headerStyleRight := props.Text{
		Size:  9,
		Style: fontstyle.Bold,
		Align: align.Right,
	}

	m.AddRow(7,
		col.New(1).Add(text.New("#", headerStyle)),
		col.New(4).Add(text.New("Description", headerStyle)),
		col.New(1).Add(text.New("Qty", headerStyleRight)),
		col.New(2).Add(text.New("Unit Price", headerStyleRight)),
		col.New(1).Add(text.New("VAT %", headerStyleRight)),
		col.New(1).Add(text.New("Discount", headerStyleRight)),
		col.New(2).Add(text.New("Total", headerStyleRight)),
	).WithStyle(&props.Cell{
		BackgroundColor: &props.Color{Red: 240, Green: 240, Blue: 240},
		BorderType:      border.Bottom,
		BorderThickness: 0.5,
	})

	// Table rows
	for _, line := range invoice.Lines {
		cellStyle := props.Text{
			Size:  9,
			Align: align.Left,
		}
		cellStyleRight := props.Text{
			Size:  9,
			Align: align.Right,
		}

		m.AddRow(6,
			col.New(1).Add(text.New(fmt.Sprintf("%d", line.LineNumber), cellStyle)),
			col.New(4).Add(text.New(truncateText(line.Description, 50), cellStyle)),
			col.New(1).Add(text.New(formatDecimal(line.Quantity, 2), cellStyleRight)),
			col.New(2).Add(text.New(formatMoney(line.UnitPrice, invoice.Currency), cellStyleRight)),
			col.New(1).Add(text.New(formatDecimal(line.VATRate, 0)+"%", cellStyleRight)),
			col.New(1).Add(text.New(formatDiscount(line.DiscountPercent), cellStyleRight)),
			col.New(2).Add(text.New(formatMoney(line.LineTotal, invoice.Currency), cellStyleRight)),
		).WithStyle(&props.Cell{
			BorderType:      border.Bottom,
			BorderThickness: 0.2,
		})
	}

	m.AddRow(5)
}

func (s *Service) addTotals(m core.Maroto, invoice *invoicing.Invoice, t *tenant.Tenant) {
	totalStyle := props.Text{
		Size:  10,
		Align: align.Right,
	}
	totalBoldStyle := props.Text{
		Size:  11,
		Style: fontstyle.Bold,
		Align: align.Right,
	}
	labelStyle := props.Text{
		Size:  10,
		Align: align.Left,
	}
	labelBoldStyle := props.Text{
		Size:  11,
		Style: fontstyle.Bold,
		Align: align.Left,
	}

	// Subtotal
	m.AddRow(6,
		col.New(8),
		col.New(2).Add(text.New("Subtotal:", labelStyle)),
		col.New(2).Add(text.New(formatMoney(invoice.Subtotal, invoice.Currency), totalStyle)),
	)

	// VAT
	m.AddRow(6,
		col.New(8),
		col.New(2).Add(text.New("VAT:", labelStyle)),
		col.New(2).Add(text.New(formatMoney(invoice.VATAmount, invoice.Currency), totalStyle)),
	)

	// Total
	m.AddRow(1,
		col.New(8),
		col.New(4).Add(
			line.New(props.Line{
				Thickness: 0.5,
			}),
		),
	)

	m.AddRow(8,
		col.New(8),
		col.New(2).Add(text.New("TOTAL:", labelBoldStyle)),
		col.New(2).Add(text.New(formatMoney(invoice.Total, invoice.Currency), totalBoldStyle)),
	)

	// Amount paid and due (if applicable)
	if invoice.AmountPaid.GreaterThan(decimal.Zero) {
		m.AddRow(6,
			col.New(8),
			col.New(2).Add(text.New("Paid:", labelStyle)),
			col.New(2).Add(text.New(formatMoney(invoice.AmountPaid, invoice.Currency), totalStyle)),
		)

		amountDue := invoice.AmountDue()
		m.AddRow(6,
			col.New(8),
			col.New(2).Add(text.New("Amount Due:", labelBoldStyle)),
			col.New(2).Add(text.New(formatMoney(amountDue, invoice.Currency), totalBoldStyle)),
		)
	}

	m.AddRow(10)
}

func (s *Service) addFooter(m core.Maroto, invoice *invoicing.Invoice, settings PDFSettings) {
	// Bank details
	if settings.BankDetails != "" {
		m.AddRow(6,
			col.New(12).Add(
				text.New("Payment Details:", props.Text{
					Size:  10,
					Style: fontstyle.Bold,
					Align: align.Left,
				}),
			),
		)
		for _, line := range strings.Split(settings.BankDetails, "\n") {
			m.AddRow(5,
				col.New(12).Add(
					text.New(line, props.Text{
						Size:  9,
						Align: align.Left,
					}),
				),
			)
		}
		m.AddRow(5)
	}

	// Invoice terms
	if settings.InvoiceTerms != "" {
		m.AddRow(6,
			col.New(12).Add(
				text.New("Terms & Conditions:", props.Text{
					Size:  10,
					Style: fontstyle.Bold,
					Align: align.Left,
				}),
			),
		)
		m.AddRow(5,
			col.New(12).Add(
				text.New(settings.InvoiceTerms, props.Text{
					Size:  8,
					Align: align.Left,
				}),
			),
		)
		m.AddRow(5)
	}

	// Notes
	if invoice.Notes != "" {
		m.AddRow(6,
			col.New(12).Add(
				text.New("Notes:", props.Text{
					Size:  10,
					Style: fontstyle.Bold,
					Align: align.Left,
				}),
			),
		)
		m.AddRow(5,
			col.New(12).Add(
				text.New(invoice.Notes, props.Text{
					Size:  9,
					Align: align.Left,
				}),
			),
		)
		m.AddRow(5)
	}

	// Footer text
	if settings.FooterText != "" {
		m.AddRow(10)
		m.AddRow(6,
			col.New(12).Add(
				text.New(settings.FooterText, props.Text{
					Size:  10,
					Style: fontstyle.Italic,
					Align: align.Center,
				}),
			),
		)
	}
}

// Helper functions

func formatMoney(amount decimal.Decimal, currency string) string {
	return fmt.Sprintf("%s %s", currency, amount.StringFixed(2))
}

func formatDecimal(d decimal.Decimal, precision int32) string {
	return d.StringFixed(precision)
}

func formatDiscount(d decimal.Decimal) string {
	if d.IsZero() {
		return "-"
	}
	return fmt.Sprintf("%.0f%%", d.InexactFloat64())
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
