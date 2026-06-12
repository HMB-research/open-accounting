package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type taxHandlerRepository struct {
	vatRows      []tax.VATAggregateRow
	vatErr       error
	infRows      []tax.KMDINFReportRow
	infErr       error
	ossRows      []tax.EUVATOSSReportRow
	ossErr       error
	saveErr      error
	getDecl      *tax.KMDDeclaration
	getDeclErr   error
	listDecls    []tax.KMDDeclaration
	listDeclsErr error
	savedDecls   []*tax.KMDDeclaration
}

func (r *taxHandlerRepository) QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]tax.VATAggregateRow, error) {
	if r.vatErr != nil {
		return nil, r.vatErr
	}
	return r.vatRows, nil
}

func (r *taxHandlerRepository) QueryKMDINFData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time, threshold decimal.Decimal) ([]tax.KMDINFReportRow, error) {
	if r.infErr != nil {
		return nil, r.infErr
	}
	return r.infRows, nil
}

func (r *taxHandlerRepository) QueryEUVATOSSData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time, includeB2B bool) ([]tax.EUVATOSSReportRow, error) {
	if r.ossErr != nil {
		return nil, r.ossErr
	}
	return r.ossRows, nil
}

func (r *taxHandlerRepository) SaveDeclaration(ctx context.Context, schemaName string, decl *tax.KMDDeclaration) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.savedDecls = append(r.savedDecls, decl)
	return nil
}

func (r *taxHandlerRepository) GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*tax.KMDDeclaration, error) {
	if r.getDeclErr != nil {
		return nil, r.getDeclErr
	}
	if r.getDecl != nil && r.getDecl.TenantID == tenantID && r.getDecl.Year == year && r.getDecl.Month == month {
		return r.getDecl, nil
	}
	return nil, nil
}

func (r *taxHandlerRepository) ListDeclarations(ctx context.Context, schemaName, tenantID string) ([]tax.KMDDeclaration, error) {
	if r.listDeclsErr != nil {
		return nil, r.listDeclsErr
	}
	return r.listDecls, nil
}

func setupTaxHandlerTest(t *testing.T) (*Handlers, *mockTenantRepository, *taxHandlerRepository) {
	t.Helper()

	h, tenantRepo := setupTenantTestHandlers()
	taxRepo := &taxHandlerRepository{}
	h.taxService = tax.NewServiceWithRepository(taxRepo)

	return h, tenantRepo, taxRepo
}

func TestTaxHandlersKMDWorkflow(t *testing.T) {
	h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
	tenantRecord.Settings.RegCode = "12345678"

	taxRepo.vatRows = []tax.VATAggregateRow{
		{VATRate: decimal.NewFromInt(22), IsOutput: true, TaxBase: decimal.NewFromInt(1000), TaxAmount: decimal.NewFromInt(220)},
		{VATRate: decimal.NewFromInt(22), IsOutput: false, TaxBase: decimal.NewFromInt(300), TaxAmount: decimal.NewFromInt(66)},
	}
	generated := invokeTaxHandlerJSON[tax.KMDDeclaration](t, http.StatusOK, h.HandleGenerateKMD, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd",
		tax.CreateKMDRequest{Year: 2026, Month: 2},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, 2026, generated.Year)
	require.Equal(t, 2, generated.Month)
	require.True(t, generated.TotalOutputVAT.Equal(decimal.NewFromInt(220)))
	require.True(t, generated.TotalInputVAT.Equal(decimal.NewFromInt(66)))
	require.Len(t, taxRepo.savedDecls, 1)

	now := time.Now().UTC()
	taxRepo.listDecls = []tax.KMDDeclaration{{
		ID:             "decl-1",
		TenantID:       tenantRecord.ID,
		Year:           2026,
		Month:          2,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(220),
		TotalInputVAT:  decimal.NewFromInt(66),
		CreatedAt:      now,
		UpdatedAt:      now,
	}}
	declarations := invokeTaxHandlerJSON[[]tax.KMDDeclaration](t, http.StatusOK, h.HandleListKMD, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, declarations, 1)
	require.Equal(t, "decl-1", declarations[0].ID)

	taxRepo.infRows = []tax.KMDINFReportRow{{
		Part:                       tax.KMDINFPartSales,
		ContactID:                  "contact-1",
		ContactName:                "Alpha OU",
		ContactRegCode:             "12345678",
		InvoiceID:                  "invoice-1",
		InvoiceNumber:              "INV-1",
		InvoiceDate:                time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
		InvoiceType:                "SALES",
		TaxableAmount:              decimal.NewFromInt(1200),
		VATAmount:                  decimal.NewFromInt(264),
		TotalAmount:                decimal.NewFromInt(1464),
		PartnerPeriodTaxableAmount: decimal.NewFromInt(1200),
	}}
	infReport := invokeTaxHandlerJSON[tax.KMDINFReport](t, http.StatusOK, h.HandleGenerateKMDINF, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/2/inf?threshold=1000",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Len(t, infReport.Rows, 1)
	require.Equal(t, tax.KMDINFPartSales, infReport.Rows[0].Part)
	require.True(t, infReport.Threshold.Equal(decimal.NewFromInt(1000)))

	taxRepo.ossRows = []tax.EUVATOSSReportRow{{
		CountryCode:   "DE",
		VATRate:       decimal.NewFromInt(19),
		InvoiceCount:  1,
		LineCount:     1,
		TaxableAmount: decimal.NewFromInt(100),
		VATAmount:     decimal.NewFromInt(19),
		TotalAmount:   decimal.NewFromInt(119),
	}}
	ossReport := invokeTaxHandlerJSON[tax.EUVATOSSReport](t, http.StatusOK, h.HandleGenerateEUVATOSS, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/eu-vat/oss?year=2026&quarter=1&include_b2b=true",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Len(t, ossReport.Rows, 1)
	require.Equal(t, "DE", ossReport.Rows[0].CountryCode)
	require.Equal(t, "Germany", ossReport.Rows[0].CountryName)
	require.True(t, ossReport.IncludeB2B)

	taxRepo.getDecl = &tax.KMDDeclaration{
		ID:             "decl-export",
		TenantID:       tenantRecord.ID,
		Year:           2026,
		Month:          2,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(220),
		TotalInputVAT:  decimal.NewFromInt(66),
		Rows: []tax.KMDRow{
			{Code: tax.KMDRow1, Description: "Taxable sales", TaxBase: decimal.NewFromInt(1000), TaxAmount: decimal.NewFromInt(220)},
			{Code: tax.KMDRow4, Description: "Input VAT", TaxBase: decimal.NewFromInt(300), TaxAmount: decimal.NewFromInt(66)},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	xmlResp := invokeTaxHandlerRaw(t, http.StatusOK, h.HandleExportKMD, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/2/xml",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "application/xml", xmlResp.Header().Get("Content-Type"))
	require.Contains(t, xmlResp.Header().Get("Content-Disposition"), "KMD_2026_2.xml")
	require.Contains(t, xmlResp.Body.String(), "<regNr>12345678</regNr>")
	require.Contains(t, xmlResp.Body.String(), "<periood>2026-02</periood>")
}

func TestTaxHandlersKMDImportHistory(t *testing.T) {
	h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
	tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")

	result := invokeTaxHandlerJSON[tax.ImportKMDHistoryResult](t, http.StatusOK, h.HandleImportKMDHistory, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd/import-history",
		map[string]any{
			"csv_content": "year,month,row_code,tax_base,tax_amount\n2025,12,1,1000.00,220.00\n",
		},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "kmd-history.csv", result.FileName)
	require.Equal(t, 1, result.RowsProcessed)
	require.Equal(t, 1, result.DeclarationsCreated)
	require.Equal(t, 1, result.RowsImported)
	require.Len(t, taxRepo.savedDecls, 1)
	require.Equal(t, 2025, taxRepo.savedDecls[0].Year)

	errBody := invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleImportKMDHistory, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd/import-history",
		map[string]string{"csv_content": "  "},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "csv_content is required", errBody["error"])
}

func TestTaxHandlersValidationAndErrorPaths(t *testing.T) {
	h, tenantRepo, taxRepo := setupTaxHandlerTest(t)
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
	tenantRecord.Settings.RegCode = "12345678"

	errBody := invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateKMD, taxHandlerRequestWithBody(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd",
		strings.NewReader("{"),
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "Invalid request body", errBody["error"])

	taxRepo.vatErr = errors.New("vat data unavailable")
	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleGenerateKMD, taxHandlerRequest(
		http.MethodPost,
		"/tenants/tenant-1/tax/kmd",
		tax.CreateKMDRequest{Year: 2026, Month: 2},
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Contains(t, errBody["error"], "vat data unavailable")
	taxRepo.vatErr = nil

	taxRepo.listDeclsErr = errors.New("list failed")
	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleListKMD, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "list failed", errBody["error"])
	taxRepo.listDeclsErr = nil

	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateKMDINF, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/2/inf?threshold=bad",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "Invalid threshold", errBody["error"])

	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateKMDINF, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/2/inf?threshold=0",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "threshold must be positive", errBody["error"])

	taxRepo.infErr = errors.New("inf query failed")
	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleGenerateKMDINF, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/2/inf",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Contains(t, errBody["error"], "inf query failed")
	taxRepo.infErr = nil

	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateEUVATOSS, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/eu-vat/oss?year=bad&quarter=1",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "Invalid year", errBody["error"])

	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateEUVATOSS, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/eu-vat/oss?year=2026&quarter=5",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "Invalid quarter", errBody["error"])

	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusBadRequest, h.HandleGenerateEUVATOSS, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/eu-vat/oss?year=2026&quarter=1&include_b2b=maybe",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Equal(t, "Invalid include_b2b", errBody["error"])

	taxRepo.ossErr = errors.New("oss query failed")
	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusInternalServerError, h.HandleGenerateEUVATOSS, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/eu-vat/oss?year=2026&quarter=1",
		nil,
		map[string]string{"tenantID": "tenant-1"},
	))
	require.Contains(t, errBody["error"], "oss query failed")
	taxRepo.ossErr = nil

	tenantRepo.getTenantErr = tenant.ErrTenantNotFound
	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusNotFound, h.HandleExportKMD, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/2/xml",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "Tenant not found", errBody["error"])
	tenantRepo.getTenantErr = nil

	taxRepo.getDeclErr = errors.New("declaration missing")
	errBody = invokeTaxHandlerJSON[map[string]string](t, http.StatusNotFound, h.HandleExportKMD, taxHandlerRequest(
		http.MethodGet,
		"/tenants/tenant-1/tax/kmd/2026/2/xml",
		nil,
		map[string]string{"tenantID": "tenant-1", "year": "2026", "month": "2"},
	))
	require.Equal(t, "Declaration not found", errBody["error"])
}

func taxHandlerRequest(method, path string, body any, params map[string]string) *http.Request {
	req := makeAuthenticatedRequest(method, path, body, createTestClaims("user-1", "user@example.com", "tenant-1", "owner"))
	return withURLParams(req, params)
}

func taxHandlerRequestWithBody(method, path string, body *strings.Reader, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, body)
	return withURLParams(req, params)
}

func invokeTaxHandlerJSON[T any](t *testing.T, wantStatus int, handler func(http.ResponseWriter, *http.Request), req *http.Request) T {
	t.Helper()

	rec := invokeTaxHandlerRaw(t, wantStatus, handler, req)
	var result T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	return result
}

func invokeTaxHandlerRaw(t *testing.T, wantStatus int, handler func(http.ResponseWriter, *http.Request), req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	handler(rec, req)
	require.Equal(t, wantStatus, rec.Code, fmt.Sprintf("body=%s", rec.Body.String()))
	return rec
}
