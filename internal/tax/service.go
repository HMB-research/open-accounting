package tax

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// VATEntry represents a VAT entry for aggregation
type VATEntry struct {
	VATCode   string
	TaxBase   float64
	TaxAmount float64
	IsOutput  bool
}

// Service provides tax declaration operations
type Service struct {
	repo Repository
}

// NewService creates a new tax service with a PostgreSQL repository
func NewService(db *pgxpool.Pool) *Service {
	return &Service{repo: NewPostgresRepository(db)}
}

// NewServiceWithRepository creates a new tax service with an injected repository
func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

// EnsureSchema creates tax tables if they don't exist
func (s *Service) EnsureSchema(ctx context.Context, schemaName string) error {
	return s.repo.EnsureSchema(ctx, schemaName)
}

// GenerateKMD generates a KMD declaration for a given period
func (s *Service) GenerateKMD(ctx context.Context, tenantID, schemaName string, req *CreateKMDRequest) (*KMDDeclaration, error) {
	if err := s.repo.EnsureSchema(ctx, schemaName); err != nil {
		return nil, fmt.Errorf("ensure schema: %w", err)
	}

	// Calculate period boundaries
	startDate := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0).Add(-time.Second)

	// Query VAT data from journal entries
	vatRows, err := s.repo.QueryVATData(ctx, schemaName, tenantID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("query VAT data: %w", err)
	}

	// Aggregate into KMD rows
	kmdRows := make([]KMDRow, 0)
	var totalOutput, totalInput decimal.Decimal

	for _, row := range vatRows {
		code := mapVATRateToKMDCode(row.VATRate, row.IsOutput)
		desc := getKMDRowDescription(code)

		kmdRows = append(kmdRows, KMDRow{
			Code:        code,
			Description: desc,
			TaxBase:     row.TaxBase.Abs(),
			TaxAmount:   row.TaxAmount.Abs(),
		})

		if row.IsOutput {
			totalOutput = totalOutput.Add(row.TaxAmount.Abs())
		} else {
			totalInput = totalInput.Add(row.TaxAmount.Abs())
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
	if err := s.repo.SaveDeclaration(ctx, schemaName, decl); err != nil {
		return nil, fmt.Errorf("save declaration: %w", err)
	}

	return decl, nil
}

// GetKMD retrieves a KMD declaration for a given period
func (s *Service) GetKMD(ctx context.Context, tenantID, schemaName, yearStr, monthStr string) (*KMDDeclaration, error) {
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return nil, fmt.Errorf("invalid year: %w", err)
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		return nil, fmt.Errorf("invalid month: %w", err)
	}

	return s.repo.GetDeclaration(ctx, schemaName, tenantID, year, month)
}

// ListKMD lists all KMD declarations for a tenant
func (s *Service) ListKMD(ctx context.Context, tenantID, schemaName string) ([]KMDDeclaration, error) {
	return s.repo.ListDeclarations(ctx, schemaName, tenantID)
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
	}
	return KMDRow4 // Input VAT
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

// aggregateVATByCode aggregates VAT entries by code (used for testing)
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
