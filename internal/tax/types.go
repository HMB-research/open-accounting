package tax

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// KMDDeclaration represents an Estonian VAT declaration (Käibemaksudeklaratsioon)
type KMDDeclaration struct {
	ID                 string                 `json:"id"`
	TenantID           string                 `json:"tenant_id"`
	Year               int                    `json:"year"`
	Month              int                    `json:"month"`
	Status             string                 `json:"status"` // DRAFT, SUBMITTED, ACCEPTED
	TotalOutputVAT     decimal.Decimal        `json:"total_output_vat"`
	TotalInputVAT      decimal.Decimal        `json:"total_input_vat"`
	Rows               []KMDRow               `json:"rows"`
	RemediationActions []KMDRemediationAction `json:"remediation_actions,omitempty"`
	SubmittedAt        *time.Time             `json:"submitted_at,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// KMDRemediationAction describes one operator action for VAT declaration review or submission.
type KMDRemediationAction struct {
	Code           string `json:"code"`
	Severity       string `json:"severity"`
	Scope          string `json:"scope"`
	OwnerRole      string `json:"owner_role"`
	WorkspaceQueue string `json:"workspace_queue,omitempty"`
	AssignmentKey  string `json:"assignment_key,omitempty"`
	Priority       string `json:"priority,omitempty"`
	DueInDays      int    `json:"due_in_days,omitempty"`
	Message        string `json:"message"`
	Action         string `json:"action"`
	Period         string `json:"period,omitempty"`
	EntityType     string `json:"entity_type,omitempty"`
	EntityID       string `json:"entity_id,omitempty"`
	UIPath         string `json:"ui_path,omitempty"`
	CLICommand     string `json:"cli_command,omitempty"`
}

// KMDINFPart identifies the sales or purchase side of the KMD INF appendix.
type KMDINFPart string

const (
	KMDINFPartSales     KMDINFPart = "A"
	KMDINFPartPurchases KMDINFPart = "B"
)

// KMDINFDefaultThreshold is the partner-period threshold excluding VAT.
var KMDINFDefaultThreshold = decimal.NewFromInt(1000)

// KMDINFReport contains invoice-level KMD INF appendix rows for a VAT period.
type KMDINFReport struct {
	TenantID    string              `json:"tenant_id"`
	Year        int                 `json:"year"`
	Month       int                 `json:"month"`
	Threshold   decimal.Decimal     `json:"threshold"`
	GeneratedAt time.Time           `json:"generated_at"`
	Summary     []KMDINFPartSummary `json:"summary"`
	Rows        []KMDINFReportRow   `json:"rows"`
}

// KMDINFPartSummary summarizes one KMD INF appendix part.
type KMDINFPartSummary struct {
	Part          KMDINFPart      `json:"part"`
	PartnerCount  int             `json:"partner_count"`
	InvoiceCount  int             `json:"invoice_count"`
	TaxableAmount decimal.Decimal `json:"taxable_amount"`
	VATAmount     decimal.Decimal `json:"vat_amount"`
	TotalAmount   decimal.Decimal `json:"total_amount"`
}

// KMDINFReportRow represents one invoice row in the KMD INF appendix report.
type KMDINFReportRow struct {
	Part                       KMDINFPart      `json:"part"`
	ContactID                  string          `json:"contact_id"`
	ContactName                string          `json:"contact_name"`
	ContactRegCode             string          `json:"contact_reg_code,omitempty"`
	ContactVATNumber           string          `json:"contact_vat_number,omitempty"`
	InvoiceID                  string          `json:"invoice_id"`
	InvoiceNumber              string          `json:"invoice_number"`
	InvoiceDate                time.Time       `json:"invoice_date"`
	InvoiceType                string          `json:"invoice_type"`
	TaxableAmount              decimal.Decimal `json:"taxable_amount"`
	VATAmount                  decimal.Decimal `json:"vat_amount"`
	TotalAmount                decimal.Decimal `json:"total_amount"`
	PartnerPeriodTaxableAmount decimal.Decimal `json:"partner_period_taxable_amount"`
}

// KMDINFReportRequest contains KMD INF generation options.
type KMDINFReportRequest struct {
	Year      int             `json:"year"`
	Month     int             `json:"month"`
	Threshold decimal.Decimal `json:"threshold,omitempty"`
}

// EUVATOSSReport contains quarterly EU VAT One Stop Shop reporting totals.
type EUVATOSSReport struct {
	TenantID      string                   `json:"tenant_id"`
	Year          int                      `json:"year"`
	Quarter       int                      `json:"quarter"`
	PeriodStart   time.Time                `json:"period_start"`
	PeriodEnd     time.Time                `json:"period_end"`
	Scheme        string                   `json:"scheme"`
	Currency      string                   `json:"currency"`
	IncludeB2B    bool                     `json:"include_b2b"`
	GeneratedAt   time.Time                `json:"generated_at"`
	Summary       []EUVATOSSCountrySummary `json:"summary"`
	Rows          []EUVATOSSReportRow      `json:"rows"`
	TaxableAmount decimal.Decimal          `json:"taxable_amount"`
	VATAmount     decimal.Decimal          `json:"vat_amount"`
	TotalAmount   decimal.Decimal          `json:"total_amount"`
	InvoiceCount  int                      `json:"invoice_count"`
	LineCount     int                      `json:"line_count"`
}

// EUVATOSSCountrySummary summarizes OSS report rows for one member state.
type EUVATOSSCountrySummary struct {
	CountryCode   string          `json:"country_code"`
	CountryName   string          `json:"country_name"`
	InvoiceCount  int             `json:"invoice_count"`
	LineCount     int             `json:"line_count"`
	TaxableAmount decimal.Decimal `json:"taxable_amount"`
	VATAmount     decimal.Decimal `json:"vat_amount"`
	TotalAmount   decimal.Decimal `json:"total_amount"`
}

// EUVATOSSReportRow represents one OSS destination-country and VAT-rate aggregate.
type EUVATOSSReportRow struct {
	CountryCode   string          `json:"country_code"`
	CountryName   string          `json:"country_name"`
	VATRate       decimal.Decimal `json:"vat_rate"`
	InvoiceCount  int             `json:"invoice_count"`
	LineCount     int             `json:"line_count"`
	TaxableAmount decimal.Decimal `json:"taxable_amount"`
	VATAmount     decimal.Decimal `json:"vat_amount"`
	TotalAmount   decimal.Decimal `json:"total_amount"`
}

// EUVATOSSReportRequest contains EU VAT OSS report options.
type EUVATOSSReportRequest struct {
	Year       int  `json:"year"`
	Quarter    int  `json:"quarter"`
	IncludeB2B bool `json:"include_b2b,omitempty"`
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

// ImportKMDHistoryRequest contains CSV payload for historical KMD declarations.
// Each CSV row represents one KMD row inside a declaration period.
type ImportKMDHistoryRequest struct {
	CSVContent string `json:"csv_content"`
	FileName   string `json:"file_name,omitempty"`
}

// ImportKMDHistoryResult summarizes a historical KMD declaration import.
type ImportKMDHistoryResult struct {
	FileName            string                     `json:"file_name,omitempty"`
	RowsProcessed       int                        `json:"rows_processed"`
	DeclarationsCreated int                        `json:"declarations_created"`
	RowsImported        int                        `json:"rows_imported"`
	RowsSkipped         int                        `json:"rows_skipped"`
	Errors              []ImportKMDHistoryRowError `json:"errors,omitempty"`
}

// ImportKMDHistoryRowError describes a row-level historical KMD import failure.
type ImportKMDHistoryRowError struct {
	Row         int    `json:"row"`
	Year        int    `json:"year,omitempty"`
	Month       int    `json:"month,omitempty"`
	RowCode     string `json:"row_code,omitempty"`
	Description string `json:"description,omitempty"`
	Message     string `json:"message"`
}

// KMDExportFormat represents export formats
type KMDExportFormat string

const (
	KMDFormatXML  KMDExportFormat = "XML"
	KMDFormatJSON KMDExportFormat = "JSON"
)
