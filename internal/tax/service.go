package tax

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/database"
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

const (
	euVATOSSSchemeUnion  = "UNION"
	euVATOSSBaseCurrency = "EUR"
)

var euVATOSSCountryNames = map[string]string{
	"AT": "Austria",
	"BE": "Belgium",
	"BG": "Bulgaria",
	"CY": "Cyprus",
	"CZ": "Czechia",
	"DE": "Germany",
	"DK": "Denmark",
	"EE": "Estonia",
	"EL": "Greece",
	"ES": "Spain",
	"FI": "Finland",
	"FR": "France",
	"HR": "Croatia",
	"HU": "Hungary",
	"IE": "Ireland",
	"IT": "Italy",
	"LT": "Lithuania",
	"LU": "Luxembourg",
	"LV": "Latvia",
	"MT": "Malta",
	"NL": "Netherlands",
	"PL": "Poland",
	"PT": "Portugal",
	"RO": "Romania",
	"SE": "Sweden",
	"SI": "Slovenia",
	"SK": "Slovakia",
}

// NewService creates a new tax service with an ORM-backed repository.
func NewService(db *pgxpool.Pool) *Service {
	if db == nil {
		return &Service{}
	}
	gormDB, err := database.NewGormDBFromPool(context.Background(), db)
	if err != nil {
		panic(fmt.Errorf("create tax GORM repository: %w", err))
	}
	return &Service{repo: NewGORMRepository(gormDB)}
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

// GenerateKMDINF generates a KMD INF appendix report for a VAT period.
func (s *Service) GenerateKMDINF(ctx context.Context, tenantID, schemaName string, req *KMDINFReportRequest) (*KMDINFReport, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.Year < 2020 || req.Year > 2100 {
		return nil, fmt.Errorf("invalid year")
	}
	if req.Month < 1 || req.Month > 12 {
		return nil, fmt.Errorf("invalid month")
	}

	threshold := req.Threshold
	if threshold.IsZero() {
		threshold = KMDINFDefaultThreshold
	}
	if threshold.LessThanOrEqual(decimal.Zero) {
		return nil, fmt.Errorf("threshold must be positive")
	}

	startDate := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)

	rows, err := s.repo.QueryKMDINFData(ctx, schemaName, tenantID, startDate, endDate, threshold)
	if err != nil {
		return nil, fmt.Errorf("query KMD INF data: %w", err)
	}

	report := &KMDINFReport{
		TenantID:    tenantID,
		Year:        req.Year,
		Month:       req.Month,
		Threshold:   threshold,
		GeneratedAt: time.Now(),
		Rows:        rows,
		Summary:     summarizeKMDINFRows(rows),
	}

	return report, nil
}

// GenerateEUVATOSS generates a quarterly EU VAT OSS report.
func (s *Service) GenerateEUVATOSS(ctx context.Context, tenantID, schemaName string, req *EUVATOSSReportRequest) (*EUVATOSSReport, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.Year < 2020 || req.Year > 2100 {
		return nil, fmt.Errorf("invalid year")
	}
	if req.Quarter < 1 || req.Quarter > 4 {
		return nil, fmt.Errorf("invalid quarter")
	}

	startDate := time.Date(req.Year, time.Month((req.Quarter-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 3, 0)

	rows, err := s.repo.QueryEUVATOSSData(ctx, schemaName, tenantID, startDate, endDate, req.IncludeB2B)
	if err != nil {
		return nil, fmt.Errorf("query EU VAT OSS data: %w", err)
	}

	normalizedRows := normalizeEUVATOSSRows(rows)
	report := &EUVATOSSReport{
		TenantID:    tenantID,
		Year:        req.Year,
		Quarter:     req.Quarter,
		PeriodStart: startDate,
		PeriodEnd:   endDate.Add(-time.Nanosecond),
		Scheme:      euVATOSSSchemeUnion,
		Currency:    euVATOSSBaseCurrency,
		IncludeB2B:  req.IncludeB2B,
		GeneratedAt: time.Now(),
		Rows:        normalizedRows,
		Summary:     summarizeEUVATOSSRows(normalizedRows),
	}

	for _, row := range normalizedRows {
		report.TaxableAmount = report.TaxableAmount.Add(row.TaxableAmount)
		report.VATAmount = report.VATAmount.Add(row.VATAmount)
		report.TotalAmount = report.TotalAmount.Add(row.TotalAmount)
		report.LineCount += row.LineCount
		report.InvoiceCount += row.InvoiceCount
	}

	return report, nil
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

func summarizeKMDINFRows(rows []KMDINFReportRow) []KMDINFPartSummary {
	parts := []KMDINFPart{KMDINFPartSales, KMDINFPartPurchases}
	summaries := make(map[KMDINFPart]*KMDINFPartSummary, len(parts))
	partnersByPart := make(map[KMDINFPart]map[string]bool, len(parts))
	for _, part := range parts {
		summaries[part] = &KMDINFPartSummary{Part: part}
		partnersByPart[part] = make(map[string]bool)
	}

	for _, row := range rows {
		summary, ok := summaries[row.Part]
		if !ok {
			summary = &KMDINFPartSummary{Part: row.Part}
			summaries[row.Part] = summary
			partnersByPart[row.Part] = make(map[string]bool)
			parts = append(parts, row.Part)
		}
		summary.InvoiceCount++
		summary.TaxableAmount = summary.TaxableAmount.Add(row.TaxableAmount)
		summary.VATAmount = summary.VATAmount.Add(row.VATAmount)
		summary.TotalAmount = summary.TotalAmount.Add(row.TotalAmount)
		partnersByPart[row.Part][row.ContactID] = true
	}

	result := make([]KMDINFPartSummary, 0, len(parts))
	for _, part := range parts {
		summary := summaries[part]
		summary.PartnerCount = len(partnersByPart[part])
		result = append(result, *summary)
	}
	return result
}

func normalizeEUVATOSSRows(rows []EUVATOSSReportRow) []EUVATOSSReportRow {
	normalized := make([]EUVATOSSReportRow, len(rows))
	for i, row := range rows {
		row.CountryCode = strings.ToUpper(strings.TrimSpace(row.CountryCode))
		row.CountryName = euVATOSSCountryName(row.CountryCode)
		normalized[i] = row
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].CountryCode == normalized[j].CountryCode {
			return normalized[i].VATRate.LessThan(normalized[j].VATRate)
		}
		return normalized[i].CountryCode < normalized[j].CountryCode
	})
	return normalized
}

func summarizeEUVATOSSRows(rows []EUVATOSSReportRow) []EUVATOSSCountrySummary {
	summaries := make(map[string]*EUVATOSSCountrySummary)
	order := make([]string, 0)
	for _, row := range rows {
		summary, ok := summaries[row.CountryCode]
		if !ok {
			summary = &EUVATOSSCountrySummary{
				CountryCode: row.CountryCode,
				CountryName: row.CountryName,
			}
			summaries[row.CountryCode] = summary
			order = append(order, row.CountryCode)
		}
		summary.InvoiceCount += row.InvoiceCount
		summary.LineCount += row.LineCount
		summary.TaxableAmount = summary.TaxableAmount.Add(row.TaxableAmount)
		summary.VATAmount = summary.VATAmount.Add(row.VATAmount)
		summary.TotalAmount = summary.TotalAmount.Add(row.TotalAmount)
	}
	sort.Strings(order)
	result := make([]EUVATOSSCountrySummary, 0, len(order))
	for _, code := range order {
		result = append(result, *summaries[code])
	}
	return result
}

func euVATOSSCountryName(code string) string {
	if name, ok := euVATOSSCountryNames[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return name
	}
	return strings.ToUpper(strings.TrimSpace(code))
}
