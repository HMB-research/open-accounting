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
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/payroll"
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
	err = app.run(context.Background(), []string{"accounts", "import", "--file", importFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 accounts, skipped 0 rows")
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/contacts":
			require.Equal(t, "CUSTOMER", r.URL.Query().Get("type"))
			require.Equal(t, "acme", r.URL.Query().Get("search"))
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":           "contact-1",
				"name":         "Acme",
				"contact_type": "CUSTOMER",
				"is_active":    true,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/contacts":
			var req contacts.CreateContactRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Acme", req.Name)
			assert.True(t, req.CreditLimit.Equal(decimal.RequireFromString("1500")))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "contact-1",
				"name":         req.Name,
				"contact_type": req.ContactType,
				"email":        req.Email,
				"is_active":    true,
			})
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "Bearer oa_saved_token", r.Header.Get("Authorization"))

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/tenants/tenant-1/employees":
			require.Equal(t, "true", r.URL.Query().Get("active_only"))
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":              "emp-1",
				"employee_number": "EMP-001",
				"first_name":      "Mari",
				"last_name":       "Maasikas",
				"employment_type": "FULL_TIME",
				"email":           "mari@example.com",
				"is_active":       true,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tenants/tenant-1/employees":
			var req payroll.CreateEmployeeRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "Mari", req.FirstName)
			assert.Equal(t, "Maasikas", req.LastName)
			assert.Equal(t, payroll.EmploymentFullTime, req.EmploymentType)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":              "emp-1",
				"employee_number": req.EmployeeNumber,
				"first_name":      req.FirstName,
				"last_name":       req.LastName,
				"employment_type": req.EmploymentType,
				"is_active":       true,
			})
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
	err = app.run(context.Background(), []string{"employees", "import", "--file", employeesFile})
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "Processed 1 rows, created 1 employees, set 1 salaries, skipped 0 rows")
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
