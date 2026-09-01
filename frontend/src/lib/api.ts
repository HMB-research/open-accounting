import { browser } from "$app/environment";
import { env } from "$env/dynamic/public";
import Decimal from "decimal.js";
import { authStore } from "./stores/auth";
import {
  createApiTransport,
  DEFAULT_RETRY_CONFIG,
  getApiResponseError,
} from "./api-request";
import type { ApiTransport, RetryConfig } from "./api-request";

export {
  calculateBackoffDelay,
  DEFAULT_RETRY_CONFIG,
  isRetryableError,
  TEST_RETRY_CONFIG,
} from "./api-request";
export type { RetryConfig } from "./api-request";

/**
 * Get the API base URL.
 *
 * IMPORTANT: This must be a function (lazy evaluation) instead of a constant.
 *
 * Root Cause:
 * -----------
 * $env/dynamic/public reads environment variables at runtime on the server,
 * then injects them into the client during SSR hydration. If we read the value
 * at module initialization time (const API_BASE = env.PUBLIC_API_URL), the client
 * may not have the values yet (before hydration completes), resulting in undefined
 * and falling back to localhost:8080.
 *
 * Solution:
 * ---------
 * Use a function that reads the env value when actually needed (at request time),
 * not at module initialization time. This ensures the value is read after hydration.
 *
 * @returns The API base URL from PUBLIC_API_URL env var, or localhost:8080 as fallback
 */
export function getApiBase(): string {
  let url = env.PUBLIC_API_URL || "http://localhost:8080";

  // Ensure URL has a protocol - if missing, add https://
  // This prevents URLs like "example.com/api" being treated as relative paths
  if (url && !url.startsWith("http://") && !url.startsWith("https://")) {
    url = `https://${url}`;
  }

  return url;
}

/**
 * Build a query string from a filter object.
 * Handles undefined/null values by skipping them, and converts
 * boolean values to 'true' string.
 *
 * @param filter - Object with filter parameters
 * @returns Query string with leading '?' or empty string if no params
 */
export function buildQuery(filter?: object): string {
  if (!filter) return "";

  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(filter)) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    if (typeof value === "boolean") {
      params.set(key, "true");
    } else {
      params.set(key, String(value));
    }
  }

  const queryString = params.toString();
  return queryString ? `?${queryString}` : "";
}

function getCurrentTenantContext(): string | null {
  if (!browser || typeof window === "undefined") {
    return null;
  }

  try {
    return (
      new URL(window.location.href).searchParams.get("tenant")?.trim() || null
    );
  } catch {
    return null;
  }
}

interface TokenResponse {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  user: {
    id: string;
    email: string;
    name: string;
  };
}

interface ApiError {
  error: string;
}

class ApiClient {
  private readonly transport: ApiTransport;
  // A page commonly starts several authenticated requests in parallel. Refresh
  // tokens are intentionally single-use, so those requests must share one
  // rotation rather than competing to revoke the same token.
  private refreshInFlight: Promise<boolean> | null = null;

  constructor() {
    this.transport = createApiTransport({
      getApiBase,
      getAccessToken: () => this.accessToken,
      getRefreshToken: () => this.refreshToken,
      refreshAccessToken: () => this.refreshAccessToken(),
      clearTokens: () => this.clearTokens(),
      getTenantContext: getCurrentTenantContext,
      onSessionExpired: () => {
        if (browser) {
          window.location.href = "/login";
        }
      },
    });
  }

  /**
   * Get the current access token from the auth store
   */
  private get accessToken(): string | null {
    return authStore.getAccessToken();
  }

  /**
   * Get the current refresh token from the auth store
   */
  private get refreshToken(): string | null {
    return authStore.getRefreshToken();
  }

  /**
   * Set tokens after login - use the auth store
   * @param access Access token
   * @param refresh Refresh token
   * @param rememberMe Whether to persist tokens across browser sessions
   */
  setTokens(access: string, refresh: string, rememberMe: boolean = false) {
    authStore.setTokens(access, refresh, rememberMe);
  }

  /**
   * Clear tokens on logout - use the auth store
   */
  clearTokens() {
    authStore.clearTokens();
  }

  /**
   * Check if user is authenticated
   */
  get isAuthenticated(): boolean {
    return !!this.accessToken;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    skipAuth = false,
    retryConfig: RetryConfig = DEFAULT_RETRY_CONFIG,
  ): Promise<T> {
    const response = await this.transport.request(method, path, body, {
      skipAuth,
      retryConfig,
    });
    return this.processResponse<T>(response);
  }

  /**
   * Process the response and extract data
   */
  private async processResponse<T>(response: Response): Promise<T> {
    let data: unknown;
    try {
      data = await response.json();
    } catch {
      // Server returned non-JSON response (HTML error page, empty body, etc.)
      if (!response.ok) {
        throw new Error(`Request failed with status ${response.status}`);
      }
      // For successful responses with no/invalid JSON, return empty object
      data = {};
    }

    if (!response.ok) {
      throw new Error(
        (data as ApiError).error ||
          `Request failed with status ${response.status}`,
      );
    }

    return this.parseDecimals(data) as T;
  }

  private async downloadFile(
    path: string,
    fileName: string,
    errorMessage: string,
  ) {
    const response = await this.transport.requestOnce("GET", path, undefined, {
      skipAuth: !this.accessToken,
    });

    if (!response.ok) {
      throw await getApiResponseError(response, errorMessage);
    }

    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
  }

  private parseDecimals(obj: unknown, key?: string): unknown {
    if (
      typeof obj === "string" &&
      /^-?\d+(\.\d+)?$/.test(obj) &&
      this.shouldParseDecimalField(key)
    ) {
      return new Decimal(obj);
    }
    if (Array.isArray(obj)) {
      return obj.map((item) => this.parseDecimals(item));
    }
    if (obj !== null && typeof obj === "object") {
      const result: Record<string, unknown> = {};
      for (const [key, value] of Object.entries(obj)) {
        result[key] = this.parseDecimals(value, key);
      }
      return result;
    }
    return obj;
  }

  private shouldParseDecimalField(key?: string): boolean {
    if (!key) return false;

    const normalized = key.toLowerCase();
    const stringFieldHints = [
      "id",
      "code",
      "number",
      "email",
      "phone",
      "postal",
      "country",
      "reg",
      "reference",
      "status",
      "type",
      "name",
      "date",
      "time",
      "currency",
      "token",
      "url",
      "slug",
      "iban",
      "bic",
      "swift",
      "file",
      "message",
      "description",
    ];
    if (
      stringFieldHints.some(
        (hint) =>
          normalized === hint ||
          normalized.endsWith(`_${hint}`) ||
          normalized.includes(`${hint}_`),
      )
    ) {
      return false;
    }

    const exactDecimalFields = [
      "reserved_qty",
      "available_qty",
      "inventory_value",
      "subledger_value",
      "general_ledger_balance",
      "difference",
    ];
    if (exactDecimalFields.includes(normalized)) {
      return true;
    }

    return [
      "amount",
      "balance",
      "total",
      "subtotal",
      "price",
      "rate",
      "percent",
      "percentage",
      "quantity",
      "debit",
      "credit",
      "vat",
      "tax",
      "discount",
      "income",
      "revenue",
      "expense",
      "payable",
      "receivable",
      "cost",
      "budget",
      "salary",
      "days",
      "hours",
      "net",
      "base",
      "current",
      "opening",
      "paid",
      "used",
      "limit",
    ].some((hint) => normalized.includes(hint));
  }

  private async refreshAccessToken(): Promise<boolean> {
    if (this.refreshInFlight) {
      return this.refreshInFlight;
    }

    this.refreshInFlight = this.performRefreshAccessToken();
    try {
      return await this.refreshInFlight;
    } finally {
      this.refreshInFlight = null;
    }
  }

  private async performRefreshAccessToken(): Promise<boolean> {
    try {
      const data = await this.request<
        Pick<TokenResponse, "access_token" | "refresh_token">
      >(
        "POST",
        "/api/v1/auth/refresh",
        { refresh_token: this.refreshToken },
        true,
      );
      if (
        typeof data.access_token !== "string" ||
        !data.access_token ||
        typeof data.refresh_token !== "string" ||
        !data.refresh_token
      ) {
        throw new Error("Invalid refresh response");
      }
      authStore.updateTokens(data.access_token, data.refresh_token);
      return true;
    } catch {
      this.clearTokens();
      return false;
    }
  }

  // Auth endpoints
  async register(email: string, password: string, name: string) {
    return this.request<{ id: string; email: string; name: string }>(
      "POST",
      "/api/v1/auth/register",
      { email, password, name },
      true,
    );
  }

  async login(
    email: string,
    password: string,
    rememberMe: boolean = false,
    tenantId?: string,
  ): Promise<TokenResponse> {
    const data = await this.request<TokenResponse>(
      "POST",
      "/api/v1/auth/login",
      { email, password, tenant_id: tenantId },
      true,
    );
    this.setTokens(data.access_token, data.refresh_token, rememberMe);
    return data;
  }

  logout() {
    this.clearTokens();
  }

  // User endpoints
  async getCurrentUser() {
    return this.request<{
      id: string;
      email: string;
      name: string;
      created_at: string;
    }>("GET", "/api/v1/me");
  }

  async getMyTenants() {
    return this.request<TenantMembership[]>("GET", "/api/v1/me/tenants");
  }

  // Tenant endpoints
  async createTenant(name: string, slug: string, settings?: TenantSettings) {
    return this.request<Tenant>("POST", "/api/v1/tenants", {
      name,
      slug,
      settings,
    });
  }

  async getTenant(tenantId: string) {
    return this.request<Tenant>("GET", `/api/v1/tenants/${tenantId}`);
  }

  async updateTenant(
    tenantId: string,
    data: { name?: string; settings?: Partial<TenantSettings> },
  ) {
    return this.request<Tenant>("PUT", `/api/v1/tenants/${tenantId}`, data);
  }

  async completeOnboarding(tenantId: string) {
    return this.request<{ success: boolean }>(
      "POST",
      `/api/v1/tenants/${tenantId}/complete-onboarding`,
    );
  }

  async listPeriodCloseEvents(tenantId: string, limit: number = 20) {
    return this.request<PeriodCloseEvent[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/period-close-events?limit=${limit}`,
    );
  }

  async listTenantAuditEvents(tenantId: string, limit: number = 50) {
    return this.request<TenantAuditEvent[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/audit-events?limit=${limit}`,
    );
  }

  async listTenantUsers(tenantId: string) {
    return this.request<TenantUser[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/users`,
    );
  }

  async updateTenantUserRole(
    tenantId: string,
    userId: string,
    role: EditableTenantRole,
  ) {
    return this.request<{ status: string }>(
      "PUT",
      `/api/v1/tenants/${tenantId}/users/${userId}/role`,
      { role },
    );
  }

  async updateTenantUserStatus(
    tenantId: string,
    userId: string,
    isActive: boolean,
  ) {
    return this.request<{ status: string; is_active: boolean }>(
      "PUT",
      `/api/v1/tenants/${tenantId}/users/${userId}/status`,
      { is_active: isActive },
    );
  }

  async removeTenantUser(tenantId: string, userId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/users/${userId}`,
    );
  }

  async listTenantUserAuthSessions(
    tenantId: string,
    userId: string,
    includeInactive = false,
  ) {
    const query = buildQuery({ include_inactive: includeInactive });
    return this.request<RefreshSession[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/users/${userId}/sessions${query}`,
    );
  }

  async revokeTenantUserAuthSession(
    tenantId: string,
    userId: string,
    sessionId: string,
  ) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/users/${userId}/sessions/${sessionId}`,
    );
  }

  async revokeTenantUserAuthSessions(tenantId: string, userId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/users/${userId}/sessions`,
    );
  }

  async listTenantUserAPITokens(tenantId: string, userId: string) {
    return this.request<APIToken[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/users/${userId}/api-tokens`,
    );
  }

  async revokeTenantUserAPIToken(
    tenantId: string,
    userId: string,
    tokenId: string,
  ) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/users/${userId}/api-tokens/${tokenId}`,
    );
  }

  async listTenantUserSecurityAuditEvents(
    tenantId: string,
    userId: string,
    limit: number = 50,
  ) {
    const query = buildQuery({ limit });
    return this.request<SecurityAuditEvent[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/users/${userId}/security-events${query}`,
    );
  }

  async listInvitations(tenantId: string) {
    return this.request<UserInvitation[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/invitations`,
    );
  }

  async createInvitation(tenantId: string, data: CreateInvitationRequest) {
    return this.request<UserInvitation>(
      "POST",
      `/api/v1/tenants/${tenantId}/invitations`,
      data,
    );
  }

  async revokeInvitation(tenantId: string, invitationId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/invitations/${invitationId}`,
    );
  }

  async closePeriod(tenantId: string, data: ClosePeriodRequest) {
    return this.request<PeriodCloseResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/period-close`,
      data,
    );
  }

  async reopenPeriod(tenantId: string, data: ReopenPeriodRequest) {
    return this.request<PeriodCloseResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/period-reopen`,
      data,
    );
  }

  async getYearEndCloseStatus(
    tenantId: string,
    periodEndDate: string,
    inventoryValuationMethod = "",
  ) {
    const query = buildQuery({
      period_end_date: periodEndDate,
      inventory_valuation_method: inventoryValuationMethod,
    });
    return this.request<YearEndCloseStatus>(
      "GET",
      `/api/v1/tenants/${tenantId}/year-end-close-status${query}`,
    );
  }

  async createYearEndCarryForward(
    tenantId: string,
    data: CreateYearEndCarryForwardRequest,
  ) {
    return this.request<YearEndCarryForwardResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/year-end-carry-forward`,
      data,
    );
  }

  async reverseYearEndCarryForward(
    tenantId: string,
    data: ReverseYearEndCarryForwardRequest,
  ) {
    return this.request<YearEndCarryForwardReversalResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/year-end-carry-forward/reverse`,
      data,
    );
  }

  async listDocuments(
    tenantId: string,
    entityType: DocumentAttachment["entity_type"],
    entityId: string,
  ) {
    const query = buildQuery({ entity_type: entityType, entity_id: entityId });
    return this.request<DocumentAttachment[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/documents${query}`,
    );
  }

  async listDocumentReviewSummaries(
    tenantId: string,
    entityType: DocumentAttachment["entity_type"],
    entityIds: string[],
  ) {
    return this.request<DocumentReviewSummary[]>(
      "POST",
      `/api/v1/tenants/${tenantId}/documents/review-summary`,
      {
        entity_type: entityType,
        entity_ids: entityIds,
      },
    );
  }

  async getDocumentReviewQueue(
    tenantId: string,
    filter: DocumentReviewQueueFilter = {},
  ) {
    const query = buildQuery(filter);
    return this.request<DocumentReviewQueue>(
      "GET",
      `/api/v1/tenants/${tenantId}/documents/review-queue${query}`,
    );
  }

  async getDocumentRetentionReview(
    tenantId: string,
    filter: DocumentRetentionReviewFilter = {},
  ) {
    const query = buildQuery(filter);
    return this.request<DocumentRetentionReview>(
      "GET",
      `/api/v1/tenants/${tenantId}/documents/retention${query}`,
    );
  }

  async evaluateDocumentEvidencePolicy(
    tenantId: string,
    data: EvidencePolicyRequest,
  ) {
    return this.request<EvidencePolicyResult[]>(
      "POST",
      `/api/v1/tenants/${tenantId}/documents/evidence-policy`,
      data,
    );
  }

  async listMigrationProviderPresets(tenantId: string) {
    return this.request<MigrationProviderPresetInfo[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/migration/provider-presets`,
    );
  }

  async validateMigrationBundle(tenantId: string, data: ValidateBundleRequest) {
    return this.request<BundleValidationReport>(
      "POST",
      `/api/v1/tenants/${tenantId}/migration/validate`,
      data,
    );
  }

  async planMigrationExecution(
    tenantId: string,
    data: PlanMigrationExecutionRequest,
  ) {
    return this.request<MigrationExecutionPlan>(
      "POST",
      `/api/v1/tenants/${tenantId}/migration/execution-plan`,
      data,
    );
  }

  async executeMigration(tenantId: string, data: ExecuteMigrationRequest) {
    return this.request<MigrationExecutionRun>(
      "POST",
      `/api/v1/tenants/${tenantId}/migration/execute`,
      data,
    );
  }

  async discoverSmartAccountsSyncSources(tenantId: string) {
    return this.request<SmartAccountsSourceDiscovery>(
      "GET",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/sources`,
    );
  }

  async getSmartAccountsSyncStatus(tenantId: string, sourceCompanyId: string) {
    return this.request<SmartAccountsSyncStatus>(
      "GET",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/status?source_company_id=${encodeURIComponent(sourceCompanyId)}`,
    );
  }

  async createSmartAccountsBrowserPairing(tenantId: string) {
    return this.request<SmartAccountsBrowserPairingIssue>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-pairings`,
    );
  }

  async getSmartAccountsBrowserPairing(tenantId: string, pairingId: string) {
    return this.request<SmartAccountsBrowserPairingStatus>(
      "GET",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-pairings/${encodeURIComponent(pairingId)}`,
    );
  }

  // Browser discovery is a same-window metadata relay only. The result is
  // sent to OA under the normal owner session; its opaque source binding is
  // resolved server-side and never supplied by this request.
  async createSmartAccountsBrowserDiscovery(tenantId: string, data: SmartAccountsBrowserDiscoveryStartRequest) {
    return this.request<SmartAccountsBrowserDiscoveryIssue>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-discoveries`,
      data,
    );
  }

  async submitSmartAccountsBrowserDiscoveryReceipt(tenantId: string, discoveryId: string, data: SmartAccountsBrowserDiscoveryRelayResult) {
    return this.request<SmartAccountsBrowserDiscoveryReceipt>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-discoveries/${encodeURIComponent(discoveryId)}/receipt`,
      data,
    );
  }

  async getSmartAccountsBrowserDiscoveryReceipt(tenantId: string, discoveryId: string) {
    return this.request<SmartAccountsBrowserDiscoveryReceipt>(
      "GET",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-discoveries/${encodeURIComponent(discoveryId)}`,
    );
  }

  // The reviewed schema assertion contains only owner confirmation. OA resolves
  // the paired opaque source internally and returns only binding-safe status and
  // a digest; no header, CSV, cookie, credential, or bridge data crosses this
  // browser-facing boundary.
  async reviewSmartAccountsBrowserCSVSchema(tenantId: string, discoveryId: string, resourceId: string, schemaId: string) {
    return this.request<SmartAccountsBrowserCSVSchemaApprovalResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-discoveries/${encodeURIComponent(discoveryId)}/resources/${encodeURIComponent(resourceId)}/schemas/${encodeURIComponent(schemaId)}/review`,
      { review_confirmed: true },
    );
  }

  async startSmartAccountsBrowserOnboarding(data: SmartAccountsBrowserOnboardingRequest) {
    return this.request<SmartAccountsBrowserOnboardingResponse>(
      "POST",
      "/api/v1/smartaccounts-sync/browser-onboarding",
      data,
    );
  }

  async getSmartAccountsBrowserOnboarding(sourceCompanyId: string) {
    return this.request<SmartAccountsBrowserOnboardingResult>(
      "GET",
      `/api/v1/smartaccounts-sync/browser-onboarding/${encodeURIComponent(sourceCompanyId)}`,
    );
  }

  async issueSmartAccountsBrowserOnboardingCatalog(data: SmartAccountsBrowserOnboardingCatalogIssueRequest) {
    return this.request<SmartAccountsBrowserOnboardingCatalogIssue>(
      "POST",
      "/api/v1/smartaccounts-sync/browser-onboarding/catalogs",
      data,
    );
  }

  async getSmartAccountsBrowserOnboardingCatalog(catalogId: string) {
    return this.request<SmartAccountsBrowserOnboardingCatalogStatus>(
      "GET",
      `/api/v1/smartaccounts-sync/browser-onboarding/catalogs/${encodeURIComponent(catalogId)}`,
    );
  }

  async startSmartAccountsBrowserOnboardingBatch(data: SmartAccountsBrowserOnboardingBatchRequest) {
    return this.request<SmartAccountsBrowserOnboardingBatchResponse>(
      "POST",
      "/api/v1/smartaccounts-sync/browser-onboarding/batches",
      data,
    );
  }

  async getSmartAccountsBrowserOnboardingBatch(batchId: string) {
    return this.request<SmartAccountsBrowserOnboardingBatchResponse>(
      "GET",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}`,
    );
  }

  async resumeSmartAccountsBrowserOnboardingBatch(batchId: string) {
    return this.request<SmartAccountsBrowserOnboardingBatchResponse>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/resume`,
      { owner_confirmed: true },
    );
  }

  async prepareSmartAccountsBrowserOnboardingBatchWorkflow(
    batchId: string,
    data: SmartAccountsBrowserBatchWorkflowPreparationRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowStatus>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow`,
      data,
    );
  }

  async getSmartAccountsBrowserOnboardingBatchWorkflow(batchId: string) {
    return this.request<SmartAccountsBrowserBatchWorkflowStatus>(
      "GET",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow`,
    );
  }

  async resumeSmartAccountsBrowserOnboardingBatchWorkflow(batchId: string) {
    return this.request<SmartAccountsBrowserBatchWorkflowStatus>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/resume`,
      {},
    );
  }

  async acquireSmartAccountsBrowserOnboardingBatchDiscovery(
    batchId: string,
    data: SmartAccountsBrowserBatchWorkflowDiscoveryAcquireRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowDiscoveryAcquireResponse>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/discovery/acquire`,
      data,
    );
  }

  async reissueSmartAccountsBrowserOnboardingBatchDiscovery(
    batchId: string,
    sourceCompanyId: string,
    data: SmartAccountsBrowserBatchWorkflowDiscoveryAcquireRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowDiscoveryAcquireResponse>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/sources/${encodeURIComponent(sourceCompanyId)}/discovery/reissue`,
      data,
    );
  }

  async completeSmartAccountsBrowserOnboardingBatchDiscovery(
    batchId: string,
    sourceCompanyId: string,
    data: SmartAccountsBrowserBatchWorkflowDiscoveryCompleteRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowSource>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/sources/${encodeURIComponent(sourceCompanyId)}/discovery/complete`,
      data,
    );
  }

  async requireSmartAccountsBrowserOnboardingBatchSchema(
    batchId: string,
    sourceCompanyId: string,
    data: SmartAccountsBrowserBatchWorkflowPhaseRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowSource>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/sources/${encodeURIComponent(sourceCompanyId)}/schema/require`,
      data,
    );
  }

  async refreshSmartAccountsBrowserOnboardingBatchSchema(
    batchId: string,
    sourceCompanyId: string,
    data: SmartAccountsBrowserBatchWorkflowPhaseRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowSource>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/sources/${encodeURIComponent(sourceCompanyId)}/schema/refresh`,
      data,
    );
  }

  async confirmSmartAccountsBrowserOnboardingBatchSchema(
    batchId: string,
    sourceCompanyId: string,
    data: SmartAccountsBrowserBatchWorkflowSchemaConfirmationRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowSource>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/sources/${encodeURIComponent(sourceCompanyId)}/schema/confirm`,
      data,
    );
  }

  async openSmartAccountsBrowserOnboardingBatchTransfer(batchId: string) {
    return this.request<SmartAccountsBrowserBatchWorkflowStatus>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/transfer/open`,
      {},
    );
  }

  async confirmSmartAccountsBrowserOnboardingBatchTransfer(
    batchId: string,
    data: SmartAccountsBrowserBatchWorkflowTransferConfirmationRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowStatus>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/transfer/confirm`,
      data,
    );
  }

  async acquireSmartAccountsBrowserOnboardingBatchCapture(
    batchId: string,
    data: SmartAccountsBrowserBatchWorkflowCaptureAcquireRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowCaptureAcquireResponse>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/capture/acquire`,
      data,
    );
  }

  async completeSmartAccountsBrowserOnboardingBatchCapture(
    batchId: string,
    sourceCompanyId: string,
    data: SmartAccountsBrowserBatchWorkflowCaptureCompleteRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowCaptureCompleteResponse>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/sources/${encodeURIComponent(sourceCompanyId)}/capture/complete`,
      data,
    );
  }

  async previewSmartAccountsBrowserOnboardingBatchSource(
    batchId: string,
    sourceCompanyId: string,
    data: SmartAccountsBrowserBatchWorkflowPreviewRequest,
  ) {
    return this.request<SmartAccountsBrowserBatchWorkflowSource>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/workflow/sources/${encodeURIComponent(sourceCompanyId)}/preview`,
      data,
    );
  }

  async createSmartAccountsBrowserCapture(tenantId: string, data: SmartAccountsBrowserCaptureStartRequest) {
    return this.request<SmartAccountsBrowserCaptureIssue>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-captures`,
      data,
    );
  }

  async resumeSmartAccountsBrowserCapture(tenantId: string, runId: string, data: SmartAccountsBrowserCaptureResumeRequest) {
    return this.request<SmartAccountsBrowserCaptureIssue>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-captures/${encodeURIComponent(runId)}/resume`,
      data,
    );
  }

  async getSmartAccountsBrowserCaptureStatus(tenantId: string, runId: string) {
    return this.request<SmartAccountsBrowserCaptureStatus>(
      "GET",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-captures/${encodeURIComponent(runId)}`,
    );
  }

  async startSmartAccountsBrowserCaptureWorkflow(tenantId: string, data: SmartAccountsBrowserCaptureWorkflowRequest) {
    return this.request<SmartAccountsBrowserCaptureWorkflowStatus>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-capture-workflows`,
      data,
    );
  }

  async getSmartAccountsBrowserCaptureWorkflowStatus(tenantId: string, workflowId: string) {
    return this.request<SmartAccountsBrowserCaptureWorkflowStatus>(
      "GET",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-capture-workflows/${encodeURIComponent(workflowId)}`,
    );
  }

  // Master detail has its own current-snapshot relay. The issue response is
  // forwarded directly to extension memory; status never returns a token.
  async issueSmartAccountsBrowserMasterDetails(tenantId: string, data: SmartAccountsBrowserMasterDetailAuthorizeRequest) {
    return this.request<SmartAccountsBrowserMasterDetailIssueSet>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-master-details`,
      data,
    );
  }

  async getSmartAccountsBrowserMasterDetailStatus(tenantId: string, runId: string) {
    return this.request<SmartAccountsBrowserMasterDetailStatus>(
      "GET",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-master-details/${encodeURIComponent(runId)}`,
    );
  }

  async resumeSmartAccountsBrowserMasterDetail(tenantId: string, runId: string, data: SmartAccountsBrowserMasterDetailResumeRequest) {
    return this.request<SmartAccountsBrowserMasterDetailIssue>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/browser-master-details/${encodeURIComponent(runId)}/resume`,
      data,
    );
  }

  async configureSmartAccountsSync(
    tenantId: string,
    data: ConfigureSmartAccountsSyncRequest,
  ) {
    return this.request<SmartAccountsSyncStatus>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/control`,
      data,
    );
  }

  async requestSmartAccountsSyncDryRun(
    tenantId: string,
    sourceCompanyId: string,
    data: SmartAccountsCaptureRequest,
  ) {
    return this.request<SmartAccountsSyncStatus>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/dry-run?source_company_id=${encodeURIComponent(sourceCompanyId)}`,
      data,
    );
  }

  async confirmSmartAccountsFinancialApply(
    tenantId: string,
    sourceCompanyId: string,
    confirm: boolean,
  ) {
    return this.request<SmartAccountsSyncStatus>(
      "POST",
      `/api/v1/tenants/${tenantId}/smartaccounts-sync/apply?source_company_id=${encodeURIComponent(sourceCompanyId)}`,
      { confirm },
    );
  }

  async previewSmartAccountsPackage(tenantId: string, packageId: string, data: SmartAccountsPackagePreviewRequest) {
    return this.request<SmartAccountsPackagePreview>("POST", `/api/v1/tenants/${tenantId}/smartaccounts-sync/packages/${encodeURIComponent(packageId)}/preview`, data);
  }

  async applySmartAccountsPackage(tenantId: string, data: SmartAccountsPackageApplyRequest) {
    return this.request<SmartAccountsPackagePreview>("POST", `/api/v1/tenants/${tenantId}/smartaccounts-sync/packages/apply`, data);
  }

  // Count-only, tenant-scoped evidence coverage for an already checksum-finalized
  // package. It is not a preview, apply authorization, or full-sync claim.
  async getSmartAccountsPackageArchiveCoverage(tenantId: string, packageId: string) {
    return this.request<SmartAccountsArchiveCoverageReport>(
      "GET",
      `/api/v1/tenants/${encodeURIComponent(tenantId)}/smartaccounts-sync/packages/${encodeURIComponent(packageId)}/archive-coverage`,
    );
  }

  // Reconciliation is deliberately split between owner-safe technical evidence
  // and tenant-scoped interactive accountant attestations. These endpoints
  // expose only fixed status, count, and digest handles; never source rows,
  // proof payloads, monetary values, or browser capabilities.
  async evaluateSmartAccountsReconciliation(batchId: string, sourceCompanyId: string) {
    return this.request<SmartAccountsReconciliationEvaluationResponse>(
      "POST",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/sources/${encodeURIComponent(sourceCompanyId)}/reconciliation`,
      {},
    );
  }

  async getSmartAccountsReconciliation(batchId: string, sourceCompanyId: string) {
    return this.request<SmartAccountsReconciliationEvaluation>(
      "GET",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/sources/${encodeURIComponent(sourceCompanyId)}/reconciliation`,
    );
  }

  // Accountant-only, no-store view of one tenant/batch/source binding. It is
  // intentionally separate from the owner-only evaluation and selected/all
  // roll-up routes, and returns the same digest-only evaluation handle.
  async getSmartAccountsTenantReconciliation(tenantId: string, batchId: string, sourceCompanyId: string) {
    return this.request<SmartAccountsReconciliationEvaluation>(
      "GET",
      `/api/v1/tenants/${encodeURIComponent(tenantId)}/smartaccounts-sync/reconciliation/batches/${encodeURIComponent(batchId)}/sources/${encodeURIComponent(sourceCompanyId)}`,
    );
  }

  async getSmartAccountsReconciliationRollup(batchId: string) {
    return this.request<SmartAccountsReconciliationRollup>(
      "GET",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/reconciliation`,
    );
  }

  // This is an owner-only, count/code-only product-coverage gate. It is not a
  // reconciliation approval and cannot start or apply a sync.
  async getSmartAccountsFullClaimEligibility(batchId: string) {
    return this.request<SmartAccountsFullClaimEligibility>(
      "GET",
      `/api/v1/smartaccounts-sync/browser-onboarding/batches/${encodeURIComponent(batchId)}/full-claim-eligibility`,
    );
  }

  async approveSmartAccountsTolerancePolicy(
    tenantId: string,
    sourceCompanyId: string,
    data: SmartAccountsTolerancePolicyApprovalRequest,
  ) {
    return this.request<SmartAccountsTolerancePolicy>(
      "POST",
      `/api/v1/tenants/${encodeURIComponent(tenantId)}/smartaccounts-sync/sources/${encodeURIComponent(sourceCompanyId)}/tolerance-policies`,
      data,
    );
  }

  async getSmartAccountsTolerancePolicyCandidate(
    tenantId: string,
    sourceCompanyId: string,
    data: SmartAccountsTolerancePolicyCandidateRequest,
  ) {
    return this.request<SmartAccountsTolerancePolicyCandidate>(
      "POST",
      `/api/v1/tenants/${encodeURIComponent(tenantId)}/smartaccounts-sync/sources/${encodeURIComponent(sourceCompanyId)}/tolerance-policy-candidates`,
      data,
    );
  }

  // An owner or accountant can resolve the current approved policy only for
  // this exact staged package/preview. The UI keeps the opaque policy ID only
  // long enough to submit the separately confirmed financial apply; the
  // server revalidates the binding and does not trust a supplied digest.
  async resolveSmartAccountsTolerancePolicy(
    tenantId: string,
    sourceCompanyId: string,
    data: SmartAccountsTolerancePolicyResolutionRequest,
  ) {
    return this.request<SmartAccountsTolerancePolicyResolution>(
      "POST",
      `/api/v1/tenants/${encodeURIComponent(tenantId)}/smartaccounts-sync/sources/${encodeURIComponent(sourceCompanyId)}/tolerance-policy-resolutions`,
      data,
    );
  }

  async approveSmartAccountsReconciliation(
    tenantId: string,
    evaluationId: string,
    data: SmartAccountsReconciliationApprovalRequest,
  ) {
    return this.request<SmartAccountsReconciliationEvaluation>(
      "POST",
      `/api/v1/tenants/${encodeURIComponent(tenantId)}/smartaccounts-sync/reconciliation/evaluations/${encodeURIComponent(evaluationId)}/approval`,
      data,
    );
  }

  // Reference masters are deliberately separate from the GL executor. The
  // corresponding API never receives raw canonical payloads or posts money.
  async previewSmartAccountsReferenceMasters(tenantId: string, packageId: string, data: SmartAccountsReferencePreviewRequest = {}) {
    return this.request<SmartAccountsReferencePreview>("POST", `/api/v1/tenants/${tenantId}/smartaccounts-sync/packages/${encodeURIComponent(packageId)}/reference-preview`, data);
  }

  async applySmartAccountsReferenceMasters(tenantId: string, data: SmartAccountsReferenceApplyRequest) {
    return this.request<SmartAccountsReferencePreview>("POST", `/api/v1/tenants/${tenantId}/smartaccounts-sync/reference-masters/apply`, data);
  }

  async listMigrationExecutionRuns(
    tenantId: string,
    filter: MigrationExecutionRunFilter = {},
  ) {
    const query = buildQuery(filter);
    return this.request<MigrationExecutionRun[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/migration/execution-runs${query}`,
    );
  }

  async getMigrationExecutionRun(tenantId: string, runId: string) {
    return this.request<MigrationExecutionRun>(
      "GET",
      `/api/v1/tenants/${tenantId}/migration/execution-runs/${runId}`,
    );
  }

  async watchMigrationExecutionRun(
    tenantId: string,
    runId: string,
    options: WatchMigrationExecutionRunOptions,
  ): Promise<void> {
    const params = new URLSearchParams();
    if (options.intervalMs !== undefined) {
      params.set("interval_ms", String(options.intervalMs));
    }
    if (options.maxEvents !== undefined) {
      params.set("max_events", String(options.maxEvents));
    }
    const query = params.toString();
    const path = `/api/v1/tenants/${tenantId}/migration/execution-runs/${runId}/events${query ? `?${query}` : ""}`;
    const response = await this.transport.requestOnce("GET", path, undefined, {
      headers: { Accept: "text/event-stream" },
      signal: options.signal,
    });

    if (!response.ok) {
      throw await getApiResponseError(
        response,
        `Migration execution run stream failed with status ${response.status}`,
      );
    }

    if (!response.body) {
      throw new Error("Migration execution run stream is not available.");
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";

    const emitFrame = async (frame: string) => {
      const dataLines: string[] = [];
      let eventType = "";
      let sequence = 0;

      for (const line of frame.split("\n")) {
        if (!line || line.startsWith(":")) {
          continue;
        }
        if (line.startsWith("event:")) {
          eventType = line.slice("event:".length).trim();
          continue;
        }
        if (line.startsWith("id:")) {
          const parsed = Number(line.slice("id:".length).trim());
          if (Number.isFinite(parsed)) {
            sequence = parsed;
          }
          continue;
        }
        if (line.startsWith("data:")) {
          dataLines.push(line.slice("data:".length).trimStart());
        }
      }

      if (dataLines.length === 0) {
        return;
      }

      const parsed = JSON.parse(
        dataLines.join("\n"),
      ) as Partial<MigrationExecutionRunEvent>;
      const event = this.parseDecimals({
        ...parsed,
        type: parsed.type || eventType,
        sequence: parsed.sequence || sequence,
      }) as MigrationExecutionRunEvent;
      await options.onEvent(event);
    };

    const flushFrames = async () => {
      let frameEnd = buffer.indexOf("\n\n");
      while (frameEnd >= 0) {
        const frame = buffer.slice(0, frameEnd);
        buffer = buffer.slice(frameEnd + 2);
        await emitFrame(frame);
        frameEnd = buffer.indexOf("\n\n");
      }
    };

    while (true) {
      const { value, done } = await reader.read();
      if (done) {
        break;
      }
      buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, "\n");
      await flushFrames();
    }

    buffer += decoder.decode().replace(/\r\n/g, "\n");
    if (buffer.trim()) {
      await emitFrame(buffer);
    }
  }

  async listExpenses(tenantId: string, filter: ListExpensesFilter = {}) {
    const query = buildQuery(filter);
    return this.request<ExpenseClaim[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/expenses${query}`,
    );
  }

  async submitExpense(tenantId: string, expenseId: string) {
    return this.request<ExpenseClaim>(
      "POST",
      `/api/v1/tenants/${tenantId}/expenses/${expenseId}/submit`,
    );
  }

  async approveExpense(tenantId: string, expenseId: string) {
    return this.request<ExpenseClaim>(
      "POST",
      `/api/v1/tenants/${tenantId}/expenses/${expenseId}/approve`,
    );
  }

  async postExpense(tenantId: string, expenseId: string) {
    return this.request<ExpenseClaim>(
      "POST",
      `/api/v1/tenants/${tenantId}/expenses/${expenseId}/post`,
    );
  }

  async updateDocumentRetention(
    tenantId: string,
    documentId: string,
    data: DocumentRetentionUpdateRequest,
  ) {
    return this.request<DocumentAttachment>(
      "PATCH",
      `/api/v1/tenants/${tenantId}/documents/${documentId}/retention`,
      data,
    );
  }

  async uploadDocument(
    tenantId: string,
    entityType: DocumentAttachment["entity_type"],
    entityId: string,
    file: File,
    options?: {
      document_type?: DocumentAttachment["document_type"];
      notes?: string;
      retention_until?: string;
      replaces_document_id?: string;
      replacement_note?: string;
    },
  ) {
    const formData = new FormData();
    formData.set("entity_type", entityType);
    formData.set("entity_id", entityId);
    formData.set("file", file);
    if (options?.document_type) {
      formData.set("document_type", options.document_type);
    }
    if (options?.notes) {
      formData.set("notes", options.notes);
    }
    if (options?.retention_until) {
      formData.set("retention_until", options.retention_until);
    }
    if (options?.replaces_document_id) {
      formData.set("replaces_document_id", options.replaces_document_id);
    }
    if (options?.replacement_note) {
      formData.set("replacement_note", options.replacement_note);
    }

    return this.request<DocumentAttachment>(
      "POST",
      `/api/v1/tenants/${tenantId}/documents`,
      formData,
    );
  }

  async markDocumentReviewed(tenantId: string, documentId: string) {
    return this.request<DocumentAttachment>(
      "POST",
      `/api/v1/tenants/${tenantId}/documents/${documentId}/mark-reviewed`,
    );
  }

  async reviewDocument(
    tenantId: string,
    documentId: string,
    data: ReviewDocumentRequest,
  ) {
    return this.request<DocumentAttachment>(
      "POST",
      `/api/v1/tenants/${tenantId}/documents/${documentId}/review`,
      data,
    );
  }

  async deleteDocument(tenantId: string, documentId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/documents/${documentId}`,
    );
  }

  async downloadDocument(
    tenantId: string,
    documentId: string,
    fileName: string,
  ) {
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/documents/${documentId}/download`,
      fileName,
      "Failed to download document",
    );
  }

  async downloadYearEndCloseAuditArchive(
    tenantId: string,
    periodEndDate: string,
    inventoryValuationMethod = "",
  ) {
    const query = buildQuery({
      period_end_date: periodEndDate,
      inventory_valuation_method: inventoryValuationMethod,
    });
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/year-end-close-audit-archive${query}`,
      `year-end-close-audit-${periodEndDate}.zip`,
      "Failed to download year-end audit archive",
    );
  }

  // Account endpoints
  async listAccounts(tenantId: string, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request<Account[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/accounts${query}`,
    );
  }

  async createAccount(tenantId: string, data: CreateAccountRequest) {
    return this.request<Account>(
      "POST",
      `/api/v1/tenants/${tenantId}/accounts`,
      data,
    );
  }

  async updateAccount(
    tenantId: string,
    accountId: string,
    data: UpdateAccountRequest,
  ) {
    return this.request<Account>(
      "PUT",
      `/api/v1/tenants/${tenantId}/accounts/${accountId}`,
      data,
    );
  }

  async deleteAccount(tenantId: string, accountId: string) {
    return this.request<Account>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/accounts/${accountId}`,
    );
  }

  async importAccounts(tenantId: string, data: ImportAccountsRequest) {
    return this.request<ImportAccountsResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/accounts/import`,
      data,
    );
  }

  async getAccount(tenantId: string, accountId: string) {
    return this.request<Account>(
      "GET",
      `/api/v1/tenants/${tenantId}/accounts/${accountId}`,
    );
  }

  // Journal entry endpoints
  async listJournalEntries(tenantId: string, limit = 50) {
    return this.request<JournalEntry[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/journal-entries?limit=${limit}`,
    );
  }

  async getJournalEntry(tenantId: string, entryId: string) {
    return this.request<JournalEntry>(
      "GET",
      `/api/v1/tenants/${tenantId}/journal-entries/${entryId}`,
    );
  }

  async createJournalEntry(tenantId: string, data: CreateJournalEntryRequest) {
    return this.request<JournalEntry>(
      "POST",
      `/api/v1/tenants/${tenantId}/journal-entries`,
      data,
    );
  }

  async importOpeningBalances(
    tenantId: string,
    data: ImportOpeningBalancesRequest,
  ) {
    return this.request<ImportOpeningBalancesResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/journal-entries/import-opening-balances`,
      data,
    );
  }

  async postJournalEntry(tenantId: string, entryId: string, reason: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/journal-entries/${entryId}/post`,
      { reason },
    );
  }

  async voidJournalEntry(tenantId: string, entryId: string, reason: string) {
    return this.request<JournalEntry>(
      "POST",
      `/api/v1/tenants/${tenantId}/journal-entries/${entryId}/void`,
      { reason },
    );
  }

  // Report endpoints
  async getTrialBalance(tenantId: string, asOfDate?: string) {
    const query = asOfDate ? `?as_of_date=${asOfDate}` : "";
    return this.request<TrialBalance>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/trial-balance${query}`,
    );
  }

  async getAccountBalance(
    tenantId: string,
    accountId: string,
    asOfDate?: string,
  ) {
    const query = asOfDate ? `?as_of_date=${asOfDate}` : "";
    return this.request<{
      account_id: string;
      as_of_date: string;
      balance: Decimal;
    }>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/account-balance/${accountId}${query}`,
    );
  }

  async getBalanceSheet(tenantId: string, asOfDate?: string) {
    const query = asOfDate ? `?as_of=${asOfDate}` : "";
    return this.request<BalanceSheet>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/balance-sheet${query}`,
    );
  }

  async getIncomeStatement(
    tenantId: string,
    startDate: string,
    endDate: string,
  ) {
    return this.request<IncomeStatement>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/income-statement?start=${startDate}&end=${endDate}`,
    );
  }

  // Contact endpoints
  async listContacts(tenantId: string, filter?: ContactFilter) {
    const query = buildQuery(filter);
    return this.request<Contact[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/contacts${query}`,
    );
  }

  async createContact(tenantId: string, data: CreateContactRequest) {
    return this.request<Contact>(
      "POST",
      `/api/v1/tenants/${tenantId}/contacts`,
      data,
    );
  }

  async importContacts(tenantId: string, data: ImportContactsRequest) {
    return this.request<ImportContactsResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/contacts/import`,
      data,
    );
  }

  async getContact(tenantId: string, contactId: string) {
    return this.request<Contact>(
      "GET",
      `/api/v1/tenants/${tenantId}/contacts/${contactId}`,
    );
  }

  async updateContact(
    tenantId: string,
    contactId: string,
    data: UpdateContactRequest,
  ) {
    return this.request<Contact>(
      "PUT",
      `/api/v1/tenants/${tenantId}/contacts/${contactId}`,
      data,
    );
  }

  async deleteContact(tenantId: string, contactId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/contacts/${contactId}`,
    );
  }

  // Invoice endpoints
  async listInvoices(tenantId: string, filter?: InvoiceFilter) {
    const query = buildQuery(filter);
    return this.request<Invoice[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/invoices${query}`,
    );
  }

  async createInvoice(tenantId: string, data: CreateInvoiceRequest) {
    return this.request<Invoice>(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices`,
      data,
    );
  }

  async importInvoices(tenantId: string, data: ImportInvoicesRequest) {
    return this.request<ImportInvoicesResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/import`,
      data,
    );
  }

  async getInvoice(tenantId: string, invoiceId: string) {
    return this.request<Invoice>(
      "GET",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}`,
    );
  }

  async sendInvoice(tenantId: string, invoiceId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/send`,
    );
  }

  async voidInvoice(tenantId: string, invoiceId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/void`,
    );
  }

  async downloadInvoicePDF(
    tenantId: string,
    invoiceId: string,
    invoiceNumber: string,
  ) {
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/pdf`,
      `invoice-${invoiceNumber}.pdf`,
      "Failed to download PDF",
    );
  }

  // Payment endpoints
  async listPayments(tenantId: string, filter?: PaymentFilter) {
    const query = buildQuery(filter);
    return this.request<Payment[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/payments${query}`,
    );
  }

  async createPayment(tenantId: string, data: CreatePaymentRequest) {
    return this.request<Payment>(
      "POST",
      `/api/v1/tenants/${tenantId}/payments`,
      data,
    );
  }

  async getPayment(tenantId: string, paymentId: string) {
    return this.request<Payment>(
      "GET",
      `/api/v1/tenants/${tenantId}/payments/${paymentId}`,
    );
  }

  async allocatePayment(
    tenantId: string,
    paymentId: string,
    invoiceId: string,
    amount: string,
  ) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/payments/${paymentId}/allocate`,
      { invoice_id: invoiceId, amount },
    );
  }

  async reversePayment(
    tenantId: string,
    paymentId: string,
    data: ReversePaymentRequest,
  ) {
    return this.request<PaymentReversalResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/payments/${paymentId}/reverse`,
      data,
    );
  }

  async getUnallocatedPayments(
    tenantId: string,
    type: "RECEIVED" | "MADE" = "RECEIVED",
  ) {
    return this.request<Payment[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/payments/unallocated?type=${type}`,
    );
  }

  // Quote endpoints
  async listQuotes(tenantId: string, filter?: QuoteFilter) {
    const query = buildQuery(filter);
    return this.request<Quote[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/quotes${query}`,
    );
  }

  async createQuote(tenantId: string, data: CreateQuoteRequest) {
    return this.request<Quote>(
      "POST",
      `/api/v1/tenants/${tenantId}/quotes`,
      data,
    );
  }

  async getQuote(tenantId: string, quoteId: string) {
    return this.request<Quote>(
      "GET",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}`,
    );
  }

  async updateQuote(
    tenantId: string,
    quoteId: string,
    data: UpdateQuoteRequest,
  ) {
    return this.request<Quote>(
      "PUT",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}`,
      data,
    );
  }

  async deleteQuote(tenantId: string, quoteId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}`,
    );
  }

  async sendQuote(tenantId: string, quoteId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}/send`,
    );
  }

  async downloadQuotePDF(
    tenantId: string,
    quoteId: string,
    quoteNumber: string,
  ) {
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}/pdf`,
      `quote-${quoteNumber}.pdf`,
      "Failed to download quote PDF",
    );
  }

  async emailQuote(
    tenantId: string,
    quoteId: string,
    data: SendQuoteEmailRequest,
  ) {
    return this.request<EmailSentResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}/email`,
      data,
    );
  }

  async acceptQuote(tenantId: string, quoteId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}/accept`,
    );
  }

  async rejectQuote(tenantId: string, quoteId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}/reject`,
    );
  }

  async convertQuoteToInvoice(
    tenantId: string,
    quoteId: string,
    data: ConvertQuoteToInvoiceRequest = {},
  ) {
    return this.request<QuoteInvoiceConversionResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/quotes/${quoteId}/convert-to-invoice`,
      data,
    );
  }

  // Orders endpoints
  async listOrders(tenantId: string, filter?: OrderFilter) {
    const query = buildQuery(filter);
    return this.request<Order[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/orders${query}`,
    );
  }

  async createOrder(tenantId: string, data: CreateOrderRequest) {
    return this.request<Order>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders`,
      data,
    );
  }

  async getOrder(tenantId: string, orderId: string) {
    return this.request<Order>(
      "GET",
      `/api/v1/tenants/${tenantId}/orders/${orderId}`,
    );
  }

  async updateOrder(
    tenantId: string,
    orderId: string,
    data: UpdateOrderRequest,
  ) {
    return this.request<Order>(
      "PUT",
      `/api/v1/tenants/${tenantId}/orders/${orderId}`,
      data,
    );
  }

  async deleteOrder(tenantId: string, orderId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/orders/${orderId}`,
    );
  }

  async confirmOrder(tenantId: string, orderId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders/${orderId}/confirm`,
    );
  }

  async downloadOrderPDF(
    tenantId: string,
    orderId: string,
    orderNumber: string,
  ) {
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/orders/${orderId}/pdf`,
      `order-${orderNumber}.pdf`,
      "Failed to download order PDF",
    );
  }

  async emailOrder(
    tenantId: string,
    orderId: string,
    data: SendOrderEmailRequest,
  ) {
    return this.request<EmailSentResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders/${orderId}/email`,
      data,
    );
  }

  async processOrder(tenantId: string, orderId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders/${orderId}/process`,
    );
  }

  async shipOrder(tenantId: string, orderId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders/${orderId}/ship`,
    );
  }

  async deliverOrder(tenantId: string, orderId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders/${orderId}/deliver`,
    );
  }

  async cancelOrder(tenantId: string, orderId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders/${orderId}/cancel`,
    );
  }

  async convertOrderToInvoice(
    tenantId: string,
    orderId: string,
    data: ConvertOrderToInvoiceRequest = {},
  ) {
    return this.request<OrderInvoiceConversionResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/orders/${orderId}/convert-to-invoice`,
      data,
    );
  }

  // Fixed Assets - Categories endpoints
  async listAssetCategories(tenantId: string) {
    return this.request<AssetCategory[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/asset-categories`,
    );
  }

  async createAssetCategory(
    tenantId: string,
    data: CreateAssetCategoryRequest,
  ) {
    return this.request<AssetCategory>(
      "POST",
      `/api/v1/tenants/${tenantId}/asset-categories`,
      data,
    );
  }

  async getAssetCategory(tenantId: string, categoryId: string) {
    return this.request<AssetCategory>(
      "GET",
      `/api/v1/tenants/${tenantId}/asset-categories/${categoryId}`,
    );
  }

  async deleteAssetCategory(tenantId: string, categoryId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/asset-categories/${categoryId}`,
    );
  }

  // Fixed Assets endpoints
  async listAssets(tenantId: string, filter?: AssetFilter) {
    const params = new URLSearchParams();
    if (filter?.status) params.set("status", filter.status);
    if (filter?.category_id) params.set("category_id", filter.category_id);
    if (filter?.from_date) params.set("from_date", filter.from_date);
    if (filter?.to_date) params.set("to_date", filter.to_date);
    if (filter?.search) params.set("search", filter.search);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request<FixedAsset[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/assets${query}`,
    );
  }

  async createAsset(tenantId: string, data: CreateAssetRequest) {
    return this.request<FixedAsset>(
      "POST",
      `/api/v1/tenants/${tenantId}/assets`,
      data,
    );
  }

  async getAsset(tenantId: string, assetId: string) {
    return this.request<FixedAsset>(
      "GET",
      `/api/v1/tenants/${tenantId}/assets/${assetId}`,
    );
  }

  async updateAsset(
    tenantId: string,
    assetId: string,
    data: UpdateAssetRequest,
  ) {
    return this.request<FixedAsset>(
      "PUT",
      `/api/v1/tenants/${tenantId}/assets/${assetId}`,
      data,
    );
  }

  async deleteAsset(tenantId: string, assetId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/assets/${assetId}`,
    );
  }

  async activateAsset(tenantId: string, assetId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/assets/${assetId}/activate`,
    );
  }

  async disposeAsset(
    tenantId: string,
    assetId: string,
    data: DisposeAssetRequest,
  ) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/assets/${assetId}/dispose`,
      data,
    );
  }

  async recordDepreciation(
    tenantId: string,
    assetId: string,
    data: RecordDepreciationRequest,
  ) {
    return this.request<DepreciationEntry>(
      "POST",
      `/api/v1/tenants/${tenantId}/assets/${assetId}/depreciation`,
      data,
    );
  }

  async getDepreciationHistory(tenantId: string, assetId: string) {
    return this.request<DepreciationEntry[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/assets/${assetId}/depreciation`,
    );
  }

  // Inventory - Product Categories endpoints
  async listProductCategories(tenantId: string) {
    return this.request<ProductCategory[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/product-categories`,
    );
  }

  async createProductCategory(
    tenantId: string,
    data: CreateProductCategoryRequest,
  ) {
    return this.request<ProductCategory>(
      "POST",
      `/api/v1/tenants/${tenantId}/product-categories`,
      data,
    );
  }

  async getProductCategory(tenantId: string, categoryId: string) {
    return this.request<ProductCategory>(
      "GET",
      `/api/v1/tenants/${tenantId}/product-categories/${categoryId}`,
    );
  }

  async deleteProductCategory(tenantId: string, categoryId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/product-categories/${categoryId}`,
    );
  }

  // Inventory - Products endpoints
  async listProducts(tenantId: string, filter?: ProductFilter) {
    const params = new URLSearchParams();
    if (filter?.product_type) params.set("product_type", filter.product_type);
    if (filter?.status) params.set("status", filter.status);
    if (filter?.category_id) params.set("category_id", filter.category_id);
    if (filter?.search) params.set("search", filter.search);
    if (filter?.low_stock) params.set("low_stock", "true");
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request<Product[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/products${query}`,
    );
  }

  async createProduct(tenantId: string, data: CreateProductRequest) {
    return this.request<Product>(
      "POST",
      `/api/v1/tenants/${tenantId}/products`,
      data,
    );
  }

  async getProduct(tenantId: string, productId: string) {
    return this.request<Product>(
      "GET",
      `/api/v1/tenants/${tenantId}/products/${productId}`,
    );
  }

  async updateProduct(
    tenantId: string,
    productId: string,
    data: UpdateProductRequest,
  ) {
    return this.request<Product>(
      "PUT",
      `/api/v1/tenants/${tenantId}/products/${productId}`,
      data,
    );
  }

  async deleteProduct(tenantId: string, productId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/products/${productId}`,
    );
  }

  async getProductStockLevels(tenantId: string, productId: string) {
    return this.request<StockLevel[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/products/${productId}/stock-levels`,
    );
  }

  async getProductMovements(tenantId: string, productId: string) {
    return this.request<InventoryMovement[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/products/${productId}/movements`,
    );
  }

  // Inventory - Warehouses endpoints
  async listWarehouses(tenantId: string, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request<Warehouse[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/warehouses${query}`,
    );
  }

  async createWarehouse(tenantId: string, data: CreateWarehouseRequest) {
    return this.request<Warehouse>(
      "POST",
      `/api/v1/tenants/${tenantId}/warehouses`,
      data,
    );
  }

  async getWarehouse(tenantId: string, warehouseId: string) {
    return this.request<Warehouse>(
      "GET",
      `/api/v1/tenants/${tenantId}/warehouses/${warehouseId}`,
    );
  }

  async updateWarehouse(
    tenantId: string,
    warehouseId: string,
    data: UpdateWarehouseRequest,
  ) {
    return this.request<Warehouse>(
      "PUT",
      `/api/v1/tenants/${tenantId}/warehouses/${warehouseId}`,
      data,
    );
  }

  async deleteWarehouse(tenantId: string, warehouseId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/warehouses/${warehouseId}`,
    );
  }

  // Inventory - Stock Operations
  async adjustStock(tenantId: string, data: AdjustStockRequest) {
    return this.request<InventoryMovement>(
      "POST",
      `/api/v1/tenants/${tenantId}/inventory/adjust`,
      data,
    );
  }

  async transferStock(tenantId: string, data: TransferStockRequest) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/inventory/transfer`,
      data,
    );
  }

  async reserveStock(tenantId: string, data: StockReservationRequest) {
    return this.request<StockLevel>(
      "POST",
      `/api/v1/tenants/${tenantId}/inventory/reserve`,
      data,
    );
  }

  async releaseStock(tenantId: string, data: StockReservationRequest) {
    return this.request<StockLevel>(
      "POST",
      `/api/v1/tenants/${tenantId}/inventory/release`,
      data,
    );
  }

  async getInventoryValuation(
    tenantId: string,
    options: InventoryValuationOptions = {},
  ) {
    const params = new URLSearchParams();
    if (options.warehouse_id) params.set("warehouse_id", options.warehouse_id);
    if (options.method) params.set("method", options.method);
    const query = params.toString() ? `?${params.toString()}` : "";

    return this.request<InventoryValuationReport>(
      "GET",
      `/api/v1/tenants/${tenantId}/inventory/valuation${query}`,
    );
  }

  async getInventorySubledgerReconciliation(
    tenantId: string,
    options: InventorySubledgerReconciliationOptions = {},
  ) {
    const params = new URLSearchParams();
    if (options.warehouse_id) params.set("warehouse_id", options.warehouse_id);
    if (options.method) params.set("method", options.method);
    if (options.as_of_date) params.set("as_of_date", options.as_of_date);
    const query = params.toString() ? `?${params.toString()}` : "";

    return this.request<InventorySubledgerReconciliationReport>(
      "GET",
      `/api/v1/tenants/${tenantId}/inventory/subledger-reconciliation${query}`,
    );
  }

  // Analytics endpoints
  async getDashboardSummary(
    tenantId: string,
    startDate?: string,
    endDate?: string,
  ) {
    const query = buildQuery({ start_date: startDate, end_date: endDate });
    return this.request<DashboardSummary>(
      "GET",
      `/api/v1/tenants/${tenantId}/analytics/dashboard${query}`,
    );
  }

  async getRevenueExpenseChart(tenantId: string, months = 12) {
    return this.request<RevenueExpenseChart>(
      "GET",
      `/api/v1/tenants/${tenantId}/analytics/revenue-expense?months=${months}`,
    );
  }

  async getCashFlowChart(tenantId: string, months = 12) {
    return this.request<CashFlowChart>(
      "GET",
      `/api/v1/tenants/${tenantId}/analytics/cash-flow?months=${months}`,
    );
  }

  async getReceivablesAging(tenantId: string) {
    return this.request<AgingReport>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/aging/receivables`,
    );
  }

  async getPayablesAging(tenantId: string) {
    return this.request<AgingReport>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/aging/payables`,
    );
  }

  async getRecentActivity(tenantId: string, limit = 10) {
    return this.request<ActivityItem[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/analytics/activity?limit=${limit}`,
    );
  }

  async getCashFlowAnalytics(
    tenantId: string,
    startDate: string,
    endDate: string,
  ) {
    const query = buildQuery({ start_date: startDate, end_date: endDate });
    return this.request<CashFlowChart>(
      "GET",
      `/api/v1/tenants/${tenantId}/analytics/cash-flow${query}`,
    );
  }

  // Recurring Invoice endpoints
  async listRecurringInvoices(tenantId: string, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request<RecurringInvoice[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/recurring-invoices${query}`,
    );
  }

  async createRecurringInvoice(
    tenantId: string,
    data: CreateRecurringInvoiceRequest,
  ) {
    return this.request<RecurringInvoice>(
      "POST",
      `/api/v1/tenants/${tenantId}/recurring-invoices`,
      data,
    );
  }

  async createRecurringInvoiceFromInvoice(
    tenantId: string,
    invoiceId: string,
    data: CreateFromInvoiceRequest,
  ) {
    return this.request<RecurringInvoice>(
      "POST",
      `/api/v1/tenants/${tenantId}/recurring-invoices/from-invoice/${invoiceId}`,
      data,
    );
  }

  async getRecurringInvoice(tenantId: string, recurringId: string) {
    return this.request<RecurringInvoice>(
      "GET",
      `/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`,
    );
  }

  async updateRecurringInvoice(
    tenantId: string,
    recurringId: string,
    data: UpdateRecurringInvoiceRequest,
  ) {
    return this.request<RecurringInvoice>(
      "PUT",
      `/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`,
      data,
    );
  }

  async deleteRecurringInvoice(tenantId: string, recurringId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}`,
    );
  }

  async pauseRecurringInvoice(tenantId: string, recurringId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/pause`,
    );
  }

  async resumeRecurringInvoice(tenantId: string, recurringId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/resume`,
    );
  }

  async generateRecurringInvoice(tenantId: string, recurringId: string) {
    return this.request<GenerationResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/recurring-invoices/${recurringId}/generate`,
    );
  }

  async generateDueRecurringInvoices(tenantId: string) {
    return this.request<GenerationResult[]>(
      "POST",
      `/api/v1/tenants/${tenantId}/recurring-invoices/generate-due`,
    );
  }

  // Email endpoints
  async getSMTPConfig(tenantId: string) {
    return this.request<SMTPConfig>(
      "GET",
      `/api/v1/tenants/${tenantId}/settings/smtp`,
    );
  }

  async updateSMTPConfig(tenantId: string, data: UpdateSMTPConfigRequest) {
    return this.request<{ status: string }>(
      "PUT",
      `/api/v1/tenants/${tenantId}/settings/smtp`,
      data,
    );
  }

  async testSMTP(tenantId: string, recipientEmail: string) {
    return this.request<TestSMTPResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/settings/smtp/test`,
      { recipient_email: recipientEmail },
    );
  }

  async listEmailTemplates(tenantId: string) {
    return this.request<EmailTemplate[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/email-templates`,
    );
  }

  async updateEmailTemplate(
    tenantId: string,
    templateType: TemplateType,
    data: UpdateTemplateRequest,
  ) {
    return this.request<EmailTemplate>(
      "PUT",
      `/api/v1/tenants/${tenantId}/email-templates/${templateType}`,
      data,
    );
  }

  async getEmailLog(tenantId: string, limit = 50) {
    return this.request<EmailLog[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/email-log?limit=${limit}`,
    );
  }

  async emailInvoice(
    tenantId: string,
    invoiceId: string,
    data: SendInvoiceEmailRequest,
  ) {
    return this.request<EmailSentResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/email`,
      data,
    );
  }

  async emailPaymentReceipt(
    tenantId: string,
    paymentId: string,
    data: SendPaymentReceiptRequest,
  ) {
    return this.request<EmailSentResponse>(
      "POST",
      `/api/v1/tenants/${tenantId}/payments/${paymentId}/email-receipt`,
      data,
    );
  }

  // Reminder Rules (Automated Payment Reminders)
  async listReminderRules(tenantId: string) {
    return this.request<ReminderRule[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/reminder-rules`,
    );
  }

  async getReminderRule(tenantId: string, ruleId: string) {
    return this.request<ReminderRule>(
      "GET",
      `/api/v1/tenants/${tenantId}/reminder-rules/${ruleId}`,
    );
  }

  async createReminderRule(tenantId: string, data: CreateReminderRuleRequest) {
    return this.request<ReminderRule>(
      "POST",
      `/api/v1/tenants/${tenantId}/reminder-rules`,
      data,
    );
  }

  async updateReminderRule(
    tenantId: string,
    ruleId: string,
    data: UpdateReminderRuleRequest,
  ) {
    return this.request<ReminderRule>(
      "PUT",
      `/api/v1/tenants/${tenantId}/reminder-rules/${ruleId}`,
      data,
    );
  }

  async deleteReminderRule(tenantId: string, ruleId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/reminder-rules/${ruleId}`,
    );
  }

  async triggerReminders(tenantId: string) {
    return this.request<AutomatedReminderResult[]>(
      "POST",
      `/api/v1/tenants/${tenantId}/reminder-rules/trigger`,
    );
  }

  // Interest Calculations
  async getInterestSettings(tenantId: string) {
    return this.request<InterestSettings>(
      "GET",
      `/api/v1/tenants/${tenantId}/settings/interest`,
    );
  }

  async updateInterestSettings(
    tenantId: string,
    data: UpdateInterestSettingsRequest,
  ) {
    return this.request<InterestSettings>(
      "PUT",
      `/api/v1/tenants/${tenantId}/settings/interest`,
      data,
    );
  }

  async getInvoiceInterest(tenantId: string, invoiceId: string) {
    return this.request<InterestCalculationResult>(
      "GET",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/interest`,
    );
  }

  async getInvoiceInterestHistory(tenantId: string, invoiceId: string) {
    return this.request<InvoiceInterest[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/interest/history`,
    );
  }

  async getOverdueInvoicesWithInterest(tenantId: string) {
    return this.request<InterestCalculationResult[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/invoices/overdue-with-interest`,
    );
  }

  // Banking endpoints
  async listBankAccounts(tenantId: string, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request<BankAccount[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/bank-accounts${query}`,
    );
  }

  async createBankAccount(tenantId: string, data: CreateBankAccountRequest) {
    return this.request<BankAccount>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-accounts`,
      data,
    );
  }

  async getBankAccount(tenantId: string, accountId: string) {
    return this.request<BankAccount>(
      "GET",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`,
    );
  }

  async updateBankAccount(
    tenantId: string,
    accountId: string,
    data: UpdateBankAccountRequest,
  ) {
    return this.request<BankAccount>(
      "PUT",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`,
      data,
    );
  }

  async deleteBankAccount(tenantId: string, accountId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}`,
    );
  }

  async listBankTransactions(
    tenantId: string,
    accountId: string,
    filters?: {
      status?: TransactionStatus;
      from_date?: string;
      to_date?: string;
    },
  ) {
    const params = new URLSearchParams();
    if (filters?.status) params.set("status", filters.status);
    if (filters?.from_date) params.set("from_date", filters.from_date);
    if (filters?.to_date) params.set("to_date", filters.to_date);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request<BankTransaction[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/transactions${query}`,
    );
  }

  async getBankTransaction(tenantId: string, transactionId: string) {
    return this.request<BankTransaction>(
      "GET",
      `/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}`,
    );
  }

  async importBankTransactions(
    tenantId: string,
    accountId: string,
    data: ImportTransactionsRequest,
  ) {
    return this.request<ImportResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/import`,
      data,
    );
  }

  async getImportHistory(tenantId: string, accountId: string) {
    return this.request<BankStatementImport[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/import-history`,
    );
  }

  async getMatchSuggestions(tenantId: string, transactionId: string) {
    return this.request<MatchSuggestion[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/suggestions`,
    );
  }

  async matchBankTransaction(
    tenantId: string,
    transactionId: string,
    paymentId: string,
  ) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/match`,
      { payment_id: paymentId },
    );
  }

  async unmatchBankTransaction(tenantId: string, transactionId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/unmatch`,
    );
  }

  async reviewBankTransaction(
    tenantId: string,
    transactionId: string,
    data: UpdateBankTransactionReviewRequest,
  ) {
    return this.request<BankTransaction>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/review`,
      data,
    );
  }

  async createPaymentFromTransaction(tenantId: string, transactionId: string) {
    return this.request<{ payment_id: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-transactions/${transactionId}/create-payment`,
    );
  }

  async listReconciliations(tenantId: string, accountId: string) {
    return this.request<BankReconciliation[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/reconciliations`,
    );
  }

  async createReconciliation(
    tenantId: string,
    accountId: string,
    data: CreateReconciliationRequest,
  ) {
    return this.request<BankReconciliation>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/reconciliation`,
      data,
    );
  }

  async getReconciliation(tenantId: string, reconciliationId: string) {
    return this.request<BankReconciliation>(
      "GET",
      `/api/v1/tenants/${tenantId}/reconciliations/${reconciliationId}`,
    );
  }

  async completeReconciliation(tenantId: string, reconciliationId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/reconciliations/${reconciliationId}/complete`,
    );
  }

  async autoMatchTransactions(
    tenantId: string,
    accountId: string,
    minConfidence = 0.7,
  ) {
    return this.request<{ matched: number }>(
      "POST",
      `/api/v1/tenants/${tenantId}/bank-accounts/${accountId}/auto-match?min_confidence=${minConfidence}`,
    );
  }

  // Tax (KMD) endpoints
  async generateKMD(tenantId: string, data: CreateKMDRequest) {
    return this.request<KMDDeclaration>(
      "POST",
      `/api/v1/tenants/${tenantId}/tax/kmd`,
      data,
    );
  }

  async listKMD(tenantId: string) {
    return this.request<KMDDeclaration[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/tax/kmd`,
    );
  }

  async generateKMDINF(tenantId: string, data: KMDINFReportRequest) {
    const query = buildQuery({ threshold: data.threshold });
    return this.request<KMDINFReport>(
      "GET",
      `/api/v1/tenants/${tenantId}/tax/kmd/${data.year}/${data.month}/inf${query}`,
    );
  }

  async generateEUVATOSS(tenantId: string, data: EUVATOSSReportRequest) {
    const query = buildQuery({
      year: data.year,
      quarter: data.quarter,
      include_b2b: data.include_b2b,
    });
    return this.request<EUVATOSSReport>(
      "GET",
      `/api/v1/tenants/${tenantId}/tax/eu-vat/oss${query}`,
    );
  }

  async markKMDSubmitted(tenantId: string, year: number, month: number) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/tax/kmd/${year}/${month}/submit`,
    );
  }

  async markKMDAccepted(tenantId: string, year: number, month: number) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/tax/kmd/${year}/${month}/accept`,
    );
  }

  async downloadKMDXml(tenantId: string, year: number, month: number) {
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/tax/kmd/${year}/${month}/xml`,
      `KMD_${year}_${String(month).padStart(2, "0")}.xml`,
      "Failed to download XML",
    );
  }

  // Cash Flow Statement endpoint
  async getCashFlowStatement(
    tenantId: string,
    startDate: string,
    endDate: string,
  ) {
    return this.request<CashFlowStatement>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/cash-flow?start_date=${startDate}&end_date=${endDate}`,
    );
  }

  // Balance Confirmation endpoints
  async getBalanceConfirmationSummary(
    tenantId: string,
    type: BalanceConfirmationType,
    asOfDate: string,
  ) {
    return this.request<BalanceConfirmationSummary>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/balance-confirmations?type=${type}&as_of_date=${asOfDate}`,
    );
  }

  async getBalanceConfirmation(
    tenantId: string,
    contactId: string,
    type: BalanceConfirmationType,
    asOfDate: string,
  ) {
    return this.request<BalanceConfirmation>(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/balance-confirmations/${contactId}?type=${type}&as_of_date=${asOfDate}`,
    );
  }

  async downloadBalanceConfirmationSummary(
    tenantId: string,
    type: BalanceConfirmationType,
    asOfDate: string,
    format: ReportExportFormat,
  ) {
    const query = buildQuery({ type, as_of_date: asOfDate, format });
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/reports/balance-confirmations${query}`,
      `balance-confirmations-${type.toLowerCase()}-${asOfDate}.${format}`,
      `Failed to download balance confirmations ${format.toUpperCase()}`,
    );
  }

  async downloadBalanceConfirmation(
    tenantId: string,
    contactId: string,
    type: BalanceConfirmationType,
    asOfDate: string,
    format: ReportExportFormat,
  ) {
    const query = buildQuery({ type, as_of_date: asOfDate, format });
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/reports/balance-confirmations/${encodeURIComponent(contactId)}${query}`,
      `balance-confirmation-${contactId}-${asOfDate}.${format}`,
      `Failed to download balance confirmation ${format.toUpperCase()}`,
    );
  }

  // Payment Reminder endpoints
  async getOverdueInvoices(tenantId: string) {
    return this.request<OverdueInvoicesSummary>(
      "GET",
      `/api/v1/tenants/${tenantId}/invoices/overdue`,
    );
  }

  async sendPaymentReminder(
    tenantId: string,
    invoiceId: string,
    message?: string,
  ) {
    return this.request<ReminderResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/reminders`,
      {
        invoice_id: invoiceId,
        message,
      },
    );
  }

  async sendBulkPaymentReminders(
    tenantId: string,
    invoiceIds: string[],
    message?: string,
  ) {
    return this.request<BulkReminderResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/reminders/bulk`,
      {
        invoice_ids: invoiceIds,
        message,
      },
    );
  }

  async getInvoiceReminderHistory(tenantId: string, invoiceId: string) {
    return this.request<PaymentReminder[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/reminders`,
    );
  }

  // Cost Centers
  async listCostCenters(tenantId: string, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request<CostCenter[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/cost-centers${query}`,
    );
  }

  async getCostCenter(tenantId: string, costCenterId: string) {
    return this.request<CostCenter>(
      "GET",
      `/api/v1/tenants/${tenantId}/cost-centers/${costCenterId}`,
    );
  }

  async createCostCenter(tenantId: string, data: CreateCostCenterRequest) {
    return this.request<CostCenter>(
      "POST",
      `/api/v1/tenants/${tenantId}/cost-centers`,
      data,
    );
  }

  async updateCostCenter(
    tenantId: string,
    costCenterId: string,
    data: UpdateCostCenterRequest,
  ) {
    return this.request<CostCenter>(
      "PUT",
      `/api/v1/tenants/${tenantId}/cost-centers/${costCenterId}`,
      data,
    );
  }

  async deleteCostCenter(tenantId: string, costCenterId: string) {
    return this.request<void>(
      "DELETE",
      `/api/v1/tenants/${tenantId}/cost-centers/${costCenterId}`,
    );
  }

  async listCostAllocations(
    tenantId: string,
    filters: CostAllocationFilters = {},
  ) {
    return this.request<CostAllocation[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/cost-centers/allocations${buildQuery(filters)}`,
    );
  }

  async createCostAllocation(
    tenantId: string,
    data: CreateCostAllocationRequest,
  ) {
    return this.request<CostAllocation>(
      "POST",
      `/api/v1/tenants/${tenantId}/cost-centers/allocations`,
      data,
    );
  }

  async getCostCenterReport(
    tenantId: string,
    startDate?: string,
    endDate?: string,
  ) {
    const params = new URLSearchParams();
    if (startDate) params.append("start_date", startDate);
    if (endDate) params.append("end_date", endDate);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request<CostCenterReport>(
      "GET",
      `/api/v1/tenants/${tenantId}/cost-centers/report${query}`,
    );
  }

  // Payroll - Employee endpoints
  async listEmployees(tenantId: string, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request<Employee[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/employees${query}`,
    );
  }

  async createEmployee(tenantId: string, data: CreateEmployeeRequest) {
    return this.request<Employee>(
      "POST",
      `/api/v1/tenants/${tenantId}/employees`,
      data,
    );
  }

  async importEmployees(tenantId: string, data: ImportEmployeesRequest) {
    return this.request<ImportEmployeesResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/employees/import`,
      data,
    );
  }

  async importPayrollHistory(
    tenantId: string,
    data: ImportPayrollHistoryRequest,
  ) {
    return this.request<ImportPayrollHistoryResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/payroll-runs/import-history`,
      data,
    );
  }

  async importLeaveBalances(
    tenantId: string,
    data: ImportLeaveBalancesRequest,
  ) {
    return this.request<ImportLeaveBalancesResult>(
      "POST",
      `/api/v1/tenants/${tenantId}/leave-balances/import`,
      data,
    );
  }

  async getEmployee(tenantId: string, employeeId: string) {
    return this.request<Employee>(
      "GET",
      `/api/v1/tenants/${tenantId}/employees/${employeeId}`,
    );
  }

  async updateEmployee(
    tenantId: string,
    employeeId: string,
    data: UpdateEmployeeRequest,
  ) {
    return this.request<Employee>(
      "PUT",
      `/api/v1/tenants/${tenantId}/employees/${employeeId}`,
      data,
    );
  }

  async setBaseSalary(tenantId: string, employeeId: string, amount: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/employees/${employeeId}/salary`,
      { amount },
    );
  }

  // Payroll - Payroll Run endpoints
  async listPayrollRuns(tenantId: string, year?: number) {
    const query = year ? `?year=${year}` : "";
    return this.request<PayrollRun[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/payroll-runs${query}`,
    );
  }

  async createPayrollRun(tenantId: string, data: CreatePayrollRunRequest) {
    return this.request<PayrollRun>(
      "POST",
      `/api/v1/tenants/${tenantId}/payroll-runs`,
      data,
    );
  }

  async getPayrollRun(tenantId: string, runId: string) {
    return this.request<PayrollRun>(
      "GET",
      `/api/v1/tenants/${tenantId}/payroll-runs/${runId}`,
    );
  }

  async calculatePayroll(tenantId: string, runId: string) {
    return this.request<PayrollRun>(
      "POST",
      `/api/v1/tenants/${tenantId}/payroll-runs/${runId}/calculate`,
    );
  }

  async updatePayrollPaymentDate(
    tenantId: string,
    runId: string,
    data: UpdatePayrollRunPaymentDateRequest,
  ) {
    return this.request<PayrollRun>(
      "PATCH",
      `/api/v1/tenants/${tenantId}/payroll-runs/${runId}/payment-date`,
      data,
    );
  }

  async approvePayroll(tenantId: string, runId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/payroll-runs/${runId}/approve`,
    );
  }

  async getPayslips(tenantId: string, runId: string) {
    return this.request<Payslip[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/payroll-runs/${runId}/payslips`,
    );
  }

  async generateTSD(tenantId: string, runId: string) {
    return this.request<TSDDeclaration>(
      "POST",
      `/api/v1/tenants/${tenantId}/payroll-runs/${runId}/tsd`,
    );
  }

  // Payroll - Tax Preview
  async calculateTaxPreview(
    tenantId: string,
    grossSalary: string,
    applyBasicExemption?: boolean,
    basicExemptionAmount?: string,
    fundedPensionRate?: string,
  ) {
    return this.request<TaxCalculation>(
      "POST",
      `/api/v1/tenants/${tenantId}/payroll/tax-preview`,
      {
        gross_salary: grossSalary,
        apply_basic_exemption: applyBasicExemption,
        basic_exemption_amount: basicExemptionAmount,
        funded_pension_rate: fundedPensionRate,
      },
    );
  }

  // Payroll - TSD endpoints
  async listTSD(tenantId: string, year?: number) {
    const query = year ? `?year=${year}` : "";
    return this.request<TSDDeclaration[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/tsd${query}`,
    );
  }

  async getTSD(tenantId: string, year: number, month: number) {
    return this.request<TSDDeclaration>(
      "GET",
      `/api/v1/tenants/${tenantId}/tsd/${year}/${month}`,
    );
  }

  async downloadTSDXml(tenantId: string, year: number, month: number) {
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/tsd/${year}/${month}/xml`,
      `TSD_${year}_${String(month).padStart(2, "0")}.xml`,
      "Failed to download TSD XML",
    );
  }

  async downloadTSDCsv(tenantId: string, year: number, month: number) {
    return this.downloadFile(
      `/api/v1/tenants/${tenantId}/tsd/${year}/${month}/csv`,
      `TSD_${year}_${String(month).padStart(2, "0")}.csv`,
      "Failed to download TSD CSV",
    );
  }

  async markTSDSubmitted(
    tenantId: string,
    year: number,
    month: number,
    emtaReference: string,
  ) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/tsd/${year}/${month}/submit`,
      { emta_reference: emtaReference },
    );
  }

  async markTSDAccepted(tenantId: string, year: number, month: number) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/tsd/${year}/${month}/accept`,
    );
  }

  // Leave/Absence Management
  async listAbsenceTypes(tenantId: string, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request<AbsenceType[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/absence-types${query}`,
    );
  }

  async getAbsenceType(tenantId: string, typeId: string) {
    return this.request<AbsenceType>(
      "GET",
      `/api/v1/tenants/${tenantId}/absence-types/${typeId}`,
    );
  }

  async listLeaveBalances(tenantId: string, employeeId: string, year?: number) {
    const query = year ? `?year=${year}` : "";
    return this.request<LeaveBalance[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances${query}`,
    );
  }

  async getLeaveBalancesByYear(
    tenantId: string,
    employeeId: string,
    year: number,
  ) {
    return this.request<LeaveBalance[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances/${year}`,
    );
  }

  async updateLeaveBalance(
    tenantId: string,
    employeeId: string,
    year: number,
    typeId: string,
    data: UpdateLeaveBalanceRequest,
  ) {
    return this.request<LeaveBalance>(
      "PUT",
      `/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances/${year}/${typeId}`,
      data,
    );
  }

  async initializeLeaveBalances(
    tenantId: string,
    employeeId: string,
    year: number,
  ) {
    return this.request<LeaveBalance[]>(
      "POST",
      `/api/v1/tenants/${tenantId}/employees/${employeeId}/leave-balances/${year}/initialize`,
    );
  }

  async listLeaveRecords(tenantId: string, employeeId?: string, year?: number) {
    const params = new URLSearchParams();
    if (employeeId) params.append("employee_id", employeeId);
    if (year) params.append("year", year.toString());
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request<LeaveRecord[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/leave-records${query}`,
    );
  }

  async createLeaveRecord(tenantId: string, data: CreateLeaveRecordRequest) {
    return this.request<LeaveRecord>(
      "POST",
      `/api/v1/tenants/${tenantId}/leave-records`,
      data,
    );
  }

  async getLeaveRecord(tenantId: string, recordId: string) {
    return this.request<LeaveRecord>(
      "GET",
      `/api/v1/tenants/${tenantId}/leave-records/${recordId}`,
    );
  }

  async approveLeaveRecord(tenantId: string, recordId: string) {
    return this.request<LeaveRecord>(
      "POST",
      `/api/v1/tenants/${tenantId}/leave-records/${recordId}/approve`,
    );
  }

  async rejectLeaveRecord(tenantId: string, recordId: string, reason: string) {
    return this.request<LeaveRecord>(
      "POST",
      `/api/v1/tenants/${tenantId}/leave-records/${recordId}/reject`,
      { reason },
    );
  }

  async cancelLeaveRecord(tenantId: string, recordId: string) {
    return this.request<LeaveRecord>(
      "POST",
      `/api/v1/tenants/${tenantId}/leave-records/${recordId}/cancel`,
    );
  }

  // Plugin Registries (Admin)
  async listPluginRegistries() {
    return this.request<PluginRegistry[]>(
      "GET",
      "/api/v1/admin/plugin-registries",
    );
  }

  async addPluginRegistry(name: string, url: string, description?: string) {
    return this.request<PluginRegistry>(
      "POST",
      "/api/v1/admin/plugin-registries",
      {
        name,
        url,
        description,
      },
    );
  }

  async removePluginRegistry(registryId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/admin/plugin-registries/${registryId}`,
    );
  }

  async syncPluginRegistry(registryId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/admin/plugin-registries/${registryId}/sync`,
    );
  }

  // Plugins (Admin - Instance Level)
  async listPlugins() {
    return this.request<Plugin[]>("GET", "/api/v1/admin/plugins");
  }

  async searchPlugins(query: string) {
    return this.request<PluginSearchResult[]>(
      "GET",
      `/api/v1/admin/plugins/search?q=${encodeURIComponent(query)}`,
    );
  }

  async getPluginPermissions() {
    return this.request<Record<string, PluginPermission>>(
      "GET",
      "/api/v1/admin/plugins/permissions",
    );
  }

  async installPlugin(repositoryUrl: string) {
    return this.request<Plugin>("POST", "/api/v1/admin/plugins/install", {
      repository_url: repositoryUrl,
    });
  }

  async getPlugin(pluginId: string) {
    return this.request<Plugin>("GET", `/api/v1/admin/plugins/${pluginId}`);
  }

  async uninstallPlugin(pluginId: string) {
    return this.request<{ status: string }>(
      "DELETE",
      `/api/v1/admin/plugins/${pluginId}`,
    );
  }

  async enablePlugin(pluginId: string, permissions: string[]) {
    return this.request<Plugin>(
      "POST",
      `/api/v1/admin/plugins/${pluginId}/enable`,
      {
        granted_permissions: permissions,
      },
    );
  }

  async disablePlugin(pluginId: string) {
    return this.request<Plugin>(
      "POST",
      `/api/v1/admin/plugins/${pluginId}/disable`,
    );
  }

  // Tenant Plugin Management
  async listTenantPlugins(tenantId: string) {
    return this.request<TenantPlugin[]>(
      "GET",
      `/api/v1/tenants/${tenantId}/plugins`,
    );
  }

  async enableTenantPlugin(
    tenantId: string,
    pluginId: string,
    settings?: Record<string, unknown>,
  ) {
    return this.request<TenantPlugin>(
      "POST",
      `/api/v1/tenants/${tenantId}/plugins/${pluginId}/enable`,
      settings ? { settings } : undefined,
    );
  }

  async disableTenantPlugin(tenantId: string, pluginId: string) {
    return this.request<{ status: string }>(
      "POST",
      `/api/v1/tenants/${tenantId}/plugins/${pluginId}/disable`,
    );
  }

  async getTenantPluginSettings(tenantId: string, pluginId: string) {
    return this.request<TenantPluginSettings>(
      "GET",
      `/api/v1/tenants/${tenantId}/plugins/${pluginId}/settings`,
    );
  }

  async updateTenantPluginSettings(
    tenantId: string,
    pluginId: string,
    settings: Record<string, unknown>,
  ) {
    return this.request<TenantPluginSettings>(
      "PUT",
      `/api/v1/tenants/${tenantId}/plugins/${pluginId}/settings`,
      { settings },
    );
  }
}

// Types
export interface Tenant {
  id: string;
  name: string;
  slug: string;
  schema_name: string;
  settings: TenantSettings;
  is_active: boolean;
  onboarding_completed: boolean;
  created_at: string;
  updated_at: string;
}

export interface TenantSettings {
  default_currency: string;
  country_code: string;
  timezone: string;
  date_format: string;
  decimal_sep: string;
  thousands_sep: string;
  fiscal_year_start_month: number;
  period_lock_date?: string | null;
  vat_number?: string;
  reg_code?: string;
  address?: string;
  email?: string;
  phone?: string;
  logo?: string;
  pdf_primary_color?: string;
  pdf_footer_text?: string;
  bank_details?: string;
  invoice_terms?: string;
  inventory_issue_costing_method?: InventoryIssueCostingMethod;
  inventory_valuation_method?: InventoryValuationMethod;
  /** Pilot control: require approved evidence before high-risk accounting actions. */
  evidence_policy_mode?: 'warn' | 'block_high_risk';
}

export interface PeriodCloseEvent {
  id: string;
  tenant_id: string;
  action: "close" | "reopen";
  close_kind: "month_end" | "year_end";
  period_end_date: string;
  lock_date_before?: string | null;
  lock_date_after?: string | null;
  note?: string;
  performed_by: string;
  created_at: string;
}

export type TenantAuditAction =
  | "user_role_updated"
  | "user_removed"
  | "invitation_created"
  | "invitation_revoked"
  | "tenant_updated"
  | "user_session_revoked"
  | "user_sessions_revoked"
  | "user_api_token_revoked"
  | "user_status_updated";

export type TenantAuditTargetType = "user" | "invitation" | "tenant";

export interface TenantAuditEvent {
  id: string;
  tenant_id: string;
  actor_user_id?: string;
  action: TenantAuditAction;
  target_type: TenantAuditTargetType;
  target_id: string;
  target_email?: string;
  metadata?: Record<string, string>;
  created_at: string;
}

export type TenantRole = "owner" | "admin" | "accountant" | "viewer";

export type EditableTenantRole = Exclude<TenantRole, "owner">;

export interface TenantUser {
  tenant_id: string;
  user_id: string;
  role: TenantRole;
  is_default: boolean;
  is_active: boolean;
  created_at: string;
}

export interface RefreshSession {
  id: string;
  user_id: string;
  created_at: string;
  last_used_at?: string;
  expires_at: string;
  revoked_at?: string;
}

export interface APIToken {
  id: string;
  tenant_id: string;
  user_id: string;
  name: string;
  token_prefix: string;
  last_used_at?: string;
  expires_at?: string;
  revoked_at?: string;
  created_at: string;
}

export interface SecurityAuditEvent {
  id: string;
  actor_user_id?: string;
  actor_email?: string;
  action: string;
  target_user_id?: string;
  target_email?: string;
  request_ip?: string;
  user_agent?: string;
  metadata?: Record<string, string>;
  created_at: string;
}

export interface UserInvitation {
  id: string;
  tenant_id: string;
  tenant_name?: string;
  email: string;
  role: EditableTenantRole;
  invited_by: string;
  expires_at: string;
  accepted_at?: string;
  created_at: string;
}

export interface CreateInvitationRequest {
  email: string;
  role: EditableTenantRole;
}

export interface ClosePeriodRequest {
  period_end_date: string;
  note?: string;
  reviewer_sign_off?: boolean;
  inventory_valuation_method?: string;
}

export interface ReopenPeriodRequest {
  period_end_date: string;
  note: string;
}

export interface PeriodCloseResponse {
  tenant: Tenant;
  event: PeriodCloseEvent;
}

export interface AccountSummary {
  id: string;
  code: string;
  name: string;
}

export interface JournalEntrySummary {
  id: string;
  entry_number: string;
  entry_date: string;
  description: string;
  reference?: string;
  status: "DRAFT" | "POSTED" | "VOIDED";
}

export interface YearEndCloseStatus {
  period_end_date: string;
  fiscal_year_label: string;
  fiscal_year_start_date: string;
  fiscal_year_end_date: string;
  carry_forward_date: string;
  locked_through_date?: string | null;
  is_fiscal_year_end: boolean;
  period_closed: boolean;
  has_profit_and_loss_activity: boolean;
  carry_forward_needed: boolean;
  carry_forward_ready: boolean;
  has_retained_earnings_account: boolean;
  retained_earnings_account?: AccountSummary | null;
  net_income: Decimal;
  existing_carry_forward?: JournalEntrySummary | null;
  close_pack_evidence_entity_id?: string;
  close_pack_evidence?: EvidencePolicyResult | null;
  inventory_costing_review?: YearEndInventoryCostingReview | null;
  remediation_actions?: YearEndCloseRemediationAction[];
}

export interface YearEndInventoryCostingReview {
  valuation_method: string;
  line_count: number;
  total_quantity: Decimal;
  total_reserved: Decimal;
  total_available: Decimal;
  total_value: Decimal;
  negative_quantity_line_count: number;
  negative_available_line_count: number;
  negative_value_line_count: number;
  missing_cost_line_count: number;
  blocking_exception_line_count: number;
  ready: boolean;
  generated_at: string;
}

export interface YearEndCloseRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  entity_type?: string;
  entity_id?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface CreateYearEndCarryForwardRequest {
  period_end_date: string;
  inventory_valuation_method?: string;
}

export interface YearEndCarryForwardResult {
  journal_entry: JournalEntry;
  status: YearEndCloseStatus;
}

export interface ReverseYearEndCarryForwardRequest {
  period_end_date: string;
  reason: string;
}

export interface YearEndCarryForwardReversalResult {
  reversal_journal_entry: JournalEntry;
  status: YearEndCloseStatus;
}

export interface DocumentAttachment {
  id: string;
  tenant_id: string;
  entity_type:
    | "invoice"
    | "journal_entry"
    | "payment"
    | "bank_transaction"
    | "asset"
    | "expense"
    | "quote"
    | "order"
    | "leave_record"
    | "year_end_close"
    | "tsd_declaration"
    | "kmd_declaration";
  entity_id: string;
  document_type:
    | "supporting_document"
    | "receipt"
    | "reconciliation_evidence"
    | "contract"
    | "asset_record"
    | "tax_support"
    | "close_pack"
    | "other";
  file_name: string;
  content_type: string;
  file_size: number;
  notes?: string;
  retention_until?: string;
  review_status: "PENDING" | "REVIEWED" | "APPROVED" | "REJECTED";
  review_note?: string;
  reviewed_by?: string;
  reviewed_at?: string;
  lifecycle_status?: "ACTIVE" | "SUPERSEDED" | "ARCHIVED" | "DISPOSED";
  lifecycle_note?: string;
  superseded_by_document_id?: string;
  lifecycle_actioned_by?: string;
  lifecycle_actioned_at?: string;
  uploaded_by: string;
  created_at: string;
}

export interface ReviewDocumentRequest {
  review_status: "REVIEWED" | "APPROVED" | "REJECTED";
  review_note?: string;
}

export interface DocumentReviewSummary {
  entity_type: DocumentAttachment["entity_type"];
  entity_id: string;
  total_count: number;
  pending_review_count: number;
  reviewed_count: number;
  approved_count: number;
  rejected_count: number;
  missing_evidence: boolean;
  has_pending_review: boolean;
  has_rejected: boolean;
}

export type DocumentReviewStatusFilter =
  | DocumentAttachment["review_status"]
  | "ALL";

export interface DocumentReviewQueueFilter {
  entity_type?: DocumentAttachment["entity_type"] | "";
  document_type?: DocumentAttachment["document_type"] | "";
  review_status?: DocumentReviewStatusFilter;
  limit?: number;
}

export interface DocumentReviewQueue {
  entity_type?: DocumentAttachment["entity_type"];
  document_type?: DocumentAttachment["document_type"];
  review_status: DocumentReviewStatusFilter;
  limit: number;
  total_count: number;
  pending_review_count: number;
  reviewed_count: number;
  approved_count: number;
  rejected_count: number;
  documents: DocumentAttachment[];
}

export interface DocumentRetentionReviewFilter {
  as_of?: string;
  horizon_days?: number;
  include_missing?: boolean;
}

export interface DocumentRetentionUpdateRequest {
  retention_until?: string;
  clear_retention?: boolean;
}

export interface RetentionReminderAction {
  document_id: string;
  entity_type: DocumentAttachment["entity_type"];
  entity_id: string;
  document_type: DocumentAttachment["document_type"];
  file_name: string;
  action: string;
  message: string;
  days_until_retention?: number;
  retention_until?: string;
}

export interface DocumentRetentionReview {
  as_of_date: string;
  cutoff_date: string;
  total_count: number;
  expired_count: number;
  due_soon_count: number;
  missing_retention_count: number;
  pending_review_count: number;
  rejected_count: number;
  documents: DocumentAttachment[];
  reminder_actions?: RetentionReminderAction[];
  remediation_actions?: DocumentRemediationAction[];
}

export interface DocumentRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  entity_type?: DocumentAttachment["entity_type"];
  entity_id?: string;
  document_id?: string;
  document_type?: DocumentAttachment["document_type"];
  file_name?: string;
  due_date?: string;
  days_until_retention?: number;
  ui_path?: string;
  cli_command?: string;
}

export interface EvidencePolicyRule {
  document_types?: DocumentAttachment["document_type"][];
  min_count: number;
  require_approved?: boolean;
}

export interface EvidencePolicyRequest {
  entity_type: DocumentAttachment["entity_type"];
  entity_ids: string[];
  rules: EvidencePolicyRule[];
}

export interface EvidencePolicyRuleResult {
  rule_index: number;
  document_types?: DocumentAttachment["document_type"][];
  required_count: number;
  matching_count: number;
  approved_matching_count: number;
  accepted_count: number;
  require_approved: boolean;
  compliant: boolean;
  message?: string;
}

export interface EvidencePolicyResult {
  entity_type: DocumentAttachment["entity_type"];
  entity_id: string;
  compliant: boolean;
  total_count: number;
  pending_review_count: number;
  reviewed_count: number;
  approved_count: number;
  rejected_count: number;
  missing_evidence: boolean;
  document_type_counts?: Partial<
    Record<DocumentAttachment["document_type"], number>
  >;
  approved_document_type_counts?: Partial<
    Record<DocumentAttachment["document_type"], number>
  >;
  rule_results?: EvidencePolicyRuleResult[];
  violations?: EvidencePolicyRuleResult[];
  remediation_actions?: DocumentRemediationAction[];
}

export type MigrationFileKind =
  | "accounts"
  | "contacts"
  | "employees"
  | "expenses"
  | "invoices"
  | "e_invoices"
  | "payments"
  | "bank_accounts"
  | "bank_transactions"
  | "payroll_history"
  | "leave_balances"
  | "tsd_history"
  | "kmd_history"
  | "quotes"
  | "orders"
  | "recurring_invoices"
  | "cost_centers"
  | "cost_allocations"
  | "product_categories"
  | "warehouses"
  | "products"
  | "stock_adjustments"
  | "fixed_assets"
  | "opening_balances"
  | "journal_entries";

export type MigrationProviderPreset =
  | "generic"
  | "merit"
  | "smartaccounts"
  | "directo";
export type EInvoiceContactMode = "supplier" | "customer" | "both";
export type MigrationIssueSeverity = "ERROR" | "WARNING";

export interface MigrationProviderPresetAlias {
  source_header: string;
  canonical_header: string;
}

export interface MigrationProviderPresetKindInfo {
  kind: MigrationFileKind;
  required_column_groups?: string[][];
  preset_alias_count: number;
  sample_aliases?: MigrationProviderPresetAlias[];
}

export interface MigrationProviderPresetInfo {
  preset: MigrationProviderPreset;
  label: string;
  description: string;
  file_kind_count: number;
  preset_alias_count: number;
  file_kinds?: MigrationProviderPresetKindInfo[];
}

export interface BundleFile {
  kind: MigrationFileKind;
  file_name: string;
  csv_content?: string;
  xml_content?: string;
}

export interface ValidateBundleRequest {
  files: BundleFile[];
  e_invoice_contact_mode?: EInvoiceContactMode;
  e_invoice_invoice_type?: string;
  provider_preset?: MigrationProviderPreset;
}

export interface PlanMigrationExecutionRequest extends ValidateBundleRequest {
  bank_transaction_account_id?: string;
  opening_balance_entry_date?: string;
}

export interface ExecuteMigrationRequest extends PlanMigrationExecutionRequest {
  bank_transaction_format?: string;
  confirm?: boolean;
  resume_from_run?: MigrationExecutionRun;
  resume_from_run_id?: string;
}

export interface MigrationExecutionRunFilter {
  status?: string;
  limit?: number;
}

export interface BundleValidationSummary {
  files_validated: number;
  rows_validated: number;
  error_count: number;
  warning_count: number;
  ready: boolean;
}

export interface FileValidation {
  kind: MigrationFileKind;
  file_name: string;
  rows: number;
  headers?: string[];
  missing_columns?: string[];
}

export interface ValidationIssue {
  severity: MigrationIssueSeverity;
  kind: MigrationFileKind;
  file_name: string;
  row?: number;
  field?: string;
  value?: string;
  target_kind?: MigrationFileKind;
  message: string;
}

export interface MigrationRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  kind?: MigrationFileKind;
  file_name?: string;
  field?: string;
  target_kind?: MigrationFileKind;
  issue_count: number;
  entity_type?: string;
  entity_id?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface BundleValidationReport {
  summary: BundleValidationSummary;
  files: FileValidation[];
  issues?: ValidationIssue[];
  remediation_actions?: MigrationRemediationAction[];
}

export type MigrationExecutionStepStatus =
  | "READY"
  | "NEEDS_CONTEXT"
  | "BLOCKED";

export interface MigrationExecutionPlanSummary {
  validation_ready: boolean;
  ready: boolean;
  step_count: number;
  ready_step_count: number;
  needs_context_count: number;
  blocked_step_count: number;
}

export interface MigrationExecutionStep {
  step_number: number;
  kind: MigrationFileKind;
  file_name: string;
  status: MigrationExecutionStepStatus;
  message: string;
  action: string;
  api_method?: string;
  api_path?: string;
  cli_command?: string;
  depends_on?: MigrationFileKind[];
  context_fields?: string[];
}

export interface MigrationExecutionPlan {
  summary: MigrationExecutionPlanSummary;
  validation: BundleValidationReport;
  steps?: MigrationExecutionStep[];
  remediation_actions?: MigrationRemediationAction[];
}

export type MigrationExecutionResultStatus =
  | "PLANNED"
  | "RUNNING"
  | "SKIPPED"
  | "SUCCEEDED"
  | "FAILED";

export interface MigrationExecutionRunSummary {
  status: string;
  confirmed: boolean;
  resumed: boolean;
  plan_ready: boolean;
  validation_ready: boolean;
  step_count: number;
  running_step_count: number;
  succeeded_step_count: number;
  failed_step_count: number;
  skipped_step_count: number;
  planned_step_count: number;
  resumed_step_count: number;
  completed_step_count: number;
  remaining_step_count: number;
  progress_percent: number;
  duration_ms?: number;
  needs_context_count: number;
  blocked_step_count: number;
  active_step_number?: number;
  active_step_kind?: MigrationFileKind;
  active_step_file_name?: string;
  active_step_status?: MigrationExecutionResultStatus;
  active_step_started_at?: string;
  active_step_completed_at?: string;
  active_step_duration_ms?: number;
}

export interface MigrationExecutionStepRun {
  step_number: number;
  kind: MigrationFileKind;
  file_name: string;
  status: MigrationExecutionResultStatus;
  message?: string;
  error?: string;
  api_path?: string;
  cli_command?: string;
  response?: unknown;
  started_at?: string;
  completed_at?: string;
  duration_ms?: number;
}

export interface MigrationExecutionRun {
  id?: string;
  tenant_id?: string;
  created_by?: string;
  created_at?: string;
  updated_at?: string;
  summary: MigrationExecutionRunSummary;
  plan?: MigrationExecutionPlan;
  steps?: MigrationExecutionStepRun[];
  remediation_actions?: MigrationRemediationAction[];
}

export interface MigrationExecutionRunEvent {
  type: string;
  sequence: number;
  run?: MigrationExecutionRun;
}

export interface WatchMigrationExecutionRunOptions {
  intervalMs?: number;
  maxEvents?: number;
  signal?: AbortSignal;
  onEvent: (event: MigrationExecutionRunEvent) => void | Promise<void>;
}

export type ExpenseStatus =
  | "DRAFT"
  | "SUBMITTED"
  | "APPROVED"
  | "REJECTED"
  | "POSTED";

export interface ExpenseClaim {
  id: string;
  tenant_id: string;
  expense_number: string;
  expense_date: string;
  merchant: string;
  description?: string;
  employee_id?: string;
  contact_id?: string;
  expense_account_id: string;
  payment_account_id: string;
  amount: Decimal;
  currency: string;
  exchange_rate: Decimal;
  base_amount: Decimal;
  requires_receipt: boolean;
  status: ExpenseStatus;
  journal_entry_id?: string;
  remediation_actions?: ExpenseRemediationAction[];
  submitted_at?: string;
  submitted_by?: string;
  approved_at?: string;
  approved_by?: string;
  rejected_at?: string;
  rejected_by?: string;
  rejection_reason?: string;
  posted_at?: string;
  posted_by?: string;
  created_at: string;
  created_by: string;
  updated_at: string;
}

export interface ExpenseRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  entity_type?: string;
  entity_id?: string;
  expense_number?: string;
  status?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface ListExpensesFilter {
  status?: ExpenseStatus | "";
  limit?: number;
}

export interface TenantMembership {
  tenant: Tenant;
  role: string;
  is_default: boolean;
}

export interface SmartAccountsSyncSource {
  provider: "smartaccounts";
  source_company_id: string;
  source_company_name: string;
  default: boolean;
  bridge_verified: boolean;
  general_ledger_authoritative: boolean;
  invoice_payment_mode: "NON_POSTING";
}

export interface SmartAccountsSourceDiscovery {
  bridge_available: boolean;
  live_data_contacted: boolean;
  sources: SmartAccountsSyncSource[];
}

export interface SmartAccountsBrowserPairingIssue {
  pairing_id: string;
  pairing_token: string;
  expires_at: string;
}

export interface SmartAccountsBrowserPairingStatus {
  pairing_id: string;
  status: "ISSUED" | "CLAIMED";
  expires_at: string;
  claimed_at?: string;
  source_company_id?: string;
}

export interface SmartAccountsBrowserDiscoveryStartRequest {
  source_company_id: string;
  metadata_only_consent_confirmed: boolean;
  response_header_probe_confirmed: boolean;
}

export interface SmartAccountsBrowserDiscoveryConsent {
  version: 1;
  confirmed: true;
  confirmed_at: string;
  scope: "metadata_only" | "metadata_and_header_probe";
  response_header_probe_confirmed?: true;
}

// This issue contains a tenant/source binding and consent only. It is posted
// directly to extension memory, not persisted in UI storage, and has no
// capability, credentials, cookies, source rows, export bytes, or accounting
// instruction.
export interface SmartAccountsBrowserDiscoveryIssue {
  discovery_id: string;
  tenant_id: string;
  source_company_id: string;
  manifest_version: "smartaccounts-brave-ui-v2";
  resource_ids: string[];
  expires_at: string;
  discovery_consent: SmartAccountsBrowserDiscoveryConsent;
}

export interface SmartAccountsBrowserDiscoveryRelayResult {
  source: "smartaccounts-browser-relay";
  type: "smartaccounts-browser-relay.discovery-result.v1";
  version: 1;
  discovery_id: string;
  manifest_version: "smartaccounts-brave-ui-v2";
  contract_version: "smartaccounts-brave-discovery-contract-v1";
  status: "completed" | "awaiting_browser" | "company_binding_blocked" | "expired" | "discovery_failed";
  resources: SmartAccountsBrowserDiscoveryResource[];
}

export interface SmartAccountsBrowserDiscoveryResource {
  resource_id: string;
  capture_status: "capture_ready" | "filter_contract_required" | "page_only_contract_required" | "private_endpoint_required" | "session_blocked" | "company_binding_blocked" | "page_binding_blocked";
  binding: { session: "verified" | "blocked"; company: "verified" | "blocked"; page: "verified" | "blocked" };
  contract: {
    version: "smartaccounts-brave-discovery-contract-v1";
    page_path: string;
    request?: { method: "GET"; path: string } | null;
    filter?: { method: "POST"; path: string; control_ids: string[] } | null;
    pagination: { kind: "unobserved" | "visible_control_ids"; control_ids: string[] };
    response: { observation: "unobserved" | "head" | "range_header"; content_type: "unobserved" | "text/csv"; header_names: string[] };
  };
}

// This is the only OA browser-discovery state rendered or persisted: a digest
// and aggregate counts. It intentionally omits source selector, resource
// contract/control IDs, header names, cookies, source values, and tokens.
export interface SmartAccountsBrowserDiscoveryReceipt {
  discovery_id: string;
  status: "completed" | "awaiting_browser" | "company_binding_blocked" | "expired" | "discovery_failed";
  manifest_version: "smartaccounts-brave-ui-v2";
  contract_version: "smartaccounts-brave-discovery-contract-v1";
  contract_sha256: string;
  resource_count: number;
  capture_ready_count: number;
  filter_contract_required_count: number;
  page_only_contract_required_count: number;
  private_endpoint_required_count: number;
  binding_blocked_count: number;
}

// This public result is deliberately aggregate-only. OA keeps the audit
// assertion private and the source/header evidence remains bridge-owned.
export interface SmartAccountsBrowserCSVSchemaApprovalResponse {
  resource_id: string;
  schema_id: string;
  status: "registered";
  approval_sha256: string;
}

export interface SmartAccountsBrowserOnboardingSource {
  source_company_id: string;
  source_company_name: string;
}

export interface SmartAccountsBrowserOnboardingRequest {
  sources: SmartAccountsBrowserOnboardingSource[];
  create_missing_tenants_confirmed: boolean;
}

export type SmartAccountsBrowserOnboardingStatus =
  | "TARGET_READY"
  | "PAIRING_ISSUED"
  | "PAIRED"
  | "REVIEW_REQUIRED"
  | "FAILED";

export interface SmartAccountsBrowserOnboardingResult {
  source_company_id: string;
  source_company_name: string;
  tenant_id?: string;
  tenant_name?: string;
  pairing_id?: string;
  status: SmartAccountsBrowserOnboardingStatus;
  tenant_created: boolean;
  tenant_reused: boolean;
  reason_code?: string;
  pairing?: SmartAccountsBrowserPairingIssue;
}

export interface SmartAccountsBrowserOnboardingResponse {
  bindings: SmartAccountsBrowserOnboardingResult[];
}

export interface SmartAccountsBrowserOnboardingCatalogConsent {
  version: 1;
  confirmed: boolean;
  confirmed_at: string;
  scope: "visible_company_catalog";
}

export interface SmartAccountsBrowserOnboardingCatalogIssueRequest {
  catalog_consent: SmartAccountsBrowserOnboardingCatalogConsent;
}

export interface SmartAccountsBrowserOnboardingCatalogDigestIntent {
  version: "smartaccounts-browser-source-catalog-intent-v1";
  catalog_schema_version: "smartaccounts-browser-source-catalog-v1";
  source_id_version: "sa-browser-v1";
  digest_algorithm: "sha256";
}

export interface SmartAccountsBrowserOnboardingCatalogIssue {
  catalog_id: string;
  workflow_id: string;
  catalog_token: string;
  nonce: string;
  issued_at: string;
  expires_at: string;
  catalog_digest_intent: SmartAccountsBrowserOnboardingCatalogDigestIntent;
  catalog_consent: SmartAccountsBrowserOnboardingCatalogConsent;
}

export interface SmartAccountsBrowserOnboardingCatalogCompany {
  source_company_id: string;
  display_name: string;
}

export interface SmartAccountsBrowserOnboardingCatalogStatus {
  catalog_id: string;
  workflow_id: string;
  status: "ACCEPTED";
  catalog_sha256: string;
  catalog_count: number;
  observed_at: string;
  expires_at: string;
  companies: SmartAccountsBrowserOnboardingCatalogCompany[];
}

export type SmartAccountsBrowserOnboardingBatchMode = "selected" | "all";

export interface SmartAccountsBrowserOnboardingBatchRequest {
  catalog_receipt_id: string;
  mode: SmartAccountsBrowserOnboardingBatchMode;
  selected_source_ids: string[];
  owner_confirmed: boolean;
}

export interface SmartAccountsBrowserOnboardingBatch {
  batch_id: string;
  catalog_receipt_id: string;
  relay_observed_at: string;
  mode: SmartAccountsBrowserOnboardingBatchMode;
  selected_sources: SmartAccountsBrowserOnboardingSource[];
  observed_source_ids: string[];
  observed_sources_sha256: string;
  manifest_sha256: string;
  status: "PENDING" | "REVIEW_REQUIRED" | "READY" | "COMPLETE";
  created_at: string;
  updated_at: string;
}

export interface SmartAccountsBrowserOnboardingBatchOutcome extends SmartAccountsBrowserOnboardingResult {}

export interface SmartAccountsBrowserOnboardingBatchPairingIssue {
  batch_id: string;
  source_company_id: string;
  tenant_id: string;
  pairing: SmartAccountsBrowserPairingIssue;
}

export interface SmartAccountsBrowserOnboardingBatchResponse {
  batch: SmartAccountsBrowserOnboardingBatch;
  outcomes: SmartAccountsBrowserOnboardingBatchOutcome[];
  pairing_issues?: SmartAccountsBrowserOnboardingBatchPairingIssue[];
  reused: boolean;
}

// Safe 082 control-plane state. These fields deliberately exclude browser
// capabilities, credentials, raw exports, header values, and accounting-write
// instructions. The UI must obtain each next action from the owner API rather
// than infer a source transfer locally.
export type SmartAccountsBrowserBatchWorkflowPhase =
  | "PAIRED"
  | "DISCOVERY_REQUIRED"
  | "DISCOVERY_RUNNING"
  | "DISCOVERY_COMPLETE"
  | "SCHEMA_REVIEW_REQUIRED"
  | "SCHEMA_APPROVED"
  | "TRANSFER_CONFIRMATION_REQUIRED"
  | "CAPTURE_RUNNING"
  | "STAGED"
  | "PREVIEW_READY"
  | "REVIEW_REQUIRED"
  | "FAILED_RETRYABLE"
  | "BLOCKED";

export interface SmartAccountsBrowserBatchWorkflowTransferScope {
  mode: "partial";
  from_inclusive: string;
  to_inclusive: string;
  cutoff_at: string;
  resource_ids: ["general_ledger"];
}

export interface SmartAccountsBrowserBatchWorkflow {
  batch_id: string;
  schema_version: "smartaccounts-browser-batch-workflow-v1";
  history_from: string;
	// A bounded CSV header probe is optional and frozen at preparation. False
	// means metadata discovery uses no Range/header extraction.
	header_probe_consent_confirmed: boolean;
  preparatory_manifest_sha256: string;
  preparatory_consented_at: string;
  transfer_manifest_sha256?: string;
  transfer_scope?: SmartAccountsBrowserBatchWorkflowTransferScope;
  transfer_confirmed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SmartAccountsBrowserBatchWorkflowSource {
  batch_id: string;
  source_company_id: string;
  tenant_id: string;
  ordinal: number;
  phase: SmartAccountsBrowserBatchWorkflowPhase;
  phase_generation: number;
  attempt_count: number;
  // A lease is an opaque concurrency handle, never a browser credential. It
  // is used only in memory to complete the exact server-issued action.
  lease_id?: string;
  lease_expires_at?: string;
  discovery_id?: string;
  discovery_contract_sha256?: string;
  discovery_receipt_sha256?: string;
  schema_id?: string;
  schema_approval_sha256?: string;
  package_id?: string;
  package_sha256?: string;
  capture_run_id?: string;
  preview_id?: string;
  preview_sha256?: string;
  reason_code?: string;
  created_at: string;
  updated_at: string;
}

export interface SmartAccountsBrowserBatchWorkflowStatus {
  workflow: SmartAccountsBrowserBatchWorkflow;
  status: string;
  schema_readiness_sha256?: string;
  sources: SmartAccountsBrowserBatchWorkflowSource[];
}

export interface SmartAccountsBrowserBatchWorkflowPreparationRequest {
  history_from: string;
  owner_confirmed: boolean;
  metadata_discovery_consent_confirmed: boolean;
  header_probe_consent_confirmed: boolean;
}

export interface SmartAccountsBrowserBatchWorkflowTransferConfirmationRequest {
  owner_confirmed: boolean;
  expected_schema_sha256: string;
}

export interface SmartAccountsBrowserBatchWorkflowDiscoveryAcquireRequest {
  metadata_only_consent_confirmed: boolean;
  response_header_probe_confirmed: boolean;
}

export interface SmartAccountsBrowserBatchWorkflowDiscoveryAcquireResponse {
  source: SmartAccountsBrowserBatchWorkflowSource;
  // Forward directly to the Brave relay and discard after the in-memory
  // postMessage handoff. It contains no relay token, but is still action-only
  // control data rather than page state.
  discovery: SmartAccountsBrowserDiscoveryIssue;
}

export interface SmartAccountsBrowserBatchWorkflowDiscoveryCompleteRequest {
  lease_id: string;
  phase_generation: number;
  discovery_id: string;
  result: SmartAccountsBrowserDiscoveryRelayResult;
}

export interface SmartAccountsBrowserBatchWorkflowPhaseRequest {
  phase_generation: number;
}

export interface SmartAccountsBrowserBatchWorkflowSchemaConfirmationRequest extends SmartAccountsBrowserBatchWorkflowPhaseRequest {
  review_confirmed: true;
}

export interface SmartAccountsBrowserBatchWorkflowCaptureAcquireRequest {
  transfer_consent_confirmed: boolean;
}

export interface SmartAccountsBrowserBatchWorkflowCaptureAcquireResponse {
  source: SmartAccountsBrowserBatchWorkflowSource;
  // Forward directly to the relay and discard after the in-memory handoff.
  // The short-lived capture token must never be rendered or stored by the UI.
  capture: SmartAccountsBrowserCaptureIssue;
}

export interface SmartAccountsBrowserBatchWorkflowCaptureCompleteRequest {
  lease_id: string;
  phase_generation: number;
}

export interface SmartAccountsBrowserBatchWorkflowCaptureCompleteResponse {
  source: SmartAccountsBrowserBatchWorkflowSource;
  progress: SmartAccountsBrowserCaptureStatus;
}

export interface SmartAccountsBrowserBatchWorkflowPreviewRequest extends SmartAccountsBrowserBatchWorkflowPhaseRequest {
  use_source_chart: boolean;
}

export interface SmartAccountsBrowserCaptureScope {
  mode: "partial" | "full";
  from_inclusive?: string;
  to_inclusive?: string;
  cutoff_at: string;
  resource_ids: string[];
}

export interface SmartAccountsBrowserCaptureStartRequest {
  source_company_id: string;
  manifest_version: "smartaccounts-brave-ui-v2";
  scope: SmartAccountsBrowserCaptureScope;
}

export interface SmartAccountsBrowserCaptureIssue {
  run_id: string;
  tenant_id: string;
  capture_token: string;
  expires_at: string;
  source_company_id: string;
  manifest_version: "smartaccounts-brave-ui-v2";
  scope: SmartAccountsBrowserCaptureScope;
  status: "open";
  transfer_consent: { version: 1; confirmed: true; confirmed_at: string };
}

// The server derives the destination date/cutoff and the sole reviewed v2
// General Ledger CSV resource. journal_entries is summary evidence only.
// The owner supplies only a history-start policy and exact action-time
// transfer consent. Raw relay capability exists only in the response's
// `capture` field and must be passed directly to extension memory.
export interface SmartAccountsBrowserCaptureWorkflowRequest {
  source_company_id: string;
  from_inclusive: string;
  transfer_consent_confirmed: boolean;
}

export interface SmartAccountsBrowserCapturePlan {
  version: "smartaccounts-browser-capture-plan-v1";
  from_date_policy: "OWNER_EXPLICIT_FROM_DATE";
  run_id?: string;
  tenant_id: string;
  source_company_id: string;
  manifest_version: "smartaccounts-brave-ui-v2";
  scope: SmartAccountsBrowserCaptureScope;
  eligible_resource_ids: ["general_ledger"];
}

export interface SmartAccountsBrowserCaptureWorkflowStatus {
  workflow_id: string;
  status: "READY_FOR_CONSENT" | "CAPTURE_ISSUED";
  plan: SmartAccountsBrowserCapturePlan;
  capture?: SmartAccountsBrowserCaptureIssue;
  progress?: SmartAccountsBrowserCaptureStatus;
}

export interface SmartAccountsBrowserCaptureResumeRequest {
  transfer_consent_confirmed: boolean;
}

export interface SmartAccountsBrowserCaptureResourceStatus {
  resource_id: string;
  coverage: "export_csv" | "page_only";
  status: "pending" | "completed" | "blocked";
}

export interface SmartAccountsBrowserCaptureCoverageReceipt {
  status: "partial_coverage_recorded" | "full_coverage_blocked";
  ready: boolean;
  completed_export_count: number;
  required_export_count: number;
  blocked_page_only_count: number;
  issues?: Array<{ resource_id?: string; code: string }>;
  finalized_at: string;
}

export interface SmartAccountsBrowserCaptureStaging {
  package_id: string;
  package_sha256: string;
  status: "compiling" | "compiled_private" | "pending_receiver_configuration" | "staging" | "staging_retry_required" | "staged_review_required" | "review_required";
  issue_code?: "browser_csv_schema_or_journal_review_required";
  record_chunks_acknowledged: number;
  artifact_chunks_acknowledged: number;
  finalized: boolean;
}

// This authenticated owner view deliberately excludes the short-lived relay
// capability, its hash, raw CSV, credentials, and bridge paths.
export interface SmartAccountsBrowserCaptureStatus {
  run_id: string;
  tenant_id: string;
  source_company_id: string;
  status: "open" | "finalized_partial" | "finalized_full_blocked";
  manifest_version: "smartaccounts-brave-ui-v2";
  scope: SmartAccountsBrowserCaptureScope;
  resources: SmartAccountsBrowserCaptureResourceStatus[];
  receipt?: SmartAccountsBrowserCaptureCoverageReceipt;
  staging?: SmartAccountsBrowserCaptureStaging;
}

export type SmartAccountsBrowserMasterDetailResource = "clients" | "vendors" | "articles";
export interface SmartAccountsBrowserMasterDetailScope { from_inclusive: string; to_inclusive: string; cutoff_at: string; }
export interface SmartAccountsBrowserMasterDetailContract { version: "smartaccounts-browser-master-detail-v1"; resource: SmartAccountsBrowserMasterDetailResource; origin: "https://sa.smartaccounts.eu"; list_page_path: string; detail_path_prefix: string; detail_result_page_path: string; fields: Array<{ name: string; kind: string; required?: boolean; enums?: string[] }>; }
export interface SmartAccountsBrowserMasterDetailTransferConsent { version: "smartaccounts-browser-master-detail-transfer-consent-v1"; confirmed: true; confirmed_at: string; }
export interface SmartAccountsBrowserMasterDetailAuthorizeRequest { source_company_id: string; transfer_consent_confirmed: true; batch_id?: string; refresh?: boolean; }
export interface SmartAccountsBrowserMasterDetailResumeRequest { transfer_consent_confirmed: true; }
export interface SmartAccountsBrowserMasterDetailIssue {
  run_id: string;
  tenant_id: string;
  source_company_id: string;
  manifest_version: "smartaccounts-browser-master-detail-v1";
  resource_id: SmartAccountsBrowserMasterDetailResource;
  schema_id: "clients_detail_v1" | "vendors_detail_v1" | "articles_detail_v1";
  source_schema: "smartaccounts-browser-master-detail-v1/clients_detail_v1" | "smartaccounts-browser-master-detail-v1/vendors_detail_v1" | "smartaccounts-browser-master-detail-v1/articles_detail_v1";
  contract: SmartAccountsBrowserMasterDetailContract;
  contract_sha256: string;
  approval_sha256: string;
  scope: SmartAccountsBrowserMasterDetailScope;
  snapshot_policy: "current_snapshot_only";
  snapshot_date: string;
  expires_at: string;
  transfer_consent: SmartAccountsBrowserMasterDetailTransferConsent;
  capture_token: string;
  sequence: 1 | 2 | 3;
}
export interface SmartAccountsBrowserMasterDetailIssueSet { batch_id: string; issues: SmartAccountsBrowserMasterDetailIssue[]; }
export interface SmartAccountsBrowserMasterDetailStatus {
  run_id: string;
  tenant_id: string;
  source_company_id: string;
  manifest_version: "smartaccounts-browser-master-detail-v1";
  resource_id: SmartAccountsBrowserMasterDetailResource;
  schema_id: string;
  source_schema: string;
  contract_sha256: string;
  approval_sha256: string;
  scope: SmartAccountsBrowserMasterDetailScope;
  snapshot_policy: "current_snapshot_only";
  snapshot_date: string;
  status: "open" | "finalized_archived_evidence" | "STAGED_REVIEW_REQUIRED";
  ndjson_sha256?: string;
  record_count?: number;
  package_id?: string;
  package_sha256?: string;
}

export interface ConfigureSmartAccountsSyncRequest {
  source_credential_reference: string;
  smartaccounts_gl_authoritative: boolean;
  invoice_payment_mode: "NON_POSTING";
}

export interface SmartAccountsCaptureRequest {
  scope_mode?: "full_history" | "window";
  date_from?: string;
  date_to?: string;
	resource_ids?: string[];
  max_pages?: number;
  rate_budget?: number;
  resume_run_id?: string;
}

export interface SmartAccountsCaptureResourceStatus {
  resource_id: string;
  endpoint_status: string;
  status: string;
  reason_code?: string;
  page_count?: number;
  deleted_count?: number;
  byte_count?: number;
  sha256?: string;
  scope_sha256?: string;
  next_eligible_at?: string;
}

export interface SmartAccountsCaptureProgress {
  run_id: string;
  status: string;
  scope_mode: "full_history" | "window";
  date_from?: string;
  date_to?: string;
	resource_ids?: string[];
  source_as_of_date: string;
  cutoff_at: string;
  resources: SmartAccountsCaptureResourceStatus[];
  summary: {
    total: number;
    completed: number;
    running: number;
    interrupted: number;
    rate_limited: number;
    review_required: number;
    dependency_required: number;
    brave_discovery_required: number;
  };
  staging?: SmartAccountsCaptureStaging;
}
export interface SmartAccountsCaptureStaging { package_id: string; package_sha256: string; status: string; record_chunks_acknowledged: number; artifact_chunks_acknowledged: number; finalized: boolean; }
export interface SmartAccountsPackagePreviewRequest { use_source_chart?: boolean; account_mappings?: Array<{source_account_external_id:string;target_account_id:string}>; account_imports?: Array<{source_account_external_id:string;code:string;name:string;account_type:string}>; }
export interface SmartAccountsPackageApplyRequest { confirm: boolean; preview_id: string; preview_sha256: string; tolerance_policy_id: string; }
export interface SmartAccountsPackagePreview { id:string; tenant_id:string; package_id:string; source_company_id:string; scope_sha256?: string; status:string; preview_sha256:string; financial_writes_planned:boolean; financial_writes_applied:boolean; journals?: unknown[]; account_imports?: Array<{source_account_external_id:string;code:string;name:string;account_type:string}>; account_reconciliation?: unknown[]; non_posting_record_count:number; issues?: Array<{code:string;message:string}>; }
export interface SmartAccountsArchiveCoverageBucket { domain: string; disposition: 'GL_APPLY_GATED' | 'REFERENCE_APPLY_GATED' | 'ARCHIVE_ONLY' | 'REVIEW_REQUIRED' | 'UNCONSUMED'; record_count: number; }
export interface SmartAccountsArchiveCoverageReport {
	package_id: string;
	package_sha256: string;
	manifest_sha256: string;
	scope_mode: string;
	declared_record_count: number;
	observed_record_count: number;
	artifact_count: number;
	integrity_ok: boolean;
	unconsumed_record_count: number;
	review_required_record_count: number;
	buckets: SmartAccountsArchiveCoverageBucket[];
}
export type SmartAccountsReconciliationStatus =
	| 'NOT_EVALUATED'
	| 'EVIDENCE_PENDING'
	| 'READY_FOR_ACCOUNTANT'
	| 'PASS'
	| 'PARTIAL_FAILURE';

export interface SmartAccountsReconciliationEvaluation {
	evaluation_id: string;
	batch_id: string;
	source_company_id: string;
	tenant_id: string;
	package_id?: string;
	gl_preview_id?: string;
	gl_preview_sha256?: string;
	gl_state: 'EVIDENCE_PENDING' | 'APPLIED' | 'APPLIED_REPLAY_VERIFIED';
	reference_state: 'NOT_APPLICABLE' | 'EVIDENCE_PENDING' | 'APPLIED';
	claim_kind?: 'full' | 'partial';
	expected_coverage_state?: 'full' | 'partial' | 'unknown';
	variance_within_policy: boolean;
	gl_revision_unresolved: number;
	gl_tombstone_unresolved: number;
	reference_revision_unresolved: number;
	reference_tombstone_unresolved: number;
	blockers?: string[];
	evidence_sha256?: string;
	tolerance_sha256?: string;
	status: SmartAccountsReconciliationStatus;
	created_at: string;
	updated_at: string;
	accountant_approved_at?: string;
}

export interface SmartAccountsReconciliationEvaluationResponse {
	evaluation: SmartAccountsReconciliationEvaluation;
	reused: boolean;
}

export interface SmartAccountsReconciliationRollup {
	batch_id: string;
	status: 'IN_PROGRESS' | 'ACCOUNTANT_REVIEW_REQUIRED' | 'PASS' | 'PARTIAL_FAILURE';
	selected_count: number;
	pass_count: number;
	pending_count: number;
	review_count: number;
	failure_count: number;
}

export interface SmartAccountsFullClaimEligibility {
	status: 'ELIGIBLE' | 'NOT_ELIGIBLE';
	full_claim_eligible: boolean;
	selected_count: number;
	current_pass_count: number;
	current_pass_gap_count: number;
	tombstone_gap_source_count: number;
	source_coverage_gap_count: number;
	domain_evidence_gap_source_count: number;
	matrix_blocker_count: number;
	matrix_filter_contract_gap_count: number;
	matrix_page_only_gap_count: number;
	matrix_review_required_count: number;
	matrix_unconsumed_count: number;
	matrix_missing_endpoint_count: number;
	matrix_schema_gap_count: number;
	matrix_coverage_gap_count: number;
	blocking_codes?: string[];
}

// Candidate values are derived only by OA from the exact staged package,
// scope, preview and currency set. The browser never submits a policy rule,
// variance, total, source row, or free-form digest.
export interface SmartAccountsTolerancePolicyCandidateRequest {
	package_id: string;
	preview_id: string;
}

export interface SmartAccountsTolerancePolicyCandidate {
	algorithm_version: 'smartaccounts-exact-match-v1';
	label: string;
	candidate_sha256: string;
}

export interface SmartAccountsTolerancePolicyApprovalRequest {
	confirmed: true;
	package_id: string;
	preview_id: string;
	expected_candidate_sha256: string;
}

export interface SmartAccountsTolerancePolicyResolutionRequest {
	package_id: string;
	preview_id: string;
}

// Deliberately omit the server's derived tolerance digest from the UI type.
// It is neither displayed nor retained; the following financial action takes
// only this opaque policy identifier and the server re-checks its binding.
export interface SmartAccountsTolerancePolicyResolution {
	policy_id: string;
	algorithm_version: 'smartaccounts-exact-match-v1';
	label: string;
	approved_at: string;
}

export interface SmartAccountsTolerancePolicy {
	policy_id: string;
	algorithm_version: 'smartaccounts-exact-match-v1';
	tenant_id: string;
	source_company_id: string;
	package_id: string;
	scope_sha256: string;
	preview_sha256: string;
	tolerance_policy_sha256: string;
	approved_at: string;
}

export interface SmartAccountsReconciliationApprovalRequest {
	confirmed: true;
	evidence_sha256: string;
	tolerance_sha256: string;
}
export interface SmartAccountsReferencePreviewRequest { entity_types?: Array<"account" | "customer" | "vendor" | "item">; }
export interface SmartAccountsReferenceApplyRequest { confirm: boolean; preview_id: string; preview_sha256: string; }
export interface SmartAccountsReferencePreview {
	id: string;
	tenant_id: string;
	package_id: string;
	source_company_id: string;
	status: "PREVIEW_READY" | "REVIEW_REQUIRED" | "APPLIED";
	preview_sha256: string;
	actions?: Array<{ entity_type: string; external_id: string; target_id: string; revision: string; action: "CREATE" | "RESUME" | "ALREADY_APPLIED" }>;
	reconciliation: Array<{ entity_type: string; source_records: number; create_planned: number; already_applied: number; review_required: number; tombstones: number }>;
	issues?: Array<{ code: string; entity_type?: string; external_id?: string; message: string }>;
	applied_at?: string;
}

export interface SmartAccountsSyncStatus {
  provider: "smartaccounts";
  source_company_id?: string;
  source_company_name?: string;
  configured: boolean;
  secret_reference_configured: boolean;
  smartaccounts_gl_authoritative: boolean;
  invoice_payment_mode: "NON_POSTING";
  capture_status: string;
  plan_status: string;
  reconciliation_status: string;
  financial_apply_eligible: boolean;
  explicit_confirmation_required: boolean;
  financial_writes_started: boolean;
  dry_run_requested_at?: string;
  capture_run_id?: string;
  capture_progress?: SmartAccountsCaptureProgress;
	capture_progresses?: SmartAccountsCaptureProgress[];
  next_action: string;
}

export interface Account {
  id: string;
  tenant_id: string;
  code: string;
  name: string;
  account_type: "ASSET" | "LIABILITY" | "EQUITY" | "REVENUE" | "EXPENSE";
  parent_id?: string;
  is_active: boolean;
  is_system: boolean;
  description?: string;
  created_at: string;
}

export interface CreateAccountRequest {
  code: string;
  name: string;
  account_type: Account["account_type"];
  parent_id?: string;
  description?: string;
}

export interface UpdateAccountRequest {
  code: string;
  name: string;
  account_type: Account["account_type"];
  parent_id?: string;
  description?: string;
}

export interface ImportAccountsRequest {
  csv_content: string;
  file_name?: string;
}

export interface ImportAccountsRowError {
  row: number;
  code?: string;
  name?: string;
  message: string;
}

export interface ImportAccountsResult {
  file_name?: string;
  rows_processed: number;
  accounts_created: number;
  rows_skipped: number;
  errors?: ImportAccountsRowError[];
}

export interface JournalEntry {
  id: string;
  tenant_id: string;
  entry_number: string;
  entry_date: string;
  description: string;
  reference?: string;
  source_type?: string;
  source_id?: string;
  requires_evidence: boolean;
  status: "DRAFT" | "POSTED" | "VOIDED";
  lines: JournalEntryLine[];
  posted_at?: string;
  posted_by?: string;
  voided_at?: string;
  voided_by?: string;
  void_reason?: string;
  created_at: string;
  created_by: string;
}

export interface JournalEntryLine {
  id: string;
  account_id: string;
  account?: Account;
  description?: string;
  debit_amount: Decimal;
  credit_amount: Decimal;
  currency: string;
  exchange_rate: Decimal;
  base_debit: Decimal;
  base_credit: Decimal;
}

export interface CreateJournalEntryRequest {
  entry_date: string;
  description: string;
  reference?: string;
  source_type?: string;
  source_id?: string;
  requires_evidence?: boolean;
  lines: {
    account_id: string;
    description?: string;
    debit_amount: string;
    credit_amount: string;
    currency?: string;
    exchange_rate?: string;
  }[];
}

export interface ImportOpeningBalancesRequest {
  entry_date: string;
  csv_content: string;
  file_name?: string;
  description?: string;
  reference?: string;
}

export interface ImportOpeningBalancesResult {
  file_name?: string;
  rows_processed: number;
  lines_imported: number;
  total_debit: Decimal;
  total_credit: Decimal;
  journal_entry: JournalEntry;
}

export interface TrialBalance {
  tenant_id: string;
  as_of_date: string;
  generated_at: string;
  accounts: AccountBalance[];
  total_debits: Decimal;
  total_credits: Decimal;
  is_balanced: boolean;
}

export interface AccountBalance {
  account_id: string;
  account_code: string;
  account_name: string;
  account_type: Account["account_type"];
  debit_balance: Decimal;
  credit_balance: Decimal;
  net_balance: Decimal;
}

export interface BalanceSheet {
  tenant_id: string;
  as_of_date: string;
  generated_at: string;
  assets: AccountBalance[];
  liabilities: AccountBalance[];
  equity: AccountBalance[];
  total_assets: Decimal;
  total_liabilities: Decimal;
  total_equity: Decimal;
  retained_earnings: Decimal;
  is_balanced: boolean;
}

export interface IncomeStatement {
  tenant_id: string;
  start_date: string;
  end_date: string;
  generated_at: string;
  revenue: AccountBalance[];
  expenses: AccountBalance[];
  total_revenue: Decimal;
  total_expenses: Decimal;
  net_income: Decimal;
}

// Contact types
export type ContactType = "CUSTOMER" | "SUPPLIER" | "BOTH";

export interface Contact {
  id: string;
  tenant_id: string;
  code?: string;
  name: string;
  contact_type: ContactType;
  reg_code?: string;
  vat_number?: string;
  email?: string;
  phone?: string;
  address_line1?: string;
  address_line2?: string;
  city?: string;
  postal_code?: string;
  country_code: string;
  payment_terms_days: number;
  credit_limit?: Decimal;
  default_account_id?: string;
  is_active: boolean;
  notes?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateContactRequest {
  code?: string;
  name: string;
  contact_type: ContactType;
  reg_code?: string;
  vat_number?: string;
  email?: string;
  phone?: string;
  address_line1?: string;
  address_line2?: string;
  city?: string;
  postal_code?: string;
  country_code?: string;
  payment_terms_days?: number;
  credit_limit?: string;
  default_account_id?: string;
  notes?: string;
}

export interface UpdateContactRequest {
  name?: string;
  reg_code?: string;
  vat_number?: string;
  email?: string;
  phone?: string;
  address_line1?: string;
  address_line2?: string;
  city?: string;
  postal_code?: string;
  country_code?: string;
  payment_terms_days?: number;
  credit_limit?: string;
  default_account_id?: string;
  notes?: string;
  is_active?: boolean;
}

export interface ContactFilter {
  type?: ContactType;
  active_only?: boolean;
  search?: string;
}

export interface ImportContactsRequest {
  csv_content: string;
  file_name?: string;
}

export interface ImportContactsRowError {
  row: number;
  name?: string;
  message: string;
}

export interface ImportContactsResult {
  file_name?: string;
  rows_processed: number;
  contacts_created: number;
  rows_skipped: number;
  errors?: ImportContactsRowError[];
}

// Invoice types
export type InvoiceType = "SALES" | "PURCHASE" | "CREDIT_NOTE";
export type InvoiceStatus =
  | "DRAFT"
  | "SENT"
  | "PARTIALLY_PAID"
  | "PAID"
  | "OVERDUE"
  | "VOIDED";

export interface Invoice {
  id: string;
  tenant_id: string;
  invoice_number: string;
  invoice_type: InvoiceType;
  contact_id: string;
  contact?: Contact;
  issue_date: string;
  due_date: string;
  currency: string;
  exchange_rate: Decimal;
  subtotal: Decimal;
  vat_amount: Decimal;
  total: Decimal;
  base_subtotal: Decimal;
  base_vat_amount: Decimal;
  base_total: Decimal;
  amount_paid: Decimal;
  status: InvoiceStatus;
  reference?: string;
  notes?: string;
  lines: InvoiceLine[];
  journal_entry_id?: string;
  einvoice_sent_at?: string;
  einvoice_id?: string;
  created_at: string;
  created_by: string;
  updated_at: string;
}

export interface InvoiceLine {
  id: string;
  tenant_id: string;
  invoice_id: string;
  line_number: number;
  description: string;
  quantity: Decimal;
  unit?: string;
  unit_price: Decimal;
  discount_percent: Decimal;
  vat_rate: Decimal;
  line_subtotal: Decimal;
  line_vat: Decimal;
  line_total: Decimal;
  account_id?: string;
  product_id?: string;
}

export interface CreateInvoiceRequest {
  invoice_type: InvoiceType;
  contact_id: string;
  issue_date: string;
  due_date: string;
  currency?: string;
  exchange_rate?: string;
  reference?: string;
  notes?: string;
  lines: CreateInvoiceLineRequest[];
}

export interface CreateInvoiceLineRequest {
  description: string;
  quantity: string;
  unit?: string;
  unit_price: string;
  discount_percent?: string;
  vat_rate: string;
  account_id?: string;
  product_id?: string;
}

export interface ImportInvoicesRequest {
  csv_content: string;
  file_name?: string;
}

export interface ImportInvoicesRowError {
  row: number;
  invoice_number?: string;
  message: string;
}

export interface ImportInvoicesResult {
  file_name?: string;
  rows_processed: number;
  invoices_created: number;
  lines_imported: number;
  rows_skipped: number;
  errors?: ImportInvoicesRowError[];
}

export interface InvoiceFilter {
  type?: InvoiceType;
  status?: InvoiceStatus;
  contact_id?: string;
  from_date?: string;
  to_date?: string;
  search?: string;
}

// Payment types
export type PaymentType = "RECEIVED" | "MADE";

export interface Payment {
  id: string;
  tenant_id: string;
  payment_number: string;
  payment_type: PaymentType;
  contact_id?: string;
  payment_date: string;
  amount: Decimal;
  currency: string;
  exchange_rate: Decimal;
  base_amount: Decimal;
  payment_method?: string;
  bank_account?: string;
  reference?: string;
  notes?: string;
  allocations: PaymentAllocation[];
  journal_entry_id?: string;
  reversal_of_payment_id?: string;
  reversed_by_payment_id?: string;
  reversed_at?: string;
  reversed_by?: string;
  reversal_reason?: string;
  created_at: string;
  created_by: string;
}

export interface PaymentAllocation {
  id: string;
  tenant_id: string;
  payment_id: string;
  invoice_id: string;
  amount: Decimal;
  created_at: string;
}

export interface CreatePaymentRequest {
  payment_type: PaymentType;
  contact_id?: string;
  payment_date: string;
  amount: string;
  currency?: string;
  exchange_rate?: string;
  payment_method?: string;
  bank_account?: string;
  reference?: string;
  notes?: string;
  allocations?: AllocationRequest[];
}

export interface AllocationRequest {
  invoice_id: string;
  amount: string;
}

export interface PaymentFilter {
  type?: PaymentType;
  method?: string;
  contact_id?: string;
  from_date?: string;
  to_date?: string;
}

export interface ReversePaymentRequest {
  payment_date?: string;
  reason: string;
  reference?: string;
  notes?: string;
}

export interface PaymentReversalResult {
  original_payment: Payment;
  reversal_payment: Payment;
}

// Quote types
export type QuoteStatus =
  | "DRAFT"
  | "SENT"
  | "ACCEPTED"
  | "REJECTED"
  | "EXPIRED"
  | "CONVERTED";

export interface Quote {
  id: string;
  tenant_id: string;
  quote_number: string;
  contact_id: string;
  contact?: Contact;
  quote_date: string;
  valid_until?: string;
  status: QuoteStatus;
  currency: string;
  exchange_rate: Decimal;
  subtotal: Decimal;
  vat_amount: Decimal;
  total: Decimal;
  notes?: string;
  converted_to_order_id?: string;
  converted_to_invoice_id?: string;
  lines: QuoteLine[];
  created_at: string;
  created_by: string;
  updated_at: string;
}

export interface QuoteLine {
  id: string;
  tenant_id: string;
  quote_id: string;
  line_number: number;
  description: string;
  quantity: Decimal;
  unit?: string;
  unit_price: Decimal;
  discount_percent: Decimal;
  vat_rate: Decimal;
  line_subtotal: Decimal;
  line_vat: Decimal;
  line_total: Decimal;
  product_id?: string;
}

export interface CreateQuoteRequest {
  contact_id: string;
  quote_date: string;
  valid_until?: string;
  currency?: string;
  exchange_rate?: string;
  notes?: string;
  lines: CreateQuoteLineRequest[];
}

export interface CreateQuoteLineRequest {
  description: string;
  quantity: string;
  unit?: string;
  unit_price: string;
  discount_percent?: string;
  vat_rate: string;
  product_id?: string;
}

export interface UpdateQuoteRequest {
  contact_id: string;
  quote_date: string;
  valid_until?: string;
  currency?: string;
  exchange_rate?: string;
  notes?: string;
  lines: CreateQuoteLineRequest[];
}

export interface QuoteFilter {
  status?: QuoteStatus;
  contact_id?: string;
  from_date?: string;
  to_date?: string;
  search?: string;
}

export interface ConvertQuoteToInvoiceRequest {
  issue_date?: string;
  due_date?: string;
  notes?: string;
}

export interface QuoteInvoiceConversionResult {
  quote: Quote;
  invoice: Invoice;
}

// Order types
export type OrderStatus =
  | "PENDING"
  | "CONFIRMED"
  | "PROCESSING"
  | "SHIPPED"
  | "DELIVERED"
  | "CANCELED";

export interface Order {
  id: string;
  tenant_id: string;
  order_number: string;
  contact_id: string;
  contact?: Contact;
  order_date: string;
  expected_delivery?: string;
  status: OrderStatus;
  currency: string;
  exchange_rate: Decimal;
  subtotal: Decimal;
  vat_amount: Decimal;
  total: Decimal;
  notes?: string;
  quote_id?: string;
  converted_to_invoice_id?: string;
  lines: OrderLine[];
  created_at: string;
  created_by: string;
  updated_at: string;
}

export interface OrderLine {
  id: string;
  tenant_id: string;
  order_id: string;
  line_number: number;
  description: string;
  quantity: Decimal;
  unit?: string;
  unit_price: Decimal;
  discount_percent: Decimal;
  vat_rate: Decimal;
  line_subtotal: Decimal;
  line_vat: Decimal;
  line_total: Decimal;
  product_id?: string;
}

export interface CreateOrderRequest {
  contact_id: string;
  order_date: string;
  expected_delivery?: string;
  currency?: string;
  exchange_rate?: string;
  notes?: string;
  quote_id?: string;
  lines: CreateOrderLineRequest[];
}

export interface CreateOrderLineRequest {
  description: string;
  quantity: string;
  unit?: string;
  unit_price: string;
  discount_percent?: string;
  vat_rate: string;
  product_id?: string;
}

export interface UpdateOrderRequest {
  contact_id: string;
  order_date: string;
  expected_delivery?: string;
  currency?: string;
  exchange_rate?: string;
  notes?: string;
  lines: CreateOrderLineRequest[];
}

export interface OrderFilter {
  status?: OrderStatus;
  contact_id?: string;
  from_date?: string;
  to_date?: string;
  search?: string;
}

export interface ConvertOrderToInvoiceRequest {
  issue_date?: string;
  due_date?: string;
  notes?: string;
}

export interface OrderInvoiceConversionResult {
  order: Order;
  invoice: Invoice;
}

// Fixed Asset types
export type AssetStatus = "DRAFT" | "ACTIVE" | "DISPOSED" | "SOLD";
export type DepreciationMethod =
  | "STRAIGHT_LINE"
  | "DECLINING_BALANCE"
  | "UNITS_OF_PRODUCTION";
export type DisposalMethod = "SOLD" | "SCRAPPED" | "DONATED" | "LOST";

export interface AssetCategory {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  depreciation_method: DepreciationMethod;
  default_useful_life_months: number;
  default_residual_value_percent: Decimal;
  asset_account_id?: string;
  depreciation_expense_account_id?: string;
  accumulated_depreciation_account_id?: string;
  created_at: string;
  updated_at: string;
}

export interface FixedAsset {
  id: string;
  tenant_id: string;
  asset_number: string;
  name: string;
  description?: string;
  category_id?: string;
  category?: AssetCategory;
  status: AssetStatus;
  purchase_date: string;
  purchase_cost: Decimal;
  supplier_id?: string;
  serial_number?: string;
  location?: string;
  depreciation_method: DepreciationMethod;
  useful_life_months: number;
  residual_value: Decimal;
  depreciation_start_date?: string;
  accumulated_depreciation: Decimal;
  book_value: Decimal;
  last_depreciation_date?: string;
  disposal_date?: string;
  disposal_method?: DisposalMethod;
  disposal_proceeds?: Decimal;
  disposal_notes?: string;
  disposal_journal_entry_id?: string;
  asset_account_id?: string;
  depreciation_expense_account_id?: string;
  accumulated_depreciation_account_id?: string;
  created_at: string;
  created_by: string;
  updated_at: string;
}

export interface DepreciationEntry {
  id: string;
  tenant_id: string;
  asset_id: string;
  depreciation_date: string;
  period_start: string;
  period_end: string;
  depreciation_amount: Decimal;
  accumulated_total: Decimal;
  book_value_after: Decimal;
  journal_entry_id?: string;
  created_at: string;
  created_by: string;
}

export interface CreateAssetCategoryRequest {
  name: string;
  description?: string;
  depreciation_method?: DepreciationMethod;
  default_useful_life_months?: number;
  default_residual_value_percent?: string;
  asset_account_id?: string;
  depreciation_expense_account_id?: string;
  accumulated_depreciation_account_id?: string;
}

export interface CreateAssetRequest {
  name: string;
  description?: string;
  category_id?: string;
  purchase_date: string;
  purchase_cost: string;
  supplier_id?: string;
  serial_number?: string;
  location?: string;
  depreciation_method?: DepreciationMethod;
  useful_life_months?: number;
  residual_value?: string;
  depreciation_start_date?: string;
  asset_account_id?: string;
  depreciation_expense_account_id?: string;
  accumulated_depreciation_account_id?: string;
}

export interface UpdateAssetRequest {
  name: string;
  description?: string;
  category_id?: string;
  serial_number?: string;
  location?: string;
  depreciation_method?: DepreciationMethod;
  useful_life_months?: number;
  residual_value?: string;
  asset_account_id?: string;
  depreciation_expense_account_id?: string;
  accumulated_depreciation_account_id?: string;
}

export interface DisposeAssetRequest {
  disposal_date: string;
  disposal_method: DisposalMethod;
  disposal_proceeds?: string;
  disposal_notes?: string;
  disposal_proceeds_account_id?: string;
  disposal_gain_loss_account_id?: string;
}

export interface AssetFilter {
  status?: AssetStatus;
  category_id?: string;
  from_date?: string;
  to_date?: string;
  search?: string;
}

export interface RecordDepreciationRequest {
  period_start: string;
  period_end: string;
}

// Inventory types
export type ProductType = "GOODS" | "SERVICE";
export type ProductStatus = "ACTIVE" | "INACTIVE";
export type MovementType = "IN" | "OUT" | "ADJUSTMENT" | "TRANSFER";

export interface Product {
  id: string;
  tenant_id: string;
  code: string;
  name: string;
  description?: string;
  product_type: ProductType;
  category_id?: string;
  unit?: string;
  purchase_price: Decimal;
  sales_price: Decimal;
  vat_rate: Decimal;
  min_stock_level: Decimal;
  current_stock: Decimal;
  reorder_point: Decimal;
  sale_account_id?: string;
  purchase_account_id?: string;
  inventory_account_id?: string;
  track_inventory: boolean;
  is_active: boolean;
  barcode?: string;
  supplier_id?: string;
  lead_time_days: number;
  created_at: string;
  updated_at: string;
}

export interface ProductCategory {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  parent_id?: string;
  created_at: string;
  updated_at: string;
}

export interface Warehouse {
  id: string;
  tenant_id: string;
  code: string;
  name: string;
  address?: string;
  is_default: boolean;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface StockLevel {
  id: string;
  tenant_id: string;
  product_id: string;
  warehouse_id: string;
  quantity: Decimal;
  reserved_qty: Decimal;
  available_qty: Decimal;
  last_updated: string;
}

export interface InventoryMovement {
  id: string;
  tenant_id: string;
  product_id: string;
  warehouse_id: string;
  movement_type: MovementType;
  quantity: Decimal;
  unit_cost: Decimal;
  total_cost: Decimal;
  lot_number?: string;
  serial_number?: string;
  expiry_date?: string;
  reference?: string;
  source_type?: string;
  source_id?: string;
  to_warehouse_id?: string;
  notes?: string;
  movement_date: string;
  created_at: string;
  created_by: string;
}

export interface CreateProductRequest {
  code?: string;
  name: string;
  description?: string;
  product_type: string;
  category_id?: string;
  unit?: string;
  purchase_price?: string;
  sales_price: string;
  vat_rate?: string;
  min_stock_level?: string;
  reorder_point?: string;
  sale_account_id?: string;
  purchase_account_id?: string;
  inventory_account_id?: string;
  track_inventory?: boolean;
  barcode?: string;
  supplier_id?: string;
  lead_time_days?: number;
}

export interface UpdateProductRequest {
  name: string;
  description?: string;
  category_id?: string;
  unit?: string;
  purchase_price?: string;
  sales_price: string;
  vat_rate?: string;
  min_stock_level?: string;
  reorder_point?: string;
  sale_account_id?: string;
  purchase_account_id?: string;
  inventory_account_id?: string;
  track_inventory?: boolean;
  is_active?: boolean;
  barcode?: string;
  supplier_id?: string;
  lead_time_days?: number;
}

export interface CreateProductCategoryRequest {
  name: string;
  description?: string;
  parent_id?: string;
}

export interface CreateWarehouseRequest {
  code: string;
  name: string;
  address?: string;
  is_default?: boolean;
}

export interface UpdateWarehouseRequest {
  name: string;
  address?: string;
  is_default?: boolean;
  is_active?: boolean;
}

export interface AdjustStockRequest {
  product_id: string;
  warehouse_id: string;
  quantity: string;
  unit_cost?: string;
  lot_number?: string;
  serial_number?: string;
  expiry_date?: string;
  reason?: string;
}

export interface TransferStockRequest {
  product_id: string;
  from_warehouse_id: string;
  to_warehouse_id: string;
  quantity: string;
  notes?: string;
}

export interface StockReservationRequest {
  product_id: string;
  warehouse_id: string;
  quantity: string;
  reason?: string;
}

export type InventoryValuationMethod =
  | "standard-cost"
  | "weighted-average"
  | "fifo"
  | "STANDARD_COST"
  | "WEIGHTED_AVERAGE"
  | "FIFO";

export type InventoryIssueCostingMethod =
  | "lot"
  | "weighted-average"
  | "standard-cost"
  | "LOT"
  | "WEIGHTED_AVERAGE"
  | "STANDARD_COST";

export interface InventoryValuationOptions {
  warehouse_id?: string;
  method?: InventoryValuationMethod;
}

export interface InventoryValuationLine {
  product_id: string;
  product_code: string;
  product_name: string;
  warehouse_id?: string;
  warehouse_code?: string;
  warehouse_name?: string;
  quantity: Decimal;
  reserved_qty: Decimal;
  available_qty: Decimal;
  unit_cost: Decimal;
  inventory_value: Decimal;
}

export interface InventoryValuationReport {
  tenant_id: string;
  warehouse_id?: string;
  valuation_method: string;
  lines: InventoryValuationLine[];
  total_quantity: Decimal;
  total_reserved: Decimal;
  total_available: Decimal;
  total_value: Decimal;
  generated_at: string;
}

export interface InventorySubledgerReconciliationOptions {
  warehouse_id?: string;
  method?: InventoryValuationMethod;
  as_of_date?: string;
}

export interface InventorySubledgerReconciliationLine {
  product_id: string;
  product_code: string;
  product_name: string;
  warehouse_id?: string;
  warehouse_code?: string;
  warehouse_name?: string;
  inventory_account_id?: string;
  account_code?: string;
  account_name?: string;
  account_type?: string;
  quantity: Decimal;
  inventory_value: Decimal;
  status: string;
}

export interface InventorySubledgerReconciliationAccountLine {
  account_id: string;
  account_code: string;
  account_name: string;
  account_type: string;
  product_line_count: number;
  subledger_value: Decimal;
  general_ledger_balance: Decimal;
  difference: Decimal;
  balanced: boolean;
}

export interface InventorySubledgerReconciliationReport {
  tenant_id: string;
  warehouse_id?: string;
  valuation_method: string;
  as_of_date: string;
  lines: InventorySubledgerReconciliationLine[];
  account_lines: InventorySubledgerReconciliationAccountLine[];
  total_subledger_value: Decimal;
  total_general_ledger_balance: Decimal;
  total_difference: Decimal;
  missing_account_line_count: number;
  unknown_account_line_count: number;
  invalid_account_type_line_count: number;
  difference_account_count: number;
  blocking_exception_line_count: number;
  blocking_exception_account_count: number;
  ready: boolean;
  generated_at: string;
}

export interface ProductFilter {
  product_type?: ProductType;
  status?: ProductStatus;
  category_id?: string;
  search?: string;
  low_stock?: boolean;
}

// Analytics types
export interface DashboardSummary {
  total_revenue: Decimal;
  total_expenses: Decimal;
  net_income: Decimal;
  revenue_change: Decimal;
  expenses_change: Decimal;
  total_receivables: Decimal;
  total_payables: Decimal;
  overdue_receivables: Decimal;
  overdue_payables: Decimal;
  draft_invoices: number;
  pending_invoices: number;
  overdue_invoices: number;
  period_start: string;
  period_end: string;
}

export interface RevenueExpenseChart {
  labels: string[];
  revenue: Decimal[];
  expenses: Decimal[];
  profit: Decimal[];
}

export interface CashFlowChart {
  labels: string[];
  inflows: Decimal[];
  outflows: Decimal[];
  net: Decimal[];
}

export interface AgingBucket {
  label: string;
  amount: Decimal;
  count: number;
}

export interface AgingReport {
  report_type: string;
  as_of_date: string;
  total: Decimal;
  buckets: AgingBucket[];
}

export type ActivityType = "INVOICE" | "PAYMENT" | "ENTRY" | "CONTACT";

export interface ActivityItem {
  id: string;
  type: ActivityType;
  action: string;
  description: string;
  amount?: string;
  created_at: string;
}

// Recurring Invoice types
export type Frequency =
  | "WEEKLY"
  | "BIWEEKLY"
  | "MONTHLY"
  | "QUARTERLY"
  | "YEARLY";

export interface RecurringInvoice {
  id: string;
  tenant_id: string;
  name: string;
  contact_id: string;
  contact_name?: string;
  invoice_type: string;
  currency: string;
  frequency: Frequency;
  start_date: string;
  end_date?: string;
  next_generation_date: string;
  payment_terms_days: number;
  reference?: string;
  notes?: string;
  is_active: boolean;
  last_generated_at?: string;
  generated_count: number;
  lines: RecurringInvoiceLine[];
  created_at: string;
  created_by: string;
  updated_at: string;
  // Email configuration
  send_email_on_generation: boolean;
  email_template_type?: string;
  recipient_email_override?: string;
  attach_pdf_to_email: boolean;
  email_subject_override?: string;
  email_message?: string;
}

export interface RecurringInvoiceLine {
  id: string;
  recurring_invoice_id: string;
  line_number: number;
  description: string;
  quantity: Decimal;
  unit?: string;
  unit_price: Decimal;
  discount_percent: Decimal;
  vat_rate: Decimal;
  account_id?: string;
  product_id?: string;
}

export interface CreateRecurringInvoiceRequest {
  name: string;
  contact_id: string;
  invoice_type?: string;
  currency?: string;
  frequency: Frequency;
  start_date: string;
  end_date?: string;
  payment_terms_days?: number;
  reference?: string;
  notes?: string;
  lines: CreateRecurringInvoiceLineRequest[];
  // Email configuration
  send_email_on_generation?: boolean;
  email_template_type?: string;
  recipient_email_override?: string;
  attach_pdf_to_email?: boolean;
  email_subject_override?: string;
  email_message?: string;
}

export interface CreateRecurringInvoiceLineRequest {
  description: string;
  quantity: string;
  unit?: string;
  unit_price: string;
  discount_percent?: string;
  vat_rate: string;
  account_id?: string;
  product_id?: string;
}

export interface UpdateRecurringInvoiceRequest {
  name?: string;
  contact_id?: string;
  frequency?: Frequency;
  end_date?: string;
  payment_terms_days?: number;
  reference?: string;
  notes?: string;
  lines?: CreateRecurringInvoiceLineRequest[];
  // Email configuration
  send_email_on_generation?: boolean;
  email_template_type?: string;
  recipient_email_override?: string;
  attach_pdf_to_email?: boolean;
  email_subject_override?: string;
  email_message?: string;
}

export interface CreateFromInvoiceRequest {
  name: string;
  frequency: Frequency;
  start_date: string;
  end_date?: string;
  payment_terms_days?: number;
}

export interface GenerationResult {
  recurring_invoice_id: string;
  generated_invoice_id: string;
  generated_invoice_number: string;
  // Email delivery status
  email_sent: boolean;
  email_status?: string;
  email_log_id?: string;
  email_error?: string;
}

// Email types
export type TemplateType =
  | "INVOICE_SEND"
  | "QUOTE_SEND"
  | "ORDER_CONFIRM"
  | "PAYMENT_RECEIPT"
  | "OVERDUE_REMINDER";
export type EmailStatus = "PENDING" | "SENT" | "FAILED";

export interface SMTPConfig {
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password?: string;
  smtp_from_email: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
}

export interface UpdateSMTPConfigRequest {
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password?: string;
  smtp_from_email: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
}

export interface TestSMTPResponse {
  success: boolean;
  message: string;
}

export interface EmailTemplate {
  id: string;
  tenant_id: string;
  template_type: TemplateType;
  subject: string;
  body_html: string;
  body_text?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface UpdateTemplateRequest {
  subject: string;
  body_html: string;
  body_text?: string;
  is_active: boolean;
}

export interface EmailLog {
  id: string;
  tenant_id: string;
  email_type: string;
  recipient_email: string;
  recipient_name?: string;
  subject: string;
  status: EmailStatus;
  sent_at?: string;
  error_message?: string;
  related_id?: string;
  created_at: string;
}

export interface SendInvoiceEmailRequest {
  recipient_email: string;
  recipient_name?: string;
  subject?: string;
  message?: string;
  attach_pdf: boolean;
}

export interface SendQuoteEmailRequest {
  recipient_email: string;
  recipient_name?: string;
  subject?: string;
  message?: string;
  attach_pdf: boolean;
  require_approved_evidence?: boolean;
}

export interface SendOrderEmailRequest {
  recipient_email: string;
  recipient_name?: string;
  subject?: string;
  message?: string;
  attach_pdf: boolean;
  require_approved_evidence?: boolean;
}

export interface SendPaymentReceiptRequest {
  recipient_email: string;
  recipient_name?: string;
  subject?: string;
  message?: string;
  require_approved_evidence?: boolean;
}

export interface EmailSentResponse {
  success: boolean;
  log_id: string;
  message: string;
}

// Reminder Rule types
export type TriggerType = "BEFORE_DUE" | "ON_DUE" | "AFTER_DUE";

export interface ReminderRule {
  id: string;
  tenant_id: string;
  name: string;
  trigger_type: TriggerType;
  days_offset: number;
  email_template_type: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateReminderRuleRequest {
  name: string;
  trigger_type: TriggerType;
  days_offset: number;
  email_template_type?: string;
  is_active: boolean;
}

export interface UpdateReminderRuleRequest {
  name?: string;
  email_template_type?: string;
  is_active?: boolean;
}

export interface AutomatedReminderResult {
  tenant_id: string;
  rule_id: string;
  rule_name: string;
  invoices_found: number;
  reminders_sent: number;
  skipped: number;
  failed: number;
  errors?: string[];
  run_at: string;
}

// Banking types
export type TransactionStatus = "UNMATCHED" | "MATCHED" | "RECONCILED";
export type FollowUpStatus = "NONE" | "EVIDENCE_REQUIRED" | "READY_TO_MATCH";
export type ReconciliationStatus = "IN_PROGRESS" | "COMPLETED";

export interface BankAccount {
  id: string;
  tenant_id: string;
  name: string;
  account_number: string;
  bank_name?: string;
  currency: string;
  balance?: Decimal;
  gl_account_id?: string;
  is_default?: boolean;
  is_active: boolean;
  created_at: string;
  updated_at?: string;
}

export interface BankTransaction {
  id: string;
  tenant_id: string;
  bank_account_id: string;
  transaction_date: string;
  value_date?: string;
  description: string;
  reference?: string;
  amount: Decimal;
  currency: string;
  counterparty_name?: string;
  counterparty_account?: string;
  status: TransactionStatus;
  follow_up_status?: FollowUpStatus;
  review_note?: string;
  reviewed_by?: string;
  reviewed_at?: string;
  matched_payment_id?: string;
  reconciliation_id?: string;
  import_id?: string;
  created_at: string;
  remediation_actions?: BankRemediationAction[];
}

export interface BankRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  entity_type?: string;
  entity_id?: string;
  bank_account_id?: string;
  transaction_status?: string;
  follow_up_status?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface UpdateBankTransactionReviewRequest {
  follow_up_status?: FollowUpStatus;
  review_note?: string;
}

export interface BankReconciliation {
  id: string;
  tenant_id: string;
  bank_account_id: string;
  statement_date: string;
  opening_balance: Decimal;
  closing_balance: Decimal;
  calculated_balance?: Decimal;
  difference?: Decimal;
  status: ReconciliationStatus;
  completed_at?: string;
  completed_by?: string;
  transactions_matched: number;
  transactions_unmatched: number;
  created_at: string;
  created_by: string;
}

export interface BankStatementImport {
  id: string;
  tenant_id: string;
  bank_account_id: string;
  file_name: string;
  transactions_imported: number;
  transactions_matched: number;
  duplicates_skipped: number;
  created_at: string;
  created_by: string;
}

export interface MatchSuggestion {
  payment_id: string;
  payment_number: string;
  payment_date: string;
  amount: Decimal;
  contact_name?: string;
  reference?: string;
  confidence: number;
  match_reason: string;
}

export interface CreateBankAccountRequest {
  name: string;
  account_number: string;
  bank_name?: string;
  currency?: string;
  gl_account_id?: string;
}

export interface UpdateBankAccountRequest {
  name?: string;
  bank_name?: string;
  gl_account_id?: string;
  is_active?: boolean;
}

export type BankTransactionImportFormat =
  | "auto"
  | "generic"
  | "lhv"
  | "camt053"
  | "lhv-camt";

export interface ImportTransactionsRequest {
  csv_content: string;
  file_name: string;
  format?: BankTransactionImportFormat;
  skip_duplicates?: boolean;
}

export interface ImportResult {
  import_id: string;
  transactions_imported: number;
  duplicates_skipped: number;
  errors?: string[];
}

export interface CreateReconciliationRequest {
  statement_date: string;
  opening_balance: string;
  closing_balance: string;
}

// Tax (KMD) types
export interface KMDRow {
  code: string;
  description: string;
  tax_base: Decimal;
  tax_amount: Decimal;
}

export interface KMDDeclaration {
  id: string;
  tenant_id: string;
  year: number;
  month: number;
  status: "DRAFT" | "SUBMITTED" | "ACCEPTED";
  total_output_vat: Decimal;
  total_input_vat: Decimal;
  rows: KMDRow[];
  remediation_actions?: KMDRemediationAction[];
  submitted_at?: string;
  created_at: string;
  updated_at: string;
}

export interface KMDRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  period?: string;
  entity_type?: string;
  entity_id?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface TaxReportRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  period?: string;
  entity_type?: string;
  entity_id?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface CreateKMDRequest {
  year: number;
  month: number;
}

export interface KMDINFReportRequest {
  year: number;
  month: number;
  threshold?: string | number | Decimal;
}

export interface KMDINFPartSummary {
  part: "A" | "B";
  partner_count: number;
  invoice_count: number;
  taxable_amount: Decimal;
  vat_amount: Decimal;
  total_amount: Decimal;
}

export interface KMDINFReportRow {
  part: "A" | "B";
  contact_id: string;
  contact_name: string;
  contact_reg_code?: string;
  contact_vat_number?: string;
  invoice_id: string;
  invoice_number: string;
  invoice_date: string;
  invoice_type: string;
  taxable_amount: Decimal;
  vat_amount: Decimal;
  total_amount: Decimal;
  partner_period_taxable_amount: Decimal;
}

export interface KMDINFReport {
  tenant_id: string;
  year: number;
  month: number;
  threshold: Decimal;
  generated_at: string;
  summary: KMDINFPartSummary[];
  rows: KMDINFReportRow[];
  remediation_actions?: TaxReportRemediationAction[];
}

export interface EUVATOSSReportRequest {
  year: number;
  quarter: number;
  include_b2b?: boolean;
}

export interface EUVATOSSCountrySummary {
  country_code: string;
  country_name: string;
  invoice_count: number;
  line_count: number;
  taxable_amount: Decimal;
  vat_amount: Decimal;
  total_amount: Decimal;
}

export interface EUVATOSSReportRow {
  country_code: string;
  country_name: string;
  vat_rate: Decimal;
  invoice_count: number;
  line_count: number;
  taxable_amount: Decimal;
  vat_amount: Decimal;
  total_amount: Decimal;
}

export interface EUVATOSSReport {
  tenant_id: string;
  year: number;
  quarter: number;
  period_start: string;
  period_end: string;
  scheme: string;
  currency: string;
  include_b2b: boolean;
  generated_at: string;
  summary: EUVATOSSCountrySummary[];
  rows: EUVATOSSReportRow[];
  taxable_amount: Decimal;
  vat_amount: Decimal;
  total_amount: Decimal;
  invoice_count: number;
  line_count: number;
  remediation_actions?: TaxReportRemediationAction[];
}

// Cash Flow types
export interface CashFlowItem {
  code: string;
  description: string;
  description_et: string;
  amount: string;
  is_subtotal: boolean;
}

export interface CashFlowStatement {
  tenant_id: string;
  start_date: string;
  end_date: string;
  operating_activities: CashFlowItem[];
  investing_activities: CashFlowItem[];
  financing_activities: CashFlowItem[];
  total_operating: string;
  total_investing: string;
  total_financing: string;
  net_cash_change: string;
  opening_cash: string;
  closing_cash: string;
  generated_at: string;
}

// Balance Confirmation types
export type BalanceConfirmationType = "RECEIVABLE" | "PAYABLE";
export type ReportExportFormat = "csv" | "xlsx" | "pdf";

export interface BalanceInvoice {
  invoice_id: string;
  invoice_number: string;
  invoice_date: string;
  due_date: string;
  total_amount: string;
  amount_paid: string;
  outstanding_amount: string;
  currency: string;
  days_overdue: number;
}

export interface ContactBalance {
  contact_id: string;
  contact_name: string;
  contact_code?: string;
  contact_email?: string;
  balance: string;
  invoice_count: number;
  oldest_invoice?: string;
}

export interface BalanceConfirmationSummary {
  type: BalanceConfirmationType;
  as_of_date: string;
  total_balance: string;
  contact_count: number;
  invoice_count: number;
  contacts: ContactBalance[];
  generated_at: string;
}

export interface BalanceConfirmation {
  id: string;
  tenant_id: string;
  contact_id: string;
  contact_name: string;
  contact_code?: string;
  contact_email?: string;
  type: BalanceConfirmationType;
  as_of_date: string;
  total_balance: string;
  invoices: BalanceInvoice[];
  generated_at: string;
}

// Payment Reminder types
export type ReminderStatus = "PENDING" | "SENT" | "FAILED" | "CANCELLED";

export interface PaymentReminder {
  id: string;
  tenant_id: string;
  invoice_id: string;
  invoice_number: string;
  contact_id: string;
  contact_name: string;
  contact_email: string;
  reminder_number: number;
  status: ReminderStatus;
  sent_at?: string;
  error_message?: string;
  created_at: string;
  updated_at: string;
}

export interface OverdueInvoice {
  id: string;
  invoice_number: string;
  contact_id: string;
  contact_name: string;
  contact_email?: string;
  issue_date: string;
  due_date: string;
  total: string;
  amount_paid: string;
  outstanding_amount: string;
  currency: string;
  days_overdue: number;
  reminder_count: number;
  last_reminder_at?: string;
}

export interface OverdueInvoicesSummary {
  total_overdue: string;
  invoice_count: number;
  contact_count: number;
  average_days_overdue: number;
  invoices: OverdueInvoice[];
  generated_at: string;
}

export interface SendReminderRequest {
  invoice_id: string;
  message?: string;
}

export interface SendBulkRemindersRequest {
  invoice_ids: string[];
  message?: string;
}

export interface ReminderResult {
  invoice_id: string;
  invoice_number: string;
  success: boolean;
  message: string;
  reminder_id?: string;
}

export interface BulkReminderResult {
  total_requested: number;
  successful: number;
  failed: number;
  results: ReminderResult[];
}

// Payroll types
export type EmploymentType = "FULL_TIME" | "PART_TIME" | "CONTRACT";
export type PayrollStatus =
  | "DRAFT"
  | "CALCULATED"
  | "APPROVED"
  | "PAID"
  | "DECLARED";
export type TSDStatus = "DRAFT" | "SUBMITTED" | "ACCEPTED" | "REJECTED";

export interface Employee {
  id: string;
  tenant_id: string;
  employee_number?: string;
  first_name: string;
  last_name: string;
  personal_code?: string;
  email?: string;
  phone?: string;
  address?: string;
  bank_account?: string;
  start_date: string;
  end_date?: string;
  position?: string;
  department?: string;
  employment_type: EmploymentType;
  tax_residency: string;
  apply_basic_exemption: boolean;
  basic_exemption_amount: Decimal;
  funded_pension_rate: Decimal;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateEmployeeRequest {
  employee_number?: string;
  first_name: string;
  last_name: string;
  personal_code?: string;
  email?: string;
  phone?: string;
  address?: string;
  bank_account?: string;
  start_date: string;
  position?: string;
  department?: string;
  employment_type: EmploymentType;
  apply_basic_exemption: boolean;
  basic_exemption_amount?: string;
  funded_pension_rate?: string;
}

export interface UpdateEmployeeRequest {
  employee_number?: string;
  first_name?: string;
  last_name?: string;
  personal_code?: string;
  email?: string;
  phone?: string;
  address?: string;
  bank_account?: string;
  end_date?: string;
  position?: string;
  department?: string;
  employment_type?: EmploymentType;
  apply_basic_exemption?: boolean;
  basic_exemption_amount?: string;
  funded_pension_rate?: string;
  is_active?: boolean;
}

export interface ImportEmployeesRequest {
  csv_content: string;
  file_name?: string;
}

export interface ImportEmployeesRowError {
  row: number;
  employee_name?: string;
  employee_number?: string;
  message: string;
}

export interface ImportEmployeesResult {
  file_name?: string;
  rows_processed: number;
  employees_created: number;
  salaries_created: number;
  rows_skipped: number;
  errors?: ImportEmployeesRowError[];
}

export interface ImportPayrollHistoryRequest {
  csv_content: string;
  file_name?: string;
}

export interface ImportPayrollHistoryRowError {
  row: number;
  period_year?: number;
  period_month?: number;
  employee_name?: string;
  employee_number?: string;
  message: string;
}

export interface ImportPayrollHistoryResult {
  file_name?: string;
  rows_processed: number;
  payroll_runs_created: number;
  payslips_created: number;
  rows_skipped: number;
  errors?: ImportPayrollHistoryRowError[];
}

export interface ImportLeaveBalancesRequest {
  csv_content: string;
  file_name?: string;
}

export interface ImportLeaveBalanceRowError {
  row: number;
  year?: number;
  employee_name?: string;
  employee_number?: string;
  absence_type_code?: string;
  message: string;
}

export interface ImportLeaveBalancesResult {
  file_name?: string;
  rows_processed: number;
  leave_balances_created: number;
  leave_balances_updated: number;
  rows_skipped: number;
  errors?: ImportLeaveBalanceRowError[];
}

export interface SalaryComponent {
  id: string;
  tenant_id: string;
  employee_id: string;
  component_type: string;
  name: string;
  amount: Decimal;
  is_taxable: boolean;
  is_recurring: boolean;
  effective_from: string;
  effective_to?: string;
  created_at: string;
}

export interface PayrollRun {
  id: string;
  tenant_id: string;
  period_year: number;
  period_month: number;
  status: PayrollStatus;
  payment_date?: string;
  total_gross: Decimal;
  total_net: Decimal;
  total_employer_cost: Decimal;
  notes?: string;
  remediation_actions?: PayrollRunRemediationAction[];
  created_by?: string;
  approved_by?: string;
  approved_at?: string;
  created_at: string;
  updated_at: string;
  payslips?: Payslip[];
}

export interface PayrollRunRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  period?: string;
  entity_type?: string;
  entity_id?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface TSDRemediationAction {
  code: string;
  severity: string;
  scope: string;
  owner_role: string;
  workspace_queue?: string;
  assignment_key?: string;
  priority?: string;
  due_in_days?: number;
  message: string;
  action: string;
  period?: string;
  entity_type?: string;
  entity_id?: string;
  ui_path?: string;
  cli_command?: string;
}

export interface CreatePayrollRunRequest {
  period_year: number;
  period_month: number;
  payment_date?: string;
  notes?: string;
}

export interface UpdatePayrollRunPaymentDateRequest {
  payment_date: string;
}

export interface Payslip {
  id: string;
  tenant_id: string;
  payroll_run_id: string;
  employee_id: string;
  gross_salary: Decimal;
  taxable_income: Decimal;
  income_tax: Decimal;
  unemployment_insurance_employee: Decimal;
  funded_pension: Decimal;
  other_deductions: Decimal;
  net_salary: Decimal;
  social_tax: Decimal;
  unemployment_insurance_employer: Decimal;
  total_employer_cost: Decimal;
  basic_exemption_applied: Decimal;
  payment_status: string;
  paid_at?: string;
  created_at: string;
  employee?: Employee;
}

export interface TSDDeclaration {
  id: string;
  tenant_id: string;
  period_year: number;
  period_month: number;
  payroll_run_id?: string;
  total_payments: Decimal;
  total_income_tax: Decimal;
  total_social_tax: Decimal;
  total_unemployment_employer: Decimal;
  total_unemployment_employee: Decimal;
  total_funded_pension: Decimal;
  status: TSDStatus;
  submitted_at?: string;
  emta_reference?: string;
  remediation_actions?: TSDRemediationAction[];
  created_at: string;
  updated_at: string;
  rows?: TSDRow[];
}

export interface TSDRow {
  id: string;
  tenant_id: string;
  declaration_id: string;
  employee_id: string;
  personal_code: string;
  first_name: string;
  last_name: string;
  payment_type: string;
  gross_payment: Decimal;
  basic_exemption: Decimal;
  taxable_amount: Decimal;
  income_tax: Decimal;
  social_tax: Decimal;
  unemployment_insurance_employer: Decimal;
  unemployment_insurance_employee: Decimal;
  funded_pension: Decimal;
  created_at: string;
}

export interface TaxCalculation {
  gross_salary: Decimal;
  basic_exemption: Decimal;
  taxable_income: Decimal;
  income_tax: Decimal;
  unemployment_employee: Decimal;
  funded_pension: Decimal;
  total_deductions: Decimal;
  net_salary: Decimal;
  social_tax: Decimal;
  unemployment_employer: Decimal;
  total_employer_cost: Decimal;
}

// Leave/Absence Management Types
export type LeaveStatus = "PENDING" | "APPROVED" | "REJECTED" | "CANCELLED";

export interface AbsenceType {
  id: string;
  tenant_id: string;
  code: string;
  name: string;
  name_et: string;
  description?: string;
  is_paid: boolean;
  affects_salary: boolean;
  requires_document: boolean;
  document_type?: string;
  default_days_per_year: Decimal;
  max_carryover_days: Decimal;
  tsd_code?: string;
  emta_code?: string;
  is_system: boolean;
  is_active: boolean;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface LeaveBalance {
  id: string;
  tenant_id: string;
  employee_id: string;
  absence_type_id: string;
  year: number;
  entitled_days: Decimal;
  carryover_days: Decimal;
  used_days: Decimal;
  pending_days: Decimal;
  remaining_days: Decimal;
  notes?: string;
  created_at: string;
  updated_at: string;
  absence_type?: AbsenceType;
}

export interface LeaveRecord {
  id: string;
  tenant_id: string;
  employee_id: string;
  absence_type_id: string;
  start_date: string;
  end_date: string;
  total_days: Decimal;
  working_days: Decimal;
  status: LeaveStatus;
  document_number?: string;
  document_date?: string;
  document_url?: string;
  requested_at: string;
  requested_by?: string;
  approved_at?: string;
  approved_by?: string;
  rejected_at?: string;
  rejected_by?: string;
  rejection_reason?: string;
  payroll_run_id?: string;
  notes?: string;
  created_at: string;
  updated_at: string;
  absence_type?: AbsenceType;
  employee?: Employee;
}

export interface CreateLeaveRecordRequest {
  employee_id: string;
  absence_type_id: string;
  start_date: string;
  end_date: string;
  total_days: Decimal;
  working_days: Decimal;
  document_number?: string;
  document_date?: string;
  notes?: string;
}

export interface UpdateLeaveBalanceRequest {
  entitled_days?: Decimal;
  carryover_days?: Decimal;
  notes?: string;
}

export interface RejectLeaveRequest {
  reason: string;
}

// Plugin Types
export type PluginState = "installed" | "enabled" | "disabled" | "failed";
export type PermissionRisk = "low" | "medium" | "high" | "critical";
export type PermissionCategory = "data" | "system" | "database" | "dangerous";
export type RepositoryType = "github" | "gitlab";

export interface PluginRegistry {
  id: string;
  name: string;
  url: string;
  description?: string;
  is_official: boolean;
  is_active: boolean;
  last_synced_at?: string;
  created_at: string;
  updated_at: string;
}

export interface Plugin {
  id: string;
  name: string;
  display_name: string;
  description?: string;
  version: string;
  repository_url: string;
  repository_type: RepositoryType;
  author?: string;
  license?: string;
  homepage_url?: string;
  state: PluginState;
  granted_permissions: string[];
  manifest: PluginManifest;
  installed_at: string;
  updated_at: string;
}

export interface PluginManifest {
  name: string;
  display_name: string;
  version: string;
  description?: string;
  author?: string;
  license?: string;
  homepage?: string;
  min_app_version?: string;
  permissions: string[];
  backend?: PluginBackendConfig;
  frontend?: PluginFrontendConfig;
  database?: PluginDatabaseConfig;
  settings?: Record<string, unknown>;
}

export interface PluginBackendConfig {
  package?: string;
  entry?: string;
  hooks?: PluginHook[];
  routes?: PluginRoute[];
}

export interface PluginHook {
  event: string;
  handler: string;
}

export interface PluginRoute {
  method: string;
  path: string;
  handler: string;
}

export interface PluginFrontendConfig {
  components?: string;
  navigation?: PluginNavItem[];
  slots?: PluginSlot[];
}

export interface PluginNavItem {
  label: string;
  icon?: string;
  path: string;
  position?: string;
}

export interface PluginSlot {
  name: string;
  component: string;
  label?: string;
  description?: string;
  path?: string;
  kind?: "card" | "link" | "action";
  badge?: string;
  order?: number;
}

export interface PluginDatabaseConfig {
  migrations?: string;
}

export interface PluginPermission {
  name: string;
  category: PermissionCategory;
  risk: PermissionRisk;
  description: string;
}

export interface TenantPlugin {
  id: string;
  tenant_id: string;
  plugin_id: string;
  is_enabled: boolean;
  settings: Record<string, unknown>;
  enabled_at?: string;
  created_at: string;
  updated_at: string;
  plugin?: Plugin;
}

export interface TenantPluginSettings {
  plugin_id: string;
  settings: Record<string, unknown>;
  schema?: Record<string, unknown>;
}

export interface PluginSearchResult {
  plugin: PluginInfo;
  registry: string;
}

export interface PluginInfo {
  name: string;
  display_name: string;
  description?: string;
  repository: string;
  version: string;
  author?: string;
  license?: string;
  tags?: string[];
}

// Cost Center Types
export type BudgetPeriod = "MONTHLY" | "QUARTERLY" | "ANNUAL";

export interface CostCenter {
  id: string;
  tenant_id: string;
  code: string;
  name: string;
  description?: string;
  parent_id?: string;
  is_active: boolean;
  budget_amount?: string;
  budget_period: BudgetPeriod;
  created_at: string;
  updated_at: string;
  children?: CostCenter[];
  total_spent?: string;
  budget_used_percentage?: string;
}

export interface CreateCostCenterRequest {
  code: string;
  name: string;
  description?: string;
  parent_id?: string;
  is_active: boolean;
  budget_amount?: string;
  budget_period?: BudgetPeriod;
}

export interface UpdateCostCenterRequest {
  code: string;
  name: string;
  description?: string;
  parent_id?: string;
  is_active: boolean;
  budget_amount?: string;
  budget_period?: BudgetPeriod;
}

export interface CostAllocation {
  id: string;
  tenant_id: string;
  cost_center_id: string;
  journal_entry_line_id: string;
  amount: Decimal;
  allocation_percentage?: Decimal;
  allocation_date: string;
  notes?: string;
  created_at: string;
  updated_at: string;
  cost_center_code?: string;
  cost_center_name?: string;
}

export interface CreateCostAllocationRequest {
  cost_center_id: string;
  journal_entry_line_id: string;
  amount: string;
  allocation_percentage?: string;
  allocation_date: string;
  notes?: string;
}

export interface CostAllocationFilters {
  cost_center_id?: string;
  journal_entry_line_id?: string;
  start_date?: string;
  end_date?: string;
}

export interface CostCenterSummary {
  cost_center: CostCenter;
  total_expenses: string;
  budget_amount: string;
  budget_used_percentage: string;
  is_over_budget: boolean;
  period_start: string;
  period_end: string;
}

export interface CostCenterReport {
  tenant_id: string;
  period_start: string;
  period_end: string;
  generated_at: string;
  cost_centers: CostCenterSummary[];
  total_expenses: string;
  total_budget: string;
}

// Interest calculation types
export interface InterestSettings {
  rate: number;
  annual_rate: number;
  description: string;
  is_enabled: boolean;
}

export interface UpdateInterestSettingsRequest {
  rate: number;
}

export interface InterestCalculationResult {
  invoice_id: string;
  invoice_number: string;
  due_date: string;
  days_overdue: number;
  outstanding_amount: string;
  interest_rate: string;
  daily_interest: string;
  total_interest: string;
  total_with_interest: string;
  calculated_at: string;
  currency: string;
}

export interface InvoiceInterest {
  id: string;
  invoice_id: string;
  calculated_at: string;
  days_overdue: number;
  principal_amount: string;
  interest_rate: string;
  interest_amount: string;
  total_with_interest: string;
  created_at: string;
}

export const api = new ApiClient();
