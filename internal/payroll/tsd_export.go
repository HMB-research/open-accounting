package payroll

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// TSD XML structures for e-MTA submission
// Based on Estonian Tax and Customs Board TSD XML schema (01.01.2025)

// TSDDocument represents the root TSD XML document
type TSDDocument struct {
	XMLName       xml.Name       `xml:"tpiDeklaratsioon"`
	XMLNs         string         `xml:"xmlns,attr"`
	Header        TSDHeader      `xml:"dpiPais"`
	Declaration   TSDDecl        `xml:"dpiKeha"`
	Annexes       TSDLisad       `xml:"dpiLisad"`
}

// TSDHeader contains document metadata
type TSDHeader struct {
	TaxpayerRegCode string `xml:"dpiMkIsik>rpiMkIsikKood"`     // Company registry code
	TaxpayerName    string `xml:"dpiMkIsik>rpiMkIsikNimi"`     // Company name
	Period          string `xml:"dpiPeriood"`                   // YYYYMM format
	DocumentType    string `xml:"dpiDokLiik"`                   // TSD
	Version         string `xml:"dpiVersioon,omitempty"`        // Version number
}

// TSDDecl contains the main declaration body
type TSDDecl struct {
	// Summary totals
	TotalPayments      string `xml:"dpsMaksedKokku,omitempty"`      // Total payments
	TotalIncomeTax     string `xml:"dpsTm,omitempty"`               // Total income tax
	TotalSocialTax     string `xml:"dpsSm,omitempty"`               // Total social tax
	TotalUnempEE       string `xml:"dpsTkm,omitempty"`              // Total unemployment ins. employee
	TotalUnempER       string `xml:"dpsTkmTootja,omitempty"`        // Total unemployment ins. employer
	TotalPension       string `xml:"dpsKp,omitempty"`               // Total funded pension
}

// TSDLisad contains all annexes
type TSDLisad struct {
	Annex1 *TSDLisa1 `xml:"dpiLisa1,omitempty"` // Payments to resident natural persons
}

// TSDLisa1 is Annex 1 - Payments to resident natural persons
type TSDLisa1 struct {
	Rows []TSDLisa1Row `xml:"l1Rida"`
}

// TSDLisa1Row represents a single row in Annex 1
type TSDLisa1Row struct {
	RowNumber      int    `xml:"l1Jrk"`                   // Row number
	PersonalCode   string `xml:"l1Isikukood"`             // Estonian personal ID code
	FirstName      string `xml:"l1Eesnimi"`               // First name
	LastName       string `xml:"l1Perenimi"`              // Last name
	PaymentType    string `xml:"l1MakseliikKood"`         // Payment type code (10 = salary)
	GrossPayment   string `xml:"l1Mk"`                    // Gross payment
	BasicExemption string `xml:"l1Mv,omitempty"`          // Basic exemption applied
	TaxableAmount  string `xml:"l1Mmv,omitempty"`         // Taxable amount
	IncomeTax      string `xml:"l1Tm"`                    // Income tax withheld
	SocialTax      string `xml:"l1Sm"`                    // Social tax
	UnempEE        string `xml:"l1Tkm,omitempty"`         // Unemployment insurance (employee)
	UnempER        string `xml:"l1TkmTootja,omitempty"`   // Unemployment insurance (employer)
	FundedPension  string `xml:"l1Kp,omitempty"`          // Funded pension contribution
}

// TSDCompanyInfo contains company information for XML generation
type TSDCompanyInfo struct {
	RegistryCode string
	Name         string
}

// ExportTSDOptions contains options for TSD XML export
type ExportTSDOptions struct {
	Company       TSDCompanyInfo
	IncludeZeros  bool // Include rows with zero amounts
}

// ExportTSDToXML generates TSD XML for e-MTA submission
func (s *Service) ExportTSDToXML(ctx context.Context, schemaName, tenantID string, year, month int, company TSDCompanyInfo) ([]byte, error) {
	// Get the TSD declaration
	tsd, err := s.GetTSD(ctx, schemaName, tenantID, year, month)
	if err != nil {
		return nil, fmt.Errorf("get TSD: %w", err)
	}

	// Build the XML document
	doc := TSDDocument{
		XMLNs: "http://www.emta.ee/xml/tsd",
		Header: TSDHeader{
			TaxpayerRegCode: company.RegistryCode,
			TaxpayerName:    company.Name,
			Period:          fmt.Sprintf("%d%02d", year, month),
			DocumentType:    "TSD",
			Version:         "1",
		},
		Declaration: TSDDecl{
			TotalPayments:  formatDecimal(tsd.TotalPayments),
			TotalIncomeTax: formatDecimal(tsd.TotalIncomeTax),
			TotalSocialTax: formatDecimal(tsd.TotalSocialTax),
			TotalUnempEE:   formatDecimal(tsd.TotalUnemploymentEE),
			TotalUnempER:   formatDecimal(tsd.TotalUnemploymentER),
			TotalPension:   formatDecimal(tsd.TotalFundedPension),
		},
	}

	// Build Annex 1 rows
	if len(tsd.Rows) > 0 {
		annex1 := &TSDLisa1{
			Rows: make([]TSDLisa1Row, 0, len(tsd.Rows)),
		}

		for i, row := range tsd.Rows {
			xmlRow := TSDLisa1Row{
				RowNumber:      i + 1,
				PersonalCode:   row.PersonalCode,
				FirstName:      row.FirstName,
				LastName:       row.LastName,
				PaymentType:    row.PaymentType,
				GrossPayment:   formatDecimal(row.GrossPayment),
				BasicExemption: formatDecimalIfPositive(row.BasicExemption),
				TaxableAmount:  formatDecimal(row.TaxableAmount),
				IncomeTax:      formatDecimal(row.IncomeTax),
				SocialTax:      formatDecimal(row.SocialTax),
				UnempEE:        formatDecimalIfPositive(row.UnemploymentEE),
				UnempER:        formatDecimalIfPositive(row.UnemploymentER),
				FundedPension:  formatDecimalIfPositive(row.FundedPension),
			}
			annex1.Rows = append(annex1.Rows, xmlRow)
		}

		doc.Annexes.Annex1 = annex1
	}

	// Generate XML with proper formatting
	var buf bytes.Buffer
	buf.WriteString(xml.Header)

	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	if err := encoder.Encode(doc); err != nil {
		return nil, fmt.Errorf("encode XML: %w", err)
	}

	return buf.Bytes(), nil
}

// ExportTSDToCSV generates TSD data in CSV format for e-MTA
func (s *Service) ExportTSDToCSV(ctx context.Context, schemaName, tenantID string, year, month int) ([]byte, error) {
	tsd, err := s.GetTSD(ctx, schemaName, tenantID, year, month)
	if err != nil {
		return nil, fmt.Errorf("get TSD: %w", err)
	}

	var buf bytes.Buffer

	// CSV header for Annex 1
	buf.WriteString("row_number;personal_code;first_name;last_name;payment_type;gross_payment;basic_exemption;taxable_amount;income_tax;social_tax;unemployment_ee;unemployment_er;funded_pension\n")

	for i, row := range tsd.Rows {
		line := fmt.Sprintf("%d;%s;%s;%s;%s;%s;%s;%s;%s;%s;%s;%s;%s\n",
			i+1,
			row.PersonalCode,
			row.FirstName,
			row.LastName,
			row.PaymentType,
			formatDecimal(row.GrossPayment),
			formatDecimal(row.BasicExemption),
			formatDecimal(row.TaxableAmount),
			formatDecimal(row.IncomeTax),
			formatDecimal(row.SocialTax),
			formatDecimal(row.UnemploymentEE),
			formatDecimal(row.UnemploymentER),
			formatDecimal(row.FundedPension),
		)
		buf.WriteString(line)
	}

	return buf.Bytes(), nil
}

// GenerateTSDFilename creates a standard filename for TSD export
func GenerateTSDFilename(registryCode string, year, month int, format string) string {
	timestamp := time.Now().Format("20060102")
	return fmt.Sprintf("TSD_%s_%d%02d_%s.%s", registryCode, year, month, timestamp, format)
}

// formatDecimal formats a decimal for XML output (2 decimal places)
func formatDecimal(d decimal.Decimal) string {
	return d.StringFixed(2)
}

// formatDecimalIfPositive returns formatted decimal or empty string if zero/negative
func formatDecimalIfPositive(d decimal.Decimal) string {
	if d.IsPositive() {
		return d.StringFixed(2)
	}
	return ""
}

// TSD Payment Type Codes for Annex 1 (Estonian residents)
const (
	PaymentTypeSalary           = "10"  // Regular salary/wages
	PaymentTypeVacationPay      = "11"  // Vacation pay
	PaymentTypeSickPay          = "12"  // Sick pay
	PaymentTypeBonus            = "13"  // Bonuses
	PaymentTypeTermination      = "14"  // Termination compensation
	PaymentTypeBoard            = "21"  // Board member fees
	PaymentTypeContract         = "22"  // Contract work
	PaymentTypeRoyalties        = "30"  // Royalties
	PaymentTypeRent             = "40"  // Rent payments
	PaymentTypeInterest         = "50"  // Interest payments
	PaymentTypeDividends        = "60"  // Dividends (Annex 7)
	PaymentTypePension          = "70"  // Pension payments
	PaymentTypeBenefit          = "80"  // Benefits
	PaymentTypeFringeBenefit    = "42"  // Fringe benefits
)

// ValidatePersonalCode validates Estonian personal identification code (isikukood)
func ValidatePersonalCode(code string) bool {
	if len(code) != 11 {
		return false
	}

	// Check that all characters are digits
	for _, c := range code {
		if c < '0' || c > '9' {
			return false
		}
	}

	// Calculate check digit
	weights1 := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 1}
	weights2 := []int{3, 4, 5, 6, 7, 8, 9, 1, 2, 3}

	sum := 0
	for i := 0; i < 10; i++ {
		sum += int(code[i]-'0') * weights1[i]
	}

	checkDigit := sum % 11
	if checkDigit == 10 {
		sum = 0
		for i := 0; i < 10; i++ {
			sum += int(code[i]-'0') * weights2[i]
		}
		checkDigit = sum % 11
		if checkDigit == 10 {
			checkDigit = 0
		}
	}

	return int(code[10]-'0') == checkDigit
}

// TSDSummary provides a summary of TSD declaration for display
type TSDSummary struct {
	Period              string          `json:"period"`
	EmployeeCount       int             `json:"employee_count"`
	TotalGrossPayments  decimal.Decimal `json:"total_gross_payments"`
	TotalTaxes          decimal.Decimal `json:"total_taxes"`
	TotalEmployerCosts  decimal.Decimal `json:"total_employer_costs"`
	Status              TSDStatus       `json:"status"`
	SubmittedAt         *time.Time      `json:"submitted_at,omitempty"`
}

// GetTSDSummary returns a summary of TSD declaration
func (s *Service) GetTSDSummary(ctx context.Context, schemaName, tenantID string, year, month int) (*TSDSummary, error) {
	tsd, err := s.GetTSD(ctx, schemaName, tenantID, year, month)
	if err != nil {
		return nil, err
	}

	totalTaxes := tsd.TotalIncomeTax.Add(tsd.TotalUnemploymentEE).Add(tsd.TotalFundedPension)
	totalEmployerCosts := tsd.TotalSocialTax.Add(tsd.TotalUnemploymentER)

	return &TSDSummary{
		Period:             fmt.Sprintf("%d-%02d", year, month),
		EmployeeCount:      len(tsd.Rows),
		TotalGrossPayments: tsd.TotalPayments,
		TotalTaxes:         totalTaxes,
		TotalEmployerCosts: totalEmployerCosts,
		Status:             tsd.Status,
		SubmittedAt:        tsd.SubmittedAt,
	}, nil
}

// MarkTSDSubmitted marks a TSD declaration as submitted to e-MTA
func (s *Service) MarkTSDSubmitted(ctx context.Context, schemaName, tenantID, declarationID, emtaReference string) error {
	query := fmt.Sprintf(`
		UPDATE %s.tsd_declarations
		SET status = $1, submitted_at = $2, emta_reference = $3, updated_at = $4
		WHERE tenant_id = $5 AND id = $6
	`, schemaName)

	now := time.Now()
	_, err := s.db.Exec(ctx, query, TSDSubmitted, now, emtaReference, now, tenantID, declarationID)
	if err != nil {
		return fmt.Errorf("mark TSD submitted: %w", err)
	}

	return nil
}

// MarkTSDAccepted marks a TSD declaration as accepted by e-MTA
func (s *Service) MarkTSDAccepted(ctx context.Context, schemaName, tenantID, declarationID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.tsd_declarations
		SET status = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4
	`, schemaName)

	_, err := s.db.Exec(ctx, query, TSDAccepted, time.Now(), tenantID, declarationID)
	if err != nil {
		return fmt.Errorf("mark TSD accepted: %w", err)
	}

	return nil
}

// MarkTSDRejected marks a TSD declaration as rejected by e-MTA
func (s *Service) MarkTSDRejected(ctx context.Context, schemaName, tenantID, declarationID string) error {
	query := fmt.Sprintf(`
		UPDATE %s.tsd_declarations
		SET status = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4
	`, schemaName)

	_, err := s.db.Exec(ctx, query, TSDRejected, time.Now(), tenantID, declarationID)
	if err != nil {
		return fmt.Errorf("mark TSD rejected: %w", err)
	}

	return nil
}
