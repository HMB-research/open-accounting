import { B as BROWSER } from './false-CRHihH2U.js';
import Decimal from 'decimal.js';

const browser = BROWSER;
const API_BASE = "http://localhost:8080";
class ApiClient {
  accessToken = null;
  refreshToken = null;
  constructor() {
  }
  setTokens(access, refresh) {
    this.accessToken = access;
    this.refreshToken = refresh;
  }
  clearTokens() {
    this.accessToken = null;
    this.refreshToken = null;
  }
  get isAuthenticated() {
    return !!this.accessToken;
  }
  async request(method, path, body, skipAuth = false) {
    const headers = {
      "Content-Type": "application/json"
    };
    if (!skipAuth && this.accessToken) {
      headers["Authorization"] = `Bearer ${this.accessToken}`;
    }
    const response = await fetch(`${API_BASE}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : void 0
    });
    if (response.status === 401 && this.refreshToken && !skipAuth) {
      const refreshed = await this.refreshAccessToken();
      if (refreshed) {
        return this.request(method, path, body, false);
      }
      throw new Error("Session expired. Please log in again.");
    }
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.error || "Request failed");
    }
    return this.parseDecimals(data);
  }
  parseDecimals(obj) {
    if (typeof obj === "string" && /^-?\d+(\.\d+)?$/.test(obj)) {
      return new Decimal(obj);
    }
    if (Array.isArray(obj)) {
      return obj.map((item) => this.parseDecimals(item));
    }
    if (obj !== null && typeof obj === "object") {
      const result = {};
      for (const [key, value] of Object.entries(obj)) {
        result[key] = this.parseDecimals(value);
      }
      return result;
    }
    return obj;
  }
  async refreshAccessToken() {
    try {
      const data = await this.request(
        "POST",
        "/api/v1/auth/refresh",
        { refresh_token: this.refreshToken },
        true
      );
      this.accessToken = data.access_token;
      if (browser) ;
      return true;
    } catch {
      this.clearTokens();
      return false;
    }
  }
  // Auth endpoints
  async register(email, password, name) {
    return this.request(
      "POST",
      "/api/v1/auth/register",
      { email, password, name },
      true
    );
  }
  async login(email, password, tenantId) {
    const data = await this.request(
      "POST",
      "/api/v1/auth/login",
      { email, password, tenant_id: tenantId },
      true
    );
    this.setTokens(data.access_token, data.refresh_token);
    return data;
  }
  logout() {
    this.clearTokens();
  }
  // User endpoints
  async getCurrentUser() {
    return this.request(
      "GET",
      "/api/v1/me"
    );
  }
  async getMyTenants() {
    return this.request("GET", "/api/v1/me/tenants");
  }
  // Tenant endpoints
  async createTenant(name, slug, settings) {
    return this.request("POST", "/api/v1/tenants", { name, slug, settings });
  }
  async getTenant(tenantId) {
    return this.request("GET", `/api/v1/tenants/${tenantId}`);
  }
  // Account endpoints
  async listAccounts(tenantId, activeOnly = false) {
    const query = activeOnly ? "?active_only=true" : "";
    return this.request("GET", `/api/v1/tenants/${tenantId}/accounts${query}`);
  }
  async createAccount(tenantId, data) {
    return this.request("POST", `/api/v1/tenants/${tenantId}/accounts`, data);
  }
  async getAccount(tenantId, accountId) {
    return this.request("GET", `/api/v1/tenants/${tenantId}/accounts/${accountId}`);
  }
  // Journal entry endpoints
  async getJournalEntry(tenantId, entryId) {
    return this.request(
      "GET",
      `/api/v1/tenants/${tenantId}/journal-entries/${entryId}`
    );
  }
  async createJournalEntry(tenantId, data) {
    return this.request("POST", `/api/v1/tenants/${tenantId}/journal-entries`, data);
  }
  async postJournalEntry(tenantId, entryId) {
    return this.request(
      "POST",
      `/api/v1/tenants/${tenantId}/journal-entries/${entryId}/post`
    );
  }
  async voidJournalEntry(tenantId, entryId, reason) {
    return this.request(
      "POST",
      `/api/v1/tenants/${tenantId}/journal-entries/${entryId}/void`,
      { reason }
    );
  }
  // Report endpoints
  async getTrialBalance(tenantId, asOfDate) {
    const query = asOfDate ? `?as_of_date=${asOfDate}` : "";
    return this.request(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/trial-balance${query}`
    );
  }
  async getAccountBalance(tenantId, accountId, asOfDate) {
    const query = asOfDate ? `?as_of_date=${asOfDate}` : "";
    return this.request(
      "GET",
      `/api/v1/tenants/${tenantId}/reports/account-balance/${accountId}${query}`
    );
  }
  // Contact endpoints
  async listContacts(tenantId, filter) {
    const params = new URLSearchParams();
    if (filter?.type) params.set("type", filter.type);
    if (filter?.active_only) params.set("active_only", "true");
    if (filter?.search) params.set("search", filter.search);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request("GET", `/api/v1/tenants/${tenantId}/contacts${query}`);
  }
  async createContact(tenantId, data) {
    return this.request("POST", `/api/v1/tenants/${tenantId}/contacts`, data);
  }
  async getContact(tenantId, contactId) {
    return this.request("GET", `/api/v1/tenants/${tenantId}/contacts/${contactId}`);
  }
  async updateContact(tenantId, contactId, data) {
    return this.request("PUT", `/api/v1/tenants/${tenantId}/contacts/${contactId}`, data);
  }
  async deleteContact(tenantId, contactId) {
    return this.request(
      "DELETE",
      `/api/v1/tenants/${tenantId}/contacts/${contactId}`
    );
  }
  // Invoice endpoints
  async listInvoices(tenantId, filter) {
    const params = new URLSearchParams();
    if (filter?.type) params.set("type", filter.type);
    if (filter?.status) params.set("status", filter.status);
    if (filter?.contact_id) params.set("contact_id", filter.contact_id);
    if (filter?.from_date) params.set("from_date", filter.from_date);
    if (filter?.to_date) params.set("to_date", filter.to_date);
    if (filter?.search) params.set("search", filter.search);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request("GET", `/api/v1/tenants/${tenantId}/invoices${query}`);
  }
  async createInvoice(tenantId, data) {
    return this.request("POST", `/api/v1/tenants/${tenantId}/invoices`, data);
  }
  async getInvoice(tenantId, invoiceId) {
    return this.request("GET", `/api/v1/tenants/${tenantId}/invoices/${invoiceId}`);
  }
  async sendInvoice(tenantId, invoiceId) {
    return this.request(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/send`
    );
  }
  async voidInvoice(tenantId, invoiceId) {
    return this.request(
      "POST",
      `/api/v1/tenants/${tenantId}/invoices/${invoiceId}/void`
    );
  }
  // Payment endpoints
  async listPayments(tenantId, filter) {
    const params = new URLSearchParams();
    if (filter?.type) params.set("type", filter.type);
    if (filter?.contact_id) params.set("contact_id", filter.contact_id);
    if (filter?.from_date) params.set("from_date", filter.from_date);
    if (filter?.to_date) params.set("to_date", filter.to_date);
    const query = params.toString() ? `?${params.toString()}` : "";
    return this.request("GET", `/api/v1/tenants/${tenantId}/payments${query}`);
  }
  async createPayment(tenantId, data) {
    return this.request("POST", `/api/v1/tenants/${tenantId}/payments`, data);
  }
  async getPayment(tenantId, paymentId) {
    return this.request("GET", `/api/v1/tenants/${tenantId}/payments/${paymentId}`);
  }
  async allocatePayment(tenantId, paymentId, invoiceId, amount) {
    return this.request(
      "POST",
      `/api/v1/tenants/${tenantId}/payments/${paymentId}/allocate`,
      { invoice_id: invoiceId, amount }
    );
  }
  async getUnallocatedPayments(tenantId, type = "RECEIVED") {
    return this.request(
      "GET",
      `/api/v1/tenants/${tenantId}/payments/unallocated?type=${type}`
    );
  }
}
const api = new ApiClient();

export { api as a };
//# sourceMappingURL=api-BA0AUtWR.js.map
