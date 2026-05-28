package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

func newTestCLIApp() (*cliApp, *strings.Builder, *strings.Builder) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	return &cliApp{stdout: stdout, stderr: stderr}, stdout, stderr
}

func configureCLIEnv(t *testing.T) {
	t.Helper()

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", tempHome)
	t.Setenv("OA_BASE_URL", "")
	t.Setenv("OA_API_TOKEN", "")
	t.Setenv("OA_TENANT_ID", "")
}

func writeTempCSV(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestCLIAppRunHelpAndUnknownCommand(t *testing.T) {
	t.Parallel()

	app, stdout, _ := newTestCLIApp()

	require.NoError(t, app.run(context.Background(), nil))
	assert.Contains(t, stdout.String(), "Open Accounting CLI")

	stdout.Reset()
	require.NoError(t, app.run(context.Background(), []string{"help"}))
	assert.Contains(t, stdout.String(), "Commands:")

	err := app.run(context.Background(), []string{"nope"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown command "nope"`)
}

func TestCLIAuthInitStatusAndLogoutFlow(t *testing.T) {
	configureCLIEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/auth/login":
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt-123"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/me/tenants":
			require.Equal(t, "Bearer jwt-123", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode([]tenant.TenantMembership{
				{
					Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens":
			require.Equal(t, "Bearer jwt-123", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "oa_raw_token_123456789",
				"api_token": map[string]any{
					"id":           "token-1",
					"tenant_id":    "tenant-1",
					"user_id":      "user-1",
					"name":         "CLI Token",
					"token_prefix": "oa_raw_token_",
					"created_at":   "2026-03-12T00:00:00Z",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/me":
			require.Equal(t, "Bearer oa_raw_token_123456789", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]string{
				"id":    "user-1",
				"name":  "CLI User",
				"email": "cli@example.com",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"auth",
		"init",
		"--base-url", server.URL,
		"--email", "cli@example.com",
		"--password", "secret",
		"--tenant", "alpha",
		"--token-name", "CLI Token",
		"--expires-in-days", "30",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Stored API token for tenant Alpha (tenant-1)")
	assert.Contains(t, stdout.String(), "Token preview")

	cfg, err := loadStoredConfig()
	require.NoError(t, err)
	assert.Equal(t, server.URL, cfg.BaseURL)
	assert.Equal(t, "tenant-1", cfg.TenantID)
	assert.Equal(t, "oa_raw_token_123456789", cfg.APIToken)

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "status"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "CLI User <cli@example.com>")
	assert.Contains(t, stdout.String(), "Tenant: Alpha (tenant-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"auth", "logout"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Removed local CLI config")

	_, err = loadStoredConfig()
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestCLITokenCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":           "token-1",
				"name":         "CLI",
				"token_prefix": "oa_token_123",
				"created_at":   "2026-03-12T00:00:00Z",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "oa_created_token",
				"api_token": map[string]any{
					"id":           "token-2",
					"name":         "Nightly",
					"token_prefix": "oa_created_to",
					"created_at":   "2026-03-12T00:00:00Z",
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/api-tokens/token-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"tokens", "list", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "CLI"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"tokens", "create", "--name", "Nightly"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created token Nightly (token-2)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tokens", "revoke", "--id", "token-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Revoked token token-1")
}

func TestCLIAccountsCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	importFile := writeTempCSV(t, "accounts.csv", "code,name,type\n1000,Cash,ASSET\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/accounts":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":           "acc-1",
				"code":         "1000",
				"name":         "Cash",
				"account_type": "ASSET",
				"is_active":    true,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/accounts":
			var req accounting.CreateAccountRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, accounting.AccountTypeAsset, req.AccountType)
			assert.Equal(t, "1000", req.Code)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "acc-1",
				"code":         req.Code,
				"name":         req.Name,
				"account_type": req.AccountType,
				"is_active":    true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/accounts/acc-1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "acc-1",
				"tenant_id":    "tenant-1",
				"code":         "1000",
				"name":         "Cash",
				"account_type": "ASSET",
				"is_active":    true,
				"is_system":    false,
				"description":  "Main cash account",
				"created_at":   "2026-03-12T00:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/accounts/import":
			var req accounting.ImportAccountsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "accounts.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "Cash")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":   1,
				"accounts_created": 1,
				"rows_skipped":     0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"accounts", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"code": "1000"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"accounts",
		"create",
		"--code", "1000",
		"--name", "Cash",
		"--type", "asset",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created account 1000 (acc-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"accounts", "get", "--id", "acc-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Account 1000 Cash")
	assert.Contains(t, stdout.String(), "Main cash account")

	stdout.Reset()
	err = app.run(context.Background(), []string{"accounts", "import", "--file", importFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 accounts, skipped 0 rows")
}

func TestCLIInvoiceLifecycleCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	invoicePayload := func(status string) map[string]any {
		return map[string]any{
			"id":             "inv-1",
			"tenant_id":      "tenant-1",
			"invoice_number": "INV-00001",
			"invoice_type":   "SALES",
			"contact_id":     "contact-1",
			"contact": map[string]any{
				"id":           "contact-1",
				"name":         "Acme",
				"contact_type": "CUSTOMER",
				"is_active":    true,
			},
			"issue_date":      "2026-03-15T00:00:00Z",
			"due_date":        "2026-03-29T00:00:00Z",
			"currency":        "EUR",
			"exchange_rate":   "1.00",
			"subtotal":        "180.00",
			"vat_amount":      "39.60",
			"total":           "219.60",
			"base_subtotal":   "180.00",
			"base_vat_amount": "39.60",
			"base_total":      "219.60",
			"amount_paid":     "0.00",
			"status":          status,
			"reference":       "REF-1",
			"notes":           "March services",
			"created_at":      "2026-03-15T12:00:00Z",
			"created_by":      "user-1",
			"updated_at":      "2026-03-15T12:00:00Z",
			"lines": []map[string]any{{
				"id":               "line-1",
				"tenant_id":        "tenant-1",
				"invoice_id":       "inv-1",
				"line_number":      1,
				"description":      "Consulting",
				"quantity":         "2.00",
				"unit":             "hour",
				"unit_price":       "100.00",
				"discount_percent": "10.00",
				"vat_rate":         "22.00",
				"line_subtotal":    "180.00",
				"line_vat":         "39.60",
				"line_total":       "219.60",
				"account_id":       "acc-1",
				"product_id":       "prod-1",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices":
			w.Header().Set("Content-Type", "application/json")
			require.Equal(t, "SALES", r.URL.Query().Get("type"))
			require.Equal(t, "DRAFT", r.URL.Query().Get("status"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			require.Equal(t, "INV", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{invoicePayload("DRAFT")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices":
			w.Header().Set("Content-Type", "application/json")
			var req invoicing.CreateInvoiceRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, invoicing.InvoiceTypeSales, req.InvoiceType)
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-15", req.IssueDate.Format("2006-01-02"))
			assert.Equal(t, "2026-03-29", req.DueDate.Format("2006-01-02"))
			assert.Equal(t, "EUR", req.Currency)
			require.Len(t, req.Lines, 1)
			line := req.Lines[0]
			assert.Equal(t, "Consulting", line.Description)
			assert.True(t, line.Quantity.Equal(decimal.RequireFromString("2.00")))
			assert.True(t, line.UnitPrice.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, line.DiscountPercent.Equal(decimal.RequireFromString("10.00")))
			assert.True(t, line.VATRate.Equal(decimal.RequireFromString("22.00")))
			require.NotNil(t, line.AccountID)
			require.NotNil(t, line.ProductID)
			assert.Equal(t, "acc-1", *line.AccountID)
			assert.Equal(t, "prod-1", *line.ProductID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(invoicePayload("DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(invoicePayload("DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/pdf":
			w.Header().Set("Content-Type", "application/pdf")
			_, _ = w.Write([]byte("%PDF-1.4 invoice"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/send":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/inv-1/void":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "voided"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"invoices", "list",
		"--type", "sales",
		"--status", "draft",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--search", "INV",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"invoice_number": "INV-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"invoices", "create",
		"--type", "sales",
		"--contact-id", "contact-1",
		"--issue-date", "2026-03-15",
		"--due-date", "2026-03-29",
		"--currency", "eur",
		"--reference", "REF-1",
		"--notes", "March services",
		"--line", "description=Consulting,quantity=2,unit=hour,unit_price=100.00,discount_percent=10.00,vat_rate=22.00,account_id=acc-1,product_id=prod-1",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created invoice INV-00001 (inv-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "get", "--id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Invoice INV-00001")
	assert.Contains(t, stdout.String(), "Consulting")

	stdout.Reset()
	outputPath := filepath.Join(t.TempDir(), "invoice.pdf")
	err = app.run(context.Background(), []string{"invoices", "pdf", "--id", "inv-1", "--output", outputPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote Invoice PDF")
	pdf, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.4 invoice", string(pdf))

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "send", "--id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Sent invoice inv-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "void", "--id", "inv-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Voided invoice inv-1")
}

func TestCLIQuoteCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	quotePayload := func(id, number, status string) map[string]any {
		return map[string]any{
			"id":           id,
			"tenant_id":    "tenant-1",
			"quote_number": number,
			"contact_id":   "contact-1",
			"contact": map[string]any{
				"id":           "contact-1",
				"name":         "Acme",
				"contact_type": "CUSTOMER",
				"is_active":    true,
			},
			"quote_date":    "2026-03-15T00:00:00Z",
			"valid_until":   "2026-04-15T00:00:00Z",
			"status":        status,
			"currency":      "EUR",
			"exchange_rate": "1.00",
			"subtotal":      "180.00",
			"vat_amount":    "39.60",
			"total":         "219.60",
			"notes":         "March offer",
			"created_at":    "2026-03-15T12:00:00Z",
			"created_by":    "user-1",
			"updated_at":    "2026-03-15T12:00:00Z",
			"lines": []map[string]any{{
				"id":               "line-1",
				"tenant_id":        "tenant-1",
				"quote_id":         id,
				"line_number":      1,
				"description":      "Consulting",
				"quantity":         "2.00",
				"unit":             "hour",
				"unit_price":       "100.00",
				"discount_percent": "10.00",
				"vat_rate":         "22.00",
				"line_subtotal":    "180.00",
				"line_vat":         "39.60",
				"line_total":       "219.60",
				"product_id":       "prod-1",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/quotes":
			require.Equal(t, "DRAFT", r.URL.Query().Get("status"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			require.Equal(t, "QUO", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{quotePayload("quote-1", "QUO-00001", "DRAFT")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes":
			var req quotes.CreateQuoteRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-15", req.QuoteDate.Format("2006-01-02"))
			require.NotNil(t, req.ValidUntil)
			assert.Equal(t, "2026-04-15", req.ValidUntil.Format("2006-01-02"))
			assert.Equal(t, "EUR", req.Currency)
			assert.Equal(t, "March offer", req.Notes)
			require.Len(t, req.Lines, 1)
			line := req.Lines[0]
			assert.Equal(t, "Consulting", line.Description)
			assert.True(t, line.Quantity.Equal(decimal.RequireFromString("2.00")))
			assert.True(t, line.UnitPrice.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, line.DiscountPercent.Equal(decimal.RequireFromString("10.00")))
			assert.True(t, line.VATRate.Equal(decimal.RequireFromString("22.00")))
			require.NotNil(t, line.ProductID)
			assert.Equal(t, "prod-1", *line.ProductID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(quotePayload("quote-1", "QUO-00001", "DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1":
			_ = json.NewEncoder(w).Encode(quotePayload("quote-1", "QUO-00001", "DRAFT"))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1":
			var req quotes.UpdateQuoteRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-16", req.QuoteDate.Format("2006-01-02"))
			assert.Equal(t, "Updated offer", req.Notes)
			require.Len(t, req.Lines, 1)
			assert.Equal(t, "Updated consulting", req.Lines[0].Description)
			_ = json.NewEncoder(w).Encode(quotePayload("quote-1", "QUO-00002", "DRAFT"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1/send":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1/accept":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1/reject":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "rejected"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/quotes/quote-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"quotes", "list",
		"--status", "draft",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--search", "QUO",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"quote_number": "QUO-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"quotes", "create",
		"--contact-id", "contact-1",
		"--quote-date", "2026-03-15",
		"--valid-until", "2026-04-15",
		"--currency", "eur",
		"--notes", "March offer",
		"--line", "description=Consulting,quantity=2,unit=hour,unit_price=100.00,discount_percent=10.00,vat_rate=22.00,product_id=prod-1",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created quote QUO-00001 (quote-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "get", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Quote QUO-00001")
	assert.Contains(t, stdout.String(), "Consulting")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"quotes", "update",
		"--id", "quote-1",
		"--contact-id", "contact-1",
		"--quote-date", "2026-03-16",
		"--currency", "eur",
		"--notes", "Updated offer",
		"--line", "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Quote QUO-00002")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "send", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Sent quote quote-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "accept", "--id", "quote-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "accepted"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "reject", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Rejected quote quote-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"quotes", "delete", "--id", "quote-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted quote quote-1")
}

func TestCLIOrderCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	orderPayload := func(id, number, status string) map[string]any {
		return map[string]any{
			"id":           id,
			"tenant_id":    "tenant-1",
			"order_number": number,
			"contact_id":   "contact-1",
			"contact": map[string]any{
				"id":           "contact-1",
				"name":         "Acme",
				"contact_type": "CUSTOMER",
				"is_active":    true,
			},
			"order_date":        "2026-03-15T00:00:00Z",
			"expected_delivery": "2026-03-22T00:00:00Z",
			"status":            status,
			"currency":          "EUR",
			"exchange_rate":     "1.00",
			"subtotal":          "180.00",
			"vat_amount":        "39.60",
			"total":             "219.60",
			"notes":             "March order",
			"quote_id":          "quote-1",
			"created_at":        "2026-03-15T12:00:00Z",
			"created_by":        "user-1",
			"updated_at":        "2026-03-15T12:00:00Z",
			"lines": []map[string]any{{
				"id":               "line-1",
				"tenant_id":        "tenant-1",
				"order_id":         id,
				"line_number":      1,
				"description":      "Consulting",
				"quantity":         "2.00",
				"unit":             "hour",
				"unit_price":       "100.00",
				"discount_percent": "10.00",
				"vat_rate":         "22.00",
				"line_subtotal":    "180.00",
				"line_vat":         "39.60",
				"line_total":       "219.60",
				"product_id":       "prod-1",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/orders":
			require.Equal(t, "CONFIRMED", r.URL.Query().Get("status"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			require.Equal(t, "ORD", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{orderPayload("order-1", "ORD-00001", "CONFIRMED")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders":
			var req orders.CreateOrderRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-15", req.OrderDate.Format("2006-01-02"))
			require.NotNil(t, req.ExpectedDelivery)
			assert.Equal(t, "2026-03-22", req.ExpectedDelivery.Format("2006-01-02"))
			assert.Equal(t, "EUR", req.Currency)
			assert.Equal(t, "March order", req.Notes)
			require.NotNil(t, req.QuoteID)
			assert.Equal(t, "quote-1", *req.QuoteID)
			require.Len(t, req.Lines, 1)
			line := req.Lines[0]
			assert.Equal(t, "Consulting", line.Description)
			assert.True(t, line.Quantity.Equal(decimal.RequireFromString("2.00")))
			assert.True(t, line.UnitPrice.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, line.DiscountPercent.Equal(decimal.RequireFromString("10.00")))
			assert.True(t, line.VATRate.Equal(decimal.RequireFromString("22.00")))
			require.NotNil(t, line.ProductID)
			assert.Equal(t, "prod-1", *line.ProductID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(orderPayload("order-1", "ORD-00001", "PENDING"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1":
			_ = json.NewEncoder(w).Encode(orderPayload("order-1", "ORD-00001", "CONFIRMED"))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1":
			var req orders.UpdateOrderRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contact-1", req.ContactID)
			assert.Equal(t, "2026-03-16", req.OrderDate.Format("2006-01-02"))
			assert.Equal(t, "Updated order", req.Notes)
			require.Len(t, req.Lines, 1)
			assert.Equal(t, "Updated consulting", req.Lines[0].Description)
			_ = json.NewEncoder(w).Encode(orderPayload("order-1", "ORD-00002", "CONFIRMED"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/confirm":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "confirmed"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/process":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "processing"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/ship":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "shipped"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/deliver":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "delivered"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1/cancel":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "canceled"})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/orders/order-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"orders", "list",
		"--status", "confirmed",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--search", "ORD",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"order_number": "ORD-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"orders", "create",
		"--contact-id", "contact-1",
		"--order-date", "2026-03-15",
		"--expected-delivery", "2026-03-22",
		"--currency", "eur",
		"--notes", "March order",
		"--quote-id", "quote-1",
		"--line", "description=Consulting,quantity=2,unit=hour,unit_price=100.00,discount_percent=10.00,vat_rate=22.00,product_id=prod-1",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created order ORD-00001 (order-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "get", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Order ORD-00001")
	assert.Contains(t, stdout.String(), "Consulting")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"orders", "update",
		"--id", "order-1",
		"--contact-id", "contact-1",
		"--order-date", "2026-03-16",
		"--currency", "eur",
		"--notes", "Updated order",
		"--line", "description=Updated consulting,quantity=3,unit=hour,unit_price=100.00,vat_rate=22.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Order ORD-00002")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "confirm", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Confirmed order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "process", "--id", "order-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"status": "processing"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "ship", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Shipped order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "deliver", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Delivered order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "cancel", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Canceled order order-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"orders", "delete", "--id", "order-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted order order-1")
}

func TestCLIAssetCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	categoryPayload := map[string]any{
		"id":                                  "cat-1",
		"tenant_id":                           "tenant-1",
		"name":                                "Equipment",
		"description":                         "Office equipment",
		"depreciation_method":                 "STRAIGHT_LINE",
		"default_useful_life_months":          60,
		"default_residual_value_percent":      "10.00",
		"asset_account_id":                    "acc-asset",
		"depreciation_expense_account_id":     "acc-expense",
		"accumulated_depreciation_account_id": "acc-accum",
		"created_at":                          "2026-03-15T12:00:00Z",
		"updated_at":                          "2026-03-15T12:00:00Z",
	}
	assetPayload := func(status string) map[string]any {
		return map[string]any{
			"id":                                  "asset-1",
			"tenant_id":                           "tenant-1",
			"asset_number":                        "FA-00001",
			"name":                                "Laptop",
			"description":                         "Developer laptop",
			"category_id":                         "cat-1",
			"status":                              status,
			"purchase_date":                       "2026-03-15T00:00:00Z",
			"purchase_cost":                       "1200.00",
			"supplier_id":                         "supplier-1",
			"serial_number":                       "SN-1",
			"location":                            "Tallinn",
			"depreciation_method":                 "STRAIGHT_LINE",
			"useful_life_months":                  36,
			"residual_value":                      "100.00",
			"depreciation_start_date":             "2026-04-01T00:00:00Z",
			"accumulated_depreciation":            "50.00",
			"book_value":                          "1150.00",
			"last_depreciation_date":              "2026-04-30T00:00:00Z",
			"asset_account_id":                    "acc-asset",
			"depreciation_expense_account_id":     "acc-expense",
			"accumulated_depreciation_account_id": "acc-accum",
			"created_at":                          "2026-03-15T12:00:00Z",
			"created_by":                          "user-1",
			"updated_at":                          "2026-03-15T12:00:00Z",
		}
	}
	depreciationPayload := map[string]any{
		"id":                  "dep-1",
		"tenant_id":           "tenant-1",
		"asset_id":            "asset-1",
		"depreciation_date":   "2026-04-30T00:00:00Z",
		"period_start":        "2026-04-01T00:00:00Z",
		"period_end":          "2026-04-30T00:00:00Z",
		"depreciation_amount": "25.00",
		"accumulated_total":   "75.00",
		"book_value_after":    "1125.00",
		"created_at":          "2026-04-30T12:00:00Z",
		"created_by":          "user-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories":
			_ = json.NewEncoder(w).Encode([]map[string]any{categoryPayload})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories":
			var req assets.CreateCategoryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Equipment", req.Name)
			assert.Equal(t, assets.DepreciationStraightLine, req.DepreciationMethod)
			assert.Equal(t, 60, req.DefaultUsefulLifeMonths)
			assert.True(t, req.DefaultResidualValuePercent.Equal(decimal.RequireFromString("10.00")))
			require.NotNil(t, req.AssetAccountID)
			assert.Equal(t, "acc-asset", *req.AssetAccountID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(categoryPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories/cat-1":
			_ = json.NewEncoder(w).Encode(categoryPayload)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/asset-categories/cat-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/assets":
			require.Equal(t, "ACTIVE", r.URL.Query().Get("status"))
			require.Equal(t, "cat-1", r.URL.Query().Get("category_id"))
			require.Equal(t, "Laptop", r.URL.Query().Get("search"))
			_ = json.NewEncoder(w).Encode([]map[string]any{assetPayload("ACTIVE")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets":
			var req assets.CreateAssetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Laptop", req.Name)
			assert.Equal(t, "2026-03-15", req.PurchaseDate.Format("2006-01-02"))
			assert.True(t, req.PurchaseCost.Equal(decimal.RequireFromString("1200.00")))
			assert.Equal(t, assets.DepreciationStraightLine, req.DepreciationMethod)
			assert.Equal(t, 36, req.UsefulLifeMonths)
			assert.True(t, req.ResidualValue.Equal(decimal.RequireFromString("100.00")))
			require.NotNil(t, req.DepreciationStartDate)
			assert.Equal(t, "2026-04-01", req.DepreciationStartDate.Format("2006-01-02"))
			require.NotNil(t, req.CategoryID)
			require.NotNil(t, req.SupplierID)
			assert.Equal(t, "cat-1", *req.CategoryID)
			assert.Equal(t, "supplier-1", *req.SupplierID)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(assetPayload("DRAFT"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1":
			_ = json.NewEncoder(w).Encode(assetPayload("ACTIVE"))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1":
			var req assets.UpdateAssetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Updated laptop", req.Name)
			assert.Equal(t, "Tartu", req.Location)
			assert.Equal(t, 48, req.UsefulLifeMonths)
			assert.True(t, req.ResidualValue.Equal(decimal.RequireFromString("150.00")))
			_ = json.NewEncoder(w).Encode(assetPayload("ACTIVE"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/activate":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "active"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/dispose":
			var req assets.DisposeAssetRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2026-05-01", req.DisposalDate.Format("2006-01-02"))
			assert.Equal(t, assets.DisposalSold, req.DisposalMethod)
			assert.True(t, req.DisposalProceeds.Equal(decimal.RequireFromString("900.00")))
			assert.Equal(t, "Sold to employee", req.DisposalNotes)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "disposed"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/depreciation":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(depreciationPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1/depreciation":
			_ = json.NewEncoder(w).Encode([]map[string]any{depreciationPayload})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/assets/asset-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"assets", "categories", "list", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Equipment"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "categories", "create",
		"--name", "Equipment",
		"--description", "Office equipment",
		"--depreciation-method", "straight_line",
		"--useful-life-months", "60",
		"--residual-percent", "10.00",
		"--asset-account-id", "acc-asset",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created asset category Equipment (cat-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "categories", "get", "--id", "cat-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Asset category Equipment")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "categories", "delete", "--id", "cat-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted asset category cat-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "list", "--status", "active", "--category-id", "cat-1", "--search", "Laptop", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"asset_number": "FA-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "create",
		"--name", "Laptop",
		"--description", "Developer laptop",
		"--category-id", "cat-1",
		"--purchase-date", "2026-03-15",
		"--purchase-cost", "1200.00",
		"--supplier-id", "supplier-1",
		"--serial-number", "SN-1",
		"--location", "Tallinn",
		"--depreciation-method", "straight_line",
		"--useful-life-months", "36",
		"--residual-value", "100.00",
		"--depreciation-start-date", "2026-04-01",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created asset FA-00001 (asset-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "get", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Asset FA-00001 Laptop")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "update",
		"--id", "asset-1",
		"--name", "Updated laptop",
		"--location", "Tartu",
		"--useful-life-months", "48",
		"--residual-value", "150.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Asset FA-00001")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "activate", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Activated asset asset-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"assets", "dispose",
		"--id", "asset-1",
		"--disposal-date", "2026-05-01",
		"--method", "sold",
		"--proceeds", "900.00",
		"--notes", "Sold to employee",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Disposed asset asset-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "depreciate", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Recorded depreciation 25")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "depreciation", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "dep-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"assets", "delete", "--id", "asset-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted asset asset-1")
}

func TestCLICostCenterCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	costCenterPayload := map[string]any{
		"id":            "cc-1",
		"tenant_id":     "tenant-1",
		"code":          "CC001",
		"name":          "Sales",
		"description":   "Sales team",
		"is_active":     true,
		"budget_amount": "1000.00",
		"budget_period": "MONTHLY",
		"created_at":    "2026-03-15T12:00:00Z",
		"updated_at":    "2026-03-15T12:00:00Z",
	}
	reportPayload := map[string]any{
		"tenant_id":      "tenant-1",
		"period_start":   "2026-03-01T00:00:00Z",
		"period_end":     "2026-03-31T00:00:00Z",
		"generated_at":   "2026-03-31T12:00:00Z",
		"total_expenses": "250.00",
		"total_budget":   "1000.00",
		"cost_centers": []map[string]any{{
			"cost_center":            costCenterPayload,
			"total_expenses":         "250.00",
			"budget_amount":          "1000.00",
			"budget_used_percentage": "25.00",
			"is_over_budget":         false,
			"period_start":           "2026-03-01T00:00:00Z",
			"period_end":             "2026-03-31T00:00:00Z",
		}},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{costCenterPayload})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers":
			var req accounting.CreateCostCenterRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "CC001", req.Code)
			assert.Equal(t, "Sales", req.Name)
			assert.True(t, req.IsActive)
			require.NotNil(t, req.BudgetAmount)
			assert.True(t, req.BudgetAmount.Equal(decimal.RequireFromString("1000.00")))
			assert.Equal(t, accounting.BudgetPeriodMonthly, req.BudgetPeriod)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(costCenterPayload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/cc-1":
			_ = json.NewEncoder(w).Encode(costCenterPayload)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/cc-1":
			var req accounting.UpdateCostCenterRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "CC002", req.Code)
			assert.Equal(t, "Sales updated", req.Name)
			require.NotNil(t, req.BudgetAmount)
			assert.True(t, req.BudgetAmount.Equal(decimal.RequireFromString("1200.00")))
			payload := map[string]any{}
			for key, value := range costCenterPayload {
				payload[key] = value
			}
			payload["code"] = "CC002"
			payload["name"] = "Sales updated"
			payload["budget_amount"] = "1200.00"
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/report":
			require.Equal(t, "2026-03-01", r.URL.Query().Get("start_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("end_date"))
			_ = json.NewEncoder(w).Encode(reportPayload)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/cost-centers/cc-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"cost-centers", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"code": "CC001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"cost-centers", "create",
		"--code", "CC001",
		"--name", "Sales",
		"--description", "Sales team",
		"--budget-amount", "1000.00",
		"--budget-period", "monthly",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created cost center CC001 Sales (cc-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"cost-centers", "get", "--id", "cc-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Cost center CC001 Sales")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"cost-centers", "update",
		"--id", "cc-1",
		"--code", "CC002",
		"--name", "Sales updated",
		"--budget-amount", "1200.00",
		"--budget-period", "monthly",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Cost center CC002 Sales updated")

	stdout.Reset()
	err = app.run(context.Background(), []string{"cost-centers", "report", "--start", "2026-03-01", "--end", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Total expenses: 250")
	assert.Contains(t, stdout.String(), "Sales")

	stdout.Reset()
	err = app.run(context.Background(), []string{"cost-centers", "delete", "--id", "cc-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted cost center cc-1")
}

func TestCLIJournalEntryCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	journalPayload := func(id, number string, status accounting.JournalEntryStatus) map[string]any {
		return map[string]any{
			"id":           id,
			"tenant_id":    "tenant-1",
			"entry_number": number,
			"entry_date":   "2026-03-31T00:00:00Z",
			"description":  "Manual accrual",
			"reference":    "ACC-1",
			"source_type":  "MANUAL",
			"status":       status,
			"created_at":   "2026-03-31T12:00:00Z",
			"created_by":   "user-1",
			"lines": []map[string]any{
				{
					"id":               "line-1",
					"tenant_id":        "tenant-1",
					"journal_entry_id": id,
					"account_id":       "acc-1",
					"description":      "Expense",
					"debit_amount":     "100.00",
					"credit_amount":    "0.00",
					"currency":         "EUR",
					"exchange_rate":    "1.00",
					"base_debit":       "100.00",
					"base_credit":      "0.00",
				},
				{
					"id":               "line-2",
					"tenant_id":        "tenant-1",
					"journal_entry_id": id,
					"account_id":       "acc-2",
					"description":      "Accrual",
					"debit_amount":     "0.00",
					"credit_amount":    "100.00",
					"currency":         "EUR",
					"exchange_rate":    "1.00",
					"base_debit":       "0.00",
					"base_credit":      "100.00",
				},
			},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries":
			require.Equal(t, "25", r.URL.Query().Get("limit"))
			_ = json.NewEncoder(w).Encode([]map[string]any{journalPayload("je-1", "JE-2026-001", accounting.StatusDraft)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries":
			var req accounting.CreateJournalEntryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "2026-03-31", req.EntryDate.Format("2006-01-02"))
			assert.Equal(t, "Manual accrual", req.Description)
			assert.Equal(t, "ACC-1", req.Reference)
			require.Len(t, req.Lines, 2)
			assert.Equal(t, "acc-1", req.Lines[0].AccountID)
			assert.True(t, req.Lines[0].DebitAmount.Equal(decimal.RequireFromString("100.00")))
			assert.True(t, req.Lines[1].CreditAmount.Equal(decimal.RequireFromString("100.00")))
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(journalPayload("je-1", "JE-2026-001", accounting.StatusDraft))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/je-1":
			_ = json.NewEncoder(w).Encode(journalPayload("je-1", "JE-2026-001", accounting.StatusDraft))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/je-1/post":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "posted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/je-1/void":
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Duplicate entry", req["reason"])
			payload := journalPayload("je-rev-1", "JE-2026-002", accounting.StatusPosted)
			payload["void_reason"] = "Duplicate entry"
			_ = json.NewEncoder(w).Encode(payload)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"journal", "list", "--limit", "25", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"entry_number": "JE-2026-001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"journal", "create",
		"--entry-date", "2026-03-31",
		"--description", "Manual accrual",
		"--reference", "ACC-1",
		"--source-type", "MANUAL",
		"--line", "account_id=acc-1,description=Expense,debit=100.00",
		"--line", "account_id=acc-2,description=Accrual,credit=100.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created journal entry JE-2026-001 (je-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"journal", "get", "--id", "je-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Journal entry JE-2026-001")
	assert.Contains(t, stdout.String(), "Balanced: true")

	stdout.Reset()
	err = app.run(context.Background(), []string{"journal", "post", "--id", "je-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Posted journal entry je-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"journal", "void", "--id", "je-1", "--reason", "Duplicate entry"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Voided journal entry je-1 with reversal JE-2026-002")
}

func TestCLIContactsInvoicesAndJournalCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	contactsFile := writeTempCSV(t, "contacts.csv", "name,email\nAcme,hello@example.com\n")
	invoicesFile := writeTempCSV(t, "invoices.csv", "invoice_number,contact_name,total\nINV-1,Acme,100\n")
	openingBalancesFile := writeTempCSV(t, "opening-balances.csv", "account_code,debit,credit\n1000,500,0\n")
	contactPayload := func(name string, active bool) map[string]any {
		return map[string]any{
			"id":                 "contact-1",
			"tenant_id":          "tenant-1",
			"code":               "CUST-1",
			"name":               name,
			"contact_type":       "CUSTOMER",
			"reg_code":           "12345678",
			"vat_number":         "EE123456789",
			"email":              "hello@example.com",
			"phone":              "+372 555 1234",
			"address_line1":      "123 Main St",
			"city":               "Tallinn",
			"postal_code":        "10111",
			"country_code":       "EE",
			"payment_terms_days": 14,
			"credit_limit":       "1500.00",
			"is_active":          active,
			"notes":              "Key customer",
			"created_at":         "2026-03-12T00:00:00Z",
			"updated_at":         "2026-03-12T00:00:00Z",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/contacts":
			require.Equal(t, "CUSTOMER", r.URL.Query().Get("type"))
			require.Equal(t, "acme", r.URL.Query().Get("search"))
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{contactPayload("Acme", true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/contacts":
			var req contacts.CreateContactRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Acme", req.Name)
			assert.True(t, req.CreditLimit.Equal(decimal.RequireFromString("1500")))
			_ = json.NewEncoder(w).Encode(contactPayload(req.Name, true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/contact-1":
			_ = json.NewEncoder(w).Encode(contactPayload("Acme", true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/contact-1":
			var req contacts.UpdateContactRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.NotNil(t, req.Name)
			assert.Equal(t, "Acme Updated", *req.Name)
			require.NotNil(t, req.PaymentTermsDays)
			assert.Equal(t, 30, *req.PaymentTermsDays)
			require.NotNil(t, req.CreditLimit)
			assert.True(t, req.CreditLimit.Equal(decimal.RequireFromString("2500.00")))
			require.NotNil(t, req.IsActive)
			assert.False(t, *req.IsActive)
			_ = json.NewEncoder(w).Encode(contactPayload("Acme Updated", false))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/contact-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/contacts/import":
			var req contacts.ImportContactsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "contacts.csv", req.FileName)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":   1,
				"contacts_created": 1,
				"rows_skipped":     0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/invoices/import":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":   1,
				"invoices_created": 1,
				"lines_imported":   1,
				"rows_skipped":     0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/journal-entries/import-opening-balances":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"journal_entry": map[string]any{
					"id":           "je-1",
					"entry_number": "JE-2026-001",
				},
				"lines_imported": 1,
				"total_debit":    "500.00",
				"total_credit":   "500.00",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"contacts",
		"list",
		"--type", "customer",
		"--search", "acme",
		"--active-only",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"name": "Acme"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"contacts",
		"create",
		"--name", "Acme",
		"--email", "hello@example.com",
		"--credit-limit", "1500",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created contact Acme (contact-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"contacts", "get", "--id", "contact-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Contact Acme")
	assert.Contains(t, stdout.String(), "Credit limit: 1500")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"contacts",
		"update",
		"--id", "contact-1",
		"--name", "Acme Updated",
		"--payment-terms-days", "30",
		"--credit-limit", "2500.00",
		"--active", "false",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Contact Acme Updated")
	assert.Contains(t, stdout.String(), "Active: false")

	stdout.Reset()
	err = app.run(context.Background(), []string{"contacts", "delete", "--id", "contact-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted contact contact-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"contacts", "import", "--file", contactsFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 contacts, skipped 0 rows")

	stdout.Reset()
	err = app.run(context.Background(), []string{"invoices", "import", "--file", invoicesFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 invoices, imported 1 lines, skipped 0 rows")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"journal",
		"import-opening-balances",
		"--file", openingBalancesFile,
		"--entry-date", "2026-01-01",
		"--description", "Opening balances",
		"--reference", "OB-2026",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created posted journal entry JE-2026-001")
	assert.Contains(t, stdout.String(), "debit 500")
}

func TestCLIPaymentCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	paymentPayload := func(id, number string) map[string]any {
		return map[string]any{
			"id":             id,
			"tenant_id":      "tenant-1",
			"payment_number": number,
			"payment_type":   "RECEIVED",
			"contact_id":     "contact-1",
			"payment_date":   "2026-03-15T00:00:00Z",
			"amount":         "100.00",
			"currency":       "EUR",
			"exchange_rate":  "1.00",
			"base_amount":    "100.00",
			"payment_method": "BANK_TRANSFER",
			"bank_account":   "EE471000001020145685",
			"reference":      "REF-1",
			"notes":          "March receipt",
			"created_at":     "2026-03-15T12:00:00Z",
			"created_by":     "user-1",
			"allocations": []map[string]any{{
				"id":         "alloc-1",
				"tenant_id":  "tenant-1",
				"payment_id": id,
				"invoice_id": "inv-1",
				"amount":     "60.00",
				"created_at": "2026-03-15T12:05:00Z",
			}},
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payments":
			require.Equal(t, "RECEIVED", r.URL.Query().Get("type"))
			require.Equal(t, "BANK_TRANSFER", r.URL.Query().Get("method"))
			require.Equal(t, "contact-1", r.URL.Query().Get("contact_id"))
			require.Equal(t, "2026-03-01", r.URL.Query().Get("from_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("to_date"))
			_ = json.NewEncoder(w).Encode([]map[string]any{paymentPayload("pay-1", "PMT-00001")})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payments":
			var req payments.CreatePaymentRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, payments.PaymentTypeReceived, req.PaymentType)
			require.NotNil(t, req.ContactID)
			assert.Equal(t, "contact-1", *req.ContactID)
			assert.Equal(t, "2026-03-15", req.PaymentDate.Format("2006-01-02"))
			assert.True(t, req.Amount.Equal(decimal.RequireFromString("100.00")))
			assert.Equal(t, "EUR", req.Currency)
			assert.Equal(t, "BANK_TRANSFER", req.PaymentMethod)
			assert.Equal(t, "REF-1", req.Reference)
			require.Len(t, req.Allocations, 1)
			assert.Equal(t, "inv-1", req.Allocations[0].InvoiceID)
			assert.True(t, req.Allocations[0].Amount.Equal(decimal.RequireFromString("60.00")))
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(paymentPayload("pay-1", "PMT-00001"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payments/pay-1":
			_ = json.NewEncoder(w).Encode(paymentPayload("pay-1", "PMT-00001"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payments/pay-1/allocate":
			var req payments.AllocationRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "inv-2", req.InvoiceID)
			assert.True(t, req.Amount.Equal(decimal.RequireFromString("40.00")))
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "allocated"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payments/unallocated":
			require.Equal(t, "RECEIVED", r.URL.Query().Get("type"))
			payload := paymentPayload("pay-2", "PMT-00002")
			payload["allocations"] = []map[string]any{}
			_ = json.NewEncoder(w).Encode([]map[string]any{payload})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{
		"payments", "list",
		"--type", "received",
		"--method", "BANK_TRANSFER",
		"--contact-id", "contact-1",
		"--from", "2026-03-01",
		"--to", "2026-03-31",
		"--json",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"payment_number": "PMT-00001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"payments", "create",
		"--type", "received",
		"--amount", "100.00",
		"--date", "2026-03-15",
		"--currency", "eur",
		"--method", "BANK_TRANSFER",
		"--contact-id", "contact-1",
		"--bank-account", "EE471000001020145685",
		"--reference", "REF-1",
		"--notes", "March receipt",
		"--allocate", "inv-1:60.00",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created payment PMT-00001 (pay-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payments", "get", "--id", "pay-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Payment PMT-00001")
	assert.Contains(t, stdout.String(), "inv-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payments", "allocate", "--id", "pay-1", "--invoice-id", "inv-2", "--amount", "40.00"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Allocated 40 to invoice inv-2 for payment pay-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payments", "unallocated", "--type", "received"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "PMT-00002")
}

func TestCLIReportsCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/trial-balance":
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":     "tenant-1",
				"as_of_date":    "2026-03-31T00:00:00Z",
				"generated_at":  "2026-03-31T12:00:00Z",
				"total_debits":  "500.00",
				"total_credits": "500.00",
				"is_balanced":   true,
				"accounts": []map[string]any{{
					"account_id":     "acc-1",
					"account_code":   "1000",
					"account_name":   "Cash",
					"account_type":   "ASSET",
					"debit_balance":  "500.00",
					"credit_balance": "0.00",
					"net_balance":    "500.00",
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/account-balance/acc-1":
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"account_id": "acc-1",
				"as_of_date": "2026-03-31",
				"balance":    "500.00",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/balance-sheet":
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":         "tenant-1",
				"as_of_date":        "2026-03-31T00:00:00Z",
				"generated_at":      "2026-03-31T12:00:00Z",
				"assets":            []map[string]any{},
				"liabilities":       []map[string]any{},
				"equity":            []map[string]any{},
				"total_assets":      "500.00",
				"total_liabilities": "200.00",
				"total_equity":      "300.00",
				"retained_earnings": "100.00",
				"is_balanced":       true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/income-statement":
			require.Equal(t, "2026-01-01", r.URL.Query().Get("start"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("end"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":      "tenant-1",
				"start_date":     "2026-01-01T00:00:00Z",
				"end_date":       "2026-03-31T00:00:00Z",
				"generated_at":   "2026-03-31T12:00:00Z",
				"revenue":        []map[string]any{},
				"expenses":       []map[string]any{},
				"total_revenue":  "1200.00",
				"total_expenses": "700.00",
				"net_income":     "500.00",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/cash-flow":
			require.Equal(t, "2026-01-01", r.URL.Query().Get("start_date"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("end_date"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tenant_id":            "tenant-1",
				"start_date":           "2026-01-01",
				"end_date":             "2026-03-31",
				"operating_activities": []map[string]any{{"code": "CF_OPER_TOTAL", "description": "Operating total", "amount": "500.00"}},
				"investing_activities": []map[string]any{},
				"financing_activities": []map[string]any{},
				"total_operating":      "500.00",
				"total_investing":      "0.00",
				"total_financing":      "0.00",
				"net_cash_change":      "500.00",
				"opening_cash":         "0.00",
				"closing_cash":         "500.00",
				"generated_at":         "2026-03-31T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/aging/receivables":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"report_type": "receivables",
				"as_of_date":  "2026-03-31T00:00:00Z",
				"total":       "900.00",
				"buckets": []map[string]any{{
					"label":  "Current",
					"amount": "700.00",
					"count":  3,
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/balance-confirmations":
			require.Equal(t, "RECEIVABLE", r.URL.Query().Get("type"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":          "RECEIVABLE",
				"as_of_date":    "2026-03-31",
				"total_balance": "900.00",
				"contact_count": 1,
				"invoice_count": 2,
				"contacts": []map[string]any{{
					"contact_id":    "contact-1",
					"contact_name":  "Acme",
					"contact_code":  "CUST-1",
					"contact_email": "billing@example.com",
					"balance":       "900.00",
					"invoice_count": 2,
				}},
				"generated_at": "2026-03-31T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/reports/balance-confirmations/contact-1":
			require.Equal(t, "RECEIVABLE", r.URL.Query().Get("type"))
			require.Equal(t, "2026-03-31", r.URL.Query().Get("as_of_date"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "confirmation-1",
				"tenant_id":     "tenant-1",
				"contact_id":    "contact-1",
				"contact_name":  "Acme",
				"contact_code":  "CUST-1",
				"type":          "RECEIVABLE",
				"as_of_date":    "2026-03-31",
				"total_balance": "900.00",
				"invoices": []map[string]any{{
					"invoice_id":         "invoice-1",
					"invoice_number":     "INV-1",
					"invoice_date":       "2026-03-01",
					"due_date":           "2026-03-15",
					"total_amount":       "1000.00",
					"amount_paid":        "100.00",
					"outstanding_amount": "900.00",
					"currency":           "EUR",
					"days_overdue":       16,
				}},
				"generated_at": "2026-03-31T12:00:00Z",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"reports", "trial-balance", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Trial balance as of 2026-03-31")
	assert.Contains(t, stdout.String(), "1000")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "account-balance", "--account-id", "acc-1", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "500.00")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "balance-sheet", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Balance sheet as of 2026-03-31")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "income-statement", "--start", "2026-01-01", "--end", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Net income: 500")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "cash-flow", "--start", "2026-01-01", "--end", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Closing cash: 500")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "aging", "--type", "receivables", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"report_type": "receivables"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "balance-confirmations", "--type", "receivable", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Total balance: 900")

	stdout.Reset()
	err = app.run(context.Background(), []string{"reports", "balance-confirmation", "--contact-id", "contact-1", "--type", "RECEIVABLE", "--as-of", "2026-03-31"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "INV-1")
}

func TestCLIEmployeesCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	employeesFile := writeTempCSV(t, "employees.csv", "employee_number,first_name,last_name,start_date,base_salary\nEMP-001,Mari,Maasikas,2026-01-15,3200\n")
	employeePayload := func(firstName, lastName string, active bool) map[string]any {
		return map[string]any{
			"id":                     "emp-1",
			"tenant_id":              "tenant-1",
			"employee_number":        "EMP-001",
			"first_name":             firstName,
			"last_name":              lastName,
			"personal_code":          "49001010001",
			"email":                  "mari@example.com",
			"phone":                  "+372 555 1234",
			"address":                "Tallinn",
			"bank_account":           "EE471000001020145685",
			"start_date":             "2026-01-15T00:00:00Z",
			"position":               "Accountant",
			"department":             "Finance",
			"employment_type":        "FULL_TIME",
			"tax_residency":          "EE",
			"apply_basic_exemption":  true,
			"basic_exemption_amount": "700.00",
			"funded_pension_rate":    "0.02",
			"is_active":              active,
			"created_at":             "2026-01-15T00:00:00Z",
			"updated_at":             "2026-01-15T00:00:00Z",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/employees":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{employeePayload("Mari", "Maasikas", true)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees":
			var req payroll.CreateEmployeeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Mari", req.FirstName)
			assert.Equal(t, "Maasikas", req.LastName)
			assert.Equal(t, payroll.EmploymentFullTime, req.EmploymentType)
			_ = json.NewEncoder(w).Encode(employeePayload(req.FirstName, req.LastName, true))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1":
			_ = json.NewEncoder(w).Encode(employeePayload("Mari", "Maasikas", true))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1":
			var req payroll.UpdateEmployeeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Maria", req.FirstName)
			assert.Equal(t, "Finance", req.Department)
			require.NotNil(t, req.ApplyBasicExemption)
			assert.False(t, *req.ApplyBasicExemption)
			require.NotNil(t, req.IsActive)
			assert.True(t, *req.IsActive)
			_ = json.NewEncoder(w).Encode(employeePayload("Maria", "Maasikas", true))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees/emp-1/salary":
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Contains(t, req, "amount")
			assert.Contains(t, req, "effective_from")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "salary updated"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees/import":
			var req payroll.ImportEmployeesRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "employees.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "EMP-001")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":    1,
				"employees_created": 1,
				"salaries_created":  1,
				"rows_skipped":      0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"employees", "list", "--active-only", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"employee_number": "EMP-001"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"employees",
		"create",
		"--employee-number", "EMP-001",
		"--first-name", "Mari",
		"--last-name", "Maasikas",
		"--start-date", "2026-01-15",
		"--employment-type", "FULL_TIME",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created employee Mari Maasikas (emp-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"employees", "get", "--id", "emp-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Employee Mari Maasikas")
	assert.Contains(t, stdout.String(), "Position: Accountant")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"employees", "update",
		"--id", "emp-1",
		"--first-name", "Maria",
		"--department", "Finance",
		"--apply-basic-exemption", "false",
		"--active", "true",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Employee Maria Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"employees", "set-salary", "--id", "emp-1", "--amount", "3200.00", "--effective-from", "2026-03-01"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Set base salary for employee emp-1 to 3200")

	stdout.Reset()
	err = app.run(context.Background(), []string{"employees", "import", "--file", employeesFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 employees, set 1 salaries, skipped 0 rows")
}

func TestCLIPayrollRunCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	payrollRunPayload := func(id, status string, year, month int) map[string]any {
		return map[string]any{
			"id":                  id,
			"tenant_id":           "tenant-1",
			"period_year":         year,
			"period_month":        month,
			"status":              status,
			"payment_date":        "2026-03-31T00:00:00Z",
			"total_gross":         "3200.00",
			"total_net":           "2534.80",
			"total_employer_cost": "4281.60",
			"notes":               "March payroll",
			"created_at":          "2026-03-20T12:00:00Z",
			"updated_at":          "2026-03-20T12:00:00Z",
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs":
			require.Equal(t, "2026", r.URL.Query().Get("year"))
			_ = json.NewEncoder(w).Encode([]map[string]any{payrollRunPayload("run-1", "DRAFT", 2026, 3)})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs":
			var req payroll.CreatePayrollRunRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, 2026, req.PeriodYear)
			assert.Equal(t, 3, req.PeriodMonth)
			require.NotNil(t, req.PaymentDate)
			assert.Equal(t, "March payroll", req.Notes)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(payrollRunPayload("run-1", "DRAFT", 2026, 3))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1":
			payload := payrollRunPayload("run-1", "DRAFT", 2026, 3)
			payload["payslips"] = []map[string]any{{
				"id":                              "payslip-1",
				"tenant_id":                       "tenant-1",
				"payroll_run_id":                  "run-1",
				"employee_id":                     "emp-1",
				"gross_salary":                    "3200.00",
				"taxable_income":                  "2500.00",
				"income_tax":                      "550.00",
				"unemployment_insurance_employee": "51.20",
				"funded_pension":                  "64.00",
				"net_salary":                      "2534.80",
				"social_tax":                      "1056.00",
				"unemployment_insurance_employer": "25.60",
				"total_employer_cost":             "4281.60",
				"basic_exemption_applied":         "700.00",
				"payment_status":                  "PENDING",
				"created_at":                      "2026-03-20T12:00:00Z",
				"employee": map[string]any{
					"id":              "emp-1",
					"tenant_id":       "tenant-1",
					"employee_number": "EMP-001",
					"first_name":      "Mari",
					"last_name":       "Maasikas",
					"start_date":      "2026-01-01T00:00:00Z",
					"employment_type": "FULL_TIME",
					"is_active":       true,
					"created_at":      "2026-01-01T00:00:00Z",
					"updated_at":      "2026-01-01T00:00:00Z",
				},
			}}
			_ = json.NewEncoder(w).Encode(payload)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/calculate":
			_ = json.NewEncoder(w).Encode(payrollRunPayload("run-1", "CALCULATED", 2026, 3))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/approve":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/payslips":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":                              "payslip-1",
				"tenant_id":                       "tenant-1",
				"payroll_run_id":                  "run-1",
				"employee_id":                     "emp-1",
				"gross_salary":                    "3200.00",
				"taxable_income":                  "2500.00",
				"income_tax":                      "550.00",
				"unemployment_insurance_employee": "51.20",
				"funded_pension":                  "64.00",
				"net_salary":                      "2534.80",
				"social_tax":                      "1056.00",
				"unemployment_insurance_employer": "25.60",
				"total_employer_cost":             "4281.60",
				"basic_exemption_applied":         "700.00",
				"payment_status":                  "PENDING",
				"created_at":                      "2026-03-20T12:00:00Z",
				"employee": map[string]any{
					"id":              "emp-1",
					"tenant_id":       "tenant-1",
					"employee_number": "EMP-001",
					"first_name":      "Mari",
					"last_name":       "Maasikas",
					"start_date":      "2026-01-01T00:00:00Z",
					"employment_type": "FULL_TIME",
					"is_active":       true,
					"created_at":      "2026-01-01T00:00:00Z",
					"updated_at":      "2026-01-01T00:00:00Z",
				},
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll/tax-preview":
			var req map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Contains(t, req, "gross_salary")
			assert.Equal(t, true, req["apply_basic_exemption"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"gross_salary":          "3200.00",
				"basic_exemption":       "700.00",
				"taxable_income":        "2500.00",
				"income_tax":            "550.00",
				"unemployment_employee": "51.20",
				"funded_pension":        "64.00",
				"total_deductions":      "665.20",
				"net_salary":            "2534.80",
				"social_tax":            "1056.00",
				"unemployment_employer": "25.60",
				"total_employer_cost":   "4281.60",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"payroll", "runs", "list", "--year", "2026"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"payroll", "runs", "create",
		"--year", "2026",
		"--month", "3",
		"--payment-date", "2026-03-31",
		"--notes", "March payroll",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created payroll run 2026-03 (run-1)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "get", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Payroll run 2026-03")
	assert.Contains(t, stdout.String(), "Mari Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "calculate", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "CALCULATED")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "approve", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Approved payroll run run-1")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "runs", "payslips", "--id", "run-1"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Mari Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"payroll", "tax-preview", "--gross-salary", "3200.00"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Net salary: 2534.8")
}

func TestCLIPayrollImportHistoryCommand(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	payrollFile := writeTempCSV(t, "payroll-history.csv", "period_year,period_month,employee_number,gross_salary\n2025,12,EMP-100,3200.00\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/import-history":
			var req payroll.ImportPayrollHistoryRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "payroll-history.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "EMP-100")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":       1,
				"payroll_runs_created": 1,
				"payslips_created":     1,
				"rows_skipped":         0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"payroll", "import-history", "--file", payrollFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 payroll runs, created 1 payslips, skipped 0 rows")
}

func TestCLIPayrollImportLeaveBalancesCommand(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	leaveFile := writeTempCSV(t, "leave-balances.csv", "year,employee_number,absence_type_code,entitled_days\n2025,EMP-100,ANNUAL_LEAVE,28\n")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/leave-balances/import":
			var req payroll.ImportLeaveBalancesRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "leave-balances.csv", req.FileName)
			assert.Contains(t, req.CSVContent, "ANNUAL_LEAVE")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"rows_processed":         1,
				"leave_balances_created": 1,
				"leave_balances_updated": 0,
				"rows_skipped":           0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"payroll", "import-leave-balances", "--file", leaveFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 leave balances, updated 0 leave balances, skipped 0 rows")
}

func TestCLITaxAndTSDCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":                          "tsd-1",
				"tenant_id":                   "tenant-1",
				"period_year":                 2026,
				"period_month":                3,
				"total_payments":              "3200.00",
				"total_income_tax":            "500.00",
				"total_social_tax":            "1056.00",
				"total_unemployment_employer": "25.60",
				"total_unemployment_employee": "51.20",
				"total_funded_pension":        "64.00",
				"status":                      "DRAFT",
				"created_at":                  "2026-03-31T12:00:00Z",
				"updated_at":                  "2026-03-31T12:00:00Z",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":                          "tsd-1",
				"tenant_id":                   "tenant-1",
				"period_year":                 2026,
				"period_month":                3,
				"total_payments":              "3200.00",
				"total_income_tax":            "500.00",
				"total_social_tax":            "1056.00",
				"total_unemployment_employer": "25.60",
				"total_unemployment_employee": "51.20",
				"total_funded_pension":        "64.00",
				"status":                      "DRAFT",
				"created_at":                  "2026-03-31T12:00:00Z",
				"updated_at":                  "2026-03-31T12:00:00Z",
				"rows": []map[string]any{{
					"id":                              "row-1",
					"tenant_id":                       "tenant-1",
					"declaration_id":                  "tsd-1",
					"employee_id":                     "emp-1",
					"personal_code":                   "49001010001",
					"first_name":                      "Mari",
					"last_name":                       "Maasikas",
					"payment_type":                    "10",
					"gross_payment":                   "3200.00",
					"basic_exemption":                 "700.00",
					"taxable_amount":                  "2500.00",
					"income_tax":                      "500.00",
					"social_tax":                      "1056.00",
					"unemployment_insurance_employer": "25.60",
					"unemployment_insurance_employee": "51.20",
					"funded_pension":                  "64.00",
					"created_at":                      "2026-03-31T12:00:00Z",
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/payroll-runs/run-1/tsd":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":               "tsd-2",
				"tenant_id":        "tenant-1",
				"period_year":      2026,
				"period_month":     4,
				"total_payments":   "4000.00",
				"total_income_tax": "650.00",
				"total_social_tax": "1320.00",
				"status":           "DRAFT",
				"created_at":       "2026-04-30T12:00:00Z",
				"updated_at":       "2026-04-30T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3/xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte("<TSD>ok</TSD>"))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3/csv":
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("period,total\n2026-03,3200.00\n"))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/tsd/2026/3/submit":
			w.Header().Set("Content-Type", "application/json")
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "EMTA-123", req["emta_reference"])
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tax/kmd":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":               "kmd-1",
				"tenant_id":        "tenant-1",
				"year":             2026,
				"month":            3,
				"status":           "DRAFT",
				"total_output_vat": "220.00",
				"total_input_vat":  "80.00",
				"rows":             []map[string]any{},
				"created_at":       "2026-03-31T12:00:00Z",
				"updated_at":       "2026-03-31T12:00:00Z",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/tax/kmd":
			w.Header().Set("Content-Type", "application/json")
			var req map[string]int
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, 2026, req["year"])
			assert.Equal(t, 3, req["month"])
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":               "kmd-1",
				"tenant_id":        "tenant-1",
				"year":             2026,
				"month":            3,
				"status":           "DRAFT",
				"total_output_vat": "220.00",
				"total_input_vat":  "80.00",
				"rows": []map[string]any{{
					"code":        "1",
					"description": "Taxable sales",
					"tax_base":    "1000.00",
					"tax_amount":  "220.00",
				}},
				"created_at": "2026-03-31T12:00:00Z",
				"updated_at": "2026-03-31T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/tax/kmd/2026/3/xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte("<KMD>ok</KMD>"))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"tsd", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "get", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Mari Maasikas")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "generate", "--run-id", "run-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"id": "tsd-2"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "export-xml", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Equal(t, "<TSD>ok</TSD>", stdout.String())

	stdout.Reset()
	outputPath := filepath.Join(t.TempDir(), "tsd.csv")
	err = app.run(context.Background(), []string{"tsd", "export-csv", "--year", "2026", "--month", "3", "--output", outputPath})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Wrote TSD CSV")
	exported, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(exported), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tsd", "mark-submitted", "--year", "2026", "--month", "3", "--emta-reference", "EMTA-123"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Marked TSD 2026-03 as submitted")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tax", "kmd", "list"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "2026-03")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tax", "kmd", "generate", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "KMD 2026-03")
	assert.Contains(t, stdout.String(), "Payable: 140")

	stdout.Reset()
	err = app.run(context.Background(), []string{"tax", "kmd", "export-xml", "--year", "2026", "--month", "3"})
	require.NoError(t, err)
	assert.Equal(t, "<KMD>ok</KMD>", stdout.String())
}

func TestCLIDocumentCommands(t *testing.T) {
	configureCLIEnv(t)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL:    "https://placeholder.example.com",
		TenantID:   "tenant-1",
		TenantName: "Alpha",
		TenantSlug: "alpha",
		APIToken:   "oa_saved_token",
	}))

	uploadPath := writeTempCSV(t, "evidence.txt", "statement line")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/documents":
			assert.Equal(t, "payment", r.URL.Query().Get("entity_type"))
			assert.Equal(t, "pay-1", r.URL.Query().Get("entity_id"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":            "doc-1",
				"tenant_id":     "tenant-1",
				"entity_type":   "payment",
				"entity_id":     "pay-1",
				"document_type": "receipt",
				"file_name":     "receipt.pdf",
				"content_type":  "application/pdf",
				"file_size":     1024,
				"review_status": "PENDING",
				"uploaded_by":   "user-1",
				"created_at":    "2026-03-12T00:00:00Z",
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/documents":
			require.NoError(t, r.ParseMultipartForm(2<<20))
			assert.Equal(t, "bank_transaction", r.FormValue("entity_type"))
			assert.Equal(t, "txn-1", r.FormValue("entity_id"))
			assert.Equal(t, documents.DocumentTypeReconciliation, r.FormValue("document_type"))
			assert.Equal(t, "Statement evidence", r.FormValue("notes"))
			assert.Equal(t, "2027-03-31", r.FormValue("retention_until"))
			file, header, err := r.FormFile("file")
			require.NoError(t, err)
			defer func() { _ = file.Close() }()
			payload, err := io.ReadAll(file)
			require.NoError(t, err)
			assert.Equal(t, "evidence.txt", header.Filename)
			assert.Equal(t, "statement line", string(payload))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "doc-2",
				"tenant_id":     "tenant-1",
				"entity_type":   "bank_transaction",
				"entity_id":     "txn-1",
				"document_type": "reconciliation_evidence",
				"file_name":     "evidence.txt",
				"content_type":  "text/plain",
				"file_size":     len(payload),
				"review_status": "PENDING",
				"uploaded_by":   "user-1",
				"created_at":    "2026-03-12T00:00:00Z",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/documents/doc-2/mark-reviewed":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "doc-2",
				"tenant_id":     "tenant-1",
				"entity_type":   "bank_transaction",
				"entity_id":     "txn-1",
				"document_type": "reconciliation_evidence",
				"file_name":     "evidence.txt",
				"content_type":  "text/plain",
				"file_size":     14,
				"review_status": "REVIEWED",
				"uploaded_by":   "user-1",
				"created_at":    "2026-03-12T00:00:00Z",
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/tenants/tenant-1/documents/doc-2":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	t.Setenv("OA_BASE_URL", server.URL)

	app, stdout, _ := newTestCLIApp()

	err := app.run(context.Background(), []string{"documents", "list", "--entity-type", "payment", "--entity-id", "pay-1", "--json"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), `"file_name": "receipt.pdf"`)

	stdout.Reset()
	err = app.run(context.Background(), []string{
		"documents",
		"upload",
		"--entity-type", "bank_transaction",
		"--entity-id", "txn-1",
		"--file", uploadPath,
		"--document-type", "reconciliation_evidence",
		"--notes", "Statement evidence",
		"--retention-until", "2027-03-31",
	})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Uploaded evidence.txt (doc-2)")

	stdout.Reset()
	err = app.run(context.Background(), []string{"documents", "mark-reviewed", "--id", "doc-2"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Marked document doc-2 as reviewed")

	stdout.Reset()
	err = app.run(context.Background(), []string{"documents", "delete", "--id", "doc-2"})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Deleted document doc-2")
}

func TestCLIHelperFunctionsAndErrors(t *testing.T) {
	configureCLIEnv(t)

	app, _, _ := newTestCLIApp()

	err := app.runAuth(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth subcommand required")

	err = app.runTokens(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tokens subcommand required")

	err = app.runAccounts(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accounts subcommand required")

	err = app.runContacts(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contacts subcommand required")

	err = app.runEmployees(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "employees subcommand required")

	err = app.runInvoices(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoices subcommand required")

	err = app.runPayments(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payments subcommand required")

	err = app.runTSD(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tsd subcommand required")

	err = app.runTax(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tax subcommand required")

	err = app.runReports(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reports subcommand required")

	err = app.runDocuments(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "documents subcommand required")

	err = app.runJournal(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "journal subcommand required")

	password, err := resolvePassword("secret", false)
	require.NoError(t, err)
	assert.Equal(t, "secret", password)

	_, err = resolvePassword("", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password is required")

	originalStdin := os.Stdin
	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString("stdin-secret\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = originalStdin
	})

	password, err = resolvePassword("", true)
	require.NoError(t, err)
	assert.Equal(t, "stdin-secret", password)

	csvPath := writeTempCSV(t, "rows.csv", "code,name\n1000,Cash\n")
	content, fileName, err := readCSVInput(csvPath)
	require.NoError(t, err)
	assert.Equal(t, "rows.csv", fileName)
	assert.Contains(t, content, "Cash")

	originalStdin = os.Stdin
	r, w, err = os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString("from,stdin\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	content, fileName, err = readCSVInput("-")
	require.NoError(t, err)
	assert.Equal(t, "stdin.csv", fileName)
	assert.Equal(t, "from,stdin\n", content)

	originalStdin = os.Stdin
	r, w, err = os.Pipe()
	require.NoError(t, err)
	_, err = w.WriteString("binary-stdin")
	require.NoError(t, err)
	require.NoError(t, w.Close())
	os.Stdin = r
	data, fileName, err := readFileInput("-", "stdin.bin")
	require.NoError(t, err)
	assert.Equal(t, "stdin.bin", fileName)
	assert.Equal(t, []byte("binary-stdin"), data)

	assert.True(t, isValidAccountType(accounting.AccountTypeRevenue))
	assert.False(t, isValidAccountType("INVALID"))

	year, month, err := parseYearMonthFlags("2026", "3")
	require.NoError(t, err)
	assert.Equal(t, 2026, year)
	assert.Equal(t, 3, month)

	_, _, err = parseYearMonthFlags("2026", "13")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "month must be between 1 and 12")

	invoiceType, err := parseRequiredInvoiceType("credit_note")
	require.NoError(t, err)
	assert.Equal(t, invoicing.InvoiceTypeCreditNote, invoiceType)

	_, err = parseRequiredInvoiceType("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid invoice type")

	invoiceStatus, err := parseOptionalInvoiceStatus("partially_paid")
	require.NoError(t, err)
	assert.Equal(t, invoicing.StatusPartiallyPaid, invoiceStatus)

	quoteStatus, err := parseOptionalQuoteStatus("converted")
	require.NoError(t, err)
	assert.Equal(t, quotes.QuoteStatusConverted, quoteStatus)

	_, err = parseOptionalQuoteStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid quote status")

	orderStatus, err := parseOptionalOrderStatus("processing")
	require.NoError(t, err)
	assert.Equal(t, orders.OrderStatusProcessing, orderStatus)

	_, err = parseOptionalOrderStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid order status")

	assetStatus, err := parseOptionalAssetStatus("disposed")
	require.NoError(t, err)
	assert.Equal(t, assets.AssetStatusDisposed, assetStatus)

	_, err = parseOptionalAssetStatus("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid asset status")

	depreciationMethod, err := parseOptionalDepreciationMethod("units_of_production")
	require.NoError(t, err)
	assert.Equal(t, assets.DepreciationUnitsOfProd, depreciationMethod)

	_, err = parseOptionalDepreciationMethod("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid depreciation method")

	disposalMethod, err := parseRequiredDisposalMethod("scrapped")
	require.NoError(t, err)
	assert.Equal(t, assets.DisposalScrapped, disposalMethod)

	_, err = parseRequiredDisposalMethod("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "method is required")

	budgetPeriod, err := parseOptionalBudgetPeriod("quarterly")
	require.NoError(t, err)
	assert.Equal(t, accounting.BudgetPeriodQuarterly, budgetPeriod)

	_, err = parseOptionalBudgetPeriod("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid budget period")

	optionalAmount, err := parseOptionalNonNegativeDecimalPtr("budget-amount", "12.50")
	require.NoError(t, err)
	require.NotNil(t, optionalAmount)
	assert.True(t, optionalAmount.Equal(decimal.RequireFromString("12.50")))

	optionalAmount, err = parseOptionalNonNegativeDecimalPtr("budget-amount", "")
	require.NoError(t, err)
	assert.Nil(t, optionalAmount)

	var invoiceLines invoiceLineFlags
	require.NoError(t, invoiceLines.Set("description=Service,quantity=1,unit_price=100,vat_rate=22"))
	assert.Equal(t, "Service", invoiceLines.String())

	err = invoiceLines.Set("description=Missing price,quantity=1,vat_rate=22")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unit_price is required")

	var quoteLines quoteLineFlags
	require.NoError(t, quoteLines.Set("description=Offer,qty=2,price=50,vat=22,product=prod-1"))
	assert.Equal(t, "Offer", quoteLines.String())
	require.NotNil(t, quoteLines[0].ProductID)
	assert.Equal(t, "prod-1", *quoteLines[0].ProductID)

	err = quoteLines.Set("description=Missing vat,quantity=1,unit_price=100")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vat_rate is required")

	var orderLines orderLineFlags
	require.NoError(t, orderLines.Set("description=Order line,qty=2,price=50,vat=22,product=prod-2"))
	assert.Equal(t, "Order line", orderLines.String())
	require.NotNil(t, orderLines[0].ProductID)
	assert.Equal(t, "prod-2", *orderLines[0].ProductID)

	err = orderLines.Set("description=Missing quantity,unit_price=100,vat_rate=22")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quantity is required")

	var journalLines journalLineFlags
	require.NoError(t, journalLines.Set("account_id=acc-1,debit=100,description=Debit line"))
	require.NoError(t, journalLines.Set("account_id=acc-2,credit=100,description=Credit line"))
	assert.Equal(t, "acc-1,acc-2", journalLines.String())

	err = journalLines.Set("account_id=acc-3,debit=10,credit=10")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one")

	paymentType, err := parseRequiredPaymentType("made")
	require.NoError(t, err)
	assert.Equal(t, payments.PaymentTypeMade, paymentType)

	_, err = parseRequiredPaymentType("unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment type")

	var allocations allocationFlags
	require.NoError(t, allocations.Set("inv-1:12.50"))
	assert.Equal(t, "inv-1:12.5", allocations.String())

	err = allocations.Set("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoice-id:amount")

	var exportBuf strings.Builder
	err = writeExportOutput(&exportBuf, "", []byte("raw export"), "Raw")
	require.NoError(t, err)
	assert.Equal(t, "raw export", exportBuf.String())

	require.NoError(t, saveConfig(&cliConfig{BaseURL: "https://api.example.com"}))
	_, _, err = app.loadAuthenticatedClient()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no API token configured")
}

func TestCLIHelperEdgeCases(t *testing.T) {
	_, err := resolveTenantMembership(nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no tenant memberships found")

	memberships := []tenant.TenantMembership{
		{Tenant: tenant.Tenant{ID: "tenant-1", Name: "Alpha", Slug: "alpha"}},
	}
	_, err = resolveTenantMembership(memberships, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tenant "missing" not found`)

	tempDir := t.TempDir()
	badPath := filepath.Join(tempDir, "missing.csv")
	_, _, err = readCSVInput(badPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read file")

	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	require.NoError(t, saveConfig(&cliConfig{
		BaseURL: "https://api.example.com",
	}))
	require.NoError(t, deleteConfig())
	require.NoError(t, deleteConfig())
}

func TestLoadStoredConfigRejectsInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	path, err := configPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("{bad json"), 0o600))

	_, err = loadStoredConfig()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode config")
}

func TestParseDaysToExpiryAndOptionalIntEdgeCases(t *testing.T) {
	t.Parallel()

	assert.Nil(t, parseDaysToExpiry(-1))

	_, err := parseOptionalInt(" 42 ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse integer")

	value, err := parseOptionalInt("42")
	require.NoError(t, err)
	assert.Equal(t, 42, value)

	expiresAt := parseDaysToExpiry(1)
	require.NotNil(t, expiresAt)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), *expiresAt, 2*time.Second)
}
