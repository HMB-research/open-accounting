package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/tenant"
)

type apiClient struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
}

type currentUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func newAPIClient(baseURL, apiToken string) *apiClient {
	return &apiClient{
		baseURL:  normalizeBaseURL(baseURL),
		apiToken: apiToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *apiClient) login(ctx context.Context, email, password string) (*loginResponse, error) {
	var resp loginResponse
	err := c.request(ctx, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "", &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getCurrentUser(ctx context.Context) (*currentUser, error) {
	var resp currentUser
	if err := c.request(ctx, http.MethodGet, "/api/v1/me", nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listMyTenants(ctx context.Context, bearerToken string) ([]tenant.TenantMembership, error) {
	var resp []tenant.TenantMembership
	if err := c.request(ctx, http.MethodGet, "/api/v1/me/tenants", nil, bearerToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createAPIToken(ctx context.Context, tenantID string, req *apitoken.CreateRequest, bearerToken string) (*apitoken.CreateResult, error) {
	var resp apitoken.CreateResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "api-tokens"), req, bearerToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listAPITokens(ctx context.Context, tenantID string) ([]apitoken.APIToken, error) {
	var resp []apitoken.APIToken
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "api-tokens"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) revokeAPIToken(ctx context.Context, tenantID, tokenID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "api-tokens", tokenID), nil, c.apiToken, nil)
}

func (c *apiClient) listAccounts(ctx context.Context, tenantID string, activeOnly bool) ([]accounting.Account, error) {
	query := ""
	if activeOnly {
		query = "?active_only=true"
	}
	var resp []accounting.Account
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "accounts")+query, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createAccount(ctx context.Context, tenantID string, req *accounting.CreateAccountRequest) (*accounting.Account, error) {
	var resp accounting.Account
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "accounts"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importAccounts(ctx context.Context, tenantID string, req *accounting.ImportAccountsRequest) (*accounting.ImportAccountsResult, error) {
	var resp accounting.ImportAccountsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "accounts", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listContacts(ctx context.Context, tenantID string, filter contacts.ContactFilter) ([]contacts.Contact, error) {
	values := url.Values{}
	if filter.ContactType != "" {
		values.Set("type", string(filter.ContactType))
	}
	if filter.ActiveOnly {
		values.Set("active_only", "true")
	}
	if filter.Search != "" {
		values.Set("search", filter.Search)
	}

	urlPath := path.Join("/api/v1/tenants", tenantID, "contacts")
	if encoded := values.Encode(); encoded != "" {
		urlPath += "?" + encoded
	}

	var resp []contacts.Contact
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createContact(ctx context.Context, tenantID string, req *contacts.CreateContactRequest) (*contacts.Contact, error) {
	var resp contacts.Contact
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "contacts"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importContacts(ctx context.Context, tenantID string, req *contacts.ImportContactsRequest) (*contacts.ImportContactsResult, error) {
	var resp contacts.ImportContactsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "contacts", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importInvoices(ctx context.Context, tenantID string, req *invoicing.ImportInvoicesRequest) (*invoicing.ImportInvoicesResult, error) {
	var resp invoicing.ImportInvoicesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importOpeningBalances(ctx context.Context, tenantID string, req *accounting.ImportOpeningBalancesRequest) (*accounting.ImportOpeningBalancesResult, error) {
	var resp accounting.ImportOpeningBalancesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entries", "import-opening-balances"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) request(ctx context.Context, method, apiPath string, body any, bearerToken string, out any) error {
	fullURL := c.baseURL + apiPath

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, apiPath, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp)
	}

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func decodeAPIError(resp *http.Response) error {
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err == nil {
		if message, ok := payload["error"].(string); ok && strings.TrimSpace(message) != "" {
			return fmt.Errorf("%s", message)
		}
	}
	return fmt.Errorf("request failed with status %s", resp.Status)
}

func parseDaysToExpiry(days int) *time.Time {
	if days <= 0 {
		return nil
	}
	expiresAt := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return &expiresAt
}

func parseOptionalInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse integer %q: %w", value, err)
	}
	return parsed, nil
}
