package tax

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// KMDDeclaration represents an Estonian VAT declaration (Käibemaksudeklaratsioon)
type KMDDeclaration struct {
	ID             string          `json:"id"`
	TenantID       string          `json:"tenant_id"`
	Year           int             `json:"year"`
	Month          int             `json:"month"`
	Status         string          `json:"status"` // DRAFT, SUBMITTED, ACCEPTED
	TotalOutputVAT decimal.Decimal `json:"total_output_vat"`
	TotalInputVAT  decimal.Decimal `json:"total_input_vat"`
	Rows           []KMDRow        `json:"rows"`
	SubmittedAt    *time.Time      `json:"submitted_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// KMDRow represents a single row in the KMD declaration
type KMDRow struct {
	Code        string          `json:"code"`        // KMD row code (1, 2, 3, etc.)
	Description string          `json:"description"` // Row description
	TaxBase     decimal.Decimal `json:"tax_base"`    // Taxable amount (maksustatav käive)
	TaxAmount   decimal.Decimal `json:"tax_amount"`  // VAT amount (käibemaks)
}

// KMD row codes as per Estonian Tax and Customs Board
const (
	KMDRow1  = "1"  // Taxable sales at standard rate (20%/22%/24%)
	KMDRow2  = "2"  // Taxable sales at reduced rate (9%)
	KMDRow21 = "21" // Taxable sales at 13% (accommodation)
	KMDRow3  = "3"  // Zero-rated exports
	KMDRow31 = "31" // Zero-rated intra-EU supplies
	KMDRow4  = "4"  // Input VAT on domestic purchases
	KMDRow5  = "5"  // Input VAT on imports
	KMDRow6  = "6"  // Input VAT on fixed assets
	KMDRow7  = "7"  // Adjustments to input VAT
	KMDRow8  = "8"  // Output VAT payable
	KMDRow9  = "9"  // Input VAT deductible
	KMDRow10 = "10" // VAT payable to tax authority
	KMDRow11 = "11" // VAT refundable from tax authority
)

// Period returns the declaration period as YYYY-MM
func (d *KMDDeclaration) Period() string {
	return fmt.Sprintf("%d-%02d", d.Year, d.Month)
}

// CalculatePayable calculates the net VAT payable (output - input)
func (d *KMDDeclaration) CalculatePayable() decimal.Decimal {
	return d.TotalOutputVAT.Sub(d.TotalInputVAT)
}

// Validate validates a KMD row
func (r *KMDRow) Validate() error {
	if r.Code == "" {
		return fmt.Errorf("code is required")
	}
	return nil
}

// CreateKMDRequest represents a request to generate a KMD declaration
type CreateKMDRequest struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

// KMDExportFormat represents export formats
type KMDExportFormat string

const (
	KMDFormatXML  KMDExportFormat = "XML"
	KMDFormatJSON KMDExportFormat = "JSON"
)
