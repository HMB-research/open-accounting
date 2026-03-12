package main

import (
	"context"
	"encoding/json"
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

	err = app.runInvoices(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invoices subcommand required")

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

	assert.True(t, isValidAccountType(accounting.AccountTypeRevenue))
	assert.False(t, isValidAccountType("INVALID"))

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

	value, err := parseOptionalInt(" 42 ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse integer")

	value, err = parseOptionalInt("42")
	require.NoError(t, err)
	assert.Equal(t, 42, value)

	expiresAt := parseDaysToExpiry(1)
	require.NotNil(t, expiresAt)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), *expiresAt, 2*time.Second)
}
