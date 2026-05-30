package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/HMB-research/open-accounting/internal/accounting"
	"github.com/HMB-research/open-accounting/internal/analytics"
	"github.com/HMB-research/open-accounting/internal/apitoken"
	"github.com/HMB-research/open-accounting/internal/assets"
	"github.com/HMB-research/open-accounting/internal/banking"
	"github.com/HMB-research/open-accounting/internal/contacts"
	"github.com/HMB-research/open-accounting/internal/cutover"
	"github.com/HMB-research/open-accounting/internal/documents"
	"github.com/HMB-research/open-accounting/internal/email"
	"github.com/HMB-research/open-accounting/internal/expenses"
	"github.com/HMB-research/open-accounting/internal/inventory"
	"github.com/HMB-research/open-accounting/internal/invoicing"
	"github.com/HMB-research/open-accounting/internal/orders"
	"github.com/HMB-research/open-accounting/internal/payments"
	"github.com/HMB-research/open-accounting/internal/payroll"
	"github.com/HMB-research/open-accounting/internal/plugin"
	"github.com/HMB-research/open-accounting/internal/quotes"
	"github.com/HMB-research/open-accounting/internal/recurring"
	"github.com/HMB-research/open-accounting/internal/reports"
	"github.com/HMB-research/open-accounting/internal/tax"
	"github.com/HMB-research/open-accounting/internal/tenant"
	"github.com/HMB-research/open-accounting/internal/webhooks"
)

type apiClient struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

type loginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token,omitempty"`
	TokenType    string       `json:"token_type,omitempty"`
	ExpiresIn    int          `json:"expires_in,omitempty"`
	User         *currentUser `json:"user,omitempty"`
}

type currentUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type refreshSession struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type accountBalanceReport struct {
	AccountID string `json:"account_id"`
	AsOfDate  string `json:"as_of_date"`
	Balance   string `json:"balance"`
}

type periodCloseMutationResponse struct {
	Tenant *tenant.Tenant           `json:"tenant"`
	Event  *tenant.PeriodCloseEvent `json:"event"`
}

type documentReviewSummaryRequest struct {
	EntityType string   `json:"entity_type"`
	EntityIDs  []string `json:"entity_ids"`
}

type documentRetentionUpdateRequest struct {
	RetentionUntil string `json:"retention_until,omitempty"`
	ClearRetention bool   `json:"clear_retention,omitempty"`
}

type downloadedDocument struct {
	FileName    string
	ContentType string
	Content     []byte
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

func (c *apiClient) login(ctx context.Context, email, password, tenantID string) (*loginResponse, error) {
	var resp loginResponse
	req := map[string]string{
		"email":    email,
		"password": password,
	}
	if strings.TrimSpace(tenantID) != "" {
		req["tenant_id"] = strings.TrimSpace(tenantID)
	}
	err := c.request(ctx, http.MethodPost, "/api/v1/auth/login", req, "", &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) register(ctx context.Context, email, password, name string) (*currentUser, error) {
	var resp currentUser
	err := c.request(ctx, http.MethodPost, "/api/v1/auth/register", map[string]string{
		"email":    email,
		"password": password,
		"name":     name,
	}, "", &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) refreshAccessToken(ctx context.Context, refreshToken, tenantID string) (*loginResponse, error) {
	req := map[string]string{"refresh_token": refreshToken}
	if strings.TrimSpace(tenantID) != "" {
		req["tenant_id"] = strings.TrimSpace(tenantID)
	}
	var resp loginResponse
	if err := c.request(ctx, http.MethodPost, "/api/v1/auth/refresh", req, "", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) logout(ctx context.Context, refreshToken string) error {
	return c.request(ctx, http.MethodPost, "/api/v1/auth/logout", map[string]string{
		"refresh_token": refreshToken,
	}, "", nil)
}

func (c *apiClient) listAuthSessions(ctx context.Context, includeInactive bool, accessToken string) ([]refreshSession, error) {
	path := "/api/v1/auth/sessions"
	if includeInactive {
		path += "?include_inactive=true"
	}
	bearerToken := strings.TrimSpace(accessToken)
	if bearerToken == "" {
		bearerToken = c.apiToken
	}
	var sessions []refreshSession
	if err := c.request(ctx, http.MethodGet, path, nil, bearerToken, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (c *apiClient) revokeAuthSession(ctx context.Context, sessionID, accessToken string) error {
	bearerToken := strings.TrimSpace(accessToken)
	if bearerToken == "" {
		bearerToken = c.apiToken
	}
	return c.request(ctx, http.MethodDelete, "/api/v1/auth/sessions/"+url.PathEscape(sessionID), nil, bearerToken, nil)
}

func (c *apiClient) changePassword(ctx context.Context, currentPassword, newPassword, accessToken string) error {
	bearerToken := strings.TrimSpace(accessToken)
	if bearerToken == "" {
		bearerToken = c.apiToken
	}
	return c.request(ctx, http.MethodPut, "/api/v1/auth/password", map[string]string{
		"current_password": currentPassword,
		"new_password":     newPassword,
	}, bearerToken, nil)
}

func (c *apiClient) health(ctx context.Context) (string, error) {
	payload, err := c.requestRaw(ctx, http.MethodGet, "/health", nil, "")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func (c *apiClient) demoStatus(ctx context.Context, user int, secret string) (json.RawMessage, error) {
	return c.demoRequest(ctx, http.MethodGet, "/api/demo/status?user="+strconv.Itoa(user), secret)
}

func (c *apiClient) demoReset(ctx context.Context, user int, secret string) (json.RawMessage, error) {
	apiPath := "/api/demo/reset"
	if user > 0 {
		apiPath += "?user=" + strconv.Itoa(user)
	}
	return c.demoRequest(ctx, http.MethodPost, apiPath, secret)
}

func (c *apiClient) demoRequest(ctx context.Context, method, apiPath, secret string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+apiPath, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(secret) != "" {
		req.Header.Set("X-Demo-Secret", strings.TrimSpace(secret))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, apiPath, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp)
	}
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read demo response: %w", err)
	}
	return json.RawMessage(payload), nil
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

func (c *apiClient) createTenant(ctx context.Context, req *tenant.CreateTenantRequest) (*tenant.Tenant, error) {
	var resp tenant.Tenant
	if err := c.request(ctx, http.MethodPost, "/api/v1/tenants", req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getTenant(ctx context.Context, tenantID string) (*tenant.Tenant, error) {
	var resp tenant.Tenant
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateTenant(ctx context.Context, tenantID string, req *tenant.UpdateTenantRequest) (*tenant.Tenant, error) {
	var resp tenant.Tenant
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) completeTenantOnboarding(ctx context.Context, tenantID string) error {
	return c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "complete-onboarding"), nil, c.apiToken, nil)
}

func (c *apiClient) listTenantAuditEvents(ctx context.Context, tenantID string, limit int) ([]tenant.TenantAuditEvent, error) {
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}

	var resp []tenant.TenantAuditEvent
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "audit-events"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listTenantUsers(ctx context.Context, tenantID string) ([]tenant.TenantUser, error) {
	var resp []tenant.TenantUser
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "users"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) removeTenantUser(ctx context.Context, tenantID, userID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "users", userID), nil, c.apiToken, nil)
}

func (c *apiClient) updateTenantUserRole(ctx context.Context, tenantID, userID, role string) error {
	return c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "users", userID, "role"), map[string]string{"role": role}, c.apiToken, nil)
}

func (c *apiClient) listInvitations(ctx context.Context, tenantID string) ([]tenant.UserInvitation, error) {
	var resp []tenant.UserInvitation
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invitations"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createInvitation(ctx context.Context, tenantID string, req *tenant.CreateInvitationRequest) (*tenant.UserInvitation, error) {
	var resp tenant.UserInvitation
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invitations"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) revokeInvitation(ctx context.Context, tenantID, invitationID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "invitations", invitationID), nil, c.apiToken, nil)
}

func (c *apiClient) getInvitationByToken(ctx context.Context, token string) (*tenant.UserInvitation, error) {
	var resp tenant.UserInvitation
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/invitations", token), nil, "", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) acceptInvitation(ctx context.Context, req *tenant.AcceptInvitationRequest) (*tenant.TenantMembership, error) {
	var resp tenant.TenantMembership
	if err := c.request(ctx, http.MethodPost, "/api/v1/invitations/accept", req, "", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listPluginRegistries(ctx context.Context) ([]plugin.Registry, error) {
	var resp []plugin.Registry
	if err := c.request(ctx, http.MethodGet, "/api/v1/admin/plugin-registries", nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) addPluginRegistry(ctx context.Context, req *plugin.CreateRegistryRequest) (*plugin.Registry, error) {
	var resp plugin.Registry
	if err := c.request(ctx, http.MethodPost, "/api/v1/admin/plugin-registries", req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) removePluginRegistry(ctx context.Context, registryID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/admin/plugin-registries", registryID), nil, c.apiToken, nil)
}

func (c *apiClient) syncPluginRegistry(ctx context.Context, registryID string) error {
	return c.request(ctx, http.MethodPost, path.Join("/api/v1/admin/plugin-registries", registryID, "sync"), nil, c.apiToken, nil)
}

func (c *apiClient) listAdminPlugins(ctx context.Context) ([]plugin.Plugin, error) {
	var resp []plugin.Plugin
	if err := c.request(ctx, http.MethodGet, "/api/v1/admin/plugins", nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) searchAdminPlugins(ctx context.Context, query string) ([]plugin.PluginSearchResult, error) {
	var resp []plugin.PluginSearchResult
	if err := c.request(ctx, http.MethodGet, "/api/v1/admin/plugins/search?q="+url.QueryEscape(query), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getAdminPlugin(ctx context.Context, pluginID string) (*plugin.Plugin, error) {
	var resp plugin.Plugin
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/admin/plugins", pluginID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) installAdminPlugin(ctx context.Context, req *plugin.InstallPluginRequest) (*plugin.Plugin, error) {
	var resp plugin.Plugin
	if err := c.request(ctx, http.MethodPost, "/api/v1/admin/plugins/install", req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) enableAdminPlugin(ctx context.Context, pluginID string, permissions []string) error {
	return c.request(ctx, http.MethodPost, path.Join("/api/v1/admin/plugins", pluginID, "enable"), &plugin.EnablePluginRequest{GrantedPermissions: permissions}, c.apiToken, nil)
}

func (c *apiClient) disableAdminPlugin(ctx context.Context, pluginID string) error {
	return c.request(ctx, http.MethodPost, path.Join("/api/v1/admin/plugins", pluginID, "disable"), nil, c.apiToken, nil)
}

func (c *apiClient) uninstallAdminPlugin(ctx context.Context, pluginID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/admin/plugins", pluginID), nil, c.apiToken, nil)
}

func (c *apiClient) listPluginPermissions(ctx context.Context) (map[string]plugin.Permission, error) {
	var resp map[string]plugin.Permission
	if err := c.request(ctx, http.MethodGet, "/api/v1/admin/plugins/permissions", nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listTenantPlugins(ctx context.Context, tenantID string) ([]plugin.TenantPlugin, error) {
	var resp []plugin.TenantPlugin
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "plugins"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listWebhookEventTypes(ctx context.Context, tenantID string) ([]string, error) {
	var resp []string
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "webhooks", "events"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listWebhookEndpoints(ctx context.Context, tenantID string, activeOnly bool) ([]webhooks.Endpoint, error) {
	values := url.Values{}
	if activeOnly {
		values.Set("active_only", "true")
	}
	var resp []webhooks.Endpoint
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "webhooks"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createWebhookEndpoint(ctx context.Context, tenantID string, req *webhooks.CreateEndpointRequest) (*webhooks.Endpoint, error) {
	var resp webhooks.Endpoint
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "webhooks"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getWebhookEndpoint(ctx context.Context, tenantID, endpointID string) (*webhooks.Endpoint, error) {
	var resp webhooks.Endpoint
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "webhooks", endpointID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateWebhookEndpoint(ctx context.Context, tenantID, endpointID string, req *webhooks.UpdateEndpointRequest) (*webhooks.Endpoint, error) {
	var resp webhooks.Endpoint
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "webhooks", endpointID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteWebhookEndpoint(ctx context.Context, tenantID, endpointID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "webhooks", endpointID), nil, c.apiToken, nil)
}

func (c *apiClient) listWebhookDeliveries(ctx context.Context, tenantID, endpointID string, limit int) ([]webhooks.Delivery, error) {
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	var resp []webhooks.Delivery
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "webhooks", endpointID, "deliveries"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) testWebhookEndpoint(ctx context.Context, tenantID, endpointID string, req *webhooks.TestDeliveryRequest) (*webhooks.DeliveryResult, error) {
	var resp webhooks.DeliveryResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "webhooks", endpointID, "test"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listExpenses(ctx context.Context, tenantID string, filter expenses.ListExpensesFilter) ([]expenses.Expense, error) {
	values := url.Values{}
	if filter.Status != "" {
		values.Set("status", string(filter.Status))
	}
	if filter.Limit > 0 {
		values.Set("limit", strconv.Itoa(filter.Limit))
	}
	var resp []expenses.Expense
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "expenses"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createExpense(ctx context.Context, tenantID string, req *expenses.CreateExpenseRequest) (*expenses.Expense, error) {
	var resp expenses.Expense
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "expenses"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importExpenses(ctx context.Context, tenantID string, req *expenses.ImportExpensesRequest) (*expenses.ImportExpensesResult, error) {
	var resp expenses.ImportExpensesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "expenses", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getExpense(ctx context.Context, tenantID, expenseID string) (*expenses.Expense, error) {
	var resp expenses.Expense
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "expenses", expenseID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateExpenseStatus(ctx context.Context, tenantID, expenseID, action string) (*expenses.Expense, error) {
	var resp expenses.Expense
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "expenses", expenseID, action), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) rejectExpense(ctx context.Context, tenantID, expenseID string, req *expenses.RejectExpenseRequest) (*expenses.Expense, error) {
	var resp expenses.Expense
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "expenses", expenseID, "reject"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) enableTenantPlugin(ctx context.Context, tenantID, pluginID string, settings json.RawMessage) error {
	return c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "plugins", pluginID, "enable"), &plugin.TenantPluginSettingsRequest{Settings: settings}, c.apiToken, nil)
}

func (c *apiClient) disableTenantPlugin(ctx context.Context, tenantID, pluginID string) error {
	return c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "plugins", pluginID, "disable"), nil, c.apiToken, nil)
}

func (c *apiClient) getTenantPluginSettings(ctx context.Context, tenantID, pluginID string) (json.RawMessage, error) {
	payload, err := c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "plugins", pluginID, "settings"), nil, c.apiToken)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(payload), nil
}

func (c *apiClient) updateTenantPluginSettings(ctx context.Context, tenantID, pluginID string, settings json.RawMessage) error {
	return c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "plugins", pluginID, "settings"), settings, c.apiToken, nil)
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

func (c *apiClient) validateMigrationBundle(ctx context.Context, tenantID string, req *cutover.ValidateBundleRequest) (*cutover.BundleValidationReport, error) {
	var resp cutover.BundleValidationReport
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "migration", "validate"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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

func (c *apiClient) getAccountHierarchy(ctx context.Context, tenantID string, activeOnly bool) ([]accounting.AccountHierarchyRow, error) {
	query := ""
	if activeOnly {
		query = "?active_only=true"
	}
	var resp []accounting.AccountHierarchyRow
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "accounts", "hierarchy")+query, nil, c.apiToken, &resp); err != nil {
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

func (c *apiClient) getAccount(ctx context.Context, tenantID, accountID string) (*accounting.Account, error) {
	var resp accounting.Account
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "accounts", accountID), nil, c.apiToken, &resp); err != nil {
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

func (c *apiClient) getContact(ctx context.Context, tenantID, contactID string) (*contacts.Contact, error) {
	var resp contacts.Contact
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "contacts", contactID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateContact(ctx context.Context, tenantID, contactID string, req *contacts.UpdateContactRequest) (*contacts.Contact, error) {
	var resp contacts.Contact
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "contacts", contactID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteContact(ctx context.Context, tenantID, contactID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "contacts", contactID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
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

func (c *apiClient) listInvoices(ctx context.Context, tenantID string, filter invoicing.InvoiceFilter) ([]invoicing.Invoice, error) {
	values := url.Values{}
	if filter.InvoiceType != "" {
		values.Set("type", string(filter.InvoiceType))
	}
	if filter.Status != "" {
		values.Set("status", string(filter.Status))
	}
	if strings.TrimSpace(filter.ContactID) != "" {
		values.Set("contact_id", strings.TrimSpace(filter.ContactID))
	}
	if filter.FromDate != nil {
		values.Set("from_date", filter.FromDate.Format("2006-01-02"))
	}
	if filter.ToDate != nil {
		values.Set("to_date", filter.ToDate.Format("2006-01-02"))
	}
	if strings.TrimSpace(filter.Search) != "" {
		values.Set("search", strings.TrimSpace(filter.Search))
	}

	var resp []invoicing.Invoice
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "invoices"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createInvoice(ctx context.Context, tenantID string, req *invoicing.CreateInvoiceRequest) (*invoicing.Invoice, error) {
	var resp invoicing.Invoice
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getInvoice(ctx context.Context, tenantID, invoiceID string) (*invoicing.Invoice, error) {
	var resp invoicing.Invoice
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) downloadInvoicePDF(ctx context.Context, tenantID, invoiceID string) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID, "pdf"), nil, c.apiToken)
}

func (c *apiClient) sendInvoice(ctx context.Context, tenantID, invoiceID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID, "send"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) voidInvoice(ctx context.Context, tenantID, invoiceID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID, "void"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listPayments(ctx context.Context, tenantID string, filter payments.PaymentFilter) ([]payments.Payment, error) {
	values := url.Values{}
	if filter.PaymentType != "" {
		values.Set("type", string(filter.PaymentType))
	}
	if strings.TrimSpace(filter.PaymentMethod) != "" {
		values.Set("method", strings.TrimSpace(filter.PaymentMethod))
	}
	if strings.TrimSpace(filter.ContactID) != "" {
		values.Set("contact_id", strings.TrimSpace(filter.ContactID))
	}
	if filter.FromDate != nil {
		values.Set("from_date", filter.FromDate.Format("2006-01-02"))
	}
	if filter.ToDate != nil {
		values.Set("to_date", filter.ToDate.Format("2006-01-02"))
	}

	var resp []payments.Payment
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "payments"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createPayment(ctx context.Context, tenantID string, req *payments.CreatePaymentRequest) (*payments.Payment, error) {
	var resp payments.Payment
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payments"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importPayments(ctx context.Context, tenantID string, req *payments.ImportPaymentsRequest) (*payments.ImportPaymentsResult, error) {
	var resp payments.ImportPaymentsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payments", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportSEPAPayments(ctx context.Context, tenantID string, req *payments.SEPAExportRequest) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payments", "sepa-export"), req, c.apiToken)
}

func (c *apiClient) getPayment(ctx context.Context, tenantID, paymentID string) (*payments.Payment, error) {
	var resp payments.Payment
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "payments", paymentID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) allocatePayment(ctx context.Context, tenantID, paymentID string, req *payments.AllocationRequest) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payments", paymentID, "allocate"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listUnallocatedPayments(ctx context.Context, tenantID string, paymentType payments.PaymentType) ([]payments.Payment, error) {
	values := url.Values{}
	if paymentType != "" {
		values.Set("type", string(paymentType))
	}

	var resp []payments.Payment
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "payments", "unallocated"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getOverdueInvoices(ctx context.Context, tenantID string) (*invoicing.OverdueInvoicesSummary, error) {
	var resp invoicing.OverdueInvoicesSummary
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invoices", "overdue"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) sendPaymentReminder(ctx context.Context, tenantID string, req *invoicing.SendReminderRequest) (*invoicing.ReminderResult, error) {
	var resp invoicing.ReminderResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices", "reminders"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) sendBulkPaymentReminders(ctx context.Context, tenantID string, req *invoicing.SendBulkRemindersRequest) (*invoicing.BulkReminderResult, error) {
	var resp invoicing.BulkReminderResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices", "reminders", "bulk"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listInvoiceReminderHistory(ctx context.Context, tenantID, invoiceID string) ([]invoicing.PaymentReminder, error) {
	var resp []invoicing.PaymentReminder
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID, "reminders"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listReminderRules(ctx context.Context, tenantID string) ([]invoicing.ReminderRule, error) {
	var resp []invoicing.ReminderRule
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "reminder-rules"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createReminderRule(ctx context.Context, tenantID string, req *invoicing.CreateReminderRuleRequest) (*invoicing.ReminderRule, error) {
	var resp invoicing.ReminderRule
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "reminder-rules"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getReminderRule(ctx context.Context, tenantID, ruleID string) (*invoicing.ReminderRule, error) {
	var resp invoicing.ReminderRule
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "reminder-rules", ruleID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateReminderRule(ctx context.Context, tenantID, ruleID string, req *invoicing.UpdateReminderRuleRequest) (*invoicing.ReminderRule, error) {
	var resp invoicing.ReminderRule
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "reminder-rules", ruleID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteReminderRule(ctx context.Context, tenantID, ruleID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "reminder-rules", ruleID), nil, c.apiToken, nil)
}

func (c *apiClient) triggerReminderRules(ctx context.Context, tenantID string) ([]invoicing.AutomatedReminderResult, error) {
	var resp []invoicing.AutomatedReminderResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "reminder-rules", "trigger"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getSMTPConfig(ctx context.Context, tenantID string) (*email.SMTPConfig, error) {
	var resp email.SMTPConfig
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "settings", "smtp"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateSMTPConfig(ctx context.Context, tenantID string, req *email.UpdateSMTPConfigRequest) error {
	var resp map[string]string
	return c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "settings", "smtp"), req, c.apiToken, &resp)
}

func (c *apiClient) testSMTP(ctx context.Context, tenantID string, req *email.TestSMTPRequest) (*email.TestSMTPResponse, error) {
	var resp email.TestSMTPResponse
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "settings", "smtp", "test"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listEmailTemplates(ctx context.Context, tenantID string) ([]email.EmailTemplate, error) {
	var resp []email.EmailTemplate
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "email-templates"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) updateEmailTemplate(ctx context.Context, tenantID string, templateType email.TemplateType, req *email.UpdateTemplateRequest) (*email.EmailTemplate, error) {
	var resp email.EmailTemplate
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "email-templates", string(templateType)), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listEmailLog(ctx context.Context, tenantID string, limit int) ([]email.EmailLog, error) {
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	pathValue := path.Join("/api/v1/tenants", tenantID, "email-log")
	if encoded := values.Encode(); encoded != "" {
		pathValue += "?" + encoded
	}

	var resp []email.EmailLog
	if err := c.request(ctx, http.MethodGet, pathValue, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) emailInvoice(ctx context.Context, tenantID, invoiceID string, req *email.SendInvoiceRequest) (*email.EmailSentResponse, error) {
	var resp email.EmailSentResponse
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID, "email"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) emailPaymentReceipt(ctx context.Context, tenantID, paymentID string, req *email.SendPaymentReceiptRequest) (*email.EmailSentResponse, error) {
	var resp email.EmailSentResponse
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payments", paymentID, "email-receipt"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getInterestSettings(ctx context.Context, tenantID string) (*invoicing.InterestSettings, error) {
	var resp invoicing.InterestSettings
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "settings", "interest"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateInterestSettings(ctx context.Context, tenantID string, req *invoicing.UpdateInterestSettingsRequest) (*invoicing.InterestSettings, error) {
	var resp invoicing.InterestSettings
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "settings", "interest"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listOverdueInvoicesWithInterest(ctx context.Context, tenantID string) ([]invoicing.InterestCalculationResult, error) {
	var resp []invoicing.InterestCalculationResult
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invoices", "overdue-with-interest"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getInvoiceInterest(ctx context.Context, tenantID, invoiceID string) (*invoicing.InterestCalculationResult, error) {
	var resp invoicing.InterestCalculationResult
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID, "interest"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listInvoiceInterestHistory(ctx context.Context, tenantID, invoiceID string) ([]invoicing.InvoiceInterest, error) {
	var resp []invoicing.InvoiceInterest
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "invoices", invoiceID, "interest", "history"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listPeriodCloseEvents(ctx context.Context, tenantID string, limit int) ([]tenant.PeriodCloseEvent, error) {
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	pathValue := path.Join("/api/v1/tenants", tenantID, "period-close-events")
	if encoded := values.Encode(); encoded != "" {
		pathValue += "?" + encoded
	}

	var resp []tenant.PeriodCloseEvent
	if err := c.request(ctx, http.MethodGet, pathValue, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) closePeriod(ctx context.Context, tenantID string, req *tenant.ClosePeriodRequest) (*periodCloseMutationResponse, error) {
	var resp periodCloseMutationResponse
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "period-close"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) reopenPeriod(ctx context.Context, tenantID string, req *tenant.ReopenPeriodRequest) (*periodCloseMutationResponse, error) {
	var resp periodCloseMutationResponse
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "period-reopen"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getYearEndCloseStatus(ctx context.Context, tenantID, periodEndDate string) (*accounting.YearEndCloseStatus, error) {
	values := url.Values{}
	values.Set("period_end_date", periodEndDate)
	var resp accounting.YearEndCloseStatus
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "year-end-close-status")+"?"+values.Encode(), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getYearEndClosePack(ctx context.Context, tenantID, periodEndDate string) (*accounting.YearEndClosePack, error) {
	values := url.Values{}
	values.Set("period_end_date", periodEndDate)
	var resp accounting.YearEndClosePack
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "year-end-close-pack")+"?"+values.Encode(), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getYearEndCloseAuditEvidence(ctx context.Context, tenantID, periodEndDate string) (*accounting.YearEndCloseAuditEvidence, error) {
	values := url.Values{}
	values.Set("period_end_date", periodEndDate)
	var resp accounting.YearEndCloseAuditEvidence
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "year-end-close-audit-evidence")+"?"+values.Encode(), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) downloadYearEndCloseAuditArchive(ctx context.Context, tenantID, periodEndDate string) ([]byte, error) {
	values := url.Values{}
	values.Set("period_end_date", periodEndDate)
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "year-end-close-audit-archive")+"?"+values.Encode(), nil, c.apiToken)
}

func (c *apiClient) createYearEndCarryForward(ctx context.Context, tenantID string, req *accounting.CreateYearEndCarryForwardRequest) (*accounting.YearEndCarryForwardResult, error) {
	var resp accounting.YearEndCarryForwardResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "year-end-carry-forward"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) reverseYearEndCarryForward(ctx context.Context, tenantID string, req *accounting.ReverseYearEndCarryForwardRequest) (*accounting.YearEndCarryForwardReversalResult, error) {
	var resp accounting.YearEndCarryForwardReversalResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "year-end-carry-forward", "reverse"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listBankAccounts(ctx context.Context, tenantID string, activeOnly bool) ([]banking.BankAccount, error) {
	values := url.Values{}
	if activeOnly {
		values.Set("active_only", "true")
	}

	var resp []banking.BankAccount
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "bank-accounts"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createBankAccount(ctx context.Context, tenantID string, req *banking.CreateBankAccountRequest) (*banking.BankAccount, error) {
	var resp banking.BankAccount
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-accounts"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getBankAccount(ctx context.Context, tenantID, accountID string) (*banking.BankAccount, error) {
	var resp banking.BankAccount
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateBankAccount(ctx context.Context, tenantID, accountID string, req *banking.UpdateBankAccountRequest) (*banking.BankAccount, error) {
	var resp banking.BankAccount
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteBankAccount(ctx context.Context, tenantID, accountID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID), nil, c.apiToken, nil)
}

func (c *apiClient) listBankMatchRules(ctx context.Context, tenantID string, filter banking.BankMatchRuleFilter) ([]banking.BankMatchRule, error) {
	values := url.Values{}
	if strings.TrimSpace(filter.BankAccountID) != "" {
		values.Set("bank_account_id", strings.TrimSpace(filter.BankAccountID))
	}
	if filter.ActiveOnly {
		values.Set("active_only", "true")
	}
	if filter.IncludeGlobal {
		values.Set("include_global", "true")
	}

	var resp []banking.BankMatchRule
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "bank-match-rules"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createBankMatchRule(ctx context.Context, tenantID string, req *banking.CreateBankMatchRuleRequest) (*banking.BankMatchRule, error) {
	var resp banking.BankMatchRule
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-match-rules"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getBankMatchRule(ctx context.Context, tenantID, ruleID string) (*banking.BankMatchRule, error) {
	var resp banking.BankMatchRule
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "bank-match-rules", ruleID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateBankMatchRule(ctx context.Context, tenantID, ruleID string, req *banking.UpdateBankMatchRuleRequest) (*banking.BankMatchRule, error) {
	var resp banking.BankMatchRule
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "bank-match-rules", ruleID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteBankMatchRule(ctx context.Context, tenantID, ruleID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "bank-match-rules", ruleID), nil, c.apiToken, nil)
}

func (c *apiClient) listBankTransactions(ctx context.Context, tenantID, accountID string, filter banking.TransactionFilter) ([]banking.BankTransaction, error) {
	values := url.Values{}
	if filter.Status != "" {
		values.Set("status", string(filter.Status))
	}
	if filter.FromDate != nil {
		values.Set("from_date", filter.FromDate.Format("2006-01-02"))
	}
	if filter.ToDate != nil {
		values.Set("to_date", filter.ToDate.Format("2006-01-02"))
	}

	var resp []banking.BankTransaction
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID, "transactions"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) importBankTransactions(ctx context.Context, tenantID, accountID string, req *banking.ImportCSVRequest) (*banking.ImportResult, error) {
	var resp banking.ImportResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID, "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listBankImportHistory(ctx context.Context, tenantID, accountID string) ([]banking.BankStatementImport, error) {
	var resp []banking.BankStatementImport
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID, "import-history"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getBankTransaction(ctx context.Context, tenantID, transactionID string) (*banking.BankTransaction, error) {
	var resp banking.BankTransaction
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "bank-transactions", transactionID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listBankMatchSuggestions(ctx context.Context, tenantID, transactionID string) ([]banking.MatchSuggestion, error) {
	var resp []banking.MatchSuggestion
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "bank-transactions", transactionID, "suggestions"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) matchBankTransaction(ctx context.Context, tenantID, transactionID string, req *banking.MatchTransactionRequest) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-transactions", transactionID, "match"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) unmatchBankTransaction(ctx context.Context, tenantID, transactionID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-transactions", transactionID, "unmatch"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) reviewBankTransaction(ctx context.Context, tenantID, transactionID string, req *banking.UpdateTransactionReviewRequest) (*banking.BankTransaction, error) {
	var resp banking.BankTransaction
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-transactions", transactionID, "review"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) createPaymentFromBankTransaction(ctx context.Context, tenantID, transactionID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-transactions", transactionID, "create-payment"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listBankReconciliations(ctx context.Context, tenantID, accountID string) ([]banking.BankReconciliation, error) {
	var resp []banking.BankReconciliation
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID, "reconciliations"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createBankReconciliation(ctx context.Context, tenantID, accountID string, req *banking.CreateReconciliationRequest) (*banking.BankReconciliation, error) {
	var resp banking.BankReconciliation
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID, "reconciliation"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getBankReconciliation(ctx context.Context, tenantID, reconciliationID string) (*banking.BankReconciliation, error) {
	var resp banking.BankReconciliation
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "reconciliations", reconciliationID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) completeBankReconciliation(ctx context.Context, tenantID, reconciliationID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "reconciliations", reconciliationID, "complete"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) autoMatchBankTransactions(ctx context.Context, tenantID, accountID string, minConfidence float64) (map[string]int, error) {
	values := url.Values{}
	if minConfidence > 0 {
		values.Set("min_confidence", strconv.FormatFloat(minConfidence, 'f', -1, 64))
	}

	var resp map[string]int
	if err := c.request(ctx, http.MethodPost, withQuery(path.Join("/api/v1/tenants", tenantID, "bank-accounts", accountID, "auto-match"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listQuotes(ctx context.Context, tenantID string, filter quotes.QuoteFilter) ([]quotes.Quote, error) {
	values := url.Values{}
	if filter.Status != "" {
		values.Set("status", string(filter.Status))
	}
	if strings.TrimSpace(filter.ContactID) != "" {
		values.Set("contact_id", strings.TrimSpace(filter.ContactID))
	}
	if filter.FromDate != nil {
		values.Set("from_date", filter.FromDate.Format("2006-01-02"))
	}
	if filter.ToDate != nil {
		values.Set("to_date", filter.ToDate.Format("2006-01-02"))
	}
	if strings.TrimSpace(filter.Search) != "" {
		values.Set("search", strings.TrimSpace(filter.Search))
	}

	var resp []quotes.Quote
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "quotes"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createQuote(ctx context.Context, tenantID string, req *quotes.CreateQuoteRequest) (*quotes.Quote, error) {
	var resp quotes.Quote
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "quotes"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importQuotes(ctx context.Context, tenantID string, req *quotes.ImportQuotesRequest) (*quotes.ImportQuotesResult, error) {
	var resp quotes.ImportQuotesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "quotes", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getQuote(ctx context.Context, tenantID, quoteID string) (*quotes.Quote, error) {
	var resp quotes.Quote
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "quotes", quoteID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateQuote(ctx context.Context, tenantID, quoteID string, req *quotes.UpdateQuoteRequest) (*quotes.Quote, error) {
	var resp quotes.Quote
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "quotes", quoteID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteQuote(ctx context.Context, tenantID, quoteID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "quotes", quoteID), nil, c.apiToken, nil)
}

func (c *apiClient) updateQuoteStatus(ctx context.Context, tenantID, quoteID, action string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "quotes", quoteID, action), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) convertQuoteToInvoice(ctx context.Context, tenantID, quoteID string, req *quotes.ConvertQuoteToInvoiceRequest) (*quotes.QuoteInvoiceConversionResult, error) {
	var resp quotes.QuoteInvoiceConversionResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "quotes", quoteID, "convert-to-invoice"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listOrders(ctx context.Context, tenantID string, filter orders.OrderFilter) ([]orders.Order, error) {
	values := url.Values{}
	if filter.Status != "" {
		values.Set("status", string(filter.Status))
	}
	if strings.TrimSpace(filter.ContactID) != "" {
		values.Set("contact_id", strings.TrimSpace(filter.ContactID))
	}
	if filter.FromDate != nil {
		values.Set("from_date", filter.FromDate.Format("2006-01-02"))
	}
	if filter.ToDate != nil {
		values.Set("to_date", filter.ToDate.Format("2006-01-02"))
	}
	if strings.TrimSpace(filter.Search) != "" {
		values.Set("search", strings.TrimSpace(filter.Search))
	}

	var resp []orders.Order
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "orders"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createOrder(ctx context.Context, tenantID string, req *orders.CreateOrderRequest) (*orders.Order, error) {
	var resp orders.Order
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "orders"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importOrders(ctx context.Context, tenantID string, req *orders.ImportOrdersRequest) (*orders.ImportOrdersResult, error) {
	var resp orders.ImportOrdersResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "orders", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getOrder(ctx context.Context, tenantID, orderID string) (*orders.Order, error) {
	var resp orders.Order
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "orders", orderID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) checkOrderStock(ctx context.Context, tenantID, orderID, warehouseID string) (*orders.OrderStockCheck, error) {
	values := url.Values{}
	if strings.TrimSpace(warehouseID) != "" {
		values.Set("warehouse_id", strings.TrimSpace(warehouseID))
	}
	var resp orders.OrderStockCheck
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "orders", orderID, "stock-check"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) reserveOrderStock(ctx context.Context, tenantID, orderID string, req *orders.OrderStockReservationRequest) (*orders.OrderStockReservationResult, error) {
	var resp orders.OrderStockReservationResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "orders", orderID, "reserve-stock"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) releaseOrderStock(ctx context.Context, tenantID, orderID string, req *orders.OrderStockReservationRequest) (*orders.OrderStockReservationResult, error) {
	var resp orders.OrderStockReservationResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "orders", orderID, "release-stock"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listOrderStockReservations(ctx context.Context, tenantID, orderID string) ([]orders.OrderStockReservation, error) {
	var resp []orders.OrderStockReservation
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "orders", orderID, "stock-reservations"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getOrderPickList(ctx context.Context, tenantID, orderID, warehouseID string) (*orders.OrderPickList, error) {
	values := url.Values{}
	if strings.TrimSpace(warehouseID) != "" {
		values.Set("warehouse_id", strings.TrimSpace(warehouseID))
	}
	var resp orders.OrderPickList
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "orders", orderID, "pick-list"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateOrder(ctx context.Context, tenantID, orderID string, req *orders.UpdateOrderRequest) (*orders.Order, error) {
	var resp orders.Order
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "orders", orderID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteOrder(ctx context.Context, tenantID, orderID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "orders", orderID), nil, c.apiToken, nil)
}

func (c *apiClient) updateOrderStatus(ctx context.Context, tenantID, orderID, action string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "orders", orderID, action), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listRecurringInvoices(ctx context.Context, tenantID string, activeOnly bool) ([]recurring.RecurringInvoice, error) {
	values := url.Values{}
	if activeOnly {
		values.Set("active_only", "true")
	}

	var resp []recurring.RecurringInvoice
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "recurring-invoices"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createRecurringInvoice(ctx context.Context, tenantID string, req *recurring.CreateRecurringInvoiceRequest) (*recurring.RecurringInvoice, error) {
	var resp recurring.RecurringInvoice
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "recurring-invoices"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importRecurringInvoices(ctx context.Context, tenantID string, req *recurring.ImportRecurringInvoicesRequest) (*recurring.ImportRecurringInvoicesResult, error) {
	var resp recurring.ImportRecurringInvoicesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) createRecurringInvoiceFromInvoice(ctx context.Context, tenantID, invoiceID string, req *recurring.CreateFromInvoiceRequest) (*recurring.RecurringInvoice, error) {
	var resp recurring.RecurringInvoice
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", "from-invoice", invoiceID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getRecurringInvoice(ctx context.Context, tenantID, recurringID string) (*recurring.RecurringInvoice, error) {
	var resp recurring.RecurringInvoice
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", recurringID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateRecurringInvoice(ctx context.Context, tenantID, recurringID string, req *recurring.UpdateRecurringInvoiceRequest) (*recurring.RecurringInvoice, error) {
	var resp recurring.RecurringInvoice
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", recurringID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteRecurringInvoice(ctx context.Context, tenantID, recurringID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", recurringID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) updateRecurringInvoiceStatus(ctx context.Context, tenantID, recurringID, action string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", recurringID, action), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) generateRecurringInvoice(ctx context.Context, tenantID, recurringID string) (*recurring.GenerationResult, error) {
	var resp recurring.GenerationResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", recurringID, "generate"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) generateDueRecurringInvoices(ctx context.Context, tenantID string) ([]recurring.GenerationResult, error) {
	var resp []recurring.GenerationResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "recurring-invoices", "generate-due"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listAssetCategories(ctx context.Context, tenantID string) ([]assets.AssetCategory, error) {
	var resp []assets.AssetCategory
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "asset-categories"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createAssetCategory(ctx context.Context, tenantID string, req *assets.CreateCategoryRequest) (*assets.AssetCategory, error) {
	var resp assets.AssetCategory
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "asset-categories"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getAssetCategory(ctx context.Context, tenantID, categoryID string) (*assets.AssetCategory, error) {
	var resp assets.AssetCategory
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "asset-categories", categoryID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteAssetCategory(ctx context.Context, tenantID, categoryID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "asset-categories", categoryID), nil, c.apiToken, nil)
}

func (c *apiClient) listAssets(ctx context.Context, tenantID string, filter assets.AssetFilter) ([]assets.FixedAsset, error) {
	values := url.Values{}
	if filter.Status != "" {
		values.Set("status", string(filter.Status))
	}
	if strings.TrimSpace(filter.CategoryID) != "" {
		values.Set("category_id", strings.TrimSpace(filter.CategoryID))
	}
	if strings.TrimSpace(filter.Search) != "" {
		values.Set("search", strings.TrimSpace(filter.Search))
	}

	var resp []assets.FixedAsset
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "assets"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createAsset(ctx context.Context, tenantID string, req *assets.CreateAssetRequest) (*assets.FixedAsset, error) {
	var resp assets.FixedAsset
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "assets"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importAssets(ctx context.Context, tenantID string, req *assets.ImportAssetsRequest) (*assets.ImportAssetsResult, error) {
	var resp assets.ImportAssetsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "assets", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getAsset(ctx context.Context, tenantID, assetID string) (*assets.FixedAsset, error) {
	var resp assets.FixedAsset
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "assets", assetID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateAsset(ctx context.Context, tenantID, assetID string, req *assets.UpdateAssetRequest) (*assets.FixedAsset, error) {
	var resp assets.FixedAsset
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "assets", assetID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteAsset(ctx context.Context, tenantID, assetID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "assets", assetID), nil, c.apiToken, nil)
}

func (c *apiClient) activateAsset(ctx context.Context, tenantID, assetID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "assets", assetID, "activate"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) disposeAsset(ctx context.Context, tenantID, assetID string, req *assets.DisposeAssetRequest) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "assets", assetID, "dispose"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) recordAssetDepreciation(ctx context.Context, tenantID, assetID string) (*assets.DepreciationEntry, error) {
	var resp assets.DepreciationEntry
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "assets", assetID, "depreciation"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listAssetDepreciation(ctx context.Context, tenantID, assetID string) ([]assets.DepreciationEntry, error) {
	var resp []assets.DepreciationEntry
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "assets", assetID, "depreciation"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listProductCategories(ctx context.Context, tenantID string) ([]inventory.ProductCategory, error) {
	var resp []inventory.ProductCategory
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "product-categories"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createProductCategory(ctx context.Context, tenantID string, req *inventory.CreateCategoryRequest) (*inventory.ProductCategory, error) {
	var resp inventory.ProductCategory
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "product-categories"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importProductCategories(ctx context.Context, tenantID string, req *inventory.ImportProductCategoriesRequest) (*inventory.ImportProductCategoriesResult, error) {
	var resp inventory.ImportProductCategoriesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "product-categories", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getProductCategory(ctx context.Context, tenantID, categoryID string) (*inventory.ProductCategory, error) {
	var resp inventory.ProductCategory
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "product-categories", categoryID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteProductCategory(ctx context.Context, tenantID, categoryID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "product-categories", categoryID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listProducts(ctx context.Context, tenantID string, filter inventory.ProductFilter) ([]inventory.Product, error) {
	values := url.Values{}
	if filter.ProductType != "" {
		values.Set("product_type", string(filter.ProductType))
	}
	if filter.Status != "" {
		values.Set("status", string(filter.Status))
	}
	if strings.TrimSpace(filter.CategoryID) != "" {
		values.Set("category_id", strings.TrimSpace(filter.CategoryID))
	}
	if strings.TrimSpace(filter.Search) != "" {
		values.Set("search", strings.TrimSpace(filter.Search))
	}
	if filter.LowStock {
		values.Set("low_stock", "true")
	}

	var resp []inventory.Product
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "products"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createProduct(ctx context.Context, tenantID string, req *inventory.CreateProductRequest) (*inventory.Product, error) {
	var resp inventory.Product
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "products"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importProducts(ctx context.Context, tenantID string, req *inventory.ImportProductsRequest) (*inventory.ImportProductsResult, error) {
	var resp inventory.ImportProductsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "products", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getProduct(ctx context.Context, tenantID, productID string) (*inventory.Product, error) {
	var resp inventory.Product
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "products", productID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateProduct(ctx context.Context, tenantID, productID string, req *inventory.UpdateProductRequest) (*inventory.Product, error) {
	var resp inventory.Product
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "products", productID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteProduct(ctx context.Context, tenantID, productID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "products", productID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listStockLevels(ctx context.Context, tenantID, productID string) ([]inventory.StockLevel, error) {
	var resp []inventory.StockLevel
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "products", productID, "stock-levels"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listInventoryMovements(ctx context.Context, tenantID, productID string) ([]inventory.InventoryMovement, error) {
	var resp []inventory.InventoryMovement
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "products", productID, "movements"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getInventoryValuation(ctx context.Context, tenantID, warehouseID, method string) (*inventory.InventoryValuationReport, error) {
	values := url.Values{}
	if strings.TrimSpace(warehouseID) != "" {
		values.Set("warehouse_id", strings.TrimSpace(warehouseID))
	}
	if strings.TrimSpace(method) != "" {
		values.Set("method", strings.TrimSpace(method))
	}

	var resp inventory.InventoryValuationReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "inventory", "valuation"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listWarehouses(ctx context.Context, tenantID string, activeOnly bool) ([]inventory.Warehouse, error) {
	values := url.Values{}
	if activeOnly {
		values.Set("active_only", "true")
	}

	var resp []inventory.Warehouse
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "warehouses"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createWarehouse(ctx context.Context, tenantID string, req *inventory.CreateWarehouseRequest) (*inventory.Warehouse, error) {
	var resp inventory.Warehouse
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "warehouses"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importWarehouses(ctx context.Context, tenantID string, req *inventory.ImportWarehousesRequest) (*inventory.ImportWarehousesResult, error) {
	var resp inventory.ImportWarehousesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "warehouses", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getWarehouse(ctx context.Context, tenantID, warehouseID string) (*inventory.Warehouse, error) {
	var resp inventory.Warehouse
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "warehouses", warehouseID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateWarehouse(ctx context.Context, tenantID, warehouseID string, req *inventory.UpdateWarehouseRequest) (*inventory.Warehouse, error) {
	var resp inventory.Warehouse
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "warehouses", warehouseID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteWarehouse(ctx context.Context, tenantID, warehouseID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "warehouses", warehouseID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) adjustStock(ctx context.Context, tenantID string, req *inventory.AdjustStockRequest) (*inventory.InventoryMovement, error) {
	var resp inventory.InventoryMovement
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "inventory", "adjust"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importStockAdjustments(ctx context.Context, tenantID string, req *inventory.ImportStockAdjustmentsRequest) (*inventory.ImportStockAdjustmentsResult, error) {
	var resp inventory.ImportStockAdjustmentsResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "inventory", "stock-import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) transferStock(ctx context.Context, tenantID string, req *inventory.TransferStockRequest) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "inventory", "transfer"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) reserveStock(ctx context.Context, tenantID string, req *inventory.StockReservationRequest) (*inventory.StockLevel, error) {
	var resp inventory.StockLevel
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "inventory", "reserve"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) releaseStock(ctx context.Context, tenantID string, req *inventory.StockReservationRequest) (*inventory.StockLevel, error) {
	var resp inventory.StockLevel
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "inventory", "release"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listCostCenters(ctx context.Context, tenantID string, activeOnly bool) ([]accounting.CostCenter, error) {
	values := url.Values{}
	if activeOnly {
		values.Set("active_only", "true")
	}

	var resp []accounting.CostCenter
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "cost-centers"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createCostCenter(ctx context.Context, tenantID string, req *accounting.CreateCostCenterRequest) (*accounting.CostCenter, error) {
	var resp accounting.CostCenter
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "cost-centers"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importCostCenters(ctx context.Context, tenantID string, req *accounting.ImportCostCentersRequest) (*accounting.ImportCostCentersResult, error) {
	var resp accounting.ImportCostCentersResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "cost-centers", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getCostCenter(ctx context.Context, tenantID, costCenterID string) (*accounting.CostCenter, error) {
	var resp accounting.CostCenter
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "cost-centers", costCenterID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateCostCenter(ctx context.Context, tenantID, costCenterID string, req *accounting.UpdateCostCenterRequest) (*accounting.CostCenter, error) {
	var resp accounting.CostCenter
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "cost-centers", costCenterID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteCostCenter(ctx context.Context, tenantID, costCenterID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "cost-centers", costCenterID), nil, c.apiToken, nil)
}

func (c *apiClient) getCostCenterReport(ctx context.Context, tenantID string, startDate, endDate *time.Time) (*accounting.CostCenterReport, error) {
	values := url.Values{}
	if startDate != nil {
		values.Set("start_date", startDate.Format("2006-01-02"))
	}
	if endDate != nil {
		values.Set("end_date", endDate.Format("2006-01-02"))
	}

	var resp accounting.CostCenterReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "cost-centers", "report"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportCostCenterReport(ctx context.Context, tenantID string, startDate, endDate *time.Time, format string) ([]byte, error) {
	values := url.Values{}
	if startDate != nil {
		values.Set("start_date", startDate.Format("2006-01-02"))
	}
	if endDate != nil {
		values.Set("end_date", endDate.Format("2006-01-02"))
	}
	values.Set("format", strings.TrimSpace(format))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "cost-centers", "report"), values), nil, c.apiToken)
}

func (c *apiClient) getBudgetVsActualReport(ctx context.Context, tenantID string, startDate, endDate *time.Time) (*accounting.CostCenterReport, error) {
	values := url.Values{}
	if startDate != nil {
		values.Set("start_date", startDate.Format("2006-01-02"))
	}
	if endDate != nil {
		values.Set("end_date", endDate.Format("2006-01-02"))
	}

	var resp accounting.CostCenterReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "budget-vs-actual"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportBudgetVsActualReport(ctx context.Context, tenantID string, startDate, endDate *time.Time, format string) ([]byte, error) {
	values := url.Values{}
	if startDate != nil {
		values.Set("start_date", startDate.Format("2006-01-02"))
	}
	if endDate != nil {
		values.Set("end_date", endDate.Format("2006-01-02"))
	}
	values.Set("format", strings.TrimSpace(format))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "budget-vs-actual"), values), nil, c.apiToken)
}

func (c *apiClient) importOpeningBalances(ctx context.Context, tenantID string, req *accounting.ImportOpeningBalancesRequest) (*accounting.ImportOpeningBalancesResult, error) {
	var resp accounting.ImportOpeningBalancesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entries", "import-opening-balances"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importJournalEntries(ctx context.Context, tenantID string, req *accounting.ImportJournalEntriesRequest) (*accounting.ImportJournalEntriesResult, error) {
	var resp accounting.ImportJournalEntriesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entries", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listJournalEntries(ctx context.Context, tenantID string, limit int) ([]accounting.JournalEntry, error) {
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}

	var resp []accounting.JournalEntry
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "journal-entries"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getJournalEntry(ctx context.Context, tenantID, entryID string) (*accounting.JournalEntry, error) {
	var resp accounting.JournalEntry
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "journal-entries", entryID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) createJournalEntry(ctx context.Context, tenantID string, req *accounting.CreateJournalEntryRequest) (*accounting.JournalEntry, error) {
	var resp accounting.JournalEntry
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entries"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listJournalEntryTemplates(ctx context.Context, tenantID string, activeOnly bool) ([]accounting.JournalEntryTemplate, error) {
	values := url.Values{}
	if activeOnly {
		values.Set("active_only", "true")
	}

	var resp []accounting.JournalEntryTemplate
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "journal-entry-templates"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createJournalEntryTemplate(ctx context.Context, tenantID string, req *accounting.CreateJournalEntryTemplateRequest) (*accounting.JournalEntryTemplate, error) {
	var resp accounting.JournalEntryTemplate
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entry-templates"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getJournalEntryTemplate(ctx context.Context, tenantID, templateID string) (*accounting.JournalEntryTemplate, error) {
	var resp accounting.JournalEntryTemplate
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "journal-entry-templates", templateID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) generateJournalEntryTemplate(ctx context.Context, tenantID, templateID string, req *accounting.GenerateJournalEntryTemplateRequest) (*accounting.JournalEntryTemplateGenerationResult, error) {
	var resp accounting.JournalEntryTemplateGenerationResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entry-templates", templateID, "generate"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) generateDueJournalEntryTemplates(ctx context.Context, tenantID string, req *accounting.GenerateDueJournalEntryTemplatesRequest) ([]accounting.JournalEntryTemplateGenerationResult, error) {
	var resp []accounting.JournalEntryTemplateGenerationResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entry-templates", "generate-due"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) applyJournalEntryTemplate(ctx context.Context, tenantID, templateID string, req *accounting.ApplyJournalEntryTemplateRequest) (*accounting.JournalEntry, error) {
	var resp accounting.JournalEntry
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entry-templates", templateID, "apply"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) postJournalEntry(ctx context.Context, tenantID, entryID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entries", entryID, "post"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) voidJournalEntry(ctx context.Context, tenantID, entryID, reason string) (*accounting.JournalEntry, error) {
	var resp accounting.JournalEntry
	body := map[string]string{"reason": strings.TrimSpace(reason)}
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "journal-entries", entryID, "void"), body, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listEmployees(ctx context.Context, tenantID string, activeOnly bool) ([]payroll.Employee, error) {
	urlPath := path.Join("/api/v1/tenants", tenantID, "employees")
	if activeOnly {
		urlPath += "?active_only=true"
	}

	var resp []payroll.Employee
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createEmployee(ctx context.Context, tenantID string, req *payroll.CreateEmployeeRequest) (*payroll.Employee, error) {
	var resp payroll.Employee
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "employees"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getEmployee(ctx context.Context, tenantID, employeeID string) (*payroll.Employee, error) {
	var resp payroll.Employee
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "employees", employeeID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateEmployee(ctx context.Context, tenantID, employeeID string, req *payroll.UpdateEmployeeRequest) (*payroll.Employee, error) {
	var resp payroll.Employee
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "employees", employeeID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) setBaseSalary(ctx context.Context, tenantID, employeeID string, amount decimal.Decimal, effectiveFrom time.Time) (map[string]string, error) {
	var resp map[string]string
	body := map[string]any{
		"amount":         amount,
		"effective_from": effectiveFrom,
	}
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "employees", employeeID, "salary"), body, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listSalaryComponents(ctx context.Context, tenantID, employeeID, activeOn string) ([]payroll.SalaryComponent, error) {
	values := url.Values{}
	if strings.TrimSpace(activeOn) != "" {
		values.Set("active_on", strings.TrimSpace(activeOn))
	}

	var resp []payroll.SalaryComponent
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "employees", employeeID, "salary-components"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) addSalaryComponent(ctx context.Context, tenantID, employeeID string, req *payroll.CreateSalaryComponentRequest) (*payroll.SalaryComponent, error) {
	var resp payroll.SalaryComponent
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "employees", employeeID, "salary-components"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importEmployees(ctx context.Context, tenantID string, req *payroll.ImportEmployeesRequest) (*payroll.ImportEmployeesResult, error) {
	var resp payroll.ImportEmployeesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "employees", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importPayrollHistory(ctx context.Context, tenantID string, req *payroll.ImportPayrollHistoryRequest) (*payroll.ImportPayrollHistoryResult, error) {
	var resp payroll.ImportPayrollHistoryResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", "import-history"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importLeaveBalances(ctx context.Context, tenantID string, req *payroll.ImportLeaveBalancesRequest) (*payroll.ImportLeaveBalancesResult, error) {
	var resp payroll.ImportLeaveBalancesResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "leave-balances", "import"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listAbsenceTypes(ctx context.Context, tenantID string, activeOnly bool) ([]payroll.AbsenceType, error) {
	values := url.Values{}
	if activeOnly {
		values.Set("active_only", "true")
	}

	var resp []payroll.AbsenceType
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "absence-types"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getAbsenceType(ctx context.Context, tenantID, typeID string) (*payroll.AbsenceType, error) {
	var resp payroll.AbsenceType
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "absence-types", typeID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listLeaveBalances(ctx context.Context, tenantID, employeeID string, year int) ([]payroll.LeaveBalance, error) {
	values := url.Values{}
	if year > 0 {
		values.Set("year", strconv.Itoa(year))
	}

	var resp []payroll.LeaveBalance
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "employees", employeeID, "leave-balances"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getLeaveBalancesByYear(ctx context.Context, tenantID, employeeID string, year int) ([]payroll.LeaveBalance, error) {
	var resp []payroll.LeaveBalance
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "employees", employeeID, "leave-balances", strconv.Itoa(year)), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) updateLeaveBalance(ctx context.Context, tenantID, employeeID string, year int, absenceTypeID string, req *payroll.UpdateLeaveBalanceRequest) (*payroll.LeaveBalance, error) {
	var resp payroll.LeaveBalance
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "employees", employeeID, "leave-balances", strconv.Itoa(year), absenceTypeID), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) initializeLeaveBalances(ctx context.Context, tenantID, employeeID string, year int) ([]payroll.LeaveBalance, error) {
	var resp []payroll.LeaveBalance
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "employees", employeeID, "leave-balances", strconv.Itoa(year), "initialize"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listLeaveRecords(ctx context.Context, tenantID, employeeID string, year int) ([]payroll.LeaveRecord, error) {
	values := url.Values{}
	if strings.TrimSpace(employeeID) != "" {
		values.Set("employee_id", strings.TrimSpace(employeeID))
	}
	if year > 0 {
		values.Set("year", strconv.Itoa(year))
	}

	var resp []payroll.LeaveRecord
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "leave-records"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createLeaveRecord(ctx context.Context, tenantID string, req *payroll.CreateLeaveRecordRequest) (*payroll.LeaveRecord, error) {
	var resp payroll.LeaveRecord
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "leave-records"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getLeaveRecord(ctx context.Context, tenantID, recordID string) (*payroll.LeaveRecord, error) {
	var resp payroll.LeaveRecord
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "leave-records", recordID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) approveLeaveRecord(ctx context.Context, tenantID, recordID string) (*payroll.LeaveRecord, error) {
	var resp payroll.LeaveRecord
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "leave-records", recordID, "approve"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) rejectLeaveRecord(ctx context.Context, tenantID, recordID string, req *payroll.RejectLeaveRequest) (*payroll.LeaveRecord, error) {
	var resp payroll.LeaveRecord
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "leave-records", recordID, "reject"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) cancelLeaveRecord(ctx context.Context, tenantID, recordID string) (*payroll.LeaveRecord, error) {
	var resp payroll.LeaveRecord
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "leave-records", recordID, "cancel"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listPayrollRuns(ctx context.Context, tenantID string, year int) ([]payroll.PayrollRun, error) {
	values := url.Values{}
	if year > 0 {
		values.Set("year", strconv.Itoa(year))
	}

	var resp []payroll.PayrollRun
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "payroll-runs"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) createPayrollRun(ctx context.Context, tenantID string, req *payroll.CreatePayrollRunRequest) (*payroll.PayrollRun, error) {
	var resp payroll.PayrollRun
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getPayrollRun(ctx context.Context, tenantID, runID string) (*payroll.PayrollRun, error) {
	var resp payroll.PayrollRun
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) calculatePayrollRun(ctx context.Context, tenantID, runID string) (*payroll.PayrollRun, error) {
	var resp payroll.PayrollRun
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "calculate"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) processPayrollRun(ctx context.Context, tenantID, runID string, req *payroll.ProcessPayrollRunRequest) (*payroll.PayrollRunProcessResult, error) {
	var resp payroll.PayrollRunProcessResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "process"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) approvePayrollRun(ctx context.Context, tenantID, runID string) (map[string]string, error) {
	var resp map[string]string
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "approve"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listPayslips(ctx context.Context, tenantID, runID string) ([]payroll.Payslip, error) {
	var resp []payroll.Payslip
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "payslips"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) downloadPayslipPDF(ctx context.Context, tenantID, runID, payslipID string) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "payslips", payslipID, "pdf"), nil, c.apiToken)
}

func (c *apiClient) calculateTaxPreview(ctx context.Context, tenantID string, grossSalary decimal.Decimal, applyBasicExemption bool, fundedPensionRate decimal.Decimal) (*payroll.TaxCalculation, error) {
	var resp payroll.TaxCalculation
	body := map[string]any{
		"gross_salary":          grossSalary,
		"apply_basic_exemption": applyBasicExemption,
		"funded_pension_rate":   fundedPensionRate,
	}
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll", "tax-preview"), body, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) listTSD(ctx context.Context, tenantID string) ([]payroll.TSDDeclaration, error) {
	var resp []payroll.TSDDeclaration
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getTSD(ctx context.Context, tenantID string, year, month int) (*payroll.TSDDeclaration, error) {
	var resp payroll.TSDDeclaration
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month)), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) generateTSD(ctx context.Context, tenantID, runID string) (*payroll.TSDDeclaration, error) {
	var resp payroll.TSDDeclaration
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "payroll-runs", runID, "tsd"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportTSDXML(ctx context.Context, tenantID string, year, month int) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month), "xml"), nil, c.apiToken)
}

func (c *apiClient) exportTSDCSV(ctx context.Context, tenantID string, year, month int) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month), "csv"), nil, c.apiToken)
}

func (c *apiClient) markTSDSubmitted(ctx context.Context, tenantID string, year, month int, emtaReference string) (map[string]string, error) {
	var resp map[string]string
	body := map[string]string{"emta_reference": emtaReference}
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "tsd", strconv.Itoa(year), strconv.Itoa(month), "submit"), body, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listKMD(ctx context.Context, tenantID string) ([]tax.KMDDeclaration, error) {
	var resp []tax.KMDDeclaration
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tax", "kmd"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) generateKMD(ctx context.Context, tenantID string, req *tax.CreateKMDRequest) (*tax.KMDDeclaration, error) {
	var resp tax.KMDDeclaration
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "tax", "kmd"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) importKMDHistory(ctx context.Context, tenantID string, req *tax.ImportKMDHistoryRequest) (*tax.ImportKMDHistoryResult, error) {
	var resp tax.ImportKMDHistoryResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "tax", "kmd", "import-history"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) generateKMDINF(ctx context.Context, tenantID string, year, month int, threshold decimal.Decimal) (*tax.KMDINFReport, error) {
	values := url.Values{}
	if !threshold.IsZero() {
		values.Set("threshold", threshold.String())
	}

	var resp tax.KMDINFReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "tax", "kmd", strconv.Itoa(year), strconv.Itoa(month), "inf"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) generateEUVATOSS(ctx context.Context, tenantID string, year, quarter int, includeB2B bool) (*tax.EUVATOSSReport, error) {
	values := url.Values{}
	values.Set("year", strconv.Itoa(year))
	values.Set("quarter", strconv.Itoa(quarter))
	if includeB2B {
		values.Set("include_b2b", "true")
	}

	var resp tax.EUVATOSSReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "tax", "eu-vat", "oss"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportKMDXML(ctx context.Context, tenantID string, year, month int) ([]byte, error) {
	return c.requestRaw(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "tax", "kmd", strconv.Itoa(year), strconv.Itoa(month), "xml"), nil, c.apiToken)
}

func (c *apiClient) getTrialBalance(ctx context.Context, tenantID, asOfDate string) (*accounting.TrialBalance, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}

	var resp accounting.TrialBalance
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "trial-balance"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportTrialBalanceCSV(ctx context.Context, tenantID, asOfDate string) ([]byte, error) {
	values := url.Values{"format": []string{"csv"}}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "trial-balance"), values), nil, c.apiToken)
}

func (c *apiClient) exportTrialBalanceXLSX(ctx context.Context, tenantID, asOfDate string) ([]byte, error) {
	values := url.Values{"format": []string{"xlsx"}}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "trial-balance"), values), nil, c.apiToken)
}

func (c *apiClient) exportTrialBalancePDF(ctx context.Context, tenantID, asOfDate string) ([]byte, error) {
	values := url.Values{"format": []string{"pdf"}}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "trial-balance"), values), nil, c.apiToken)
}

func (c *apiClient) getAccountBalanceReport(ctx context.Context, tenantID, accountID, asOfDate string) (*accountBalanceReport, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}

	var resp accountBalanceReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "account-balance", accountID), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportAccountBalanceReport(ctx context.Context, tenantID, accountID, asOfDate, format string) ([]byte, error) {
	values := url.Values{"format": []string{format}}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of_date", strings.TrimSpace(asOfDate))
	}
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "account-balance", accountID), values), nil, c.apiToken)
}

func (c *apiClient) getBalanceSheet(ctx context.Context, tenantID, asOfDate string) (*accounting.BalanceSheet, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of", strings.TrimSpace(asOfDate))
	}

	var resp accounting.BalanceSheet
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-sheet"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportBalanceSheetCSV(ctx context.Context, tenantID, asOfDate string) ([]byte, error) {
	values := url.Values{"format": []string{"csv"}}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of", strings.TrimSpace(asOfDate))
	}
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-sheet"), values), nil, c.apiToken)
}

func (c *apiClient) exportBalanceSheetXLSX(ctx context.Context, tenantID, asOfDate string) ([]byte, error) {
	values := url.Values{"format": []string{"xlsx"}}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of", strings.TrimSpace(asOfDate))
	}
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-sheet"), values), nil, c.apiToken)
}

func (c *apiClient) exportBalanceSheetPDF(ctx context.Context, tenantID, asOfDate string) ([]byte, error) {
	values := url.Values{"format": []string{"pdf"}}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of", strings.TrimSpace(asOfDate))
	}
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-sheet"), values), nil, c.apiToken)
}

func (c *apiClient) getIncomeStatement(ctx context.Context, tenantID, startDate, endDate string) (*accounting.IncomeStatement, error) {
	values := url.Values{}
	values.Set("start", strings.TrimSpace(startDate))
	values.Set("end", strings.TrimSpace(endDate))

	var resp accounting.IncomeStatement
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "income-statement"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getConsolidatedReport(ctx context.Context, tenantID, asOfDate, startDate, endDate, tenantIDs string) (*reports.ConsolidatedFinancialReport, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of", strings.TrimSpace(asOfDate))
	}
	if strings.TrimSpace(startDate) != "" {
		values.Set("start", strings.TrimSpace(startDate))
	}
	if strings.TrimSpace(endDate) != "" {
		values.Set("end", strings.TrimSpace(endDate))
	}
	if strings.TrimSpace(tenantIDs) != "" {
		values.Set("tenant_ids", strings.TrimSpace(tenantIDs))
	}

	var resp reports.ConsolidatedFinancialReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "consolidated"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getAnnualReport(ctx context.Context, tenantID, periodEndDate, cashFlowMethod string) (*reports.AnnualReport, error) {
	values := url.Values{}
	values.Set("period_end_date", strings.TrimSpace(periodEndDate))
	if strings.TrimSpace(cashFlowMethod) != "" {
		values.Set("cash_flow_method", strings.TrimSpace(cashFlowMethod))
	}

	var resp reports.AnnualReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "annual"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportIncomeStatementCSV(ctx context.Context, tenantID, startDate, endDate string) ([]byte, error) {
	values := url.Values{"format": []string{"csv"}}
	values.Set("start", strings.TrimSpace(startDate))
	values.Set("end", strings.TrimSpace(endDate))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "income-statement"), values), nil, c.apiToken)
}

func (c *apiClient) exportIncomeStatementXLSX(ctx context.Context, tenantID, startDate, endDate string) ([]byte, error) {
	values := url.Values{"format": []string{"xlsx"}}
	values.Set("start", strings.TrimSpace(startDate))
	values.Set("end", strings.TrimSpace(endDate))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "income-statement"), values), nil, c.apiToken)
}

func (c *apiClient) exportIncomeStatementPDF(ctx context.Context, tenantID, startDate, endDate string) ([]byte, error) {
	values := url.Values{"format": []string{"pdf"}}
	values.Set("start", strings.TrimSpace(startDate))
	values.Set("end", strings.TrimSpace(endDate))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "income-statement"), values), nil, c.apiToken)
}

func (c *apiClient) getCashFlowStatement(ctx context.Context, tenantID, startDate, endDate, method string, overrides reports.CashFlowMappingOverrides) (*reports.CashFlowStatement, error) {
	values := cashFlowStatementQuery(startDate, endDate, method, overrides)

	var resp reports.CashFlowStatement
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "cash-flow"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getCashFlowMapping(ctx context.Context, tenantID string) (*reports.CashFlowMappingOverrides, error) {
	var resp reports.CashFlowMappingOverrides
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "reports", "cash-flow", "mapping"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) updateCashFlowMapping(ctx context.Context, tenantID string, mapping reports.CashFlowMappingOverrides) (*reports.CashFlowMappingOverrides, error) {
	var resp reports.CashFlowMappingOverrides
	if err := c.request(ctx, http.MethodPut, path.Join("/api/v1/tenants", tenantID, "reports", "cash-flow", "mapping"), mapping, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportCashFlowStatementCSV(ctx context.Context, tenantID, startDate, endDate, method string, overrides reports.CashFlowMappingOverrides) ([]byte, error) {
	values := cashFlowStatementQuery(startDate, endDate, method, overrides)
	values.Set("format", "csv")
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "cash-flow"), values), nil, c.apiToken)
}

func (c *apiClient) exportCashFlowStatementXLSX(ctx context.Context, tenantID, startDate, endDate, method string, overrides reports.CashFlowMappingOverrides) ([]byte, error) {
	values := cashFlowStatementQuery(startDate, endDate, method, overrides)
	values.Set("format", "xlsx")
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "cash-flow"), values), nil, c.apiToken)
}

func (c *apiClient) exportCashFlowStatementPDF(ctx context.Context, tenantID, startDate, endDate, method string, overrides reports.CashFlowMappingOverrides) ([]byte, error) {
	values := cashFlowStatementQuery(startDate, endDate, method, overrides)
	values.Set("format", "pdf")
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "cash-flow"), values), nil, c.apiToken)
}

func cashFlowStatementQuery(startDate, endDate, method string, overrides reports.CashFlowMappingOverrides) url.Values {
	values := url.Values{}
	values.Set("start_date", strings.TrimSpace(startDate))
	values.Set("end_date", strings.TrimSpace(endDate))
	if strings.TrimSpace(method) != "" {
		values.Set("method", strings.TrimSpace(method))
	}
	if len(overrides.OperatingAccountCodes) > 0 {
		values.Set("operating_accounts", strings.Join(overrides.OperatingAccountCodes, ","))
	}
	if len(overrides.InvestingAccountCodes) > 0 {
		values.Set("investing_accounts", strings.Join(overrides.InvestingAccountCodes, ","))
	}
	if len(overrides.FinancingAccountCodes) > 0 {
		values.Set("financing_accounts", strings.Join(overrides.FinancingAccountCodes, ","))
	}
	return values
}

func (c *apiClient) getBalanceConfirmationSummary(ctx context.Context, tenantID, balanceType, asOfDate string) (*reports.BalanceConfirmationSummary, error) {
	values := url.Values{}
	values.Set("type", strings.TrimSpace(balanceType))
	values.Set("as_of_date", strings.TrimSpace(asOfDate))

	var resp reports.BalanceConfirmationSummary
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-confirmations"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportBalanceConfirmationSummary(ctx context.Context, tenantID, balanceType, asOfDate, format string) ([]byte, error) {
	values := url.Values{}
	values.Set("type", strings.TrimSpace(balanceType))
	values.Set("as_of_date", strings.TrimSpace(asOfDate))
	values.Set("format", strings.TrimSpace(format))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-confirmations"), values), nil, c.apiToken)
}

func (c *apiClient) getBalanceConfirmation(ctx context.Context, tenantID, contactID, balanceType, asOfDate string) (*reports.BalanceConfirmation, error) {
	values := url.Values{}
	values.Set("type", strings.TrimSpace(balanceType))
	values.Set("as_of_date", strings.TrimSpace(asOfDate))

	var resp reports.BalanceConfirmation
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-confirmations", contactID), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportBalanceConfirmation(ctx context.Context, tenantID, contactID, balanceType, asOfDate, format string) ([]byte, error) {
	values := url.Values{}
	values.Set("type", strings.TrimSpace(balanceType))
	values.Set("as_of_date", strings.TrimSpace(asOfDate))
	values.Set("format", strings.TrimSpace(format))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "balance-confirmations", contactID), values), nil, c.apiToken)
}

func (c *apiClient) getContactStatement(ctx context.Context, tenantID, contactID, balanceType, startDate, endDate string) (*reports.ContactStatement, error) {
	values := contactStatementQuery(balanceType, startDate, endDate)

	var resp reports.ContactStatement
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "contact-statements", contactID), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportContactStatement(ctx context.Context, tenantID, contactID, balanceType, startDate, endDate, format string) ([]byte, error) {
	values := contactStatementQuery(balanceType, startDate, endDate)
	values.Set("format", strings.TrimSpace(format))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "contact-statements", contactID), values), nil, c.apiToken)
}

func contactStatementQuery(balanceType, startDate, endDate string) url.Values {
	values := url.Values{}
	values.Set("type", strings.TrimSpace(balanceType))
	values.Set("start_date", strings.TrimSpace(startDate))
	values.Set("end_date", strings.TrimSpace(endDate))
	return values
}

func (c *apiClient) getSalesMarginReport(ctx context.Context, tenantID, startDate, endDate string) (*reports.SalesMarginReport, error) {
	return c.getSalesMarginLikeReport(ctx, tenantID, "sales-margin", startDate, endDate)
}

func (c *apiClient) exportSalesMarginReport(ctx context.Context, tenantID, startDate, endDate, format string) ([]byte, error) {
	return c.exportSalesMarginLikeReport(ctx, tenantID, "sales-margin", startDate, endDate, format)
}

func (c *apiClient) getCustomerProfitabilityReport(ctx context.Context, tenantID, startDate, endDate string) (*reports.SalesMarginReport, error) {
	return c.getSalesMarginLikeReport(ctx, tenantID, "customer-profitability", startDate, endDate)
}

func (c *apiClient) exportCustomerProfitabilityReport(ctx context.Context, tenantID, startDate, endDate, format string) ([]byte, error) {
	return c.exportSalesMarginLikeReport(ctx, tenantID, "customer-profitability", startDate, endDate, format)
}

func (c *apiClient) getSalesMarginLikeReport(ctx context.Context, tenantID, reportPath, startDate, endDate string) (*reports.SalesMarginReport, error) {
	values := salesMarginQuery(startDate, endDate)

	var resp reports.SalesMarginReport
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", reportPath), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportSalesMarginLikeReport(ctx context.Context, tenantID, reportPath, startDate, endDate, format string) ([]byte, error) {
	values := salesMarginQuery(startDate, endDate)
	values.Set("format", strings.TrimSpace(format))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", reportPath), values), nil, c.apiToken)
}

func salesMarginQuery(startDate, endDate string) url.Values {
	values := url.Values{}
	values.Set("start_date", strings.TrimSpace(startDate))
	values.Set("end_date", strings.TrimSpace(endDate))
	return values
}

func (c *apiClient) getAgingReport(ctx context.Context, tenantID, reportType string) (*analytics.AgingReport, error) {
	var resp analytics.AgingReport
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "reports", "aging", reportType), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) exportAgingReport(ctx context.Context, tenantID, reportType, format string) ([]byte, error) {
	values := url.Values{}
	values.Set("format", strings.TrimSpace(format))
	return c.requestRaw(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "reports", "aging", reportType), values), nil, c.apiToken)
}

func (c *apiClient) getDashboardSummary(ctx context.Context, tenantID string) (*analytics.DashboardSummary, error) {
	var resp analytics.DashboardSummary
	if err := c.request(ctx, http.MethodGet, path.Join("/api/v1/tenants", tenantID, "analytics", "dashboard"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getRevenueExpenseChart(ctx context.Context, tenantID string, months int) (*analytics.RevenueExpenseChart, error) {
	values := url.Values{}
	if months > 0 {
		values.Set("months", strconv.Itoa(months))
	}

	var resp analytics.RevenueExpenseChart
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "analytics", "revenue-expense"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getCashFlowChart(ctx context.Context, tenantID string, months int) (*analytics.CashFlowChart, error) {
	values := url.Values{}
	if months > 0 {
		values.Set("months", strconv.Itoa(months))
	}

	var resp analytics.CashFlowChart
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "analytics", "cash-flow"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) getRecentActivity(ctx context.Context, tenantID string, limit int) ([]analytics.ActivityItem, error) {
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}

	var resp []analytics.ActivityItem
	if err := c.request(ctx, http.MethodGet, withQuery(path.Join("/api/v1/tenants", tenantID, "analytics", "activity"), values), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listDocuments(ctx context.Context, tenantID, entityType, entityID string) ([]documents.Document, error) {
	values := url.Values{}
	values.Set("entity_type", entityType)
	values.Set("entity_id", entityID)

	var resp []documents.Document
	urlPath := path.Join("/api/v1/tenants", tenantID, "documents") + "?" + values.Encode()
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) listDocumentReviewSummaries(ctx context.Context, tenantID string, req *documentReviewSummaryRequest) ([]documents.ReviewSummary, error) {
	var resp []documents.ReviewSummary
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "documents", "review-summary"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getDocumentReviewQueue(ctx context.Context, tenantID string, filter *documents.ReviewQueueFilter) (*documents.ReviewQueue, error) {
	values := url.Values{}
	if filter != nil {
		if strings.TrimSpace(filter.EntityType) != "" {
			values.Set("entity_type", strings.TrimSpace(filter.EntityType))
		}
		if strings.TrimSpace(filter.DocumentType) != "" {
			values.Set("document_type", strings.TrimSpace(filter.DocumentType))
		}
		if strings.TrimSpace(filter.ReviewStatus) != "" {
			values.Set("review_status", strings.TrimSpace(filter.ReviewStatus))
		}
		if filter.Limit > 0 {
			values.Set("limit", strconv.Itoa(filter.Limit))
		}
	}

	var resp documents.ReviewQueue
	urlPath := withQuery(path.Join("/api/v1/tenants", tenantID, "documents", "review-queue"), values)
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) evaluateDocumentEvidencePolicy(ctx context.Context, tenantID string, req *documents.EvidencePolicyRequest) ([]documents.EvidencePolicyResult, error) {
	var resp []documents.EvidencePolicyResult
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "documents", "evidence-policy"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) getDocumentRetentionReview(ctx context.Context, tenantID, asOfDate string, horizonDays int, includeMissing bool) (*documents.RetentionReview, error) {
	values := url.Values{}
	if strings.TrimSpace(asOfDate) != "" {
		values.Set("as_of", strings.TrimSpace(asOfDate))
	}
	if horizonDays >= 0 {
		values.Set("horizon_days", strconv.Itoa(horizonDays))
	}
	if includeMissing {
		values.Set("include_missing", "true")
	}

	var resp documents.RetentionReview
	urlPath := withQuery(path.Join("/api/v1/tenants", tenantID, "documents", "retention"), values)
	if err := c.request(ctx, http.MethodGet, urlPath, nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) uploadDocument(ctx context.Context, tenantID string, req *documents.UploadDocumentRequest, fileContent []byte) (*documents.Document, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("entity_type", strings.TrimSpace(req.EntityType)); err != nil {
		return nil, fmt.Errorf("write entity_type: %w", err)
	}
	if err := writer.WriteField("entity_id", strings.TrimSpace(req.EntityID)); err != nil {
		return nil, fmt.Errorf("write entity_id: %w", err)
	}
	if strings.TrimSpace(req.DocumentType) != "" {
		if err := writer.WriteField("document_type", strings.TrimSpace(req.DocumentType)); err != nil {
			return nil, fmt.Errorf("write document_type: %w", err)
		}
	}
	if strings.TrimSpace(req.Notes) != "" {
		if err := writer.WriteField("notes", strings.TrimSpace(req.Notes)); err != nil {
			return nil, fmt.Errorf("write notes: %w", err)
		}
	}
	if req.RetentionUntil != nil {
		if err := writer.WriteField("retention_until", req.RetentionUntil.Format("2006-01-02")); err != nil {
			return nil, fmt.Errorf("write retention_until: %w", err)
		}
	}

	part, err := writer.CreateFormFile("file", strings.TrimSpace(req.FileName))
	if err != nil {
		return nil, fmt.Errorf("create multipart file: %w", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		return nil, fmt.Errorf("write multipart file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	fullURL := c.baseURL + path.Join("/api/v1/tenants", tenantID, "documents")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if strings.TrimSpace(c.apiToken) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.apiToken))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request POST /api/v1/tenants/%s/documents: %w", tenantID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp)
	}

	var doc documents.Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &doc, nil
}

func (c *apiClient) downloadDocument(ctx context.Context, tenantID, documentID string) (*downloadedDocument, error) {
	fullURL := c.baseURL + path.Join("/api/v1/tenants", tenantID, "documents", documentID, "download")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if strings.TrimSpace(c.apiToken) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.apiToken))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request GET /api/v1/tenants/%s/documents/%s/download: %w", tenantID, documentID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read download response: %w", err)
	}
	fileName := documentID
	if disposition := strings.TrimSpace(resp.Header.Get("Content-Disposition")); disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil && strings.TrimSpace(params["filename"]) != "" {
			fileName = strings.TrimSpace(params["filename"])
		}
	}
	return &downloadedDocument{
		FileName:    fileName,
		ContentType: resp.Header.Get("Content-Type"),
		Content:     content,
	}, nil
}

func (c *apiClient) updateDocumentRetention(ctx context.Context, tenantID, documentID, retentionUntil string, clearRetention bool) (*documents.Document, error) {
	req := documentRetentionUpdateRequest{
		RetentionUntil: strings.TrimSpace(retentionUntil),
		ClearRetention: clearRetention,
	}

	var resp documents.Document
	if err := c.request(ctx, http.MethodPatch, path.Join("/api/v1/tenants", tenantID, "documents", documentID, "retention"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) markDocumentReviewed(ctx context.Context, tenantID, documentID string) (*documents.Document, error) {
	var resp documents.Document
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "documents", documentID, "mark-reviewed"), nil, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) reviewDocument(ctx context.Context, tenantID, documentID string, req *documents.ReviewDocumentRequest) (*documents.Document, error) {
	var resp documents.Document
	if err := c.request(ctx, http.MethodPost, path.Join("/api/v1/tenants", tenantID, "documents", documentID, "review"), req, c.apiToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *apiClient) deleteDocument(ctx context.Context, tenantID, documentID string) error {
	return c.request(ctx, http.MethodDelete, path.Join("/api/v1/tenants", tenantID, "documents", documentID), nil, c.apiToken, nil)
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

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
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

func (c *apiClient) requestRaw(ctx context.Context, method, apiPath string, body any, bearerToken string) ([]byte, error) {
	fullURL := c.baseURL + apiPath

	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(bearerToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bearerToken))
	}

	//nolint:gosec // The CLI intentionally talks to a user-configured Open Accounting base URL.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, apiPath, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, decodeAPIError(resp)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return content, nil
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

func withQuery(apiPath string, values url.Values) string {
	if encoded := values.Encode(); encoded != "" {
		return apiPath + "?" + encoded
	}
	return apiPath
}
