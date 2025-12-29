# Estonian Tax Compliance Implementation Plan

> **Status:** ✅ COMPLETED (2025-12-29)

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Estonian VAT declaration (KMD) generation, updated VAT rates (22%→24%), and e-Tax XML export to achieve parity with Merit Aktiva and SmartAccounts.

**Architecture:** Extend the existing VAT rates system with KMD form generation. Create a new `internal/tax` package for Estonian tax compliance. VAT declarations aggregate data from posted journal entries by tax code and period, generating XML in e-MTA format.

**Tech Stack:** Go backend, PostgreSQL, XML generation (encoding/xml), SvelteKit frontend

---

## Task 1: Update VAT Rates Schema

**Files:**
- Modify: `migrations/001_initial_schema.up.sql` (reference only)
- Create: `migrations/002_vat_rates_update.up.sql`
- Create: `migrations/002_vat_rates_update.down.sql`

**Step 1: Write migration for new VAT rates**

```sql
-- migrations/002_vat_rates_update.up.sql
-- Add new Estonian VAT rates for 2024-2025

-- Standard rate increased to 22% (Jan 1, 2024)
INSERT INTO vat_rates (id, country_code, rate_type, rate, valid_from, valid_to, description)
VALUES
    (gen_random_uuid(), 'EE', 'STANDARD', 22.00, '2024-01-01', '2025-06-30', 'Estonian standard VAT 22%'),
    (gen_random_uuid(), 'EE', 'STANDARD', 24.00, '2025-07-01', NULL, 'Estonian standard VAT 24%'),
    (gen_random_uuid(), 'EE', 'ACCOMMODATION', 13.00, '2025-01-01', NULL, 'Accommodation services VAT 13%'),
    (gen_random_uuid(), 'EE', 'PRESS', 9.00, '2025-01-01', NULL, 'Press publications VAT 9%')
ON CONFLICT DO NOTHING;

-- Update old 20% rate to have end date
UPDATE vat_rates
SET valid_to = '2023-12-31'
WHERE country_code = 'EE' AND rate = 20.00 AND rate_type = 'STANDARD' AND valid_to IS NULL;
```

**Step 2: Write down migration**

```sql
-- migrations/002_vat_rates_update.down.sql
DELETE FROM vat_rates WHERE country_code = 'EE' AND rate IN (22.00, 24.00, 13.00) AND rate_type IN ('STANDARD', 'ACCOMMODATION', 'PRESS');

UPDATE vat_rates
SET valid_to = NULL
WHERE country_code = 'EE' AND rate = 20.00 AND rate_type = 'STANDARD';
```

**Step 3: Run migration**

Run: `go run ./cmd/migrate -db "$DATABASE_URL" -path migrations -direction up`
Expected: Migration applied successfully

**Step 4: Commit**

```bash
git add migrations/002_vat_rates_update.up.sql migrations/002_vat_rates_update.down.sql
git commit -m "feat(tax): add Estonian VAT rate updates for 2024-2025"
```

---

## Task 2: Create Tax Package Types

**Files:**
- Create: `internal/tax/types.go`
- Test: `internal/tax/types_test.go`

**Step 1: Write the failing test**

```go
// internal/tax/types_test.go
package tax

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestKMDRow_Validate(t *testing.T) {
	row := KMDRow{
		Code:        "1",
		Description: "Taxable sales at standard rate",
		TaxBase:     decimal.NewFromInt(1000),
		TaxAmount:   decimal.NewFromInt(220),
	}
	err := row.Validate()
	assert.NoError(t, err)
}

func TestKMDRow_Validate_InvalidCode(t *testing.T) {
	row := KMDRow{
		Code:        "",
		Description: "Test",
		TaxBase:     decimal.NewFromInt(1000),
		TaxAmount:   decimal.NewFromInt(220),
	}
	err := row.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "code is required")
}

func TestKMDDeclaration_Period(t *testing.T) {
	decl := KMDDeclaration{
		TenantID:    "test-tenant",
		Year:        2025,
		Month:       1,
		SubmittedAt: nil,
	}
	assert.Equal(t, "2025-01", decl.Period())
}

func TestKMDDeclaration_CalculatePayable(t *testing.T) {
	decl := KMDDeclaration{
		TotalOutputVAT: decimal.NewFromInt(500),
		TotalInputVAT:  decimal.NewFromInt(200),
	}
	payable := decl.CalculatePayable()
	assert.True(t, payable.Equal(decimal.NewFromInt(300)))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tax/... -v`
Expected: FAIL with "no Go files in internal/tax"

**Step 3: Write the types**

```go
// internal/tax/types.go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tax/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tax/types.go internal/tax/types_test.go
git commit -m "feat(tax): add KMD declaration types for Estonian VAT"
```

---

## Task 3: Create Tax Service

**Files:**
- Create: `internal/tax/service.go`
- Test: `internal/tax/service_test.go`

**Step 1: Write the failing test**

```go
// internal/tax/service_test.go
package tax

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestService_GenerateKMD_EmptyPeriod(t *testing.T) {
	// This test will need a mock database - for now test the aggregation logic
	rows := aggregateVATByCode([]VATEntry{
		{VATCode: "1", TaxBase: 1000, TaxAmount: 220, IsOutput: true},
		{VATCode: "1", TaxBase: 500, TaxAmount: 110, IsOutput: true},
		{VATCode: "4", TaxBase: 300, TaxAmount: 66, IsOutput: false},
	})

	assert.Len(t, rows, 2)

	// Find row 1 (output VAT)
	var row1 *KMDRow
	for i := range rows {
		if rows[i].Code == "1" {
			row1 = &rows[i]
			break
		}
	}
	assert.NotNil(t, row1)
	assert.Equal(t, "1500", row1.TaxBase.String())
	assert.Equal(t, "330", row1.TaxAmount.String())
}

// VATEntry for testing aggregation
type VATEntry struct {
	VATCode   string
	TaxBase   float64
	TaxAmount float64
	IsOutput  bool
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tax/... -v -run TestService`
Expected: FAIL with "undefined: aggregateVATByCode"

**Step 3: Write minimal implementation**

```go
// internal/tax/service.go
package tax

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service provides tax declaration operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new tax service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// EnsureSchema creates tax tables if they don't exist
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.kmd_declarations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			tenant_id UUID NOT NULL,
			year INTEGER NOT NULL,
			month INTEGER NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'DRAFT',
			total_output_vat NUMERIC(28,8) NOT NULL DEFAULT 0,
			total_input_vat NUMERIC(28,8) NOT NULL DEFAULT 0,
			submitted_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (tenant_id, year, month)
		);

		CREATE TABLE IF NOT EXISTS %s.kmd_rows (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			declaration_id UUID NOT NULL REFERENCES %s.kmd_declarations(id) ON DELETE CASCADE,
			code VARCHAR(10) NOT NULL,
			description TEXT NOT NULL,
			tax_base NUMERIC(28,8) NOT NULL DEFAULT 0,
			tax_amount NUMERIC(28,8) NOT NULL DEFAULT 0
		);

		CREATE INDEX IF NOT EXISTS idx_kmd_declarations_tenant ON %s.kmd_declarations(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_kmd_rows_declaration ON %s.kmd_rows(declaration_id);
	`, schemaName, schemaName, schemaName, schemaName, schemaName)

	_, err := s.db.Exec(ctx, query)
	return err
}

// GenerateKMD generates a KMD declaration for a given period
func (s *Service) GenerateKMD(ctx context.Context, tenantID, schemaName string, req *CreateKMDRequest) (*KMDDeclaration, error) {
	if err := s.EnsureSchema(ctx, schemaName); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	// Calculate period boundaries
	startDate := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	// Query VAT data from journal entries
	rows, err := s.db.Query(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(jl.vat_rate, 0) as vat_rate,
			CASE
				WHEN a.account_type = 'REVENUE' THEN true
				ELSE false
			END as is_output,
			SUM(CASE WHEN jl.is_debit THEN -jl.amount ELSE jl.amount END) as tax_base,
			SUM(CASE WHEN jl.is_debit THEN -jl.amount ELSE jl.amount END) * COALESCE(jl.vat_rate, 0) / 100 as tax_amount
		FROM %s.journal_entries je
		JOIN %s.journal_lines jl ON je.id = jl.journal_entry_id
		JOIN %s.accounts a ON jl.account_id = a.id
		WHERE je.tenant_id = $1
			AND je.status = 'POSTED'
			AND je.entry_date >= $2
			AND je.entry_date <= $3
			AND COALESCE(jl.vat_rate, 0) > 0
		GROUP BY jl.vat_rate, a.account_type
	`, schemaName, schemaName, schemaName), tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query VAT data: %w", err)
	}
	defer rows.Close()

	// Aggregate into KMD rows
	kmdRows := make([]KMDRow, 0)
	var totalOutput, totalInput decimal.Decimal

	for rows.Next() {
		var vatRate decimal.Decimal
		var isOutput bool
		var taxBase, taxAmount decimal.Decimal

		if err := rows.Scan(&vatRate, &isOutput, &taxBase, &taxAmount); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		code := mapVATRateToKMDCode(vatRate, isOutput)
		desc := getKMDRowDescription(code)

		kmdRows = append(kmdRows, KMDRow{
			Code:        code,
			Description: desc,
			TaxBase:     taxBase.Abs(),
			TaxAmount:   taxAmount.Abs(),
		})

		if isOutput {
			totalOutput = totalOutput.Add(taxAmount.Abs())
		} else {
			totalInput = totalInput.Add(taxAmount.Abs())
		}
	}

	// Create declaration
	decl := &KMDDeclaration{
		ID:             uuid.New().String(),
		TenantID:       tenantID,
		Year:           req.Year,
		Month:          req.Month,
		Status:         "DRAFT",
		TotalOutputVAT: totalOutput,
		TotalInputVAT:  totalInput,
		Rows:           kmdRows,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Save to database
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.kmd_declarations (id, tenant_id, year, month, status, total_output_vat, total_input_vat, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (tenant_id, year, month) DO UPDATE SET
			status = EXCLUDED.status,
			total_output_vat = EXCLUDED.total_output_vat,
			total_input_vat = EXCLUDED.total_input_vat,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`, schemaName), decl.ID, tenantID, req.Year, req.Month, decl.Status, decl.TotalOutputVAT, decl.TotalInputVAT, decl.CreatedAt, decl.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert declaration: %w", err)
	}

	// Delete old rows and insert new ones
	_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.kmd_rows WHERE declaration_id = $1`, schemaName), decl.ID)
	if err != nil {
		return nil, fmt.Errorf("delete old rows: %w", err)
	}

	for _, row := range decl.Rows {
		_, err = tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.kmd_rows (declaration_id, code, description, tax_base, tax_amount)
			VALUES ($1, $2, $3, $4, $5)
		`, schemaName), decl.ID, row.Code, row.Description, row.TaxBase, row.TaxAmount)
		if err != nil {
			return nil, fmt.Errorf("insert row: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return decl, nil
}

// mapVATRateToKMDCode maps a VAT rate to the appropriate KMD row code
func mapVATRateToKMDCode(rate decimal.Decimal, isOutput bool) string {
	rateFloat, _ := rate.Float64()

	if isOutput {
		switch {
		case rateFloat >= 20:
			return KMDRow1 // Standard rate (20/22/24%)
		case rateFloat == 13:
			return KMDRow21 // Accommodation (13%)
		case rateFloat == 9:
			return KMDRow2 // Reduced rate (9%)
		case rateFloat == 0:
			return KMDRow3 // Zero-rated
		default:
			return KMDRow1
		}
	} else {
		return KMDRow4 // Input VAT
	}
}

// getKMDRowDescription returns the description for a KMD row code
func getKMDRowDescription(code string) string {
	descriptions := map[string]string{
		KMDRow1:  "Maksustatav käive standardmääraga / Taxable sales at standard rate",
		KMDRow2:  "Maksustatav käive vähendatud määraga 9% / Taxable sales at 9%",
		KMDRow21: "Maksustatav käive vähendatud määraga 13% / Taxable sales at 13%",
		KMDRow3:  "Nullmääraga käive (eksport) / Zero-rated exports",
		KMDRow31: "Nullmääraga käive (EL-i sisene) / Zero-rated intra-EU",
		KMDRow4:  "Sisendkäibemaks / Input VAT on domestic purchases",
		KMDRow5:  "Sisendkäibemaks impordilt / Input VAT on imports",
		KMDRow6:  "Sisendkäibemaks põhivaralt / Input VAT on fixed assets",
	}
	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Unknown"
}

// Helper for test - aggregate VAT entries
func aggregateVATByCode(entries []VATEntry) []KMDRow {
	aggregated := make(map[string]*KMDRow)

	for _, e := range entries {
		code := e.VATCode
		if _, exists := aggregated[code]; !exists {
			aggregated[code] = &KMDRow{
				Code:        code,
				Description: getKMDRowDescription(code),
				TaxBase:     decimal.Zero,
				TaxAmount:   decimal.Zero,
			}
		}
		aggregated[code].TaxBase = aggregated[code].TaxBase.Add(decimal.NewFromFloat(e.TaxBase))
		aggregated[code].TaxAmount = aggregated[code].TaxAmount.Add(decimal.NewFromFloat(e.TaxAmount))
	}

	rows := make([]KMDRow, 0, len(aggregated))
	for _, row := range aggregated {
		rows = append(rows, *row)
	}
	return rows
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tax/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tax/service.go internal/tax/service_test.go
git commit -m "feat(tax): add KMD generation service"
```

---

## Task 4: Add XML Export for e-MTA

**Files:**
- Create: `internal/tax/export.go`
- Test: `internal/tax/export_test.go`

**Step 1: Write the failing test**

```go
// internal/tax/export_test.go
package tax

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestExportKMDToXML(t *testing.T) {
	decl := &KMDDeclaration{
		ID:             "test-id",
		TenantID:       "tenant-1",
		Year:           2025,
		Month:          1,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(330),
		TotalInputVAT:  decimal.NewFromInt(66),
		Rows: []KMDRow{
			{Code: "1", Description: "Standard rate", TaxBase: decimal.NewFromInt(1500), TaxAmount: decimal.NewFromInt(330)},
			{Code: "4", Description: "Input VAT", TaxBase: decimal.NewFromInt(300), TaxAmount: decimal.NewFromInt(66)},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	xml, err := ExportKMDToXML(decl, "12345678")
	assert.NoError(t, err)
	assert.Contains(t, string(xml), "<?xml version=")
	assert.Contains(t, string(xml), "<KMD>")
	assert.Contains(t, string(xml), "<periood>2025-01</periood>")
	assert.Contains(t, string(xml), "<rida1>1500")
	assert.Contains(t, string(xml), "</KMD>")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tax/... -v -run TestExport`
Expected: FAIL with "undefined: ExportKMDToXML"

**Step 3: Write the XML export implementation**

```go
// internal/tax/export.go
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tax/... -v -run TestExport`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/tax/export.go internal/tax/export_test.go
git commit -m "feat(tax): add KMD XML export for Estonian e-MTA"
```

---

## Task 5: Add Tax API Handlers

**Files:**
- Modify: `cmd/api/handlers.go`
- Modify: `cmd/api/handlers_business.go`
- Modify: `cmd/api/main.go`

**Step 1: Add tax service to handlers**

```go
// In cmd/api/handlers.go - add to Handlers struct
import "github.com/openaccounting/openaccounting/internal/tax"

// Add to Handlers struct:
// taxService *tax.Service

// In cmd/api/main.go - initialize tax service:
// taxService := tax.NewService(pool)
// Pass to NewHandlers
```

**Step 2: Add tax handlers**

```go
// In cmd/api/handlers_business.go - add these handlers:

// HandleGenerateKMD generates a KMD declaration for a period
func (h *Handlers) HandleGenerateKMD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")

	var req tax.CreateKMDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	schemaName, err := h.tenantService.GetSchemaName(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	decl, err := h.taxService.GenerateKMD(r.Context(), tenantID, schemaName, &req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, decl)
}

// HandleExportKMD exports a KMD declaration to XML
func (h *Handlers) HandleExportKMD(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	year := chi.URLParam(r, "year")
	month := chi.URLParam(r, "month")

	// Get tenant settings for registration number
	tenant, err := h.tenantService.GetByID(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	schemaName, err := h.tenantService.GetSchemaName(r.Context(), tenantID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Tenant not found")
		return
	}

	// Get declaration
	decl, err := h.taxService.GetKMD(r.Context(), tenantID, schemaName, year, month)
	if err != nil {
		respondError(w, http.StatusNotFound, "Declaration not found")
		return
	}

	// Get registration number from tenant settings
	regNr := ""
	if settings, ok := tenant.Settings["registration_number"].(string); ok {
		regNr = settings
	}

	xml, err := tax.ExportKMDToXML(decl, regNr)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=KMD_%s_%s.xml", year, month))
	w.Write(xml)
}
```

**Step 3: Add routes**

```go
// In cmd/api/main.go - add routes:
r.Route("/tenants/{tenantID}/tax", func(r chi.Router) {
	r.Use(h.AuthMiddleware)
	r.Post("/kmd", h.HandleGenerateKMD)
	r.Get("/kmd/{year}/{month}/xml", h.HandleExportKMD)
	r.Get("/kmd", h.HandleListKMD)
})
```

**Step 4: Build and verify**

Run: `go build ./...`
Expected: Build successful

**Step 5: Commit**

```bash
git add cmd/api/handlers.go cmd/api/handlers_business.go cmd/api/main.go
git commit -m "feat(api): add KMD tax declaration endpoints"
```

---

## Task 6: Add Frontend Tax Page

**Files:**
- Create: `frontend/src/routes/tax/+page.svelte`
- Modify: `frontend/src/lib/api.ts`
- Modify: `frontend/src/routes/dashboard/+page.svelte`

**Step 1: Add API types and methods**

```typescript
// In frontend/src/lib/api.ts - add types:
export interface KMDRow {
  code: string;
  description: string;
  tax_base: string;
  tax_amount: string;
}

export interface KMDDeclaration {
  id: string;
  tenant_id: string;
  year: number;
  month: number;
  status: string;
  total_output_vat: string;
  total_input_vat: string;
  rows: KMDRow[];
  submitted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateKMDRequest {
  year: number;
  month: number;
}

// Add methods:
async generateKMD(tenantId: string, req: CreateKMDRequest): Promise<KMDDeclaration> {
  return this.request(`/tenants/${tenantId}/tax/kmd`, {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

async downloadKMDXml(tenantId: string, year: number, month: number): Promise<Blob> {
  const response = await fetch(`${this.baseUrl}/tenants/${tenantId}/tax/kmd/${year}/${month}/xml`, {
    headers: this.getHeaders(),
  });
  return response.blob();
}
```

**Step 2: Create tax page**

```svelte
<!-- frontend/src/routes/tax/+page.svelte -->
<script lang="ts">
  import { api, type KMDDeclaration } from '$lib/api';
  import { goto } from '$app/navigation';

  let tenantId = $state('');
  let loading = $state(true);
  let generating = $state(false);
  let error = $state<string | null>(null);
  let declarations = $state<KMDDeclaration[]>([]);

  let selectedYear = $state(new Date().getFullYear());
  let selectedMonth = $state(new Date().getMonth() + 1);

  $effect(() => {
    loadData();
  });

  async function loadData() {
    loading = true;
    try {
      const memberships = await api.getMyTenants();
      if (memberships.length === 0) {
        error = 'No tenant available';
        return;
      }
      tenantId = memberships[0].tenant.id;
      // TODO: Load existing declarations
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load data';
    } finally {
      loading = false;
    }
  }

  async function generateKMD() {
    generating = true;
    error = null;
    try {
      const decl = await api.generateKMD(tenantId, {
        year: selectedYear,
        month: selectedMonth,
      });
      declarations = [decl, ...declarations];
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to generate KMD';
    } finally {
      generating = false;
    }
  }

  async function downloadXml(decl: KMDDeclaration) {
    try {
      const blob = await api.downloadKMDXml(tenantId, decl.year, decl.month);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `KMD_${decl.year}_${String(decl.month).padStart(2, '0')}.xml`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to download XML';
    }
  }

  function formatCurrency(value: string): string {
    return new Intl.NumberFormat('et-EE', {
      style: 'currency',
      currency: 'EUR',
    }).format(parseFloat(value));
  }
</script>

<svelte:head>
  <title>Tax Declarations - Open Accounting</title>
</svelte:head>

<div class="max-w-6xl mx-auto px-4 py-8">
  <div class="flex items-center justify-between mb-6">
    <div class="flex items-center gap-4">
      <button onclick={() => goto('/dashboard')} class="text-gray-600 hover:text-gray-800">
        &larr; Back
      </button>
      <h1 class="text-2xl font-bold text-gray-900">VAT Declarations (KMD)</h1>
    </div>
  </div>

  {#if error}
    <div class="bg-red-50 text-red-600 p-4 rounded-lg mb-4">{error}</div>
  {/if}

  {#if loading}
    <div class="flex justify-center py-12">
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
    </div>
  {:else}
    <!-- Generate KMD Form -->
    <div class="bg-white rounded-lg shadow p-6 mb-6">
      <h2 class="text-lg font-semibold mb-4">Generate VAT Declaration</h2>
      <div class="flex gap-4 items-end">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Year</label>
          <select bind:value={selectedYear} class="border border-gray-300 rounded-lg px-3 py-2">
            {#each [2024, 2025, 2026] as year}
              <option value={year}>{year}</option>
            {/each}
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">Month</label>
          <select bind:value={selectedMonth} class="border border-gray-300 rounded-lg px-3 py-2">
            {#each Array.from({ length: 12 }, (_, i) => i + 1) as month}
              <option value={month}>{String(month).padStart(2, '0')}</option>
            {/each}
          </select>
        </div>
        <button
          onclick={generateKMD}
          disabled={generating}
          class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-400"
        >
          {generating ? 'Generating...' : 'Generate KMD'}
        </button>
      </div>
    </div>

    <!-- Declarations List -->
    {#if declarations.length > 0}
      <div class="bg-white rounded-lg shadow">
        <div class="px-6 py-4 border-b">
          <h2 class="text-lg font-semibold">Generated Declarations</h2>
        </div>
        <div class="divide-y">
          {#each declarations as decl}
            <div class="px-6 py-4 flex items-center justify-between">
              <div>
                <div class="font-medium">{decl.year}-{String(decl.month).padStart(2, '0')}</div>
                <div class="text-sm text-gray-500">
                  Output VAT: {formatCurrency(decl.total_output_vat)} |
                  Input VAT: {formatCurrency(decl.total_input_vat)} |
                  Payable: {formatCurrency(String(parseFloat(decl.total_output_vat) - parseFloat(decl.total_input_vat)))}
                </div>
              </div>
              <div class="flex gap-2">
                <span class="px-2 py-1 text-xs rounded-full {decl.status === 'DRAFT' ? 'bg-yellow-100 text-yellow-800' : 'bg-green-100 text-green-800'}">
                  {decl.status}
                </span>
                <button
                  onclick={() => downloadXml(decl)}
                  class="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50"
                >
                  Download XML
                </button>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {:else}
      <div class="bg-white rounded-lg shadow p-12 text-center text-gray-500">
        <p>No VAT declarations yet. Generate one to get started.</p>
      </div>
    {/if}
  {/if}
</div>
```

**Step 3: Add navigation link**

In `frontend/src/routes/dashboard/+page.svelte`, add to quick links:
```svelte
<a href="/tax?tenant={selectedTenant.id}" class="btn btn-secondary">Tax</a>
```

**Step 4: Build frontend**

Run: `npm run build --prefix frontend`
Expected: Build successful

**Step 5: Commit**

```bash
git add frontend/src/routes/tax/+page.svelte frontend/src/lib/api.ts frontend/src/routes/dashboard/+page.svelte
git commit -m "feat(frontend): add VAT declaration (KMD) page"
```

---

## Task 7: Run Full Test Suite

**Step 1: Run backend tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 2: Run frontend check**

Run: `npm run check --prefix frontend`
Expected: No errors

**Step 3: Build everything**

Run: `go build ./... && npm run build --prefix frontend`
Expected: Build successful

**Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete Estonian VAT declaration (KMD) implementation"
```

---

## Summary

This plan implements:
1. Updated Estonian VAT rates (22%, 24%, 13%, 9%)
2. KMD declaration types and service
3. XML export in e-MTA format
4. API endpoints for KMD generation and export
5. Frontend page for managing VAT declarations

**Total tasks:** 7
**Estimated time:** Each task 15-30 minutes

After this phase, the next priority features would be:
- Fixed Assets Module
- Payroll Foundation
- Inventory Management
