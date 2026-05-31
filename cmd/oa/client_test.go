package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/contacts"
)

func TestNewAPIClientNormalizesBaseURL(t *testing.T) {
	t.Parallel()

	client := newAPIClient("api.example.com/", "token-123")
	assert.Equal(t, "https://api.example.com", client.baseURL)
	assert.Equal(t, "token-123", client.apiToken)
}

func TestAPIClientRequestAddsAuthorizationAndDecodesJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/me", r.URL.Path)
		assert.Equal(t, "Bearer token-123", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"id":    "user-1",
			"email": "user@example.com",
			"name":  "Test User",
		})
	}))
	defer server.Close()

	client := newAPIClient(server.URL, "token-123")
	user, err := client.getCurrentUser(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "user-1", user.ID)
	assert.Equal(t, "user@example.com", user.Email)
}

func TestAPIClientListContactsBuildsQueryParameters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/tenants/tenant-1/contacts", r.URL.Path)
		assert.Equal(t, "CUSTOMER", r.URL.Query().Get("type"))
		assert.Equal(t, "true", r.URL.Query().Get("active_only"))
		assert.Equal(t, "acme", r.URL.Query().Get("search"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	}))
	defer server.Close()

	client := newAPIClient(server.URL, "token-123")
	contactsList, err := client.listContacts(context.Background(), "tenant-1", contacts.ContactFilter{
		ContactType: contacts.ContactTypeCustomer,
		ActiveOnly:  true,
		Search:      "acme",
	})
	require.NoError(t, err)
	assert.Empty(t, contactsList)
}

func TestAPIClientRequestReturnsDecodedAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "bad request payload",
		})
	}))
	defer server.Close()

	client := newAPIClient(server.URL, "token-123")
	_, err := client.getCurrentUser(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad request payload")
}

func TestAPIClientReportExportsBuildQueryParameters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer token-123", r.Header.Get("Authorization"))
		assert.Equal(t, "*/*", r.Header.Get("Accept"))

		switch r.URL.Path {
		case "/api/v1/tenants/tenant-1/reports/balance-sheet":
			assert.Equal(t, "2026-03-31", r.URL.Query().Get("as_of"))
			switch r.URL.Query().Get("format") {
			case "csv":
				_, _ = w.Write([]byte("balance,csv"))
			case "xlsx":
				_, _ = w.Write([]byte("balance-xlsx"))
			default:
				t.Fatalf("unexpected balance sheet format: %s", r.URL.RawQuery)
			}
		case "/api/v1/tenants/tenant-1/reports/income-statement":
			assert.Equal(t, "2026-01-01", r.URL.Query().Get("start"))
			assert.Equal(t, "2026-03-31", r.URL.Query().Get("end"))
			switch r.URL.Query().Get("format") {
			case "csv":
				_, _ = w.Write([]byte("income,csv"))
			case "xlsx":
				_, _ = w.Write([]byte("income-xlsx"))
			default:
				t.Fatalf("unexpected income statement format: %s", r.URL.RawQuery)
			}
		default:
			t.Fatalf("unexpected export request: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newAPIClient(server.URL, "token-123")
	balanceCSV, err := client.exportBalanceSheetCSV(context.Background(), "tenant-1", " 2026-03-31 ")
	require.NoError(t, err)
	assert.Equal(t, []byte("balance,csv"), balanceCSV)

	balanceXLSX, err := client.exportBalanceSheetXLSX(context.Background(), "tenant-1", "2026-03-31")
	require.NoError(t, err)
	assert.Equal(t, []byte("balance-xlsx"), balanceXLSX)

	incomeCSV, err := client.exportIncomeStatementCSV(context.Background(), "tenant-1", " 2026-01-01 ", " 2026-03-31 ")
	require.NoError(t, err)
	assert.Equal(t, []byte("income,csv"), incomeCSV)

	incomeXLSX, err := client.exportIncomeStatementXLSX(context.Background(), "tenant-1", "2026-01-01", "2026-03-31")
	require.NoError(t, err)
	assert.Equal(t, []byte("income-xlsx"), incomeXLSX)
}

func TestParseDaysToExpiry(t *testing.T) {
	t.Parallel()

	assert.Nil(t, parseDaysToExpiry(0))

	expiresAt := parseDaysToExpiry(7)
	require.NotNil(t, expiresAt)
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), *expiresAt, 2*time.Second)
}

func TestParseOptionalInt(t *testing.T) {
	t.Parallel()

	value, err := parseOptionalInt("")
	require.NoError(t, err)
	assert.Equal(t, 0, value)

	value, err = parseOptionalInt("14")
	require.NoError(t, err)
	assert.Equal(t, 14, value)

	_, err = parseOptionalInt("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse integer")
}
