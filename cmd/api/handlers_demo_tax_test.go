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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/testutil"
)

type mockTaxRepository struct {
	ensureSchemaErr        error
	queryVATDataResult     []tax.VATAggregateRow
	queryVATDataErr        error
	saveDeclarationErr     error
	getDeclarationResult   *tax.KMDDeclaration
	getDeclarationErr      error
	listDeclarationsResult []tax.KMDDeclaration
	listDeclarationsErr    error
	savedDeclarations      []*tax.KMDDeclaration
}

func (m *mockTaxRepository) EnsureSchema(ctx context.Context, schemaName string) error {
	return m.ensureSchemaErr
}

func (m *mockTaxRepository) QueryVATData(ctx context.Context, schemaName, tenantID string, startDate, endDate time.Time) ([]tax.VATAggregateRow, error) {
	if m.queryVATDataErr != nil {
		return nil, m.queryVATDataErr
	}
	return m.queryVATDataResult, nil
}

func (m *mockTaxRepository) SaveDeclaration(ctx context.Context, schemaName string, decl *tax.KMDDeclaration) error {
	if m.saveDeclarationErr != nil {
		return m.saveDeclarationErr
	}
	m.savedDeclarations = append(m.savedDeclarations, decl)
	return nil
}

func (m *mockTaxRepository) GetDeclaration(ctx context.Context, schemaName, tenantID string, year, month int) (*tax.KMDDeclaration, error) {
	if m.getDeclarationErr != nil {
		return nil, m.getDeclarationErr
	}
	return m.getDeclarationResult, nil
}

func (m *mockTaxRepository) ListDeclarations(ctx context.Context, schemaName, tenantID string) ([]tax.KMDDeclaration, error) {
	if m.listDeclarationsErr != nil {
		return nil, m.listDeclarationsErr
	}
	return m.listDeclarationsResult, nil
}

func setupTaxHandlers() (*Handlers, *mockTenantRepository, *mockTaxRepository) {
	tenantRepo := newMockTenantRepository()
	tenantSvc := tenant.NewServiceWithRepository(tenantRepo)
	taxRepo := &mockTaxRepository{}

	return &Handlers{
		tenantService: tenantSvc,
		taxService:    tax.NewServiceWithRepository(taxRepo),
	}, tenantRepo, taxRepo
}

func TestKMDHandlers(t *testing.T) {
	now := time.Now().UTC()
	h, tenantRepo, taxRepo := setupTaxHandlers()
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
	tenantRecord.Settings.RegCode = "12345678"

	taxRepo.queryVATDataResult = []tax.VATAggregateRow{{
		VATRate:   decimal.NewFromInt(22),
		IsOutput:  true,
		TaxBase:   decimal.NewFromInt(100),
		TaxAmount: decimal.NewFromInt(22),
	}}

	req := makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/tax/kmd", map[string]int{
		"year":  2025,
		"month": 2,
	}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.HandleGenerateKMD(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var generated tax.KMDDeclaration
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&generated))
	assert.Equal(t, 2025, generated.Year)
	assert.Equal(t, 2, generated.Month)
	assert.Equal(t, "22", generated.TotalOutputVAT.String())
	require.Len(t, taxRepo.savedDeclarations, 1)

	taxRepo.listDeclarationsResult = []tax.KMDDeclaration{
		{
			ID:             "decl-1",
			TenantID:       tenantRecord.ID,
			Year:           2025,
			Month:          2,
			Status:         "DRAFT",
			TotalOutputVAT: decimal.NewFromInt(22),
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/tax/kmd", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.HandleListKMD(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var declarations []tax.KMDDeclaration
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&declarations))
	require.Len(t, declarations, 1)
	assert.Equal(t, 2025, declarations[0].Year)

	taxRepo.getDeclarationResult = &tax.KMDDeclaration{
		ID:             "decl-export",
		TenantID:       tenantRecord.ID,
		Year:           2025,
		Month:          2,
		Status:         "DRAFT",
		TotalOutputVAT: decimal.NewFromInt(22),
		TotalInputVAT:  decimal.NewFromInt(10),
		Rows: []tax.KMDRow{
			{Code: tax.KMDRow1, Description: "Taxable sales", TaxBase: decimal.NewFromInt(100), TaxAmount: decimal.NewFromInt(22)},
			{Code: tax.KMDRow4, Description: "Input VAT", TaxBase: decimal.NewFromInt(0), TaxAmount: decimal.NewFromInt(10)},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/tax/kmd/2025/2/xml", nil), map[string]string{
		"tenantID": "tenant-1",
		"year":     "2025",
		"month":    "2",
	})
	rr = httptest.NewRecorder()
	h.HandleExportKMD(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/xml")
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "KMD_2025_2.xml")
	assert.Contains(t, rr.Body.String(), "<regNr>12345678</regNr>")
	assert.Contains(t, rr.Body.String(), "<periood>2025-02</periood>")
}

func TestKMDHandlersValidationAndErrorPaths(t *testing.T) {
	h, tenantRepo, taxRepo := setupTaxHandlers()
	tenantRecord := tenantRepo.addTestTenant("tenant-1", "Tax Tenant", "tax-tenant")
	tenantRecord.Settings.RegCode = "12345678"

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/tenants/tenant-1/tax/kmd", strings.NewReader("{")), map[string]string{"tenantID": "tenant-1"})
	rr := httptest.NewRecorder()
	h.HandleGenerateKMD(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

	taxRepo.ensureSchemaErr = errors.New("schema unavailable")
	req = makeAuthenticatedRequest(http.MethodPost, "/tenants/tenant-1/tax/kmd", map[string]int{"year": 2025, "month": 2}, nil)
	req = withURLParams(req, map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.HandleGenerateKMD(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "schema unavailable")
	taxRepo.ensureSchemaErr = nil

	taxRepo.listDeclarationsErr = errors.New("list failed")
	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/tax/kmd", nil), map[string]string{"tenantID": "tenant-1"})
	rr = httptest.NewRecorder()
	h.HandleListKMD(rr, req)
	require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "list failed")
	taxRepo.listDeclarationsErr = nil

	tenantRepo.getTenantErr = tenant.ErrTenantNotFound
	req = withURLParams(httptest.NewRequest(http.MethodGet, "/tenants/tenant-1/tax/kmd/2025/2/xml", nil), map[string]string{
		"tenantID": "tenant-1",
		"year":     "2025",
		"month":    "2",
	})
	rr = httptest.NewRecorder()
	h.HandleExportKMD(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Tenant not found")
	tenantRepo.getTenantErr = nil

	taxRepo.getDeclarationErr = errors.New("declaration missing")
	rr = httptest.NewRecorder()
	h.HandleExportKMD(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Declaration not found")
}

func TestDemoHandlersValidation(t *testing.T) {
	h := &Handlers{}

	req := httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	rr := httptest.NewRecorder()
	h.DemoReset(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Demo mode is not enabled")

	t.Setenv("DEMO_MODE", "true")

	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Demo reset not configured")

	t.Setenv("DEMO_RESET_SECRET", "demo-secret")

	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	req.Header.Set("X-Demo-Secret", "wrong-secret")
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Invalid or missing secret key")

	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset?user=99", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Invalid user parameter")

	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=1", nil)
	rr = httptest.NewRecorder()
	t.Setenv("DEMO_MODE", "false")
	h.DemoStatus(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Demo mode is not enabled")

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("DEMO_RESET_SECRET", "")
	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=1", nil)
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Demo status not configured")

	t.Setenv("DEMO_RESET_SECRET", "demo-secret")
	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=1", nil)
	req.Header.Set("X-Demo-Secret", "wrong-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Invalid or missing secret key")

	req = httptest.NewRequest(http.MethodGet, "/api/demo/status", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "User parameter is required")

	req = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=99", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoStatus(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Invalid user parameter")
}

func TestDemoHandlersResetAndStatus(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	h := &Handlers{pool: pool}
	ctx := context.Background()

	t.Setenv("DEMO_MODE", "true")
	t.Setenv("DEMO_RESET_SECRET", "demo-secret")

	cleanupDemoUsers := func(users ...int) {
		t.Helper()
		for _, userNum := range users {
			schema := fmt.Sprintf("tenant_demo%d", userNum)
			_, _ = pool.Exec(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
			_, _ = pool.Exec(ctx, "DELETE FROM tenant_users WHERE tenant_id IN (SELECT id FROM tenants WHERE slug = $1)", fmt.Sprintf("demo%d", userNum))
			_, _ = pool.Exec(ctx, "DELETE FROM tenants WHERE slug = $1", fmt.Sprintf("demo%d", userNum))
			_, _ = pool.Exec(ctx, "DELETE FROM users WHERE email = $1", fmt.Sprintf("demo%d@example.com", userNum))
		}
	}

	cleanupDemoUsers(1, 2, 3, 4)
	t.Cleanup(func() { cleanupDemoUsers(1, 2, 3, 4) })

	req := httptest.NewRequest(http.MethodPost, "/api/demo/reset", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr := httptest.NewRecorder()
	h.DemoReset(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), "Demo database reset successfully")

	statusReq := httptest.NewRequest(http.MethodGet, "/api/demo/status?user=1", nil)
	statusReq.Header.Set("X-Demo-Secret", "demo-secret")
	statusRR := httptest.NewRecorder()
	h.DemoStatus(statusRR, statusReq)
	require.Equal(t, http.StatusOK, statusRR.Code, statusRR.Body.String())

	var status DemoStatusResponse
	require.NoError(t, json.NewDecoder(statusRR.Body).Decode(&status))
	assert.Equal(t, 1, status.User)
	assert.GreaterOrEqual(t, status.Accounts.Count, 10)
	assert.GreaterOrEqual(t, status.Contacts.Count, 3)
	assert.GreaterOrEqual(t, status.Invoices.Count, 2)
	assert.GreaterOrEqual(t, status.Employees.Count, 1)
	assert.GreaterOrEqual(t, status.PayrollRuns.Count, 1)
	assert.NotEmpty(t, status.Accounts.Keys)
	assert.NotEmpty(t, status.Employees.Keys)
	assert.NotEmpty(t, status.PayrollRuns.Keys)

	req = httptest.NewRequest(http.MethodPost, "/api/demo/reset?user=2", nil)
	req.Header.Set("X-Demo-Secret", "demo-secret")
	rr = httptest.NewRecorder()
	h.DemoReset(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	statusReq = httptest.NewRequest(http.MethodGet, "/api/demo/status?user=2", nil)
	statusReq.Header.Set("X-Demo-Secret", "demo-secret")
	statusRR = httptest.NewRecorder()
	h.DemoStatus(statusRR, statusReq)
	require.Equal(t, http.StatusOK, statusRR.Code, statusRR.Body.String())

	status = DemoStatusResponse{}
	require.NoError(t, json.NewDecoder(statusRR.Body).Decode(&status))
	assert.Equal(t, 2, status.User)
	assert.GreaterOrEqual(t, status.Accounts.Count, 10)
	assert.GreaterOrEqual(t, status.Contacts.Count, 3)

	missingStatus := h.getEntityStatus(ctx, "tenant_missing", "accounts", "name")
	assert.Equal(t, 0, missingStatus.Count)
	assert.Empty(t, missingStatus.Keys)

	missingConcat := h.getEntityStatusConcat(ctx, "tenant_missing", "employees", "first_name", "last_name")
	assert.Equal(t, 0, missingConcat.Count)
	assert.Empty(t, missingConcat.Keys)

	missingPeriod := h.getEntityStatusPeriod(ctx, "tenant_missing", "payroll_runs")
	assert.Equal(t, 0, missingPeriod.Count)
	assert.Empty(t, missingPeriod.Keys)

	sql := getDemoSeedSQLForUsers([]int{1, 3})
	assert.Contains(t, sql, "demo1@example.com")
	assert.Contains(t, sql, "demo3@example.com")
	assert.Contains(t, sql, "tenant_demo1")
	assert.Contains(t, sql, "tenant_demo3")
}
