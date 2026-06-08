package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HMB-research/open-accounting/internal/auth"
	"github.com/HMB-research/open-accounting/internal/contacts"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errReadCloser) Close() error {
	return nil
}

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

func TestAPIClientRequestRawAndDemoBranches(t *testing.T) {
	t.Parallel()

	requestCounts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/raw-success":
			requestCounts["raw-success"]++
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "*/*", r.Header.Get("Accept"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "Bearer raw-token", r.Header.Get("Authorization"))
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "value", req["key"])
			_, _ = w.Write([]byte("raw-ok"))
		case "/raw-error":
			requestCounts["raw-error"]++
			w.WriteHeader(http.StatusTeapot)
			_, _ = w.Write([]byte(`{"message":"missing error field"}`))
		case "/bad-json":
			requestCounts["bad-json"]++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{"))
		case "/api/demo/status":
			requestCounts["demo-status"]++
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "3", r.URL.Query().Get("user"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))
			assert.Equal(t, "demo-secret", r.Header.Get("X-Demo-Secret"))
			_, _ = w.Write([]byte(`{"status":"ready"}`))
		case "/api/demo/reset":
			requestCounts["demo-reset"]++
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "demo offline"})
		default:
			t.Fatalf("unexpected client helper request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := newAPIClient(server.URL, "token-123")

	raw, err := client.requestRaw(context.Background(), http.MethodPost, "/raw-success", map[string]string{"key": "value"}, " raw-token ")
	require.NoError(t, err)
	assert.Equal(t, []byte("raw-ok"), raw)

	_, err = client.requestRaw(context.Background(), http.MethodGet, "/raw-error", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "418 I'm a teapot")

	var decoded map[string]string
	err = client.request(context.Background(), http.MethodGet, "/bad-json", nil, "", &decoded)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")

	demoPayload, err := client.demoStatus(context.Background(), 3, " demo-secret ")
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":"ready"}`, string(demoPayload))

	_, err = client.demoReset(context.Background(), 0, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "demo offline")

	assert.Equal(t, map[string]int{
		"raw-success": 1,
		"raw-error":   1,
		"bad-json":    1,
		"demo-status": 1,
		"demo-reset":  1,
	}, requestCounts)
}

func TestAPIClientTransportErrorBranches(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("transport unavailable")
	client := &apiClient{
		baseURL:  "https://api.example.com",
		apiToken: "token-123",
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, transportErr
		})},
	}

	err := client.request(context.Background(), http.MethodGet, "/api/v1/me", nil, "token-123", &map[string]string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport unavailable")

	_, err = client.requestRaw(context.Background(), http.MethodGet, "/health", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport unavailable")

	_, err = client.demoStatus(context.Background(), 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transport unavailable")
}

func TestAPIClientRequestConstructionAndReadErrors(t *testing.T) {
	t.Parallel()

	client := &apiClient{baseURL: "http://[::1", httpClient: http.DefaultClient}
	err := client.request(context.Background(), http.MethodGet, "/api/v1/me", nil, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")

	_, err = client.requestRaw(context.Background(), http.MethodGet, "/health", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")

	_, err = client.demoStatus(context.Background(), 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create request")

	client = newAPIClient("https://api.example.com", "token-123")
	err = client.request(context.Background(), http.MethodPost, "/api/v1/bad", map[string]any{"bad": make(chan int)}, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encode request body")

	_, err = client.requestRaw(context.Background(), http.MethodPost, "/api/v1/bad", map[string]any{"bad": make(chan int)}, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encode request body")

	client = &apiClient{
		baseURL: "https://api.example.com",
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       errReadCloser{},
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, err = client.requestRaw(context.Background(), http.MethodGet, "/health", nil, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response")

	_, err = client.demoStatus(context.Background(), 1, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read demo response")

	client = &apiClient{
		baseURL: "https://api.example.com",
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusConflict,
				Status:     "409 Conflict",
				Body:       io.NopCloser(bytes.NewBufferString("not-json")),
				Header:     make(http.Header),
			}, nil
		})},
	}

	err = client.request(context.Background(), http.MethodGet, "/api/v1/conflict", nil, "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "409 Conflict")
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

func TestAPIClientAuthSessionRequestsUseFallbackToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 9, 30, 0, 0, time.UTC)
	requestCounts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fallback-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/security-events":
			requestCounts["security-events"]++
			assert.Equal(t, "25", r.URL.Query().Get("limit"))
			_ = json.NewEncoder(w).Encode([]auth.SecurityAuditEvent{{
				ID:        "event-1",
				Action:    auth.SecurityAuditActionLogin,
				CreatedAt: now,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/auth/sessions":
			requestCounts["sessions"]++
			assert.Equal(t, "true", r.URL.Query().Get("include_inactive"))
			_ = json.NewEncoder(w).Encode([]refreshSession{{
				ID:        "session-1",
				UserID:    "user-1",
				CreatedAt: now,
				ExpiresAt: now.Add(24 * time.Hour),
			}})
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/api/v1/auth/sessions/session%2F1":
			requestCounts["revoke-session"]++
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/auth/sessions":
			requestCounts["revoke-all"]++
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/auth/password":
			requestCounts["change-password"]++
			var req map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "old-secret", req["current_password"])
			assert.Equal(t, "new-secret", req["new_password"])
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected auth client request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	client := newAPIClient(server.URL, "fallback-token")
	events, err := client.listSecurityAuditEvents(context.Background(), 25, "")
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, auth.SecurityAuditActionLogin, events[0].Action)

	sessions, err := client.listAuthSessions(context.Background(), true, "")
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "session-1", sessions[0].ID)

	require.NoError(t, client.revokeAuthSession(context.Background(), "session/1", ""))
	require.NoError(t, client.revokeAllAuthSessions(context.Background(), ""))
	require.NoError(t, client.changePassword(context.Background(), "old-secret", "new-secret", ""))

	assert.Equal(t, map[string]int{
		"security-events": 1,
		"sessions":        1,
		"revoke-session":  1,
		"revoke-all":      1,
		"change-password": 1,
	}, requestCounts)
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
