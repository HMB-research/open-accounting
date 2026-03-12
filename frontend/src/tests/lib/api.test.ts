import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import Decimal from "decimal.js";
import { api } from "$lib/api";

// Mock fetch globally for API tests
const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

describe("API Client - Core Functionality", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.clearTokens();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("Token Management", () => {
    it("should start without authentication", () => {
      expect(api.isAuthenticated).toBe(false);
    });

    it("should set tokens correctly", () => {
      api.setTokens("access-token-123", "refresh-token-456");
      expect(api.isAuthenticated).toBe(true);
    });

    it("should clear tokens correctly", () => {
      api.setTokens("access-token-123", "refresh-token-456");
      api.clearTokens();
      expect(api.isAuthenticated).toBe(false);
    });

    it("should logout correctly", () => {
      api.setTokens("access-token-123", "refresh-token-456");
      api.logout();
      expect(api.isAuthenticated).toBe(false);
    });
  });

  describe("Authentication Endpoints", () => {
    it("should register a new user", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          id: "user-123",
          email: "test@example.com",
          name: "Test User",
        }),
      });

      const result = await api.register(
        "test@example.com",
        "password123",
        "Test User",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/auth/register"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            email: "test@example.com",
            password: "password123",
            name: "Test User",
          }),
        }),
      );
      expect(result.email).toBe("test@example.com");
    });

    it("should login and set tokens", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "access-123",
          refresh_token: "refresh-456",
          token_type: "Bearer",
          expires_in: 3600,
          user: { id: "user-123", email: "test@example.com", name: "Test" },
        }),
      });

      const result = await api.login("test@example.com", "password");

      expect(result.access_token).toBe("access-123");
      expect(api.isAuthenticated).toBe(true);
    });

    it("should login with tenant ID", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          access_token: "access-123",
          refresh_token: "refresh-456",
          token_type: "Bearer",
          expires_in: 3600,
          user: { id: "user-123", email: "test@example.com", name: "Test" },
        }),
      });

      await api.login("test@example.com", "password", false, "tenant-id-123");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          body: JSON.stringify({
            email: "test@example.com",
            password: "password",
            tenant_id: "tenant-id-123",
          }),
        }),
      );
    });
  });

  describe("Error Handling", () => {
    it("should throw error on failed request", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        json: async () => ({ error: "Invalid request" }),
      });

      await expect(api.register("bad@email", "pass", "Name")).rejects.toThrow(
        "Invalid request",
      );
    });

    it("should throw generic error if no error message", async () => {
      // Mock 4 responses (initial + 3 retries) for 5xx error with retry logic
      const errorResponse = {
        ok: false,
        status: 500,
        json: async () => ({}),
      };
      mockFetch
        .mockResolvedValueOnce(errorResponse)
        .mockResolvedValueOnce(errorResponse)
        .mockResolvedValueOnce(errorResponse)
        .mockResolvedValueOnce(errorResponse);

      await expect(api.register("bad@email", "pass", "Name")).rejects.toThrow(
        "Request failed",
      );
    }, 30000); // Increase timeout to account for retry delays
  });

  describe("Decimal Parsing", () => {
    it("should parse decimal strings in response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          amount: "1234.56",
          name: "Test Invoice",
          lines: [{ total: "100.00" }, { total: "200.50" }],
        }),
      });

      api.setTokens("token", "refresh");
      const result = (await api.getInvoice(
        "tenant-123",
        "invoice-456",
      )) as unknown as { amount: Decimal; lines: Array<{ total: Decimal }> };

      expect(result.amount).toBeInstanceOf(Decimal);
      expect(result.amount.toString()).toBe("1234.56");
      expect(result.lines[0].total).toBeInstanceOf(Decimal);
    });
  });

  describe("User Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should get current user", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          id: "user-123",
          email: "test@example.com",
          name: "Test",
          created_at: "2024-01-01",
        }),
      });

      const result = await api.getCurrentUser();

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/me"),
        expect.objectContaining({
          method: "GET",
          headers: expect.objectContaining({
            Authorization: "Bearer valid-token",
          }),
        }),
      );
      expect(result.email).toBe("test@example.com");
    });

    it("should get user tenants", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [
          {
            tenant: { id: "tenant-1", name: "Tenant One" },
            role: "admin",
            is_default: true,
          },
        ],
      });

      const result = await api.getMyTenants();

      expect(result).toHaveLength(1);
      expect(result[0].role).toBe("admin");
    });
  });

  describe("Tenant Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should create tenant", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({
          id: "tenant-new",
          name: "New Tenant",
          slug: "new-tenant",
        }),
      });

      const result = await api.createTenant("New Tenant", "new-tenant");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants"),
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining("New Tenant"),
        }),
      );
      expect(result.name).toBe("New Tenant");
    });

    it("should get tenant", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "tenant-123", name: "My Tenant" }),
      });

      const result = await api.getTenant("tenant-123");

      expect(result.id).toBe("tenant-123");
    });

    it("should update tenant", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "tenant-123", name: "Updated Tenant" }),
      });

      const result = await api.updateTenant("tenant-123", {
        name: "Updated Tenant",
      });

      expect(result.name).toBe("Updated Tenant");
    });

    it("should complete onboarding", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ success: true }),
      });

      const result = await api.completeOnboarding("tenant-123");

      expect(result.success).toBe(true);
    });

    it("should list period close events", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [
          { id: "evt-1", action: "close", period_end_date: "2026-01-31" },
        ],
      });

      const result = await api.listPeriodCloseEvents("tenant-123", 10);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/period-close-events?limit=10"),
        expect.objectContaining({ method: "GET" }),
      );
      expect(result).toHaveLength(1);
    });

    it("should close a period", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          tenant: { id: "tenant-123", name: "Tenant" },
          event: { id: "evt-1", action: "close", period_end_date: "2026-01-31" },
        }),
      });

      const result = await api.closePeriod("tenant-123", {
        period_end_date: "2026-01-31",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/period-close"),
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining("2026-01-31"),
        }),
      );
      expect(result.event.action).toBe("close");
    });

    it("should reopen a period", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          tenant: { id: "tenant-123", name: "Tenant" },
          event: { id: "evt-2", action: "reopen", period_end_date: "2026-01-31" },
        }),
      });

      const result = await api.reopenPeriod("tenant-123", {
        period_end_date: "2026-01-31",
        note: "Undo close",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/period-reopen"),
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining("Undo close"),
        }),
      );
      expect(result.event.action).toBe("reopen");
    });

    it("should list recent journal entries", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "je-1", entry_number: "JE-001" }],
      });

      const result = await api.listJournalEntries("tenant-123", 25);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/journal-entries?limit=25"),
        expect.objectContaining({ method: "GET" }),
      );
      expect(result[0].entry_number).toBe("JE-001");
    });

    it("should list documents for an entity", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "doc-1", entity_type: "invoice", entity_id: "inv-1" }],
      });

      const result = await api.listDocuments("tenant-123", "invoice", "inv-1");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/documents?entity_type=invoice&entity_id=inv-1"),
        expect.objectContaining({ method: "GET" }),
      );
      expect(result[0].id).toBe("doc-1");
    });

    it("should upload documents with multipart form data", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "doc-1", entity_type: "payment", entity_id: "pay-1" }),
      });

      const file = new File(["receipt"], "receipt.pdf", { type: "application/pdf" });
      await api.uploadDocument("tenant-123", "payment", "pay-1", file);

      expect(mockFetch).toHaveBeenCalledTimes(1);
      const [, requestInit] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(requestInit.method).toBe("POST");
      expect(requestInit.body).toBeInstanceOf(FormData);
      const formData = requestInit.body as FormData;
      expect(formData.get("entity_type")).toBe("payment");
      expect(formData.get("entity_id")).toBe("pay-1");
      expect(formData.get("file")).toBe(file);
    });

    it("should delete a document", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "deleted" }),
      });

      const result = await api.deleteDocument("tenant-123", "doc-1");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/documents/doc-1"),
        expect.objectContaining({ method: "DELETE" }),
      );
      expect(result.status).toBe("deleted");
    });

    it("should download a document and trigger a file save", async () => {
      api.setTokens("valid-token", "refresh-token");

      const blob = new Blob(["receipt"]);
      const createObjectURL = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:doc-1");
      const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => {});
      const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});

      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        blob: async () => blob,
      });

      await api.downloadDocument("tenant-123", "doc-1", "receipt.pdf");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/documents/doc-1/download"),
        expect.objectContaining({
          method: "GET",
          headers: expect.objectContaining({
            Authorization: "Bearer valid-token",
          }),
        }),
      );
      expect(createObjectURL).toHaveBeenCalledWith(blob);
      expect(click).toHaveBeenCalledTimes(1);
      expect(revokeObjectURL).toHaveBeenCalledWith("blob:doc-1");
    });
  });

  describe("Account Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list accounts", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "acc-1", code: "1000", name: "Cash" }],
      });

      const result = await api.listAccounts("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should list active accounts only", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [
          { id: "acc-1", code: "1000", name: "Cash", is_active: true },
        ],
      });

      await api.listAccounts("tenant-123", true);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("active_only=true"),
        expect.any(Object),
      );
    });

    it("should create account", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({
          id: "acc-new",
          code: "ACC-2000",
          name: "Bank",
          account_type: "ASSET",
        }),
      });

      const result = await api.createAccount("tenant-123", {
        code: "ACC-2000",
        name: "Bank",
        account_type: "ASSET",
      });

      expect(result.code).toBe("ACC-2000");
    });

    it("should import accounts", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          file_name: "accounts.csv",
          rows_processed: 2,
          accounts_created: 1,
          rows_skipped: 1,
          errors: [{ row: 3, code: "1100", message: "duplicate code" }],
        }),
      });

      const result = await api.importAccounts("tenant-123", {
        file_name: "accounts.csv",
        csv_content: "code,name,account_type\n1100,Cash,ASSET\n",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/accounts/import"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            file_name: "accounts.csv",
            csv_content: "code,name,account_type\n1100,Cash,ASSET\n",
          }),
        }),
      );
      expect(result.accounts_created).toBe(1);
      expect(String(result.errors?.[0].code)).toBe("1100");
    });

    it("should get account", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "acc-1", code: "1000", name: "Cash" }),
      });

      const result = await api.getAccount("tenant-123", "acc-1");

      expect(result.id).toBe("acc-1");
    });
  });

  describe("Journal Entry Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should get journal entry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          id: "je-1",
          entry_number: "JE-001",
          status: "DRAFT",
        }),
      });

      const result = await api.getJournalEntry("tenant-123", "je-1");

      expect(result.entry_number).toBe("JE-001");
    });

    it("should create journal entry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({
          id: "je-new",
          entry_number: "JE-002",
          status: "DRAFT",
        }),
      });

      const result = await api.createJournalEntry("tenant-123", {
        entry_date: "2024-01-15",
        description: "Test entry",
        lines: [
          { account_id: "acc-1", debit_amount: "100", credit_amount: "0" },
          { account_id: "acc-2", debit_amount: "0", credit_amount: "100" },
        ],
      });

      expect(result.entry_number).toBe("JE-002");
    });

    it("should import opening balances", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          file_name: "opening-balances.csv",
          rows_processed: 2,
          lines_imported: 2,
          total_debit: "1000.00",
          total_credit: "1000.00",
          journal_entry: {
            id: "je-opening",
            entry_number: "JE-003",
            status: "POSTED",
            source_type: "OPENING_BALANCE",
            lines: [],
          },
        }),
      });

      const result = await api.importOpeningBalances("tenant-123", {
        file_name: "opening-balances.csv",
        entry_date: "2026-01-01",
        description: "Opening balances",
        reference: "OB-2026",
        csv_content: "account_code,debit,credit\n1000,1000,0\n3000,0,1000\n",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining(
          "/api/v1/tenants/tenant-123/journal-entries/import-opening-balances",
        ),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            file_name: "opening-balances.csv",
            entry_date: "2026-01-01",
            description: "Opening balances",
            reference: "OB-2026",
            csv_content:
              "account_code,debit,credit\n1000,1000,0\n3000,0,1000\n",
          }),
        }),
      );
      expect(result.journal_entry.entry_number).toBe("JE-003");
      expect(result.total_debit).toBeInstanceOf(Decimal);
      expect(result.total_debit.toString()).toBe("1000");
    });

    it("should post journal entry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "POSTED" }),
      });

      const result = await api.postJournalEntry("tenant-123", "je-1");

      expect(result.status).toBe("POSTED");
    });

    it("should void journal entry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "je-1", status: "VOIDED" }),
      });

      const result = await api.voidJournalEntry(
        "tenant-123",
        "je-1",
        "Duplicate entry",
      );

      expect(result.status).toBe("VOIDED");
    });
  });

  describe("Report Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should get trial balance", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          is_balanced: true,
          total_debits: "1000",
          total_credits: "1000",
        }),
      });

      const result = await api.getTrialBalance("tenant-123");

      expect(result.is_balanced).toBe(true);
    });

    it("should get trial balance with date", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ is_balanced: true }),
      });

      await api.getTrialBalance("tenant-123", "2024-01-31");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("as_of_date=2024-01-31"),
        expect.any(Object),
      );
    });

    it("should get account balance", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ account_id: "acc-1", balance: "5000" }),
      });

      const result = await api.getAccountBalance("tenant-123", "acc-1");

      expect(result.account_id).toBe("acc-1");
    });

    it("should get balance sheet", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ is_balanced: true, total_assets: "10000" }),
      });

      const result = await api.getBalanceSheet("tenant-123");

      expect(result.is_balanced).toBe(true);
    });

    it("should get income statement", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ net_income: "5000" }),
      });

      const result = await api.getIncomeStatement(
        "tenant-123",
        "2024-01-01",
        "2024-12-31",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("start=2024-01-01"),
        expect.any(Object),
      );
    });
  });

  describe("Contact Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list contacts", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "contact-1", name: "Customer A" }],
      });

      const result = await api.listContacts("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should list contacts with filters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [],
      });

      await api.listContacts("tenant-123", {
        type: "CUSTOMER",
        active_only: true,
        search: "test",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringMatching(/type=CUSTOMER.*active_only=true.*search=test/),
        expect.any(Object),
      );
    });

    it("should create contact", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "contact-new", name: "New Customer" }),
      });

      const result = await api.createContact("tenant-123", {
        name: "New Customer",
        contact_type: "CUSTOMER",
      });

      expect(result.name).toBe("New Customer");
    });

    it("should import contacts", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          file_name: "contacts.csv",
          rows_processed: 2,
          contacts_created: 1,
          rows_skipped: 1,
          errors: [
            { row: 3, name: "Duplicate Customer", message: "duplicate name" },
          ],
        }),
      });

      const result = await api.importContacts("tenant-123", {
        file_name: "contacts.csv",
        csv_content: "name,email\nCustomer A,a@example.com\n",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/contacts/import"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            file_name: "contacts.csv",
            csv_content: "name,email\nCustomer A,a@example.com\n",
          }),
        }),
      );
      expect(result.contacts_created).toBe(1);
      expect(result.errors?.[0].row).toBe(3);
    });

    it("should get contact", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "contact-1", name: "Customer A" }),
      });

      const result = await api.getContact("tenant-123", "contact-1");

      expect(result.id).toBe("contact-1");
    });

    it("should update contact", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "contact-1", name: "Updated Customer" }),
      });

      const result = await api.updateContact("tenant-123", "contact-1", {
        name: "Updated Customer",
      });

      expect(result.name).toBe("Updated Customer");
    });

    it("should delete contact", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "deleted" }),
      });

      const result = await api.deleteContact("tenant-123", "contact-1");

      expect(result.status).toBe("deleted");
    });
  });

  describe("Invoice Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list invoices", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "inv-1", invoice_number: "INV-001" }],
      });

      const result = await api.listInvoices("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should list invoices with filters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [],
      });

      await api.listInvoices("tenant-123", {
        type: "SALES",
        status: "PAID",
        contact_id: "contact-1",
        from_date: "2024-01-01",
        to_date: "2024-12-31",
        search: "test",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("type=SALES"),
        expect.any(Object),
      );
    });

    it("should create invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "inv-new", invoice_number: "INV-002" }),
      });

      const result = await api.createInvoice("tenant-123", {
        invoice_type: "SALES",
        contact_id: "contact-1",
        issue_date: "2024-01-15",
        due_date: "2024-02-15",
        lines: [
          {
            description: "Service",
            quantity: "1",
            unit_price: "100",
            vat_rate: "22",
          },
        ],
      });

      expect(result.invoice_number).toBe("INV-002");
    });

    it("should import invoices", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          file_name: "invoices.csv",
          rows_processed: 2,
          invoices_created: 1,
          lines_imported: 2,
          rows_skipped: 0,
        }),
      });

      const result = await api.importInvoices("tenant-123", {
        file_name: "invoices.csv",
        csv_content:
          "invoice_number,invoice_type,contact_name,issue_date,due_date,line_description,quantity,unit_price,vat_rate\n" +
          "INV-EXT-001,SALES,Acme Corp,2026-02-01,2026-02-15,Implementation work,1,100.00,22\n",
      });

      expect(result.invoices_created).toBe(1);
      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/invoices/import"),
        expect.objectContaining({
          method: "POST",
        }),
      );
    });

    it("should get invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "inv-1", invoice_number: "INV-001" }),
      });

      const result = await api.getInvoice("tenant-123", "inv-1");

      expect(result.invoice_number).toBe("INV-001");
    });

    it("should send invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "sent" }),
      });

      const result = await api.sendInvoice("tenant-123", "inv-1");

      expect(result.status).toBe("sent");
    });

    it("should void invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "voided" }),
      });

      const result = await api.voidInvoice("tenant-123", "inv-1");

      expect(result.status).toBe("voided");
    });
  });

  describe("Payment Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list payments", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "pay-1", payment_number: "PAY-001" }],
      });

      const result = await api.listPayments("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should list payments with filters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [],
      });

      await api.listPayments("tenant-123", {
        type: "RECEIVED",
        contact_id: "contact-1",
        from_date: "2024-01-01",
        to_date: "2024-12-31",
      });

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("type=RECEIVED"),
        expect.any(Object),
      );
    });

    it("should create payment", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "pay-new", payment_number: "PAY-002" }),
      });

      const result = await api.createPayment("tenant-123", {
        payment_type: "RECEIVED",
        payment_date: "2024-01-20",
        amount: "500",
      });

      expect(result.payment_number).toBe("PAY-002");
    });

    it("should get payment", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "pay-1", payment_number: "PAY-001" }),
      });

      const result = await api.getPayment("tenant-123", "pay-1");

      expect(result.payment_number).toBe("PAY-001");
    });

    it("should allocate payment", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "allocated" }),
      });

      const result = await api.allocatePayment(
        "tenant-123",
        "pay-1",
        "inv-1",
        "250",
      );

      expect(result.status).toBe("allocated");
    });

    it("should get unallocated payments", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "pay-1" }],
      });

      await api.getUnallocatedPayments("tenant-123", "RECEIVED");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("type=RECEIVED"),
        expect.any(Object),
      );
    });
  });

  describe("Analytics Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should get dashboard summary", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ total_revenue: "10000", net_income: "5000" }),
      });

      const result = await api.getDashboardSummary("tenant-123");

      expect(result).toBeDefined();
    });

    it("should get revenue expense chart", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          labels: ["Jan", "Feb"],
          revenue: ["1000", "1200"],
        }),
      });

      const result = await api.getRevenueExpenseChart("tenant-123", 6);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("months=6"),
        expect.any(Object),
      );
    });

    it("should get cash flow chart", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ labels: ["Jan"], inflows: ["5000"] }),
      });

      await api.getCashFlowChart("tenant-123");

      expect(mockFetch).toHaveBeenCalled();
    });

    it("should get receivables aging", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ total: "5000", buckets: [] }),
      });

      const result = await api.getReceivablesAging("tenant-123");

      expect(result).toBeDefined();
    });

    it("should get payables aging", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ total: "3000", buckets: [] }),
      });

      const result = await api.getPayablesAging("tenant-123");

      expect(result).toBeDefined();
    });
  });

  describe("Recurring Invoice Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list recurring invoices", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "rec-1", name: "Monthly Subscription" }],
      });

      const result = await api.listRecurringInvoices("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should list active recurring invoices only", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [],
      });

      await api.listRecurringInvoices("tenant-123", true);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("active_only=true"),
        expect.any(Object),
      );
    });

    it("should create recurring invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "rec-new", name: "New Subscription" }),
      });

      const result = await api.createRecurringInvoice("tenant-123", {
        name: "New Subscription",
        contact_id: "contact-1",
        frequency: "MONTHLY",
        start_date: "2024-01-01",
        lines: [
          {
            description: "Service",
            quantity: "1",
            unit_price: "100",
            vat_rate: "22",
          },
        ],
      });

      expect(result.name).toBe("New Subscription");
    });

    it("should create recurring from invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "rec-new" }),
      });

      const result = await api.createRecurringInvoiceFromInvoice(
        "tenant-123",
        "inv-1",
        {
          name: "From Invoice",
          frequency: "MONTHLY",
          start_date: "2024-02-01",
        },
      );

      expect(result.id).toBe("rec-new");
    });

    it("should get recurring invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "rec-1", name: "Monthly Subscription" }),
      });

      const result = await api.getRecurringInvoice("tenant-123", "rec-1");

      expect(result.name).toBe("Monthly Subscription");
    });

    it("should update recurring invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "rec-1", name: "Updated Subscription" }),
      });

      const result = await api.updateRecurringInvoice("tenant-123", "rec-1", {
        name: "Updated Subscription",
      });

      expect(result.name).toBe("Updated Subscription");
    });

    it("should delete recurring invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "deleted" }),
      });

      const result = await api.deleteRecurringInvoice("tenant-123", "rec-1");

      expect(result.status).toBe("deleted");
    });

    it("should pause recurring invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "paused" }),
      });

      const result = await api.pauseRecurringInvoice("tenant-123", "rec-1");

      expect(result.status).toBe("paused");
    });

    it("should resume recurring invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "resumed" }),
      });

      const result = await api.resumeRecurringInvoice("tenant-123", "rec-1");

      expect(result.status).toBe("resumed");
    });

    it("should generate recurring invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          recurring_invoice_id: "rec-1",
          generated_invoice_id: "inv-new",
        }),
      });

      const result = await api.generateRecurringInvoice("tenant-123", "rec-1");

      expect(result.generated_invoice_id).toBe("inv-new");
    });

    it("should generate due recurring invoices", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [
          { recurring_invoice_id: "rec-1", generated_invoice_id: "inv-new" },
        ],
      });

      const result = await api.generateDueRecurringInvoices("tenant-123");

      expect(result).toHaveLength(1);
    });
  });

  describe("Email Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should get SMTP config", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ smtp_host: "smtp.example.com", smtp_port: "587" }),
      });

      const result = await api.getSMTPConfig("tenant-123");

      expect(result.smtp_host).toBe("smtp.example.com");
    });

    it("should update SMTP config", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "updated" }),
      });

      const result = await api.updateSMTPConfig("tenant-123", {
        smtp_host: "smtp.new.com",
        smtp_port: 587,
        smtp_username: "user",
        smtp_from_email: "from@example.com",
        smtp_from_name: "From Name",
        smtp_use_tls: true,
      });

      expect(result.status).toBe("updated");
    });

    it("should test SMTP", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ success: true, message: "Email sent" }),
      });

      const result = await api.testSMTP("tenant-123", "test@example.com");

      expect(result.success).toBe(true);
    });

    it("should list email templates", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "tmpl-1", template_type: "INVOICE_SEND" }],
      });

      const result = await api.listEmailTemplates("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should update email template", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "tmpl-1", subject: "New Subject" }),
      });

      const result = await api.updateEmailTemplate(
        "tenant-123",
        "INVOICE_SEND",
        {
          subject: "New Subject",
          body_html: "<p>Hello</p>",
          is_active: true,
        },
      );

      expect(result.subject).toBe("New Subject");
    });

    it("should get email log", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "log-1", status: "SENT" }],
      });

      const result = await api.getEmailLog("tenant-123", 25);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("limit=25"),
        expect.any(Object),
      );
    });

    it("should email invoice", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ success: true, log_id: "log-1", message: "Sent" }),
      });

      const result = await api.emailInvoice("tenant-123", "inv-1", {
        recipient_email: "customer@example.com",
        attach_pdf: true,
      });

      expect(result.success).toBe(true);
    });

    it("should email payment receipt", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ success: true, log_id: "log-1", message: "Sent" }),
      });

      const result = await api.emailPaymentReceipt("tenant-123", "pay-1", {
        recipient_email: "customer@example.com",
      });

      expect(result.success).toBe(true);
    });
  });

  describe("Banking Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list bank accounts", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "bank-1", name: "Main Account" }],
      });

      const result = await api.listBankAccounts("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should create bank account", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "bank-new", name: "New Account" }),
      });

      const result = await api.createBankAccount("tenant-123", {
        name: "New Account",
        account_number: "1234567890",
      });

      expect(result.name).toBe("New Account");
    });

    it("should get bank account", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "bank-1", name: "Main Account" }),
      });

      const result = await api.getBankAccount("tenant-123", "bank-1");

      expect(result.name).toBe("Main Account");
    });

    it("should update bank account", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "bank-1", name: "Updated Account" }),
      });

      const result = await api.updateBankAccount("tenant-123", "bank-1", {
        name: "Updated Account",
      });

      expect(result.name).toBe("Updated Account");
    });

    it("should delete bank account", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => undefined,
      });

      await api.deleteBankAccount("tenant-123", "bank-1");

      expect(mockFetch).toHaveBeenCalled();
    });

    it("should list bank transactions", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "txn-1" }],
      });

      const result = await api.listBankTransactions("tenant-123", "bank-1", {
        status: "UNMATCHED",
        from_date: "2024-01-01",
        to_date: "2024-12-31",
      });

      expect(result).toHaveLength(1);
    });

    it("should get bank transaction", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "txn-1" }),
      });

      const result = await api.getBankTransaction("tenant-123", "txn-1");

      expect(result.id).toBe("txn-1");
    });

    it("should import bank transactions", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ import_id: "imp-1", transactions_imported: "5" }),
      });

      const result = await api.importBankTransactions("tenant-123", "bank-1", {
        csv_content: "date,desc,amount\n2024-01-01,Test,100",
        file_name: "statement.csv",
        mapping: {
          date_column: 0,
          description_column: 1,
          amount_column: 2,
          date_format: "YYYY-MM-DD",
          decimal_separator: ".",
          skip_header: true,
        },
      });

      expect(result.import_id).toBe("imp-1");
    });

    it("should get import history", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "imp-1" }],
      });

      const result = await api.getImportHistory("tenant-123", "bank-1");

      expect(result).toHaveLength(1);
    });

    it("should get match suggestions", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ payment_id: "pay-1", confidence: "0.95" }],
      });

      const result = await api.getMatchSuggestions("tenant-123", "txn-1");

      expect(result).toHaveLength(1);
    });

    it("should match bank transaction", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "matched" }),
      });

      const result = await api.matchBankTransaction(
        "tenant-123",
        "txn-1",
        "pay-1",
      );

      expect(result.status).toBe("matched");
    });

    it("should unmatch bank transaction", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "unmatched" }),
      });

      const result = await api.unmatchBankTransaction("tenant-123", "txn-1");

      expect(result.status).toBe("unmatched");
    });

    it("should create payment from transaction", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ payment_id: "pay-new" }),
      });

      const result = await api.createPaymentFromTransaction(
        "tenant-123",
        "txn-1",
      );

      expect(result.payment_id).toBe("pay-new");
    });

    it("should list reconciliations", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "rec-1" }],
      });

      const result = await api.listReconciliations("tenant-123", "bank-1");

      expect(result).toHaveLength(1);
    });

    it("should create reconciliation", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "rec-new" }),
      });

      const result = await api.createReconciliation("tenant-123", "bank-1", {
        statement_date: "2024-01-31",
        opening_balance: "1000",
        closing_balance: "1500",
      });

      expect(result.id).toBe("rec-new");
    });

    it("should get reconciliation", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "rec-1" }),
      });

      const result = await api.getReconciliation("tenant-123", "rec-1");

      expect(result.id).toBe("rec-1");
    });

    it("should complete reconciliation", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "completed" }),
      });

      const result = await api.completeReconciliation("tenant-123", "rec-1");

      expect(result.status).toBe("completed");
    });

    it("should auto match transactions", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ matched: "10" }),
      });

      const result = await api.autoMatchTransactions(
        "tenant-123",
        "bank-1",
        0.8,
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("min_confidence=0.8"),
        expect.any(Object),
      );
    });
  });

  describe("Tax (KMD) Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should generate KMD", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "kmd-1", year: "2024", month: "1" }),
      });

      const result = await api.generateKMD("tenant-123", {
        year: 2024,
        month: 1,
      });

      expect(result.id).toBe("kmd-1");
    });

    it("should list KMD", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "kmd-1" }],
      });

      const result = await api.listKMD("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should get cash flow statement", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          tenant_id: "tenant-123",
          start_date: "2024-01-01",
          end_date: "2024-01-31",
          operating_activities: [],
          investing_activities: [],
          financing_activities: [],
          total_operating: "1000",
          total_investing: "-200",
          total_financing: "300",
          net_cash_change: "1100",
          opening_cash: "500",
          closing_cash: "1600",
          generated_at: "2024-02-01T00:00:00Z",
        }),
      });

      const result = await api.getCashFlowStatement(
        "tenant-123",
        "2024-01-01",
        "2024-01-31",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining(
          "/api/v1/tenants/tenant-123/reports/cash-flow?start_date=2024-01-01&end_date=2024-01-31",
        ),
        expect.any(Object),
      );
      expect(result.net_cash_change.toString()).toBe("1100");
    });

    it("should get balance confirmation summary", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          type: "RECEIVABLE",
          as_of_date: "2024-01-31",
          total_balance: "1234.50",
          contact_count: 2,
          invoice_count: 3,
          contacts: [],
          generated_at: "2024-02-01T00:00:00Z",
        }),
      });

      const result = await api.getBalanceConfirmationSummary(
        "tenant-123",
        "RECEIVABLE",
        "2024-01-31",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining(
          "/api/v1/tenants/tenant-123/reports/balance-confirmations?type=RECEIVABLE&as_of_date=2024-01-31",
        ),
        expect.any(Object),
      );
      expect(result.total_balance.toString()).toBe("1234.5");
    });

    it("should get balance confirmation detail", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          id: "bc-1",
          tenant_id: "tenant-123",
          contact_id: "contact-1",
          contact_name: "Acme",
          type: "PAYABLE",
          as_of_date: "2024-01-31",
          total_balance: "250.00",
          invoices: [],
          generated_at: "2024-02-01T00:00:00Z",
        }),
      });

      const result = await api.getBalanceConfirmation(
        "tenant-123",
        "contact-1",
        "PAYABLE",
        "2024-01-31",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining(
          "/api/v1/tenants/tenant-123/reports/balance-confirmations/contact-1?type=PAYABLE&as_of_date=2024-01-31",
        ),
        expect.any(Object),
      );
      expect(result.total_balance.toString()).toBe("250");
    });

    it("should get overdue invoices summary", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          total_overdue: "89.90",
          invoice_count: 1,
          contact_count: 1,
          average_days_overdue: 12,
          invoices: [{ id: "inv-1" }],
          generated_at: "2024-02-01T00:00:00Z",
        }),
      });

      const result = await api.getOverdueInvoices("tenant-123");

      expect(result.total_overdue.toString()).toBe("89.9");
      expect(result.invoices).toHaveLength(1);
    });

    it("should send payment reminder", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          success: true,
          reminder_id: "rem-1",
          message: "Sent",
        }),
      });

      const result = await api.sendPaymentReminder(
        "tenant-123",
        "inv-1",
        "Please pay",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/invoices/reminders"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            invoice_id: "inv-1",
            message: "Please pay",
          }),
        }),
      );
      expect(result.success).toBe(true);
    });

    it("should send bulk payment reminders", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          total_requested: 2,
          successful: 2,
          failed: 0,
          results: [],
        }),
      });

      const result = await api.sendBulkPaymentReminders(
        "tenant-123",
        ["inv-1", "inv-2"],
        "Bulk notice",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/invoices/reminders/bulk"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            invoice_ids: ["inv-1", "inv-2"],
            message: "Bulk notice",
          }),
        }),
      );
      expect(result.successful).toBe(2);
    });

    it("should get invoice reminder history", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "rem-1" }],
      });

      const result = await api.getInvoiceReminderHistory("tenant-123", "inv-1");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/invoices/inv-1/reminders"),
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });
  });

  describe("Payroll Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list employees", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "emp-1", first_name: "John" }],
      });

      const result = await api.listEmployees("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should create employee", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "emp-new", first_name: "Jane" }),
      });

      const result = await api.createEmployee("tenant-123", {
        first_name: "Jane",
        last_name: "Doe",
        start_date: "2024-01-01",
        employment_type: "FULL_TIME",
        apply_basic_exemption: true,
      });

      expect(result.first_name).toBe("Jane");
    });

    it("should get employee", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "emp-1", first_name: "John" }),
      });

      const result = await api.getEmployee("tenant-123", "emp-1");

      expect(result.first_name).toBe("John");
    });

    it("should update employee", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "emp-1", first_name: "Johnny" }),
      });

      const result = await api.updateEmployee("tenant-123", "emp-1", {
        first_name: "Johnny",
      });

      expect(result.first_name).toBe("Johnny");
    });

    it("should set base salary", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "updated" }),
      });

      const result = await api.setBaseSalary("tenant-123", "emp-1", "5000");

      expect(result.status).toBe("updated");
    });

    it("should list payroll runs", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "run-1" }],
      });

      const result = await api.listPayrollRuns("tenant-123", 2024);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("year=2024"),
        expect.any(Object),
      );
    });

    it("should create payroll run", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "run-new" }),
      });

      const result = await api.createPayrollRun("tenant-123", {
        period_year: 2024,
        period_month: 1,
      });

      expect(result.id).toBe("run-new");
    });

    it("should get payroll run", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "run-1" }),
      });

      const result = await api.getPayrollRun("tenant-123", "run-1");

      expect(result.id).toBe("run-1");
    });

    it("should calculate payroll", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "run-1", status: "CALCULATED" }),
      });

      const result = await api.calculatePayroll("tenant-123", "run-1");

      expect(result.status).toBe("CALCULATED");
    });

    it("should approve payroll", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "run-1", status: "APPROVED" }),
      });

      const result = await api.approvePayroll("tenant-123", "run-1");

      expect(result.status).toBe("APPROVED");
    });

    it("should get payslips", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "slip-1" }],
      });

      const result = await api.getPayslips("tenant-123", "run-1");

      expect(result).toHaveLength(1);
    });

    it("should generate TSD", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "tsd-1" }),
      });

      const result = await api.generateTSD("tenant-123", "run-1");

      expect(result.id).toBe("tsd-1");
    });

    it("should calculate tax preview", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ gross_salary: "5000", net_salary: "3500" }),
      });

      const result = await api.calculateTaxPreview(
        "tenant-123",
        "5000",
        "654",
        "2",
      );

      expect(result).toBeDefined();
    });

    it("should list TSD", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "tsd-1" }],
      });

      const result = await api.listTSD("tenant-123", 2024);

      expect(result).toHaveLength(1);
    });

    it("should get TSD", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "tsd-1" }),
      });

      const result = await api.getTSD("tenant-123", 2024, 1);

      expect(result.id).toBe("tsd-1");
    });

    it("should mark TSD submitted", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "submitted" }),
      });

      const result = await api.markTSDSubmitted(
        "tenant-123",
        2024,
        1,
        "EMTA-REF-123",
      );

      expect(result.status).toBe("submitted");
    });
  });

  describe("Cost Center Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list cost centers with active filter", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "cc-1", name: "Sales" }],
      });

      const result = await api.listCostCenters("tenant-123", true);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/cost-centers?active_only=true"),
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });

    it("should get cost center", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "cc-1", name: "Sales" }),
      });

      const result = await api.getCostCenter("tenant-123", "cc-1");

      expect(result.id).toBe("cc-1");
    });

    it("should create cost center", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "cc-new", name: "Support" }),
      });

      const result = await api.createCostCenter("tenant-123", {
        code: "SUP",
        name: "Support",
        description: "Support team",
        is_active: true,
      });

      expect(result.name).toBe("Support");
    });

    it("should update cost center", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "cc-1", name: "Updated" }),
      });

      const result = await api.updateCostCenter("tenant-123", "cc-1", {
        code: "SUP",
        name: "Updated",
        is_active: true,
      });

      expect(result.name).toBe("Updated");
    });

    it("should delete cost center", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => undefined,
      });

      await api.deleteCostCenter("tenant-123", "cc-1");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/cost-centers/cc-1"),
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("should get cost center report with date filters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          period_start: "2024-01-01",
          period_end: "2024-01-31",
          cost_centers: [],
        }),
      });

      const result = await api.getCostCenterReport(
        "tenant-123",
        "2024-01-01",
        "2024-01-31",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining(
          "/api/v1/tenants/tenant-123/cost-centers/report?start_date=2024-01-01&end_date=2024-01-31",
        ),
        expect.any(Object),
      );
      expect(result.period_end).toBe("2024-01-31");
    });
  });

  describe("Leave Management Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list absence types with active filter", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "abs-1", name: "Vacation" }],
      });

      const result = await api.listAbsenceTypes("tenant-123", true);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/absence-types?active_only=true"),
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });

    it("should get absence type", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "abs-1", name: "Vacation" }),
      });

      const result = await api.getAbsenceType("tenant-123", "abs-1");

      expect(result.id).toBe("abs-1");
    });

    it("should list leave balances for a year", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "lb-1", remaining_days: "10" }],
      });

      const result = await api.listLeaveBalances("tenant-123", "emp-1", 2024);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/employees/emp-1/leave-balances?year=2024"),
        expect.any(Object),
      );
      expect(result[0].remaining_days.toString()).toBe("10");
    });

    it("should get leave balances by year", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ type_id: "vacation", balance_days: "10" }],
      });

      const result = await api.getLeaveBalancesByYear(
        "tenant-123",
        "emp-1",
        2024,
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/employees/emp-1/leave-balances/2024"),
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });

    it("should update leave balance", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "lb-1", remaining_days: "12" }),
      });

      const result = await api.updateLeaveBalance(
        "tenant-123",
        "emp-1",
        2024,
        "vacation",
        { entitled_days: new Decimal("12") },
      );

      expect(result.remaining_days.toString()).toBe("12");
    });

    it("should initialize leave balances", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "lb-1", remaining_days: "15" }],
      });

      const result = await api.initializeLeaveBalances(
        "tenant-123",
        "emp-1",
        2024,
      );

      expect(result[0].remaining_days.toString()).toBe("15");
    });

    it("should list leave records with filters", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "leave-1" }],
      });

      const result = await api.listLeaveRecords("tenant-123", "emp-1", 2024);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/leave-records?employee_id=emp-1&year=2024"),
        expect.any(Object),
      );
      expect(result).toHaveLength(1);
    });

    it("should create leave record", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "leave-new", status: "PENDING" }),
      });

      const result = await api.createLeaveRecord("tenant-123", {
        employee_id: "emp-1",
        absence_type_id: "vacation",
        start_date: "2024-07-01",
        end_date: "2024-07-05",
        total_days: new Decimal("5"),
        working_days: new Decimal("5"),
      });

      expect(result.id).toBe("leave-new");
    });

    it("should get leave record", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "leave-1", status: "PENDING" }),
      });

      const result = await api.getLeaveRecord("tenant-123", "leave-1");

      expect(result.id).toBe("leave-1");
    });

    it("should approve leave record", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "leave-1", status: "APPROVED" }),
      });

      const result = await api.approveLeaveRecord("tenant-123", "leave-1");

      expect(result.status).toBe("APPROVED");
    });

    it("should reject leave record", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "leave-1", status: "REJECTED" }),
      });

      const result = await api.rejectLeaveRecord(
        "tenant-123",
        "leave-1",
        "Missing approval",
      );

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/v1/tenants/tenant-123/leave-records/leave-1/reject"),
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ reason: "Missing approval" }),
        }),
      );
      expect(result.status).toBe("REJECTED");
    });

    it("should cancel leave record", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "leave-1", status: "CANCELLED" }),
      });

      const result = await api.cancelLeaveRecord("tenant-123", "leave-1");

      expect(result.status).toBe("CANCELLED");
    });
  });

  describe("Plugin Endpoints", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
    });

    it("should list plugin registries", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "reg-1", name: "Official" }],
      });

      const result = await api.listPluginRegistries();

      expect(result).toHaveLength(1);
    });

    it("should add plugin registry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "reg-new", name: "Custom" }),
      });

      const result = await api.addPluginRegistry(
        "Custom",
        "https://plugins.example.com",
        "Custom registry",
      );

      expect(result.name).toBe("Custom");
    });

    it("should remove plugin registry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "removed" }),
      });

      const result = await api.removePluginRegistry("reg-1");

      expect(result.status).toBe("removed");
    });

    it("should sync plugin registry", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "synced" }),
      });

      const result = await api.syncPluginRegistry("reg-1");

      expect(result.status).toBe("synced");
    });

    it("should list plugins", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "plugin-1", name: "test-plugin" }],
      });

      const result = await api.listPlugins();

      expect(result).toHaveLength(1);
    });

    it("should search plugins", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ plugin: { name: "test" }, registry: "official" }],
      });

      const result = await api.searchPlugins("test");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("q=test"),
        expect.any(Object),
      );
    });

    it("should get plugin permissions", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          "read:invoices": { name: "Read Invoices", risk: "low" },
        }),
      });

      const result = await api.getPluginPermissions();

      expect(result).toBeDefined();
    });

    it("should install plugin", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 201,
        json: async () => ({ id: "plugin-new" }),
      });

      const result = await api.installPlugin("https://github.com/test/plugin");

      expect(result.id).toBe("plugin-new");
    });

    it("should get plugin", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "plugin-1" }),
      });

      const result = await api.getPlugin("plugin-1");

      expect(result.id).toBe("plugin-1");
    });

    it("should uninstall plugin", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "uninstalled" }),
      });

      const result = await api.uninstallPlugin("plugin-1");

      expect(result.status).toBe("uninstalled");
    });

    it("should enable plugin", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "plugin-1", state: "enabled" }),
      });

      const result = await api.enablePlugin("plugin-1", ["read:invoices"]);

      expect(result.state).toBe("enabled");
    });

    it("should disable plugin", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "plugin-1", state: "disabled" }),
      });

      const result = await api.disablePlugin("plugin-1");

      expect(result.state).toBe("disabled");
    });

    it("should list tenant plugins", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => [{ id: "tp-1", plugin_id: "plugin-1" }],
      });

      const result = await api.listTenantPlugins("tenant-123");

      expect(result).toHaveLength(1);
    });

    it("should enable tenant plugin", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "tp-new", is_enabled: true }),
      });

      const result = await api.enableTenantPlugin("tenant-123", "plugin-1", {
        key: "value",
      });

      expect(result.is_enabled).toBe(true);
    });

    it("should disable tenant plugin", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "disabled" }),
      });

      const result = await api.disableTenantPlugin("tenant-123", "plugin-1");

      expect(result.status).toBe("disabled");
    });

    it("should get tenant plugin settings", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          plugin_id: "plugin-1",
          settings: { key: "value" },
        }),
      });

      const result = await api.getTenantPluginSettings(
        "tenant-123",
        "plugin-1",
      );

      expect(result.plugin_id).toBe("plugin-1");
    });

    it("should update tenant plugin settings", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          plugin_id: "plugin-1",
          settings: { key: "new-value" },
        }),
      });

      const result = await api.updateTenantPluginSettings(
        "tenant-123",
        "plugin-1",
        { key: "new-value" },
      );

      expect(result.settings.key).toBe("new-value");
    });
  });

  describe("PDF Download Functions", () => {
    beforeEach(() => {
      api.setTokens("valid-token", "refresh-token");
      // Mock DOM methods
      vi.stubGlobal("URL", {
        createObjectURL: vi.fn().mockReturnValue("blob:test-url"),
        revokeObjectURL: vi.fn(),
      });
    });

    it("should download invoice PDF", async () => {
      const mockBlob = new Blob(["test pdf content"], {
        type: "application/pdf",
      });
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        blob: async () => mockBlob,
      });

      const mockClick = vi.fn();
      const mockAppendChild = vi.fn();
      const mockRemoveChild = vi.fn();
      vi.spyOn(document, "createElement").mockReturnValue({
        href: "",
        download: "",
        click: mockClick,
      } as unknown as HTMLAnchorElement);
      vi.spyOn(document.body, "appendChild").mockImplementation(
        mockAppendChild,
      );
      vi.spyOn(document.body, "removeChild").mockImplementation(
        mockRemoveChild,
      );

      await api.downloadInvoicePDF("tenant-123", "inv-1", "INV-001");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/invoices/inv-1/pdf"),
        expect.any(Object),
      );
      expect(mockClick).toHaveBeenCalled();
    });

    it("should throw error on failed PDF download", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 404,
      });

      await expect(
        api.downloadInvoicePDF("tenant-123", "inv-1", "INV-001"),
      ).rejects.toThrow("Failed to download PDF");
    });

    it("should download KMD XML", async () => {
      const mockBlob = new Blob(["<xml>test</xml>"], {
        type: "application/xml",
      });
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        blob: async () => mockBlob,
      });

      const mockClick = vi.fn();
      vi.spyOn(document, "createElement").mockReturnValue({
        href: "",
        download: "",
        click: mockClick,
      } as unknown as HTMLAnchorElement);
      vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);
      vi.spyOn(document.body, "removeChild").mockImplementation((node) => node);

      await api.downloadKMDXml("tenant-123", 2024, 1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/tax/kmd/2024/1/xml"),
        expect.any(Object),
      );
    });

    it("should throw error on failed KMD XML download", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      await expect(api.downloadKMDXml("tenant-123", 2024, 1)).rejects.toThrow(
        "Failed to download XML",
      );
    });

    it("should download TSD XML", async () => {
      const mockBlob = new Blob(["<xml>test</xml>"], {
        type: "application/xml",
      });
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        blob: async () => mockBlob,
      });

      const mockClick = vi.fn();
      vi.spyOn(document, "createElement").mockReturnValue({
        href: "",
        download: "",
        click: mockClick,
      } as unknown as HTMLAnchorElement);
      vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);
      vi.spyOn(document.body, "removeChild").mockImplementation((node) => node);

      await api.downloadTSDXml("tenant-123", 2024, 1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/tsd/2024/1/xml"),
        expect.any(Object),
      );
    });

    it("should throw error on failed TSD XML download", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      await expect(api.downloadTSDXml("tenant-123", 2024, 1)).rejects.toThrow(
        "Failed to download TSD XML",
      );
    });

    it("should download TSD CSV", async () => {
      const mockBlob = new Blob(["col1,col2\nval1,val2"], { type: "text/csv" });
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        blob: async () => mockBlob,
      });

      const mockClick = vi.fn();
      vi.spyOn(document, "createElement").mockReturnValue({
        href: "",
        download: "",
        click: mockClick,
      } as unknown as HTMLAnchorElement);
      vi.spyOn(document.body, "appendChild").mockImplementation((node) => node);
      vi.spyOn(document.body, "removeChild").mockImplementation((node) => node);

      await api.downloadTSDCsv("tenant-123", 2024, 1);

      expect(mockFetch).toHaveBeenCalledWith(
        expect.stringContaining("/tsd/2024/1/csv"),
        expect.any(Object),
      );
    });

    it("should throw error on failed TSD CSV download", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      await expect(api.downloadTSDCsv("tenant-123", 2024, 1)).rejects.toThrow(
        "Failed to download TSD CSV",
      );
    });
  });

  describe("Token Refresh Flow", () => {
    it("should refresh token on 401 and retry request", async () => {
      api.setTokens("old-token", "valid-refresh-token");

      // First call returns 401
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: "Token expired" }),
      });

      // Refresh token call
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ access_token: "new-token" }),
      });

      // Retry of original request
      mockFetch.mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ id: "user-123" }),
      });

      const result = await api.getCurrentUser();

      expect(result.id).toBe("user-123");
      expect(mockFetch).toHaveBeenCalledTimes(3);
    });

    it("should throw error if refresh fails", async () => {
      api.setTokens("old-token", "invalid-refresh-token");

      // First call returns 401
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: "Token expired" }),
      });

      // Refresh token call fails
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        json: async () => ({ error: "Invalid refresh token" }),
      });

      await expect(api.getCurrentUser()).rejects.toThrow("Session expired");
      expect(api.isAuthenticated).toBe(false);
    });
  });
});
